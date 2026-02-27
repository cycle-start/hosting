import { useEffect, useState, useRef, useCallback } from 'react'
import { Button } from '@/components/ui/button'
import { Check, AlertCircle, Loader2, Play, Square, RotateCcw } from 'lucide-react'
import { cn } from '@/lib/utils'
import * as api from '@/lib/api'
import type { Config, DeployStepDef, DeployStepID, ExecEvent, StepStatus } from '@/lib/types'

interface StepState {
  status: StepStatus
  output: string[]
  exitCode?: number
}

function makeInitialState(): StepState {
  return { status: 'pending', output: [] }
}

export function DeploySteps({ config }: { config: Config; outputDir: string }) {
  const [steps, setSteps] = useState<DeployStepDef[]>([])
  const [states, setStates] = useState<Record<string, StepState>>({})
  const [activeStep, setActiveStep] = useState<DeployStepID | null>(null)
  const abortRef = useRef<(() => void) | null>(null)

  useEffect(() => {
    api.getSteps().then((s) => {
      setSteps(s)
      const initial: Record<string, StepState> = {}
      for (const step of s) {
        initial[step.id] = makeInitialState()
      }
      setStates(initial)
    })
  }, [])

  const canRun = useCallback(
    (index: number) => {
      if (activeStep) return false
      for (let i = 0; i < index; i++) {
        if (states[steps[i].id]?.status !== 'success') return false
      }
      return true
    },
    [activeStep, states, steps],
  )

  const runStep = useCallback(
    (id: DeployStepID) => {
      setActiveStep(id)
      setStates((prev) => ({
        ...prev,
        [id]: { status: 'running', output: [] },
      }))

      const { abort, done } = api.executeStep(id, (event: ExecEvent) => {
        if (event.type === 'output' && event.data != null) {
          setStates((prev) => ({
            ...prev,
            [id]: {
              ...prev[id],
              output: [...prev[id].output, event.data!],
            },
          }))
        } else if (event.type === 'done') {
          const ok = event.exit_code === 0
          setStates((prev) => ({
            ...prev,
            [id]: {
              ...prev[id],
              status: ok ? 'success' : 'failed',
              exitCode: event.exit_code,
            },
          }))
          setActiveStep(null)
        } else if (event.type === 'error') {
          setStates((prev) => ({
            ...prev,
            [id]: {
              ...prev[id],
              status: 'failed',
              output: [...prev[id].output, event.data || 'Unknown error'],
            },
          }))
          setActiveStep(null)
        }
      })

      abortRef.current = abort

      done.catch(() => {
        // AbortError or network error
        setStates((prev) => ({
          ...prev,
          [id]: {
            ...prev[id],
            status: prev[id].status === 'running' ? 'failed' : prev[id].status,
          },
        }))
        setActiveStep(null)
      })
    },
    [],
  )

  const cancelStep = useCallback(() => {
    abortRef.current?.()
  }, [])

  if (steps.length === 0) {
    return (
      <div className="flex items-center gap-2 text-sm text-muted-foreground">
        <Loader2 className="h-4 w-4 animate-spin" />
        Loading steps...
      </div>
    )
  }

  return (
    <div className="space-y-3">
      {steps.map((step, i) => (
        <StepCard
          key={step.id}
          step={step}
          index={i}
          state={states[step.id] || makeInitialState()}
          canRun={canRun(i)}
          onRun={() => runStep(step.id)}
          onCancel={cancelStep}
        />
      ))}

      {config.deploy_mode === 'multi' && steps.length > 0 && steps[0].id !== 'ssh_ca' && (
        <p className="text-xs text-muted-foreground mt-2">
          Ensure SSH access to all machines before running Ansible.
        </p>
      )}
    </div>
  )
}

function StepCard({
  step,
  index,
  state,
  canRun,
  onRun,
  onCancel,
}: {
  step: DeployStepDef
  index: number
  state: StepState
  canRun: boolean
  onRun: () => void
  onCancel: () => void
}) {
  const hasOutput = state.output.length > 0

  return (
    <div className="rounded-lg border bg-card overflow-hidden">
      <div className="flex items-center gap-3 px-4 py-3">
        <StepStatusIcon status={state.status} index={index} />
        <div className="flex-1 min-w-0">
          <div className="text-sm font-medium">{step.label}</div>
          <div className="text-xs text-muted-foreground">{step.description}</div>
        </div>
        <div className="shrink-0">
          {state.status === 'running' ? (
            <Button variant="outline" size="sm" onClick={onCancel}>
              <Square className="h-3 w-3 mr-1.5" />
              Cancel
            </Button>
          ) : state.status === 'failed' ? (
            <Button variant="outline" size="sm" onClick={onRun} disabled={!canRun}>
              <RotateCcw className="h-3 w-3 mr-1.5" />
              Retry
            </Button>
          ) : state.status === 'success' ? null : (
            <Button size="sm" onClick={onRun} disabled={!canRun}>
              <Play className="h-3 w-3 mr-1.5" />
              Run
            </Button>
          )}
        </div>
      </div>

      {hasOutput && (
        <div className="border-t">
          <TerminalOutput lines={state.output} />
          {state.status === 'failed' && state.exitCode != null && (
            <div className="px-4 py-2 bg-destructive/10 text-destructive text-xs font-mono">
              Process exited with code {state.exitCode}
            </div>
          )}
        </div>
      )}
    </div>
  )
}

