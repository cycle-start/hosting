import { useState, useEffect } from 'react'
import { Save, Settings } from 'lucide-react'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Textarea } from '@/components/ui/textarea'
import { Skeleton } from '@/components/ui/skeleton'
import { ResourceHeader } from '@/components/shared/resource-header'
import { EmptyState } from '@/components/shared/empty-state'
import { usePlatformConfig, useUpdatePlatformConfig } from '@/lib/hooks'

export function PlatformConfigPage() {
  const { data, isLoading, isError } = usePlatformConfig()
  const updateMutation = useUpdatePlatformConfig()
  const [jsonText, setJsonText] = useState('')
  const [parseError, setParseError] = useState('')

  useEffect(() => {
    if (data) {
      setJsonText(JSON.stringify(data, null, 2))
    }
  }, [data])

  const handleSave = async () => {
    setParseError('')
    let parsed
    try {
      parsed = JSON.parse(jsonText)
    } catch {
      setParseError('Invalid JSON')
      return
    }

    try {
      await updateMutation.mutateAsync(parsed)
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

  return (
    <div className="space-y-6">
      <ResourceHeader
        title="Platform Configuration"
        subtitle="Edit the platform configuration as JSON"
        actions={
          <Button onClick={handleSave} disabled={updateMutation.isPending}>
            <Save className="mr-2 h-4 w-4" />
            {updateMutation.isPending ? 'Saving...' : 'Save'}
          </Button>
        }
      />

      <div className="space-y-2">
        <Textarea
          className="min-h-[500px] font-mono text-sm"
          value={jsonText}
          onChange={(e) => { setJsonText(e.target.value); setParseError('') }}
        />
        {parseError && <p className="text-sm text-destructive">{parseError}</p>}
      </div>
    </div>
  )
}
