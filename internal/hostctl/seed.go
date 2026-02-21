package hostctl

import (
	"crypto/rand"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// tenantSeedCtx tracks created resources per tenant for fixture template resolution.
type tenantSeedCtx struct {
	dbHost     string
	dbName     string
	dbID       string
	dbUsername string
	dbPassword string
	fqdn       string
	webrootID  string
}

func Seed(configPath string, timeout time.Duration) error {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("read config: %w", err)
	}

	var cfg SeedConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("parse config: %w", err)
	}

	apiKey := cfg.APIKey
	if apiKey == "" {
		apiKey = os.Getenv("HOSTING_API_KEY")
	}
	if apiKey == "" {
		return fmt.Errorf("no API key: set api_key in config or HOSTING_API_KEY env var")
	}
	client := NewClient(cfg.APIURL, apiKey)

	// Resolve references
	regionID, err := client.FindRegionByName(cfg.Region)
	if err != nil {
		return fmt.Errorf("resolve region %q: %w", cfg.Region, err)
	}
	fmt.Printf("Region %q: %s\n", cfg.Region, regionID)

	clusterID, err := client.FindClusterByName(regionID, cfg.Cluster)
	if err != nil {
		return fmt.Errorf("resolve cluster %q: %w", cfg.Cluster, err)
	}
	fmt.Printf("Cluster %q: %s\n", cfg.Cluster, clusterID)

	// In-memory maps for tracking created resources
	tenantMap := map[string]string{} // tenant name -> ID
	shardMap := map[string]string{}  // shard name -> ID
	brandMap := map[string]string{}  // brand name -> ID

	// Resolve shard names to IDs (needed for tenant creation)
	resolveShardID := func(shardName string) (string, error) {
		if id, ok := shardMap[shardName]; ok {
			return id, nil
		}
		id, err := client.FindShardByName(clusterID, shardName)
		if err != nil {
			return "", fmt.Errorf("resolve shard %q: %w", shardName, err)
		}
		shardMap[shardName] = id
		return id, nil
	}

	// 1. Create brands and set allowed clusters
	for _, b := range cfg.Brands {
		// Check if brand already exists by name.
		brandID, err := client.FindBrandByName(b.Name)
		if err == nil {
			fmt.Printf("Brand %q: exists (%s, skipping)\n", b.Name, brandID)
			brandMap[b.Name] = brandID
		} else {
			fmt.Printf("Creating brand %q...\n", b.Name)
			body := map[string]any{
				"name":             b.Name,
				"base_hostname":    b.BaseHostname,
				"primary_ns":       b.PrimaryNS,
				"secondary_ns":     b.SecondaryNS,
				"hostmaster_email": b.HostmasterEmail,
				"mail_hostname":    b.MailHostname,
				"spf_includes":     b.SPFIncludes,
				"dkim_selector":    b.DKIMSelector,
				"dkim_public_key":  b.DKIMPublicKey,
				"dmarc_policy":     b.DMARCPolicy,
			}
			if b.ID != "" {
				body["id"] = b.ID
			}
			resp, err := client.Post("/brands", body)
			if err != nil {
				return fmt.Errorf("create brand %q: %w", b.Name, err)
			}
			brandID, err = extractID(resp)
			if err != nil {
				return fmt.Errorf("parse brand ID: %w", err)
			}
			brandMap[b.Name] = brandID
			fmt.Printf("  Brand %q: %s created\n", b.Name, brandID)
		}

		// Set allowed clusters (resolve names to IDs).
		if len(b.AllowedClusters) > 0 {
			var clusterIDs []string
			for _, cName := range b.AllowedClusters {
				cID, err := client.FindClusterByName(regionID, cName)
				if err != nil {
					return fmt.Errorf("resolve cluster %q for brand %q: %w", cName, b.Name, err)
				}
				clusterIDs = append(clusterIDs, cID)
			}
			_, err := client.Put(fmt.Sprintf("/brands/%s/clusters", brandID), map[string]any{
				"cluster_ids": clusterIDs,
			})
			if err != nil {
				return fmt.Errorf("set brand %q clusters: %w", b.Name, err)
			}
			fmt.Printf("  Brand %q: allowed clusters set (%v)\n", b.Name, b.AllowedClusters)
		}
	}

	// Discover web node IPs once (needed for fixture deployment).
	var webNodeIPs []string
	needsFixture := false
	for _, t := range cfg.Tenants {
		for _, w := range t.Webroots {
			if w.Fixture != nil {
				needsFixture = true
				break
			}
		}
		if needsFixture {
			break
		}
	}
	if needsFixture {
		webNodeIPs, err = findNodeIPsByRole(client, clusterID, "web")
		if err != nil {
			return fmt.Errorf("find web node IPs: %w", err)
		}
		fmt.Printf("Web nodes: %v\n", webNodeIPs)
	}

	// 3. Create tenants with all nested resources in one request
	for _, t := range cfg.Tenants {
		fmt.Printf("Creating tenant %q...\n", t.Name)

		webShardID, err := resolveShardID(t.Shard)
		if err != nil {
			return err
		}

		// Build email account lookup: fqdn string -> []EmailAcctDef
		emailsByFQDN := map[string][]EmailAcctDef{}
		for _, e := range t.EmailAccounts {
			emailsByFQDN[e.FQDN] = append(emailsByFQDN[e.FQDN], e)
		}

		// Build subscriptions: generate IDs and track name -> ID map
		subMap := map[string]string{} // subscription name -> generated ID
		var subscriptions []map[string]any
		// Auto-create a "default" subscription if none specified
		if len(t.Subscriptions) == 0 {
			t.Subscriptions = []SubscriptionDef{{Name: "default"}}
		}
		for _, s := range t.Subscriptions {
			id := s.ID
			if id == "" {
				id = generateUUID()
			}
			subMap[s.Name] = id
			subscriptions = append(subscriptions, map[string]any{
				"id":   id,
				"name": s.Name,
			})
		}

		// resolveSubID finds the subscription ID for a resource's subscription name.
		// Falls back to the first subscription if not specified.
		resolveSubID := func(subName string) string {
			if subName != "" {
				if id, ok := subMap[subName]; ok {
					return id
				}
			}
			// Default: use first subscription
			return subMap[t.Subscriptions[0].Name]
		}

		// Resolve brand
		tenantBody := map[string]any{
			"region_id":     regionID,
			"cluster_id":    clusterID,
			"shard_id":      webShardID,
			"sftp_enabled":  t.SFTPEnabled,
			"ssh_enabled":   t.SSHEnabled,
			"subscriptions": subscriptions,
		}
		if t.Brand != "" {
			brandID, ok := brandMap[t.Brand]
			if !ok {
				return fmt.Errorf("tenant %q: brand %q not found (must be defined in brands section)", t.Name, t.Brand)
			}
			tenantBody["brand_id"] = brandID
		}
		if t.CustomerID == "" {
			return fmt.Errorf("tenant %q: customer_id is required", t.Name)
		}
		tenantBody["customer_id"] = t.CustomerID

		// Nested SSH keys
		if len(t.SSHKeys) > 0 {
			var keys []map[string]any
			for _, sk := range t.SSHKeys {
				publicKey := sk.PublicKey
				if publicKey == "${SSH_PUBLIC_KEY}" {
					publicKey, err = sshPublicKeyContent()
					if err != nil {
						return fmt.Errorf("resolve SSH public key: %w", err)
					}
				}
				keys = append(keys, map[string]any{
					"name":       sk.Name,
					"public_key": publicKey,
				})
			}
			tenantBody["ssh_keys"] = keys
		}

		// Nested egress rules
		if len(t.EgressRules) > 0 {
			var rules []map[string]any
			for _, r := range t.EgressRules {
				rules = append(rules, map[string]any{
					"cidr":        r.CIDR,
					"description": r.Description,
				})
			}
			tenantBody["egress_rules"] = rules
		}

		// Nested webroots (with FQDNs + email accounts, daemons, cron jobs)
		if len(t.Webroots) > 0 {
			var webroots []map[string]any
			for _, w := range t.Webroots {
				wr := map[string]any{
					"subscription_id": resolveSubID(w.Subscription),
					"runtime":         w.Runtime,
					"runtime_version": w.RuntimeVersion,
				}
				if w.PublicFolder != "" {
					wr["public_folder"] = w.PublicFolder
				}
				if w.RuntimeConfig != nil {
					wr["runtime_config"] = w.RuntimeConfig
				}
				if w.EnvFileName != "" {
					wr["env_file_name"] = w.EnvFileName
				}
				if w.EnvShellSource != nil {
					wr["env_shell_source"] = *w.EnvShellSource
				}

				// FQDNs with nested email accounts
				if len(w.FQDNs) > 0 {
					var fqdns []map[string]any
					for _, f := range w.FQDNs {
						fqdnEntry := map[string]any{
							"fqdn":        f.FQDN,
							"ssl_enabled": f.SSLEnabled,
						}
						if emails, ok := emailsByFQDN[f.FQDN]; ok {
							fqdnEntry["email_accounts"] = buildEmailAccountEntries(emails, resolveSubID)
						}
						fqdns = append(fqdns, fqdnEntry)
					}
					wr["fqdns"] = fqdns
				}

				// Daemons
				if len(w.Daemons) > 0 {
					var daemons []map[string]any
					for _, d := range w.Daemons {
						entry := map[string]any{"command": d.Command}
						if d.ProxyPath != "" {
							entry["proxy_path"] = d.ProxyPath
						}
						if d.NumProcs > 0 {
							entry["num_procs"] = d.NumProcs
						}
						daemons = append(daemons, entry)
					}
					wr["daemons"] = daemons
				}

				// Cron jobs
				if len(w.CronJobs) > 0 {
					var jobs []map[string]any
					for _, j := range w.CronJobs {
						entry := map[string]any{
							"schedule": j.Schedule,
							"command":  j.Command,
						}
						if j.WorkingDirectory != "" {
							entry["working_directory"] = j.WorkingDirectory
						}
						jobs = append(jobs, entry)
					}
					wr["cron_jobs"] = jobs
				}

				webroots = append(webroots, wr)
			}
			tenantBody["webroots"] = webroots
		}

		// Nested databases (without users — usernames need auto-generated DB names)
		if len(t.Databases) > 0 {
			var dbs []map[string]any
			for _, d := range t.Databases {
				dbShardID, err := resolveShardID(d.Shard)
				if err != nil {
					return err
				}
				dbEntry := map[string]any{
					"subscription_id": resolveSubID(d.Subscription),
					"shard_id":        dbShardID,
				}
				if len(d.AccessRules) > 0 {
					var rules []map[string]any
					for _, r := range d.AccessRules {
						rules = append(rules, map[string]any{
							"cidr":        r.CIDR,
							"description": r.Description,
						})
					}
					dbEntry["access_rules"] = rules
				}
				dbs = append(dbs, dbEntry)
			}
			tenantBody["databases"] = dbs
		}

		// Nested valkey instances (with users)
		if len(t.ValkeyInstances) > 0 {
			var instances []map[string]any
			for _, v := range t.ValkeyInstances {
				vkShardID, err := resolveShardID(v.Shard)
				if err != nil {
					return err
				}
				entry := map[string]any{
					"subscription_id": resolveSubID(v.Subscription),
					"shard_id":        vkShardID,
				}
				if v.MaxMemoryMB > 0 {
					entry["max_memory_mb"] = v.MaxMemoryMB
				}
				if len(v.Users) > 0 {
					var users []map[string]any
					for _, u := range v.Users {
						userEntry := map[string]any{
							"username":   u.Username,
							"password":   u.Password,
							"privileges": u.Privileges,
						}
						if u.KeyPattern != "" {
							userEntry["key_pattern"] = u.KeyPattern
						}
						users = append(users, userEntry)
					}
					entry["users"] = users
				}
				instances = append(instances, entry)
			}
			tenantBody["valkey_instances"] = instances
		}

		// Nested S3 buckets
		if len(t.S3Buckets) > 0 {
			var buckets []map[string]any
			for _, s := range t.S3Buckets {
				s3ShardID, err := resolveShardID(s.Shard)
				if err != nil {
					return err
				}
				entry := map[string]any{
					"subscription_id": resolveSubID(s.Subscription),
					"shard_id":        s3ShardID,
				}
				if s.Public != nil {
					entry["public"] = *s.Public
				}
				if s.QuotaBytes != nil {
					entry["quota_bytes"] = *s.QuotaBytes
				}
				buckets = append(buckets, entry)
			}
			tenantBody["s3_buckets"] = buckets
		}

		// ONE API call creates tenant + all nested resources
		resp, err := client.Post("/tenants", tenantBody)
		if err != nil {
			return fmt.Errorf("create tenant %q: %w", t.Name, err)
		}

		tenantID, err := extractID(resp)
		if err != nil {
			return fmt.Errorf("parse tenant ID: %w", err)
		}
		tenantMap[t.Name] = tenantID
		tenantName := extractNameFromResp(resp)

		// Await ONE workflow — all nested resources provisioned when this returns
		fmt.Printf("  Tenant %q: %s (name=%s), awaiting workflow...\n", t.Name, tenantID, tenantName)
		if err := client.AwaitWorkflow(fmt.Sprintf("create-tenant-%s", tenantID)); err != nil {
			return fmt.Errorf("await tenant %q: %w", t.Name, err)
		}
		fmt.Printf("  Tenant %q: all resources active\n", t.Name)

		// Create zones that reference this tenant
		for _, z := range cfg.Zones {
			if z.Tenant == t.Name {
				zoneID, err := client.FindZoneByName(z.Name)
				if err == nil {
					fmt.Printf("  Zone %q: exists (%s, skipping)\n", z.Name, zoneID)
					continue
				}

				fmt.Printf("  Creating zone %q for tenant %q...\n", z.Name, t.Name)
				zoneBody := map[string]any{
					"name":            z.Name,
					"tenant_id":       tenantID,
					"subscription_id": resolveSubID(z.Subscription),
					"region_id":       regionID,
				}
				if z.Brand != "" {
					bID, ok := brandMap[z.Brand]
					if !ok {
						return fmt.Errorf("zone %q: brand %q not found (must be defined in brands section)", z.Name, z.Brand)
					}
					zoneBody["brand_id"] = bID
				}
				resp, err := client.Post("/zones", zoneBody)
				if err != nil {
					return fmt.Errorf("create zone %q: %w", z.Name, err)
				}
				zoneID, err = extractID(resp)
				if err != nil {
					return fmt.Errorf("parse zone ID: %w", err)
				}
				fmt.Printf("    Zone %q: %s, awaiting workflow...\n", z.Name, zoneID)
				if err := client.AwaitWorkflow(fmt.Sprintf("create-zone-%s", zoneID)); err != nil {
					return fmt.Errorf("await zone %q: %w", z.Name, err)
				}
				fmt.Printf("    Zone %q: active\n", z.Name)
			}
		}

		// --- Post-provisioning: resources that need IDs from the provisioned state ---
		ctx := &tenantSeedCtx{}

		// Track first FQDN for fixture template resolution
		if len(t.Webroots) > 0 && len(t.Webroots[0].FQDNs) > 0 {
			ctx.fqdn = t.Webroots[0].FQDNs[0].FQDN
		}

		// List webroots (for fixture deployment paths and backup source IDs)
		type resourceRef struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		}
		var webroots []resourceRef
		if len(t.Webroots) > 0 {
			webrootResp, err := client.Get(fmt.Sprintf("/tenants/%s/webroots", tenantID))
			if err != nil {
				return fmt.Errorf("list webroots for tenant %q: %w", t.Name, err)
			}
			if items, err := webrootResp.Items(); err == nil {
				_ = json.Unmarshal(items, &webroots)
			}
			if len(webroots) > 0 {
				ctx.webrootID = webroots[0].ID
			}
		}

		// List databases (for user creation, fixture .env, backup source IDs)
		var databases []resourceRef
		if len(t.Databases) > 0 {
			dbResp, err := client.Get(fmt.Sprintf("/tenants/%s/databases", tenantID))
			if err != nil {
				return fmt.Errorf("list databases for tenant %q: %w", t.Name, err)
			}
			if items, err := dbResp.Items(); err == nil {
				_ = json.Unmarshal(items, &databases)
			}

			// DB host for fixture .env
			dbNodeIPs, err := findNodeIPsByRole(client, clusterID, "database")
			if err != nil {
				return fmt.Errorf("find database node IPs: %w", err)
			}
			ctx.dbHost = dbNodeIPs[0]
		}

		// Create database users (need auto-generated DB name for username construction)
		for i, d := range t.Databases {
			if i >= len(databases) {
				break
			}
			db := databases[i]
			ctx.dbName = db.Name
			if ctx.dbID == "" {
				ctx.dbID = db.ID
			}

			for _, u := range d.Users {
				username := db.Name + u.Suffix
				fmt.Printf("  Creating database user %q...\n", username)
				userResp, err := client.Post(fmt.Sprintf("/databases/%s/users", db.ID), map[string]any{
					"username":   username,
					"password":   u.Password,
					"privileges": u.Privileges,
				})
				if err != nil {
					return fmt.Errorf("create database user %q: %w", username, err)
				}
				userID, err := extractID(userResp)
				if err != nil {
					return err
				}
				if ctx.dbUsername == "" {
					ctx.dbUsername = username
					ctx.dbPassword = u.Password
				}
				fmt.Printf("    User %q: %s, awaiting...\n", username, userID)
				if err := client.AwaitWorkflow(fmt.Sprintf("create-database-user-%s", userID)); err != nil {
					return fmt.Errorf("await database user %q: %w", username, err)
				}
				fmt.Printf("    User %q: active\n", username)
			}
		}

		// Deploy fixtures and set env vars (need provisioned filesystem)
		for i, w := range t.Webroots {
			if i >= len(webroots) {
				break
			}
			wr := webroots[i]

			// Set env vars via API
			if len(w.EnvVars) > 0 {
				vars := make([]map[string]any, 0, len(w.EnvVars))
				for _, ev := range w.EnvVars {
					vars = append(vars, map[string]any{
						"name":   ev.Name,
						"value":  ev.Value,
						"secret": ev.Secret,
					})
				}
				fmt.Printf("  Setting %d env vars on webroot %s...\n", len(vars), wr.Name)
				if _, err := client.Put(fmt.Sprintf("/webroots/%s/env-vars", wr.ID), map[string]any{
					"vars": vars,
				}); err != nil {
					return fmt.Errorf("set env vars for webroot %s: %w", wr.Name, err)
				}
			}

			// Deploy fixture
			if w.Fixture != nil {
				if err := seedFixture(tenantName, wr.Name, w.Fixture, ctx, cfg.LBTrafficURL, ctx.fqdn, webNodeIPs); err != nil {
					return fmt.Errorf("fixture for webroot %s of tenant %q: %w", wr.Name, t.Name, err)
				}
			}
		}

		// Create backups (need source IDs)
		if len(t.Backups) > 0 {
			if err := seedBackups(client, tenantID, t.Backups, ctx.webrootID, ctx.dbID); err != nil {
				return fmt.Errorf("backups for tenant %q: %w", t.Name, err)
			}
		}
	}

	// 4. Create OIDC clients
	for _, c := range cfg.OIDCClients {
		fmt.Printf("Creating OIDC client %q...\n", c.ID)
		_, err := client.Post("/oidc/clients", map[string]any{
			"id":            c.ID,
			"secret":        c.Secret,
			"name":          c.Name,
			"redirect_uris": c.RedirectURIs,
		})
		if err != nil {
			return fmt.Errorf("create OIDC client %q: %w", c.ID, err)
		}
		fmt.Printf("  OIDC client %q: created\n", c.ID)
	}

	// Print summary
	fmt.Println("\nSeed complete!")
	fmt.Printf("  Zones:   %d\n", len(cfg.Zones))
	fmt.Printf("  Tenants: %d\n", len(cfg.Tenants))

	_ = tenantMap // suppress unused warning

	return nil
}

