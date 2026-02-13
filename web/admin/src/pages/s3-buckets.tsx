import { useNavigate } from '@tanstack/react-router'
import { type ColumnDef } from '@tanstack/react-table'
import { HardDrive, Users } from 'lucide-react'
import { ResourceHeader } from '@/components/shared/resource-header'
import { DataTable } from '@/components/shared/data-table'
import { StatsCard } from '@/components/shared/stats-card'
import { StatusBadge } from '@/components/shared/status-badge'
import { CopyButton } from '@/components/shared/copy-button'
import { formatDate, truncateID } from '@/lib/utils'
import { useDashboardStats, useTenants } from '@/lib/hooks'
import type { Tenant } from '@/lib/types'

export function S3BucketsPage() {
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
    { accessorKey: 'region_id', header: 'Region' },
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
      <ResourceHeader title="S3 Buckets" subtitle="S3 buckets are managed per tenant. Click a tenant to view its buckets." />

      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        <StatsCard label="Total Tenants" value={stats?.tenants ?? 0} icon={Users} />
        <StatsCard label="S3 Buckets" value={0} icon={HardDrive} />
      </div>

      <div>
        <h2 className="mb-4 text-lg font-semibold">Select a tenant</h2>
        <DataTable
          columns={columns}
          data={tenants}
          loading={isLoading}
          searchColumn="id"
          searchPlaceholder="Search tenants..."
          onRowClick={(t) => navigate({ to: '/tenants/$id', params: { id: t.id } })}
        />
      </div>
    </div>
  )
}
