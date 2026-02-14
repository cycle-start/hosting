import { useNavigate } from '@tanstack/react-router'
import { type ColumnDef } from '@tanstack/react-table'
import { FolderOpen, Users } from 'lucide-react'
import { ResourceHeader } from '@/components/shared/resource-header'
import { DataTable } from '@/components/shared/data-table'
import { StatsCard } from '@/components/shared/stats-card'
import { StatusBadge } from '@/components/shared/status-badge'
import { CopyButton } from '@/components/shared/copy-button'
import { formatDate, truncateID } from '@/lib/utils'
import { useDashboardStats, useTenants } from '@/lib/hooks'
import type { Tenant } from '@/lib/types'

export function WebrootsPage() {
  const navigate = useNavigate()
  const { data: stats } = useDashboardStats()
  const { data: tenantsData, isLoading } = useTenants({ limit: 50 })
  const tenants = tenantsData?.items ?? []

  const columns: ColumnDef<Tenant>[] = [
    {
      accessorKey: 'id',
      header: 'ID',
      cell: ({ row }) => (
        <div className="flex items-center gap-1">
          <code className="text-xs">{truncateID(row.original.id)}</code>
          <CopyButton value={row.original.id} />
        </div>
      ),
    },
    { accessorKey: 'name', header: 'Tenant Name' },
    {
      accessorKey: 'region_name',
      header: 'Region',
      cell: ({ row }) => row.original.region_name || row.original.region_id,
    },
    {
      accessorKey: 'status',
      header: 'Status',
      cell: ({ row }) => <StatusBadge status={row.original.status} />,
    },
    {
      accessorKey: 'created_at',
      header: 'Created',
      cell: ({ row }) => <span className="text-sm text-muted-foreground">{formatDate(row.original.created_at)}</span>,
    },
  ]

  return (
    <div className="space-y-6">
      <ResourceHeader title="Webroots" subtitle="Webroots are managed per tenant. Click a tenant to view its webroots." />

      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        <StatsCard label="Total FQDNs" value={stats?.fqdns ?? 0} icon={FolderOpen} />
        <StatsCard label="Total Tenants" value={stats?.tenants ?? 0} icon={Users} />
      </div>

      <div>
        <h2 className="mb-4 text-lg font-semibold">Select a tenant</h2>
        <DataTable
          columns={columns}
          data={tenants}
          loading={isLoading}
          searchColumn="name"
          searchPlaceholder="Search tenants..."
          onRowClick={(t) => navigate({ to: '/tenants/$id', params: { id: t.id } })}
        />
      </div>
    </div>
  )
}
