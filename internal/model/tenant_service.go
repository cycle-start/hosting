package model

import "time"

type TenantService struct {
	ID        string    `json:"id" db:"id"`
	TenantID  string    `json:"tenant_id" db:"tenant_id"`
	Service   string    `json:"service" db:"service"`
	NodeID    string    `json:"node_id" db:"node_id"`
	Hostname  string    `json:"hostname" db:"hostname"`
	Enabled   bool      `json:"enabled" db:"enabled"`
	Status    string    `json:"status" db:"status"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

const (
	ServiceSSH   = "ssh"
	ServiceMySQL = "mysql"
	ServiceMail  = "mail"
	ServiceS3    = "s3"
)
