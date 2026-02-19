import { MapPin, Server, Users, Database, Globe, Boxes, Link2, HardDrive, AlertCircle, ShieldAlert, Clock } from 'lucide-react'
import { type ColumnDef } from '@tanstack/react-table'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { StatsCard } from '@/components/shared/stats-card'
import { StatusBadge } from '@/components/shared/status-badge'
import { DataTable } from '@/components/shared/data-table'
import { Skeleton } from '@/components/ui/skeleton'
import { useDashboardStats } from '@/lib/hooks'

const shardColumns: ColumnDef<{ shard_id: string; shard_name: string; role: string; count: number }>[] = [
  { accessorKey: 'shard_name', header: 'Shard' },
  { accessorKey: 'role', header: 'Role', cell: ({ row }) => <StatusBadge status={row.original.role} /> },
  { accessorKey: 'count', header: 'Tenants' },
]

const clusterColumns: ColumnDef<{ cluster_id: string; cluster_name: string; count: number }>[] = [
  { accessorKey: 'cluster_name', header: 'Cluster' },
  { accessorKey: 'count', header: 'Nodes' },
]

export function DashboardPage() {
  const { data: stats, isLoading } = useDashboardStats()

  if (isLoading || !stats) {
    return (
      <div className="space-y-6">
        <h1 className="text-2xl font-bold tracking-tight">Dashboard</h1>
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
          {Array.from({ length: 8 }).map((_, i) => (
            <Skeleton key={i} className="h-24" />
          ))}
        </div>
      </div>
    )
  }

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold tracking-tight">Dashboard</h1>

      {/* Health Overview */}
      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <StatsCard
          label="Open Incidents"
          value={stats.incidents_open}
          icon={AlertCircle}
          className={stats.incidents_open > 0 ? 'border-destructive/50' : ''}
        />
        <StatsCard
          label="Critical"
          value={stats.incidents_critical}
          icon={AlertCircle}
          className={stats.incidents_critical > 0 ? 'border-destructive/50' : ''}
        />
        <StatsCard
          label="Escalated"
          value={stats.incidents_escalated}
          icon={ShieldAlert}
          className={stats.incidents_escalated > 0 ? 'border-orange-500/50' : ''}
        />
        <StatsCard
          label="MTTR (30d)"
          value={stats.mttr_minutes != null ? `${Math.round(stats.mttr_minutes)}m` : 'N/A'}
          icon={Clock}
        />
      </div>

      {/* Infrastructure */}
      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <StatsCard label="Regions" value={stats.regions} icon={MapPin} />
        <StatsCard label="Clusters" value={stats.clusters} icon={Server} />
        <StatsCard label="Nodes" value={stats.nodes} icon={HardDrive} />
        <StatsCard label="Tenants" value={stats.tenants} icon={Users} />
        <StatsCard label="Databases" value={stats.databases} icon={Database} />
        <StatsCard label="Zones" value={stats.zones} icon={Globe} />
        <StatsCard label="Valkey" value={stats.valkey_instances} icon={Boxes} />
        <StatsCard label="FQDNs" value={stats.fqdns} icon={Link2} />
      </div>

      <div className="grid gap-6 lg:grid-cols-3">
        <Card>
          <CardHeader>
            <CardTitle className="text-base">Tenants by Status</CardTitle>
          </CardHeader>
          <CardContent>
            {stats.tenants_by_status?.length ? (
              <div className="space-y-3">
                {stats.tenants_by_status.map((s) => (
                  <div key={s.status} className="flex items-center justify-between">
                    <StatusBadge status={s.status} />
                    <span className="text-sm font-medium">{s.count}</span>
                  </div>
                ))}
              </div>
            ) : (
              <p className="text-sm text-muted-foreground">No tenants yet</p>
            )}
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle className="text-base">Incidents by Status</CardTitle>
          </CardHeader>
          <CardContent>
            {stats.incidents_by_status?.length ? (
              <div className="space-y-3">
                {stats.incidents_by_status.map((s) => (
                  <div key={s.status} className="flex items-center justify-between">
                    <StatusBadge status={s.status} />
                    <span className="text-sm font-medium">{s.count}</span>
                  </div>
                ))}
              </div>
            ) : (
              <p className="text-sm text-muted-foreground">No incidents</p>
            )}
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle className="text-base">Nodes per Cluster</CardTitle>
          </CardHeader>
          <CardContent>
            <DataTable columns={clusterColumns} data={stats.nodes_per_cluster ?? []} emptyMessage="No clusters" />
          </CardContent>
        </Card>
      </div>

      <Card>
        <CardHeader>
          <CardTitle className="text-base">Tenants per Shard</CardTitle>
        </CardHeader>
        <CardContent>
          <DataTable columns={shardColumns} data={stats.tenants_per_shard ?? []} emptyMessage="No shards" />
        </CardContent>
      </Card>
    </div>
  )
}
