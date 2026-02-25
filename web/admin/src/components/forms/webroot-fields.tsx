import { useMemo } from 'react'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { ArraySection } from './array-section'
import { FQDNFields } from './fqdn-fields'
import { SubscriptionSelect } from './subscription-select'
import { useRegionRuntimes } from '@/lib/hooks'
import type { WebrootFormData, FQDNFormData } from '@/lib/types'

interface Props { value: WebrootFormData; onChange: (v: WebrootFormData) => void; tenantId?: string; regionId?: string }

export function WebrootFields({ value, onChange, tenantId, regionId }: Props) {
  const { data: regionRuntimesData } = useRegionRuntimes(regionId ?? '')

  // Group flat region runtimes into { runtime → versions[] }
  const runtimeGroups = useMemo(() => {
    const items = regionRuntimesData?.items ?? []
    const order: string[] = []
    const map: Record<string, string[]> = {}
    for (const r of items) {
      if (!r.available) continue
      if (!map[r.runtime]) {
        order.push(r.runtime)
        map[r.runtime] = []
      }
      map[r.runtime].push(r.version)
    }
    return order.map(rt => ({ runtime: rt, versions: map[rt] }))
  }, [regionRuntimesData])

  const runtimeNames = runtimeGroups.map(g => g.runtime)
  const versions = runtimeGroups.find(g => g.runtime === value.runtime)?.versions ?? []
  const hasRuntimes = runtimeGroups.length > 0

  return (
    <div className="space-y-3">
      {tenantId && <SubscriptionSelect tenantId={tenantId} value={value.subscription_id} onChange={(subscription_id) => onChange({ ...value, subscription_id })} />}
      <div className="grid grid-cols-2 gap-4">
        <div className="space-y-2">
          <Label>Runtime</Label>
          <Select
            value={value.runtime}
            onValueChange={(v) => {
              const group = runtimeGroups.find(g => g.runtime === v)
              onChange({ ...value, runtime: v, runtime_version: group?.versions[0] ?? '' })
            }}
            disabled={!regionId || !hasRuntimes}
          >
            <SelectTrigger><SelectValue placeholder={!regionId ? 'Select a region first' : 'Select runtime'} /></SelectTrigger>
            <SelectContent>
              {runtimeNames.map(r => <SelectItem key={r} value={r}>{r}</SelectItem>)}
            </SelectContent>
          </Select>
        </div>
        <div className="space-y-2">
          <Label>Version</Label>
          <Select
            value={value.runtime_version}
            onValueChange={(v) => onChange({ ...value, runtime_version: v })}
            disabled={!regionId || !hasRuntimes || versions.length === 0}
          >
            <SelectTrigger><SelectValue placeholder={!regionId ? 'Select a region first' : 'Select version'} /></SelectTrigger>
            <SelectContent>
              {versions.map(v => <SelectItem key={v} value={v}>{v}</SelectItem>)}
            </SelectContent>
          </Select>
        </div>
      </div>
      <div className="grid grid-cols-2 gap-4">
        <div className="space-y-2">
          <Label>Public Folder</Label>
          <Input placeholder="public" value={value.public_folder} onChange={(e) => onChange({ ...value, public_folder: e.target.value })} />
        </div>
        <div className="space-y-2">
          <Label>Env File Name</Label>
          <Input placeholder=".env.hosting" value={value.env_file_name} onChange={(e) => onChange({ ...value, env_file_name: e.target.value })} />
        </div>
      </div>
      <ArraySection<FQDNFormData>
        title="FQDNs"
        items={value.fqdns ?? []}
        onChange={(fqdns) => onChange({ ...value, fqdns })}
        defaultItem={() => ({ fqdn: '', ssl_enabled: true })}
        renderItem={(item, _, onItemChange) => <FQDNFields value={item} onChange={onItemChange} />}
      />
    </div>
  )
}
