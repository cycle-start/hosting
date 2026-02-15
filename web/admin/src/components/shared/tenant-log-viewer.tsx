import { useState, useMemo, useRef, useEffect } from 'react'
import { ChevronDown, ChevronRight, ExternalLink, Pause, Play, ScrollText } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Skeleton } from '@/components/ui/skeleton'
import { Tooltip, TooltipContent, TooltipTrigger, TooltipProvider } from '@/components/ui/tooltip'
import { EmptyState } from '@/components/shared/empty-state'
import { cn, formatRelative, formatDate } from '@/lib/utils'
import { useTenantLogs } from '@/lib/hooks'
import type { LogEntry } from '@/lib/types'

interface TenantLogViewerProps {
  tenantId: string
  webrootId?: string
  title?: string
}

interface ParsedAccessLog {
  method?: string
  uri?: string
  status?: number
  [key: string]: unknown
}

const TIME_RANGES = [
  { label: '15m', value: '15m' },
  { label: '1h', value: '1h' },
  { label: '6h', value: '6h' },
  { label: '24h', value: '24h' },
  { label: '7d', value: '7d' },
]

const LOG_TYPES = [
  { label: 'All', value: 'all' },
  { label: 'Access', value: 'access' },
  { label: 'Error', value: 'error' },
  { label: 'PHP Error', value: 'php-error' },
  { label: 'PHP Slow', value: 'php-slow' },
  { label: 'Application', value: 'app' },
]

const LOG_TYPE_COLORS: Record<string, string> = {
  'access': 'bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200',
  'error': 'bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200',
  'php-error': 'bg-orange-100 text-orange-800 dark:bg-orange-900 dark:text-orange-200',
  'php-slow': 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-200',
  'app': 'bg-purple-100 text-purple-800 dark:bg-purple-900 dark:text-purple-200',
}

const STATUS_COLORS: Record<string, string> = {
  '2': 'text-green-600 dark:text-green-400',
  '3': 'text-blue-600 dark:text-blue-400',
  '4': 'text-yellow-600 dark:text-yellow-400',
  '5': 'text-red-600 dark:text-red-400',
}

function parseLogLine(line: string): ParsedAccessLog {
  try {
    return JSON.parse(line)
  } catch {
    return { msg: line }
  }
}

function getStatusColor(status?: number): string {
  if (!status) return ''
  const category = String(status).charAt(0)
  return STATUS_COLORS[category] || ''
}

function TenantLogEntryRow({ entry }: { entry: LogEntry }) {
  const [expanded, setExpanded] = useState(false)
  const parsed = useMemo(() => parseLogLine(entry.line), [entry.line])
  const logType = entry.labels?.log_type || 'unknown'
  const isAccess = logType === 'access'

  const summary = useMemo(() => {
    if (isAccess && parsed.method && parsed.uri) {
      const status = parsed.status ? ` ${parsed.status}` : ''
      return `${parsed.method} ${parsed.uri}${status}`
    }
    return (parsed as Record<string, unknown>).msg as string || (parsed as Record<string, unknown>).message as string || entry.line
  }, [isAccess, parsed, entry.line])

  return (
    <div className="border-b border-border/50 last:border-0">
      <button
        onClick={() => setExpanded(!expanded)}
        className="w-full flex items-start gap-2 px-3 py-1.5 text-left hover:bg-muted/50 transition-colors text-sm"
      >
        {expanded ? (
          <ChevronDown className="h-3.5 w-3.5 mt-0.5 shrink-0 text-muted-foreground" />
        ) : (
          <ChevronRight className="h-3.5 w-3.5 mt-0.5 shrink-0 text-muted-foreground" />
        )}

        <TooltipProvider>
          <Tooltip>
            <TooltipTrigger asChild>
              <span className="text-xs text-muted-foreground shrink-0 w-16 tabular-nums">
                {formatRelative(entry.timestamp)}
              </span>
            </TooltipTrigger>
            <TooltipContent side="top">
              <p className="text-xs">{formatDate(entry.timestamp)}</p>
            </TooltipContent>
          </Tooltip>
        </TooltipProvider>

        <span className={cn('text-[10px] font-medium rounded px-1.5 py-0.5 shrink-0', LOG_TYPE_COLORS[logType] || 'bg-gray-100 text-gray-600 dark:bg-gray-800 dark:text-gray-400')}>
          {logType}
        </span>

        {isAccess && parsed.status ? (
          <span className={cn('truncate text-foreground', getStatusColor(parsed.status as number))}>
            {summary}
          </span>
        ) : (
          <span className="truncate text-foreground">
            {summary}
          </span>
        )}
      </button>

      {expanded && (
        <div className="px-3 pb-2 pl-9">
          <pre className="text-xs bg-muted/50 rounded p-2 overflow-x-auto whitespace-pre-wrap break-all font-mono">
            {JSON.stringify(parsed, null, 2)}
          </pre>
        </div>
      )}
    </div>
  )
}

