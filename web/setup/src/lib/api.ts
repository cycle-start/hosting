import type { Config, ValidateResponse, GenerateResponse, RoleInfo } from './types'

const BASE = '/api'

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`${BASE}${path}`, {
    headers: { 'Content-Type': 'application/json' },
    ...init,
  })
  if (!res.ok) {
    const body = await res.text()
    throw new Error(`${res.status}: ${body}`)
  }
  return res.json()
}

export async function getConfig(): Promise<Config> {
  return request<Config>('/config')
}

export async function putConfig(config: Config): Promise<Config> {
  return request<Config>('/config', {
    method: 'PUT',
    body: JSON.stringify(config),
  })
}

export async function validate(): Promise<ValidateResponse> {
  return request<ValidateResponse>('/validate', { method: 'POST' })
}

export async function generate(): Promise<GenerateResponse> {
  return request<GenerateResponse>('/generate', { method: 'POST' })
}

export async function getRoles(): Promise<RoleInfo[]> {
  return request<RoleInfo[]>('/roles')
}
