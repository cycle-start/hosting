package hostctl

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

func Seed(configPath string, timeout time.Duration) error {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("read config: %w", err)
	}

	var cfg SeedConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("parse config: %w", err)
	}

	client := NewClient(cfg.APIURL)

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
	fqdnMap := map[string]string{}  // fqdn string -> ID
	zoneMap := map[string]string{}  // zone name -> ID
	tenantMap := map[string]string{} // tenant name -> ID

	// 1. Create zones (without tenant link first)
	for _, z := range cfg.Zones {
		fmt.Printf("Creating zone %q...\n", z.Name)
		resp, err := client.Post("/zones", map[string]any{
			"name":      z.Name,
			"region_id": regionID,
		})
		if err != nil {
			return fmt.Errorf("create zone %q: %w", z.Name, err)
		}

		zoneID, err := extractID(resp)
		if err != nil {
			return fmt.Errorf("parse zone ID: %w", err)
		}
		zoneMap[z.Name] = zoneID

		fmt.Printf("  Zone %q: %s, waiting for active...\n", z.Name, zoneID)
		if err := client.WaitForStatus(fmt.Sprintf("/zones/%s", zoneID), "active", timeout); err != nil {
			return fmt.Errorf("wait for zone %q: %w", z.Name, err)
		}
		fmt.Printf("  Zone %q: active\n", z.Name)
	}

	// 2. Create tenants and their resources
	for _, t := range cfg.Tenants {
		shardID, err := client.FindShardByName(clusterID, t.Shard)
		if err != nil {
			return fmt.Errorf("resolve shard %q for tenant %q: %w", t.Shard, t.Name, err)
		}

		fmt.Printf("Creating tenant %q...\n", t.Name)
		resp, err := client.Post("/tenants", map[string]any{
			"name":         t.Name,
			"region_id":    regionID,
			"cluster_id":   clusterID,
			"shard_id":     shardID,
			"sftp_enabled": t.SFTPEnabled,
		})
		if err != nil {
			return fmt.Errorf("create tenant %q: %w", t.Name, err)
		}

		tenantID, err := extractID(resp)
		if err != nil {
			return fmt.Errorf("parse tenant ID: %w", err)
		}
		tenantMap[t.Name] = tenantID

		fmt.Printf("  Tenant %q: %s, waiting for active...\n", t.Name, tenantID)
		if err := client.WaitForStatus(fmt.Sprintf("/tenants/%s", tenantID), "active", timeout); err != nil {
			return fmt.Errorf("wait for tenant %q: %w", t.Name, err)
		}
		fmt.Printf("  Tenant %q: active\n", t.Name)

		// Link zones that reference this tenant
		for _, z := range cfg.Zones {
			if z.Tenant == t.Name {
				zoneID := zoneMap[z.Name]
				fmt.Printf("  Linking zone %q to tenant %q...\n", z.Name, t.Name)
				_, err := client.Put(fmt.Sprintf("/zones/%s/tenant", zoneID), map[string]any{
					"tenant_id": tenantID,
				})
				if err != nil {
					return fmt.Errorf("link zone %q to tenant %q: %w", z.Name, t.Name, err)
				}
			}
		}

		// Create webroots
		for _, w := range t.Webroots {
			if err := seedWebroot(client, tenantID, w, fqdnMap, timeout); err != nil {
				return fmt.Errorf("webroot %q for tenant %q: %w", w.Name, t.Name, err)
			}
		}

		// Create databases
		for _, d := range t.Databases {
			if err := seedDatabase(client, tenantID, clusterID, d, timeout); err != nil {
				return fmt.Errorf("database %q for tenant %q: %w", d.Name, t.Name, err)
			}
		}

		// Create valkey instances
		for _, v := range t.ValkeyInstances {
			if err := seedValkeyInstance(client, tenantID, clusterID, v, timeout); err != nil {
				return fmt.Errorf("valkey instance %q for tenant %q: %w", v.Name, t.Name, err)
			}
		}

		// Create email accounts
		for _, e := range t.EmailAccounts {
			if err := seedEmailAccount(client, e, fqdnMap, timeout); err != nil {
				return fmt.Errorf("email account %q for tenant %q: %w", e.Address, t.Name, err)
			}
		}
	}

	// Print summary
	fmt.Println("\nSeed complete!")
	fmt.Printf("  Zones:   %d\n", len(cfg.Zones))
	fmt.Printf("  Tenants: %d\n", len(cfg.Tenants))

	_ = tenantMap // suppress unused warning

	return nil
}

