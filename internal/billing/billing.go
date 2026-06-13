// Package billing applies a paid order to a user: it extends the user's
// expiry by the plan's duration, (re)sets the data quota, clears usage and
// re-enables access. It is the single place the "what a payment does" logic
// lives, shared by the manual-settle endpoint and any payment webhook.
package billing

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/w99betaCODER/Wisp/internal/cluster"
	"github.com/w99betaCODER/Wisp/internal/config"
	"github.com/w99betaCODER/Wisp/internal/model"
	"github.com/w99betaCODER/Wisp/internal/store"
	"github.com/w99betaCODER/Wisp/internal/xray"
)

// Apply settles order: it loads the order's user and plan and grants the plan.
func Apply(ctx context.Context, st store.Store, xc xray.Client, cl *cluster.Cluster, cfg config.Config, order model.Order) error {
	user, err := st.GetUser(order.UserID)
	if err != nil {
		return fmt.Errorf("billing: load user %q: %w", order.UserID, err)
	}
	plan, err := st.GetPlan(order.PlanID)
	if err != nil {
		return fmt.Errorf("billing: load plan %q: %w", order.PlanID, err)
	}
	return Grant(ctx, st, xc, cl, cfg, user, plan)
}

// Grant applies a plan to a user: extends the expiry (from whichever is later —
// now or the current expiry), (re)sets the data quota, clears usage, re-enables
// access and re-syncs to the local Xray and nodes if access had lapsed. Other
// fields on user (e.g. a deducted balance) are persisted as passed in.
func Grant(ctx context.Context, st store.Store, xc xray.Client, cl *cluster.Cluster, cfg config.Config, user model.User, plan model.Plan) error {
	now := time.Now().UTC()
	base := now
	if user.ExpiresAt != nil && user.ExpiresAt.After(now) {
		base = *user.ExpiresAt // still active → stack the new term on top
	}
	expiry := base.AddDate(0, 0, plan.DurationDays)

	wasDisabled := !user.Enabled
	user.ExpiresAt = &expiry
	user.DataLimit = plan.DataLimit
	user.Used = 0
	user.Enabled = true
	if err := st.UpdateUser(user); err != nil {
		return fmt.Errorf("billing: update user: %w", err)
	}

	// If access had lapsed, the user was removed from the proxies — add back.
	if wasDisabled {
		if err := xc.AddUser(ctx, cfg.InboundTag, user.Email, user.UUID, cfg.Node.Flow); err != nil {
			log.Printf("billing: local xray add %q: %v", user.Email, err)
		}
		cl.AddUser(user.Email, user.UUID, cfg.Node.Flow)
	}
	log.Printf("billing: applied plan %q to %q (until %s)", plan.Name, user.Email, expiry.Format(time.DateOnly))
	return nil
}
