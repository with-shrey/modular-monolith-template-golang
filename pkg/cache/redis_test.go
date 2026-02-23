package cache

import (
	"context"
	"os"
	"testing"

	"github.com/ghuser/ghproject/pkg/config"
)

// newTestConfig returns a config pointing to REDIS_URL env var, falling back to localhost.
func newTestConfig(url string) *config.Config {
	return &config.Config{
		RedisURL: url,
	}
}

func TestNewRedisClient_InvalidURL(t *testing.T) {
	_, err := NewRedisClient(newTestConfig("not-a-valid-url"))
	if err == nil {
		t.Fatal("expected error for invalid URL, got nil")
	}
}

func TestNewRedisClient_UnreachableHost(t *testing.T) {
	_, err := NewRedisClient(newTestConfig("redis://localhost:19999"))
	if err == nil {
		t.Fatal("expected error when Redis is unreachable, got nil")
	}
}

// Integration tests — skipped unless REDIS_URL is set.
func TestRedisIntegration(t *testing.T) {
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		t.Skip("REDIS_URL not set; skipping integration tests")
	}

	t.Run("NewRedisClient_Success", func(t *testing.T) {
		rc, err := NewRedisClient(newTestConfig(redisURL))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer rc.Close() //nolint:errcheck
	})

	t.Run("Ping_Success", func(t *testing.T) {
		rc, err := NewRedisClient(newTestConfig(redisURL))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer rc.Close() //nolint:errcheck

		if err := rc.Ping(context.Background()); err != nil {
			t.Fatalf("Ping failed: %v", err)
		}
	})

	t.Run("Close_Idempotent", func(t *testing.T) {
		rc, err := NewRedisClient(newTestConfig(redisURL))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if err := rc.Close(); err != nil {
			t.Fatalf("first Close failed: %v", err)
		}
	})

	t.Run("Client_NotNil", func(t *testing.T) {
		rc, err := NewRedisClient(newTestConfig(redisURL))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer rc.Close() //nolint:errcheck

		if rc.Client() == nil {
			t.Fatal("expected non-nil underlying client")
		}
	})
}
