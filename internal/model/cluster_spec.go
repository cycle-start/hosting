package model

type ClusterSpec struct {
	Shards         []ClusterShardSpec        `json:"shards"`
	Infrastructure ClusterInfrastructureSpec `json:"infrastructure"`
}

type ClusterShardSpec struct {
	Name      string `json:"name"`
	Role      string `json:"role"`
	NodeCount int    `json:"node_count"`
}

type ClusterInfrastructureSpec struct {
	HAProxy   bool `json:"haproxy"`
	PowerDNS bool `json:"powerdns"`
	Valkey    bool `json:"valkey"`
}
