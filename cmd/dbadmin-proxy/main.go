package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"time"
)

func main() {
	listenAddr := envOr("LISTEN_ADDR", "127.0.0.1:4180")
	coreAPIURL := requireEnv("CORE_API_URL")
	coreAPIToken := requireEnv("CORE_API_TOKEN")
	upstreamURL := envOr("CLOUDBEAVER_URL", "http://127.0.0.1:8978")
	cookieSecret := requireEnv("COOKIE_SECRET")

	upstream, err := url.Parse(upstreamURL)
	if err != nil {
		log.Fatalf("invalid CLOUDBEAVER_URL: %v", err)
	}

	proxy := httputil.NewSingleHostReverseProxy(upstream)

	mux := http.NewServeMux()

	// Auth login: validate session token server-to-server, set cookie, create CB connection.
	mux.HandleFunc("/auth/login", func(w http.ResponseWriter, r *http.Request) {
		token := r.URL.Query().Get("token")
		if token == "" {
			http.Error(w, "missing token parameter", http.StatusBadRequest)
			return
		}

		session, err := validateSession(coreAPIURL, coreAPIToken, token)
		if err != nil {
			log.Printf("session validation failed: %v", err)
			http.Error(w, "invalid or expired session", http.StatusForbidden)
			return
		}

		// Set signed session cookie (valid 24h).
		expiry := time.Now().Add(24 * time.Hour)
		cookieVal := signCookie(session.TenantID, expiry, cookieSecret)
		http.SetCookie(w, &http.Cookie{
			Name:     "dbadmin_session",
			Value:    cookieVal,
			Path:     "/",
			Expires:  expiry,
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
		})

		redirectPath := "/"

		// If database info is present, create a CloudBeaver connection.
		if session.Database != nil {
			connID, err := createCBConnection(upstreamURL, session.Database)
			if err != nil {
				log.Printf("failed to create CloudBeaver connection: %v", err)
			} else {
				log.Printf("created CloudBeaver connection %s for %s", connID, session.Database.DatabaseName)
				redirectPath = "/#/sql/" + connID
			}
		}

		http.Redirect(w, r, redirectPath, http.StatusFound)
	})

	// Auth logout: clear cookie.
	mux.HandleFunc("/auth/logout", func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{
			Name:     "dbadmin_session",
			Value:    "",
			Path:     "/",
			MaxAge:   -1,
			HttpOnly: true,
		})
		http.Error(w, "logged out", http.StatusOK)
	})

	// All other requests: check cookie, proxy to CloudBeaver.
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("dbadmin_session")
		if err != nil || !verifyCookie(cookie.Value, cookieSecret) {
			http.Error(w, "unauthorized — use the admin panel to open DB Admin", http.StatusUnauthorized)
			return
		}

		proxy.ServeHTTP(w, r)
	})

	log.Printf("dbadmin-proxy listening on %s (upstream: %s)", listenAddr, upstreamURL)
	if err := http.ListenAndServe(listenAddr, mux); err != nil {
		log.Fatalf("listen: %v", err)
	}
}

type sessionResult struct {
	TenantID string        `json:"tenant_id"`
	Database *databaseInfo `json:"database,omitempty"`
}

type databaseInfo struct {
	DatabaseName string `json:"database_name"`
	Host         string `json:"host"`
	Port         int    `json:"port"`
}

// validateSession calls the core API to validate and consume a login session.
func validateSession(apiURL, apiToken, sessionID string) (*sessionResult, error) {
	req, err := http.NewRequest("POST", apiURL+"/internal/v1/login-sessions/validate", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiToken)
	req.Header.Set("Content-Type", "application/json")

	q := req.URL.Query()
	q.Set("session_id", sessionID)
	req.URL.RawQuery = q.Encode()

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("api request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("api returned %d", resp.StatusCode)
	}

	var result sessionResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &result, nil
}

// createCBConnection creates a temporary connection in CloudBeaver via its GraphQL API.
func createCBConnection(cbURL string, db *databaseInfo) (string, error) {
	query := `mutation createConnection($config: ConnectionConfig!) {
		createConnection(config: $config) { id name }
	}`

	variables := map[string]any{
		"config": map[string]any{
			"driverId":     "mysql:mysql8",
			"name":         db.DatabaseName,
			"host":         db.Host,
			"port":         fmt.Sprintf("%d", db.Port),
			"databaseName": db.DatabaseName,
		},
	}

	body := map[string]any{
		"query":     query,
		"variables": variables,
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	resp, err := http.Post(cbURL+"/api/gql", "application/json", bytes.NewReader(jsonBody))
	if err != nil {
		return "", fmt.Errorf("graphql request: %w", err)
	}
	defer resp.Body.Close()

	var gqlResp struct {
		Data struct {
			CreateConnection struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"createConnection"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&gqlResp); err != nil {
		return "", fmt.Errorf("decode graphql response: %w", err)
	}

	if len(gqlResp.Errors) > 0 {
		return "", fmt.Errorf("graphql error: %s", gqlResp.Errors[0].Message)
	}

	return gqlResp.Data.CreateConnection.ID, nil
}

// signCookie produces "tenantID:expiry_unix:hmac_hex".
func signCookie(tenantID string, expiry time.Time, secret string) string {
	payload := fmt.Sprintf("%s:%d", tenantID, expiry.Unix())
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	sig := hex.EncodeToString(mac.Sum(nil))
	return payload + ":" + sig
}

// verifyCookie checks "tenantID:expiry_unix:hmac_hex" is valid and not expired.
func verifyCookie(value, secret string) bool {
	// Split into exactly 3 parts: tenantID, expiry, signature.
	parts := splitN(value, ':', 3)
	if len(parts) != 3 {
		return false
	}

	payload := parts[0] + ":" + parts[1]
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	expected := hex.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(parts[2]), []byte(expected)) {
		return false
	}

	var expiry int64
	if _, err := fmt.Sscanf(parts[1], "%d", &expiry); err != nil {
		return false
	}
	return time.Now().Unix() < expiry
}

// splitN splits s on sep into at most n parts.
func splitN(s string, sep byte, n int) []string {
	var parts []string
	for i := 0; i < n-1; i++ {
		idx := -1
		for j := 0; j < len(s); j++ {
			if s[j] == sep {
				idx = j
				break
			}
		}
		if idx < 0 {
			break
		}
		parts = append(parts, s[:idx])
		s = s[idx+1:]
	}
	parts = append(parts, s)
	return parts
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func requireEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("required environment variable %s is not set", key)
	}
	return v
}
