import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { SubscriptionSelect } from './subscription-select'
import type { ZoneFormData } from '@/lib/types'

interface Props { value: ZoneFormData; onChange: (v: ZoneFormData) => void; tenantId?: string }

export function ZoneFields({ value, onChange, tenantId }: Props) {
  return (
    <div className="space-y-2">
      {tenantId && <SubscriptionSelect tenantId={tenantId} value={value.subscription_id} onChange={(subscription_id) => onChange({ ...value, subscription_id })} />}
      <Label>Zone Name</Label>
      <Input placeholder="example.com" value={value.name} onChange={(e) => onChange({ ...value, name: e.target.value })} />
    </div>
  )
}
