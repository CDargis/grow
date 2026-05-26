import imageCompression from 'browser-image-compression'
import type {
  Plant, Environment, Log, Milestone, MilestoneType, PlantObservation,
  CreatePlantRequest, UpdatePlantDetailsRequest, CreateEnvironmentRequest, CreateLogRequest,
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
    update: (id: string, body: UpdatePlantDetailsRequest) =>
      request<Plant>(`/plants/${id}`, { method: 'PATCH', body: JSON.stringify(body) }),
    delete:            (id: string)                    => request<void>(`/plants/${id}`, { method: 'DELETE' }),
    assignEnvironment: (id: string, environmentId: string | null) =>
      request<void>(`/plants/${id}/environment`, { method: 'PUT', body: JSON.stringify({ environmentId }) }),
    updatePhase:  (id: string, phase: string, date?: string, loggedAt?: string) =>
      request<void>(`/plants/${id}/phase`,  { method: 'PUT', body: JSON.stringify({ phase, ...(date ? { date } : {}), ...(loggedAt ? { loggedAt } : {}) }) }),
    updateAvatar: (id: string, avatarKey: string) =>
      request<void>(`/plants/${id}/avatar`, { method: 'PUT', body: JSON.stringify({ avatarKey }) }),
    calibrate: (id: string) =>
      request<void>(`/plants/${id}/calibrate`, { method: 'POST', body: '{}' }),
  },

  environments: {
    list:   ()                                  => request<Environment[]>('/environments'),
    get:    (id: string)                        => request<Environment>(`/environments/${id}`),
    create: (body: CreateEnvironmentRequest)    => request<Environment>('/environments', { method: 'POST', body: JSON.stringify(body) }),
    update: (id: string, body: CreateEnvironmentRequest) => request<Environment>(`/environments/${id}`, { method: 'PATCH', body: JSON.stringify(body) }),
    delete: (id: string)                        => request<void>(`/environments/${id}`, { method: 'DELETE' }),
  },

  logs: {
    listForPlant: (plantId: string)             => request<Log[]>(`/plants/${plantId}/logs`),
    listForDate:  (date: string)                => request<Log[]>(`/logs?date=${date}`),
    create:       (plantId: string, body: CreateLogRequest) =>
      request<Log>(`/plants/${plantId}/logs`, { method: 'POST', body: JSON.stringify(body) }),
    update: (plantId: string, logId: string, body: CreateLogRequest) =>
      request<Log>(`/plants/${plantId}/logs/${logId}`, { method: 'PATCH', body: JSON.stringify(body) }),
    delete: (plantId: string, logId: string)    => request<void>(`/plants/${plantId}/logs/${logId}`, { method: 'DELETE' }),
  },

  milestones: {
    listForPlant: (plantId: string) => request<Milestone[]>(`/plants/${plantId}/milestones`),
    confirm: (plantId: string, milestoneType: MilestoneType, confirmedDate: string) =>
      request<void>(`/plants/${plantId}/milestones/${milestoneType}`, {
        method: 'PATCH', body: JSON.stringify({ action: 'confirm', confirmedDate }),
      }),
    skip: (plantId: string, milestoneType: MilestoneType) =>
      request<void>(`/plants/${plantId}/milestones/${milestoneType}`, {
        method: 'PATCH', body: JSON.stringify({ action: 'skip' }),
      }),
  },

  observations: {
    listForPlant: (plantId: string) => request<PlantObservation[]>(`/plants/${plantId}/observations`),
  },

  media: {
    getUploadUrl: (prefix: string, contentType: string) =>
      request<{ uploadUrl: string; key: string }>('/media/upload', {
        method: 'POST',
        body: JSON.stringify({ prefix, contentType }),
      }),
    getDownloadUrl: (key: string) =>
      request<{ url: string }>(`/media/url?key=${encodeURIComponent(key)}`),
    uploadFile: async (file: File, prefix: string): Promise<string> => {
      const toUpload = file.type.startsWith('image/')
        ? await imageCompression(file, { maxSizeMB: 1, maxWidthOrHeight: 1920, useWebWorker: true })
        : file
      const { uploadUrl, key } = await api.media.getUploadUrl(prefix, toUpload.type)
      await fetch(uploadUrl, {
        method: 'PUT',
        body: toUpload,
        headers: { 'Content-Type': toUpload.type },
      })
      return key
    },
  },
}
