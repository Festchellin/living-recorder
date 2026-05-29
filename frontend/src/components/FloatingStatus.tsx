import { createPortal } from 'react-dom'
import { Loader2, CheckCircle2, XCircle } from 'lucide-react'

export interface StatusMsg {
  id: number
  text: string
  type: 'loading' | 'success' | 'error'
}

interface FloatingStatusProps {
  messages: StatusMsg[]
}

export function FloatingStatus({ messages }: FloatingStatusProps) {
  if (messages.length === 0) return null

  return createPortal(
    <div className="fixed bottom-4 right-4 z-[9999] w-80 max-h-72 overflow-y-auto space-y-2 pointer-events-none">
      {messages.map((msg) => (
        <div
          key={msg.id}
          className={`pointer-events-auto rounded-lg px-4 py-2.5 flex items-center gap-2.5 text-sm shadow-lg backdrop-blur-sm border ${
            msg.type === 'loading'
              ? 'bg-black/70 border-iridescent-blue/40'
              : msg.type === 'success'
              ? 'bg-black/60 border-green-500/30'
              : 'bg-black/60 border-red-500/30'
          }`}
        >
          {msg.type === 'loading' ? (
            <Loader2 className="h-4 w-4 text-iridescent-blue animate-spin flex-shrink-0" />
          ) : msg.type === 'success' ? (
            <CheckCircle2 className="h-4 w-4 text-green-400 flex-shrink-0" />
          ) : (
            <XCircle className="h-4 w-4 text-red-400 flex-shrink-0" />
          )}
          <span className="text-white/90 truncate">{msg.text}</span>
        </div>
      ))}
    </div>,
    document.body
  )
}
