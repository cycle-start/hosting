package request

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-playground/validator/v10"
)

var validate = validator.New()

func Decode(r *http.Request, v any) error {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	if err := validate.Struct(v); err != nil {
		return fmt.Errorf("validation error: %w", err)
	}
	return nil
}

func RequireID(s string) (string, error) {
	if s == "" {
		return "", fmt.Errorf("missing required ID")
	}
	return s, nil
}
