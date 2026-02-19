import { useState } from 'react'
import { useParams, useNavigate } from '@tanstack/react-router'
import { type ColumnDef } from '@tanstack/react-table'
import { Plus, Trash2, Pencil, Globe, Play, Pause, RotateCcw, Terminal, Clock, Key, Eye, EyeOff, Lock } from 'lucide-react'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { Skeleton } from '@/components/ui/skeleton'
import { Textarea } from '@/components/ui/textarea'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from '@/components/ui/dialog'
import { ResourceHeader } from '@/components/shared/resource-header'
import { DataTable } from '@/components/shared/data-table'
import { StatusBadge } from '@/components/shared/status-badge'
import { EmptyState } from '@/components/shared/empty-state'
import { ConfirmDialog } from '@/components/shared/confirm-dialog'
import { Breadcrumb } from '@/components/shared/breadcrumb'
import { CopyButton } from '@/components/shared/copy-button'
import { LogViewer } from '@/components/shared/log-viewer'
import { TenantLogViewer } from '@/components/shared/tenant-log-viewer'
import { formatDate } from '@/lib/utils'
import { rules, validateField } from '@/lib/validation'
import {
  useWebroot, useFQDNs, useDaemons, useCronJobs,
  useUpdateWebroot, useCreateFQDN, useDeleteFQDN,
  useCreateDaemon, useUpdateDaemon, useDeleteDaemon, useEnableDaemon, useDisableDaemon, useRetryDaemon,
  useCreateCronJob, useUpdateCronJob, useDeleteCronJob, useEnableCronJob, useDisableCronJob, useRetryCronJob,
  useEnvVars, useSetEnvVars, useDeleteEnvVar,
} from '@/lib/hooks'
import type { FQDN, Daemon, CronJob, WebrootEnvVar } from '@/lib/types'

const runtimes = ['php', 'node', 'python', 'ruby', 'static']
const stopSignals = ['TERM', 'INT', 'QUIT', 'KILL', 'HUP']

