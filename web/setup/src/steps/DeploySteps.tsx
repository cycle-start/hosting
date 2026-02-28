import { useEffect, useState, useRef, useCallback } from 'react'
import { Button } from '@/components/ui/button'
import { Check, AlertCircle, Loader2, Play, Square, RotateCcw, Copy } from 'lucide-react'
import { cn } from '@/lib/utils'
import * as api from '@/lib/api'
import type { Config, DeployStepDef, DeployStepID, ExecEvent, StepStatus } from '@/lib/types'

interface StepState {
  status: StepStatus
  output: string[]
  exitCode?: number
  startedAt?: number
  finishedAt?: number
}

function makeInitialState(): StepState {
  return { status: 'pending', output: [] }
}

function formatElapsed(ms: number): string {
  const totalSec = Math.floor(ms / 1000)
  if (totalSec < 60) return `${totalSec}s`
  const min = Math.floor(totalSec / 60)
  const sec = totalSec % 60
  return sec > 0 ? `${min}m${sec}s` : `${min}m`
}

export function DeploySteps({ config, onStepChange }: { config: Config; outputDir: string; onStepChange?: (stepId: DeployStepID, status: StepStatus) => void }) {
  const [steps, setSteps] = useState<DeployStepDef[]>([])
  const [states, setStates] = useState<Record<string, StepState>>({})
  const [activeStep, setActiveStep] = useState<DeployStepID | null>(null)
  const [runAllActive, setRunAllActive] = useState(false)
  const abortRef = useRef<(() => void) | null>(null)
  const runAllAbortRef = useRef(false)
  const statesRef = useRef(states)
  statesRef.current = states

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
    (_index: number) => {
      if (activeStep) return false
      return true
    },
    [activeStep],
  )

  const executeStepAsync = useCallback(
    (id: DeployStepID): Promise<number> => {
      return new Promise((resolve) => {
        setActiveStep(id)
        setStates((prev) => ({
          ...prev,
          [id]: { status: 'running', output: [], startedAt: Date.now() },
        }))
        onStepChange?.(id, 'running')

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
            const status = ok ? 'success' : 'failed' as StepStatus
            setStates((prev) => ({
              ...prev,
              [id]: {
                ...prev[id],
                status,
                exitCode: event.exit_code,
                finishedAt: Date.now(),
              },
            }))
            onStepChange?.(id, status)
            setActiveStep(null)
            resolve(event.exit_code ?? 1)
          } else if (event.type === 'error') {
            setStates((prev) => ({
              ...prev,
              [id]: {
                ...prev[id],
                status: 'failed',
                output: [...prev[id].output, event.data || 'Unknown error'],
                finishedAt: Date.now(),
              },
            }))
            onStepChange?.(id, 'failed')
            setActiveStep(null)
            resolve(1)
          }
        })

        abortRef.current = abort

        done.catch(() => {
          setStates((prev) => ({
            ...prev,
            [id]: {
              ...prev[id],
              status: prev[id].status === 'running' ? 'failed' : prev[id].status,
            },
          }))
          setActiveStep(null)
          resolve(1)
        })
      })
    },
    [],
  )

  const runStep = useCallback(
    (id: DeployStepID) => {
      executeStepAsync(id)
    },
    [executeStepAsync],
  )

  const cancelStep = useCallback(() => {
    abortRef.current?.()
  }, [])

  const runAllSteps = useCallback(async () => {
    setRunAllActive(true)
    runAllAbortRef.current = false

    for (const step of steps) {
      if (runAllAbortRef.current) break

      // Skip already-succeeded steps
      const current = statesRef.current[step.id]
      if (current?.status === 'success') continue

      const exitCode = await executeStepAsync(step.id)
      if (exitCode !== 0 || runAllAbortRef.current) break
    }

    setRunAllActive(false)
  }, [steps, executeStepAsync])

  const cancelAll = useCallback(() => {
    runAllAbortRef.current = true
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

  const allSucceeded = steps.length > 0 && steps.every((s) => states[s.id]?.status === 'success')

  return (
    <div className="space-y-3">
      <div>
        {runAllActive ? (
          <Button variant="outline" className="w-full" onClick={cancelAll}>
            <Square className="h-3.5 w-3.5 mr-2" />
            Cancel All
          </Button>
        ) : (
          <Button className="w-full" onClick={runAllSteps} disabled={!!activeStep || allSucceeded}>
            <Play className="h-3.5 w-3.5 mr-2" />
            {allSucceeded ? 'All Steps Complete' : 'Install All'}
          </Button>
        )}
      </div>

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
          <div className="text-sm font-medium flex items-center gap-2">
            {step.label}
            <StepTimer state={state} />
          </div>
          <div className="text-xs text-muted-foreground">{step.description}</div>
        </div>
        <div className="shrink-0 flex items-center gap-2">
          {step.command && (
            <CopyCommandButton command={step.command} />
          )}
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
          ) : state.status === 'success' ? (
            <Button variant="outline" size="sm" onClick={onRun} disabled={!canRun}>
              <RotateCcw className="h-3 w-3 mr-1.5" />
              Re-run
            </Button>
          ) : (
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

function StepTimer({ state }: { state: StepState }) {
  const [now, setNow] = useState(Date.now())

  useEffect(() => {
    if (state.status !== 'running' || !state.startedAt) return
    const id = setInterval(() => setNow(Date.now()), 1000)
    return () => clearInterval(id)
  }, [state.status, state.startedAt])

  if (!state.startedAt) return null

  const elapsed = (state.finishedAt || now) - state.startedAt
  if (elapsed < 1000) return null

  return (
    <span className="text-[10px] font-normal text-muted-foreground tabular-nums">
      {formatElapsed(elapsed)}
    </span>
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

function CopyCommandButton({ command }: { command: string }) {
  const [copied, setCopied] = useState(false)

  const copy = () => {
    navigator.clipboard.writeText(command)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  return (
    <button
      onClick={copy}
      className="rounded-md p-1.5 text-muted-foreground hover:text-foreground hover:bg-accent transition-colors"
      title={command}
    >
      {copied ? <Check className="h-3.5 w-3.5 text-green-400" /> : <Copy className="h-3.5 w-3.5" />}
    </button>
  )
}
