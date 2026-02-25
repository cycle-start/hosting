import { useState } from 'react'
import { useNavigate } from '@tanstack/react-router'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Separator } from '@/components/ui/separator'
import { Breadcrumb } from '@/components/shared/breadcrumb'
import { ResourceHeader } from '@/components/shared/resource-header'
import { RegionClusterShardSelect } from '@/components/forms/region-cluster-shard-select'
import { ArraySection } from '@/components/forms/array-section'
import { ZoneFields } from '@/components/forms/zone-fields'
import { WebrootFields } from '@/components/forms/webroot-fields'
import { DatabaseFields } from '@/components/forms/database-fields'
import { ValkeyInstanceFields } from '@/components/forms/valkey-instance-fields'
import { S3BucketFields } from '@/components/forms/s3-bucket-fields'
import { SSHKeyFields } from '@/components/forms/ssh-key-fields'
import { useCreateTenant, useBrands } from '@/lib/hooks'
import type {
  CreateTenantRequest, ZoneFormData, WebrootFormData,
  DatabaseFormData, ValkeyInstanceFormData, S3BucketFormData, SSHKeyFormData,
} from '@/lib/types'

export function CreateTenantPage() {
  const navigate = useNavigate()
  const createMutation = useCreateTenant()
  const { data: brandsData } = useBrands()

  const [brandId, setBrandId] = useState('')
  const [customerId, setCustomerId] = useState('')
  const [regionId, setRegionId] = useState('')
  const [clusterId, setClusterId] = useState('')
  const [shardId, setShardId] = useState('')
  const [sftpEnabled, setSftpEnabled] = useState(true)

  const [zones, setZones] = useState<ZoneFormData[]>([])
  const [webroots, setWebroots] = useState<WebrootFormData[]>([])
  const [databases, setDatabases] = useState<DatabaseFormData[]>([])
  const [valkeyInstances, setValkeyInstances] = useState<ValkeyInstanceFormData[]>([])
  const [s3Buckets, setS3Buckets] = useState<S3BucketFormData[]>([])
  const [sshKeys, setSSHKeys] = useState<SSHKeyFormData[]>([])

  const handleSubmit = async () => {
    const payload: CreateTenantRequest = {
      brand_id: brandId,
      customer_id: customerId,
      region_id: regionId,
      cluster_id: clusterId,
      shard_id: shardId,
      sftp_enabled: sftpEnabled,
    }
    if (zones.length > 0) payload.zones = zones
    if (webroots.length > 0) payload.webroots = webroots
    if (databases.length > 0) payload.databases = databases
    if (valkeyInstances.length > 0) payload.valkey_instances = valkeyInstances
    if (s3Buckets.length > 0) payload.s3_buckets = s3Buckets
    if (sshKeys.length > 0) payload.ssh_keys = sshKeys

    try {
      const tenant = await createMutation.mutateAsync(payload)
      toast.success('Tenant created')
      navigate({ to: '/tenants/$id', params: { id: tenant.id } })
    } catch (e: unknown) {
      toast.error(e instanceof Error ? e.message : 'Failed to create tenant')
    }
  }

  const canSubmit = brandId && customerId && regionId && clusterId && shardId && !createMutation.isPending

  return (
    <div className="space-y-6 max-w-4xl">
      <Breadcrumb segments={[
        { label: 'Tenants', href: '/tenants' },
        { label: 'New Tenant' },
      ]} />

      <ResourceHeader title="Create Tenant" />

      <Card>
        <CardHeader><CardTitle>Tenant Details</CardTitle></CardHeader>
        <CardContent className="space-y-4">
          <div className="space-y-2">
            <Label>Brand</Label>
            <Select value={brandId} onValueChange={setBrandId}>
              <SelectTrigger><SelectValue placeholder="Select brand" /></SelectTrigger>
              <SelectContent>
                {(brandsData?.items ?? []).map(b => (
                  <SelectItem key={b.id} value={b.id}>{b.name}</SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <div className="space-y-2">
            <Label>Customer ID</Label>
            <Input
              placeholder="External customer identifier"
              value={customerId}
              onChange={(e) => setCustomerId(e.target.value)}
            />
          </div>
          <RegionClusterShardSelect
            regionId={regionId} onRegionChange={setRegionId}
            clusterId={clusterId} onClusterChange={setClusterId}
            shardId={shardId} onShardChange={setShardId}
          />
          <div className="flex items-center gap-2">
            <Switch checked={sftpEnabled} onCheckedChange={setSftpEnabled} />
            <Label>SFTP Enabled</Label>
          </div>
        </CardContent>
      </Card>

      <Separator />

      <ArraySection<ZoneFormData>
        title="Zones"
        items={zones}
        onChange={setZones}
        defaultItem={() => ({ subscription_id: '', name: '' })}
        renderItem={(item, _, onChange) => <ZoneFields value={item} onChange={onChange} />}
      />

      <ArraySection<WebrootFormData>
        title="Webroots"
        items={webroots}
        onChange={setWebroots}
        defaultItem={() => ({ subscription_id: '', runtime: 'php', runtime_version: '8.5', public_folder: 'public', env_file_name: '.env.hosting' })}
        renderItem={(item, _, onChange) => <WebrootFields value={item} onChange={onChange} regionId={regionId} />}
      />

      <ArraySection<DatabaseFormData>
        title="Databases"
        items={databases}
        onChange={setDatabases}
        defaultItem={() => ({ subscription_id: '', shard_id: '' })}
        renderItem={(item, _, onChange) => <DatabaseFields value={item} onChange={onChange} clusterId={clusterId} />}
      />

      <ArraySection<ValkeyInstanceFormData>
        title="Valkey Instances"
        items={valkeyInstances}
        onChange={setValkeyInstances}
        defaultItem={() => ({ subscription_id: '', shard_id: '', max_memory_mb: 64 })}
        renderItem={(item, _, onChange) => <ValkeyInstanceFields value={item} onChange={onChange} clusterId={clusterId} />}
      />

      <ArraySection<S3BucketFormData>
        title="S3 Buckets"
        items={s3Buckets}
        onChange={setS3Buckets}
        defaultItem={() => ({ subscription_id: '', shard_id: '' })}
        renderItem={(item, _, onChange) => <S3BucketFields value={item} onChange={onChange} clusterId={clusterId} />}
      />

      <ArraySection<SSHKeyFormData>
        title="SSH Keys"
        items={sshKeys}
        onChange={setSSHKeys}
        defaultItem={() => ({ name: '', public_key: '' })}
        renderItem={(item, _, onChange) => <SSHKeyFields value={item} onChange={onChange} />}
      />

      <div className="flex justify-end gap-2 pt-4">
        <Button variant="outline" onClick={() => navigate({ to: '/tenants' })}>Cancel</Button>
        <Button onClick={handleSubmit} disabled={!canSubmit}>
          {createMutation.isPending ? 'Creating...' : 'Create Tenant'}
        </Button>
      </div>
    </div>
  )
}
