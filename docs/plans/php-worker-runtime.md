# PHP Worker Runtime

## Problem Statement

The hosting platform currently supports web-facing runtimes: PHP (FPM), Node.js, Python (Gunicorn), Ruby (Puma), and Static. Each webroot gets an nginx server block and a runtime process to serve HTTP traffic. However, many applications need long-running background processes that have no HTTP interface -- Laravel queue workers (`php artisan queue:work`), custom daemons, scheduled task runners, and similar workloads.

Today, tenants have no way to run persistent PHP background processes on the platform. They must resort to cron-based workarounds or external queue processors, which are unreliable, inefficient, and unmonitored.

This plan introduces `php-worker` as a new runtime type. A webroot with runtime `php-worker` runs a long-lived PHP process managed by supervisord instead of PHP-FPM. It has NO nginx server block (no HTTP traffic), but can access the same webroot files on CephFS. This enables first-class support for Laravel queue workers, custom PHP daemons, and similar background processes.

---

## Architecture Overview

```
                    +-------------------+
                    |    core-api       |
                    |  (desired state)  |
                    +--------+----------+
                             |
                    Temporal |  workflow
                             |
                    +--------+----------+
                    |    node-agent     |
                    |  (web shard)      |
                    +--------+----------+
                             |
              +--------------+--------------+
              |                             |
     Web-facing webroots           Worker webroots
     (nginx + PHP-FPM/Node/etc)   (supervisord only)
              |                             |
     +--------+--------+          +--------+--------+
     | nginx server     |          | supervisord     |
     | block + runtime  |          | program config  |
     | process          |          | (no nginx)      |
     +------------------+          +-----------------+
```

A single web node runs both nginx (for web-facing webroots) and supervisord (for worker webroots). They share the same CephFS mount at `/var/www/storage/{tenant}/webroots/{name}`, so a worker webroot can access files from a sibling web-facing webroot if they are in the same tenant directory, or more commonly, the worker webroot IS the same codebase as the web webroot but configured with a different runtime.

### Typical use case: Laravel application

A tenant deploys a Laravel application and creates two webroots:

1. **Webroot `app`** -- runtime `php`, public folder `public`, FQDNs attached. Serves HTTP traffic via nginx + PHP-FPM.
2. **Webroot `queue-worker`** -- runtime `php-worker`, no FQDNs, no nginx. Runs `php artisan queue:work --queue=default --tries=3` via supervisord. The `directory` config points to the `app` webroot's directory so the worker operates on the same codebase.

---

## Why Supervisord (Not Systemd)

The existing runtimes (Node.js, Python, Ruby) use systemd service units. For the PHP worker runtime, supervisord is a better fit:

| Concern | systemd | supervisord |
|---------|---------|-------------|
| **Multi-process workers** | Requires a separate unit per instance, or a template unit. Management of N processes is manual. | Native `numprocs` directive spawns N identical processes from one config. |
| **Anti-thrash protection** | `StartLimitIntervalSec` + `StartLimitBurst` -- blunt; puts the unit in a failed state that requires manual intervention or timer-based reset. | `startsecs` + `startretries` + `autorestart=unexpected` -- fine-grained; process must survive N seconds to count as started, retries are configurable, and FATAL state is clearly visible. |
| **Log capture** | `StandardOutput=append:...` works but log rotation requires logrotate integration. | Built-in `stdout_logfile_maxbytes` + `stdout_logfile_backups` for per-program log rotation with zero external dependencies. |
| **Status introspection** | `systemctl status` -- verbose, requires parsing. | `supervisorctl status` -- clean machine-readable output: `program_name STATE pid uptime`. Easy to parse programmatically for health monitoring. |
| **Tenant isolation** | Runs as tenant user via `User=`. | Runs as tenant user via `user=`. Equivalent. |
| **Process groups** | Not native; requires slice/scope management. | `group:` directive groups related programs for batch start/stop. |
| **Graceful shutdown** | Sends SIGTERM, configurable `TimeoutStopSec`. | Sends configurable `stopsignal`, configurable `stopwaitsecs`. Equivalent. |

The key differentiator is `numprocs`. A single Laravel application commonly needs 1-8 queue worker processes. With systemd, this requires template units (`worker@1.service`, `worker@2.service`, ...) and manual enabling/disabling of each instance. With supervisord, a single config file with `numprocs=4` handles everything.

Additionally, supervisord provides a clear FATAL state for processes that crash on startup, which maps naturally to the platform's `failed` status. The agent can detect FATAL processes via `supervisorctl status` and report them back to core-api.

### Coexistence with systemd

Supervisord itself is managed as a systemd service (`supervisor.service`). The node-agent does not need to manage supervisord's lifecycle directly -- it is started on boot via systemd and the agent only interacts with it through `supervisorctl` and config files. This is the same pattern used by the existing `php-fpm` runtime (the agent manages pool configs, not the FPM master process itself).

---

## Supervisord Installation (Packer)

### Changes to `packer/scripts/web.sh`

