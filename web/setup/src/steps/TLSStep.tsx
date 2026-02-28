import { Label } from '@/components/ui/label'
import { Input } from '@/components/ui/input'
import { FieldError } from '@/components/field-error'
import { fieldError } from '@/lib/validation'
import type { Config, ValidationError } from '@/lib/types'
import { cn } from '@/lib/utils'
import { ShieldCheck, FileKey, Shield, Globe } from 'lucide-react'

interface Props {
  config: Config
  onChange: (config: Config) => void
  errors: ValidationError[]
}

const defaultSSO = { mode: '' as const, admin_email: '', admin_password: '', issuer_url: '', client_id: '', client_secret: '' }

export function TLSStep({ config, onChange, errors }: Props) {
  const tls = config.tls
  const sso = config.sso ?? defaultSSO

  const updateTLS = (updates: Partial<typeof tls>) => {
    onChange({ ...config, tls: { ...tls, ...updates } })
  }

  const updateSSO = (updates: Partial<typeof sso>) => {
    onChange({ ...config, sso: { ...sso, ...updates } })
  }

  const fe = (field: string) => fieldError(errors, field)

  return (
    <div className="space-y-8">
      <div>
        <h2 className="text-xl font-semibold">Security</h2>
        <p className="text-muted-foreground mt-1">
          Configure TLS certificates and single sign-on for the platform.
        </p>
      </div>

      {/* TLS Section */}
      <div className="space-y-4">
        <h3 className="text-sm font-medium text-muted-foreground uppercase tracking-wide">TLS Certificates</h3>
        <div className="grid gap-4 max-w-lg">
          <div className="grid grid-cols-2 gap-3">
            {(['letsencrypt', 'manual'] as const).map((mode) => {
              const selected = tls.mode === mode
              const Icon = mode === 'letsencrypt' ? ShieldCheck : FileKey
              return (
                <button
                  key={mode}
                  onClick={() => updateTLS({ mode })}
                  className={cn(
                    'flex items-center gap-3 rounded-lg border p-4 text-left transition-colors hover:bg-accent/50',
                    selected && 'border-primary bg-accent/50 ring-1 ring-primary'
                  )}
                >
                  <Icon className="h-5 w-5 shrink-0" />
                  <div>
                    <div className="font-medium text-sm">
                      {mode === 'letsencrypt' ? "Let's Encrypt" : 'Manual'}
                    </div>
                    <div className="text-xs text-muted-foreground">
                      {mode === 'letsencrypt'
                        ? 'Automatic certificate provisioning'
                        : 'Provide your own certificates'}
                    </div>
                  </div>
                </button>
              )
            })}
          </div>

          {tls.mode === 'letsencrypt' && (
            <div className="space-y-2">
              <Label htmlFor="tls_email">Contact Email</Label>
              <Input
                id="tls_email"
                type="email"
                placeholder="admin@example.com"
                value={tls.email}
                onChange={(e) => updateTLS({ email: e.target.value })}
                className={fe('tls.email') ? 'border-destructive' : ''}
              />
              <FieldError error={fe('tls.email')} />
              {!fe('tls.email') && (
                <p className="text-xs text-muted-foreground">
                  Let's Encrypt will send expiration warnings to this address.
                </p>
              )}
            </div>
          )}

          {tls.mode === 'manual' && (
            <p className="text-sm text-muted-foreground border-t pt-4">
              You'll need to provide TLS certificates manually after deployment.
              Place them in the configured certificate directory on each web/LB node.
            </p>
          )}
        </div>
      </div>

      {/* SSO Section */}
      <div className="space-y-4">
        <h3 className="text-sm font-medium text-muted-foreground uppercase tracking-wide">Single Sign-On (SSO)</h3>
        <div className="grid gap-4 max-w-lg">
          <div className="grid grid-cols-3 gap-3">
            {([
              { mode: 'internal' as const, icon: Shield, label: 'Built-in', desc: 'Authelia (self-hosted IdP)' },
              { mode: 'external' as const, icon: Globe, label: 'External', desc: 'Your own OIDC provider' },
              { mode: '' as const, icon: FileKey, label: 'Disabled', desc: 'No SSO authentication' },
            ]).map(({ mode, icon: Icon, label, desc }) => {
              const selected = (sso.mode || '') === mode
              return (
                <button
                  key={mode}
                  onClick={() => updateSSO({ mode })}
                  className={cn(
                    'flex items-center gap-3 rounded-lg border p-4 text-left transition-colors hover:bg-accent/50',
                    selected && 'border-primary bg-accent/50 ring-1 ring-primary'
                  )}
                >
                  <Icon className="h-5 w-5 shrink-0" />
                  <div>
                    <div className="font-medium text-sm">{label}</div>
                    <div className="text-xs text-muted-foreground">{desc}</div>
                  </div>
                </button>
              )
            })}
          </div>

          {sso.mode === 'internal' && (
            <div className="space-y-4 border-l-2 border-primary/30 pl-4">
              <p className="text-xs text-muted-foreground">
                These credentials are used to log into all control plane services (Grafana, Headlamp, Temporal UI, Prometheus, Admin UI).
              </p>

              <div className="space-y-2">
                <Label htmlFor="sso_admin_email">Admin Email</Label>
                <Input
                  id="sso_admin_email"
                  type="email"
                  placeholder="admin@example.com"
                  value={sso.admin_email}
                  onChange={(e) => updateSSO({ admin_email: e.target.value })}
                  className={fe('sso.admin_email') ? 'border-destructive' : ''}
                />
                <FieldError error={fe('sso.admin_email')} />
              </div>

              <div className="space-y-2">
                <Label htmlFor="sso_admin_password">Admin Password</Label>
                <Input
                  id="sso_admin_password"
                  type="password"
                  placeholder="Min 8 characters"
                  value={sso.admin_password}
                  onChange={(e) => updateSSO({ admin_password: e.target.value })}
                  className={fe('sso.admin_password') ? 'border-destructive' : ''}
                />
                <FieldError error={fe('sso.admin_password')} />
              </div>
            </div>
          )}

          {sso.mode === 'external' && (
            <div className="space-y-4 border-l-2 border-primary/30 pl-4">
              <p className="text-xs text-muted-foreground">
                Provide your OIDC provider details. All control plane services will authenticate through this provider.
              </p>

              <div className="space-y-2">
                <Label htmlFor="sso_issuer_url">Issuer URL</Label>
                <Input
                  id="sso_issuer_url"
                  placeholder="https://login.microsoftonline.com/<tenant>/v2.0"
                  value={sso.issuer_url}
                  onChange={(e) => updateSSO({ issuer_url: e.target.value })}
                  className={fe('sso.issuer_url') ? 'border-destructive' : ''}
                />
                <FieldError error={fe('sso.issuer_url')} />
              </div>

              <div className="space-y-2">
                <Label htmlFor="sso_client_id">Client ID</Label>
                <Input
                  id="sso_client_id"
                  placeholder="Application (Client) ID"
                  value={sso.client_id}
                  onChange={(e) => updateSSO({ client_id: e.target.value })}
                  className={fe('sso.client_id') ? 'border-destructive' : ''}
                />
                <FieldError error={fe('sso.client_id')} />
              </div>

              <div className="space-y-2">
                <Label htmlFor="sso_client_secret">Client Secret</Label>
                <Input
                  id="sso_client_secret"
                  type="password"
                  placeholder="Client secret value"
                  value={sso.client_secret}
                  onChange={(e) => updateSSO({ client_secret: e.target.value })}
                  className={fe('sso.client_secret') ? 'border-destructive' : ''}
                />
                <FieldError error={fe('sso.client_secret')} />
              </div>

              <div className="rounded-lg border bg-muted/30 p-3 space-y-2">
                <p className="text-xs text-muted-foreground">
                  Register an app in your OIDC provider and add these redirect URIs:
                </p>
                <div className="text-xs font-mono space-y-1">
                  {[
                    `https://admin.${config.brand?.platform_domain || 'your-domain.com'}/auth/callback`,
                    `https://grafana.${config.brand?.platform_domain || 'your-domain.com'}/login/generic_oauth`,
                    `https://headlamp.${config.brand?.platform_domain || 'your-domain.com'}/oidc-callback`,
                    `https://temporal.${config.brand?.platform_domain || 'your-domain.com'}/auth/sso/callback`,
                    `https://prometheus.${config.brand?.platform_domain || 'your-domain.com'}/oauth2/callback`,
                  ].map((uri) => (
                    <div key={uri} className="flex items-center gap-2">
                      <span className="text-muted-foreground select-all break-all">{uri}</span>
                    </div>
                  ))}
                </div>
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
