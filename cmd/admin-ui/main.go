package main

import (
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
)

func main() {
	listenAddr := envOr("LISTEN_ADDR", ":3001")
	coreAPIURL := envOr("CORE_API_URL", "http://localhost:8090")
	staticDir := envOr("STATIC_DIR", "./dist")

	target, err := url.Parse(coreAPIURL)
	if err != nil {
		log.Fatalf("invalid CORE_API_URL: %v", err)
	}

	proxy := httputil.NewSingleHostReverseProxy(target)

	mux := http.NewServeMux()

	// Proxy API requests to core-api (with WebSocket support).
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		if isWebSocket(r) {
			proxyWebSocket(w, r, target)
			return
		}
		proxy.ServeHTTP(w, r)
	})

	// Proxy docs to core-api
	mux.HandleFunc("/docs/", func(w http.ResponseWriter, r *http.Request) {
		proxy.ServeHTTP(w, r)
	})
	mux.HandleFunc("/docs", func(w http.ResponseWriter, r *http.Request) {
		proxy.ServeHTTP(w, r)
	})

	// Health check
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	// Serve SPA with fallback to index.html
	spa := spaHandler{staticDir: staticDir}
	mux.Handle("/", spa)

	log.Printf("Admin UI listening on %s (proxying API to %s, static from %s)", listenAddr, coreAPIURL, staticDir)
	if err := http.ListenAndServe(listenAddr, mux); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

type spaHandler struct {
	staticDir string
}

func (h spaHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := h.staticDir + r.URL.Path

	// Check if file exists
	fi, err := os.Stat(path)
	if err != nil || fi.IsDir() {
		// Serve index.html for SPA routing
		http.ServeFile(w, r, h.staticDir+"/index.html")
		return
	}

	// Cache static assets aggressively
	if strings.Contains(r.URL.Path, "/assets/") {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	}

	http.ServeFile(w, r, path)
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func isWebSocket(r *http.Request) bool {
	return strings.EqualFold(r.Header.Get("Upgrade"), "websocket")
}

// proxyWebSocket hijacks the client connection and tunnels raw TCP to the upstream.
func proxyWebSocket(w http.ResponseWriter, r *http.Request, target *url.URL) {
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "websocket hijack not supported", http.StatusInternalServerError)
		return
	}

	upstream, err := net.Dial("tcp", target.Host)
	if err != nil {
		http.Error(w, "upstream unreachable", http.StatusBadGateway)
		return
	}
	defer upstream.Close()

	// Rewrite the request URL to the upstream and forward it verbatim.
	r.URL.Scheme = target.Scheme
	r.URL.Host = target.Host
	r.Host = target.Host
	if err := r.Write(upstream); err != nil {
		http.Error(w, "failed to write to upstream", http.StatusBadGateway)
		return
	}

	client, buf, err := hijacker.Hijack()
	if err != nil {
		log.Printf("websocket hijack failed: %v", err)
		return
	}
	defer client.Close()

	// Flush any buffered data from the hijacked connection.
	if buf.Reader.Buffered() > 0 {
		buffered := make([]byte, buf.Reader.Buffered())
		buf.Read(buffered)
		upstream.Write(buffered)
	}

	// Bidirectional copy.
	done := make(chan struct{}, 2)
	go func() { io.Copy(client, upstream); done <- struct{}{} }()
	go func() { io.Copy(upstream, client); done <- struct{}{} }()
	<-done
}
