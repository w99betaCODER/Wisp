// Package cluster fans user operations out to every enabled node agent.
package cluster

import (
	"context"
	"crypto/tls"
	"log"
	"sync"
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

// AddUser pushes the user to every enabled node, concurrently and in the
// background. It returns immediately: the DB and local Xray are authoritative,
// so a slow or unreachable node must never block the request. Per-node failures
// are logged; drift is reconciled by the enforcer's periodic re-sync.
func (c *Cluster) AddUser(email, uuid, flow string) {
	c.dispatch("add user "+email, func(ctx context.Context, nc *nodeclient.Client) error {
		return nc.AddUser(ctx, email, uuid, flow)
	})
}

// RemoveUser drops the user from every enabled node (background, best-effort).
func (c *Cluster) RemoveUser(email string) {
	c.dispatch("remove user "+email, func(ctx context.Context, nc *nodeclient.Client) error {
		return nc.RemoveUser(ctx, email)
	})
}

// dispatch runs fn against every enabled node in its own goroutine and returns
// at once. Each call gets a fresh, detached context bounded by the timeout.
func (c *Cluster) dispatch(what string, fn func(context.Context, *nodeclient.Client) error) {
	nodes, err := c.store.ListNodes()
	if err != nil {
		log.Printf("cluster: list nodes: %v", err)
		return
	}
	for _, n := range nodes {
		if !n.Enabled {
			continue
		}
		n := n
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
			defer cancel()
			nc := nodeclient.New(n.Address, c.tls, c.timeout)
			if err := fn(ctx, nc); err != nil {
				log.Printf("cluster: node %q (%s): %s: %v", n.Name, n.Address, what, err)
			}
		}()
	}
}

// CollectStats polls every enabled node for per-user traffic deltas (in
// parallel) and sums them by email. A node that errors is logged and skipped.
func (c *Cluster) CollectStats(ctx context.Context) map[string]int64 {
	total := make(map[string]int64)
	nodes, err := c.store.ListNodes()
	if err != nil {
		log.Printf("cluster: list nodes: %v", err)
		return total
	}

	var (
		mu sync.Mutex
		wg sync.WaitGroup
	)
	for _, n := range nodes {
		if !n.Enabled {
			continue
		}
		n := n
		wg.Add(1)
		go func() {
			defer wg.Done()
			cctx, cancel := context.WithTimeout(ctx, c.timeout)
			defer cancel()
			stats, err := nodeclient.New(n.Address, c.tls, c.timeout).Stats(cctx)
			if err != nil {
				log.Printf("cluster: node %q (%s): stats: %v", n.Name, n.Address, err)
				return
			}
			mu.Lock()
			for email, bytes := range stats {
				total[email] += bytes
			}
			mu.Unlock()
		}()
	}
	wg.Wait()
	return total
}
