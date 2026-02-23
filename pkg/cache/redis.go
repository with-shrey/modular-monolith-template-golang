package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/ghuser/ghproject/pkg/config"
)

// RedisClient wraps redis.Client with production-ready configuration.
type RedisClient struct {
	client *redis.Client
}

// NewRedisClient creates a new Redis client with connection pooling and production-ready settings.
// It parses the Redis URL from config, applies pool settings, and verifies connectivity via Ping.
func NewRedisClient(cfg *config.Config) (*RedisClient, error) {
	opts, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse redis URL: %w", err)
	}

	// Connection pool settings
	// PoolSize: maximum number of connections in the pool
	opts.PoolSize = 10

	// MinIdleConns: minimum number of idle connections to keep open
	opts.MinIdleConns = 2

	// MaxRetries: maximum number of retries before giving up on a command
	opts.MaxRetries = 3

	// DialTimeout: timeout for establishing a new connection
	opts.DialTimeout = 5 * time.Second

	// ReadTimeout: timeout for socket reads
	opts.ReadTimeout = 3 * time.Second

	// WriteTimeout: timeout for socket writes
	opts.WriteTimeout = 3 * time.Second

	// PoolTimeout: timeout waiting for a connection from the pool
	opts.PoolTimeout = 4 * time.Second

	rdb := redis.NewClient(opts)

	// Verify connectivity with a 2s deadline
	pingCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := rdb.Ping(pingCtx).Err(); err != nil {
		_ = rdb.Close()
		return nil, fmt.Errorf("failed to ping redis: %w", err)
	}

	return &RedisClient{client: rdb}, nil
}

// Ping checks the Redis connection health.
func (r *RedisClient) Ping(ctx context.Context) error {
	if err := r.client.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("redis ping: %w", err)
	}
	return nil
}

// Close gracefully shuts down the Redis connection pool.
func (r *RedisClient) Close() error {
	if r.client == nil {
		return nil
	}
	if err := r.client.Close(); err != nil {
		return fmt.Errorf("redis close: %w", err)
	}
	return nil
}

// Client returns the underlying redis.Client for direct use.
func (r *RedisClient) Client() *redis.Client {
	return r.client
}
