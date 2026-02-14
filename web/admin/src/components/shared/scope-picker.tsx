import { Checkbox } from '@/components/ui/checkbox'
import { Label } from '@/components/ui/label'

const SCOPE_GROUPS = [
  {
    label: 'Infrastructure',
    resources: ['brands', 'regions', 'clusters', 'shards', 'nodes'],
  },
  {
    label: 'Hosting',
    resources: ['tenants', 'webroots', 'fqdns', 'certificates', 'sftp_keys', 'backups'],
  },
  {
    label: 'Databases',
    resources: ['databases', 'database_users'],
  },
  {
    label: 'DNS',
    resources: ['zones', 'zone_records'],
  },
  {
    label: 'Email',
    resources: ['email'],
  },
  {
    label: 'Storage',
    resources: ['s3', 'valkey'],
  },
  {
    label: 'Platform',
    resources: ['platform', 'api_keys', 'audit_logs'],
  },
] as const

const ACTIONS = ['read', 'write', 'delete'] as const

function formatResource(r: string) {
  return r.replace(/_/g, ' ').replace(/\b\w/g, c => c.toUpperCase())
}

interface ScopePickerProps {
  value: string[]
  onChange: (scopes: string[]) => void
}

export function ScopePicker({ value, onChange }: ScopePickerProps) {
  const isWildcard = value.includes('*:*')

  const hasScope = (resource: string, action: string) =>
    isWildcard || value.includes(`${resource}:${action}`)

  const toggleScope = (resource: string, action: string) => {
    const scope = `${resource}:${action}`
    if (isWildcard) {
      // Switching from wildcard: enable everything except this one
      const all: string[] = []
      for (const group of SCOPE_GROUPS) {
        for (const r of group.resources) {
          for (const a of ACTIONS) {
            const s = `${r}:${a}`
            if (s !== scope) all.push(s)
          }
        }
      }
      onChange(all)
    } else if (value.includes(scope)) {
      onChange(value.filter(s => s !== scope))
    } else {
      onChange([...value, scope])
    }
  }

  const toggleAll = () => {
    if (isWildcard) {
      onChange([])
    } else {
      onChange(['*:*'])
    }
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center space-x-2">
        <Checkbox
          id="scope-all"
          checked={isWildcard}
          onCheckedChange={toggleAll}
        />
        <Label htmlFor="scope-all" className="font-medium">
          Full access (all scopes)
        </Label>
      </div>

      {!isWildcard && (
        <div className="space-y-4 pl-2">
          {SCOPE_GROUPS.map(group => (
            <div key={group.label}>
              <p className="text-sm font-medium text-muted-foreground mb-2">{group.label}</p>
              <div className="space-y-1.5">
                {group.resources.map(resource => (
                  <div key={resource} className="flex items-center gap-4">
                    <span className="text-sm w-32 truncate">{formatResource(resource)}</span>
                    {ACTIONS.map(action => (
                      <div key={action} className="flex items-center space-x-1">
                        <Checkbox
                          id={`scope-${resource}-${action}`}
                          checked={hasScope(resource, action)}
                          onCheckedChange={() => toggleScope(resource, action)}
                        />
                        <Label htmlFor={`scope-${resource}-${action}`} className="text-xs">
                          {action}
                        </Label>
                      </div>
                    ))}
                  </div>
                ))}
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
