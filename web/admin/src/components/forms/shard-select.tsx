import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { useShards } from '@/lib/hooks'

interface Props {
  clusterId: string
  role: string
  value: string
  onChange: (id: string) => void
  label?: string
}

export function ShardSelect({ clusterId, role, value, onChange, label = 'Shard' }: Props) {
  const { data: shardsData } = useShards(clusterId)
  const shards = (shardsData?.items ?? []).filter(s => s.role === role)

  return (
    <div className="space-y-2">
      <Label>{label}</Label>
      <Select value={value} onValueChange={onChange} disabled={!clusterId}>
        <SelectTrigger><SelectValue placeholder="Select shard..." /></SelectTrigger>
        <SelectContent>
          {shards.map(s => <SelectItem key={s.id} value={s.id}>{s.name}</SelectItem>)}
        </SelectContent>
      </Select>
    </div>
  )
}