export function TenantLogViewer({ tenantId, webrootId: fixedWebrootId, title }: TenantLogViewerProps) {
  const [timeRange, setTimeRange] = useState('1h')
  const [paused, setPaused] = useState(false)
  const [logType, setLogType] = useState<string>('all')
  const scrollRef = useRef<HTMLDivElement>(null)

  const { data, isLoading } = useTenantLogs(
    tenantId,
    logType,
    fixedWebrootId || undefined,
    timeRange,
    !paused,
  )

  const entries = data?.entries ?? []

  // Auto-scroll to bottom when new entries arrive
  useEffect(() => {
    if (scrollRef.current && !paused) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight
    }
  }, [entries.length, paused])

  const grafanaUrl = `http://grafana.hosting.test/explore?orgId=1&left=${encodeURIComponent(JSON.stringify({ datasource: 'tenant-loki', queries: [{ expr: `{tenant_id="${tenantId}"}`, refId: 'A' }] }))}`

  return (
    <div className="space-y-3">
      {title && <h3 className="text-lg font-semibold">{title}</h3>}

      {/* Toolbar */}
      <div className="flex items-center gap-2 flex-wrap">
        <Select value={timeRange} onValueChange={setTimeRange}>
          <SelectTrigger className="w-24 h-8">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {TIME_RANGES.map(r => (
              <SelectItem key={r.value} value={r.value}>{r.label}</SelectItem>
            ))}
          </SelectContent>
        </Select>

        <Select value={logType} onValueChange={setLogType}>
          <SelectTrigger className="w-36 h-8">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {LOG_TYPES.map(t => (
              <SelectItem key={t.value} value={t.value}>{t.label}</SelectItem>
            ))}
          </SelectContent>
        </Select>

        <Button
          variant="outline"
          size="sm"
          className="h-8"
          onClick={() => setPaused(!paused)}
        >
          {paused ? <Play className="h-3.5 w-3.5 mr-1" /> : <Pause className="h-3.5 w-3.5 mr-1" />}
          {paused ? 'Resume' : 'Pause'}
        </Button>

        <div className="flex-1" />

        <span className="text-xs text-muted-foreground">
          {entries.length} entries
        </span>

        <a
          href={grafanaUrl}
          target="_blank"
          rel="noopener noreferrer"
          className="inline-flex items-center gap-1 text-xs text-muted-foreground hover:text-foreground transition-colors"
        >
          <ExternalLink className="h-3 w-3" />
          Grafana
        </a>
      </div>

      {/* Log stream */}
      {isLoading ? (
        <div className="space-y-2">
          <Skeleton className="h-6 w-full" />
          <Skeleton className="h-6 w-full" />
          <Skeleton className="h-6 w-full" />
          <Skeleton className="h-6 w-3/4" />
        </div>
      ) : entries.length === 0 ? (
        <EmptyState
          icon={ScrollText}
          title="No logs found"
          description="No logs found for this time range. Try increasing the time window."
        />
      ) : (
        <div
          ref={scrollRef}
          className="border rounded-md max-h-[500px] overflow-y-auto bg-background"
        >
          {entries.map((entry, i) => (
            <TenantLogEntryRow key={`${entry.timestamp}-${i}`} entry={entry} />
          ))}
        </div>
      )}
    </div>
  )
}
