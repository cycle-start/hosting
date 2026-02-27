import type { Config, RoleInfo, ValidationError, StepID } from '@/lib/types'
import { errorsForStep } from '@/lib/validation'
import { AlertTriangle, CheckCircle2, ChevronRight } from 'lucide-react'

interface Props {
  config: Config
  roles: RoleInfo[]
  errors: ValidationError[]
  onGoToStep: (step: StepID) => void
}

export function ReviewStep({ config, roles, errors, onGoToStep }: Props) {
  const roleLabel = (id: string) => roles.find((r) => r.id === id)?.label || id

  const sectionErrors = (step: StepID) => errorsForStep(errors, step)

  return (
    <div className="space-y-6">
      <div>
        <h2 className="text-xl font-semibold">Review & Generate</h2>
        <p className="text-muted-foreground mt-1">
          Review your configuration before generating deployment files.
        </p>
      </div>

      {errors.length > 0 && (
        <div className="rounded-lg border border-destructive/50 bg-destructive/10 p-4 space-y-2">
          <div className="flex items-center gap-2 text-destructive font-medium text-sm">
            <AlertTriangle className="h-4 w-4" />
            {errors.length} validation {errors.length === 1 ? 'error' : 'errors'} found
          </div>
          <p className="text-sm text-muted-foreground">
            Fix the errors below before generating. Click a section to jump to it.
          </p>
        </div>
      )}

      <div className="grid gap-3 max-w-lg">
        <Section
          title="Deployment Mode"
          errors={sectionErrors('deploy_mode')}
          onEdit={() => onGoToStep('deploy_mode')}
        >
          <Value
            label="Mode"
            value={
              config.deploy_mode === 'single'
                ? 'This Machine'
                : 'Multiple Machines'
            }
          />
        </Section>

        <Section
          title="Region & Cluster"
          errors={sectionErrors('region')}
          onEdit={() => onGoToStep('region')}
        >
          <Value label="Region" value={config.region_name} />
          <Value label="Cluster" value={config.cluster_name} />
        </Section>

        <Section
          title="Brand"
          errors={sectionErrors('brand')}
          onEdit={() => onGoToStep('brand')}
        >
          <Value label="Name" value={config.brand.name} />
          <Value label="Platform Domain" value={config.brand.platform_domain} />
          <Value label="Customer Domain" value={config.brand.customer_domain} />
          <Value label="Primary NS" value={config.brand.primary_ns ? `${config.brand.primary_ns}${config.brand.primary_ns_ip ? ` (${config.brand.primary_ns_ip})` : ''}` : ''} />
          <Value label="Secondary NS" value={config.brand.secondary_ns ? `${config.brand.secondary_ns}${config.brand.secondary_ns_ip ? ` (${config.brand.secondary_ns_ip})` : ''}` : ''} />
          <Value label="Mail Hostname" value={config.brand.mail_hostname} />
          <Value label="Hostmaster" value={config.brand.hostmaster_email} />
          <Value label="Stalwart Token" value={config.email.stalwart_admin_token ? 'Configured' : '---'} />
        </Section>

        <Section
          title="Control Plane"
          errors={sectionErrors('control_plane')}
          onEdit={() => onGoToStep('control_plane')}
        >
          <Value
            label="Database"
            value={
              config.control_plane.database.mode === 'builtin'
                ? 'Built-in PostgreSQL'
                : `External: ${config.control_plane.database.host}:${config.control_plane.database.port}`
            }
          />
        </Section>

        {config.deploy_mode === 'multi' && (
          <Section
            title="Machines"
            errors={sectionErrors('nodes')}
            onEdit={() => onGoToStep('nodes')}
          >
            {(config.nodes || []).map((n, i) => (
              <Value
                key={i}
                label={n.hostname || `Node ${i + 1}`}
                value={`${n.ip || '—'} — ${(n.roles || []).map(roleLabel).join(', ') || 'no roles'}`}
              />
            ))}
            {(config.nodes || []).length === 0 && (
              <p className="text-sm text-muted-foreground italic">No machines configured</p>
            )}
          </Section>
        )}

        <Section
          title="TLS"
          errors={sectionErrors('tls')}
          onEdit={() => onGoToStep('tls')}
        >
          <Value
            label="Mode"
            value={
              config.tls.mode === 'letsencrypt'
                ? `Let's Encrypt (${config.tls.email || '—'})`
                : 'Manual certificates'
            }
          />
        </Section>

        <Section
          title="API Key"
          errors={sectionErrors('review')}
          onEdit={() => onGoToStep('review')}
        >
          <Value
            label="Key"
            value={config.api_key ? `${config.api_key.slice(0, 12)}...` : '---'}
          />
        </Section>

        {errors.length === 0 && (
          <div className="flex items-center gap-2 text-sm text-green-400 pt-2">
            <CheckCircle2 className="h-4 w-4" />
            Configuration is valid. Click "Generate Files" to create deployment files.
          </div>
        )}
      </div>
    </div>
  )
}

function Section({
  title,
  errors,
  onEdit,
  children,
}: {
  title: string
  errors: { field: string; message: string }[]
  onEdit: () => void
  children: React.ReactNode
}) {
  const hasErrors = errors.length > 0

  return (
    <button
      onClick={onEdit}
      className={`rounded-lg border bg-card p-4 space-y-2 text-left w-full transition-colors hover:bg-accent/30 group ${hasErrors ? 'border-destructive/50' : ''}`}
    >
      <div className="flex items-center justify-between">
        <h3 className="text-sm font-medium flex items-center gap-2">
          {hasErrors && <AlertTriangle className="h-3.5 w-3.5 text-destructive" />}
          {title}
        </h3>
        <ChevronRight className="h-4 w-4 text-muted-foreground opacity-0 group-hover:opacity-100 transition-opacity" />
      </div>
      <div className="space-y-1">{children}</div>
      {hasErrors && (
        <div className="space-y-1 border-t border-destructive/20 pt-2 mt-2">
          {errors.map((err, i) => (
            <p key={i} className="text-xs text-destructive">
              {err.message}
            </p>
          ))}
        </div>
      )}
    </button>
  )
}

function Value({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex justify-between text-sm">
      <span className="text-muted-foreground">{label}</span>
      <span className="font-mono text-xs">{value || '—'}</span>
    </div>
  )
}
