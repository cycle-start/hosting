import { useEffect, useState, useCallback } from 'react'
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
  const [visitedSteps, setVisitedSteps] = useState<Set<StepID>>(new Set(['deploy_mode']))

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
    try {
      const errs = await doValidate()
      if (errs.length > 0) return

      const result = await api.generate()
      setGenerated(result.files)
    } catch (err: any) {
      setErrors([{ field: 'general', message: err.message }])
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

  return (
    <div className="min-h-screen flex flex-col">
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
      </header>

      <div className="flex flex-1">
        {/* Sidebar stepper */}
        <nav className="w-56 border-r p-4 space-y-1 shrink-0">
          {visibleSteps.map((s, i) => {
            const status = getStepStatus(s, i)
            return (
              <button
                key={s.id}
                onClick={() => goToStep(i)}
                className={cn(
                  'w-full flex items-center gap-3 rounded-md px-3 py-2 text-sm transition-colors text-left',
                  status === 'current'
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
        <main className="flex-1 p-8 max-w-3xl">
          {generated ? (
            <GeneratedView files={generated} config={config} onBack={() => setGenerated(null)} />
          ) : (
            <>
              {step.id === 'deploy_mode' && (
                <DeployModeStep config={config} onChange={handleChange} />
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

          <div className="flex items-center gap-3">
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
                    Generate Files
                  </>
                )}
              </Button>
            ) : (
              <Button onClick={goNext}>
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

function GeneratedView({
  files,
  config,
  onBack,
}: {
  files: GeneratedFile[]
  config: Config
  onBack: () => void
}) {
  const [selected, setSelected] = useState(0)
  const [copied, setCopied] = useState(false)

  const copyContent = () => {
    navigator.clipboard.writeText(files[selected]?.content || '')
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
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
          <p className="text-muted-foreground mt-1">
            {files.length} files written to disk, including <code className="text-xs font-mono bg-muted px-1 py-0.5 rounded">setup.yaml</code> manifest.
            Edit the manifest by hand or re-run the wizard to modify, then <code className="text-xs font-mono bg-muted px-1 py-0.5 rounded">setup generate</code> to regenerate.
          </p>
        </div>
        <Button variant="outline" onClick={onBack}>
          Back to Review
        </Button>
      </div>

      {/* Next steps */}
      <div className="rounded-lg border bg-card p-5 space-y-4">
        <h3 className="text-sm font-medium flex items-center gap-2">
          <ArrowRight className="h-4 w-4 text-primary" />
          Next Steps
        </h3>
        <p className="text-sm text-muted-foreground">
          You can close this wizard now (Ctrl+C in the terminal). The generated files are saved to disk.
          Continue in a new terminal:
        </p>
        <ol className="text-sm text-muted-foreground space-y-4 ml-6 list-decimal">
          {config.deploy_mode === 'multi' && (
            <>
              <li>
                <span className="text-foreground font-medium">Ensure SSH access to all machines.</span>
                {' '}Ansible will connect via SSH to install and configure services on each node.
                Verify you can reach them with your SSH key.
              </li>
              <li>
                <span className="text-foreground font-medium">Generate an SSH CA keypair</span>
                {' '}(if you haven't already). The platform uses this to issue short-lived SSH certificates for node-to-node communication.
                <code className="block mt-1.5 rounded bg-muted px-3 py-2 text-xs font-mono text-foreground">
                  ssh-keygen -t ed25519 -f ssh_ca -N ""
                </code>
              </li>
            </>
          )}
          <li>
            <span className="text-foreground font-medium">Provision {config.deploy_mode === 'single' ? 'this machine' : 'all machines'} with Ansible.</span>
            {' '}This installs packages, configures services, and deploys agents{config.deploy_mode === 'multi' ? ' across all nodes' : ''}.
            <code className="block mt-1.5 rounded bg-muted px-3 py-2 text-xs font-mono text-foreground">
              ansible-playbook ansible/site.yml -i ansible/inventory/static.ini
            </code>
          </li>
          <li>
            <span className="text-foreground font-medium">Register the API key.</span>
            {' '}This creates the authentication key that all components use to communicate with the control plane API.
            <code className="block mt-1.5 rounded bg-muted px-3 py-2 text-xs font-mono text-foreground">
              ./bin/core-api create-api-key --name setup --raw-key {config.api_key}
            </code>
          </li>
          <li>
            <span className="text-foreground font-medium">Register the cluster topology.</span>
            {' '}This tells the control plane about the region, cluster, nodes, shards, and available runtimes.
            <code className="block mt-1.5 rounded bg-muted px-3 py-2 text-xs font-mono text-foreground">
              ./bin/hostctl cluster apply -f cluster.yaml
            </code>
          </li>
          <li>
            <span className="text-foreground font-medium">Seed the initial brand.</span>
            {' '}Creates the first brand with its domains, nameservers, and mail configuration.
            <code className="block mt-1.5 rounded bg-muted px-3 py-2 text-xs font-mono text-foreground">
              ./bin/hostctl seed -f seed.yaml
            </code>
          </li>
        </ol>
      </div>

      {/* File browser */}
      <div>
        <h3 className="text-sm font-medium mb-3 flex items-center gap-2">
          <FileText className="h-4 w-4 text-muted-foreground" />
          Generated Files
        </h3>
        <div className="rounded-lg border bg-card overflow-hidden min-h-[400px]">
          <div className="flex border-b bg-muted/50 overflow-x-auto">
            {files.map((f, i) => (
              <button
                key={f.path}
                onClick={() => setSelected(i)}
                className={cn(
                  'px-4 py-2 text-xs font-mono whitespace-nowrap border-b-2 transition-colors',
                  i === selected
                    ? 'border-primary text-foreground bg-card'
                    : 'border-transparent text-muted-foreground hover:text-foreground'
                )}
              >
                {f.path}
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
    </div>
  )
}
