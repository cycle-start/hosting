package core

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"math/big"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/edvin/hosting/internal/model"
	"github.com/edvin/hosting/internal/platform"
)

type OIDCService struct {
	db        DB
	issuerURL string

	mu         sync.Mutex
	signingKey *rsa.PrivateKey
	keyID      string
}

func NewOIDCService(db DB, issuerURL string) *OIDCService {
	return &OIDCService{db: db, issuerURL: issuerURL}
}

// EnsureSigningKey generates and stores an RSA-2048 signing key if none exists.
func (s *OIDCService) EnsureSigningKey(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.signingKey != nil {
		return nil
	}

	// Check DB for existing active key.
	var k model.OIDCSigningKey
	err := s.db.QueryRow(ctx,
		`SELECT id, public_key_pem, private_key_pem FROM oidc_signing_keys WHERE active = true LIMIT 1`,
	).Scan(&k.ID, &k.PublicKeyPEM, &k.PrivateKeyPEM)
	if err == nil {
		block, _ := pem.Decode([]byte(k.PrivateKeyPEM))
		if block == nil {
			return fmt.Errorf("oidc: invalid PEM in signing key %s", k.ID)
		}
		key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			return fmt.Errorf("oidc: parse private key: %w", err)
		}
		s.signingKey = key
		s.keyID = k.ID
		return nil
	}

	// Generate new key.
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return fmt.Errorf("oidc: generate RSA key: %w", err)
	}

	privPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})
	pubPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PUBLIC KEY",
		Bytes: x509.MarshalPKCS1PublicKey(&key.PublicKey),
	})

	id := platform.NewID()
	_, err = s.db.Exec(ctx,
		`INSERT INTO oidc_signing_keys (id, algorithm, public_key_pem, private_key_pem, active) VALUES ($1, $2, $3, $4, true)`,
		id, "RS256", string(pubPEM), string(privPEM),
	)
	if err != nil {
		return fmt.Errorf("oidc: store signing key: %w", err)
	}

	s.signingKey = key
	s.keyID = id
	return nil
}

// JWKS returns the public key in JWK Set format.
func (s *OIDCService) JWKS() (json.RawMessage, error) {
	s.mu.Lock()
	key := s.signingKey
	kid := s.keyID
	s.mu.Unlock()

	if key == nil {
		return nil, fmt.Errorf("oidc: no signing key loaded")
	}

	n := base64.RawURLEncoding.EncodeToString(key.PublicKey.N.Bytes())
	e := base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.PublicKey.E)).Bytes())

	jwks := map[string]any{
		"keys": []map[string]any{
			{
				"kty": "RSA",
				"use": "sig",
				"alg": "RS256",
				"kid": kid,
				"n":   n,
				"e":   e,
			},
		},
	}

	data, err := json.Marshal(jwks)
	if err != nil {
		return nil, fmt.Errorf("oidc: marshal jwks: %w", err)
	}
	return data, nil
}

// Discovery returns the OpenID Connect discovery document.
func (s *OIDCService) Discovery() map[string]any {
	return map[string]any{
		"issuer":                 s.issuerURL,
		"authorization_endpoint": s.issuerURL + "/oidc/authorize",
		"token_endpoint":         s.issuerURL + "/oidc/token",
		"jwks_uri":               s.issuerURL + "/oidc/jwks",
		"response_types_supported": []string{"code"},
		"subject_types_supported":  []string{"public"},
		"id_token_signing_alg_values_supported": []string{"RS256"},
		"scopes_supported":                      []string{"openid"},
		"grant_types_supported":                 []string{"authorization_code"},
	}
}

// CreateLoginSession creates a short-lived login session for a tenant.
func (s *OIDCService) CreateLoginSession(ctx context.Context, tenantID string) (*model.OIDCLoginSession, error) {
	session := &model.OIDCLoginSession{
		ID:        platform.NewID(),
		TenantID:  tenantID,
		ExpiresAt: time.Now().Add(30 * time.Second),
		CreatedAt: time.Now(),
	}

	_, err := s.db.Exec(ctx,
		`INSERT INTO oidc_login_sessions (id, tenant_id, expires_at, created_at) VALUES ($1, $2, $3, $4)`,
		session.ID, session.TenantID, session.ExpiresAt, session.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("oidc: create login session: %w", err)
	}
	return session, nil
}

// ValidateLoginSession checks a login session is valid and marks it used.
func (s *OIDCService) ValidateLoginSession(ctx context.Context, sessionID string) (*model.OIDCLoginSession, error) {
	var sess model.OIDCLoginSession
	err := s.db.QueryRow(ctx,
		`SELECT id, tenant_id, expires_at, used FROM oidc_login_sessions WHERE id = $1`, sessionID,
	).Scan(&sess.ID, &sess.TenantID, &sess.ExpiresAt, &sess.Used)
	if err != nil {
		return nil, fmt.Errorf("oidc: login session not found: %w", err)
	}

	if sess.Used {
		return nil, fmt.Errorf("oidc: login session already used")
	}
	if time.Now().After(sess.ExpiresAt) {
		return nil, fmt.Errorf("oidc: login session expired")
	}

	_, err = s.db.Exec(ctx, `UPDATE oidc_login_sessions SET used = true WHERE id = $1`, sessionID)
	if err != nil {
		return nil, fmt.Errorf("oidc: mark login session used: %w", err)
	}

	return &sess, nil
}

