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
	coreDB     *pgxpool.Pool
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
// zone, if the zone exists and no custom-managed record already exists.
func (a *DNS) AutoCreateDNSRecords(ctx context.Context, params AutoCreateDNSRecordsParams) error {
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

	hasCustom, err := a.hasCustomRecord(ctx, params.FQDN, "A", "AAAA")
	if err != nil {
		return fmt.Errorf("check custom managed record: %w", err)
	}

	for _, addr := range params.LBAddresses {
		var recordType string
		if addr.Family == 4 {
			recordType = "A"
		} else if addr.Family == 6 {
			recordType = "AAAA"
		} else {
			continue
		}

		// Write to PowerDNS only if no custom override exists.
		if !hasCustom {
			if err := a.powerdnsDB.WriteDNSRecord(ctx, WriteDNSRecordParams{
				DomainID: domainID,
				Name:     params.FQDN,
				Type:     recordType,
				Content:  addr.Address,
				TTL:      300,
			}); err != nil {
				return fmt.Errorf("create %s record for %s: %w", recordType, addr.Address, err)
			}
		}

		// Always record in core DB (auto records exist regardless of override state).
		_, err = a.coreDB.Exec(ctx,
			`INSERT INTO zone_records (id, zone_id, type, name, content, ttl, managed_by, source_type, source_fqdn_id, status, created_at, updated_at)
			 SELECT gen_random_uuid(), z.id, $1, $2, $3, 300, 'auto', 'fqdn', $4, 'active', now(), now()
			 FROM zones z WHERE z.name = $5 AND z.status = 'active'
			 AND NOT EXISTS (SELECT 1 FROM zone_records zr WHERE zr.zone_id = z.id AND zr.type = $1 AND zr.name = $2 AND zr.content = $3 AND zr.managed_by = 'auto')`,
			recordType, params.FQDN, addr.Address, params.SourceFQDNID, zoneName,
		)
		if err != nil {
			return fmt.Errorf("record auto %s in core db: %w", recordType, err)
		}
	}

	return nil
}

// AutoDeleteDNSRecords removes auto-managed DNS records for an FQDN
// from both the PowerDNS database and core DB.
func (a *DNS) AutoDeleteDNSRecords(ctx context.Context, fqdn string) error {
	zoneName, err := a.findZoneForFQDN(ctx, fqdn)
	if err != nil {
		return fmt.Errorf("find zone for fqdn: %w", err)
	}
	if zoneName == "" {
		// Even without a zone, clean up any orphaned core DB records.
		_, _ = a.coreDB.Exec(ctx,
			`DELETE FROM zone_records WHERE name = $1 AND managed_by = 'auto' AND source_type = 'fqdn'`, fqdn)
		return nil
	}

	domainID, err := a.powerdnsDB.GetDNSZoneIDByName(ctx, zoneName)
	if err != nil {
		return fmt.Errorf("get dns zone id: %w", err)
	}

	if domainID > 0 {
		_ = a.powerdnsDB.DeleteDNSRecord(ctx, DeleteDNSRecordParams{DomainID: domainID, Name: fqdn, Type: "A"})
		_ = a.powerdnsDB.DeleteDNSRecord(ctx, DeleteDNSRecordParams{DomainID: domainID, Name: fqdn, Type: "AAAA"})
	}

	_, err = a.coreDB.Exec(ctx,
		`DELETE FROM zone_records WHERE name = $1 AND managed_by = 'auto' AND source_type = 'fqdn'`, fqdn)
	if err != nil {
		return fmt.Errorf("delete auto FQDN records from core db: %w", err)
	}

	return nil
}

// AutoCreateEmailDNSRecordsParams holds parameters for auto-creating email DNS records.
type AutoCreateEmailDNSRecordsParams struct {
	FQDN         string `json:"fqdn"`
	MailHostname string `json:"mail_hostname"`
	SPFIncludes  string `json:"spf_includes"`
	DKIMSelector string `json:"dkim_selector"`
	DKIMPublicKey string `json:"dkim_public_key"`
	DMARCPolicy  string `json:"dmarc_policy"`
	SourceFQDNID string `json:"source_fqdn_id"`
}

