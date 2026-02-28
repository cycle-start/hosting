package core

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/edvin/hosting/internal/controlpanel/config"
	"github.com/edvin/hosting/internal/controlpanel/model"
)

type OIDCService struct {
	db        DB
	auth      *AuthService
	providers []config.OIDCProvider
	jwtSecret []byte
}

func NewOIDCService(db DB, auth *AuthService, providers []config.OIDCProvider, jwtSecret []byte) *OIDCService {
	return &OIDCService{
		db:        db,
		auth:      auth,
		providers: providers,
		jwtSecret: jwtSecret,
	}
}

// ProviderInfo is a public-facing provider summary.
type ProviderInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Providers returns the list of enabled OIDC providers.
func (s *OIDCService) Providers() []ProviderInfo {
	out := make([]ProviderInfo, len(s.providers))
	for i, p := range s.providers {
		out[i] = ProviderInfo{ID: p.ID, Name: p.Name}
	}
	return out
}

// GetProvider looks up a configured provider by ID.
func (s *OIDCService) GetProvider(id string) *config.OIDCProvider {
	for i := range s.providers {
		if s.providers[i].ID == id {
			return &s.providers[i]
		}
	}
	return nil
}

// oidcState is the JSON payload embedded in the OAuth state parameter.
type oidcState struct {
	Mode      string `json:"mode"`
	PartnerID string `json:"partner_id"`
	UserID    string `json:"user_id,omitempty"`
	Provider  string `json:"provider"`
	Nonce     string `json:"nonce"`
	Exp       int64  `json:"exp"`
}

// AuthorizeURL builds the OAuth authorization URL with a signed state parameter.
func (s *OIDCService) AuthorizeURL(provider *config.OIDCProvider, partnerID, mode, userID, callbackURL string) (string, error) {
	nonce := make([]byte, 16)
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("generate nonce: %w", err)
	}

	state := oidcState{
		Mode:      mode,
		PartnerID: partnerID,
		UserID:    userID,
		Provider:  provider.ID,
		Nonce:     hex.EncodeToString(nonce),
		Exp:       time.Now().Add(10 * time.Minute).Unix(),
	}

	stateJSON, err := json.Marshal(state)
	if err != nil {
		return "", fmt.Errorf("marshal state: %w", err)
	}

	signedState := s.signState(stateJSON)

	params := url.Values{
		"client_id":     {provider.ClientID},
		"redirect_uri":  {callbackURL},
		"response_type": {"code"},
		"scope":         {strings.Join(provider.Scopes, " ")},
		"state":         {signedState},
	}

	return provider.AuthURL + "?" + params.Encode(), nil
}

// CallbackResult holds the parsed result of an OIDC callback.
type CallbackResult struct {
	Mode      string
	PartnerID string
	UserID    string
	Provider  string
	Subject   string
	Email     string
}

// HandleCallback validates the state, exchanges the code for a token, and fetches userinfo.
func (s *OIDCService) HandleCallback(code, rawState, callbackURL string) (*CallbackResult, error) {
	state, err := s.verifyState(rawState)
	if err != nil {
		return nil, fmt.Errorf("invalid state: %w", err)
	}

	provider := s.GetProvider(state.Provider)
	if provider == nil {
		return nil, fmt.Errorf("unknown provider: %s", state.Provider)
	}

	// Exchange code for token
	tokenResp, err := s.exchangeCode(provider, code, callbackURL)
	if err != nil {
		return nil, fmt.Errorf("token exchange: %w", err)
	}

	// Fetch userinfo
	sub, email, err := s.fetchUserInfo(provider, tokenResp.AccessToken)
	if err != nil {
		return nil, fmt.Errorf("userinfo: %w", err)
	}

	return &CallbackResult{
		Mode:      state.Mode,
		PartnerID: state.PartnerID,
		UserID:    state.UserID,
		Provider:  state.Provider,
		Subject:   sub,
		Email:     email,
	}, nil
}

// LoginByOIDC looks up a user by their OIDC connection and issues a JWT.
func (s *OIDCService) LoginByOIDC(ctx context.Context, partnerID, provider, subject string) (string, *model.User, error) {
	var userID string
	err := s.db.QueryRow(ctx,
		`SELECT user_id FROM user_oidc_connections
		 WHERE partner_id = $1 AND provider = $2 AND subject = $3`,
		partnerID, provider, subject,
	).Scan(&userID)
	if err != nil {
		return "", nil, fmt.Errorf("no oidc connection found")
	}

	var user model.User
	err = s.db.QueryRow(ctx,
		`SELECT id, partner_id, email, password_hash, display_name, locale, last_customer_id, created_at, updated_at
		 FROM users WHERE id = $1`, userID,
	).Scan(&user.ID, &user.PartnerID, &user.Email, &user.PasswordHash,
		&user.DisplayName, &user.Locale, &user.LastCustomerID, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		return "", nil, fmt.Errorf("get user: %w", err)
	}

	token, err := s.auth.IssueToken(&user)
	if err != nil {
		return "", nil, fmt.Errorf("issue token: %w", err)
	}

	return token, &user, nil
}

