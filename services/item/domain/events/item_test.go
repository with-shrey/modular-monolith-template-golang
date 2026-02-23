package events_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/ghuser/ghproject/services/item/domain/events"
)

func TestItemCreatedEvent_JSONRoundTrip(t *testing.T) {
	original := events.ItemCreatedEvent{
		EventID:    uuid.MustParse("550e8400-e29b-41d4-a716-446655440001"),
		Version:    1,
		ItemID:     uuid.MustParse("550e8400-e29b-41d4-a716-446655440000"),
		OrgID:      uuid.MustParse("660e8400-e29b-41d4-a716-446655440000"),
		Name:       "Test Widget",
		OccurredAt: time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC),
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("expected non-empty JSON")
	}

	var decoded events.ItemCreatedEvent
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if decoded.EventID != original.EventID {
		t.Errorf("EventID: got %v, want %v", decoded.EventID, original.EventID)
	}
	if decoded.Version != original.Version {
		t.Errorf("Version: got %d, want %d", decoded.Version, original.Version)
	}
	if decoded.ItemID != original.ItemID {
		t.Errorf("ItemID: got %v, want %v", decoded.ItemID, original.ItemID)
	}
	if decoded.OrgID != original.OrgID {
		t.Errorf("OrgID: got %v, want %v", decoded.OrgID, original.OrgID)
	}
	if decoded.Name != original.Name {
		t.Errorf("Name: got %q, want %q", decoded.Name, original.Name)
	}
	if !decoded.OccurredAt.Equal(original.OccurredAt) {
		t.Errorf("OccurredAt: got %v, want %v", decoded.OccurredAt, original.OccurredAt)
	}
}

func TestItemCreatedEvent_JSONFieldNames(t *testing.T) {
	evt := events.ItemCreatedEvent{
		EventID:    uuid.New(),
		Version:    1,
		ItemID:     uuid.New(),
		OrgID:      uuid.New(),
		Name:       "Widget",
		OccurredAt: time.Now().UTC(),
	}

	data, err := json.Marshal(evt)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal to map failed: %v", err)
	}

	for _, field := range []string{"event_id", "version", "item_id", "org_id", "name", "occurred_at"} {
		if _, ok := raw[field]; !ok {
			t.Errorf("expected JSON field %q not found in: %s", field, data)
		}
	}
}

func TestTopicItemCreated_Value(t *testing.T) {
	if events.TopicItemCreated == "" {
		t.Fatal("TopicItemCreated must not be empty")
	}
	if events.TopicItemCreated != "item.created" {
		t.Errorf("expected %q, got %q", "item.created", events.TopicItemCreated)
	}
}
