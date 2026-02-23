package auth

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
)

func TestWithOrgID_OrgIDFromCtx(t *testing.T) {
	orgID := uuid.New()
	ctx := WithOrgID(context.Background(), orgID)

	got, err := OrgIDFromCtx(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != orgID {
		t.Fatalf("expected %v, got %v", orgID, got)
	}
}

func TestOrgIDFromCtx_EmptyContext(t *testing.T) {
	_, err := OrgIDFromCtx(context.Background())
	if !errors.Is(err, ErrOrgIDNotFound) {
		t.Fatalf("expected ErrOrgIDNotFound, got %v", err)
	}
}

func TestOrgIDFromCtx_NilUUID(t *testing.T) {
	ctx := WithOrgID(context.Background(), uuid.Nil)
	_, err := OrgIDFromCtx(ctx)
	if !errors.Is(err, ErrOrgIDNotFound) {
		t.Fatalf("expected ErrOrgIDNotFound for uuid.Nil, got %v", err)
	}
}

func TestOrgIDFromCtx_Isolation(t *testing.T) {
	orgID1 := uuid.New()
	orgID2 := uuid.New()

	ctx1 := WithOrgID(context.Background(), orgID1)
	ctx2 := WithOrgID(context.Background(), orgID2)

	got1, _ := OrgIDFromCtx(ctx1)
	got2, _ := OrgIDFromCtx(ctx2)

	if got1 != orgID1 {
		t.Fatalf("ctx1: expected %v, got %v", orgID1, got1)
	}
	if got2 != orgID2 {
		t.Fatalf("ctx2: expected %v, got %v", orgID2, got2)
	}
	if got1 == got2 {
		t.Fatal("expected different OrgIDs in isolated contexts")
	}
}
