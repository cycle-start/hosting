package model

import "time"

// TenantEgressRule represents a per-tenant network egress restriction.
// Rules control which destination CIDRs a tenant's processes can reach.
type TenantEgressRule struct {
	ID            string    `json:"id" db:"id"`
	TenantID      string    `json:"tenant_id" db:"tenant_id"`
	CIDR          string    `json:"cidr" db:"cidr"`
	Description   string    `json:"description" db:"description"`
	Status        string    `json:"status" db:"status"`
	StatusMessage *string   `json:"status_message,omitempty" db:"status_message"`
	CreatedAt     time.Time `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time `json:"updated_at" db:"updated_at"`
}