Add `supervisor` to the apt packages:

```bash
apt-get install -y \
  nginx \
  php8.3-fpm php8.3-cli php8.3-mysql php8.3-curl \
  php8.3-mbstring php8.3-xml php8.3-zip \
  supervisor \        # NEW: process manager for php-worker runtime
  ceph-common \
  openssh-server
```

### Base supervisord configuration

Create a Packer file `packer/files/supervisor-hosting.conf` installed to `/etc/supervisor/conf.d/hosting.conf`:

```ini
; Base configuration for hosting platform worker processes.
; Individual worker configs are written to /etc/supervisor/conf.d/worker-*.conf
; by the node-agent.

[supervisord]
; Override log location for consistency with platform logging.
childlogdir=/var/log/supervisor

[inet_http_server]
; Local-only HTTP interface for supervisorctl and monitoring.
; Bound to localhost only -- not accessible from outside the VM.
port=127.0.0.1:9001

[rpcinterface:supervisor]
supervisor.rpcinterface_factory = supervisor.rpcinterface:make_main_rpcinterface
```

The `inet_http_server` on localhost:9001 is optional but useful for future Prometheus metrics export via a supervisor exporter. It is NOT required for basic operation since `supervisorctl` communicates over the Unix socket by default.

### Enable supervisord on boot

Supervisord on Ubuntu/Debian is enabled by default when installed via apt. Verify in Packer:

```bash
systemctl enable supervisor
```

---

## Data Model Changes

### `runtime_config` JSON Schema for `php-worker`

The existing `runtime_config` field (`json.RawMessage` / `JSONB` in the DB) is the right place for worker-specific configuration. No schema migration is needed -- the field already exists and is opaque JSON.

For `php-worker` webroots, `runtime_config` carries:

```json
{
  "command": "php artisan queue:work --queue=default --tries=3",
  "directory": "",
  "num_procs": 1,
  "stop_signal": "TERM",
  "stop_wait_secs": 30,
  "start_secs": 10,
  "start_retries": 3,
  "environment": {}
}
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `command` | string | **required** | The command to execute. Must start with `php`. |
| `directory` | string | `""` (webroot dir) | Working directory, relative to `/var/www/storage/{tenant}/webroots/`. If empty, defaults to the webroot's own directory. If set (e.g., `"app"`), resolves to `/var/www/storage/{tenant}/webroots/app`. This allows a worker webroot to run in a sibling webroot's directory. |
| `num_procs` | int | `1` | Number of worker process instances (1-8). supervisord's `numprocs` directive. |
| `stop_signal` | string | `"TERM"` | Signal to send for graceful shutdown. Laravel workers handle SIGTERM. |
| `stop_wait_secs` | int | `30` | Seconds to wait after stop signal before SIGKILL. Laravel queue workers need time to finish the current job. |
| `start_secs` | int | `10` | Process must survive this many seconds to be considered successfully started. |
| `start_retries` | int | `3` | Number of start attempts before supervisord marks the process as FATAL. |
| `environment` | map | `{}` | Additional environment variables passed to the worker process (e.g., `{"QUEUE_CONNECTION": "redis"}`). |

### Go struct for parsing `runtime_config`

```go
// WorkerConfig holds the parsed runtime_config for php-worker webroots.
type WorkerConfig struct {
    Command      string            `json:"command"`
    Directory    string            `json:"directory"`
    NumProcs     int               `json:"num_procs"`
    StopSignal   string            `json:"stop_signal"`
    StopWaitSecs int               `json:"stop_wait_secs"`
    StartSecs    int               `json:"start_secs"`
    StartRetries int               `json:"start_retries"`
    Environment  map[string]string `json:"environment"`
}
```

Defaults are applied at parse time in the runtime manager, not in the database or API layer. This follows the pattern of the existing PHP runtime where `RuntimeVersion` defaults to `"8.5"` inside the manager.

### Validation of `command`

The `command` field requires careful validation to prevent arbitrary command execution:

1. **Must be non-empty**: Required field.
2. **Must start with `php`**: The first token must be `php` (or `/usr/bin/php`). This is a PHP worker runtime, not a general-purpose process runner.
3. **No shell metacharacters**: Reject commands containing `;`, `|`, `&`, `$()`, backticks, `>`, `<`. The command is passed directly to supervisord's `command=` directive, which uses `exec` (no shell), but we validate defensively.
4. **Max length**: 1024 characters.

This validation happens in the API request validation layer (not in the runtime manager) to fail fast with a clear 400 error.

### No changes to `model.Webroot`

The `model.Webroot` struct does not need new fields. Everything worker-specific lives in `RuntimeConfig` (already `json.RawMessage`). The `PublicFolder` field is irrelevant for workers and will be empty, but this is not enforced -- it simply has no effect.

### No `enabled` field

After careful consideration, the worker's enabled/disabled state is controlled by the existing `Status` field:

- `active` = worker is running (supervisord process is started).
- `suspended` = worker is stopped (supervisord process is stopped, config remains on disk for quick restart).
- `failed` = worker crashed and could not be restarted (supervisord FATAL state detected by agent).

This reuses the existing status lifecycle without adding new fields. Tenant suspension already cascades to webroot status, so suspending a tenant automatically stops all their workers.

---

## API Changes

### Request validation: `internal/api/request/webroot.go`

Add `php-worker` to the valid runtime list:

```go
type CreateWebroot struct {
    Name           string          `json:"name" validate:"required,slug"`
    Runtime        string          `json:"runtime" validate:"required,oneof=php php-worker node python ruby static"`
    RuntimeVersion string          `json:"runtime_version" validate:"required"`
    RuntimeConfig  json.RawMessage `json:"runtime_config"`
    PublicFolder   string          `json:"public_folder"`
    FQDNs          []CreateFQDNNested `json:"fqdns" validate:"omitempty,dive"`
}

