import { type LucideIcon } from 'lucide-react'
import { TrendingUp, TrendingDown } from 'lucide-react'
import { Card, CardContent } from '@/components/ui/card'
import { cn } from '@/lib/utils'

interface StatsCardProps {
  label: string
  value: string | number
  icon: LucideIcon
  delta?: {
    value: number
    label?: string
  }
  className?: string
}

export function StatsCard({ label, value, icon: Icon, delta, className }: StatsCardProps) {
  const isPositive = delta ? delta.value >= 0 : undefined

  return (
    <Card className={cn('', className)}>
      <CardContent className="p-6">
        <div className="flex items-center justify-between">
          <div className="space-y-1">
            <p className="text-sm font-medium text-muted-foreground">{label}</p>
            <p className="text-2xl font-bold tracking-tight">{value}</p>
          </div>
          <div className="rounded-md bg-primary/10 p-2.5">
            <Icon className="h-5 w-5 text-primary" />
          </div>
        </div>
        {delta !== undefined && (
          <div className="mt-3 flex items-center gap-1 text-xs">
            {isPositive ? (
              <TrendingUp className="h-3 w-3 text-emerald-500" />
            ) : (
              <TrendingDown className="h-3 w-3 text-red-500" />
            )}
            <span
              className={cn(
                'font-medium',
                isPositive ? 'text-emerald-500' : 'text-red-500'
              )}
            >
              {isPositive ? '+' : ''}
              {delta.value}%
            </span>
            {delta.label && (
              <span className="text-muted-foreground">{delta.label}</span>
            )}
          </div>
        )}
      </CardContent>
    </Card>
  )
}
