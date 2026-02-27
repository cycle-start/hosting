package setup

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

// GenerateResult describes the files that were generated.
type GenerateResult struct {
	OutputDir string          `json:"output_dir"`
	Files     []GeneratedFile `json:"files"`
}

// GeneratedFile describes a single generated file.
type GeneratedFile struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

// ProgressFunc is called with status messages during generation.
type ProgressFunc func(msg string)

// Generate produces all deployment configuration files from the setup manifest.
func Generate(cfg *Config, outputDir string, progress ProgressFunc) (*GenerateResult, error) {
	if progress == nil {
		progress = func(string) {}
	}
	absDir, err := filepath.Abs(outputDir)
	if err != nil {
		absDir = outputDir
	}
	result := &GenerateResult{OutputDir: absDir}

	progress("Writing setup manifest...")

	// 1. Write the setup manifest itself
	manifestPath := filepath.Join(outputDir, ManifestFilename)
	if err := WriteManifest(cfg, manifestPath); err != nil {
		return nil, fmt.Errorf("write manifest: %w", err)
	}
	manifestData, _ := os.ReadFile(manifestPath)
	result.Files = append(result.Files, GeneratedFile{
		Path:    ManifestFilename,
		Content: string(manifestData),
	})

	// Generate Ceph keys and web client keyring via Docker
	cephFSID := uuid.New().String()

	progress("Generating Ceph web client keyring (docker)...")
	cephKeyring, cephWebKey, err := generateCephKeyring()
	if err != nil {
		return nil, fmt.Errorf("generate ceph keyring: %w", err)
	}

	// Determine the controlplane IP
	controlplaneIP := cfg.singleNodeIP()
	if cfg.DeployMode == DeployModeMulti {
		for _, n := range cfg.Nodes {
			for _, r := range n.Roles {
				if r == RoleControlPlane {
					controlplaneIP = n.IP
					break
				}
			}
		}
	}

	// Determine storage node IP
	storageNodeIP := controlplaneIP
	if cfg.DeployMode == DeployModeMulti {
		for _, n := range cfg.Nodes {
			for _, r := range n.Roles {
				if r == RoleStorage {
					storageNodeIP = n.IP
					break
				}
			}
		}
	}

	progress("Generating Ansible inventory and group vars...")

	// 2. Generate Ansible inventory (static.ini)
	ini := generateInventory(cfg, controlplaneIP)
	result.Files = append(result.Files, GeneratedFile{
		Path:    "generated/ansible/inventory/static.ini",
		Content: ini,
	})

	// 3. Generate group_vars/all.yml
	allYml := generateAllGroupVars(cfg, controlplaneIP, storageNodeIP, cephFSID, cephWebKey)
	result.Files = append(result.Files, GeneratedFile{
		Path:    "generated/ansible/inventory/group_vars/all.yml",
		Content: allYml,
	})

	// 3b. Generate per-role group_vars
	for _, gv := range generateRoleGroupVars(cfg, controlplaneIP) {
		result.Files = append(result.Files, gv)
	}

	progress("Generating cluster topology and seed data...")

	// 4. Generate cluster.yaml (for hostctl cluster apply)
	clusterYml := generateClusterYAML(cfg, controlplaneIP)
	result.Files = append(result.Files, GeneratedFile{
		Path:    "generated/cluster.yaml",
		Content: clusterYml,
	})

	// 5. Generate seed.yaml (brand + initial data)
	seedYml := generateSeedYAML(cfg)
	result.Files = append(result.Files, GeneratedFile{
		Path:    "generated/seed.yaml",
		Content: seedYml,
	})

	// 6. Include the pre-generated Ceph web client keyring
	result.Files = append(result.Files, GeneratedFile{
		Path:    "generated/ceph.client.web.keyring",
		Content: cephKeyring,
	})

	progress("Writing files to disk...")

	// Write deployment files to disk (manifest already written above)
	for _, f := range result.Files[1:] {
		fullPath := filepath.Join(outputDir, f.Path)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
			return nil, fmt.Errorf("mkdir %s: %w", filepath.Dir(fullPath), err)
		}
		if err := os.WriteFile(fullPath, []byte(f.Content), 0o644); err != nil {
			return nil, fmt.Errorf("write %s: %w", fullPath, err)
		}
	}

	return result, nil
}