func seedWebroot(client *Client, tenantID string, def WebrootDef, fqdnMap map[string]string, timeout time.Duration) error {
	body := map[string]any{
		"name":            def.Name,
		"runtime":         def.Runtime,
		"runtime_version": def.RuntimeVersion,
	}
	if def.PublicFolder != "" {
		body["public_folder"] = def.PublicFolder
	}
	if def.RuntimeConfig != nil {
		body["runtime_config"] = def.RuntimeConfig
	}

	fmt.Printf("  Creating webroot %q...\n", def.Name)
	resp, err := client.Post(fmt.Sprintf("/tenants/%s/webroots", tenantID), body)
	if err != nil {
		return fmt.Errorf("create: %w", err)
	}

	webrootID, err := extractID(resp)
	if err != nil {
		return err
	}

	fmt.Printf("    Webroot %q: %s, waiting for active...\n", def.Name, webrootID)
	if err := client.WaitForStatus(fmt.Sprintf("/webroots/%s", webrootID), "active", timeout); err != nil {
		return fmt.Errorf("wait: %w", err)
	}
	fmt.Printf("    Webroot %q: active\n", def.Name)

	// Create FQDNs
	for _, f := range def.FQDNs {
		fmt.Printf("    Creating FQDN %q...\n", f.FQDN)
		fqdnResp, err := client.Post(fmt.Sprintf("/webroots/%s/fqdns", webrootID), map[string]any{
			"fqdn":        f.FQDN,
			"ssl_enabled": f.SSLEnabled,
		})
		if err != nil {
			return fmt.Errorf("create FQDN %q: %w", f.FQDN, err)
		}

		fqdnID, err := extractID(fqdnResp)
		if err != nil {
			return err
		}
		fqdnMap[f.FQDN] = fqdnID

		fmt.Printf("      FQDN %q: %s, waiting for active...\n", f.FQDN, fqdnID)
		if err := client.WaitForStatus(fmt.Sprintf("/fqdns/%s", fqdnID), "active", timeout); err != nil {
			return fmt.Errorf("wait for FQDN %q: %w", f.FQDN, err)
		}
		fmt.Printf("      FQDN %q: active\n", f.FQDN)
	}

	return nil
}

func seedDatabase(client *Client, tenantID, clusterID string, def DatabaseDef, timeout time.Duration) error {
	shardID, err := client.FindShardByName(clusterID, def.Shard)
	if err != nil {
		return fmt.Errorf("resolve shard %q: %w", def.Shard, err)
	}

	fmt.Printf("  Creating database %q...\n", def.Name)
	resp, err := client.Post(fmt.Sprintf("/tenants/%s/databases", tenantID), map[string]any{
		"name":     def.Name,
		"shard_id": shardID,
	})
	if err != nil {
		return fmt.Errorf("create: %w", err)
	}

	dbID, err := extractID(resp)
	if err != nil {
		return err
	}

	fmt.Printf("    Database %q: %s, waiting for active...\n", def.Name, dbID)
	if err := client.WaitForStatus(fmt.Sprintf("/databases/%s", dbID), "active", timeout); err != nil {
		return fmt.Errorf("wait: %w", err)
	}
	fmt.Printf("    Database %q: active\n", def.Name)

	// Create users
	for _, u := range def.Users {
		fmt.Printf("    Creating database user %q...\n", u.Username)
		userResp, err := client.Post(fmt.Sprintf("/databases/%s/users", dbID), map[string]any{
			"username":   u.Username,
			"password":   u.Password,
			"privileges": u.Privileges,
		})
		if err != nil {
			return fmt.Errorf("create user %q: %w", u.Username, err)
		}

		userID, err := extractID(userResp)
		if err != nil {
			return err
		}

		fmt.Printf("      User %q: %s, waiting for active...\n", u.Username, userID)
		if err := client.WaitForStatus(fmt.Sprintf("/database-users/%s", userID), "active", timeout); err != nil {
			return fmt.Errorf("wait for user %q: %w", u.Username, err)
		}
		fmt.Printf("      User %q: active\n", u.Username)
	}

	return nil
}

func seedValkeyInstance(client *Client, tenantID, clusterID string, def ValkeyInstanceDef, timeout time.Duration) error {
	shardID, err := client.FindShardByName(clusterID, def.Shard)
	if err != nil {
		return fmt.Errorf("resolve shard %q: %w", def.Shard, err)
	}

	body := map[string]any{
		"name":     def.Name,
		"shard_id": shardID,
	}
	if def.MaxMemoryMB > 0 {
		body["max_memory_mb"] = def.MaxMemoryMB
	}

	fmt.Printf("  Creating valkey instance %q...\n", def.Name)
	resp, err := client.Post(fmt.Sprintf("/tenants/%s/valkey-instances", tenantID), body)
	if err != nil {
		return fmt.Errorf("create: %w", err)
	}

	instanceID, err := extractID(resp)
	if err != nil {
		return err
	}

	fmt.Printf("    Valkey %q: %s, waiting for active...\n", def.Name, instanceID)
	if err := client.WaitForStatus(fmt.Sprintf("/valkey-instances/%s", instanceID), "active", timeout); err != nil {
		return fmt.Errorf("wait: %w", err)
	}
	fmt.Printf("    Valkey %q: active\n", def.Name)

	// Create users
	for _, u := range def.Users {
		userBody := map[string]any{
			"username":   u.Username,
			"password":   u.Password,
			"privileges": u.Privileges,
		}
		if u.KeyPattern != "" {
			userBody["key_pattern"] = u.KeyPattern
		}

		fmt.Printf("    Creating valkey user %q...\n", u.Username)
		userResp, err := client.Post(fmt.Sprintf("/valkey-instances/%s/users", instanceID), userBody)
		if err != nil {
			return fmt.Errorf("create user %q: %w", u.Username, err)
		}

		userID, err := extractID(userResp)
		if err != nil {
			return err
		}

		fmt.Printf("      User %q: %s, waiting for active...\n", u.Username, userID)
		if err := client.WaitForStatus(fmt.Sprintf("/valkey-users/%s", userID), "active", timeout); err != nil {
			return fmt.Errorf("wait for user %q: %w", u.Username, err)
		}
		fmt.Printf("      User %q: active\n", u.Username)
	}

	return nil
}