// buildEmailAccountEntries converts seed email account defs to nested API request entries.
func buildEmailAccountEntries(emails []EmailAcctDef, resolveSubID func(string) string) []map[string]any {
	var accounts []map[string]any
	for _, e := range emails {
		acct := map[string]any{
			"subscription_id": resolveSubID(e.Subscription),
			"address":         e.Address,
		}
		if e.DisplayName != "" {
			acct["display_name"] = e.DisplayName
		}
		if e.QuotaBytes > 0 {
			acct["quota_bytes"] = e.QuotaBytes
		}
		if len(e.Aliases) > 0 {
			var aliases []map[string]any
			for _, a := range e.Aliases {
				aliases = append(aliases, map[string]any{"address": a.Address})
			}
			acct["aliases"] = aliases
		}
		if len(e.Forwards) > 0 {
			var forwards []map[string]any
			for _, fw := range e.Forwards {
				fwEntry := map[string]any{"destination": fw.Destination}
				if fw.KeepCopy != nil {
					fwEntry["keep_copy"] = *fw.KeepCopy
				}
				forwards = append(forwards, fwEntry)
			}
			acct["forwards"] = forwards
		}
		if e.AutoReply != nil {
			acct["autoreply"] = map[string]any{
				"subject": e.AutoReply.Subject,
				"body":    e.AutoReply.Body,
				"enabled": e.AutoReply.Enabled,
			}
		}
		accounts = append(accounts, acct)
	}
	return accounts
}

