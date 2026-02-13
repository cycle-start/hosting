import { useState } from 'react'
import { useParams, useNavigate } from '@tanstack/react-router'
import { type ColumnDef } from '@tanstack/react-table'
import { Pause, Play, Trash2, Plus, RotateCcw, Loader2, FolderOpen, Database as DatabaseIcon, Globe, Boxes, Key, Archive } from 'lucide-react'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
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
import { rules, validateField } from '@/lib/validation'
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
import type { Webroot, Database, ValkeyInstance, SFTPKey, Backup, Zone } from '@/lib/types'

const runtimes = ['php', 'node', 'python', 'ruby', 'static']

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
  const [wrName, setWrName] = useState('')
  const [wrRuntime, setWrRuntime] = useState('php')
  const [wrVersion, setWrVersion] = useState('8.5')
  const [wrPublicFolder, setWrPublicFolder] = useState('public')
  const [dbName, setDbName] = useState('')
  const [dbShard, setDbShard] = useState('')
  const [vkName, setVkName] = useState('')
  const [vkShard, setVkShard] = useState('')
  const [vkMemory, setVkMemory] = useState('64')
  const [sftpName, setSftpName] = useState('')
  const [sftpKey, setSftpKey] = useState('')
  const [bkType, setBkType] = useState('web')
  const [bkSource, setBkSource] = useState('')
  const [znName, setZnName] = useState('')
  const [formTouched, setFormTouched] = useState<Record<string, boolean>>({})

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

  const touch = (field: string) => setFormTouched(p => ({ ...p, [field]: true }))
  const err = (field: string, value: string, fieldRules: ((v: string) => string | null)[]) =>
    formTouched[field] ? validateField(value, fieldRules) : null

  const wrNameErr = err('wrName', wrName, [rules.required(), rules.slug()])
  const dbNameErr = err('dbName', dbName, [rules.required(), rules.slug()])
  const dbShardErr = err('dbShard', dbShard, [rules.required()])
  const vkNameErr = err('vkName', vkName, [rules.required(), rules.slug()])
  const vkShardErr = err('vkShard', vkShard, [rules.required()])
  const vkMemErr = err('vkMemory', vkMemory, [rules.required(), rules.min(1)])
  const sftpNameErr = err('sftpName', sftpName, [rules.required(), rules.minLength(1), rules.maxLength(255)])
  const sftpKeyErr = err('sftpKey', sftpKey, [rules.required()])
  const znNameErr = err('znName', znName, [rules.required()])

  const resetForm = () => {
    setWrName(''); setWrRuntime('php'); setWrVersion('8.5'); setWrPublicFolder('public')
    setDbName(''); setDbShard('')
    setVkName(''); setVkShard(''); setVkMemory('64')
    setSftpName(''); setSftpKey('')
    setBkType('web'); setBkSource('')
    setZnName('')
    setFormTouched({})
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
    setFormTouched({ wrName: true })
    if (validateField(wrName, [rules.required(), rules.slug()])) return
    try {
      await createWebrootMut.mutateAsync({ tenant_id: id, name: wrName, runtime: wrRuntime, runtime_version: wrVersion, public_folder: wrPublicFolder || undefined })
      toast.success('Webroot created'); setCreateWebrootOpen(false); resetForm()
    } catch (e: unknown) { toast.error(e instanceof Error ? e.message : 'Failed') }
  }

  const handleCreateDb = async () => {
    setFormTouched({ dbName: true, dbShard: true })
    if (validateField(dbName, [rules.required(), rules.slug()]) || validateField(dbShard, [rules.required()])) return
    try {
      await createDbMut.mutateAsync({ tenant_id: id, name: dbName, shard_id: dbShard })
      toast.success('Database created'); setCreateDbOpen(false); resetForm()
    } catch (e: unknown) { toast.error(e instanceof Error ? e.message : 'Failed') }
  }

  const handleCreateValkey = async () => {
    setFormTouched({ vkName: true, vkShard: true, vkMemory: true })
    if (validateField(vkName, [rules.required(), rules.slug()]) || validateField(vkShard, [rules.required()]) || validateField(vkMemory, [rules.required(), rules.min(1)])) return
    try {
      await createValkeyMut.mutateAsync({ tenant_id: id, name: vkName, shard_id: vkShard, max_memory_mb: parseInt(vkMemory) || 64 })
      toast.success('Valkey instance created'); setCreateValkeyOpen(false); resetForm()
    } catch (e: unknown) { toast.error(e instanceof Error ? e.message : 'Failed') }
  }

  const handleCreateSftp = async () => {
    setFormTouched({ sftpName: true, sftpKey: true })
    if (validateField(sftpName, [rules.required(), rules.minLength(1), rules.maxLength(255)]) || validateField(sftpKey, [rules.required()])) return
    try {
      await createSftpMut.mutateAsync({ tenant_id: id, name: sftpName, public_key: sftpKey })
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
    setFormTouched({ znName: true })
    if (validateField(znName, [rules.required()])) return
    try {
      await createZoneMut.mutateAsync({ name: znName, tenant_id: id, region_id: tenant.region_id })
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

  const FieldError = ({ error }: { error: string | null }) =>
    error ? <p className="text-xs text-destructive">{error}</p> : null

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
        <DialogContent>
          <DialogHeader><DialogTitle>Create Webroot</DialogTitle></DialogHeader>
          <div className="space-y-4">
            <div className="space-y-2">
              <Label>Name</Label>
              <Input placeholder="my-site" value={wrName} onChange={(e) => setWrName(e.target.value)} onBlur={() => touch('wrName')} />
              <FieldError error={wrNameErr} />
            </div>
            <div className="grid grid-cols-2 gap-4">
              <div className="space-y-2">
                <Label>Runtime</Label>
                <Select value={wrRuntime} onValueChange={setWrRuntime}>
                  <SelectTrigger><SelectValue /></SelectTrigger>
                  <SelectContent>
                    {runtimes.map(r => <SelectItem key={r} value={r}>{r}</SelectItem>)}
                  </SelectContent>
                </Select>
              </div>
              <div className="space-y-2">
                <Label>Version</Label>
                <Input placeholder="8.5" value={wrVersion} onChange={(e) => setWrVersion(e.target.value)} />
              </div>
            </div>
            <div className="space-y-2">
              <Label>Public Folder</Label>
              <Input placeholder="public" value={wrPublicFolder} onChange={(e) => setWrPublicFolder(e.target.value)} />
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setCreateWebrootOpen(false)}>Cancel</Button>
            <Button onClick={handleCreateWebroot} disabled={createWebrootMut.isPending}>
              {createWebrootMut.isPending ? 'Creating...' : 'Create'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Create Database Dialog */}
      <Dialog open={createDbOpen} onOpenChange={setCreateDbOpen}>
        <DialogContent>
          <DialogHeader><DialogTitle>Create Database</DialogTitle></DialogHeader>
          <div className="space-y-4">
            <div className="space-y-2">
              <Label>Name</Label>
              <Input placeholder="my-database" value={dbName} onChange={(e) => setDbName(e.target.value)} onBlur={() => touch('dbName')} />
              <FieldError error={dbNameErr} />
            </div>
            <div className="space-y-2">
              <Label>Shard ID</Label>
              <Input placeholder="Shard ID for database placement" value={dbShard} onChange={(e) => setDbShard(e.target.value)} onBlur={() => touch('dbShard')} />
              <FieldError error={dbShardErr} />
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setCreateDbOpen(false)}>Cancel</Button>
            <Button onClick={handleCreateDb} disabled={createDbMut.isPending}>
              {createDbMut.isPending ? 'Creating...' : 'Create'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Create Valkey Dialog */}
      <Dialog open={createValkeyOpen} onOpenChange={setCreateValkeyOpen}>
        <DialogContent>
          <DialogHeader><DialogTitle>Create Valkey Instance</DialogTitle></DialogHeader>
          <div className="space-y-4">
            <div className="space-y-2">
              <Label>Name</Label>
              <Input placeholder="my-cache" value={vkName} onChange={(e) => setVkName(e.target.value)} onBlur={() => touch('vkName')} />
              <FieldError error={vkNameErr} />
            </div>
            <div className="space-y-2">
              <Label>Shard ID</Label>
              <Input placeholder="Shard ID for Valkey placement" value={vkShard} onChange={(e) => setVkShard(e.target.value)} onBlur={() => touch('vkShard')} />
              <FieldError error={vkShardErr} />
            </div>
            <div className="space-y-2">
              <Label>Max Memory (MB)</Label>
              <Input type="number" value={vkMemory} onChange={(e) => setVkMemory(e.target.value)} onBlur={() => touch('vkMemory')} />
              <FieldError error={vkMemErr} />
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setCreateValkeyOpen(false)}>Cancel</Button>
            <Button onClick={handleCreateValkey} disabled={createValkeyMut.isPending}>
              {createValkeyMut.isPending ? 'Creating...' : 'Create'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Create SFTP Key Dialog */}
      <Dialog open={createSftpOpen} onOpenChange={setCreateSftpOpen}>
        <DialogContent>
          <DialogHeader><DialogTitle>Add SFTP Key</DialogTitle></DialogHeader>
          <div className="space-y-4">
            <div className="space-y-2">
              <Label>Name</Label>
              <Input placeholder="My laptop key" value={sftpName} onChange={(e) => setSftpName(e.target.value)} onBlur={() => touch('sftpName')} />
              <FieldError error={sftpNameErr} />
            </div>
            <div className="space-y-2">
              <Label>Public Key</Label>
              <Textarea placeholder="ssh-ed25519 AAAA..." rows={4} value={sftpKey} onChange={(e) => setSftpKey(e.target.value)} onBlur={() => touch('sftpKey')} />
              <FieldError error={sftpKeyErr} />
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setCreateSftpOpen(false)}>Cancel</Button>
            <Button onClick={handleCreateSftp} disabled={createSftpMut.isPending}>
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
            <div className="space-y-2">
              <Label>Zone Name</Label>
              <Input placeholder="example.com" value={znName} onChange={(e) => setZnName(e.target.value)} onBlur={() => touch('znName')} />
              <FieldError error={znNameErr} />
            </div>
            <p className="text-xs text-muted-foreground">Region: {tenant.region_id} (inherited from tenant)</p>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setCreateZoneOpen(false)}>Cancel</Button>
            <Button onClick={handleCreateZone} disabled={createZoneMut.isPending}>
              {createZoneMut.isPending ? 'Creating...' : 'Create'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
