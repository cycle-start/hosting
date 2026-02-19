export interface PaginatedResponse<T> {
  items: T[]
  has_more: boolean
  next_cursor?: string
}

class ApiClient {
  private baseUrl = '/api/v1'
  private apiKey: string | null = null

  setApiKey(key: string | null) {
    this.apiKey = key
    if (key) {
      localStorage.setItem('api_key', key)
    } else {
      localStorage.removeItem('api_key')
    }
  }

  getApiKey(): string | null {
    if (this.apiKey) return this.apiKey
    this.apiKey = localStorage.getItem('api_key')
    return this.apiKey
  }

  isAuthenticated(): boolean {
    return !!this.getApiKey()
  }

  private async request<T>(method: string, path: string, body?: unknown): Promise<T> {
    const headers: Record<string, string> = {
      'Content-Type': 'application/json',
    }
    const key = this.getApiKey()
    if (key) {
      headers['Authorization'] = `Bearer ${key}`
    }

    const res = await fetch(`${this.baseUrl}${path}`, {
      method,
      headers,
      body: body ? JSON.stringify(body) : undefined,
    })

    if (res.status === 401) {
      this.setApiKey(null)
      if (window.location.pathname !== '/login') {
        window.location.href = '/login'
      }
      throw new Error('Invalid API key')
    }

    if (res.status === 204) {
      return undefined as T
    }

    const data = await res.json()

    if (!res.ok) {
      throw new Error(data.error || `Request failed: ${res.status}`)
    }

    return data as T
  }

  get<T>(path: string) {
    return this.request<T>('GET', path)
  }
  post<T>(path: string, body?: unknown) {
    return this.request<T>('POST', path, body)
  }
  put<T>(path: string, body?: unknown) {
    return this.request<T>('PUT', path, body)
  }
  patch<T>(path: string, body?: unknown) {
    return this.request<T>('PATCH', path, body)
  }
  delete<T>(path: string) {
    return this.request<T>('DELETE', path)
  }
}

export const api = new ApiClient()
