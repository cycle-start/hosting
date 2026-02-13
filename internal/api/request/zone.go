package request

type CreateZone struct {
	Name     string  `json:"name" validate:"required,fqdn"`
	BrandID  string  `json:"brand_id"`
	TenantID *string `json:"tenant_id"`
	RegionID string  `json:"region_id" validate:"required"`
}

type UpdateZone struct {
	TenantID *string `json:"tenant_id"`
}

type ReassignZoneTenant struct {
	TenantID *string `json:"tenant_id"`
}
