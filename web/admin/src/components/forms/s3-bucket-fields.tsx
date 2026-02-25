import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { ShardSelect } from './shard-select'
import { SubscriptionSelect } from './subscription-select'
import type { S3BucketFormData } from '@/lib/types'

interface Props { value: S3BucketFormData; onChange: (v: S3BucketFormData) => void; clusterId: string; tenantId?: string }

export function S3BucketFields({ value, onChange, clusterId, tenantId }: Props) {
  return (
    <div className="space-y-3">
      {tenantId && <SubscriptionSelect tenantId={tenantId} value={value.subscription_id} onChange={(subscription_id) => onChange({ ...value, subscription_id })} />}
      <ShardSelect clusterId={clusterId} role="s3" value={value.shard_id} onChange={(shard_id) => onChange({ ...value, shard_id })} />
      <div className="flex items-center gap-2">
        <Switch checked={value.public ?? false} onCheckedChange={(pub) => onChange({ ...value, public: pub })} />
        <Label>Public Read</Label>
      </div>
      <div className="space-y-2">
        <Label>Quota (bytes, 0 = unlimited)</Label>
        <Input type="number" placeholder="0" value={value.quota_bytes ?? 0} onChange={(e) => onChange({ ...value, quota_bytes: parseInt(e.target.value) || 0 })} />
      </div>
    </div>
  )
}
