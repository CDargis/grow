import type {
  Plant, Environment, Log,
  CreatePlantRequest, CreateEnvironmentRequest, CreateLogRequest,
} from '@/types'

const BASE = '/api'

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`${BASE}${path}`, {
    headers: { 'Content-Type': 'application/json', ...init?.headers },
    ...init,
  })
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: res.statusText }))
    throw new Error(err.error ?? res.statusText)
  }
  if (res.status === 204) return undefined as T
  return res.json()
}

// ── Plants ────────────────────────────────────────────────────────────────────

export const api = {
  plants: {
    list:   ()                          => request<Plant[]>('/plants'),
    get:    (id: string)                => request<Plant>(`/plants/${id}`),
    create: (body: CreatePlantRequest)  => request<Plant>('/plants', { method: 'POST', body: JSON.stringify(body) }),
    delete:            (id: string)                    => request<void>(`/plants/${id}`, { method: 'DELETE' }),
    assignEnvironment: (id: string, environmentId: string | null) =>
      request<void>(`/plants/${id}/environment`, { method: 'PUT', body: JSON.stringify({ environmentId }) }),
  },

  environments: {
    list:   ()                                  => request<Environment[]>('/environments'),
    get:    (id: string)                        => request<Environment>(`/environments/${id}`),
    create: (body: CreateEnvironmentRequest)    => request<Environment>('/environments', { method: 'POST', body: JSON.stringify(body) }),
    delete: (id: string)                        => request<void>(`/environments/${id}`, { method: 'DELETE' }),
  },

  logs: {
    listForPlant: (plantId: string)             => request<Log[]>(`/plants/${plantId}/logs`),
    listForDate:  (date: string)                => request<Log[]>(`/logs?date=${date}`),
    create:       (plantId: string, body: CreateLogRequest) =>
      request<Log>(`/plants/${plantId}/logs`, { method: 'POST', body: JSON.stringify(body) }),
    delete: (plantId: string, logId: string)    => request<void>(`/plants/${plantId}/logs/${logId}`, { method: 'DELETE' }),
  },

  media: {
    getUploadUrl: (prefix: string, contentType: string) =>
      request<{ uploadUrl: string; key: string }>('/media/upload', {
        method: 'POST',
        body: JSON.stringify({ prefix, contentType }),
      }),
    uploadFile: async (file: File, prefix: string): Promise<string> => {
      const { uploadUrl, key } = await api.media.getUploadUrl(prefix, file.type)
      await fetch(uploadUrl, {
        method: 'PUT',
        body: file,
        headers: { 'Content-Type': file.type },
      })
      return key
    },
  },
}
