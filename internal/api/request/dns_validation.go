package request

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net"
	"strconv"
	"strings"
)

// ValidateZoneRecord validates DNS record content, name, and priority based on the record type.
func ValidateZoneRecord(recordType, name, content string, priority *int) error {
	if err := validateRecordName(name); err != nil {
		return err
	}
	if err := validateRecordContent(recordType, content); err != nil {
		return err
	}
	return validateRecordPriority(recordType, priority)
}

// validateRecordName checks that the DNS record name is valid.
// Accepts "@" for zone apex, "*" or wildcard prefixes like "*.sub", and standard hostnames.
func validateRecordName(name string) error {
	if name == "@" || name == "*" {
		return nil
	}
	// Wildcard: must start with "*." followed by a valid hostname (or just "*." + label).
	check := name
	if strings.HasPrefix(name, "*.") {
		check = name[2:]
		if check == "" {
			return fmt.Errorf("record name: wildcard must be followed by a hostname")
		}
	}
	if !isValidHostname(check) {
		return fmt.Errorf("record name %q is not a valid DNS name", name)
	}
	return nil
}

// validateRecordContent validates the content field based on the record type.
func validateRecordContent(recordType, content string) error {
	switch recordType {
	case "A":
		ip := net.ParseIP(content)
		if ip == nil || ip.To4() == nil {
			return fmt.Errorf("A record content must be a valid IPv4 address")
		}
	case "AAAA":
		ip := net.ParseIP(content)
		if ip == nil || ip.To4() != nil {
			return fmt.Errorf("AAAA record content must be a valid IPv6 address")
		}
	case "CNAME", "NS", "PTR", "ALIAS", "DNAME":
		if !isValidHostname(content) {
			return fmt.Errorf("%s record content must be a valid hostname", recordType)
		}
	case "MX":
		if !isValidHostname(content) {
			return fmt.Errorf("MX record content must be a valid hostname")
		}
	case "TXT":
		if content == "" {
			return fmt.Errorf("TXT record content must not be empty")
		}
		if len(content) > 4096 {
			return fmt.Errorf("TXT record content must not exceed 4096 characters")
		}
	case "SRV":
		if err := validateSRVContent(content); err != nil {
			return err
		}
	case "CAA":
		if err := validateCAAContent(content); err != nil {
			return err
		}
	case "TLSA":
		if err := validateTLSAContent(content); err != nil {
			return err
		}
	case "DS":
		if err := validateDSContent(content); err != nil {
			return err
		}
	case "DNSKEY":
		if err := validateDNSKEYContent(content); err != nil {
			return err
		}
	case "NAPTR":
		if err := validateNAPTRContent(content); err != nil {
			return err
		}
	case "SSHFP":
		if err := validateSSHFPContent(content); err != nil {
			return err
		}
	case "HTTPS", "SVCB":
		if err := validateSVCBContent(content); err != nil {
			return err
		}
	case "LOC":
		if content == "" {
			return fmt.Errorf("LOC record content must not be empty")
		}
	}
	return nil
}

// validateRecordPriority enforces priority requirements per record type.
func validateRecordPriority(recordType string, priority *int) error {
	switch recordType {
	case "MX", "SRV":
		if priority == nil {
			return fmt.Errorf("%s record requires a priority value", recordType)
		}
		if *priority < 0 || *priority > 65535 {
			return fmt.Errorf("%s record priority must be between 0 and 65535", recordType)
		}
	case "HTTPS", "SVCB":
		// Priority is optional for HTTPS/SVCB.
		if priority != nil && (*priority < 0 || *priority > 65535) {
			return fmt.Errorf("%s record priority must be between 0 and 65535", recordType)
		}
	default:
		if priority != nil {
			return fmt.Errorf("%s record must not have a priority value", recordType)
		}
	}
	return nil
}

