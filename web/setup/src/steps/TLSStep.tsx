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

  const updateTLS = (updates: Partial<typeof tls>) => {
    onChange({ ...config, tls: { ...tls, ...updates } })
  }

  const fe = (field: string) => fieldError(errors, field)

  return (
    <div className="space-y-6">
      <div>
        <h2 className="text-xl font-semibold">TLS Certificates</h2>
        <p className="text-muted-foreground mt-1">
          How should TLS certificates be managed for the platform and customer sites?
        </p>
      </div>

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
  )
}
