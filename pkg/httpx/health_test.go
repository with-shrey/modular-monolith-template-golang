package httpx_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ghuser/ghproject/pkg/httpx"
)

type stubChecker struct{ err error }

func (s *stubChecker) Ping(_ context.Context) error { return s.err }

func TestHealthHandler_AllHealthy(t *testing.T) {
	h := httpx.HealthHandler(httpx.HealthChecks{
		Database: &stubChecker{},
		Redis:    &stubChecker{},
		EventBus: &stubChecker{},
	})
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/health", http.NoBody))

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var resp map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["status"] != "ok" {
		t.Errorf("status: got %q, want %q", resp["status"], "ok")
	}
}

func TestHealthHandler_DatabaseDown(t *testing.T) {
	h := httpx.HealthHandler(httpx.HealthChecks{
		Database: &stubChecker{err: errors.New("conn refused")},
		Redis:    &stubChecker{},
		EventBus: &stubChecker{},
	})
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/health", http.NoBody))

	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rr.Code)
	}
	var resp map[string]string
	_ = json.NewDecoder(rr.Body).Decode(&resp)
	if resp["status"] != "degraded" || resp["database"] != "unreachable" {
		t.Errorf("unexpected response: %+v", resp)
	}
}

func TestHealthHandler_RedisDown(t *testing.T) {
	h := httpx.HealthHandler(httpx.HealthChecks{
		Database: &stubChecker{},
		Redis:    &stubChecker{err: errors.New("timeout")},
		EventBus: &stubChecker{},
	})
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/health", http.NoBody))

	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rr.Code)
	}
	var resp map[string]string
	_ = json.NewDecoder(rr.Body).Decode(&resp)
	if resp["status"] != "degraded" || resp["redis"] != "unreachable" {
		t.Errorf("unexpected response: %+v", resp)
	}
}

func TestHealthHandler_EventBusDown(t *testing.T) {
	h := httpx.HealthHandler(httpx.HealthChecks{
		Database: &stubChecker{},
		Redis:    &stubChecker{},
		EventBus: &stubChecker{err: errors.New("timeout")},
	})
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/health", http.NoBody))

	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rr.Code)
	}
	var resp map[string]string
	_ = json.NewDecoder(rr.Body).Decode(&resp)
	if resp["status"] != "degraded" || resp["event_bus"] != "unreachable" {
		t.Errorf("unexpected response: %+v", resp)
	}
}

func TestHealthHandler_AllDown(t *testing.T) {
	h := httpx.HealthHandler(httpx.HealthChecks{
		Database: &stubChecker{err: errors.New("down")},
		Redis:    &stubChecker{err: errors.New("down")},
		EventBus: &stubChecker{err: errors.New("down")},
	})
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/health", http.NoBody))

	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rr.Code)
	}
	var resp map[string]string
	_ = json.NewDecoder(rr.Body).Decode(&resp)
	if resp["database"] != "unreachable" || resp["redis"] != "unreachable" || resp["event_bus"] != "unreachable" {
		t.Errorf("expected all services unreachable: %+v", resp)
	}
}

func TestHealthHandler_ContentType(t *testing.T) {
	h := httpx.HealthHandler(httpx.HealthChecks{
		Database: &stubChecker{},
		Redis:    &stubChecker{},
		EventBus: &stubChecker{},
	})
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/health", http.NoBody))

	ct := rr.Header().Get("Content-Type")
	if ct != "application/json; charset=utf-8" {
		t.Errorf("Content-Type: got %q, want %q", ct, "application/json; charset=utf-8")
	}
}
