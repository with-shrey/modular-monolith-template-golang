package logger

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"runtime/debug"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"go.opentelemetry.io/otel/trace"

	"github.com/ghuser/ghproject/pkg/config"
)

// Logger is the project-wide logging interface. Implementations must provide
// context-aware and plain logging methods plus With for structured attributes.
// The concrete slogLogger embeds *slog.Logger so all standard slog features
// (Log, LogAttrs, Enabled, Handler, etc.) are available on the concrete type.
type Logger interface {
	Info(msg string, args ...any)
	Error(msg string, args ...any)
	Warn(msg string, args ...any)
	Debug(msg string, args ...any)
	InfoContext(ctx context.Context, msg string, args ...any)
	ErrorContext(ctx context.Context, msg string, args ...any)
	WarnContext(ctx context.Context, msg string, args ...any)
	DebugContext(ctx context.Context, msg string, args ...any)
	// With returns a new Logger with the given key-value pairs bound as attributes.
	With(args ...any) Logger
	// ToSlog returns the underlying *slog.Logger for third-party libraries.
	ToSlog() *slog.Logger
}

// New returns a Logger backed by a trace-aware JSON slog handler.
// trace_id, span_id, and request_id are injected from context automatically.
func New(cfg *config.Config) Logger {
	opts := &slog.HandlerOptions{Level: parseLevel(cfg.LogLevel)}
	sl := slog.New(&traceHandler{slog.NewJSONHandler(os.Stdout, opts)})
	return &slogLogger{Logger: sl}
}

// slogLogger embeds *slog.Logger so every slog method (Info, ErrorContext,
// Log, LogAttrs, Enabled, Handler, …) is promoted with zero boilerplate.
// Only With is overridden to return the Logger interface instead of *slog.Logger.
type slogLogger struct {
	*slog.Logger
}

// With returns a new Logger with the given key-value pairs bound as attributes.
func (l *slogLogger) With(args ...any) Logger {
	return &slogLogger{Logger: l.Logger.With(args...)}
}

// ToSlog returns the underlying *slog.Logger for third-party libraries.
func (l *slogLogger) ToSlog() *slog.Logger {
	return l.Logger
}

// traceHandler wraps a slog.Handler and injects OTel trace_id, span_id,
// and chi request_id from context into every log record automatically.
type traceHandler struct {
	slog.Handler
}

func (h *traceHandler) Handle(ctx context.Context, r slog.Record) error {
	if span := trace.SpanFromContext(ctx); span.IsRecording() {
		sc := span.SpanContext()
		r.AddAttrs(
			slog.String("trace_id", sc.TraceID().String()),
			slog.String("span_id", sc.SpanID().String()),
		)
	}
	if requestID := middleware.GetReqID(ctx); requestID != "" {
		r.AddAttrs(slog.String("request_id", requestID))
	}
	return h.Handler.Handle(ctx, r)
}

func (h *traceHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &traceHandler{h.Handler.WithAttrs(attrs)}
}

func (h *traceHandler) WithGroup(name string) slog.Handler {
	return &traceHandler{h.Handler.WithGroup(name)}
}

// Middleware returns a chi-compatible middleware that logs each request.
func Middleware(log Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := &responseWriter{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(ww, r)

			log.InfoContext(r.Context(), "request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", ww.status,
				"latency_ms", time.Since(start).Milliseconds(),
				"remote_addr", r.RemoteAddr,
			)
		})
	}
}

// Recovery returns a chi-compatible middleware that recovers from panics and logs them.
func Recovery(log Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					log.ErrorContext(r.Context(), "panic recovered",
						"error", err,
						"stack", string(debug.Stack()),
					)
					http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// responseWriter wraps http.ResponseWriter to capture the status code.
type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(status int) {
	rw.status = status
	rw.ResponseWriter.WriteHeader(status)
}

func parseLevel(s string) slog.Level {
	switch s {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
