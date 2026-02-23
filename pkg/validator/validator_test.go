package validator_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	pkgvalidator "github.com/ghuser/ghproject/pkg/validator"
)

type sampleStruct struct {
	OrgID string `validate:"required,uuid"`
	Name  string `validate:"required,min=1,max=10"`
	Email string `validate:"omitempty,email"`
}

func TestValidate_valid(t *testing.T) {
	s := sampleStruct{
		OrgID: "550e8400-e29b-41d4-a716-446655440000",
		Name:  "hello",
	}
	if err := pkgvalidator.Validate(&s); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

func TestValidate_missingRequired(t *testing.T) {
	s := sampleStruct{}
	if err := pkgvalidator.Validate(&s); err == nil {
		t.Fatal("expected validation error for empty struct")
	}
}

func TestFormatValidationErrors_required(t *testing.T) {
	s := sampleStruct{}
	err := pkgvalidator.Validate(&s)
	m := pkgvalidator.FormatValidationErrors(err)
	if m["OrgID"] != "This field is required" {
		t.Errorf("unexpected OrgID message: %q", m["OrgID"])
	}
	if m["Name"] != "This field is required" {
		t.Errorf("unexpected Name message: %q", m["Name"])
	}
}

func TestFormatValidationErrors_uuid(t *testing.T) {
	s := sampleStruct{OrgID: "not-a-uuid", Name: "ok"}
	err := pkgvalidator.Validate(&s)
	m := pkgvalidator.FormatValidationErrors(err)
	if m["OrgID"] != "Must be a valid UUID" {
		t.Errorf("unexpected OrgID message: %q", m["OrgID"])
	}
}

func TestFormatValidationErrors_min(t *testing.T) {
	s := sampleStruct{OrgID: "550e8400-e29b-41d4-a716-446655440000", Name: ""}
	err := pkgvalidator.Validate(&s)
	m := pkgvalidator.FormatValidationErrors(err)
	// empty string fails "required" before "min"
	if _, ok := m["Name"]; !ok {
		t.Error("expected Name validation error")
	}
}

func TestFormatValidationErrors_max(t *testing.T) {
	s := sampleStruct{OrgID: "550e8400-e29b-41d4-a716-446655440000", Name: "12345678901"} // 11 chars > max=10
	err := pkgvalidator.Validate(&s)
	m := pkgvalidator.FormatValidationErrors(err)
	if m["Name"] != "Maximum length is 10" {
		t.Errorf("unexpected Name message: %q", m["Name"])
	}
}

func TestFormatValidationErrors_nonValidationError(t *testing.T) {
	m := pkgvalidator.FormatValidationErrors(http.ErrNoCookie)
	if len(m) != 0 {
		t.Errorf("expected empty map for non-validation error, got %v", m)
	}
}

// --- ValidateRequest ---

type itemReq struct {
	OrgID string `json:"org_id" validate:"required,uuid"`
	Name  string `json:"name"   validate:"required,min=1,max=255"`
}

func TestValidateRequest_valid(t *testing.T) {
	body := `{"org_id":"550e8400-e29b-41d4-a716-446655440000","name":"widget"}`
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	req, ok := pkgvalidator.ValidateRequest[itemReq](w, r)
	if !ok {
		t.Fatalf("expected ok=true, got false. Response: %s", w.Body.String())
	}
	if req.Name != "widget" {
		t.Errorf("unexpected Name: %q", req.Name)
	}
}

func TestValidateRequest_invalidJSON(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("{bad json"))
	w := httptest.NewRecorder()

	_, ok := pkgvalidator.ValidateRequest[itemReq](w, r)
	if ok {
		t.Fatal("expected ok=false for malformed JSON")
	}
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "Invalid JSON") {
		t.Errorf("expected 'Invalid JSON' in body, got: %s", w.Body.String())
	}
}

func TestValidateRequest_missingField(t *testing.T) {
	body := `{"name":"widget"}`
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	w := httptest.NewRecorder()

	_, ok := pkgvalidator.ValidateRequest[itemReq](w, r)
	if ok {
		t.Fatal("expected ok=false for missing org_id")
	}
	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "Validation failed") {
		t.Errorf("expected 'Validation failed' in body, got: %s", w.Body.String())
	}
}

func TestValidateRequest_invalidUUID(t *testing.T) {
	body := `{"org_id":"not-uuid","name":"widget"}`
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	w := httptest.NewRecorder()

	_, ok := pkgvalidator.ValidateRequest[itemReq](w, r)
	if ok {
		t.Fatal("expected ok=false for invalid UUID")
	}
	if !strings.Contains(w.Body.String(), "UUID") {
		t.Errorf("expected UUID error in body, got: %s", w.Body.String())
	}
}
