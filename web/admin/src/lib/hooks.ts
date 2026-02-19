import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api, type PaginatedResponse } from './api'
import type {
  Region, Cluster, Shard, Node, Tenant, Webroot, FQDN, Certificate,
  Zone, ZoneRecord, Database, DatabaseUser,
  ValkeyInstance, ValkeyUser, EmailAccount, EmailAlias, EmailForward, EmailAutoReply,
  S3Bucket, S3AccessKey,
  SSHKey, CronJob, Daemon, Backup, Brand,
  APIKey, APIKeyCreateResponse, AuditLogEntry, DashboardStats,
  PlatformConfig, ListParams, AuditListParams, TenantResourceSummary,
  CreateTenantRequest, WebrootEnvVar,
  FQDNFormData, DatabaseUserFormData, ValkeyUserFormData,
  LogQueryResponse,
  Incident, IncidentEvent, IncidentListParams,
  CapabilityGap, CapabilityGapListParams,
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

// Brands
export function useBrands(params?: ListParams) {
  return useQuery({
    queryKey: ['brands', params],
    queryFn: () => api.get<PaginatedResponse<Brand>>(listPath('/brands', params)),
  })
}

export function useBrand(id: string) {
  return useQuery({
    queryKey: ['brand', id],
    queryFn: () => api.get<Brand>(`/brands/${id}`),
    enabled: !!id,
  })
}

export function useCreateBrand() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (data: { name: string; base_hostname: string; primary_ns: string; secondary_ns: string; hostmaster_email: string; mail_hostname?: string; spf_includes?: string; dkim_selector?: string; dkim_public_key?: string; dmarc_policy?: string }) =>
      api.post<Brand>('/brands', data),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['brands'] }),
  })
}

export function useUpdateBrand() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (data: { id: string; name?: string; base_hostname?: string; primary_ns?: string; secondary_ns?: string; hostmaster_email?: string; mail_hostname?: string; spf_includes?: string; dkim_selector?: string; dkim_public_key?: string; dmarc_policy?: string }) =>
      api.put<Brand>(`/brands/${data.id}`, data),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['brands'] })
      qc.invalidateQueries({ queryKey: ['brand'] })
    },
  })
}

export function useDeleteBrand() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => api.delete(`/brands/${id}`),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['brands'] }),
  })
}

export function useBrandClusters(brandId: string) {
  return useQuery({
    queryKey: ['brand-clusters', brandId],
    queryFn: () => api.get<{ cluster_ids: string[] }>(`/brands/${brandId}/clusters`),
    enabled: !!brandId,
  })
}

export function useSetBrandClusters() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (data: { brand_id: string; cluster_ids: string[] }) =>
      api.put(`/brands/${data.brand_id}/clusters`, { cluster_ids: data.cluster_ids }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['brand-clusters'] }),
  })
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
    mutationFn: (data: CreateTenantRequest) =>
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
    mutationFn: (data: { tenant_id: string; runtime: string; runtime_version: string; runtime_config?: Record<string, unknown>; public_folder?: string; fqdns?: FQDNFormData[] }) =>
      api.post<Webroot>(`/tenants/${data.tenant_id}/webroots`, data),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['webroots'] }),
  })
}

export function useUpdateWebroot() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (data: { id: string; runtime?: string; runtime_version?: string; runtime_config?: Record<string, unknown>; public_folder?: string; env_file_name?: string; env_shell_source?: boolean }) =>
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

// Webroot Env Vars
export function useEnvVars(webrootId: string) {
  return useQuery({
    queryKey: ['env-vars', webrootId],
    queryFn: () => api.get<{ items: WebrootEnvVar[] }>(`/webroots/${webrootId}/env-vars`),
    enabled: !!webrootId,
  })
}

export function useSetEnvVars() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (data: { webroot_id: string; vars: { name: string; value: string; secret: boolean }[] }) =>
      api.put(`/webroots/${data.webroot_id}/env-vars`, { vars: data.vars }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['env-vars'] }),
  })
}

export function useDeleteEnvVar() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (data: { webroot_id: string; name: string }) =>
      api.delete(`/webroots/${data.webroot_id}/env-vars/${data.name}`),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['env-vars'] }),
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
export function useTenantEmailAccounts(tenantId: string) {
  return useQuery({
    queryKey: ['tenant-email-accounts', tenantId],
    queryFn: () => api.get<PaginatedResponse<EmailAccount>>(listPath(`/tenants/${tenantId}/email-accounts`)),
    enabled: !!tenantId,
  })
}

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
    mutationFn: (data: { name: string; region_id: string; brand_id?: string; tenant_id?: string }) =>
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

