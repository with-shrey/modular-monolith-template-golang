package errhttp

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	itemdomain "github.com/ghuser/ghproject/services/item/domain"
)

func TestWriteError_StatusCodes(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
	}{
		{"ErrItemNotFound", itemdomain.ErrItemNotFound, http.StatusNotFound},
		{"ErrItemAlreadyExists", itemdomain.ErrItemAlreadyExists, http.StatusConflict},
		{"ErrInvalidItemName", itemdomain.ErrInvalidItemName, http.StatusUnprocessableEntity},
		{"wrapped ErrItemNotFound", fmt.Errorf("get item: %w", itemdomain.ErrItemNotFound), http.StatusNotFound},
		{"wrapped ErrInvalidItemName", fmt.Errorf("%w: too long", itemdomain.ErrInvalidItemName), http.StatusUnprocessableEntity},
		{"unknown error", errors.New("something unexpected"), http.StatusInternalServerError},
		{"generic wrapped error", fmt.Errorf("context: %w", errors.New("db down")), http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			WriteError(w, tt.err)

			if w.Code != tt.wantStatus {
				t.Fatalf("expected status %d, got %d", tt.wantStatus, w.Code)
			}
		})
	}
}

func TestWriteError_JSONBody(t *testing.T) {
	w := httptest.NewRecorder()
	WriteError(w, itemdomain.ErrItemNotFound)

	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("response body is not valid JSON: %v", err)
	}
	if _, ok := body["error"]; !ok {
		t.Fatal("response body missing 'error' key")
	}
}

func TestWriteError_ContentType(t *testing.T) {
	w := httptest.NewRecorder()
	WriteError(w, itemdomain.ErrItemNotFound)

	ct := w.Header().Get("Content-Type")
	if ct == "" {
		t.Fatal("Content-Type header not set")
	}
}
