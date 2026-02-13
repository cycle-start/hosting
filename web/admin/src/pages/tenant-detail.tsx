import { useParams, useNavigate } from '@tanstack/react-router'
import { type ColumnDef } from '@tanstack/react-table'
import { ArrowLeft, Pause, Play, Trash2 } from 'lucide-react'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Skeleton } from '@/components/ui/skeleton'
import { ResourceHeader } from '@/components/shared/resource-header'
import { DataTable } from '@/components/shared/data-table'
import { StatusBadge } from '@/components/shared/status-badge'
import { CopyButton } from '@/components/shared/copy-button'
import { ConfirmDialog } from '@/components/shared/confirm-dialog'
import { formatDate, truncateID } from '@/lib/utils'
import {
  useTenant, useWebroots, useDatabases, useValkeyInstances,
  useSFTPKeys, useBackups, useSuspendTenant, useUnsuspendTenant,
  useDeleteTenant,
} from '@/lib/hooks'
import type { Webroot, Database, ValkeyInstance, SFTPKey, Backup } from '@/lib/types'
import { useState } from 'react'

const webrootColumns: ColumnDef<Webroot>[] = [
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
    accessorKey: 'runtime',
    header: 'Runtime',
    cell: ({ row }) => `${row.original.runtime} ${row.original.runtime_version}`,
  },
  { accessorKey: 'public_folder', header: 'Public Folder' },
  {
    accessorKey: 'status',
    header: 'Status',
    cell: ({ row }) => <StatusBadge status={row.original.status} />,
  },
]

const databaseColumns: ColumnDef<Database>[] = [
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
]

const valkeyColumns: ColumnDef<ValkeyInstance>[] = [
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
  { accessorKey: 'port', header: 'Port' },
  { accessorKey: 'max_memory_mb', header: 'Max Memory (MB)' },
  {
    accessorKey: 'status',
    header: 'Status',
    cell: ({ row }) => <StatusBadge status={row.original.status} />,
  },
]

const sftpColumns: ColumnDef<SFTPKey>[] = [
  { accessorKey: 'name', header: 'Name' },
  { accessorKey: 'fingerprint', header: 'Fingerprint', cell: ({ row }) => <code className="text-xs">{row.original.fingerprint}</code> },
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
]

const backupColumns: ColumnDef<Backup>[] = [
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
  { accessorKey: 'source_name', header: 'Source' },
  {
    accessorKey: 'size_bytes',
    header: 'Size',
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
    accessorKey: 'status',
    header: 'Status',
    cell: ({ row }) => <StatusBadge status={row.original.status} />,
  },
  {
    accessorKey: 'created_at',
    header: 'Created',
    cell: ({ row }) => <span className="text-sm text-muted-foreground">{formatDate(row.original.created_at)}</span>,
  },
]

export function TenantDetailPage() {
  const { id } = useParams({ from: '/tenants/$id' as never })
  const navigate = useNavigate()
  const [deleteOpen, setDeleteOpen] = useState(false)

  const { data: tenant, isLoading } = useTenant(id)
  const { data: webrootsData, isLoading: webrootsLoading } = useWebroots(id)
  const { data: databasesData, isLoading: databasesLoading } = useDatabases(id)
  const { data: valkeyData, isLoading: valkeyLoading } = useValkeyInstances(id)
  const { data: sftpData, isLoading: sftpLoading } = useSFTPKeys(id)
  const { data: backupsData, isLoading: backupsLoading } = useBackups(id)
  const suspendMutation = useSuspendTenant()
  const unsuspendMutation = useUnsuspendTenant()
  const deleteMutation = useDeleteTenant()

  if (isLoading || !tenant) {
    return (
      <div className="space-y-6">
        <Skeleton className="h-10 w-64" />
        <Skeleton className="h-64 w-full" />
      </div>
    )
  }

  const handleSuspend = async () => {
    try {
      await suspendMutation.mutateAsync(id)
      toast.success('Tenant suspended')
    } catch (e: unknown) {
      toast.error(e instanceof Error ? e.message : 'Failed')
    }
  }

  const handleUnsuspend = async () => {
    try {
      await unsuspendMutation.mutateAsync(id)
      toast.success('Tenant unsuspended')
    } catch (e: unknown) {
      toast.error(e instanceof Error ? e.message : 'Failed')
    }
  }

  const handleDelete = async () => {
    try {
      await deleteMutation.mutateAsync(id)
      toast.success('Tenant deleted')
      navigate({ to: '/tenants' })
    } catch (e: unknown) {
      toast.error(e instanceof Error ? e.message : 'Failed')
    }
  }

  return (
    <div className="space-y-6">
      <Button variant="ghost" size="sm" onClick={() => navigate({ to: '/tenants' })}>
        <ArrowLeft className="mr-2 h-4 w-4" /> Back to Tenants
      </Button>

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
      </div>

      <Tabs defaultValue="webroots">
        <TabsList>
          <TabsTrigger value="webroots">Webroots ({webrootsData?.items?.length ?? 0})</TabsTrigger>
          <TabsTrigger value="databases">Databases ({databasesData?.items?.length ?? 0})</TabsTrigger>
          <TabsTrigger value="valkey">Valkey ({valkeyData?.items?.length ?? 0})</TabsTrigger>
          <TabsTrigger value="sftp">SFTP Keys ({sftpData?.items?.length ?? 0})</TabsTrigger>
          <TabsTrigger value="backups">Backups ({backupsData?.items?.length ?? 0})</TabsTrigger>
        </TabsList>

        <TabsContent value="webroots">
          <DataTable columns={webrootColumns} data={webrootsData?.items ?? []} loading={webrootsLoading} emptyMessage="No webroots" />
        </TabsContent>

        <TabsContent value="databases">
          <DataTable columns={databaseColumns} data={databasesData?.items ?? []} loading={databasesLoading} emptyMessage="No databases" />
        </TabsContent>

        <TabsContent value="valkey">
          <DataTable columns={valkeyColumns} data={valkeyData?.items ?? []} loading={valkeyLoading} emptyMessage="No Valkey instances" />
        </TabsContent>

        <TabsContent value="sftp">
          <DataTable columns={sftpColumns} data={sftpData?.items ?? []} loading={sftpLoading} emptyMessage="No SFTP keys" />
        </TabsContent>

        <TabsContent value="backups">
          <DataTable columns={backupColumns} data={backupsData?.items ?? []} loading={backupsLoading} emptyMessage="No backups" />
        </TabsContent>
      </Tabs>

      <ConfirmDialog
        open={deleteOpen}
        onOpenChange={setDeleteOpen}
        title="Delete Tenant"
        description={`Are you sure you want to delete "${tenant.name}"? All associated resources will be removed.`}
        confirmLabel="Delete"
        variant="destructive"
        onConfirm={handleDelete}
        loading={deleteMutation.isPending}
      />
    </div>
  )
}
