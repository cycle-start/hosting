package request

// CreateAPIKey holds the request body for creating an API key.
type CreateAPIKey struct {
	Name   string   `json:"name" validate:"required,min=1,max=255"`
	Scopes []string `json:"scopes"`
}
