import { useEffect, useState } from 'react'
import { Button } from '@/components/ui/button'
import { ChevronLeft, ChevronRight, Loader2, Download, Check } from 'lucide-react'
import type { Config, RoleInfo, ValidationError, GeneratedFile } from '@/lib/types'
import * as api from '@/lib/api'
import { cn } from '@/lib/utils'

import { DeployModeStep } from '@/steps/DeployModeStep'
import { RegionStep } from '@/steps/RegionStep'
import { BrandStep } from '@/steps/BrandStep'
import { ControlPlaneStep } from '@/steps/ControlPlaneStep'
import { NodesStep } from '@/steps/NodesStep'
import { StorageStep } from '@/steps/StorageStep'
import { TLSStep } from '@/steps/TLSStep'
import { ReviewStep } from '@/steps/ReviewStep'

type StepID =
  | 'deploy_mode'
  | 'region'
  | 'brand'
  | 'control_plane'
  | 'nodes'
  | 'storage'
  | 'tls'
  | 'review'

interface Step {
  id: StepID
  label: string
  visible: (config: Config) => boolean
}

const STEPS: Step[] = [
  { id: 'deploy_mode', label: 'Deployment', visible: () => true },
  { id: 'region', label: 'Region', visible: () => true },
  { id: 'brand', label: 'Brand', visible: () => true },
  { id: 'control_plane', label: 'Database', visible: () => true },
  { id: 'nodes', label: 'Machines', visible: (c) => c.deploy_mode === 'multi' },
  { id: 'storage', label: 'Storage', visible: () => true },
  { id: 'tls', label: 'TLS', visible: () => true },
  { id: 'review', label: 'Review', visible: () => true },
]