// AutoCreateEmailDNSRecords creates MX, SPF, DKIM, and DMARC records for an FQDN
// in the matching zone, if the zone exists. Used when creating an email account.
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

	// MX record.
	if params.MailHostname != "" {
		if err := a.createAutoRecord(ctx, autoRecordDef{
			zoneName:     zoneName,
			domainID:     domainID,
			fqdn:         params.FQDN,
			recordType:   "MX",
			content:      params.MailHostname,
			ttl:          300,
			priority:     intPtr(10),
			sourceType:   model.SourceTypeEmailMX,
			sourceFQDNID: params.SourceFQDNID,
		}); err != nil {
			return fmt.Errorf("create MX record: %w", err)
		}
	}

	// SPF record.
	spfContent := buildSPFRecord(params.SPFIncludes)
	if err := a.createAutoRecord(ctx, autoRecordDef{
		zoneName:     zoneName,
		domainID:     domainID,
		fqdn:         params.FQDN,
		recordType:   "TXT",
		content:      spfContent,
		ttl:          300,
		sourceType:   model.SourceTypeEmailSPF,
		sourceFQDNID: params.SourceFQDNID,
	}); err != nil {
		return fmt.Errorf("create SPF record: %w", err)
	}

	// DKIM record (only if brand has DKIM configured).
	if params.DKIMSelector != "" && params.DKIMPublicKey != "" {
		dkimName := fmt.Sprintf("%s._domainkey.%s", params.DKIMSelector, params.FQDN)
		dkimContent := fmt.Sprintf("v=DKIM1; k=rsa; p=%s", params.DKIMPublicKey)
		if err := a.createAutoRecord(ctx, autoRecordDef{
			zoneName:     zoneName,
			domainID:     domainID,
			fqdn:         dkimName,
			recordType:   "TXT",
			content:      dkimContent,
			ttl:          300,
			sourceType:   model.SourceTypeEmailDKIM,
			sourceFQDNID: params.SourceFQDNID,
		}); err != nil {
			return fmt.Errorf("create DKIM record: %w", err)
		}
	}

	// DMARC record (only if brand has DMARC configured).
	if params.DMARCPolicy != "" {
		dmarcName := "_dmarc." + params.FQDN
		if err := a.createAutoRecord(ctx, autoRecordDef{
			zoneName:     zoneName,
			domainID:     domainID,
			fqdn:         dmarcName,
			recordType:   "TXT",
			content:      params.DMARCPolicy,
			ttl:          300,
			sourceType:   model.SourceTypeEmailDMARC,
			sourceFQDNID: params.SourceFQDNID,
		}); err != nil {
			return fmt.Errorf("create DMARC record: %w", err)
		}
	}

	return nil
}

