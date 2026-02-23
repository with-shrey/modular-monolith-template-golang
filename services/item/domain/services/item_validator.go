// Package services contains stateless domain services for the item bounded context.
// Domain services enforce business rules that operate purely on domain types
// and have zero external dependencies beyond stdlib and the domain layer.
package services

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/google/uuid"

	"github.com/ghuser/ghproject/services/item/domain/models"
)

// ValidateName enforces business rules for ItemName beyond the structural
// constraints enforced by the ItemName constructor (length 1–255).
//
// Business rules:
//   - No leading or trailing whitespace
//   - No control characters (Unicode category Cc)
//   - No consecutive spaces
//   - Must not be only whitespace characters
func ValidateName(name models.ItemName) error {
	s := name.String()

	if s != strings.TrimSpace(s) {
		return fmt.Errorf("item name must not have leading or trailing whitespace")
	}

	if strings.TrimSpace(s) == "" {
		return fmt.Errorf("item name must not be only whitespace")
	}

	for _, r := range s {
		if unicode.IsControl(r) {
			return fmt.Errorf("item name must not contain control characters")
		}
	}

	if strings.Contains(s, "  ") {
		return fmt.Errorf("item name must not contain consecutive spaces")
	}

	return nil
}

// ValidateItemForCreation performs cross-field validation on a fully-constructed
// Item aggregate before it is persisted. It assumes the Item was built via
// models.NewItem (so structural constraints are already satisfied) and
// adds business-level checks that span multiple fields.
func ValidateItemForCreation(item *models.Item) error {
	if item == nil {
		return fmt.Errorf("item cannot be nil")
	}

	if err := ValidateName(item.Name); err != nil {
		return fmt.Errorf("invalid name: %w", err)
	}

	if item.OrgID == uuid.Nil {
		return fmt.Errorf("org_id must be set")
	}

	if item.ID == uuid.Nil {
		return fmt.Errorf("id must be set")
	}

	return nil
}
