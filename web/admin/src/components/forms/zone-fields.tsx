import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import type { ZoneFormData } from '@/lib/types'

interface Props { value: ZoneFormData; onChange: (v: ZoneFormData) => void }

export function ZoneFields({ value, onChange }: Props) {
  return (
    <div className="space-y-2">
      <Label>Zone Name</Label>
      <Input placeholder="example.com" value={value.name} onChange={(e) => onChange({ ...value, name: e.target.value })} />
    </div>
  )
}
