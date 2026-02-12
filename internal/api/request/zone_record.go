package request

type CreateZoneRecord struct {
	Type     string `json:"type" validate:"required,oneof=A AAAA CNAME MX TXT SRV NS CAA PTR"`
	Name     string `json:"name" validate:"required"`
	Content  string `json:"content" validate:"required"`
	TTL      int    `json:"ttl" validate:"omitempty,min=60,max=86400"`
	Priority *int   `json:"priority"`
}

type UpdateZoneRecord struct {
	Content  string `json:"content" validate:"omitempty"`
	TTL      *int   `json:"ttl" validate:"omitempty,min=60,max=86400"`
	Priority *int   `json:"priority"`
}
