import { useEffect, useRef, useState } from 'react'
import { createWebSocket, StatusMessage } from '@/lib/ws'

export function useWebSocket() {
  const [lastMessage, setLastMessage] = useState<StatusMessage | null>(null)
  const wsRef = useRef<WebSocket | null>(null)

  useEffect(() => {
    wsRef.current = createWebSocket(setLastMessage)
    return () => {
      wsRef.current?.close()
    }
  }, [])

  return { lastMessage }
}
