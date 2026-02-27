import { Label } from '@/components/ui/label'
import { Input } from '@/components/ui/input'
import { FieldError } from '@/components/field-error'
import { fieldError } from '@/lib/validation'
import type { Config, ValidationError } from '@/lib/types'
import { cn } from '@/lib/utils'
import { Database, Server } from 'lucide-react'

interface Props {
  config: Config
  onChange: (config: Config) => void
  errors: ValidationError[]
}

export function ControlPlaneStep({ config, onChange, errors }: Props) {
  const db = config.control_plane.database

  const updateDB = (updates: Partial<typeof db>) => {
    onChange({
      ...config,
      control_plane: {
        ...config.control_plane,
        database: { ...db, ...updates },
      },
    })
  }

  const fe = (field: string) => fieldError(errors, field)

  return (
    <div className="space-y-6">
      <div>
        <h2 className="text-xl font-semibold">Control Plane Database</h2>
        <p className="text-muted-foreground mt-1">
          The control plane needs a PostgreSQL database for platform state
          (regions, clusters, tenants, etc.).
        </p>
      </div>

      <div className="grid gap-4 max-w-lg">
        <div className="grid grid-cols-2 gap-3">
          {(['builtin', 'external'] as const).map((mode) => {
            const selected = db.mode === mode
            const Icon = mode === 'builtin' ? Server : Database
            return (
              <button
                key={mode}
                onClick={() => updateDB({ mode })}
                className={cn(
                  'flex items-center gap-3 rounded-lg border p-4 text-left transition-colors hover:bg-accent/50',
                  selected && 'border-primary bg-accent/50 ring-1 ring-primary'
                )}
              >
                <Icon className="h-5 w-5 shrink-0" />
                <div>
                  <div className="font-medium text-sm">
                    {mode === 'builtin' ? 'Built-in' : 'External'}
                  </div>
                  <div className="text-xs text-muted-foreground">
                    {mode === 'builtin'
                      ? 'We install PostgreSQL'
                      : 'Use your own instance'}
                  </div>
                </div>
              </button>
            )
          })}
        </div>

        {db.mode === 'external' && (
          <div className="space-y-4 border-t pt-4">
            <div className="grid grid-cols-2 gap-3">
              <div className="space-y-2">
                <Label htmlFor="db_host">Host</Label>
                <Input
                  id="db_host"
                  placeholder="db.example.com"
                  value={db.host}
                  onChange={(e) => updateDB({ host: e.target.value })}
                  className={fe('control_plane.database.host') ? 'border-destructive' : ''}
                />
                <FieldError error={fe('control_plane.database.host')} />
              </div>
              <div className="space-y-2">
                <Label htmlFor="db_port">Port</Label>
                <Input
                  id="db_port"
                  type="number"
                  placeholder="5432"
                  value={db.port || ''}
                  onChange={(e) => updateDB({ port: parseInt(e.target.value) || 0 })}
                  className={fe('control_plane.database.port') ? 'border-destructive' : ''}
                />
                <FieldError error={fe('control_plane.database.port')} />
              </div>
            </div>
            <div className="space-y-2">
              <Label htmlFor="db_name">Database Name</Label>
              <Input
                id="db_name"
                placeholder="hosting"
                value={db.name}
                onChange={(e) => updateDB({ name: e.target.value })}
                className={fe('control_plane.database.name') ? 'border-destructive' : ''}
              />
              <FieldError error={fe('control_plane.database.name')} />
            </div>
            <div className="grid grid-cols-2 gap-3">
              <div className="space-y-2">
                <Label htmlFor="db_user">User</Label>
                <Input
                  id="db_user"
                  placeholder="hosting"
                  value={db.user}
                  onChange={(e) => updateDB({ user: e.target.value })}
                  className={fe('control_plane.database.user') ? 'border-destructive' : ''}
                />
                <FieldError error={fe('control_plane.database.user')} />
              </div>
              <div className="space-y-2">
                <Label htmlFor="db_password">Password</Label>
                <Input
                  id="db_password"
                  type="password"
                  value={db.password}
                  onChange={(e) => updateDB({ password: e.target.value })}
                />
              </div>
            </div>
          </div>
        )}
      </div>
    </div>
  )
}
