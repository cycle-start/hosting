package core

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/argon2"

	"github.com/edvin/hosting/internal/controlpanel/model"
)

type AuthService struct {
	db        DB
	jwtSecret []byte
	jwtIssuer string
}

func NewAuthService(db DB, jwtSecret, jwtIssuer string) *AuthService {
	return &AuthService{
		db:        db,
		jwtSecret: []byte(jwtSecret),
		jwtIssuer: jwtIssuer,
	}
}

// Login authenticates a user by email and password within a partner, returning a JWT on success.
func (s *AuthService) Login(ctx context.Context, email, password, partnerID string) (string, error) {
	var user model.User
	err := s.db.QueryRow(ctx,
		`SELECT id, partner_id, email, password_hash, display_name, locale, last_customer_id, created_at, updated_at
		 FROM users WHERE email = $1 AND partner_id = $2`, email, partnerID,
	).Scan(&user.ID, &user.PartnerID, &user.Email, &user.PasswordHash,
		&user.DisplayName, &user.Locale, &user.LastCustomerID, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		return "", fmt.Errorf("invalid credentials")
	}

	if !verifyArgon2(password, user.PasswordHash) {
		return "", fmt.Errorf("invalid credentials")
	}

	token, err := s.IssueToken(&user)
	if err != nil {
		return "", fmt.Errorf("issue token: %w", err)
	}

	return token, nil
}

// IssueToken creates a signed JWT for the given user.
func (s *AuthService) IssueToken(user *model.User) (string, error) {
	now := time.Now()
	claims := model.JWTClaims{
		Sub:       user.ID,
		PartnerID: user.PartnerID,
		Email:     user.Email,
		Locale:    user.Locale,
		Iat:     now.Unix(),
		Exp:     now.Add(24 * time.Hour).Unix(),
		Iss:     s.jwtIssuer,
	}
	return s.signJWT(claims)
}

// ValidateToken parses and validates a JWT, returning the claims.
func (s *AuthService) ValidateToken(tokenStr string) (*model.JWTClaims, error) {
	parts := strings.Split(tokenStr, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid token format")
	}

	// Verify signature
	signingInput := parts[0] + "." + parts[1]
	expectedSig := s.hmacSign([]byte(signingInput))
	actualSig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, fmt.Errorf("invalid signature encoding")
	}
	if subtle.ConstantTimeCompare(expectedSig, actualSig) != 1 {
		return nil, fmt.Errorf("invalid signature")
	}

	// Decode payload
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("invalid payload encoding")
	}

	var claims model.JWTClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, fmt.Errorf("invalid claims: %w", err)
	}

	if time.Now().Unix() > claims.Exp {
		return nil, fmt.Errorf("token expired")
	}

	return &claims, nil
}

func (s *AuthService) signJWT(claims model.JWTClaims) (string, error) {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))

	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	payload := base64.RawURLEncoding.EncodeToString(claimsJSON)

	signingInput := header + "." + payload
	sig := base64.RawURLEncoding.EncodeToString(s.hmacSign([]byte(signingInput)))

	return signingInput + "." + sig, nil
}

func (s *AuthService) hmacSign(data []byte) []byte {
	mac := hmac.New(sha256.New, s.jwtSecret)
	mac.Write(data)
	return mac.Sum(nil)
}

// verifyArgon2 checks a password against a PHC-format argon2id hash.
// Format: $argon2id$v=19$m=65536,t=3,p=4$<salt>$<hash>
func verifyArgon2(password, hash string) bool {
	parts := strings.Split(hash, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false
	}

	// Parse parameters from "m=65536,t=3,p=4"
	paramParts := strings.Split(parts[3], ",")
	if len(paramParts) != 3 {
		return false
	}

	memory, err := parseParam(paramParts[0], "m=")
	if err != nil {
		return false
	}
	iterations, err := parseParam(paramParts[1], "t=")
	if err != nil {
		return false
	}
	parallelism, err := parseParam(paramParts[2], "p=")
	if err != nil {
		return false
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false
	}
	expectedHash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false
	}

	computed := argon2.IDKey([]byte(password), salt, uint32(iterations), uint32(memory), uint8(parallelism), uint32(len(expectedHash)))
	return subtle.ConstantTimeCompare(computed, expectedHash) == 1
}

func parseParam(s, prefix string) (int, error) {
	if !strings.HasPrefix(s, prefix) {
		return 0, fmt.Errorf("missing prefix %s", prefix)
	}
	return strconv.Atoi(s[len(prefix):])
}
