import { Label } from '@/components/ui/label'
import { Input } from '@/components/ui/input'
import { FieldError } from '@/components/field-error'
import { fieldError } from '@/lib/validation'
import type { Config, ValidationError } from '@/lib/types'

interface Props {
  config: Config
  onChange: (config: Config) => void
  errors: ValidationError[]
}

export function RegionStep({ config, onChange, errors }: Props) {
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
      </div>
    </div>
  )
}
