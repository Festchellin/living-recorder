# Preview Quality Control Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Allow users to control preview stream quality per-cell with two presets (流畅/画质) plus custom resolution/fps/CRF.

**Architecture:** Backend reads query params from WebSocket URL to build dynamic ffmpeg args. Frontend adds a gear-icon popover in each `PreviewCell` with preset buttons and expandable custom dropdowns. `useMpegtsPreview` hook accepts a config object and appends params to WS URL.

**Tech Stack:** Go (Gin/gorilla/websocket), TypeScript, React, mpegts.js

---

## File Structure

| File | Status | Responsibility |
|------|--------|---------------|
| `backend/handlers/stream.go` | Modify | Parse query params, build dynamic ffmpeg args |
| `frontend/src/hooks/useMpegts.ts` | Modify | Accept `PreviewConfig`, append to WS URL, reconnect on change |
| `frontend/src/components/PreviewQualityPopover.tsx` | Create | Preset buttons + custom dropdowns |
| `frontend/src/components/PreviewCell.tsx` | Modify | Add gear icon, manage config state, render popover |

---

### Task 1: Backend — dynamic ffmpeg args from query params

**Files:**
- Modify: `backend/handlers/stream.go:189-247`

- [ ] **Step 1: Parse query params from WebSocket request**

After `conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)`, read params from the original HTTP request:

```go
func (h *StreamHandler) PreviewWS(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	var stream models.Stream
	if err := h.db.First(&stream, id).Error; err != nil {
		return
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("preview ws upgrade failed: %v", err)
		return
	}
	defer conn.Close()

	// Parse quality parameters from query string
	q := c.Request.URL.Query()
	width := defaultInt(q.Get("width"), 640)
	height := defaultInt(q.Get("height"), 360)
	fps := defaultInt(q.Get("fps"), 10)
	crf := defaultInt(q.Get("crf"), 35)

	// ...
}
```

Add a helper near the bottom of the file (before the package ends):

```go
func defaultInt(s string, def int) int {
	if s == "" {
		return def
	}
	v, err := strconv.Atoi(s)
	if err != nil || v <= 0 {
		return def
	}
	return v
}
```

- [ ] **Step 2: Build dynamic ffmpeg scale filter**

Replace the three hardcoded scale lines:

```go
	switch hwEnc {
	case "h264_vaapi":
		args = append(args, "-c:v", "h264_vaapi")
		scaleFilter := fmt.Sprintf("scale=%d:%d,format=nv12,hwupload", width, height)
		args = append(args, "-vf", scaleFilter)
	case "h264_nvenc":
		args = append(args, "-c:v", "h264_nvenc")
		args = append(args, "-preset", "p1")
		args = append(args, "-vf", fmt.Sprintf("scale=%d:%d", width, height))
	case "h264_qsv":
		args = append(args, "-c:v", "h264_qsv")
		args = append(args, "-preset", "1")
		args = append(args, "-global_quality", fmt.Sprintf("%d", crf))
		args = append(args, "-vf", fmt.Sprintf("scale=%d:%d", width, height))
	case "h264_amf":
		args = append(args, "-c:v", "h264_amf")
		args = append(args, "-quality", "speed")
		args = append(args, "-qp_i", fmt.Sprintf("%d", crf))
		args = append(args, "-qp_p", fmt.Sprintf("%d", crf))
		args = append(args, "-vf", fmt.Sprintf("scale=%d:%d", width, height))
	case "h264_videotoolbox":
		args = append(args, "-c:v", "h264_videotoolbox")
		args = append(args, "-encoder", "speed")
		args = append(args, "-vf", fmt.Sprintf("scale=%d:%d", width, height))
		// videotoolbox doesn't support CRF; ignore crf param
	default:
		args = append(args, "-c:v", "libx264")
		args = append(args, "-preset", "ultrafast")
		args = append(args, "-tune", "zerolatency")
		args = append(args, "-vf", fmt.Sprintf("scale=%d:%d", width, height))
	}
```

- [ ] **Step 3: Replace hardcoded -r and -crf**

Replace:
```go
	args = append(args, "-r", "10")
	args = append(args, "-crf", "35")
```

