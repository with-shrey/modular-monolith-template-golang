package httpx

import (
	"encoding/json"
	"net/http"
)

// JSON writes v as JSON with the given status code. Content-Type and
// X-Content-Type-Options headers are set automatically. Encoding errors are
// silently discarded — use this for handler responses, not for streaming.
func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// JSONError writes a standard {"error": message} JSON response.
func JSONError(w http.ResponseWriter, status int, message string) {
	JSON(w, status, map[string]string{"error": message})
}

// SafeError returns the error message for client responses.
// In production (isProduction=true), internal server errors (5xx) are replaced
// with a generic message to avoid leaking implementation details.
func SafeError(err error, status int, isProduction bool) string {
	if isProduction && status >= http.StatusInternalServerError {
		return http.StatusText(status)
	}
	return err.Error()
}
