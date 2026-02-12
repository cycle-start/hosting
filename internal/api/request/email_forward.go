package request

type CreateEmailForward struct {
	Destination string `json:"destination" validate:"required,email"`
	KeepCopy    *bool  `json:"keep_copy"`
}
