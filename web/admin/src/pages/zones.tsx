import { useState } from 'react'
import { useNavigate } from '@tanstack/react-router'
import { type ColumnDef } from '@tanstack/react-table'
import { Plus, Trash2, Globe } from 'lucide-react'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter,
} from '@/components/ui/dialog'
import { ResourceHeader } from '@/components/shared/resource-header'
import { DataTable } from '@/components/shared/data-table'
import { EmptyState } from '@/components/shared/empty-state'
import { StatusBadge } from '@/components/shared/status-badge'
import { ConfirmDialog } from '@/components/shared/confirm-dialog'
import { CopyButton } from '@/components/shared/copy-button'
import { formatDate, truncateID } from '@/lib/utils'
import { useZones, useCreateZone, useDeleteZone } from '@/lib/hooks'
import type { Zone } from '@/lib/types'

export function ZonesPage() {
  const navigate = useNavigate()
  const [createOpen, setCreateOpen] = useState(false)
  const [deleteTarget, setDeleteTarget] = useState<Zone | null>(null)
  const [formName, setFormName] = useState('')
  const [formTenant, setFormTenant] = useState('')
  const [formRegion, setFormRegion] = useState('')

  const { data, isLoading } = useZones()
  const createMutation = useCreateZone()
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
    { accessorKey: 'region_id', header: 'Region' },
    {
      accessorKey: 'tenant_id',
      header: 'Tenant',
      cell: ({ row }) => row.original.tenant_id ? <code className="text-xs">{truncateID(row.original.tenant_id)}</code> : <span className="text-muted-foreground">-</span>,
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

  const handleCreate = async () => {
    try {
      await createMutation.mutateAsync({ name: formName, tenant_id: formTenant, region_id: formRegion })
      toast.success('Zone created')
      setCreateOpen(false)
      setFormName('')
      setFormTenant('')
      setFormRegion('')
    } catch (e: unknown) {
      toast.error(e instanceof Error ? e.message : 'Failed to create zone')
    }
  }

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
        subtitle={`${zones.length} zone${zones.length !== 1 ? 's' : ''}`}
        actions={
          <Button onClick={() => setCreateOpen(true)}>
            <Plus className="mr-2 h-4 w-4" /> Create Zone
          </Button>
        }
      />

      {!isLoading && zones.length === 0 ? (
        <EmptyState
          icon={Globe}
          title="No zones"
          description="Create a DNS zone to get started."
          action={{ label: 'Create Zone', onClick: () => setCreateOpen(true) }}
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

      <Dialog open={createOpen} onOpenChange={setCreateOpen}>
        <DialogContent>
          <DialogHeader><DialogTitle>Create Zone</DialogTitle></DialogHeader>
          <div className="space-y-4">
            <div className="space-y-2">
              <Label>Zone Name</Label>
              <Input placeholder="example.com" value={formName} onChange={(e) => setFormName(e.target.value)} />
            </div>
            <div className="space-y-2">
              <Label>Tenant ID</Label>
              <Input placeholder="Tenant UUID" value={formTenant} onChange={(e) => setFormTenant(e.target.value)} />
            </div>
            <div className="space-y-2">
              <Label>Region ID</Label>
              <Input placeholder="e.g. osl-1" value={formRegion} onChange={(e) => setFormRegion(e.target.value)} />
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setCreateOpen(false)}>Cancel</Button>
            <Button onClick={handleCreate} disabled={createMutation.isPending || !formName || !formTenant || !formRegion}>
              {createMutation.isPending ? 'Creating...' : 'Create'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

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
