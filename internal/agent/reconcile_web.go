package agent

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/edvin/hosting/internal/agent/runtime"
	"github.com/edvin/hosting/internal/model"
)

func (r *Reconciler) reconcileWeb(ctx context.Context, ds *model.DesiredState) ([]DriftEvent, error) {
	var events []DriftEvent
	fixes := 0

	// Build desired state maps.
	desiredTenants := make(map[string]model.DesiredTenant)
	for _, t := range ds.Tenants {
		desiredTenants[t.Name] = t
	}

	// Build the expected nginx config set for orphan detection.
	expectedConfigs := make(map[string]bool)

	// Check each desired tenant.
	for _, t := range ds.Tenants {
		if fixes >= r.maxFixes {
			events = append(events, DriftEvent{
				Timestamp: time.Now(), NodeID: r.nodeID, Kind: "reconciler",
				Resource: "max_fixes", Action: "skipped",
				Detail: fmt.Sprintf("reached max fixes limit (%d)", r.maxFixes),
			})
			break
		}

		// Check Linux user exists.
		if err := exec.CommandContext(ctx, "id", t.Name).Run(); err != nil {
			if r.circuitOpen {
				events = append(events, DriftEvent{
					Timestamp: time.Now(), NodeID: r.nodeID, Kind: "tenant_user",
					Resource: t.Name, Action: "reported",
					Detail: "missing Linux user (circuit breaker open)",
				})
			} else {
				unlock := r.LockResource("tenant", t.Name, "")
				createErr := r.server.TenantManager().Create(ctx, &TenantInfo{
					ID: t.ID, Name: t.Name, UID: t.UID,
					SFTPEnabled: t.SFTPEnabled, SSHEnabled: t.SSHEnabled,
				})
				unlock()
				if createErr != nil {
					events = append(events, DriftEvent{
						Timestamp: time.Now(), NodeID: r.nodeID, Kind: "tenant_user",
						Resource: t.Name, Action: "reported",
						Detail: fmt.Sprintf("failed to create user: %v", createErr),
					})
				} else {
					events = append(events, DriftEvent{
						Timestamp: time.Now(), NodeID: r.nodeID, Kind: "tenant_user",
						Resource: t.Name, Action: "auto_fixed",
						Detail: "recreated missing Linux user",
					})
					fixes++
				}
			}
		}

		// Check webroot directories and nginx configs.
		for _, wr := range t.Webroots {
			wrDir := filepath.Join(r.server.TenantManager().WebStorageDir(), t.Name, "webroots", wr.Name)
			configName := fmt.Sprintf("%s_%s.conf", t.Name, wr.Name)
			expectedConfigs[configName] = true
			nginxConfigDir := filepath.Join(r.server.nginx.configDir, "sites-enabled")

			// Check webroot directory exists.
			if _, err := os.Stat(wrDir); os.IsNotExist(err) {
				if !r.circuitOpen && fixes < r.maxFixes {
					unlock := r.LockResource("webroot", t.Name, wr.Name)
					createErr := r.server.WebrootManager().Create(ctx, &runtime.WebrootInfo{
						ID:           wr.ID,
						TenantName:   t.Name,
						Name:         wr.Name,
						Runtime:      wr.Runtime,
						PublicFolder: wr.PublicFolder,
					})
					unlock()
					action := "auto_fixed"
					detail := "recreated missing webroot directory"
					if createErr != nil {
						action = "reported"
						detail = fmt.Sprintf("failed to create webroot dir: %v", createErr)
					} else {
						fixes++
					}
					events = append(events, DriftEvent{
						Timestamp: time.Now(), NodeID: r.nodeID, Kind: "webroot_dir",
						Resource: t.Name + "/" + wr.Name, Action: action, Detail: detail,
					})
				}
			}

			// Check nginx config exists.
			configPath := filepath.Join(nginxConfigDir, configName)
			if _, err := os.Stat(configPath); os.IsNotExist(err) {
				if !r.circuitOpen && fixes < r.maxFixes {
					unlock := r.LockResource("nginx", t.Name, wr.Name)
					regenerateErr := r.regenerateNginxConfig(ctx, t, wr)
					unlock()
					action := "auto_fixed"
					detail := "regenerated missing nginx config"
					if regenerateErr != nil {
						action = "reported"
						detail = fmt.Sprintf("failed to regenerate nginx config: %v", regenerateErr)
					} else {
						fixes++
					}
					events = append(events, DriftEvent{
						Timestamp: time.Now(), NodeID: r.nodeID, Kind: "nginx_config",
						Resource: t.Name + "/" + wr.Name, Action: action, Detail: detail,
					})
				}
			}

			// Check runtime process via socket/pid file existence.
			// The runtime.Manager interface does not have an IsRunning method,
			// so we check for the existence of the expected socket file as a proxy.
			if wr.Runtime != "" && wr.Runtime != "static" {
				if socketMissing := r.checkRuntimeSocket(t.Name, wr); socketMissing {
					if !r.circuitOpen && fixes < r.maxFixes {
						rtMgr, ok := r.server.Runtimes()[wr.Runtime]
						if ok {
							wrInfo := &runtime.WebrootInfo{
								ID:             wr.ID,
								TenantName:     t.Name,
								Name:           wr.Name,
								Runtime:        wr.Runtime,
								RuntimeVersion: wr.RuntimeVersion,
								RuntimeConfig:  wr.RuntimeConfig,
								PublicFolder:   wr.PublicFolder,
							}
							unlock := r.LockResource("runtime", t.Name, wr.Name)
							// Configure and start the runtime.
							configErr := rtMgr.Configure(ctx, wrInfo)
							if configErr == nil {
								configErr = rtMgr.Start(ctx, wrInfo)
							}
							unlock()
							action := "auto_fixed"
							detail := fmt.Sprintf("restarted %s %s runtime", wr.Runtime, wr.RuntimeVersion)
							if configErr != nil {
								action = "reported"
								detail = fmt.Sprintf("failed to restart runtime: %v", configErr)
							} else {
								fixes++
							}
							events = append(events, DriftEvent{
								Timestamp: time.Now(), NodeID: r.nodeID, Kind: "runtime",
								Resource: t.Name + "/" + wr.Name, Action: action, Detail: detail,
							})
						}
					}
				}
			}
		}
	}

	// Detect and remove orphaned nginx configs using the existing manager method.
	if !r.circuitOpen {
		removed, err := r.server.NginxManager().CleanOrphanedConfigs(expectedConfigs)
		if err != nil {
			r.logger.Warn().Err(err).Msg("failed to clean orphaned nginx configs")
		}
		for _, name := range removed {
			events = append(events, DriftEvent{
				Timestamp: time.Now(), NodeID: r.nodeID, Kind: "nginx_config",
				Resource: name, Action: "auto_fixed",
				Detail: "removed orphaned nginx config",
			})
			fixes++
		}
	}

	// Reload nginx if we made any fixes.
	if fixes > 0 {
		if err := r.server.NginxManager().Reload(ctx); err != nil {
			r.logger.Warn().Err(err).Msg("nginx reload after reconciliation failed")
		}
	}

	return events, nil
}

