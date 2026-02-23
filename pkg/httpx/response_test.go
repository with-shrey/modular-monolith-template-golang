package httpx_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ghuser/ghproject/pkg/httpx"
)

func TestJSON_setsHeaders(t *testing.T) {
	w := httptest.NewRecorder()
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "ok"})

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Errorf("unexpected Content-Type: %q", ct)
	}
	if xct := w.Header().Get("X-Content-Type-Options"); xct != "nosniff" {
		t.Errorf("expected nosniff, got %q", xct)
	}
}

func TestJSON_encodesBody(t *testing.T) {
	w := httptest.NewRecorder()
	httpx.JSON(w, http.StatusCreated, map[string]string{"id": "abc"})

	var body map[string]string
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if body["id"] != "abc" {
		t.Errorf("unexpected body: %v", body)
	}
	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d", w.Code)
	}
}

func TestJSONError(t *testing.T) {
	w := httptest.NewRecorder()
	httpx.JSONError(w, http.StatusBadRequest, "something went wrong")

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
	var body map[string]string
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if body["error"] != "something went wrong" {
		t.Errorf("unexpected error message: %q", body["error"])
	}
}
