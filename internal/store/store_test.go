package store

import (
	"errors"
	"testing"
	"time"

	"github.com/w99betaCODER/Wisp/internal/model"
)

func TestMemoryStore_CRUD(t *testing.T) {
	s := NewMemoryStore()

	u := model.User{ID: "abc", Email: "a@example.com", CreatedAt: time.Now()}
	if err := s.CreateUser(u); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	got, err := s.GetUser("abc")
	if err != nil {
		t.Fatalf("GetUser: %v", err)
	}
	if got.Email != "a@example.com" {
		t.Fatalf("got email %q, want a@example.com", got.Email)
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
}

func TestMemoryStore_GetMissing(t *testing.T) {
	s := NewMemoryStore()
	if _, err := s.GetUser("nope"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}