export function WebrootDetailPage() {
  const { id: tenantId, webrootId } = useParams({ from: '/auth/tenants/$id/webroots/$webrootId' as never })
  const navigate = useNavigate()

  const [editOpen, setEditOpen] = useState(false)
  const [createFqdnOpen, setCreateFqdnOpen] = useState(false)
  const [deleteFqdn, setDeleteFqdn] = useState<FQDN | null>(null)
  const [touched, setTouched] = useState<Record<string, boolean>>({})

  // Edit form
  const [editRuntime, setEditRuntime] = useState('')
  const [editVersion, setEditVersion] = useState('')
  const [editPublicFolder, setEditPublicFolder] = useState('')
  const [editEnvFileName, setEditEnvFileName] = useState('')
  const [editEnvShellSource, setEditEnvShellSource] = useState(false)
  const [editServiceHostname, setEditServiceHostname] = useState(true)

  // Create FQDN form
  const [fqdnValue, setFqdnValue] = useState('')
  const [fqdnSsl, setFqdnSsl] = useState(true)

  // Daemon state
  const [createDaemonOpen, setCreateDaemonOpen] = useState(false)
  const [editDaemon, setEditDaemon] = useState<Daemon | null>(null)
  const [deleteDaemon, setDeleteDaemon] = useState<Daemon | null>(null)
  const [daemonCommand, setDaemonCommand] = useState('')
  const [daemonProxyPath, setDaemonProxyPath] = useState('')
  const [daemonNumProcs, setDaemonNumProcs] = useState('1')
  const [daemonStopSignal, setDaemonStopSignal] = useState('TERM')
  const [daemonStopWait, setDaemonStopWait] = useState('30')
  const [daemonMaxMem, setDaemonMaxMem] = useState('512')
  const [daemonEnv, setDaemonEnv] = useState('')

  // Cron job state
  const [createCronOpen, setCreateCronOpen] = useState(false)
  const [editCron, setEditCron] = useState<CronJob | null>(null)
  const [deleteCron, setDeleteCron] = useState<CronJob | null>(null)
  const [cronSchedule, setCronSchedule] = useState('')
  const [cronCommand, setCronCommand] = useState('')
  const [cronWorkDir, setCronWorkDir] = useState('')
  const [cronTimeout, setCronTimeout] = useState('300')
  const [cronMaxMem, setCronMaxMem] = useState('512')

  // Env var state
  const [addEnvOpen, setAddEnvOpen] = useState(false)
  const [envName, setEnvName] = useState('')
  const [envValue, setEnvValue] = useState('')
  const [envSecret, setEnvSecret] = useState(false)
  const [deleteEnvVar, setDeleteEnvVar] = useState<WebrootEnvVar | null>(null)
  const [showSecrets, setShowSecrets] = useState<Record<string, boolean>>({})

  const { data: webroot, isLoading } = useWebroot(webrootId)
  const { data: fqdnsData, isLoading: fqdnsLoading } = useFQDNs(webrootId)
  const { data: daemonsData, isLoading: daemonsLoading } = useDaemons(webrootId)
  const { data: cronJobsData, isLoading: cronJobsLoading } = useCronJobs(webrootId)
  const { data: envVarsData } = useEnvVars(webrootId)
  const updateMut = useUpdateWebroot()
  const createFqdnMut = useCreateFQDN()
  const deleteFqdnMut = useDeleteFQDN()
  const createDaemonMut = useCreateDaemon()
  const updateDaemonMut = useUpdateDaemon()
  const deleteDaemonMut = useDeleteDaemon()
  const enableDaemonMut = useEnableDaemon()
  const disableDaemonMut = useDisableDaemon()
  const retryDaemonMut = useRetryDaemon()
  const createCronMut = useCreateCronJob()
  const updateCronMut = useUpdateCronJob()
  const deleteCronMut = useDeleteCronJob()
  const enableCronMut = useEnableCronJob()
  const disableCronMut = useDisableCronJob()
  const retryCronMut = useRetryCronJob()
  const setEnvVarsMut = useSetEnvVars()
  const deleteEnvVarMut = useDeleteEnvVar()

  if (isLoading || !webroot) {
    return <div className="space-y-6"><Skeleton className="h-10 w-64" /><Skeleton className="h-64 w-full" /></div>
  }

  const touch = (f: string) => setTouched(p => ({ ...p, [f]: true }))
  const fqdnErr = touched['fqdn'] ? validateField(fqdnValue, [rules.required(), rules.fqdn()]) : null

  const openEdit = () => {
    setEditRuntime(webroot.runtime)
    setEditVersion(webroot.runtime_version)
    setEditPublicFolder(webroot.public_folder)
    setEditEnvFileName(webroot.env_file_name || '.env.hosting')
    setEditEnvShellSource(webroot.env_shell_source || false)
    setEditServiceHostname(webroot.service_hostname_enabled ?? true)
    setEditOpen(true)
  }

  const handleUpdate = async () => {
    try {
      await updateMut.mutateAsync({ id: webrootId, runtime: editRuntime, runtime_version: editVersion, public_folder: editPublicFolder, env_file_name: editEnvFileName, env_shell_source: editEnvShellSource, service_hostname_enabled: editServiceHostname })
      toast.success('Webroot updated'); setEditOpen(false)
    } catch (e: unknown) { toast.error(e instanceof Error ? e.message : 'Failed') }
  }

  const handleAddEnvVar = async () => {
    setTouched({ envName: true })
    if (!envName.trim()) return
    const existing = envVarsData?.items ?? []
    const newVars = [...existing.filter(v => v.name !== envName).map(v => ({
      name: v.name, value: v.is_secret ? '' : v.value, secret: v.is_secret,
    })), { name: envName, value: envValue, secret: envSecret }]
    try {
      await setEnvVarsMut.mutateAsync({ webroot_id: webrootId, vars: newVars })
      toast.success('Env var added'); setAddEnvOpen(false)
      setEnvName(''); setEnvValue(''); setEnvSecret(false); setTouched({})
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

  // Daemon helpers
  const parseEnvString = (s: string): Record<string, string> => {
    const env: Record<string, string> = {}
    for (const line of s.split('\n')) {
      const trimmed = line.trim()
      if (!trimmed || trimmed.startsWith('#')) continue
      const eqIdx = trimmed.indexOf('=')
      if (eqIdx > 0) env[trimmed.substring(0, eqIdx)] = trimmed.substring(eqIdx + 1)
    }
    return env
  }

  const envToString = (env: Record<string, string>): string =>
    Object.entries(env).map(([k, v]) => `${k}=${v}`).join('\n')

  const resetDaemonForm = () => {
    setDaemonCommand(''); setDaemonProxyPath(''); setDaemonNumProcs('1')
    setDaemonStopSignal('TERM'); setDaemonStopWait('30'); setDaemonMaxMem('512'); setDaemonEnv('')
    setTouched({})
  }

  const openCreateDaemon = () => { resetDaemonForm(); setCreateDaemonOpen(true) }

  const openEditDaemon = (d: Daemon) => {
    setDaemonCommand(d.command)
    setDaemonProxyPath(d.proxy_path ?? '')
    setDaemonNumProcs(String(d.num_procs))
    setDaemonStopSignal(d.stop_signal)
    setDaemonStopWait(String(d.stop_wait_secs))
    setDaemonMaxMem(String(d.max_memory_mb))
    setDaemonEnv(envToString(d.environment))
    setEditDaemon(d)
  }

  const handleCreateDaemon = async () => {
    setTouched({ daemonCmd: true })
    if (!daemonCommand.trim()) return
    try {
      await createDaemonMut.mutateAsync({
        webroot_id: webrootId,
        command: daemonCommand,
        proxy_path: daemonProxyPath || undefined,
        num_procs: parseInt(daemonNumProcs) || 1,
        stop_signal: daemonStopSignal,
        stop_wait_secs: parseInt(daemonStopWait) || 30,
        max_memory_mb: parseInt(daemonMaxMem) || 512,
        environment: parseEnvString(daemonEnv),
      })
      toast.success('Daemon created'); setCreateDaemonOpen(false); resetDaemonForm()
    } catch (e: unknown) { toast.error(e instanceof Error ? e.message : 'Failed') }
  }

  const handleUpdateDaemon = async () => {
    if (!editDaemon || !daemonCommand.trim()) return
    try {
      await updateDaemonMut.mutateAsync({
        id: editDaemon.id,
        command: daemonCommand,
        proxy_path: daemonProxyPath || undefined,
        num_procs: parseInt(daemonNumProcs) || 1,
        stop_signal: daemonStopSignal,
        stop_wait_secs: parseInt(daemonStopWait) || 30,
        max_memory_mb: parseInt(daemonMaxMem) || 512,
        environment: parseEnvString(daemonEnv),
      })
      toast.success('Daemon updated'); setEditDaemon(null)
    } catch (e: unknown) { toast.error(e instanceof Error ? e.message : 'Failed') }
  }

  // Cron job helpers
  const resetCronForm = () => {
    setCronSchedule(''); setCronCommand(''); setCronWorkDir(''); setCronTimeout('300'); setCronMaxMem('512')
    setTouched({})
  }

  const openCreateCron = () => { resetCronForm(); setCreateCronOpen(true) }

  const openEditCron = (c: CronJob) => {
    setCronSchedule(c.schedule)
    setCronCommand(c.command)
    setCronWorkDir(c.working_directory)
    setCronTimeout(String(c.timeout_seconds))
    setCronMaxMem(String(c.max_memory_mb))
    setEditCron(c)
  }

  const handleCreateCron = async () => {
    setTouched({ cronSchedule: true, cronCmd: true })
    if (!cronSchedule.trim() || !cronCommand.trim()) return
    try {
      await createCronMut.mutateAsync({
        webroot_id: webrootId,
        schedule: cronSchedule,
        command: cronCommand,
        working_directory: cronWorkDir || undefined,
        timeout_seconds: parseInt(cronTimeout) || 300,
        max_memory_mb: parseInt(cronMaxMem) || 512,
      })
      toast.success('Cron job created'); setCreateCronOpen(false); resetCronForm()
    } catch (e: unknown) { toast.error(e instanceof Error ? e.message : 'Failed') }
  }

  const handleUpdateCron = async () => {
    if (!editCron || !cronSchedule.trim() || !cronCommand.trim()) return
    try {
      await updateCronMut.mutateAsync({
        id: editCron.id,
        schedule: cronSchedule,
        command: cronCommand,
        working_directory: cronWorkDir || undefined,
        timeout_seconds: parseInt(cronTimeout) || 300,
        max_memory_mb: parseInt(cronMaxMem) || 512,
      })
      toast.success('Cron job updated'); setEditCron(null)
    } catch (e: unknown) { toast.error(e instanceof Error ? e.message : 'Failed') }
  }

  const fqdnColumns: ColumnDef<FQDN>[] = [
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

  const daemonColumns: ColumnDef<Daemon>[] = [
    {
      accessorKey: 'name', header: 'Name',
      cell: ({ row }) => <span className="font-mono text-sm">{row.original.name}</span>,
    },
    {
      accessorKey: 'command', header: 'Command',
      cell: ({ row }) => <span className="font-mono text-xs truncate max-w-[200px] block">{row.original.command}</span>,
    },
    {
      accessorKey: 'proxy_path', header: 'Proxy Path',
      cell: ({ row }) => row.original.proxy_path
        ? <span className="font-mono text-sm">{row.original.proxy_path} :{row.original.proxy_port}</span>
        : <span className="text-muted-foreground">-</span>,
    },
    {
      accessorKey: 'enabled', header: 'Enabled',
      cell: ({ row }) => row.original.enabled
        ? <span className="text-green-600">Yes</span>
        : <span className="text-muted-foreground">No</span>,
    },
    {
      accessorKey: 'status', header: 'Status',
      cell: ({ row }) => <StatusBadge status={row.original.status} />,
    },
    {
      id: 'actions',
      cell: ({ row }) => {
        const d = row.original
        return (
          <div className="flex gap-1" onClick={(e) => e.stopPropagation()}>
            {d.status === 'failed' && (
              <Button variant="ghost" size="icon" title="Retry" onClick={() => retryDaemonMut.mutateAsync(d.id).then(() => toast.success('Retrying')).catch(() => toast.error('Failed'))}>
                <RotateCcw className="h-4 w-4" />
              </Button>
            )}
            {d.status === 'active' && d.enabled && (
              <Button variant="ghost" size="icon" title="Disable" onClick={() => disableDaemonMut.mutateAsync(d.id).then(() => toast.success('Disabling')).catch(() => toast.error('Failed'))}>
                <Pause className="h-4 w-4" />
              </Button>
            )}
            {d.status === 'active' && !d.enabled && (
              <Button variant="ghost" size="icon" title="Enable" onClick={() => enableDaemonMut.mutateAsync(d.id).then(() => toast.success('Enabling')).catch(() => toast.error('Failed'))}>
                <Play className="h-4 w-4" />
              </Button>
            )}
            <Button variant="ghost" size="icon" onClick={() => openEditDaemon(d)}>
              <Pencil className="h-4 w-4" />
            </Button>
            <Button variant="ghost" size="icon" onClick={() => setDeleteDaemon(d)}>
              <Trash2 className="h-4 w-4 text-destructive" />
            </Button>
          </div>
        )
      },
    },
  ]

  const cronColumns: ColumnDef<CronJob>[] = [
    {
      accessorKey: 'name', header: 'Name',
      cell: ({ row }) => <span className="font-mono text-sm">{row.original.name}</span>,
    },
    {
      accessorKey: 'schedule', header: 'Schedule',
      cell: ({ row }) => <span className="font-mono text-xs">{row.original.schedule}</span>,
    },
    {
      accessorKey: 'command', header: 'Command',
      cell: ({ row }) => <span className="font-mono text-xs truncate max-w-[200px] block">{row.original.command}</span>,
    },
    {
      accessorKey: 'enabled', header: 'Enabled',
      cell: ({ row }) => row.original.enabled
        ? <span className="text-green-600">Yes</span>
        : <span className="text-muted-foreground">No</span>,
    },
    {
      accessorKey: 'status', header: 'Status',
      cell: ({ row }) => <StatusBadge status={row.original.status} />,
    },
    {
      id: 'actions',
      cell: ({ row }) => {
        const c = row.original
        return (
          <div className="flex gap-1" onClick={(e) => e.stopPropagation()}>
            {c.status === 'failed' && (
              <Button variant="ghost" size="icon" title="Retry" onClick={() => retryCronMut.mutateAsync(c.id).then(() => toast.success('Retrying')).catch(() => toast.error('Failed'))}>
                <RotateCcw className="h-4 w-4" />
              </Button>
            )}
            {c.status === 'active' && c.enabled && (
              <Button variant="ghost" size="icon" title="Disable" onClick={() => disableCronMut.mutateAsync(c.id).then(() => toast.success('Disabling')).catch(() => toast.error('Failed'))}>
                <Pause className="h-4 w-4" />
              </Button>
            )}
            {c.status === 'active' && !c.enabled && (
              <Button variant="ghost" size="icon" title="Enable" onClick={() => enableCronMut.mutateAsync(c.id).then(() => toast.success('Enabling')).catch(() => toast.error('Failed'))}>
                <Play className="h-4 w-4" />
              </Button>
            )}
            <Button variant="ghost" size="icon" onClick={() => openEditCron(c)}>
              <Pencil className="h-4 w-4" />
            </Button>
            <Button variant="ghost" size="icon" onClick={() => setDeleteCron(c)}>
              <Trash2 className="h-4 w-4 text-destructive" />
            </Button>
          </div>
        )
      },
    },
  ]

  const daemonFormContent = (
    <div className="space-y-4">
      <div className="space-y-2">
        <Label>Command *</Label>
        <Input placeholder="php artisan reverb:start --port=$PORT" value={daemonCommand} onChange={(e) => setDaemonCommand(e.target.value)} onBlur={() => touch('daemonCmd')} />
        {touched['daemonCmd'] && !daemonCommand.trim() && <p className="text-xs text-destructive">Required</p>}
      </div>
      <div className="space-y-2">
        <Label>Proxy Path</Label>
        <Input placeholder="/ws (optional, enables nginx proxy)" value={daemonProxyPath} onChange={(e) => setDaemonProxyPath(e.target.value)} />
        <p className="text-xs text-muted-foreground">When set, nginx proxies this path to the daemon. $PORT env var is injected automatically.</p>
      </div>
      <div className="grid grid-cols-2 gap-4">
        <div className="space-y-2">
          <Label>Num Procs</Label>
          <Input type="number" min="1" max="8" value={daemonNumProcs} onChange={(e) => setDaemonNumProcs(e.target.value)} />
        </div>
        <div className="space-y-2">
          <Label>Stop Signal</Label>
          <Select value={daemonStopSignal} onValueChange={setDaemonStopSignal}>
            <SelectTrigger><SelectValue /></SelectTrigger>
            <SelectContent>
              {stopSignals.map(s => <SelectItem key={s} value={s}>{s}</SelectItem>)}
            </SelectContent>
          </Select>
        </div>
      </div>
      <div className="grid grid-cols-2 gap-4">
        <div className="space-y-2">
          <Label>Stop Wait (seconds)</Label>
          <Input type="number" min="1" max="300" value={daemonStopWait} onChange={(e) => setDaemonStopWait(e.target.value)} />
        </div>
        <div className="space-y-2">
          <Label>Max Memory (MB)</Label>
          <Input type="number" min="16" max="4096" value={daemonMaxMem} onChange={(e) => setDaemonMaxMem(e.target.value)} />
        </div>
      </div>
      <div className="space-y-2">
        <Label>Environment</Label>
        <Textarea placeholder="KEY=value (one per line)" value={daemonEnv} onChange={(e) => setDaemonEnv(e.target.value)} rows={3} className="font-mono text-sm" />
      </div>
    </div>
  )

  const cronFormContent = (
    <div className="space-y-4">
      <div className="space-y-2">
        <Label>Schedule *</Label>
        <Input placeholder="*/5 * * * *" value={cronSchedule} onChange={(e) => setCronSchedule(e.target.value)} onBlur={() => touch('cronSchedule')} className="font-mono" />
        {touched['cronSchedule'] && !cronSchedule.trim() && <p className="text-xs text-destructive">Required</p>}
        <p className="text-xs text-muted-foreground">Cron expression (minute hour day month weekday)</p>
      </div>
      <div className="space-y-2">
        <Label>Command *</Label>
        <Input placeholder="php artisan schedule:run" value={cronCommand} onChange={(e) => setCronCommand(e.target.value)} onBlur={() => touch('cronCmd')} />
        {touched['cronCmd'] && !cronCommand.trim() && <p className="text-xs text-destructive">Required</p>}
      </div>
      <div className="space-y-2">
        <Label>Working Directory</Label>
        <Input placeholder="(defaults to webroot root)" value={cronWorkDir} onChange={(e) => setCronWorkDir(e.target.value)} />
      </div>
      <div className="grid grid-cols-2 gap-4">
        <div className="space-y-2">
          <Label>Timeout (seconds)</Label>
          <Input type="number" min="1" max="86400" value={cronTimeout} onChange={(e) => setCronTimeout(e.target.value)} />
        </div>
        <div className="space-y-2">
          <Label>Max Memory (MB)</Label>
          <Input type="number" min="16" max="4096" value={cronMaxMem} onChange={(e) => setCronMaxMem(e.target.value)} />
        </div>
      </div>
    </div>
  )

  return (
    <div className="space-y-6">
      <Breadcrumb segments={[
        { label: 'Tenants', href: '/tenants' },
        { label: tenantId, href: `/tenants/${tenantId}` },
        { label: 'Webroots', href: `/tenants/${tenantId}`, hash: 'webroots' },
        { label: webroot.name },
      ]} />

      <ResourceHeader
        title={webroot.name}
        subtitle={`${webroot.runtime} ${webroot.runtime_version} | Public: ${webroot.public_folder} | Env: ${webroot.env_file_name || '.env.hosting'}${webroot.env_shell_source ? ' (shell-sourced)' : ''} | Service hostname: ${webroot.service_hostname_enabled ? 'on' : 'off'}`}
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

      {/* FQDNs */}
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
          <DataTable columns={fqdnColumns} data={fqdnsData?.items ?? []} loading={fqdnsLoading} searchColumn="fqdn" searchPlaceholder="Search FQDNs..."
            onRowClick={(f) => navigate({ to: '/tenants/$id/fqdns/$fqdnId', params: { id: tenantId, fqdnId: f.id } })} />
        )}
      </div>

      {/* Daemons */}
      <div>
        <div className="mb-4 flex items-center justify-between">
          <h2 className="text-lg font-semibold">Daemons</h2>
          <Button size="sm" onClick={openCreateDaemon}>
            <Plus className="mr-2 h-4 w-4" /> Add Daemon
          </Button>
        </div>
        {!daemonsLoading && (daemonsData?.items?.length ?? 0) === 0 ? (
          <EmptyState icon={Terminal} title="No Daemons" description="Add a long-running process (WebSocket server, queue worker, etc.)." action={{ label: 'Add Daemon', onClick: openCreateDaemon }} />
        ) : (
          <DataTable columns={daemonColumns} data={daemonsData?.items ?? []} loading={daemonsLoading} searchColumn="name" searchPlaceholder="Search daemons..." />
        )}
      </div>

      {/* Cron Jobs */}
      <div>
        <div className="mb-4 flex items-center justify-between">
          <h2 className="text-lg font-semibold">Cron Jobs</h2>
          <Button size="sm" onClick={openCreateCron}>
            <Plus className="mr-2 h-4 w-4" /> Add Cron Job
          </Button>
        </div>
        {!cronJobsLoading && (cronJobsData?.items?.length ?? 0) === 0 ? (
          <EmptyState icon={Clock} title="No Cron Jobs" description="Schedule recurring tasks for this webroot." action={{ label: 'Add Cron Job', onClick: openCreateCron }} />
        ) : (
          <DataTable columns={cronColumns} data={cronJobsData?.items ?? []} loading={cronJobsLoading} searchColumn="name" searchPlaceholder="Search cron jobs..." />
        )}
      </div>

      {/* Environment Variables */}
      <div>
        <div className="mb-4 flex items-center justify-between">
          <h2 className="text-lg font-semibold">Environment Variables</h2>
          <Button size="sm" onClick={() => { setEnvName(''); setEnvValue(''); setEnvSecret(false); setTouched({}); setAddEnvOpen(true) }}>
            <Plus className="mr-2 h-4 w-4" /> Add Variable
          </Button>
        </div>
        {(envVarsData?.items?.length ?? 0) === 0 ? (
          <EmptyState icon={Key} title="No Environment Variables" description="Add env vars for this webroot. Secrets are encrypted at rest." action={{ label: 'Add Variable', onClick: () => setAddEnvOpen(true) }} />
        ) : (
          <div className="rounded-md border">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b bg-muted/50">
                  <th className="px-4 py-2 text-left font-medium">Name</th>
                  <th className="px-4 py-2 text-left font-medium">Value</th>
                  <th className="px-4 py-2 text-left font-medium w-20">Secret</th>
                  <th className="px-4 py-2 w-16"></th>
                </tr>
              </thead>
              <tbody>
                {envVarsData?.items?.map((v) => (
                  <tr key={v.name} className="border-b last:border-0">
                    <td className="px-4 py-2 font-mono text-sm">{v.name}</td>
                    <td className="px-4 py-2 font-mono text-xs">
                      {v.is_secret ? (
                        <span className="flex items-center gap-1 text-muted-foreground">
                          <Lock className="h-3 w-3" /> ***
                        </span>
                      ) : (
                        <span className="truncate max-w-[300px] block">{v.value}</span>
                      )}
                    </td>
                    <td className="px-4 py-2">
                      {v.is_secret ? <span className="text-amber-600">Yes</span> : <span className="text-muted-foreground">No</span>}
                    </td>
                    <td className="px-4 py-2">
                      <Button variant="ghost" size="icon" onClick={() => setDeleteEnvVar(v)}>
                        <Trash2 className="h-4 w-4 text-destructive" />
                      </Button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>

      {/* Logs */}
      <TenantLogViewer tenantId={tenantId} webrootId={webrootId} title="Access Logs" />
      <LogViewer query={`{app=~"core-api|worker|node-agent"} |= "${webrootId}"`} title="Platform Logs" />

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
            <div className="grid grid-cols-2 gap-4">
              <div className="space-y-2">
                <Label>Env File Name</Label>
                <Input placeholder=".env.hosting" value={editEnvFileName} onChange={(e) => setEditEnvFileName(e.target.value)} />
                <p className="text-xs text-muted-foreground">Filename for the env file in the webroot directory</p>
              </div>
              <div className="space-y-2 flex flex-col justify-center pt-4">
                <div className="flex items-center gap-2">
                  <Switch checked={editEnvShellSource} onCheckedChange={setEditEnvShellSource} />
                  <Label>Auto-source in SSH</Label>
                </div>
                <p className="text-xs text-muted-foreground">Source env file in SSH shell sessions via .bashrc</p>
              </div>
            </div>
            <div className="space-y-2 flex flex-col justify-center pt-4">
              <div className="flex items-center gap-2">
                <Switch checked={editServiceHostname} onCheckedChange={setEditServiceHostname} />
                <Label>Service Hostname</Label>
              </div>
              <p className="text-xs text-muted-foreground">Enable auto-generated {'{webroot}.{tenant}.{brand}'} hostname</p>
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

      {/* Create Daemon */}
      <Dialog open={createDaemonOpen} onOpenChange={setCreateDaemonOpen}>
        <DialogContent className="max-w-lg">
          <DialogHeader><DialogTitle>Add Daemon</DialogTitle></DialogHeader>
          {daemonFormContent}
          <DialogFooter>
            <Button variant="outline" onClick={() => setCreateDaemonOpen(false)}>Cancel</Button>
            <Button onClick={handleCreateDaemon} disabled={createDaemonMut.isPending}>
              {createDaemonMut.isPending ? 'Creating...' : 'Create'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Edit Daemon */}
      <Dialog open={!!editDaemon} onOpenChange={(o) => !o && setEditDaemon(null)}>
        <DialogContent className="max-w-lg">
          <DialogHeader><DialogTitle>Edit Daemon</DialogTitle></DialogHeader>
          {daemonFormContent}
          <DialogFooter>
            <Button variant="outline" onClick={() => setEditDaemon(null)}>Cancel</Button>
            <Button onClick={handleUpdateDaemon} disabled={updateDaemonMut.isPending}>
              {updateDaemonMut.isPending ? 'Saving...' : 'Save'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Create Cron Job */}
      <Dialog open={createCronOpen} onOpenChange={setCreateCronOpen}>
        <DialogContent className="max-w-lg">
          <DialogHeader><DialogTitle>Add Cron Job</DialogTitle></DialogHeader>
          {cronFormContent}
          <DialogFooter>
            <Button variant="outline" onClick={() => setCreateCronOpen(false)}>Cancel</Button>
            <Button onClick={handleCreateCron} disabled={createCronMut.isPending}>
              {createCronMut.isPending ? 'Creating...' : 'Create'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Edit Cron Job */}
      <Dialog open={!!editCron} onOpenChange={(o) => !o && setEditCron(null)}>
        <DialogContent className="max-w-lg">
          <DialogHeader><DialogTitle>Edit Cron Job</DialogTitle></DialogHeader>
          {cronFormContent}
          <DialogFooter>
            <Button variant="outline" onClick={() => setEditCron(null)}>Cancel</Button>
            <Button onClick={handleUpdateCron} disabled={updateCronMut.isPending}>
              {updateCronMut.isPending ? 'Saving...' : 'Save'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Delete FQDN */}
      <ConfirmDialog open={!!deleteFqdn} onOpenChange={(o) => !o && setDeleteFqdn(null)} title="Delete FQDN"
        description={`Delete "${deleteFqdn?.fqdn}"? Certificates and email accounts will be removed.`}
        confirmLabel="Delete" variant="destructive" loading={deleteFqdnMut.isPending}
        onConfirm={async () => { try { await deleteFqdnMut.mutateAsync(deleteFqdn!.id); toast.success('FQDN deleted'); setDeleteFqdn(null) } catch (e: unknown) { toast.error(e instanceof Error ? e.message : 'Failed') } }} />

      {/* Delete Daemon */}
      <ConfirmDialog open={!!deleteDaemon} onOpenChange={(o) => !o && setDeleteDaemon(null)} title="Delete Daemon"
        description={`Delete daemon "${deleteDaemon?.name}"? The process will be stopped and removed from all nodes.`}
        confirmLabel="Delete" variant="destructive" loading={deleteDaemonMut.isPending}
        onConfirm={async () => { try { await deleteDaemonMut.mutateAsync(deleteDaemon!.id); toast.success('Daemon deleted'); setDeleteDaemon(null) } catch (e: unknown) { toast.error(e instanceof Error ? e.message : 'Failed') } }} />

      {/* Delete Cron Job */}
      <ConfirmDialog open={!!deleteCron} onOpenChange={(o) => !o && setDeleteCron(null)} title="Delete Cron Job"
        description={`Delete cron job "${deleteCron?.name}"? The timer will be stopped and removed from all nodes.`}
        confirmLabel="Delete" variant="destructive" loading={deleteCronMut.isPending}
        onConfirm={async () => { try { await deleteCronMut.mutateAsync(deleteCron!.id); toast.success('Cron job deleted'); setDeleteCron(null) } catch (e: unknown) { toast.error(e instanceof Error ? e.message : 'Failed') } }} />

      {/* Add Env Var */}
      <Dialog open={addEnvOpen} onOpenChange={setAddEnvOpen}>
        <DialogContent>
          <DialogHeader><DialogTitle>Add Environment Variable</DialogTitle></DialogHeader>
          <div className="space-y-4">
            <div className="space-y-2">
              <Label>Name *</Label>
              <Input placeholder="DATABASE_URL" value={envName} onChange={(e) => setEnvName(e.target.value.toUpperCase())} onBlur={() => touch('envName')} className="font-mono" />
              {touched['envName'] && !envName.trim() && <p className="text-xs text-destructive">Required</p>}
              <p className="text-xs text-muted-foreground">Letters, digits, underscores. Must start with letter or underscore.</p>
            </div>
            <div className="space-y-2">
              <Label>Value</Label>
              <Input placeholder="value" value={envValue} onChange={(e) => setEnvValue(e.target.value)} className="font-mono" type={envSecret ? 'password' : 'text'} />
            </div>
            <div className="flex items-center gap-2">
              <Switch checked={envSecret} onCheckedChange={setEnvSecret} />
              <Label>Secret (encrypted at rest)</Label>
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setAddEnvOpen(false)}>Cancel</Button>
            <Button onClick={handleAddEnvVar} disabled={setEnvVarsMut.isPending}>
              {setEnvVarsMut.isPending ? 'Adding...' : 'Add'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Delete Env Var */}
      <ConfirmDialog open={!!deleteEnvVar} onOpenChange={(o) => !o && setDeleteEnvVar(null)} title="Delete Environment Variable"
        description={`Delete "${deleteEnvVar?.name}"? This will trigger a re-convergence.`}
        confirmLabel="Delete" variant="destructive" loading={deleteEnvVarMut.isPending}
        onConfirm={async () => { try { await deleteEnvVarMut.mutateAsync({ webroot_id: webrootId, name: deleteEnvVar!.name }); toast.success('Env var deleted'); setDeleteEnvVar(null) } catch (e: unknown) { toast.error(e instanceof Error ? e.message : 'Failed') } }} />
    </div>
  )
}
