import { useState } from 'react'
import { useParams } from '@tanstack/react-router'
import { type ColumnDef } from '@tanstack/react-table'
import { Plus, Trash2, Key, Copy, Check, ScrollText, HardDrive } from 'lucide-react'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { Skeleton } from '@/components/ui/skeleton'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter, DialogDescription } from '@/components/ui/dialog'
import { ResourceHeader } from '@/components/shared/resource-header'
import { DataTable } from '@/components/shared/data-table'
import { StatusBadge } from '@/components/shared/status-badge'
import { EmptyState } from '@/components/shared/empty-state'
import { ConfirmDialog } from '@/components/shared/confirm-dialog'
import { Breadcrumb } from '@/components/shared/breadcrumb'
import { LogViewer } from '@/components/shared/log-viewer'
import { CopyButton } from '@/components/shared/copy-button'
import { formatDate } from '@/lib/utils'
import {
  useTenant,
  useS3Bucket, useUpdateS3Bucket,
  useS3AccessKeys, useCreateS3AccessKey, useDeleteS3AccessKey,
} from '@/lib/hooks'
import type { S3AccessKey } from '@/lib/types'

const s3Tabs = ['access-keys', 'logs']
function getS3TabFromHash() {
  const hash = window.location.hash.slice(1)
  return s3Tabs.includes(hash) ? hash : 'access-keys'
}

