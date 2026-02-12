package config

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
)

// TemporalTLS builds a *tls.Config from the Temporal TLS fields.
// Returns nil, nil if no cert/key is configured (plaintext mode).
func (c *Config) TemporalTLS() (*tls.Config, error) {
	if c.TemporalTLSCert == "" && c.TemporalTLSKey == "" {
		return nil, nil
	}

	cert, err := tls.LoadX509KeyPair(c.TemporalTLSCert, c.TemporalTLSKey)
	if err != nil {
		return nil, fmt.Errorf("load temporal client cert: %w", err)
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
	}

	if c.TemporalTLSCACert != "" {
		caPEM, err := os.ReadFile(c.TemporalTLSCACert)
		if err != nil {
			return nil, fmt.Errorf("read temporal CA cert: %w", err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(caPEM) {
			return nil, fmt.Errorf("failed to parse temporal CA cert")
		}
		tlsConfig.RootCAs = pool
	}

	if c.TemporalTLSServerName != "" {
		tlsConfig.ServerName = c.TemporalTLSServerName
	}

	return tlsConfig, nil
}
