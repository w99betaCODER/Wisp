package store

import (
	"errors"
	"testing"
	"time"

	"github.com/w99betaCODER/Wisp/internal/model"
)

func TestSQLiteStore_CRUD(t *testing.T) {
	// ":memory:" gives each test an isolated, disk-free database.
	s, err := NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	defer s.Close()

	exp := time.Now().Add(24 * time.Hour).UTC().Truncate(time.Second)
	u := model.User{
		ID:        "abc",
		Email:     "a@example.com",
		UUID:      "11111111-1111-4111-8111-111111111111",
		Enabled:   true,
		CreatedAt: time.Now().UTC().Truncate(time.Second),
		ExpiresAt: &exp,
	}
	if err := s.CreateUser(u); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	got, err := s.GetUser("abc")
	if err != nil {
		t.Fatalf("GetUser: %v", err)
	}
	if got.Email != u.Email || got.UUID != u.UUID {
		t.Fatalf("got %+v, want email/uuid of %+v", got, u)
	}
	if got.ExpiresAt == nil || !got.ExpiresAt.Equal(exp) {
		t.Fatalf("expires_at round-trip failed: got %v, want %v", got.ExpiresAt, exp)
	}

	users, err := s.ListUsers()
	if err != nil {
		t.Fatalf("ListUsers: %v", err)
	}
	if len(users) != 1 {
		t.Fatalf("got %d users, want 1", len(users))
	}

	if err := s.DeleteUser("abc"); err != nil {
		t.Fatalf("DeleteUser: %v", err)
	}
	if _, err := s.GetUser("abc"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}
	if err := s.DeleteUser("abc"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound deleting missing, got %v", err)
	}
}
