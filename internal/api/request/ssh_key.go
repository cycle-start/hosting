package request

// CreateSSHKey holds the request body for creating an SSH key.
type CreateSSHKey struct {
	Name      string `json:"name" validate:"required,min=1,max=255"`
	PublicKey string `json:"public_key" validate:"required"`
}
