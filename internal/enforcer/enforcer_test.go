package enforcer

import (
	"context"
	"testing"
	"time"

	"github.com/w99betaCODER/Wisp/internal/cluster"
	"github.com/w99betaCODER/Wisp/internal/config"
	"github.com/w99betaCODER/Wisp/internal/model"
	"github.com/w99betaCODER/Wisp/internal/store"
)

// fakeXray is a stub xray.Client: it returns canned stats and records which
// users were removed, so tests need no real Xray.
type fakeXray struct {
	stats   map[string]int64
	removed []string
}

func (f *fakeXray) AddUser(context.Context, string, string, string, string) error { return nil }
func (f *fakeXray) RemoveUser(_ context.Context, _, email string) error {
	f.removed = append(f.removed, email)
	return nil
}
func (f *fakeXray) Stats(context.Context, bool) (map[string]int64, error) { return f.stats, nil }
func (f *fakeXray) Close() error                                          { return nil }

func sweepWith(t *testing.T, u model.User, stats map[string]int64) (model.User, *fakeXray) {
	t.Helper()
	st := store.NewMemoryStore()
	if err := st.CreateUser(u); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	fx := &fakeXray{stats: stats}
	// cluster with no nodes => CollectStats/RemoveUser are network no-ops.
	e := New(st, fx, cluster.New(st, nil), config.Config{InboundTag: "t"})
	e.Sweep(context.Background())

	got, err := st.GetUser(u.ID)
	if err != nil {
		t.Fatalf("GetUser: %v", err)
	}
	return got, fx
}

func TestSweep_DisablesOverQuota(t *testing.T) {
	got, fx := sweepWith(t,
		model.User{ID: "1", Email: "a", Enabled: true, DataLimit: 1000},
		map[string]int64{"a": 1500},
	)
	if got.Used != 1500 {
		t.Fatalf("Used = %d, want 1500", got.Used)
	}
	if got.Enabled {
		t.Fatal("user over quota should be disabled")
	}
	if len(fx.removed) != 1 || fx.removed[0] != "a" {
		t.Fatalf("expected RemoveUser(a), got %v", fx.removed)
	}
}

func TestSweep_DisablesExpired(t *testing.T) {
	past := time.Now().Add(-time.Hour)
	got, fx := sweepWith(t,
		model.User{ID: "2", Email: "b", Enabled: true, ExpiresAt: &past},
		map[string]int64{},
	)
	if got.Enabled {
		t.Fatal("expired user should be disabled")
	}
	if len(fx.removed) != 1 || fx.removed[0] != "b" {
		t.Fatalf("expected RemoveUser(b), got %v", fx.removed)
	}
}

func TestSweep_KeepsHealthyUser(t *testing.T) {
	got, fx := sweepWith(t,
		model.User{ID: "3", Email: "c", Enabled: true, DataLimit: 1000},
		map[string]int64{"c": 500},
	)
	if got.Used != 500 {
		t.Fatalf("Used = %d, want 500", got.Used)
	}
	if !got.Enabled {
		t.Fatal("user under quota should stay enabled")
	}
	if len(fx.removed) != 0 {
		t.Fatalf("expected no removals, got %v", fx.removed)
	}
}
