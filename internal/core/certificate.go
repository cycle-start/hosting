package core

import (
	"context"
	"fmt"

	"github.com/edvin/hosting/internal/model"
	temporalclient "go.temporal.io/sdk/client"
)

type CertificateService struct {
	db DB
	tc temporalclient.Client
}

func NewCertificateService(db DB, tc temporalclient.Client) *CertificateService {
	return &CertificateService{db: db, tc: tc}
}

func (s *CertificateService) Upload(ctx context.Context, cert *model.Certificate) error {
	cert.Type = model.CertTypeCustom

	_, err := s.db.Exec(ctx,
		`INSERT INTO certificates (id, fqdn_id, type, cert_pem, key_pem, chain_pem, issued_at, expires_at, status, is_active, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`,
		cert.ID, cert.FQDNID, cert.Type, cert.CertPEM, cert.KeyPEM, cert.ChainPEM,
		cert.IssuedAt, cert.ExpiresAt, cert.Status, cert.IsActive, cert.CreatedAt, cert.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert certificate: %w", err)
	}

	workflowID := fmt.Sprintf("certificate-%s", cert.ID)
	_, err = s.tc.ExecuteWorkflow(ctx, temporalclient.StartWorkflowOptions{
		ID:        workflowID,
		TaskQueue: "hosting-tasks",
	}, "UploadCustomCertWorkflow", cert.ID)
	if err != nil {
		return fmt.Errorf("start UploadCustomCertWorkflow: %w", err)
	}

	return nil
}

func (s *CertificateService) GetByID(ctx context.Context, id string) (*model.Certificate, error) {
	var c model.Certificate
	err := s.db.QueryRow(ctx,
		`SELECT id, fqdn_id, type, cert_pem, key_pem, chain_pem, issued_at, expires_at, status, is_active, created_at, updated_at
		 FROM certificates WHERE id = $1`, id,
	).Scan(&c.ID, &c.FQDNID, &c.Type, &c.CertPEM, &c.KeyPEM, &c.ChainPEM,
		&c.IssuedAt, &c.ExpiresAt, &c.Status, &c.IsActive, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get certificate %s: %w", id, err)
	}
	return &c, nil
}

func (s *CertificateService) ListByFQDN(ctx context.Context, fqdnID string, limit int, cursor string) ([]model.Certificate, bool, error) {
	query := `SELECT id, fqdn_id, type, cert_pem, key_pem, chain_pem, issued_at, expires_at, status, is_active, created_at, updated_at FROM certificates WHERE fqdn_id = $1`
	args := []any{fqdnID}
	argIdx := 2

	if cursor != "" {
		query += fmt.Sprintf(` AND id > $%d`, argIdx)
		args = append(args, cursor)
		argIdx++
	}

	query += ` ORDER BY id`
	query += fmt.Sprintf(` LIMIT $%d`, argIdx)
	args = append(args, limit+1)

	rows, err := s.db.Query(ctx, query, args...)
	if err != nil {
		return nil, false, fmt.Errorf("list certificates for fqdn %s: %w", fqdnID, err)
	}
	defer rows.Close()

	var certs []model.Certificate
	for rows.Next() {
		var c model.Certificate
		if err := rows.Scan(&c.ID, &c.FQDNID, &c.Type, &c.CertPEM, &c.KeyPEM, &c.ChainPEM,
			&c.IssuedAt, &c.ExpiresAt, &c.Status, &c.IsActive, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, false, fmt.Errorf("scan certificate: %w", err)
		}
		certs = append(certs, c)
	}
	if err := rows.Err(); err != nil {
		return nil, false, fmt.Errorf("iterate certificates: %w", err)
	}

	hasMore := len(certs) > limit
	if hasMore {
		certs = certs[:limit]
	}
	return certs, hasMore, nil
}
