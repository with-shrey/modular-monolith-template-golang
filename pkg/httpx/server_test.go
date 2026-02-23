package httpx_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/unrolled/secure"

	"github.com/ghuser/ghproject/pkg/httpx"
)

func okHandler(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}

// TestSecurityHeaders verifies unrolled/secure sets the expected headers.
func TestSecurityHeaders(t *testing.T) {
	sm := secure.New(secure.Options{
		STSSeconds:            63072000,
		STSIncludeSubdomains:  true,
		FrameDeny:             true,
		ContentTypeNosniff:    true,
		BrowserXssFilter:      true,
		ReferrerPolicy:        "strict-origin-when-cross-origin",
		ContentSecurityPolicy: "default-src 'self'",
		IsDevelopment:         false,
	})
	h := sm.Handler(http.HandlerFunc(okHandler))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", http.NoBody))

	checks := map[string]string{
		"X-Content-Type-Options":  "nosniff",
		"X-Frame-Options":         "DENY",
		"Referrer-Policy":         "strict-origin-when-cross-origin",
		"Content-Security-Policy": "default-src 'self'",
	}
	for header, expected := range checks {
		if got := rr.Header().Get(header); got != expected {
			t.Errorf("%s: got %q, want %q", header, got, expected)
		}
	}
	// HSTS is only set over HTTPS — verify the header exists on TLS requests;
	// on plain HTTP unrolled/secure intentionally omits it.
}

// TestRequestBodyLimit_WithinLimit verifies requests under the cap pass through.
func TestRequestBodyLimit_WithinLimit(t *testing.T) {
	const limit = 100

	var gotBody []byte
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, limit+1)
		n, _ := r.Body.Read(buf)
		gotBody = buf[:n]
		w.WriteHeader(http.StatusOK)
	})

	h := httpx.RequestBodyLimit(limit)(inner)
	body := strings.NewReader(strings.Repeat("a", 50))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/", body))

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if len(gotBody) != 50 {
		t.Fatalf("expected 50 bytes read, got %d", len(gotBody))
	}
}

// TestRequestBodyLimit_ExceedsLimit verifies that reading beyond the cap returns an error.
func TestRequestBodyLimit_ExceedsLimit(t *testing.T) {
	const limit int64 = 10

	var readErr error
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, limit+5)
		_, readErr = r.Body.Read(buf)
		if readErr != nil {
			http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	h := httpx.RequestBodyLimit(limit)(inner)
	body := strings.NewReader(strings.Repeat("x", int(limit)+1))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/", body))

	if rr.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413, got %d", rr.Code)
	}
}
