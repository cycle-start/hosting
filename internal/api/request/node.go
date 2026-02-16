package request

type CreateNode struct {
	ID         string   `json:"id"`
	Hostname   string   `json:"hostname" validate:"required"`
	IPAddress  *string  `json:"ip_address"`
	IP6Address *string  `json:"ip6_address"`
	ShardID    *string  `json:"shard_id"`
	ShardIDs   []string `json:"shard_ids"`
	Roles      []string `json:"roles" validate:"required,min=1"`
}

type UpdateNode struct {
	Hostname   string   `json:"hostname"`
	IPAddress  *string  `json:"ip_address"`
	IP6Address *string  `json:"ip6_address"`
	ShardID    *string  `json:"shard_id"`
	ShardIDs   []string `json:"shard_ids"`
	Roles      []string `json:"roles"`
	Status     string   `json:"status"`
}
