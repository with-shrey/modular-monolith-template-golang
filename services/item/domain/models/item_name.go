package models

import "fmt"

// ItemName is a value object representing a valid item name.
// Encapsulates validation rules: 1 <= len(name) <= 255.
type ItemName string

const (
	minItemNameLength = 1
	maxItemNameLength = 255
)

// NewItemName constructs a valid ItemName or returns an error if constraints are violated.
func NewItemName(s string) (ItemName, error) {
	if len(s) < minItemNameLength {
		return "", fmt.Errorf("item name must be at least %d character", minItemNameLength)
	}
	if len(s) > maxItemNameLength {
		return "", fmt.Errorf("item name must not exceed %d characters", maxItemNameLength)
	}
	return ItemName(s), nil
}

// String returns the underlying string value.
func (n ItemName) String() string {
	return string(n)
}