With:
```go
	args = append(args, "-r", fmt.Sprintf("%d", fps))
	if hwEnc != "h264_videotoolbox" && hwEnc != "h264_amf" && hwEnc != "h264_qsv" {
		args = append(args, "-crf", fmt.Sprintf("%d", crf))
	}
```
(AMF and QSV already set their QP in Step 2's switch block; videotoolbox ignores CRF.)

- [ ] **Step 4: Build and verify**

```bash
cd backend && go build ./... && go vet ./...
```
Expected: clean build, no errors.

- [ ] **Step 5: Commit**

```bash
git add backend/handlers/stream.go
git commit -m "feat: dynamic preview quality via query params"
```

---

### Task 2: Frontend — update useMpegts hook with config

**Files:**
- Modify: `frontend/src/hooks/useMpegts.ts`

- [ ] **Step 1: Add PreviewConfig interface and update hook signature**

```typescript
import { useEffect, useRef, useState } from 'react'
import mpegts from 'mpegts.js'

export interface PreviewConfig {
  width: number
  height: number
  fps: number
  crf: number
}

export function useMpegtsPreview(streamId: number | null, config?: PreviewConfig) {
  const videoRef = useRef<HTMLVideoElement>(null)
  const playerRef = useRef<mpegts.Player | null>(null)
  const [status, setStatus] = useState('')

  useEffect(() => {
    if (!streamId) return
    let cancelled = false

    setStatus('连接中...')

    const proto = location.protocol === 'https:' ? 'wss:' : 'ws:'
    let wsUrl = `${proto}//${location.host}/api/preview/${streamId}/ws`
    if (config) {
      wsUrl += `?width=${config.width}&height=${config.height}&fps=${config.fps}&crf=${config.crf}`
    }

    if (mpegts.isSupported()) {
      const player = mpegts.createPlayer(
        { type: 'mpegts', isLive: true, url: wsUrl },
        {
          enableWorker: false,
          lazyLoad: false,
          liveBufferLatencyChasing: true,
          fixAudioTimestampGap: true,
        },
      )
      playerRef.current = player
      player.attachMediaElement(videoRef.current!)
      player.load()
      player.play()
      setStatus('已连接')

      player.on(mpegts.Events.ERROR, () => {
        if (!cancelled) setStatus('连接失败')
      })
    } else {
      setStatus('浏览器不支持 MSE')
    }

    return () => {
      cancelled = true
      if (playerRef.current) {
        playerRef.current.destroy()
        playerRef.current = null
      }
    }
  }, [streamId, config])

  return { videoRef, status }
}
```

- [ ] **Step 2: Verify TypeScript compilation**

```bash
cd frontend && npx tsc --noEmit
```
Expected: clean compilation.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/hooks/useMpegts.ts
git commit -m "feat: add PreviewConfig to useMpegtsPreview hook"
```

---

### Task 3: Frontend — create PreviewQualityPopover component

**Files:**
- Create: `frontend/src/components/PreviewQualityPopover.tsx`

- [ ] **Step 1: Create the component file**

```typescript
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

function isPresetActive(config: PreviewConfig, key: keyof typeof PRESETS): boolean {
  const p = PRESETS[key]
  return config.width === p.width && config.height === p.height && config.fps === p.fps && config.crf === p.crf
}

function findPresetKey(config: PreviewConfig): string | null {
  for (const key of ['smooth', 'quality'] as const) {
    if (isPresetActive(config, key)) return key
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
    <div className="absolute bottom-12 right-0 z-50 w-56 rounded-lg border border-white/10 bg-gray-900 p-3 shadow-xl"
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
```

- [ ] **Step 2: Verify TypeScript compilation**

```bash
cd frontend && npx tsc --noEmit
```
Expected: clean compilation.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/components/PreviewQualityPopover.tsx
git commit -m "feat: add PreviewQualityPopover component"
```

---

### Task 4: Frontend — integrate popover into PreviewCell

**Files:**
- Modify: `frontend/src/components/PreviewCell.tsx`

- [ ] **Step 1: Update PreviewCell to use config state + popover**

```typescript
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
```

- [ ] **Step 2: Verify TypeScript compilation**

```bash
cd frontend && npx tsc --noEmit
```
Expected: clean compilation.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/components/PreviewCell.tsx
git commit -m "feat: integrate preview quality popover into PreviewCell"
```

---

### Task 5: Rebuild and verify

**Files:**
- Rebuild: full project

- [ ] **Step 1: Rebuild binaries**

```bash
cd backend && go build -o ../build/living-recorder-linux . && GOOS=windows GOARCH=amd64 go build -o ../build/living-recorder.exe .
```

- [ ] **Step 2: Verify frontend builds**

```bash
cd frontend && npm run build
```

Expected: clean build with no errors.
