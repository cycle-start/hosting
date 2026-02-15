import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { ShardSelect } from './shard-select'
import { ArraySection } from './array-section'
import { ValkeyUserFields } from './valkey-user-fields'
import type { ValkeyInstanceFormData, ValkeyUserFormData } from '@/lib/types'

interface Props { value: ValkeyInstanceFormData; onChange: (v: ValkeyInstanceFormData) => void; clusterId: string }

export function ValkeyInstanceFields({ value, onChange, clusterId }: Props) {
  return (
    <div className="space-y-3">
      <div className="space-y-2">
        <Label>Max Memory (MB)</Label>
        <Input type="number" value={value.max_memory_mb ?? 64} onChange={(e) => onChange({ ...value, max_memory_mb: parseInt(e.target.value) || 64 })} />
      </div>
      <ShardSelect clusterId={clusterId} role="valkey" value={value.shard_id} onChange={(shard_id) => onChange({ ...value, shard_id })} />
      <ArraySection<ValkeyUserFormData>
        title="Users"
        items={value.users ?? []}
        onChange={(users) => onChange({ ...value, users })}
        defaultItem={() => ({ username: '', password: '', privileges: ['+@all'] })}
        renderItem={(item, _, onItemChange) => <ValkeyUserFields value={item} onChange={onItemChange} />}
        addLabel="Add User"
      />
    </div>
  )
}
