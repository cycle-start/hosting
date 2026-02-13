package request

type CreateS3AccessKey struct {
	Permissions string `json:"permissions" validate:"omitempty,oneof=read-only read-write"`
}
