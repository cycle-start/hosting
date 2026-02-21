export interface Region {
  id: string
  name: string
  config: Record<string, unknown> | null
  created_at: string
  updated_at: string
}

export interface Cluster {
  id: string
  region_id: string
  name: string
  config: Record<string, unknown> | null
  status: string
  spec: Record<string, unknown> | null
  created_at: string
  updated_at: string
}

export interface ClusterLBAddress {
  id: string
  cluster_id: string
  address: string
  family: number
  label: string
  created_at: string
}

export interface Shard {
  id: string
  cluster_id: string
  name: string
  role: string
  lb_backend: string
  config: Record<string, unknown> | null
  status: string
  created_at: string
  updated_at: string
}

export interface Node {
  id: string
  cluster_id: string
  hostname: string
  ip_address?: string | null
  ip6_address?: string | null
  roles: string[]
  shards?: { shard_id: string; shard_role: string; shard_index: number }[]
  status: string
  created_at: string
  updated_at: string
}

export interface Brand {
  id: string
  name: string
  base_hostname: string
  primary_ns: string
  secondary_ns: string
  hostmaster_email: string
  mail_hostname?: string
  spf_includes?: string
  dkim_selector?: string
  dkim_public_key?: string
  dmarc_policy?: string
  status: string
  created_at: string
  updated_at: string
}

export interface Tenant {
  id: string
  name: string
  brand_id: string
  customer_id: string
  region_id: string
  cluster_id: string
  shard_id?: string | null
  uid: number
  sftp_enabled: boolean
  status: string
  status_message?: string
  created_at: string
  updated_at: string
  region_name?: string
  cluster_name?: string
  shard_name?: string
}

export interface Subscription {
  id: string
  tenant_id: string
  name: string
  status: string
  created_at: string
  updated_at: string
}

export interface Webroot {
  id: string
  tenant_id: string
  subscription_id: string
  name: string
  runtime: string
  runtime_version: string
  runtime_config: Record<string, unknown> | null
  public_folder: string
  env_file_name: string
  env_shell_source: boolean
  service_hostname_enabled: boolean
  status: string
  status_message?: string
  created_at: string
  updated_at: string
}

export interface WebrootEnvVar {
  name: string
  value: string
  is_secret: boolean
}

export interface FQDN {
  id: string
  tenant_id: string
  fqdn: string
  webroot_id?: string | null
  ssl_enabled: boolean
  status: string
  status_message?: string
  created_at: string
  updated_at: string
}

export interface Certificate {
  id: string
  fqdn_id: string
  type: string
  cert_pem?: string
  key_pem?: string
  chain_pem?: string
  issued_at?: string | null
  expires_at?: string | null
  status: string
  status_message?: string
  is_active: boolean
  created_at: string
  updated_at: string
}

export interface Zone {
  id: string
  brand_id: string
  tenant_id: string
  subscription_id: string
  tenant_name?: string | null
  name: string
  region_id: string
  status: string
  status_message?: string
  created_at: string
  updated_at: string
  region_name?: string
}

export interface ZoneRecord {
  id: string
  zone_id: string
  type: string
  name: string
  content: string
  ttl: number
  priority?: number | null
  managed_by: string
  source_fqdn_id?: string | null
  status: string
  status_message?: string
  created_at: string
  updated_at: string
}

export interface Database {
  id: string
  tenant_id: string
  subscription_id: string
  name: string
  shard_id?: string | null
  node_id?: string | null
  status: string
  status_message?: string
  created_at: string
  updated_at: string
  shard_name?: string
}

export interface DatabaseUser {
  id: string
  database_id: string
  username: string
  password?: string
  privileges: string[]
  status: string
  status_message?: string
  created_at: string
  updated_at: string
}

export interface ValkeyInstance {
  id: string
  tenant_id: string
  subscription_id: string
  name: string
  shard_id?: string | null
  port: number
  max_memory_mb: number
  password?: string
  status: string
  status_message?: string
  created_at: string
  updated_at: string
  shard_name?: string
}

