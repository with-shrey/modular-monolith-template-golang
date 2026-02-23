package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/gorilla/sessions"

	"github.com/ghuser/ghproject/pkg/config"
	"github.com/ghuser/ghproject/pkg/logger"
)

// newTestStore returns a gorilla CookieStore (no Redis required) for unit tests.
// In production the RedisStore is used; the sessions.Store interface is identical.
func newTestStore() sessions.Store {
	return sessions.NewCookieStore(
		[]byte("test-auth-key-must-be-32-bytes!!"),
		[]byte("test-enc-key-must-be-32-bytes!!!"),
	)
}

// newTestLogger creates a logger that discards output.
func newTestLogger() logger.Logger {
	return logger.New(&config.Config{LogLevel: "error"})
}

// requestWithSession builds an *http.Request that carries a valid session
// cookie containing the given orgID.
func requestWithSession(t *testing.T, store sessions.Store, orgID uuid.UUID) *http.Request {
	t.Helper()

	// Write the session cookie into a recorder, then copy it to the real request.
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/item", nil)

	session, err := store.Get(r, sessionName)
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	session.Values[sessionOrgIDKey] = orgID.String()
	if err := session.Save(r, w); err != nil {
		t.Fatalf("save session: %v", err)
	}

	// Copy Set-Cookie header from recorder to a fresh request.
	req := httptest.NewRequest(http.MethodPost, "/api/item", nil)
	for _, c := range w.Result().Cookies() {
		req.AddCookie(c)
	}
	return req
}

func TestRequireAuth_ValidSession(t *testing.T) {
	store := newTestStore()
	log := newTestLogger()
	orgID := uuid.New()

	var capturedOrgID uuid.UUID
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedOrgID, _ = OrgIDFromCtx(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	r := requestWithSession(t, store, orgID)
	w := httptest.NewRecorder()
	RequireAuth(store, log)(next).ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if capturedOrgID != orgID {
		t.Fatalf("expected OrgID %v in context, got %v", orgID, capturedOrgID)
	}
}

func TestRequireAuth_MissingCookie(t *testing.T) {
	store := newTestStore()
	log := newTestLogger()

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler should not be called")
	})

	r := httptest.NewRequest(http.MethodPost, "/api/item", nil)
	w := httptest.NewRecorder()
	RequireAuth(store, log)(next).ServeHTTP(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestRequireAuth_SessionMissingOrgID(t *testing.T) {
	store := newTestStore()
	log := newTestLogger()

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler should not be called")
	})

	// Build a session with no org_id value.
	writeReq := httptest.NewRequest(http.MethodPost, "/api/item", nil)
	w1 := httptest.NewRecorder()
	session, _ := store.Get(writeReq, sessionName)
	// intentionally no session.Values[sessionOrgIDKey]
	_ = session.Save(writeReq, w1)

	r := httptest.NewRequest(http.MethodPost, "/api/item", nil)
	for _, c := range w1.Result().Cookies() {
		r.AddCookie(c)
	}

	w := httptest.NewRecorder()
	RequireAuth(store, log)(next).ServeHTTP(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestRequireAuth_InvalidOrgIDInSession(t *testing.T) {
	store := newTestStore()
	log := newTestLogger()

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler should not be called")
	})

	writeReq := httptest.NewRequest(http.MethodPost, "/api/item", nil)
	w1 := httptest.NewRecorder()
	session, _ := store.Get(writeReq, sessionName)
	session.Values[sessionOrgIDKey] = "not-a-valid-uuid"
	_ = session.Save(writeReq, w1)

	r := httptest.NewRequest(http.MethodPost, "/api/item", nil)
	for _, c := range w1.Result().Cookies() {
		r.AddCookie(c)
	}

	w := httptest.NewRecorder()
	RequireAuth(store, log)(next).ServeHTTP(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}
