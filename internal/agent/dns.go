package agent

import "github.com/rs/zerolog"

// DNSManager handles local DNS operations on the node.
// In v1, DNS is primarily managed through service DB writes in Temporal activities.
// This manager is reserved for any node-local DNS operations needed in the future.
type DNSManager struct {
	logger zerolog.Logger
}

// NewDNSManager creates a new DNSManager.
func NewDNSManager(logger zerolog.Logger) *DNSManager {
	return &DNSManager{
		logger: logger.With().Str("component", "dns-manager").Logger(),
	}
}
