import type { LucideIcon } from 'lucide-react'
import { StatusBadge } from '@/components/shared/status-badge'
import { cn } from '@/lib/utils'

interface ResourceHeaderProps {
  title: string
  subtitle?: string
  meta?: string
  status?: string
  icon?: LucideIcon
  actions?: React.ReactNode
  className?: string
}

export function ResourceHeader({
  title,
  subtitle,
  meta,
  status,
  icon: Icon,
  actions,
  className,
}: ResourceHeaderProps) {
  return (
    <div
      className={cn(
        'flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between',
        className
      )}
    >
      <div className="flex items-start gap-3">
        {Icon && (
          <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg bg-primary/10">
            <Icon className="h-5 w-5 text-primary" />
          </div>
        )}
        <div className="space-y-1">
          <div className="flex items-center gap-3">
            <h1 className="text-2xl font-bold tracking-tight">{title}</h1>
            {status && <StatusBadge status={status} />}
          </div>
          {subtitle && (
            <p className="text-sm text-muted-foreground">{subtitle}</p>
          )}
          {meta && (
            <p className="text-xs text-muted-foreground">{meta}</p>
          )}
        </div>
      </div>
      {actions && <div className="flex items-center gap-2">{actions}</div>}
    </div>
  )
}
