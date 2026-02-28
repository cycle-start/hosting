package setup

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"math"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Server is the setup wizard HTTP server.
type Server struct {
	mu        sync.Mutex
	config    *Config
	outputDir string
	staticFS  fs.FS // embedded frontend assets
}

// NewServer creates a new setup wizard server.
// If an existing setup.yaml is found in outputDir, it is loaded as the initial config.
func NewServer(outputDir string, staticFS fs.FS) *Server {
	cfg := DefaultConfig()

	// Try loading existing manifest
	manifestPath := filepath.Join(outputDir, ManifestFilename)
	if existing, err := LoadManifest(manifestPath); err == nil {
		cfg = existing
	}

	return &Server{
		config:    cfg,
		outputDir: outputDir,
		staticFS:  staticFS,
	}
}

// Handler returns the HTTP handler for the setup wizard.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	// API routes
	mux.HandleFunc("GET /api/config", s.handleGetConfig)
	mux.HandleFunc("PUT /api/config", s.handlePutConfig)
	mux.HandleFunc("POST /api/validate", s.handleValidate)
	mux.HandleFunc("POST /api/generate", s.handleGenerate)
	mux.HandleFunc("GET /api/roles", s.handleGetRoles)
	mux.HandleFunc("GET /api/info", s.handleGetInfo)
	mux.HandleFunc("GET /api/steps", s.handleGetSteps)
	mux.HandleFunc("POST /api/execute", s.handleExecute)
	mux.HandleFunc("GET /api/pods", s.handleGetPods)
	mux.HandleFunc("GET /api/pod-debug", s.handlePodDebug)

	// SPA: serve static files, fall back to index.html
	mux.Handle("/", s.spaHandler())

	return mux
}

func (s *Server) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	writeJSON(w, http.StatusOK, s.config)
}

func (s *Server) handlePutConfig(w http.ResponseWriter, r *http.Request) {
	var cfg Config
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid JSON: " + err.Error()})
		return
	}

	s.mu.Lock()
	s.config = &cfg
	s.mu.Unlock()

	writeJSON(w, http.StatusOK, cfg)
}

func (s *Server) handleValidate(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	cfg := s.config
	s.mu.Unlock()

	errs := Validate(cfg)
	writeJSON(w, http.StatusOK, map[string]any{
		"valid":  len(errs) == 0,
		"errors": errs,
	})
}

func (s *Server) handleGenerate(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	cfg := s.config
	s.mu.Unlock()

	errs := Validate(cfg)
	if len(errs) > 0 {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{
			"valid":  false,
			"errors": errs,
		})
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Streaming not supported"})
		return
	}

	w.Header().Set("Content-Type", "application/x-ndjson")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Content-Type-Options", "nosniff")

	progress := func(msg string) {
		writeEvent(w, flusher, ExecEvent{Type: "output", Data: msg, Stream: "stdout"})
	}

	result, err := Generate(cfg, s.outputDir, progress)
	if err != nil {
		writeEvent(w, flusher, ExecEvent{Type: "error", Data: err.Error()})
		return
	}

	// Send the result as a special "result" event with the JSON payload.
	resultJSON, _ := json.Marshal(result)
	writeEvent(w, flusher, ExecEvent{Type: "done", Data: string(resultJSON)})
}

func (s *Server) handleGetRoles(w http.ResponseWriter, r *http.Request) {
	type roleInfo struct {
		ID          NodeRole `json:"id"`
		Label       string   `json:"label"`
		Description string   `json:"description"`
	}

	roles := []roleInfo{
		{RoleControlPlane, "Control Plane", "API server, Temporal, admin UI, PostgreSQL"},
		{RoleWeb, "Web", "Nginx, PHP/Node runtimes, web hosting"},
		{RoleDatabase, "Database", "MySQL/MariaDB for tenant databases"},
		{RoleDNS, "DNS", "PowerDNS authoritative nameserver"},
		{RoleValkey, "Valkey", "Valkey (Redis-compatible) key-value store"},
		{RoleEmail, "Email", "Stalwart mail server (IMAP/SMTP)"},
		{RoleStorage, "Storage", "Ceph object/file storage (S3 + CephFS)"},
		{RoleLB, "Load Balancer", "HAProxy reverse proxy and TLS termination"},
		{RoleGateway, "Gateway", "WireGuard VPN gateway for tenant access"},
		{RoleDBAdmin, "DB Admin", "phpMyAdmin database management UI"},
	}

	writeJSON(w, http.StatusOK, roles)
}

func (s *Server) handleGetInfo(w http.ResponseWriter, r *http.Request) {
	absDir, err := filepath.Abs(s.outputDir)
	if err != nil {
		absDir = s.outputDir
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"output_dir": absDir,
	})
}

