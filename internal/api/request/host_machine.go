package request

import "encoding/json"

type CreateHostMachine struct {
	Hostname      string          `json:"hostname" validate:"required"`
	IPAddress     string          `json:"ip_address" validate:"required"`
	DockerHost    string          `json:"docker_host" validate:"required"`
	CACertPEM     string          `json:"ca_cert_pem"`
	ClientCertPEM string          `json:"client_cert_pem"`
	ClientKeyPEM  string          `json:"client_key_pem"`
	Capacity      json.RawMessage `json:"capacity"`
	Roles         []string        `json:"roles"`
}

type UpdateHostMachine struct {
	Hostname      string          `json:"hostname"`
	IPAddress     string          `json:"ip_address"`
	DockerHost    string          `json:"docker_host"`
	CACertPEM     string          `json:"ca_cert_pem"`
	ClientCertPEM string          `json:"client_cert_pem"`
	ClientKeyPEM  string          `json:"client_key_pem"`
	Capacity      json.RawMessage `json:"capacity"`
	Roles         []string        `json:"roles"`
	Status        string          `json:"status"`
}
