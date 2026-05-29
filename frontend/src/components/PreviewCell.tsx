import { useMpegtsPreview } from '@/hooks/useMpegts'
import { X, Monitor } from 'lucide-react'

interface PreviewCellProps {
  streamId?: number | null
  streamName?: string
  onDrop: (streamId: number, streamName: string) => void
  onRemove: () => void
}

export function PreviewCell({ streamId, streamName, onDrop, onRemove }: PreviewCellProps) {
  const { videoRef, status } = useMpegtsPreview(streamId ?? null)

  const handleDragOver = (e: React.DragEvent) => {
    e.preventDefault()
    e.currentTarget.classList.add('border-iridescent-blue')
  }

  const handleDragLeave = (e: React.DragEvent) => {
    e.currentTarget.classList.remove('border-iridescent-blue')
  }

  const handleDrop = (e: React.DragEvent) => {
    e.preventDefault()
    e.currentTarget.classList.remove('border-iridescent-blue')
    const id = e.dataTransfer.getData('text/streamId')
    const name = e.dataTransfer.getData('text/streamName')
    if (id) onDrop(Number(id), name)
  }

  return (
    <div
      className="relative bg-black/40 rounded-lg border border-white/10 overflow-hidden flex items-center justify-center transition-colors duration-200"
      onDragOver={handleDragOver}
      onDragLeave={handleDragLeave}
      onDrop={handleDrop}
    >
      {streamId ? (
        <>
          <video ref={videoRef} autoPlay playsInline muted className="w-full h-full object-contain" />
          <div className="absolute top-2 left-2 px-2 py-0.5 rounded bg-black/60 text-xs text-white/80">
            {streamName}
          </div>
          {status !== '已连接' && (
            <div className="absolute inset-0 flex flex-col items-center justify-center bg-black/40">
              {status && (
                <>
                  <div className="w-6 h-6 border-2 border-iridescent-blue border-t-transparent rounded-full animate-spin mb-2" />
                  <span className="text-xs text-white/50">{status}</span>
                </>
              )}
            </div>
          )}
          <button
            onClick={onRemove}
            className="absolute top-2 right-2 p-1 rounded bg-black/60 hover:bg-black/80 transition-colors"
          >
            <X className="h-3 w-3 text-white/70" />
          </button>
        </>
      ) : (
        <div className="flex flex-col items-center gap-2 text-white/30">
          <Monitor className="h-8 w-8" />
          <span className="text-xs">拖拽信号源到此</span>
        </div>
      )}
    </div>
  )
}
