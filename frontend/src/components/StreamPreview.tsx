import { useMpegtsPreview } from '@/hooks/useMpegts'
import { X } from 'lucide-react'

interface StreamPreviewProps {
  streamId: number
  onClose: () => void
}

export function StreamPreview({ streamId, onClose }: StreamPreviewProps) {
  const { videoRef, status } = useMpegtsPreview(streamId)

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/70 backdrop-blur-sm">
      <div className="glass-strong rounded-xl overflow-hidden w-full max-w-3xl mx-4">
        <div className="flex items-center justify-between px-4 py-3 border-b border-white/10">
          <span className="text-sm text-white/70">
            {status || '正在连接...'}
          </span>
          <button
            onClick={onClose}
            className="p-1 rounded-lg hover:bg-white/10 transition-colors cursor-pointer"
          >
            <X className="h-5 w-5 text-white/70" />
          </button>
        </div>
        <div className="bg-black/50 aspect-video flex items-center justify-center relative">
          <video
            ref={videoRef}
            autoPlay
            playsInline
            controls
            className="w-full h-full object-contain"
          />
          {status !== '已连接' && status !== '连接断开' && (
            <div className="absolute flex flex-col items-center gap-2">
              <div className="w-8 h-8 border-2 border-iridescent-blue border-t-transparent rounded-full animate-spin" />
              <span className="text-sm text-white/50">{status || '正在连接...'}</span>
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