// isValidHostname checks if s is a valid DNS hostname.
// Labels separated by dots, each label 1-63 chars, alphanumeric + hyphens,
// no leading/trailing hyphens, total max 253 chars.
func isValidHostname(s string) bool {
	if s == "" || len(s) > 253 {
		return false
	}
	// Remove trailing dot (FQDN notation).
	s = strings.TrimSuffix(s, ".")
	if s == "" {
		return false
	}
	labels := strings.Split(s, ".")
	for _, label := range labels {
		if !isValidLabel(label) {
			return false
		}
	}
	return true
}

// isValidLabel checks if a single DNS label is valid.
func isValidLabel(label string) bool {
	n := len(label)
	if n == 0 || n > 63 {
		return false
	}
	if label[0] == '-' || label[n-1] == '-' {
		return false
	}
	for _, c := range label {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_') {
			return false
		}
	}
	return true
}

// validateSRVContent validates SRV record content: "{weight} {port} {target}".
func validateSRVContent(content string) error {
	parts := strings.Fields(content)
	if len(parts) != 3 {
		return fmt.Errorf("SRV record content must be in format: weight port target")
	}
	w, err := strconv.Atoi(parts[0])
	if err != nil || w < 0 || w > 65535 {
		return fmt.Errorf("SRV record weight must be an integer between 0 and 65535")
	}
	p, err := strconv.Atoi(parts[1])
	if err != nil || p < 0 || p > 65535 {
		return fmt.Errorf("SRV record port must be an integer between 0 and 65535")
	}
	if parts[2] != "." && !isValidHostname(parts[2]) {
		return fmt.Errorf("SRV record target must be a valid hostname")
	}
	return nil
}

// validateCAAContent validates CAA record content: '{flag} {tag} "{value}"'.
func validateCAAContent(content string) error {
	parts := strings.SplitN(content, " ", 3)
	if len(parts) < 3 {
		return fmt.Errorf("CAA record content must be in format: flag tag value")
	}
	flag, err := strconv.Atoi(parts[0])
	if err != nil || flag < 0 || flag > 255 {
		return fmt.Errorf("CAA record flag must be an integer between 0 and 255")
	}
	validTags := map[string]bool{
		"issue": true, "issuewild": true, "iodef": true,
		"contactemail": true, "contactphone": true,
	}
	if !validTags[parts[1]] {
		return fmt.Errorf("CAA record tag must be one of: issue, issuewild, iodef, contactemail, contactphone")
	}
	if parts[2] == "" {
		return fmt.Errorf("CAA record value must not be empty")
	}
	return nil
}

// validateTLSAContent validates TLSA record content: "{usage} {selector} {matching_type} {hex}".
func validateTLSAContent(content string) error {
	parts := strings.Fields(content)
	if len(parts) != 4 {
		return fmt.Errorf("TLSA record content must be in format: usage selector matching_type certificate_data")
	}
	usage, err := strconv.Atoi(parts[0])
	if err != nil || usage < 0 || usage > 3 {
		return fmt.Errorf("TLSA record usage must be an integer between 0 and 3")
	}
	selector, err := strconv.Atoi(parts[1])
	if err != nil || selector < 0 || selector > 1 {
		return fmt.Errorf("TLSA record selector must be 0 or 1")
	}
	matchType, err := strconv.Atoi(parts[2])
	if err != nil || matchType < 0 || matchType > 2 {
		return fmt.Errorf("TLSA record matching type must be 0, 1, or 2")
	}
	if _, err := hex.DecodeString(parts[3]); err != nil {
		return fmt.Errorf("TLSA record certificate data must be a valid hex string")
	}
	return nil
}

// validateDSContent validates DS record content: "{keytag} {algo} {digesttype} {hex}".
func validateDSContent(content string) error {
	parts := strings.Fields(content)
	if len(parts) != 4 {
		return fmt.Errorf("DS record content must be in format: keytag algorithm digest_type digest")
	}
	keytag, err := strconv.Atoi(parts[0])
	if err != nil || keytag < 0 || keytag > 65535 {
		return fmt.Errorf("DS record keytag must be an integer between 0 and 65535")
	}
	if _, err := strconv.Atoi(parts[1]); err != nil {
		return fmt.Errorf("DS record algorithm must be an integer")
	}
	if _, err := strconv.Atoi(parts[2]); err != nil {
		return fmt.Errorf("DS record digest type must be an integer")
	}
	if _, err := hex.DecodeString(parts[3]); err != nil {
		return fmt.Errorf("DS record digest must be a valid hex string")
	}
	return nil
}

