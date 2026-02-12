package request

type CreateClusterLBAddress struct {
	Address string `json:"address" validate:"required"`
	Label   string `json:"label"`
}
