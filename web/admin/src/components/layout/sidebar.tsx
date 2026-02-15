import { useState, useEffect } from 'react'
import { Link, useRouterState } from '@tanstack/react-router'
import {
  LayoutDashboard,
  Globe,
  Server,
  Users,
  MapPin,
  KeyRound,
  ScrollText,
  Settings,
  ChevronLeft,
  ChevronRight,
  FileText,
  Search,
  Tag,
} from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Separator } from '@/components/ui/separator'
import { ScrollArea } from '@/components/ui/scroll-area'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { cn } from '@/lib/utils'

interface NavItem {
  label: string
  href: string
  icon: React.ElementType
  external?: boolean
}

interface NavSection {
  title: string
  items: NavItem[]
}

const navSections: NavSection[] = [
  {
    title: 'Overview',
    items: [
      { label: 'Dashboard', href: '/', icon: LayoutDashboard },
    ],
  },
  {
    title: 'Infrastructure',
    items: [
      { label: 'Regions', href: '/regions', icon: MapPin },
      { label: 'Clusters', href: '/clusters', icon: Server },
    ],
  },
  {
    title: 'Hosting',
    items: [
      { label: 'Brands', href: '/brands', icon: Tag },
      { label: 'Tenants', href: '/tenants', icon: Users },
      { label: 'Zones', href: '/zones', icon: Globe },
    ],
  },
  {
    title: 'Settings',
    items: [
      { label: 'Platform Config', href: '/platform-config', icon: Settings },
      { label: 'API Keys', href: '/api-keys', icon: KeyRound },
      { label: 'Audit Log', href: '/audit-log', icon: ScrollText },
      { label: 'API Docs', href: '/docs', icon: FileText, external: true },
    ],
  },
]

function getStoredCollapsed(): boolean {
  try {
    return localStorage.getItem('sidebar_collapsed') === 'true'
  } catch {
    return false
  }
}

interface SidebarProps {
  onSearchClick?: () => void
}

export function Sidebar({ onSearchClick }: SidebarProps) {
  const [collapsed, setCollapsed] = useState(getStoredCollapsed)
  const routerState = useRouterState()
  const currentPath = routerState.location.pathname

  useEffect(() => {
    try {
      localStorage.setItem('sidebar_collapsed', String(collapsed))
    } catch {
      // ignore
    }
  }, [collapsed])

  const isActive = (href: string) => {
    if (href === '/') return currentPath === '/'
    return currentPath.startsWith(href)
  }

  return (
    <div
      className={cn(
        'relative flex h-screen flex-col border-r bg-background transition-all duration-300',
        collapsed ? 'w-16' : 'w-64'
      )}
    >
      {/* Brand */}
      <div className="flex h-14 items-center border-b px-4">
        <div className="flex items-center gap-2 overflow-hidden">
          <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-md bg-primary">
            <Server className="h-4 w-4 text-primary-foreground" />
          </div>
          {!collapsed && (
            <span className="text-sm font-semibold tracking-tight whitespace-nowrap">
              Hosting Admin
            </span>
          )}
        </div>
      </div>

      {/* Search */}
      <div className="px-2 py-2">
        <Button
          variant="outline"
          size="sm"
          className={cn('w-full justify-start gap-2 text-muted-foreground', collapsed && 'justify-center px-0')}
          onClick={onSearchClick}
        >
          <Search className="h-4 w-4" />
          {!collapsed && <>Search... <kbd className="ml-auto rounded border bg-muted px-1.5 text-[10px] font-medium">⌘K</kbd></>}
        </Button>
      </div>

      {/* Navigation */}
      <ScrollArea className="flex-1 py-2">
        <nav className="space-y-1 px-2">
          {navSections.map((section, sectionIdx) => (
            <div key={section.title}>
              {sectionIdx > 0 && (
                <Separator className="my-2" />
              )}
              {!collapsed && (
                <p className="mb-1 px-2 pt-2 text-[11px] font-semibold uppercase tracking-wider text-muted-foreground">
                  {section.title}
                </p>
              )}
              {section.items.map((item) => {
                const active = isActive(item.href)
                const className = cn(
                  'flex items-center gap-3 rounded-md px-2 py-2 text-sm font-medium transition-colors',
                  active
                    ? 'bg-primary/10 text-primary'
                    : 'text-muted-foreground hover:bg-accent hover:text-accent-foreground',
                  collapsed && 'justify-center px-0'
                )
                const linkContent = item.external ? (
                  <a
                    href={item.href}
                    target="_blank"
                    rel="noopener noreferrer"
                    className={className}
                  >
                    <item.icon className="h-4 w-4 shrink-0" />
                    {!collapsed && <span>{item.label}</span>}
                  </a>
                ) : (
                  <Link
                    to={item.href}
                    className={className}
                  >
                    <item.icon className={cn('h-4 w-4 shrink-0', active && 'text-primary')} />
                    {!collapsed && <span>{item.label}</span>}
                  </Link>
                )

                if (collapsed) {
                  return (
                    <Tooltip key={item.href} delayDuration={0}>
                      <TooltipTrigger asChild>{linkContent}</TooltipTrigger>
                      <TooltipContent side="right">
                        <p>{item.label}</p>
                      </TooltipContent>
                    </Tooltip>
                  )
                }

                return <div key={item.href}>{linkContent}</div>
              })}
            </div>
          ))}
        </nav>
      </ScrollArea>

      {/* Collapse toggle */}
      <div className="border-t p-2">
        <Button
          variant="ghost"
          size="sm"
          className={cn('w-full', collapsed ? 'px-0' : '')}
          onClick={() => setCollapsed(!collapsed)}
        >
          {collapsed ? (
            <ChevronRight className="h-4 w-4" />
          ) : (
            <>
              <ChevronLeft className="h-4 w-4 mr-2" />
              <span className="text-xs">Collapse</span>
            </>
          )}
        </Button>
      </div>
    </div>
  )
}
