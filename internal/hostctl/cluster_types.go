package hostctl

type ClusterConfig struct {
	APIURL       string           `yaml:"api_url"`
	Region       RegionDef        `yaml:"region"`
	Cluster      ClusterDef       `yaml:"cluster"`
	Hosts        []HostDef        `yaml:"hosts"`
	NodeProfiles []NodeProfileDef `yaml:"node_profiles"`
}

type RegionDef struct {
	Name string `yaml:"name"`
	ID   string `yaml:"id"`
}

type ClusterDef struct {
	Name          string            `yaml:"name"`
	Provisioner   string            `yaml:"provisioner"` // "docker" (default) or "external"
	LBAddresses   []LBAddressDef    `yaml:"lb_addresses"`
	Config        map[string]any    `yaml:"config"`
	Spec          ClusterSpecDef    `yaml:"spec"`
	ExternalNodes []ExternalNodeDef `yaml:"external_nodes"`
}

type ExternalNodeDef struct {
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
	ServiceDB bool `yaml:"service_db"`
	Valkey    bool `yaml:"valkey"`
}

type HostDef struct {
	Hostname   string      `yaml:"hostname"`
	IPAddress  string      `yaml:"ip_address"`
	DockerHost string      `yaml:"docker_host"`
	Capacity   CapacityDef `yaml:"capacity"`
	Roles      []string    `yaml:"roles"`
}

type CapacityDef struct {
	MaxNodes int `yaml:"max_nodes"`
}

type NodeProfileDef struct {
	Name        string            `yaml:"name"`
	Role        string            `yaml:"role"`
	Image       string            `yaml:"image"`
	Env         map[string]string `yaml:"env"`
	Volumes     []string          `yaml:"volumes"`
	Ports       []PortDef         `yaml:"ports"`
	Resources   ResourcesDef      `yaml:"resources"`
	HealthCheck *HealthCheckDef   `yaml:"health_check"`
	Privileged  bool              `yaml:"privileged"`
	NetworkMode string            `yaml:"network_mode"`
}

type PortDef struct {
	Host      int `yaml:"host"`
	Container int `yaml:"container"`
}

type ResourcesDef struct {
	MemoryMB  int64 `yaml:"memory_mb"`
	CPUShares int64 `yaml:"cpu_shares"`
}

type HealthCheckDef struct {
	Test     []string `yaml:"test"`
	Interval string   `yaml:"interval"`
	Timeout  string   `yaml:"timeout"`
	Retries  int      `yaml:"retries"`
}
