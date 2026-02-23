package models

import (
	"time"

	"github.com/google/uuid"
)

// Item is the core aggregate for this bounded context.
type Item struct {
	ID        uuid.UUID
	OrgID     uuid.UUID // tenant scope — always filter by this in queries
	Name      ItemName
	CreatedAt time.Time
}

// NewItem constructs a valid Item aggregate with generated ID and current timestamp.
func NewItem(orgID uuid.UUID, name ItemName) (*Item, error) {
	return &Item{
		ID:        uuid.New(),
		OrgID:     orgID,
		Name:      name,
		CreatedAt: time.Now().UTC(),
	}, nil
}
