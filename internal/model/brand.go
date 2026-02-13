package model

import "time"

type Brand struct {
	ID               string    `json:"id" db:"id"`
	Name             string    `json:"name" db:"name"`
	BaseHostname     string    `json:"base_hostname" db:"base_hostname"`
	PrimaryNS        string    `json:"primary_ns" db:"primary_ns"`
	SecondaryNS      string    `json:"secondary_ns" db:"secondary_ns"`
	HostmasterEmail  string    `json:"hostmaster_email" db:"hostmaster_email"`
	Status           string    `json:"status" db:"status"`
	CreatedAt        time.Time `json:"created_at" db:"created_at"`
	UpdatedAt        time.Time `json:"updated_at" db:"updated_at"`
}