export function useUpdateZoneRecord() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (data: { id: string; content?: string; ttl?: number; priority?: number | null }) =>
      api.put<ZoneRecord>(`/zone-records/${data.id}`, data),
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

export function useRetryZoneRecord() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => api.post(`/zone-records/${id}/retry`),
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
    mutationFn: (data: { tenant_id: string; shard_id: string; users?: DatabaseUserFormData[] }) =>
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

// OIDC Login Sessions
export function useCreateLoginSession() {
  return useMutation({
    mutationFn: ({ tenantId, databaseId }: { tenantId: string; databaseId?: string }) => {
      const params = databaseId ? `?database_id=${databaseId}` : ''
      return api.post<{ session_id: string; expires_at: string }>(`/tenants/${tenantId}/login-sessions${params}`)
    },
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
    mutationFn: (data: { tenant_id: string; shard_id: string; max_memory_mb?: number; users?: ValkeyUserFormData[] }) =>
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

// S3 Buckets
export function useS3Buckets(tenantId: string, params?: ListParams) {
  return useQuery({
    queryKey: ['s3-buckets', tenantId, params],
    queryFn: () => api.get<PaginatedResponse<S3Bucket>>(listPath(`/tenants/${tenantId}/s3-buckets`, params)),
    enabled: !!tenantId,
  })
}

export function useS3Bucket(id: string) {
  return useQuery({
    queryKey: ['s3-bucket', id],
    queryFn: () => api.get<S3Bucket>(`/s3-buckets/${id}`),
    enabled: !!id,
  })
}

export function useCreateS3Bucket() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (data: { tenant_id: string; shard_id: string; public?: boolean; quota_bytes?: number }) =>
      api.post<S3Bucket>(`/tenants/${data.tenant_id}/s3-buckets`, data),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['s3-buckets'] }),
  })
}

export function useUpdateS3Bucket() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (data: { id: string; public?: boolean; quota_bytes?: number }) =>
      api.put<S3Bucket>(`/s3-buckets/${data.id}`, data),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['s3-buckets'] })
      qc.invalidateQueries({ queryKey: ['s3-bucket'] })
    },
  })
}

export function useDeleteS3Bucket() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => api.delete(`/s3-buckets/${id}`),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['s3-buckets'] }),
  })
}

// S3 Access Keys
export function useS3AccessKeys(bucketId: string) {
  return useQuery({
    queryKey: ['s3-access-keys', bucketId],
    queryFn: () => api.get<PaginatedResponse<S3AccessKey>>(listPath(`/s3-buckets/${bucketId}/access-keys`)),
    enabled: !!bucketId,
  })
}

export function useCreateS3AccessKey() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (data: { bucket_id: string; permissions?: string }) =>
      api.post<S3AccessKey>(`/s3-buckets/${data.bucket_id}/access-keys`, data),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['s3-access-keys'] }),
  })
}

export function useDeleteS3AccessKey() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => api.delete(`/s3-access-keys/${id}`),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['s3-access-keys'] }),
  })
}

// SSH Keys
export function useSSHKeys(tenantId: string) {
  return useQuery({
    queryKey: ['ssh-keys', tenantId],
    queryFn: () => api.get<PaginatedResponse<SSHKey>>(listPath(`/tenants/${tenantId}/ssh-keys`)),
    enabled: !!tenantId,
  })
}

export function useCreateSSHKey() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (data: { tenant_id: string; name: string; public_key: string }) =>
      api.post<SSHKey>(`/tenants/${data.tenant_id}/ssh-keys`, data),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['ssh-keys'] }),
  })
}

export function useDeleteSSHKey() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => api.delete(`/ssh-keys/${id}`),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['ssh-keys'] }),
  })
}

// Cron Jobs
export function useCronJobs(webrootId: string) {
  return useQuery({
    queryKey: ['cron-jobs', webrootId],
    queryFn: () => api.get<PaginatedResponse<CronJob>>(listPath(`/webroots/${webrootId}/cron-jobs`)),
    enabled: !!webrootId,
  })
}

export function useCreateCronJob() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (data: { webroot_id: string; schedule: string; command: string; working_directory?: string; timeout_seconds?: number; max_memory_mb?: number }) =>
      api.post<CronJob>(`/webroots/${data.webroot_id}/cron-jobs`, data),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['cron-jobs'] }),
  })
}

export function useUpdateCronJob() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (data: { id: string; schedule?: string; command?: string; working_directory?: string; timeout_seconds?: number; max_memory_mb?: number }) =>
      api.put<CronJob>(`/cron-jobs/${data.id}`, data),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['cron-jobs'] }),
  })
}

