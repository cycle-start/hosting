package activity

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

// NodeACMEActivity handles ACME challenge file placement on nodes.
type NodeACMEActivity struct{}

// NewNodeACMEActivity creates a new NodeACMEActivity.
func NewNodeACMEActivity() *NodeACMEActivity {
	return &NodeACMEActivity{}
}

// PlaceHTTP01Challenge writes the ACME challenge response file.
func (a *NodeACMEActivity) PlaceHTTP01Challenge(ctx context.Context, params PlaceHTTP01ChallengeParams) error {
	challengeDir := filepath.Join(params.WebrootPath, ".well-known", "acme-challenge")
	if err := os.MkdirAll(challengeDir, 0755); err != nil {
		return fmt.Errorf("create challenge dir: %w", err)
	}

	challengeFile := filepath.Join(challengeDir, params.Token)
	if err := os.WriteFile(challengeFile, []byte(params.KeyAuth), 0644); err != nil {
		return fmt.Errorf("write challenge file: %w", err)
	}

	return nil
}

// CleanupHTTP01Challenge removes the ACME challenge response file.
func (a *NodeACMEActivity) CleanupHTTP01Challenge(ctx context.Context, params CleanupHTTP01ChallengeParams) error {
	challengeFile := filepath.Join(params.WebrootPath, ".well-known", "acme-challenge", params.Token)
	os.Remove(challengeFile)
	// Best effort: also remove empty dirs.
	os.Remove(filepath.Join(params.WebrootPath, ".well-known", "acme-challenge"))
	os.Remove(filepath.Join(params.WebrootPath, ".well-known"))
	return nil
}
