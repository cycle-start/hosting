package platform

import "fmt"

// ServiceHostname generates a service hostname for a tenant.
// Example: ssh.acme.no-1.hosting.example.com
func ServiceHostname(baseHostname, tenantName, service string) string {
	return fmt.Sprintf("%s.%s.%s", service, tenantName, baseHostname)
}
