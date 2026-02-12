package request

type CreateEmailAlias struct {
	Address string `json:"address" validate:"required,email"`
}
