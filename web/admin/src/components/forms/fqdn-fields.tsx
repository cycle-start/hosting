import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { ArraySection } from './array-section'
import { EmailAccountFields } from './email-account-fields'
import type { FQDNFormData, EmailAccountFormData } from '@/lib/types'

interface Props { value: FQDNFormData; onChange: (v: FQDNFormData) => void }

export function FQDNFields({ value, onChange }: Props) {
  return (
    <div className="space-y-3">
      <div className="flex gap-4 items-end">
        <div className="flex-1 space-y-2">
          <Label>FQDN</Label>
          <Input placeholder="example.com" value={value.fqdn} onChange={(e) => onChange({ ...value, fqdn: e.target.value })} />
        </div>
        <div className="flex items-center gap-2 pb-2">
          <Switch checked={value.ssl_enabled ?? true} onCheckedChange={(checked) => onChange({ ...value, ssl_enabled: checked })} />
          <Label>SSL</Label>
        </div>
      </div>
      <ArraySection<EmailAccountFormData>
        title="Email Accounts"
        items={value.email_accounts ?? []}
        onChange={(email_accounts) => onChange({ ...value, email_accounts })}
        defaultItem={() => ({ address: '' })}
        renderItem={(item, _, onItemChange) => <EmailAccountFields value={item} onChange={onItemChange} />}
        addLabel="Add Email Account"
      />
    </div>
  )
}
