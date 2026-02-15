package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/edvin/hosting/internal/agent/runtime"
)

// newTestNginxManager creates an NginxManager with a temporary config directory.
func newTestNginxManager(t *testing.T) *NginxManager {
	t.Helper()
	tmpDir := t.TempDir()
	cfg := Config{
		NginxConfigDir: tmpDir,
		CertDir:        filepath.Join(tmpDir, "certs"),
	}
	return NewNginxManager(zerolog.Nop(), cfg)
}

func TestGenerateConfig_PHPRuntime(t *testing.T) {
	mgr := newTestNginxManager(t)

	webroot := &runtime.WebrootInfo{
		ID:             "wr-001",
		TenantName:     "tenant1",
		Name:           "mysite",
		Runtime:        "php",
		RuntimeVersion: "8.2",
		PublicFolder:   "public",
	}
	fqdns := []*FQDNInfo{
		{FQDN: "example.com", SSLEnabled: false},
	}

	config, err := mgr.GenerateConfig(webroot, fqdns)
	require.NoError(t, err)

	// Verify PHP-specific directives.
	assert.Contains(t, config, "fastcgi_pass unix:/run/php/tenant1-php8.2.sock")
	assert.Contains(t, config, "location ~ \\.php$")
	assert.Contains(t, config, "fastcgi_param SCRIPT_FILENAME")
	assert.Contains(t, config, "include snippets/fastcgi-php.conf")
	assert.Contains(t, config, "index.php")

	// Verify try_files for PHP.
	assert.Contains(t, config, "/index.php?$query_string")

	// Verify server_name.
	assert.Contains(t, config, "server_name example.com")

	// Verify document root includes public folder (under webroots/).
	assert.Contains(t, config, "root /var/www/storage/tenant1/webroots/mysite/public")

	// Verify .ht deny block.
	assert.Contains(t, config, "location ~ /\\.ht")
	assert.Contains(t, config, "deny all")

	// Verify log paths on local SSD.
	assert.Contains(t, config, "access_log /var/log/hosting/tenant1/wr-001-access.log hosting_json")
	assert.Contains(t, config, "error_log  /var/log/hosting/tenant1/wr-001-error.log warn")

	// Verify node identification headers.
	assert.Contains(t, config, "add_header X-Served-By $hostname always")
}

func TestGenerateConfig_NodeIdentificationHeaders(t *testing.T) {
	mgr := newTestNginxManager(t)
	mgr.SetShardName("web-1")

	webroot := &runtime.WebrootInfo{
		TenantName: "tenant1",
		Name:       "mysite",
		Runtime:    "static",
	}
	fqdns := []*FQDNInfo{{FQDN: "example.com"}}

	config, err := mgr.GenerateConfig(webroot, fqdns)
	require.NoError(t, err)

	// X-Served-By uses nginx's $hostname variable (resolved at runtime).
	assert.Contains(t, config, `add_header X-Served-By $hostname always`)
	// X-Shard is baked into the config at generation time.
	assert.Contains(t, config, `add_header X-Shard "web-1" always`)
}

func TestGenerateConfig_PHPRuntime_DefaultVersion(t *testing.T) {
	mgr := newTestNginxManager(t)

	webroot := &runtime.WebrootInfo{
		TenantName:     "tenant1",
		Name:           "mysite",
		Runtime:        "php",
		RuntimeVersion: "", // Empty; nginx template uses it directly.
	}
	fqdns := []*FQDNInfo{
		{FQDN: "example.com"},
	}

	config, err := mgr.GenerateConfig(webroot, fqdns)
	require.NoError(t, err)

	// With empty RuntimeVersion, the template just renders php.sock (without version).
	assert.Contains(t, config, "fastcgi_pass unix:/run/php/tenant1-php.sock")
}

