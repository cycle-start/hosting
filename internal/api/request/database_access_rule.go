package request

// CreateDatabaseAccessRule holds the request body for creating a database access rule.
type CreateDatabaseAccessRule struct {
	CIDR        string `json:"cidr" validate:"required,cidrv4|cidrv6"`
	Description string `json:"description" validate:"max=255"`
}
