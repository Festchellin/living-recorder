import { useState } from 'react'
import type { PreviewConfig } from '@/hooks/useMpegts'

const PRESETS = {
  smooth: { label: '流畅', width: 640, height: 360, fps: 10, crf: 35 },
  quality: { label: '画质', width: 1280, height: 720, fps: 25, crf: 23 },
}

const RESOLUTION_OPTIONS = [
  { label: '360p', width: 640, height: 360 },
  { label: '480p', width: 854, height: 480 },
  { label: '720p', width: 1280, height: 720 },
  { label: '1080p', width: 1920, height: 1080 },
]

const FPS_OPTIONS = [5, 10, 15, 25, 30]

const QUALITY_OPTIONS = [
  { label: '低 (CRF 35)', crf: 35 },
  { label: '中 (CRF 28)', crf: 28 },
  { label: '高 (CRF 23)', crf: 23 },
  { label: '最高 (CRF 18)', crf: 18 },
]

function findPresetKey(config: PreviewConfig): string | null {
  for (const key of ['smooth', 'quality'] as const) {
    const p = PRESETS[key]
    if (config.width === p.width && config.height === p.height && config.fps === p.fps && config.crf === p.crf) {
      return key
    }
  }
  return null
}

interface PreviewQualityPopoverProps {
  config: PreviewConfig
  onChange: (config: PreviewConfig) => void
  onClose: () => void
}

export function PreviewQualityPopover({ config, onChange, onClose }: PreviewQualityPopoverProps) {
  const [showAdvanced, setShowAdvanced] = useState(false)
  const activePreset = findPresetKey(config)

  const setPreset = (key: keyof typeof PRESETS) => {
    onChange({ ...PRESETS[key] })
  }

  const currentResLabel = RESOLUTION_OPTIONS.find(r => r.width === config.width && r.height === config.height)?.label

  return (
    <div
      className="absolute bottom-12 right-0 z-50 w-56 rounded-lg border border-white/10 bg-gray-900 p-3 shadow-xl"
      onClick={e => e.stopPropagation()}
      onMouseDown={e => e.stopPropagation()}
    >
      <div className="mb-2 text-xs font-medium text-white/70">预览画质</div>

      <div className="mb-3 flex gap-1">
        <button
          className={`flex-1 rounded px-2 py-1 text-xs transition-colors ${
            activePreset === 'smooth'
              ? 'bg-iridescent-blue text-white'
              : 'bg-white/10 text-white/70 hover:bg-white/20'
          }`}
          onClick={() => setPreset('smooth')}
        >
          流畅
        </button>
        <button
          className={`flex-1 rounded px-2 py-1 text-xs transition-colors ${
            activePreset === 'quality'
              ? 'bg-iridescent-blue text-white'
              : 'bg-white/10 text-white/70 hover:bg-white/20'
          }`}
          onClick={() => setPreset('quality')}
        >
          画质
        </button>
      </div>

      <button
        className="mb-2 flex w-full items-center justify-between rounded px-1 py-1 text-xs text-white/50 hover:text-white/80"
        onClick={() => setShowAdvanced(!showAdvanced)}
      >
        高级设置
        <span className={`transition-transform ${showAdvanced ? 'rotate-180' : ''}`}>▼</span>
      </button>

      {showAdvanced && (
        <div className="space-y-2">
          <div>
            <div className="mb-1 text-[11px] text-white/40">分辨率</div>
            <select
              className="w-full rounded border border-white/10 bg-white/5 px-2 py-1 text-xs text-white/80 outline-none"
              value={currentResLabel ?? '360p'}
              onChange={e => {
                const opt = RESOLUTION_OPTIONS.find(r => r.label === e.target.value)
                if (opt) onChange({ ...config, width: opt.width, height: opt.height })
              }}
            >
              {RESOLUTION_OPTIONS.map(r => (
                <option key={r.label} value={r.label}>{r.label}</option>
              ))}
            </select>
          </div>
          <div>
            <div className="mb-1 text-[11px] text-white/40">帧率</div>
            <select
              className="w-full rounded border border-white/10 bg-white/5 px-2 py-1 text-xs text-white/80 outline-none"
              value={config.fps}
              onChange={e => onChange({ ...config, fps: Number(e.target.value) })}
            >
              {FPS_OPTIONS.map(f => (
                <option key={f} value={f}>{f} fps</option>
              ))}
            </select>
          </div>
          <div>
            <div className="mb-1 text-[11px] text-white/40">画质</div>
            <select
              className="w-full rounded border border-white/10 bg-white/5 px-2 py-1 text-xs text-white/80 outline-none"
              value={QUALITY_OPTIONS.find(q => q.crf === config.crf)?.label ?? '中 (CRF 28)'}
              onChange={e => {
                const opt = QUALITY_OPTIONS.find(q => q.label === e.target.value)
                if (opt) onChange({ ...config, crf: opt.crf })
              }}
            >
              {QUALITY_OPTIONS.map(q => (
                <option key={q.crf} value={q.label}>{q.label}</option>
              ))}
            </select>
          </div>
        </div>
      )}

      <div className="mt-2 flex justify-end">
        <button
          className="rounded bg-white/10 px-2 py-1 text-xs text-white/70 hover:bg-white/20"
          onClick={onClose}
        >
          关闭
        </button>
      </div>
    </div>
  )
}
