package activity

import (
	"context"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// CertificateActivity contains activities for certificate management.
type CertificateActivity struct {
	coreDB *pgxpool.Pool
}

// NewCertificateActivity creates a new CertificateActivity struct.
func NewCertificateActivity(coreDB *pgxpool.Pool) *CertificateActivity {
	return &CertificateActivity{coreDB: coreDB}
}

// ValidateCustomCert parses an X.509 certificate and verifies the private key matches.
func (a *CertificateActivity) ValidateCustomCert(ctx context.Context, certPEM, keyPEM string) error {
	_, err := tls.X509KeyPair([]byte(certPEM), []byte(keyPEM))
	if err != nil {
		return fmt.Errorf("certificate and key do not match: %w", err)
	}

	// Also validate the certificate is parseable and not expired.
	block, _ := pem.Decode([]byte(certPEM))
	if block == nil {
		return fmt.Errorf("failed to decode certificate PEM")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return fmt.Errorf("failed to parse certificate: %w", err)
	}

	if time.Now().After(cert.NotAfter) {
		return fmt.Errorf("certificate has expired at %s", cert.NotAfter)
	}

	// Verify the key type is supported.
	keyBlock, _ := pem.Decode([]byte(keyPEM))
	if keyBlock == nil {
		return fmt.Errorf("failed to decode private key PEM")
	}

	key, err := parsePrivateKey(keyBlock.Bytes)
	if err != nil {
		return fmt.Errorf("failed to parse private key: %w", err)
	}
	_ = key

	return nil
}

// parsePrivateKey tries to parse a private key in PKCS8, PKCS1, or EC formats.
func parsePrivateKey(der []byte) (any, error) {
	if key, err := x509.ParsePKCS8PrivateKey(der); err == nil {
		switch key.(type) {
		case *rsa.PrivateKey, *ecdsa.PrivateKey, ed25519.PrivateKey:
			return key, nil
		default:
			return nil, fmt.Errorf("unsupported private key type in PKCS8")
		}
	}
	if key, err := x509.ParsePKCS1PrivateKey(der); err == nil {
		return key, nil
	}
	if key, err := x509.ParseECPrivateKey(der); err == nil {
		return key, nil
	}
	return nil, fmt.Errorf("failed to parse private key")
}

// StoreCertParams holds parameters for storing a certificate.
type StoreCertParams struct {
	ID        string
	CertPEM   string
	KeyPEM    string
	ChainPEM  string
	IssuedAt  time.Time
	ExpiresAt time.Time
}

// StoreCertificate updates a certificate row with PEM data and timestamps.
func (a *CertificateActivity) StoreCertificate(ctx context.Context, params StoreCertParams) error {
	_, err := a.coreDB.Exec(ctx,
		`UPDATE certificates
		 SET cert_pem = $1, key_pem = $2, chain_pem = $3,
		     issued_at = $4, expires_at = $5, updated_at = now()
		 WHERE id = $6`,
		params.CertPEM, params.KeyPEM, params.ChainPEM,
		params.IssuedAt, params.ExpiresAt, params.ID,
	)
	if err != nil {
		return fmt.Errorf("store certificate: %w", err)
	}
	return nil
}

// DeactivateOtherCerts sets is_active=false for all other certificates on this FQDN.
func (a *CertificateActivity) DeactivateOtherCerts(ctx context.Context, fqdnID, activeCertID string) error {
	_, err := a.coreDB.Exec(ctx,
		`UPDATE certificates SET is_active = false, updated_at = now()
		 WHERE fqdn_id = $1 AND id != $2 AND is_active = true`,
		fqdnID, activeCertID,
	)
	if err != nil {
		return fmt.Errorf("deactivate other certs: %w", err)
	}
	return nil
}

// ActivateCertificate sets is_active=true and status=active for a certificate.
func (a *CertificateActivity) ActivateCertificate(ctx context.Context, certID string) error {
	_, err := a.coreDB.Exec(ctx,
		`UPDATE certificates SET is_active = true, status = 'active', updated_at = now()
		 WHERE id = $1`, certID,
	)
	if err != nil {
		return fmt.Errorf("activate certificate: %w", err)
	}
	return nil
}

// DeleteCertificate removes a certificate row from the database.
func (a *CertificateActivity) DeleteCertificate(ctx context.Context, certID string) error {
	_, err := a.coreDB.Exec(ctx,
		`DELETE FROM certificates WHERE id = $1`, certID,
	)
	if err != nil {
		return fmt.Errorf("delete certificate: %w", err)
	}
	return nil
}
