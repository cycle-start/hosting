import { Link } from '@tanstack/react-router'
import { ChevronRight } from 'lucide-react'
import { Fragment } from 'react'

export interface BreadcrumbSegment {
  label: string
  href?: string
  hash?: string
}

interface BreadcrumbProps {
  segments: BreadcrumbSegment[]
}

export function Breadcrumb({ segments }: BreadcrumbProps) {
  return (
    <nav className="flex items-center gap-1 text-sm text-muted-foreground">
      {segments.map((seg, i) => (
        <Fragment key={i}>
          {i > 0 && <ChevronRight className="h-3 w-3 shrink-0" />}
          {seg.href ? (
            <Link to={seg.href} hash={seg.hash} className="hover:text-foreground transition-colors truncate max-w-[200px]">
              {seg.label}
            </Link>
          ) : (
            <span className="text-foreground font-medium truncate max-w-[200px]">{seg.label}</span>
          )}
        </Fragment>
      ))}
    </nav>
  )
}
