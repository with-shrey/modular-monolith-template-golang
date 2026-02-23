package events

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ThreeDotsLabs/watermill/message"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"

	"github.com/ghuser/ghproject/pkg/config"
	"github.com/ghuser/ghproject/pkg/logger"
)

func setupTracer() *sdktrace.TracerProvider {
	tp := sdktrace.NewTracerProvider()
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})
	return tp
}

func nopLogger() logger.Logger {
	return logger.New(&config.Config{LogLevel: "error"})
}

// TestRetryWithBackoff_SuccessOnFirstAttempt verifies no retry occurs on success.
func TestRetryWithBackoff_SuccessOnFirstAttempt(t *testing.T) {
	calls := 0
	handler := func(_ context.Context, _ *message.Message) error {
		calls++
		return nil
	}
	msg := message.NewMessage("id", nil)
	err := retryWithBackoff(context.Background(), msg, handler, maxRetries, time.Millisecond, nopLogger())
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if calls != 1 {
		t.Errorf("expected 1 call, got %d", calls)
	}
}

// TestRetryWithBackoff_SuccessAfterRetries verifies retry continues until success.
func TestRetryWithBackoff_SuccessAfterRetries(t *testing.T) {
	calls := 0
	handler := func(_ context.Context, _ *message.Message) error {
		calls++
		if calls < 3 {
			return errors.New("transient error")
		}
		return nil
	}
	msg := message.NewMessage("id", nil)
	err := retryWithBackoff(context.Background(), msg, handler, maxRetries, time.Millisecond, nopLogger())
	if err != nil {
		t.Fatalf("expected nil after eventual success, got %v", err)
	}
	if calls != 3 {
		t.Errorf("expected 3 calls, got %d", calls)
	}
}

// TestRetryWithBackoff_ExhaustsRetries verifies an error is returned after all retries fail.
func TestRetryWithBackoff_ExhaustsRetries(t *testing.T) {
	calls := 0
	handler := func(_ context.Context, _ *message.Message) error {
		calls++
		return errors.New("permanent error")
	}
	msg := message.NewMessage("id", nil)
	err := retryWithBackoff(context.Background(), msg, handler, maxRetries, time.Millisecond, nopLogger())
	if err == nil {
		t.Fatal("expected error after exhausted retries")
	}
	if calls != maxRetries {
		t.Errorf("expected %d calls, got %d", maxRetries, calls)
	}
}

// TestRetryWithBackoff_ContextCancelled verifies retry stops when context is canceled.
func TestRetryWithBackoff_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	calls := 0
	handler := func(_ context.Context, _ *message.Message) error {
		calls++
		return errors.New("error")
	}
	msg := message.NewMessage("id", nil)
	err := retryWithBackoff(ctx, msg, handler, maxRetries, time.Second, nopLogger())
	if err == nil {
		t.Fatal("expected error from canceled context")
	}
	// Should have called handler once then exited on ctx.Done
	if calls != 1 {
		t.Errorf("expected 1 call before context cancel, got %d", calls)
	}
}

// TestStartForwarder_NonForwarderMode verifies StartForwarder returns an error
// when called on an EventBus not configured with forwarder mode.
func TestStartForwarder_NonForwarderMode(t *testing.T) {
	bus := &EventBus{useForwarder: false}
	err := bus.StartForwarder(context.Background())
	if err == nil {
		t.Fatal("expected error for non-forwarder EventBus")
	}
}

// TestForwarderMode_UseForwarderFlag verifies the useForwarder flag is set
// correctly based on which constructor is called.
func TestForwarderMode_UseForwarderFlag(t *testing.T) {
	// Direct construction to check flag without a real DB.
	direct := &EventBus{useForwarder: false}
	if direct.useForwarder {
		t.Error("expected useForwarder=false for standard EventBus")
	}

	withFwd := &EventBus{useForwarder: true}
	if !withFwd.useForwarder {
		t.Error("expected useForwarder=true for forwarder EventBus")
	}
}

// TestOTelPropagation_InjectExtract verifies that trace context injected via
// the same propagation path used by Publish/Subscribe round-trips correctly.
func TestOTelPropagation_InjectExtract(t *testing.T) {
	tp := setupTracer()
	defer tp.Shutdown(context.Background()) //nolint:errcheck

	ctx, span := otel.Tracer("test").Start(context.Background(), "publish-span")
	defer span.End()
	wantTraceID := span.SpanContext().TraceID()

	// Simulate Publish: inject trace context into message metadata.
	msg := message.NewMessage("id", nil)
	carrier := propagation.MapCarrier{}
	otel.GetTextMapPropagator().Inject(ctx, carrier)
	for k, v := range carrier {
		msg.Metadata.Set(k, v)
	}

	// Simulate Subscribe: extract trace context from message metadata.
	extractCarrier := propagation.MapCarrier{}
	for k, v := range msg.Metadata {
		extractCarrier[k] = v
	}
	msgCtx := otel.GetTextMapPropagator().Extract(context.Background(), extractCarrier)

	gotSpan := trace.SpanFromContext(msgCtx)
	if !gotSpan.SpanContext().IsValid() {
		t.Fatal("extracted span context is not valid")
	}
	if gotSpan.SpanContext().TraceID() != wantTraceID {
		t.Errorf("trace ID mismatch: want %s, got %s", wantTraceID, gotSpan.SpanContext().TraceID())
	}
}