// validateDNSKEYContent validates DNSKEY record content: "{flags} {proto} {algo} {base64key}".
func validateDNSKEYContent(content string) error {
	parts := strings.Fields(content)
	if len(parts) < 4 {
		return fmt.Errorf("DNSKEY record content must be in format: flags protocol algorithm public_key")
	}
	flags, err := strconv.Atoi(parts[0])
	if err != nil || flags < 0 || flags > 65535 {
		return fmt.Errorf("DNSKEY record flags must be an integer between 0 and 65535")
	}
	proto, err := strconv.Atoi(parts[1])
	if err != nil || proto != 3 {
		return fmt.Errorf("DNSKEY record protocol must be 3")
	}
	if _, err := strconv.Atoi(parts[2]); err != nil {
		return fmt.Errorf("DNSKEY record algorithm must be an integer")
	}
	// Key data may contain spaces in base64 — rejoin remaining parts.
	keyData := strings.Join(parts[3:], "")
	if _, err := base64.StdEncoding.DecodeString(keyData); err != nil {
		return fmt.Errorf("DNSKEY record public key must be valid base64")
	}
	return nil
}

// validateNAPTRContent validates NAPTR record content.
// Format: {order} {pref} "{flags}" "{service}" "{regexp}" replacement
func validateNAPTRContent(content string) error {
	parts := strings.Fields(content)
	if len(parts) < 6 {
		return fmt.Errorf("NAPTR record content must be in format: order preference flags service regexp replacement")
	}
	order, err := strconv.Atoi(parts[0])
	if err != nil || order < 0 || order > 65535 {
		return fmt.Errorf("NAPTR record order must be an integer between 0 and 65535")
	}
	pref, err := strconv.Atoi(parts[1])
	if err != nil || pref < 0 || pref > 65535 {
		return fmt.Errorf("NAPTR record preference must be an integer between 0 and 65535")
	}
	return nil
}

// validateSSHFPContent validates SSHFP record content: "{algo} {fptype} {hex}".
func validateSSHFPContent(content string) error {
	parts := strings.Fields(content)
	if len(parts) != 3 {
		return fmt.Errorf("SSHFP record content must be in format: algorithm fingerprint_type fingerprint")
	}
	algo, err := strconv.Atoi(parts[0])
	if err != nil || algo < 1 || algo > 4 {
		return fmt.Errorf("SSHFP record algorithm must be an integer between 1 and 4")
	}
	fpType, err := strconv.Atoi(parts[1])
	if err != nil || fpType < 1 || fpType > 2 {
		return fmt.Errorf("SSHFP record fingerprint type must be 1 or 2")
	}
	if _, err := hex.DecodeString(parts[2]); err != nil {
		return fmt.Errorf("SSHFP record fingerprint must be a valid hex string")
	}
	return nil
}

// validateSVCBContent validates HTTPS/SVCB record content.
// Basic format: "{priority} {target} [params...]" or just a target for AliasMode.
func validateSVCBContent(content string) error {
	parts := strings.Fields(content)
	if len(parts) < 2 {
		return fmt.Errorf("HTTPS/SVCB record content must have at least a priority and target")
	}
	if _, err := strconv.Atoi(parts[0]); err != nil {
		return fmt.Errorf("HTTPS/SVCB record priority must be an integer")
	}
	// Target can be "." (use owner name) or a valid hostname.
	if parts[1] != "." && !isValidHostname(parts[1]) {
		return fmt.Errorf("HTTPS/SVCB record target must be a valid hostname or \".\"")
	}
	return nil
}
