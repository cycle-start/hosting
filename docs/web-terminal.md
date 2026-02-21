# Web Terminal

Browser-based SSH access to tenant shells via the admin UI, using SSH CA certificates and xterm.js.

## Architecture

```
Browser (xterm.js) → WebSocket → core-api SSH proxy → SSH with ephemeral cert → web node shell
```

The control plane acts as an SSH proxy: it signs a short-lived SSH certificate on demand using the platform's CA key, connects to the web node via SSH as the tenant user, and bridges the SSH session to the browser over WebSocket.

**No per-tenant key deployment** — sshd on web nodes trusts the CA public key, so any certificate signed by the CA is accepted. Certificates are ephemeral (60s TTL) and never stored.

## Setup

### 1. Generate CA Key Pair

```bash
just generate-ssh-ca
```

This creates `ssh_ca` (private key) and `ssh_ca.pub` (public key).

### 2. Deploy CA Public Key to Web Nodes

Add the public key content to your Ansible inventory/group vars:

```yaml
ssh_ca_public_key: "ssh-ed25519 AAAA... hosting-platform-ca"
```

Then deploy:

```bash
just ansible-role web --tags ssh_hardening
```

This places the public key at `/etc/ssh/hosting_ca.pub` and configures sshd to trust it via `TrustedUserCAKeys`.

### 3. Configure Core API

The `ssh_ca` file is automatically picked up by `just vm-deploy` (via `--set-file`). Just deploy:

```bash
just vm-deploy
```

The terminal endpoint is only registered when `SSH_CA_PRIVATE_KEY` is set.

## API Endpoint

### `GET /api/v1/tenants/{tenantID}/terminal?token={apiKey}`

WebSocket upgrade endpoint. Authenticates via the `token` query parameter (standard approach since the WebSocket API doesn't support custom headers).

**Flow:**
1. Validate API key from query param
2. Look up tenant — check `ssh_enabled` is true
3. Resolve a web node IP from the tenant's shard
4. Sign ephemeral SSH certificate (60s TTL, principal = tenant name)
5. Upgrade to WebSocket
6. Dial SSH to node, request PTY + shell
7. Bidirectional pipe: WebSocket binary frames = terminal I/O, text frames = control messages

**Control messages** (client → server, JSON text frames):
```json
{"type": "resize", "cols": 120, "rows": 40}
```

## Admin UI

The tenant detail page shows a "Terminal" button when `ssh_enabled` is true. Clicking it opens a dialog with an xterm.js terminal connected via WebSocket.

## Security

- **Short-lived certificates**: 60-second TTL with 30-second clock skew allowance
- **Per-connection ephemeral keys**: Each session generates a new Ed25519 key pair
- **Principal enforcement**: Certificate is only valid for the specific tenant username
- **No key storage**: Neither the ephemeral private key nor the certificate is persisted
- **CA key in Kubernetes Secret**: The CA private key is stored as a Kubernetes secret, not on disk

## Configuration

| Env Var | Description | Default |
|---------|-------------|---------|
| `SSH_CA_PRIVATE_KEY` | PEM-encoded SSH CA private key | (empty — terminal disabled) |

## Files

| File | Purpose |
|------|---------|
| `internal/sshca/sshca.go` | CA key parsing and certificate signing |
| `internal/sshca/sshca_test.go` | Unit tests for certificate signing |
| `internal/api/handler/terminal.go` | WebSocket SSH proxy handler |
| `web/admin/src/components/shared/web-terminal.tsx` | xterm.js terminal component |
| `ansible/roles/ssh_hardening/files/00-hosting-base.conf` | sshd config with `TrustedUserCAKeys` |
