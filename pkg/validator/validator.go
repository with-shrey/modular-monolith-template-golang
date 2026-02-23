package validator

import (
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strings"

	"github.com/go-playground/validator/v10"

	"github.com/ghuser/ghproject/pkg/httpx"
)

var validate *validator.Validate

func init() {
	validate = validator.New(validator.WithRequiredStructEnabled())

	validate.RegisterTagNameFunc(func(fld reflect.StructField) string {
		name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]

		// ignore unexported or explicitly ignored
		if name == "-" || name == "" {
			return fld.Name
		}
		return name
	})
}

// Validate runs struct-level validation using go-playground/validator tags.
func Validate(s any) error {
	return validate.Struct(s)
}

// FormatValidationErrors converts validator.ValidationErrors into a map of
// field name → human-readable message.
func FormatValidationErrors(err error) map[string]string {
	errs := make(map[string]string)
	var ve validator.ValidationErrors
	if !isValidationErrors(err, &ve) {
		return errs
	}
	for _, e := range ve {
		errs[e.Field()] = formatFieldError(e)
	}
	return errs
}

func isValidationErrors(err error, target *validator.ValidationErrors) bool {
	ve, ok := err.(validator.ValidationErrors)
	if ok {
		*target = ve
	}
	return ok
}

func formatFieldError(e validator.FieldError) string {
	switch e.Tag() {
	case "required":
		return "This field is required"
	case "uuid", "uuid4":
		return "Must be a valid UUID"
	case "min":
		return fmt.Sprintf("Minimum length is %s", e.Param())
	case "max":
		return fmt.Sprintf("Maximum length is %s", e.Param())
	case "email":
		return "Must be a valid email address"
	case "url":
		return "Must be a valid URL"
	case "numeric":
		return "Must be a numeric value"
	case "alpha":
		return "Must contain only letters"
	case "alphanum":
		return "Must contain only letters and numbers"
	case "gte":
		return fmt.Sprintf("Must be greater than or equal to %s", e.Param())
	case "lte":
		return fmt.Sprintf("Must be less than or equal to %s", e.Param())
	default:
		return fmt.Sprintf("Validation failed on '%s'", e.Tag())
	}
}

// ValidateRequest decodes the JSON request body into T, validates it, and
// writes an appropriate error response if either step fails.
// Returns (parsedStruct, true) on success or (nil, false) on failure.
func ValidateRequest[T any](w http.ResponseWriter, r *http.Request) (*T, bool) {
	var req T
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.JSONError(w, http.StatusBadRequest, "Invalid JSON")
		return nil, false
	}
	if err := Validate(&req); err != nil {
		httpx.JSON(w, http.StatusUnprocessableEntity, map[string]any{
			"error":  "Validation failed",
			"fields": FormatValidationErrors(err),
		})
		return nil, false
	}
	return &req, true
}