func TestGenerateConfig_NodeRuntime(t *testing.T) {
	mgr := newTestNginxManager(t)

	webroot := &runtime.WebrootInfo{
		TenantName: "tenant1",
		Name:       "nodeapp",
		Runtime:    "node",
	}
	fqdns := []*FQDNInfo{
		{FQDN: "nodeapp.example.com"},
	}

	config, err := mgr.GenerateConfig(webroot, fqdns)
	require.NoError(t, err)

	// Verify proxy_pass is present with the computed port.
	assert.Contains(t, config, "proxy_pass http://127.0.0.1:")
	assert.Contains(t, config, "proxy_http_version 1.1")
	assert.Contains(t, config, "proxy_set_header Upgrade $http_upgrade")
	assert.Contains(t, config, `proxy_set_header Connection "upgrade"`)
	assert.Contains(t, config, "proxy_set_header Host $host")
	assert.Contains(t, config, "proxy_set_header X-Real-IP $remote_addr")
	assert.Contains(t, config, "proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for")
	assert.Contains(t, config, "proxy_set_header X-Forwarded-Proto $scheme")

	// Verify try_files for node.
	assert.Contains(t, config, "@app")

	// Verify server_name.
	assert.Contains(t, config, "server_name nodeapp.example.com")

	// Should NOT contain PHP directives.
	assert.NotContains(t, config, "fastcgi_pass")
	assert.NotContains(t, config, "index.php")
}

func TestGenerateConfig_NodeRuntime_PortDeterminism(t *testing.T) {
	mgr := newTestNginxManager(t)

	webroot := &runtime.WebrootInfo{
		TenantName: "tenant1",
		Name:       "nodeapp",
		Runtime:    "node",
	}
	fqdns := []*FQDNInfo{{FQDN: "example.com"}}

	config1, err := mgr.GenerateConfig(webroot, fqdns)
	require.NoError(t, err)

	config2, err := mgr.GenerateConfig(webroot, fqdns)
	require.NoError(t, err)

	// Same inputs should produce the same configuration (deterministic port).
	assert.Equal(t, config1, config2)
}

func TestGenerateConfig_PythonRuntime(t *testing.T) {
	mgr := newTestNginxManager(t)

	webroot := &runtime.WebrootInfo{
		TenantName: "tenant2",
		Name:       "djangoapp",
		Runtime:    "python",
	}
	fqdns := []*FQDNInfo{
		{FQDN: "django.example.com"},
	}

	config, err := mgr.GenerateConfig(webroot, fqdns)
	require.NoError(t, err)

	// Verify proxy_pass to gunicorn socket.
	assert.Contains(t, config, "proxy_pass http://unix:/run/gunicorn/tenant2-djangoapp.sock")
	assert.Contains(t, config, "proxy_set_header Host $host")
	assert.Contains(t, config, "proxy_set_header X-Real-IP $remote_addr")
	assert.Contains(t, config, "proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for")
	assert.Contains(t, config, "proxy_set_header X-Forwarded-Proto $scheme")

	// Verify try_files for python.
	assert.Contains(t, config, "@app")

	// Should NOT contain PHP or Node directives.
	assert.NotContains(t, config, "fastcgi_pass")
	assert.NotContains(t, config, "proxy_pass http://127.0.0.1")
}

func TestGenerateConfig_RubyRuntime(t *testing.T) {
	mgr := newTestNginxManager(t)

	webroot := &runtime.WebrootInfo{
		TenantName: "tenant3",
		Name:       "railsapp",
		Runtime:    "ruby",
	}
	fqdns := []*FQDNInfo{
		{FQDN: "rails.example.com"},
	}

	config, err := mgr.GenerateConfig(webroot, fqdns)
	require.NoError(t, err)

	// Verify proxy_pass to puma socket.
	assert.Contains(t, config, "proxy_pass http://unix:/run/puma/tenant3-railsapp.sock")
	assert.Contains(t, config, "proxy_set_header Host $host")
	assert.Contains(t, config, "proxy_set_header X-Real-IP $remote_addr")
	assert.Contains(t, config, "proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for")
	assert.Contains(t, config, "proxy_set_header X-Forwarded-Proto $scheme")

	// Verify try_files for ruby.
	assert.Contains(t, config, "@app")

	// Should NOT contain PHP, Node, or Python directives.
	assert.NotContains(t, config, "fastcgi_pass")
	assert.NotContains(t, config, "proxy_pass http://127.0.0.1")
	assert.NotContains(t, config, "gunicorn")
}

