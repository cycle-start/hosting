package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/rs/zerolog"

	"github.com/edvin/hosting/internal/model"
)

// HealthReporter collects and reports node health.
type HealthReporter struct {
	logger   zerolog.Logger
	nodeID   string
	nodeRole string
	client   *APIClient
	server   *Server
	interval time.Duration

	healthStatus prometheus.Gauge
	reportTotal  *prometheus.CounterVec

	// Last reconciliation result (set by Reconciler).
	lastReconcileTime   time.Time
	lastReconcileResult string
	driftDetected       int
	driftFixed          int
	driftUnfixed        int
}

// NewHealthReporter creates a new health reporter.
func NewHealthReporter(
	logger zerolog.Logger,
	nodeID, nodeRole string,
	client *APIClient,
	server *Server,
	interval time.Duration,
) *HealthReporter {
	return &HealthReporter{
		logger:   logger.With().Str("component", "health-reporter").Logger(),
		nodeID:   nodeID,
		nodeRole: nodeRole,
		client:   client,
		server:   server,
		interval: interval,
		healthStatus: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "node_agent_health_status",
			Help: "Current health status (1=healthy, 0.5=degraded, 0=unhealthy)",
		}),
		reportTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "node_agent_health_report_total",
			Help: "Total health reports sent",
		}, []string{"result"}),
	}
}

// SetReconcileResult records the latest reconciliation outcome for health reporting.
func (h *HealthReporter) SetReconcileResult(t time.Time, result string, detected, fixed, unfixed int) {
	h.lastReconcileTime = t
	h.lastReconcileResult = result
	h.driftDetected = detected
	h.driftFixed = fixed
	h.driftUnfixed = unfixed
}

// RunLoop runs the periodic health reporting loop.
func (h *HealthReporter) RunLoop(ctx context.Context) {
	ticker := time.NewTicker(h.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			h.report(ctx)
		}
	}
}

func (h *HealthReporter) report(ctx context.Context) {
	checks := h.collectChecks(ctx)

	// Derive overall status.
	status := "healthy"
	for _, v := range checks {
		checkMap, ok := v.(map[string]any)
		if !ok {
			continue
		}
		if s, ok := checkMap["status"].(string); ok {
			if s == "unhealthy" {
				status = "unhealthy"
				break
			}
			if s == "degraded" && status == "healthy" {
				status = "degraded"
			}
		}
	}

	switch status {
	case "healthy":
		h.healthStatus.Set(1)
	case "degraded":
		h.healthStatus.Set(0.5)
	default:
		h.healthStatus.Set(0)
	}

	checksJSON, _ := json.Marshal(checks)

	reconciliation := map[string]any{
		"last_run":       h.lastReconcileTime,
		"last_result":    h.lastReconcileResult,
		"drift_detected": h.driftDetected,
		"drift_fixed":    h.driftFixed,
		"drift_unfixed":  h.driftUnfixed,
	}
	reconciliationJSON, _ := json.Marshal(reconciliation)

	health := &model.NodeHealth{
		NodeID:         h.nodeID,
		Status:         status,
		Checks:         checksJSON,
		Reconciliation: reconciliationJSON,
		ReportedAt:     time.Now(),
	}

	if err := h.client.ReportHealth(ctx, h.nodeID, health); err != nil {
		h.reportTotal.WithLabelValues("failure").Inc()
		h.logger.Warn().Err(err).Msg("failed to report health")
	} else {
		h.reportTotal.WithLabelValues("success").Inc()
		h.logger.Debug().Str("status", status).Msg("health reported")
	}
}

func (h *HealthReporter) collectChecks(ctx context.Context) map[string]any {
	checks := make(map[string]any)

	// Disk check.
	checks["disk"] = h.checkDisk()

	// Memory check.
	checks["memory"] = h.checkMemory()

	// Load check.
	checks["load"] = h.checkLoad()

	// Role-specific checks.
	switch h.nodeRole {
	case "web":
		checks["nginx"] = h.checkNginx(ctx)
	case "database":
		checks["mysql"] = h.checkMySQL(ctx)
	case "valkey":
		checks["valkey"] = h.checkValkey(ctx)
	case "lb":
		checks["haproxy"] = h.checkHAProxy()
	}

	return checks
}

