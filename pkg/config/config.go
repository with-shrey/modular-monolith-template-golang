package config

import (
	"fmt"
	"strings"

	"github.com/ardanlabs/conf/v3"
	"github.com/joho/godotenv"
)

// Environment name constants used in ENVIRONMENT config field.
const (
	EnvDevelopment = "development"
	EnvProduction  = "production"
	EnvTesting     = "testing"
)

// Config holds all configuration for the application
type Config struct {
	// Database
	DefinitionDatabaseURL string `conf:"default:postgres://hasty:password@localhost:5432/hastyconnect?sslmode=disable,env:DEFINITION_DATABASE_URL"`
	// Redis
	RedisURL string `conf:"default:redis://localhost:6379,env:REDIS_URL"`

	// MinIO/S3
	MinioEndpoint     string `conf:"default:localhost:9000,env:MINIO_ENDPOINT"`
	MinioBucket       string `conf:"default:hasty-snapshots,env:MINIO_BUCKET"`
	MinioRootUser     string `conf:"default:minioadmin,env:MINIO_ROOT_USER"`
	MinioRootPassword string `conf:"default:minioadmin,env:MINIO_ROOT_PASSWORD,noprint"`

	// Application
	LogLevel    string `conf:"default:info,env:LOG_LEVEL"`
	Environment string `conf:"default:development,enum:development|testing|production,env:ENVIRONMENT"`

	// Session
	SessionAuthKey       string `conf:"default:dev-auth-key-32-bytes-long!!!,env:SESSION_AUTH_KEY"`
	SessionEncryptionKey string `conf:"default:dev-encryption-key-32-bytes!!,env:SESSION_ENCRYPTION_KEY"`

	// CORS — comma-separated list of allowed origins; use * to allow all (dev only)
	CORSAllowedOrigins string `conf:"default:*,env:CORS_ALLOWED_ORIGINS"`

	// Temporal
	TemporalHostPort  string `conf:"default:localhost:7233,env:TEMPORAL_HOST_PORT"`
	TemporalNamespace string `conf:"default:default,env:TEMPORAL_NAMESPACE"`

	// Observability
	ServiceName    string `conf:"default:hastyconnect,env:SERVICE_NAME"`
	ServiceVersion string `conf:"default:dev,env:SERVICE_VERSION"`
	OtelEndpoint   string `conf:"default:http://localhost,env:OTEL_ENDPOINT"`
	SentryDSN      string `conf:"default:http://localhost,env:SENTRY_DSN,noprint"`
}

// Load reads configuration from environment variables with sensible defaults
func Load() (*Config, error) {
	var cfg Config
	_ = godotenv.Load()
	if _, err := conf.Parse("", &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	return &cfg, nil
}

// ValidateForProduction enforces security requirements when ENVIRONMENT=production.
// Returns an error if any critical settings are missing or unsafe.
// No-ops for non-production environments.
func ValidateForProduction(cfg *Config) error {
	if cfg.Environment != EnvProduction {
		return nil
	}

	var errs []string

	if len(cfg.SessionAuthKey) < 32 {
		errs = append(errs, fmt.Sprintf(
			"SESSION_AUTH_KEY must be at least 32 bytes (got %d); generate with: openssl rand -base64 32",
			len(cfg.SessionAuthKey),
		))
	}

	if len(cfg.SessionEncryptionKey) < 16 {
		errs = append(errs, fmt.Sprintf(
			"SESSION_ENCRYPTION_KEY must be at least 16 bytes (got %d); generate with: openssl rand -base64 16",
			len(cfg.SessionEncryptionKey),
		))
	}

	if cfg.LogLevel == "debug" {
		errs = append(errs, "LOG_LEVEL must not be 'debug' in production (may leak sensitive data)")
	}

	if len(errs) == 0 {
		return nil
	}

	return fmt.Errorf("production config validation failed: %s", strings.Join(errs, "; "))
}
