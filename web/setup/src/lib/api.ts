import type { Config, ValidateResponse, GenerateResponse, RoleInfo, DeployStepDef, DeployStepID, ExecEvent, PodsResponse, PodDebugResponse } from './types'

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

export async function generate(
  onProgress: (msg: string) => void,
): Promise<GenerateResponse> {
  const res = await fetch(`${BASE}/generate`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
  })

  if (!res.ok) {
    const body = await res.text()
    throw new Error(`${res.status}: ${body}`)
  }

  const reader = res.body!.getReader()
  const decoder = new TextDecoder()
  let buffer = ''
  let result: GenerateResponse | null = null
  let error: string | null = null

  const processEvent = (event: ExecEvent) => {
    if (event.type === 'output' && event.data) {
      onProgress(event.data)
    } else if (event.type === 'done' && event.data) {
      result = JSON.parse(event.data) as GenerateResponse
    } else if (event.type === 'error') {
      error = event.data || 'Generation failed'
    }
  }

  for (;;) {
    const { done, value } = await reader.read()
    if (done) break

    buffer += decoder.decode(value, { stream: true })
    const lines = buffer.split('\n')
    buffer = lines.pop()!

    for (const line of lines) {
      if (!line.trim()) continue
      try {
        processEvent(JSON.parse(line) as ExecEvent)
      } catch {
        // skip malformed JSON lines
      }
    }
  }

  if (buffer.trim()) {
    try {
      processEvent(JSON.parse(buffer) as ExecEvent)
    } catch {
      // skip
    }
  }

  if (error) {
    throw new Error(error)
  }
  if (!result) {
    throw new Error('No result received from generate')
  }
  return result
}

export async function getRoles(): Promise<RoleInfo[]> {
  return request<RoleInfo[]>('/roles')
}

export async function getInfo(): Promise<{ output_dir: string }> {
  return request<{ output_dir: string }>('/info')
}

export async function getSteps(): Promise<DeployStepDef[]> {
  return request<DeployStepDef[]>('/steps')
}

export async function getPods(): Promise<PodsResponse> {
  return request<PodsResponse>('/pods')
}

export async function getPodDebug(namespace: string, name: string): Promise<PodDebugResponse> {
  return request<PodDebugResponse>(`/pod-debug?namespace=${encodeURIComponent(namespace)}&name=${encodeURIComponent(name)}`)
}

export function executeStep(
  stepId: DeployStepID,
  onEvent: (event: ExecEvent) => void,
): { abort: () => void; done: Promise<void> } {
  const controller = new AbortController()

  const done = (async () => {
    const res = await fetch(`${BASE}/execute`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ step: stepId }),
      signal: controller.signal,
    })

    if (!res.ok) {
      const body = await res.text()
      onEvent({ type: 'error', data: `${res.status}: ${body}` })
      return
    }

    const reader = res.body!.getReader()
    const decoder = new TextDecoder()
    let buffer = ''

    for (;;) {
      const { done, value } = await reader.read()
      if (done) break

      buffer += decoder.decode(value, { stream: true })
      const lines = buffer.split('\n')
      buffer = lines.pop()! // keep incomplete line in buffer

      for (const line of lines) {
        if (!line.trim()) continue
        try {
          onEvent(JSON.parse(line) as ExecEvent)
        } catch {
          // skip malformed lines
        }
      }
    }

    // process remaining buffer
    if (buffer.trim()) {
      try {
        onEvent(JSON.parse(buffer) as ExecEvent)
      } catch {
        // skip
      }
    }
  })()

  return { abort: () => controller.abort(), done }
}
