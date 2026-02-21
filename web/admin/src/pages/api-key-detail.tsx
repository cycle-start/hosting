import { useState, useEffect } from 'react'
import { useParams, useNavigate } from '@tanstack/react-router'
import { ArrowLeft, Save, KeyRound } from 'lucide-react'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { ResourceHeader } from '@/components/shared/resource-header'
import { CopyButton } from '@/components/shared/copy-button'
import { Breadcrumb } from '@/components/shared/breadcrumb'
import { ConfirmDialog } from '@/components/shared/confirm-dialog'
import { ScopePicker } from '@/components/shared/scope-picker'
import { BrandPicker } from '@/components/shared/brand-picker'
import { formatDate } from '@/lib/utils'
import { useAPIKey, useUpdateAPIKey, useRevokeAPIKey } from '@/lib/hooks'

export function APIKeyDetailPage() {
  const { id } = useParams({ from: '/auth/api-keys/$id' as never })
  const navigate = useNavigate()
  const [revokeOpen, setRevokeOpen] = useState(false)
  const [editing, setEditing] = useState(false)
  const [editName, setEditName] = useState('')
  const [editScopes, setEditScopes] = useState<string[]>([])
  const [editBrands, setEditBrands] = useState<string[]>([])

  const { data: apiKey, isLoading } = useAPIKey(id)
  const updateMutation = useUpdateAPIKey()
  const revokeMutation = useRevokeAPIKey()

  useEffect(() => {
    if (apiKey && !editing) {
      setEditName(apiKey.name)
      setEditScopes(apiKey.scopes)
      setEditBrands(apiKey.brands)
    }
  }, [apiKey, editing])

  if (isLoading || !apiKey) {
    return (
      <div className="space-y-6">
        <Skeleton className="h-10 w-64" />
        <Skeleton className="h-64 w-full" />
      </div>
    )
  }

  const isRevoked = !!apiKey.revoked_at

  const startEditing = () => {
    setEditName(apiKey.name)
    setEditScopes([...apiKey.scopes])
    setEditBrands([...apiKey.brands])
    setEditing(true)
  }

  const handleSave = async () => {
    if (editScopes.length === 0 || editBrands.length === 0) {
      toast.error('Scopes and brands are required')
      return
    }
    try {
      await updateMutation.mutateAsync({
        id: apiKey.id,
        name: editName,
        scopes: editScopes,
        brands: editBrands,
      })
      toast.success('API key updated')
      setEditing(false)
    } catch (e: unknown) {
      toast.error(e instanceof Error ? e.message : 'Failed to update API key')
    }
  }

  const handleRevoke = async () => {
    try {
      await revokeMutation.mutateAsync(apiKey.id)
      toast.success('API key revoked')
      navigate({ to: '/api-keys' })
    } catch (e: unknown) {
      toast.error(e instanceof Error ? e.message : 'Failed to revoke API key')
    }
  }

  return (
    <div className="space-y-6 max-w-4xl">
      <Breadcrumb segments={[
        { label: 'API Keys', href: '/api-keys' },
        { label: apiKey.name },
      ]} />

      <ResourceHeader
        icon={KeyRound}
        title={apiKey.name}
        actions={
          <div className="flex gap-2">
            <Button variant="outline" onClick={() => navigate({ to: '/api-keys' })}>
              <ArrowLeft className="mr-2 h-4 w-4" /> Back
            </Button>
            {!isRevoked && (
              <Button variant="destructive" onClick={() => setRevokeOpen(true)}>Revoke</Button>
            )}
          </div>
        }
      />

      <Card>
        <CardHeader className="flex flex-row items-center justify-between">
          <CardTitle>Key Details</CardTitle>
          {!editing && !isRevoked && <Button variant="outline" size="sm" onClick={startEditing}>Edit</Button>}
        </CardHeader>
        <CardContent>
          <div className="grid grid-cols-2 gap-4 mb-6">
            <div>
              <p className="text-sm text-muted-foreground">ID</p>
              <div className="flex items-center gap-1">
                <code className="text-sm">{apiKey.id}</code>
                <CopyButton value={apiKey.id} />
              </div>
            </div>
            <div>
              <p className="text-sm text-muted-foreground">Key Prefix</p>
              <code className="text-sm">{apiKey.key_prefix}...</code>
            </div>
            <div>
              <p className="text-sm text-muted-foreground">Status</p>
              {isRevoked
                ? <span className="text-sm text-destructive">Revoked {formatDate(apiKey.revoked_at!)}</span>
                : <span className="text-sm text-emerald-500">Active</span>
              }
            </div>
            <div>
              <p className="text-sm text-muted-foreground">Created</p>
              <p className="text-sm">{formatDate(apiKey.created_at)}</p>
            </div>
          </div>

          {editing ? (
            <div className="space-y-6 border-t pt-6">
              <div className="space-y-2">
                <Label>Name</Label>
                <Input value={editName} onChange={(e) => setEditName(e.target.value)} />
              </div>

              <div className="space-y-2">
                <Label>Scopes</Label>
                <ScopePicker value={editScopes} onChange={setEditScopes} />
              </div>

              <div className="space-y-2">
                <Label>Brand Access</Label>
                <BrandPicker value={editBrands} onChange={setEditBrands} />
              </div>

              <div className="flex gap-2">
                <Button onClick={handleSave} disabled={updateMutation.isPending || !editName || editScopes.length === 0 || editBrands.length === 0}>
                  <Save className="mr-2 h-4 w-4" />
                  {updateMutation.isPending ? 'Saving...' : 'Save'}
                </Button>
                <Button variant="outline" onClick={() => setEditing(false)}>Cancel</Button>
              </div>
            </div>
          ) : (
            <div className="space-y-6 border-t pt-6">
              <div>
                <p className="text-sm text-muted-foreground mb-1">Name</p>
                <p className="text-sm font-medium">{apiKey.name}</p>
              </div>

              <div>
                <p className="text-sm text-muted-foreground mb-1">Scopes</p>
                {apiKey.scopes.includes('*:*') ? (
                  <p className="text-sm font-medium">Full access (all scopes)</p>
                ) : (
                  <div className="flex flex-wrap gap-1">
                    {apiKey.scopes.map(scope => (
                      <code key={scope} className="text-xs bg-muted px-1.5 py-0.5 rounded">{scope}</code>
                    ))}
                  </div>
                )}
              </div>

              <div>
                <p className="text-sm text-muted-foreground mb-1">Brand Access</p>
                {apiKey.brands.includes('*') ? (
                  <p className="text-sm font-medium">All brands (platform admin)</p>
                ) : (
                  <div className="flex flex-wrap gap-1">
                    {apiKey.brands.map(brand => (
                      <code key={brand} className="text-xs bg-muted px-1.5 py-0.5 rounded">{brand}</code>
                    ))}
                  </div>
                )}
              </div>
            </div>
          )}
        </CardContent>
      </Card>

      <ConfirmDialog
        open={revokeOpen}
        onOpenChange={setRevokeOpen}
        title="Revoke API Key"
        description={`Are you sure you want to revoke "${apiKey.name}"? This action is irreversible and the key will immediately stop authenticating.`}
        confirmLabel="Revoke"
        variant="destructive"
        onConfirm={handleRevoke}
        loading={revokeMutation.isPending}
      />
    </div>
  )
}
