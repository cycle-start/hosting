package setup

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

// opsGitignore is the default .gitignore for the operations root.
// It excludes secrets and ephemeral artifacts while allowing
// configuration and generated infrastructure files to be tracked.
const opsGitignore = `# Secrets — NEVER commit
.env
.env.local
ssh_ca
ssh_ca.pub

# Kubeconfig contains cluster credentials
generated/kubeconfig.yaml

# Ceph keyring contains storage credentials
generated/ceph.client.web.keyring

# Compiled binaries (downloaded or built from release)
bin/

# IDE / OS
.idea/
.vscode/
*.swp
*.swo
.DS_Store
Thumbs.db

# Node modules (if running setup wizard from source)
node_modules/
dist/

# Build artifacts
*.exe
coverage.out
*.prof
`

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

	// 7. Fetch kubeconfig from the controlplane node
	progress("Fetching kubeconfig from controlplane...")
	kubeconfig, err := fetchKubeconfig(cfg, controlplaneIP)
	if err != nil {
		progress(fmt.Sprintf("Warning: could not fetch kubeconfig: %v", err))
	} else {
		result.Files = append(result.Files, GeneratedFile{
			Path:    "generated/kubeconfig.yaml",
			Content: kubeconfig,
		})
	}

	// 8. Write .gitignore if one doesn't exist
	gitignorePath := filepath.Join(outputDir, ".gitignore")
	if _, err := os.Stat(gitignorePath); os.IsNotExist(err) {
		progress("Writing .gitignore...")
		if err := os.WriteFile(gitignorePath, []byte(opsGitignore), 0o644); err != nil {
			return nil, fmt.Errorf("write .gitignore: %w", err)
		}
		result.Files = append(result.Files, GeneratedFile{
			Path:    ".gitignore",
			Content: opsGitignore,
		})
	}

	// 9. Write .env file at project root (merge with existing)
	progress("Updating .env file...")
	envPath := filepath.Join(outputDir, ".env")
	dotEnv, err := generateDotEnv(cfg, envPath)
	if err != nil {
		return nil, fmt.Errorf("generate .env: %w", err)
	}
	if err := os.WriteFile(envPath, []byte(dotEnv), 0o600); err != nil {
		return nil, fmt.Errorf("write .env: %w", err)
	}
	result.Files = append(result.Files, GeneratedFile{
		Path:    ".env",
		Content: dotEnv,
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

// generateDotEnv builds a .env file from config, merging with any existing
// .env at envPath. Config-derived values take precedence; unknown keys from
// the existing file are preserved at the end.
func generateDotEnv(cfg *Config, envPath string) (string, error) {
	// Read existing .env to preserve manually-set vars
	existing := parseEnvFile(envPath)

	// Build the set of config-derived vars
	vars := map[string]string{
		"BASE_DOMAIN":        cfg.Brand.PlatformDomain,
		"HOSTING_API_KEY":    cfg.APIKey,
		"STALWART_ADMIN_TOKEN": cfg.Email.StalwartAdminToken,
	}

	// Generate SECRET_ENCRYPTION_KEY if not already set
	if existing["SECRET_ENCRYPTION_KEY"] != "" {
		vars["SECRET_ENCRYPTION_KEY"] = existing["SECRET_ENCRYPTION_KEY"]
	} else {
		key := make([]byte, 32)
		rand.Read(key)
		vars["SECRET_ENCRYPTION_KEY"] = fmt.Sprintf("%x", key)
	}

	// Database URLs
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

	if cfg.ControlPlane.Database.Mode == "external" {
		db := cfg.ControlPlane.Database
		dsn := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s",
			db.User, db.Password, db.Host, db.Port, db.Name, db.SSLMode)
		vars["CORE_DATABASE_URL"] = dsn
	} else {
		vars["CORE_DATABASE_URL"] = fmt.Sprintf("postgres://hosting:hosting@%s:5432/hosting_core", controlplaneIP)
	}
	vars["POWERDNS_DATABASE_URL"] = fmt.Sprintf("postgres://hosting:hosting@%s:5433/hosting_powerdns", controlplaneIP)

	// SSO (Azure AD)
	if cfg.SSO.Enabled && cfg.SSO.ClientID != "" {
		vars["OIDC_TENANT_ID"] = cfg.SSO.TenantID
		vars["OIDC_CLIENT_ID"] = cfg.SSO.ClientID
		vars["OIDC_CLIENT_SECRET"] = cfg.SSO.ClientSecret

		// Preserve existing cookie secret or generate a new one
		if existing["OAUTH2_PROXY_COOKIE_SECRET"] != "" {
			vars["OAUTH2_PROXY_COOKIE_SECRET"] = existing["OAUTH2_PROXY_COOKIE_SECRET"]
		} else {
			cookieSecret := make([]byte, 32)
			rand.Read(cookieSecret)
			vars["OAUTH2_PROXY_COOKIE_SECRET"] = base64.StdEncoding.EncodeToString(cookieSecret)
		}
	}

	// Write the .env in a readable grouped format
	var b strings.Builder

	section := func(comment string, keys ...string) {
		b.WriteString(fmt.Sprintf("# %s\n", comment))
		for _, k := range keys {
			if v, ok := vars[k]; ok {
				b.WriteString(fmt.Sprintf("%s=%s\n", k, v))
				delete(existing, k) // consumed
			}
		}
		b.WriteString("\n")
	}

	section("Base domain for control plane hostnames",
		"BASE_DOMAIN")
	section("API key for Ansible dynamic inventory and admin tooling",
		"HOSTING_API_KEY")
	section("Core DB (Postgres)",
		"CORE_DATABASE_URL")
	section("PowerDNS DB (Postgres)",
		"POWERDNS_DATABASE_URL")
	section("Stalwart mail server admin token",
		"STALWART_ADMIN_TOKEN")
	section("AES-256 key for encrypting secrets in core DB (hex-encoded, 32 bytes)",
		"SECRET_ENCRYPTION_KEY")

	if cfg.SSO.Enabled && cfg.SSO.ClientID != "" {
		section("SSO (Azure AD / Entra ID)",
			"OIDC_TENANT_ID", "OIDC_CLIENT_ID", "OIDC_CLIENT_SECRET", "OAUTH2_PROXY_COOKIE_SECRET")
	}

	// Preserve any remaining keys from the existing .env that we didn't set
	if len(existing) > 0 {
		b.WriteString("# Additional settings (preserved from existing .env)\n")
		for k, v := range existing {
			b.WriteString(fmt.Sprintf("%s=%s\n", k, v))
		}
		b.WriteString("\n")
	}

	return b.String(), nil
}

// parseEnvFile reads a .env file and returns a map of key→value.
// Returns an empty map if the file doesn't exist.
func parseEnvFile(path string) map[string]string {
	m := make(map[string]string)
	data, err := os.ReadFile(path)
	if err != nil {
		return m
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if k, v, ok := strings.Cut(line, "="); ok {
			m[strings.TrimSpace(k)] = strings.TrimSpace(v)
		}
	}
	return m
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

	// Base domain for hostname-based routing (HAProxy ACLs, etc.)
	if cfg.Brand.PlatformDomain != "" {
		b.WriteString(fmt.Sprintf("\nbase_domain: \"%s\"\n", cfg.Brand.PlatformDomain))
	}

	return b.String()
}

func generateRoleGroupVars(cfg *Config, controlplaneIP string) []GeneratedFile {
	const base = "generated/ansible/inventory/group_vars"
	var files []GeneratedFile

	// In single-node mode, the web and lb roles share a machine so nginx must
	// listen on a non-standard port to avoid conflicting with HAProxy on 80/443.
	isSingleNode := cfg.DeployMode == DeployModeSingle

	var webBuf strings.Builder
	webBuf.WriteString("node_role: web\n")
	webBuf.WriteString("shard_name: web-1\n\n")
	webBuf.WriteString("node_agent_nginx_config_dir: /etc/nginx\n")
	webBuf.WriteString("node_agent_web_storage_dir: /var/www/storage\n")
	webBuf.WriteString("node_agent_cert_dir: /etc/ssl/hosting\n")
	webBuf.WriteString("node_agent_ssh_config_dir: /etc/ssh/sshd_config.d\n\n")
	webBuf.WriteString("php_versions:\n")
	for _, v := range cfg.PHPVersions {
		webBuf.WriteString(fmt.Sprintf("  - \"%s\"\n", v))
	}
	webBuf.WriteString("\nphp_extensions:\n")
	webBuf.WriteString("  - fpm\n")
	webBuf.WriteString("  - cli\n")
	webBuf.WriteString("  - mysql\n")
	webBuf.WriteString("  - curl\n")
	webBuf.WriteString("  - mbstring\n")
	webBuf.WriteString("  - xml\n")
	webBuf.WriteString("  - zip\n")
	webYml := webBuf.String()
	if isSingleNode {
		webYml += "\n# Single-node: nginx listens on 8080 to avoid conflicting with HAProxy on 80\n"
		webYml += "nginx_listen_port: \"8080\"\n"
	}

	// controlplane.yml
	if isSingleNode {
		files = append(files, GeneratedFile{
			Path: base + "/controlplane.yml",
			Content: `# Single-node: disable Traefik since HAProxy handles all ingress
k3s_disable_traefik: true
`,
		})
	}

	// web.yml
	files = append(files, GeneratedFile{
		Path:    base + "/web.yml",
		Content: webYml,
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
	lbYml := `node_role: lb
shard_name: lb-1
`
	if isSingleNode {
		lbYml += "\n# Single-node: web nginx listens on 8080, HAProxy backends must match\n"
		lbYml += "web_backend_port: \"8080\"\n"
	}
	if cfg.SSO.Enabled && cfg.SSO.ClientID != "" {
		lbYml += "\n# SSO: route prometheus traffic through oauth2-proxy\n"
		lbYml += "prometheus_backend_port: \"4180\"\n"
	}
	files = append(files, GeneratedFile{
		Path:    base + "/lb.yml",
		Content: lbYml,
	})

	// gateway.yml
	files = append(files, GeneratedFile{
		Path: base + "/gateway.yml",
		Content: `node_role: gateway
shard_name: gateway-1
`,
	})

	// dbadmin.yml
	dbadminYml := `node_role: dbadmin
shard_name: dbadmin-1
`
	if isSingleNode {
		dbadminYml += "\n# Single-node: dbadmin uses HTTPS-only on 8443 to avoid conflicting with HAProxy\n"
		dbadminYml += "dbadmin_listen_port: \"8443\"\n"
	}
	files = append(files, GeneratedFile{
		Path:    base + "/dbadmin.yml",
		Content: dbadminYml,
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
	for _, v := range cfg.PHPVersions {
		b.WriteString(fmt.Sprintf("  - runtime: php\n    version: \"%s\"\n", v))
	}
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
		"sh", "-c",
		"ceph-authtool --create-keyring /tmp/kr -n client.web --gen-key "+
			"--cap mon 'allow r' --cap osd 'allow rw pool=cephfs_data' --cap mds 'allow rw' >&2 && cat /tmp/kr",
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

// fetchKubeconfig fetches the k3s kubeconfig from the controlplane node via SSH
// and rewrites the server URL to use the controlplane IP instead of 127.0.0.1.
func fetchKubeconfig(cfg *Config, controlplaneIP string) (string, error) {
	isLocal := controlplaneIP == "127.0.0.1" || controlplaneIP == "localhost"

	var out []byte
	var err error
	if isLocal {
		out, err = exec.Command("sudo", "cat", "/etc/rancher/k3s/k3s.yaml").Output()
	} else {
		sshUser := cfg.SSHUser
		if sshUser == "" {
			sshUser = "ubuntu"
		}
		out, err = exec.Command("ssh",
			"-o", "StrictHostKeyChecking=no",
			"-o", "UserKnownHostsFile=/dev/null",
			"-o", "LogLevel=ERROR",
			fmt.Sprintf("%s@%s", sshUser, controlplaneIP),
			"sudo cat /etc/rancher/k3s/k3s.yaml",
		).Output()
	}
	if err != nil {
		return "", fmt.Errorf("fetch k3s kubeconfig: %w", err)
	}

	kubeconfig := string(out)
	if !isLocal {
		kubeconfig = strings.ReplaceAll(kubeconfig,
			"https://127.0.0.1:6443",
			fmt.Sprintf("https://%s:6443", controlplaneIP),
		)
	}

	return kubeconfig, nil
}

