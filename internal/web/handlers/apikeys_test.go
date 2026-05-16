package webhandlers_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"golang.org/x/crypto/bcrypt"

	dbpkg "probakgo/internal/db"
	"probakgo/internal/session"
	"probakgo/internal/store"
	webhandlers "probakgo/internal/web/handlers"
)

func init() {
	session.Init("test-session-key-32-bytes-long!!", false)
}

func openHandlerDB(t *testing.T) *store.Store {
	t.Helper()
	db, err := dbpkg.Open(":memory:")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return store.New(db)
}

// sessionCookies returns cookies for a logged-in session without touching the HTTP server.
func sessionCookies(t *testing.T, username, role string) []*http.Cookie {
	t.Helper()
	dummy := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	if err := session.SetUser(rr, dummy, username, role); err != nil {
		t.Fatalf("session.SetUser: %v", err)
	}
	return rr.Result().Cookies()
}

// withChiID wraps r with a chi route context containing a single "id" URL param.
func withChiID(r *http.Request, id string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", id)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

// TestCreateAPIKeyPost_EmptyName verifies that a missing name field redirects with a flash message.
func TestCreateAPIKeyPost_EmptyName(t *testing.T) {
	st := openHandlerDB(t)
	h := webhandlers.New(st, nil, nil)

	req := httptest.NewRequest("POST", "/api-keys",
		strings.NewReader("name=&server_name="))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	rr := httptest.NewRecorder()
	h.CreateAPIKeyPost(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("want 303, got %d", rr.Code)
	}
	loc := rr.Header().Get("Location")
	if !strings.Contains(loc, "flash=") {
		t.Errorf("want flash in redirect location, got %q", loc)
	}
}

func TestCreateUserPostWritesAuditLog(t *testing.T) {
	st := openHandlerDB(t)
	h := webhandlers.New(st, nil, nil)

	req := httptest.NewRequest("POST", "/users",
		strings.NewReader("username=alice&password=secret123&role=reader"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for _, c := range sessionCookies(t, "admin", "admin") {
		req.AddCookie(c)
	}

	rr := httptest.NewRecorder()
	h.CreateUserPost(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Fatalf("want 303, got %d", rr.Code)
	}
	rows, err := st.ListAuditLogs(context.Background(), 10)
	if err != nil {
		t.Fatalf("ListAuditLogs: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("audit rows: got %d, want 1", len(rows))
	}
	if rows[0].Action != "user.create" {
		t.Fatalf("action: got %q, want user.create", rows[0].Action)
	}
	if rows[0].ActorUsername != "admin" {
		t.Fatalf("actor: got %q, want admin", rows[0].ActorUsername)
	}
	if strings.Contains(rows[0].Metadata, "secret123") {
		t.Fatal("audit metadata contains the submitted password")
	}
}

// TestRevealAPIKeyPost_InvalidID verifies that a non-numeric id path param returns 400.
func TestRevealAPIKeyPost_InvalidID(t *testing.T) {
	st := openHandlerDB(t)
	h := webhandlers.New(st, nil, nil)

	req := httptest.NewRequest("POST", "/api-keys/abc/reveal", nil)
	req = withChiID(req, "abc")

	rr := httptest.NewRecorder()
	h.RevealAPIKeyPost(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rr.Code)
	}
}

// TestRevealAPIKeyPost_WrongPassword verifies that a wrong password returns 401 JSON.
func TestRevealAPIKeyPost_WrongPassword(t *testing.T) {
	ctx := context.Background()
	st := openHandlerDB(t)

	hash, _ := bcrypt.GenerateFromPassword([]byte("correct-pass"), bcrypt.MinCost)
	if _, err := st.CreateUser(ctx, "alice", string(hash), "admin"); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	k, _ := st.CreateAPIKey(ctx, "my-key", "", "")

	h := webhandlers.New(st, nil, nil)

	body := strings.NewReader("password=wrong-pass")
	req := httptest.NewRequest("POST", "/api-keys/1/reveal", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = withChiID(req, fmt.Sprintf("%d", k.ID))
	for _, c := range sessionCookies(t, "alice", "admin") {
		req.AddCookie(c)
	}

	rr := httptest.NewRecorder()
	h.RevealAPIKeyPost(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", rr.Code)
	}
	var resp map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode JSON: %v", err)
	}
	if resp["error"] == "" {
		t.Error("want non-empty error field in JSON")
	}
}

// TestRevealAPIKeyPost_ValidPassword verifies that valid credentials return the key in JSON.
func TestRevealAPIKeyPost_ValidPassword(t *testing.T) {
	ctx := context.Background()
	st := openHandlerDB(t)

	hash, _ := bcrypt.GenerateFromPassword([]byte("secret"), bcrypt.MinCost)
	if _, err := st.CreateUser(ctx, "bob", string(hash), "admin"); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	k, err := st.CreateAPIKey(ctx, "deploy-key", "", "")
	if err != nil {
		t.Fatalf("CreateAPIKey: %v", err)
	}

	h := webhandlers.New(st, nil, nil)

	body := strings.NewReader("password=secret")
	req := httptest.NewRequest("POST", "/api-keys/reveal", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = withChiID(req, fmt.Sprintf("%d", k.ID))
	for _, c := range sessionCookies(t, "bob", "admin") {
		req.AddCookie(c)
	}

	rr := httptest.NewRecorder()
	h.RevealAPIKeyPost(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("want 200, got %d", rr.Code)
	}
	var resp map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode JSON: %v", err)
	}
	if resp["key"] != k.Key {
		t.Errorf("key: want %q, got %q", k.Key, resp["key"])
	}
}

// TestRevealAPIKeyPost_KeyNotFound verifies that a missing key ID returns 404 JSON.
func TestRevealAPIKeyPost_KeyNotFound(t *testing.T) {
	ctx := context.Background()
	st := openHandlerDB(t)

	hash, _ := bcrypt.GenerateFromPassword([]byte("pass"), bcrypt.MinCost)
	if _, err := st.CreateUser(ctx, "carol", string(hash), "admin"); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	h := webhandlers.New(st, nil, nil)

	body := strings.NewReader("password=pass")
	req := httptest.NewRequest("POST", "/api-keys/9999/reveal", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = withChiID(req, "9999")
	for _, c := range sessionCookies(t, "carol", "admin") {
		req.AddCookie(c)
	}

	rr := httptest.NewRecorder()
	h.RevealAPIKeyPost(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("want 404, got %d", rr.Code)
	}
}
