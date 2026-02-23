package repositories

import (
	"context"

	"github.com/google/uuid"

	"github.com/ghuser/ghproject/services/item/domain/models"
)

// QueryOpts contains pagination parameters for list queries.
type QueryOpts struct {
	Limit  int // Maximum number of records to return
	Offset int // Number of records to skip
}

// ItemRepository is the persistence interface for the Item aggregate.
// The domain layer owns this interface; infrastructure implements it.
type ItemRepository interface {
	Save(ctx context.Context, item *models.Item) error
	GetByID(ctx context.Context, orgID, id uuid.UUID) (*models.Item, error)

	// FindByOrgID retrieves a paginated list of items for the given org.
	// Returns the items slice and the total count (ignoring pagination).
	FindByOrgID(ctx context.Context, orgID uuid.UUID, opts QueryOpts) ([]*models.Item, int, error)

	// Update persists changes to an existing Item.
	Update(ctx context.Context, item *models.Item) error

	// Delete removes an item by ID scoped to the given org.
	Delete(ctx context.Context, orgID, id uuid.UUID) error

	// Exists reports whether an item with the given ID exists for the given org.
	Exists(ctx context.Context, orgID, id uuid.UUID) (bool, error)
}
