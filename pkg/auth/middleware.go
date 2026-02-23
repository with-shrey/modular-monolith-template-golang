package auth

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/gorilla/sessions"

	"github.com/ghuser/ghproject/pkg/httpx"
	"github.com/ghuser/ghproject/pkg/logger"
)

const sessionName = "hastyconnect_session"
const sessionOrgIDKey = "org_id"

// RequireAuth is a chi middleware that enforces authentication via session cookies.
// It reads the session cookie, extracts the OrgID, and injects it into the request context.
// Returns 401 Unauthorized if the session is missing, invalid, or lacks a valid org_id.
//
// After this middleware, handlers can safely call auth.OrgIDFromCtx(r.Context()).
func RequireAuth(store sessions.Store, log logger.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			session, err := store.Get(r, sessionName)
			if err != nil {
				log.WarnContext(r.Context(), "invalid session cookie", "error", err)
				httpx.JSON(w, http.StatusUnauthorized, map[string]string{"error": "authentication required"})
				return
			}

			orgIDStr, ok := session.Values[sessionOrgIDKey].(string)
			if !ok || orgIDStr == "" {
				log.WarnContext(r.Context(), "session missing org_id")
				httpx.JSON(w, http.StatusUnauthorized, map[string]string{"error": "authentication required"})
				return
			}

			orgID, err := uuid.Parse(orgIDStr)
			if err != nil {
				log.WarnContext(r.Context(), "invalid org_id in session", "org_id", orgIDStr, "error", err)
				httpx.JSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid session data"})
				return
			}

			ctx := WithOrgID(r.Context(), orgID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
