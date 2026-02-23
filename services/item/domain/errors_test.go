package domain

import (
	"errors"
	"fmt"
	"testing"
)

func TestSentinelErrors_NonNil(t *testing.T) {
	if ErrItemNotFound == nil {
		t.Fatal("ErrItemNotFound must not be nil")
	}
	if ErrItemAlreadyExists == nil {
		t.Fatal("ErrItemAlreadyExists must not be nil")
	}
	if ErrInvalidItemName == nil {
		t.Fatal("ErrInvalidItemName must not be nil")
	}
}

func TestSentinelErrors_Messages(t *testing.T) {
	if ErrItemNotFound.Error() != "item not found" {
		t.Fatalf("unexpected message: %q", ErrItemNotFound.Error())
	}
	if ErrItemAlreadyExists.Error() != "item already exists" {
		t.Fatalf("unexpected message: %q", ErrItemAlreadyExists.Error())
	}
	if ErrInvalidItemName.Error() != "invalid item name" {
		t.Fatalf("unexpected message: %q", ErrInvalidItemName.Error())
	}
}

func TestSentinelErrors_WrappedIdentity(t *testing.T) {
	wrapped := fmt.Errorf("context: %w", ErrItemNotFound)
	if !errors.Is(wrapped, ErrItemNotFound) {
		t.Fatal("errors.Is must match wrapped ErrItemNotFound")
	}

	wrapped2 := fmt.Errorf("%w: %w", ErrInvalidItemName, errors.New("too long"))
	if !errors.Is(wrapped2, ErrInvalidItemName) {
		t.Fatal("errors.Is must match double-wrapped ErrInvalidItemName")
	}
}
