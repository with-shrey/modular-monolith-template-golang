package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/ghuser/ghproject/pkg/database"
	"github.com/ghuser/ghproject/pkg/events"
	itemdomain "github.com/ghuser/ghproject/services/item/domain"
	domainevents "github.com/ghuser/ghproject/services/item/domain/events"
	"github.com/ghuser/ghproject/services/item/domain/models"
	"github.com/ghuser/ghproject/services/item/domain/repositories"
	"github.com/ghuser/ghproject/services/item/infrastructure/persistence/postgres/db"
)

// ItemRepository implements repositories.ItemRepository against PostgreSQL.
type ItemRepository struct {
	db  *database.Database
	bus *events.EventBus
}

// NewItemRepository returns an ItemRepository backed by the given connection pool
// and event bus. The bus is used to publish ItemCreatedEvents after a successful save.
func NewItemRepository(database *database.Database, bus *events.EventBus) *ItemRepository {
	return &ItemRepository{db: database, bus: bus}
}

// Save persists a new Item and publishes an ItemCreatedEvent within the same transaction.
// Returns ErrItemAlreadyExists on unique constraint violations.
func (r *ItemRepository) Save(ctx context.Context, item *models.Item) error {
	return r.db.WithTx(ctx, func(tx *sql.Tx) error {
		q := db.New(tx)
		if err := q.InsertItem(ctx, db.InsertItemParams{
			ID:        item.ID,
			OrgID:     item.OrgID,
			Name:      item.Name.String(),
			CreatedAt: item.CreatedAt,
		}); err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23505" {
				return itemdomain.ErrItemAlreadyExists
			}
			return fmt.Errorf("insert item: %w", err)
		}

		if r.bus != nil {
			if err := r.publishCreated(tx, item); err != nil {
				return fmt.Errorf("publish item created: %w", err)
			}
		}
		return nil
	})
}

// GetByID retrieves an Item by ID scoped to the given org. Returns ErrItemNotFound if not found.
func (r *ItemRepository) GetByID(ctx context.Context, orgID, id uuid.UUID) (*models.Item, error) {
	q := db.New(r.db.DB())
	row, err := q.GetItemByID(ctx, db.GetItemByIDParams{
		ID:    id,
		OrgID: orgID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, itemdomain.ErrItemNotFound
		}
		return nil, fmt.Errorf("query item: %w", err)
	}
	return rowToItem(row), nil
}

// FindByOrgID retrieves a paginated list of items and total count for the given org.
func (r *ItemRepository) FindByOrgID(ctx context.Context, orgID uuid.UUID, opts repositories.QueryOpts) ([]*models.Item, int, error) {
	q := db.New(r.db.DB())

	rows, err := q.FindItemsByOrgID(ctx, db.FindItemsByOrgIDParams{
		OrgID:  orgID,
		Limit:  int32(opts.Limit),
		Offset: int32(opts.Offset),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("query items: %w", err)
	}

	total, err := q.CountItemsByOrgID(ctx, orgID)
	if err != nil {
		return nil, 0, fmt.Errorf("count items: %w", err)
	}

	items := make([]*models.Item, len(rows))
	for i, row := range rows {
		items[i] = rowToItem(row)
	}
	return items, int(total), nil
}

// Update persists a name change to an existing Item.
func (r *ItemRepository) Update(ctx context.Context, item *models.Item) error {
	q := db.New(r.db.DB())
	if err := q.UpdateItem(ctx, db.UpdateItemParams{
		ID:    item.ID,
		OrgID: item.OrgID,
		Name:  item.Name.String(),
	}); err != nil {
		return fmt.Errorf("update item: %w", err)
	}
	return nil
}

// Delete removes an item by ID scoped to the given org.
func (r *ItemRepository) Delete(ctx context.Context, orgID, id uuid.UUID) error {
	q := db.New(r.db.DB())
	if err := q.DeleteItem(ctx, db.DeleteItemParams{
		ID:    id,
		OrgID: orgID,
	}); err != nil {
		return fmt.Errorf("delete item: %w", err)
	}
	return nil
}

// Exists reports whether an item with the given ID exists for the given org.
func (r *ItemRepository) Exists(ctx context.Context, orgID, id uuid.UUID) (bool, error) {
	q := db.New(r.db.DB())
	exists, err := q.ItemExists(ctx, db.ItemExistsParams{
		ID:    id,
		OrgID: orgID,
	})
	if err != nil {
		return false, fmt.Errorf("check item exists: %w", err)
	}
	return exists, nil
}

func (r *ItemRepository) publishCreated(tx *sql.Tx, item *models.Item) error {
	event := domainevents.ItemCreatedEvent{
		EventID:    uuid.New(),
		Version:    1,
		ItemID:     item.ID,
		OrgID:      item.OrgID,
		Name:       item.Name.String(),
		OccurredAt: item.CreatedAt,
	}
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}
	msg := message.NewMessage(watermill.NewUUID(), payload)
	msg.Metadata.Set("event_id", event.EventID.String())
	msg.Metadata.Set("event_version", "1")
	p, err := r.bus.NewTxPublisher(tx)
	if err != nil {
		return fmt.Errorf("create publisher: %w", err)
	}
	return p.Publish(domainevents.TopicItemCreated, msg)
}

// rowToItem maps a db.ItemItem to a domain models.Item.
func rowToItem(row db.ItemItem) *models.Item {
	return &models.Item{
		ID:        row.ID,
		OrgID:     row.OrgID,
		Name:      models.ItemName(row.Name),
		CreatedAt: row.CreatedAt,
	}
}
