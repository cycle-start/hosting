import { useState } from 'react'
import { type ColumnDef } from '@tanstack/react-table'
import { ShieldAlert } from 'lucide-react'
import { useCapabilityGaps, useUpdateCapabilityGap, useGapIncidents } from '@/lib/hooks'
import type { CapabilityGap } from '@/lib/types'
import { formatRelative } from '@/lib/utils'
import { DataTable } from '@/components/shared/data-table'
import { StatusBadge } from '@/components/shared/status-badge'
import { ResourceHeader } from '@/components/shared/resource-header'
import { EmptyState } from '@/components/shared/empty-state'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Card, CardContent } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { MoreHorizontal } from 'lucide-react'

function GapIncidentsDialog({ gapId, toolName, open, onOpenChange }: { gapId: string; toolName: string; open: boolean; onOpenChange: (open: boolean) => void }) {
  const { data, isLoading } = useGapIncidents(open ? gapId : '')

  const incidents = data?.items ?? []

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-2xl">
        <DialogHeader>
          <DialogTitle>Incidents linked to {toolName}</DialogTitle>
        </DialogHeader>
        {isLoading ? (
          <Skeleton className="h-32 w-full" />
        ) : incidents.length === 0 ? (
          <p className="text-sm text-muted-foreground py-4 text-center">No incidents linked to this gap.</p>
        ) : (
          <div className="space-y-3 max-h-96 overflow-y-auto">
            {incidents.map((inc) => (
              <Card key={inc.id}>
                <CardContent className="py-3 px-4">
                  <div className="flex items-start justify-between gap-4">
                    <div className="min-w-0 flex-1">
                      <div className="flex items-center gap-2 mb-1">
                        <a href={`/incidents/${inc.id}`} className="text-sm font-medium hover:underline">{inc.title}</a>
                        <StatusBadge status={inc.status} />
                        <StatusBadge status={inc.severity} />
                      </div>
                      <p className="text-xs text-muted-foreground">{inc.type} | {inc.source}</p>
                    </div>
                    <span className="text-xs text-muted-foreground whitespace-nowrap">
                      {formatRelative(inc.created_at)}
                    </span>
                  </div>
                </CardContent>
              </Card>
            ))}
          </div>
        )}
      </DialogContent>
    </Dialog>
  )
}

export function CapabilityGapsPage() {
  const [status, setStatus] = useState('')
  const [category, setCategory] = useState('')

  const { data, isLoading } = useCapabilityGaps({
    status: status || undefined,
    ...(category ? { category } : {}),
  } as Record<string, string>)

  const updateMutation = useUpdateCapabilityGap()
  const [incidentsGap, setIncidentsGap] = useState<{ id: string; tool_name: string } | null>(null)

  const gaps = data?.items ?? []

  const columns: ColumnDef<CapabilityGap>[] = [
    {
      accessorKey: 'tool_name',
      header: 'Tool',
      cell: ({ row }) => (
        <span className="font-mono text-sm font-medium">{row.original.tool_name}</span>
      ),
    },
    {
      accessorKey: 'description',
      header: 'Description',
      cell: ({ row }) => (
        <span className="text-sm text-muted-foreground line-clamp-2">{row.original.description}</span>
      ),
    },
    {
      accessorKey: 'category',
      header: 'Category',
      cell: ({ row }) => (
        <span className="text-sm capitalize">{row.original.category}</span>
      ),
    },
    {
      accessorKey: 'occurrences',
      header: 'Hits',
      cell: ({ row }) => (
        <div className="flex items-center gap-2">
          <div
            className="h-2 rounded-full bg-primary"
            style={{ width: `${Math.min(row.original.occurrences * 8, 80)}px` }}
          />
          <span className="text-sm font-medium">{row.original.occurrences}</span>
        </div>
      ),
    },
    {
      accessorKey: 'status',
      header: 'Status',
      cell: ({ row }) => <StatusBadge status={row.original.status} />,
    },
    {
      accessorKey: 'created_at',
      header: 'First Seen',
      cell: ({ row }) => (
        <span className="text-sm text-muted-foreground">{formatRelative(row.original.created_at)}</span>
      ),
    },
    {
      id: 'actions',
      cell: ({ row }) => (
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button variant="ghost" size="icon" onClick={(e) => e.stopPropagation()}>
              <MoreHorizontal className="h-4 w-4" />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end">
            <DropdownMenuItem
              onClick={() => setIncidentsGap({ id: row.original.id, tool_name: row.original.tool_name })}
            >
              View Incidents
            </DropdownMenuItem>
            {row.original.status === 'open' && (
              <>
                <DropdownMenuItem
                  onClick={() => updateMutation.mutate({ id: row.original.id, status: 'implemented' })}
                >
                  Mark Implemented
                </DropdownMenuItem>
                <DropdownMenuItem
                  onClick={() => updateMutation.mutate({ id: row.original.id, status: 'wont_fix' })}
                >
                  Won't Fix
                </DropdownMenuItem>
              </>
            )}
            {row.original.status !== 'open' && (
              <DropdownMenuItem
                onClick={() => updateMutation.mutate({ id: row.original.id, status: 'open' })}
              >
                Reopen
              </DropdownMenuItem>
            )}
          </DropdownMenuContent>
        </DropdownMenu>
      ),
    },
  ]

  return (
    <div className="space-y-6">
      <ResourceHeader
        title="Capability Gaps"
        subtitle={`${gaps.length} gaps tracked`}
      />

      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <div className="space-y-2">
          <Label>Status</Label>
          <Select value={status || 'all'} onValueChange={(v) => setStatus(v === 'all' ? '' : v)}>
            <SelectTrigger><SelectValue placeholder="All statuses" /></SelectTrigger>
            <SelectContent>
              <SelectItem value="all">All statuses</SelectItem>
              <SelectItem value="open">Open</SelectItem>
              <SelectItem value="implemented">Implemented</SelectItem>
              <SelectItem value="wont_fix">Won't Fix</SelectItem>
            </SelectContent>
          </Select>
        </div>
        <div className="space-y-2">
          <Label>Category</Label>
          <Select value={category || 'all'} onValueChange={(v) => setCategory(v === 'all' ? '' : v)}>
            <SelectTrigger><SelectValue placeholder="All categories" /></SelectTrigger>
            <SelectContent>
              <SelectItem value="all">All categories</SelectItem>
              <SelectItem value="investigation">Investigation</SelectItem>
              <SelectItem value="remediation">Remediation</SelectItem>
              <SelectItem value="notification">Notification</SelectItem>
            </SelectContent>
          </Select>
        </div>
      </div>

      {!isLoading && gaps.length === 0 ? (
        <EmptyState
          icon={ShieldAlert}
          title="No capability gaps"
          description="No capability gaps have been reported yet."
        />
      ) : (
        <DataTable
          columns={columns}
          data={gaps}
          loading={isLoading}
          searchColumn="tool_name"
          searchPlaceholder="Search gaps..."
        />
      )}

      {incidentsGap && (
        <GapIncidentsDialog
          gapId={incidentsGap.id}
          toolName={incidentsGap.tool_name}
          open={!!incidentsGap}
          onOpenChange={(open) => { if (!open) setIncidentsGap(null) }}
        />
      )}
    </div>
  )
}
