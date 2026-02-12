package model

import (
	"encoding/json"
	"time"
)

type HostMachine struct {
	ID            string          `json:"id" db:"id"`
	ClusterID     string          `json:"cluster_id" db:"cluster_id"`
	Hostname      string          `json:"hostname" db:"hostname"`
	IPAddress     string          `json:"ip_address" db:"ip_address"`
	DockerHost    string          `json:"docker_host" db:"docker_host"`
	CACertPEM     string          `json:"ca_cert_pem,omitempty" db:"ca_cert_pem"`
	ClientCertPEM string          `json:"client_cert_pem,omitempty" db:"client_cert_pem"`
	ClientKeyPEM  string          `json:"client_key_pem,omitempty" db:"client_key_pem"`
	Capacity      json.RawMessage `json:"capacity" db:"capacity"`
	Roles         []string        `json:"roles" db:"roles"`
	Status        string          `json:"status" db:"status"`
	CreatedAt     time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at" db:"updated_at"`
}
