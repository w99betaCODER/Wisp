package billing

import (
	"context"
	"testing"
	"time"

	"github.com/w99betaCODER/Wisp/internal/cluster"
	"github.com/w99betaCODER/Wisp/internal/config"
	"github.com/w99betaCODER/Wisp/internal/model"
	"github.com/w99betaCODER/Wisp/internal/store"
)

type fakeXray struct{ added []string }

func (f *fakeXray) AddUser(_ context.Context, _, email, _, _ string) error {
	f.added = append(f.added, email)
	return nil
}
func (f *fakeXray) RemoveUser(context.Context, string, string) error      { return nil }
func (f *fakeXray) Stats(context.Context, bool) (map[string]int64, error) { return nil, nil }
func (f *fakeXray) Close() error                                          { return nil }

func near(t *testing.T, got *time.Time, want time.Time) {
	t.Helper()
	if got == nil {
		t.Fatal("expiry is nil")
	}
	if d := got.Sub(want); d < -time.Minute || d > time.Minute {
		t.Fatalf("expiry = %v, want ~%v (off by %v)", got, want, d)
	}
}

func TestApply_NewUserStartsFromNow(t *testing.T) {
	st := store.NewMemoryStore()
	_ = st.CreateUser(model.User{ID: "u", Email: "a", Enabled: false, Used: 9999})
	_ = st.CreatePlan(model.Plan{ID: "p", DurationDays: 30, DataLimit: 1000})

	fx := &fakeXray{}
	err := Apply(context.Background(), st, fx, cluster.New(st, nil),
		config.Config{InboundTag: "t"}, model.Order{ID: "o", UserID: "u", PlanID: "p"})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	u, _ := st.GetUser("u")
	if !u.Enabled || u.DataLimit != 1000 || u.Used != 0 {
		t.Fatalf("user not provisioned: %+v", u)
	}
	near(t, u.ExpiresAt, time.Now().UTC().AddDate(0, 0, 30))
	if len(fx.added) != 1 { // was disabled, so re-added to the proxy
		t.Fatalf("expected re-add, got %v", fx.added)
	}
}

func TestApply_ActiveUserStacksDuration(t *testing.T) {
	future := time.Now().UTC().AddDate(0, 0, 10)
	st := store.NewMemoryStore()
	_ = st.CreateUser(model.User{ID: "u", Email: "a", Enabled: true, ExpiresAt: &future})
	_ = st.CreatePlan(model.Plan{ID: "p", DurationDays: 30})

	fx := &fakeXray{}
	err := Apply(context.Background(), st, fx, cluster.New(st, nil),
		config.Config{InboundTag: "t"}, model.Order{ID: "o", UserID: "u", PlanID: "p"})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	u, _ := st.GetUser("u")
	near(t, u.ExpiresAt, future.AddDate(0, 0, 30)) // stacked on remaining time
	if len(fx.added) != 0 {                        // already enabled, no re-add
		t.Fatalf("expected no re-add, got %v", fx.added)
	}
}
