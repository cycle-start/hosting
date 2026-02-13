import { useState } from 'react'
import { type ColumnDef } from '@tanstack/react-table'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select, SelectContent, SelectItem, SelectTrigger, SelectValue,
} from '@/components/ui/select'
import { Badge } from '@/components/ui/badge'
import { ResourceHeader } from '@/components/shared/resource-header'
import { DataTable } from '@/components/shared/data-table'
import { CopyButton } from '@/components/shared/copy-button'
import { formatDate, truncateID } from '@/lib/utils'
import { useAuditLogs } from '@/lib/hooks'
import type { AuditLogEntry } from '@/lib/types'

const methodColors: Record<string, string> = {
  GET: 'bg-blue-500/10 text-blue-500',
  POST: 'bg-emerald-500/10 text-emerald-500',
  PUT: 'bg-yellow-500/10 text-yellow-500',
  PATCH: 'bg-orange-500/10 text-orange-500',
  DELETE: 'bg-red-500/10 text-red-500',
}

const columns: ColumnDef<AuditLogEntry>[] = [
  {
    accessorKey: 'id',
    header: 'ID',
    cell: ({ row }) => (
      <div className="flex items-center gap-1">
        <code className="text-xs">{truncateID(row.original.id)}</code>
        <CopyButton value={row.original.id} />
      </div>
    ),
  },
  {
    accessorKey: 'method',
    header: 'Method',
    cell: ({ row }) => (
      <span className={`inline-flex rounded-full px-2 py-0.5 text-xs font-semibold ${methodColors[row.original.method] || ''}`}>
        {row.original.method}
      </span>
    ),
  },
  {
    accessorKey: 'path',
    header: 'Path',
    cell: ({ row }) => <code className="text-xs">{row.original.path}</code>,
  },
  { accessorKey: 'resource_type', header: 'Resource Type' },
  {
    accessorKey: 'resource_id',
    header: 'Resource ID',
    cell: ({ row }) => row.original.resource_id
      ? <code className="text-xs">{truncateID(row.original.resource_id)}</code>
      : <span className="text-muted-foreground">-</span>,
  },
  {
    accessorKey: 'status_code',
    header: 'Status',
    cell: ({ row }) => {
      const code = row.original.status_code
      const color = code >= 400 ? 'destructive' : code >= 300 ? 'secondary' : 'default'
      return <Badge variant={color as 'default'}>{code}</Badge>
    },
  },
  {
    accessorKey: 'created_at',
    header: 'Time',
    cell: ({ row }) => <span className="text-sm text-muted-foreground">{formatDate(row.original.created_at)}</span>,
  },
]

export function AuditLogPage() {
  const [resourceType, setResourceType] = useState('')
  const [method, setMethod] = useState('')
  const [dateFrom, setDateFrom] = useState('')
  const [dateTo, setDateTo] = useState('')

  const { data, isLoading } = useAuditLogs({
    resource_type: resourceType || undefined,
    action: method || undefined,
    date_from: dateFrom || undefined,
    date_to: dateTo || undefined,
    limit: 50,
  })

  const entries = data?.items ?? []

  return (
    <div className="space-y-6">
      <ResourceHeader title="Audit Log" subtitle="API request history" />

      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <div className="space-y-2">
          <Label>Resource Type</Label>
          <Input
            placeholder="e.g. tenant"
            value={resourceType}
            onChange={(e) => setResourceType(e.target.value)}
          />
        </div>
        <div className="space-y-2">
          <Label>Method</Label>
          <Select value={method} onValueChange={(v) => setMethod(v === 'all' ? '' : v)}>
            <SelectTrigger><SelectValue placeholder="All methods" /></SelectTrigger>
            <SelectContent>
              <SelectItem value="all">All methods</SelectItem>
              <SelectItem value="GET">GET</SelectItem>
              <SelectItem value="POST">POST</SelectItem>
              <SelectItem value="PUT">PUT</SelectItem>
              <SelectItem value="PATCH">PATCH</SelectItem>
              <SelectItem value="DELETE">DELETE</SelectItem>
            </SelectContent>
          </Select>
        </div>
        <div className="space-y-2">
          <Label>From</Label>
          <Input type="datetime-local" value={dateFrom} onChange={(e) => setDateFrom(e.target.value)} />
        </div>
        <div className="space-y-2">
          <Label>To</Label>
          <Input type="datetime-local" value={dateTo} onChange={(e) => setDateTo(e.target.value)} />
        </div>
      </div>

      <DataTable columns={columns} data={entries} loading={isLoading} emptyMessage="No audit log entries" />
    </div>
  )
}