func TestGenerateConfig_StaticRuntime(t *testing.T) {
	mgr := newTestNginxManager(t)

	webroot := &runtime.WebrootInfo{
		TenantName:   "tenant4",
		Name:         "staticsite",
		Runtime:      "static",
		PublicFolder: "",
	}
	fqdns := []*FQDNInfo{
		{FQDN: "static.example.com"},
	}

	config, err := mgr.GenerateConfig(webroot, fqdns)
	require.NoError(t, err)

	// Verify try_files with =404 for static.
	assert.Contains(t, config, "try_files $uri $uri/ =404")

	// Verify document root without public folder (under webroots/).
	assert.Contains(t, config, "root /var/www/storage/tenant4/webroots/staticsite")

	// Should NOT contain PHP, Node, Python, or Ruby directives.
	assert.NotContains(t, config, "fastcgi_pass")
	assert.NotContains(t, config, "proxy_pass")
	assert.NotContains(t, config, "index.php")
}

func TestGenerateConfig_WithSSL(t *testing.T) {
	tmpDir := t.TempDir()
	certDir := filepath.Join(tmpDir, "certs")
	cfg := Config{
		NginxConfigDir: tmpDir,
		CertDir:        certDir,
	}
	mgr := NewNginxManager(zerolog.Nop(), cfg)

	// Create actual certificate files on disk so SSL is enabled.
	fqdnCertDir := filepath.Join(certDir, "secure.example.com")
	require.NoError(t, os.MkdirAll(fqdnCertDir, 0700))
	require.NoError(t, os.WriteFile(filepath.Join(fqdnCertDir, "fullchain.pem"), []byte("cert"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(fqdnCertDir, "privkey.pem"), []byte("key"), 0600))

	webroot := &runtime.WebrootInfo{
		TenantName: "tenant1",
		Name:       "securesite",
		Runtime:    "static",
	}
	fqdns := []*FQDNInfo{
		{FQDN: "secure.example.com", SSLEnabled: true},
	}

	config, err := mgr.GenerateConfig(webroot, fqdns)
	require.NoError(t, err)

	// Verify SSL directives.
	assert.Contains(t, config, "listen 443 ssl")
	assert.Contains(t, config, "listen [::]:443 ssl")
	assert.Contains(t, config, "ssl_certificate     "+filepath.Join(certDir, "secure.example.com/fullchain.pem"))
	assert.Contains(t, config, "ssl_certificate_key "+filepath.Join(certDir, "secure.example.com/privkey.pem"))
	assert.Contains(t, config, "ssl_protocols       TLSv1.2 TLSv1.3")
	assert.Contains(t, config, "ssl_ciphers         HIGH:!aNULL:!MD5")
	assert.Contains(t, config, "ssl_prefer_server_ciphers on")

	// Verify HTTP-to-HTTPS redirect block.
	assert.Contains(t, config, "return 301 https://$host$request_uri")

	// Verify the redirect block has the correct listen directives.
	// The redirect server block should listen on port 80.
	lines := strings.Split(config, "\n")
	foundRedirectBlock := false
	for i, line := range lines {
		if strings.Contains(line, "return 301") {
			// Look back to verify listen 80 in the same server block.
			for j := i; j >= 0; j-- {
				if strings.Contains(lines[j], "listen 80") {
					foundRedirectBlock = true
					break
				}
				if strings.Contains(lines[j], "server {") {
					break
				}
			}
			break
		}
	}
	assert.True(t, foundRedirectBlock, "redirect server block should listen on port 80")
}

func TestGenerateConfig_WithSSL_CertsMissing_FallbackToHTTP(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := Config{
		NginxConfigDir: tmpDir,
		CertDir:        filepath.Join(tmpDir, "certs"), // No cert files created on disk.
	}
	mgr := NewNginxManager(zerolog.Nop(), cfg)

	webroot := &runtime.WebrootInfo{
		TenantName: "tenant1",
		Name:       "pendingssl",
		Runtime:    "static",
	}
	fqdns := []*FQDNInfo{
		{FQDN: "pending.example.com", SSLEnabled: true},
	}

	config, err := mgr.GenerateConfig(webroot, fqdns)
	require.NoError(t, err)

	// SSL should NOT be enabled because cert files don't exist on disk.
	assert.NotContains(t, config, "listen 443 ssl")
	assert.NotContains(t, config, "ssl_certificate")
	assert.NotContains(t, config, "ssl_certificate_key")
	assert.NotContains(t, config, "return 301 https://")

	// Should fall back to plain HTTP.
	assert.Contains(t, config, "listen 80")
	assert.Contains(t, config, "listen [::]:80")
	assert.Contains(t, config, "server_name pending.example.com")
}

func TestGenerateConfig_WithSSL_PartialCerts_FallbackToHTTP(t *testing.T) {
	tmpDir := t.TempDir()
	certDir := filepath.Join(tmpDir, "certs")
	cfg := Config{
		NginxConfigDir: tmpDir,
		CertDir:        certDir,
	}
	mgr := NewNginxManager(zerolog.Nop(), cfg)

	// Only create the fullchain.pem — missing privkey.pem.
	fqdnCertDir := filepath.Join(certDir, "partial.example.com")
	require.NoError(t, os.MkdirAll(fqdnCertDir, 0700))
	require.NoError(t, os.WriteFile(filepath.Join(fqdnCertDir, "fullchain.pem"), []byte("cert"), 0600))

	webroot := &runtime.WebrootInfo{
		TenantName: "tenant1",
		Name:       "partialssl",
		Runtime:    "static",
	}
	fqdns := []*FQDNInfo{
		{FQDN: "partial.example.com", SSLEnabled: true},
	}

	config, err := mgr.GenerateConfig(webroot, fqdns)
	require.NoError(t, err)

	// SSL should NOT be enabled because privkey.pem is missing.
	assert.NotContains(t, config, "listen 443 ssl")
	assert.NotContains(t, config, "ssl_certificate")
	assert.NotContains(t, config, "ssl_certificate_key")

	// Should fall back to plain HTTP.
	assert.Contains(t, config, "listen 80")
	assert.Contains(t, config, "listen [::]:80")
}

func TestGenerateConfig_WithSSL_NoHTTP(t *testing.T) {
	mgr := newTestNginxManager(t)

	webroot := &runtime.WebrootInfo{
		TenantName: "tenant1",
		Name:       "httpsite",
		Runtime:    "static",
	}
	fqdns := []*FQDNInfo{
		{FQDN: "http.example.com", SSLEnabled: false},
	}

	config, err := mgr.GenerateConfig(webroot, fqdns)
	require.NoError(t, err)

	// Verify no SSL directives are present.
	assert.NotContains(t, config, "listen 443 ssl")
	assert.NotContains(t, config, "ssl_certificate")
	assert.NotContains(t, config, "ssl_certificate_key")
	assert.NotContains(t, config, "return 301 https://")

	// Verify plain HTTP listen.
	assert.Contains(t, config, "listen 80")
	assert.Contains(t, config, "listen [::]:80")
}

func TestGenerateConfig_MultipleFQDNs(t *testing.T) {
	mgr := newTestNginxManager(t)

	webroot := &runtime.WebrootInfo{
		TenantName: "tenant1",
		Name:       "multisite",
		Runtime:    "static",
	}
	fqdns := []*FQDNInfo{
		{FQDN: "example.com"},
		{FQDN: "www.example.com"},
		{FQDN: "alias.example.com"},
	}

	config, err := mgr.GenerateConfig(webroot, fqdns)
	require.NoError(t, err)

	// Verify all FQDNs appear in server_name.
	assert.Contains(t, config, "server_name example.com www.example.com alias.example.com")
}

func TestGenerateConfig_MultipleFQDNs_SSLFromFirst(t *testing.T) {
	tmpDir := t.TempDir()
	certDir := filepath.Join(tmpDir, "certs")
	cfg := Config{
		NginxConfigDir: tmpDir,
		CertDir:        certDir,
	}
	mgr := NewNginxManager(zerolog.Nop(), cfg)

	// Create cert files for the SSL-enabled FQDN.
	fqdnCertDir := filepath.Join(certDir, "www.example.com")
	require.NoError(t, os.MkdirAll(fqdnCertDir, 0700))
	require.NoError(t, os.WriteFile(filepath.Join(fqdnCertDir, "fullchain.pem"), []byte("cert"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(fqdnCertDir, "privkey.pem"), []byte("key"), 0600))

	webroot := &runtime.WebrootInfo{
		TenantName: "tenant1",
		Name:       "multisite",
		Runtime:    "static",
	}
	fqdns := []*FQDNInfo{
		{FQDN: "example.com", SSLEnabled: false},
		{FQDN: "www.example.com", SSLEnabled: true},
		{FQDN: "alias.example.com", SSLEnabled: false},
	}

	config, err := mgr.GenerateConfig(webroot, fqdns)
	require.NoError(t, err)

	// SSL cert should come from the first ssl-enabled FQDN.
	assert.Contains(t, config, "ssl_certificate     "+filepath.Join(certDir, "www.example.com/fullchain.pem"))
	assert.Contains(t, config, "ssl_certificate_key "+filepath.Join(certDir, "www.example.com/privkey.pem"))
	assert.Contains(t, config, "listen 443 ssl")
}

func TestGenerateConfig_WithPublicFolder(t *testing.T) {
	mgr := newTestNginxManager(t)

	webroot := &runtime.WebrootInfo{
		TenantName:   "tenant1",
		Name:         "laravelapp",
		Runtime:      "php",
		PublicFolder: "public",
	}
	fqdns := []*FQDNInfo{
		{FQDN: "laravel.example.com"},
	}

	config, err := mgr.GenerateConfig(webroot, fqdns)
	require.NoError(t, err)

	// Verify root includes public folder (under webroots/).
	assert.Contains(t, config, "root /var/www/storage/tenant1/webroots/laravelapp/public")
}

func TestGenerateConfig_WithoutPublicFolder(t *testing.T) {
	mgr := newTestNginxManager(t)

	webroot := &runtime.WebrootInfo{
		TenantName:   "tenant1",
		Name:         "plainsite",
		Runtime:      "static",
		PublicFolder: "",
	}
	fqdns := []*FQDNInfo{
		{FQDN: "plain.example.com"},
	}

	config, err := mgr.GenerateConfig(webroot, fqdns)
	require.NoError(t, err)

	// Verify root is just the webroot storage path without a public subfolder.
	assert.Contains(t, config, "root /var/www/storage/tenant1/webroots/plainsite")
	// The root should NOT end with /public.
	assert.NotContains(t, config, "root /var/www/storage/tenant1/webroots/plainsite/public")
}

func TestGenerateConfig_NoFQDNs(t *testing.T) {
	mgr := newTestNginxManager(t)

	webroot := &runtime.WebrootInfo{
		TenantName: "tenant1",
		Name:       "nodomainsite",
		Runtime:    "static",
	}
	// Empty FQDN list.
	fqdns := []*FQDNInfo{}

	config, err := mgr.GenerateConfig(webroot, fqdns)
	require.NoError(t, err)

	// When no FQDNs, should use "_" as server_name.
	assert.Contains(t, config, "server_name _")

	// Should still generate a valid config.
	assert.Contains(t, config, "server {")
	assert.Contains(t, config, "listen 80")
}

func TestGenerateConfig_NilFQDNs(t *testing.T) {
	mgr := newTestNginxManager(t)

	webroot := &runtime.WebrootInfo{
		TenantName: "tenant1",
		Name:       "nodomainsite",
		Runtime:    "static",
	}

	config, err := mgr.GenerateConfig(webroot, nil)
	require.NoError(t, err)

	// When nil FQDNs, should use "_" as server_name.
	assert.Contains(t, config, "server_name _")
}

func TestGenerateConfig_AutoGeneratedComment(t *testing.T) {
	mgr := newTestNginxManager(t)

	webroot := &runtime.WebrootInfo{
		TenantName: "mytenant",
		Name:       "mywebroot",
		Runtime:    "static",
	}
	fqdns := []*FQDNInfo{{FQDN: "example.com"}}

	config, err := mgr.GenerateConfig(webroot, fqdns)
	require.NoError(t, err)

	// Verify the auto-generated comment header.
	assert.Contains(t, config, "Auto-generated by node-agent for mytenant/mywebroot")
	assert.Contains(t, config, "DO NOT EDIT MANUALLY")
}

func TestWriteConfig(t *testing.T) {
	mgr := newTestNginxManager(t)

	err := mgr.WriteConfig("tenant1", "mysite", "test-config-content")
	require.NoError(t, err)

	// Verify the file was written.
	confPath := filepath.Join(mgr.configDir, "sites-enabled", "tenant1_mysite.conf")
	data, err := os.ReadFile(confPath)
	require.NoError(t, err)
	assert.Equal(t, "test-config-content", string(data))
}

func TestWriteConfig_CreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := Config{NginxConfigDir: tmpDir}
	mgr := NewNginxManager(zerolog.Nop(), cfg)

	err := mgr.WriteConfig("tenant1", "mysite", "config-data")
	require.NoError(t, err)

	// Verify the sites-enabled directory was created.
	info, err := os.Stat(filepath.Join(tmpDir, "sites-enabled"))
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}

func TestRemoveConfig(t *testing.T) {
	mgr := newTestNginxManager(t)

	// First write a config.
	err := mgr.WriteConfig("tenant1", "mysite", "test-config-content")
	require.NoError(t, err)

	// Then remove it.
	err = mgr.RemoveConfig("tenant1", "mysite")
	require.NoError(t, err)

	// Verify the file no longer exists.
	confPath := filepath.Join(mgr.configDir, "sites-enabled", "tenant1_mysite.conf")
	_, err = os.Stat(confPath)
	assert.True(t, os.IsNotExist(err))
}

func TestRemoveConfig_NonExistent(t *testing.T) {
	mgr := newTestNginxManager(t)

	// Removing a non-existent config should not error.
	err := mgr.RemoveConfig("tenant1", "nonexistent")
	assert.NoError(t, err)
}

func TestComputeNodePort_Range(t *testing.T) {
	// Test that computeNodePort always returns a port in range [3000, 9999].
	testCases := []struct {
		tenant  string
		webroot string
	}{
		{"tenant1", "site1"},
		{"tenant2", "site2"},
		{"a", "b"},
		{"longtenantname", "longwebrootname"},
		{"", ""},
		{"test", "app123"},
	}

	for _, tc := range testCases {
		port := computeNodePort(tc.tenant, tc.webroot)
		assert.GreaterOrEqual(t, port, uint32(3000), "port for %s/%s should be >= 3000", tc.tenant, tc.webroot)
		assert.Less(t, port, uint32(10000), "port for %s/%s should be < 10000", tc.tenant, tc.webroot)
	}
}

func TestComputeNodePort_Deterministic(t *testing.T) {
	// Same inputs should always produce the same port.
	port1 := computeNodePort("tenant1", "site1")
	port2 := computeNodePort("tenant1", "site1")
	assert.Equal(t, port1, port2)
}

func TestComputeNodePort_DifferentInputs(t *testing.T) {
	// Different inputs should generally produce different ports
	// (not guaranteed for all inputs due to hash collisions, but these specific ones should differ).
	port1 := computeNodePort("tenant1", "site1")
	port2 := computeNodePort("tenant2", "site2")
	assert.NotEqual(t, port1, port2)
}

func TestInstallCertificate(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := Config{CertDir: tmpDir}
	mgr := NewNginxManager(zerolog.Nop(), cfg)

	cert := &CertificateInfo{
		FQDN:     "example.com",
		CertPEM:  "-----BEGIN CERTIFICATE-----\ncert-content\n-----END CERTIFICATE-----",
		KeyPEM:   "-----BEGIN PRIVATE KEY-----\nkey-content\n-----END PRIVATE KEY-----",
		ChainPEM: "-----BEGIN CERTIFICATE-----\nchain-content\n-----END CERTIFICATE-----",
	}

	err := mgr.InstallCertificate(nil, cert)
	require.NoError(t, err)

	// Verify fullchain.pem contains cert + chain.
	fullchain, err := os.ReadFile(filepath.Join(tmpDir, "example.com", "fullchain.pem"))
	require.NoError(t, err)
	assert.Contains(t, string(fullchain), "cert-content")
	assert.Contains(t, string(fullchain), "chain-content")

	// Verify privkey.pem.
	privkey, err := os.ReadFile(filepath.Join(tmpDir, "example.com", "privkey.pem"))
	require.NoError(t, err)
	assert.Contains(t, string(privkey), "key-content")
}

func TestInstallCertificate_NoChain(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := Config{CertDir: tmpDir}
	mgr := NewNginxManager(zerolog.Nop(), cfg)

	cert := &CertificateInfo{
		FQDN:     "example.com",
		CertPEM:  "cert-only",
		KeyPEM:   "key-only",
		ChainPEM: "",
	}

	err := mgr.InstallCertificate(nil, cert)
	require.NoError(t, err)

	// Verify fullchain.pem contains just the cert (no chain appended).
	fullchain, err := os.ReadFile(filepath.Join(tmpDir, "example.com", "fullchain.pem"))
	require.NoError(t, err)
	assert.Equal(t, "cert-only", string(fullchain))
}

func TestCleanOrphanedConfigs_RemovesOrphaned(t *testing.T) {
	mgr := newTestNginxManager(t)
	sitesDir := filepath.Join(mgr.configDir, "sites-enabled")
	require.NoError(t, os.MkdirAll(sitesDir, 0755))

	// Create some config files: two expected, one orphaned.
	require.NoError(t, os.WriteFile(filepath.Join(sitesDir, "tenant1_main.conf"), []byte("conf1"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(sitesDir, "tenant1_blog.conf"), []byte("conf2"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(sitesDir, "tenant2_old.conf"), []byte("orphaned"), 0644))

	expected := map[string]bool{
		"tenant1_main.conf": true,
		"tenant1_blog.conf": true,
	}

	removed, err := mgr.CleanOrphanedConfigs(expected)
	require.NoError(t, err)

	assert.Equal(t, []string{"tenant2_old.conf"}, removed)

	// Verify expected files still exist.
	assert.True(t, fileExists(filepath.Join(sitesDir, "tenant1_main.conf")))
	assert.True(t, fileExists(filepath.Join(sitesDir, "tenant1_blog.conf")))

	// Verify orphaned file was removed.
	assert.False(t, fileExists(filepath.Join(sitesDir, "tenant2_old.conf")))
}

func TestCleanOrphanedConfigs_SkipsNonWebrootFiles(t *testing.T) {
	mgr := newTestNginxManager(t)
	sitesDir := filepath.Join(mgr.configDir, "sites-enabled")
	require.NoError(t, os.MkdirAll(sitesDir, 0755))

	// Create non-webroot config files (no underscore in the base name).
	require.NoError(t, os.WriteFile(filepath.Join(sitesDir, "default.conf"), []byte("default"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(sitesDir, "custom.conf"), []byte("custom"), 0644))
	// Also a non-.conf file.
	require.NoError(t, os.WriteFile(filepath.Join(sitesDir, "readme.txt"), []byte("readme"), 0644))

	// No expected configs (empty set).
	removed, err := mgr.CleanOrphanedConfigs(map[string]bool{})
	require.NoError(t, err)

	// Nothing should be removed because none of the files match the webroot pattern.
	assert.Empty(t, removed)

	// Verify files still exist.
	assert.True(t, fileExists(filepath.Join(sitesDir, "default.conf")))
	assert.True(t, fileExists(filepath.Join(sitesDir, "custom.conf")))
	assert.True(t, fileExists(filepath.Join(sitesDir, "readme.txt")))
}

func TestCleanOrphanedConfigs_KeepsAllExpected(t *testing.T) {
	mgr := newTestNginxManager(t)
	sitesDir := filepath.Join(mgr.configDir, "sites-enabled")
	require.NoError(t, os.MkdirAll(sitesDir, 0755))

	// Create only expected files.
	require.NoError(t, os.WriteFile(filepath.Join(sitesDir, "t1_site.conf"), []byte("c1"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(sitesDir, "t2_site.conf"), []byte("c2"), 0644))

	expected := map[string]bool{
		"t1_site.conf": true,
		"t2_site.conf": true,
	}

	removed, err := mgr.CleanOrphanedConfigs(expected)
	require.NoError(t, err)

	assert.Empty(t, removed)

	// All files still present.
	assert.True(t, fileExists(filepath.Join(sitesDir, "t1_site.conf")))
	assert.True(t, fileExists(filepath.Join(sitesDir, "t2_site.conf")))
}

func TestCleanOrphanedConfigs_EmptyDirectory(t *testing.T) {
	mgr := newTestNginxManager(t)
	sitesDir := filepath.Join(mgr.configDir, "sites-enabled")
	require.NoError(t, os.MkdirAll(sitesDir, 0755))

	removed, err := mgr.CleanOrphanedConfigs(map[string]bool{})
	require.NoError(t, err)
	assert.Empty(t, removed)
}

func TestCleanOrphanedConfigs_MissingDirectory(t *testing.T) {
	// sites-enabled directory doesn't exist at all.
	mgr := newTestNginxManager(t)

	removed, err := mgr.CleanOrphanedConfigs(map[string]bool{})
	require.NoError(t, err)
	assert.Nil(t, removed)
}

func TestCleanOrphanedConfigs_MixedFiles(t *testing.T) {
	mgr := newTestNginxManager(t)
	sitesDir := filepath.Join(mgr.configDir, "sites-enabled")
	require.NoError(t, os.MkdirAll(sitesDir, 0755))

	// Mix of expected, orphaned, non-webroot, and non-conf files.
	require.NoError(t, os.WriteFile(filepath.Join(sitesDir, "tenant1_main.conf"), []byte("expected"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(sitesDir, "tenant2_old.conf"), []byte("orphaned1"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(sitesDir, "tenant3_stale.conf"), []byte("orphaned2"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(sitesDir, "default.conf"), []byte("system"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(sitesDir, "notes.txt"), []byte("ignore"), 0644))

	expected := map[string]bool{
		"tenant1_main.conf": true,
	}

	removed, err := mgr.CleanOrphanedConfigs(expected)
	require.NoError(t, err)

	// Should have removed exactly the two orphaned webroot configs.
	assert.Len(t, removed, 2)
	assert.Contains(t, removed, "tenant2_old.conf")
	assert.Contains(t, removed, "tenant3_stale.conf")

	// Expected and non-webroot files remain.
	assert.True(t, fileExists(filepath.Join(sitesDir, "tenant1_main.conf")))
	assert.True(t, fileExists(filepath.Join(sitesDir, "default.conf")))
	assert.True(t, fileExists(filepath.Join(sitesDir, "notes.txt")))

	// Orphaned files removed.
	assert.False(t, fileExists(filepath.Join(sitesDir, "tenant2_old.conf")))
	assert.False(t, fileExists(filepath.Join(sitesDir, "tenant3_stale.conf")))
}