func generateInventory(cfg *Config, controlplaneIP string) string {
	var b strings.Builder

	if cfg.DeployMode == DeployModeSingle {
		ip := cfg.singleNodeIP()
		isLocal := ip == "127.0.0.1" || ip == "localhost"
		var hostEntry string
		if isLocal {
			hostEntry = "localhost ansible_connection=local"
		} else {
			hostEntry = fmt.Sprintf("%s ansible_host=%s", ip, ip)
		}
		b.WriteString("[controlplane]\n")
		b.WriteString(hostEntry + "\n\n")
		for _, role := range []string{"web", "db", "dns", "valkey", "email", "storage", "lb", "gateway", "dbadmin"} {
			b.WriteString(fmt.Sprintf("[%s]\n", role))
			extra := ""
			if role == "db" {
				extra = " mysql_server_id=1"
			}
			b.WriteString(hostEntry + extra + "\n\n")
		}
		return b.String()
	}

	// Multi-node: group hosts by role
	roleMap := map[NodeRole][]NodeConfig{}
	for _, n := range cfg.Nodes {
		for _, r := range n.Roles {
			roleMap[r] = append(roleMap[r], n)
		}
	}

	roleToGroup := map[NodeRole]string{
		RoleControlPlane: "controlplane",
		RoleWeb:          "web",
		RoleDatabase:     "db",
		RoleDNS:          "dns",
		RoleValkey:       "valkey",
		RoleEmail:        "email",
		RoleStorage:      "storage",
		RoleLB:           "lb",
		RoleGateway:      "gateway",
		RoleDBAdmin:      "dbadmin",
	}

	for _, role := range AllRoles {
		group := roleToGroup[role]
		nodes := roleMap[role]
		b.WriteString(fmt.Sprintf("[%s]\n", group))
		for i, n := range nodes {
			extra := ""
			if role == RoleDatabase {
				extra = fmt.Sprintf(" mysql_server_id=%d", i+1)
			}
			b.WriteString(fmt.Sprintf("%s ansible_host=%s%s\n", n.Hostname, n.IP, extra))
		}
		b.WriteString("\n")
	}

	return b.String()
}

func generateAllGroupVars(cfg *Config, controlplaneIP, storageNodeIP, cephFSID, cephWebKey string) string {
	var b strings.Builder

	b.WriteString("# Generated by hosting setup wizard\n\n")

	// SSH user
	sshUser := cfg.SSHUser
	if sshUser == "" {
		sshUser = "ubuntu"
	}
	b.WriteString(fmt.Sprintf("ansible_user: \"%s\"\n\n", sshUser))

	b.WriteString(fmt.Sprintf("loki_endpoint: \"http://%s:3100\"\n", controlplaneIP))
	b.WriteString(fmt.Sprintf("loki_tenant_endpoint: \"http://%s:3101\"\n", controlplaneIP))
	b.WriteString("node_agent_binary_path: \"{{ playbook_dir }}/../bin/node-agent\"\n")
	b.WriteString("dbadmin_proxy_binary_path: \"{{ playbook_dir }}/../bin/dbadmin-proxy\"\n")

	// SSH CA public key placeholder
	b.WriteString("ssh_ca_public_key: \"{{ lookup('file', playbook_dir + '/../ssh_ca.pub') }}\"\n\n")

	b.WriteString("# Node identity\n")
	b.WriteString(fmt.Sprintf("temporal_address: \"%s:7233\"\n", controlplaneIP))
	b.WriteString(fmt.Sprintf("region_id: \"%s\"\n", cfg.RegionName))
	b.WriteString(fmt.Sprintf("cluster_id: \"%s\"\n", cfg.ClusterName))
	b.WriteString(fmt.Sprintf("controlplane_ip: \"%s\"\n", controlplaneIP))
	b.WriteString(fmt.Sprintf("core_api_url: \"http://%s:8090/api/v1\"\n", controlplaneIP))
	b.WriteString(fmt.Sprintf("core_api_token: \"{{ lookup('env', 'CORE_API_TOKEN') | default('%s', true) }}\"\n", cfg.APIKey))
	b.WriteString("node_id: \"{{ inventory_hostname }}\"\n\n")

	b.WriteString("# Ceph\n")
	b.WriteString(fmt.Sprintf("ceph_fsid: \"%s\"\n", cephFSID))
	b.WriteString(fmt.Sprintf("ceph_web_key: \"%s\"\n", cephWebKey))
	b.WriteString("ceph_web_keyring_file: \"{{ inventory_dir }}/../../ceph.client.web.keyring\"\n")
	b.WriteString(fmt.Sprintf("storage_node_ip: \"%s\"\n\n", storageNodeIP))

	b.WriteString("# Stalwart\n")
	b.WriteString(fmt.Sprintf("stalwart_hostname: \"%s\"\n", cfg.Brand.MailHostname))
	b.WriteString(fmt.Sprintf("stalwart_admin_password: \"%s\"\n", cfg.Email.StalwartAdminToken))

	return b.String()
}

