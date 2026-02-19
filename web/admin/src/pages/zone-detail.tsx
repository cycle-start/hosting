import { useState } from 'react'
import { useParams, useNavigate } from '@tanstack/react-router'
import { type ColumnDef } from '@tanstack/react-table'
import { ArrowLeft, Plus, Trash2, Pencil, RotateCw, ChevronDown, ChevronRight, Lock } from 'lucide-react'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import {
  Select, SelectContent, SelectItem, SelectTrigger, SelectValue,
} from '@/components/ui/select'
import {
  Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter,
} from '@/components/ui/dialog'
import {
  Tooltip, TooltipContent, TooltipTrigger,
} from '@/components/ui/tooltip'
import { Skeleton } from '@/components/ui/skeleton'
import { ResourceHeader } from '@/components/shared/resource-header'
import { DataTable } from '@/components/shared/data-table'
import { StatusBadge } from '@/components/shared/status-badge'
import { ConfirmDialog } from '@/components/shared/confirm-dialog'
import { LogViewer } from '@/components/shared/log-viewer'
import { formatDate } from '@/lib/utils'
import {
  useZone, useZoneRecords, useCreateZoneRecord, useUpdateZoneRecord,
  useDeleteZoneRecord, useRetryZoneRecord,
} from '@/lib/hooks'
import type { ZoneRecord } from '@/lib/types'

const recordTypes = [
  'A', 'AAAA', 'CNAME', 'MX', 'TXT', 'SRV', 'NS', 'CAA', 'PTR',
  'ALIAS', 'HTTPS', 'SVCB', 'TLSA', 'DNSKEY', 'DS', 'NAPTR', 'LOC', 'SSHFP', 'DNAME',
]

const typesMeta: Record<string, { placeholder: string; hint?: string; usesPriority?: boolean; usesTextarea?: boolean }> = {
  A:      { placeholder: '192.0.2.1' },
  AAAA:   { placeholder: '2001:db8::1' },
  CNAME:  { placeholder: 'target.example.com.' },
  MX:     { placeholder: 'mail.example.com.', usesPriority: true },
  TXT:    { placeholder: '"v=spf1 include:example.com ~all"', usesTextarea: true },
  SRV:    { placeholder: '0 5 5060 sip.example.com.', hint: 'weight port target', usesPriority: true },
  NS:     { placeholder: 'ns1.example.com.' },
  CAA:    { placeholder: '0 issue "letsencrypt.org"', hint: 'flag tag value' },
  PTR:    { placeholder: 'host.example.com.' },
  ALIAS:  { placeholder: 'target.example.com.', hint: 'Zone apex CNAME alternative' },
  HTTPS:  { placeholder: '1 . alpn="h2,h3"', hint: 'priority target params', usesPriority: true },
  SVCB:   { placeholder: '1 . alpn="h2"', hint: 'priority target params', usesPriority: true },
  TLSA:   { placeholder: '3 1 1 abc123...', hint: 'usage selector matching-type cert-data' },
  DNSKEY: { placeholder: '257 3 13 base64...', hint: 'flags protocol algorithm public-key' },
  DS:     { placeholder: '12345 13 2 abc123...', hint: 'key-tag algorithm digest-type digest' },
  NAPTR:  { placeholder: '100 10 "s" "SIP+D2U" "" _sip._udp.example.com.', hint: 'order pref flags service regexp replacement', usesPriority: true },
  LOC:    { placeholder: '51 30 12.748 N 0 7 39.612 W 0m', hint: 'latitude longitude altitude' },
  SSHFP:  { placeholder: '4 2 abc123...', hint: 'algorithm fingerprint-type fingerprint' },
  DNAME:  { placeholder: 'target.example.com.', hint: 'Delegation name — rewrites subtree' },
}

function getMeta(type: string) {
  return typesMeta[type] || { placeholder: '' }
}

// Format long DNS content for readable display.
function ContentCell({ content, type }: { content: string; type: string }) {
  const [expanded, setExpanded] = useState(false)

  // Short values render inline.
  if (content.length <= 50) {
    return <code className="text-xs">{content}</code>
  }

  // Long TXT/DKIM values: show truncated with expand toggle.
  if (!expanded) {
    return (
      <button
        className="flex items-center gap-1 text-left group"
        onClick={(e) => { e.stopPropagation(); setExpanded(true) }}
      >
        <code className="text-xs truncate max-w-[300px] block">{content}</code>
        <ChevronRight className="h-3 w-3 shrink-0 text-muted-foreground group-hover:text-foreground" />
      </button>
    )
  }

  return (
    <button
      className="flex items-start gap-1 text-left group"
      onClick={(e) => { e.stopPropagation(); setExpanded(false) }}
    >
      <code className="text-xs break-all whitespace-pre-wrap max-w-[400px] block">{content}</code>
      <ChevronDown className="h-3 w-3 shrink-0 mt-0.5 text-muted-foreground group-hover:text-foreground" />
    </button>
  )
}

