import { useState } from 'react'
import { useParams, useNavigate } from '@tanstack/react-router'
import { ArrowLeft, Save, Plus, X, Tag } from 'lucide-react'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { Badge } from '@/components/ui/badge'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { ResourceHeader } from '@/components/shared/resource-header'
import { StatusBadge } from '@/components/shared/status-badge'
import { CopyButton } from '@/components/shared/copy-button'
import { Breadcrumb } from '@/components/shared/breadcrumb'
import { ConfirmDialog } from '@/components/shared/confirm-dialog'
import { formatDate } from '@/lib/utils'
import { useBrand, useUpdateBrand, useDeleteBrand, useBrandClusters, useSetBrandClusters, useRegions, useClusters } from '@/lib/hooks'

export function BrandDetailPage() {
  const { id } = useParams({ from: '/auth/brands/$id' as never })
  const navigate = useNavigate()
  const [deleteOpen, setDeleteOpen] = useState(false)
  const [editing, setEditing] = useState(false)
  const [editName, setEditName] = useState('')
  const [editBaseHostname, setEditBaseHostname] = useState('')
  const [editPrimaryNS, setEditPrimaryNS] = useState('')
  const [editSecondaryNS, setEditSecondaryNS] = useState('')
  const [editHostmaster, setEditHostmaster] = useState('')
  const [editMailHostname, setEditMailHostname] = useState('')
  const [editSpfIncludes, setEditSpfIncludes] = useState('')
  const [editDkimSelector, setEditDkimSelector] = useState('')
  const [editDkimPublicKey, setEditDkimPublicKey] = useState('')
  const [editDmarcPolicy, setEditDmarcPolicy] = useState('')
  const [selectedRegion, setSelectedRegion] = useState('')
  const [addClusterId, setAddClusterId] = useState('')

  const { data: brand, isLoading } = useBrand(id)
  const { data: brandClusters } = useBrandClusters(id)
  const { data: regionsData } = useRegions()
  const { data: clustersData } = useClusters(selectedRegion)
  const updateMutation = useUpdateBrand()
  const deleteMutation = useDeleteBrand()
  const setClustersMutation = useSetBrandClusters()

  const clusterIds = brandClusters?.cluster_ids ?? []
  const regions = regionsData?.items ?? []
  const clusters = clustersData?.items ?? []

  if (isLoading || !brand) {
    return (
      <div className="space-y-6">
        <Skeleton className="h-10 w-64" />
        <Skeleton className="h-64 w-full" />
      </div>
    )
  }

  const startEditing = () => {
    setEditName(brand.name)
    setEditBaseHostname(brand.base_hostname)
    setEditPrimaryNS(brand.primary_ns)
    setEditSecondaryNS(brand.secondary_ns)
    setEditHostmaster(brand.hostmaster_email)
    setEditMailHostname(brand.mail_hostname ?? '')
    setEditSpfIncludes(brand.spf_includes ?? '')
    setEditDkimSelector(brand.dkim_selector ?? '')
    setEditDkimPublicKey(brand.dkim_public_key ?? '')
    setEditDmarcPolicy(brand.dmarc_policy ?? '')
    setEditing(true)
  }

  const handleSave = async () => {
    try {
      await updateMutation.mutateAsync({
        id: brand.id,
        name: editName,
        base_hostname: editBaseHostname,
        primary_ns: editPrimaryNS,
        secondary_ns: editSecondaryNS,
        hostmaster_email: editHostmaster,
        mail_hostname: editMailHostname || undefined,
        spf_includes: editSpfIncludes || undefined,
        dkim_selector: editDkimSelector || undefined,
        dkim_public_key: editDkimPublicKey || undefined,
        dmarc_policy: editDmarcPolicy || undefined,
      })
      toast.success('Brand updated')
      setEditing(false)
    } catch (e: unknown) {
      toast.error(e instanceof Error ? e.message : 'Failed to update brand')
    }
  }

  const handleDelete = async () => {
    try {
      await deleteMutation.mutateAsync(brand.id)
      toast.success('Brand deleted')
      navigate({ to: '/brands' })
    } catch (e: unknown) {
      toast.error(e instanceof Error ? e.message : 'Failed to delete brand')
    }
  }

  const handleAddCluster = async () => {
    if (!addClusterId || clusterIds.includes(addClusterId)) return
    try {
      await setClustersMutation.mutateAsync({
        brand_id: brand.id,
        cluster_ids: [...clusterIds, addClusterId],
      })
      toast.success('Cluster added')
      setAddClusterId('')
    } catch (e: unknown) {
      toast.error(e instanceof Error ? e.message : 'Failed to add cluster')
    }
  }

  const handleRemoveCluster = async (clusterId: string) => {
    try {
      await setClustersMutation.mutateAsync({
        brand_id: brand.id,
        cluster_ids: clusterIds.filter(c => c !== clusterId),
      })
      toast.success('Cluster removed')
    } catch (e: unknown) {
      toast.error(e instanceof Error ? e.message : 'Failed to remove cluster')
    }
  }

  return (
    <div className="space-y-6 max-w-4xl">
      <Breadcrumb segments={[
        { label: 'Brands', href: '/brands' },
        { label: brand.name },
      ]} />

      <ResourceHeader
        icon={Tag}
        title={brand.name}
        actions={
          <div className="flex gap-2">
            <Button variant="outline" onClick={() => navigate({ to: '/brands' })}>
              <ArrowLeft className="mr-2 h-4 w-4" /> Back
            </Button>
            <Button variant="destructive" onClick={() => setDeleteOpen(true)}>Delete</Button>
          </div>
        }
      />

      <Card>
        <CardHeader className="flex flex-row items-center justify-between">
          <CardTitle>Brand Details</CardTitle>
          {!editing && <Button variant="outline" size="sm" onClick={startEditing}>Edit</Button>}
        </CardHeader>
        <CardContent>
          {editing ? (
            <div className="space-y-4">
              <div className="space-y-2">
                <Label>Name</Label>
                <Input value={editName} onChange={(e) => setEditName(e.target.value)} />
              </div>
              <div className="space-y-2">
                <Label>Base Hostname</Label>
                <Input value={editBaseHostname} onChange={(e) => setEditBaseHostname(e.target.value)} />
              </div>
              <div className="space-y-2">
                <Label>Primary NS</Label>
                <Input value={editPrimaryNS} onChange={(e) => setEditPrimaryNS(e.target.value)} />
              </div>
              <div className="space-y-2">
                <Label>Secondary NS</Label>
                <Input value={editSecondaryNS} onChange={(e) => setEditSecondaryNS(e.target.value)} />
              </div>
              <div className="space-y-2">
                <Label>Hostmaster Email</Label>
                <Input value={editHostmaster} onChange={(e) => setEditHostmaster(e.target.value)} />
              </div>
              <div className="flex gap-2">
                <Button onClick={handleSave} disabled={updateMutation.isPending}>
                  <Save className="mr-2 h-4 w-4" />
                  {updateMutation.isPending ? 'Saving...' : 'Save'}
                </Button>
                <Button variant="outline" onClick={() => setEditing(false)}>Cancel</Button>
              </div>
            </div>
          ) : (
            <div className="grid grid-cols-2 gap-4">
              <div>
                <p className="text-sm text-muted-foreground">ID</p>
                <div className="flex items-center gap-1">
                  <code className="text-sm">{brand.id}</code>
                  <CopyButton value={brand.id} />
                </div>
              </div>
              <div>
                <p className="text-sm text-muted-foreground">Status</p>
                <StatusBadge status={brand.status} />
              </div>
              <div>
                <p className="text-sm text-muted-foreground">Name</p>
                <p className="text-sm font-medium">{brand.name}</p>
              </div>
              <div>
                <p className="text-sm text-muted-foreground">Base Hostname</p>
                <p className="text-sm font-medium">{brand.base_hostname}</p>
              </div>
              <div>
                <p className="text-sm text-muted-foreground">Primary NS</p>
                <p className="text-sm font-medium">{brand.primary_ns}</p>
              </div>
              <div>
                <p className="text-sm text-muted-foreground">Secondary NS</p>
                <p className="text-sm font-medium">{brand.secondary_ns}</p>
              </div>
              <div>
                <p className="text-sm text-muted-foreground">Hostmaster Email</p>
                <p className="text-sm font-medium">{brand.hostmaster_email}</p>
              </div>
              <div>
                <p className="text-sm text-muted-foreground">Created</p>
                <p className="text-sm">{formatDate(brand.created_at)}</p>
              </div>
            </div>
          )}
        </CardContent>
      </Card>

      <Card>
        <CardHeader className="flex flex-row items-center justify-between">
          <CardTitle>Email DNS Configuration</CardTitle>
        </CardHeader>
        <CardContent>
          {editing ? (
            <div className="space-y-4">
              <div className="space-y-2">
                <Label>Mail Hostname</Label>
                <Input placeholder="e.g. mail.example.com" value={editMailHostname} onChange={(e) => setEditMailHostname(e.target.value)} />
              </div>
              <div className="space-y-2">
                <Label>SPF Includes</Label>
                <Input placeholder="e.g. _spf.google.com" value={editSpfIncludes} onChange={(e) => setEditSpfIncludes(e.target.value)} />
              </div>
              <div className="space-y-2">
                <Label>DKIM Selector</Label>
                <Input placeholder="e.g. default" value={editDkimSelector} onChange={(e) => setEditDkimSelector(e.target.value)} />
              </div>
              <div className="space-y-2">
                <Label>DKIM Public Key</Label>
                <Input placeholder="DKIM public key" value={editDkimPublicKey} onChange={(e) => setEditDkimPublicKey(e.target.value)} />
              </div>
              <div className="space-y-2">
                <Label>DMARC Policy</Label>
                <Input placeholder="e.g. v=DMARC1; p=none" value={editDmarcPolicy} onChange={(e) => setEditDmarcPolicy(e.target.value)} />
              </div>
            </div>
          ) : (
            <div className="grid grid-cols-2 gap-4">
              <div>
                <p className="text-sm text-muted-foreground">Mail Hostname</p>
                <p className="text-sm font-medium">{brand.mail_hostname || '-'}</p>
              </div>
              <div>
                <p className="text-sm text-muted-foreground">SPF Includes</p>
                <p className="text-sm font-medium">{brand.spf_includes || '-'}</p>
              </div>
              <div>
                <p className="text-sm text-muted-foreground">DKIM Selector</p>
                <p className="text-sm font-medium">{brand.dkim_selector || '-'}</p>
              </div>
              <div>
                <p className="text-sm text-muted-foreground">DKIM Public Key</p>
                <p className="text-sm font-medium truncate">{brand.dkim_public_key || '-'}</p>
              </div>
              <div className="col-span-2">
                <p className="text-sm text-muted-foreground">DMARC Policy</p>
                <p className="text-sm font-medium">{brand.dmarc_policy || '-'}</p>
              </div>
            </div>
          )}
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Allowed Clusters</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <p className="text-sm text-muted-foreground">
            {clusterIds.length === 0
              ? 'No cluster restrictions. Tenants in this brand can be placed on any cluster.'
              : 'Tenants in this brand can only be placed on these clusters.'}
          </p>

          {clusterIds.length > 0 && (
            <div className="flex flex-wrap gap-2">
              {clusterIds.map(cid => (
                <Badge key={cid} variant="secondary" className="gap-1">
                  {cid}
                  <button onClick={() => handleRemoveCluster(cid)} className="ml-1 hover:text-destructive">
                    <X className="h-3 w-3" />
                  </button>
                </Badge>
              ))}
            </div>
          )}

          <div className="flex items-end gap-2">
            <div className="space-y-2 flex-1">
              <Label>Region</Label>
              <Select value={selectedRegion} onValueChange={(v) => { setSelectedRegion(v); setAddClusterId('') }}>
                <SelectTrigger><SelectValue placeholder="Select region" /></SelectTrigger>
                <SelectContent>
                  {regions.map(r => (
                    <SelectItem key={r.id} value={r.id}>{r.name}</SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-2 flex-1">
              <Label>Cluster</Label>
              <Select value={addClusterId} onValueChange={setAddClusterId} disabled={!selectedRegion}>
                <SelectTrigger><SelectValue placeholder="Select cluster" /></SelectTrigger>
                <SelectContent>
                  {clusters.filter(c => !clusterIds.includes(c.id)).map(c => (
                    <SelectItem key={c.id} value={c.id}>{c.name}</SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <Button onClick={handleAddCluster} disabled={!addClusterId || setClustersMutation.isPending}>
              <Plus className="mr-2 h-4 w-4" /> Add
            </Button>
          </div>
        </CardContent>
      </Card>

      <ConfirmDialog
        open={deleteOpen}
        onOpenChange={setDeleteOpen}
        title="Delete Brand"
        description={`Are you sure you want to delete brand "${brand.name}"? This action cannot be undone.`}
        confirmLabel="Delete"
        variant="destructive"
        onConfirm={handleDelete}
        loading={deleteMutation.isPending}
      />
    </div>
  )
}
