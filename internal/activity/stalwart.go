package activity

import (
	"context"
	"fmt"

	"github.com/edvin/hosting/internal/model"
	"github.com/edvin/hosting/internal/stalwart"
)

type Stalwart struct {
	client     *stalwart.Client
	jmapClient *stalwart.JMAPClient
	db         DB
}

func NewStalwart(db DB) *Stalwart {
	return &Stalwart{
		client:     stalwart.NewClient(),
		jmapClient: stalwart.NewJMAPClient(),
		db:         db,
	}
}

type StalwartDomainParams struct {
	BaseURL    string `json:"base_url"`
	AdminToken string `json:"admin_token"`
	Domain     string `json:"domain"`
}

func (a *Stalwart) StalwartCreateDomain(ctx context.Context, params StalwartDomainParams) error {
	return a.client.CreateDomain(ctx, params.BaseURL, params.AdminToken, params.Domain)
}

func (a *Stalwart) StalwartDeleteDomain(ctx context.Context, params StalwartDomainParams) error {
	return a.client.DeleteDomain(ctx, params.BaseURL, params.AdminToken, params.Domain)
}

type StalwartCreateAccountParams struct {
	BaseURL     string `json:"base_url"`
	AdminToken  string `json:"admin_token"`
	Address     string `json:"address"`
	DisplayName string `json:"display_name"`
	QuotaBytes  int64  `json:"quota_bytes"`
	Password    string `json:"password"`
}

func (a *Stalwart) StalwartCreateAccount(ctx context.Context, params StalwartCreateAccountParams) error {
	return a.client.CreateAccount(ctx, params.BaseURL, params.AdminToken, stalwart.CreateAccountParams{
		Address:     params.Address,
		DisplayName: params.DisplayName,
		QuotaBytes:  params.QuotaBytes,
		Password:    params.Password,
	})
}

type StalwartDeleteAccountParams struct {
	BaseURL    string `json:"base_url"`
	AdminToken string `json:"admin_token"`
	Address    string `json:"address"`
}

func (a *Stalwart) StalwartDeleteAccount(ctx context.Context, params StalwartDeleteAccountParams) error {
	return a.client.DeleteAccount(ctx, params.BaseURL, params.AdminToken, params.Address)
}

// StalwartAliasParams holds parameters for alias add/remove operations.
type StalwartAliasParams struct {
	BaseURL     string `json:"base_url"`
	AdminToken  string `json:"admin_token"`
	AccountName string `json:"account_name"`
	Address     string `json:"address"`
}

// StalwartAddAlias adds an email address to a principal's emails array.
func (a *Stalwart) StalwartAddAlias(ctx context.Context, params StalwartAliasParams) error {
	return a.client.UpdateAccount(ctx, params.BaseURL, params.AdminToken, params.AccountName, []stalwart.PatchOp{
		{Action: "addItem", Field: "emails", Value: params.Address},
	})
}

// StalwartRemoveAlias removes an email address from a principal's emails array.
func (a *Stalwart) StalwartRemoveAlias(ctx context.Context, params StalwartAliasParams) error {
	return a.client.UpdateAccount(ctx, params.BaseURL, params.AdminToken, params.AccountName, []stalwart.PatchOp{
		{Action: "removeItem", Field: "emails", Value: params.Address},
	})
}

// StalwartSyncForwardParams holds parameters for syncing the forwarding Sieve script.
type StalwartSyncForwardParams struct {
	BaseURL        string `json:"base_url"`
	AdminToken     string `json:"admin_token"`
	AccountName    string `json:"account_name"`
	EmailAccountID string `json:"email_account_id"`
}

// StalwartSyncForwardScript regenerates and deploys the forwarding Sieve script
// based on the current active forwards in the database.
func (a *Stalwart) StalwartSyncForwardScript(ctx context.Context, params StalwartSyncForwardParams) error {
	// Query active forwards from the core DB.
	rows, err := a.db.Query(ctx,
		`SELECT destination, keep_copy FROM email_forwards
		 WHERE email_account_id = $1 AND status != $2
		 ORDER BY destination`,
		params.EmailAccountID, model.StatusDeleting,
	)
	if err != nil {
		return fmt.Errorf("query forwards: %w", err)
	}
	defer rows.Close()

	var rules []stalwart.ForwardRule
	for rows.Next() {
		var r stalwart.ForwardRule
		if err := rows.Scan(&r.Destination, &r.KeepCopy); err != nil {
			return fmt.Errorf("scan forward: %w", err)
		}
		rules = append(rules, r)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate forwards: %w", err)
	}

	script := stalwart.GenerateForwardScript(rules)
	scriptName := "hosting-forwards"

	if script == "" {
		// No forwards — delete the script if it exists.
		return a.jmapClient.DeleteSieveScript(ctx, params.BaseURL, params.AdminToken, params.AccountName, scriptName)
	}

	return a.jmapClient.DeploySieveScript(ctx, params.BaseURL, params.AdminToken, params.AccountName, scriptName, script)
}

// StalwartVacationParams holds parameters for setting vacation auto-reply.
type StalwartVacationParams struct {
	BaseURL     string                  `json:"base_url"`
	AdminToken  string                  `json:"admin_token"`
	AccountName string                  `json:"account_name"`
	Vacation    *stalwart.VacationParams `json:"vacation"`
}

// StalwartSetVacation sets or clears the vacation auto-reply via JMAP.
func (a *Stalwart) StalwartSetVacation(ctx context.Context, params StalwartVacationParams) error {
	return a.jmapClient.SetVacationResponse(ctx, params.BaseURL, params.AdminToken, params.AccountName, params.Vacation)
}
