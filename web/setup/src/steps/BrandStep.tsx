import { Label } from '@/components/ui/label'
import { Input } from '@/components/ui/input'
import type { Config } from '@/lib/types'

interface Props {
  config: Config
  onChange: (config: Config) => void
}

export function BrandStep({ config, onChange }: Props) {
  const brand = config.brand

  const updateBrand = (updates: Partial<typeof brand>) => {
    onChange({ ...config, brand: { ...brand, ...updates } })
  }

  return (
    <div className="space-y-6">
      <div>
        <h2 className="text-xl font-semibold">Brand & Domains</h2>
        <p className="text-muted-foreground mt-1">
          Configure your brand identity and the domains used by the platform.
        </p>
      </div>

      <div className="grid gap-6 max-w-lg">
        <div className="space-y-2">
          <Label htmlFor="brand_name">Brand Name</Label>
          <Input
            id="brand_name"
            placeholder="e.g. Acme Hosting"
            value={brand.name}
            onChange={(e) => updateBrand({ name: e.target.value })}
          />
        </div>

        <div className="border-t pt-4">
          <h3 className="text-sm font-medium mb-3">Domains</h3>
          <div className="grid gap-4">
            <div className="space-y-2">
              <Label htmlFor="platform_domain">Platform Domain</Label>
              <Input
                id="platform_domain"
                placeholder="e.g. platform.example.com"
                value={brand.platform_domain}
                onChange={(e) => updateBrand({ platform_domain: e.target.value })}
              />
              <p className="text-xs text-muted-foreground">
                Admin UI, API, and control plane services will be accessible under this domain.
              </p>
            </div>

            <div className="space-y-2">
              <Label htmlFor="customer_domain">Customer Domain</Label>
              <Input
                id="customer_domain"
                placeholder="e.g. hosting.example.com"
                value={brand.customer_domain}
                onChange={(e) => updateBrand({ customer_domain: e.target.value })}
              />
              <p className="text-xs text-muted-foreground">
                Hosted sites get service hostnames under this domain (e.g. site123.hosting.example.com).
              </p>
            </div>
          </div>
        </div>

        <div className="border-t pt-4">
          <h3 className="text-sm font-medium mb-3">DNS (Nameservers)</h3>
          <div className="grid gap-4">
            <div className="grid grid-cols-2 gap-3">
              <div className="space-y-2">
                <Label htmlFor="primary_ns">Primary NS</Label>
                <Input
                  id="primary_ns"
                  placeholder="ns1.hosting.example.com"
                  value={brand.primary_ns}
                  onChange={(e) => updateBrand({ primary_ns: e.target.value })}
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="primary_ns_ip">Primary NS IP</Label>
                <Input
                  id="primary_ns_ip"
                  placeholder="203.0.113.10"
                  value={brand.primary_ns_ip}
                  onChange={(e) => updateBrand({ primary_ns_ip: e.target.value })}
                />
              </div>
            </div>
            <div className="grid grid-cols-2 gap-3">
              <div className="space-y-2">
                <Label htmlFor="secondary_ns">Secondary NS</Label>
                <Input
                  id="secondary_ns"
                  placeholder="ns2.hosting.example.com"
                  value={brand.secondary_ns}
                  onChange={(e) => updateBrand({ secondary_ns: e.target.value })}
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="secondary_ns_ip">Secondary NS IP</Label>
                <Input
                  id="secondary_ns_ip"
                  placeholder="203.0.113.11"
                  value={brand.secondary_ns_ip}
                  onChange={(e) => updateBrand({ secondary_ns_ip: e.target.value })}
                />
              </div>
            </div>
            <p className="text-xs text-muted-foreground">
              These nameservers will be authoritative for customer DNS zones.
              Register glue records at your registrar pointing to these IPs.
            </p>
          </div>
        </div>

        <div className="border-t pt-4">
          <h3 className="text-sm font-medium mb-3">Email</h3>
          <div className="grid gap-4">
            <div className="space-y-2">
              <Label htmlFor="mail_hostname">Mail Hostname (MX)</Label>
              <Input
                id="mail_hostname"
                placeholder="mail.hosting.example.com"
                value={brand.mail_hostname}
                onChange={(e) => updateBrand({ mail_hostname: e.target.value })}
              />
              <p className="text-xs text-muted-foreground">
                MX records for customer domains will point to this hostname.
              </p>
            </div>

            <div className="space-y-2">
              <Label htmlFor="hostmaster_email">Hostmaster Email</Label>
              <Input
                id="hostmaster_email"
                placeholder="hostmaster@example.com"
                value={brand.hostmaster_email}
                onChange={(e) => updateBrand({ hostmaster_email: e.target.value })}
              />
              <p className="text-xs text-muted-foreground">
                Used in DNS SOA records as the responsible party contact.
              </p>
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}
