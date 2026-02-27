import { Plus, Trash2, AlertCircle, Server, Check } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { FieldError } from '@/components/field-error'
import { fieldError } from '@/lib/validation'
import type { Config, NodeConfig, NodeRole, RoleInfo, ValidationError } from '@/lib/types'
import { cn } from '@/lib/utils'

interface Props {
  config: Config
  onChange: (config: Config) => void
  roles: RoleInfo[]
  errors: ValidationError[]
}

export function NodesStep({ config, onChange, roles, errors }: Props) {
  const nodes = config.nodes || []

  const updateNodes = (newNodes: NodeConfig[]) => {
    onChange({ ...config, nodes: newNodes })
  }

  const addNode = () => {
    updateNodes([...nodes, { hostname: '', ip: '', roles: [] }])
  }

  const removeNode = (index: number) => {
    updateNodes(nodes.filter((_, i) => i !== index))
  }

  const updateNode = (index: number, updates: Partial<NodeConfig>) => {
    updateNodes(nodes.map((n, i) => (i === index ? { ...n, ...updates } : n)))
  }

  const toggleRoleOnMachine = (machineIndex: number, role: NodeRole) => {
    const node = nodes[machineIndex]
    const currentRoles = node.roles || []
    if (currentRoles.includes(role)) {
      updateNode(machineIndex, { roles: currentRoles.filter((r) => r !== role) })
    } else {
      updateNode(machineIndex, { roles: [...currentRoles, role] })
    }
  }

  const isRoleOnMachine = (machineIndex: number, role: NodeRole) =>
    (nodes[machineIndex]?.roles || []).includes(role)

  const isRoleAssigned = (role: NodeRole) =>
    nodes.some((n) => (n.roles || []).includes(role))

  const topErrors = errors.filter((e) => e.field === 'nodes')

  const machineName = (node: NodeConfig, i: number) =>
    node.hostname || node.ip || `Machine ${i + 1}`

  return (
    <div className="space-y-8">
      <div>
        <h2 className="text-xl font-semibold">Machine Inventory</h2>
        <p className="text-muted-foreground mt-1">
          Define your machines, then assign roles below. Each role needs at least one machine.
        </p>
      </div>

      {topErrors.length > 0 && (
        <div className="rounded-lg border border-destructive/50 bg-destructive/10 p-3 space-y-1">
          {topErrors.map((err, i) => (
            <p key={i} className="text-sm text-destructive flex items-center gap-2">
              <AlertCircle className="h-3.5 w-3.5 shrink-0" />
              {err.message}
            </p>
          ))}
        </div>
      )}

      {/* Section 1: Machines */}
      <div className="space-y-3">
        <h3 className="text-sm font-medium">Machines</h3>
        <div className="space-y-2">
          {nodes.map((node, i) => {
            const hostnameErr = fieldError(errors, `nodes[${i}].hostname`)
            const ipErr = fieldError(errors, `nodes[${i}].ip`)

            return (
              <div key={i} className="flex items-center gap-3">
                <div className="flex items-center justify-center h-8 w-8 rounded-md bg-muted text-muted-foreground shrink-0">
                  <Server className="h-4 w-4" />
                </div>
                <div className="flex-1 grid grid-cols-2 gap-2">
                  <div>
                    <Input
                      placeholder="Hostname"
                      value={node.hostname}
                      onChange={(e) => updateNode(i, { hostname: e.target.value })}
                      className={cn('h-8 text-sm', hostnameErr && 'border-destructive')}
                    />
                    <FieldError error={hostnameErr} />
                  </div>
                  <div>
                    <Input
                      placeholder="IP address"
                      value={node.ip}
                      onChange={(e) => updateNode(i, { ip: e.target.value })}
                      className={cn('h-8 text-sm', ipErr && 'border-destructive')}
                    />
                    <FieldError error={ipErr} />
                  </div>
                </div>
                <Button
                  variant="ghost"
                  size="icon"
                  className="h-8 w-8 text-muted-foreground hover:text-destructive shrink-0"
                  onClick={() => removeNode(i)}
                >
                  <Trash2 className="h-3.5 w-3.5" />
                </Button>
              </div>
            )
          })}
        </div>
        <Button variant="outline" size="sm" onClick={addNode}>
          <Plus className="h-3.5 w-3.5 mr-1.5" />
          Add Machine
        </Button>
      </div>

      {/* Section 2: Role Assignment Grid */}
      {nodes.length > 0 && (
        <div className="space-y-3">
          <h3 className="text-sm font-medium">Role Assignment</h3>
          <div className="rounded-lg border bg-card overflow-hidden">
            {/* Header row */}
            <div className="grid border-b bg-muted/50 px-4 py-2 text-xs font-medium text-muted-foreground"
              style={{ gridTemplateColumns: `1fr repeat(${nodes.length}, 80px)` }}
            >
              <div>Role</div>
              {nodes.map((node, i) => (
                <div key={i} className="text-center truncate" title={machineName(node, i)}>
                  {machineName(node, i)}
                </div>
              ))}
            </div>
            {/* Role rows */}
            {roles.map((role) => {
              const assigned = isRoleAssigned(role.id)
              return (
                <div
                  key={role.id}
                  className={cn(
                    'grid items-center px-4 py-2 border-b last:border-b-0',
                    !assigned && 'bg-destructive/5'
                  )}
                  style={{ gridTemplateColumns: `1fr repeat(${nodes.length}, 80px)` }}
                >
                  <div className="min-w-0 pr-2">
                    <div className={cn('text-sm', !assigned && 'text-destructive')}>
                      {role.label}
                    </div>
                    <div className="text-xs text-muted-foreground truncate">
                      {role.description}
                    </div>
                  </div>
                  {nodes.map((_, mi) => {
                    const active = isRoleOnMachine(mi, role.id)
                    return (
                      <div key={mi} className="flex justify-center">
                        <button
                          onClick={() => toggleRoleOnMachine(mi, role.id)}
                          className={cn(
                            'h-6 w-6 rounded border flex items-center justify-center transition-colors',
                            active
                              ? 'bg-primary border-primary text-primary-foreground'
                              : 'border-input hover:border-foreground/30'
                          )}
                        >
                          {active && <Check className="h-3.5 w-3.5" />}
                        </button>
                      </div>
                    )
                  })}
                </div>
              )
            })}
          </div>
        </div>
      )}
    </div>
  )
}
