package events

import (
	"time"

	"github.com/google/uuid"
)

// TopicItemCreated is the Watermill topic published when an Item is created.
const TopicItemCreated = "item.created"

// ItemCreatedEvent is published after a new Item is persisted.
// Consumers subscribe via EventBus.Subscribe(ctx, events.TopicItemCreated).
type ItemCreatedEvent struct {
	EventID    uuid.UUID `json:"event_id"` // Unique publish-time identifier for deduplication
	Version    int       `json:"version"`  // Schema version; increment on breaking changes
	ItemID     uuid.UUID `json:"item_id"`
	OrgID      uuid.UUID `json:"org_id"`
	Name       string    `json:"name"`
	OccurredAt time.Time `json:"occurred_at"`
}