func generateRoleGroupVars(cfg *Config, controlplaneIP string) []GeneratedFile {
	const base = "generated/ansible/inventory/group_vars"
	var files []GeneratedFile

	// web.yml
	files = append(files, GeneratedFile{
		Path: base + "/web.yml",
		Content: `node_role: web
shard_name: web-1

node_agent_nginx_config_dir: /etc/nginx
node_agent_web_storage_dir: /var/www/storage
node_agent_cert_dir: /etc/ssl/hosting
node_agent_ssh_config_dir: /etc/ssh/sshd_config.d

php_versions:
  - "8.3"
  - "8.5"

php_extensions:
  - fpm
  - cli
  - mysql
  - curl
  - mbstring
  - xml
  - zip
`,
	})

	// db.yml
	files = append(files, GeneratedFile{
		Path: base + "/db.yml",
		Content: `node_role: database
shard_name: db-1

node_agent_mysql_dsn: "root@tcp(127.0.0.1:3306)/"

mysql_repl_password: "repl_pass"
`,
	})

	// dns.yml
	files = append(files, GeneratedFile{
		Path: base + "/dns.yml",
		Content: fmt.Sprintf(`node_role: dns
shard_name: dns-1

powerdns_db_host: "%s"
powerdns_db_port: "5433"
powerdns_db_name: "hosting_powerdns"
powerdns_db_user: "hosting"
powerdns_db_password: "hosting"
`, controlplaneIP),
	})

	// valkey.yml
	files = append(files, GeneratedFile{
		Path: base + "/valkey.yml",
		Content: `node_role: valkey
shard_name: valkey-1

node_agent_valkey_config_dir: /etc/valkey
node_agent_valkey_data_dir: /var/lib/valkey
`,
	})

	// email.yml
	files = append(files, GeneratedFile{
		Path: base + "/email.yml",
		Content: fmt.Sprintf(`node_role: email
shard_name: email-1

stalwart_hostname: "%s"
stalwart_admin_password: "%s"
`, cfg.Brand.MailHostname, cfg.Email.StalwartAdminToken),
	})

	// storage.yml
	files = append(files, GeneratedFile{
		Path: base + "/storage.yml",
		Content: `node_role: storage
shard_name: storage-1

ceph_s3_enabled: true
ceph_filestore_enabled: true

node_agent_rgw_endpoint: "http://localhost:7480"
`,
	})

	// lb.yml
	files = append(files, GeneratedFile{
		Path: base + "/lb.yml",
		Content: `node_role: lb
shard_name: lb-1
`,
	})

	// gateway.yml
	files = append(files, GeneratedFile{
		Path: base + "/gateway.yml",
		Content: `node_role: gateway
shard_name: gateway-1
`,
	})

	// dbadmin.yml
	files = append(files, GeneratedFile{
		Path: base + "/dbadmin.yml",
		Content: `node_role: dbadmin
shard_name: dbadmin-1
`,
	})

	return files
}

