package model

type JWTClaims struct {
	Sub       string `json:"sub"`
	PartnerID string `json:"partner_id"`
	Email     string `json:"email"`
	Locale    string `json:"locale"`
	Exp       int64  `json:"exp"`
	Iat       int64  `json:"iat"`
	Iss       string `json:"iss"`
}
