package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/w99betaCODER/Wisp/internal/cluster"
	"github.com/w99betaCODER/Wisp/internal/config"
	"github.com/w99betaCODER/Wisp/internal/model"
	"github.com/w99betaCODER/Wisp/internal/store"
	"github.com/w99betaCODER/Wisp/internal/xray"
)

// testServer builds a Server backed by a memory store and a no-op Xray client,
// bootstrapped with a super-admin (admin/secret) so auth is enabled.
func testServer(t *testing.T) (http.Handler, store.Store) {
	t.Helper()
	st := store.NewMemoryStore()
	cfg := config.Config{
		AdminUser:     "admin",
		AdminPass:     "secret",
		SessionSecret: "test-secret",
		InboundTag:    "vless-reality",
	}
	if err := Bootstrap(st, cfg); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	srv := New(cfg, st, xray.NewNoopClient(), cluster.New(st, nil))
	return srv.Routes(), st
}

// login signs in and returns the session cookie.
func login(t *testing.T, h http.Handler, user, pass string) *http.Cookie {
	t.Helper()
	body, _ := json.Marshal(loginRequest{Username: user, Password: pass})
	req := httptest.NewRequest("POST", "/api/login", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("login %s: got %d, want 200 (%s)", user, rec.Code, rec.Body.String())
	}
	for _, c := range rec.Result().Cookies() {
		if c.Name == sessionCookie {
			return c
		}
	}
	t.Fatalf("login %s: no session cookie set", user)
	return nil
}

// do issues a request authenticated with the given cookie and returns the recorder.
func do(h http.Handler, cookie *http.Cookie, method, path, body string) *httptest.ResponseRecorder {
	var r *http.Request
	if body != "" {
		r = httptest.NewRequest(method, path, strings.NewReader(body))
	} else {
		r = httptest.NewRequest(method, path, nil)
	}
	if cookie != nil {
		r.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	return rec
}

func TestLoginRejectsBadPassword(t *testing.T) {
	h, _ := testServer(t)
	body, _ := json.Marshal(loginRequest{Username: "admin", Password: "wrong"})
	rec := do(h, nil, "POST", "/api/login", string(body))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("got %d, want 401", rec.Code)
	}
}

func TestProtectedRequiresAuth(t *testing.T) {
	h, _ := testServer(t)
	if rec := do(h, nil, "GET", "/api/users", ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated /api/users: got %d, want 401", rec.Code)
	}
}

func TestResellerScoping(t *testing.T) {
	h, _ := testServer(t)
	super := login(t, h, "admin", "secret")

	// Super creates a reseller.
	rec := do(h, super, "POST", "/api/admins", `{"username":"reseller","password":"pw","role":"reseller"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create reseller: got %d (%s)", rec.Code, rec.Body.String())
	}
	reseller := login(t, h, "reseller", "pw")

	// Each creates a user.
	su := mustCreateUser(t, h, super, "super-user@vpn")
	ru := mustCreateUser(t, h, reseller, "reseller-user@vpn")

	// The reseller's user must be owned by the reseller.
	if ru.OwnerID == "" || ru.OwnerID == su.OwnerID {
		t.Fatalf("reseller user owner = %q, want the reseller's id", ru.OwnerID)
	}

	// Super sees both users; reseller sees only their own.
	if got := listUsers(t, h, super); len(got) != 2 {
		t.Fatalf("super sees %d users, want 2", len(got))
	}
	mine := listUsers(t, h, reseller)
	if len(mine) != 1 || mine[0].Email != "reseller-user@vpn" {
		t.Fatalf("reseller sees %d users (%v), want only their own", len(mine), mine)
	}

	// Reseller cannot touch the super's user (404, not 403, to avoid leaking it).
	if rec := do(h, reseller, "GET", "/api/users/"+su.ID, ""); rec.Code != http.StatusNotFound {
		t.Fatalf("reseller GET super's user: got %d, want 404", rec.Code)
	}
	if rec := do(h, reseller, "DELETE", "/api/users/"+su.ID, ""); rec.Code != http.StatusNotFound {
		t.Fatalf("reseller DELETE super's user: got %d, want 404", rec.Code)
	}
}

func TestResellerForbiddenFromInfra(t *testing.T) {
	h, _ := testServer(t)
	super := login(t, h, "admin", "secret")
	do(h, super, "POST", "/api/admins", `{"username":"r","password":"pw","role":"reseller"}`)
	reseller := login(t, h, "r", "pw")

	cases := []struct {
		method, path, body string
	}{
		{"POST", "/api/nodes", `{"name":"n","address":"1.2.3.4:9000"}`},
		{"POST", "/api/plans", `{"name":"p","price_cents":100,"duration_days":30}`},
		{"GET", "/api/admins", ""},
		{"POST", "/api/admins", `{"username":"x","password":"pw"}`},
	}
	for _, c := range cases {
		if rec := do(h, reseller, c.method, c.path, c.body); rec.Code != http.StatusForbidden {
			t.Errorf("reseller %s %s: got %d, want 403", c.method, c.path, rec.Code)
		}
	}
}

func TestOpenModeActsAsSuper(t *testing.T) {
	// No AdminPass → auth disabled → every request acts as super.
	st := store.NewMemoryStore()
	cfg := config.Config{InboundTag: "t"}
	srv := New(cfg, st, xray.NewNoopClient(), cluster.New(st, nil))
	h := srv.Routes()

	if rec := do(h, nil, "GET", "/api/users", ""); rec.Code != http.StatusOK {
		t.Fatalf("open-mode /api/users: got %d, want 200", rec.Code)
	}
	if rec := do(h, nil, "POST", "/api/nodes", `{"name":"n","address":"1.2.3.4:9000"}`); rec.Code != http.StatusCreated {
		t.Fatalf("open-mode create node: got %d, want 201 (%s)", rec.Code, rec.Body.String())
	}
}

// --- helpers -------------------------------------------------------------

func mustCreateUser(t *testing.T, h http.Handler, cookie *http.Cookie, email string) model.User {
	t.Helper()
	rec := do(h, cookie, "POST", "/api/users", `{"email":"`+email+`"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create user %s: got %d (%s)", email, rec.Code, rec.Body.String())
	}
	var u model.User
	if err := json.Unmarshal(rec.Body.Bytes(), &u); err != nil {
		t.Fatalf("decode user: %v", err)
	}
	return u
}

func listUsers(t *testing.T, h http.Handler, cookie *http.Cookie) []model.User {
	t.Helper()
	rec := do(h, cookie, "GET", "/api/users", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("list users: got %d", rec.Code)
	}
	var us []model.User
	if err := json.Unmarshal(rec.Body.Bytes(), &us); err != nil {
		t.Fatalf("decode users: %v", err)
	}
	return us
}
