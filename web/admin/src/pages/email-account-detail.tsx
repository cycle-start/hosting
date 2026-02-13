import { useState } from 'react'
import { useParams } from '@tanstack/react-router'
import { type ColumnDef } from '@tanstack/react-table'
import { Plus, Trash2, Mail, Forward, Reply } from 'lucide-react'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { Switch } from '@/components/ui/switch'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Skeleton } from '@/components/ui/skeleton'
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
  useEmailAccount, useEmailAliases, useEmailForwards, useEmailAutoReply,
  useCreateEmailAlias, useDeleteEmailAlias,
  useCreateEmailForward, useDeleteEmailForward,
  useUpsertEmailAutoReply, useDeleteEmailAutoReply,
} from '@/lib/hooks'
import type { EmailAlias, EmailForward } from '@/lib/types'

export function EmailAccountDetailPage() {
  const { id: tenantId, accountId } = useParams({ from: '/tenants/$id/email-accounts/$accountId' as never })

  const [createAliasOpen, setCreateAliasOpen] = useState(false)
  const [createForwardOpen, setCreateForwardOpen] = useState(false)
  const [deleteAlias, setDeleteAlias] = useState<EmailAlias | null>(null)
  const [deleteForward, setDeleteForward] = useState<EmailForward | null>(null)
  const [deleteAutoReplyOpen, setDeleteAutoReplyOpen] = useState(false)
  const [touched, setTouched] = useState<Record<string, boolean>>({})

  // Alias form
  const [aliasAddr, setAliasAddr] = useState('')
  // Forward form
  const [fwdDest, setFwdDest] = useState('')
  const [fwdKeepCopy, setFwdKeepCopy] = useState(false)
  // Auto-reply form
  const [arSubject, setArSubject] = useState('')
  const [arBody, setArBody] = useState('')
  const [arStartDate, setArStartDate] = useState('')
  const [arEndDate, setArEndDate] = useState('')
  const [arEnabled, setArEnabled] = useState(true)

  const { data: account, isLoading } = useEmailAccount(accountId)
  const { data: aliasesData, isLoading: aliasesLoading } = useEmailAliases(accountId)
  const { data: forwardsData, isLoading: forwardsLoading } = useEmailForwards(accountId)
  const { data: autoReply, isLoading: arLoading, error: arError } = useEmailAutoReply(accountId)

  const createAliasMut = useCreateEmailAlias()
  const deleteAliasMut = useDeleteEmailAlias()
  const createForwardMut = useCreateEmailForward()
  const deleteForwardMut = useDeleteEmailForward()
  const upsertArMut = useUpsertEmailAutoReply()
  const deleteArMut = useDeleteEmailAutoReply()

  if (isLoading || !account) {
    return <div className="space-y-6"><Skeleton className="h-10 w-64" /><Skeleton className="h-64 w-full" /></div>
  }

  const touch = (f: string) => setTouched(p => ({ ...p, [f]: true }))
  const aliasErr = touched['alias'] ? validateField(aliasAddr, [rules.required(), rules.email()]) : null
  const fwdErr = touched['fwd'] ? validateField(fwdDest, [rules.required(), rules.email()]) : null

  const handleCreateAlias = async () => {
    setTouched({ alias: true })
    if (validateField(aliasAddr, [rules.required(), rules.email()])) return
    try {
      await createAliasMut.mutateAsync({ account_id: accountId, address: aliasAddr })
      toast.success('Alias created'); setCreateAliasOpen(false); setAliasAddr(''); setTouched({})
    } catch (e: unknown) { toast.error(e instanceof Error ? e.message : 'Failed') }
  }

  const handleCreateForward = async () => {
    setTouched({ fwd: true })
    if (validateField(fwdDest, [rules.required(), rules.email()])) return
    try {
      await createForwardMut.mutateAsync({ account_id: accountId, destination: fwdDest, keep_copy: fwdKeepCopy })
      toast.success('Forward created'); setCreateForwardOpen(false); setFwdDest(''); setFwdKeepCopy(false); setTouched({})
    } catch (e: unknown) { toast.error(e instanceof Error ? e.message : 'Failed') }
  }

  const handleSaveAutoReply = async () => {
    if (!arSubject.trim() || !arBody.trim()) { toast.error('Subject and body are required'); return }
    try {
      await upsertArMut.mutateAsync({
        account_id: accountId, subject: arSubject, body: arBody, enabled: arEnabled,
        start_date: arStartDate || undefined, end_date: arEndDate || undefined,
      })
      toast.success('Auto-reply saved')
    } catch (e: unknown) { toast.error(e instanceof Error ? e.message : 'Failed') }
  }

  // Populate auto-reply form from data
  const hasAutoReply = autoReply && !arError
  if (hasAutoReply && !arSubject && !touched['ar_loaded']) {
    setArSubject(autoReply.subject)
    setArBody(autoReply.body)
    setArStartDate(autoReply.start_date ?? '')
    setArEndDate(autoReply.end_date ?? '')
    setArEnabled(autoReply.enabled)
    setTouched(p => ({ ...p, ar_loaded: true }))
  }

  const aliasColumns: ColumnDef<EmailAlias>[] = [
    { accessorKey: 'address', header: 'Address', cell: ({ row }) => <span className="font-medium">{row.original.address}</span> },
    { accessorKey: 'status', header: 'Status', cell: ({ row }) => <StatusBadge status={row.original.status} /> },
    {
      accessorKey: 'created_at', header: 'Created',
      cell: ({ row }) => <span className="text-sm text-muted-foreground">{formatDate(row.original.created_at)}</span>,
    },
    {
      id: 'actions',
      cell: ({ row }) => (
        <Button variant="ghost" size="icon" onClick={(e) => { e.stopPropagation(); setDeleteAlias(row.original) }}>
          <Trash2 className="h-4 w-4 text-destructive" />
        </Button>
      ),
    },
  ]

  const forwardColumns: ColumnDef<EmailForward>[] = [
    { accessorKey: 'destination', header: 'Destination', cell: ({ row }) => <span className="font-medium">{row.original.destination}</span> },
    { accessorKey: 'keep_copy', header: 'Keep Copy', cell: ({ row }) => row.original.keep_copy ? 'Yes' : 'No' },
    { accessorKey: 'status', header: 'Status', cell: ({ row }) => <StatusBadge status={row.original.status} /> },
    {
      id: 'actions',
      cell: ({ row }) => (
        <Button variant="ghost" size="icon" onClick={(e) => { e.stopPropagation(); setDeleteForward(row.original) }}>
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
        { label: 'Email' },
        { label: account.address },
      ]} />

      <ResourceHeader
        title={account.address}
        subtitle={`${account.display_name || 'No display name'} | Quota: ${Math.round(account.quota_bytes / 1024 / 1024)} MB`}
        status={account.status}
      />

      <div className="flex items-center gap-2 text-sm text-muted-foreground">
        <span>ID: <code>{account.id}</code></span>
        <CopyButton value={account.id} />
        <span className="ml-4">Created: {formatDate(account.created_at)}</span>
      </div>

      <Tabs defaultValue="aliases">
        <TabsList>
          <TabsTrigger value="aliases">Aliases ({aliasesData?.items?.length ?? 0})</TabsTrigger>
          <TabsTrigger value="forwards">Forwards ({forwardsData?.items?.length ?? 0})</TabsTrigger>
          <TabsTrigger value="autoreply">Auto-Reply</TabsTrigger>
        </TabsList>

        <TabsContent value="aliases">
          <div className="mb-4 flex justify-end">
            <Button size="sm" onClick={() => { setAliasAddr(''); setTouched({}); setCreateAliasOpen(true) }}>
              <Plus className="mr-2 h-4 w-4" /> Add Alias
            </Button>
          </div>
          {!aliasesLoading && (aliasesData?.items?.length ?? 0) === 0 ? (
            <EmptyState icon={Mail} title="No aliases" description="Add an email alias for this account." action={{ label: 'Add Alias', onClick: () => setCreateAliasOpen(true) }} />
          ) : (
            <DataTable columns={aliasColumns} data={aliasesData?.items ?? []} loading={aliasesLoading} emptyMessage="No aliases" />
          )}
        </TabsContent>

        <TabsContent value="forwards">
          <div className="mb-4 flex justify-end">
            <Button size="sm" onClick={() => { setFwdDest(''); setFwdKeepCopy(false); setTouched({}); setCreateForwardOpen(true) }}>
              <Plus className="mr-2 h-4 w-4" /> Add Forward
            </Button>
          </div>
          {!forwardsLoading && (forwardsData?.items?.length ?? 0) === 0 ? (
            <EmptyState icon={Forward} title="No forwards" description="Forward emails to another address." action={{ label: 'Add Forward', onClick: () => setCreateForwardOpen(true) }} />
          ) : (
            <DataTable columns={forwardColumns} data={forwardsData?.items ?? []} loading={forwardsLoading} emptyMessage="No forwards" />
          )}
        </TabsContent>

        <TabsContent value="autoreply">
          {arLoading ? (
            <Skeleton className="h-48 w-full" />
          ) : (
            <div className="rounded-lg border p-6 space-y-4 max-w-lg">
              <div className="flex items-center justify-between">
                <h3 className="font-medium">Auto-Reply Settings</h3>
                {hasAutoReply && (
                  <Button variant="ghost" size="sm" className="text-destructive" onClick={() => setDeleteAutoReplyOpen(true)}>
                    <Trash2 className="mr-1 h-3 w-3" /> Remove
                  </Button>
                )}
              </div>
              <div className="space-y-2">
                <Label>Subject</Label>
                <Input placeholder="Out of office" value={arSubject} onChange={(e) => setArSubject(e.target.value)} />
              </div>
              <div className="space-y-2">
                <Label>Body</Label>
                <Textarea rows={4} placeholder="I'm currently away..." value={arBody} onChange={(e) => setArBody(e.target.value)} />
              </div>
              <div className="grid grid-cols-2 gap-4">
                <div className="space-y-2">
                  <Label>Start Date (optional)</Label>
                  <Input type="datetime-local" value={arStartDate} onChange={(e) => setArStartDate(e.target.value)} />
                </div>
                <div className="space-y-2">
                  <Label>End Date (optional)</Label>
                  <Input type="datetime-local" value={arEndDate} onChange={(e) => setArEndDate(e.target.value)} />
                </div>
              </div>
              <div className="flex items-center gap-2">
                <Switch checked={arEnabled} onCheckedChange={setArEnabled} />
                <Label>Enabled</Label>
              </div>
              <Button onClick={handleSaveAutoReply} disabled={upsertArMut.isPending}>
                {upsertArMut.isPending ? 'Saving...' : 'Save Auto-Reply'}
              </Button>
            </div>
          )}
        </TabsContent>
      </Tabs>

      {/* Create Alias */}
      <Dialog open={createAliasOpen} onOpenChange={setCreateAliasOpen}>
        <DialogContent>
          <DialogHeader><DialogTitle>Add Email Alias</DialogTitle></DialogHeader>
          <div className="space-y-4">
            <div className="space-y-2">
              <Label>Alias Address</Label>
              <Input placeholder="alias@example.com" value={aliasAddr} onChange={(e) => setAliasAddr(e.target.value)} onBlur={() => touch('alias')} />
              {aliasErr && <p className="text-xs text-destructive">{aliasErr}</p>}
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setCreateAliasOpen(false)}>Cancel</Button>
            <Button onClick={handleCreateAlias} disabled={createAliasMut.isPending}>
              {createAliasMut.isPending ? 'Adding...' : 'Add'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Create Forward */}
      <Dialog open={createForwardOpen} onOpenChange={setCreateForwardOpen}>
        <DialogContent>
          <DialogHeader><DialogTitle>Add Email Forward</DialogTitle></DialogHeader>
          <div className="space-y-4">
            <div className="space-y-2">
              <Label>Destination Address</Label>
              <Input placeholder="other@example.com" value={fwdDest} onChange={(e) => setFwdDest(e.target.value)} onBlur={() => touch('fwd')} />
              {fwdErr && <p className="text-xs text-destructive">{fwdErr}</p>}
            </div>
            <div className="flex items-center gap-2">
              <Switch checked={fwdKeepCopy} onCheckedChange={setFwdKeepCopy} />
              <Label>Keep a copy in this mailbox</Label>
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setCreateForwardOpen(false)}>Cancel</Button>
            <Button onClick={handleCreateForward} disabled={createForwardMut.isPending}>
              {createForwardMut.isPending ? 'Adding...' : 'Add'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Delete Alias */}
      <ConfirmDialog open={!!deleteAlias} onOpenChange={(o) => !o && setDeleteAlias(null)} title="Delete Alias"
        description={`Delete alias "${deleteAlias?.address}"?`}
        confirmLabel="Delete" variant="destructive" loading={deleteAliasMut.isPending}
        onConfirm={async () => { try { await deleteAliasMut.mutateAsync(deleteAlias!.id); toast.success('Alias deleted'); setDeleteAlias(null) } catch (e: unknown) { toast.error(e instanceof Error ? e.message : 'Failed') } }} />

      {/* Delete Forward */}
      <ConfirmDialog open={!!deleteForward} onOpenChange={(o) => !o && setDeleteForward(null)} title="Delete Forward"
        description={`Delete forward to "${deleteForward?.destination}"?`}
        confirmLabel="Delete" variant="destructive" loading={deleteForwardMut.isPending}
        onConfirm={async () => { try { await deleteForwardMut.mutateAsync(deleteForward!.id); toast.success('Forward deleted'); setDeleteForward(null) } catch (e: unknown) { toast.error(e instanceof Error ? e.message : 'Failed') } }} />

      {/* Delete Auto-Reply */}
      <ConfirmDialog open={deleteAutoReplyOpen} onOpenChange={setDeleteAutoReplyOpen} title="Delete Auto-Reply"
        description="Remove the auto-reply configuration?"
        confirmLabel="Delete" variant="destructive" loading={deleteArMut.isPending}
        onConfirm={async () => {
          try {
            await deleteArMut.mutateAsync(accountId); toast.success('Auto-reply removed'); setDeleteAutoReplyOpen(false)
            setArSubject(''); setArBody(''); setArStartDate(''); setArEndDate(''); setArEnabled(true)
            setTouched(p => { const { ar_loaded: _, ...rest } = p; return rest })
          } catch (e: unknown) { toast.error(e instanceof Error ? e.message : 'Failed') }
        }} />
    </div>
  )
}
