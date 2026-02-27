import { Server, Network } from 'lucide-react'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import type { Config, DeployMode } from '@/lib/types'
import { cn } from '@/lib/utils'

interface Props {
  config: Config
  onChange: (config: Config) => void
  outputDir: string
}

const modes: { id: DeployMode; label: string; description: string; icon: typeof Server }[] = [
  {
    id: 'single',
    label: 'All-in-One',
    description: 'Run all services on a single host. Great for evaluation and small deployments.',
    icon: Server,
  },
  {
    id: 'multi',
    label: 'Custom',
    description: 'Define hosts and assign roles — one machine or many.',
    icon: Network,
  },
]

export function DeployModeStep({ config, onChange, outputDir }: Props) {
  return (
    <div className="space-y-6">
      <div>
        <h2 className="text-xl font-semibold">Deployment Mode</h2>
        <p className="text-muted-foreground mt-1">
          How would you like to deploy the hosting platform?
        </p>
      </div>

      <div className="grid gap-4">
        {modes.map((mode) => {
          const Icon = mode.icon
          const selected = config.deploy_mode === mode.id
          return (
            <button
              key={mode.id}
              onClick={() => onChange({ ...config, deploy_mode: mode.id })}
              className={cn(
                'flex items-start gap-4 rounded-lg border p-4 text-left transition-colors hover:bg-accent/50',
                selected && 'border-primary bg-accent/50 ring-1 ring-primary'
              )}
            >
              <div className={cn(
                'rounded-md p-2',
                selected ? 'bg-primary text-primary-foreground' : 'bg-muted'
              )}>
                <Icon className="h-6 w-6" />
              </div>
              <div>
                <div className="font-medium">{mode.label}</div>
                <div className="text-sm text-muted-foreground mt-0.5">
                  {mode.description}
                </div>
              </div>
            </button>
          )
        })}
      </div>

      {config.deploy_mode === 'single' && (
        <div className="max-w-lg space-y-2">
          <Label htmlFor="target_host">Target Host IP</Label>
          <Input
            id="target_host"
            placeholder="127.0.0.1"
            value={config.target_host}
            onChange={(e) => onChange({ ...config, target_host: e.target.value })}
          />
          <p className="text-xs text-muted-foreground">
            The IP address of the machine to deploy to. Use 127.0.0.1 to deploy locally,
            or enter a remote IP to target another host via SSH.
          </p>
        </div>
      )}

      <div className="max-w-lg space-y-2">
        <Label htmlFor="ssh_user">SSH User</Label>
        <Input
          id="ssh_user"
          placeholder="ubuntu"
          value={config.ssh_user}
          onChange={(e) => onChange({ ...config, ssh_user: e.target.value })}
        />
        <p className="text-xs text-muted-foreground">
          The SSH user Ansible will use to connect to machines. Must have passwordless sudo.
        </p>
      </div>

      {outputDir && (
        <div className="rounded-lg border bg-muted/30 px-4 py-3 text-sm text-muted-foreground">
          Output directory: <code className="font-mono text-xs bg-muted px-1 py-0.5 rounded">{outputDir}/</code>
          <br />
          <span className="text-xs">To change this, restart with <code className="font-mono bg-muted px-1 py-0.5 rounded">setup -output &lt;path&gt;</code></span>
        </div>
      )}
    </div>
  )
}
