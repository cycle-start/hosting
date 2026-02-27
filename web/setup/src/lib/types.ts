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

export interface Config {
  deploy_mode: DeployMode
  region_name: string
  cluster_name: string
  brand: BrandConfig
  control_plane: ControlPlaneConfig
  nodes: NodeConfig[]
  tls: TLSConfig
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
