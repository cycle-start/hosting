package request

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"

	"github.com/go-playground/validator/v10"
)

var validate = validator.New()

var nameRegex = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,62}$`)

func init() {
	validate.RegisterValidation("slug", func(fl validator.FieldLevel) bool {
		return nameRegex.MatchString(fl.Field().String())
	})
}

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
