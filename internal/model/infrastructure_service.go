package model

import (
	"encoding/json"
	"time"
)

type InfrastructureService struct {
	ID            string          `json:"id" db:"id"`
	ClusterID     string          `json:"cluster_id" db:"cluster_id"`
	HostMachineID string          `json:"host_machine_id" db:"host_machine_id"`
	ServiceType   string          `json:"service_type" db:"service_type"`
	ContainerID   string          `json:"container_id" db:"container_id"`
	ContainerName string          `json:"container_name" db:"container_name"`
	Image         string          `json:"image" db:"image"`
	Config        json.RawMessage `json:"config" db:"config"`
	Status        string          `json:"status" db:"status"`
	CreatedAt     time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at" db:"updated_at"`
}

const (
	InfraServiceHAProxy   = "haproxy"
	InfraServiceServiceDB = "service_db"
	InfraServiceValkey    = "valkey"
)
