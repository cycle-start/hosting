import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api, type PaginatedResponse } from './api'
import type {
  Region, Cluster, Shard, Node, Tenant, Webroot, FQDN,
  Zone, ZoneRecord, Database, DatabaseUser,
  ValkeyInstance, SFTPKey, Backup,
  APIKey, APIKeyCreateResponse, AuditLogEntry, DashboardStats,
  PlatformConfig, ListParams, AuditListParams,
} from './types'

function buildQuery(params?: Record<string, unknown>): string {
  if (!params) return ''
  const sp = new URLSearchParams()
  for (const [k, v] of Object.entries(params)) {
    if (v !== undefined && v !== null && v !== '') sp.set(k, String(v))
  }
  return sp.toString()
}

function listPath(base: string, params?: ListParams) {
  const q = buildQuery(params as Record<string, unknown>)
  return q ? `${base}?${q}` : base
}

// Dashboard
export function useDashboardStats() {
  return useQuery({
    queryKey: ['dashboard-stats'],
    queryFn: () => api.get<DashboardStats>('/dashboard/stats'),
  })
}

// Regions
export function useRegions(params?: ListParams) {
  return useQuery({
    queryKey: ['regions', params],
    queryFn: () => api.get<PaginatedResponse<Region>>(listPath('/regions', params)),
  })
}

export function useRegion(id: string) {
  return useQuery({
    queryKey: ['region', id],
    queryFn: () => api.get<Region>(`/regions/${id}`),
    enabled: !!id,
  })
}

export function useCreateRegion() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (data: { id: string; name: string }) => api.post<Region>('/regions', data),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['regions'] }),
  })
}

export function useDeleteRegion() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => api.delete(`/regions/${id}`),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['regions'] }),
  })
}

// Clusters
export function useClusters(regionId: string, params?: ListParams) {
  return useQuery({
    queryKey: ['clusters', regionId, params],
    queryFn: () => api.get<PaginatedResponse<Cluster>>(listPath(`/regions/${regionId}/clusters`, params)),
    enabled: !!regionId,
  })
}

export function useCluster(id: string) {
  return useQuery({
    queryKey: ['cluster', id],
    queryFn: () => api.get<Cluster>(`/clusters/${id}`),
    enabled: !!id,
  })
}

export function useCreateCluster() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (data: { id: string; name: string; region_id: string }) =>
      api.post<Cluster>(`/regions/${data.region_id}/clusters`, data),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['clusters'] }),
  })
}

export function useDeleteCluster() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => api.delete(`/clusters/${id}`),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['clusters'] }),
  })
}

// Shards
export function useShards(clusterId: string, params?: ListParams) {
  return useQuery({
    queryKey: ['shards', clusterId, params],
    queryFn: () => api.get<PaginatedResponse<Shard>>(listPath(`/clusters/${clusterId}/shards`, params)),
    enabled: !!clusterId,
  })
}

// Nodes
export function useNodes(clusterId: string, params?: ListParams) {
  return useQuery({
    queryKey: ['nodes', clusterId, params],
    queryFn: () => api.get<PaginatedResponse<Node>>(listPath(`/clusters/${clusterId}/nodes`, params)),
    enabled: !!clusterId,
  })
}

// Tenants
export function useTenants(params?: ListParams) {
  return useQuery({
    queryKey: ['tenants', params],
    queryFn: () => api.get<PaginatedResponse<Tenant>>(listPath('/tenants', params)),
  })
}

export function useTenant(id: string) {
  return useQuery({
    queryKey: ['tenant', id],
    queryFn: () => api.get<Tenant>(`/tenants/${id}`),
    enabled: !!id,
  })
}

export function useCreateTenant() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (data: { name: string; region_id: string; cluster_id: string; shard_id: string }) =>
      api.post<Tenant>('/tenants', data),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['tenants'] }),
  })
}

export function useDeleteTenant() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => api.delete(`/tenants/${id}`),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['tenants'] }),
  })
}

export function useSuspendTenant() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => api.post(`/tenants/${id}/suspend`),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['tenants'] })
      qc.invalidateQueries({ queryKey: ['tenant'] })
    },
  })
}

export function useUnsuspendTenant() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => api.post(`/tenants/${id}/unsuspend`),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['tenants'] })
      qc.invalidateQueries({ queryKey: ['tenant'] })
    },
  })
}

// Webroots
export function useWebroots(tenantId: string) {
  return useQuery({
    queryKey: ['webroots', tenantId],
    queryFn: () => api.get<PaginatedResponse<Webroot>>(listPath(`/tenants/${tenantId}/webroots`)),
    enabled: !!tenantId,
  })
}

// FQDNs
export function useFQDNs(webrootId: string) {
  return useQuery({
    queryKey: ['fqdns', webrootId],
    queryFn: () => api.get<PaginatedResponse<FQDN>>(listPath(`/webroots/${webrootId}/fqdns`)),
    enabled: !!webrootId,
  })
}

