import { useState, useEffect } from 'react'
import { Save, Settings, Webhook, Code, Bot, Plus, X } from 'lucide-react'
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

const COMMON_INCIDENT_TYPES = [
  'disk_pressure',
  'node_health_missing',
  'convergence_stuck',
  'replication_lag',
  'replication_broken',
  'cert_expiring',
  'cephfs_unmounted',
]

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

function AgentTab({ config, onSave, isSaving }: {
  config: PlatformConfig
  onSave: (updates: PlatformConfig) => void
  isSaving: boolean
}) {
  const [systemPrompt, setSystemPrompt] = useState('')
  const [concurrencyEntries, setConcurrencyEntries] = useState<{ type: string; limit: string }[]>([])

  useEffect(() => {
    setSystemPrompt(config['agent.system_prompt'] || '')

    const entries: { type: string; limit: string }[] = []
    for (const key of Object.keys(config)) {
      if (key.startsWith('agent.concurrency.')) {
        const typeName = key.slice('agent.concurrency.'.length)
        entries.push({ type: typeName, limit: config[key] || '' })
      }
    }
    setConcurrencyEntries(entries)
  }, [config])

  const handleSave = () => {
    const updates: PlatformConfig = {}
    updates['agent.system_prompt'] = systemPrompt
    for (const entry of concurrencyEntries) {
      if (entry.type.trim()) {
        updates[`agent.concurrency.${entry.type.trim()}`] = entry.limit
      }
    }
    onSave(updates)
  }

  const addEntry = () => {
    setConcurrencyEntries(prev => [...prev, { type: '', limit: '1' }])
  }

  const removeEntry = (index: number) => {
    setConcurrencyEntries(prev => prev.filter((_, i) => i !== index))
  }

  const updateEntry = (index: number, field: 'type' | 'limit', value: string) => {
    setConcurrencyEntries(prev => prev.map((entry, i) =>
      i === index ? { ...entry, [field]: value } : entry
    ))
  }

  return (
    <div className="space-y-4">
      <Card>
        <CardHeader>
          <CardTitle className="text-base">System Prompt</CardTitle>
          <CardDescription>
            Override the default system prompt for the LLM investigation agent. Leave empty to use the built-in default.
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-2">
          <Textarea
            className="min-h-[200px] font-mono"
            placeholder="Leave empty to use the built-in default system prompt..."
            value={systemPrompt}
            onChange={e => setSystemPrompt(e.target.value)}
          />
          <p className="text-xs text-muted-foreground">
            The default system prompt covers platform architecture, responsibilities, decision framework, and constraints.
          </p>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="text-base">Investigation Concurrency</CardTitle>
          <CardDescription>
            Configure how many follower incidents can be investigated in parallel per incident type. Types not listed here use the global default (AGENT_FOLLOWER_CONCURRENT).
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          {concurrencyEntries.length > 0 && (
            <div className="space-y-2">
              {concurrencyEntries.map((entry, index) => (
                <div key={index} className="flex items-center gap-2">
                  <Input
                    className="flex-1"
                    placeholder="Incident type (e.g., replication_lag)"
                    value={entry.type}
                    onChange={e => updateEntry(index, 'type', e.target.value)}
                  />
                  <Input
                    className="w-24"
                    type="number"
                    min={1}
                    placeholder="Limit"
                    value={entry.limit}
                    onChange={e => updateEntry(index, 'limit', e.target.value)}
                  />
                  <Button
                    variant="ghost"
                    size="icon"
                    onClick={() => removeEntry(index)}
                  >
                    <X className="h-4 w-4" />
                  </Button>
                </div>
              ))}
            </div>
          )}
          <Button variant="outline" size="sm" onClick={addEntry}>
            <Plus className="mr-2 h-4 w-4" />
            Add Type
          </Button>
          <p className="text-xs text-muted-foreground">
            Common types: {COMMON_INCIDENT_TYPES.join(', ')}
          </p>
        </CardContent>
      </Card>

      <div className="flex justify-end">
        <Button onClick={handleSave} disabled={isSaving}>
          <Save className="mr-2 h-4 w-4" />
          {isSaving ? 'Saving...' : 'Save Agent Settings'}
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
          <TabsTrigger value="agent">
            <Bot className="mr-2 h-4 w-4" />
            Agent
          </TabsTrigger>
          <TabsTrigger value="raw">
            <Code className="mr-2 h-4 w-4" />
            Raw JSON
          </TabsTrigger>
        </TabsList>
        <TabsContent value="webhooks" className="mt-4">
          <WebhooksTab config={config} onSave={handleSave} isSaving={updateMutation.isPending} />
        </TabsContent>
        <TabsContent value="agent" className="mt-4">
          <AgentTab config={config} onSave={handleSave} isSaving={updateMutation.isPending} />
        </TabsContent>
        <TabsContent value="raw" className="mt-4">
          <RawJsonTab config={config} onSave={handleSave} isSaving={updateMutation.isPending} />
        </TabsContent>
      </Tabs>
    </div>
  )
}
