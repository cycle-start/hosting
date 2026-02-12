package activity

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// PowerDNSDB contains activities that write to the PowerDNS database.
type PowerDNSDB struct {
	db *pgxpool.Pool
}

// NewPowerDNSDB creates a new PowerDNSDB activity struct.
func NewPowerDNSDB(db *pgxpool.Pool) *PowerDNSDB {
	return &PowerDNSDB{db: db}
}

// WriteDNSZoneParams holds parameters for creating a DNS zone.
type WriteDNSZoneParams struct {
	Name string
	Type string // "NATIVE"
}

// WriteDNSZone inserts a new zone into the PowerDNS domains table and returns its ID.
func (a *PowerDNSDB) WriteDNSZone(ctx context.Context, params WriteDNSZoneParams) (int, error) {
	var id int
	err := a.db.QueryRow(ctx,
		`INSERT INTO domains (name, type) VALUES ($1, $2) RETURNING id`,
		params.Name, params.Type,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("write dns zone: %w", err)
	}
	return id, nil
}

// DeleteDNSZone removes a zone from the PowerDNS domains table by name.
func (a *PowerDNSDB) DeleteDNSZone(ctx context.Context, domainName string) error {
	_, err := a.db.Exec(ctx, `DELETE FROM domains WHERE name = $1`, domainName)
	if err != nil {
		return fmt.Errorf("delete dns zone: %w", err)
	}
	return nil
}

// WriteDNSRecordParams holds parameters for creating a DNS record.
type WriteDNSRecordParams struct {
	DomainID int
	Name     string
	Type     string
	Content  string
	TTL      int
	Priority *int
}

// WriteDNSRecord inserts a new record into the PowerDNS records table.
func (a *PowerDNSDB) WriteDNSRecord(ctx context.Context, params WriteDNSRecordParams) error {
	_, err := a.db.Exec(ctx,
		`INSERT INTO records (domain_id, name, type, content, ttl, prio) VALUES ($1, $2, $3, $4, $5, $6)`,
		params.DomainID, params.Name, params.Type, params.Content, params.TTL, params.Priority,
	)
	if err != nil {
		return fmt.Errorf("write dns record: %w", err)
	}
	return nil
}

// UpdateDNSRecordParams holds parameters for updating a DNS record.
type UpdateDNSRecordParams struct {
	DomainID int
	Name     string
	Type     string
	Content  string
	TTL      int
	Priority *int
}

// UpdateDNSRecord updates an existing record in the PowerDNS records table.
func (a *PowerDNSDB) UpdateDNSRecord(ctx context.Context, params UpdateDNSRecordParams) error {
	_, err := a.db.Exec(ctx,
		`UPDATE records SET content = $1, ttl = $2, prio = $3 WHERE domain_id = $4 AND name = $5 AND type = $6`,
		params.Content, params.TTL, params.Priority, params.DomainID, params.Name, params.Type,
	)
	if err != nil {
		return fmt.Errorf("update dns record: %w", err)
	}
	return nil
}

// DeleteDNSRecordParams holds parameters for deleting a DNS record.
type DeleteDNSRecordParams struct {
	DomainID int
	Name     string
	Type     string
}

// DeleteDNSRecord removes a record from the PowerDNS records table by name, type, and domain ID.
func (a *PowerDNSDB) DeleteDNSRecord(ctx context.Context, params DeleteDNSRecordParams) error {
	_, err := a.db.Exec(ctx,
		`DELETE FROM records WHERE domain_id = $1 AND name = $2 AND type = $3`,
		params.DomainID, params.Name, params.Type,
	)
	if err != nil {
		return fmt.Errorf("delete dns record: %w", err)
	}
	return nil
}

// DeleteDNSRecordsByDomain removes all records for a given domain ID.
func (a *PowerDNSDB) DeleteDNSRecordsByDomain(ctx context.Context, domainID int) error {
	_, err := a.db.Exec(ctx, `DELETE FROM records WHERE domain_id = $1`, domainID)
	if err != nil {
		return fmt.Errorf("delete dns records by domain: %w", err)
	}
	return nil
}

// GetDNSZoneIDByName looks up a PowerDNS domain ID by its name.
func (a *PowerDNSDB) GetDNSZoneIDByName(ctx context.Context, name string) (int, error) {
	var id int
	err := a.db.QueryRow(ctx, `SELECT id FROM domains WHERE name = $1`, name).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("get dns zone id by name: %w", err)
	}
	return id, nil
}
