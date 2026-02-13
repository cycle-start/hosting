import { useState } from 'react'
import { useNavigate } from '@tanstack/react-router'
import { type ColumnDef } from '@tanstack/react-table'
import { Plus, Users, MoreHorizontal, Pause, Play, Trash2 } from 'lucide-react'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { ResourceHeader } from '@/components/shared/resource-header'
import { DataTable } from '@/components/shared/data-table'
import { EmptyState } from '@/components/shared/empty-state'
import { StatusBadge } from '@/components/shared/status-badge'
import { ConfirmDialog } from '@/components/shared/confirm-dialog'
import { CopyButton } from '@/components/shared/copy-button'
import { formatDate, truncateID } from '@/lib/utils'
import { useTenants, useDeleteTenant, useSuspendTenant, useUnsuspendTenant } from '@/lib/hooks'
import type { Tenant } from '@/lib/types'

export function TenantsPage() {
  const navigate = useNavigate()
  const [deleteTarget, setDeleteTarget] = useState<Tenant | null>(null)

  const { data, isLoading } = useTenants()
  const deleteMutation = useDeleteTenant()
  const suspendMutation = useSuspendTenant()
  const unsuspendMutation = useUnsuspendTenant()

  const tenants = data?.items ?? []

  const handleSuspend = async (id: string) => {
    try {
      await suspendMutation.mutateAsync(id)
      toast.success('Tenant suspended')
    } catch (e: unknown) {
      toast.error(e instanceof Error ? e.message : 'Failed to suspend tenant')
    }
  }

  const handleUnsuspend = async (id: string) => {
    try {
      await unsuspendMutation.mutateAsync(id)
      toast.success('Tenant unsuspended')
    } catch (e: unknown) {
      toast.error(e instanceof Error ? e.message : 'Failed to unsuspend tenant')
    }
  }

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
    { accessorKey: 'name', header: 'Name' },
    { accessorKey: 'region_id', header: 'Region' },
    { accessorKey: 'cluster_id', header: 'Cluster' },
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
      cell: ({ row }) => {
        const t = row.original
        return (
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button variant="ghost" size="icon" onClick={(e) => e.stopPropagation()}>
                <MoreHorizontal className="h-4 w-4" />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end">
              {t.status === 'active' && (
                <DropdownMenuItem onClick={() => handleSuspend(t.id)}>
                  <Pause className="mr-2 h-4 w-4" /> Suspend
                </DropdownMenuItem>
              )}
              {t.status === 'suspended' && (
                <DropdownMenuItem onClick={() => handleUnsuspend(t.id)}>
                  <Play className="mr-2 h-4 w-4" /> Unsuspend
                </DropdownMenuItem>
              )}
              <DropdownMenuItem className="text-destructive" onClick={() => setDeleteTarget(t)}>
                <Trash2 className="mr-2 h-4 w-4" /> Delete
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        )
      },
    },
  ]

  const handleDelete = async () => {
    if (!deleteTarget) return
    try {
      await deleteMutation.mutateAsync(deleteTarget.id)
      toast.success('Tenant deleted')
      setDeleteTarget(null)
    } catch (e: unknown) {
      toast.error(e instanceof Error ? e.message : 'Failed to delete tenant')
    }
  }

  return (
    <div className="space-y-6">
      <ResourceHeader
        title="Tenants"
        subtitle={`${tenants.length} tenant${tenants.length !== 1 ? 's' : ''}`}
        actions={
          <Button onClick={() => navigate({ to: '/tenants/new' })}>
            <Plus className="mr-2 h-4 w-4" /> Create Tenant
          </Button>
        }
      />

      {!isLoading && tenants.length === 0 ? (
        <EmptyState
          icon={Users}
          title="No tenants"
          description="Create your first tenant to get started."
          action={{ label: 'Create Tenant', onClick: () => navigate({ to: '/tenants/new' }) }}
        />
      ) : (
        <DataTable
          columns={columns}
          data={tenants}
          loading={isLoading}
          searchColumn="name"
          searchPlaceholder="Search tenants..."
          onRowClick={(t) => navigate({ to: '/tenants/$id', params: { id: t.id } })}
        />
      )}

      <ConfirmDialog
        open={!!deleteTarget}
        onOpenChange={(open) => !open && setDeleteTarget(null)}
        title="Delete Tenant"
        description={`Are you sure you want to delete tenant "${deleteTarget?.name}"? All associated resources will be removed.`}
        confirmLabel="Delete"
        variant="destructive"
        onConfirm={handleDelete}
        loading={deleteMutation.isPending}
      />
    </div>
  )
}