export default function App() {
  const [config, setConfig] = useState<Config | null>(null)
  const [roles, setRoles] = useState<RoleInfo[]>([])
  const [currentStep, setCurrentStep] = useState(0)
  const [saving, setSaving] = useState(false)
  const [errors, setErrors] = useState<ValidationError[]>([])
  const [generated, setGenerated] = useState<GeneratedFile[] | null>(null)
  const [generating, setGenerating] = useState(false)

  useEffect(() => {
    api.getConfig().then(setConfig)
    api.getRoles().then(setRoles)
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
    setSaving(true)
    try {
      await api.putConfig(newConfig)
    } finally {
      setSaving(false)
    }
  }

  const goNext = async () => {
    if (isLast) return
    setCurrentStep((i) => Math.min(i + 1, visibleSteps.length - 1))
  }

  const goPrev = () => {
    setCurrentStep((i) => Math.max(i - 1, 0))
  }

  const goToStep = (index: number) => {
    setCurrentStep(index)
  }

  const handleValidate = async () => {
    const result = await api.validate()
    setErrors(result.errors || [])
    return result.valid
  }

  const handleGoToReview = async () => {
    // Save, validate, and go to review
    await api.putConfig(config)
    const valid = await handleValidate()
    const reviewIndex = visibleSteps.findIndex((s) => s.id === 'review')
    if (reviewIndex >= 0) {
      setCurrentStep(reviewIndex)
    }
    return valid
  }

  const handleGenerate = async () => {
    setGenerating(true)
    try {
      await api.putConfig(config)
      const valid = await handleValidate()
      if (!valid) return

      const result = await api.generate()
      setGenerated(result.files)
    } catch (err: any) {
      setErrors([{ field: 'general', message: err.message }])
    } finally {
      setGenerating(false)
    }
  }

  return (
    <div className="min-h-screen flex flex-col">
      {/* Header */}
      <header className="border-b px-6 py-4">
        <h1 className="text-lg font-semibold">Hosting Platform Setup</h1>
        <p className="text-sm text-muted-foreground">
          Configure your deployment in a few steps
        </p>
      </header>

      <div className="flex flex-1">
        {/* Sidebar stepper */}
        <nav className="w-56 border-r p-4 space-y-1 shrink-0">
          {visibleSteps.map((s, i) => (
            <button
              key={s.id}
              onClick={() => goToStep(i)}
              className={cn(
                'w-full flex items-center gap-3 rounded-md px-3 py-2 text-sm transition-colors text-left',
                i === currentStep
                  ? 'bg-accent text-accent-foreground font-medium'
                  : 'text-muted-foreground hover:text-foreground hover:bg-accent/50'
              )}
            >
              <span
                className={cn(
                  'flex h-6 w-6 items-center justify-center rounded-full text-xs font-medium',
                  i === currentStep
                    ? 'bg-primary text-primary-foreground'
                    : i < currentStep
                      ? 'bg-primary/20 text-primary'
                      : 'bg-muted text-muted-foreground'
                )}
              >
                {i < currentStep ? (
                  <Check className="h-3 w-3" />
                ) : (
                  i + 1
                )}
              </span>
              {s.label}
            </button>
          ))}
        </nav>

        {/* Main content */}
        <main className="flex-1 p-8 max-w-3xl">
          {generated ? (
            <GeneratedView files={generated} onBack={() => setGenerated(null)} />
          ) : (
            <>
              {step.id === 'deploy_mode' && (
                <DeployModeStep config={config} onChange={handleChange} />
              )}
              {step.id === 'region' && (
                <RegionStep config={config} onChange={handleChange} />
              )}
              {step.id === 'brand' && (
                <BrandStep config={config} onChange={handleChange} />
              )}
              {step.id === 'control_plane' && (
                <ControlPlaneStep config={config} onChange={handleChange} />
              )}
              {step.id === 'nodes' && (
                <NodesStep config={config} onChange={handleChange} roles={roles} />
              )}
              {step.id === 'storage' && (
                <StorageStep config={config} onChange={handleChange} />
              )}
              {step.id === 'tls' && (
                <TLSStep config={config} onChange={handleChange} />
              )}
              {step.id === 'review' && (
                <ReviewStep config={config} roles={roles} errors={errors} />
              )}
            </>
          )}
        </main>
      </div>

      {/* Footer navigation */}
      {!generated && (
        <footer className="border-t px-6 py-4 flex items-center justify-between">
          <Button
            variant="outline"
            onClick={goPrev}
            disabled={isFirst}
          >
            <ChevronLeft className="h-4 w-4 mr-1" />
            Back
          </Button>

          <div className="flex items-center gap-2">
            {saving && (
              <span className="text-xs text-muted-foreground flex items-center gap-1">
                <Loader2 className="h-3 w-3 animate-spin" />
                Saving...
              </span>
            )}

            {isLast ? (
              <Button onClick={handleGenerate} disabled={generating}>
                {generating ? (
                  <>
                    <Loader2 className="h-4 w-4 mr-2 animate-spin" />
                    Generating...
                  </>
                ) : (
                  <>
                    <Download className="h-4 w-4 mr-2" />
                    Generate
                  </>
                )}
              </Button>
            ) : (
              <Button onClick={step.id === 'tls' ? handleGoToReview : goNext}>
                Next
                <ChevronRight className="h-4 w-4 ml-1" />
              </Button>
            )}
          </div>
        </footer>
      )}
    </div>
  )
}

function GeneratedView({
  files,
  onBack,
}: {
  files: GeneratedFile[]
  onBack: () => void
}) {
  const [selected, setSelected] = useState(0)

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-xl font-semibold flex items-center gap-2">
            <Check className="h-5 w-5 text-green-400" />
            Files Generated
          </h2>
          <p className="text-muted-foreground mt-1">
            {files.length} files have been written to disk.
          </p>
        </div>
        <Button variant="outline" onClick={onBack}>
          Back to Review
        </Button>
      </div>

      <div className="grid grid-cols-[200px_1fr] gap-4 min-h-[400px]">
        <div className="space-y-1">
          {files.map((f, i) => (
            <button
              key={f.path}
              onClick={() => setSelected(i)}
              className={cn(
                'w-full text-left rounded-md px-3 py-1.5 text-xs font-mono transition-colors',
                i === selected
                  ? 'bg-accent text-accent-foreground'
                  : 'text-muted-foreground hover:text-foreground hover:bg-accent/50'
              )}
            >
              {f.path}
            </button>
          ))}
        </div>
        <pre className="rounded-lg border bg-card p-4 text-xs font-mono overflow-auto whitespace-pre">
          {files[selected]?.content}
        </pre>
      </div>
    </div>
  )
}
