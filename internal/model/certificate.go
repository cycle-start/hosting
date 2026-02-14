package model

import "time"

type Certificate struct {
	ID        string     `json:"id" db:"id"`
	FQDNID    string     `json:"fqdn_id" db:"fqdn_id"`
	Type      string     `json:"type" db:"type"`
	CertPEM   string     `json:"cert_pem,omitempty" db:"cert_pem"`
	KeyPEM    string     `json:"key_pem,omitempty" db:"key_pem"`
	ChainPEM  string     `json:"chain_pem,omitempty" db:"chain_pem"`
	IssuedAt  *time.Time `json:"issued_at,omitempty" db:"issued_at"`
	ExpiresAt *time.Time `json:"expires_at,omitempty" db:"expires_at"`
	Status        string     `json:"status" db:"status"`
	StatusMessage *string    `json:"status_message,omitempty" db:"status_message"`
	IsActive  bool       `json:"is_active" db:"is_active"`
	CreatedAt time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt time.Time  `json:"updated_at" db:"updated_at"`
}

const (
	CertTypeLetsEncrypt = "lets_encrypt"
	CertTypeCustom      = "custom"
)
