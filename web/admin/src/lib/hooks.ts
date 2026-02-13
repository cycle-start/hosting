import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api, type PaginatedResponse } from './api'
import type {
  Region, Cluster, Shard, Node, Tenant, Webroot, FQDN, Certificate,
  Zone, ZoneRecord, Database, DatabaseUser,
  ValkeyInstance, ValkeyUser, EmailAccount, EmailAlias, EmailForward, EmailAutoReply,
  SFTPKey, Backup,
  APIKey, APIKeyCreateResponse, AuditLogEntry, DashboardStats,
  PlatformConfig, ListParams, AuditListParams, TenantResourceSummary,
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

export function useTenantResourceSummary(tenantId: string) {
  return useQuery({
    queryKey: ['tenants', tenantId, 'resource-summary'],
    queryFn: () => api.get<TenantResourceSummary>(`/tenants/${tenantId}/resource-summary`),
    enabled: !!tenantId,
    refetchInterval: 5000,
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

export function useWebroot(id: string) {
  return useQuery({
    queryKey: ['webroot', id],
    queryFn: () => api.get<Webroot>(`/webroots/${id}`),
    enabled: !!id,
  })
}

export function useCreateWebroot() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (data: { tenant_id: string; name: string; runtime: string; runtime_version: string; runtime_config?: Record<string, unknown>; public_folder?: string }) =>
      api.post<Webroot>(`/tenants/${data.tenant_id}/webroots`, data),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['webroots'] }),
  })
}

export function useUpdateWebroot() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (data: { id: string; runtime?: string; runtime_version?: string; runtime_config?: Record<string, unknown>; public_folder?: string }) =>
      api.put<Webroot>(`/webroots/${data.id}`, data),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['webroots'] })
      qc.invalidateQueries({ queryKey: ['webroot'] })
    },
  })
}

export function useDeleteWebroot() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => api.delete(`/webroots/${id}`),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['webroots'] }),
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

export function useFQDN(id: string) {
  return useQuery({
    queryKey: ['fqdn', id],
    queryFn: () => api.get<FQDN>(`/fqdns/${id}`),
    enabled: !!id,
  })
}

export function useCreateFQDN() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (data: { webroot_id: string; fqdn: string; ssl_enabled?: boolean }) =>
      api.post<FQDN>(`/webroots/${data.webroot_id}/fqdns`, data),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['fqdns'] }),
  })
}

export function useDeleteFQDN() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => api.delete(`/fqdns/${id}`),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['fqdns'] }),
  })
}

// Certificates
export function useCertificates(fqdnId: string) {
  return useQuery({
    queryKey: ['certificates', fqdnId],
    queryFn: () => api.get<PaginatedResponse<Certificate>>(listPath(`/fqdns/${fqdnId}/certificates`)),
    enabled: !!fqdnId,
  })
}

export function useUploadCertificate() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (data: { fqdn_id: string; cert_pem: string; key_pem: string; chain_pem?: string }) =>
      api.post<Certificate>(`/fqdns/${data.fqdn_id}/certificates`, data),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['certificates'] }),
  })
}

// Email Accounts
export function useEmailAccounts(fqdnId: string) {
  return useQuery({
    queryKey: ['email-accounts', fqdnId],
    queryFn: () => api.get<PaginatedResponse<EmailAccount>>(listPath(`/fqdns/${fqdnId}/email-accounts`)),
    enabled: !!fqdnId,
  })
}

export function useEmailAccount(id: string) {
  return useQuery({
    queryKey: ['email-account', id],
    queryFn: () => api.get<EmailAccount>(`/email-accounts/${id}`),
    enabled: !!id,
  })
}

export function useCreateEmailAccount() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (data: { fqdn_id: string; address: string; display_name?: string; quota_bytes?: number }) =>
      api.post<EmailAccount>(`/fqdns/${data.fqdn_id}/email-accounts`, data),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['email-accounts'] }),
  })
}

export function useDeleteEmailAccount() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => api.delete(`/email-accounts/${id}`),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['email-accounts'] }),
  })
}

// Email Aliases
export function useEmailAliases(accountId: string) {
  return useQuery({
    queryKey: ['email-aliases', accountId],
    queryFn: () => api.get<PaginatedResponse<EmailAlias>>(listPath(`/email-accounts/${accountId}/aliases`)),
    enabled: !!accountId,
  })
}

export function useCreateEmailAlias() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (data: { account_id: string; address: string }) =>
      api.post<EmailAlias>(`/email-accounts/${data.account_id}/aliases`, data),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['email-aliases'] }),
  })
}

export function useDeleteEmailAlias() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => api.delete(`/email-aliases/${id}`),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['email-aliases'] }),
  })
}

// Email Forwards
export function useEmailForwards(accountId: string) {
  return useQuery({
    queryKey: ['email-forwards', accountId],
    queryFn: () => api.get<PaginatedResponse<EmailForward>>(listPath(`/email-accounts/${accountId}/forwards`)),
    enabled: !!accountId,
  })
}

