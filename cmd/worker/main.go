package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ThreeDotsLabs/watermill/message"

	"github.com/ghuser/ghproject/pkg/app"
	"github.com/ghuser/ghproject/pkg/cache"
	"github.com/ghuser/ghproject/pkg/config"
	"github.com/ghuser/ghproject/pkg/database"
	"github.com/ghuser/ghproject/pkg/events"
	"github.com/ghuser/ghproject/pkg/logger"
	"github.com/ghuser/ghproject/pkg/telemetry"
	itemEvents "github.com/ghuser/ghproject/services/item/domain/events"
)

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

	ctx := context.Background()

	otelShutdown, _, err := telemetry.Setup(ctx, cfg)
	if err != nil {
		log.Error("failed to setup otel", "error", err)
		os.Exit(1)
	}
	defer otelShutdown(ctx) //nolint:errcheck

	if err := telemetry.SetupSentry(cfg); err != nil {
		log.Warn("failed to setup sentry, continuing without crash reporting", "error", err)
	}
	defer telemetry.SentryFlush()

	pool, err := database.NewPool(ctx, cfg.DefinitionDatabaseURL, log)
	if err != nil {
		log.Error("failed to connect to database", "error", err)
		os.Exit(1) //nolint:gocritic
	}
	defer pool.Close()
	log.Info("database pool connected")

	eventBus, err := events.NewEventBus(cfg, log)
	if err != nil {
		log.Error("failed to setup event bus", "error", err)
		os.Exit(1) //nolint:gocritic
	}
	defer eventBus.Close() //nolint:errcheck

	redisClient, err := cache.NewRedisClient(cfg)
	if err != nil {
		log.Error("failed to connect to redis", "error", err)
		os.Exit(1) //nolint:gocritic
	}
	defer redisClient.Close() //nolint:errcheck
	log.Info("redis connected")

	//temporalClient, err := workflows.NewTemporalClient(ctx, cfg.TemporalHostPort, cfg.TemporalNamespace, log)
	//if err != nil {
	//	log.Error("failed to initialize temporal client", "error", err)
	//	os.Exit(1) //nolint:gocritic
	//}
	//defer temporalClient.Close()

	appConfig := &app.Application{
		Db:       pool,
		Logger:   log,
		EventBus: eventBus,
		Redis:    redisClient,
		//TemporalClient: temporalClient,
	}

	if err := registerSubscribers(ctx, appConfig); err != nil {
		log.Error("failed to register subscribers", "error", err)
		os.Exit(1) //nolint:gocritic
	}

	outboxCtx, cancelOutbox := context.WithCancel(ctx)
	go runOutboxRelay(outboxCtx, appConfig)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("shutting down worker...")
	cancelOutbox()

	// EventBus.Close() (via defer) waits up to 30s for in-flight handlers.
	log.Info("worker stopped")
}

// registerSubscribers wires all domain event handlers.
// Add new topics here as more services publish events.
func registerSubscribers(ctx context.Context, a *app.Application) error {
	errCh, err := a.EventBus.Subscribe(ctx, itemEvents.TopicItemCreated, handleItemCreated(a))
	if err != nil {
		return err
	}

	// Drain subscriber errors in background so the channel never blocks.
	go func() {
		for err := range errCh {
			a.Logger.ErrorContext(ctx, "subscriber error",
				"topic", itemEvents.TopicItemCreated,
				"error", err,
			)
		}
	}()

	a.Logger.Info("event subscribers registered", "topics", []string{itemEvents.TopicItemCreated})
	return nil
}

// handleItemCreated returns a handler for item.created events.
// Handlers must be idempotent — EventBus retries up to 3× on failure.
// Warms the Redis read-model cache so subsequent GetByID calls are served from cache.
func handleItemCreated(a *app.Application) func(context.Context, *message.Message) error {
	itemCache := cache.NewItemCache(a.Redis)
	return func(ctx context.Context, msg *message.Message) error {
		var evt itemEvents.ItemCreatedEvent
		if err := json.Unmarshal(msg.Payload, &evt); err != nil {
			return err
		}

		if err := itemCache.Set(ctx, &cache.CachedItem{
			ID:        evt.ItemID,
			OrgID:     evt.OrgID,
			Name:      evt.Name,
			CreatedAt: evt.OccurredAt,
		}); err != nil {
			// Cache warming is best-effort; log but do not fail the handler.
			a.Logger.WarnContext(ctx, "cache warm failed for item.created",
				"item_id", evt.ItemID, "error", err)
		} else {
			a.Logger.InfoContext(ctx, "cache warmed",
				"item_id", evt.ItemID, "org_id", evt.OrgID)
		}

		return nil
	}
}

// runOutboxRelay polls the outbox for unpublished events and forwards them to
// the EventBus. Runs until ctx is cancelled.
// The Watermill Forwarder (started in cmd/api/main.go) handles at-least-once
// delivery; this relay is a secondary safety net for future outbox tables.
func runOutboxRelay(ctx context.Context, a *app.Application) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			a.Logger.Info("outbox relay shutting down")
			return
		case <-ticker.C:
			// TODO: query outbox table, publish unpublished events, mark as published
		}
	}
}
