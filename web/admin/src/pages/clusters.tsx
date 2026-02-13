import { useState } from 'react'
import { type ColumnDef } from '@tanstack/react-table'
import { Plus, Trash2, Server } from 'lucide-react'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select, SelectContent, SelectItem, SelectTrigger, SelectValue,
} from '@/components/ui/select'
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
import { useRegions, useClusters, useCreateCluster, useDeleteCluster } from '@/lib/hooks'
import type { Cluster } from '@/lib/types'

export function ClustersPage() {
  const [selectedRegion, setSelectedRegion] = useState('')
  const [createOpen, setCreateOpen] = useState(false)
  const [deleteTarget, setDeleteTarget] = useState<Cluster | null>(null)
  const [formId, setFormId] = useState('')
  const [formName, setFormName] = useState('')
  const [formRegion, setFormRegion] = useState('')

  const { data: regionsData } = useRegions({ limit: 100 })
  const { data, isLoading } = useClusters(selectedRegion)
  const createMutation = useCreateCluster()
  const deleteMutation = useDeleteCluster()

  const regions = regionsData?.items ?? []
  const clusters = data?.items ?? []

  // Auto-select first region
  if (!selectedRegion && regions.length > 0) {
    setSelectedRegion(regions[0].id)
  }

  const columns: ColumnDef<Cluster>[] = [
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
      await createMutation.mutateAsync({ id: formId, name: formName, region_id: formRegion })
      toast.success('Cluster created')
      setCreateOpen(false)
      setFormId('')
      setFormName('')
      setFormRegion('')
    } catch (e: unknown) {
      toast.error(e instanceof Error ? e.message : 'Failed to create cluster')
    }
  }

  const handleDelete = async () => {
    if (!deleteTarget) return
    try {
      await deleteMutation.mutateAsync(deleteTarget.id)
      toast.success('Cluster deleted')
      setDeleteTarget(null)
    } catch (e: unknown) {
      toast.error(e instanceof Error ? e.message : 'Failed to delete cluster')
    }
  }

  return (
    <div className="space-y-6">
      <ResourceHeader
        title="Clusters"
        actions={
          <Button onClick={() => { setFormRegion(selectedRegion); setCreateOpen(true) }}>
            <Plus className="mr-2 h-4 w-4" /> Create Cluster
          </Button>
        }
      />

      <div className="max-w-xs">
        <Label className="mb-2 block text-sm">Region</Label>
        <Select value={selectedRegion} onValueChange={setSelectedRegion}>
          <SelectTrigger>
            <SelectValue placeholder="Select a region" />
          </SelectTrigger>
          <SelectContent>
            {regions.map((r) => (
              <SelectItem key={r.id} value={r.id}>{r.name} ({r.id})</SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>

      {!selectedRegion ? (
        <EmptyState icon={Server} title="Select a region" description="Choose a region above to view its clusters." />
      ) : !isLoading && clusters.length === 0 ? (
        <EmptyState
          icon={Server}
          title="No clusters"
          description="This region has no clusters yet."
          action={{ label: 'Create Cluster', onClick: () => { setFormRegion(selectedRegion); setCreateOpen(true) } }}
        />
      ) : (
        <DataTable columns={columns} data={clusters} loading={isLoading} searchColumn="name" searchPlaceholder="Search clusters..." />
      )}

      <Dialog open={createOpen} onOpenChange={setCreateOpen}>
        <DialogContent>
          <DialogHeader><DialogTitle>Create Cluster</DialogTitle></DialogHeader>
          <div className="space-y-4">
            <div className="space-y-2">
              <Label>Region</Label>
              <Select value={formRegion} onValueChange={setFormRegion}>
                <SelectTrigger><SelectValue placeholder="Select region" /></SelectTrigger>
                <SelectContent>
                  {regions.map((r) => (
                    <SelectItem key={r.id} value={r.id}>{r.name}</SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
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
            <Button onClick={handleCreate} disabled={createMutation.isPending || !formId || !formName || !formRegion}>
              {createMutation.isPending ? 'Creating...' : 'Create'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <ConfirmDialog
        open={!!deleteTarget}
        onOpenChange={(open) => !open && setDeleteTarget(null)}
        title="Delete Cluster"
        description={`Are you sure you want to delete cluster "${deleteTarget?.name}"?`}
        confirmLabel="Delete"
        variant="destructive"
        onConfirm={handleDelete}
        loading={deleteMutation.isPending}
      />
    </div>
  )
}
