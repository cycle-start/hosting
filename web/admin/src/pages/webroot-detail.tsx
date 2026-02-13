import { useState } from 'react'
import { useParams, useNavigate } from '@tanstack/react-router'
import { type ColumnDef } from '@tanstack/react-table'
import { Plus, Trash2, Pencil, Globe } from 'lucide-react'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { Skeleton } from '@/components/ui/skeleton'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from '@/components/ui/dialog'
import { ResourceHeader } from '@/components/shared/resource-header'
import { DataTable } from '@/components/shared/data-table'
import { StatusBadge } from '@/components/shared/status-badge'
import { EmptyState } from '@/components/shared/empty-state'
import { ConfirmDialog } from '@/components/shared/confirm-dialog'
import { Breadcrumb } from '@/components/shared/breadcrumb'
import { CopyButton } from '@/components/shared/copy-button'
import { formatDate } from '@/lib/utils'
import { rules, validateField } from '@/lib/validation'
import {
  useWebroot, useFQDNs,
  useUpdateWebroot, useCreateFQDN, useDeleteFQDN,
} from '@/lib/hooks'
import type { FQDN } from '@/lib/types'

const runtimes = ['php', 'node', 'python', 'ruby', 'static']

export function WebrootDetailPage() {
  const { id: tenantId, webrootId } = useParams({ from: '/tenants/$id/webroots/$webrootId' as never })
  const navigate = useNavigate()

  const [editOpen, setEditOpen] = useState(false)
  const [createFqdnOpen, setCreateFqdnOpen] = useState(false)
  const [deleteFqdn, setDeleteFqdn] = useState<FQDN | null>(null)
  const [touched, setTouched] = useState<Record<string, boolean>>({})

  // Edit form
  const [editRuntime, setEditRuntime] = useState('')
  const [editVersion, setEditVersion] = useState('')
  const [editPublicFolder, setEditPublicFolder] = useState('')

  // Create FQDN form
  const [fqdnValue, setFqdnValue] = useState('')
  const [fqdnSsl, setFqdnSsl] = useState(true)

  const { data: webroot, isLoading } = useWebroot(webrootId)
  const { data: fqdnsData, isLoading: fqdnsLoading } = useFQDNs(webrootId)
  const updateMut = useUpdateWebroot()
  const createFqdnMut = useCreateFQDN()
  const deleteFqdnMut = useDeleteFQDN()

  if (isLoading || !webroot) {
    return <div className="space-y-6"><Skeleton className="h-10 w-64" /><Skeleton className="h-64 w-full" /></div>
  }

  const touch = (f: string) => setTouched(p => ({ ...p, [f]: true }))
  const fqdnErr = touched['fqdn'] ? validateField(fqdnValue, [rules.required(), rules.fqdn()]) : null

  const openEdit = () => {
    setEditRuntime(webroot.runtime)
    setEditVersion(webroot.runtime_version)
    setEditPublicFolder(webroot.public_folder)
    setEditOpen(true)
  }

  const handleUpdate = async () => {
    try {
      await updateMut.mutateAsync({ id: webrootId, runtime: editRuntime, runtime_version: editVersion, public_folder: editPublicFolder })
      toast.success('Webroot updated'); setEditOpen(false)
    } catch (e: unknown) { toast.error(e instanceof Error ? e.message : 'Failed') }
  }

  const handleCreateFqdn = async () => {
    setTouched({ fqdn: true })
    if (validateField(fqdnValue, [rules.required(), rules.fqdn()])) return
    try {
      await createFqdnMut.mutateAsync({ webroot_id: webrootId, fqdn: fqdnValue, ssl_enabled: fqdnSsl })
      toast.success('FQDN created'); setCreateFqdnOpen(false); setFqdnValue(''); setFqdnSsl(true); setTouched({})
    } catch (e: unknown) { toast.error(e instanceof Error ? e.message : 'Failed') }
  }

  const columns: ColumnDef<FQDN>[] = [
    {
      accessorKey: 'fqdn', header: 'Hostname',
      cell: ({ row }) => <span className="font-medium">{row.original.fqdn}</span>,
    },
    {
      accessorKey: 'ssl_enabled', header: 'SSL',
      cell: ({ row }) => row.original.ssl_enabled ? <span className="text-green-600">Enabled</span> : <span className="text-muted-foreground">Disabled</span>,
    },
    {
      accessorKey: 'status', header: 'Status',
      cell: ({ row }) => <StatusBadge status={row.original.status} />,
    },
    {
      accessorKey: 'created_at', header: 'Created',
      cell: ({ row }) => <span className="text-sm text-muted-foreground">{formatDate(row.original.created_at)}</span>,
    },
    {
      id: 'actions',
      cell: ({ row }) => (
        <Button variant="ghost" size="icon" onClick={(e) => { e.stopPropagation(); setDeleteFqdn(row.original) }}>
          <Trash2 className="h-4 w-4 text-destructive" />
        </Button>
      ),
    },
  ]

  return (
    <div className="space-y-6">
      <Breadcrumb segments={[
        { label: 'Tenants', href: '/tenants' },
        { label: tenantId, href: `/tenants/${tenantId}` },
        { label: 'Webroots' },
        { label: webroot.name },
      ]} />

      <ResourceHeader
        title={webroot.name}
        subtitle={`${webroot.runtime} ${webroot.runtime_version} | Public: ${webroot.public_folder}`}
        status={webroot.status}
        actions={
          <Button variant="outline" size="sm" onClick={openEdit}>
            <Pencil className="mr-2 h-4 w-4" /> Edit
          </Button>
        }
      />

      <div className="flex items-center gap-2 text-sm text-muted-foreground">
        <span>ID: <code>{webroot.id}</code></span>
        <CopyButton value={webroot.id} />
        <span className="ml-4">Created: {formatDate(webroot.created_at)}</span>
      </div>

      <div>
        <div className="mb-4 flex items-center justify-between">
          <h2 className="text-lg font-semibold">FQDNs</h2>
          <Button size="sm" onClick={() => { setFqdnValue(''); setFqdnSsl(true); setTouched({}); setCreateFqdnOpen(true) }}>
            <Plus className="mr-2 h-4 w-4" /> Add FQDN
          </Button>
        </div>
        {!fqdnsLoading && (fqdnsData?.items?.length ?? 0) === 0 ? (
          <EmptyState icon={Globe} title="No FQDNs" description="Add a domain name to this webroot." action={{ label: 'Add FQDN', onClick: () => setCreateFqdnOpen(true) }} />
        ) : (
          <DataTable columns={columns} data={fqdnsData?.items ?? []} loading={fqdnsLoading} searchColumn="fqdn" searchPlaceholder="Search FQDNs..."
            onRowClick={(f) => navigate({ to: '/tenants/$id/fqdns/$fqdnId', params: { id: tenantId, fqdnId: f.id } })} />
        )}
      </div>

      {/* Edit Webroot */}
      <Dialog open={editOpen} onOpenChange={setEditOpen}>
        <DialogContent>
          <DialogHeader><DialogTitle>Edit Webroot</DialogTitle></DialogHeader>
          <div className="space-y-4">
            <div className="grid grid-cols-2 gap-4">
              <div className="space-y-2">
                <Label>Runtime</Label>
                <Select value={editRuntime} onValueChange={setEditRuntime}>
                  <SelectTrigger><SelectValue /></SelectTrigger>
                  <SelectContent>
                    {runtimes.map(r => <SelectItem key={r} value={r}>{r}</SelectItem>)}
                  </SelectContent>
                </Select>
              </div>
              <div className="space-y-2">
                <Label>Version</Label>
                <Input value={editVersion} onChange={(e) => setEditVersion(e.target.value)} />
              </div>
            </div>
            <div className="space-y-2">
              <Label>Public Folder</Label>
              <Input value={editPublicFolder} onChange={(e) => setEditPublicFolder(e.target.value)} />
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setEditOpen(false)}>Cancel</Button>
            <Button onClick={handleUpdate} disabled={updateMut.isPending}>
              {updateMut.isPending ? 'Saving...' : 'Save'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Create FQDN */}
      <Dialog open={createFqdnOpen} onOpenChange={setCreateFqdnOpen}>
        <DialogContent>
          <DialogHeader><DialogTitle>Add FQDN</DialogTitle></DialogHeader>
          <div className="space-y-4">
            <div className="space-y-2">
              <Label>Domain Name</Label>
              <Input placeholder="example.com" value={fqdnValue} onChange={(e) => setFqdnValue(e.target.value)} onBlur={() => touch('fqdn')} />
              {fqdnErr && <p className="text-xs text-destructive">{fqdnErr}</p>}
            </div>
            <div className="flex items-center gap-2">
              <Switch checked={fqdnSsl} onCheckedChange={setFqdnSsl} />
              <Label>Enable SSL</Label>
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setCreateFqdnOpen(false)}>Cancel</Button>
            <Button onClick={handleCreateFqdn} disabled={createFqdnMut.isPending}>
              {createFqdnMut.isPending ? 'Adding...' : 'Add'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Delete FQDN */}
      <ConfirmDialog open={!!deleteFqdn} onOpenChange={(o) => !o && setDeleteFqdn(null)} title="Delete FQDN"
        description={`Delete "${deleteFqdn?.fqdn}"? Certificates and email accounts will be removed.`}
        confirmLabel="Delete" variant="destructive" loading={deleteFqdnMut.isPending}
        onConfirm={async () => { try { await deleteFqdnMut.mutateAsync(deleteFqdn!.id); toast.success('FQDN deleted'); setDeleteFqdn(null) } catch (e: unknown) { toast.error(e instanceof Error ? e.message : 'Failed') } }} />
    </div>
  )
}