// ValidateClient validates an OIDC client's credentials and redirect URI.
func (s *OIDCService) ValidateClient(ctx context.Context, clientID, secret, redirectURI string) (*model.OIDCClient, error) {
	var client model.OIDCClient
	err := s.db.QueryRow(ctx,
		`SELECT id, secret_hash, name, redirect_uris FROM oidc_clients WHERE id = $1`, clientID,
	).Scan(&client.ID, &client.SecretHash, &client.Name, &client.RedirectURIs)
	if err != nil {
		return nil, fmt.Errorf("oidc: client %q not found: %w", clientID, err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(client.SecretHash), []byte(secret)); err != nil {
		return nil, fmt.Errorf("oidc: invalid client secret")
	}

	if redirectURI != "" {
		valid := false
		for _, uri := range client.RedirectURIs {
			if uri == redirectURI {
				valid = true
				break
			}
		}
		if !valid {
			return nil, fmt.Errorf("oidc: invalid redirect_uri %q", redirectURI)
		}
	}

	return &client, nil
}

// CreateAuthCode creates an authorization code for the given client and tenant.
func (s *OIDCService) CreateAuthCode(ctx context.Context, clientID, tenantID, redirectURI, scope, nonce string) (string, error) {
	codeBytes := make([]byte, 32)
	if _, err := rand.Read(codeBytes); err != nil {
		return "", fmt.Errorf("oidc: generate auth code: %w", err)
	}
	code := base64.RawURLEncoding.EncodeToString(codeBytes)

	_, err := s.db.Exec(ctx,
		`INSERT INTO oidc_auth_codes (code, client_id, tenant_id, redirect_uri, scope, nonce, expires_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		code, clientID, tenantID, redirectURI, scope, nonce,
		time.Now().Add(60*time.Second),
	)
	if err != nil {
		return "", fmt.Errorf("oidc: store auth code: %w", err)
	}
	return code, nil
}

// ExchangeCode validates an auth code and returns an ID token.
func (s *OIDCService) ExchangeCode(ctx context.Context, code, clientID, clientSecret, redirectURI string) (string, error) {
	// Validate client.
	_, err := s.ValidateClient(ctx, clientID, clientSecret, redirectURI)
	if err != nil {
		return "", err
	}

	// Look up and validate auth code.
	var ac model.OIDCAuthCode
	err = s.db.QueryRow(ctx,
		`SELECT code, client_id, tenant_id, redirect_uri, scope, nonce, expires_at, used
		 FROM oidc_auth_codes WHERE code = $1`, code,
	).Scan(&ac.Code, &ac.ClientID, &ac.TenantID, &ac.RedirectURI,
		&ac.Scope, &ac.Nonce, &ac.ExpiresAt, &ac.Used)
	if err != nil {
		return "", fmt.Errorf("oidc: auth code not found: %w", err)
	}

	if ac.Used {
		return "", fmt.Errorf("oidc: auth code already used")
	}
	if time.Now().After(ac.ExpiresAt) {
		return "", fmt.Errorf("oidc: auth code expired")
	}
	if ac.ClientID != clientID {
		return "", fmt.Errorf("oidc: auth code client mismatch")
	}
	if redirectURI != "" && ac.RedirectURI != redirectURI {
		return "", fmt.Errorf("oidc: auth code redirect_uri mismatch")
	}

	// Mark used.
	_, err = s.db.Exec(ctx, `UPDATE oidc_auth_codes SET used = true WHERE code = $1`, code)
	if err != nil {
		return "", fmt.Errorf("oidc: mark auth code used: %w", err)
	}

	// Sign ID token with tenant as the subject.
	token, err := s.signIDToken(ac.TenantID, clientID, ac.Nonce)
	if err != nil {
		return "", err
	}

	return token, nil
}

// signIDToken produces a signed JWT ID token with the tenant as subject.
func (s *OIDCService) signIDToken(tenantID, clientID, nonce string) (string, error) {
	s.mu.Lock()
	key := s.signingKey
	kid := s.keyID
	s.mu.Unlock()

	if key == nil {
		return "", fmt.Errorf("oidc: no signing key loaded")
	}

	now := time.Now()
	claims := map[string]any{
		"iss":                s.issuerURL,
		"sub":                tenantID,
		"aud":                clientID,
		"exp":                now.Add(1 * time.Hour).Unix(),
		"iat":                now.Unix(),
		"preferred_username": tenantID,
		"groups":             []string{tenantID},
	}
	if nonce != "" {
		claims["nonce"] = nonce
	}

	header := map[string]string{
		"alg": "RS256",
		"typ": "JWT",
		"kid": kid,
	}

	headerJSON, _ := json.Marshal(header)
	claimsJSON, _ := json.Marshal(claims)

	headerB64 := base64.RawURLEncoding.EncodeToString(headerJSON)
	claimsB64 := base64.RawURLEncoding.EncodeToString(claimsJSON)

	signingInput := headerB64 + "." + claimsB64

	h := sha256.New()
	h.Write([]byte(signingInput))
	digest := h.Sum(nil)

	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest)
	if err != nil {
		return "", fmt.Errorf("oidc: sign token: %w", err)
	}

	sigB64 := base64.RawURLEncoding.EncodeToString(sig)
	return signingInput + "." + sigB64, nil
}

// CreateClient creates an OIDC client with a bcrypt-hashed secret.
func (s *OIDCService) CreateClient(ctx context.Context, id, secret, name string, redirectURIs []string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(secret), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("oidc: hash client secret: %w", err)
	}

	_, err = s.db.Exec(ctx,
		`INSERT INTO oidc_clients (id, secret_hash, name, redirect_uris) VALUES ($1, $2, $3, $4)
		 ON CONFLICT (id) DO UPDATE SET secret_hash = $2, name = $3, redirect_uris = $4`,
		id, string(hash), name, redirectURIs,
	)
	if err != nil {
		return fmt.Errorf("oidc: create client: %w", err)
	}
	return nil
}
