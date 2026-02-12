package model

import "time"

type ValkeyUser struct {
	ID               string   `json:"id" db:"id"`
	ValkeyInstanceID string   `json:"valkey_instance_id" db:"valkey_instance_id"`
	Username         string   `json:"username" db:"username"`
	Password         string   `json:"password,omitempty" db:"password"`
	Privileges       []string `json:"privileges" db:"privileges"`
	KeyPattern       string   `json:"key_pattern" db:"key_pattern"`
	Status           string   `json:"status" db:"status"`
	CreatedAt        time.Time `json:"created_at" db:"created_at"`
	UpdatedAt        time.Time `json:"updated_at" db:"updated_at"`
}
