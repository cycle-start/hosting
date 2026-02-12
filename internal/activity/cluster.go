package activity

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"text/template"

	"github.com/edvin/hosting/internal/deployer"
	"github.com/edvin/hosting/internal/model"
)

type Cluster struct {
	deployer deployer.Deployer
	db       DB
}

func NewCluster(d deployer.Deployer, db DB) *Cluster {
	return &Cluster{deployer: d, db: db}
}

type ValidateHostReachableParams struct {
	Host model.HostMachine `json:"host"`
}

func (a *Cluster) ValidateHostReachable(ctx context.Context, params ValidateHostReachableParams) error {
	// For unix socket hosts, verify the deployer can list containers.
	// For TCP hosts, verify the Docker API is reachable.
	if strings.HasPrefix(params.Host.DockerHost, "unix://") {
		_, err := a.deployer.InspectContainer(ctx, &params.Host, "nonexistent")
		// "not found" is fine — it means the Docker API responded.
		if err != nil && !strings.Contains(err.Error(), "No such container") && !strings.Contains(err.Error(), "not found") {
			return fmt.Errorf("host %s: docker not reachable via %s: %w", params.Host.Hostname, params.Host.DockerHost, err)
		}
		return nil
	}

	// For remote Docker hosts, ping via the deployer.
	_, err := a.deployer.InspectContainer(ctx, &params.Host, "nonexistent")
	if err != nil && !strings.Contains(err.Error(), "No such container") && !strings.Contains(err.Error(), "not found") {
		return fmt.Errorf("host %s: docker not reachable via %s: %w", params.Host.Hostname, params.Host.DockerHost, err)
	}
	return nil
}

type SelectHostForInfraParams struct {
	ClusterID   string `json:"cluster_id"`
	ServiceType string `json:"service_type"`
}

func (a *Cluster) SelectHostForInfra(ctx context.Context, params SelectHostForInfraParams) (*model.HostMachine, error) {
	var h model.HostMachine
	err := a.db.QueryRow(ctx,
		`SELECT hm.id, hm.cluster_id, hm.hostname, hm.ip_address::text, hm.docker_host, hm.ca_cert_pem, hm.client_cert_pem, hm.client_key_pem, hm.capacity, hm.roles, hm.status, hm.created_at, hm.updated_at
		 FROM host_machines hm
		 LEFT JOIN infrastructure_services isvc ON isvc.host_machine_id = hm.id AND isvc.status != $1
		 WHERE hm.cluster_id = $2 AND hm.status = $3
		 GROUP BY hm.id
		 ORDER BY COUNT(isvc.id) ASC
		 LIMIT 1`, model.StatusDeleted, params.ClusterID, model.StatusActive,
	).Scan(&h.ID, &h.ClusterID, &h.Hostname, &h.IPAddress, &h.DockerHost, &h.CACertPEM, &h.ClientCertPEM, &h.ClientKeyPEM, &h.Capacity, &h.Roles, &h.Status, &h.CreatedAt, &h.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("select host for infra: %w", err)
	}
	return &h, nil
}

type ConfigureHAProxyBackendsParams struct {
	ClusterID string `json:"cluster_id"`
}

// haproxyBackend holds data for generating a backend section in haproxy.cfg.
type haproxyBackend struct {
	Name    string
	Servers []haproxyServer
}

type haproxyServer struct {
	Name          string
	ContainerName string
	Port          int
}

var haproxyCfgTmpl = template.Must(template.New("haproxy").Parse(`global
    log stdout format raw local0
    maxconn 4096
    stats socket /var/run/haproxy/admin.sock mode 660 level admin expose-fd listeners
    stats timeout 30s

defaults
    log     global
    mode    http
    option  httplog
    option  dontlognull
    timeout connect 5000ms
    timeout client  50000ms
    timeout server  50000ms

# Stats UI
frontend stats
    bind *:8404
    stats enable
    stats uri /stats
    stats refresh 10s

# Main HTTP frontend
frontend http
    bind *:80
    use_backend %[req.hdr(host),lower,map(/var/lib/haproxy/maps/fqdn-to-shard.map,shard-default)]

# Default backend (returns 503 for unmapped FQDNs)
backend shard-default
    mode http
    http-request deny deny_status 503
{{range .}}
backend {{.Name}}
    balance hdr(Host)
    hash-type consistent{{range .Servers}}
    server {{.Name}} {{.ContainerName}}:{{.Port}} check{{end}}
{{end}}`))

