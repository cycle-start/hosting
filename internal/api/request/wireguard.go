package request

type CreateWireGuardPeer struct {
	Name           string `json:"name" validate:"required"`
	SubscriptionID string `json:"subscription_id" validate:"required"`
}
