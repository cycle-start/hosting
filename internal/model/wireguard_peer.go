package model

import "time"

type WireGuardPeer struct {
	ID             string    `json:"id" db:"id"`
	TenantID       string    `json:"tenant_id" db:"tenant_id"`
	SubscriptionID string    `json:"subscription_id" db:"subscription_id"`
	Name           string    `json:"name" db:"name"`
	PublicKey      string    `json:"public_key" db:"public_key"`
	PresharedKey   string    `json:"-" db:"preshared_key"`
	AssignedIP     string    `json:"assigned_ip" db:"assigned_ip"`
	PeerIndex      int       `json:"peer_index" db:"peer_index"`
	Endpoint       string    `json:"endpoint" db:"endpoint"`
	Status         string    `json:"status" db:"status"`
	StatusMessage  *string   `json:"status_message,omitempty" db:"status_message"`
	CreatedAt      time.Time `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time `json:"updated_at" db:"updated_at"`
}
