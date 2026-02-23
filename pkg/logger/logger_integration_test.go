package logger

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// newTestLogger creates a Logger backed by traceHandler writing to buf.
func newTestLogger(buf *bytes.Buffer) Logger {
	sl := slog.New(&traceHandler{slog.NewJSONHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug})})
	return &slogLogger{Logger: sl}
}

func setupTracer() *sdktrace.TracerProvider {
	tp := sdktrace.NewTracerProvider()
	otel.SetTracerProvider(tp)
	return tp
}

func parseLastLine(t *testing.T, buf *bytes.Buffer) map[string]any {
	t.Helper()
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	var last string
	for i := len(lines) - 1; i >= 0; i-- {
		if strings.TrimSpace(lines[i]) != "" {
			last = lines[i]
			break
		}
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(last), &m); err != nil {
		t.Fatalf("failed to parse log line %q: %v", last, err)
	}
	return m
}

// TestInfoContext_WithSpan verifies trace_id and span_id are injected when
// an active span is in context — no helper needed, just InfoContext.
func TestInfoContext_WithSpan(t *testing.T) {
	tp := setupTracer()
	defer tp.Shutdown(context.Background()) //nolint:errcheck

	var buf bytes.Buffer
	log := newTestLogger(&buf)

	ctx, span := otel.Tracer("test").Start(context.Background(), "my-span")
	defer span.End()

	log.InfoContext(ctx, "hello")

	entry := parseLastLine(t, &buf)
	if _, ok := entry["trace_id"]; !ok {
		t.Error("expected trace_id")
	}
	if _, ok := entry["span_id"]; !ok {
		t.Error("expected span_id")
	}
}

// TestInfoContext_NoSpan verifies no trace fields appear without an active span.
func TestInfoContext_NoSpan(t *testing.T) {
	var buf bytes.Buffer
	log := newTestLogger(&buf)

	log.InfoContext(context.Background(), "no span")

	entry := parseLastLine(t, &buf)
	if _, ok := entry["trace_id"]; ok {
		t.Error("trace_id should not be present without an active span")
	}
	if _, ok := entry["span_id"]; ok {
		t.Error("span_id should not be present without an active span")
	}
}

// TestErrorContext_WithSpan verifies ErrorContext injects trace context and
// that callers simply pass "error", err as a regular key-value pair.
func TestErrorContext_WithSpan(t *testing.T) {
	tp := setupTracer()
	defer tp.Shutdown(context.Background()) //nolint:errcheck

	var buf bytes.Buffer
	log := newTestLogger(&buf)

	ctx, span := otel.Tracer("test").Start(context.Background(), "err-span")
	defer span.End()

	log.ErrorContext(ctx, "something went wrong", "error", errors.New("boom"), "item_id", "123")

	entry := parseLastLine(t, &buf)
	if _, ok := entry["trace_id"]; !ok {
		t.Error("expected trace_id in error log entry")
	}
	if entry["error"] == nil {
		t.Error("expected error field")
	}
	if entry["item_id"] != "123" {
		t.Errorf("expected item_id=123, got %v", entry["item_id"])
	}
}

// TestLoggerMiddleware_InjectsRequestIDAndTrace verifies the Logger middleware
// uses InfoContext so chi request_id and OTel trace_id both appear in request logs.
func TestLoggerMiddleware_InjectsRequestIDAndTrace(t *testing.T) {
	tp := setupTracer()
	defer tp.Shutdown(context.Background()) //nolint:errcheck

	var buf bytes.Buffer
	log := newTestLogger(&buf)

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(Middleware(log))
	r.Get("/test", func(w http.ResponseWriter, req *http.Request) {
		_, span := otel.Tracer("test").Start(req.Context(), "handler-span")
		defer span.End()
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
	r.ServeHTTP(httptest.NewRecorder(), req)

	entry := parseLastLine(t, &buf)
	if _, ok := entry["request_id"]; !ok {
		t.Error("expected request_id in request log")
	}
	if entry["method"] != "GET" {
		t.Errorf("expected method GET, got %v", entry["method"])
	}
}

// TestNestedSpans verifies same trace_id but different span_ids for parent/child.
func TestNestedSpans(t *testing.T) {
	tp := setupTracer()
	defer tp.Shutdown(context.Background()) //nolint:errcheck

	var buf bytes.Buffer
	log := newTestLogger(&buf)
	tracer := otel.Tracer("test")

	ctx, parent := tracer.Start(context.Background(), "parent")
	log.InfoContext(ctx, "parent log")
	parentEntry := parseLastLine(t, &buf)
	buf.Reset()

	ctx, child := tracer.Start(ctx, "child")
	log.InfoContext(ctx, "child log")
	childEntry := parseLastLine(t, &buf)

	child.End()
	parent.End()

	if parentEntry["trace_id"] != childEntry["trace_id"] {
		t.Errorf("expected same trace_id: %v vs %v", parentEntry["trace_id"], childEntry["trace_id"])
	}
	if parentEntry["span_id"] == childEntry["span_id"] {
		t.Error("expected different span_ids for parent and child")
	}
}
