import { useEffect } from 'react'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { useRegions, useClusters, useShards } from '@/lib/hooks'

interface Props {
  regionId: string; onRegionChange: (id: string) => void
  clusterId: string; onClusterChange: (id: string) => void
  shardId: string; onShardChange: (id: string) => void
  shardRole?: string
}

export function RegionClusterShardSelect({ regionId, onRegionChange, clusterId, onClusterChange, shardId, onShardChange, shardRole = 'web' }: Props) {
  const { data: regionsData } = useRegions()
  const { data: clustersData } = useClusters(regionId)
  const { data: shardsData } = useShards(clusterId)

  const regions = regionsData?.items ?? []
  const clusters = clustersData?.items ?? []
  const shards = (shardsData?.items ?? []).filter(s => s.role === shardRole)

  // Auto-select first available option when data loads
  useEffect(() => {
    if (!regionId && regions.length > 0) onRegionChange(regions[0].id)
  }, [regionId, regions]) // eslint-disable-line react-hooks/exhaustive-deps

  useEffect(() => {
    if (regionId && !clusterId && clusters.length > 0) onClusterChange(clusters[0].id)
  }, [regionId, clusterId, clusters]) // eslint-disable-line react-hooks/exhaustive-deps

  useEffect(() => {
    if (clusterId && !shardId && shards.length > 0) onShardChange(shards[0].id)
  }, [clusterId, shardId, shards]) // eslint-disable-line react-hooks/exhaustive-deps

  return (
    <div className="grid grid-cols-3 gap-4">
      <div className="space-y-2">
        <Label>Region</Label>
        <Select value={regionId || undefined} onValueChange={(v) => { onRegionChange(v); onClusterChange(''); onShardChange('') }}>
          <SelectTrigger><SelectValue placeholder="Select region..." /></SelectTrigger>
          <SelectContent>
            {regions.map(r => <SelectItem key={r.id} value={r.id}>{r.name}</SelectItem>)}
          </SelectContent>
        </Select>
      </div>
      <div className="space-y-2">
        <Label>Cluster</Label>
        <Select value={clusterId || undefined} onValueChange={(v) => { onClusterChange(v); onShardChange('') }} disabled={!regionId}>
          <SelectTrigger><SelectValue placeholder="Select cluster..." /></SelectTrigger>
          <SelectContent>
            {clusters.map(c => <SelectItem key={c.id} value={c.id}>{c.name}</SelectItem>)}
          </SelectContent>
        </Select>
      </div>
      <div className="space-y-2">
        <Label>Shard</Label>
        <Select value={shardId || undefined} onValueChange={onShardChange} disabled={!clusterId}>
          <SelectTrigger><SelectValue placeholder="Select shard..." /></SelectTrigger>
          <SelectContent>
            {shards.map(s => <SelectItem key={s.id} value={s.id}>{s.name}</SelectItem>)}
          </SelectContent>
        </Select>
      </div>
    </div>
  )
}
