export type DeployMode = 'single' | 'multi'

export type NodeRole =
  | 'controlplane'
  | 'web'
  | 'database'
  | 'dns'
  | 'valkey'
  | 'email'
  | 'storage'
  | 'lb'
  | 'gateway'
  | 'dbadmin'

export interface BrandConfig {
  name: string
  platform_domain: string
  customer_domain: string
  hostmaster_email: string
  mail_hostname: string
  primary_ns: string
  primary_ns_ip: string
  secondary_ns: string
  secondary_ns_ip: string
}

export interface ControlPlaneDB {
  mode: 'builtin' | 'external'
  host: string
  port: number
  name: string
  user: string
  password: string
  ssl_mode: string
}

export interface ControlPlaneConfig {
  database: ControlPlaneDB
}

export interface NodeConfig {
  hostname: string
  ip: string
  roles: NodeRole[]
}

export interface TLSConfig {
  mode: 'letsencrypt' | 'manual'
  email: string
}

export interface EmailConfig {
  stalwart_admin_token: string
}

export interface Config {
  deploy_mode: DeployMode
  target_host: string
  ssh_user: string
  region_name: string
  cluster_name: string
  brand: BrandConfig
  control_plane: ControlPlaneConfig
  nodes: NodeConfig[]
  tls: TLSConfig
  email: EmailConfig
  api_key: string
}

export interface RoleInfo {
  id: NodeRole
  label: string
  description: string
}

export interface ValidationError {
  field: string
  message: string
}

export interface ValidateResponse {
  valid: boolean
  errors: ValidationError[]
}

export interface GeneratedFile {
  path: string
  content: string
}

export interface GenerateResponse {
  output_dir: string
  files: GeneratedFile[]
}

export type StepID =
  | 'deploy_mode'
  | 'region'
  | 'brand'
  | 'control_plane'
  | 'nodes'
  | 'tls'
  | 'review'
  | 'install'

export type DeployStepID = 'ssh_ca' | 'ansible' | 'register_api_key' | 'cluster_apply' | 'seed'

export interface DeployStepDef {
  id: DeployStepID
  label: string
  description: string
  multi_only: boolean
}

export interface ExecEvent {
  type: 'output' | 'done' | 'error'
  data?: string
  stream?: 'stdout' | 'stderr'
  exit_code?: number
}

export type StepStatus = 'pending' | 'running' | 'success' | 'failed'
