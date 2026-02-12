package request

type CreateValkeyUser struct {
	Username   string   `json:"username" validate:"required,slug"`
	Password   string   `json:"password" validate:"required,min=8"`
	Privileges []string `json:"privileges" validate:"required,min=1"`
	KeyPattern string   `json:"key_pattern" validate:"omitempty"`
}

type UpdateValkeyUser struct {
	Password   string   `json:"password" validate:"omitempty,min=8"`
	Privileges []string `json:"privileges" validate:"omitempty,min=1"`
	KeyPattern string   `json:"key_pattern" validate:"omitempty"`
}
