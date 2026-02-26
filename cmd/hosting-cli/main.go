package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/edvin/hosting/internal/cli"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "import":
		cmdImport(os.Args[2:])
	case "profiles":
		cmdProfiles(os.Args[2:])
	case "use":
		cmdUse(os.Args[2:])
	case "active":
		cmdActive()
	case "tunnel":
		cmdTunnel(os.Args[2:])
	case "proxy":
		cmdProxy(os.Args[2:])
	case "status":
		cmdStatus()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func cmdImport(args []string) {
	fs := flag.NewFlagSet("import", flag.ExitOnError)
	tenantID := fs.String("tenant", "", "Tenant ID (also used as profile name unless -name is given)")
	name := fs.String("name", "", "Override profile name (default: tenant ID, or filename)")
	setActive := fs.Bool("set-active", true, "Set this profile as active after import")
	fs.Parse(args)

	if fs.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "Usage: hosting-cli import [-tenant TENANT_ID] [-name NAME] <config-file>")
		os.Exit(1)
	}

	profile, err := cli.Import(fs.Arg(0), *name, *tenantID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Imported profile %q", profile.Name)
	if profile.TenantID != "" {
		fmt.Printf(" (tenant: %s)", profile.TenantID)
	}
	fmt.Println()

	if *setActive {
		if err := cli.SetActive(profile.Name); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not set active profile: %v\n", err)
		} else {
			fmt.Printf("Active profile set to %q\n", profile.Name)
		}
	}
}

func cmdProfiles(args []string) {
	profiles, err := cli.ListProfiles()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if len(profiles) == 0 {
		fmt.Println("No profiles found. Import one with: hosting-cli import <config-file>")
		return
	}

	active, _ := cli.GetActive()

	fmt.Printf("%-20s %-30s %s\n", "NAME", "TENANT", "ACTIVE")
	for _, p := range profiles {
		marker := ""
		if p.Name == active {
			marker = " *"
		}
		tenant := p.TenantID
		if tenant == "" {
			tenant = "-"
		}
		fmt.Printf("%-20s %-30s %s\n", p.Name, tenant, marker)
	}

	// Handle delete subcommand.
	if len(args) > 0 && args[0] == "delete" {
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: hosting-cli profiles delete <name>")
			os.Exit(1)
		}
		if err := cli.DeleteProfile(args[1]); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Deleted profile %q\n", args[1])
	}
}

func cmdUse(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: hosting-cli use <profile-name>")
		os.Exit(1)
	}

	name := args[0]
	if err := cli.SetActive(name); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Active profile set to %q\n", name)
}

func cmdActive() {
	active, err := cli.GetActive()
	if err != nil || active == "" {
		fmt.Println("No active profile. Set one with: hosting-cli use <name>")
		return
	}

	profile, cfg, err := cli.LoadProfile(active)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading active profile: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Active profile: %s\n", profile.Name)
	if profile.TenantID != "" {
		fmt.Printf("Tenant:         %s\n", profile.TenantID)
	}
	fmt.Printf("Address:        %s\n", cfg.Address.String())
	fmt.Printf("Endpoint:       %s\n", cfg.Endpoint)
	if len(cfg.Services) > 0 {
		fmt.Println("Services:")
		for _, svc := range cfg.Services {
			fmt.Printf("  %s → %s (port %d)\n", svc.Type, svc.Address, svc.DefaultPort())
		}
	}
}

