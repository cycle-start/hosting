# Hosting Platform

Web hosting platform with DNS, databases, email, and automated provisioning.

## Getting started

See [docs/getting-started.md](docs/getting-started.md).

## Quick reference

```bash
just dev-up                                              # Start everything
go run ./cmd/hostctl cluster apply -f clusters/dev.yaml  # Bootstrap cluster
go run ./cmd/hostctl seed -f seeds/dev-tenants.yaml      # Seed test data
just down-clean                                          # Teardown
just --list                                              # All commands
```
