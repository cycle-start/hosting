import { useState } from 'react'
import { useParams, useNavigate } from '@tanstack/react-router'
import { type ColumnDef } from '@tanstack/react-table'
import { Pause, Play, Trash2, Plus, RotateCcw, Loader2, FolderOpen, Database as DatabaseIcon, Globe, Boxes, Key, Archive } from 'lucide-react'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Skeleton } from '@/components/ui/skeleton'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from '@/components/ui/dialog'
import { ResourceHeader } from '@/components/shared/resource-header'
import { DataTable } from '@/components/shared/data-table'
import { StatusBadge } from '@/components/shared/status-badge'
import { CopyButton } from '@/components/shared/copy-button'
import { EmptyState } from '@/components/shared/empty-state'
import { ConfirmDialog } from '@/components/shared/confirm-dialog'
import { Breadcrumb } from '@/components/shared/breadcrumb'
import { formatDate } from '@/lib/utils'
import {
  useTenant, useTenantResourceSummary, useWebroots, useDatabases, useValkeyInstances,
  useSFTPKeys, useBackups, useZones, useSuspendTenant, useUnsuspendTenant,
  useDeleteTenant, useCreateWebroot, useDeleteWebroot,
  useCreateDatabase, useDeleteDatabase,
  useCreateValkeyInstance, useDeleteValkeyInstance,
  useCreateSFTPKey, useDeleteSFTPKey,
  useCreateBackup, useDeleteBackup, useRestoreBackup,
  useCreateZone, useDeleteZone,
} from '@/lib/hooks'
import type { Webroot, Database, ValkeyInstance, SFTPKey, Backup, Zone, WebrootFormData, DatabaseFormData, ValkeyInstanceFormData, SFTPKeyFormData, ZoneFormData } from '@/lib/types'
import { WebrootFields } from '@/components/forms/webroot-fields'
import { DatabaseFields } from '@/components/forms/database-fields'
import { ValkeyInstanceFields } from '@/components/forms/valkey-instance-fields'
import { SFTPKeyFields } from '@/components/forms/sftp-key-fields'
import { ZoneFields } from '@/components/forms/zone-fields'

