// Package enforcer periodically accounts traffic and disables users that have
// exceeded their data quota or passed their expiry date.
package enforcer

import (
	"context"
	"log"
	"time"

	"github.com/w99betaCODER/Wisp/internal/cluster"
	"github.com/w99betaCODER/Wisp/internal/config"
	"github.com/w99betaCODER/Wisp/internal/model"
	"github.com/w99betaCODER/Wisp/internal/store"
	"github.com/w99betaCODER/Wisp/internal/xray"
)

// Enforcer runs the periodic accounting + quota/expiry sweep.
type Enforcer struct {
	store    store.Store
	xray     xray.Client      // local Xray (single-server); Noop otherwise
	cluster  *cluster.Cluster // remote nodes
	cfg      config.Config
	interval time.Duration
}

// New constructs an Enforcer.
func New(st store.Store, xc xray.Client, cl *cluster.Cluster, cfg config.Config) *Enforcer {
	return &Enforcer{
		store:    st,
		xray:     xc,
		cluster:  cl,
		cfg:      cfg,
		interval: cfg.EnforceInterval,
	}
}

// Run sweeps every interval until ctx is cancelled.
func (e *Enforcer) Run(ctx context.Context) {
	log.Printf("enforcer started (interval %s)", e.interval)
	t := time.NewTicker(e.interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			e.Sweep(ctx)
		}
	}
}

// Sweep performs one accounting + enforcement pass. It is exported so it can
// be triggered directly in tests.
func (e *Enforcer) Sweep(ctx context.Context) {
	deltas := e.collectDeltas(ctx)

	users, err := e.store.ListUsers()
	if err != nil {
		log.Printf("enforcer: list users: %v", err)
		return
	}

	// Index by email and add the traffic each user consumed since last sweep.
	byEmail := make(map[string]model.User, len(users))
	for _, u := range users {
		byEmail[u.Email] = u
	}
	for email, d := range deltas {
		u, ok := byEmail[email]
		if !ok || d <= 0 {
			continue
		}
		u.Used += d
		byEmail[email] = u
		if err := e.store.UpdateUser(u); err != nil {
			log.Printf("enforcer: update usage for %q: %v", email, err)
		}
	}

	// Disable anyone over quota or past expiry.
	now := time.Now().UTC()
	for _, u := range byEmail {
		if !u.Enabled {
			continue
		}
		switch {
		case u.DataLimit > 0 && u.Used >= u.DataLimit:
			e.disable(ctx, u, "data limit reached")
		case u.ExpiresAt != nil && now.After(*u.ExpiresAt):
			e.disable(ctx, u, "subscription expired")
		}
	}
}

// collectDeltas merges traffic deltas from the local Xray and all nodes.
func (e *Enforcer) collectDeltas(ctx context.Context) map[string]int64 {
	deltas := make(map[string]int64)
	if local, err := e.xray.Stats(ctx, true); err != nil {
		log.Printf("enforcer: local xray stats: %v", err)
	} else {
		for email, b := range local {
			deltas[email] += b
		}
	}
	for email, b := range e.cluster.CollectStats(ctx) {
		deltas[email] += b
	}
	return deltas
}

// disable marks a user disabled and removes them from the local Xray and all
// nodes so their access stops immediately.
func (e *Enforcer) disable(ctx context.Context, u model.User, reason string) {
	u.Enabled = false
	if err := e.store.UpdateUser(u); err != nil {
		log.Printf("enforcer: disable %q: %v", u.Email, err)
		return
	}
	if err := e.xray.RemoveUser(ctx, e.cfg.InboundTag, u.Email); err != nil {
		log.Printf("enforcer: local remove %q: %v", u.Email, err)
	}
	e.cluster.RemoveUser(ctx, u.Email)
	log.Printf("enforcer: disabled %q (%s)", u.Email, reason)
}
