package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const (
	// ItemCacheTTL is the time-to-live for cached items.
	ItemCacheTTL = 24 * time.Hour

	itemCacheKeyPrefix = "item"
)

// CachedItem is the denormalized read model stored in Redis.
// Fields are stored as a Redis hash. Additional fields from other aggregates
// can be added here for read optimization without touching the domain model.
type CachedItem struct {
	ID        uuid.UUID `json:"id"`
	OrgID     uuid.UUID `json:"org_id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

// ItemCache provides structured read/write operations for item cache entries.
// Keys are scoped by orgID to prevent cross-tenant data leakage.
// Key format: "item:{orgID}:{itemID}"
type ItemCache struct {
	client *RedisClient
}

// NewItemCache creates a new ItemCache backed by the given RedisClient.
func NewItemCache(r *RedisClient) *ItemCache {
	return &ItemCache{client: r}
}

// Get retrieves a cached item by org + item ID.
// Returns redis.Nil error when the key does not exist or has expired.
func (c *ItemCache) Get(ctx context.Context, orgID, itemID uuid.UUID) (*CachedItem, error) {
	key := c.key(orgID, itemID)
	vals, err := c.client.Client().HGetAll(ctx, key).Result()
	if err != nil {
		return nil, fmt.Errorf("cache get: %w", err)
	}
	if len(vals) == 0 {
		return nil, redis.Nil // key not found
	}

	id, err := uuid.Parse(vals["id"])
	if err != nil {
		return nil, fmt.Errorf("cache parse id: %w", err)
	}
	oid, err := uuid.Parse(vals["org_id"])
	if err != nil {
		return nil, fmt.Errorf("cache parse org_id: %w", err)
	}
	createdAt, err := time.Parse(time.RFC3339Nano, vals["created_at"])
	if err != nil {
		return nil, fmt.Errorf("cache parse created_at: %w", err)
	}

	return &CachedItem{
		ID:        id,
		OrgID:     oid,
		Name:      vals["name"],
		CreatedAt: createdAt,
	}, nil
}

// Set writes a cached item as a Redis hash with a 24-hour TTL.
// Uses a pipeline to set all fields and the TTL atomically.
func (c *ItemCache) Set(ctx context.Context, item *CachedItem) error {
	key := c.key(item.OrgID, item.ID)
	pipe := c.client.Client().Pipeline()
	pipe.HSet(ctx, key,
		"id", item.ID.String(),
		"org_id", item.OrgID.String(),
		"name", item.Name,
		"created_at", item.CreatedAt.UTC().Format(time.RFC3339Nano),
	)
	pipe.Expire(ctx, key, ItemCacheTTL)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("cache set: %w", err)
	}
	return nil
}

// Delete removes a cached item.
func (c *ItemCache) Delete(ctx context.Context, orgID, itemID uuid.UUID) error {
	if err := c.client.Client().Del(ctx, c.key(orgID, itemID)).Err(); err != nil {
		return fmt.Errorf("cache delete: %w", err)
	}
	return nil
}

// key builds the Redis key: "item:{orgID}:{itemID}"
func (c *ItemCache) key(orgID, itemID uuid.UUID) string {
	return fmt.Sprintf("%s:%s:%s", itemCacheKeyPrefix, orgID, itemID)
}