type UpdateWebroot struct {
    Runtime        string          `json:"runtime" validate:"omitempty,oneof=php php-worker node python ruby static"`
    RuntimeVersion string          `json:"runtime_version"`
    RuntimeConfig  json.RawMessage `json:"runtime_config"`
    PublicFolder   *string         `json:"public_folder"`
}
```

### Additional request validation for `php-worker`

When `Runtime == "php-worker"`, the handler must validate:

1. `RuntimeConfig` is present and contains a valid `command` field.
2. `command` passes the security validation described above.
3. `num_procs` (if present) is between 1 and 8.
4. `stop_signal` (if present) is one of: `TERM`, `INT`, `QUIT`, `USR1`, `USR2`.
5. `FQDNs` is empty (workers have no HTTP interface). Reject with 400 if FQDNs are provided.

This validation should be implemented as a dedicated validation function called from the handler after parsing the request, following the existing pattern of "parse/validate request -> build model -> call service -> return response".

```go
// validateWorkerConfig validates runtime_config for php-worker webroots.
func validateWorkerConfig(config json.RawMessage) error {
    var wc WorkerConfig
    if err := json.Unmarshal(config, &wc); err != nil {
        return fmt.Errorf("invalid runtime_config: %w", err)
    }
    if wc.Command == "" {
        return fmt.Errorf("command is required for php-worker runtime")
    }
    // ... further validation
}
```

### Swagger annotations

Update the Create/Update webroot descriptions to document `php-worker` as a valid runtime and link to `runtime_config` schema.

---

## Runtime Manager: `internal/agent/runtime/phpworker.go`

### Structure

```go
package runtime

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "os"
    "os/exec"
    "path/filepath"
    "strings"
    "text/template"

    "github.com/rs/zerolog"
)
```

### Supervisord config template

```go
const workerConfigTemplate = `; Auto-generated by node-agent for {{ .TenantName }}/{{ .WebrootName }}
; DO NOT EDIT MANUALLY

[program:worker-{{ .TenantName }}-{{ .WebrootName }}{{ if gt .NumProcs 1 }}_%(process_num)02d{{ end }}]
command={{ .Command }}
directory={{ .WorkingDir }}
user={{ .TenantName }}
numprocs={{ .NumProcs }}
{{ if gt .NumProcs 1 -}}
process_name=%(program_name)s_%(process_num)02d
{{ end -}}
autostart=true
autorestart=unexpected
startsecs={{ .StartSecs }}
startretries={{ .StartRetries }}
stopsignal={{ .StopSignal }}
stopwaitsecs={{ .StopWaitSecs }}
stdout_logfile=/var/www/storage/{{ .TenantName }}/logs/worker-{{ .WebrootName }}.log
stdout_logfile_maxbytes=10MB
stdout_logfile_backups=3
stderr_logfile=/var/www/storage/{{ .TenantName }}/logs/worker-{{ .WebrootName }}.error.log
stderr_logfile_maxbytes=10MB
stderr_logfile_backups=3
{{ if .Environment -}}
environment={{ .Environment }}
{{ end -}}
`
```

Template data struct:

```go
type workerConfigData struct {
    TenantName   string
    WebrootName  string
    Command      string
    WorkingDir   string
    NumProcs     int
    StopSignal   string
    StopWaitSecs int
    StartSecs    int
    StartRetries int
    Environment  string // supervisord format: KEY="value",KEY2="value2"
}
```

### Manager implementation

```go
// PHPWorker manages PHP background worker processes via supervisord.
type PHPWorker struct {
    logger zerolog.Logger
}