// Connect stores an OIDC connection for a user.
func (s *OIDCService) Connect(ctx context.Context, userID, partnerID, provider, subject, email string) error {
	id := uuid.New().String()
	_, err := s.db.Exec(ctx,
		`INSERT INTO user_oidc_connections (id, user_id, partner_id, provider, subject, email)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		id, userID, partnerID, provider, subject, email,
	)
	if err != nil {
		return fmt.Errorf("insert oidc connection: %w", err)
	}
	return nil
}

// OIDCConnection represents a stored OIDC connection.
type OIDCConnection struct {
	Provider  string    `json:"provider"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

// ListConnections returns all OIDC connections for a user.
func (s *OIDCService) ListConnections(ctx context.Context, userID string) ([]OIDCConnection, error) {
	rows, err := s.db.Query(ctx,
		`SELECT provider, email, created_at FROM user_oidc_connections WHERE user_id = $1 ORDER BY created_at`, userID,
	)
	if err != nil {
		return nil, fmt.Errorf("list oidc connections: %w", err)
	}
	defer rows.Close()

	var conns []OIDCConnection
	for rows.Next() {
		var c OIDCConnection
		if err := rows.Scan(&c.Provider, &c.Email, &c.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan oidc connection: %w", err)
		}
		conns = append(conns, c)
	}
	return conns, rows.Err()
}

// Disconnect removes an OIDC connection for a user.
func (s *OIDCService) Disconnect(ctx context.Context, userID, provider string) error {
	tag, err := s.db.Exec(ctx,
		`DELETE FROM user_oidc_connections WHERE user_id = $1 AND provider = $2`, userID, provider,
	)
	if err != nil {
		return fmt.Errorf("delete oidc connection: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("connection not found")
	}
	return nil
}

// signState HMAC-signs the state JSON and returns a base64url-encoded "payload.signature" string.
func (s *OIDCService) signState(stateJSON []byte) string {
	payload := base64.RawURLEncoding.EncodeToString(stateJSON)
	mac := hmac.New(sha256.New, s.jwtSecret)
	mac.Write([]byte(payload))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return payload + "." + sig
}

// verifyState verifies the HMAC signature and decodes the state.
func (s *OIDCService) verifyState(raw string) (*oidcState, error) {
	parts := strings.SplitN(raw, ".", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid state format")
	}

	payload := parts[0]
	sig, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("decode signature: %w", err)
	}

	mac := hmac.New(sha256.New, s.jwtSecret)
	mac.Write([]byte(payload))
	if !hmac.Equal(mac.Sum(nil), sig) {
		return nil, fmt.Errorf("invalid signature")
	}

	stateJSON, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		return nil, fmt.Errorf("decode payload: %w", err)
	}

	var state oidcState
	if err := json.Unmarshal(stateJSON, &state); err != nil {
		return nil, fmt.Errorf("unmarshal state: %w", err)
	}

	if time.Now().Unix() > state.Exp {
		return nil, fmt.Errorf("state expired")
	}

	return &state, nil
}

// tokenResponse holds the OAuth token endpoint response.
type tokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
}

func (s *OIDCService) exchangeCode(provider *config.OIDCProvider, code, callbackURL string) (*tokenResponse, error) {
	data := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {callbackURL},
		"client_id":     {provider.ClientID},
		"client_secret": {provider.ClientSecret},
	}

	resp, err := http.PostForm(provider.TokenURL, data)
	if err != nil {
		return nil, fmt.Errorf("post token endpoint: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read token response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token endpoint returned %d: %s", resp.StatusCode, string(body))
	}

	var tok tokenResponse
	if err := json.Unmarshal(body, &tok); err != nil {
		return nil, fmt.Errorf("decode token response: %w", err)
	}

	if tok.AccessToken == "" {
		return nil, fmt.Errorf("empty access token")
	}

	return &tok, nil
}

func (s *OIDCService) fetchUserInfo(provider *config.OIDCProvider, accessToken string) (sub string, email string, err error) {
	req, err := http.NewRequest("GET", provider.UserInfoURL, nil)
	if err != nil {
		return "", "", fmt.Errorf("create userinfo request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("fetch userinfo: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", fmt.Errorf("read userinfo response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("userinfo endpoint returned %d: %s", resp.StatusCode, string(body))
	}

	var info struct {
		Sub   string `json:"sub"`
		Email string `json:"email"`
	}
	if err := json.Unmarshal(body, &info); err != nil {
		return "", "", fmt.Errorf("decode userinfo: %w", err)
	}

	if info.Sub == "" {
		return "", "", fmt.Errorf("missing sub claim")
	}

	return info.Sub, info.Email, nil
}
