import { useState } from 'react'
import { useNavigate } from '@tanstack/react-router'
import { type ColumnDef } from '@tanstack/react-table'
import { Plus, Trash2, Tag } from 'lucide-react'
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
import { useBrands, useCreateBrand, useDeleteBrand } from '@/lib/hooks'
import type { Brand } from '@/lib/types'

export function BrandsPage() {
  const navigate = useNavigate()
  const [createOpen, setCreateOpen] = useState(false)
  const [deleteTarget, setDeleteTarget] = useState<Brand | null>(null)
  const [formId, setFormId] = useState('')
  const [formName, setFormName] = useState('')
  const [formBaseHostname, setFormBaseHostname] = useState('')
  const [formPrimaryNS, setFormPrimaryNS] = useState('')
  const [formSecondaryNS, setFormSecondaryNS] = useState('')
  const [formHostmaster, setFormHostmaster] = useState('')

  const { data, isLoading } = useBrands()
  const createMutation = useCreateBrand()
  const deleteMutation = useDeleteBrand()

  const brands = data?.items ?? []

  const columns: ColumnDef<Brand>[] = [
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
    { accessorKey: 'base_hostname', header: 'Base Hostname' },
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
      await createMutation.mutateAsync({
        id: formId,
        name: formName,
        base_hostname: formBaseHostname,
        primary_ns: formPrimaryNS,
        secondary_ns: formSecondaryNS,
        hostmaster_email: formHostmaster,
      })
      toast.success('Brand created')
      setCreateOpen(false)
      setFormId(''); setFormName(''); setFormBaseHostname('')
      setFormPrimaryNS(''); setFormSecondaryNS(''); setFormHostmaster('')
    } catch (e: unknown) {
      toast.error(e instanceof Error ? e.message : 'Failed to create brand')
    }
  }

  const handleDelete = async () => {
    if (!deleteTarget) return
    try {
      await deleteMutation.mutateAsync(deleteTarget.id)
      toast.success('Brand deleted')
      setDeleteTarget(null)
    } catch (e: unknown) {
      toast.error(e instanceof Error ? e.message : 'Failed to delete brand')
    }
  }

  const canCreate = formId && formName && formBaseHostname && formPrimaryNS && formSecondaryNS && formHostmaster && !createMutation.isPending

  return (
    <div className="space-y-6">
      <ResourceHeader
        title="Brands"
        subtitle={`${brands.length} brand${brands.length !== 1 ? 's' : ''}`}
        actions={
          <Button onClick={() => setCreateOpen(true)}>
            <Plus className="mr-2 h-4 w-4" /> Create Brand
          </Button>
        }
      />

      {!isLoading && brands.length === 0 ? (
        <EmptyState
          icon={Tag}
          title="No brands"
          description="Create your first brand to get started. Brands define DNS settings and isolate tenants."
          action={{ label: 'Create Brand', onClick: () => setCreateOpen(true) }}
        />
      ) : (
        <DataTable
          columns={columns}
          data={brands}
          loading={isLoading}
          searchColumn="name"
          searchPlaceholder="Search brands..."
          onRowClick={(b) => navigate({ to: '/brands/$id', params: { id: b.id } })}
        />
      )}

      <Dialog open={createOpen} onOpenChange={setCreateOpen}>
        <DialogContent>
          <DialogHeader><DialogTitle>Create Brand</DialogTitle></DialogHeader>
          <div className="space-y-4">
            <div className="space-y-2">
              <Label>ID</Label>
              <Input placeholder="e.g. acme" value={formId} onChange={(e) => setFormId(e.target.value)} />
            </div>
            <div className="space-y-2">
              <Label>Name</Label>
              <Input placeholder="e.g. Acme Hosting" value={formName} onChange={(e) => setFormName(e.target.value)} />
            </div>
            <div className="space-y-2">
              <Label>Base Hostname</Label>
              <Input placeholder="e.g. acme.hosting" value={formBaseHostname} onChange={(e) => setFormBaseHostname(e.target.value)} />
            </div>
            <div className="space-y-2">
              <Label>Primary NS</Label>
              <Input placeholder="e.g. ns1.acme.hosting" value={formPrimaryNS} onChange={(e) => setFormPrimaryNS(e.target.value)} />
            </div>
            <div className="space-y-2">
              <Label>Secondary NS</Label>
              <Input placeholder="e.g. ns2.acme.hosting" value={formSecondaryNS} onChange={(e) => setFormSecondaryNS(e.target.value)} />
            </div>
            <div className="space-y-2">
              <Label>Hostmaster Email</Label>
              <Input placeholder="e.g. hostmaster.acme.hosting" value={formHostmaster} onChange={(e) => setFormHostmaster(e.target.value)} />
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setCreateOpen(false)}>Cancel</Button>
            <Button onClick={handleCreate} disabled={!canCreate}>
              {createMutation.isPending ? 'Creating...' : 'Create'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <ConfirmDialog
        open={!!deleteTarget}
        onOpenChange={(open) => !open && setDeleteTarget(null)}
        title="Delete Brand"
        description={`Are you sure you want to delete brand "${deleteTarget?.name}"? This action cannot be undone.`}
        confirmLabel="Delete"
        variant="destructive"
        onConfirm={handleDelete}
        loading={deleteMutation.isPending}
      />
    </div>
  )
}
