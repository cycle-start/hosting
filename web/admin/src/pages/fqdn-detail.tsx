import { useState } from 'react'
import { useParams, useNavigate } from '@tanstack/react-router'
import { type ColumnDef } from '@tanstack/react-table'
import { Plus, Trash2, Shield, Mail, ScrollText, Globe } from 'lucide-react'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Skeleton } from '@/components/ui/skeleton'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from '@/components/ui/dialog'
import { ResourceHeader } from '@/components/shared/resource-header'
import { DataTable } from '@/components/shared/data-table'
import { StatusBadge } from '@/components/shared/status-badge'
import { EmptyState } from '@/components/shared/empty-state'
import { ConfirmDialog } from '@/components/shared/confirm-dialog'
import { Breadcrumb } from '@/components/shared/breadcrumb'
import { LogViewer } from '@/components/shared/log-viewer'
import { TenantLogViewer } from '@/components/shared/tenant-log-viewer'
import { CopyButton } from '@/components/shared/copy-button'
import { formatDate } from '@/lib/utils'
import { rules, validateField } from '@/lib/validation'
import { Switch } from '@/components/ui/switch'
import {
  useTenant, useWebroot,
  useFQDN, useCertificates, useEmailAccounts,
  useUploadCertificate, useCreateEmailAccount, useDeleteEmailAccount,
  useUpdateFQDN,
} from '@/lib/hooks'
import type { Certificate, EmailAccount } from '@/lib/types'

const fqdnTabs = ['certificates', 'email', 'access-logs', 'platform-logs']
function getFQDNTabFromHash() {
  const hash = window.location.hash.slice(1)
  return fqdnTabs.includes(hash) ? hash : 'certificates'
}

