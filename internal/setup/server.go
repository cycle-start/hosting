package setup

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
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

	result, err := Generate(cfg, s.outputDir)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, result)
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

	result, err := Generate(cfg, outputDir)
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

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(data); err != nil {
		fmt.Fprintf(w, `{"error": "encode: %s"}`, err.Error())
	}
}