// regenerateNginxConfig generates and writes an nginx config for a webroot.
func (r *Reconciler) regenerateNginxConfig(ctx context.Context, t model.DesiredTenant, wr model.DesiredWebroot) error {
	wrInfo := &runtime.WebrootInfo{
		ID:             wr.ID,
		TenantName:     t.Name,
		Name:           wr.Name,
		Runtime:        wr.Runtime,
		RuntimeVersion: wr.RuntimeVersion,
		RuntimeConfig:  wr.RuntimeConfig,
		PublicFolder:   wr.PublicFolder,
	}

	fqdnInfos := r.buildFQDNInfos(wr)

	config, err := r.server.NginxManager().GenerateConfig(wrInfo, fqdnInfos)
	if err != nil {
		return fmt.Errorf("generate nginx config: %w", err)
	}

	if err := r.server.NginxManager().WriteConfig(t.Name, wr.Name, config); err != nil {
		return fmt.Errorf("write nginx config: %w", err)
	}

	return nil
}

// buildFQDNInfos converts desired FQDNs to the agent FQDNInfo slice.
func (r *Reconciler) buildFQDNInfos(wr model.DesiredWebroot) []*FQDNInfo {
	var infos []*FQDNInfo
	for _, f := range wr.FQDNs {
		infos = append(infos, &FQDNInfo{
			FQDN:       f.FQDN,
			WebrootID:  wr.ID,
			SSLEnabled: f.SSLEnabled,
		})
	}
	return infos
}

// checkRuntimeSocket checks whether the expected socket/pid file for a runtime
// exists on disk. Returns true if the socket is missing (indicating the runtime
// is not running).
func (r *Reconciler) checkRuntimeSocket(tenantName string, wr model.DesiredWebroot) bool {
	version := wr.RuntimeVersion
	if version == "" {
		version = "8.5" // PHP default
	}

	var socketPath string
	switch wr.Runtime {
	case "php":
		socketPath = fmt.Sprintf("/run/php/%s-php%s.sock", tenantName, version)
	case "python":
		socketPath = fmt.Sprintf("/run/gunicorn/%s-%s.sock", tenantName, wr.Name)
	case "ruby":
		socketPath = fmt.Sprintf("/run/puma/%s-%s.sock", tenantName, wr.Name)
	case "node":
		// Node.js uses TCP ports, not sockets -- skip socket check.
		return false
	default:
		return false
	}

	_, err := os.Stat(socketPath)
	return os.IsNotExist(err)
}
