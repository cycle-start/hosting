import { Label } from '@/components/ui/label'
import { Input } from '@/components/ui/input'
import { FieldError } from '@/components/field-error'
import { fieldError } from '@/lib/validation'
import type { Config, ValidationError } from '@/lib/types'
import { cn } from '@/lib/utils'
import { ShieldCheck, FileKey } from 'lucide-react'

interface Props {
  config: Config
  onChange: (config: Config) => void
  errors: ValidationError[]
}

export function TLSStep({ config, onChange, errors }: Props) {
  const tls = config.tls
  const sso = config.sso

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
        <h2 className="text-xl font-semibold">TLS & Security</h2>
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
          <label className="flex items-center gap-3 cursor-pointer">
            <input
              type="checkbox"
              checked={sso.enabled}
              onChange={(e) => updateSSO({ enabled: e.target.checked })}
              className="h-4 w-4 rounded border-input"
            />
            <div>
              <div className="font-medium text-sm">Enable Azure AD SSO</div>
              <div className="text-xs text-muted-foreground">
                Secure Grafana, Headlamp, Temporal UI, Prometheus, and Admin UI with Azure AD login.
              </div>
            </div>
          </label>

          {sso.enabled && (
            <div className="space-y-4 border-l-2 border-primary/30 pl-4">
              <div className="space-y-2">
                <Label htmlFor="sso_tenant_id">Azure AD Tenant ID</Label>
                <Input
                  id="sso_tenant_id"
                  placeholder="xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
                  value={sso.tenant_id}
                  onChange={(e) => updateSSO({ tenant_id: e.target.value })}
                  className={fe('sso.tenant_id') ? 'border-destructive' : ''}
                />
                <FieldError error={fe('sso.tenant_id')} />
              </div>

              <div className="space-y-2">
                <Label htmlFor="sso_client_id">Application (Client) ID</Label>
                <Input
                  id="sso_client_id"
                  placeholder="xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
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

              <p className="text-xs text-muted-foreground">
                Register an app in Azure AD (Entra ID) and add these redirect URIs:
              </p>
              <ul className="text-xs font-mono text-muted-foreground space-y-0.5 list-disc list-inside">
                <li>https://admin.{'{'}domain{'}'}/auth/callback</li>
                <li>https://grafana.{'{'}domain{'}'}/login/generic_oauth</li>
                <li>https://headlamp.{'{'}domain{'}'}/oidc-callback</li>
                <li>https://temporal.{'{'}domain{'}'}/auth/sso/callback</li>
                <li>https://prometheus.{'{'}domain{'}'}/oauth2/callback</li>
              </ul>
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