export function ZoneDetailPage() {
  const { id } = useParams({ from: '/auth/zones/$id' as never })
  const navigate = useNavigate()

  // Dialog state
  const [createOpen, setCreateOpen] = useState(false)
  const [editTarget, setEditTarget] = useState<ZoneRecord | null>(null)
  const [deleteTarget, setDeleteTarget] = useState<ZoneRecord | null>(null)

  // Create form
  const [formType, setFormType] = useState('A')
  const [formName, setFormName] = useState('')
  const [formContent, setFormContent] = useState('')
  const [formTTL, setFormTTL] = useState('3600')
  const [formPriority, setFormPriority] = useState('')

  // Edit form
  const [editContent, setEditContent] = useState('')
  const [editTTL, setEditTTL] = useState('')
  const [editPriority, setEditPriority] = useState('')

  const { data: zone, isLoading: zoneLoading } = useZone(id)
  const { data: recordsData, isLoading: recordsLoading } = useZoneRecords(id)
  const createMutation = useCreateZoneRecord()
  const updateMutation = useUpdateZoneRecord()
  const deleteMutation = useDeleteZoneRecord()
  const retryMutation = useRetryZoneRecord()

  const records = recordsData?.items ?? []

  const openEdit = (record: ZoneRecord) => {
    setEditTarget(record)
    setEditContent(record.content)
    setEditTTL(String(record.ttl))
    setEditPriority(record.priority != null ? String(record.priority) : '')
  }

  const columns: ColumnDef<ZoneRecord>[] = [
    {
      accessorKey: 'type',
      header: 'Type',
      size: 70,
      cell: ({ row }) => (
        <Badge variant="outline" className="font-mono text-xs font-semibold">
          {row.original.type}
        </Badge>
      ),
    },
    {
      accessorKey: 'name',
      header: 'Name',
      size: 240,
      cell: ({ row }) => (
        <code className="text-xs">{row.original.name}</code>
      ),
    },
    {
      accessorKey: 'content',
      header: 'Content',
      cell: ({ row }) => (
        <ContentCell content={row.original.content} type={row.original.type} />
      ),
    },
    {
      accessorKey: 'ttl',
      header: 'TTL',
      size: 60,
      cell: ({ row }) => (
        <span className="text-xs text-muted-foreground">{row.original.ttl}</span>
      ),
    },
    {
      accessorKey: 'priority',
      header: 'Pri',
      size: 50,
      cell: ({ row }) => (
        <span className="text-xs text-muted-foreground">
          {row.original.priority ?? '-'}
        </span>
      ),
    },
    {
      accessorKey: 'managed_by',
      header: 'Source',
      size: 80,
      cell: ({ row }) => {
        const mb = row.original.managed_by
        if (mb === 'auto') {
          return (
            <Tooltip>
              <TooltipTrigger>
                <Badge variant="secondary" className="text-xs gap-1">
                  <Lock className="h-3 w-3" /> Auto
                </Badge>
              </TooltipTrigger>
              <TooltipContent>Managed automatically — cannot be edited</TooltipContent>
            </Tooltip>
          )
        }
        return <Badge variant="outline" className="text-xs">Custom</Badge>
      },
    },
    {
      accessorKey: 'status',
      header: 'Status',
      size: 80,
      cell: ({ row }) => <StatusBadge status={row.original.status} />,
    },
    {
      id: 'actions',
      size: 100,
      cell: ({ row }) => {
        const r = row.original
        const isCustom = r.managed_by === 'custom'
        const isFailed = r.status === 'failed'
        return (
          <div className="flex items-center gap-1 justify-end">
            {isFailed && (
              <Button
                variant="ghost" size="icon" className="h-7 w-7"
                onClick={(e) => { e.stopPropagation(); handleRetry(r.id) }}
                title="Retry"
              >
                <RotateCw className="h-3.5 w-3.5 text-yellow-500" />
              </Button>
            )}
            {isCustom && (
              <Button
                variant="ghost" size="icon" className="h-7 w-7"
                onClick={(e) => { e.stopPropagation(); openEdit(r) }}
                title="Edit"
              >
                <Pencil className="h-3.5 w-3.5" />
              </Button>
            )}
            {isCustom && (
              <Button
                variant="ghost" size="icon" className="h-7 w-7"
                onClick={(e) => { e.stopPropagation(); setDeleteTarget(r) }}
                title="Delete"
              >
                <Trash2 className="h-3.5 w-3.5 text-destructive" />
              </Button>
            )}
          </div>
        )
      },
    },
  ]

  const resetCreateForm = () => {
    setFormType('A')
    setFormName('')
    setFormContent('')
    setFormTTL('3600')
    setFormPriority('')
  }

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
      toast.success('Record created — provisioning')
      setCreateOpen(false)
      resetCreateForm()
    } catch (e: unknown) {
      toast.error(e instanceof Error ? e.message : 'Failed to create record')
    }
  }

  const handleUpdate = async () => {
    if (!editTarget) return
    try {
      await updateMutation.mutateAsync({
        id: editTarget.id,
        content: editContent || undefined,
        ttl: editTTL ? parseInt(editTTL) : undefined,
        priority: editPriority ? parseInt(editPriority) : undefined,
      })
      toast.success('Record updated — provisioning')
      setEditTarget(null)
    } catch (e: unknown) {
      toast.error(e instanceof Error ? e.message : 'Failed to update record')
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

  const handleRetry = async (recordId: string) => {
    try {
      await retryMutation.mutateAsync(recordId)
      toast.success('Retrying record provisioning')
    } catch (e: unknown) {
      toast.error(e instanceof Error ? e.message : 'Failed to retry')
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

  const createMeta = getMeta(formType)
  const editMeta = editTarget ? getMeta(editTarget.type) : { placeholder: '' }

  // Build subtitle parts.
  const subtitleParts: string[] = []
  subtitleParts.push(`Region: ${zone.region_name || zone.region_id}`)
  if (zone.tenant_name) {
    subtitleParts.push(`Tenant: ${zone.tenant_name}`)
  } else if (zone.tenant_id) {
    subtitleParts.push(`Tenant: ${zone.tenant_id}`)
  }

  return (
    <div className="space-y-6">
      <Button variant="ghost" size="sm" onClick={() => navigate({ to: '/zones' })}>
        <ArrowLeft className="mr-2 h-4 w-4" /> Back to Zones
      </Button>

      <ResourceHeader
        title={zone.name}
        subtitle={subtitleParts.join(' | ')}
        status={zone.status}
        meta={`Created ${formatDate(zone.created_at)}`}
        actions={
          <Button onClick={() => setCreateOpen(true)}>
            <Plus className="mr-2 h-4 w-4" /> Add Record
          </Button>
        }
      />

      <DataTable
        columns={columns}
        data={records}
        loading={recordsLoading}
        globalSearch
        searchPlaceholder="Filter records (type, name, content...)"
        emptyMessage="No DNS records"
      />

      {/* Logs */}
      <LogViewer query={`{app=~"core-api|worker|node-agent"} |= "${id}"`} title="Logs" />

      {/* Create Dialog */}
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
              {createMeta.usesTextarea ? (
                <Textarea
                  placeholder={createMeta.placeholder}
                  value={formContent}
                  onChange={(e) => setFormContent(e.target.value)}
                  className="font-mono text-xs"
                />
              ) : (
                <Input
                  placeholder={createMeta.placeholder}
                  value={formContent}
                  onChange={(e) => setFormContent(e.target.value)}
                  className="font-mono"
                />
              )}
              {createMeta.hint && (
                <p className="text-xs text-muted-foreground">{createMeta.hint}</p>
              )}
            </div>
            <div className="grid grid-cols-2 gap-4">
              <div className="space-y-2">
                <Label>TTL</Label>
                <Input type="number" value={formTTL} onChange={(e) => setFormTTL(e.target.value)} />
              </div>
              {createMeta.usesPriority && (
                <div className="space-y-2">
                  <Label>Priority</Label>
                  <Input type="number" placeholder="10" value={formPriority} onChange={(e) => setFormPriority(e.target.value)} />
                </div>
              )}
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

      {/* Edit Dialog */}
      <Dialog open={!!editTarget} onOpenChange={(open) => !open && setEditTarget(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>
              Edit {editTarget?.type} Record — {editTarget?.name}
            </DialogTitle>
          </DialogHeader>
          <div className="space-y-4">
            <div className="space-y-2">
              <Label>Content</Label>
              {editMeta.usesTextarea ? (
                <Textarea
                  placeholder={editMeta.placeholder}
                  value={editContent}
                  onChange={(e) => setEditContent(e.target.value)}
                  className="font-mono text-xs"
                />
              ) : (
                <Input
                  placeholder={editMeta.placeholder}
                  value={editContent}
                  onChange={(e) => setEditContent(e.target.value)}
                  className="font-mono"
                />
              )}
              {editMeta.hint && (
                <p className="text-xs text-muted-foreground">{editMeta.hint}</p>
              )}
            </div>
            <div className="grid grid-cols-2 gap-4">
              <div className="space-y-2">
                <Label>TTL</Label>
                <Input type="number" value={editTTL} onChange={(e) => setEditTTL(e.target.value)} />
              </div>
              {editMeta.usesPriority && (
                <div className="space-y-2">
                  <Label>Priority</Label>
                  <Input type="number" value={editPriority} onChange={(e) => setEditPriority(e.target.value)} />
                </div>
              )}
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setEditTarget(null)}>Cancel</Button>
            <Button onClick={handleUpdate} disabled={updateMutation.isPending || !editContent}>
              {updateMutation.isPending ? 'Saving...' : 'Save'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Delete Confirm */}
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
