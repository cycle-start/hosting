export interface Partner {
  id: string;
  brand_id: string;
  name: string;
  hostname: string;
  primary_color: string;
  status: string;
  created_at: string;
  updated_at: string;
}

export interface User {
  id: string;
  partner_id: string;
  email: string;
  display_name: string | null;
  locale: string;
  last_customer_id: string | null;
  created_at: string;
  updated_at: string;
}

export interface Customer {
  id: string;
  partner_id: string;
  name: string;
  email: string | null;
  status: string;
  created_at: string;
  updated_at: string;
}

export interface Subscription {
  id: string;
  customer_id: string;
  tenant_id: string;
  product_name: string;
  product_description: string | null;
  modules: string[];
  status: string;
  updated_at: string;
}

export interface Webroot {
  id: string;
  tenant_id: string;
  runtime: string;
  runtime_version: string;
  public_folder: string;
  status: string;
  status_message: string | null;
  created_at: string;
  updated_at: string;
}

export interface EnvVar {
  name: string;
  value: string;
  is_secret: boolean;
}

export interface VaultEncryptResponse {
  token: string;
}

export interface FQDN {
  id: string;
  tenant_id: string;
  webroot_id: string | null;
  fqdn: string;
  ssl_enabled: boolean;
  status: string;
  created_at: string;
  updated_at: string;
}

export interface Daemon {
  id: string;
  webroot_id: string;
  command: string;
  proxy_path: string;
  proxy_port: number;
  num_procs: number;
  stop_signal: string;
  stop_wait_secs: number;
  max_memory_mb: number;
  enabled: boolean;
  status: string;
  status_message: string | null;
  created_at: string;
  updated_at: string;
}

export interface CronJob {
  id: string;
  tenant_id: string;
  webroot_id: string;
  schedule: string;
  command: string;
  working_directory: string;
  enabled: boolean;
  timeout_seconds: number;
  max_memory_mb: number;
  status: string;
  status_message: string | null;
  created_at: string;
  updated_at: string;
}

export interface Database {
  id: string;
  tenant_id: string;
  subscription_id: string;
  shard_id: string;
  node_id: string;
  status: string;
  status_message: string | null;
  suspend_reason: string | null;
  created_at: string;
  updated_at: string;
}

export interface DatabaseUser {
  id: string;
  database_id: string;
  username: string;
  privileges: string[];
  status: string;
  status_message: string | null;
  created_at: string;
  updated_at: string;
}

export interface ValkeyInstance {
  id: string;
  tenant_id: string;
  subscription_id: string;
  port: number;
  max_memory_mb: number;
  status: string;
  status_message: string | null;
  suspend_reason: string | null;
  created_at: string;
  updated_at: string;
}

export interface ValkeyUser {
  id: string;
  valkey_instance_id: string;
  username: string;
  privileges: string[];
  key_pattern: string;
  status: string;
  status_message: string | null;
  created_at: string;
  updated_at: string;
}

export interface S3Bucket {
  id: string;
  tenant_id: string;
  subscription_id: string;
  public: boolean;
  quota_bytes: number;
  status: string;
  status_message: string | null;
  suspend_reason: string | null;
  created_at: string;
  updated_at: string;
}

export interface S3AccessKey {
  id: string;
  s3_bucket_id: string;
  access_key_id: string;
  secret_access_key?: string;
  permissions: string[];
  status: string;
  status_message: string | null;
  created_at: string;
  updated_at: string;
}

export interface EmailAccount {
  id: string;
  fqdn_id: string;
  subscription_id: string;
  address: string;
  display_name: string;
  quota_bytes: number;
  status: string;
  status_message: string | null;
  tenant_id: string;
  created_at: string;
  updated_at: string;
}

export interface EmailAlias {
  id: string;
  email_account_id: string;
  address: string;
  status: string;
  created_at: string;
  updated_at: string;
}

export interface EmailForward {
  id: string;
  email_account_id: string;
  destination: string;
  keep_copy: boolean;
  status: string;
  created_at: string;
  updated_at: string;
}

export interface EmailAutoreply {
  id: string;
  email_account_id: string;
  subject: string;
  body: string;
  start_date: string;
  end_date: string;
  enabled: boolean;
  status: string;
  created_at: string;
  updated_at: string;
}

export interface Zone {
  id: string;
  brand_id: string;
  tenant_id: string;
  subscription_id: string;
  name: string;
  region_id: string;
  status: string;
  status_message: string | null;
  suspend_reason: string | null;
  created_at: string;
  updated_at: string;
}

export interface ZoneRecord {
  id: string;
  zone_id: string;
  type: string;
  name: string;
  content: string;
  ttl: number;
  priority?: number;
  managed_by: string;
  source_type: string;
  status: string;
  status_message: string | null;
  created_at: string;
  updated_at: string;
}

export interface SSHKey {
  id: string;
  tenant_id: string;
  name: string;
  public_key: string;
  fingerprint: string;
  status: string;
  status_message: string | null;
  created_at: string;
  updated_at: string;
}

export interface Backup {
  id: string;
  tenant_id: string;
  type: string;
  source_id: string;
  source_name: string;
  storage_path: string;
  size_bytes: number;
  status: string;
  status_message: string | null;
  started_at: string | null;
  completed_at: string | null;
  created_at: string;
}

export interface RuntimeGroup {
  runtime: string;
  versions: string[];
}

export interface DashboardData {
  customer_name: string;
  subscriptions: Subscription[];
  enabled_modules: string[];
}

export interface WireGuardPeer {
  id: string;
  tenant_id: string;
  subscription_id: string;
  name: string;
  public_key: string;
  assigned_ip: string;
  peer_index: number;
  endpoint: string;
  status: string;
  status_message: string | null;
  created_at: string;
  updated_at: string;
}

export interface WireGuardPeerCreateResult {
  peer: WireGuardPeer;
  private_key: string;
  client_config: string;
}

export interface OIDCProvider {
  id: string;
  name: string;
}

export interface OIDCConnection {
  provider: string;
  email: string;
  created_at: string;
}

export interface ListResponse<T> {
  items: T[];
}
