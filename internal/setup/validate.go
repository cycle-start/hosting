package setup

import (
	"fmt"
	"net"
	"strings"
)

// ValidationError represents a field-level validation error.
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// Validate checks the config and returns any validation errors.
func Validate(cfg *Config) []ValidationError {
	var errs []ValidationError
	add := func(field, msg string) {
		errs = append(errs, ValidationError{Field: field, Message: msg})
	}

	// Deploy mode
	switch cfg.DeployMode {
	case DeployModeSingle, DeployModeMulti:
	default:
		add("deploy_mode", "Must be single or multi")
	}

	// Target host (all-in-one mode)
	if cfg.DeployMode == DeployModeSingle && cfg.TargetHost != "" && cfg.TargetHost != "localhost" && !isValidIP(cfg.TargetHost) {
		add("target_host", "Must be a valid IP address")
	}

	// Region & cluster
	if cfg.RegionName == "" {
		add("region_name", "Region name is required")
	}
	if cfg.ClusterName == "" {
		add("cluster_name", "Cluster name is required")
	}

	// Brand
	if cfg.Brand.Name == "" {
		add("brand.name", "Brand name is required")
	}
	if cfg.Brand.PlatformDomain == "" {
		add("brand.platform_domain", "Platform domain is required")
	}
	if cfg.Brand.CustomerDomain == "" {
		add("brand.customer_domain", "Customer domain is required")
	}
	if cfg.Brand.PrimaryNS == "" {
		add("brand.primary_ns", "Primary nameserver hostname is required")
	}
	if cfg.Brand.PrimaryNSIP != "" && !isValidIP(cfg.Brand.PrimaryNSIP) {
		add("brand.primary_ns_ip", "Must be a valid IP address")
	}
	if cfg.Brand.SecondaryNS == "" {
		add("brand.secondary_ns", "Secondary nameserver hostname is required")
	}
	if cfg.Brand.SecondaryNSIP != "" && !isValidIP(cfg.Brand.SecondaryNSIP) {
		add("brand.secondary_ns_ip", "Must be a valid IP address")
	}
	if cfg.Brand.MailHostname == "" {
		add("brand.mail_hostname", "Mail hostname is required")
	}
	if cfg.Brand.HostmasterEmail == "" {
		add("brand.hostmaster_email", "Hostmaster email is required")
	}

	// Control plane DB
	if cfg.ControlPlane.Database.Mode == "external" {
		if cfg.ControlPlane.Database.Host == "" {
			add("control_plane.database.host", "Database host is required for external mode")
		}
		if cfg.ControlPlane.Database.Port <= 0 || cfg.ControlPlane.Database.Port > 65535 {
			add("control_plane.database.port", "Port must be 1-65535")
		}
		if cfg.ControlPlane.Database.Name == "" {
			add("control_plane.database.name", "Database name is required")
		}
		if cfg.ControlPlane.Database.User == "" {
			add("control_plane.database.user", "Database user is required")
		}
	}

	// Nodes (multi-node mode)
	if cfg.DeployMode == DeployModeMulti {
		if len(cfg.Nodes) == 0 {
			add("nodes", "At least one node is required in multi-node mode")
		}
		hasControlPlane := false
		for i, n := range cfg.Nodes {
			prefix := fmt.Sprintf("nodes[%d]", i)
			if n.Hostname == "" {
				add(prefix+".hostname", "Hostname is required")
			}
			if n.IP == "" {
				add(prefix+".ip", "IP address is required")
			} else if !isValidIP(n.IP) {
				add(prefix+".ip", "Must be a valid IP address")
			}
			if len(n.Roles) == 0 {
				add(prefix+".roles", "At least one role must be assigned")
			}
			for _, r := range n.Roles {
				if r == RoleControlPlane {
					hasControlPlane = true
				}
			}
		}
		if !hasControlPlane {
			add("nodes", "At least one node must have the controlplane role")
		}

		// Every role must be assigned to at least one node
		assignedRoles := map[NodeRole]bool{}
		for _, n := range cfg.Nodes {
			for _, r := range n.Roles {
				assignedRoles[r] = true
			}
		}
		var missing []string
		roleLabels := map[NodeRole]string{
			RoleControlPlane: "Control Plane", RoleWeb: "Web", RoleDatabase: "Database",
			RoleDNS: "DNS", RoleValkey: "Valkey", RoleEmail: "Email",
			RoleStorage: "Storage", RoleLB: "Load Balancer", RoleGateway: "Gateway",
			RoleDBAdmin: "DB Admin",
		}
		for _, role := range AllRoles {
			if !assignedRoles[role] {
				missing = append(missing, roleLabels[role])
			}
		}
		if len(missing) > 0 {
			add("nodes", fmt.Sprintf("Unassigned roles: %s", strings.Join(missing, ", ")))
		}
	}

	// PHP versions
	if len(cfg.PHPVersions) == 0 {
		add("php_versions", "At least one PHP version must be selected")
	}

	// TLS
	if cfg.TLS.Mode == "letsencrypt" && cfg.TLS.Email == "" {
		add("tls.email", "Email is required for Let's Encrypt")
	}

	// Email
	if cfg.Email.StalwartAdminToken == "" {
		add("email.stalwart_admin_token", "Stalwart admin token is required")
	}

	// API key
	if cfg.APIKey == "" {
		add("api_key", "API key is required")
	}

	return errs
}

func isValidIP(s string) bool {
	return net.ParseIP(strings.TrimSpace(s)) != nil
}
