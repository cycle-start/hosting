import { useState } from 'react'
import { type ColumnDef } from '@tanstack/react-table'
import { useNavigate } from '@tanstack/react-router'
import { Plus, Trash2, KeyRound, Copy, Check } from 'lucide-react'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription, DialogFooter,
} from '@/components/ui/dialog'
import { ResourceHeader } from '@/components/shared/resource-header'
import { DataTable } from '@/components/shared/data-table'
import { EmptyState } from '@/components/shared/empty-state'
import { ConfirmDialog } from '@/components/shared/confirm-dialog'
import { ScopePicker } from '@/components/shared/scope-picker'
import { BrandPicker } from '@/components/shared/brand-picker'
import { formatDate } from '@/lib/utils'
import { useAPIKeys, useCreateAPIKey, useRevokeAPIKey } from '@/lib/hooks'
import type { APIKey } from '@/lib/types'

function formatScopes(scopes: string[]): string {
  if (!scopes || scopes.length === 0) return 'none'
  if (scopes.includes('*:*')) return 'full access'
  return `${scopes.length} scope${scopes.length !== 1 ? 's' : ''}`
}

function formatBrands(brands: string[]): string {
  if (!brands || brands.length === 0) return 'none'
  if (brands.includes('*')) return 'all brands'
  return `${brands.length} brand${brands.length !== 1 ? 's' : ''}`
}