func (s *Server) handleGetSteps(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	cfg := *s.config
	s.mu.Unlock()

	all := AllSteps()
	steps := make([]StepDef, 0, len(all))
	for _, step := range all {
		if step.MultiOnly && cfg.DeployMode != DeployModeMulti {
			continue
		}
		step.Command = FormatCommand(step.ID, &cfg, s.outputDir)
		steps = append(steps, step)
	}
	writeJSON(w, http.StatusOK, steps)
}

func (s *Server) handleExecute(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Step StepID `json:"step"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid JSON: " + err.Error()})
		return
	}

	if !validStepIDs()[req.Step] {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Unknown step: " + string(req.Step)})
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Streaming not supported"})
		return
	}

	w.Header().Set("Content-Type", "application/x-ndjson")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Content-Type-Options", "nosniff")

	s.executeStep(w, r, flusher, req.Step)
}

func (s *Server) spaHandler() http.Handler {
	fileServer := http.FileServer(http.FS(s.staticFS))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Try to serve the exact file
		path := r.URL.Path
		if path == "/" {
			path = "/index.html"
		}

		// Check if file exists
		f, err := s.staticFS.Open(strings.TrimPrefix(path, "/"))
		if err == nil {
			f.Close()
			// Cache static assets aggressively
			if strings.Contains(path, "/assets/") {
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			}
			fileServer.ServeHTTP(w, r)
			return
		}

		// SPA fallback: serve index.html
		r.URL.Path = "/"
		fileServer.ServeHTTP(w, r)
	})
}

// GenerateFromManifest loads a manifest file and generates deployment files.
// This is the CLI entry point for `setup generate`.
func GenerateFromManifest(manifestPath, outputDir string) error {
	cfg, err := LoadManifest(manifestPath)
	if err != nil {
		return err
	}

	errs := Validate(cfg)
	if len(errs) > 0 {
		fmt.Fprintf(os.Stderr, "Validation errors in %s:\n", manifestPath)
		for _, e := range errs {
			fmt.Fprintf(os.Stderr, "  %s: %s\n", e.Field, e.Message)
		}
		return fmt.Errorf("%d validation errors", len(errs))
	}

	result, err := Generate(cfg, outputDir, func(msg string) {
		fmt.Println(msg)
	})
	if err != nil {
		return err
	}

	for _, f := range result.Files {
		if f.Path == ManifestFilename {
			continue // Don't re-announce the manifest
		}
		fmt.Printf("  %s\n", f.Path)
	}

	return nil
}

// PodInfo is a simplified view of a Kubernetes pod for the UI.
type PodInfo struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Ready     string `json:"ready"`
	Status    string `json:"status"`
	Restarts  int    `json:"restarts"`
	Age       string `json:"age"`
}

func (s *Server) handleGetPods(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	outputDir := s.outputDir
	s.mu.Unlock()

	kubeconfigPath := filepath.Join(outputDir, "generated", "kubeconfig.yaml")
	if _, err := os.Stat(kubeconfigPath); os.IsNotExist(err) {
		writeJSON(w, http.StatusOK, map[string]any{
			"available": false,
		})
		return
	}

	absKubeconfig, _ := filepath.Abs(kubeconfigPath)

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "kubectl", "get", "pods", "-A", "-o", "json")
	cmd.Env = append(os.Environ(), "KUBECONFIG="+absKubeconfig)

	out, err := cmd.Output()
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"available":      true,
			"kubeconfig_path": absKubeconfig,
			"error":          fmt.Sprintf("kubectl failed: %v", err),
			"pods":           []PodInfo{},
		})
		return
	}

	// Parse kubectl JSON output
	var podList struct {
		Items []struct {
			Metadata struct {
				Name              string    `json:"name"`
				Namespace         string    `json:"namespace"`
				CreationTimestamp time.Time `json:"creationTimestamp"`
			} `json:"metadata"`
			Status struct {
				Phase             string `json:"phase"`
				ContainerStatuses []struct {
					Ready        bool `json:"ready"`
					RestartCount int  `json:"restartCount"`
					State        struct {
						Waiting *struct {
							Reason string `json:"reason"`
						} `json:"waiting"`
						Terminated *struct {
							Reason string `json:"reason"`
						} `json:"terminated"`
					} `json:"state"`
				} `json:"containerStatuses"`
				InitContainerStatuses []struct {
					Ready        bool `json:"ready"`
					RestartCount int  `json:"restartCount"`
					State        struct {
						Waiting *struct {
							Reason string `json:"reason"`
						} `json:"waiting"`
						Terminated *struct {
							Reason   string `json:"reason"`
							ExitCode int    `json:"exitCode"`
						} `json:"terminated"`
					} `json:"state"`
				} `json:"initContainerStatuses"`
			} `json:"status"`
			Spec struct {
				InitContainers []struct{} `json:"initContainers"`
				Containers     []struct{} `json:"containers"`
			} `json:"spec"`
		} `json:"items"`
	}

	if err := json.Unmarshal(out, &podList); err != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"available":      true,
			"kubeconfig_path": absKubeconfig,
			"error":          fmt.Sprintf("parse error: %v", err),
			"pods":           []PodInfo{},
		})
		return
	}

	pods := make([]PodInfo, 0, len(podList.Items))
	for _, item := range podList.Items {
		totalContainers := len(item.Spec.Containers)
		readyContainers := 0
		restarts := 0
		effectiveStatus := string(item.Status.Phase)

		// Check init containers first
		for _, ic := range item.Status.InitContainerStatuses {
			restarts += ic.RestartCount
			if ic.State.Waiting != nil && ic.State.Waiting.Reason != "" {
				effectiveStatus = "Init:" + ic.State.Waiting.Reason
			} else if ic.State.Terminated != nil && ic.State.Terminated.ExitCode != 0 {
				effectiveStatus = "Init:" + ic.State.Terminated.Reason
			}
		}

		for _, cs := range item.Status.ContainerStatuses {
			restarts += cs.RestartCount
			if cs.Ready {
				readyContainers++
			}
			if cs.State.Waiting != nil && cs.State.Waiting.Reason != "" {
				effectiveStatus = cs.State.Waiting.Reason
			} else if cs.State.Terminated != nil && cs.State.Terminated.Reason != "" {
				effectiveStatus = cs.State.Terminated.Reason
			}
		}

		pods = append(pods, PodInfo{
			Name:      item.Metadata.Name,
			Namespace: item.Metadata.Namespace,
			Ready:     fmt.Sprintf("%d/%d", readyContainers, totalContainers),
			Status:    effectiveStatus,
			Restarts:  restarts,
			Age:       formatDuration(time.Since(item.Metadata.CreationTimestamp)),
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"available":      true,
		"kubeconfig_path": absKubeconfig,
		"pods":           pods,
	})
}

func (s *Server) handlePodDebug(w http.ResponseWriter, r *http.Request) {
	namespace := r.URL.Query().Get("namespace")
	name := r.URL.Query().Get("name")

	if namespace == "" || name == "" {
		writeJSON(w, http.StatusOK, map[string]string{
			"logs":     "",
			"describe": "",
			"error":    "namespace and name are required",
		})
		return
	}

	s.mu.Lock()
	outputDir := s.outputDir
	s.mu.Unlock()

	kubeconfigPath := filepath.Join(outputDir, "generated", "kubeconfig.yaml")
	absKubeconfig, _ := filepath.Abs(kubeconfigPath)
	env := append(os.Environ(), "KUBECONFIG="+absKubeconfig)

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	type cmdResult struct {
		output string
		err    error
	}

	logsCh := make(chan cmdResult, 1)
	describeCh := make(chan cmdResult, 1)

	go func() {
		cmd := exec.CommandContext(ctx, "kubectl", "logs", name, "-n", namespace, "--all-containers", "--tail=100")
		cmd.Env = env
		out, err := cmd.CombinedOutput()
		logsCh <- cmdResult{string(out), err}
	}()

	go func() {
		cmd := exec.CommandContext(ctx, "kubectl", "describe", "pod", name, "-n", namespace)
		cmd.Env = env
		out, err := cmd.CombinedOutput()
		describeCh <- cmdResult{string(out), err}
	}()

	logsResult := <-logsCh
	describeResult := <-describeCh

	resp := map[string]string{
		"logs":     logsResult.output,
		"describe": describeResult.output,
	}

	var errs []string
	if logsResult.err != nil {
		errs = append(errs, "logs: "+logsResult.err.Error())
	}
	if describeResult.err != nil {
		errs = append(errs, "describe: "+describeResult.err.Error())
	}
	if len(errs) > 0 {
		resp["error"] = strings.Join(errs, "; ")
	}

	writeJSON(w, http.StatusOK, resp)
}

// formatDuration returns a human-readable short duration string.
func formatDuration(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	totalSeconds := int(math.Round(d.Seconds()))
	if totalSeconds < 60 {
		return fmt.Sprintf("%ds", totalSeconds)
	}
	totalMinutes := totalSeconds / 60
	if totalMinutes < 60 {
		return fmt.Sprintf("%dm", totalMinutes)
	}
	hours := totalMinutes / 60
	minutes := totalMinutes % 60
	if hours < 24 {
		if minutes == 0 {
			return fmt.Sprintf("%dh", hours)
		}
		return fmt.Sprintf("%dh%dm", hours, minutes)
	}
	days := hours / 24
	return fmt.Sprintf("%dd", days)
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(data); err != nil {
		fmt.Fprintf(w, `{"error": "encode: %s"}`, err.Error())
	}
}
