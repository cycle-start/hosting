package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/edvin/hosting/internal/hostctl"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "cluster":
		if len(os.Args) < 3 || os.Args[2] != "apply" {
			fmt.Fprintln(os.Stderr, "Usage: hostctl cluster apply -f <cluster-definition.yaml>")
			os.Exit(1)
		}
		fs := flag.NewFlagSet("cluster apply", flag.ExitOnError)
		file := fs.String("f", "", "Path to cluster definition YAML file (required)")
		timeout := fs.Duration("timeout", 10*time.Minute, "Timeout for async operations")
		fs.Parse(os.Args[3:])

		if *file == "" {
			fmt.Fprintln(os.Stderr, "Error: -f flag is required")
			fs.Usage()
			os.Exit(1)
		}

		if err := hostctl.ClusterApply(*file, *timeout); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	case "seed":
		fs := flag.NewFlagSet("seed", flag.ExitOnError)
		file := fs.String("f", "", "Path to seed definition YAML file (required)")
		timeout := fs.Duration("timeout", 10*time.Minute, "Timeout for async operations")
		fs.Parse(os.Args[2:])

		if *file == "" {
			fmt.Fprintln(os.Stderr, "Error: -f flag is required")
			fs.Usage()
			os.Exit(1)
		}

		if err := hostctl.Seed(*file, *timeout); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	case "converge-shard":
		fs := flag.NewFlagSet("converge-shard", flag.ExitOnError)
		apiURL := fs.String("api", "http://localhost:8080", "Core API base URL")
		fs.Parse(os.Args[2:])

		if fs.NArg() < 1 {
			fmt.Fprintln(os.Stderr, "Usage: hostctl converge-shard [-api URL] <shard-id>")
			os.Exit(1)
		}

		if err := hostctl.ConvergeShard(*apiURL, fs.Arg(0)); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintln(os.Stderr, `Usage:
  hostctl cluster apply -f <cluster-definition.yaml>
  hostctl seed -f <seed-definition.yaml>
  hostctl converge-shard [-api URL] <shard-id>

Commands:
  cluster apply    Bootstrap cluster infrastructure from a YAML definition
  seed             Seed test data (tenants, webroots, FQDNs, zones, databases, email)
  converge-shard   Trigger shard convergence (push all resources to all nodes)

Flags:
  -f string         Path to YAML configuration file (required for cluster/seed)
  -api string       Core API base URL (default: http://localhost:8080)
  -timeout duration Timeout for async operations (default: 10m)`)
}
