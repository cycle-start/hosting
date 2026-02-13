import { useState } from 'react'
import { type ColumnDef } from '@tanstack/react-table'
import { Plus, Trash2, MapPin } from 'lucide-react'
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
import { ConfirmDialog } from '@/components/shared/confirm-dialog'
import { CopyButton } from '@/components/shared/copy-button'
import { formatDate, truncateID } from '@/lib/utils'
import { useRegions, useCreateRegion, useDeleteRegion } from '@/lib/hooks'
import type { Region } from '@/lib/types'

export function RegionsPage() {
  const [createOpen, setCreateOpen] = useState(false)
  const [deleteTarget, setDeleteTarget] = useState<Region | null>(null)
  const [formId, setFormId] = useState('')
  const [formName, setFormName] = useState('')

  const { data, isLoading } = useRegions()
  const createMutation = useCreateRegion()
  const deleteMutation = useDeleteRegion()

  const columns: ColumnDef<Region>[] = [
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
      await createMutation.mutateAsync({ id: formId, name: formName })
      toast.success('Region created')
      setCreateOpen(false)
      setFormId('')
      setFormName('')
    } catch (e: unknown) {
      toast.error(e instanceof Error ? e.message : 'Failed to create region')
    }
  }

  const handleDelete = async () => {
    if (!deleteTarget) return
    try {
      await deleteMutation.mutateAsync(deleteTarget.id)
      toast.success('Region deleted')
      setDeleteTarget(null)
    } catch (e: unknown) {
      toast.error(e instanceof Error ? e.message : 'Failed to delete region')
    }
  }

  const regions = data?.items ?? []

  return (
    <div className="space-y-6">
      <ResourceHeader
        title="Regions"
        subtitle={`${regions.length} region${regions.length !== 1 ? 's' : ''}`}
        actions={
          <Button onClick={() => setCreateOpen(true)}>
            <Plus className="mr-2 h-4 w-4" /> Create Region
          </Button>
        }
      />

      {!isLoading && regions.length === 0 ? (
        <EmptyState
          icon={MapPin}
          title="No regions"
          description="Create your first region to get started."
          action={{ label: 'Create Region', onClick: () => setCreateOpen(true) }}
        />
      ) : (
        <DataTable columns={columns} data={regions} loading={isLoading} searchColumn="name" searchPlaceholder="Search regions..." />
      )}

      <Dialog open={createOpen} onOpenChange={setCreateOpen}>
        <DialogContent>
          <DialogHeader><DialogTitle>Create Region</DialogTitle></DialogHeader>
          <div className="space-y-4">
            <div className="space-y-2">
              <Label>ID</Label>
              <Input placeholder="e.g. osl-1" value={formId} onChange={(e) => setFormId(e.target.value)} />
            </div>
            <div className="space-y-2">
              <Label>Name</Label>
              <Input placeholder="e.g. Oslo 1" value={formName} onChange={(e) => setFormName(e.target.value)} />
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setCreateOpen(false)}>Cancel</Button>
            <Button onClick={handleCreate} disabled={createMutation.isPending || !formId || !formName}>
              {createMutation.isPending ? 'Creating...' : 'Create'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <ConfirmDialog
        open={!!deleteTarget}
        onOpenChange={(open) => !open && setDeleteTarget(null)}
        title="Delete Region"
        description={`Are you sure you want to delete region "${deleteTarget?.name}"? This action cannot be undone.`}
        confirmLabel="Delete"
        variant="destructive"
        onConfirm={handleDelete}
        loading={deleteMutation.isPending}
      />
    </div>
  )
}
