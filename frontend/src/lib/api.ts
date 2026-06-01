const BASE_URL = import.meta.env.VITE_API_URL || ''

export interface Stream {
  id: number
  name: string
  url: string
  protocol: string
  enabled: boolean
  status: string
  remark: string
  group_id: number | null
  group?: Group
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
  stream_id: number | null
  stream?: Stream
  file_path: string
  file_size: number
  duration: number
  status: string
  error_msg: string
  level: string
  message: string
  event_type: string
  started_at: string
  ended_at: string
}

export interface Group {
  id: number
  name: string
  parent_id: number | null
  children?: Group[]
  sort_order: number
  created_at: string
  updated_at: string
}

export interface ImportResult {
  success: number
  skipped: number
  errors?: { line: number; message: string }[]
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
  groups: {
    list: () => request<Group[]>('/api/groups'),
    create: (data: { name: string; parent_id?: number | null }) =>
      request<Group>('/api/groups', { method: 'POST', body: JSON.stringify(data) }),
    update: (id: number, data: { name?: string; parent_id?: number | null }) =>
      request<Group>(`/api/groups/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
    delete: (id: number) => request<void>(`/api/groups/${id}`, { method: 'DELETE' }),
    reorder: (parentId: number | null, order: number[]) =>
      request<void>('/api/groups/reorder', {
        method: 'PUT',
        body: JSON.stringify({ parent_id: parentId, order }),
      }),
  },
  streams: {
    list: () => request<Stream[]>(`/api/streams?_=${Date.now()}`),
    get: (id: number) => request<Stream>(`/api/streams/${id}`),

    create: (data: Partial<Stream>) =>
      request<Stream>('/api/streams', { method: 'POST', body: JSON.stringify(data) }),
    update: (id: number, data: Partial<Stream>) =>
      request<Stream>(`/api/streams/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
    delete: (id: number) => request<void>(`/api/streams/${id}`, { method: 'DELETE' }),
    start: (id: number) => request<void>(`/api/streams/${id}/start`, { method: 'POST' }),
    stop: (id: number) => request<void>(`/api/streams/${id}/stop`, { method: 'POST' }),
    startAll: () => request<{success: number; errors?: string[]}>('/api/streams/start-all', { method: 'POST' }),
    stopAll: () => request<{success: number; errors?: string[]}>('/api/streams/stop-all', { method: 'POST' }),
    logs: (id: number) => request<RecordLog[]>(`/api/streams/${id}/logs`),
    probe: (id: number) => request<{ reachable: boolean }>(`/api/streams/${id}/probe`),
    probeInfo: (id: number) => request<{ width: number; height: number; fps: number }>(`/api/streams/${id}/probe-info`),

    export: (format: string, ids?: number[]) =>
      fetch(`${BASE_URL}/api/streams/export`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ format, ids }),
      }).then(async (r) => {
        const disp = r.headers.get('Content-Disposition') || ''
        const match = disp.match(/filename="?(.+?)"?$/)
        const filename = match?.[1] || `streams.${format}`
        const blob = await r.blob()
        return { blob, filename }
      }),

    import: async (file: File) => {
      const form = new FormData()
      form.append('file', file)
      const res = await fetch(`${BASE_URL}/api/streams/import`, {
        method: 'POST',
        body: form,
      })
      const body = await res.json()
      if (body.code !== 0) throw new Error(body.message)
      return body.data as ImportResult
    },
  },
  tasks: {
    list: () => request<RecordTask[]>('/api/tasks'),
    create: (data: Partial<RecordTask>) =>
      request<RecordTask>('/api/tasks', { method: 'POST', body: JSON.stringify(data) }),
    update: (id: number, data: Partial<RecordTask>) =>
      request<RecordTask>(`/api/tasks/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
    delete: (id: number) => request<void>(`/api/tasks/${id}`, { method: 'DELETE' }),
  },
  logs: {
    recent: () => request<RecordLog[]>('/api/logs/recent'),
  },
  status: {
    get: () => request<Status>('/api/status'),
  },
  config: {
    get: () => request<Record<string, unknown>>('/api/config'),
    update: (data: Record<string, unknown>) =>
      request<{ code: number; message: string }>('/api/config', { method: 'PUT', body: JSON.stringify(data) }),
    testS3: (data: Record<string, unknown>) =>
      request<{ code: number; message: string }>('/api/config/test-s3', {
        method: 'POST',
        body: JSON.stringify(data),
      }),
  },
}

export function buildTree(groups: Group[]): Group[] {
  const sorted = [...groups].sort((a, b) => a.sort_order - b.sort_order)
  const map = new Map<number, Group>()
  const roots: Group[] = []

  for (const g of sorted) {
    map.set(g.id, { ...g, children: [] })
  }

  for (const g of sorted) {
    const node = map.get(g.id)!
    if (g.parent_id != null && map.has(g.parent_id)) {
      map.get(g.parent_id)!.children!.push(node)
    } else {
      roots.push(node)
    }
  }

  return roots
}

export function flattenTree(tree: Group[], depth = 0): (Group & { depth: number })[] {
  const result: (Group & { depth: number })[] = []
  for (const node of tree) {
    result.push({ ...node, depth })
    if (node.children && node.children.length > 0) {
      result.push(...flattenTree(node.children, depth + 1))
    }
  }
  return result
}

export function getGroupPath(groups: Group[], groupId: number | null): string {
  if (!groupId) return ''
  const group = groups.find((g) => g.id === groupId)
  if (!group) return ''
  const parts: string[] = [group.name]
  let pid = group.parent_id
  while (pid != null) {
    const parent = groups.find((g) => g.id === pid)
    if (!parent) break
    parts.unshift(parent.name)
    pid = parent.parent_id
  }
  return parts.join(' > ')
}