// NewPHPWorker creates a new PHP worker runtime manager.
func NewPHPWorker(logger zerolog.Logger) *PHPWorker {
    return &PHPWorker{
        logger: logger.With().Str("runtime", "php-worker").Logger(),
    }
}
```

Note: `PHPWorker` does NOT take a `ServiceManager` parameter. Unlike the other runtimes that delegate to systemd via `ServiceManager`, this runtime interacts with supervisord directly via `supervisorctl`. This is because supervisord is a separate process management layer and the existing `ServiceManager` interface (DaemonReload, Start, Stop, Reload, Restart, Signal) does not map cleanly to supervisorctl operations.

### Config file path

```go
func (w *PHPWorker) configPath(webroot *WebrootInfo) string {
    return fmt.Sprintf("/etc/supervisor/conf.d/worker-%s-%s.conf",
        webroot.TenantName, webroot.Name)
}
```

### Program name

```go
func (w *PHPWorker) programName(webroot *WebrootInfo) string {
    return fmt.Sprintf("worker-%s-%s", webroot.TenantName, webroot.Name)
}
```

### Parse runtime config with defaults

```go
func (w *PHPWorker) parseConfig(webroot *WebrootInfo) (*WorkerConfig, error) {
    var cfg WorkerConfig
    if webroot.RuntimeConfig != "" {
        if err := json.Unmarshal([]byte(webroot.RuntimeConfig), &cfg); err != nil {
            return nil, fmt.Errorf("parse worker runtime_config: %w", err)
        }
    }

    // Apply defaults.
    if cfg.NumProcs <= 0 {
        cfg.NumProcs = 1
    }
    if cfg.NumProcs > 8 {
        cfg.NumProcs = 8
    }
    if cfg.StopSignal == "" {
        cfg.StopSignal = "TERM"
    }
    if cfg.StopWaitSecs <= 0 {
        cfg.StopWaitSecs = 30
    }
    if cfg.StartSecs <= 0 {
        cfg.StartSecs = 10
    }
    if cfg.StartRetries <= 0 {
        cfg.StartRetries = 3
    }

    return &cfg, nil
}
```

### Configure

Writes the supervisord program config file and tells supervisord to reread.

```go
func (w *PHPWorker) Configure(ctx context.Context, webroot *WebrootInfo) error {
    cfg, err := w.parseConfig(webroot)
    if err != nil {
        return err
    }

    // Determine working directory.
    workingDir := filepath.Join("/var/www/storage", webroot.TenantName, "webroots", webroot.Name)
    if cfg.Directory != "" {
        workingDir = filepath.Join("/var/www/storage", webroot.TenantName, "webroots", cfg.Directory)
    }

    // Format environment for supervisord.
    var envParts []string
    for k, v := range cfg.Environment {
        envParts = append(envParts, fmt.Sprintf(`%s="%s"`, k, v))
    }

    data := workerConfigData{
        TenantName:   webroot.TenantName,
        WebrootName:  webroot.Name,
        Command:      cfg.Command,
        WorkingDir:   workingDir,
        NumProcs:     cfg.NumProcs,
        StopSignal:   cfg.StopSignal,
        StopWaitSecs: cfg.StopWaitSecs,
        StartSecs:    cfg.StartSecs,
        StartRetries: cfg.StartRetries,
        Environment:  strings.Join(envParts, ","),
    }

    var buf bytes.Buffer
    if err := workerConfigTmpl.Execute(&buf, data); err != nil {
        return fmt.Errorf("render worker config template: %w", err)
    }

    configPath := w.configPath(webroot)
    w.logger.Info().
        Str("tenant", webroot.TenantName).
        Str("webroot", webroot.Name).
        Str("command", cfg.Command).
        Int("num_procs", cfg.NumProcs).
        Str("path", configPath).
        Msg("writing supervisord worker config")

    if err := os.WriteFile(configPath, buf.Bytes(), 0644); err != nil {
        return fmt.Errorf("write worker config: %w", err)
    }

    // Tell supervisord to reread config files.
    return w.supervisorctl(ctx, "reread")
}
```

### Start

```go
func (w *PHPWorker) Start(ctx context.Context, webroot *WebrootInfo) error {
    program := w.programName(webroot)
    w.logger.Info().Str("program", program).Msg("starting worker via supervisord")

    // 'update' applies any reread changes (adds new programs, removes old ones).
    if err := w.supervisorctl(ctx, "update"); err != nil {
        return fmt.Errorf("supervisorctl update: %w", err)
    }

    // Start the program (or program group if numprocs > 1).
    return w.supervisorctl(ctx, "start", program+":*")
}
```

### Stop

```go
func (w *PHPWorker) Stop(ctx context.Context, webroot *WebrootInfo) error {
    program := w.programName(webroot)
    w.logger.Info().Str("program", program).Msg("stopping worker via supervisord")
    // Ignore error -- process may not be running.
    _ = w.supervisorctl(ctx, "stop", program+":*")
    return nil
}
```

### Reload

```go
func (w *PHPWorker) Reload(ctx context.Context, webroot *WebrootInfo) error {
    program := w.programName(webroot)
    w.logger.Info().Str("program", program).Msg("reloading worker via supervisord")

    // Reread config files, then restart the program to pick up changes.
    if err := w.supervisorctl(ctx, "reread"); err != nil {
        return fmt.Errorf("supervisorctl reread: %w", err)
    }
    if err := w.supervisorctl(ctx, "update"); err != nil {
        return fmt.Errorf("supervisorctl update: %w", err)
    }
    return w.supervisorctl(ctx, "restart", program+":*")
}
```

### Remove

```go
func (w *PHPWorker) Remove(ctx context.Context, webroot *WebrootInfo) error {
    // Stop the program first.
    w.Stop(ctx, webroot)

    configPath := w.configPath(webroot)
    w.logger.Info().Str("path", configPath).Msg("removing supervisord worker config")

    if err := os.Remove(configPath); err != nil && !os.IsNotExist(err) {
        return fmt.Errorf("remove worker config: %w", err)
    }

    // Tell supervisord to pick up the removal.
    if err := w.supervisorctl(ctx, "reread"); err != nil {
        return err
    }
    return w.supervisorctl(ctx, "update")
}
```

### supervisorctl helper

```go
func (w *PHPWorker) supervisorctl(ctx context.Context, args ...string) error {
    cmd := exec.CommandContext(ctx, "supervisorctl", args...)
    output, err := cmd.CombinedOutput()
    if err != nil {
        return fmt.Errorf("supervisorctl %v: %s: %w", args, string(output), err)
    }
    return nil
}
```

---

## Registration in Agent Server

### `internal/agent/server.go`

Add `php-worker` to the runtime map:

```go
runtimes := map[string]runtime.Manager{
    "php":        runtime.NewPHP(logger, svcMgr),
    "php-worker": runtime.NewPHPWorker(logger),    // NEW
    "node":       runtime.NewNode(logger, svcMgr),
    "python":     runtime.NewPython(logger, svcMgr),
    "ruby":       runtime.NewRuby(logger, svcMgr),
    "static":     runtime.NewStatic(logger),
}
```

No other changes needed in `server.go`. The runtime map is passed through to `NodeLocal` activities, which already look up runtimes by `info.Runtime` key.

---

## Activity Layer: Nginx Bypass

### The critical change: skip nginx for `php-worker`

The `CreateWebroot` and `UpdateWebroot` activities in `internal/activity/node_local.go` currently always generate and write an nginx config. For `php-worker` webroots, this must be skipped.

#### `CreateWebroot` changes

```go
func (a *NodeLocal) CreateWebroot(ctx context.Context, params CreateWebrootParams) error {
    // ... (unchanged: build info, fqdns) ...

    // Create webroot directories.
    if err := a.webroot.Create(ctx, info); err != nil {
        return asNonRetryable(fmt.Errorf("create webroot: %w", err))
    }

    // Configure and start runtime.
    rt, ok := a.runtimes[info.Runtime]
    if !ok {
        return fmt.Errorf("unsupported runtime: %s", info.Runtime)
    }
    if err := rt.Configure(ctx, info); err != nil {
        return asNonRetryable(fmt.Errorf("configure runtime: %w", err))
    }
    if err := rt.Start(ctx, info); err != nil {
        return asNonRetryable(fmt.Errorf("start runtime: %w", err))
    }

    // Skip nginx config for worker runtimes (no HTTP traffic).
    if info.Runtime == "php-worker" {
        return nil
    }

    // Generate and write nginx config.
    nginxConfig, err := a.nginx.GenerateConfig(info, fqdns)
    // ... (unchanged) ...
}
```

The same pattern applies to `UpdateWebroot` and `DeleteWebroot`. The `DeleteWebroot` activity already iterates all runtimes and calls `Remove()` on each, which will correctly invoke `PHPWorker.Remove()` for worker webroots.

### Helper function

To avoid scattering `info.Runtime == "php-worker"` checks, introduce a helper:

```go
// isWorkerRuntime returns true if the runtime type has no HTTP interface.
func isWorkerRuntime(rt string) bool {
    return rt == "php-worker"
}
```

This is forward-looking -- if a `node-worker` runtime is added later, only this function needs updating.

---

## Convergence Workflow Changes

### `internal/workflow/converge_shard.go`

The `convergeWebShard` function builds an `expectedConfigs` map of all nginx config filenames. Worker webroots must be excluded from this map (they have no nginx config), otherwise the orphan cleaner will never find their config (which is correct) but the map will contain an entry that never matches a file (harmless but confusing). More importantly, the nginx config generation step in the webroot creation loop must skip worker webroots.

The convergence workflow dispatches `CreateWebroot` activities, which already handle the nginx bypass (see above). No further changes needed in the workflow itself, but the `expectedConfigs` map should exclude worker webroots:

```go
for _, webroot := range webroots {
    if webroot.Status != model.StatusActive {
        continue
    }

    // Only web-facing webroots have nginx configs.
    if webroot.Runtime != "php-worker" {
        confName := fmt.Sprintf("%s_%s.conf", tenant.ID, webroot.Name)
        expectedConfigs[confName] = true
    }

    // ... (rest unchanged: fetch FQDNs, build webrootEntries) ...
}
```

---

## Anti-Thrash Protection

### How supervisord handles it

Supervisord provides built-in protection against rapidly-crashing processes:

1. **`startsecs=10`**: A process must run for at least 10 seconds to be considered "successfully started". If it exits before 10 seconds, supervisord counts it as a failed start attempt.
2. **`startretries=3`**: After 3 consecutive failed start attempts, supervisord transitions the process to `FATAL` state and stops trying.
3. **`autorestart=unexpected`**: Only restarts on unexpected exit codes (non-zero). If the process exits cleanly (exit code 0), it is not restarted. This prevents infinite restart loops for processes that crash immediately.

Combined, this means: if a worker process crashes within 10 seconds of starting, supervisord will retry 3 times, then give up and mark it as FATAL. No infinite restart loops.

### Agent detection of FATAL state

The node-agent should detect FATAL workers during convergence and report them:

```go
// CheckWorkerStatus checks supervisord for FATAL worker processes
// and returns a list of program names in FATAL state.
func (w *PHPWorker) CheckStatus(ctx context.Context, webroot *WebrootInfo) (string, error) {
    program := w.programName(webroot)
    cmd := exec.CommandContext(ctx, "supervisorctl", "status", program+":*")
    output, _ := cmd.CombinedOutput()
    // Parse output: "worker-tenant-webroot:worker-tenant-webroot_00 FATAL ..."
    // Return the state: "RUNNING", "STOPPED", "FATAL", "STARTING", etc.
    return parseSupervisdorState(string(output)), nil
}
```

When convergence detects a FATAL worker, it should:

1. Set the webroot's status to `failed` with `status_message` describing the FATAL state.
2. NOT attempt to restart -- the process has already exhausted its retries. Human intervention is needed (fix the command, fix the code, etc.).
3. Log the event as a drift event.

When the tenant fixes the issue and triggers a retry (`POST /webroots/{id}/retry`), the workflow will reconfigure supervisord (removing the old FATAL config and writing a fresh one) and start the process again.

---

## Logging

### Supervisord log configuration

Each worker program gets two log files:

- **stdout**: `/var/www/storage/{tenant}/logs/worker-{webroot}.log`
- **stderr**: `/var/www/storage/{tenant}/logs/worker-{webroot}.error.log`

These are in the same directory as other runtime logs (PHP error log, nginx access/error logs), making them discoverable alongside existing logs.

### Log rotation

Supervisord handles rotation natively:

```ini
stdout_logfile_maxbytes=10MB
stdout_logfile_backups=3
stderr_logfile_maxbytes=10MB
stderr_logfile_backups=3
```

This keeps at most 40MB of logs per worker (10MB * 4 files for stdout, same for stderr). The rotated files are named `worker-{webroot}.log.1`, `worker-{webroot}.log.2`, etc.

### Log access

Worker logs are accessible to tenants via the same mechanisms as other logs:

1. **SFTP/SSH**: Tenants can read `/var/www/storage/{tenant}/logs/worker-*.log` directly.
2. **Future log API**: When a log access API is implemented, worker logs will be included alongside PHP-FPM and nginx logs.

### Vector integration

The existing Vector configuration on web nodes (`packer/files/vector-web.toml`) collects logs from `/var/www/storage/*/logs/*.log`. Worker logs in the same directory will be automatically picked up by Vector for centralized logging. No Vector configuration changes needed.

---

## Monitoring

### Prometheus metrics

Expose worker status via the node-agent's `/metrics` endpoint:

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `hosting_worker_processes_total` | Gauge | `tenant`, `webroot`, `state` | Number of worker processes by state (RUNNING, STOPPED, FATAL, STARTING) |
| `hosting_worker_uptime_seconds` | Gauge | `tenant`, `webroot` | Uptime of the worker process (0 if not running) |

Collection method: parse `supervisorctl status` output, which returns one line per process with state and uptime.

### Health check integration

The node health report should include a `supervisor` section:

```json
{
  "supervisor": {
    "status": "healthy",
    "programs_running": 12,
    "programs_expected": 12,
    "programs_fatal": 0
  }
}
```

If any programs are in FATAL state, the supervisor health status should be `degraded` (not `unhealthy`, since the supervisor daemon itself is still running -- individual program failures do not indicate a system-level problem).

---

## Security Considerations

### Command injection prevention

The `command` field is written directly into the supervisord config file's `command=` directive. Supervisord does NOT pass this through a shell -- it uses `execve()` directly, splitting on whitespace. This means shell metacharacters (`;`, `|`, `&`, etc.) will NOT cause command injection in supervisord itself.

However, we still validate the command defensively at the API layer (see Validation section above) because:

1. The command could be `php -r 'system("rm -rf /")'` -- valid PHP, catastrophic.
2. We cannot prevent all malicious PHP code, but we can ensure the command is at least plausibly a PHP worker command.
3. The tenant user's Linux permissions (no sudo, restricted `open_basedir` in php.ini) provide the real security boundary. The command validation is defense-in-depth.

### Tenant isolation

Worker processes run as the tenant's Linux user, inheriting the same restrictions as PHP-FPM pools:

- **User/group isolation**: `user={tenantName}` in supervisord config.
- **Filesystem access**: Limited to `/var/www/storage/{tenant}/` and `/tmp/` by Linux permissions and `open_basedir`.
- **No privilege escalation**: The tenant user has no sudo access, no crontab, and `/proc` is mounted with `hidepid=2`.
- **Resource limits**: Not yet implemented but planned. supervisord supports `environment=` to set PHP memory limits, and future work could add cgroup limits via systemd slice membership.

### PHP version enforcement

The worker command must use the system PHP binary corresponding to `runtime_version`. The command validation should ensure the `php` binary resolves to the correct version. For PHP 8.3, the command should be `php8.3` explicitly, or the `PATH` should be set in the supervisord environment to prefer the correct version.

Recommendation: the runtime manager should prepend the versioned PHP binary path. If the user specifies `command: "php artisan queue:work"`, the manager rewrites it to `/usr/bin/php8.3 artisan queue:work` based on `RuntimeVersion`. This ensures version consistency without burdening the user.

```go
// resolveCommand prefixes the command with the versioned PHP binary.
func resolveCommand(command, version string) string {
    if version == "" {
        version = "8.5"
    }
    phpBin := fmt.Sprintf("/usr/bin/php%s", version)
    // Replace leading "php " with the versioned binary.
    if strings.HasPrefix(command, "php ") {
        return phpBin + command[3:]
    }
    // If command starts with an absolute path to php, leave it.
    if strings.HasPrefix(command, "/usr/bin/php") {
        return command
    }
    // Otherwise prepend.
    return phpBin + " " + command
}
```

---

## Webroot Lifecycle Interactions

### Creating a `php-worker` webroot

1. API handler validates request: `runtime=php-worker`, `runtime_config` has `command`, no FQDNs.
2. Core service inserts webroot, signals provisioning workflow.
3. `CreateWebrootWorkflow` runs on each node:
   a. Creates webroot directories on CephFS.
   b. Calls `PHPWorker.Configure()` -- writes supervisord config.
   c. Calls `PHPWorker.Start()` -- runs `supervisorctl update` + `supervisorctl start`.
   d. **Skips** nginx config generation and reload.
4. Sets webroot status to `active`.

### Updating a `php-worker` webroot

1. API handler validates updated `runtime_config`.
2. `UpdateWebrootWorkflow` runs on each node:
   a. Calls `PHPWorker.Configure()` -- rewrites supervisord config.
   b. Calls `PHPWorker.Reload()` -- reread + update + restart.
   c. **Skips** nginx config generation and reload.
3. Sets webroot status to `active`.

### Deleting a `php-worker` webroot

1. `DeleteWebrootWorkflow` runs on each node:
   a. **Skips** nginx config removal (no config exists).
   b. Calls `PHPWorker.Remove()` -- stops process, removes supervisord config, runs reread + update.
   c. Removes webroot directories.
2. Sets webroot status to `deleted`.

### Switching runtime from `php` to `php-worker` (or vice versa)

This is a destructive operation: the old runtime must be fully removed and the new one configured. The `UpdateWebroot` activity already handles this by:

1. Calling the old runtime's `Remove()` (implicitly -- all runtimes are tried in `DeleteWebroot`).
2. Calling the new runtime's `Configure()` + `Start()`.

However, the current `UpdateWebroot` activity does NOT remove old runtimes -- it only calls `Configure()` + `Reload()` on the current runtime. A runtime switch requires additional logic:

```go
// In UpdateWebroot, detect runtime change and handle cleanup.
// This is a future enhancement -- for now, runtime changes require
// delete + recreate of the webroot. Document this limitation.
```

**Recommendation for initial implementation**: Do not support in-place runtime switching. If a user wants to change a webroot from `php` to `php-worker`, they must delete the webroot and create a new one. This is documented in the API. In-place runtime switching can be added as a future enhancement once the basic worker runtime is proven.

---

## Test Plan

### Unit tests: `internal/agent/runtime/phpworker_test.go`

Following the pattern of `php_test.go`, `node_test.go`, etc.:

1. **TestPHPWorker_Configure**: Verify that the correct supervisord config file is written with expected content (command, user, directory, numprocs, log paths).
2. **TestPHPWorker_Configure_Defaults**: Verify default values are applied when `runtime_config` is minimal (`{"command": "php artisan queue:work"}`).
3. **TestPHPWorker_Configure_CustomDirectory**: Verify that a custom `directory` resolves to the correct path under the tenant's storage.
4. **TestPHPWorker_Configure_Environment**: Verify environment variables are formatted correctly in supervisord syntax.
5. **TestPHPWorker_Configure_NumProcs**: Verify that `numprocs > 1` adds `process_name` with `%(process_num)` and appends `_%(process_num)02d` to the program name.
6. **TestPHPWorker_Remove**: Verify config file is removed.
7. **TestPHPWorker_ConfigPath**: Verify the config file path follows the expected pattern.
8. **TestResolveCommand**: Verify PHP version is correctly prepended to commands.

### Integration tests (requires running infrastructure)

1. **Create worker webroot**: Verify supervisord process starts, webroot has no nginx config.
2. **Update worker config**: Change `num_procs`, verify supervisord restarts with new count.
3. **Delete worker webroot**: Verify supervisord process stops, config is removed.
4. **Anti-thrash**: Create a worker with an invalid command, verify it reaches FATAL state and the agent reports `failed` status.
5. **Convergence**: Trigger shard convergence, verify worker webroots are correctly provisioned alongside web webroots, and no spurious nginx configs are created for workers.

### E2E tests

Add to `tests/e2e/`:

1. Create a tenant.
2. Create a `php-worker` webroot with `command: "php -r 'while(true) { sleep(1); echo time().PHP_EOL; }'"`.
3. Verify webroot reaches `active` status.
4. Verify `supervisorctl status` shows the worker RUNNING.
5. Verify no nginx server block exists for the worker webroot.
6. Update `num_procs` to 2, verify two processes appear.
7. Delete the webroot, verify the process stops.

---

## Rollout Plan

### Phase 1: Infrastructure (Packer + supervisord)

1. Add `supervisor` to `packer/scripts/web.sh`.
2. Add base supervisord config via Packer.
3. Rebuild web node images.
4. Verify supervisord is running on all web nodes.

### Phase 2: Runtime Manager

1. Implement `internal/agent/runtime/phpworker.go`.
2. Register in `internal/agent/server.go`.
3. Write unit tests.
4. Verify: agent compiles and existing tests pass.

### Phase 3: API + Validation

1. Add `php-worker` to request validation (`internal/api/request/webroot.go`).
2. Add `runtime_config` validation for `php-worker` in handler.
3. Update Swagger annotations.
4. Verify: API accepts and rejects appropriate requests.

### Phase 4: Activity Layer (Nginx Bypass)

1. Add `isWorkerRuntime()` helper.
2. Modify `CreateWebroot`, `UpdateWebroot`, `DeleteWebroot` activities to skip nginx for worker runtimes.
3. Modify `convergeWebShard` to exclude worker webroots from `expectedConfigs`.
4. Run existing webroot tests to ensure no regressions.

### Phase 5: Testing + E2E

1. Run integration tests on a dev cluster.
2. Add E2E tests.
3. Verify full lifecycle: create, update, delete, anti-thrash, convergence.

### Phase 6: Monitoring

1. Add Prometheus metrics for worker processes.
2. Add FATAL state detection during convergence and status reporting.
3. Add supervisor health check to node health report.

---

## Files Changed (Summary)

| File | Change |
|------|--------|
| `packer/scripts/web.sh` | Add `supervisor` to apt packages |
| `packer/files/supervisor-hosting.conf` | **New** -- base supervisord config |
| `internal/agent/runtime/phpworker.go` | **New** -- PHPWorker runtime manager |
| `internal/agent/runtime/phpworker_test.go` | **New** -- unit tests |
| `internal/agent/server.go` | Register `php-worker` in runtime map |
| `internal/api/request/webroot.go` | Add `php-worker` to `oneof` validator |
| `internal/api/handler/webroot.go` | Add `runtime_config` validation for `php-worker` |
| `internal/activity/node_local.go` | Skip nginx config for worker runtimes |
| `internal/workflow/converge_shard.go` | Exclude worker webroots from `expectedConfigs` |
| `internal/api/docs/*` | Regenerated Swagger docs |

---

## Open Questions

1. **Should workers share a webroot directory with a sibling web webroot, or should they always have their own?** The current design supports both via the `directory` config field. A worker can point to any webroot under the same tenant. This is the most flexible approach but introduces a coupling between webroots. Alternative: require workers to be created as part of the same webroot (a webroot can have both a PHP-FPM runtime and a worker process). This would be a bigger design change and is deferred.

2. **Resource limits (CPU, memory)?** Supervisord does not natively support cgroups. Options: (a) set PHP `memory_limit` via the environment, (b) wrap the command in `systemd-run --scope --user` to get cgroup limits, (c) defer to future cgroup-based tenant isolation. Recommendation: start with PHP `memory_limit` in the environment and defer cgroup limits.

3. **Should `num_procs` max be higher than 8?** For now, 8 is sufficient for most Laravel queue worker scenarios. This can be increased later if needed. The limit exists primarily to prevent accidental resource exhaustion on shared web nodes.

4. **Graceful restart during deployments?** When a tenant deploys new code, they need to restart their workers to pick up the changes. This could be exposed as a dedicated `POST /webroots/{id}/restart` endpoint, or the existing `PUT /webroots/{id}` (update) could trigger a restart. Recommendation: add a `POST /webroots/{id}/restart` endpoint as a follow-up feature.

5. **Multiple worker types per application?** A Laravel app might need separate workers for different queues (default, high-priority, emails). The current design handles this by creating multiple `php-worker` webroots, each with a different command. This is slightly verbose but explicit and manageable. Alternative: support multiple commands per webroot in `runtime_config` as an array. Deferred for simplicity.
