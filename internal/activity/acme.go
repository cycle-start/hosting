package activity

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"time"

	"golang.org/x/crypto/acme"
)

// ACMEActivity handles ACME certificate provisioning.
type ACMEActivity struct {
	email        string
	directoryURL string
}

// NewACMEActivity creates a new ACMEActivity.
func NewACMEActivity(email, directoryURL string) *ACMEActivity {
	return &ACMEActivity{email: email, directoryURL: directoryURL}
}

// ACMEOrderParams holds parameters for ordering a certificate.
type ACMEOrderParams struct {
	FQDN string
}

// ACMEOrderResult contains the order URL and authorizations.
type ACMEOrderResult struct {
	OrderURL   string
	AuthzURLs  []string
	AccountKey []byte // PEM-encoded ECDSA private key
}

// CreateOrder creates an ACME account (if needed) and submits a new order.
func (a *ACMEActivity) CreateOrder(ctx context.Context, params ACMEOrderParams) (*ACMEOrderResult, error) {
	// Generate an account key.
	accountKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate account key: %w", err)
	}

	client := &acme.Client{
		Key:          accountKey,
		DirectoryURL: a.directoryURL,
	}

	// Register account (or retrieve existing).
	acct := &acme.Account{Contact: []string{"mailto:" + a.email}}
	_, err = client.Register(ctx, acct, acme.AcceptTOS)
	if err != nil && err != acme.ErrAccountAlreadyExists {
		return nil, fmt.Errorf("register ACME account: %w", err)
	}

	// Create order.
	order, err := client.AuthorizeOrder(ctx, acme.DomainIDs(params.FQDN))
	if err != nil {
		return nil, fmt.Errorf("authorize order: %w", err)
	}

	// Serialize account key.
	keyDER, err := x509.MarshalECPrivateKey(accountKey)
	if err != nil {
		return nil, fmt.Errorf("marshal account key: %w", err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	return &ACMEOrderResult{
		OrderURL:   order.URI,
		AuthzURLs:  order.AuthzURLs,
		AccountKey: keyPEM,
	}, nil
}

// ACMEChallengeParams holds parameters for getting the HTTP-01 challenge details.
type ACMEChallengeParams struct {
	AuthzURL   string
	AccountKey []byte // PEM-encoded
}

// ACMEChallengeResult holds the challenge token and response.
type ACMEChallengeResult struct {
	ChallengeURL string
	Token        string
	KeyAuth      string
}

// GetHTTP01Challenge retrieves the HTTP-01 challenge for an authorization.
func (a *ACMEActivity) GetHTTP01Challenge(ctx context.Context, params ACMEChallengeParams) (*ACMEChallengeResult, error) {
	accountKey, err := parseECKey(params.AccountKey)
	if err != nil {
		return nil, err
	}

	client := &acme.Client{Key: accountKey, DirectoryURL: a.directoryURL}

	authz, err := client.GetAuthorization(ctx, params.AuthzURL)
	if err != nil {
		return nil, fmt.Errorf("get authorization: %w", err)
	}

	// Find the HTTP-01 challenge.
	var challenge *acme.Challenge
	for _, c := range authz.Challenges {
		if c.Type == "http-01" {
			challenge = c
			break
		}
	}
	if challenge == nil {
		return nil, fmt.Errorf("no http-01 challenge found")
	}

	// Compute key authorization.
	keyAuth, err := client.HTTP01ChallengeResponse(challenge.Token)
	if err != nil {
		return nil, fmt.Errorf("compute key auth: %w", err)
	}

	return &ACMEChallengeResult{
		ChallengeURL: challenge.URI,
		Token:        challenge.Token,
		KeyAuth:      keyAuth,
	}, nil
}

// PlaceHTTP01ChallengeParams is used to write the challenge file to a node.
type PlaceHTTP01ChallengeParams struct {
	WebrootPath string // e.g. /var/www/storage/{tenant}/{webroot}/{public_folder}
	Token       string
	KeyAuth     string
}

// ACMEAcceptParams holds params for accepting the challenge.
type ACMEAcceptParams struct {
	ChallengeURL string
	AccountKey   []byte
}

// AcceptChallenge tells the ACME server we're ready for validation.
func (a *ACMEActivity) AcceptChallenge(ctx context.Context, params ACMEAcceptParams) error {
	accountKey, err := parseECKey(params.AccountKey)
	if err != nil {
		return err
	}

	client := &acme.Client{Key: accountKey, DirectoryURL: a.directoryURL}

	_, err = client.Accept(ctx, &acme.Challenge{URI: params.ChallengeURL})
	if err != nil {
		return fmt.Errorf("accept challenge: %w", err)
	}

	return nil
}

// ACMEFinalizeParams holds params for finalizing the order.
type ACMEFinalizeParams struct {
	OrderURL   string
	FQDN       string
	AccountKey []byte
}

// ACMEFinalizeResult holds the issued certificate PEM data.
type ACMEFinalizeResult struct {
	CertPEM   string
	KeyPEM    string
	ChainPEM  string
	IssuedAt  time.Time
	ExpiresAt time.Time
}

// FinalizeOrder waits for the order to be ready, creates a CSR, and finalizes.
func (a *ACMEActivity) FinalizeOrder(ctx context.Context, params ACMEFinalizeParams) (*ACMEFinalizeResult, error) {
	accountKey, err := parseECKey(params.AccountKey)
	if err != nil {
		return nil, err
	}

	client := &acme.Client{Key: accountKey, DirectoryURL: a.directoryURL}

	// Wait for order to be ready.
	order, err := client.WaitOrder(ctx, params.OrderURL)
	if err != nil {
		return nil, fmt.Errorf("wait order: %w", err)
	}

	// Generate certificate key.
	certKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate cert key: %w", err)
	}

	// Create CSR.
	csr, err := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{
		DNSNames: []string{params.FQDN},
	}, certKey)
	if err != nil {
		return nil, fmt.Errorf("create CSR: %w", err)
	}

	// Finalize order.
	certDER, _, err := client.CreateOrderCert(ctx, order.FinalizeURL, csr, true)
	if err != nil {
		return nil, fmt.Errorf("create order cert: %w", err)
	}

	// Encode cert PEM.
	var certPEM, chainPEM []byte
	for i, der := range certDER {
		block := &pem.Block{Type: "CERTIFICATE", Bytes: der}
		if i == 0 {
			certPEM = pem.EncodeToMemory(block)
		} else {
			chainPEM = append(chainPEM, pem.EncodeToMemory(block)...)
		}
	}

	// Encode key PEM.
	keyDER, err := x509.MarshalECPrivateKey(certKey)
	if err != nil {
		return nil, fmt.Errorf("marshal cert key: %w", err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	// Parse cert to get dates.
	cert, err := x509.ParseCertificate(certDER[0])
	if err != nil {
		return nil, fmt.Errorf("parse issued cert: %w", err)
	}

	return &ACMEFinalizeResult{
		CertPEM:   string(certPEM),
		KeyPEM:    string(keyPEM),
		ChainPEM:  string(chainPEM),
		IssuedAt:  cert.NotBefore,
		ExpiresAt: cert.NotAfter,
	}, nil
}

// CleanupHTTP01ChallengeParams is used to remove the challenge file from a node.
type CleanupHTTP01ChallengeParams struct {
	WebrootPath string
	Token       string
}

func parseECKey(keyPEM []byte) (*ecdsa.PrivateKey, error) {
	block, _ := pem.Decode(keyPEM)
	if block == nil {
		return nil, fmt.Errorf("failed to decode account key PEM")
	}
	key, err := x509.ParseECPrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse EC key: %w", err)
	}
	return key, nil
}
