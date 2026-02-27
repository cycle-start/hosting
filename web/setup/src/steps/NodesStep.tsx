import { useState } from 'react'
import { Plus, Trash2, X } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import type { Config, NodeConfig, NodeRole, RoleInfo } from '@/lib/types'
import { cn } from '@/lib/utils'

interface Props {
  config: Config
  onChange: (config: Config) => void
  roles: RoleInfo[]
}

const ROLE_COLORS: Record<NodeRole, string> = {
  controlplane: 'bg-purple-500/20 text-purple-400 border-purple-500/30',
  web: 'bg-blue-500/20 text-blue-400 border-blue-500/30',
  database: 'bg-amber-500/20 text-amber-400 border-amber-500/30',
  dns: 'bg-green-500/20 text-green-400 border-green-500/30',
  valkey: 'bg-red-500/20 text-red-400 border-red-500/30',
  email: 'bg-cyan-500/20 text-cyan-400 border-cyan-500/30',
  storage: 'bg-orange-500/20 text-orange-400 border-orange-500/30',
  lb: 'bg-indigo-500/20 text-indigo-400 border-indigo-500/30',
  gateway: 'bg-pink-500/20 text-pink-400 border-pink-500/30',
  dbadmin: 'bg-teal-500/20 text-teal-400 border-teal-500/30',
}

export function NodesStep({ config, onChange, roles }: Props) {
  const [addingRole, setAddingRole] = useState<number | null>(null)

  const nodes = config.nodes

  const updateNodes = (nodes: NodeConfig[]) => {
    onChange({ ...config, nodes })
  }

  const addNode = () => {
    updateNodes([
      ...nodes,
      { hostname: '', ip: '', roles: [] },
    ])
  }

  const removeNode = (index: number) => {
    updateNodes(nodes.filter((_, i) => i !== index))
  }

  const updateNode = (index: number, updates: Partial<NodeConfig>) => {
    updateNodes(nodes.map((n, i) => (i === index ? { ...n, ...updates } : n)))
  }

  const addRole = (nodeIndex: number, role: NodeRole) => {
    const node = nodes[nodeIndex]
    if (!node.roles.includes(role)) {
      updateNode(nodeIndex, { roles: [...node.roles, role] })
    }
    setAddingRole(null)
  }

  const removeRole = (nodeIndex: number, role: NodeRole) => {
    const node = nodes[nodeIndex]
    updateNode(nodeIndex, { roles: node.roles.filter((r) => r !== role) })
  }

  return (
    <div className="space-y-6">
      <div>
        <h2 className="text-xl font-semibold">Machine Inventory</h2>
        <p className="text-muted-foreground mt-1">
          Add your machines and assign roles to each one. A machine can have
          multiple roles.
        </p>
      </div>

      <div className="space-y-4">
        {nodes.map((node, i) => (
          <div
            key={i}
            className="rounded-lg border bg-card p-4 space-y-3"
          >
            <div className="flex items-start gap-3">
              <div className="flex-1 grid grid-cols-2 gap-3">
                <div className="space-y-1.5">
                  <Label className="text-xs">Hostname</Label>
                  <Input
                    placeholder="e.g. web-1"
                    value={node.hostname}
                    onChange={(e) =>
                      updateNode(i, { hostname: e.target.value })
                    }
                  />
                </div>
                <div className="space-y-1.5">
                  <Label className="text-xs">IP Address</Label>
                  <Input
                    placeholder="e.g. 10.0.0.10"
                    value={node.ip}
                    onChange={(e) => updateNode(i, { ip: e.target.value })}
                  />
                </div>
              </div>
              <Button
                variant="ghost"
                size="icon"
                className="mt-5 text-muted-foreground hover:text-destructive"
                onClick={() => removeNode(i)}
              >
                <Trash2 className="h-4 w-4" />
              </Button>
            </div>

            <div>
              <Label className="text-xs">Roles</Label>
              <div className="flex flex-wrap gap-2 mt-1.5">
                {node.roles.map((role) => {
                  const info = roles.find((r) => r.id === role)
                  return (
                    <span
                      key={role}
                      className={cn(
                        'inline-flex items-center gap-1 rounded-md border px-2 py-0.5 text-xs font-medium',
                        ROLE_COLORS[role]
                      )}
                    >
                      {info?.label || role}
                      <button
                        onClick={() => removeRole(i, role)}
                        className="hover:opacity-70"
                      >
                        <X className="h-3 w-3" />
                      </button>
                    </span>
                  )
                })}

                {addingRole === i ? (
                  <div className="flex flex-wrap gap-1">
                    {roles
                      .filter((r) => !node.roles.includes(r.id))
                      .map((role) => (
                        <button
                          key={role.id}
                          onClick={() => addRole(i, role.id)}
                          className={cn(
                            'rounded-md border px-2 py-0.5 text-xs font-medium transition-colors hover:opacity-80',
                            ROLE_COLORS[role.id]
                          )}
                        >
                          + {role.label}
                        </button>
                      ))}
                    <button
                      onClick={() => setAddingRole(null)}
                      className="rounded-md px-2 py-0.5 text-xs text-muted-foreground hover:text-foreground"
                    >
                      Cancel
                    </button>
                  </div>
                ) : (
                  <button
                    onClick={() => setAddingRole(i)}
                    className="inline-flex items-center gap-1 rounded-md border border-dashed px-2 py-0.5 text-xs text-muted-foreground hover:text-foreground hover:border-foreground/30 transition-colors"
                  >
                    <Plus className="h-3 w-3" />
                    Add role
                  </button>
                )}
              </div>
            </div>
          </div>
        ))}

        <Button variant="outline" onClick={addNode} className="w-full">
          <Plus className="h-4 w-4 mr-2" />
          Add Machine
        </Button>
      </div>
    </div>
  )
}