func seedEmailAccount(client *Client, def EmailAcctDef, fqdnMap map[string]string, timeout time.Duration) error {
	fqdnID, ok := fqdnMap[def.FQDN]
	if !ok {
		return fmt.Errorf("FQDN %q not found (must be created in a webroot first)", def.FQDN)
	}

	body := map[string]any{
		"address": def.Address,
	}
	if def.DisplayName != "" {
		body["display_name"] = def.DisplayName
	}
	if def.QuotaBytes > 0 {
		body["quota_bytes"] = def.QuotaBytes
	}

	fmt.Printf("  Creating email account %q...\n", def.Address)
	resp, err := client.Post(fmt.Sprintf("/fqdns/%s/email-accounts", fqdnID), body)
	if err != nil {
		return fmt.Errorf("create: %w", err)
	}

	emailID, err := extractID(resp)
	if err != nil {
		return err
	}

	fmt.Printf("    Email %q: %s, waiting for active...\n", def.Address, emailID)
	if err := client.WaitForStatus(fmt.Sprintf("/email-accounts/%s", emailID), "active", timeout); err != nil {
		return fmt.Errorf("wait: %w", err)
	}
	fmt.Printf("    Email %q: active\n", def.Address)

	// Create aliases
	for _, a := range def.Aliases {
		if err := seedEmailAlias(client, emailID, a, timeout); err != nil {
			return fmt.Errorf("alias %q: %w", a.Address, err)
		}
	}

	// Create forwards
	for _, f := range def.Forwards {
		if err := seedEmailForward(client, emailID, f, timeout); err != nil {
			return fmt.Errorf("forward %q: %w", f.Destination, err)
		}
	}

	// Create auto-reply
	if def.AutoReply != nil {
		if err := seedEmailAutoReply(client, emailID, *def.AutoReply); err != nil {
			return fmt.Errorf("autoreply: %w", err)
		}
	}

	return nil
}

func seedEmailAlias(client *Client, emailAccountID string, def EmailAliasDef, timeout time.Duration) error {
	fmt.Printf("    Creating email alias %q...\n", def.Address)
	resp, err := client.Post(fmt.Sprintf("/email-accounts/%s/aliases", emailAccountID), map[string]any{
		"address": def.Address,
	})
	if err != nil {
		return fmt.Errorf("create: %w", err)
	}

	aliasID, err := extractID(resp)
	if err != nil {
		return err
	}

	fmt.Printf("      Alias %q: %s, waiting for active...\n", def.Address, aliasID)
	if err := client.WaitForStatus(fmt.Sprintf("/email-aliases/%s", aliasID), "active", timeout); err != nil {
		return fmt.Errorf("wait: %w", err)
	}
	fmt.Printf("      Alias %q: active\n", def.Address)

	return nil
}

func seedEmailForward(client *Client, emailAccountID string, def EmailForwardDef, timeout time.Duration) error {
	body := map[string]any{
		"destination": def.Destination,
	}
	if def.KeepCopy != nil {
		body["keep_copy"] = *def.KeepCopy
	}

	fmt.Printf("    Creating email forward to %q...\n", def.Destination)
	resp, err := client.Post(fmt.Sprintf("/email-accounts/%s/forwards", emailAccountID), body)
	if err != nil {
		return fmt.Errorf("create: %w", err)
	}

	forwardID, err := extractID(resp)
	if err != nil {
		return err
	}

	fmt.Printf("      Forward to %q: %s, waiting for active...\n", def.Destination, forwardID)
	if err := client.WaitForStatus(fmt.Sprintf("/email-forwards/%s", forwardID), "active", timeout); err != nil {
		return fmt.Errorf("wait: %w", err)
	}
	fmt.Printf("      Forward to %q: active\n", def.Destination)

	return nil
}

func seedEmailAutoReply(client *Client, emailAccountID string, def EmailAutoReplyDef) error {
	fmt.Printf("    Setting email auto-reply...\n")
	_, err := client.Put(fmt.Sprintf("/email-accounts/%s/autoreply", emailAccountID), map[string]any{
		"subject": def.Subject,
		"body":    def.Body,
		"enabled": def.Enabled,
	})
	if err != nil {
		return fmt.Errorf("set: %w", err)
	}
	fmt.Printf("      Auto-reply: set\n")

	return nil
}