export function APIKeysPage() {
  const navigate = useNavigate()
  const [createOpen, setCreateOpen] = useState(false)
  const [revokeTarget, setRevokeTarget] = useState<APIKey | null>(null)
  const [createdKey, setCreatedKey] = useState('')
  const [formName, setFormName] = useState('')
  const [formScopes, setFormScopes] = useState<string[]>(['*:*'])
  const [formBrands, setFormBrands] = useState<string[]>(['*'])
  const [copied, setCopied] = useState(false)

  const { data, isLoading } = useAPIKeys()
  const createMutation = useCreateAPIKey()
  const revokeMutation = useRevokeAPIKey()

  const keys = data?.items ?? []

  const columns: ColumnDef<APIKey>[] = [
    { accessorKey: 'name', header: 'Name' },
    {
      accessorKey: 'key_prefix',
      header: 'Key Prefix',
      cell: ({ row }) => <code className="text-xs">{row.original.key_prefix}...</code>,
    },
    {
      accessorKey: 'scopes',
      header: 'Scopes',
      cell: ({ row }) => <span className="text-sm">{formatScopes(row.original.scopes)}</span>,
    },
    {
      accessorKey: 'brands',
      header: 'Brands',
      cell: ({ row }) => <span className="text-sm">{formatBrands(row.original.brands)}</span>,
    },
    {
      accessorKey: 'created_at',
      header: 'Created',
      cell: ({ row }) => <span className="text-sm text-muted-foreground">{formatDate(row.original.created_at)}</span>,
    },
    {
      accessorKey: 'revoked_at',
      header: 'Status',
      cell: ({ row }) => row.original.revoked_at
        ? <span className="text-sm text-destructive">Revoked {formatDate(row.original.revoked_at)}</span>
        : <span className="text-sm text-emerald-500">Active</span>,
    },
    {
      id: 'actions',
      cell: ({ row }) => !row.original.revoked_at && (
        <Button variant="ghost" size="icon" onClick={(e) => { e.stopPropagation(); setRevokeTarget(row.original) }}>
          <Trash2 className="h-4 w-4 text-destructive" />
        </Button>
      ),
    },
  ]

  const handleCreate = async () => {
    try {
      const result = await createMutation.mutateAsync({ name: formName, scopes: formScopes, brands: formBrands })
      setCreatedKey(result.key)
      setFormName('')
      setFormScopes(['*:*'])
      setFormBrands(['*'])
      toast.success('API key created')
    } catch (e: unknown) {
      toast.error(e instanceof Error ? e.message : 'Failed to create API key')
    }
  }

  const handleRevoke = async () => {
    if (!revokeTarget) return
    try {
      await revokeMutation.mutateAsync(revokeTarget.id)
      toast.success('API key revoked')
      setRevokeTarget(null)
    } catch (e: unknown) {
      toast.error(e instanceof Error ? e.message : 'Failed to revoke API key')
    }
  }

  const handleCopyKey = async () => {
    await navigator.clipboard.writeText(createdKey)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  const handleCloseCreate = () => {
    setCreateOpen(false)
    setCreatedKey('')
    setFormName('')
    setFormScopes(['*:*'])
    setFormBrands(['*'])
  }

  return (
    <div className="space-y-6">
      <ResourceHeader
        title="API Keys"
        subtitle={`${keys.length} key${keys.length !== 1 ? 's' : ''}`}
        actions={
          <Button onClick={() => setCreateOpen(true)}>
            <Plus className="mr-2 h-4 w-4" /> Create API Key
          </Button>
        }
      />

      {!isLoading && keys.length === 0 ? (
        <EmptyState
          icon={KeyRound}
          title="No API keys"
          description="Create an API key to authenticate with the platform."
          action={{ label: 'Create API Key', onClick: () => setCreateOpen(true) }}
        />
      ) : (
        <DataTable
          columns={columns}
          data={keys}
          loading={isLoading}
          searchColumn="name"
          searchPlaceholder="Search keys..."
          onRowClick={(row) => navigate({ to: '/api-keys/$id', params: { id: row.id } })}
        />
      )}

      <Dialog open={createOpen} onOpenChange={handleCloseCreate}>
        <DialogContent className="max-w-2xl max-h-[80vh] overflow-y-auto">
          <DialogHeader>
            <DialogTitle>{createdKey ? 'API Key Created' : 'Create API Key'}</DialogTitle>
            {createdKey && (
              <DialogDescription>
                Copy this key now. It will not be shown again.
              </DialogDescription>
            )}
          </DialogHeader>

          {createdKey ? (
            <div className="space-y-4">
              <div className="flex items-center gap-2 rounded-md border bg-muted p-3">
                <code className="flex-1 break-all text-sm">{createdKey}</code>
                <Button variant="ghost" size="icon" onClick={handleCopyKey}>
                  {copied ? <Check className="h-4 w-4 text-emerald-500" /> : <Copy className="h-4 w-4" />}
                </Button>
              </div>
              <DialogFooter>
                <Button onClick={handleCloseCreate}>Done</Button>
              </DialogFooter>
            </div>
          ) : (
            <>
              <div className="space-y-6">
                <div className="space-y-2">
                  <Label>Name</Label>
                  <Input placeholder="e.g. admin-key" value={formName} onChange={(e) => setFormName(e.target.value)} />
                </div>

                <div className="space-y-2">
                  <Label>Scopes</Label>
                  <ScopePicker value={formScopes} onChange={setFormScopes} />
                </div>

                <div className="space-y-2">
                  <Label>Brand Access</Label>
                  <BrandPicker value={formBrands} onChange={setFormBrands} />
                </div>
              </div>
              <DialogFooter>
                <Button variant="outline" onClick={handleCloseCreate}>Cancel</Button>
                <Button onClick={handleCreate} disabled={createMutation.isPending || !formName || formScopes.length === 0 || formBrands.length === 0}>
                  {createMutation.isPending ? 'Creating...' : 'Create'}
                </Button>
              </DialogFooter>
            </>
          )}
        </DialogContent>
      </Dialog>

      <ConfirmDialog
        open={!!revokeTarget}
        onOpenChange={(open) => !open && setRevokeTarget(null)}
        title="Revoke API Key"
        description={`Are you sure you want to revoke "${revokeTarget?.name}"? This cannot be undone.`}
        confirmLabel="Revoke"
        variant="destructive"
        onConfirm={handleRevoke}
        loading={revokeMutation.isPending}
      />
    </div>
  )
}
