import { useState } from 'react'
import { useParams } from '@tanstack/react-router'
import { AlertCircle, CheckCircle, ArrowUpCircle, XCircle, Plus, MessageSquarePlus } from 'lucide-react'
import {
  useIncident,
  useIncidentEvents,
  useIncidentGaps,
  useResolveIncident,
  useEscalateIncident,
  useCancelIncident,
  useAddIncidentEvent,
} from '@/lib/hooks'
import { formatDate, formatRelative } from '@/lib/utils'
import { StatusBadge } from '@/components/shared/status-badge'
import { ResourceHeader } from '@/components/shared/resource-header'
import { Breadcrumb } from '@/components/shared/breadcrumb'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Textarea } from '@/components/ui/textarea'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'

export function IncidentDetailPage() {
  const { id } = useParams({ strict: false }) as { id: string }
  const { data: incident, isLoading } = useIncident(id)
  const { data: eventsData, isLoading: eventsLoading } = useIncidentEvents(id)
  const { data: gapsData, isLoading: gapsLoading } = useIncidentGaps(id)
  const resolveMutation = useResolveIncident()
  const escalateMutation = useEscalateIncident()
  const cancelMutation = useCancelIncident()
  const addEventMutation = useAddIncidentEvent()

  const [activeTab, setActiveTab] = useState('timeline')
  const [resolveOpen, setResolveOpen] = useState(false)
  const [escalateOpen, setEscalateOpen] = useState(false)
  const [cancelOpen, setCancelOpen] = useState(false)
  const [actionText, setActionText] = useState('')
  const [noteOpen, setNoteOpen] = useState(false)
  const [noteAction, setNoteAction] = useState('commented')
  const [noteDetail, setNoteDetail] = useState('')

  const events = eventsData?.items ?? []
  const gaps = gapsData?.items ?? []

  if (isLoading || !incident) {
    return (
      <div className="space-y-6">
        <Skeleton className="h-8 w-64" />
        <Skeleton className="h-64 w-full" />
      </div>
    )
  }

  const isActive = ['open', 'investigating', 'remediating'].includes(incident.status)

  return (
    <div className="space-y-6">
      <Breadcrumb
        segments={[
          { label: 'Incidents', href: '/incidents' },
          { label: incident.title },
        ]}
      />

      <ResourceHeader
        title={incident.title}
        subtitle={`${incident.type} | ${incident.source}`}
        status={incident.status}
        actions={
          isActive ? (
            <div className="flex gap-2">
              <Button
                variant="outline"
                size="sm"
                onClick={() => { setActionText(''); setResolveOpen(true) }}
              >
                <CheckCircle className="mr-2 h-4 w-4" /> Resolve
              </Button>
              <Button
                variant="outline"
                size="sm"
                onClick={() => { setActionText(''); setEscalateOpen(true) }}
              >
                <ArrowUpCircle className="mr-2 h-4 w-4" /> Escalate
              </Button>
              <Button
                variant="outline"
                size="sm"
                onClick={() => { setActionText(''); setCancelOpen(true) }}
              >
                <XCircle className="mr-2 h-4 w-4" /> Cancel
              </Button>
            </div>
          ) : undefined
        }
      />

      <Tabs value={activeTab} onValueChange={setActiveTab}>
        <TabsList>
          <TabsTrigger value="timeline">Timeline ({events.length})</TabsTrigger>
          <TabsTrigger value="gaps">Capability Gaps ({gaps.length})</TabsTrigger>
          <TabsTrigger value="details">Details</TabsTrigger>
        </TabsList>

        <TabsContent value="timeline" className="space-y-4">
          {isActive && !noteOpen && (
            <Button
              variant="outline"
              size="sm"
              onClick={() => { setNoteAction('commented'); setNoteDetail(''); setNoteOpen(true) }}
            >
              <Plus className="mr-2 h-4 w-4" /> Add Note
            </Button>
          )}
          {isActive && noteOpen && (
            <Card>
              <CardContent className="py-4 px-4 space-y-3">
                <div className="flex items-center gap-2 mb-1">
                  <MessageSquarePlus className="h-4 w-4 text-muted-foreground" />
                  <span className="text-sm font-medium">Add Note</span>
                </div>
                <div className="space-y-2">
                  <Label>Action</Label>
                  <Select value={noteAction} onValueChange={setNoteAction}>
                    <SelectTrigger><SelectValue /></SelectTrigger>
                    <SelectContent>
                      <SelectItem value="commented">Commented</SelectItem>
                      <SelectItem value="investigated">Investigated</SelectItem>
                      <SelectItem value="attempted_fix">Attempted Fix</SelectItem>
                      <SelectItem value="fix_succeeded">Fix Succeeded</SelectItem>
                      <SelectItem value="fix_failed">Fix Failed</SelectItem>
                    </SelectContent>
                  </Select>
                </div>
                <div className="space-y-2">
                  <Label>Detail</Label>
                  <Textarea
                    value={noteDetail}
                    onChange={(e) => setNoteDetail(e.target.value)}
                    placeholder="Describe what happened..."
                  />
                </div>
                <div className="flex gap-2">
                  <Button
                    size="sm"
                    disabled={!noteDetail.trim() || addEventMutation.isPending}
                    onClick={() => {
                      addEventMutation.mutate(
                        { incidentId: id, actor: 'admin', action: noteAction, detail: noteDetail },
                        { onSuccess: () => { setNoteOpen(false); setNoteDetail(''); setNoteAction('commented') } },
                      )
                    }}
                  >
                    {addEventMutation.isPending ? 'Saving...' : 'Save'}
                  </Button>
                  <Button variant="outline" size="sm" onClick={() => setNoteOpen(false)}>
                    Cancel
                  </Button>
                </div>
              </CardContent>
            </Card>
          )}
          {eventsLoading ? (
            <Skeleton className="h-32 w-full" />
          ) : events.length === 0 ? (
            <Card>
              <CardContent className="py-8 text-center text-muted-foreground">
                No events recorded yet.
              </CardContent>
            </Card>
          ) : (
            <div className="space-y-3">
              {events.map((evt) => (
                <Card key={evt.id}>
                  <CardContent className="py-3 px-4">
                    <div className="flex items-start justify-between gap-4">
                      <div className="min-w-0 flex-1">
                        <div className="flex items-center gap-2 mb-1">
                          <StatusBadge status={evt.action} />
                          <span className="text-xs text-muted-foreground">{evt.actor}</span>
                        </div>
                        <p className="text-sm">{evt.detail}</p>
                        {evt.metadata && Object.keys(evt.metadata).length > 0 && (
                          <details className="mt-2">
                            <summary className="text-xs text-muted-foreground cursor-pointer">
                              Metadata
                            </summary>
                            <pre className="mt-1 text-xs bg-muted p-2 rounded overflow-x-auto">
                              {JSON.stringify(evt.metadata, null, 2)}
                            </pre>
                          </details>
                        )}
                      </div>
                      <span className="text-xs text-muted-foreground whitespace-nowrap">
                        {formatRelative(evt.created_at)}
                      </span>
                    </div>
                  </CardContent>
                </Card>
              ))}
            </div>
          )}
        </TabsContent>

        <TabsContent value="gaps" className="space-y-4">
          {gapsLoading ? (
            <Skeleton className="h-32 w-full" />
          ) : gaps.length === 0 ? (
            <Card>
              <CardContent className="py-8 text-center text-muted-foreground">
                No capability gaps linked to this incident.
              </CardContent>
            </Card>
          ) : (
            <div className="space-y-3">
              {gaps.map((gap) => (
                <Card key={gap.id}>
                  <CardContent className="py-3 px-4">
                    <div className="flex items-start justify-between gap-4">
                      <div className="min-w-0 flex-1">
                        <div className="flex items-center gap-2 mb-1">
                          <span className="font-mono text-sm font-medium">{gap.tool_name}</span>
                          <StatusBadge status={gap.status} />
                        </div>
                        <p className="text-sm text-muted-foreground">{gap.description}</p>
                        <div className="flex items-center gap-4 mt-1 text-xs text-muted-foreground">
                          <span>Category: {gap.category}</span>
                          <span>Occurrences: {gap.occurrences}</span>
                        </div>
                      </div>
                    </div>
                  </CardContent>
                </Card>
              ))}
            </div>
          )}
        </TabsContent>

        <TabsContent value="details">
          <Card>
            <CardHeader>
              <CardTitle>Incident Details</CardTitle>
            </CardHeader>
            <CardContent>
              <dl className="grid grid-cols-1 gap-4 sm:grid-cols-2">
                <div>
                  <dt className="text-sm font-medium text-muted-foreground">ID</dt>
                  <dd className="text-sm font-mono">{incident.id}</dd>
                </div>
                <div>
                  <dt className="text-sm font-medium text-muted-foreground">Status</dt>
                  <dd><StatusBadge status={incident.status} /></dd>
                </div>
                <div>
                  <dt className="text-sm font-medium text-muted-foreground">Severity</dt>
                  <dd><StatusBadge status={incident.severity} /></dd>
                </div>
                <div>
                  <dt className="text-sm font-medium text-muted-foreground">Type</dt>
                  <dd className="text-sm">{incident.type}</dd>
                </div>
                <div>
                  <dt className="text-sm font-medium text-muted-foreground">Source</dt>
                  <dd className="text-sm">{incident.source}</dd>
                </div>
                <div>
                  <dt className="text-sm font-medium text-muted-foreground">Assigned To</dt>
                  <dd className="text-sm">{incident.assigned_to || 'Unassigned'}</dd>
                </div>
                {incident.resource_type && (
                  <div>
                    <dt className="text-sm font-medium text-muted-foreground">Resource</dt>
                    <dd className="text-sm">{incident.resource_type}/{incident.resource_id}</dd>
                  </div>
                )}
                <div>
                  <dt className="text-sm font-medium text-muted-foreground">Dedupe Key</dt>
                  <dd className="text-sm font-mono break-all">{incident.dedupe_key}</dd>
                </div>
                <div>
                  <dt className="text-sm font-medium text-muted-foreground">Detected</dt>
                  <dd className="text-sm">{formatDate(incident.detected_at)}</dd>
                </div>
                {incident.resolved_at && (
                  <div>
                    <dt className="text-sm font-medium text-muted-foreground">Resolved</dt>
                    <dd className="text-sm">{formatDate(incident.resolved_at)}</dd>
                  </div>
                )}
                {incident.escalated_at && (
                  <div>
                    <dt className="text-sm font-medium text-muted-foreground">Escalated</dt>
                    <dd className="text-sm">{formatDate(incident.escalated_at)}</dd>
                  </div>
                )}
                {incident.resolution && (
                  <div className="sm:col-span-2">
                    <dt className="text-sm font-medium text-muted-foreground">Resolution</dt>
                    <dd className="text-sm">{incident.resolution}</dd>
                  </div>
                )}
                {incident.detail && (
                  <div className="sm:col-span-2">
                    <dt className="text-sm font-medium text-muted-foreground">Detail</dt>
                    <dd className="text-sm whitespace-pre-wrap">{incident.detail}</dd>
                  </div>
                )}
              </dl>
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>

      {/* Resolve Dialog */}
      <Dialog open={resolveOpen} onOpenChange={setResolveOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Resolve Incident</DialogTitle>
            <DialogDescription>Describe what was done to resolve this incident.</DialogDescription>
          </DialogHeader>
          <div className="space-y-2">
            <Label>Resolution</Label>
            <Textarea
              value={actionText}
              onChange={(e) => setActionText(e.target.value)}
              placeholder="Describe the resolution..."
            />
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setResolveOpen(false)}>Cancel</Button>
            <Button
              disabled={!actionText.trim() || resolveMutation.isPending}
              onClick={() => {
                resolveMutation.mutate({ id: incident.id, resolution: actionText }, {
                  onSuccess: () => setResolveOpen(false),
                })
              }}
            >
              {resolveMutation.isPending ? 'Resolving...' : 'Resolve'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Escalate Dialog */}
      <Dialog open={escalateOpen} onOpenChange={setEscalateOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Escalate Incident</DialogTitle>
            <DialogDescription>Provide a reason for escalation.</DialogDescription>
          </DialogHeader>
          <div className="space-y-2">
            <Label>Reason</Label>
            <Textarea
              value={actionText}
              onChange={(e) => setActionText(e.target.value)}
              placeholder="Why is this being escalated?"
            />
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setEscalateOpen(false)}>Cancel</Button>
            <Button
              variant="destructive"
              disabled={!actionText.trim() || escalateMutation.isPending}
              onClick={() => {
                escalateMutation.mutate({ id: incident.id, reason: actionText }, {
                  onSuccess: () => setEscalateOpen(false),
                })
              }}
            >
              {escalateMutation.isPending ? 'Escalating...' : 'Escalate'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Cancel Dialog */}
      <Dialog open={cancelOpen} onOpenChange={setCancelOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Cancel Incident</DialogTitle>
            <DialogDescription>Mark this incident as a false positive.</DialogDescription>
          </DialogHeader>
          <div className="space-y-2">
            <Label>Reason (optional)</Label>
            <Textarea
              value={actionText}
              onChange={(e) => setActionText(e.target.value)}
              placeholder="Why is this a false positive?"
            />
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setCancelOpen(false)}>Back</Button>
            <Button
              variant="destructive"
              disabled={cancelMutation.isPending}
              onClick={() => {
                cancelMutation.mutate({ id: incident.id, reason: actionText }, {
                  onSuccess: () => setCancelOpen(false),
                })
              }}
            >
              {cancelMutation.isPending ? 'Cancelling...' : 'Cancel Incident'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
