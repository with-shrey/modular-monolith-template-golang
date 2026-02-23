package services

import (
	"testing"

	"github.com/google/uuid"

	"github.com/ghuser/ghproject/services/item/domain/models"
)

func TestValidateName(t *testing.T) {
	tests := []struct {
		name    string
		input   models.ItemName
		wantErr bool
	}{
		{"valid name", "Valid Item Name", false},
		{"valid name with special chars", "Item-Name_123!@#", false},
		{"valid single space between words", "item name", false},
		{"leading whitespace", " Name", true},
		{"trailing whitespace", "Name ", true},
		{"leading and trailing whitespace", " Name ", true},
		{"only whitespace", "   ", true},
		{"tab character (control)", "Name\tName", true},
		{"newline character (control)", "Name\nName", true},
		{"null byte (control)", "Name\x00", true},
		{"DEL character", "Name\x7F", true},
		{"consecutive spaces", "Item  Name", true},
		{"three consecutive spaces", "Item   Name", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateName(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ValidateName(%q) error = %v, wantErr = %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestValidateItemForCreation(t *testing.T) {
	validName := models.ItemName("Valid Item")
	validOrgID := uuid.New()
	validID := uuid.New()

	makeItem := func(id, orgID uuid.UUID, name models.ItemName) *models.Item {
		return &models.Item{ID: id, OrgID: orgID, Name: name}
	}

	t.Run("nil item returns error", func(t *testing.T) {
		if err := ValidateItemForCreation(nil); err == nil {
			t.Fatal("expected error for nil item")
		}
	})

	t.Run("valid item returns nil", func(t *testing.T) {
		item := makeItem(validID, validOrgID, validName)
		if err := ValidateItemForCreation(item); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("zero OrgID returns error", func(t *testing.T) {
		item := makeItem(validID, uuid.Nil, validName)
		if err := ValidateItemForCreation(item); err == nil {
			t.Fatal("expected error for zero OrgID")
		}
	})

	t.Run("zero ID returns error", func(t *testing.T) {
		item := makeItem(uuid.Nil, validOrgID, validName)
		if err := ValidateItemForCreation(item); err == nil {
			t.Fatal("expected error for zero ID")
		}
	})

	t.Run("invalid name propagates error", func(t *testing.T) {
		item := makeItem(validID, validOrgID, models.ItemName(" leading space"))
		if err := ValidateItemForCreation(item); err == nil {
			t.Fatal("expected error for invalid name")
		}
	})

	t.Run("name with control chars propagates error", func(t *testing.T) {
		item := makeItem(validID, validOrgID, models.ItemName("name\x00control"))
		if err := ValidateItemForCreation(item); err == nil {
			t.Fatal("expected error for control character in name")
		}
	})
}
