import { Label } from '@/components/ui/label'
import { Input } from '@/components/ui/input'
import { FieldError } from '@/components/field-error'
import { fieldError } from '@/lib/validation'
import type { Config, ValidationError } from '@/lib/types'

const AVAILABLE_PHP = ['8.0', '8.1', '8.2', '8.3', '8.4', '8.5']

interface Props {
  config: Config
  onChange: (config: Config) => void
  errors: ValidationError[]
}

export function RegionStep({ config, onChange, errors }: Props) {
  const togglePhp = (version: string) => {
    const current = config.php_versions || []
    if (current.includes(version)) {
      onChange({ ...config, php_versions: current.filter(v => v !== version) })
    } else {
      onChange({ ...config, php_versions: [...current, version].sort() })
    }
  }

  return (
    <div className="space-y-6">
      <div>
        <h2 className="text-xl font-semibold">Region & Cluster</h2>
        <p className="text-muted-foreground mt-1">
          Name your region and cluster. A region contains one or more clusters.
        </p>
      </div>

      <div className="grid gap-4 max-w-md">
        <div className="space-y-2">
          <Label htmlFor="region_name">Region Name</Label>
          <Input
            id="region_name"
            placeholder="e.g. eu-west-1, us-east, default"
            value={config.region_name}
            onChange={(e) => onChange({ ...config, region_name: e.target.value })}
            className={fieldError(errors, 'region_name') ? 'border-destructive' : ''}
          />
          <FieldError error={fieldError(errors, 'region_name')} />
          {!fieldError(errors, 'region_name') && (
            <p className="text-xs text-muted-foreground">
              A logical grouping for your infrastructure (e.g. a datacenter location).
            </p>
          )}
        </div>

        <div className="space-y-2">
          <Label htmlFor="cluster_name">Cluster Name</Label>
          <Input
            id="cluster_name"
            placeholder="e.g. cluster-1, production"
            value={config.cluster_name}
            onChange={(e) => onChange({ ...config, cluster_name: e.target.value })}
            className={fieldError(errors, 'cluster_name') ? 'border-destructive' : ''}
          />
          <FieldError error={fieldError(errors, 'cluster_name')} />
          {!fieldError(errors, 'cluster_name') && (
            <p className="text-xs text-muted-foreground">
              The name for this cluster within the region.
            </p>
          )}
        </div>

        <div className="space-y-2">
          <Label>PHP Versions</Label>
          <p className="text-xs text-muted-foreground">
            Select which PHP versions to pre-install on web nodes.
          </p>
          <div className="flex flex-wrap gap-2">
            {AVAILABLE_PHP.map((v) => {
              const selected = (config.php_versions || []).includes(v)
              return (
                <button
                  key={v}
                  type="button"
                  onClick={() => togglePhp(v)}
                  className={`inline-flex items-center gap-1.5 rounded-full px-3 py-1 text-sm font-medium border transition-colors ${
                    selected
                      ? 'bg-primary text-primary-foreground border-primary'
                      : 'bg-card text-muted-foreground border-border hover:bg-accent/50'
                  }`}
                >
                  {selected && (
                    <svg className="h-3.5 w-3.5" viewBox="0 0 16 16" fill="currentColor">
                      <path d="M13.78 4.22a.75.75 0 0 1 0 1.06l-7.25 7.25a.75.75 0 0 1-1.06 0L2.22 9.28a.75.75 0 0 1 1.06-1.06L6 10.94l6.72-6.72a.75.75 0 0 1 1.06 0Z" />
                    </svg>
                  )}
                  PHP {v}
                </button>
              )
            })}
          </div>
          <FieldError error={fieldError(errors, 'php_versions')} />
        </div>
      </div>
    </div>
  )
}
