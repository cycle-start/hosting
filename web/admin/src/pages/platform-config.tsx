import { useState, useEffect } from 'react'
import { Save, Settings, Webhook, Code } from 'lucide-react'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { Skeleton } from '@/components/ui/skeleton'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { ResourceHeader } from '@/components/shared/resource-header'
import { EmptyState } from '@/components/shared/empty-state'
import { usePlatformConfig, useUpdatePlatformConfig } from '@/lib/hooks'
import type { PlatformConfig } from '@/lib/types'

const WEBHOOK_TRIGGERS = [
  { key: 'critical', label: 'Critical Incidents', description: 'Fires when a new critical incident is created' },
  { key: 'escalated', label: 'Escalated Incidents', description: 'Fires when an incident is escalated' },
] as const

function WebhooksTab({ config, onSave, isSaving }: {
  config: PlatformConfig
  onSave: (updates: PlatformConfig) => void
  isSaving: boolean
}) {
  const [webhooks, setWebhooks] = useState<Record<string, { url: string; template: string }>>({})

  useEffect(() => {
    const parsed: Record<string, { url: string; template: string }> = {}
    for (const trigger of WEBHOOK_TRIGGERS) {
      parsed[trigger.key] = {
        url: config[`webhook.${trigger.key}.url`] || '',
        template: config[`webhook.${trigger.key}.template`] || 'generic',
      }
    }
    setWebhooks(parsed)
  }, [config])

  const handleSave = () => {
    const updates: PlatformConfig = {}
    for (const trigger of WEBHOOK_TRIGGERS) {
      const wh = webhooks[trigger.key]
      if (wh) {
        updates[`webhook.${trigger.key}.url`] = wh.url
        updates[`webhook.${trigger.key}.template`] = wh.template
      }
    }
    onSave(updates)
  }

  const updateWebhook = (triggerKey: string, field: 'url' | 'template', value: string) => {
    setWebhooks(prev => ({
      ...prev,
      [triggerKey]: { ...prev[triggerKey], [field]: value },
    }))
  }

  return (
    <div className="space-y-4">
      {WEBHOOK_TRIGGERS.map(trigger => (
        <Card key={trigger.key}>
          <CardHeader>
            <CardTitle className="text-base">{trigger.label}</CardTitle>
            <CardDescription>{trigger.description}</CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor={`${trigger.key}-url`}>Webhook URL</Label>
              <Input
                id={`${trigger.key}-url`}
                placeholder="https://hooks.slack.com/services/... or any HTTP endpoint"
                value={webhooks[trigger.key]?.url || ''}
                onChange={e => updateWebhook(trigger.key, 'url', e.target.value)}
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor={`${trigger.key}-template`}>Payload Template</Label>
              <Select
                value={webhooks[trigger.key]?.template || 'generic'}
                onValueChange={v => updateWebhook(trigger.key, 'template', v)}
              >
                <SelectTrigger id={`${trigger.key}-template`} className="w-48">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="generic">Generic JSON</SelectItem>
                  <SelectItem value="slack">Slack</SelectItem>
                </SelectContent>
              </Select>
              <p className="text-xs text-muted-foreground">
                {webhooks[trigger.key]?.template === 'slack'
                  ? 'Sends Slack Block Kit formatted messages'
                  : 'Sends a JSON payload with event type and incident data'}
              </p>
            </div>
          </CardContent>
        </Card>
      ))}
      <div className="flex justify-end">
        <Button onClick={handleSave} disabled={isSaving}>
          <Save className="mr-2 h-4 w-4" />
          {isSaving ? 'Saving...' : 'Save Webhooks'}
        </Button>
      </div>
    </div>
  )
}

function RawJsonTab({ config, onSave, isSaving }: {
  config: PlatformConfig
  onSave: (updates: PlatformConfig) => void
  isSaving: boolean
}) {
  const [jsonText, setJsonText] = useState('')
  const [parseError, setParseError] = useState('')

  useEffect(() => {
    setJsonText(JSON.stringify(config, null, 2))
  }, [config])

  const handleSave = () => {
    setParseError('')
    let parsed: PlatformConfig
    try {
      parsed = JSON.parse(jsonText)
    } catch {
      setParseError('Invalid JSON')
      return
    }
    onSave(parsed)
  }

  return (
    <div className="space-y-4">
      <Textarea
        className="min-h-[500px] font-mono text-sm"
        value={jsonText}
        onChange={e => { setJsonText(e.target.value); setParseError('') }}
      />
      {parseError && <p className="text-sm text-destructive">{parseError}</p>}
      <div className="flex justify-end">
        <Button onClick={handleSave} disabled={isSaving}>
          <Save className="mr-2 h-4 w-4" />
          {isSaving ? 'Saving...' : 'Save'}
        </Button>
      </div>
    </div>
  )
}

export function PlatformConfigPage() {
  const { data, isLoading, isError } = usePlatformConfig()
  const updateMutation = useUpdatePlatformConfig()

  const handleSave = async (updates: PlatformConfig) => {
    try {
      await updateMutation.mutateAsync(updates)
      toast.success('Configuration updated')
    } catch (e: unknown) {
      toast.error(e instanceof Error ? e.message : 'Failed to update configuration')
    }
  }

  if (isLoading) {
    return (
      <div className="space-y-6">
        <Skeleton className="h-10 w-64" />
        <Skeleton className="h-96 w-full" />
      </div>
    )
  }

  if (isError) {
    return (
      <div className="space-y-6">
        <ResourceHeader title="Platform Configuration" />
        <EmptyState icon={Settings} title="Error" description="Failed to load platform configuration." />
      </div>
    )
  }

  const config = data || {}

  return (
    <div className="space-y-6">
      <ResourceHeader
        title="Platform Configuration"
        subtitle="Manage platform-wide settings"
      />

      <Tabs defaultValue="webhooks">
        <TabsList>
          <TabsTrigger value="webhooks">
            <Webhook className="mr-2 h-4 w-4" />
            Webhooks
          </TabsTrigger>
          <TabsTrigger value="raw">
            <Code className="mr-2 h-4 w-4" />
            Raw JSON
          </TabsTrigger>
        </TabsList>
        <TabsContent value="webhooks" className="mt-4">
          <WebhooksTab config={config} onSave={handleSave} isSaving={updateMutation.isPending} />
        </TabsContent>
        <TabsContent value="raw" className="mt-4">
          <RawJsonTab config={config} onSave={handleSave} isSaving={updateMutation.isPending} />
        </TabsContent>
      </Tabs>
    </div>
  )
}
