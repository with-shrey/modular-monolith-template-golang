package domain

import "errors"

// Sentinel errors for the item domain. Use errors.Is() to check these.
var (
	// ErrItemNotFound indicates the requested item does not exist.
	ErrItemNotFound = errors.New("item not found")

	// ErrItemAlreadyExists indicates an item with the same unique constraint already exists.
	ErrItemAlreadyExists = errors.New("item already exists")

	// ErrInvalidItemName indicates the item name violates domain constraints.
	ErrInvalidItemName = errors.New("invalid item name")
)
