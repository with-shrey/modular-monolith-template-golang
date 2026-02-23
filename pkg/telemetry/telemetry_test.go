package telemetry

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ghuser/ghproject/pkg/config"
)

func baseConfig() *config.Config {
	return &config.Config{
		ServiceName:    "test-service",
		ServiceVersion: "test",
		Environment:    "testing",
		OtelEndpoint:   "", // disabled
	}
}

func TestSetup_NoOtelEndpoint(t *testing.T) {
	shutdown, handler, err := Setup(context.Background(), baseConfig())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if shutdown == nil {
		t.Fatal("expected non-nil shutdown")
	}
	if handler == nil {
		t.Fatal("expected non-nil metrics handler")
	}
	if err := shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown error: %v", err)
	}
}

func TestSetup_MetricsHandlerServesPrometheusFormat(t *testing.T) {
	_, handler, err := Setup(context.Background(), baseConfig())
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, httptest.NewRequest("GET", "/metrics", http.NoBody))

	if rr.Code != 200 {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	ct := rr.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/plain") {
		t.Errorf("expected text/plain content-type, got %q", ct)
	}
}
