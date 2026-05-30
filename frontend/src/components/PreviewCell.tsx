import { useState, useRef, useEffect } from 'react'
import { useMpegtsPreview } from '@/hooks/useMpegts'
import type { PreviewConfig } from '@/hooks/useMpegts'
import { PreviewQualityPopover } from './PreviewQualityPopover'
import { X, Monitor, Settings } from 'lucide-react'

const DEFAULT_PREVIEW_CONFIG: PreviewConfig = {
  width: 640,
  height: 360,
  fps: 10,
  crf: 35,
}

interface PreviewCellProps {
  streamId?: number | null
  streamName?: string
  onDrop: (streamId: number, streamName: string) => void
  onRemove: () => void
}

export function PreviewCell({ streamId, streamName, onDrop, onRemove }: PreviewCellProps) {
  const [previewConfig, setPreviewConfig] = useState<PreviewConfig>(DEFAULT_PREVIEW_CONFIG)
  const [showSettings, setShowSettings] = useState(false)
  const { videoRef, status } = useMpegtsPreview(streamId ?? null, previewConfig)

  const cellRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    function handleClickOutside(e: MouseEvent) {
      if (cellRef.current && !cellRef.current.contains(e.target as Node)) {
        setShowSettings(false)
      }
    }
    if (showSettings) {
      document.addEventListener('mousedown', handleClickOutside)
      return () => document.removeEventListener('mousedown', handleClickOutside)
    }
  }, [showSettings])

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
      ref={cellRef}
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
          <div className="absolute top-2 right-2 flex gap-1">
            <button
              onClick={() => setShowSettings(!showSettings)}
              className="p-1 rounded bg-black/60 hover:bg-black/80 transition-colors"
              title="画质设置"
            >
              <Settings className="h-3 w-3 text-white/70" />
            </button>
            <button
              onClick={onRemove}
              className="p-1 rounded bg-black/60 hover:bg-black/80 transition-colors"
            >
              <X className="h-3 w-3 text-white/70" />
            </button>
          </div>
          {showSettings && (
            <PreviewQualityPopover
              config={previewConfig}
              onChange={setPreviewConfig}
              onClose={() => setShowSettings(false)}
            />
          )}
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
