package request

// CreateTenantEgressRule holds the request body for creating an egress rule.
type CreateTenantEgressRule struct {
	CIDR        string `json:"cidr" validate:"required,cidrv4|cidrv6"`
	Action      string `json:"action" validate:"required,oneof=deny allow"`
	Description string `json:"description" validate:"max=255"`
}
