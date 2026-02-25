package mcpserver

import (
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/mark3labs/mcp-go/server"
	"github.com/rs/zerolog"
)

// Server is the MCP server that proxies tool calls to the REST API.
type Server struct {
	router chi.Router
	logger zerolog.Logger
	cfg    *Config
}

// New creates and configures a new MCP server from the given config and swagger spec.
func New(cfg *Config, specData []byte, logger zerolog.Logger) (*Server, error) {
	spec, err := ParseSpec(specData)
	if err != nil {
		return nil, err
	}

	proxy := NewProxyHandler(cfg.APIURL, logger)
	groups, _ := BuildTools(spec, cfg, proxy.Handler)

	router := chi.NewRouter()
	router.Use(middleware.RequestID)
	router.Use(middleware.RealIP)
	router.Use(middleware.Recoverer)

	// Health check
	router.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	// Mount each group as a separate MCP server, and collect all tools for the unified endpoint.
	var allTools []server.ServerTool
	router.Route("/mcp", func(r chi.Router) {
		for groupName, tools := range groups {
			groupDesc := cfg.Groups[groupName].Description
			if groupDesc == "" {
				groupDesc = "Hosting platform " + groupName + " tools"
			}

			mcpSrv := server.NewMCPServer(
				"hosting-"+groupName,
				"1.0.0",
				server.WithInstructions(groupDesc),
			)
			mcpSrv.AddTools(tools...)

			httpSrv := server.NewStreamableHTTPServer(mcpSrv,
				server.WithEndpointPath("/"),
			)

			r.Mount("/"+groupName, httpSrv)
			allTools = append(allTools, tools...)

			logger.Info().
				Str("group", groupName).
				Int("tools", len(tools)).
				Msg("mounted MCP tool group")
		}

		// Unified endpoint with all tools for agents that need full platform access.
		allSrv := server.NewMCPServer(
			"hosting",
			"1.0.0",
			server.WithInstructions("Hosting platform management — infrastructure, tenants, databases, DNS, email, storage, web, and operations tools."),
		)
		allSrv.AddTools(allTools...)
		r.Mount("/all", server.NewStreamableHTTPServer(allSrv, server.WithEndpointPath("/")))
		logger.Info().Int("tools", len(allTools)).Msg("mounted unified MCP endpoint at /mcp/all")

		// Index endpoint listing available groups
		r.Get("/", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			var lines []string
			// Sort group names for deterministic output
			var names []string
			for name := range groups {
				names = append(names, name)
			}
			sort.Strings(names)

			for _, name := range names {
				tools := groups[name]
				desc := cfg.Groups[name].Description
				lines = append(lines, fmt.Sprintf(`{"name":%q,"endpoint":"/mcp/%s","tools":%d,"description":%q}`,
					name, name, len(tools), desc))
			}
			lines = append(lines, fmt.Sprintf(`{"name":"all","endpoint":"/mcp/all","tools":%d,"description":"All tools from every group"}`, len(allTools)))
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("[" + strings.Join(lines, ",") + "]"))
		})
	})

	return &Server{
		router: router,
		logger: logger,
		cfg:    cfg,
	}, nil
}

// ServeHTTP implements http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}

// FetchSpec downloads the swagger spec from the API.
func FetchSpec(apiURL, specPath string) ([]byte, error) {
	url := strings.TrimRight(apiURL, "/") + specPath
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("fetch spec from %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch spec from %s: HTTP %d", url, resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}
