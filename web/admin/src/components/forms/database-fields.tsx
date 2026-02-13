import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { ShardSelect } from './shard-select'
import { ArraySection } from './array-section'
import { DatabaseUserFields } from './database-user-fields'
import type { DatabaseFormData, DatabaseUserFormData } from '@/lib/types'

interface Props { value: DatabaseFormData; onChange: (v: DatabaseFormData) => void; clusterId: string }

export function DatabaseFields({ value, onChange, clusterId }: Props) {
  return (
    <div className="space-y-3">
      <div className="space-y-2">
        <Label>Name</Label>
        <Input placeholder="my-database" value={value.name} onChange={(e) => onChange({ ...value, name: e.target.value })} />
      </div>
      <ShardSelect clusterId={clusterId} role="database" value={value.shard_id} onChange={(shard_id) => onChange({ ...value, shard_id })} />
      <ArraySection<DatabaseUserFormData>
        title="Users"
        items={value.users ?? []}
        onChange={(users) => onChange({ ...value, users })}
        defaultItem={() => ({ username: '', password: '', privileges: ['ALL'] })}
        renderItem={(item, _, onItemChange) => <DatabaseUserFields value={item} onChange={onItemChange} />}
        addLabel="Add User"
      />
    </div>
  )
}
