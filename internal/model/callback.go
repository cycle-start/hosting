package model

// CallbackPayload is the JSON body POSTed to a callback URL when a
// provisioning workflow completes.
type CallbackPayload struct {
	ResourceType  string `json:"resource_type"`
	ResourceID    string `json:"resource_id"`
	Status        string `json:"status"`
	StatusMessage string `json:"status_message,omitempty"`
}
