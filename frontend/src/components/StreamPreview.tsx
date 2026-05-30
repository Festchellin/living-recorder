import { useState, useRef, useEffect } from 'react'
import { useMpegtsPreview } from '@/hooks/useMpegts'
import type { PreviewConfig } from '@/hooks/useMpegts'
import { PreviewQualityPopover } from './PreviewQualityPopover'
import { X, Settings } from 'lucide-react'

const DEFAULT_PREVIEW_CONFIG: PreviewConfig = {
  width: 640,
  height: 360,
  fps: 10,
  crf: 35,
}

interface StreamPreviewProps {
  streamId: number
  onClose: () => void
}

export function StreamPreview({ streamId, onClose }: StreamPreviewProps) {
  const [previewConfig, setPreviewConfig] = useState<PreviewConfig>(DEFAULT_PREVIEW_CONFIG)
  const [showSettings, setShowSettings] = useState(false)
  const { videoRef, status } = useMpegtsPreview(streamId, previewConfig)

  const headerRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    function handleClickOutside(e: MouseEvent) {
      if (headerRef.current && !headerRef.current.contains(e.target as Node)) {
        setShowSettings(false)
      }
    }
    if (showSettings) {
      document.addEventListener('mousedown', handleClickOutside)
      return () => document.removeEventListener('mousedown', handleClickOutside)
    }
  }, [showSettings])

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/70 backdrop-blur-sm">
      <div className="glass-strong rounded-xl overflow-hidden w-full max-w-3xl mx-4">
        <div ref={headerRef} className="relative flex items-center justify-between px-4 py-3 border-b border-white/10">
          <span className="text-sm text-white/70">
            {status || '正在连接...'}
          </span>
          <div className="flex items-center gap-2">
            <button
              onClick={() => setShowSettings(!showSettings)}
              className="p-1 rounded-lg hover:bg-white/10 transition-colors cursor-pointer"
              title="画质设置"
            >
              <Settings className="h-4 w-4 text-white/70" />
            </button>
            <button
              onClick={onClose}
              className="p-1 rounded-lg hover:bg-white/10 transition-colors cursor-pointer"
            >
              <X className="h-5 w-5 text-white/70" />
            </button>
          </div>
          {showSettings && (
            <PreviewQualityPopover
              streamId={streamId}
              config={previewConfig}
              onChange={setPreviewConfig}
              onClose={() => setShowSettings(false)}
            />
          )}
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
