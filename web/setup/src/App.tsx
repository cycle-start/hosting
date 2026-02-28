import { useEffect, useRef, useState } from 'react'
import * as Tabs from '@radix-ui/react-tabs'
import { Button } from '@/components/ui/button'
import { ChevronLeft, ChevronRight, Loader2, Download, Check, AlertCircle, Terminal, FileText, ArrowRight, Copy } from 'lucide-react'
import type { Config, RoleInfo, ValidationError, GeneratedFile, StepID } from '@/lib/types'
import { errorsForStep } from '@/lib/validation'
import * as api from '@/lib/api'
import { cn } from '@/lib/utils'

import { DeployModeStep } from '@/steps/DeployModeStep'
import { RegionStep } from '@/steps/RegionStep'
import { BrandStep } from '@/steps/BrandStep'
import { ControlPlaneStep } from '@/steps/ControlPlaneStep'
import { NodesStep } from '@/steps/NodesStep'
import { TLSStep } from '@/steps/TLSStep'
import { ReviewStep } from '@/steps/ReviewStep'
import { DeploySteps } from '@/steps/DeploySteps'
import { ControlPlaneStatus } from '@/steps/ControlPlaneStatus'
import { OverviewTab } from '@/steps/OverviewTab'

interface Step {
  id: StepID
  label: string
  visible: (config: Config) => boolean
}

const STEPS: Step[] = [
  { id: 'deploy_mode', label: 'Deployment', visible: () => true },
  { id: 'region', label: 'Region', visible: () => true },
  { id: 'brand', label: 'Brand', visible: () => true },
  { id: 'control_plane', label: 'Infrastructure', visible: () => true },
  { id: 'nodes', label: 'Machines', visible: (c) => c.deploy_mode === 'multi' },
  { id: 'tls', label: 'Security', visible: () => true },
  { id: 'review', label: 'Review', visible: () => true },
  { id: 'install', label: 'Install', visible: () => true },
]

type TopTab = 'setup' | 'files' | 'overview'

