package main

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

func main() {
	listenAddr := envOr("LISTEN_ADDR", "127.0.0.1:4180")
	coreAPIURL := requireEnv("CORE_API_URL")
	coreAPIToken := requireEnv("CORE_API_TOKEN")
	cookieSecret := requireEnv("COOKIE_SECRET")
	sessionDir := envOr("SESSION_DIR", "/tmp/dbadmin-sessions")

	// Ensure session directory exists.
	if err := os.MkdirAll(sessionDir, 0700); err != nil {
		log.Fatalf("create session dir: %v", err)
	}

	mux := http.NewServeMux()

	// Auth login: validate session token, get temp credentials, write session file, redirect to signon.
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

		if session.Database == nil {
			http.Error(w, "database_id is required for DB Admin access", http.StatusBadRequest)
			return
		}

		// Request temp MySQL credentials from the core API.
		creds, err := requestTempAccess(coreAPIURL, coreAPIToken, session.Database.ID)
		if err != nil {
			log.Printf("temp access request failed: %v", err)
			http.Error(w, "failed to create database access", http.StatusInternalServerError)
			return
		}

		// Write credentials to a session file.
		fileToken := randomAlphanumeric(32)
		sessionFile := filepath.Join(sessionDir, fileToken+".json")
		data, _ := json.Marshal(map[string]any{
			"username": creds.Username,
			"password": creds.Password,
			"host":     creds.Host,
			"port":     creds.Port,
			"database": creds.DatabaseName,
		})
		if err := os.WriteFile(sessionFile, data, 0600); err != nil {
			log.Printf("write session file: %v", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		// Set dbadmin_token cookie (short-lived, consumed by signon.php).
		http.SetCookie(w, &http.Cookie{
			Name:     "dbadmin_token",
			Value:    fileToken,
			Path:     "/",
			MaxAge:   30,
			HttpOnly: true,
			Secure:   true,
			SameSite: http.SameSiteStrictMode,
		})

		// Set signed session cookie (valid 24h) for general auth.
		expiry := time.Now().Add(24 * time.Hour)
		cookieVal := signCookie(session.TenantID, expiry, cookieSecret)
		http.SetCookie(w, &http.Cookie{
			Name:     "dbadmin_session",
			Value:    cookieVal,
			Path:     "/",
			Expires:  expiry,
			HttpOnly: true,
			Secure:   true,
			SameSite: http.SameSiteLaxMode,
		})

		log.Printf("login: created temp access for database %s, redirecting to signon", creds.DatabaseName)
		http.Redirect(w, r, "/signon.php", http.StatusFound)
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

	// Unauthorized page: shown when phpMyAdmin session expires.
	mux.HandleFunc("/auth/unauthorized", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `<!DOCTYPE html>
<html><body>
<h2>Session expired</h2>
<p>Please use the control panel to open DB Admin.</p>
</body></html>`)
	})

	log.Printf("dbadmin-proxy listening on %s", listenAddr)
	if err := http.ListenAndServe(listenAddr, mux); err != nil {
		log.Fatalf("listen: %v", err)
	}
}

type sessionResult struct {
	TenantID string        `json:"tenant_id"`
	Database *databaseInfo `json:"database,omitempty"`
}

type databaseInfo struct {
	ID           string `json:"id"`
	DatabaseName string `json:"database_name"`
	Host         string `json:"host"`
	Port         int    `json:"port"`
}

type tempAccessResult struct {
	Username     string `json:"username"`
	Password     string `json:"password"`
	Host         string `json:"host"`
	Port         int    `json:"port"`
	DatabaseName string `json:"database_name"`
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

// requestTempAccess calls the core API to create a temporary MySQL user.
func requestTempAccess(apiURL, apiToken, databaseID string) (*tempAccessResult, error) {
	req, err := http.NewRequest("POST", apiURL+"/internal/v1/databases/"+databaseID+"/temp-access", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("api request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("api returned %d", resp.StatusCode)
	}

	var result tempAccessResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &result, nil
}

// signCookie produces "tenantID:expiry_unix:hmac_hex".
func signCookie(tenantID string, expiry time.Time, secret string) string {
	payload := fmt.Sprintf("%s:%d", tenantID, expiry.Unix())
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	sig := hex.EncodeToString(mac.Sum(nil))
	return payload + ":" + sig
}

// randomAlphanumeric generates a random alphanumeric string of the given length.
func randomAlphanumeric(n int) string {
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		idx, _ := rand.Int(rand.Reader, big.NewInt(int64(len(chars))))
		b[i] = chars[idx.Int64()]
	}
	return string(b)
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