// Zones
export function useZones(params?: ListParams) {
  return useQuery({
    queryKey: ['zones', params],
    queryFn: () => api.get<PaginatedResponse<Zone>>(listPath('/zones', params)),
  })
}

export function useZone(id: string) {
  return useQuery({
    queryKey: ['zone', id],
    queryFn: () => api.get<Zone>(`/zones/${id}`),
    enabled: !!id,
  })
}

export function useCreateZone() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (data: { name: string; tenant_id: string; region_id: string }) =>
      api.post<Zone>('/zones', data),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['zones'] }),
  })
}

export function useDeleteZone() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => api.delete(`/zones/${id}`),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['zones'] }),
  })
}

// Zone Records
export function useZoneRecords(zoneId: string, params?: ListParams) {
  return useQuery({
    queryKey: ['zone-records', zoneId, params],
    queryFn: () => api.get<PaginatedResponse<ZoneRecord>>(listPath(`/zones/${zoneId}/records`, params)),
    enabled: !!zoneId,
  })
}

export function useCreateZoneRecord() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (data: { zone_id: string; type: string; name: string; content: string; ttl: number; priority?: number }) =>
      api.post<ZoneRecord>(`/zones/${data.zone_id}/records`, data),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['zone-records'] }),
  })
}

export function useDeleteZoneRecord() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => api.delete(`/zone-records/${id}`),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['zone-records'] }),
  })
}

// Databases
export function useDatabases(tenantId: string, params?: ListParams) {
  return useQuery({
    queryKey: ['databases', tenantId, params],
    queryFn: () => api.get<PaginatedResponse<Database>>(listPath(`/tenants/${tenantId}/databases`, params)),
    enabled: !!tenantId,
  })
}

export function useCreateDatabase() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (data: { tenant_id: string; name: string; shard_id: string }) =>
      api.post<Database>(`/tenants/${data.tenant_id}/databases`, data),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['databases'] }),
  })
}

export function useDeleteDatabase() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => api.delete(`/databases/${id}`),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['databases'] }),
  })
}

// Database Users
export function useDatabaseUsers(databaseId: string) {
  return useQuery({
    queryKey: ['database-users', databaseId],
    queryFn: () => api.get<PaginatedResponse<DatabaseUser>>(listPath(`/databases/${databaseId}/users`)),
    enabled: !!databaseId,
  })
}

// Valkey
export function useValkeyInstances(tenantId: string, params?: ListParams) {
  return useQuery({
    queryKey: ['valkey-instances', tenantId, params],
    queryFn: () => api.get<PaginatedResponse<ValkeyInstance>>(listPath(`/tenants/${tenantId}/valkey-instances`, params)),
    enabled: !!tenantId,
  })
}

export function useDeleteValkeyInstance() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => api.delete(`/valkey-instances/${id}`),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['valkey-instances'] }),
  })
}

// SFTP Keys
export function useSFTPKeys(tenantId: string) {
  return useQuery({
    queryKey: ['sftp-keys', tenantId],
    queryFn: () => api.get<PaginatedResponse<SFTPKey>>(listPath(`/tenants/${tenantId}/sftp-keys`)),
    enabled: !!tenantId,
  })
}

// Backups
export function useBackups(tenantId: string) {
  return useQuery({
    queryKey: ['backups', tenantId],
    queryFn: () => api.get<PaginatedResponse<Backup>>(listPath(`/tenants/${tenantId}/backups`)),
    enabled: !!tenantId,
  })
}

// API Keys
export function useAPIKeys(params?: ListParams) {
  return useQuery({
    queryKey: ['api-keys', params],
    queryFn: () => api.get<PaginatedResponse<APIKey>>(listPath('/api-keys', params)),
  })
}

export function useCreateAPIKey() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (data: { name: string; scopes: string[] }) =>
      api.post<APIKeyCreateResponse>('/api-keys', data),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['api-keys'] }),
  })
}

export function useRevokeAPIKey() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => api.delete(`/api-keys/${id}`),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['api-keys'] }),
  })
}

// Audit Logs
export function useAuditLogs(params?: AuditListParams) {
  return useQuery({
    queryKey: ['audit-logs', params],
    queryFn: () => api.get<PaginatedResponse<AuditLogEntry>>(listPath('/audit-logs', params as Record<string, unknown>)),
  })
}

// Platform Config
export function usePlatformConfig() {
  return useQuery({
    queryKey: ['platform-config'],
    queryFn: () => api.get<PlatformConfig[]>('/platform/config'),
  })
}

export function useUpdatePlatformConfig() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (data: PlatformConfig[]) => api.put('/platform/config', data),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['platform-config'] }),
  })
}
