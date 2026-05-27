export type StatusMessage = {
  type: 'status' | 'stream_update'
  active_streams?: Array<{
    stream_id: number
    status: string
    started_at: string
  }>
  stream_status?: {
    stream_id: number
    status: string
    duration_sec: number
  }
  storage_used?: number
}

export function createWebSocket(onMessage: (msg: StatusMessage) => void): WebSocket {
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  const wsUrl = `${protocol}//${window.location.host}/api/ws`
  const ws = new WebSocket(wsUrl)

  ws.onmessage = (event) => {
    try {
      const msg = JSON.parse(event.data) as StatusMessage
      onMessage(msg)
    } catch {
      // ignore malformed messages
    }
  }

  ws.onclose = () => {
    setTimeout(() => {
      createWebSocket(onMessage)
    }, 5000)
  }

  return ws
}
