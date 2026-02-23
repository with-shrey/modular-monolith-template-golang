package httpx

import (
	"context"
	"net/http"
	"time"
)

// HealthChecker is satisfied by any infrastructure dependency that exposes
// a Ping method (pgxpool.Pool, RedisClient, EventBus all qualify).
type HealthChecker interface {
	Ping(ctx context.Context) error
}

// HealthChecks holds the set of dependencies to probe in the health endpoint.
type HealthChecks struct {
	Database HealthChecker
	Redis    HealthChecker
	EventBus HealthChecker
}

type healthResponse struct {
	Status   string `json:"status"`
	Database string `json:"database"`
	Redis    string `json:"redis"`
	EventBus string `json:"event_bus"`
}

// HealthHandler returns an http.HandlerFunc that probes all registered
// HealthCheckers and reports degraded status if any of them fail.
func HealthHandler(checks HealthChecks) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		resp := healthResponse{
			Status:   "ok",
			Database: "ok",
			Redis:    "ok",
			EventBus: "ok",
		}

		if err := checks.Database.Ping(ctx); err != nil {
			resp.Status = "degraded"
			resp.Database = "unreachable"
		}
		if err := checks.Redis.Ping(ctx); err != nil {
			resp.Status = "degraded"
			resp.Redis = "unreachable"
		}
		if err := checks.EventBus.Ping(ctx); err != nil {
			resp.Status = "degraded"
			resp.EventBus = "unreachable"
		}

		status := http.StatusOK
		if resp.Status != "ok" {
			status = http.StatusServiceUnavailable
		}
		JSON(w, status, resp)
	}
}
