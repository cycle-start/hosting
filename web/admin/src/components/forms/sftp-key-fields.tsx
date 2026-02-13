import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import type { SFTPKeyFormData } from '@/lib/types'

interface Props { value: SFTPKeyFormData; onChange: (v: SFTPKeyFormData) => void }

export function SFTPKeyFields({ value, onChange }: Props) {
  return (
    <div className="space-y-3">
      <div className="space-y-2">
        <Label>Name</Label>
        <Input placeholder="deploy-key" value={value.name} onChange={(e) => onChange({ ...value, name: e.target.value })} />
      </div>
      <div className="space-y-2">
        <Label>Public Key</Label>
        <Textarea placeholder="ssh-ed25519 AAAA..." rows={3} value={value.public_key} onChange={(e) => onChange({ ...value, public_key: e.target.value })} />
      </div>
    </div>
  )
}
