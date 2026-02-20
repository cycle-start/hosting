package request

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func intPtr(v int) *int { return &v }

func TestValidateZoneRecord(t *testing.T) {
	tests := []struct {
		name     string
		rtype    string
		rname    string
		content  string
		priority *int
		wantErr  string
	}{
		// --- A records ---
		{"A valid IPv4", "A", "@", "1.2.3.4", nil, ""},
		{"A valid IPv4 alt", "A", "www", "192.168.0.1", nil, ""},
		{"A invalid content", "A", "@", "banana", nil, "valid IPv4"},
		{"A IPv6 rejected", "A", "@", "::1", nil, "valid IPv4"},
		{"A priority forbidden", "A", "@", "1.2.3.4", intPtr(10), "must not have a priority"},

		// --- AAAA records ---
		{"AAAA valid IPv6", "AAAA", "@", "2001:db8::1", nil, ""},
		{"AAAA full IPv6", "AAAA", "@", "2001:0db8:85a3:0000:0000:8a2e:0370:7334", nil, ""},
		{"AAAA invalid content", "AAAA", "@", "1.2.3.4", nil, "valid IPv6"},
		{"AAAA invalid string", "AAAA", "@", "not-an-ip", nil, "valid IPv6"},
		{"AAAA priority forbidden", "AAAA", "@", "::1", intPtr(0), "must not have a priority"},

		// --- CNAME records ---
		{"CNAME valid", "CNAME", "www", "example.com", nil, ""},
		{"CNAME valid trailing dot", "CNAME", "www", "example.com.", nil, ""},
		{"CNAME invalid", "CNAME", "www", "not valid!", nil, "valid hostname"},
		{"CNAME priority forbidden", "CNAME", "www", "example.com", intPtr(0), "must not have a priority"},

		// --- MX records ---
		{"MX valid", "MX", "@", "mail.example.com", intPtr(10), ""},
		{"MX missing priority", "MX", "@", "mail.example.com", nil, "requires a priority"},
		{"MX invalid hostname", "MX", "@", "not valid!", intPtr(10), "valid hostname"},

		// --- TXT records ---
		{"TXT valid", "TXT", "@", "v=spf1 include:example.com ~all", nil, ""},
		{"TXT empty", "TXT", "@", "", nil, "must not be empty"},
		{"TXT priority forbidden", "TXT", "@", "some text", intPtr(0), "must not have a priority"},

		// --- SRV records ---
		{"SRV valid", "SRV", "_sip._tcp", "5 5060 sipserver.example.com", intPtr(10), ""},
		{"SRV dot target", "SRV", "_sip._tcp", "0 0 .", intPtr(0), ""},
		{"SRV missing priority", "SRV", "_sip._tcp", "5 5060 sipserver.example.com", nil, "requires a priority"},
		{"SRV bad weight", "SRV", "_sip._tcp", "abc 5060 sip.example.com", intPtr(10), "weight must be"},
		{"SRV bad port", "SRV", "_sip._tcp", "5 abc sip.example.com", intPtr(10), "port must be"},
		{"SRV bad target", "SRV", "_sip._tcp", "5 5060 -invalid", intPtr(10), "target must be"},
		{"SRV too few fields", "SRV", "_sip._tcp", "5 5060", intPtr(10), "weight port target"},

		// --- NS records ---
		{"NS valid", "NS", "@", "ns1.example.com", nil, ""},
		{"NS invalid", "NS", "@", "not valid!", nil, "valid hostname"},

		// --- CAA records ---
		{"CAA valid issue", "CAA", "@", `0 issue "letsencrypt.org"`, nil, ""},
		{"CAA valid issuewild", "CAA", "@", `0 issuewild ";"`, nil, ""},
		{"CAA bad flag", "CAA", "@", `999 issue "letsencrypt.org"`, nil, "flag must be"},
		{"CAA bad tag", "CAA", "@", `0 badtag "value"`, nil, "tag must be one of"},
		{"CAA too few fields", "CAA", "@", "0 issue", nil, "flag tag value"},

		// --- PTR records ---
		{"PTR valid", "PTR", "1", "host.example.com", nil, ""},
		{"PTR invalid", "PTR", "1", "not valid!", nil, "valid hostname"},

		// --- ALIAS records ---
		{"ALIAS valid", "ALIAS", "@", "example.com", nil, ""},
		{"ALIAS invalid", "ALIAS", "@", "not valid!", nil, "valid hostname"},

		// --- DNAME records ---
		{"DNAME valid", "DNAME", "sub", "example.com", nil, ""},

		// --- TLSA records ---
		{"TLSA valid", "TLSA", "_443._tcp", "3 1 1 aabbccdd", nil, ""},
		{"TLSA bad usage", "TLSA", "_443._tcp", "5 1 1 aabbccdd", nil, "usage must be"},
		{"TLSA bad selector", "TLSA", "_443._tcp", "3 5 1 aabbccdd", nil, "selector must be"},
		{"TLSA bad match type", "TLSA", "_443._tcp", "3 1 5 aabbccdd", nil, "matching type must be"},
		{"TLSA bad hex", "TLSA", "_443._tcp", "3 1 1 notahex!", nil, "valid hex string"},
		{"TLSA too few fields", "TLSA", "_443._tcp", "3 1 1", nil, "usage selector"},

		// --- DS records ---
		{"DS valid", "DS", "@", "12345 8 2 aabbccdd", nil, ""},
		{"DS bad keytag", "DS", "@", "abc 8 2 aabbccdd", nil, "keytag must be"},
		{"DS bad hex", "DS", "@", "12345 8 2 nothex!", nil, "valid hex string"},
		{"DS too few", "DS", "@", "12345 8 2", nil, "keytag algorithm"},

		// --- DNSKEY records ---
		{"DNSKEY valid", "DNSKEY", "@", "257 3 13 dGVzdA==", nil, ""},
		{"DNSKEY bad proto", "DNSKEY", "@", "257 1 13 dGVzdA==", nil, "protocol must be 3"},
		{"DNSKEY bad base64", "DNSKEY", "@", "257 3 13 !!!invalid", nil, "valid base64"},
		{"DNSKEY too few", "DNSKEY", "@", "257 3 13", nil, "flags protocol"},

		// --- NAPTR records ---
		{"NAPTR valid", "NAPTR", "@", `100 10 "u" "sip+E2U" "!^.*$!sip:info@example.com!" .`, nil, ""},
		{"NAPTR bad order", "NAPTR", "@", `abc 10 "u" "sip" "regexp" .`, nil, "order must be"},
		{"NAPTR too few", "NAPTR", "@", `100 10 "u" "sip" "regexp"`, nil, "order preference"},

		// --- SSHFP records ---
		{"SSHFP valid", "SSHFP", "@", "1 1 aabbccdd", nil, ""},
		{"SSHFP algo 4", "SSHFP", "@", "4 2 aabbccdd", nil, ""},
		{"SSHFP bad algo", "SSHFP", "@", "5 1 aabbccdd", nil, "algorithm must be"},
		{"SSHFP bad fptype", "SSHFP", "@", "1 3 aabbccdd", nil, "fingerprint type must be"},
		{"SSHFP bad hex", "SSHFP", "@", "1 1 nothex!", nil, "valid hex string"},
		{"SSHFP too few", "SSHFP", "@", "1 1", nil, "algorithm fingerprint_type"},

		// --- HTTPS/SVCB records ---
		{"HTTPS valid", "HTTPS", "@", "1 . alpn=h2", intPtr(1), ""},
		{"HTTPS no params", "HTTPS", "@", "0 example.com", nil, ""},
		{"SVCB valid", "SVCB", "@", "1 target.example.com", intPtr(1), ""},
		{"HTTPS bad priority", "HTTPS", "@", "abc example.com", nil, "priority must be an integer"},
		{"HTTPS bad target", "HTTPS", "@", "1 -invalid!", nil, "valid hostname"},

		// --- LOC records ---
		{"LOC valid", "LOC", "@", "52 22 23.000 N 4 53 32.000 E -2.00m 0.00m 10000m 10m", nil, ""},
		{"LOC empty", "LOC", "@", "", nil, "must not be empty"},

		// --- Name validation ---
		{"name @ valid", "A", "@", "1.2.3.4", nil, ""},
		{"name wildcard valid", "A", "*", "1.2.3.4", nil, ""},
		{"name wildcard prefix", "A", "*.example", "1.2.3.4", nil, ""},
		{"name subdomain valid", "A", "sub.domain", "1.2.3.4", nil, ""},
		{"name leading hyphen", "A", "-bad", "1.2.3.4", nil, "not a valid DNS name"},
		{"name empty label", "A", "sub..domain", "1.2.3.4", nil, "not a valid DNS name"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateZoneRecord(tt.rtype, tt.rname, tt.content, tt.priority)
			if tt.wantErr == "" {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			}
		})
	}
}

func TestIsValidHostname(t *testing.T) {
	tests := []struct {
		hostname string
		valid    bool
	}{
		{"example.com", true},
		{"sub.example.com", true},
		{"example.com.", true},
		{"a.b.c.d.e.f", true},
		{"xn--nxasmq6b.example.com", true},
		{"_dmarc.example.com", true},
		{"", false},
		{".", false},
		{"-bad.com", false},
		{"bad-.com", false},
		{"exam ple.com", false},
		{"exam!ple.com", false},
		{"a." + string(make([]byte, 64)), false}, // label too long
	}

	for _, tt := range tests {
		t.Run(tt.hostname, func(t *testing.T) {
			assert.Equal(t, tt.valid, isValidHostname(tt.hostname))
		})
	}
}
