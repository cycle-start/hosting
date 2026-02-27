import type { Config, RoleInfo, ValidationError } from '@/lib/types'
import { AlertTriangle, CheckCircle2 } from 'lucide-react'

interface Props {
  config: Config
  roles: RoleInfo[]
  errors: ValidationError[]
}

export function ReviewStep({ config, roles, errors }: Props) {
  const roleLabel = (id: string) => roles.find((r) => r.id === id)?.label || id

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
            Validation Errors
          </div>
          <ul className="text-sm space-y-1">
            {errors.map((err, i) => (
              <li key={i} className="text-muted-foreground">
                <span className="font-mono text-xs">{err.field}</span>: {err.message}
              </li>
            ))}
          </ul>
        </div>
      )}

      <div className="grid gap-4 max-w-lg">
        <Section title="Deployment Mode">
          <Value
            label="Mode"
            value={
              config.deploy_mode === 'single'
                ? 'Single Machine'
                : config.deploy_mode === 'multi'
                  ? 'Multiple Machines'
                  : 'Kubernetes'
            }
          />
        </Section>

        <Section title="Region & Cluster">
          <Value label="Region" value={config.region_name} />
          <Value label="Cluster" value={config.cluster_name} />
        </Section>

        <Section title="Brand">
          <Value label="Name" value={config.brand.name} />
          <Value label="Platform Domain" value={config.brand.platform_domain} />
          <Value label="Customer Domain" value={config.brand.customer_domain} />
          <Value label="Primary NS" value={`${config.brand.primary_ns} (${config.brand.primary_ns_ip || 'no IP'})`} />
          <Value label="Secondary NS" value={`${config.brand.secondary_ns} (${config.brand.secondary_ns_ip || 'no IP'})`} />
          <Value label="Mail Hostname" value={config.brand.mail_hostname} />
        </Section>

        <Section title="Control Plane">
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
          <Section title="Machines">
            {config.nodes.map((n, i) => (
              <Value
                key={i}
                label={n.hostname || `Node ${i + 1}`}
                value={`${n.ip} — ${n.roles.map(roleLabel).join(', ') || 'no roles'}`}
              />
            ))}
            {config.nodes.length === 0 && (
              <p className="text-sm text-muted-foreground italic">No machines configured</p>
            )}
          </Section>
        )}

        <Section title="Storage">
          <Value
            label="Mode"
            value={config.storage.mode === 'builtin' ? 'Built-in Ceph' : 'External S3'}
          />
          {config.storage.mode === 'external' && (
            <Value label="Endpoint" value={config.storage.s3_endpoint} />
          )}
        </Section>

        <Section title="TLS">
          <Value
            label="Mode"
            value={
              config.tls.mode === 'letsencrypt'
                ? `Let's Encrypt (${config.tls.email})`
                : 'Manual certificates'
            }
          />
        </Section>

        {errors.length === 0 && (
          <div className="flex items-center gap-2 text-sm text-green-400">
            <CheckCircle2 className="h-4 w-4" />
            Configuration is valid. Click "Generate" to create deployment files.
          </div>
        )}
      </div>
    </div>
  )
}

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div className="rounded-lg border bg-card p-4 space-y-2">
      <h3 className="text-sm font-medium">{title}</h3>
      <div className="space-y-1">{children}</div>
    </div>
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
