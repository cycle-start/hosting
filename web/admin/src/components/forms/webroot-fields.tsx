import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { ArraySection } from './array-section'
import { FQDNFields } from './fqdn-fields'
import type { WebrootFormData, FQDNFormData } from '@/lib/types'

const runtimes = ['php', 'node', 'python', 'ruby', 'static']

interface Props { value: WebrootFormData; onChange: (v: WebrootFormData) => void }

export function WebrootFields({ value, onChange }: Props) {
  return (
    <div className="space-y-3">
      <div className="space-y-2">
        <Label>Name</Label>
        <Input placeholder="my-site" value={value.name} onChange={(e) => onChange({ ...value, name: e.target.value })} />
      </div>
      <div className="grid grid-cols-2 gap-4">
        <div className="space-y-2">
          <Label>Runtime</Label>
          <Select value={value.runtime} onValueChange={(v) => onChange({ ...value, runtime: v })}>
            <SelectTrigger><SelectValue /></SelectTrigger>
            <SelectContent>
              {runtimes.map(r => <SelectItem key={r} value={r}>{r}</SelectItem>)}
            </SelectContent>
          </Select>
        </div>
        <div className="space-y-2">
          <Label>Version</Label>
          <Input placeholder="8.5" value={value.runtime_version} onChange={(e) => onChange({ ...value, runtime_version: e.target.value })} />
        </div>
      </div>
      <div className="space-y-2">
        <Label>Public Folder</Label>
        <Input placeholder="public" value={value.public_folder} onChange={(e) => onChange({ ...value, public_folder: e.target.value })} />
      </div>
      <ArraySection<FQDNFormData>
        title="FQDNs"
        items={value.fqdns ?? []}
        onChange={(fqdns) => onChange({ ...value, fqdns })}
        defaultItem={() => ({ fqdn: '', ssl_enabled: true })}
        renderItem={(item, _, onItemChange) => <FQDNFields value={item} onChange={onItemChange} />}
      />
    </div>
  )
}
