package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"sync"

	"golang.org/x/oauth2"
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

	// OIDC SSO (Azure AD)
	oidc := newOIDCHandler(coreAPIURL)
	if oidc != nil {
		mux.HandleFunc("/auth/login", oidc.handleLogin)
		mux.HandleFunc("/auth/callback", oidc.handleCallback)
		mux.HandleFunc("/auth/sso-enabled", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"enabled":true}`))
		})
		log.Printf("SSO enabled (Azure AD tenant: %s)", os.Getenv("OIDC_TENANT_ID"))
	} else {
		mux.HandleFunc("/auth/sso-enabled", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"enabled":false}`))
		})
	}

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

// oidcHandler implements the OIDC login/callback flow for Azure AD SSO.
type oidcHandler struct {
	oauthConfig *oauth2.Config
	coreAPIURL  string
	adminAPIKey string
	// state tokens: map[state]true, cleaned up after use
	mu     sync.Mutex
	states map[string]bool
}

func newOIDCHandler(coreAPIURL string) *oidcHandler {
	clientID := os.Getenv("OIDC_CLIENT_ID")
	clientSecret := os.Getenv("OIDC_CLIENT_SECRET")
	tenantID := os.Getenv("OIDC_TENANT_ID")
	adminAPIKey := os.Getenv("ADMIN_API_KEY")

	if clientID == "" || clientSecret == "" || tenantID == "" {
		return nil
	}

	baseDomain := os.Getenv("BASE_DOMAIN")
	redirectURL := fmt.Sprintf("https://admin.%s/auth/callback", baseDomain)

	return &oidcHandler{
		oauthConfig: &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			Endpoint: oauth2.Endpoint{
				AuthURL:  fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/v2.0/authorize", tenantID),
				TokenURL: fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/v2.0/token", tenantID),
			},
			RedirectURL: redirectURL,
			Scopes:      []string{"openid", "email", "profile"},
		},
		coreAPIURL:  coreAPIURL,
		adminAPIKey: adminAPIKey,
		states:      make(map[string]bool),
	}
}

func (h *oidcHandler) handleLogin(w http.ResponseWriter, r *http.Request) {
	state := generateState()
	h.mu.Lock()
	h.states[state] = true
	h.mu.Unlock()

	url := h.oauthConfig.AuthCodeURL(state)
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

func (h *oidcHandler) handleCallback(w http.ResponseWriter, r *http.Request) {
	state := r.URL.Query().Get("state")
	h.mu.Lock()
	valid := h.states[state]
	delete(h.states, state)
	h.mu.Unlock()

	if !valid {
		http.Error(w, "invalid state parameter", http.StatusBadRequest)
		return
	}

	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "missing authorization code", http.StatusBadRequest)
		return
	}

	token, err := h.oauthConfig.Exchange(context.Background(), code)
	if err != nil {
		log.Printf("OIDC token exchange failed: %v", err)
		http.Error(w, "authentication failed", http.StatusInternalServerError)
		return
	}

	// Extract user info from the ID token (JWT)
	idTokenRaw, ok := token.Extra("id_token").(string)
	if !ok {
		http.Error(w, "no id_token in response", http.StatusInternalServerError)
		return
	}

	email, name, err := parseIDTokenClaims(idTokenRaw)
	if err != nil {
		log.Printf("Failed to parse ID token: %v", err)
		http.Error(w, "failed to parse identity", http.StatusInternalServerError)
		return
	}

	// Create an API key for this user via core-api
	apiKey, err := h.createAPIKey(email, name)
	if err != nil {
		log.Printf("Failed to create API key for %s: %v", email, err)
		http.Error(w, "failed to provision access", http.StatusInternalServerError)
		return
	}

	// Return HTML that stores the API key in localStorage and redirects
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<!DOCTYPE html>
<html><head><title>SSO Login</title></head>
<body>
<script>
localStorage.setItem('api_key', %s);
window.location.href = '/';
</script>
<p>Signing in...</p>
</body></html>`, jsonString(apiKey))
}

// createAPIKey calls core-api to create (or find existing) an API key for the SSO user.
func (h *oidcHandler) createAPIKey(email, name string) (string, error) {
	keyName := fmt.Sprintf("sso:%s", email)

	body, _ := json.Marshal(map[string]string{
		"name": keyName,
	})

	req, err := http.NewRequest("POST", h.coreAPIURL+"/api/v1/api-keys", strings.NewReader(string(body)))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+h.adminAPIKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("core-api request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("core-api returned %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		RawKey string `json:"raw_key"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}

	return result.RawKey, nil
}

// parseIDTokenClaims extracts email and name from a JWT ID token without
// full signature verification (the token was received directly from the IdP
// over TLS in the auth code exchange).
func parseIDTokenClaims(idToken string) (email, name string, err error) {
	parts := strings.Split(idToken, ".")
	if len(parts) != 3 {
		return "", "", fmt.Errorf("invalid JWT format")
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", "", fmt.Errorf("decode JWT payload: %w", err)
	}

	var claims struct {
		Email             string `json:"email"`
		PreferredUsername string `json:"preferred_username"`
		Name              string `json:"name"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return "", "", fmt.Errorf("parse JWT claims: %w", err)
	}

	email = claims.Email
	if email == "" {
		email = claims.PreferredUsername
	}
	name = claims.Name

	return email, name, nil
}

func generateState() string {
	b := make([]byte, 16)
	rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

func jsonString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
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
