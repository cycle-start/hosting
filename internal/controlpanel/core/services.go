package core

type Services struct {
	Auth         *AuthService
	Partner      *PartnerService
	User         *UserService
	Customer     *CustomerService
	Subscription *SubscriptionService
	Module       *ModuleService
	Product      *ProductService
}

func NewServices(db DB, jwtSecret, jwtIssuer string) *Services {
	return &Services{
		Auth:         NewAuthService(db, jwtSecret, jwtIssuer),
		Partner:      NewPartnerService(db),
		User:         NewUserService(db),
		Customer:     NewCustomerService(db),
		Subscription: NewSubscriptionService(db),
		Module:       NewModuleService(db),
		Product:      NewProductService(db),
	}
}