export function S3BucketDetailPage() {
  const { id: tenantId, bucketId } = useParams({ from: '/auth/tenants/$id/s3-buckets/$bucketId' as never })
  const { data: tenant } = useTenant(tenantId)
  const [activeTab, setActiveTab] = useState(getS3TabFromHash)

  const [createOpen, setCreateOpen] = useState(false)
  const [deleteTarget, setDeleteTarget] = useState<S3AccessKey | null>(null)
  const [newKey, setNewKey] = useState<S3AccessKey | null>(null)
  const [permissions, setPermissions] = useState('read-write')
  const [copied, setCopied] = useState<string | null>(null)

  const { data: bucket, isLoading } = useS3Bucket(bucketId)
  const { data: keysData, isLoading: keysLoading } = useS3AccessKeys(bucketId)
  const updateMut = useUpdateS3Bucket()
  const createMut = useCreateS3AccessKey()
  const deleteMut = useDeleteS3AccessKey()

  if (isLoading || !bucket) {
    return <div className="space-y-6"><Skeleton className="h-10 w-64" /><Skeleton className="h-64 w-full" /></div>
  }

  const handleTogglePublic = async () => {
    try {
      await updateMut.mutateAsync({ id: bucket.id, public: !bucket.public })
      toast.success(bucket.public ? 'Bucket set to private' : 'Bucket set to public')
    } catch (e: unknown) { toast.error(e instanceof Error ? e.message : 'Failed') }
  }

  const handleCreateKey = async () => {
    try {
      const key = await createMut.mutateAsync({ bucket_id: bucketId, permissions })
      setCreateOpen(false)
      setNewKey(key)
    } catch (e: unknown) { toast.error(e instanceof Error ? e.message : 'Failed') }
  }

  const handleCopy = async (text: string, field: string) => {
    await navigator.clipboard.writeText(text)
    setCopied(field)
    setTimeout(() => setCopied(null), 2000)
  }

  const formatQuota = (bytes: number) => {
    if (bytes === 0) return 'Unlimited'
    const k = 1024
    const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
    const i = Math.floor(Math.log(bytes) / Math.log(k))
    return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i]
  }

  const columns: ColumnDef<S3AccessKey>[] = [
    { accessorKey: 'access_key_id', header: 'Access Key ID', cell: ({ row }) => <code className="text-xs">{row.original.access_key_id}</code> },
    { accessorKey: 'permissions', header: 'Permissions', cell: ({ row }) => <span className="capitalize">{row.original.permissions}</span> },
    { accessorKey: 'status', header: 'Status', cell: ({ row }) => <StatusBadge status={row.original.status} /> },
    {
      accessorKey: 'created_at', header: 'Created',
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

  return (
    <div className="space-y-6">
      <Breadcrumb segments={[
        { label: 'Tenants', href: '/tenants' },
        { label: tenant?.name ?? tenantId, href: `/tenants/${tenantId}` },
        { label: 'S3 Buckets', href: `/tenants/${tenantId}`, hash: 's3' },
        { label: bucket.name },
      ]} />

      <ResourceHeader
        icon={HardDrive}
        title={bucket.name}
        subtitle={`Shard: ${bucket.shard_name || bucket.shard_id || '-'} | Quota: ${formatQuota(bucket.quota_bytes)}`}
        status={bucket.status}
      />

      <div className="flex items-center gap-2 text-sm text-muted-foreground">
        <span>ID: <code>{bucket.id}</code></span>
        <CopyButton value={bucket.id} />
        <span className="ml-4">Created: {formatDate(bucket.created_at)}</span>
      </div>

      <div className="flex items-center gap-3 rounded-lg border p-4">
        <Switch checked={bucket.public} onCheckedChange={handleTogglePublic} disabled={updateMut.isPending} />
        <div>
          <Label className="text-sm font-medium">Public Read Access</Label>
          <p className="text-xs text-muted-foreground">
            {bucket.public ? 'Anyone can read objects in this bucket via HTTP.' : 'Objects require signed S3 requests to access.'}
          </p>
        </div>
      </div>

      <Tabs value={activeTab} onValueChange={(v) => { setActiveTab(v); window.history.replaceState(null, '', `#${v}`) }}>
        <TabsList>
          <TabsTrigger value="access-keys"><Key className="mr-1.5 h-4 w-4" /> Access Keys ({keysData?.items?.length ?? 0})</TabsTrigger>
          <TabsTrigger value="logs"><ScrollText className="mr-1.5 h-4 w-4" /> Logs</TabsTrigger>
        </TabsList>

        <TabsContent value="access-keys">
          <div className="mb-4 flex justify-end">
            <Button size="sm" onClick={() => { setPermissions('read-write'); setCreateOpen(true) }}>
              <Plus className="mr-2 h-4 w-4" /> Create Access Key
            </Button>
          </div>
          {!keysLoading && (keysData?.items?.length ?? 0) === 0 ? (
            <EmptyState icon={Key} title="No access keys" description="Create an access key to use S3 APIs." action={{ label: 'Create Access Key', onClick: () => setCreateOpen(true) }} />
          ) : (
            <DataTable columns={columns} data={keysData?.items ?? []} loading={keysLoading} searchColumn="access_key_id" searchPlaceholder="Search keys..." />
          )}
        </TabsContent>

        <TabsContent value="logs">
          <LogViewer query={`{app=~"core-api|worker|node-agent"} |= "${bucketId}"`} />
        </TabsContent>
      </Tabs>

      {/* Create Access Key */}
      <Dialog open={createOpen} onOpenChange={setCreateOpen}>
        <DialogContent>
          <DialogHeader><DialogTitle>Create Access Key</DialogTitle></DialogHeader>
          <div className="space-y-4">
            <div className="space-y-2">
              <Label>Permissions</Label>
              <Select value={permissions} onValueChange={setPermissions}>
                <SelectTrigger><SelectValue /></SelectTrigger>
                <SelectContent>
                  <SelectItem value="read-write">Read-Write</SelectItem>
                  <SelectItem value="read-only">Read-Only</SelectItem>
                </SelectContent>
              </Select>
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setCreateOpen(false)}>Cancel</Button>
            <Button onClick={handleCreateKey} disabled={createMut.isPending}>
              {createMut.isPending ? 'Creating...' : 'Create'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Show New Key Credentials */}
      <Dialog open={!!newKey} onOpenChange={(o) => !o && setNewKey(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Access Key Created</DialogTitle>
            <DialogDescription>
              Save these credentials now. The secret access key will not be shown again.
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-4">
            <div className="space-y-2">
              <Label>Access Key ID</Label>
              <div className="flex items-center gap-2">
                <Input readOnly value={newKey?.access_key_id ?? ''} className="font-mono text-sm" />
                <Button variant="outline" size="icon" onClick={() => handleCopy(newKey?.access_key_id ?? '', 'access')}>
                  {copied === 'access' ? <Check className="h-4 w-4" /> : <Copy className="h-4 w-4" />}
                </Button>
              </div>
            </div>
            <div className="space-y-2">
              <Label>Secret Access Key</Label>
              <div className="flex items-center gap-2">
                <Input readOnly value={newKey?.secret_access_key ?? ''} className="font-mono text-sm" />
                <Button variant="outline" size="icon" onClick={() => handleCopy(newKey?.secret_access_key ?? '', 'secret')}>
                  {copied === 'secret' ? <Check className="h-4 w-4" /> : <Copy className="h-4 w-4" />}
                </Button>
              </div>
            </div>
          </div>
          <DialogFooter>
            <Button onClick={() => setNewKey(null)}>Done</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Delete Access Key */}
      <ConfirmDialog open={!!deleteTarget} onOpenChange={(o) => !o && setDeleteTarget(null)} title="Delete Access Key"
        description={`Delete access key "${deleteTarget?.access_key_id}"? Any applications using this key will lose access.`}
        confirmLabel="Delete" variant="destructive" loading={deleteMut.isPending}
        onConfirm={async () => { try { await deleteMut.mutateAsync(deleteTarget!.id); toast.success('Access key deleted'); setDeleteTarget(null) } catch (e: unknown) { toast.error(e instanceof Error ? e.message : 'Failed') } }} />
    </div>
  )
}