export interface ValkeyUser {
  id: string
  valkey_instance_id: string
  username: string
  password?: string
  privileges: string[]
  key_pattern: string
  status: string
  status_message?: string
  created_at: string
  updated_at: string
}

export interface EmailAccount {
  id: string
  fqdn_id: string
  subscription_id: string
  address: string
  display_name: string
  quota_bytes: number
  status: string
  status_message?: string
  created_at: string
  updated_at: string
}

export interface EmailAlias {
  id: string
  email_account_id: string
  address: string
  status: string
  status_message?: string
  created_at: string
  updated_at: string
}

export interface EmailForward {
  id: string
  email_account_id: string
  destination: string
  keep_copy: boolean
  status: string
  status_message?: string
  created_at: string
  updated_at: string
}

export interface EmailAutoReply {
  id: string
  email_account_id: string
  subject: string
  body: string
  start_date?: string | null
  end_date?: string | null
  enabled: boolean
  status: string
  status_message?: string
  created_at: string
  updated_at: string
}

export interface CronJob {
  id: string
  tenant_id: string
  webroot_id: string
  name: string
  schedule: string
  command: string
  working_directory: string
  timeout_seconds: number
  max_memory_mb: number
  enabled: boolean
  status: string
  status_message?: string
  created_at: string
  updated_at: string
}

export interface Daemon {
  id: string
  tenant_id: string
  webroot_id: string
  name: string
  command: string
  proxy_path?: string | null
  proxy_port?: number | null
  num_procs: number
  stop_signal: string
  stop_wait_secs: number
  max_memory_mb: number
  environment: Record<string, string>
  enabled: boolean
  status: string
  status_message?: string
  created_at: string
  updated_at: string
}

export interface SSHKey {
  id: string
  tenant_id: string
  name: string
  public_key?: string
  fingerprint: string
  status: string
  status_message?: string
  created_at: string
  updated_at: string
}

export interface S3Bucket {
  id: string
  tenant_id: string
  subscription_id: string
  name: string
  shard_id?: string | null
  public: boolean
  quota_bytes: number
  status: string
  status_message?: string
  created_at: string
  updated_at: string
  shard_name?: string
}

export interface S3AccessKey {
  id: string
  s3_bucket_id: string
  access_key_id: string
  secret_access_key?: string
  permissions: string
  status: string
  status_message?: string
  created_at: string
  updated_at: string
}

export interface Backup {
  id: string
  tenant_id: string
  type: string
  source_id: string
  source_name: string
  storage_path?: string
  size_bytes: number
  status: string
  status_message?: string
  started_at?: string | null
  completed_at?: string | null
  created_at: string
  updated_at: string
}

export interface APIKey {
  id: string
  name: string
  key_prefix?: string
  scopes: string[]
  brands: string[]
  created_at: string
  revoked_at?: string | null
}

export interface APIKeyCreateResponse extends APIKey {
  key: string
}

export interface AuditLogEntry {
  id: string
  api_key_id: string
  method: string
  path: string
  resource_type: string
  resource_id: string
  status_code: number
  request_body: string
  created_at: string
}

export interface DashboardStats {
  regions: number
  clusters: number
  shards: number
  nodes: number
  tenants: number
  tenants_active: number
  tenants_suspended: number
  databases: number
  zones: number
  valkey_instances: number
  fqdns: number
  tenants_per_shard: { shard_id: string; shard_name: string; role: string; count: number }[]
  nodes_per_cluster: { cluster_id: string; cluster_name: string; count: number }[]
  tenants_by_status: { status: string; count: number }[]
  incidents_open: number
  incidents_critical: number
  incidents_escalated: number
  incidents_by_status: { status: string; count: number }[]
  capability_gaps_open: number
  mttr_minutes: number | null
}

// Platform config is a flat key-value map returned by the API.
export type PlatformConfig = Record<string, string>

