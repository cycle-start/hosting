package request

type CreateEmailAccount struct {
	Address     string `json:"address" validate:"required,email"`
	DisplayName string `json:"display_name"`
	QuotaBytes  int64  `json:"quota_bytes"`
}