func seedFixture(tenantName, webrootName string, def *FixtureDef, ctx *tenantSeedCtx, lbURL, fqdn string, webNodeIPs []string) error {
	// Verify tarball exists.
	if _, err := os.Stat(def.Tarball); err != nil {
		return fmt.Errorf("fixture tarball %q: %w", def.Tarball, err)
	}

	webrootPath := fmt.Sprintf("/var/www/storage/%s/webroots/%s", tenantName, webrootName)

	// Upload and extract tarball on the first web node only (CephFS is shared across nodes).
	firstIP := webNodeIPs[0]
	fmt.Printf("    Uploading fixture to %s:%s...\n", firstIP, webrootPath)
	if err := scpFile(firstIP, def.Tarball, "/tmp/seed-fixture.tar.gz"); err != nil {
		return fmt.Errorf("scp to %s: %w", firstIP, err)
	}
	// Extract as the tenant user so files are owned correctly without chown.
	if _, err := sshExec(firstIP, fmt.Sprintf(
		"sudo -u %s tar -xzf /tmp/seed-fixture.tar.gz -C %s && sudo -u %s chmod -R 775 %s/storage %s/bootstrap/cache 2>/dev/null; rm -f /tmp/seed-fixture.tar.gz",
		tenantName, webrootPath,
		tenantName, webrootPath, webrootPath,
	)); err != nil {
		return fmt.Errorf("extract on %s: %w", firstIP, err)
	}
	fmt.Printf("    Fixture extracted on %s\n", firstIP)

	// Generate APP_KEY if needed.
	keyBytes := make([]byte, 32)
	if _, err := rand.Read(keyBytes); err != nil {
		return fmt.Errorf("generate app key: %w", err)
	}
	appKey := "base64:" + base64.StdEncoding.EncodeToString(keyBytes)

	// Resolve template vars in env_vars.
	resolved := make(map[string]string, len(def.EnvVars))
	for k, v := range def.EnvVars {
		v = strings.ReplaceAll(v, "${APP_KEY}", appKey)
		v = strings.ReplaceAll(v, "${DB_HOST}", ctx.dbHost)
		v = strings.ReplaceAll(v, "${DB_NAME}", ctx.dbName)
		v = strings.ReplaceAll(v, "${DB_USER}", ctx.dbUsername)
		v = strings.ReplaceAll(v, "${DB_PASS}", ctx.dbPassword)
		v = strings.ReplaceAll(v, "${FQDN}", fqdn)
		resolved[k] = v
	}

	// Write .env on the first web node (CephFS is shared).
	envContent := buildEnvContent(resolved)
	fmt.Printf("    Writing .env on %s...\n", firstIP)
	if _, err := sshExec(firstIP, fmt.Sprintf(
		"cat <<'ENVEOF' | sudo tee %s/.env > /dev/null\n%sENVEOF",
		webrootPath, envContent,
	)); err != nil {
		return fmt.Errorf("write .env on %s: %w", firstIP, err)
	}
	if _, err := sshExec(firstIP, fmt.Sprintf("sudo chown %s:%s %s/.env", tenantName, tenantName, webrootPath)); err != nil {
		return fmt.Errorf("chown .env on %s: %w", firstIP, err)
	}

	// Add /etc/hosts entry if requested.
	if def.HostsEntry && fqdn != "" {
		for _, ip := range webNodeIPs {
			fmt.Printf("    Adding /etc/hosts entry for %s on %s...\n", fqdn, ip)
			if _, err := sshExec(ip, fmt.Sprintf("echo '127.0.0.1 %s' | sudo tee -a /etc/hosts > /dev/null", fqdn)); err != nil {
				return fmt.Errorf("hosts entry on %s: %w", ip, err)
			}
		}
	}

	// Run setup/migrations if setup_path is defined.
	if def.SetupPath != "" && lbURL != "" {
		setupURL := lbURL + def.SetupPath
		fmt.Printf("    Running setup at %s (Host: %s)...\n", setupURL, fqdn)
		if err := retrySetup(setupURL, fqdn, 120*time.Second); err != nil {
			return fmt.Errorf("setup at %s failed: %w", setupURL, err)
		}
		fmt.Printf("    Setup complete\n")
	}

	return nil
}