export function TenantDetailPage() {
  const { id } = useParams({ from: '/tenants/$id' as never })
  const navigate = useNavigate()
  const [deleteOpen, setDeleteOpen] = useState(false)

  // Create dialogs
  const [createWebrootOpen, setCreateWebrootOpen] = useState(false)
  const [createDbOpen, setCreateDbOpen] = useState(false)
  const [createValkeyOpen, setCreateValkeyOpen] = useState(false)
  const [createSftpOpen, setCreateSftpOpen] = useState(false)
  const [createBackupOpen, setCreateBackupOpen] = useState(false)
  const [createZoneOpen, setCreateZoneOpen] = useState(false)

  // Delete targets
  const [deleteWebroot, setDeleteWebroot] = useState<Webroot | null>(null)
  const [deleteDb, setDeleteDb] = useState<Database | null>(null)
  const [deleteValkey, setDeleteValkey] = useState<ValkeyInstance | null>(null)
  const [deleteSftp, setDeleteSftp] = useState<SFTPKey | null>(null)
  const [deleteBackupTarget, setDeleteBackupTarget] = useState<Backup | null>(null)
  const [restoreBackupTarget, setRestoreBackupTarget] = useState<Backup | null>(null)
  const [deleteZoneTarget, setDeleteZoneTarget] = useState<Zone | null>(null)

  // Form state
  const defaultWebroot: WebrootFormData = { name: '', runtime: 'php', runtime_version: '8.5', public_folder: 'public' }
  const defaultDatabase: DatabaseFormData = { name: '', shard_id: '' }
  const defaultValkey: ValkeyInstanceFormData = { name: '', shard_id: '', max_memory_mb: 64 }
  const defaultSftp: SFTPKeyFormData = { name: '', public_key: '' }
  const defaultZone: ZoneFormData = { name: '' }

  const [wrForm, setWrForm] = useState<WebrootFormData>(defaultWebroot)
  const [dbForm, setDbForm] = useState<DatabaseFormData>(defaultDatabase)
  const [vkForm, setVkForm] = useState<ValkeyInstanceFormData>(defaultValkey)
  const [sftpForm, setSftpForm] = useState<SFTPKeyFormData>(defaultSftp)
  const [znForm, setZnForm] = useState<ZoneFormData>(defaultZone)
  const [bkType, setBkType] = useState('web')
  const [bkSource, setBkSource] = useState('')

  const { data: tenant, isLoading } = useTenant(id)
  const { data: summary } = useTenantResourceSummary(id)
  const { data: webrootsData, isLoading: webrootsLoading } = useWebroots(id)
  const { data: databasesData, isLoading: databasesLoading } = useDatabases(id)
  const { data: valkeyData, isLoading: valkeyLoading } = useValkeyInstances(id)
  const { data: sftpData, isLoading: sftpLoading } = useSFTPKeys(id)
  const { data: backupsData, isLoading: backupsLoading } = useBackups(id)
  const { data: zonesData, isLoading: zonesLoading } = useZones()

  const suspendMutation = useSuspendTenant()
  const unsuspendMutation = useUnsuspendTenant()
  const deleteMutation = useDeleteTenant()
  const createWebrootMut = useCreateWebroot()
  const deleteWebrootMut = useDeleteWebroot()
  const createDbMut = useCreateDatabase()
  const deleteDbMut = useDeleteDatabase()
  const createValkeyMut = useCreateValkeyInstance()
  const deleteValkeyMut = useDeleteValkeyInstance()
  const createSftpMut = useCreateSFTPKey()
  const deleteSftpMut = useDeleteSFTPKey()
  const createBackupMut = useCreateBackup()
  const deleteBackupMut = useDeleteBackup()
  const restoreBackupMut = useRestoreBackup()
  const createZoneMut = useCreateZone()
  const deleteZoneMut = useDeleteZone()

  if (isLoading || !tenant) {
    return (
      <div className="space-y-6">
        <Skeleton className="h-10 w-64" />
        <Skeleton className="h-64 w-full" />
      </div>
    )
  }

  const tenantZones = (zonesData?.items ?? []).filter(z => z.tenant_id === id)

  const inFlight = (summary?.pending ?? 0) + (summary?.provisioning ?? 0)
  const hasFailed = (summary?.failed ?? 0) > 0

  // Backup source options
  const backupSources = bkType === 'web'
    ? (webrootsData?.items ?? []).map(w => ({ id: w.id, name: w.name }))
    : (databasesData?.items ?? []).map(d => ({ id: d.id, name: d.name }))

  const resetForm = () => {
    setWrForm(defaultWebroot)
    setDbForm(defaultDatabase)
    setVkForm(defaultValkey)
    setSftpForm(defaultSftp)
    setZnForm(defaultZone)
    setBkType('web'); setBkSource('')
  }

  const handleSuspend = async () => {
    try { await suspendMutation.mutateAsync(id); toast.success('Tenant suspended') }
    catch (e: unknown) { toast.error(e instanceof Error ? e.message : 'Failed') }
  }
  const handleUnsuspend = async () => {
    try { await unsuspendMutation.mutateAsync(id); toast.success('Tenant unsuspended') }
    catch (e: unknown) { toast.error(e instanceof Error ? e.message : 'Failed') }
  }
  const handleDelete = async () => {
    try { await deleteMutation.mutateAsync(id); toast.success('Tenant deleted'); navigate({ to: '/tenants' }) }
    catch (e: unknown) { toast.error(e instanceof Error ? e.message : 'Failed') }
  }

  const handleCreateWebroot = async () => {
    if (!wrForm.name || !wrForm.runtime || !wrForm.runtime_version) return
    try {
      await createWebrootMut.mutateAsync({
        tenant_id: id, name: wrForm.name, runtime: wrForm.runtime,
        runtime_version: wrForm.runtime_version, public_folder: wrForm.public_folder || undefined,
        fqdns: wrForm.fqdns?.length ? wrForm.fqdns : undefined,
      })
      toast.success('Webroot created'); setCreateWebrootOpen(false); resetForm()
    } catch (e: unknown) { toast.error(e instanceof Error ? e.message : 'Failed') }
  }

  const handleCreateDb = async () => {
    if (!dbForm.name || !dbForm.shard_id) return
    try {
      await createDbMut.mutateAsync({
        tenant_id: id, name: dbForm.name, shard_id: dbForm.shard_id,
        users: dbForm.users?.length ? dbForm.users : undefined,
      })
      toast.success('Database created'); setCreateDbOpen(false); resetForm()
    } catch (e: unknown) { toast.error(e instanceof Error ? e.message : 'Failed') }
  }

  const handleCreateValkey = async () => {
    if (!vkForm.name || !vkForm.shard_id) return
    try {
      await createValkeyMut.mutateAsync({
        tenant_id: id, name: vkForm.name, shard_id: vkForm.shard_id,
        max_memory_mb: vkForm.max_memory_mb || 64,
        users: vkForm.users?.length ? vkForm.users : undefined,
      })
      toast.success('Valkey instance created'); setCreateValkeyOpen(false); resetForm()
    } catch (e: unknown) { toast.error(e instanceof Error ? e.message : 'Failed') }
  }

  const handleCreateSftp = async () => {
    if (!sftpForm.name || !sftpForm.public_key) return
    try {
      await createSftpMut.mutateAsync({ tenant_id: id, name: sftpForm.name, public_key: sftpForm.public_key })
      toast.success('SFTP key created'); setCreateSftpOpen(false); resetForm()
    } catch (e: unknown) { toast.error(e instanceof Error ? e.message : 'Failed') }
  }

  const handleCreateBackup = async () => {
    if (!bkSource) return
    try {
      await createBackupMut.mutateAsync({ tenant_id: id, type: bkType, source_id: bkSource })
      toast.success('Backup started'); setCreateBackupOpen(false); resetForm()
    } catch (e: unknown) { toast.error(e instanceof Error ? e.message : 'Failed') }
  }

  const handleCreateZone = async () => {
    if (!znForm.name) return
    try {
      await createZoneMut.mutateAsync({ name: znForm.name, tenant_id: id, region_id: tenant.region_id })
      toast.success('Zone created'); setCreateZoneOpen(false); resetForm()
    } catch (e: unknown) { toast.error(e instanceof Error ? e.message : 'Failed') }
  }

  const webrootColumns: ColumnDef<Webroot>[] = [
    {
      accessorKey: 'name', header: 'Name',
      cell: ({ row }) => <span className="font-medium">{row.original.name}</span>,
    },
    {
      accessorKey: 'runtime', header: 'Runtime',
      cell: ({ row }) => `${row.original.runtime} ${row.original.runtime_version}`,
    },
    { accessorKey: 'public_folder', header: 'Public Folder' },
    {
      accessorKey: 'status', header: 'Status',
      cell: ({ row }) => <StatusBadge status={row.original.status} />,
    },
    {
      id: 'actions',
      cell: ({ row }) => (
        <Button variant="ghost" size="icon" onClick={(e) => { e.stopPropagation(); setDeleteWebroot(row.original) }}>
          <Trash2 className="h-4 w-4 text-destructive" />
        </Button>
      ),
    },
  ]

  const databaseColumns: ColumnDef<Database>[] = [
    {
      accessorKey: 'name', header: 'Name',
      cell: ({ row }) => <span className="font-medium">{row.original.name}</span>,
    },
    {
      accessorKey: 'shard_id', header: 'Shard',
      cell: ({ row }) => row.original.shard_id ? <code className="text-xs">{row.original.shard_id}</code> : '-',
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
        <Button variant="ghost" size="icon" onClick={(e) => { e.stopPropagation(); setDeleteDb(row.original) }}>
          <Trash2 className="h-4 w-4 text-destructive" />
        </Button>
      ),
    },
  ]

  const valkeyColumns: ColumnDef<ValkeyInstance>[] = [
    {
      accessorKey: 'name', header: 'Name',
      cell: ({ row }) => <span className="font-medium">{row.original.name}</span>,
    },
    { accessorKey: 'port', header: 'Port' },
    { accessorKey: 'max_memory_mb', header: 'Max Memory (MB)' },
    {
      accessorKey: 'status', header: 'Status',
      cell: ({ row }) => <StatusBadge status={row.original.status} />,
    },
    {
      id: 'actions',
      cell: ({ row }) => (
        <Button variant="ghost" size="icon" onClick={(e) => { e.stopPropagation(); setDeleteValkey(row.original) }}>
          <Trash2 className="h-4 w-4 text-destructive" />
        </Button>
      ),
    },
  ]

  const zoneColumns: ColumnDef<Zone>[] = [
    {
      accessorKey: 'name', header: 'Name',
      cell: ({ row }) => <span className="font-medium">{row.original.name}</span>,
    },
    { accessorKey: 'region_id', header: 'Region' },
    {
      accessorKey: 'status', header: 'Status',
      cell: ({ row }) => <StatusBadge status={row.original.status} />,
    },
    {
      id: 'actions',
      cell: ({ row }) => (
        <Button variant="ghost" size="icon" onClick={(e) => { e.stopPropagation(); setDeleteZoneTarget(row.original) }}>
          <Trash2 className="h-4 w-4 text-destructive" />
        </Button>
      ),
    },
  ]

  const sftpColumns: ColumnDef<SFTPKey>[] = [
    { accessorKey: 'name', header: 'Name', cell: ({ row }) => <span className="font-medium">{row.original.name}</span> },
    { accessorKey: 'fingerprint', header: 'Fingerprint', cell: ({ row }) => <code className="text-xs">{row.original.fingerprint}</code> },
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
        <Button variant="ghost" size="icon" onClick={(e) => { e.stopPropagation(); setDeleteSftp(row.original) }}>
          <Trash2 className="h-4 w-4 text-destructive" />
        </Button>
      ),
    },
  ]

  const backupColumns: ColumnDef<Backup>[] = [
    { accessorKey: 'type', header: 'Type', cell: ({ row }) => <span className="capitalize">{row.original.type}</span> },
    { accessorKey: 'source_name', header: 'Source' },
    {
      accessorKey: 'size_bytes', header: 'Size',
      cell: ({ row }) => {
        const bytes = row.original.size_bytes
        if (bytes === 0) return '0 B'
        const k = 1024
        const sizes = ['B', 'KB', 'MB', 'GB']
        const i = Math.floor(Math.log(bytes) / Math.log(k))
        return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i]
      },
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
        <div className="flex gap-1">
          {row.original.status === 'completed' && (
            <Button variant="ghost" size="icon" title="Restore" onClick={(e) => { e.stopPropagation(); setRestoreBackupTarget(row.original) }}>
              <RotateCcw className="h-4 w-4" />
            </Button>
          )}
          <Button variant="ghost" size="icon" onClick={(e) => { e.stopPropagation(); setDeleteBackupTarget(row.original) }}>
            <Trash2 className="h-4 w-4 text-destructive" />
          </Button>
        </div>
      ),
    },
  ]

  return (
    <div className="space-y-6">
      <Breadcrumb segments={[
        { label: 'Tenants', href: '/tenants' },
        { label: tenant.name },
      ]} />

      <ResourceHeader
        title={tenant.name}
        subtitle={`UID: ${tenant.uid} | Region: ${tenant.region_id} | Cluster: ${tenant.cluster_id}`}
        status={tenant.status}
        actions={
          <div className="flex gap-2">
            {tenant.status === 'active' && (
              <Button variant="outline" size="sm" onClick={handleSuspend}>
                <Pause className="mr-2 h-4 w-4" /> Suspend
              </Button>
            )}
            {tenant.status === 'suspended' && (
              <Button variant="outline" size="sm" onClick={handleUnsuspend}>
                <Play className="mr-2 h-4 w-4" /> Unsuspend
              </Button>
            )}
            <Button variant="destructive" size="sm" onClick={() => setDeleteOpen(true)}>
              <Trash2 className="mr-2 h-4 w-4" /> Delete
            </Button>
          </div>
        }
      />

      <div className="flex items-center gap-2 text-sm text-muted-foreground">
        <span>ID: <code>{tenant.id}</code></span>
        <CopyButton value={tenant.id} />
        <span className="ml-4">Created: {formatDate(tenant.created_at)}</span>
        <span className="ml-4">SFTP: {tenant.sftp_enabled ? 'Enabled' : 'Disabled'}</span>
      </div>

      {inFlight > 0 && (
        <div className="rounded-lg border border-blue-200 bg-blue-50 dark:border-blue-900 dark:bg-blue-950 p-3">
          <div className="flex items-center gap-2 text-sm font-medium text-blue-700 dark:text-blue-300">
            <Loader2 className="h-4 w-4 animate-spin" />
            {inFlight} resource{inFlight !== 1 ? 's' : ''} provisioning
          </div>
          {summary && (
            <div className="mt-1 text-xs text-blue-600 dark:text-blue-400">
              {summary.pending > 0 && `${summary.pending} pending`}
              {summary.pending > 0 && summary.provisioning > 0 && ', '}
              {summary.provisioning > 0 && `${summary.provisioning} in progress`}
            </div>
          )}
        </div>
      )}

      {hasFailed && (
        <div className="rounded-lg border border-red-200 bg-red-50 dark:border-red-900 dark:bg-red-950 p-3">
          <div className="text-sm font-medium text-red-700 dark:text-red-300">
            {summary!.failed} resource{summary!.failed !== 1 ? 's' : ''} failed
          </div>
        </div>
      )}

      <Tabs defaultValue="webroots">
        <TabsList>
          <TabsTrigger value="webroots">Webroots ({webrootsData?.items?.length ?? 0})</TabsTrigger>
          <TabsTrigger value="databases">Databases ({databasesData?.items?.length ?? 0})</TabsTrigger>
          <TabsTrigger value="zones">Zones ({tenantZones.length})</TabsTrigger>
          <TabsTrigger value="valkey">Valkey ({valkeyData?.items?.length ?? 0})</TabsTrigger>
          <TabsTrigger value="sftp">SFTP Keys ({sftpData?.items?.length ?? 0})</TabsTrigger>
          <TabsTrigger value="backups">Backups ({backupsData?.items?.length ?? 0})</TabsTrigger>
        </TabsList>

        <TabsContent value="webroots">
          <div className="mb-4 flex justify-end">
            <Button size="sm" onClick={() => { resetForm(); setCreateWebrootOpen(true) }}>
              <Plus className="mr-2 h-4 w-4" /> Create Webroot
            </Button>
          </div>
          {!webrootsLoading && (webrootsData?.items?.length ?? 0) === 0 ? (
            <EmptyState icon={FolderOpen} title="No webroots" description="Create a webroot to host a website." action={{ label: 'Create Webroot', onClick: () => { resetForm(); setCreateWebrootOpen(true) } }} />
          ) : (
            <DataTable columns={webrootColumns} data={webrootsData?.items ?? []} loading={webrootsLoading} searchColumn="name" searchPlaceholder="Search webroots..."
              onRowClick={(w) => navigate({ to: '/tenants/$id/webroots/$webrootId', params: { id, webrootId: w.id } })} />
          )}
        </TabsContent>

        <TabsContent value="databases">
          <div className="mb-4 flex justify-end">
            <Button size="sm" onClick={() => { resetForm(); setCreateDbOpen(true) }}>
              <Plus className="mr-2 h-4 w-4" /> Create Database
            </Button>
          </div>
          {!databasesLoading && (databasesData?.items?.length ?? 0) === 0 ? (
            <EmptyState icon={DatabaseIcon} title="No databases" description="Create a MySQL database for this tenant." action={{ label: 'Create Database', onClick: () => { resetForm(); setCreateDbOpen(true) } }} />
          ) : (
            <DataTable columns={databaseColumns} data={databasesData?.items ?? []} loading={databasesLoading} searchColumn="name" searchPlaceholder="Search databases..."
              onRowClick={(d) => navigate({ to: '/tenants/$id/databases/$databaseId', params: { id, databaseId: d.id } })} />
          )}
        </TabsContent>

        <TabsContent value="zones">
          <div className="mb-4 flex justify-end">
            <Button size="sm" onClick={() => { resetForm(); setCreateZoneOpen(true) }}>
              <Plus className="mr-2 h-4 w-4" /> Create Zone
            </Button>
          </div>
          {!zonesLoading && tenantZones.length === 0 ? (
            <EmptyState icon={Globe} title="No zones" description="Create a DNS zone for this tenant." action={{ label: 'Create Zone', onClick: () => { resetForm(); setCreateZoneOpen(true) } }} />
          ) : (
            <DataTable columns={zoneColumns} data={tenantZones} loading={zonesLoading} searchColumn="name" searchPlaceholder="Search zones..."
              onRowClick={(z) => navigate({ to: '/zones/$id', params: { id: z.id } })} />
          )}
        </TabsContent>

        <TabsContent value="valkey">
          <div className="mb-4 flex justify-end">
            <Button size="sm" onClick={() => { resetForm(); setCreateValkeyOpen(true) }}>
              <Plus className="mr-2 h-4 w-4" /> Create Valkey Instance
            </Button>
          </div>
          {!valkeyLoading && (valkeyData?.items?.length ?? 0) === 0 ? (
            <EmptyState icon={Boxes} title="No Valkey instances" description="Create a managed Valkey (Redis) instance." action={{ label: 'Create Instance', onClick: () => { resetForm(); setCreateValkeyOpen(true) } }} />
          ) : (
            <DataTable columns={valkeyColumns} data={valkeyData?.items ?? []} loading={valkeyLoading} searchColumn="name" searchPlaceholder="Search instances..."
              onRowClick={(v) => navigate({ to: '/tenants/$id/valkey/$instanceId', params: { id, instanceId: v.id } })} />
          )}
        </TabsContent>

        <TabsContent value="sftp">
          <div className="mb-4 flex justify-end">
            <Button size="sm" onClick={() => { resetForm(); setCreateSftpOpen(true) }}>
              <Plus className="mr-2 h-4 w-4" /> Add SFTP Key
            </Button>
          </div>
          {!sftpLoading && (sftpData?.items?.length ?? 0) === 0 ? (
            <EmptyState icon={Key} title="No SFTP keys" description="Add an SSH public key for SFTP access." action={{ label: 'Add Key', onClick: () => { resetForm(); setCreateSftpOpen(true) } }} />
          ) : (
            <DataTable columns={sftpColumns} data={sftpData?.items ?? []} loading={sftpLoading} emptyMessage="No SFTP keys" />
          )}
        </TabsContent>

        <TabsContent value="backups">
          <div className="mb-4 flex justify-end">
            <Button size="sm" onClick={() => { resetForm(); setCreateBackupOpen(true) }}>
              <Plus className="mr-2 h-4 w-4" /> Create Backup
            </Button>
          </div>
          {!backupsLoading && (backupsData?.items?.length ?? 0) === 0 ? (
            <EmptyState icon={Archive} title="No backups" description="Create a backup of a webroot or database." action={{ label: 'Create Backup', onClick: () => { resetForm(); setCreateBackupOpen(true) } }} />
          ) : (
            <DataTable columns={backupColumns} data={backupsData?.items ?? []} loading={backupsLoading} emptyMessage="No backups" />
          )}
        </TabsContent>
      </Tabs>

      {/* Delete Tenant */}
      <ConfirmDialog open={deleteOpen} onOpenChange={setDeleteOpen} title="Delete Tenant"
        description={`Are you sure you want to delete "${tenant.name}"? All associated resources will be removed.`}
        confirmLabel="Delete" variant="destructive" onConfirm={handleDelete} loading={deleteMutation.isPending} />

      {/* Delete Webroot */}
      <ConfirmDialog open={!!deleteWebroot} onOpenChange={(o) => !o && setDeleteWebroot(null)} title="Delete Webroot"
        description={`Delete webroot "${deleteWebroot?.name}"? This will remove all FQDNs and configurations.`}
        confirmLabel="Delete" variant="destructive" loading={deleteWebrootMut.isPending}
        onConfirm={async () => { try { await deleteWebrootMut.mutateAsync(deleteWebroot!.id); toast.success('Webroot deleted'); setDeleteWebroot(null) } catch (e: unknown) { toast.error(e instanceof Error ? e.message : 'Failed') } }} />

      {/* Delete Database */}
      <ConfirmDialog open={!!deleteDb} onOpenChange={(o) => !o && setDeleteDb(null)} title="Delete Database"
        description={`Delete database "${deleteDb?.name}"? All data will be lost.`}
        confirmLabel="Delete" variant="destructive" loading={deleteDbMut.isPending}
        onConfirm={async () => { try { await deleteDbMut.mutateAsync(deleteDb!.id); toast.success('Database deleted'); setDeleteDb(null) } catch (e: unknown) { toast.error(e instanceof Error ? e.message : 'Failed') } }} />

      {/* Delete Valkey */}
      <ConfirmDialog open={!!deleteValkey} onOpenChange={(o) => !o && setDeleteValkey(null)} title="Delete Valkey Instance"
        description={`Delete Valkey instance "${deleteValkey?.name}"? All data will be lost.`}
        confirmLabel="Delete" variant="destructive" loading={deleteValkeyMut.isPending}
        onConfirm={async () => { try { await deleteValkeyMut.mutateAsync(deleteValkey!.id); toast.success('Valkey instance deleted'); setDeleteValkey(null) } catch (e: unknown) { toast.error(e instanceof Error ? e.message : 'Failed') } }} />

      {/* Delete SFTP Key */}
      <ConfirmDialog open={!!deleteSftp} onOpenChange={(o) => !o && setDeleteSftp(null)} title="Delete SFTP Key"
        description={`Delete SFTP key "${deleteSftp?.name}"?`}
        confirmLabel="Delete" variant="destructive" loading={deleteSftpMut.isPending}
        onConfirm={async () => { try { await deleteSftpMut.mutateAsync(deleteSftp!.id); toast.success('SFTP key deleted'); setDeleteSftp(null) } catch (e: unknown) { toast.error(e instanceof Error ? e.message : 'Failed') } }} />

      {/* Delete Backup */}
      <ConfirmDialog open={!!deleteBackupTarget} onOpenChange={(o) => !o && setDeleteBackupTarget(null)} title="Delete Backup"
        description={`Delete this ${deleteBackupTarget?.type} backup of "${deleteBackupTarget?.source_name}"?`}
        confirmLabel="Delete" variant="destructive" loading={deleteBackupMut.isPending}
        onConfirm={async () => { try { await deleteBackupMut.mutateAsync(deleteBackupTarget!.id); toast.success('Backup deleted'); setDeleteBackupTarget(null) } catch (e: unknown) { toast.error(e instanceof Error ? e.message : 'Failed') } }} />

      {/* Restore Backup */}
      <ConfirmDialog open={!!restoreBackupTarget} onOpenChange={(o) => !o && setRestoreBackupTarget(null)} title="Restore Backup"
        description={`Restore ${restoreBackupTarget?.type} backup of "${restoreBackupTarget?.source_name}"? This will overwrite current data.`}
        confirmLabel="Restore" loading={restoreBackupMut.isPending}
        onConfirm={async () => { try { await restoreBackupMut.mutateAsync(restoreBackupTarget!.id); toast.success('Restore started'); setRestoreBackupTarget(null) } catch (e: unknown) { toast.error(e instanceof Error ? e.message : 'Failed') } }} />

      {/* Delete Zone */}
      <ConfirmDialog open={!!deleteZoneTarget} onOpenChange={(o) => !o && setDeleteZoneTarget(null)} title="Delete Zone"
        description={`Delete zone "${deleteZoneTarget?.name}"? All DNS records will be removed.`}
        confirmLabel="Delete" variant="destructive" loading={deleteZoneMut.isPending}
        onConfirm={async () => { try { await deleteZoneMut.mutateAsync(deleteZoneTarget!.id); toast.success('Zone deleted'); setDeleteZoneTarget(null) } catch (e: unknown) { toast.error(e instanceof Error ? e.message : 'Failed') } }} />

      {/* Create Webroot Dialog */}
      <Dialog open={createWebrootOpen} onOpenChange={setCreateWebrootOpen}>
        <DialogContent className="max-w-2xl max-h-[80vh] overflow-y-auto">
          <DialogHeader><DialogTitle>Create Webroot</DialogTitle></DialogHeader>
          <WebrootFields value={wrForm} onChange={setWrForm} />
          <DialogFooter>
            <Button variant="outline" onClick={() => setCreateWebrootOpen(false)}>Cancel</Button>
            <Button onClick={handleCreateWebroot} disabled={createWebrootMut.isPending || !wrForm.name || !wrForm.runtime}>
              {createWebrootMut.isPending ? 'Creating...' : 'Create'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Create Database Dialog */}
      <Dialog open={createDbOpen} onOpenChange={setCreateDbOpen}>
        <DialogContent className="max-w-2xl max-h-[80vh] overflow-y-auto">
          <DialogHeader><DialogTitle>Create Database</DialogTitle></DialogHeader>
          <DatabaseFields value={dbForm} onChange={setDbForm} clusterId={tenant.cluster_id} />
          <DialogFooter>
            <Button variant="outline" onClick={() => setCreateDbOpen(false)}>Cancel</Button>
            <Button onClick={handleCreateDb} disabled={createDbMut.isPending || !dbForm.name || !dbForm.shard_id}>
              {createDbMut.isPending ? 'Creating...' : 'Create'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Create Valkey Dialog */}
      <Dialog open={createValkeyOpen} onOpenChange={setCreateValkeyOpen}>
        <DialogContent className="max-w-2xl max-h-[80vh] overflow-y-auto">
          <DialogHeader><DialogTitle>Create Valkey Instance</DialogTitle></DialogHeader>
          <ValkeyInstanceFields value={vkForm} onChange={setVkForm} clusterId={tenant.cluster_id} />
          <DialogFooter>
            <Button variant="outline" onClick={() => setCreateValkeyOpen(false)}>Cancel</Button>
            <Button onClick={handleCreateValkey} disabled={createValkeyMut.isPending || !vkForm.name || !vkForm.shard_id}>
              {createValkeyMut.isPending ? 'Creating...' : 'Create'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Create SFTP Key Dialog */}
      <Dialog open={createSftpOpen} onOpenChange={setCreateSftpOpen}>
        <DialogContent>
          <DialogHeader><DialogTitle>Add SFTP Key</DialogTitle></DialogHeader>
          <SFTPKeyFields value={sftpForm} onChange={setSftpForm} />
          <DialogFooter>
            <Button variant="outline" onClick={() => setCreateSftpOpen(false)}>Cancel</Button>
            <Button onClick={handleCreateSftp} disabled={createSftpMut.isPending || !sftpForm.name || !sftpForm.public_key}>
              {createSftpMut.isPending ? 'Adding...' : 'Add Key'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Create Backup Dialog */}
      <Dialog open={createBackupOpen} onOpenChange={setCreateBackupOpen}>
        <DialogContent>
          <DialogHeader><DialogTitle>Create Backup</DialogTitle></DialogHeader>
          <div className="space-y-4">
            <div className="space-y-2">
              <Label>Type</Label>
              <Select value={bkType} onValueChange={(v) => { setBkType(v); setBkSource('') }}>
                <SelectTrigger><SelectValue /></SelectTrigger>
                <SelectContent>
                  <SelectItem value="web">Web (Webroot)</SelectItem>
                  <SelectItem value="database">Database</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-2">
              <Label>Source</Label>
              {backupSources.length === 0 ? (
                <p className="text-sm text-muted-foreground">No {bkType === 'web' ? 'webroots' : 'databases'} available.</p>
              ) : (
                <Select value={bkSource} onValueChange={setBkSource}>
                  <SelectTrigger><SelectValue placeholder="Select source..." /></SelectTrigger>
                  <SelectContent>
                    {backupSources.map(s => <SelectItem key={s.id} value={s.id}>{s.name}</SelectItem>)}
                  </SelectContent>
                </Select>
              )}
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setCreateBackupOpen(false)}>Cancel</Button>
            <Button onClick={handleCreateBackup} disabled={createBackupMut.isPending || !bkSource}>
              {createBackupMut.isPending ? 'Creating...' : 'Create Backup'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Create Zone Dialog */}
      <Dialog open={createZoneOpen} onOpenChange={setCreateZoneOpen}>
        <DialogContent>
          <DialogHeader><DialogTitle>Create Zone</DialogTitle></DialogHeader>
          <div className="space-y-4">
            <ZoneFields value={znForm} onChange={setZnForm} />
            <p className="text-xs text-muted-foreground">Region: {tenant.region_id} (inherited from tenant)</p>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setCreateZoneOpen(false)}>Cancel</Button>
            <Button onClick={handleCreateZone} disabled={createZoneMut.isPending || !znForm.name}>
              {createZoneMut.isPending ? 'Creating...' : 'Create'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
