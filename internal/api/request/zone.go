package request

type CreateZone struct {
	Name           string `json:"name" validate:"required,fqdn"`
	BrandID        string `json:"brand_id"`
	TenantID       string `json:"tenant_id"`
	SubscriptionID string `json:"subscription_id" validate:"required"`
	RegionID       string `json:"region_id" validate:"required"`
}

type UpdateZone struct {
	TenantID *string `json:"tenant_id"`
}