func (a *Cluster) ConfigureHAProxyBackends(ctx context.Context, params ConfigureHAProxyBackendsParams) error {
	// 1. Get cluster config to find HAProxy container name.
	var cluster model.Cluster
	err := a.db.QueryRow(ctx,
		`SELECT id, region_id, name, config, status, spec, created_at, updated_at
		 FROM clusters WHERE id = $1`, params.ClusterID,
	).Scan(&cluster.ID, &cluster.RegionID, &cluster.Name, &cluster.Config,
		&cluster.Status, &cluster.Spec, &cluster.CreatedAt, &cluster.UpdatedAt)
	if err != nil {
		return fmt.Errorf("get cluster: %w", err)
	}

	containerName := defaultHAProxyContainer
	var cfg struct {
		HAProxyContainer string `json:"haproxy_container"`
	}
	if json.Unmarshal(cluster.Config, &cfg) == nil && cfg.HAProxyContainer != "" {
		containerName = cfg.HAProxyContainer
	}

	// 2. Get any active host machine for Docker access.
	var host model.HostMachine
	err = a.db.QueryRow(ctx,
		`SELECT id, cluster_id, hostname, ip_address::text, docker_host, ca_cert_pem, client_cert_pem, client_key_pem, capacity, roles, status, created_at, updated_at
		 FROM host_machines WHERE cluster_id = $1 AND status = $2
		 LIMIT 1`, params.ClusterID, model.StatusActive,
	).Scan(&host.ID, &host.ClusterID, &host.Hostname, &host.IPAddress, &host.DockerHost,
		&host.CACertPEM, &host.ClientCertPEM, &host.ClientKeyPEM,
		&host.Capacity, &host.Roles, &host.Status, &host.CreatedAt, &host.UpdatedAt)
	if err != nil {
		return fmt.Errorf("find host for haproxy: %w", err)
	}

	// 3. Query all active web shards with LBBackend set.
	shardRows, err := a.db.Query(ctx,
		`SELECT id, name, lb_backend FROM shards
		 WHERE cluster_id = $1 AND role = $2 AND status = $3 AND lb_backend != ''
		 ORDER BY name`, params.ClusterID, model.ShardRoleWeb, model.StatusActive)
	if err != nil {
		return fmt.Errorf("query web shards: %w", err)
	}
	defer shardRows.Close()

	var backends []haproxyBackend
	for shardRows.Next() {
		var shardID, shardName, lbBackend string
		if err := shardRows.Scan(&shardID, &shardName, &lbBackend); err != nil {
			return fmt.Errorf("scan shard: %w", err)
		}

		// Query active nodes for this shard + their deployments for container names.
		nodeRows, err := a.db.Query(ctx,
			`SELECT n.hostname, nd.container_name
			 FROM nodes n
			 JOIN node_deployments nd ON nd.node_id = n.id AND nd.status = $1
			 WHERE n.shard_id = $2 AND n.status = $3
			 ORDER BY n.hostname`, model.StatusActive, shardID, model.StatusActive)
		if err != nil {
			return fmt.Errorf("query nodes for shard %s: %w", shardName, err)
		}

		var servers []haproxyServer
		for nodeRows.Next() {
			var hostname, cName string
			if err := nodeRows.Scan(&hostname, &cName); err != nil {
				nodeRows.Close()
				return fmt.Errorf("scan node: %w", err)
			}
			servers = append(servers, haproxyServer{
				Name:          hostname,
				ContainerName: cName,
				Port:          80,
			})
		}
		nodeRows.Close()

		if len(servers) > 0 {
			backends = append(backends, haproxyBackend{
				Name:    lbBackend,
				Servers: servers,
			})
		}
	}
	if err := shardRows.Err(); err != nil {
		return fmt.Errorf("iterate shards: %w", err)
	}

	// 4. Generate the full haproxy.cfg.
	var buf bytes.Buffer
	if err := haproxyCfgTmpl.Execute(&buf, backends); err != nil {
		return fmt.Errorf("render haproxy.cfg: %w", err)
	}
	cfgContent := buf.String()

	// 5. Write config into the HAProxy container.
	// Use printf to write the config file (handles newlines correctly).
	writeResult, err := a.deployer.ExecInContainer(ctx, &host, containerName, []string{
		"sh", "-c", fmt.Sprintf("printf '%%s' '%s' > /usr/local/etc/haproxy/haproxy.cfg",
			strings.ReplaceAll(strings.ReplaceAll(cfgContent, "'", "'\"'\"'"), "\n", "\n")),
	})
	if err != nil {
		return fmt.Errorf("write haproxy.cfg: %w", err)
	}
	if writeResult.ExitCode != 0 {
		return fmt.Errorf("write haproxy.cfg: exit %d: %s", writeResult.ExitCode,
			strings.TrimSpace(writeResult.Stderr+writeResult.Stdout))
	}

	// 6. Validate the config.
	validateResult, err := a.deployer.ExecInContainer(ctx, &host, containerName, []string{
		"haproxy", "-c", "-f", "/usr/local/etc/haproxy/haproxy.cfg",
	})
	if err != nil {
		return fmt.Errorf("validate haproxy.cfg: %w", err)
	}
	if validateResult.ExitCode != 0 {
		return fmt.Errorf("haproxy.cfg validation failed: %s",
			strings.TrimSpace(validateResult.Stderr+validateResult.Stdout))
	}

	// 7. Restart HAProxy: stop + start.
	if err := a.deployer.StopContainer(ctx, &host, containerName); err != nil {
		return fmt.Errorf("stop haproxy: %w", err)
	}
	if err := a.deployer.StartContainer(ctx, &host, containerName); err != nil {
		return fmt.Errorf("start haproxy: %w", err)
	}

	return nil
}

type RunClusterSmokeTestParams struct {
	ClusterID string `json:"cluster_id"`
}

func (a *Cluster) RunClusterSmokeTest(ctx context.Context, params RunClusterSmokeTestParams) error {
	// Placeholder: in production this would run health checks against
	// the cluster's infrastructure services and verify connectivity.
	return nil
}