func (h *HealthReporter) checkDisk() map[string]any {
	var stat syscall.Statfs_t
	if err := syscall.Statfs("/", &stat); err != nil {
		return map[string]any{"status": "unhealthy", "error": err.Error()}
	}
	total := stat.Blocks * uint64(stat.Bsize)
	free := stat.Bavail * uint64(stat.Bsize)
	used := total - free
	pct := float64(used) / float64(total) * 100

	status := "healthy"
	if pct > 90 {
		status = "unhealthy"
	} else if pct > 80 {
		status = "degraded"
	}

	return map[string]any{
		"status":        status,
		"total_bytes":   total,
		"used_bytes":    used,
		"usage_percent": fmt.Sprintf("%.1f", pct),
	}
}

func (h *HealthReporter) checkMemory() map[string]any {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return map[string]any{"status": "unhealthy", "error": err.Error()}
	}

	var memTotal, memAvailable uint64
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "MemTotal:") {
			fmt.Sscanf(line, "MemTotal: %d kB", &memTotal)
			memTotal *= 1024
		}
		if strings.HasPrefix(line, "MemAvailable:") {
			fmt.Sscanf(line, "MemAvailable: %d kB", &memAvailable)
			memAvailable *= 1024
		}
	}

	if memTotal == 0 {
		return map[string]any{"status": "unhealthy", "error": "could not parse MemTotal"}
	}

	used := memTotal - memAvailable
	pct := float64(used) / float64(memTotal) * 100

	status := "healthy"
	if pct > 97 {
		status = "unhealthy"
	} else if pct > 90 {
		status = "degraded"
	}

	return map[string]any{
		"status":        status,
		"total_bytes":   memTotal,
		"used_bytes":    used,
		"usage_percent": fmt.Sprintf("%.1f", pct),
	}
}

func (h *HealthReporter) checkLoad() map[string]any {
	data, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return map[string]any{"error": err.Error()}
	}
	var load1, load5, load15 float64
	fmt.Sscanf(string(data), "%f %f %f", &load1, &load5, &load15)
	return map[string]any{
		"load_1m":  load1,
		"load_5m":  load5,
		"load_15m": load15,
	}
}

func (h *HealthReporter) checkNginx(ctx context.Context) map[string]any {
	cmd := exec.CommandContext(ctx, "nginx", "-t")
	if err := cmd.Run(); err != nil {
		return map[string]any{"status": "unhealthy", "config_test": "failed", "error": err.Error()}
	}

	// Check if nginx process is running.
	pidData, err := os.ReadFile("/run/nginx.pid")
	if err != nil {
		return map[string]any{"status": "unhealthy", "config_test": "ok", "error": "nginx not running"}
	}

	return map[string]any{
		"status":      "healthy",
		"config_test": "ok",
		"pid":         strings.TrimSpace(string(pidData)),
	}
}

func (h *HealthReporter) checkMySQL(ctx context.Context) map[string]any {
	cmd := exec.CommandContext(ctx, "mysqladmin", "ping", "--silent")
	if err := cmd.Run(); err != nil {
		return map[string]any{"status": "unhealthy", "error": err.Error()}
	}
	return map[string]any{"status": "healthy"}
}

func (h *HealthReporter) checkValkey(ctx context.Context) map[string]any {
	cmd := exec.CommandContext(ctx, "valkey-cli", "PING")
	output, err := cmd.Output()
	if err != nil {
		return map[string]any{"status": "unhealthy", "error": err.Error()}
	}
	if strings.TrimSpace(string(output)) != "PONG" {
		return map[string]any{"status": "degraded", "response": string(output)}
	}
	return map[string]any{"status": "healthy"}
}

func (h *HealthReporter) checkHAProxy() map[string]any {
	conn, err := net.DialTimeout("tcp", "127.0.0.1:9999", 2*time.Second)
	if err != nil {
		return map[string]any{"status": "unhealthy", "error": err.Error()}
	}
	conn.Close()
	return map[string]any{"status": "healthy", "runtime_api": "reachable"}
}
