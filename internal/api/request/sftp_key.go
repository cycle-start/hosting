package request

// CreateSFTPKey holds the request body for creating an SFTP key.
type CreateSFTPKey struct {
	Name      string `json:"name" validate:"required,min=1,max=255"`
	PublicKey string `json:"public_key" validate:"required"`
}
