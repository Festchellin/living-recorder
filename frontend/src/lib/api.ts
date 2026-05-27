const BASE_URL = import.meta.env.VITE_API_URL || ''

export interface Stream {
  id: number
  name: string
  url: string
  protocol: string
  enabled: boolean
  status: string
  created_at: string
  updated_at: string
}

export interface RecordTask {
  id: number
  stream_id: number
  stream?: Stream
  output_template: string
  segment_sec: number
  video_codec: string
  video_bitrate: string
  framerate: number
  resolution: string
  audio_codec: string
  audio_bitrate: string
  storage_type: string
  schedule_cron: string
  schedule_duration: number
  loop_record: boolean
  enabled: boolean
  created_at: string
  updated_at: string
}

export interface RecordLog {
  id: number
  stream_id: number
  file_path: string
  file_size: number
  duration: number
  status: string
  error_msg: string
  started_at: string
  ended_at: string
}

export interface Status {
  total_streams: number
  active_recordings: number
  total_recordings: number
  storage_used: number
  max_parallel: number
  version: string
}

async function request<T>(url: string, options?: RequestInit): Promise<T> {
  const res = await fetch(`${BASE_URL}${url}`, {
    headers: { 'Content-Type': 'application/json' },
    ...options,
  })
  const body = await res.json()
  if (body.code !== 0) throw new Error(body.message)
  return body.data as T
}

export const api = {
  streams: {
    list: () => request<Stream[]>('/api/streams'),
    get: (id: number) => request<Stream>(`/api/streams/${id}`),
    create: (data: Partial<Stream>) =>
      request<Stream>('/api/streams', { method: 'POST', body: JSON.stringify(data) }),
    update: (id: number, data: Partial<Stream>) =>
      request<Stream>(`/api/streams/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
    delete: (id: number) => request<void>(`/api/streams/${id}`, { method: 'DELETE' }),
    start: (id: number) => request<void>(`/api/streams/${id}/start`, { method: 'POST' }),
    stop: (id: number) => request<void>(`/api/streams/${id}/stop`, { method: 'POST' }),
    logs: (id: number) => request<RecordLog[]>(`/api/streams/${id}/logs`),
  },
  tasks: {
    list: () => request<RecordTask[]>('/api/tasks'),
    create: (data: Partial<RecordTask>) =>
      request<RecordTask>('/api/tasks', { method: 'POST', body: JSON.stringify(data) }),
    update: (id: number, data: Partial<RecordTask>) =>
      request<RecordTask>(`/api/tasks/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
    delete: (id: number) => request<void>(`/api/tasks/${id}`, { method: 'DELETE' }),
  },
  status: {
    get: () => request<Status>('/api/status'),
  },
  config: {
    get: () => request<Record<string, unknown>>('/api/config'),
    update: (data: Record<string, unknown>) =>
      request<void>('/api/config', { method: 'PUT', body: JSON.stringify(data) }),
  },
}
