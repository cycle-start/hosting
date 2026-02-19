import { cn } from '@/lib/utils'

interface StatusBadgeProps {
  status: string
  className?: string
}

const statusConfig: Record<string, { bg: string; text: string; label?: string }> = {
  active: { bg: 'bg-emerald-500/10', text: 'text-emerald-500' },
  provisioning: { bg: 'bg-blue-500/10', text: 'text-blue-500' },
  pending: { bg: 'bg-yellow-500/10', text: 'text-yellow-500' },
  suspended: { bg: 'bg-orange-500/10', text: 'text-orange-500' },
  failed: { bg: 'bg-red-500/10', text: 'text-red-500' },
  deleted: { bg: 'bg-zinc-500/10', text: 'text-zinc-500' },
  error: { bg: 'bg-red-500/10', text: 'text-red-500' },
  degraded: { bg: 'bg-yellow-500/10', text: 'text-yellow-500' },
  maintenance: { bg: 'bg-purple-500/10', text: 'text-purple-500' },
  open: { bg: 'bg-red-500/10', text: 'text-red-500' },
  investigating: { bg: 'bg-blue-500/10', text: 'text-blue-500' },
  remediating: { bg: 'bg-purple-500/10', text: 'text-purple-500' },
  resolved: { bg: 'bg-emerald-500/10', text: 'text-emerald-500' },
  escalated: { bg: 'bg-orange-500/10', text: 'text-orange-500' },
  cancelled: { bg: 'bg-zinc-500/10', text: 'text-zinc-500' },
  critical: { bg: 'bg-red-500/10', text: 'text-red-500' },
  warning: { bg: 'bg-yellow-500/10', text: 'text-yellow-500' },
  info: { bg: 'bg-blue-500/10', text: 'text-blue-500' },
  implemented: { bg: 'bg-emerald-500/10', text: 'text-emerald-500' },
  wont_fix: { bg: 'bg-zinc-500/10', text: 'text-zinc-500', label: "Won't Fix" },
}

export function StatusBadge({ status, className }: StatusBadgeProps) {
  const config = statusConfig[status.toLowerCase()]

  if (!config) {
    return (
      <span
        className={cn(
          'inline-flex items-center rounded-full border px-2.5 py-0.5 text-xs font-semibold bg-secondary text-secondary-foreground',
          className
        )}
      >
        {status}
      </span>
    )
  }

  return (
    <span
      className={cn(
        'inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-semibold',
        config.bg,
        config.text,
        className
      )}
    >
      {config.label || status.charAt(0).toUpperCase() + status.slice(1)}
    </span>
  )
}