export function useCreateEmailForward() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (data: { account_id: string; destination: string; keep_copy?: boolean }) =>
      api.post<EmailForward>(`/email-accounts/${data.account_id}/forwards`, data),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['email-forwards'] }),
  })
}

export function useDeleteEmailForward() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => api.delete(`/email-forwards/${id}`),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['email-forwards'] }),
  })
}

// Email Auto-Reply
export function useEmailAutoReply(accountId: string) {
  return useQuery({
    queryKey: ['email-autoreply', accountId],
    queryFn: () => api.get<EmailAutoReply>(`/email-accounts/${accountId}/autoreply`),
    enabled: !!accountId,
  })
}

export function useUpsertEmailAutoReply() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (data: { account_id: string; subject: string; body: string; start_date?: string; end_date?: string; enabled?: boolean }) =>
      api.put<EmailAutoReply>(`/email-accounts/${data.account_id}/autoreply`, data),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['email-autoreply'] }),
  })
}

export function useDeleteEmailAutoReply() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (accountId: string) => api.delete(`/email-accounts/${accountId}/autoreply`),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['email-autoreply'] }),
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

export function useUpdateZone() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (data: { id: string; tenant_id?: string }) =>
      api.put<Zone>(`/zones/${data.id}`, data),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['zones'] })
      qc.invalidateQueries({ queryKey: ['zone'] })
    },
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

export function useDatabase(id: string) {
  return useQuery({
    queryKey: ['database', id],
    queryFn: () => api.get<Database>(`/databases/${id}`),
    enabled: !!id,
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

export function useCreateDatabaseUser() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (data: { database_id: string; username: string; password: string; privileges: string[] }) =>
      api.post<DatabaseUser>(`/databases/${data.database_id}/users`, data),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['database-users'] }),
  })
}

export function useUpdateDatabaseUser() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (data: { id: string; password?: string; privileges?: string[] }) =>
      api.put<DatabaseUser>(`/database-users/${data.id}`, data),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['database-users'] }),
  })
}

export function useDeleteDatabaseUser() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => api.delete(`/database-users/${id}`),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['database-users'] }),
  })
}

// Valkey Instances
export function useValkeyInstances(tenantId: string, params?: ListParams) {
  return useQuery({
    queryKey: ['valkey-instances', tenantId, params],
    queryFn: () => api.get<PaginatedResponse<ValkeyInstance>>(listPath(`/tenants/${tenantId}/valkey-instances`, params)),
    enabled: !!tenantId,
  })
}

export function useValkeyInstance(id: string) {
  return useQuery({
    queryKey: ['valkey-instance', id],
    queryFn: () => api.get<ValkeyInstance>(`/valkey-instances/${id}`),
    enabled: !!id,
  })
}

export function useCreateValkeyInstance() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (data: { tenant_id: string; name: string; shard_id: string; max_memory_mb?: number }) =>
      api.post<ValkeyInstance>(`/tenants/${data.tenant_id}/valkey-instances`, data),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['valkey-instances'] }),
  })
}

export function useDeleteValkeyInstance() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => api.delete(`/valkey-instances/${id}`),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['valkey-instances'] }),
  })
}

// Valkey Users
export function useValkeyUsers(instanceId: string) {
  return useQuery({
    queryKey: ['valkey-users', instanceId],
    queryFn: () => api.get<PaginatedResponse<ValkeyUser>>(listPath(`/valkey-instances/${instanceId}/users`)),
    enabled: !!instanceId,
  })
}

export function useCreateValkeyUser() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (data: { instance_id: string; username: string; password: string; privileges: string[]; key_pattern?: string }) =>
      api.post<ValkeyUser>(`/valkey-instances/${data.instance_id}/users`, data),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['valkey-users'] }),
  })
}

export function useUpdateValkeyUser() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (data: { id: string; password?: string; privileges?: string[]; key_pattern?: string }) =>
      api.put<ValkeyUser>(`/valkey-users/${data.id}`, data),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['valkey-users'] }),
  })
}

export function useDeleteValkeyUser() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => api.delete(`/valkey-users/${id}`),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['valkey-users'] }),
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

export function useCreateSFTPKey() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (data: { tenant_id: string; name: string; public_key: string }) =>
      api.post<SFTPKey>(`/tenants/${data.tenant_id}/sftp-keys`, data),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['sftp-keys'] }),
  })
}

export function useDeleteSFTPKey() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => api.delete(`/sftp-keys/${id}`),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['sftp-keys'] }),
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

export function useCreateBackup() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (data: { tenant_id: string; type: string; source_id: string }) =>
      api.post<Backup>(`/tenants/${data.tenant_id}/backups`, data),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['backups'] }),
  })
}

export function useDeleteBackup() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => api.delete(`/backups/${id}`),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['backups'] }),
  })
}

export function useRestoreBackup() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => api.post(`/backups/${id}/restore`),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['backups'] }),
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
