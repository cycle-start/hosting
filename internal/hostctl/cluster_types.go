package hostctl

type ClusterConfig struct {
	APIURL  string     `yaml:"api_url"`
	APIKey  string     `yaml:"api_key"`
	Region  RegionDef  `yaml:"region"`
	Cluster ClusterDef `yaml:"cluster"`
}

type RegionDef struct {
	Name string `yaml:"name"`
}

type ClusterDef struct {
	Name        string            `yaml:"name"`
	LBAddresses []LBAddressDef   `yaml:"lb_addresses"`
	Config      map[string]any    `yaml:"config"`
	Spec        ClusterSpecDef    `yaml:"spec"`
	Nodes       []NodeDef         `yaml:"nodes"`
}

type NodeDef struct {
	ID        string `yaml:"id"`
	ShardName string `yaml:"shard_name"`
	IPAddress string `yaml:"ip_address"`
}

type LBAddressDef struct {
	Address string `yaml:"address"`
	Label   string `yaml:"label"`
}

type ClusterSpecDef struct {
	Shards         []ShardSpecDef       `yaml:"shards"`
	Infrastructure InfrastructureSpecDef `yaml:"infrastructure"`
}

type ShardSpecDef struct {
	Name      string `yaml:"name"`
	Role      string `yaml:"role"`
	NodeCount int    `yaml:"node_count"`
}

type InfrastructureSpecDef struct {
	HAProxy   bool `yaml:"haproxy"`
	PowerDNS bool `yaml:"powerdns"`
	Valkey    bool `yaml:"valkey"`
}
