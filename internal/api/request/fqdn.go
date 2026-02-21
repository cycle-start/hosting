package request

type CreateFQDN struct {
	FQDN          string                     `json:"fqdn" validate:"required,fqdn"`
	WebrootID     *string                    `json:"webroot_id"`
	SSLEnabled    *bool                      `json:"ssl_enabled"`
	EmailAccounts []CreateEmailAccountNested `json:"email_accounts" validate:"omitempty,dive"`
}

type UpdateFQDN struct {
	WebrootID  *string `json:"webroot_id"`
	SSLEnabled *bool   `json:"ssl_enabled"`
}
