package auth

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

// contextKey is an unexported type to prevent key collisions in context.
type contextKey string

const orgIDKey contextKey = "org_id"

// ErrOrgIDNotFound is returned when no OrgID exists in the request context.
// Handlers should return 401 when this error occurs.
var ErrOrgIDNotFound = errors.New("org_id not found in context")

// OrgIDFromCtx extracts the authenticated organization ID from the request context.
// Returns uuid.Nil and ErrOrgIDNotFound if no OrgID is set (unauthenticated request).
func OrgIDFromCtx(ctx context.Context) (uuid.UUID, error) {
	orgID, ok := ctx.Value(orgIDKey).(uuid.UUID)
	if !ok || orgID == uuid.Nil {
		return uuid.Nil, ErrOrgIDNotFound
	}
	return orgID, nil
}

// WithOrgID returns a new context with the given OrgID attached.
// Used by authentication middleware after validating the session.
func WithOrgID(ctx context.Context, orgID uuid.UUID) context.Context {
	return context.WithValue(ctx, orgIDKey, orgID)
}
