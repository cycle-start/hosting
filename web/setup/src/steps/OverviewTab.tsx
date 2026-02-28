import { ExternalLink } from 'lucide-react'
import type { Config } from '@/lib/types'

const SERVICES = [
  { category: 'Management', items: [
    { name: 'Control Panel', sub: 'home' },
    { name: 'Admin Dashboard', sub: 'admin' },
    { name: 'Database Admin', sub: 'dbadmin' },
    { name: 'API', sub: 'api' },
  ]},
  { category: 'Monitoring', items: [
    { name: 'Grafana', sub: 'grafana' },
    { name: 'Prometheus', sub: 'prometheus' },
    { name: 'Headlamp', sub: 'headlamp' },
  ]},
  { category: 'Infrastructure', items: [
    { name: 'Temporal', sub: 'temporal' },
    { name: 'MCP Server', sub: 'mcp' },
  ]},
]

export function OverviewTab({ config }: { config: Config }) {
  const baseDomain = config.brand.platform_domain

  if (!baseDomain) {
    return (
      <div className="p-8 max-w-3xl">
        <h2 className="text-xl font-semibold mb-2">Platform Services</h2>
        <p className="text-sm text-muted-foreground">
          Set a platform domain in the Brand step to see service links.
        </p>
      </div>
    )
  }

  return (
    <div className="p-8 max-w-3xl">
      <h2 className="text-xl font-semibold mb-6">Platform Services</h2>
      <div className="rounded-lg border bg-card overflow-hidden">
        {SERVICES.map((group, gi) => (
          <div key={group.category}>
            <div className="px-4 py-2 bg-muted/50 text-xs font-medium text-muted-foreground uppercase tracking-wider border-b">
              {group.category}
            </div>
            {group.items.map((svc, si) => {
              const hostname = `${svc.sub}.${baseDomain}`
              const url = `https://${hostname}`
              return (
                <div
                  key={svc.sub}
                  className="flex items-center justify-between px-4 py-3 border-b last:border-b-0 hover:bg-accent/50 transition-colors"
                >
                  <span className="text-sm font-medium">{svc.name}</span>
                  <a
                    href={url}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="flex items-center gap-2 text-sm text-muted-foreground hover:text-foreground transition-colors font-mono"
                  >
                    {hostname}
                    <ExternalLink className="h-3.5 w-3.5 shrink-0" />
                  </a>
                </div>
              )
            })}
            {gi < SERVICES.length - 1 && <div className="border-b" />}
          </div>
        ))}
      </div>
    </div>
  )
}