// AutoDeleteEmailDNSRecords removes auto-managed email DNS records (MX, SPF, DKIM, DMARC)
// for an FQDN from both the PowerDNS database and core DB.
func (a *DNS) AutoDeleteEmailDNSRecords(ctx context.Context, fqdn string) error {
	zoneName, err := a.findZoneForFQDN(ctx, fqdn)
	if err != nil {
		return fmt.Errorf("find zone for fqdn: %w", err)
	}

	if zoneName != "" {
		domainID, err := a.powerdnsDB.GetDNSZoneIDByName(ctx, zoneName)
		if err != nil {
			return fmt.Errorf("get dns zone id: %w", err)
		}

		if domainID > 0 {
			// Delete MX.
			_ = a.powerdnsDB.DeleteDNSRecord(ctx, DeleteDNSRecordParams{DomainID: domainID, Name: fqdn, Type: "MX"})

			// Delete all auto email TXT records from PowerDNS for the FQDN and subnames.
			// We need to look up the core DB records to find the exact names.
			rows, err := a.coreDB.Query(ctx,
				`SELECT DISTINCT name, type FROM zone_records
				 WHERE source_fqdn_id = (SELECT id FROM fqdns WHERE fqdn = $1 LIMIT 1)
				 AND managed_by = 'auto' AND source_type IN ('email-spf', 'email-dkim', 'email-dmarc')`, fqdn)
			if err == nil {
				defer rows.Close()
				for rows.Next() {
					var recName, recType string
					if rows.Scan(&recName, &recType) == nil {
						_ = a.powerdnsDB.DeleteDNSRecord(ctx, DeleteDNSRecordParams{
							DomainID: domainID, Name: recName, Type: recType,
						})
					}
				}
			}
		}
	}

	// Remove auto-managed email records from core DB by source type.
	_, err = a.coreDB.Exec(ctx,
		`DELETE FROM zone_records WHERE managed_by = 'auto'
		 AND source_type IN ('email-mx', 'email-spf', 'email-dkim', 'email-dmarc')
		 AND (name = $1 OR name LIKE '%._domainkey.' || $1 OR name = '_dmarc.' || $1)`, fqdn)
	if err != nil {
		return fmt.Errorf("delete auto email records from core db: %w", err)
	}

	return nil
}

// DeactivateAutoRecordsParams holds parameters for deactivating auto records
// when a custom record with the same name and type is created.
type DeactivateAutoRecordsParams struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// DeactivateAutoRecords removes auto-managed records from PowerDNS (but keeps
// them in core DB) when a custom record with matching name+type is created.
// This implements the override system: custom records take priority over auto records.
func (a *DNS) DeactivateAutoRecords(ctx context.Context, params DeactivateAutoRecordsParams) error {
	// Find matching auto records in core DB.
	rows, err := a.coreDB.Query(ctx,
		`SELECT zr.name, zr.type, z.name AS zone_name
		 FROM zone_records zr
		 JOIN zones z ON z.id = zr.zone_id
		 WHERE zr.name = $1 AND zr.type = $2 AND zr.managed_by = 'auto' AND zr.status = 'active'`,
		params.Name, params.Type)
	if err != nil {
		return fmt.Errorf("find auto records to deactivate: %w", err)
	}
	defer rows.Close()

	type autoRec struct {
		name, recType, zoneName string
	}
	var recs []autoRec
	for rows.Next() {
		var r autoRec
		if err := rows.Scan(&r.name, &r.recType, &r.zoneName); err != nil {
			return fmt.Errorf("scan auto record: %w", err)
		}
		recs = append(recs, r)
	}

	// Remove each from PowerDNS.
	for _, r := range recs {
		domainID, err := a.powerdnsDB.GetDNSZoneIDByName(ctx, r.zoneName)
		if err != nil || domainID == 0 {
			continue
		}
		_ = a.powerdnsDB.DeleteDNSRecord(ctx, DeleteDNSRecordParams{
			DomainID: domainID,
			Name:     r.name,
			Type:     r.recType,
		})
	}

	return nil
}