func cmdTunnel(args []string) {
	fs := flag.NewFlagSet("tunnel", flag.ExitOnError)
	profileName := fs.String("profile", "", "Profile to use (default: active profile)")
	fs.Parse(args)

	name := *profileName
	if name == "" && fs.NArg() > 0 {
		name = fs.Arg(0)
	}
	if name == "" {
		var err error
		name, err = cli.GetActive()
		if err != nil || name == "" {
			fmt.Fprintln(os.Stderr, "No profile specified and no active profile set.")
			fmt.Fprintln(os.Stderr, "Usage: hosting-cli tunnel [profile-name]")
			os.Exit(1)
		}
	}

	_, cfg, err := cli.LoadProfile(name)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Establishing tunnel with profile %q...\n", name)
	tunnel, err := cli.CreateTunnel(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer tunnel.Close()

	fmt.Printf("Tunnel active. Local address: %s\n", cfg.Address.Addr().String())
	fmt.Println("Press Ctrl+C to disconnect.")

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	fmt.Println("\nDisconnecting...")
}

func cmdProxy(args []string) {
	fs := flag.NewFlagSet("proxy", flag.ExitOnError)
	profileName := fs.String("profile", "", "Profile to use (default: active profile)")
	mysqlPort := fs.Int("mysql-port", 3306, "Local port for MySQL proxy")
	valkeyPort := fs.Int("valkey-port", 6379, "Local port for Valkey proxy")
	target := fs.String("target", "", "Override target address (e.g. [fd00::1]:3306)")
	localPort := fs.Int("port", 0, "Local port when using -target")
	fs.Parse(args)

	name := *profileName
	if name == "" {
		var err error
		name, err = cli.GetActive()
		if err != nil || name == "" {
			fmt.Fprintln(os.Stderr, "No profile specified and no active profile set.")
			fmt.Fprintln(os.Stderr, "Usage: hosting-cli proxy [-profile NAME] [-mysql-port 3306] [-valkey-port 6379]")
			os.Exit(1)
		}
	}

	_, cfg, err := cli.LoadProfile(name)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Establishing tunnel with profile %q...\n", name)
	tunnel, err := cli.CreateTunnel(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer tunnel.Close()

	// If a manual target is specified, proxy just that.
	if *target != "" {
		if *localPort == 0 {
			fmt.Fprintln(os.Stderr, "Error: -port is required when using -target")
			os.Exit(1)
		}
		svc := cli.ServiceEntry{Type: "custom", Address: *target}
		pt := cli.ProxyTarget{Service: svc, LocalPort: *localPort}
		listener, err := cli.StartProxy(tunnel, pt)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Proxying localhost:%d → %s\n", *localPort, *target)
		defer listener.Close()
	} else {
		// Auto-proxy services from config metadata.
		if len(cfg.Services) == 0 {
			fmt.Println("No services found in config. Use -target to specify a manual target.")
			fmt.Println("Tunnel is active. Press Ctrl+C to disconnect.")
		}

		var listeners []string
		for _, svc := range cfg.Services {
			port := svc.DefaultPort()
			switch svc.Type {
			case "mysql":
				port = *mysqlPort
			case "valkey":
				port = *valkeyPort
			}

			pt := cli.ProxyTarget{Service: svc, LocalPort: port}
			listener, err := cli.StartProxy(tunnel, pt)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to proxy %s on port %d: %v\n", svc.Type, port, err)
				continue
			}
			defer listener.Close()
			listeners = append(listeners, fmt.Sprintf("  %s → localhost:%d", svc.Type, port))
		}

		if len(listeners) > 0 {
			fmt.Println("Proxying services:")
			fmt.Println(strings.Join(listeners, "\n"))
		}
	}

	fmt.Println("\nPress Ctrl+C to disconnect.")

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	fmt.Println("\nDisconnecting...")
}

func cmdStatus() {
	active, _ := cli.GetActive()
	if active == "" {
		fmt.Println("No active profile.")
		return
	}

	profile, cfg, err := cli.LoadProfile(active)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Profile:    %s\n", profile.Name)
	if profile.TenantID != "" {
		fmt.Printf("Tenant:     %s\n", profile.TenantID)
	}
	fmt.Printf("Address:    %s\n", cfg.Address.String())
	fmt.Printf("Endpoint:   %s\n", cfg.Endpoint)
	fmt.Printf("Services:   %d\n", len(cfg.Services))
	for _, svc := range cfg.Services {
		fmt.Printf("  %s → %s\n", svc.Type, svc.Address)
	}
}

func printUsage() {
	fmt.Fprintln(os.Stderr, `hosting-cli — WireGuard tunnel client for hosting platform

Usage:
  hosting-cli import [-tenant ID] <config-file>
  hosting-cli profiles [delete <name>]
  hosting-cli use <tenant-id>
  hosting-cli active
  hosting-cli tunnel [tenant-id]
  hosting-cli proxy [-mysql-port 3306] [-valkey-port 6379]
  hosting-cli proxy -target [addr]:port -port <local-port>
  hosting-cli status

Commands:
  import     Import a WireGuard config file (profile named after tenant ID)
  profiles   List saved profiles (or delete one)
  use        Set the active tenant
  active     Show details of the active tenant profile
  tunnel     Establish a WireGuard tunnel
  proxy      Establish tunnel and proxy services to localhost
  status     Show profile and service info

Profiles are stored in ~/.config/hosting/profiles/, keyed by tenant ID.
Use "hosting-cli use <tenant-id>" to switch between tenants.`)
}
