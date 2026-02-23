package models

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestNewItem(t *testing.T) {
	orgID := uuid.New()
	name := ItemName("Test Item")

	t.Run("returns item with non-zero ID", func(t *testing.T) {
		item, err := NewItem(orgID, name)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if item.ID == (uuid.UUID{}) {
			t.Fatal("expected non-zero UUID for ID")
		}
	})

	t.Run("sets OrgID correctly", func(t *testing.T) {
		item, err := NewItem(orgID, name)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if item.OrgID != orgID {
			t.Fatalf("expected OrgID %v, got %v", orgID, item.OrgID)
		}
	})

	t.Run("sets Name correctly", func(t *testing.T) {
		item, err := NewItem(orgID, name)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if item.Name != name {
			t.Fatalf("expected Name %v, got %v", name, item.Name)
		}
	})

	t.Run("sets CreatedAt to approximately now UTC", func(t *testing.T) {
		before := time.Now().UTC()
		item, err := NewItem(orgID, name)
		after := time.Now().UTC()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if item.CreatedAt.IsZero() {
			t.Fatal("expected non-zero CreatedAt")
		}
		if item.CreatedAt.Before(before) || item.CreatedAt.After(after) {
			t.Fatalf("CreatedAt %v not between %v and %v", item.CreatedAt, before, after)
		}
	})

	t.Run("generates unique IDs on each call", func(t *testing.T) {
		item1, _ := NewItem(orgID, name)
		item2, _ := NewItem(orgID, name)
		if item1.ID == item2.ID {
			t.Fatal("expected unique IDs, got identical")
		}
	})
}
