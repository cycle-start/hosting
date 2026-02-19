package model

import "time"

type OIDCSigningKey struct {
	ID            string    `json:"id" db:"id"`
	Algorithm     string    `json:"algorithm" db:"algorithm"`
	PublicKeyPEM  string    `json:"public_key_pem" db:"public_key_pem"`
	PrivateKeyPEM string    `json:"private_key_pem" db:"private_key_pem"`
	Active        bool      `json:"active" db:"active"`
	CreatedAt     time.Time `json:"created_at" db:"created_at"`
}

type OIDCClient struct {
	ID           string    `json:"id" db:"id"`
	SecretHash   string    `json:"secret_hash" db:"secret_hash"`
	Name         string    `json:"name" db:"name"`
	RedirectURIs []string  `json:"redirect_uris" db:"redirect_uris"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
}

type OIDCAuthCode struct {
	Code        string    `json:"code" db:"code"`
	ClientID    string    `json:"client_id" db:"client_id"`
	TenantID    string    `json:"tenant_id" db:"tenant_id"`
	RedirectURI string    `json:"redirect_uri" db:"redirect_uri"`
	Scope       string    `json:"scope" db:"scope"`
	Nonce       string    `json:"nonce" db:"nonce"`
	ExpiresAt   time.Time `json:"expires_at" db:"expires_at"`
	Used        bool      `json:"used" db:"used"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
}

type OIDCLoginSession struct {
	ID         string    `json:"id" db:"id"`
	TenantID   string    `json:"tenant_id" db:"tenant_id"`
	DatabaseID *string   `json:"database_id,omitempty" db:"database_id"`
	ExpiresAt  time.Time `json:"expires_at" db:"expires_at"`
	Used       bool      `json:"used" db:"used"`
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
}