// ReactivateAutoRecords re-pushes auto-managed records to PowerDNS when the
// overriding custom record is deleted. Only reactivates if no other custom
// record with the same name+type still exists.
func (a *DNS) ReactivateAutoRecords(ctx context.Context, params DeactivateAutoRecordsParams) error {
	// Check if another custom record still overrides.
	hasCustom, err := a.hasCustomRecord(ctx, params.Name, params.Type)
	if err != nil {
		return fmt.Errorf("check remaining custom records: %w", err)
	}
	if hasCustom {
		return nil
	}

	// Find auto records to reactivate.
	rows, err := a.coreDB.Query(ctx,
		`SELECT zr.name, zr.type, zr.content, zr.ttl, zr.priority, z.name AS zone_name
		 FROM zone_records zr
		 JOIN zones z ON z.id = zr.zone_id
		 WHERE zr.name = $1 AND zr.type = $2 AND zr.managed_by = 'auto' AND zr.status = 'active'`,
		params.Name, params.Type)
	if err != nil {
		return fmt.Errorf("find auto records to reactivate: %w", err)
	}
	defer rows.Close()

	type autoRec struct {
		name, recType, content, zoneName string
		ttl                              int
		priority                         *int
	}
	var recs []autoRec
	for rows.Next() {
		var r autoRec
		if err := rows.Scan(&r.name, &r.recType, &r.content, &r.ttl, &r.priority, &r.zoneName); err != nil {
			return fmt.Errorf("scan auto record: %w", err)
		}
		recs = append(recs, r)
	}

	for _, r := range recs {
		domainID, err := a.powerdnsDB.GetDNSZoneIDByName(ctx, r.zoneName)
		if err != nil || domainID == 0 {
			continue
		}
		_ = a.powerdnsDB.WriteDNSRecord(ctx, WriteDNSRecordParams{
			DomainID: domainID,
			Name:     r.name,
			Type:     r.recType,
			Content:  r.content,
			TTL:      r.ttl,
			Priority: r.priority,
		})
	}

	return nil
}

// RetroactiveAutoRecordsParams holds parameters for creating retroactive auto
// DNS records when a zone is created for a domain that already has FQDNs/email.
type RetroactiveAutoRecordsParams struct {
	ZoneName string `json:"zone_name"`
	ZoneID   string `json:"zone_id"`
	BrandID  string `json:"brand_id"`
}

