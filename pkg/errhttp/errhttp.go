// Package errhttp maps domain sentinel errors to HTTP status codes.
// Add a case to mapErrorToStatus for each new domain sentinel error.
package errhttp

import (
	"errors"
	"net/http"

	"github.com/ghuser/ghproject/pkg/httpx"
	itemdomain "github.com/ghuser/ghproject/services/item/domain"
)

// WriteError maps err to an HTTP status code and writes a JSON error response.
// Uses errors.Is() so wrapped sentinel errors are matched correctly.
// Defaults to 500 Internal Server Error for unrecognized errors.
func WriteError(w http.ResponseWriter, err error) {
	httpx.JSONError(w, mapErrorToStatus(err), err.Error())
}

func mapErrorToStatus(err error) int {
	switch {
	case errors.Is(err, itemdomain.ErrItemNotFound):
		return http.StatusNotFound // 404
	case errors.Is(err, itemdomain.ErrItemAlreadyExists):
		return http.StatusConflict // 409
	case errors.Is(err, itemdomain.ErrInvalidItemName):
		return http.StatusUnprocessableEntity // 422
	default:
		return http.StatusInternalServerError // 500
	}
}
