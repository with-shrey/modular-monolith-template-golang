package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	httpSwagger "github.com/swaggo/http-swagger/v2"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	_ "github.com/ghuser/ghproject/docs/swagger"
	"github.com/ghuser/ghproject/pkg/app"
	"github.com/ghuser/ghproject/pkg/auth"
	"github.com/ghuser/ghproject/pkg/cache"
	"github.com/ghuser/ghproject/pkg/config"
	"github.com/ghuser/ghproject/pkg/database"
	"github.com/ghuser/ghproject/pkg/events"
	"github.com/ghuser/ghproject/pkg/httpx"
	"github.com/ghuser/ghproject/pkg/logger"
	"github.com/ghuser/ghproject/pkg/telemetry"
	itemApi "github.com/ghuser/ghproject/services/item/application/api"
)

// @title					HastyConnect API
// @version				1.0
// @description			Modular monolith API built with DDD and Clean Architecture.
// @termsOfService			http://swagger.io/terms/
// @contact.name			API Support
// @contact.email			support@hastyconnect.com
// @license.name			MIT
// @license.url			https://opensource.org/licenses/MIT
// @host					localhost:8080
// @BasePath				/api
// @schemes				http https
func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	if err := config.ValidateForProduction(cfg); err != nil {
		slog.Error("production config validation failed", "error", err)
		os.Exit(1)
	}

	log := logger.New(cfg)

	// Telemetry: OTel tracing + metrics
	ctx := context.Background()
	otelShutdown, metricsHandler, err := telemetry.Setup(ctx, cfg)
	if err != nil {
		log.Error("failed to setup otel", "error", err)
		os.Exit(1)
	}
	defer otelShutdown(ctx) //nolint:errcheck

	// Crash reporting: Sentry (optional — log and continue on failure)
	if err := telemetry.SetupSentry(cfg); err != nil {
		log.Warn("failed to setup sentry, continuing without crash reporting", "error", err)
	}
	defer telemetry.SentryFlush()

	pool, err := database.NewPool(ctx, cfg.DefinitionDatabaseURL, log)
	if err != nil {
		log.Error("failed to connect to database", "error", err)
		os.Exit(1) //nolint:gocritic // intentional: startup failure, deferred flushes are best-effort
	}
	defer pool.Close()
	log.Info("database pool connected")

	eventBus, err := events.NewEventBusWithForwarder(cfg, log)
	if err != nil {
		log.Error("failed to setup event bus", "error", err)
		os.Exit(1) //nolint:gocritic
	}
	defer eventBus.Close() //nolint:errcheck

	if err := eventBus.StartForwarder(ctx); err != nil {
		log.Error("failed to start event forwarder", "error", err)
		os.Exit(1) //nolint:gocritic
	}

	redisClient, err := cache.NewRedisClient(cfg)
	if err != nil {
		log.Error("failed to connect to redis", "error", err)
		os.Exit(1) //nolint:gocritic // intentional: startup failure
	}
	defer redisClient.Close() //nolint:errcheck
	log.Info("redis connected")

	//temporalClient, err := workflows.NewTemporalClient(ctx, cfg.TemporalHostPort, cfg.TemporalNamespace, log)
	//if err != nil {
	//	log.Error("failed to initialize temporal client", "error", err)
	//	os.Exit(1) //nolint:gocritic // intentional: startup failure
	//}
	//defer temporalClient.Close()

	sessionStore := auth.NewSessionStore(
		redisClient.Client(),
		[]byte(cfg.SessionAuthKey),
		[]byte(cfg.SessionEncryptionKey),
		cfg.Environment == config.EnvProduction,
	)
	log.Info("session store initialized", "backend", "redis")

	appConfig := &app.Application{
		Db:       pool,
		Logger:   log,
		EventBus: eventBus,
		Redis:    redisClient,
		//TemporalClient: temporalClient,
		SessionStore: sessionStore,
	}

	r := httpx.NewRouter(
		httpx.ServerConfig{
			ServiceName:        cfg.ServiceName,
			IsDevelopment:      cfg.Environment == config.EnvDevelopment,
			CORSAllowedOrigins: cfg.CORSAllowedOrigins,
		},
		logger.Middleware(log),
		logger.Recovery(log),
		telemetry.SentryMiddleware(),
		otelhttp.NewMiddleware(cfg.ServiceName),
	)

	r.Get("/health", httpx.HealthHandler(httpx.HealthChecks{
		Database: pool,
		Redis:    redisClient,
		EventBus: eventBus,
	}))
	r.Get("/metrics", metricsHandler.ServeHTTP)
	r.Get("/swagger/*", httpSwagger.Handler(httpSwagger.URL("/swagger/doc.json")))
	r.Route("/api", func(r chi.Router) {
		//r.Use(auth.RequireAuth(sessionStore, log))
		registerRoutes(r, appConfig)
	})

	srv := httpx.NewServer(":8080", r)

	go func() {
		log.Info("server listening", "addr", srv.Addr, "env", cfg.Environment)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("shutting down...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("forced shutdown", "error", err)
		os.Exit(1)
	}
	log.Info("server stopped")
}

// registerRoutes mounts all service routes under /api.
// Add each new service's route function here.
func registerRoutes(r chi.Router, a *app.Application) {
	itemApi.ItemRoutes(r, a)
}