func generateClusterYAML(cfg *Config, controlplaneIP string) string {
	var b strings.Builder

	apiURL := fmt.Sprintf("http://%s:8090/api/v1", controlplaneIP)

	b.WriteString("# Generated by hosting setup wizard\n")
	b.WriteString(fmt.Sprintf("api_url: %s\n", apiURL))
	b.WriteString(fmt.Sprintf("api_key: %s\n\n", cfg.APIKey))

	b.WriteString("region:\n")
	b.WriteString(fmt.Sprintf("  name: %s\n\n", cfg.RegionName))

	b.WriteString("cluster:\n")
	b.WriteString(fmt.Sprintf("  name: %s\n", cfg.ClusterName))

	// LB addresses
	if cfg.DeployMode == DeployModeSingle {
		ip := cfg.singleNodeIP()
		b.WriteString("  lb_addresses:\n")
		b.WriteString(fmt.Sprintf("    - address: \"%s\"\n", ip))
		b.WriteString("      label: default\n")
	} else {
		// Find LB node IPs
		var lbIPs []string
		for _, n := range cfg.Nodes {
			for _, r := range n.Roles {
				if r == RoleLB {
					lbIPs = append(lbIPs, n.IP)
				}
			}
		}
		if len(lbIPs) > 0 {
			b.WriteString("  lb_addresses:\n")
			for _, ip := range lbIPs {
				b.WriteString(fmt.Sprintf("    - address: \"%s\"\n", ip))
				b.WriteString("      label: default\n")
			}
		}
	}

	// Config section (email/Stalwart)
	emailNodeIP := cfg.singleNodeIP()
	if cfg.DeployMode == DeployModeMulti {
		for _, n := range cfg.Nodes {
			for _, r := range n.Roles {
				if r == RoleEmail {
					emailNodeIP = n.IP
					break
				}
			}
		}
	}
	b.WriteString("  config:\n")
	b.WriteString(fmt.Sprintf("    stalwart_url: \"http://%s:8080\"\n", emailNodeIP))
	b.WriteString(fmt.Sprintf("    stalwart_token: \"%s\"\n", cfg.Email.StalwartAdminToken))
	b.WriteString(fmt.Sprintf("    mail_hostname: \"%s\"\n", cfg.Brand.MailHostname))

	// Shards — determine from node role assignments
	b.WriteString("  spec:\n")
	b.WriteString("    shards:\n")

	type shardInfo struct {
		name    string
		role    string
		backend string
		count   int
	}

	var shards []shardInfo
	if cfg.DeployMode == DeployModeSingle {
		shards = []shardInfo{
			{"web-1", "web", "web-1", 1},
			{"db-1", "database", "", 1},
			{"dns-1", "dns", "", 1},
			{"valkey-1", "valkey", "", 1},
			{"email-1", "email", "", 1},
			{"storage-1", "storage", "", 1},
			{"lb-1", "lb", "", 1},
			{"gateway-1", "gateway", "", 1},
			{"dbadmin-1", "dbadmin", "", 1},
		}
	} else {
		// Count nodes per role
		roleCounts := map[NodeRole]int{}
		for _, n := range cfg.Nodes {
			for _, r := range n.Roles {
				roleCounts[r]++
			}
		}
		roleToShard := map[NodeRole]struct{ name, role, backend string }{
			RoleWeb:     {"web-1", "web", "web-1"},
			RoleDatabase: {"db-1", "database", ""},
			RoleDNS:     {"dns-1", "dns", ""},
			RoleValkey:  {"valkey-1", "valkey", ""},
			RoleEmail:   {"email-1", "email", ""},
			RoleStorage: {"storage-1", "storage", ""},
			RoleLB:      {"lb-1", "lb", ""},
			RoleGateway: {"gateway-1", "gateway", ""},
			RoleDBAdmin: {"dbadmin-1", "dbadmin", ""},
		}
		for _, role := range AllRoles {
			if role == RoleControlPlane {
				continue
			}
			if info, ok := roleToShard[role]; ok {
				count := roleCounts[role]
				if count > 0 {
					shards = append(shards, shardInfo{info.name, info.role, info.backend, count})
				}
			}
		}
	}

	for _, s := range shards {
		b.WriteString(fmt.Sprintf("      - name: %s\n", s.name))
		b.WriteString(fmt.Sprintf("        role: %s\n", s.role))
		if s.backend != "" {
			b.WriteString(fmt.Sprintf("        lb_backend: %s\n", s.backend))
		}
		b.WriteString(fmt.Sprintf("        node_count: %d\n", s.count))
	}

	// Infrastructure flags
	b.WriteString("    infrastructure:\n")
	b.WriteString("      haproxy: true\n")
	b.WriteString("      powerdns: true\n")
	b.WriteString("      valkey: true\n")

	// Nodes
	b.WriteString("  nodes:\n")
	if cfg.DeployMode == DeployModeSingle {
		ip := cfg.singleNodeIP()
		nodeID := "localhost"
		if ip != "127.0.0.1" && ip != "localhost" {
			nodeID = ip
		}
		b.WriteString(fmt.Sprintf("    - id: %s\n", nodeID))
		b.WriteString(fmt.Sprintf("      hostname: %s\n", nodeID))
		b.WriteString(fmt.Sprintf("      ip_address: \"%s\"\n", ip))
		b.WriteString("      shard_names:\n")
		for _, s := range shards {
			b.WriteString(fmt.Sprintf("        - %s\n", s.name))
		}
	} else {
		for _, n := range cfg.Nodes {
			if hasRole(n.Roles, RoleControlPlane) && !hasAnyServiceRole(n.Roles) {
				continue // Control-plane-only node, no shards
			}
			b.WriteString(fmt.Sprintf("    - id: %s\n", n.Hostname))
			b.WriteString(fmt.Sprintf("      hostname: %s\n", n.Hostname))
			b.WriteString(fmt.Sprintf("      ip_address: \"%s\"\n", n.IP))
			shardNames := nodeShardNames(n.Roles)
			if len(shardNames) > 0 {
				b.WriteString("      shard_names:\n")
				for _, s := range shardNames {
					b.WriteString(fmt.Sprintf("        - %s\n", s))
				}
			}
		}
	}

	// Cluster runtimes
	b.WriteString("\ncluster_runtimes:\n")
	b.WriteString("  - runtime: php\n")
	b.WriteString("    version: \"8.5\"\n")
	b.WriteString("  - runtime: php\n")
	b.WriteString("    version: \"8.3\"\n")
	b.WriteString("  - runtime: node\n")
	b.WriteString("    version: \"22\"\n")
	b.WriteString("  - runtime: python\n")
	b.WriteString("    version: \"3.12\"\n")
	b.WriteString("  - runtime: ruby\n")
	b.WriteString("    version: \"3.3\"\n")
	b.WriteString("  - runtime: static\n")
	b.WriteString("    version: \"1\"\n")

	return b.String()
}

