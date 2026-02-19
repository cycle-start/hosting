import { useState } from 'react'
import { useNavigate } from '@tanstack/react-router'
import { type ColumnDef } from '@tanstack/react-table'
import { Trash2, Globe } from 'lucide-react'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { ResourceHeader } from '@/components/shared/resource-header'
import { DataTable } from '@/components/shared/data-table'
import { EmptyState } from '@/components/shared/empty-state'
import { StatusBadge } from '@/components/shared/status-badge'
import { ConfirmDialog } from '@/components/shared/confirm-dialog'
import { CopyButton } from '@/components/shared/copy-button'
import { formatDate, truncateID } from '@/lib/utils'
import { useZones, useDeleteZone } from '@/lib/hooks'
import type { Zone } from '@/lib/types'

export function ZonesPage() {
  const navigate = useNavigate()
  const [deleteTarget, setDeleteTarget] = useState<Zone | null>(null)

  const { data, isLoading } = useZones()
  const deleteMutation = useDeleteZone()

  const zones = data?.items ?? []

  const columns: ColumnDef<Zone>[] = [
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
    { accessorKey: 'name', header: 'Name' },
    {
      accessorKey: 'region_name',
      header: 'Region',
      cell: ({ row }) => row.original.region_name || row.original.region_id,
    },
    {
      accessorKey: 'tenant_name',
      header: 'Tenant',
      cell: ({ row }) => row.original.tenant_name
        ? <code className="text-xs">{row.original.tenant_name}</code>
        : row.original.tenant_id
          ? <code className="text-xs text-muted-foreground">{truncateID(row.original.tenant_id)}</code>
          : <span className="text-muted-foreground">-</span>,
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
    {
      id: 'actions',
      cell: ({ row }) => (
        <Button variant="ghost" size="icon" onClick={(e) => { e.stopPropagation(); setDeleteTarget(row.original) }}>
          <Trash2 className="h-4 w-4 text-destructive" />
        </Button>
      ),
    },
  ]

  const handleDelete = async () => {
    if (!deleteTarget) return
    try {
      await deleteMutation.mutateAsync(deleteTarget.id)
      toast.success('Zone deleted')
      setDeleteTarget(null)
    } catch (e: unknown) {
      toast.error(e instanceof Error ? e.message : 'Failed to delete zone')
    }
  }

  return (
    <div className="space-y-6">
      <ResourceHeader
        title="Zones"
        subtitle={`${zones.length} zone${zones.length !== 1 ? 's' : ''} — Create zones from the tenant detail page`}
      />

      {!isLoading && zones.length === 0 ? (
        <EmptyState
          icon={Globe}
          title="No zones"
          description="DNS zones are created from the tenant detail page."
        />
      ) : (
        <DataTable
          columns={columns}
          data={zones}
          loading={isLoading}
          searchColumn="name"
          searchPlaceholder="Search zones..."
          onRowClick={(z) => navigate({ to: '/zones/$id', params: { id: z.id } })}
        />
      )}

      <ConfirmDialog
        open={!!deleteTarget}
        onOpenChange={(open) => !open && setDeleteTarget(null)}
        title="Delete Zone"
        description={`Are you sure you want to delete zone "${deleteTarget?.name}"?`}
        confirmLabel="Delete"
        variant="destructive"
        onConfirm={handleDelete}
        loading={deleteMutation.isPending}
      />
    </div>
  )
}
