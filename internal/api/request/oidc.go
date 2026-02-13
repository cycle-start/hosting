package request

type TokenRequest struct {
	GrantType   string `json:"grant_type" validate:"required"`
	Code        string `json:"code" validate:"required"`
	ClientID    string `json:"client_id" validate:"required"`
	ClientSecret string `json:"client_secret" validate:"required"`
	RedirectURI string `json:"redirect_uri"`
}