func buildEnvContent(vars map[string]string) string {
	keys := make([]string, 0, len(vars))
	for k := range vars {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var lines []string
	for _, k := range keys {
		lines = append(lines, fmt.Sprintf("%s=%s", k, vars[k]))
	}
	return strings.Join(lines, "\n") + "\n"
}

func retrySetup(url, host string, timeout time.Duration) error {
	httpClient := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		req, err := http.NewRequest(http.MethodPost, url, nil)
		if err != nil {
			return fmt.Errorf("create request: %w", err)
		}
		req.Host = host
		req.Header.Set("Content-Type", "application/json")

		resp, err := httpClient.Do(req)
		if err != nil {
			lastErr = err
			time.Sleep(3 * time.Second)
			continue
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode == 200 {
			fmt.Printf("    Migration response: %s\n", string(body))
			return nil
		}
		lastErr = fmt.Errorf("status %d: %s", resp.StatusCode, string(body))
		time.Sleep(3 * time.Second)
	}
	return fmt.Errorf("timed out: %w", lastErr)
}

func generateUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func extractNameFromResp(resp *Response) string {
	var resource struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(resp.Body, &resource); err != nil {
		return "(unknown)"
	}
	return resource.Name
}

func seedBackups(client *Client, tenantID string, backups []BackupDef, webrootID, dbID string) error {
	for i, b := range backups {
		var sourceID string
		switch b.Type {
		case "web":
			if webrootID == "" {
				return fmt.Errorf("backup #%d: type is 'web' but no webroot was created", i+1)
			}
			sourceID = webrootID
		case "database":
			if dbID == "" {
				return fmt.Errorf("backup #%d: type is 'database' but no database was created", i+1)
			}
			sourceID = dbID
		default:
			return fmt.Errorf("backup #%d: unknown type %q", i+1, b.Type)
		}

		fmt.Printf("  Creating %s backup...\n", b.Type)
		resp, err := client.Post(fmt.Sprintf("/tenants/%s/backups", tenantID), map[string]any{
			"type":      b.Type,
			"source_id": sourceID,
		})
		if err != nil {
			return fmt.Errorf("create %s backup: %w", b.Type, err)
		}

		backupID, err := extractID(resp)
		if err != nil {
			return err
		}

		fmt.Printf("    Backup %s, awaiting workflow...\n", backupID)
		if err := client.AwaitWorkflow(fmt.Sprintf("create-backup-%s", backupID)); err != nil {
			return fmt.Errorf("await %s backup: %w", b.Type, err)
		}
		fmt.Printf("    Backup %s: completed\n", backupID)
	}
	return nil
}
