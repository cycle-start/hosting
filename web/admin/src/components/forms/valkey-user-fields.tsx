import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import type { ValkeyUserFormData } from '@/lib/types'

interface Props { value: ValkeyUserFormData; onChange: (v: ValkeyUserFormData) => void }

export function ValkeyUserFields({ value, onChange }: Props) {
  return (
    <div className="space-y-3">
      <div className="grid grid-cols-2 gap-4">
        <div className="space-y-2">
          <Label>Username</Label>
          <Input placeholder="app" value={value.username} onChange={(e) => onChange({ ...value, username: e.target.value })} />
        </div>
        <div className="space-y-2">
          <Label>Password</Label>
          <Input type="password" placeholder="Min 8 characters" value={value.password} onChange={(e) => onChange({ ...value, password: e.target.value })} />
        </div>
      </div>
      <div className="grid grid-cols-2 gap-4">
        <div className="space-y-2">
          <Label>Privileges</Label>
          <Input placeholder="+@all" value={value.privileges.join(', ')} onChange={(e) => onChange({ ...value, privileges: e.target.value.split(',').map(s => s.trim()).filter(Boolean) })} />
        </div>
        <div className="space-y-2">
          <Label>Key Pattern</Label>
          <Input placeholder="~*" value={value.key_pattern ?? ''} onChange={(e) => onChange({ ...value, key_pattern: e.target.value })} />
        </div>
      </div>
    </div>
  )
}
