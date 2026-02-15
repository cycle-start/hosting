package model

import "time"

type Brand struct {
	ID               string    `json:"id" db:"id"`
	Name             string    `json:"name" db:"name"`
	BaseHostname     string    `json:"base_hostname" db:"base_hostname"`
	PrimaryNS        string    `json:"primary_ns" db:"primary_ns"`
	SecondaryNS      string    `json:"secondary_ns" db:"secondary_ns"`
	HostmasterEmail  string    `json:"hostmaster_email" db:"hostmaster_email"`
	MailHostname     string    `json:"mail_hostname" db:"mail_hostname"`
	SPFIncludes      string    `json:"spf_includes" db:"spf_includes"`
	DKIMSelector     string    `json:"dkim_selector" db:"dkim_selector"`
	DKIMPublicKey    string    `json:"dkim_public_key" db:"dkim_public_key"`
	DMARCPolicy      string    `json:"dmarc_policy" db:"dmarc_policy"`
	Status           string    `json:"status" db:"status"`
	CreatedAt        time.Time `json:"created_at" db:"created_at"`
	UpdatedAt        time.Time `json:"updated_at" db:"updated_at"`
}
