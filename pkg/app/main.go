package app

import (
	"github.com/ghuser/ghproject/pkg/cache"
	"github.com/ghuser/ghproject/pkg/database"
	"github.com/ghuser/ghproject/pkg/events"
	"github.com/ghuser/ghproject/pkg/logger"
	"github.com/ghuser/ghproject/pkg/workflows"
	"github.com/gorilla/sessions"
)

// Application holds shared infrastructure dependencies for all services.
// Pass to all service BookRoutes calls during server initialization.
//
// Logging: app.Logger is backed by a trace-aware handler — use slog's context methods
// and trace_id, span_id, and request_id are injected automatically:
//
//	app.Logger.InfoContext(ctx, "processing item", "item_id", id)
//	app.Logger.ErrorContext(ctx, "failed to save", "error", err)
//
// Use app.Logger.Info/Error (no context) only for startup and shutdown messages.
type Application struct {
	Db             *database.Database
	Logger         logger.Logger
	EventBus       *events.EventBus
	Redis          *cache.RedisClient
	TemporalClient *workflows.TemporalClient
	SessionStore   sessions.Store // Redis-backed session store; nil in worker process
}