export function useDeleteCronJob() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => api.delete(`/cron-jobs/${id}`),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['cron-jobs'] }),
  })
}

export function useEnableCronJob() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => api.post(`/cron-jobs/${id}/enable`),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['cron-jobs'] }),
  })
}

export function useDisableCronJob() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => api.post(`/cron-jobs/${id}/disable`),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['cron-jobs'] }),
  })
}

export function useRetryCronJob() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => api.post(`/cron-jobs/${id}/retry`),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['cron-jobs'] }),
  })
}

// Daemons
export function useDaemons(webrootId: string) {
  return useQuery({
    queryKey: ['daemons', webrootId],
    queryFn: () => api.get<PaginatedResponse<Daemon>>(listPath(`/webroots/${webrootId}/daemons`)),
    enabled: !!webrootId,
  })
}

export function useCreateDaemon() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (data: { webroot_id: string; command: string; proxy_path?: string; num_procs?: number; stop_signal?: string; stop_wait_secs?: number; max_memory_mb?: number; environment?: Record<string, string> }) =>
      api.post<Daemon>(`/webroots/${data.webroot_id}/daemons`, data),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['daemons'] }),
  })
}

export function useUpdateDaemon() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (data: { id: string; command?: string; proxy_path?: string; num_procs?: number; stop_signal?: string; stop_wait_secs?: number; max_memory_mb?: number; environment?: Record<string, string> }) =>
      api.put<Daemon>(`/daemons/${data.id}`, data),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['daemons'] }),
  })
}

export function useDeleteDaemon() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => api.delete(`/daemons/${id}`),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['daemons'] }),
  })
}

export function useEnableDaemon() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => api.post(`/daemons/${id}/enable`),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['daemons'] }),
  })
}

export function useDisableDaemon() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => api.post(`/daemons/${id}/disable`),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['daemons'] }),
  })
}

export function useRetryDaemon() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => api.post(`/daemons/${id}/retry`),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['daemons'] }),
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

export function useAPIKey(id: string) {
  return useQuery({
    queryKey: ['api-key', id],
    queryFn: () => api.get<APIKey>(`/api-keys/${id}`),
    enabled: !!id,
  })
}

export function useCreateAPIKey() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (data: { name: string; scopes: string[]; brands: string[] }) =>
      api.post<APIKeyCreateResponse>('/api-keys', data),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['api-keys'] }),
  })
}

export function useUpdateAPIKey() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ id, ...data }: { id: string; name: string; scopes: string[]; brands: string[] }) =>
      api.put<APIKey>(`/api-keys/${id}`, data),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['api-keys'] })
      qc.invalidateQueries({ queryKey: ['api-key'] })
    },
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
    queryFn: () => api.get<PlatformConfig>('/platform/config'),
  })
}

export function useUpdatePlatformConfig() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (data: PlatformConfig) => api.put('/platform/config', data),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['platform-config'] }),
  })
}

// Retry hooks
export function useRetryTenant() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => api.post(`/tenants/${id}/retry`),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['tenants'] })
      qc.invalidateQueries({ queryKey: ['tenant'] })
    },
  })
}

export function useRetryTenantFailed() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => api.post<{ status: string; count: number }>(`/tenants/${id}/retry-failed`),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['tenants'] })
      qc.invalidateQueries({ queryKey: ['tenant'] })
      qc.invalidateQueries({ queryKey: ['webroots'] })
      qc.invalidateQueries({ queryKey: ['databases'] })
      qc.invalidateQueries({ queryKey: ['zones'] })
      qc.invalidateQueries({ queryKey: ['valkey-instances'] })
      qc.invalidateQueries({ queryKey: ['s3-buckets'] })
      qc.invalidateQueries({ queryKey: ['ssh-keys'] })
      qc.invalidateQueries({ queryKey: ['backups'] })
    },
  })
}

export function useRetryWebroot() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => api.post(`/webroots/${id}/retry`),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['webroots'] }),
  })
}

export function useRetryDatabase() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => api.post(`/databases/${id}/retry`),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['databases'] }),
  })
}

export function useRetryValkeyInstance() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => api.post(`/valkey-instances/${id}/retry`),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['valkey-instances'] }),
  })
}

export function useRetryS3Bucket() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => api.post(`/s3-buckets/${id}/retry`),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['s3-buckets'] }),
  })
}

export function useRetrySSHKey() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => api.post(`/ssh-keys/${id}/retry`),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['ssh-keys'] }),
  })
}

export function useRetryZone() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => api.post(`/zones/${id}/retry`),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['zones'] }),
  })
}

