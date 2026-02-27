package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strconv"
	"strings"
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

	// Inject a connection-creation script into CloudBeaver's HTML when a
	// dbadmin_connect cookie is present. This ensures the connection is created
	// inside CloudBeaver's own session (which CB initializes on page load).
	proxy.ModifyResponse = func(resp *http.Response) error {
		ct := resp.Header.Get("Content-Type")
		if !strings.HasPrefix(ct, "text/html") {
			return nil
		}

		// Check if the original request had a dbadmin_connect cookie.
		connectCookie, err := resp.Request.Cookie("dbadmin_connect")
		if err != nil || connectCookie.Value == "" {
			return nil
		}

		// Read the original body.
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return err
		}

		// Inject script before </body>.
		script := fmt.Sprintf(`<script>
(function() {
  var info = JSON.parse(decodeURIComponent('%s'));
  document.cookie = 'dbadmin_connect=; Path=/; Max-Age=0; Secure';

  function tryConnect(attempts) {
    if (attempts <= 0) return;
    fetch('/api/gql', {
      method: 'POST',
      headers: {'Content-Type': 'application/json'},
      body: JSON.stringify({query: 'query { sessionState { valid } }'})
    })
    .then(function(r) { return r.json(); })
    .then(function(d) {
      if (d.data && d.data.sessionState && d.data.sessionState.valid) {
        return fetch('/api/gql', {
          method: 'POST',
          headers: {'Content-Type': 'application/json'},
          body: JSON.stringify({
            query: 'mutation createConnection($config: ConnectionConfig!) { createConnection(config: $config) { id name } }',
            variables: { config: {
              driverId: 'mysql:mysql8',
              name: info.n,
              host: info.h,
              port: String(info.p),
              databaseName: info.n
            }}
          })
        })
        .then(function(r) { return r.json(); })
        .then(function(d) {
          if (d.data && d.data.createConnection) {
            window.location.hash = '/sql/' + d.data.createConnection.id;
            window.location.reload();
          }
        });
      } else {
        setTimeout(function() { tryConnect(attempts - 1); }, 500);
      }
    })
    .catch(function() {
      setTimeout(function() { tryConnect(attempts - 1); }, 500);
    });
  }

  // Wait for CloudBeaver to initialize its session, then create the connection.
  setTimeout(function() { tryConnect(10); }, 1000);
})();
</script>`, url.QueryEscape(connectCookie.Value))

		modified := strings.Replace(string(body), "</body>", script+"</body>", 1)
		resp.Body = io.NopCloser(bytes.NewReader([]byte(modified)))
		resp.ContentLength = int64(len(modified))
		resp.Header.Set("Content-Length", strconv.Itoa(len(modified)))
		return nil
	}

	mux := http.NewServeMux()

	// Auth login: validate session token, set cookies, redirect to CloudBeaver.
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
			Secure:   true,
			SameSite: http.SameSiteLaxMode,
		})

		// If database info is present, store it in a cookie so the injected
		// script can create the connection inside CloudBeaver's own session.
		if session.Database != nil {
			connectInfo, _ := json.Marshal(map[string]any{
				"n": session.Database.DatabaseName,
				"h": session.Database.Host,
				"p": session.Database.Port,
			})
			http.SetCookie(w, &http.Cookie{
				Name:     "dbadmin_connect",
				Value:    string(connectInfo),
				Path:     "/",
				MaxAge:   30,
				Secure:   true,
				SameSite: http.SameSiteLaxMode,
			})
			log.Printf("login: redirect with connect info for %s", session.Database.DatabaseName)
		}

		http.Redirect(w, r, "/", http.StatusFound)
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
