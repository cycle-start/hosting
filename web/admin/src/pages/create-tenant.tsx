import { useState } from 'react'
import { useNavigate } from '@tanstack/react-router'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
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
import { SFTPKeyFields } from '@/components/forms/sftp-key-fields'
import { useCreateTenant } from '@/lib/hooks'
import type {
  CreateTenantRequest, ZoneFormData, WebrootFormData,
  DatabaseFormData, ValkeyInstanceFormData, SFTPKeyFormData,
} from '@/lib/types'

export function CreateTenantPage() {
  const navigate = useNavigate()
  const createMutation = useCreateTenant()

  const [name, setName] = useState('')
  const [regionId, setRegionId] = useState('')
  const [clusterId, setClusterId] = useState('')
  const [shardId, setShardId] = useState('')
  const [sftpEnabled, setSftpEnabled] = useState(false)

  const [zones, setZones] = useState<ZoneFormData[]>([])
  const [webroots, setWebroots] = useState<WebrootFormData[]>([])
  const [databases, setDatabases] = useState<DatabaseFormData[]>([])
  const [valkeyInstances, setValkeyInstances] = useState<ValkeyInstanceFormData[]>([])
  const [sftpKeys, setSftpKeys] = useState<SFTPKeyFormData[]>([])

  const handleSubmit = async () => {
    const payload: CreateTenantRequest = {
      name,
      region_id: regionId,
      cluster_id: clusterId,
      shard_id: shardId,
      sftp_enabled: sftpEnabled,
    }
    if (zones.length > 0) payload.zones = zones
    if (webroots.length > 0) payload.webroots = webroots
    if (databases.length > 0) payload.databases = databases
    if (valkeyInstances.length > 0) payload.valkey_instances = valkeyInstances
    if (sftpKeys.length > 0) payload.sftp_keys = sftpKeys

    try {
      const tenant = await createMutation.mutateAsync(payload)
      toast.success('Tenant created')
      navigate({ to: '/tenants/$id', params: { id: tenant.id } })
    } catch (e: unknown) {
      toast.error(e instanceof Error ? e.message : 'Failed to create tenant')
    }
  }

  const canSubmit = name && regionId && clusterId && shardId && !createMutation.isPending

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
            <Label>Name</Label>
            <Input placeholder="my-tenant" value={name} onChange={(e) => setName(e.target.value)} />
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
        defaultItem={() => ({ name: '' })}
        renderItem={(item, _, onChange) => <ZoneFields value={item} onChange={onChange} />}
      />

      <ArraySection<WebrootFormData>
        title="Webroots"
        items={webroots}
        onChange={setWebroots}
        defaultItem={() => ({ name: '', runtime: 'php', runtime_version: '8.5', public_folder: 'public' })}
        renderItem={(item, _, onChange) => <WebrootFields value={item} onChange={onChange} />}
      />

      <ArraySection<DatabaseFormData>
        title="Databases"
        items={databases}
        onChange={setDatabases}
        defaultItem={() => ({ name: '', shard_id: '' })}
        renderItem={(item, _, onChange) => <DatabaseFields value={item} onChange={onChange} clusterId={clusterId} />}
      />

      <ArraySection<ValkeyInstanceFormData>
        title="Valkey Instances"
        items={valkeyInstances}
        onChange={setValkeyInstances}
        defaultItem={() => ({ name: '', shard_id: '', max_memory_mb: 64 })}
        renderItem={(item, _, onChange) => <ValkeyInstanceFields value={item} onChange={onChange} clusterId={clusterId} />}
      />

      <ArraySection<SFTPKeyFormData>
        title="SFTP Keys"
        items={sftpKeys}
        onChange={setSftpKeys}
        defaultItem={() => ({ name: '', public_key: '' })}
        renderItem={(item, _, onChange) => <SFTPKeyFields value={item} onChange={onChange} />}
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
