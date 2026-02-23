# Go DDD / Clean Architecture Boilerplate

## Overview

A production-ready Go monorepo boilerplate for building multi-service backends using **Domain-Driven Design (DDD)** and **Clean Architecture**. Provides pre-wired infrastructure (Postgres, Redis, MinIO, Watermill event bus, OTel tracing, Sentry) so you ship domain logic instead of plumbing.

**Runtime model**: Two binaries — `srvname` (HTTP API + orchestration) and `worker` (background execution). Each binary wires together one or more domain services from `services/`.

## Project Structure

```
project/
├── cmd/
│   └── srvname/main.go       # Server entrypoint — wires infrastructure + mounts routes
├── services/                  # One directory per bounded context
│   └── {name}/
│       ├── domain/            # Pure business logic — no external imports
│       │   ├── models/        # Entities, value objects, aggregates
│       │   ├── repositories/  # Repository interfaces (Go interfaces only)
│       │   ├── services/      # Domain services (stateless business logic)
│       │   └── events/        # Domain event structs
│       ├── application/       # Use cases — depends only on domain
│       │   ├── services/      # Application services (orchestrate domain)
│       │   ├── handlers/      # HTTP handlers (call application services)
│       │   └── api/           # Route registration (BookRoutes)
│       └── infrastructure/    # Implementations — depends on domain interfaces
│           └── persistence/
│               └── postgres/  # Repository implementations + sqlc-generated db/
├── pkg/                       # Shared packages (reusable across all services)
│   ├── app/                   # Application container (Db, Logger, EventBus)
│   ├── auth/                  # Encrypted cookie session store
│   ├── config/                # Env-driven configuration (ardanlabs/conf)
│   ├── database/              # pgxpool setup with production-ready settings
│   ├── events/                # Watermill SQL-backed pub/sub (EventBus)
│   ├── logger/                # Structured slog + Gin request/recovery middleware
│   ├── migrator/              # goose migration runner
│   └── telemetry/             # OTel tracing + metrics, Sentry crash reporting
├── migrations/
│   └── {name}/                # One directory per service database
│       ├── *.sql              # goose migration files (append-only)
│       └── run.go             # go:embed entrypoint — go run migrations/{name}/run.go
└── deployments/
    └── docker/                # Multi-stage Dockerfiles
```

## Build & Run Commands

```bash
# Build all packages
go build ./...

# Run tests
go test ./...

# Vet
go vet ./...

# Run server
go run cmd/api/main.go

# Docker
docker compose up -d                        # Start infrastructure
docker compose up -d --build api       # Rebuild a service
docker compose down                         # Stop all

# Migrations
make migrate                                # Run all pending migrations
make migrate-{name}                         # Run migrations for one service

# sqlc (after schema changes)
make sqlc                                   # Dump schema + regenerate db layer
```

## Adding a New Service

### 1. Domain layer — pure business logic, zero external imports

```go
// services/{name}/domain/models/entity.go
type Entity struct {
    ID        uuid.UUID
    Name      string
    CreatedAt time.Time
}

// services/{name}/domain/repositories/entity.go
type EntityRepository interface {
    Save(ctx context.Context, e *Entity) error
    GetByID(ctx context.Context, id uuid.UUID) (*Entity, error)
}

// services/{name}/domain/services/logic.go
func Validate(e *Entity) error { ... }
```

### 2. Application layer — orchestrates domain, handles HTTP

```go
// services/{name}/application/services/service.go
type EntityService struct{ repo repositories.EntityRepository }

func (s *EntityService) Create(ctx context.Context, name string) (*models.Entity, error) {
    e := &models.Entity{ID: uuid.New(), Name: name, CreatedAt: time.Now()}
    if err := domainsvcs.Validate(e); err != nil { return nil, err }
    return e, s.repo.Save(ctx, e)
}

// services/{name}/application/handlers/handler.go
func (h *Handler) Create(c *gin.Context) { ... }

// services/{name}/application/api/main.go
func BookRoutes(api *gin.RouterGroup, app *app.Application) {
    svc := services.New(app)
    r := api.Group("/{name}")
    r.POST("/", handlers.NewHandler(svc).Create)
}
```

### 3. Infrastructure layer — implements domain interfaces

```go
// services/{name}/infrastructure/persistence/postgres/repository.go
type Repository struct{ pool *pgxpool.Pool }

func (r *Repository) Save(ctx context.Context, e *models.Entity) error { ... }
```

### 4. Migrations

```sql
-- migrations/{name}/00001_create_schema.sql
-- +goose Up
CREATE SCHEMA IF NOT EXISTS {name};
-- +goose Down
DROP SCHEMA IF EXISTS {name};
```

