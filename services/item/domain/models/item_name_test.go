package models

import (
	"strings"
	"testing"
)

func TestNewItemName(t *testing.T) {
	t.Run("valid single character", func(t *testing.T) {
		n, err := NewItemName("a")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if n.String() != "a" {
			t.Fatalf("expected %q, got %q", "a", n.String())
		}
	})

	t.Run("valid 255 characters", func(t *testing.T) {
		s := strings.Repeat("x", 255)
		n, err := NewItemName(s)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if n.String() != s {
			t.Fatalf("expected string of length 255, got %d", len(n.String()))
		}
	})

	t.Run("valid normal name", func(t *testing.T) {
		n, err := NewItemName("Sample Item")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if n.String() != "Sample Item" {
			t.Fatalf("expected %q, got %q", "Sample Item", n.String())
		}
	})

	t.Run("empty string returns error", func(t *testing.T) {
		_, err := NewItemName("")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("256 characters returns error", func(t *testing.T) {
		s := strings.Repeat("x", 256)
		_, err := NewItemName(s)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

func TestItemName_String(t *testing.T) {
	n := ItemName("hello")
	if n.String() != "hello" {
		t.Fatalf("expected %q, got %q", "hello", n.String())
	}
}