export default function App() {
  const [config, setConfig] = useState<Config | null>(null)
  const [roles, setRoles] = useState<RoleInfo[]>([])
  const [currentStep, setCurrentStep] = useState(0)
  const [saving, setSaving] = useState(false)
  const [errors, setErrors] = useState<ValidationError[]>([])
  const [generated, setGenerated] = useState<{ outputDir: string; files: GeneratedFile[] } | null>(null)
  const [generating, setGenerating] = useState(false)
  const [generateLog, setGenerateLog] = useState<string[]>([])
  const [generateError, setGenerateError] = useState<string | null>(null)
  const [visitedSteps, setVisitedSteps] = useState<Set<StepID>>(new Set(['deploy_mode']))
  const [outputDir, setOutputDir] = useState('')
  const [activeTab, setActiveTab] = useState<TopTab>('setup')
  const mainRef = useRef<HTMLElement>(null)

  // Scroll content area to top when step changes
  useEffect(() => {
    mainRef.current?.scrollTo(0, 0)
  }, [currentStep])

  useEffect(() => {
    api.getConfig().then(setConfig)
    api.getRoles().then(setRoles)
    api.getInfo().then((info) => setOutputDir(info.output_dir))
  }, [])

  if (!config) {
    return (
      <div className="flex items-center justify-center min-h-screen">
        <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
      </div>
    )
  }

  const visibleSteps = STEPS.filter((s) => s.visible(config))
  const step = visibleSteps[currentStep]
  const isFirst = currentStep === 0
  const isLast = currentStep === visibleSteps.length - 1

  const handleChange = async (newConfig: Config) => {
    setConfig(newConfig)
    // Clear errors for fields that changed so red borders disappear immediately
    if (errors.length > 0) {
      setErrors((prev) => prev.filter((e) => {
        const val = (f: string, obj: any): any => f.split('.').reduce((o, k) => o?.[k], obj)
        const oldVal = val(e.field, config)
        const newVal = val(e.field, newConfig)
        return oldVal === newVal
      }))
    }
    setSaving(true)
    try {
      await api.putConfig(newConfig)
    } finally {
      setSaving(false)
    }
  }

  const doValidate = async (): Promise<ValidationError[]> => {
    await api.putConfig(config)
    const result = await api.validate()
    const errs = result.errors || []
    setErrors(errs)
    return errs
  }

  const goNext = async () => {
    if (isLast) return

    // Validate before advancing
    const errs = await doValidate()
    const stepErrors = errorsForStep(errs, step.id)

    // Mark current step as visited
    setVisitedSteps((prev) => new Set([...prev, step.id]))

    if (stepErrors.length > 0) {
      // Don't advance — errors will be shown inline
      return
    }

    const nextIndex = currentStep + 1
    const nextStep = visibleSteps[nextIndex]
    setVisitedSteps((prev) => new Set([...prev, nextStep.id]))
    setCurrentStep(nextIndex)
  }

  const goPrev = () => {
    setCurrentStep((i) => Math.max(i - 1, 0))
  }

  const goToStep = (index: number) => {
    setVisitedSteps((prev) => new Set([...prev, visibleSteps[index].id]))
    setCurrentStep(index)
  }

  const handleGenerate = async () => {
    setGenerating(true)
    setGenerateLog([])
    setGenerateError(null)
    try {
      const errs = await doValidate()
      if (errs.length > 0) return

      const result = await api.generate((msg) => {
        setGenerateLog((prev) => [...prev, msg])
      })
      setGenerated({ outputDir: result.output_dir, files: result.files })
    } catch (err: any) {
      setGenerateError(err.message)
    } finally {
      setGenerating(false)
    }
  }

  // Compute step status for sidebar
  const getStepStatus = (s: Step, index: number): 'valid' | 'error' | 'current' | 'upcoming' => {
    if (index === currentStep) return 'current'
    if (!visitedSteps.has(s.id)) return 'upcoming'
    const stepErrors = errorsForStep(errors, s.id)
    return stepErrors.length > 0 ? 'error' : 'valid'
  }

  const stepErrors = errorsForStep(errors, step.id)

  const goToInstallStep = () => {
    const idx = visibleSteps.findIndex((s) => s.id === 'install')
    if (idx >= 0) {
      goToStep(idx)
      setActiveTab('setup')
    }
  }

  return (
    <Tabs.Root value={activeTab} onValueChange={(v) => setActiveTab(v as TopTab)} className="h-screen flex flex-col overflow-hidden">
      {/* Header */}
      <header className="border-b px-6 py-4 flex items-center gap-3">
        <div className="flex items-center justify-center h-8 w-8 rounded-lg bg-primary/10">
          <Terminal className="h-4 w-4 text-primary" />
        </div>
        <div>
          <h1 className="text-lg font-semibold">Hosting Platform Setup</h1>
          <p className="text-xs text-muted-foreground">
            Configure your deployment in a few steps
          </p>
        </div>
        <div className="flex-1" />
        <Tabs.List className="flex items-center gap-1">
          {(['setup', 'files', 'overview'] as const).map((tab) => (
            <Tabs.Trigger
              key={tab}
              value={tab}
              className={cn(
                'px-3 py-1.5 text-sm font-medium rounded-md transition-colors',
                'data-[state=active]:bg-accent data-[state=active]:text-accent-foreground',
                'data-[state=inactive]:text-muted-foreground data-[state=inactive]:hover:text-foreground data-[state=inactive]:hover:bg-accent/50'
              )}
            >
              {tab.charAt(0).toUpperCase() + tab.slice(1)}
            </Tabs.Trigger>
          ))}
        </Tabs.List>
      </header>

      <div className="flex flex-1 min-h-0">
        {/* Sidebar stepper */}
        <nav className="w-56 border-r p-4 space-y-1 shrink-0 overflow-y-auto">
          {visibleSteps.map((s, i) => {
            const status = getStepStatus(s, i)
            return (
              <button
                key={s.id}
                onClick={() => {
                  goToStep(i)
                  setActiveTab('setup')
                }}
                className={cn(
                  'w-full flex items-center gap-3 rounded-md px-3 py-2 text-sm transition-colors text-left',
                  activeTab === 'setup' && status === 'current'
                    ? 'bg-accent text-accent-foreground font-medium'
                    : 'text-muted-foreground hover:text-foreground hover:bg-accent/50'
                )}
              >
                <StepIndicator status={status} index={i} />
                {s.label}
              </button>
            )
          })}
        </nav>

        {/* Main content */}
        <div className="flex-1 flex flex-col min-h-0">
          {/* Setup tab — forceMount keeps deploy step state alive when switching tabs */}
          <Tabs.Content value="setup" forceMount className={cn('flex-1 flex flex-col min-h-0', activeTab !== 'setup' && 'hidden')}>
            <main ref={mainRef} className={cn('flex-1 overflow-y-auto p-8', step.id !== 'install' && 'max-w-3xl')}>
              {step.id === 'deploy_mode' && (
                <DeployModeStep config={config} onChange={handleChange} outputDir={outputDir} />
              )}
              {step.id === 'region' && (
                <RegionStep config={config} onChange={handleChange} errors={stepErrors} />
              )}
              {step.id === 'brand' && (
                <BrandStep config={config} onChange={handleChange} errors={stepErrors} />
              )}
              {step.id === 'control_plane' && (
                <ControlPlaneStep config={config} onChange={handleChange} errors={stepErrors} />
              )}
              {step.id === 'nodes' && (
                <NodesStep config={config} onChange={handleChange} roles={roles} errors={stepErrors} />
              )}
              {step.id === 'tls' && (
                <TLSStep config={config} onChange={handleChange} errors={stepErrors} />
              )}
              {step.id === 'review' && (
                <ReviewStep config={config} roles={roles} errors={errors} onGoToStep={(stepId) => {
                  const idx = visibleSteps.findIndex((s) => s.id === stepId)
                  if (idx >= 0) goToStep(idx)
                }} />
              )}
              {step.id === 'install' && (
                <InstallStep
                  config={config}
                  outputDir={outputDir}
                  generated={generated}
                  generating={generating}
                  generateLog={generateLog}
                  generateError={generateError}
                  onGenerate={handleGenerate}
                />
              )}
            </main>

            {/* Footer navigation */}
            <footer className="border-t px-6 py-4 flex items-center justify-between">
              <Button
                variant="outline"
                onClick={goPrev}
                disabled={isFirst}
              >
                <ChevronLeft className="h-4 w-4 mr-1" />
                Back
              </Button>

              <div className="flex items-center gap-3">
                {saving && (
                  <span className="text-xs text-muted-foreground flex items-center gap-1">
                    <Loader2 className="h-3 w-3 animate-spin" />
                    Saving...
                  </span>
                )}

                {!isLast && (
                  <Button onClick={goNext}>
                    Next
                    <ChevronRight className="h-4 w-4 ml-1" />
                  </Button>
                )}
              </div>
            </footer>
          </Tabs.Content>

          {/* Files tab */}
          <Tabs.Content value="files" className="flex-1">
            {generated ? (
              <FileBrowser files={generated.files} />
            ) : (
              <div className="p-8 max-w-3xl">
                <h2 className="text-xl font-semibold mb-2">Generated Files</h2>
                <p className="text-sm text-muted-foreground mb-4">
                  No files generated yet. Complete the setup and generate files first.
                </p>
                <Button variant="outline" onClick={goToInstallStep}>
                  Go to Install Step
                  <ArrowRight className="h-4 w-4 ml-2" />
                </Button>
              </div>
            )}
          </Tabs.Content>

          {/* Overview tab */}
          <Tabs.Content value="overview" className="flex-1">
            <OverviewTab config={config} />
          </Tabs.Content>
        </div>
      </div>
    </Tabs.Root>
  )
}

