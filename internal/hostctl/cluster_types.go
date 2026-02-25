package hostctl

type ClusterConfig struct {
	APIURL          string              `yaml:"api_url"`
	APIKey          string              `yaml:"api_key"`
	Region          RegionDef           `yaml:"region"`
	Cluster         ClusterDef          `yaml:"cluster"`
	ClusterRuntimes []ClusterRuntimeDef `yaml:"cluster_runtimes"`
}

type RegionDef struct {
	Name string `yaml:"name"`
}

type ClusterRuntimeDef struct {
	Runtime string `yaml:"runtime"`
	Version string `yaml:"version"`
}

type ClusterDef struct {
	Name        string            `yaml:"name"`
	LBAddresses []LBAddressDef   `yaml:"lb_addresses"`
	Config      map[string]any    `yaml:"config"`
	Spec        ClusterSpecDef    `yaml:"spec"`
	Nodes       []NodeDef         `yaml:"nodes"`
}

type NodeDef struct {
	ID         string   `yaml:"id"`
	Hostname   string   `yaml:"hostname"`
	ShardName  string   `yaml:"shard_name"`
	ShardNames []string `yaml:"shard_names"`
	IPAddress  string   `yaml:"ip_address"`
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
	LBBackend string `yaml:"lb_backend"`
	NodeCount int    `yaml:"node_count"`
}

type InfrastructureSpecDef struct {
	HAProxy   bool `yaml:"haproxy"`
	PowerDNS bool `yaml:"powerdns"`
	Valkey    bool `yaml:"valkey"`
}
