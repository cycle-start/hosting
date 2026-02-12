package request

type CreateFQDN struct {
	FQDN       string `json:"fqdn" validate:"required,fqdn"`
	SSLEnabled *bool  `json:"ssl_enabled"`
}
