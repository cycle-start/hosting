import { useEffect } from 'react'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { useSubscriptions } from '@/lib/hooks'

interface Props {
  tenantId: string
  value: string
  onChange: (id: string) => void
}

export function SubscriptionSelect({ tenantId, value, onChange }: Props) {
  const { data: subsData } = useSubscriptions(tenantId)
  const subs = subsData?.items ?? []

  // Auto-select when there's exactly one subscription
  useEffect(() => {
    if (!value && subs.length === 1) {
      onChange(subs[0].id)
    }
  }, [subs, value, onChange])

  return (
    <div className="space-y-2">
      <Label>Subscription</Label>
      <Select value={value || undefined} onValueChange={onChange} disabled={!tenantId}>
        <SelectTrigger><SelectValue placeholder="Select subscription..." /></SelectTrigger>
        <SelectContent>
          {subs.map(s => <SelectItem key={s.id} value={s.id}>{s.name}</SelectItem>)}
        </SelectContent>
      </Select>
    </div>
  )
}
