import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import type { DatabaseUserFormData } from '@/lib/types'

interface Props { value: DatabaseUserFormData; onChange: (v: DatabaseUserFormData) => void }

export function DatabaseUserFields({ value, onChange }: Props) {
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
      <div className="space-y-2">
        <Label>Privileges</Label>
        <Input placeholder="ALL, SELECT, INSERT..." value={value.privileges.join(', ')} onChange={(e) => onChange({ ...value, privileges: e.target.value.split(',').map(s => s.trim()).filter(Boolean) })} />
        <p className="text-xs text-muted-foreground">Comma-separated list of MySQL privileges</p>
      </div>
    </div>
  )
}
