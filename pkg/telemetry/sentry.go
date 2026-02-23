package telemetry

import (
	"fmt"
	"net/http"
	"time"

	"github.com/getsentry/sentry-go"
	sentryhttp "github.com/getsentry/sentry-go/http"

	"github.com/ghuser/ghproject/pkg/config"
)

// SetupSentry initializes the Sentry SDK. No-ops if DSN is empty.
func SetupSentry(cfg *config.Config) error {
	if cfg.SentryDSN == "" {
		return nil
	}
	if err := sentry.Init(sentry.ClientOptions{
		Dsn:              cfg.SentryDSN,
		Environment:      cfg.Environment,
		Release:          cfg.ServiceName + "@" + cfg.ServiceVersion,
		TracesSampleRate: 0.2,
	}); err != nil {
		return fmt.Errorf("sentry init: %w", err)
	}
	return nil
}

// SentryFlush flushes buffered events before process exit.
func SentryFlush() {
	sentry.Flush(2 * time.Second)
}

// SentryMiddleware returns a net/http middleware that captures panics and errors.
// Repanic: true so the outer Recovery middleware still handles the 500 response.
func SentryMiddleware() func(http.Handler) http.Handler {
	h := sentryhttp.New(sentryhttp.Options{Repanic: true})
	return h.Handle
}