export interface TenantResourceSummary {
  webroots: Record<string, number>
  fqdns: Record<string, number>
  certificates: Record<string, number>
  email_accounts: Record<string, number>
  email_aliases: Record<string, number>
  email_forwards: Record<string, number>
  email_autoreplies: Record<string, number>
  databases: Record<string, number>
  database_users: Record<string, number>
  zones: Record<string, number>
  zone_records: Record<string, number>
  valkey_instances: Record<string, number>
  valkey_users: Record<string, number>
  s3_buckets: Record<string, number>
  s3_access_keys: Record<string, number>
  ssh_keys: Record<string, number>
  backups: Record<string, number>
  total: number
  pending: number
  provisioning: number
  failed: number
}

// --- Log types ---

export interface LogEntry {
  timestamp: string
  line: string
  labels: Record<string, string>
}

export interface LogQueryResponse {
  entries: LogEntry[]
}

export interface ListParams {
  limit?: number
  cursor?: string
  search?: string
  status?: string
  sort?: string
  order?: 'asc' | 'desc'
}

export interface AuditListParams extends ListParams {
  resource_type?: string
  action?: string
  date_from?: string
  date_to?: string
}

// --- Nested creation form data types ---

export interface SubscriptionFormData { id: string; name: string }
export interface ZoneFormData { subscription_id: string; name: string }
export interface WebrootFormData {
  subscription_id: string; runtime: string; runtime_version: string
  public_folder: string; fqdns?: FQDNFormData[]
}
export interface FQDNFormData {
  fqdn: string; ssl_enabled?: boolean
  email_accounts?: EmailAccountFormData[]
}
export interface EmailAccountFormData {
  subscription_id: string; address: string; display_name?: string; quota_bytes?: number
  aliases?: { address: string }[]
  forwards?: { destination: string; keep_copy?: boolean }[]
  autoreply?: { subject: string; body: string; enabled: boolean }
}
export interface DatabaseFormData {
  subscription_id: string; shard_id: string
  users?: DatabaseUserFormData[]
}
export interface DatabaseUserFormData {
  username: string; password: string; privileges: string[]
}
export interface ValkeyInstanceFormData {
  subscription_id: string; shard_id: string; max_memory_mb?: number
  users?: ValkeyUserFormData[]
}
export interface ValkeyUserFormData {
  username: string; password: string; privileges: string[]
  key_pattern?: string
}
export interface S3BucketFormData {
  subscription_id: string; shard_id: string; public?: boolean; quota_bytes?: number
}

export interface Incident {
  id: string
  dedupe_key: string
  type: string
  severity: string
  status: string
  title: string
  detail: string
  resource_type?: string | null
  resource_id?: string | null
  source: string
  assigned_to?: string | null
  resolution?: string | null
  detected_at: string
  resolved_at?: string | null
  escalated_at?: string | null
  created_at: string
  updated_at: string
}

export interface IncidentEvent {
  id: string
  incident_id: string
  actor: string
  action: string
  detail: string
  metadata: Record<string, unknown>
  created_at: string
}

export interface CapabilityGap {
  id: string
  tool_name: string
  description: string
  category: string
  occurrences: number
  status: string
  implemented_at?: string | null
  created_at: string
  updated_at: string
}

export interface IncidentListParams extends ListParams {
  severity?: string
  type?: string
  resource_type?: string
  resource_id?: string
  source?: string
}

export interface CapabilityGapListParams extends ListParams {
  category?: string
}

export interface ResourceUsage {
  id: string
  resource_type: string
  resource_id: string
  tenant_id: string
  bytes_used: number
  collected_at: string
}

export interface SSHKeyFormData { name: string; public_key: string }

export interface CreateTenantRequest {
  brand_id: string
  customer_id: string
  region_id: string
  cluster_id: string
  shard_id: string
  sftp_enabled?: boolean
  subscriptions?: SubscriptionFormData[]
  zones?: ZoneFormData[]
  webroots?: WebrootFormData[]
  databases?: DatabaseFormData[]
  valkey_instances?: ValkeyInstanceFormData[]
  s3_buckets?: S3BucketFormData[]
  ssh_keys?: SSHKeyFormData[]
  fqdns?: FQDNFormData[]
}
