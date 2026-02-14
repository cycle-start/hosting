package model

import "time"

// SSHKey represents an SSH public key used for SFTP access to a tenant's storage.
type SSHKey struct {
	ID          string    `json:"id"`
	TenantID    string    `json:"tenant_id"`
	Name        string    `json:"name"`
	PublicKey   string    `json:"public_key,omitempty"`
	Fingerprint string    `json:"fingerprint"`
	Status        string    `json:"status"`
	StatusMessage *string   `json:"status_message,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
