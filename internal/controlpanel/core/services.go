package core

import "github.com/edvin/hosting/internal/controlpanel/config"

type Services struct {
	Auth         *AuthService
	Partner      *PartnerService
	User         *UserService
	Customer     *CustomerService
	Subscription *SubscriptionService
	Module       *ModuleService
	Product      *ProductService
	OIDC         *OIDCService
}

func NewServices(db DB, jwtSecret, jwtIssuer string, oidcProviders []config.OIDCProvider) *Services {
	auth := NewAuthService(db, jwtSecret, jwtIssuer)
	return &Services{
		Auth:         auth,
		Partner:      NewPartnerService(db),
		User:         NewUserService(db),
		Customer:     NewCustomerService(db),
		Subscription: NewSubscriptionService(db),
		Module:       NewModuleService(db),
		Product:      NewProductService(db),
		OIDC:         NewOIDCService(db, auth, oidcProviders, []byte(jwtSecret)),
	}
}
