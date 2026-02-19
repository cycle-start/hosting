import { StatusBadge } from '@/components/shared/status-badge'
import { cn } from '@/lib/utils'

interface ResourceHeaderProps {
  title: string
  subtitle?: string
  meta?: string
  status?: string
  actions?: React.ReactNode
  className?: string
}

export function ResourceHeader({
  title,
  subtitle,
  meta,
  status,
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
      {actions && <div className="flex items-center gap-2">{actions}</div>}
    </div>
  )
}
