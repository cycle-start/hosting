import { useEffect, useState, useCallback } from 'react'
import { Button } from '@/components/ui/button'
import { Check, AlertCircle, Loader2, RefreshCw, Copy, CheckCircle2, Bug } from 'lucide-react'
import { cn } from '@/lib/utils'
import * as api from '@/lib/api'
import type { PodInfo, PodDebugResponse, PodsResponse } from '@/lib/types'

export function ControlPlaneStatus({ outputDir }: { outputDir: string }) {
  const [data, setData] = useState<PodsResponse | null>(null)
  const [loading, setLoading] = useState(false)
  const [copied, setCopied] = useState(false)
  const [expandedPod, setExpandedPod] = useState<string | null>(null)

  const fetchPods = useCallback(async () => {
    try {
      setLoading(true)
      const res = await api.getPods()
      setData(res)
    } catch {
      // Silently ignore — endpoint returns 200 with error in body
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    fetchPods()
    const interval = setInterval(fetchPods, 5000)
    return () => clearInterval(interval)
  }, [fetchPods])

  // Don't render anything until kubeconfig exists
  if (!data || !data.available) return null

  const pods = data.pods || []
  const allReady = pods.length > 0 && pods.every(
    (p) => p.status === 'Running' && p.ready.split('/').every((n, i, arr) => i === 0 ? n === arr[1] : true)
  )

  const kubeconfigPath = data.kubeconfig_path || `${outputDir}/generated/kubeconfig.yaml`
  const snippet = `export KUBECONFIG=${kubeconfigPath}\nkubectl get pods -A`

  const copySnippet = () => {
    navigator.clipboard.writeText(snippet)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  return (
    <div className="rounded-lg border bg-card p-5 space-y-4">
      <div className="flex items-center justify-between">
        <h3 className="text-sm font-medium flex items-center gap-2">
          {allReady ? (
            <CheckCircle2 className="h-4 w-4 text-green-400" />
          ) : (
            <Loader2 className="h-4 w-4 text-primary animate-spin" />
          )}
          Control Plane Status
        </h3>
        <Button variant="outline" size="sm" onClick={fetchPods} disabled={loading}>
          <RefreshCw className={cn('h-3 w-3 mr-1.5', loading && 'animate-spin')} />
          Refresh
        </Button>
      </div>

      {/* kubectl connection snippet */}
      <div className="rounded-md border bg-[#0d1117] relative">
        <div className="flex items-center justify-between px-3 py-1.5 border-b border-white/10">
          <span className="text-[10px] text-[#6e7681] font-mono uppercase tracking-wider">Connect with kubectl</span>
          <button
            onClick={copySnippet}
            className="rounded p-1 text-[#6e7681] hover:text-[#c9d1d9] transition-colors"
            title="Copy to clipboard"
          >
            {copied ? <Check className="h-3 w-3 text-green-400" /> : <Copy className="h-3 w-3" />}
          </button>
        </div>
        <pre className="px-3 py-2 text-xs font-mono text-[#c9d1d9] whitespace-pre">{snippet}</pre>
      </div>

      {data.error && (
        <div className="rounded-md border border-destructive/50 bg-destructive/10 px-3 py-2">
          <p className="text-xs text-destructive flex items-center gap-1.5">
            <AlertCircle className="h-3 w-3 shrink-0" />
            {data.error}
          </p>
        </div>
      )}

      {/* Pod table */}
      {pods.length > 0 && (
        <div className="rounded-md border overflow-hidden">
          {allReady && (
            <div className="px-3 py-2 bg-green-500/10 border-b flex items-center gap-2">
              <Check className="h-3 w-3 text-green-400" />
              <span className="text-xs text-green-400 font-medium">All pods ready</span>
            </div>
          )}
          <div className="overflow-x-auto">
            <table className="w-full text-xs">
              <thead>
                <tr className="border-b bg-muted/50">
                  <th className="text-left px-3 py-2 font-medium text-muted-foreground">NAMESPACE</th>
                  <th className="text-left px-3 py-2 font-medium text-muted-foreground">NAME</th>
                  <th className="text-left px-3 py-2 font-medium text-muted-foreground">READY</th>
                  <th className="text-left px-3 py-2 font-medium text-muted-foreground">STATUS</th>
                  <th className="text-right px-3 py-2 font-medium text-muted-foreground">RESTARTS</th>
                  <th className="text-right px-3 py-2 font-medium text-muted-foreground">AGE</th>
                </tr>
              </thead>
              <tbody className="font-mono">
                {pods.map((pod) => {
                  const podKey = `${pod.namespace}/${pod.name}`
                  return (
                    <PodRow
                      key={podKey}
                      pod={pod}
                      expanded={expandedPod === podKey}
                      onToggleExpand={() => setExpandedPod(expandedPod === podKey ? null : podKey)}
                    />
                  )
                })}
              </tbody>
            </table>
          </div>
        </div>
      )}
    </div>
  )
}

const ERROR_STATUSES = new Set([
  'CrashLoopBackOff', 'Error', 'Failed', 'ImagePullBackOff', 'ErrImagePull', 'OOMKilled',
])

function isErrorStatus(status: string): boolean {
  return ERROR_STATUSES.has(status) || status.startsWith('Init:')
}

function PodRow({ pod, expanded, onToggleExpand }: { pod: PodInfo; expanded: boolean; onToggleExpand: () => void }) {
  const showDebug = isErrorStatus(pod.status)

  return (
    <>
      <tr className="border-b last:border-0 hover:bg-muted/30">
        <td className="px-3 py-1.5 text-muted-foreground">{pod.namespace}</td>
        <td className="px-3 py-1.5">{pod.name}</td>
        <td className="px-3 py-1.5">{pod.ready}</td>
        <td className="px-3 py-1.5">
          <StatusBadge status={pod.status} />
        </td>
        <td className="px-3 py-1.5 text-right">{pod.restarts}</td>
        <td className="px-3 py-1.5 text-right text-muted-foreground">
          <span className="inline-flex items-center gap-2">
            {pod.age}
            {showDebug && (
              <button
                onClick={onToggleExpand}
                className={cn(
                  'inline-flex items-center gap-1 rounded px-1.5 py-0.5 text-[10px] font-medium transition-colors',
                  expanded
                    ? 'bg-destructive/20 text-destructive'
                    : 'bg-destructive/10 text-destructive hover:bg-destructive/20',
                )}
              >
                <Bug className="h-3 w-3" />
                Debug
              </button>
            )}
          </span>
        </td>
      </tr>
      {expanded && showDebug && (
        <tr>
          <td colSpan={6} className="p-0">
            <PodDebugPanel namespace={pod.namespace} name={pod.name} />
          </td>
        </tr>
      )}
    </>
  )
}

function PodDebugPanel({ namespace, name }: { namespace: string; name: string }) {
  const [data, setData] = useState<PodDebugResponse | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [tab, setTab] = useState<'logs' | 'describe'>('logs')

  useEffect(() => {
    let cancelled = false
    setLoading(true)
    setError(null)
    api.getPodDebug(namespace, name)
      .then((res) => {
        if (!cancelled) {
          setData(res)
          if (res.error) setError(res.error)
        }
      })
      .catch((err) => {
        if (!cancelled) setError(String(err))
      })
      .finally(() => {
        if (!cancelled) setLoading(false)
      })
    return () => { cancelled = true }
  }, [namespace, name])

  if (loading) {
    return (
      <div className="flex items-center justify-center py-6 bg-[#0d1117]">
        <Loader2 className="h-4 w-4 animate-spin text-[#6e7681]" />
        <span className="ml-2 text-xs text-[#6e7681]">Fetching debug info...</span>
      </div>
    )
  }

  const content = tab === 'logs' ? (data?.logs || '') : (data?.describe || '')

  return (
    <div className="border-t bg-[#0d1117]">
      <div className="flex items-center gap-1 px-3 py-1.5 border-b border-white/10">
        <button
          onClick={() => setTab('logs')}
          className={cn(
            'rounded px-2 py-1 text-[10px] font-medium transition-colors',
            tab === 'logs' ? 'bg-white/10 text-[#c9d1d9]' : 'text-[#6e7681] hover:text-[#c9d1d9]',
          )}
        >
          Logs
        </button>
        <button
          onClick={() => setTab('describe')}
          className={cn(
            'rounded px-2 py-1 text-[10px] font-medium transition-colors',
            tab === 'describe' ? 'bg-white/10 text-[#c9d1d9]' : 'text-[#6e7681] hover:text-[#c9d1d9]',
          )}
        >
          Describe
        </button>
        {error && (
          <span className="ml-auto text-[10px] text-destructive flex items-center gap-1">
            <AlertCircle className="h-3 w-3" />
            {error}
          </span>
        )}
      </div>
      <pre className="px-3 py-2 text-xs font-mono text-[#c9d1d9] whitespace-pre max-h-80 overflow-y-auto">
        {content || '(empty)'}
      </pre>
    </div>
  )
}

function StatusBadge({ status }: { status: string }) {
  const color = statusColor(status)
  return (
    <span className={cn(
      'inline-flex items-center rounded-full px-1.5 py-0.5 text-[10px] font-medium',
      color,
    )}>
      {status}
    </span>
  )
}

function statusColor(status: string): string {
  switch (status) {
    case 'Running':
      return 'bg-green-500/15 text-green-400'
    case 'Succeeded':
    case 'Completed':
      return 'bg-green-500/15 text-green-400'
    case 'Pending':
    case 'ContainerCreating':
    case 'PodInitializing':
      return 'bg-yellow-500/15 text-yellow-400'
    case 'CrashLoopBackOff':
    case 'Error':
    case 'Failed':
    case 'ImagePullBackOff':
    case 'ErrImagePull':
    case 'OOMKilled':
      return 'bg-destructive/15 text-destructive'
    case 'Terminating':
      return 'bg-orange-500/15 text-orange-400'
    default:
      if (status.startsWith('Init:')) {
        return 'bg-yellow-500/15 text-yellow-400'
      }
      return 'bg-muted text-muted-foreground'
  }
}