function StepIndicator({ status, index }: { status: 'valid' | 'error' | 'current' | 'upcoming'; index: number }) {
  if (status === 'valid') {
    return (
      <span className="flex h-6 w-6 items-center justify-center rounded-full bg-green-500/20 text-green-400">
        <Check className="h-3 w-3" />
      </span>
    )
  }
  if (status === 'error') {
    return (
      <span className="flex h-6 w-6 items-center justify-center rounded-full bg-destructive/20 text-destructive">
        <AlertCircle className="h-3 w-3" />
      </span>
    )
  }
  return (
    <span
      className={cn(
        'flex h-6 w-6 items-center justify-center rounded-full text-xs font-medium',
        status === 'current'
          ? 'bg-primary text-primary-foreground'
          : 'bg-muted text-muted-foreground'
      )}
    >
      {index + 1}
    </span>
  )
}

function FileBrowser({ files }: { files: GeneratedFile[] }) {
  const [selected, setSelected] = useState(0)
  const [copied, setCopied] = useState(false)

  const copyContent = () => {
    navigator.clipboard.writeText(files[selected]?.content || '')
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  return (
    <div className="p-8 max-w-5xl">
      <h2 className="text-xl font-semibold mb-4">Generated Files</h2>
      <div className="rounded-lg border bg-card overflow-hidden min-h-[400px]">
        <div className="flex flex-wrap border-b bg-muted/50">
          {files.map((f, i) => (
            <button
              key={f.path}
              onClick={() => setSelected(i)}
              className={cn(
                'px-3 py-2 text-xs font-mono border-b-2 transition-colors',
                i === selected
                  ? 'border-primary text-foreground bg-card'
                  : 'border-transparent text-muted-foreground hover:text-foreground'
              )}
            >
              {f.path.replace(/^generated\//, '')}
            </button>
          ))}
        </div>
        <div className="relative">
          <button
            onClick={copyContent}
            className="absolute top-2 right-2 rounded-md p-1.5 text-muted-foreground hover:text-foreground hover:bg-accent transition-colors z-10"
            title="Copy to clipboard"
          >
            {copied ? <Check className="h-3.5 w-3.5 text-green-400" /> : <Copy className="h-3.5 w-3.5" />}
          </button>
          <pre className="p-4 text-xs font-mono overflow-auto whitespace-pre">
            {files[selected]?.content}
          </pre>
        </div>
      </div>
    </div>
  )
}

function InstallStep({
  config,
  outputDir,
  generated,
  generating,
  generateLog,
  generateError,
  onGenerate,
}: {
  config: Config
  outputDir: string
  generated: { outputDir: string; files: GeneratedFile[] } | null
  generating: boolean
  generateLog: string[]
  generateError: string | null
  onGenerate: () => void
}) {
  if (!generated) {
    return (
      <div className="space-y-4">
        <h2 className="text-xl font-semibold">Install</h2>
        <p className="text-sm text-muted-foreground">
          Generate the configuration files, then run the deploy steps to set up your hosting platform.
        </p>
        <Button onClick={onGenerate} disabled={generating}>
          {generating ? (
            <>
              <Loader2 className="h-4 w-4 mr-2 animate-spin" />
              Generating...
            </>
          ) : (
            <>
              <Download className="h-4 w-4 mr-2" />
              Generate Files
            </>
          )}
        </Button>
        {generateLog.length > 0 && (
          <div className="rounded-lg border bg-card overflow-hidden">
            <div className="bg-[#0d1117] text-[#c9d1d9] text-xs font-mono px-4 py-3 space-y-0.5">
              {generateLog.map((line, i) => (
                <div key={i}>{line}</div>
              ))}
              {generating && (
                <div className="flex items-center gap-2 text-muted-foreground">
                  <Loader2 className="h-3 w-3 animate-spin" />
                </div>
              )}
            </div>
          </div>
        )}
        {generateError && (
          <div className="rounded-lg border border-destructive/50 bg-destructive/10 p-4 space-y-2">
            <p className="text-sm font-medium text-destructive flex items-center gap-2">
              <AlertCircle className="h-4 w-4" />
              Generation failed
            </p>
            <p className="text-xs font-mono text-destructive/80">{generateError}</p>
          </div>
        )}
      </div>
    )
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-xl font-semibold flex items-center gap-2">
            <div className="flex items-center justify-center h-7 w-7 rounded-full bg-green-500/20">
              <Check className="h-4 w-4 text-green-400" />
            </div>
            Configuration Generated
          </h2>
          <p className="text-sm text-muted-foreground mt-1">
            Files written to <code className="text-xs font-mono bg-muted px-1 py-0.5 rounded">{generated.outputDir}/</code>.
          </p>
        </div>
        <Button variant="outline" onClick={onGenerate} disabled={generating}>
          {generating ? (
            <>
              <Loader2 className="h-4 w-4 mr-2 animate-spin" />
              Regenerating...
            </>
          ) : (
            <>
              <Download className="h-4 w-4 mr-2" />
              Regenerate Files
            </>
          )}
        </Button>
      </div>

      {/* Deploy */}
      <div className="rounded-lg border bg-card p-5 space-y-4">
        <h3 className="text-sm font-medium flex items-center gap-2">
          <ArrowRight className="h-4 w-4 text-primary" />
          Deploy
        </h3>
        <p className="text-sm text-muted-foreground">
          Run each step in order. You can also close this wizard (Ctrl+C) and run the commands manually — the generated files are saved to disk.
        </p>
        <DeploySteps config={config} outputDir={outputDir} />
      </div>

      <ControlPlaneStatus outputDir={outputDir} />
    </div>
  )
}
