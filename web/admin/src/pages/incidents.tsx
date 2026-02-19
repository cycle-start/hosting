import { useState } from 'react'
import { useNavigate } from '@tanstack/react-router'
import { type ColumnDef } from '@tanstack/react-table'
import { AlertCircle } from 'lucide-react'
import { useIncidents } from '@/lib/hooks'
import type { Incident } from '@/lib/types'
import { formatRelative } from '@/lib/utils'
import { DataTable } from '@/components/shared/data-table'
import { StatusBadge } from '@/components/shared/status-badge'
import { ResourceHeader } from '@/components/shared/resource-header'
import { EmptyState } from '@/components/shared/empty-state'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'

export function IncidentsPage() {
  const navigate = useNavigate()
  const [status, setStatus] = useState('')
  const [severity, setSeverity] = useState('')
  const [type, setType] = useState('')
  const [source, setSource] = useState('')

  const { data, isLoading } = useIncidents({
    status: status || undefined,
    severity: severity || undefined,
    type: type || undefined,
    source: source || undefined,
    limit: 50,
  })

  const incidents = data?.items ?? []

  const openCount = incidents.filter(i => ['open', 'investigating', 'remediating'].includes(i.status)).length

  const columns: ColumnDef<Incident>[] = [
    {
      accessorKey: 'severity',
      header: 'Sev',
      cell: ({ row }) => <StatusBadge status={row.original.severity} />,
      size: 80,
    },
    {
      accessorKey: 'title',
      header: 'Title',
      cell: ({ row }) => (
        <div className="min-w-0">
          <span className="font-medium">{row.original.title}</span>
          <div className="text-xs text-muted-foreground mt-0.5">{row.original.type}</div>
        </div>
      ),
    },
    {
      accessorKey: 'status',
      header: 'Status',
      cell: ({ row }) => <StatusBadge status={row.original.status} />,
    },
    {
      accessorKey: 'source',
      header: 'Source',
      cell: ({ row }) => (
        <span className="text-sm text-muted-foreground">{row.original.source}</span>
      ),
    },
    {
      accessorKey: 'assigned_to',
      header: 'Assigned',
      cell: ({ row }) => (
        <span className="text-sm text-muted-foreground">
          {row.original.assigned_to || '-'}
        </span>
      ),
    },
    {
      accessorKey: 'detected_at',
      header: 'Detected',
      cell: ({ row }) => (
        <span className="text-sm text-muted-foreground">
          {formatRelative(row.original.detected_at)}
        </span>
      ),
    },
  ]

  return (
    <div className="space-y-6">
      <ResourceHeader
        title="Incidents"
        subtitle={`${incidents.length} incidents${openCount > 0 ? ` (${openCount} active)` : ''}`}
      />

      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <div className="space-y-2">
          <Label>Status</Label>
          <Select value={status || 'all'} onValueChange={(v) => setStatus(v === 'all' ? '' : v)}>
            <SelectTrigger><SelectValue placeholder="All statuses" /></SelectTrigger>
            <SelectContent>
              <SelectItem value="all">All statuses</SelectItem>
              <SelectItem value="open">Open</SelectItem>
              <SelectItem value="investigating">Investigating</SelectItem>
              <SelectItem value="remediating">Remediating</SelectItem>
              <SelectItem value="escalated">Escalated</SelectItem>
              <SelectItem value="resolved">Resolved</SelectItem>
              <SelectItem value="cancelled">Cancelled</SelectItem>
            </SelectContent>
          </Select>
        </div>
        <div className="space-y-2">
          <Label>Severity</Label>
          <Select value={severity || 'all'} onValueChange={(v) => setSeverity(v === 'all' ? '' : v)}>
            <SelectTrigger><SelectValue placeholder="All severities" /></SelectTrigger>
            <SelectContent>
              <SelectItem value="all">All severities</SelectItem>
              <SelectItem value="critical">Critical</SelectItem>
              <SelectItem value="warning">Warning</SelectItem>
              <SelectItem value="info">Info</SelectItem>
            </SelectContent>
          </Select>
        </div>
        <div className="space-y-2">
          <Label>Type</Label>
          <Input
            placeholder="Filter by type..."
            value={type}
            onChange={(e) => setType(e.target.value)}
          />
        </div>
        <div className="space-y-2">
          <Label>Source</Label>
          <Input
            placeholder="Filter by source..."
            value={source}
            onChange={(e) => setSource(e.target.value)}
          />
        </div>
      </div>

      {!isLoading && incidents.length === 0 ? (
        <EmptyState
          icon={AlertCircle}
          title="No incidents"
          description="No incidents match the current filters."
        />
      ) : (
        <DataTable
          columns={columns}
          data={incidents}
          loading={isLoading}
          searchColumn="title"
          searchPlaceholder="Search incidents..."
          onRowClick={(inc) => navigate({ to: '/incidents/$id', params: { id: inc.id } })}
        />
      )}
    </div>
  )
}
