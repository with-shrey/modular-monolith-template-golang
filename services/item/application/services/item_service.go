package services

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	pkgcache "github.com/ghuser/ghproject/pkg/cache"
	itemdomain "github.com/ghuser/ghproject/services/item/domain"
	"github.com/ghuser/ghproject/services/item/domain/models"
	"github.com/ghuser/ghproject/services/item/domain/repositories"
	domainsvcs "github.com/ghuser/ghproject/services/item/domain/services"
)

// ItemService orchestrates creation and retrieval of Items.
// Event publishing is handled by the repository layer (outbox pattern).
// Reads are served from Redis cache when available.
type ItemService struct {
	repo  repositories.ItemRepository
	cache *pkgcache.ItemCache
}

// NewItemService returns an ItemService wired with the given repository and cache.
func NewItemService(repo repositories.ItemRepository, itemCache *pkgcache.ItemCache) *ItemService {
	return &ItemService{repo: repo, cache: itemCache}
}

// Create validates and persists an Item. The repository publishes ItemCreatedEvent.
func (s *ItemService) Create(ctx context.Context, orgID uuid.UUID, name string) (*models.Item, error) {
	itemName, err := models.NewItemName(name)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", itemdomain.ErrInvalidItemName, err)
	}

	item, err := models.NewItem(orgID, itemName)
	if err != nil {
		return nil, fmt.Errorf("create item: %w", err)
	}

	if err := domainsvcs.ValidateItemForCreation(item); err != nil {
		return nil, fmt.Errorf("%w: %w", itemdomain.ErrInvalidItemName, err)
	}

	if err := s.repo.Save(ctx, item); err != nil {
		return nil, fmt.Errorf("save item: %w", err)
	}

	return item, nil
}

// GetByID retrieves an Item using a read-through cache pattern:
//  1. Check Redis cache first.
//  2. On cache miss (or cache error), query Postgres.
//  3. Asynchronously warm the cache with the Postgres result.
func (s *ItemService) GetByID(ctx context.Context, orgID, id uuid.UUID) (*models.Item, error) {
	if s.cache != nil {
		if cached, err := s.cache.Get(ctx, orgID, id); err == nil {
			return &models.Item{
				ID:        cached.ID,
				OrgID:     cached.OrgID,
				Name:      models.ItemName(cached.Name),
				CreatedAt: cached.CreatedAt,
			}, nil
		} else if !errors.Is(err, redis.Nil) {
			// Cache error — log in production; fall through to Postgres.
			_ = err
		}
	}

	item, err := s.repo.GetByID(ctx, orgID, id)
	if err != nil {
		return nil, fmt.Errorf("get item: %w", err)
	}

	if s.cache != nil {
		go func() {
			_ = s.cache.Set(context.Background(), &pkgcache.CachedItem{
				ID:        item.ID,
				OrgID:     item.OrgID,
				Name:      item.Name.String(),
				CreatedAt: item.CreatedAt,
			})
		}()
	}

	return item, nil
}

// List returns a paginated slice of items for the org plus total count.
func (s *ItemService) List(ctx context.Context, orgID uuid.UUID, opts repositories.QueryOpts) ([]*models.Item, int, error) {
	items, total, err := s.repo.FindByOrgID(ctx, orgID, opts)
	if err != nil {
		return nil, 0, fmt.Errorf("list items: %w", err)
	}
	return items, total, nil
}

// Delete removes an item by ID scoped to the given org.
// Returns ErrItemNotFound if no matching item exists.
func (s *ItemService) Delete(ctx context.Context, orgID, id uuid.UUID) error {
	exists, err := s.repo.Exists(ctx, orgID, id)
	if err != nil {
		return fmt.Errorf("check item: %w", err)
	}
	if !exists {
		return itemdomain.ErrItemNotFound
	}
	if err := s.repo.Delete(ctx, orgID, id); err != nil {
		return fmt.Errorf("delete item: %w", err)
	}
	if s.cache != nil {
		_ = s.cache.Delete(context.Background(), orgID, id)
	}
	return nil
}
