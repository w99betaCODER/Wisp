// Package cluster fans user operations out to every enabled node agent.
package cluster

import (
	"context"
	"crypto/tls"
	"log"
	"time"

	"github.com/w99betaCODER/Wisp/internal/nodeclient"
	"github.com/w99betaCODER/Wisp/internal/store"
)

// Cluster pushes user changes to all enabled nodes listed in the store.
type Cluster struct {
	store   store.Store
	tls     *tls.Config // panel mTLS config; nil => plain HTTP to nodes
	timeout time.Duration
}

// New constructs a Cluster. Pass a nil tlsCfg for plain-HTTP development.
func New(st store.Store, tlsCfg *tls.Config) *Cluster {
	return &Cluster{store: st, tls: tlsCfg, timeout: 10 * time.Second}
}

// AddUser pushes the user to every enabled node.
//
// It is best-effort by design: a node that is down or returns an error is
// logged, not propagated, so a single unhealthy node never blocks a user
// operation. Reconciling any resulting drift is a separate concern (a future
// sync loop), kept out of the request path on purpose.
func (c *Cluster) AddUser(ctx context.Context, email, uuid, flow string) {
	c.each(ctx, "add user "+email, func(ctx context.Context, nc *nodeclient.Client) error {
		return nc.AddUser(ctx, email, uuid, flow)
	})
}

// RemoveUser drops the user from every enabled node (best-effort).
func (c *Cluster) RemoveUser(ctx context.Context, email string) {
	c.each(ctx, "remove user "+email, func(ctx context.Context, nc *nodeclient.Client) error {
		return nc.RemoveUser(ctx, email)
	})
}

// CollectStats polls every enabled node for per-user traffic deltas and sums
// them by email. A node that errors is logged and skipped.
func (c *Cluster) CollectStats(ctx context.Context) map[string]int64 {
	total := make(map[string]int64)
	nodes, err := c.store.ListNodes()
	if err != nil {
		log.Printf("cluster: list nodes: %v", err)
		return total
	}
	for _, n := range nodes {
		if !n.Enabled {
			continue
		}
		nc := nodeclient.New(n.Address, c.tls, c.timeout)
		cctx, cancel := context.WithTimeout(ctx, c.timeout)
		stats, err := nc.Stats(cctx)
		cancel()
		if err != nil {
			log.Printf("cluster: node %q (%s): stats: %v", n.Name, n.Address, err)
			continue
		}
		for email, bytes := range stats {
			total[email] += bytes
		}
	}
	return total
}

// each runs fn against every enabled node, logging per-node failures.
func (c *Cluster) each(ctx context.Context, what string, fn func(context.Context, *nodeclient.Client) error) {
	nodes, err := c.store.ListNodes()
	if err != nil {
		log.Printf("cluster: list nodes: %v", err)
		return
	}
	for _, n := range nodes {
		if !n.Enabled {
			continue
		}
		nc := nodeclient.New(n.Address, c.tls, c.timeout)
		cctx, cancel := context.WithTimeout(ctx, c.timeout)
		if err := fn(cctx, nc); err != nil {
			log.Printf("cluster: node %q (%s): %s: %v", n.Name, n.Address, what, err)
		}
		cancel()
	}
}