// RetroactiveAutoRecords scans for existing active FQDNs and email accounts
// whose domains fall under a newly-created zone and creates their auto DNS
// records. This handles the zone-after-FQDN scenario.
func (a *DNS) RetroactiveAutoRecords(ctx context.Context, params RetroactiveAutoRecordsParams) error {
	// Get the brand for mail DNS config.
	var brand model.Brand
	err := a.coreDB.QueryRow(ctx,
		`SELECT id, name, base_hostname, primary_ns, secondary_ns, hostmaster_email,
		 mail_hostname, spf_includes, dkim_selector, dkim_public_key, dmarc_policy,
		 status, created_at, updated_at
		 FROM brands WHERE id = $1`, params.BrandID,
	).Scan(&brand.ID, &brand.Name, &brand.BaseHostname, &brand.PrimaryNS, &brand.SecondaryNS,
		&brand.HostmasterEmail, &brand.MailHostname, &brand.SPFIncludes, &brand.DKIMSelector,
		&brand.DKIMPublicKey, &brand.DMARCPolicy, &brand.Status, &brand.CreatedAt, &brand.UpdatedAt)
	if err != nil {
		return fmt.Errorf("get brand: %w", err)
	}

	// Get the PowerDNS domain ID.
	domainID, err := a.powerdnsDB.GetDNSZoneIDByName(ctx, params.ZoneName)
	if err != nil {
		return fmt.Errorf("get dns zone id: %w", err)
	}
	if domainID == 0 {
		return nil
	}

	// Get cluster LB addresses for the brand's clusters.
	lbAddresses, err := a.getLBAddressesForBrand(ctx, params.BrandID)
	if err != nil {
		return fmt.Errorf("get LB addresses: %w", err)
	}

	// Find all active FQDNs whose domain falls under this zone.
	// Match: fqdn = zone_name OR fqdn ends with .zone_name
	fqdnRows, err := a.coreDB.Query(ctx,
		`SELECT f.id, f.fqdn FROM fqdns f
		 WHERE f.status = 'active'
		 AND (f.fqdn = $1 OR f.fqdn LIKE '%.' || $1)`,
		params.ZoneName)
	if err != nil {
		return fmt.Errorf("find FQDNs for zone: %w", err)
	}
	defer fqdnRows.Close()

	type fqdnRef struct {
		id, fqdn string
	}
	var fqdns []fqdnRef
	for fqdnRows.Next() {
		var f fqdnRef
		if err := fqdnRows.Scan(&f.id, &f.fqdn); err != nil {
			return fmt.Errorf("scan fqdn: %w", err)
		}
		fqdns = append(fqdns, f)
	}

	// Create A/AAAA records for each FQDN.
	for _, f := range fqdns {
		for _, addr := range lbAddresses {
			var recordType string
			if addr.Family == 4 {
				recordType = "A"
			} else if addr.Family == 6 {
				recordType = "AAAA"
			} else {
				continue
			}

			hasCustom, _ := a.hasCustomRecord(ctx, f.fqdn, recordType)
			if !hasCustom {
				_ = a.powerdnsDB.WriteDNSRecord(ctx, WriteDNSRecordParams{
					DomainID: domainID,
					Name:     f.fqdn,
					Type:     recordType,
					Content:  addr.Address,
					TTL:      300,
				})
			}

			_, _ = a.coreDB.Exec(ctx,
				`INSERT INTO zone_records (id, zone_id, type, name, content, ttl, managed_by, source_type, source_fqdn_id, status, created_at, updated_at)
				 SELECT gen_random_uuid(), $1, $2, $3, $4, 300, 'auto', 'fqdn', $5, 'active', now(), now()
				 WHERE NOT EXISTS (SELECT 1 FROM zone_records WHERE zone_id = $1 AND type = $2 AND name = $3 AND content = $4 AND managed_by = 'auto')`,
				params.ZoneID, recordType, f.fqdn, addr.Address, f.id)
		}
	}

	// Find FQDNs that have email accounts and create email DNS records.
	emailFQDNRows, err := a.coreDB.Query(ctx,
		`SELECT DISTINCT f.id, f.fqdn FROM fqdns f
		 JOIN email_accounts ea ON ea.fqdn_id = f.id
		 WHERE f.status = 'active'
		 AND (f.fqdn = $1 OR f.fqdn LIKE '%.' || $1)`,
		params.ZoneName)
	if err != nil {
		return fmt.Errorf("find email FQDNs for zone: %w", err)
	}
	defer emailFQDNRows.Close()

	var emailFQDNs []fqdnRef
	for emailFQDNRows.Next() {
		var f fqdnRef
		if err := emailFQDNRows.Scan(&f.id, &f.fqdn); err != nil {
			return fmt.Errorf("scan email fqdn: %w", err)
		}
		emailFQDNs = append(emailFQDNs, f)
	}

	// Create email DNS records for each FQDN with email accounts.
	for _, f := range emailFQDNs {
		mailHostname := brand.MailHostname
		if mailHostname == "" {
			mailHostname = "mail." + f.fqdn
		}
		if err := a.AutoCreateEmailDNSRecords(ctx, AutoCreateEmailDNSRecordsParams{
			FQDN:          f.fqdn,
			MailHostname:  mailHostname,
			SPFIncludes:   brand.SPFIncludes,
			DKIMSelector:  brand.DKIMSelector,
			DKIMPublicKey: brand.DKIMPublicKey,
			DMARCPolicy:   brand.DMARCPolicy,
			SourceFQDNID:  f.id,
		}); err != nil {
			return fmt.Errorf("create email DNS for %s: %w", f.fqdn, err)
		}
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

// --- helpers ---

// autoRecordDef defines a single auto-managed DNS record to create.
type autoRecordDef struct {
	zoneName     string
	domainID     int
	fqdn         string
	recordType   string
	content      string
	ttl          int
	priority     *int
	sourceType   string
	sourceFQDNID string
}

// createAutoRecord creates an auto-managed record in both PowerDNS and core DB.
// If a custom record with the same name+type exists, the record is stored in core
// DB only (not pushed to PowerDNS) — it will be activated when the custom record
// is deleted.
func (a *DNS) createAutoRecord(ctx context.Context, def autoRecordDef) error {
	hasCustom, err := a.hasCustomRecord(ctx, def.fqdn, def.recordType)
	if err != nil {
		return fmt.Errorf("check custom override: %w", err)
	}

	if !hasCustom && def.domainID > 0 {
		if err := a.powerdnsDB.WriteDNSRecord(ctx, WriteDNSRecordParams{
			DomainID: def.domainID,
			Name:     def.fqdn,
			Type:     def.recordType,
			Content:  def.content,
			TTL:      def.ttl,
			Priority: def.priority,
		}); err != nil {
			return fmt.Errorf("write to PowerDNS: %w", err)
		}
	}

	// Store in core DB (idempotent — skip if already exists).
	_, err = a.coreDB.Exec(ctx,
		`INSERT INTO zone_records (id, zone_id, type, name, content, ttl, priority, managed_by, source_type, source_fqdn_id, status, created_at, updated_at)
		 SELECT gen_random_uuid(), z.id, $1, $2, $3, $4, $5, 'auto', $6, $7, 'active', now(), now()
		 FROM zones z WHERE z.name = $8 AND z.status = 'active'
		 AND NOT EXISTS (SELECT 1 FROM zone_records zr WHERE zr.zone_id = z.id AND zr.type = $1 AND zr.name = $2 AND zr.managed_by = 'auto' AND zr.source_type = $6)`,
		def.recordType, def.fqdn, def.content, def.ttl, def.priority,
		def.sourceType, def.sourceFQDNID, def.zoneName,
	)
	if err != nil {
		return fmt.Errorf("record in core db: %w", err)
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

// hasCustomRecord checks if a custom-managed DNS record exists for the given
// FQDN and record types in the core DB.
func (a *DNS) hasCustomRecord(ctx context.Context, fqdn string, types ...string) (bool, error) {
	if len(types) == 0 {
		return false, nil
	}
	placeholders := make([]string, len(types))
	args := []any{fqdn}
	for i, t := range types {
		placeholders[i] = fmt.Sprintf("$%d", i+2)
		args = append(args, t)
	}
	var count int
	err := a.coreDB.QueryRow(ctx,
		fmt.Sprintf(`SELECT COUNT(*) FROM zone_records
		 WHERE name = $1 AND managed_by = 'custom' AND status = 'active'
		 AND type IN (%s)`, strings.Join(placeholders, ", ")),
		args...,
	).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// buildSPFRecord builds an SPF TXT record content from optional includes.
func buildSPFRecord(spfIncludes string) string {
	if spfIncludes == "" {
		return "v=spf1 mx ~all"
	}
	parts := strings.Split(spfIncludes, ",")
	var includes []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			includes = append(includes, "include:"+p)
		}
	}
	if len(includes) == 0 {
		return "v=spf1 mx ~all"
	}
	return "v=spf1 mx " + strings.Join(includes, " ") + " ~all"
}

// getLBAddressesForBrand gets LB addresses from clusters associated with a brand.
func (a *DNS) getLBAddressesForBrand(ctx context.Context, brandID string) ([]model.ClusterLBAddress, error) {
	rows, err := a.coreDB.Query(ctx,
		`SELECT la.id, la.cluster_id, la.address::text, la.family, la.label, la.created_at
		 FROM cluster_lb_addresses la
		 JOIN brand_clusters bc ON bc.cluster_id = la.cluster_id
		 WHERE bc.brand_id = $1
		 ORDER BY la.family, la.address
		 LIMIT 10`, brandID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var addrs []model.ClusterLBAddress
	for rows.Next() {
		var a model.ClusterLBAddress
		if err := rows.Scan(&a.ID, &a.ClusterID, &a.Address, &a.Family, &a.Label, &a.CreatedAt); err != nil {
			return nil, err
		}
		addrs = append(addrs, a)
	}
	return addrs, rows.Err()
}

func intPtr(v int) *int {
	return &v
}
