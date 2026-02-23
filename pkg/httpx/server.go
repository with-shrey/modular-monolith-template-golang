package httpx

import (
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/go-chi/httprate"
	"github.com/unrolled/secure"
)

// ServerConfig holds the options for NewRouter.
type ServerConfig struct {
	ServiceName   string
	IsDevelopment bool
	// CORSAllowedOrigins is a comma-separated list of allowed origins.
	// Pass "*" (dev only) to allow all origins.
	CORSAllowedOrigins string
}

// NewRouter returns a chi.Mux pre-wired with the project's standard middleware
// stack. Pass app-specific middlewares (logger, recovery, sentry, otel) in order;
// they are prepended before the chi built-ins.
//
// Middleware order (outermost → innermost):
//  1. recoveryMiddleware — catches panics that re-panic from sentry
//  2. sentryMiddleware   — captures panics, re-panics (Repanic: true)
//  3. RequestID          — unique X-Request-Id per request
//  4. otelMiddleware     — starts trace span per request
//  5. loggerMiddleware   — logs request + trace_id/span_id
//  6. RealIP             — sets RemoteAddr from X-Forwarded-For
//  7. RateLimit          — 100 req/min per IP
//  8. CORS               — cross-origin preflight and headers
//  9. BodyLimit          — 10 MB request body cap
//  10. Timeout           — 30 s handler deadline
//  11. Security headers   — CSP, HSTS, X-Frame-Options, Permissions-Policy, etc.
func NewRouter(
	cfg ServerConfig,
	loggerMiddleware func(http.Handler) http.Handler,
	recoveryMiddleware func(http.Handler) http.Handler,
	sentryMiddleware func(http.Handler) http.Handler,
	otelMiddleware func(http.Handler) http.Handler,
) *chi.Mux {
	sec := secure.New(secure.Options{
		STSSeconds:            63072000,
		STSIncludeSubdomains:  true,
		FrameDeny:             true,
		ContentTypeNosniff:    true,
		BrowserXssFilter:      true,
		ReferrerPolicy:        "strict-origin-when-cross-origin",
		ContentSecurityPolicy: "default-src 'self'",
		PermissionsPolicy:     "geolocation=(), microphone=(), camera=(), usb=(), magnetometer=(), gyroscope=()",
		IsDevelopment:         cfg.IsDevelopment,
	})

	r := chi.NewRouter()
	r.Use(
		recoveryMiddleware,
		sentryMiddleware,
		middleware.RequestID,
		otelMiddleware,
		loggerMiddleware,
		middleware.RealIP,
		httprate.LimitByIP(100, time.Minute),
		CORSMiddleware(cfg.CORSAllowedOrigins),
		RequestBodyLimit(10<<20), // 10 MB
		middleware.Timeout(30*time.Second),
		sec.Handler,
	)
	return r
}

// CORSMiddleware returns a CORS handler restricted to the given allowed origins.
// allowedOrigins is a comma-separated list (e.g. "https://app.example.com,http://localhost:3000").
// Pass "*" to allow all origins (development only).
func CORSMiddleware(allowedOrigins string) func(http.Handler) http.Handler {
	origins := parseOrigins(allowedOrigins)
	return cors.Handler(cors.Options{
		AllowedOrigins:   origins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Request-Id"},
		ExposedHeaders:   []string{"Link", "X-Request-Id"},
		AllowCredentials: false,
		MaxAge:           300,
	})
}

// parseOrigins splits a comma-separated origins string into a slice, trimming spaces.
func parseOrigins(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p := strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return []string{"*"}
	}
	return out
}

// RequestBodyLimit returns middleware that caps the request body at maxBytes.
// When the limit is exceeded, reads on the body return an error that handlers
// should convert to a 413 response.
func RequestBodyLimit(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			next.ServeHTTP(w, r)
		})
	}
}

// NewServer returns an *http.Server with production-ready timeouts.
func NewServer(addr string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:           addr,
		Handler:        handler,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   30 * time.Second,
		IdleTimeout:    60 * time.Second,
		MaxHeaderBytes: 1 << 20, // 1 MB
	}
}
