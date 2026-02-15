package activity

import (
	"context"
	"fmt"
	"strings"

	"github.com/edvin/hosting/internal/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DNS contains activities for automatic DNS record management.
type DNS struct {
	coreDB    *pgxpool.Pool
	powerdnsDB *PowerDNSDB
}

// NewDNS creates a new DNS activity struct.
func NewDNS(coreDB *pgxpool.Pool, powerdnsDB *PowerDNSDB) *DNS {
	return &DNS{coreDB: coreDB, powerdnsDB: powerdnsDB}
}

// AutoCreateDNSRecordsParams holds parameters for auto-creating DNS records.
type AutoCreateDNSRecordsParams struct {
	FQDN         string                   `json:"fqdn"`
	LBAddresses  []model.ClusterLBAddress `json:"lb_addresses"`
	SourceFQDNID string                   `json:"source_fqdn_id"`
}

// AutoCreateDNSRecords creates A and AAAA records for an FQDN in the matching
// zone, if the zone exists and no user-managed record already exists.
func (a *DNS) AutoCreateDNSRecords(ctx context.Context, params AutoCreateDNSRecordsParams) error {
	// Find the zone for this FQDN by walking up the domain hierarchy.
	zoneName, err := a.findZoneForFQDN(ctx, params.FQDN)
	if err != nil {
		return fmt.Errorf("find zone for fqdn: %w", err)
	}
	if zoneName == "" {
		// No matching zone managed by this platform; skip auto-DNS.
		return nil
	}

	// Get the PowerDNS domain ID for this zone.
	domainID, err := a.powerdnsDB.GetDNSZoneIDByName(ctx, zoneName)
	if err != nil {
		return fmt.Errorf("get dns zone id: %w", err)
	}

	// Check if a user-managed record already exists for this FQDN.
	userManaged, err := a.hasUserManagedRecord(ctx, params.FQDN)
	if err != nil {
		return fmt.Errorf("check user managed record: %w", err)
	}
	if userManaged {
		// User-managed record exists; do not overwrite.
		return nil
	}

	// Create DNS records from LB addresses.
	for _, addr := range params.LBAddresses {
		var recordType string
		if addr.Family == 4 {
			recordType = "A"
		} else if addr.Family == 6 {
			recordType = "AAAA"
		} else {
			continue
		}

		if err := a.powerdnsDB.WriteDNSRecord(ctx, WriteDNSRecordParams{
			DomainID: domainID,
			Name:     params.FQDN,
			Type:     recordType,
			Content:  addr.Address,
			TTL:      300,
		}); err != nil {
			return fmt.Errorf("create %s record for %s: %w", recordType, addr.Address, err)
		}

		// Also record in core DB that these are platform-managed.
		_, err = a.coreDB.Exec(ctx,
			`INSERT INTO zone_records (id, zone_id, type, name, content, ttl, managed_by, source_fqdn_id, status, created_at, updated_at)
			 SELECT gen_random_uuid(), z.id, $1, $2, $3, 300, 'platform', $4, 'active', now(), now()
			 FROM zones z WHERE z.name = $5 AND z.status = 'active'
			 ON CONFLICT DO NOTHING`,
			recordType, params.FQDN, addr.Address, params.SourceFQDNID, zoneName,
		)
		if err != nil {
			return fmt.Errorf("record platform %s in core db: %w", recordType, err)
		}
	}

	return nil
}

// AutoDeleteDNSRecords removes platform-managed DNS records for an FQDN
// from both the PowerDNS database and core DB.
func (a *DNS) AutoDeleteDNSRecords(ctx context.Context, fqdn string) error {
	zoneName, err := a.findZoneForFQDN(ctx, fqdn)
	if err != nil {
		return fmt.Errorf("find zone for fqdn: %w", err)
	}
	if zoneName == "" {
		return nil
	}

	domainID, err := a.powerdnsDB.GetDNSZoneIDByName(ctx, zoneName)
	if err != nil {
		return fmt.Errorf("get dns zone id: %w", err)
	}

	// Delete A record from PowerDNS.
	if err := a.powerdnsDB.DeleteDNSRecord(ctx, DeleteDNSRecordParams{
		DomainID: domainID,
		Name:     fqdn,
		Type:     "A",
	}); err != nil {
		return fmt.Errorf("delete A record: %w", err)
	}

	// Delete AAAA record from PowerDNS.
	if err := a.powerdnsDB.DeleteDNSRecord(ctx, DeleteDNSRecordParams{
		DomainID: domainID,
		Name:     fqdn,
		Type:     "AAAA",
	}); err != nil {
		return fmt.Errorf("delete AAAA record: %w", err)
	}

	// Remove platform-managed records from core DB.
	_, err = a.coreDB.Exec(ctx,
		`DELETE FROM zone_records WHERE name = $1 AND managed_by = 'platform'`, fqdn,
	)
	if err != nil {
		return fmt.Errorf("delete platform records from core db: %w", err)
	}

	return nil
}

// ServiceHostnameParams holds parameters for creating service hostname DNS records.
type ServiceHostnameParams struct {
	BaseHostname string
	TenantName   string
	Services     []ServiceHostnameEntry
}

// ServiceHostnameEntry defines a single service hostname entry.
type ServiceHostnameEntry struct {
	Service string
	IP      string
	IP6     string
}

// CreateServiceHostnameRecords creates DNS records for service hostnames
// such as ssh.<tenant>.<base> and mysql.<tenant>.<base>.
func (a *DNS) CreateServiceHostnameRecords(ctx context.Context, params ServiceHostnameParams) error {
	zoneName, err := a.findZoneForFQDN(ctx, params.BaseHostname)
	if err != nil {
		return fmt.Errorf("find zone for base hostname: %w", err)
	}
	if zoneName == "" {
		return nil
	}

	domainID, err := a.powerdnsDB.GetDNSZoneIDByName(ctx, zoneName)
	if err != nil {
		return fmt.Errorf("get dns zone id: %w", err)
	}

	for _, svc := range params.Services {
		hostname := fmt.Sprintf("%s.%s.%s", svc.Service, params.TenantName, params.BaseHostname)

		if svc.IP != "" {
			if err := a.powerdnsDB.WriteDNSRecord(ctx, WriteDNSRecordParams{
				DomainID: domainID,
				Name:     hostname,
				Type:     "A",
				Content:  svc.IP,
				TTL:      300,
			}); err != nil {
				return fmt.Errorf("create service A record for %s: %w", hostname, err)
			}
		}

		if svc.IP6 != "" {
			if err := a.powerdnsDB.WriteDNSRecord(ctx, WriteDNSRecordParams{
				DomainID: domainID,
				Name:     hostname,
				Type:     "AAAA",
				Content:  svc.IP6,
				TTL:      300,
			}); err != nil {
				return fmt.Errorf("create service AAAA record for %s: %w", hostname, err)
			}
		}
	}

	return nil
}

// findZoneForFQDN walks up the domain hierarchy to find a zone managed by
// this platform. For example, for "www.example.com" it checks "www.example.com",
// then "example.com", then "com".
func (a *DNS) findZoneForFQDN(ctx context.Context, fqdn string) (string, error) {
	parts := strings.Split(fqdn, ".")
	for i := range parts {
		candidate := strings.Join(parts[i:], ".")
		var name string
		err := a.coreDB.QueryRow(ctx,
			`SELECT name FROM zones WHERE name = $1 AND status = 'active'`, candidate,
		).Scan(&name)
		if err == pgx.ErrNoRows {
			continue
		}
		if err != nil {
			return "", err
		}
		return name, nil
	}
	return "", nil
}

// hasUserManagedRecord checks if a user-managed DNS record exists for the given FQDN
// in the core DB.
func (a *DNS) hasUserManagedRecord(ctx context.Context, fqdn string) (bool, error) {
	var count int
	err := a.coreDB.QueryRow(ctx,
		`SELECT COUNT(*) FROM zone_records
		 WHERE name = $1 AND managed_by = 'user' AND status = 'active'
		 AND (type = 'A' OR type = 'AAAA')`, fqdn,
	).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// AutoCreateEmailDNSRecordsParams holds parameters for auto-creating email DNS records.
type AutoCreateEmailDNSRecordsParams struct {
	FQDN             string
	MailHostname     string
	SourceFQDNID     string
}

// AutoCreateEmailDNSRecords creates MX and SPF records for an FQDN in the matching
// zone, if the zone exists. Used when creating an email account on an FQDN.
func (a *DNS) AutoCreateEmailDNSRecords(ctx context.Context, params AutoCreateEmailDNSRecordsParams) error {
	zoneName, err := a.findZoneForFQDN(ctx, params.FQDN)
	if err != nil {
		return fmt.Errorf("find zone for fqdn: %w", err)
	}
	if zoneName == "" {
		return nil
	}

	domainID, err := a.powerdnsDB.GetDNSZoneIDByName(ctx, zoneName)
	if err != nil {
		return fmt.Errorf("get dns zone id: %w", err)
	}

	// Create MX record pointing to the cluster's mail hostname.
	mxPriority := 10
	if err := a.powerdnsDB.WriteDNSRecord(ctx, WriteDNSRecordParams{
		DomainID: domainID,
		Name:     params.FQDN,
		Type:     "MX",
		Content:  params.MailHostname,
		TTL:      300,
		Priority: &mxPriority,
	}); err != nil {
		return fmt.Errorf("create MX record: %w", err)
	}

	// Create SPF TXT record.
	if err := a.powerdnsDB.WriteDNSRecord(ctx, WriteDNSRecordParams{
		DomainID: domainID,
		Name:     params.FQDN,
		Type:     "TXT",
		Content:  "v=spf1 mx ~all",
		TTL:      300,
	}); err != nil {
		return fmt.Errorf("create SPF record: %w", err)
	}

	// Record MX in core DB as platform-managed.
	_, err = a.coreDB.Exec(ctx,
		`INSERT INTO zone_records (id, zone_id, type, name, content, ttl, priority, managed_by, source_fqdn_id, status, created_at, updated_at)
		 SELECT gen_random_uuid(), z.id, 'MX', $1, $2, 300, 10, 'platform', $3, 'active', now(), now()
		 FROM zones z WHERE z.name = $4 AND z.status = 'active'
		 ON CONFLICT DO NOTHING`,
		params.FQDN, params.MailHostname, params.SourceFQDNID, zoneName,
	)
	if err != nil {
		return fmt.Errorf("record platform MX in core db: %w", err)
	}

	// Record SPF in core DB as platform-managed.
	_, err = a.coreDB.Exec(ctx,
		`INSERT INTO zone_records (id, zone_id, type, name, content, ttl, managed_by, source_fqdn_id, status, created_at, updated_at)
		 SELECT gen_random_uuid(), z.id, 'TXT', $1, $2, 300, 'platform', $3, 'active', now(), now()
		 FROM zones z WHERE z.name = $4 AND z.status = 'active'
		 ON CONFLICT DO NOTHING`,
		params.FQDN, "v=spf1 mx ~all", params.SourceFQDNID, zoneName,
	)
	if err != nil {
		return fmt.Errorf("record platform SPF in core db: %w", err)
	}

	return nil
}

// AutoDeleteEmailDNSRecords removes platform-managed email DNS records (MX, TXT)
// for an FQDN from both the PowerDNS database and core DB.
func (a *DNS) AutoDeleteEmailDNSRecords(ctx context.Context, fqdn string) error {
	zoneName, err := a.findZoneForFQDN(ctx, fqdn)
	if err != nil {
		return fmt.Errorf("find zone for fqdn: %w", err)
	}
	if zoneName == "" {
		return nil
	}

	domainID, err := a.powerdnsDB.GetDNSZoneIDByName(ctx, zoneName)
	if err != nil {
		return fmt.Errorf("get dns zone id: %w", err)
	}

	// Delete MX record from PowerDNS.
	if err := a.powerdnsDB.DeleteDNSRecord(ctx, DeleteDNSRecordParams{
		DomainID: domainID,
		Name:     fqdn,
		Type:     "MX",
	}); err != nil {
		return fmt.Errorf("delete MX record: %w", err)
	}

	// Delete SPF TXT record from PowerDNS.
	if err := a.powerdnsDB.DeleteDNSRecord(ctx, DeleteDNSRecordParams{
		DomainID: domainID,
		Name:     fqdn,
		Type:     "TXT",
	}); err != nil {
		return fmt.Errorf("delete TXT record: %w", err)
	}

	// Remove platform-managed email records from core DB.
	_, err = a.coreDB.Exec(ctx,
		`DELETE FROM zone_records WHERE name = $1 AND managed_by = 'platform'
		 AND type IN ('MX', 'TXT')`, fqdn,
	)
	if err != nil {
		return fmt.Errorf("delete platform email records from core db: %w", err)
	}

	return nil
}
