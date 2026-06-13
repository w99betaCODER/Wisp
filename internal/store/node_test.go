package store

import (
	"errors"
	"testing"
	"time"

	"github.com/w99betaCODER/Wisp/internal/model"
)

// stores returns one of each Store implementation so the same test exercises
// both the in-memory and SQLite code paths.
func stores(t *testing.T) map[string]Store {
	t.Helper()
	sq, err := NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { sq.Close() })
	return map[string]Store{
		"memory": NewMemoryStore(),
		"sqlite": sq,
	}
}

func TestNodeCRUD(t *testing.T) {
	for name, s := range stores(t) {
		t.Run(name, func(t *testing.T) {
			n := model.Node{
				ID:        "n1",
				Name:      "nl-amsterdam-1",
				Address:   "1.2.3.4:8443",
				Enabled:   true,
				CreatedAt: time.Now().UTC().Truncate(time.Second),
			}
			if err := s.CreateNode(n); err != nil {
				t.Fatalf("CreateNode: %v", err)
			}

			got, err := s.GetNode("n1")
			if err != nil {
				t.Fatalf("GetNode: %v", err)
			}
			if got.Name != n.Name || got.Address != n.Address || !got.Enabled {
				t.Fatalf("got %+v, want %+v", got, n)
			}

			nodes, err := s.ListNodes()
			if err != nil {
				t.Fatalf("ListNodes: %v", err)
			}
			if len(nodes) != 1 {
				t.Fatalf("got %d nodes, want 1", len(nodes))
			}

			if err := s.DeleteNode("n1"); err != nil {
				t.Fatalf("DeleteNode: %v", err)
			}
			if _, err := s.GetNode("n1"); !errors.Is(err, ErrNotFound) {
				t.Fatalf("expected ErrNotFound after delete, got %v", err)
			}
		})
	}
}
