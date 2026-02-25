import { useState } from 'react'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { Button } from '@/components/ui/button'
import { Textarea } from '@/components/ui/textarea'
import { Plus, X } from 'lucide-react'
import { SubscriptionSelect } from './subscription-select'
import type { EmailAccountFormData } from '@/lib/types'

interface Props { value: EmailAccountFormData; onChange: (v: EmailAccountFormData) => void; tenantId?: string }

export function EmailAccountFields({ value, onChange, tenantId }: Props) {
  const [showAutoReply, setShowAutoReply] = useState(!!value.autoreply)

  return (
    <div className="space-y-3">
      {tenantId && <SubscriptionSelect tenantId={tenantId} value={value.subscription_id} onChange={(subscription_id) => onChange({ ...value, subscription_id })} />}
      <div className="grid grid-cols-2 gap-4">
        <div className="space-y-2">
          <Label>Email Address</Label>
          <Input placeholder="info@example.com" value={value.address} onChange={(e) => onChange({ ...value, address: e.target.value })} />
        </div>
        <div className="space-y-2">
          <Label>Display Name</Label>
          <Input placeholder="Info" value={value.display_name ?? ''} onChange={(e) => onChange({ ...value, display_name: e.target.value })} />
        </div>
      </div>
      {/* Aliases */}
      <div className="space-y-2">
        <div className="flex items-center justify-between">
          <Label>Aliases</Label>
          <Button type="button" variant="outline" size="sm" onClick={() => onChange({ ...value, aliases: [...(value.aliases ?? []), { address: '' }] })}>
            <Plus className="mr-1 h-3 w-3" /> Add Alias
          </Button>
        </div>
        {(value.aliases ?? []).map((alias, i) => (
          <div key={i} className="flex gap-2">
            <Input placeholder="alias@example.com" value={alias.address} onChange={(e) => {
              const aliases = [...(value.aliases ?? [])]
              aliases[i] = { address: e.target.value }
              onChange({ ...value, aliases })
            }} />
            <Button type="button" variant="ghost" size="icon" onClick={() => onChange({ ...value, aliases: (value.aliases ?? []).filter((_, j) => j !== i) })}>
              <X className="h-4 w-4" />
            </Button>
          </div>
        ))}
      </div>
      {/* Forwards */}
      <div className="space-y-2">
        <div className="flex items-center justify-between">
          <Label>Forwards</Label>
          <Button type="button" variant="outline" size="sm" onClick={() => onChange({ ...value, forwards: [...(value.forwards ?? []), { destination: '', keep_copy: true }] })}>
            <Plus className="mr-1 h-3 w-3" /> Add Forward
          </Button>
        </div>
        {(value.forwards ?? []).map((fwd, i) => (
          <div key={i} className="flex gap-2 items-center">
            <Input placeholder="admin@gmail.com" className="flex-1" value={fwd.destination} onChange={(e) => {
              const forwards = [...(value.forwards ?? [])]
              forwards[i] = { ...forwards[i], destination: e.target.value }
              onChange({ ...value, forwards })
            }} />
            <div className="flex items-center gap-1">
              <Switch checked={fwd.keep_copy ?? true} onCheckedChange={(checked) => {
                const forwards = [...(value.forwards ?? [])]
                forwards[i] = { ...forwards[i], keep_copy: checked }
                onChange({ ...value, forwards })
              }} />
              <span className="text-xs text-muted-foreground">Keep copy</span>
            </div>
            <Button type="button" variant="ghost" size="icon" onClick={() => onChange({ ...value, forwards: (value.forwards ?? []).filter((_, j) => j !== i) })}>
              <X className="h-4 w-4" />
            </Button>
          </div>
        ))}
      </div>
      {/* Auto-Reply */}
      <div className="space-y-2">
        <div className="flex items-center gap-2">
          <Switch checked={showAutoReply} onCheckedChange={(checked) => {
            setShowAutoReply(checked)
            if (!checked) onChange({ ...value, autoreply: undefined })
            else onChange({ ...value, autoreply: { subject: '', body: '', enabled: false } })
          }} />
          <Label>Auto-Reply</Label>
        </div>
        {showAutoReply && value.autoreply && (
          <div className="space-y-2 pl-4 border-l-2">
            <Input placeholder="Subject" value={value.autoreply.subject} onChange={(e) => onChange({ ...value, autoreply: { ...value.autoreply!, subject: e.target.value } })} />
            <Textarea placeholder="Auto-reply message..." rows={2} value={value.autoreply.body} onChange={(e) => onChange({ ...value, autoreply: { ...value.autoreply!, body: e.target.value } })} />
            <div className="flex items-center gap-2">
              <Switch checked={value.autoreply.enabled} onCheckedChange={(checked) => onChange({ ...value, autoreply: { ...value.autoreply!, enabled: checked } })} />
              <Label>Enabled</Label>
            </div>
          </div>
        )}
      </div>
    </div>
  )
}
