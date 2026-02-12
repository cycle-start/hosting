package stalwart

type CreateAccountParams struct {
	Address     string `json:"address"`
	DisplayName string `json:"display_name"`
	QuotaBytes  int64  `json:"quota_bytes"`
	Password    string `json:"password"`
}

type Account struct {
	Address     string `json:"address"`
	DisplayName string `json:"display_name"`
	QuotaBytes  int64  `json:"quota_bytes"`
}

type PatchOp struct {
	Action string `json:"action"` // "set", "addItem", "removeItem"
	Field  string `json:"field"`
	Value  any    `json:"value"`
}

// ForwardRule represents a single forwarding rule for Sieve script generation.
type ForwardRule struct {
	Destination string
	KeepCopy    bool
}

// VacationParams holds parameters for setting a vacation auto-reply via JMAP.
type VacationParams struct {
	Subject   string
	Body      string
	StartDate *string // RFC3339 or nil
	EndDate   *string // RFC3339 or nil
	Enabled   bool
}