func generateSeedYAML(cfg *Config) string {
	var b strings.Builder

	controlplaneIP := cfg.singleNodeIP()
	if cfg.DeployMode == DeployModeMulti {
		for _, n := range cfg.Nodes {
			for _, r := range n.Roles {
				if r == RoleControlPlane {
					controlplaneIP = n.IP
					break
				}
			}
		}
	}

	apiURL := fmt.Sprintf("http://%s:8090/api/v1", controlplaneIP)

	b.WriteString("# Generated by hosting setup wizard\n")
	b.WriteString(fmt.Sprintf("api_url: %s\n", apiURL))
	b.WriteString(fmt.Sprintf("api_key: %s\n\n", cfg.APIKey))
	b.WriteString(fmt.Sprintf("region: %s\n", cfg.RegionName))
	b.WriteString(fmt.Sprintf("cluster: %s\n\n", cfg.ClusterName))

	b.WriteString("brands:\n")
	b.WriteString(fmt.Sprintf("  - name: %s\n", cfg.Brand.Name))
	b.WriteString(fmt.Sprintf("    base_hostname: %s\n", cfg.Brand.CustomerDomain))
	b.WriteString(fmt.Sprintf("    primary_ns: %s\n", cfg.Brand.PrimaryNS))
	b.WriteString(fmt.Sprintf("    secondary_ns: %s\n", cfg.Brand.SecondaryNS))
	b.WriteString(fmt.Sprintf("    hostmaster_email: %s\n", cfg.Brand.HostmasterEmail))
	b.WriteString(fmt.Sprintf("    mail_hostname: %s\n", cfg.Brand.MailHostname))
	b.WriteString(fmt.Sprintf("    allowed_clusters:\n"))
	b.WriteString(fmt.Sprintf("      - %s\n", cfg.ClusterName))

	return b.String()
}

func hasRole(roles []NodeRole, target NodeRole) bool {
	for _, r := range roles {
		if r == target {
			return true
		}
	}
	return false
}

func hasAnyServiceRole(roles []NodeRole) bool {
	for _, r := range roles {
		if r != RoleControlPlane {
			return true
		}
	}
	return false
}

func nodeShardNames(roles []NodeRole) []string {
	roleToShard := map[NodeRole]string{
		RoleWeb:      "web-1",
		RoleDatabase: "db-1",
		RoleDNS:      "dns-1",
		RoleValkey:   "valkey-1",
		RoleEmail:    "email-1",
		RoleStorage:  "storage-1",
		RoleLB:       "lb-1",
		RoleGateway:  "gateway-1",
		RoleDBAdmin:  "dbadmin-1",
	}
	var names []string
	for _, r := range roles {
		if s, ok := roleToShard[r]; ok {
			names = append(names, s)
		}
	}
	return names
}

// generateCephKeyring uses docker + ceph-authtool to produce a properly
// formatted keyring file with a Ceph-native key (which includes a type/timestamp
// header, unlike raw base64). Returns the keyring content and the key string.
func generateCephKeyring() (keyring string, key string, err error) {
	cmd := exec.Command("docker", "run", "--rm", "quay.io/ceph/ceph:v19",
		"ceph-authtool", "--create-keyring", "/dev/stdout",
		"-n", "client.web", "--gen-key",
		"--cap", "mon", "allow r",
		"--cap", "osd", "allow rw pool=cephfs_data",
		"--cap", "mds", "allow rw",
	)
	out, err := cmd.Output()
	if err != nil {
		return "", "", fmt.Errorf("ceph-authtool via docker: %w", err)
	}

	keyring = string(out)

	// Extract key from output: line like "\tkey = AQ..."
	for _, line := range strings.Split(keyring, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "key = ") {
			key = strings.TrimPrefix(line, "key = ")
			break
		}
	}
	if key == "" {
		return "", "", fmt.Errorf("could not extract key from ceph-authtool output")
	}

	return keyring, key, nil
}