export function FQDNDetailPage() {
  const { id: tenantId, fqdnId } = useParams({ from: '/auth/tenants/$id/fqdns/$fqdnId' as never })
  const { data: tenant } = useTenant(tenantId)
  const navigate = useNavigate()
  const [activeTab, setActiveTab] = useState(getFQDNTabFromHash)

  const [uploadCertOpen, setUploadCertOpen] = useState(false)
  const [createEmailOpen, setCreateEmailOpen] = useState(false)
  const [deleteEmail, setDeleteEmail] = useState<EmailAccount | null>(null)
  const [touched, setTouched] = useState<Record<string, boolean>>({})

  // Cert form
  const [certPem, setCertPem] = useState('')
  const [keyPem, setKeyPem] = useState('')
  const [chainPem, setChainPem] = useState('')

  // Email form
  const [emailAddr, setEmailAddr] = useState('')
  const [emailDisplay, setEmailDisplay] = useState('')
  const [emailQuota, setEmailQuota] = useState('')

  const { data: fqdn, isLoading } = useFQDN(fqdnId)
  const { data: webroot } = useWebroot(fqdn?.webroot_id ?? '')
  const { data: certsData, isLoading: certsLoading } = useCertificates(fqdnId)
  const { data: emailsData, isLoading: emailsLoading } = useEmailAccounts(fqdnId)
  const uploadCertMut = useUploadCertificate()
  const createEmailMut = useCreateEmailAccount()
  const deleteEmailMut = useDeleteEmailAccount()
  const updateFqdnMut = useUpdateFQDN()

  if (isLoading || !fqdn) {
    return <div className="space-y-6"><Skeleton className="h-10 w-64" /><Skeleton className="h-64 w-full" /></div>
  }

  const touch = (f: string) => setTouched(p => ({ ...p, [f]: true }))
  const emailErr = touched['email'] ? validateField(emailAddr, [rules.required(), rules.email()]) : null
  const certErr = touched['certPem'] ? validateField(certPem, [rules.required()]) : null
  const keyErr = touched['keyPem'] ? validateField(keyPem, [rules.required()]) : null

  const handleUploadCert = async () => {
    setTouched({ certPem: true, keyPem: true })
    if (validateField(certPem, [rules.required()]) || validateField(keyPem, [rules.required()])) return
    try {
      await uploadCertMut.mutateAsync({ fqdn_id: fqdnId, cert_pem: certPem, key_pem: keyPem, chain_pem: chainPem || undefined })
      toast.success('Certificate uploaded'); setUploadCertOpen(false); setCertPem(''); setKeyPem(''); setChainPem(''); setTouched({})
    } catch (e: unknown) { toast.error(e instanceof Error ? e.message : 'Failed') }
  }

  const handleCreateEmail = async () => {
    setTouched({ email: true })
    if (validateField(emailAddr, [rules.required(), rules.email()])) return
    try {
      await createEmailMut.mutateAsync({ fqdn_id: fqdnId, subscription_id: '', address: emailAddr, display_name: emailDisplay || undefined, quota_bytes: emailQuota ? parseInt(emailQuota) : undefined })
      toast.success('Email account created'); setCreateEmailOpen(false); setEmailAddr(''); setEmailDisplay(''); setEmailQuota(''); setTouched({})
    } catch (e: unknown) { toast.error(e instanceof Error ? e.message : 'Failed') }
  }

  const certColumns: ColumnDef<Certificate>[] = [
    { accessorKey: 'type', header: 'Type' },
    {
      accessorKey: 'is_active', header: 'Active',
      cell: ({ row }) => row.original.is_active ? <span className="text-green-600">Yes</span> : <span className="text-muted-foreground">No</span>,
    },
    {
      accessorKey: 'expires_at', header: 'Expires',
      cell: ({ row }) => row.original.expires_at ? formatDate(row.original.expires_at) : '-',
    },
    {
      accessorKey: 'status', header: 'Status',
      cell: ({ row }) => <StatusBadge status={row.original.status} />,
    },
  ]

  const emailColumns: ColumnDef<EmailAccount>[] = [
    {
      accessorKey: 'address', header: 'Address',
      cell: ({ row }) => <span className="font-medium">{row.original.address}</span>,
    },
    { accessorKey: 'display_name', header: 'Display Name' },
    {
      accessorKey: 'quota_bytes', header: 'Quota',
      cell: ({ row }) => {
        const mb = Math.round(row.original.quota_bytes / 1024 / 1024)
        return mb > 0 ? `${mb} MB` : '-'
      },
    },
    {
      accessorKey: 'status', header: 'Status',
      cell: ({ row }) => <StatusBadge status={row.original.status} />,
    },
    {
      id: 'actions',
      cell: ({ row }) => (
        <Button variant="ghost" size="icon" onClick={(e) => { e.stopPropagation(); setDeleteEmail(row.original) }}>
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
        { label: 'Webroots', href: `/tenants/${tenantId}`, hash: 'webroots' },
        { label: webroot?.name ?? fqdn.webroot_id ?? 'Webroot', href: `/tenants/${tenantId}/webroots/${fqdn.webroot_id}`, hash: 'fqdns' },
        { label: fqdn.fqdn },
      ]} />

      <ResourceHeader
        icon={Globe}
        title={fqdn.fqdn}
        subtitle={`Webroot: ${fqdn.webroot_id}`}
        status={fqdn.status}
      />

      <div className="flex items-center gap-2 text-sm text-muted-foreground">
        <span>ID: <code>{fqdn.id}</code></span>
        <CopyButton value={fqdn.id} />
        <span className="ml-4">Created: {formatDate(fqdn.created_at)}</span>
        <span className="ml-4 flex items-center gap-2">
          <Switch
            checked={fqdn.ssl_enabled}
            disabled={updateFqdnMut.isPending}
            onCheckedChange={(checked) =>
              updateFqdnMut.mutateAsync({ id: fqdnId, ssl_enabled: checked })
                .then(() => toast.success(checked ? 'SSL enabled' : 'SSL disabled'))
                .catch((e: unknown) => toast.error(e instanceof Error ? e.message : 'Failed to update SSL'))
            }
          />
          <span>SSL {fqdn.ssl_enabled ? 'Enabled' : 'Disabled'}</span>
        </span>
      </div>

      <Tabs value={activeTab} onValueChange={(v) => { setActiveTab(v); window.history.replaceState(null, '', `#${v}`) }}>
        <TabsList>
          <TabsTrigger value="certificates">Certificates ({certsData?.items?.length ?? 0})</TabsTrigger>
          <TabsTrigger value="email">Email Accounts ({emailsData?.items?.length ?? 0})</TabsTrigger>
          <TabsTrigger value="access-logs"><ScrollText className="mr-1.5 h-4 w-4" /> Access Logs</TabsTrigger>
          <TabsTrigger value="platform-logs"><ScrollText className="mr-1.5 h-4 w-4" /> Platform Logs</TabsTrigger>
        </TabsList>

        <TabsContent value="certificates">
          <div className="mb-4 flex justify-end">
            <Button size="sm" onClick={() => { setCertPem(''); setKeyPem(''); setChainPem(''); setTouched({}); setUploadCertOpen(true) }}>
              <Plus className="mr-2 h-4 w-4" /> Upload Certificate
            </Button>
          </div>
          {!certsLoading && (certsData?.items?.length ?? 0) === 0 ? (
            <EmptyState icon={Shield} title="No certificates" description="Upload an SSL certificate for this FQDN." action={{ label: 'Upload Certificate', onClick: () => setUploadCertOpen(true) }} />
          ) : (
            <DataTable columns={certColumns} data={certsData?.items ?? []} loading={certsLoading} emptyMessage="No certificates" />
          )}
        </TabsContent>

        <TabsContent value="email">
          <div className="mb-4 flex justify-end">
            <Button size="sm" onClick={() => { setEmailAddr(''); setEmailDisplay(''); setEmailQuota(''); setTouched({}); setCreateEmailOpen(true) }}>
              <Plus className="mr-2 h-4 w-4" /> Create Email Account
            </Button>
          </div>
          {!emailsLoading && (emailsData?.items?.length ?? 0) === 0 ? (
            <EmptyState icon={Mail} title="No email accounts" description="Create an email account on this domain." action={{ label: 'Create Account', onClick: () => setCreateEmailOpen(true) }} />
          ) : (
            <DataTable columns={emailColumns} data={emailsData?.items ?? []} loading={emailsLoading} searchColumn="address" searchPlaceholder="Search accounts..."
              onRowClick={(a) => navigate({ to: '/tenants/$id/email-accounts/$accountId', params: { id: tenantId, accountId: a.id } })} />
          )}
        </TabsContent>

        <TabsContent value="access-logs">
          <TenantLogViewer tenantId={tenantId} webrootId={fqdn.webroot_id ?? undefined} />
        </TabsContent>

        <TabsContent value="platform-logs">
          <LogViewer query={`{app=~"core-api|worker|node-agent"} |= "${fqdnId}"`} />
        </TabsContent>
      </Tabs>

      {/* Upload Certificate */}
      <Dialog open={uploadCertOpen} onOpenChange={setUploadCertOpen}>
        <DialogContent className="max-w-lg">
          <DialogHeader><DialogTitle>Upload Certificate</DialogTitle></DialogHeader>
          <div className="space-y-4">
            <div className="space-y-2">
              <Label>Certificate (PEM)</Label>
              <Textarea rows={4} placeholder="-----BEGIN CERTIFICATE-----" value={certPem} onChange={(e) => setCertPem(e.target.value)} onBlur={() => touch('certPem')} />
              {certErr && <p className="text-xs text-destructive">{certErr}</p>}
            </div>
            <div className="space-y-2">
              <Label>Private Key (PEM)</Label>
              <Textarea rows={4} placeholder="-----BEGIN PRIVATE KEY-----" value={keyPem} onChange={(e) => setKeyPem(e.target.value)} onBlur={() => touch('keyPem')} />
              {keyErr && <p className="text-xs text-destructive">{keyErr}</p>}
            </div>
            <div className="space-y-2">
              <Label>Chain (PEM, optional)</Label>
              <Textarea rows={3} placeholder="-----BEGIN CERTIFICATE-----" value={chainPem} onChange={(e) => setChainPem(e.target.value)} />
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setUploadCertOpen(false)}>Cancel</Button>
            <Button onClick={handleUploadCert} disabled={uploadCertMut.isPending}>
              {uploadCertMut.isPending ? 'Uploading...' : 'Upload'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Create Email Account */}
      <Dialog open={createEmailOpen} onOpenChange={setCreateEmailOpen}>
        <DialogContent>
          <DialogHeader><DialogTitle>Create Email Account</DialogTitle></DialogHeader>
          <div className="space-y-4">
            <div className="space-y-2">
              <Label>Email Address</Label>
              <Input placeholder={`user@${fqdn.fqdn}`} value={emailAddr} onChange={(e) => setEmailAddr(e.target.value)} onBlur={() => touch('email')} />
              {emailErr && <p className="text-xs text-destructive">{emailErr}</p>}
            </div>
            <div className="space-y-2">
              <Label>Display Name (optional)</Label>
              <Input placeholder="John Doe" value={emailDisplay} onChange={(e) => setEmailDisplay(e.target.value)} />
            </div>
            <div className="space-y-2">
              <Label>Quota (bytes, optional)</Label>
              <Input type="number" placeholder="1073741824" value={emailQuota} onChange={(e) => setEmailQuota(e.target.value)} />
              <p className="text-xs text-muted-foreground">1 GB = 1073741824 bytes</p>
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setCreateEmailOpen(false)}>Cancel</Button>
            <Button onClick={handleCreateEmail} disabled={createEmailMut.isPending}>
              {createEmailMut.isPending ? 'Creating...' : 'Create'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Delete Email Account */}
      <ConfirmDialog open={!!deleteEmail} onOpenChange={(o) => !o && setDeleteEmail(null)} title="Delete Email Account"
        description={`Delete email account "${deleteEmail?.address}"? All aliases, forwards, and auto-reply settings will be removed.`}
        confirmLabel="Delete" variant="destructive" loading={deleteEmailMut.isPending}
        onConfirm={async () => { try { await deleteEmailMut.mutateAsync(deleteEmail!.id); toast.success('Email account deleted'); setDeleteEmail(null) } catch (e: unknown) { toast.error(e instanceof Error ? e.message : 'Failed') } }} />
    </div>
  )
}
