import { useState, useEffect, useMemo } from 'react'
import { useNavigate, useRouterState } from '@tanstack/react-router'
import { Users, Globe, FolderOpen, Database, Boxes, HardDrive, Mail, Key, Plus, Tag } from 'lucide-react'
import {
  CommandDialog,
  CommandInput,
  CommandList,
  CommandEmpty,
  CommandGroup,
  CommandItem,
} from '@/components/ui/command'
import { StatusBadge } from '@/components/shared/status-badge'
import { useSearch, type SearchResult } from '@/lib/hooks'

interface CommandPaletteProps {
  open: boolean
  onOpenChange: (open: boolean) => void
}

const typeConfig: Record<string, { label: string; plural: string; icon: React.ElementType }> = {
  brand: { label: 'Brand', plural: 'Brands', icon: Tag },
  tenant: { label: 'Tenant', plural: 'Tenants', icon: Users },
  zone: { label: 'Zone', plural: 'Zones', icon: Globe },
  fqdn: { label: 'FQDN', plural: 'FQDNs', icon: Globe },
  webroot: { label: 'Webroot', plural: 'Webroots', icon: FolderOpen },
  database: { label: 'Database', plural: 'Databases', icon: Database },
  email_account: { label: 'Email Account', plural: 'Email Accounts', icon: Mail },
  valkey_instance: { label: 'Valkey Instance', plural: 'Valkey Instances', icon: Boxes },
  s3_bucket: { label: 'S3 Bucket', plural: 'S3 Buckets', icon: HardDrive },
}

function getDetailPath(result: SearchResult): string {
  switch (result.type) {
    case 'brand':
      return `/brands/${result.id}`
    case 'tenant':
      return `/tenants/${result.id}`
    case 'zone':
      return `/zones/${result.id}`
    case 'fqdn':
      return `/tenants/${result.tenant_id}/fqdns/${result.id}`
    case 'webroot':
      return `/tenants/${result.tenant_id}/webroots/${result.id}`
    case 'database':
      return `/tenants/${result.tenant_id}/databases/${result.id}`
    case 'email_account':
      return `/tenants/${result.tenant_id}/email-accounts/${result.id}`
    case 'valkey_instance':
      return `/tenants/${result.tenant_id}/valkey/${result.id}`
    case 's3_bucket':
      return `/tenants/${result.tenant_id}/s3-buckets/${result.id}`
    default:
      return '/'
  }
}

export function CommandPalette({ open, onOpenChange }: CommandPaletteProps) {
  const [query, setQuery] = useState('')
  const [debouncedQuery, setDebouncedQuery] = useState('')
  const navigate = useNavigate()
  const routerState = useRouterState()
  const currentPath = routerState.location.pathname

  useEffect(() => {
    const timer = setTimeout(() => setDebouncedQuery(query), 300)
    return () => clearTimeout(timer)
  }, [query])

  useEffect(() => {
    if (!open) {
      setQuery('')
      setDebouncedQuery('')
    }
  }, [open])

  const { data: searchData, isLoading } = useSearch(debouncedQuery)

  const grouped = useMemo(() => {
    if (!searchData?.results) return {}
    const groups: Record<string, SearchResult[]> = {}
    for (const r of searchData.results) {
      if (!groups[r.type]) groups[r.type] = []
      groups[r.type].push(r)
    }
    return groups
  }, [searchData])

  const tenantMatch = currentPath.match(/^\/tenants\/([^/]+)$/)
  const tenantId = tenantMatch?.[1]
  const isOnTenantDetail = !!tenantId && tenantId !== 'new'

  const handleSelect = (path: string) => {
    onOpenChange(false)
    navigate({ to: path })
  }

  const showActions = debouncedQuery.length < 2

  return (
    <CommandDialog open={open} onOpenChange={onOpenChange}>
      <CommandInput
        placeholder="Search resources or type a command..."
        value={query}
        onValueChange={setQuery}
      />
      <CommandList>
        {showActions ? (
          <>
            <CommandGroup heading="Actions">
              <CommandItem onSelect={() => handleSelect('/brands')}>
                <Plus className="mr-2 h-4 w-4" />
                Create Brand
              </CommandItem>
              <CommandItem onSelect={() => handleSelect('/tenants/new')}>
                <Plus className="mr-2 h-4 w-4" />
                Create Tenant
              </CommandItem>
              <CommandItem onSelect={() => handleSelect('/zones')}>
                <Plus className="mr-2 h-4 w-4" />
                Create Zone
              </CommandItem>
            </CommandGroup>
            {isOnTenantDetail && (
              <CommandGroup heading={`Tenant: ${tenantId}`}>
                <CommandItem onSelect={() => handleSelect(`/tenants/${tenantId}?create=webroot`)}>
                  <FolderOpen className="mr-2 h-4 w-4" />
                  Create Webroot
                </CommandItem>
                <CommandItem onSelect={() => handleSelect(`/tenants/${tenantId}?create=database`)}>
                  <Database className="mr-2 h-4 w-4" />
                  Create Database
                </CommandItem>
                <CommandItem onSelect={() => handleSelect(`/tenants/${tenantId}?create=valkey`)}>
                  <Boxes className="mr-2 h-4 w-4" />
                  Create Valkey Instance
                </CommandItem>
                <CommandItem onSelect={() => handleSelect(`/tenants/${tenantId}?create=s3_bucket`)}>
                  <HardDrive className="mr-2 h-4 w-4" />
                  Create S3 Bucket
                </CommandItem>
                <CommandItem onSelect={() => handleSelect(`/tenants/${tenantId}?create=ssh_key`)}>
                  <Key className="mr-2 h-4 w-4" />
                  Add SSH Key
                </CommandItem>
                <CommandItem onSelect={() => handleSelect(`/tenants/${tenantId}?create=zone`)}>
                  <Globe className="mr-2 h-4 w-4" />
                  Create Zone
                </CommandItem>
              </CommandGroup>
            )}
          </>
        ) : (
          <>
            {isLoading && <CommandEmpty>Searching...</CommandEmpty>}
            {!isLoading && Object.keys(grouped).length === 0 && (
              <CommandEmpty>No results found.</CommandEmpty>
            )}
            {Object.entries(grouped).map(([type, results]) => {
              const config = typeConfig[type]
              if (!config) return null
              const Icon = config.icon
              return (
                <CommandGroup key={type} heading={config.plural}>
                  {results.map((result) => (
                    <CommandItem
                      key={`${result.type}-${result.id}`}
                      value={`${result.type}-${result.label}`}
                      onSelect={() => handleSelect(getDetailPath(result))}
                    >
                      <Icon className="mr-2 h-4 w-4 shrink-0" />
                      <span className="flex-1 truncate">{result.label}</span>
                      <StatusBadge status={result.status} />
                    </CommandItem>
                  ))}
                </CommandGroup>
              )
            })}
          </>
        )}
      </CommandList>
    </CommandDialog>
  )
}
