package handler

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/edvin/hosting/internal/api/request"
	"github.com/edvin/hosting/internal/core"
	"github.com/edvin/hosting/internal/model"
	"github.com/edvin/hosting/internal/platform"
)

// parseSSHKey parses an SSH public key and returns its SHA256 fingerprint.
func parseSSHKey(publicKey string) (string, error) {
	pubKey, _, _, _, err := ssh.ParseAuthorizedKey([]byte(publicKey))
	if err != nil {
		return "", err
	}
	return ssh.FingerprintSHA256(pubKey), nil
}

// generatePassword creates a random 32-character hex password.
func generatePassword() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// createNestedFQDNs creates FQDNs and their nested email resources for a webroot.
func createNestedFQDNs(ctx context.Context, services *core.Services, webrootID string, fqdns []request.CreateFQDNNested) error {
	for _, fr := range fqdns {
		now := time.Now()
		fqdn := &model.FQDN{
			ID:        platform.NewID(),
			FQDN:      fr.FQDN,
			WebrootID: webrootID,
			Status:    model.StatusPending,
			CreatedAt: now,
			UpdatedAt: now,
		}
		if fr.SSLEnabled != nil {
			fqdn.SSLEnabled = *fr.SSLEnabled
		}
		if err := services.FQDN.Create(ctx, fqdn); err != nil {
			return fmt.Errorf("create fqdn %s: %s", fr.FQDN, err.Error())
		}
		if err := createNestedEmailAccounts(ctx, services, fqdn.ID, fr.EmailAccounts); err != nil {
			return err
		}
	}
	return nil
}

// createNestedEmailAccounts creates email accounts and their nested aliases, forwards, and auto-replies.
func createNestedEmailAccounts(ctx context.Context, services *core.Services, fqdnID string, accounts []request.CreateEmailAccountNested) error {
	for _, ar := range accounts {
		now := time.Now()
		account := &model.EmailAccount{
			ID:          platform.NewID(),
			FQDNID:      fqdnID,
			Address:     ar.Address,
			DisplayName: ar.DisplayName,
			QuotaBytes:  ar.QuotaBytes,
			Status:      model.StatusPending,
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		if err := services.EmailAccount.Create(ctx, account); err != nil {
			return fmt.Errorf("create email account %s: %s", ar.Address, err.Error())
		}

		// Aliases
		for _, al := range ar.Aliases {
			now2 := time.Now()
			alias := &model.EmailAlias{
				ID:             platform.NewID(),
				EmailAccountID: account.ID,
				Address:        al.Address,
				Status:         model.StatusPending,
				CreatedAt:      now2,
				UpdatedAt:      now2,
			}
			if err := services.EmailAlias.Create(ctx, alias); err != nil {
				return fmt.Errorf("create email alias %s: %s", al.Address, err.Error())
			}
		}

		// Forwards
		for _, fw := range ar.Forwards {
			keepCopy := true
			if fw.KeepCopy != nil {
				keepCopy = *fw.KeepCopy
			}
			now2 := time.Now()
			fwd := &model.EmailForward{
				ID:             platform.NewID(),
				EmailAccountID: account.ID,
				Destination:    fw.Destination,
				KeepCopy:       keepCopy,
				Status:         model.StatusPending,
				CreatedAt:      now2,
				UpdatedAt:      now2,
			}
			if err := services.EmailForward.Create(ctx, fwd); err != nil {
				return fmt.Errorf("create email forward %s: %s", fw.Destination, err.Error())
			}
		}

		// Auto-reply
		if ar.AutoReply != nil {
			now2 := time.Now()
			autoReply := &model.EmailAutoReply{
				ID:             platform.NewID(),
				EmailAccountID: account.ID,
				Subject:        ar.AutoReply.Subject,
				Body:           ar.AutoReply.Body,
				StartDate:      ar.AutoReply.StartDate,
				EndDate:        ar.AutoReply.EndDate,
				Enabled:        ar.AutoReply.Enabled,
				Status:         model.StatusPending,
				CreatedAt:      now2,
				UpdatedAt:      now2,
			}
			if err := services.EmailAutoReply.Upsert(ctx, autoReply); err != nil {
				return fmt.Errorf("create email autoreply for %s: %s", ar.Address, err.Error())
			}
		}
	}
	return nil
}