function StepStatusIcon({ status, index }: { status: StepStatus; index: number }) {
  if (status === 'success') {
    return (
      <span className="flex h-6 w-6 items-center justify-center rounded-full bg-green-500/20 text-green-400 shrink-0">
        <Check className="h-3 w-3" />
      </span>
    )
  }
  if (status === 'failed') {
    return (
      <span className="flex h-6 w-6 items-center justify-center rounded-full bg-destructive/20 text-destructive shrink-0">
        <AlertCircle className="h-3 w-3" />
      </span>
    )
  }
  if (status === 'running') {
    return (
      <span className="flex h-6 w-6 items-center justify-center rounded-full bg-primary/20 text-primary shrink-0">
        <Loader2 className="h-3 w-3 animate-spin" />
      </span>
    )
  }
  return (
    <span className="flex h-6 w-6 items-center justify-center rounded-full bg-muted text-muted-foreground text-xs font-medium shrink-0">
      {index + 1}
    </span>
  )
}

// ANSI 16-color palette mapped to CSS colors (GitHub dark theme inspired)
const ANSI_COLORS: Record<number, string> = {
  30: '#6e7681', 31: '#ff7b72', 32: '#3fb950', 33: '#d29922',
  34: '#58a6ff', 35: '#bc8cff', 36: '#39c5cf', 37: '#c9d1d9',
  90: '#6e7681', 91: '#ffa198', 92: '#56d364', 93: '#e3b341',
  94: '#79c0ff', 95: '#d2a8ff', 96: '#56d4dd', 97: '#f0f6fc',
}

interface AnsiSpan {
  text: string
  color?: string
  bold?: boolean
}

function parseAnsi(line: string): AnsiSpan[] {
  const spans: AnsiSpan[] = []
  const re = /\x1b\[([0-9;]*)m/g
  let lastIndex = 0
  let color: string | undefined
  let bold = false

  let match: RegExpExecArray | null
  while ((match = re.exec(line)) !== null) {
    if (match.index > lastIndex) {
      spans.push({ text: line.slice(lastIndex, match.index), color, bold })
    }
    const codes = match[1].split(';').map(Number)
    for (const code of codes) {
      if (code === 0) { color = undefined; bold = false }
      else if (code === 1) { bold = true }
      else if (ANSI_COLORS[code]) { color = ANSI_COLORS[code] }
    }
    lastIndex = re.lastIndex
  }
  if (lastIndex < line.length) {
    spans.push({ text: line.slice(lastIndex), color, bold })
  }
  if (spans.length === 0) {
    spans.push({ text: line })
  }
  return spans
}

function AnsiLine({ line }: { line: string }) {
  const spans = parseAnsi(line)
  return (
    <div>
      {spans.map((span, i) => (
        <span key={i} style={{
          color: span.color,
          fontWeight: span.bold ? 700 : undefined,
        }}>
          {span.text}
        </span>
      ))}
    </div>
  )
}

function TerminalOutput({ lines }: { lines: string[] }) {
  const containerRef = useRef<HTMLDivElement>(null)
  const shouldAutoScroll = useRef(true)

  const handleScroll = () => {
    const el = containerRef.current
    if (!el) return
    const atBottom = el.scrollHeight - el.scrollTop - el.clientHeight < 32
    shouldAutoScroll.current = atBottom
  }

  useEffect(() => {
    const el = containerRef.current
    if (el && shouldAutoScroll.current) {
      el.scrollTop = el.scrollHeight
    }
  }, [lines.length])

  return (
    <div
      ref={containerRef}
      onScroll={handleScroll}
      className={cn(
        'bg-[#0d1117] text-[#c9d1d9] text-xs font-mono',
        'px-4 py-3 max-h-80 overflow-y-auto',
        'whitespace-pre-wrap break-all',
      )}
    >
      {lines.map((line, i) => (
        <AnsiLine key={i} line={line} />
      ))}
    </div>
  )
}
