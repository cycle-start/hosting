package sshca

import (
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
	"time"

	"golang.org/x/crypto/ssh"
)

// CA holds a parsed SSH CA private key used to sign ephemeral user certificates.
type CA struct {
	signer ssh.Signer
}

// New parses a PEM-encoded private key and returns a CA that can sign certificates.
func New(pemBytes []byte) (*CA, error) {
	signer, err := ssh.ParsePrivateKey(pemBytes)
	if err != nil {
		return nil, fmt.Errorf("sshca: parse private key: %w", err)
	}
	return &CA{signer: signer}, nil
}

// Sign generates an ephemeral Ed25519 key pair, creates an SSH user certificate
// for the given principal (username), signs it with the CA key, and returns a
// signer that presents both the ephemeral key and the certificate.
func (ca *CA) Sign(username string, ttl time.Duration) (ssh.Signer, error) {
	// Generate ephemeral key pair.
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("sshca: generate ephemeral key: %w", err)
	}

	sshPub, err := ssh.NewPublicKey(pub)
	if err != nil {
		return nil, fmt.Errorf("sshca: convert public key: %w", err)
	}

	now := time.Now()
	cert := &ssh.Certificate{
		CertType:        ssh.UserCert,
		Key:             sshPub,
		ValidPrincipals: []string{username},
		ValidAfter:      uint64(now.Add(-30 * time.Second).Unix()),
		ValidBefore:     uint64(now.Add(ttl).Unix()),
	}

	if err := cert.SignCert(rand.Reader, ca.signer); err != nil {
		return nil, fmt.Errorf("sshca: sign certificate: %w", err)
	}

	ephemeralSigner, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		return nil, fmt.Errorf("sshca: create ephemeral signer: %w", err)
	}

	certSigner, err := ssh.NewCertSigner(cert, ephemeralSigner)
	if err != nil {
		return nil, fmt.Errorf("sshca: create cert signer: %w", err)
	}

	return certSigner, nil
}
