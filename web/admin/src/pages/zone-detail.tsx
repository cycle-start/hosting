import { useState } from 'react'
import { useParams, useNavigate } from '@tanstack/react-router'
import { type ColumnDef } from '@tanstack/react-table'
import { ArrowLeft, Plus, Trash2 } from 'lucide-react'
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
import { Skeleton } from '@/components/ui/skeleton'
import { ResourceHeader } from '@/components/shared/resource-header'
import { DataTable } from '@/components/shared/data-table'
import { StatusBadge } from '@/components/shared/status-badge'
import { ConfirmDialog } from '@/components/shared/confirm-dialog'
import { CopyButton } from '@/components/shared/copy-button'
import { formatDate, truncateID } from '@/lib/utils'
import { useZone, useZoneRecords, useCreateZoneRecord, useDeleteZoneRecord } from '@/lib/hooks'
import type { ZoneRecord } from '@/lib/types'

const recordTypes = ['A', 'AAAA', 'CNAME', 'MX', 'TXT', 'NS', 'SRV', 'CAA', 'PTR']

export function ZoneDetailPage() {
  const { id } = useParams({ from: '/auth/zones/$id' as never })
  const navigate = useNavigate()
  const [createOpen, setCreateOpen] = useState(false)
  const [deleteTarget, setDeleteTarget] = useState<ZoneRecord | null>(null)
  const [formType, setFormType] = useState('A')
  const [formName, setFormName] = useState('')
  const [formContent, setFormContent] = useState('')
  const [formTTL, setFormTTL] = useState('3600')
  const [formPriority, setFormPriority] = useState('')

  const { data: zone, isLoading: zoneLoading } = useZone(id)
  const { data: recordsData, isLoading: recordsLoading } = useZoneRecords(id)
  const createMutation = useCreateZoneRecord()
  const deleteMutation = useDeleteZoneRecord()

  const records = recordsData?.items ?? []

  const columns: ColumnDef<ZoneRecord>[] = [
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
    { accessorKey: 'type', header: 'Type' },
    { accessorKey: 'name', header: 'Name' },
    {
      accessorKey: 'content',
      header: 'Content',
      cell: ({ row }) => <code className="text-xs">{row.original.content}</code>,
    },
    { accessorKey: 'ttl', header: 'TTL' },
    {
      accessorKey: 'priority',
      header: 'Priority',
      cell: ({ row }) => row.original.priority ?? '-',
    },
    { accessorKey: 'managed_by', header: 'Managed By' },
    {
      accessorKey: 'status',
      header: 'Status',
      cell: ({ row }) => <StatusBadge status={row.original.status} />,
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
        zone_id: id,
        type: formType,
        name: formName,
        content: formContent,
        ttl: parseInt(formTTL) || 3600,
        priority: formPriority ? parseInt(formPriority) : undefined,
      })
      toast.success('Record created')
      setCreateOpen(false)
      setFormType('A')
      setFormName('')
      setFormContent('')
      setFormTTL('3600')
      setFormPriority('')
    } catch (e: unknown) {
      toast.error(e instanceof Error ? e.message : 'Failed to create record')
    }
  }

  const handleDelete = async () => {
    if (!deleteTarget) return
    try {
      await deleteMutation.mutateAsync(deleteTarget.id)
      toast.success('Record deleted')
      setDeleteTarget(null)
    } catch (e: unknown) {
      toast.error(e instanceof Error ? e.message : 'Failed to delete record')
    }
  }

  if (zoneLoading || !zone) {
    return (
      <div className="space-y-6">
        <Skeleton className="h-10 w-64" />
        <Skeleton className="h-64 w-full" />
      </div>
    )
  }

  return (
    <div className="space-y-6">
      <Button variant="ghost" size="sm" onClick={() => navigate({ to: '/zones' })}>
        <ArrowLeft className="mr-2 h-4 w-4" /> Back to Zones
      </Button>

      <ResourceHeader
        title={zone.name}
        subtitle={`Region: ${zone.region_name || zone.region_id}${zone.tenant_id ? ` | Tenant: ${zone.tenant_id}` : ''}`}
        status={zone.status}
        actions={
          <Button onClick={() => setCreateOpen(true)}>
            <Plus className="mr-2 h-4 w-4" /> Add Record
          </Button>
        }
      />

      <div className="flex items-center gap-2 text-sm text-muted-foreground">
        <span>ID: <code>{zone.id}</code></span>
        <CopyButton value={zone.id} />
        <span className="ml-4">Created: {formatDate(zone.created_at)}</span>
      </div>

      <DataTable
        columns={columns}
        data={records}
        loading={recordsLoading}
        searchColumn="name"
        searchPlaceholder="Search records..."
        emptyMessage="No DNS records"
      />

      <Dialog open={createOpen} onOpenChange={setCreateOpen}>
        <DialogContent>
          <DialogHeader><DialogTitle>Add DNS Record</DialogTitle></DialogHeader>
          <div className="space-y-4">
            <div className="space-y-2">
              <Label>Type</Label>
              <Select value={formType} onValueChange={setFormType}>
                <SelectTrigger><SelectValue /></SelectTrigger>
                <SelectContent>
                  {recordTypes.map((t) => <SelectItem key={t} value={t}>{t}</SelectItem>)}
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-2">
              <Label>Name</Label>
              <Input placeholder="@ or subdomain" value={formName} onChange={(e) => setFormName(e.target.value)} />
            </div>
            <div className="space-y-2">
              <Label>Content</Label>
              <Input placeholder="e.g. 192.168.1.1" value={formContent} onChange={(e) => setFormContent(e.target.value)} />
            </div>
            <div className="grid grid-cols-2 gap-4">
              <div className="space-y-2">
                <Label>TTL</Label>
                <Input type="number" value={formTTL} onChange={(e) => setFormTTL(e.target.value)} />
              </div>
              <div className="space-y-2">
                <Label>Priority (optional)</Label>
                <Input type="number" placeholder="10" value={formPriority} onChange={(e) => setFormPriority(e.target.value)} />
              </div>
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setCreateOpen(false)}>Cancel</Button>
            <Button onClick={handleCreate} disabled={createMutation.isPending || !formName || !formContent}>
              {createMutation.isPending ? 'Creating...' : 'Create'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <ConfirmDialog
        open={!!deleteTarget}
        onOpenChange={(open) => !open && setDeleteTarget(null)}
        title="Delete Record"
        description={`Delete ${deleteTarget?.type} record "${deleteTarget?.name}"?`}
        confirmLabel="Delete"
        variant="destructive"
        onConfirm={handleDelete}
        loading={deleteMutation.isPending}
      />
    </div>
  )
}