export function useRetryBackup() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => api.post(`/backups/${id}/retry`),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['backups'] }),
  })
}

// Logs
export function useLogs(query: string, range: string = '1h', enabled = true) {
  return useQuery({
    queryKey: ['logs', query, range],
    queryFn: () => api.get<LogQueryResponse>(`/logs?query=${encodeURIComponent(query)}&start=${range}&limit=500`),
    enabled,
    refetchInterval: 10000,
  })
}

export function useTenantLogs(
  tenantId: string,
  logType?: string,
  webrootId?: string,
  range: string = '1h',
  enabled = true,
) {
  const params = new URLSearchParams({ start: range, limit: '500' })
  if (logType && logType !== 'all') params.set('log_type', logType)
  if (webrootId && webrootId !== 'all') params.set('webroot_id', webrootId)

  return useQuery({
    queryKey: ['tenant-logs', tenantId, logType, webrootId, range],
    queryFn: () =>
      api.get<LogQueryResponse>(
        `/tenants/${tenantId}/logs?${params.toString()}`
      ),
    enabled,
    refetchInterval: 10000,
  })
}

// Incidents
export function useIncidents(params?: IncidentListParams) {
  return useQuery({
    queryKey: ['incidents', params],
    queryFn: () => api.get<PaginatedResponse<Incident>>(listPath('/incidents', params as Record<string, unknown>)),
  })
}

export function useIncident(id: string) {
  return useQuery({
    queryKey: ['incident', id],
    queryFn: () => api.get<Incident>(`/incidents/${id}`),
    enabled: !!id,
  })
}

export function useIncidentEvents(incidentId: string) {
  return useQuery({
    queryKey: ['incident-events', incidentId],
    queryFn: () => api.get<PaginatedResponse<IncidentEvent>>(`/incidents/${incidentId}/events?limit=100`),
    enabled: !!incidentId,
  })
}

export function useResolveIncident() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (data: { id: string; resolution: string }) =>
      api.post(`/incidents/${data.id}/resolve`, { resolution: data.resolution }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['incidents'] })
      qc.invalidateQueries({ queryKey: ['incident'] })
      qc.invalidateQueries({ queryKey: ['incident-events'] })
    },
  })
}

export function useEscalateIncident() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (data: { id: string; reason: string }) =>
      api.post(`/incidents/${data.id}/escalate`, { reason: data.reason }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['incidents'] })
      qc.invalidateQueries({ queryKey: ['incident'] })
      qc.invalidateQueries({ queryKey: ['incident-events'] })
    },
  })
}

export function useCancelIncident() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (data: { id: string; reason?: string }) =>
      api.post(`/incidents/${data.id}/cancel`, { reason: data.reason || '' }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['incidents'] })
      qc.invalidateQueries({ queryKey: ['incident'] })
      qc.invalidateQueries({ queryKey: ['incident-events'] })
    },
  })
}

// Incident-Gap linking
export function useIncidentGaps(incidentId: string) {
  return useQuery({
    queryKey: ['incident-gaps', incidentId],
    queryFn: () => api.get<PaginatedResponse<CapabilityGap>>(`/incidents/${incidentId}/gaps`),
    enabled: !!incidentId,
  })
}

export function useGapIncidents(gapId: string) {
  return useQuery({
    queryKey: ['gap-incidents', gapId],
    queryFn: () => api.get<PaginatedResponse<Incident>>(`/capability-gaps/${gapId}/incidents`),
    enabled: !!gapId,
  })
}

// Capability Gaps
export function useCapabilityGaps(params?: CapabilityGapListParams) {
  return useQuery({
    queryKey: ['capability-gaps', params],
    queryFn: () => api.get<PaginatedResponse<CapabilityGap>>(listPath('/capability-gaps', params as Record<string, unknown>)),
  })
}

export function useUpdateCapabilityGap() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (data: { id: string; status: string }) =>
      api.patch(`/capability-gaps/${data.id}`, { status: data.status }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['capability-gaps'] }),
  })
}

// Search
export interface SearchResult {
  type: 'brand' | 'tenant' | 'zone' | 'fqdn' | 'webroot' | 'database' | 'email_account' | 'valkey_instance' | 's3_bucket'
  id: string
  label: string
  tenant_id?: string
  status: string
}

export function useSearch(query: string) {
  return useQuery({
    queryKey: ['search', query],
    queryFn: () => api.get<{ results: SearchResult[] }>(`/search?q=${encodeURIComponent(query)}&limit=5`),
    enabled: query.length >= 2,
    staleTime: 30_000,
  })
}
