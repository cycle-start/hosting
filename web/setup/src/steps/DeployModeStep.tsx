import { Server, Network } from 'lucide-react'
import type { Config, DeployMode } from '@/lib/types'
import { cn } from '@/lib/utils'

interface Props {
  config: Config
  onChange: (config: Config) => void
}

const modes: { id: DeployMode; label: string; description: string; icon: typeof Server }[] = [
  {
    id: 'single',
    label: 'Single Machine',
    description: 'Everything runs on this host. Great for evaluation and small deployments.',
    icon: Server,
  },
  {
    id: 'multi',
    label: 'Multiple Machines',
    description: 'Distribute roles across several servers. Assign roles per machine.',
    icon: Network,
  },
]

export function DeployModeStep({ config, onChange }: Props) {
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
    </div>
  )
}
