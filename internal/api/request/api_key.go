package request

// CreateAPIKey holds the request body for creating an API key.
type CreateAPIKey struct {
	Name   string   `json:"name" validate:"required,min=1,max=255"`
	Scopes []string `json:"scopes" validate:"required,min=1"`
	Brands []string `json:"brands" validate:"required,min=1"`
}

// UpdateAPIKey holds the request body for updating an API key.
type UpdateAPIKey struct {
	Name   string   `json:"name" validate:"required,min=1,max=255"`
	Scopes []string `json:"scopes" validate:"required,min=1"`
	Brands []string `json:"brands" validate:"required,min=1"`
}
