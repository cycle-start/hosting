import { useState, useMemo, useRef, useEffect } from 'react'
import { ChevronDown, ChevronRight, ExternalLink, Pause, Play, ScrollText } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Skeleton } from '@/components/ui/skeleton'
import { Tooltip, TooltipContent, TooltipTrigger, TooltipProvider } from '@/components/ui/tooltip'
import { EmptyState } from '@/components/shared/empty-state'
import { cn, formatRelative, formatDate } from '@/lib/utils'
import { useLogs } from '@/lib/hooks'
import type { LogEntry } from '@/lib/types'

interface LogViewerProps {
  query: string
  title?: string
}

interface ParsedLogLine {
  level?: string
  msg?: string
  error?: string
  [key: string]: unknown
}

const TIME_RANGES = [
  { label: '15m', value: '15m' },
  { label: '1h', value: '1h' },
  { label: '6h', value: '6h' },
  { label: '24h', value: '24h' },
  { label: '7d', value: '7d' },
]

const SERVICE_COLORS: Record<string, string> = {
  'core-api': 'bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200',
  'worker': 'bg-purple-100 text-purple-800 dark:bg-purple-900 dark:text-purple-200',
  'node-agent': 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200',
}

const LEVEL_COLORS: Record<string, string> = {
  'debug': 'bg-gray-100 text-gray-600 dark:bg-gray-800 dark:text-gray-400',
  'info': 'bg-gray-100 text-gray-600 dark:bg-gray-800 dark:text-gray-400',
  'warn': 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-200',
  'warning': 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-200',
  'error': 'bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200',
  'fatal': 'bg-red-200 text-red-900 dark:bg-red-950 dark:text-red-100',
}

function parseLogLine(line: string): ParsedLogLine {
  try {
    return JSON.parse(line)
  } catch {
    return { msg: line }
  }
}

function LogEntryRow({ entry }: { entry: LogEntry }) {
  const [expanded, setExpanded] = useState(false)
  const parsed = useMemo(() => parseLogLine(entry.line), [entry.line])
  const app = entry.labels?.app || entry.labels?.service_name || 'unknown'
  const level = parsed.level || 'info'

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

        <span className={cn('text-[10px] font-medium rounded px-1.5 py-0.5 shrink-0', SERVICE_COLORS[app] || 'bg-gray-100 text-gray-600 dark:bg-gray-800 dark:text-gray-400')}>
          {app}
        </span>

        <span className={cn('text-[10px] font-medium rounded px-1.5 py-0.5 shrink-0 uppercase', LEVEL_COLORS[level] || LEVEL_COLORS['info'])}>
          {level}
        </span>

        <span className="truncate text-foreground">
          {parsed.msg || entry.line}
        </span>

        {parsed.error && (
          <span className="truncate text-destructive text-xs ml-1">
            {parsed.error}
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

export function LogViewer({ query, title }: LogViewerProps) {
  const [timeRange, setTimeRange] = useState('1h')
  const [paused, setPaused] = useState(false)
  const [serviceFilter, setServiceFilter] = useState<string>('all')
  const scrollRef = useRef<HTMLDivElement>(null)

  const { data, isLoading } = useLogs(query, timeRange, !paused)

  const entries = data?.entries ?? []

  // Get unique services from results
  const services = useMemo(() => {
    const apps = new Set<string>()
    entries.forEach(e => {
      const app = e.labels?.app || e.labels?.service_name
      if (app) apps.add(app)
    })
    return Array.from(apps).sort()
  }, [entries])

  // Filter by selected service
  const filteredEntries = useMemo(() => {
    if (serviceFilter === 'all') return entries
    return entries.filter(e => (e.labels?.app || e.labels?.service_name) === serviceFilter)
  }, [entries, serviceFilter])

  // Auto-scroll to bottom when new entries arrive
  useEffect(() => {
    if (scrollRef.current && !paused) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight
    }
  }, [filteredEntries.length, paused])

  const grafanaUrl = `http://grafana.hosting.test/explore?orgId=1&left=${encodeURIComponent(JSON.stringify({ datasource: 'loki', queries: [{ expr: query, refId: 'A' }] }))}`

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

        <Select value={serviceFilter} onValueChange={setServiceFilter}>
          <SelectTrigger className="w-36 h-8">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">All services</SelectItem>
            {services.map(s => (
              <SelectItem key={s} value={s}>{s}</SelectItem>
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
          {filteredEntries.length} entries
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
      ) : filteredEntries.length === 0 ? (
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
          {filteredEntries.map((entry, i) => (
            <LogEntryRow key={`${entry.timestamp}-${i}`} entry={entry} />
          ))}
        </div>
      )}
    </div>
  )
}