### 5. Mount in cmd/srvname/main.go

```go
func registerRoutes(api *gin.RouterGroup, a *app.Application) {
    myservice.BookRoutes(api, a)
}
```

## Clean Architecture Dependency Rules

```
Domain ← Application ← Infrastructure
  ↑            ↑
  └─────────── pkg/
```

| Layer | May import | Must not import |
|---|---|---|
| `domain/` | stdlib only | `application/`, `infrastructure/`, `pkg/` |
| `application/` | `domain/`, `pkg/` | `infrastructure/` |
| `infrastructure/` | `domain/`, `pkg/`, external drivers | `application/` |
| `cmd/` | everything | — (composition root) |

## Code Conventions

**Imports** — three groups, blank-line separated:

```go
import (
    "context"
    "fmt"

    "github.com/jackc/pgx/v5/pgxpool"

    "github.com/yourorg/project/pkg/config"
    "github.com/yourorg/project/services/myservice/domain/models"
)
```

**Error handling** — always wrap with context:

```go
if err != nil {
    return fmt.Errorf("save entity: %w", err)
}
```

**Dependencies** — inject via constructor, never globals:

```go
// Good
func NewService(repo repositories.EntityRepository) *Service {
    return &Service{repo: repo}
}

// Bad
var globalRepo repositories.EntityRepository
```

**Context** — thread through all I/O-bound calls:

```go
func (s *Service) Create(ctx context.Context, name string) (*models.Entity, error) { ... }
```

**Testing** — place `*_test.go` beside the file under test; mock repository interfaces for unit tests

**HTTP** — Gin router; handlers must only call application services, never domain services or repositories directly

## Shared Packages (`pkg/`)

| Package | API |
|---|---|
| `pkg/app` | `Application{Db, Logger, EventBus}` — pass to all `BookRoutes` calls |
| `pkg/config` | `config.Load()` → `*Config` loaded from env/dotenv via `ardanlabs/conf` |
| `pkg/database` | `database.NewPool(ctx, url)` → `*pgxpool.Pool` (25 max conns, 1h lifetime) |
| `pkg/events` | `events.NewEventBus(cfg, log)` → `Publish(topic, msgs...)` / `Subscribe(ctx, topic)` |
| `pkg/logger` | `logger.New(cfg)` JSON slog; `GinLogger(log)`, `GinRecovery(log)` middleware |
| `pkg/auth` | `auth.NewSessionStore(authKey, encKey)` → `*sessions.CookieStore` |
| `pkg/migrator` | `migrator.RunMigrations(dbUrl, fs.FS)` — runs embedded goose migrations |
| `pkg/telemetry` | `telemetry.Setup(ctx, cfg)` OTel OTLP; `SetupSentry(cfg)`; `SentryMiddleware()` |

## Environment Variables

```bash
# Databases
DEFINITION_DATABASE_URL=postgres://user:pass@localhost:5432/db?sslmode=disable

# Redis
REDIS_URL=redis://localhost:6379

# MinIO / S3
MINIO_ENDPOINT=localhost:9000
MINIO_BUCKET=hasty-snapshots
MINIO_ROOT_USER=minioadmin
MINIO_ROOT_PASSWORD=secret

# Session (generate with: openssl rand -base64 32)
SESSION_AUTH_KEY=32-or-64-byte-random-string
SESSION_ENCRYPTION_KEY=16-24-or-32-byte-random-string

# Application
LOG_LEVEL=info              # debug | info | warn | error
ENVIRONMENT=development     # development | testing | production

# Observability (leave empty to disable)
SERVICE_NAME=myservice
SERVICE_VERSION=dev
OTEL_ENDPOINT=
SENTRY_DSN=
```

## Architecture Rules

1. **Dependency direction flows inward**: `infrastructure → application → domain`
2. **No cross-service HTTP calls**: services communicate exclusively via `EventBus` (Watermill)
3. **Stateless handlers**: all durable state in Postgres or MinIO; processes are safe to restart
4. **Multi-tenant**: scope all DB queries and event topics by `org_id` when adding tenant support
5. **Migrations are append-only**: never edit existing `.sql` files; add new numbered files

## Do Not Touch

- `.env` — secrets, never commit to git
- `go.sum` — managed by `go mod tidy`
- `*/infrastructure/persistence/postgres/db/*.go` — sqlc-generated; regenerate with `make sqlc`
- `pkg/` — shared across all services; coordinate breaking changes carefully

## Task Master AI Instructions
**Import Task Master's development workflow commands and guidelines, treat as if import is in the main CLAUDE.md file.**
@./.taskmaster/CLAUDE.md
