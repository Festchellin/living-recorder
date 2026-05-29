# Multi-View Preview Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development or superpowers:executing-plans to implement this plan task-by-task.

**Goal:** A multi-camera preview grid page with selectable layouts (1/2/4/8/16/32), drag-and-drop stream assignment from a tree-grouped sidebar.

**Architecture:** Frontend-only feature reusing existing `POST/DELETE /api/streams/:id/preview` endpoints + WebRTC WHEP. WHEP connection logic extracted into reusable hook.

**Tech Stack:** React/TypeScript, WebRTC WHEP, HTML5 Drag & Drop

---

### File Structure

| File | Status | Responsibility |
|------|--------|---------------|
| `src/hooks/useWebRTC.ts` | Create | WHEP connection hook |
| `src/components/PreviewCell.tsx` | Create | Grid cell with video + drop target |
| `src/components/PreviewSidebar.tsx` | Create | Stream list panel grouped by tree |
| `src/pages/Preview.tsx` | Create | Main multi-view page |
| `src/App.tsx` | Modify | Add `/preview` route |
| `src/components/Layout.tsx` | Modify | Add nav link |
| `src/components/StreamPreview.tsx` | Refactor | Use shared `useWebRTC` hook |

---

### Task 1: Create useWebRTC hook

**Files:**
- Create: `frontend/src/hooks/useWebRTC.ts`

- [ ] **Step 1: Write the hook**

```typescript
import { useEffect, useRef, useState } from 'react'

export function useWebRTC(streamId: number | null, mediamtxUrl: string, maxRetries: number) {
  const videoRef = useRef<HTMLVideoElement>(null)
  const pcRef = useRef<RTCPeerConnection | null>(null)
  const cancelledRef = useRef(false)
  const [status, setStatus] = useState('')

  useEffect(() => {
    if (!streamId) return
    cancelledRef.current = false
    const baseUrl = mediamtxUrl.replace(/\/+$/, '')
    const whepUrl = `${baseUrl}/lr/${streamId}/whep`

    let pc: RTCPeerConnection

    const connect = async () => {
      for (let i = 0; i < maxRetries; i++) {
        if (cancelledRef.current) return
        if (i > 0) {
          setStatus(`等待推流中... (${i}/${maxRetries})`)
          await new Promise((r) => setTimeout(r, 1000))
        }

        pc = new RTCPeerConnection({
          iceServers: [{ urls: 'stun:stun.l.google.com:19302' }],
        })
        pcRef.current = pc

        pc.addTransceiver('video', { direction: 'recvonly' })
        pc.addTransceiver('audio', { direction: 'recvonly' })

        pc.ontrack = (event) => {
          if (videoRef.current && event.streams[0]) {
            videoRef.current.srcObject = event.streams[0]
            setStatus('已连接')
          }
        }

        pc.oniceconnectionstatechange = () => {
          if (pc.iceConnectionState === 'disconnected' || pc.iceConnectionState === 'failed') {
            setStatus('连接断开')
          }
        }

        try {
          const offer = await pc.createOffer()
          await pc.setLocalDescription(offer)
          if (!pc.localDescription || cancelledRef.current) return

          const res = await fetch(whepUrl, {
            method: 'POST',
            headers: { 'Content-Type': 'application/sdp' },
            body: pc.localDescription.sdp,
          })

          if (res.ok) {
            const answerSdp = await res.text()
            if (!cancelledRef.current) {
              await pc.setRemoteDescription({ type: 'answer', sdp: answerSdp })
            }
            return
          }

          if (res.status === 404 && i < maxRetries - 1) {
            pc.close()
            continue
          }

          setStatus(`连接失败 (${res.status})`)
          return
        } catch {
          if (i < maxRetries - 1) {
            pc?.close()
            continue
          }
          setStatus('连接失败')
          return
        }
      }
    }

    connect()

    return () => {
      cancelledRef.current = true
      if (pcRef.current) {
        pcRef.current.close()
        pcRef.current = null
      }
    }
  }, [streamId, mediamtxUrl, maxRetries])

  return { videoRef, status }
}
```

- [ ] **Step 2: Verify TypeScript**

Run: `cd frontend && npx tsc --noEmit`
Expected: no errors

---

### Task 2: Create PreviewCell component

**Files:**
- Create: `frontend/src/components/PreviewCell.tsx`

- [ ] **Step 1: Write the component**

```typescript
import { useEffect } from 'react'
import { useWebRTC } from '@/hooks/useWebRTC'
import { api } from '@/lib/api'
import { X, Monitor } from 'lucide-react'

interface PreviewCellProps {
  streamId?: number | null
  streamName?: string
  mediamtxUrl: string
  maxRetries: number
  onDrop: (streamId: number, streamName: string) => void
  onRemove: () => void
}

export function PreviewCell({ streamId, streamName, mediamtxUrl, maxRetries, onDrop, onRemove }: PreviewCellProps) {
  const { videoRef, status } = useWebRTC(streamId ?? null, mediamtxUrl, maxRetries)

  useEffect(() => {
    if (streamId) {
      api.streams.preview(streamId).catch(() => {})
    }
    return () => {
      if (streamId) {
        api.streams.stopPreview(streamId).catch(() => {})
      }
    }
  }, [streamId])

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
      className="relative bg-black/40 rounded-lg border border-white/10 overflow-hidden aspect-video flex items-center justify-center transition-colors duration-200"
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
```

- [ ] **Step 2: Verify TypeScript**

Run: `cd frontend && npx tsc --noEmit`
Expected: no errors

---

### Task 3: Create PreviewSidebar component

**Files:**
- Create: `frontend/src/components/PreviewSidebar.tsx`

- [ ] **Step 1: Write the component**

```typescript
import { useState } from 'react'
import { Stream, Group, buildTree } from '@/lib/api'
import { ChevronRight, ChevronDown, Video } from 'lucide-react'

interface PreviewSidebarProps {
  streams: Stream[]
  groups: Group[]
}

export function PreviewSidebar({ streams, groups }: PreviewSidebarProps) {
  const [expanded, setExpanded] = useState<Set<number>>(() => {
    return new Set(groups.map((g) => g.id))
  })
  const tree = buildTree(groups)

  const toggleExpand = (id: number) => {
    setExpanded((prev) => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
  }

  const handleDragStart = (e: React.DragEvent, id: number, name: string) => {
    e.dataTransfer.setData('text/streamId', String(id))
    e.dataTransfer.setData('text/streamName', name)
    e.dataTransfer.effectAllowed = 'copy'
  }

  const streamsByGroup = new Map<number | null, Stream[]>()
  for (const s of streams) {
    const key = s.group_id ?? 0
    if (!streamsByGroup.has(key)) streamsByGroup.set(key, [])
    streamsByGroup.get(key)!.push(s)
  }

  const renderGroup = (node: Group, depth: number): JSX.Element => {
    const isExpanded = expanded.has(node.id)
    const groupStreams = streamsByGroup.get(node.id) ?? []

    return (
      <div key={node.id}>
        <div
          className="flex items-center gap-1 px-2 py-1.5 cursor-pointer hover:bg-white/[0.04] rounded text-xs text-white/60"
          style={{ paddingLeft: `${depth * 16 + 8}px` }}
          onClick={() => toggleExpand(node.id)}
        >
          {node.children && node.children.length > 0 ? (
            isExpanded ? <ChevronDown className="h-3 w-3 text-white/40" /> : <ChevronRight className="h-3 w-3 text-white/40" />
          ) : (
            <div className="w-3" />
          )}
          {node.name}
          <span className="text-[10px] text-white/20 ml-auto">({groupStreams.length})</span>
        </div>
        {isExpanded && (
          <>
            {groupStreams.map((s) => (
              <div
                key={s.id}
                draggable
                onDragStart={(e) => handleDragStart(e, s.id, s.name)}
                className="flex items-center gap-2 px-2 py-1.5 cursor-grab active:cursor-grabbing hover:bg-white/[0.06] rounded text-xs text-white/70"
                style={{ paddingLeft: `${(depth + 1) * 16 + 8}px` }}
              >
                <Video className="h-3 w-3 text-iridescent-blue flex-shrink-0" />
                <span className="truncate">{s.name}</span>
              </div>
            ))}
            {node.children?.map((child) => renderGroup(child, depth + 1))}
          </>
        )}
      </div>
    )
  }

  const ungrouped = streamsByGroup.get(0) ?? []

  return (
    <div className="w-56 glass rounded-xl p-3 overflow-y-auto flex-shrink-0 h-full">
      <div className="text-xs text-white/40 mb-2 font-medium">信号源</div>
      {tree.map((node) => renderGroup(node, 0))}
      {ungrouped.length > 0 && (
        <div>
          <div className="px-2 py-1.5 text-xs text-white/40">未分组 ({ungrouped.length})</div>
          {ungrouped.map((s) => (
            <div
              key={s.id}
              draggable
              onDragStart={(e) => handleDragStart(e, s.id, s.name)}
              className="flex items-center gap-2 px-2 py-1.5 cursor-grab active:cursor-grabbing hover:bg-white/[0.06] rounded text-xs text-white/70"
              style={{ paddingLeft: '24px' }}
            >
              <Video className="h-3 w-3 text-iridescent-blue flex-shrink-0" />
              <span className="truncate">{s.name}</span>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
```

- [ ] **Step 2: Verify TypeScript**

Run: `cd frontend && npx tsc --noEmit`
Expected: no errors

---

### Task 4: Create Preview page

**Files:**
- Create: `frontend/src/pages/Preview.tsx`

- [ ] **Step 1: Write the page**

```typescript
import { useEffect, useState, useCallback, useRef } from 'react'
import { api, Stream, Group } from '@/lib/api'
import { PreviewCell } from '@/components/PreviewCell'
import { PreviewSidebar } from '@/components/PreviewSidebar'
import { Button } from '@/components/ui/button'

const layoutModes = [1, 2, 4, 8, 16, 32] as const
const gridCols: Record<number, number> = { 1: 1, 2: 2, 4: 2, 8: 4, 16: 4, 32: 8 }

interface CellState {
  streamId?: number
  streamName?: string
}

export default function Preview() {
  const [mode, setMode] = useState<number>(4)
  const [cells, setCells] = useState<CellState[]>(() => Array(4).fill({}))
  const savedCells = useRef<Map<number, CellState[]>>(new Map())
  const cellsRef = useRef(cells)
  cellsRef.current = cells
  const [streams, setStreams] = useState<Stream[]>([])
  const [groups, setGroups] = useState<Group[]>([])
  const [mediamtxUrl, setMediamtxUrl] = useState('http://localhost:8889')
  const [maxRetries, setMaxRetries] = useState(60)

  useEffect(() => {
    api.streams.list().then(setStreams)
    api.groups.list().then(setGroups)
    api.config.get().then((cfg) => {
      if (cfg.mediamtx_url) setMediamtxUrl(cfg.mediamtx_url as string)
      if (cfg.preview_retry) setMaxRetries(cfg.preview_retry as number)
    })
  }, [])

  const handleModeChange = (newMode: number) => {
    savedCells.current.set(mode, cells)
    const saved = savedCells.current.get(newMode)
    if (saved) {
      setCells(saved)
    } else {
      setCells(Array(newMode).fill({}))
    }
    setMode(newMode)
  }

  const handleDrop = useCallback((index: number, streamId: number, streamName: string) => {
    setCells((prev) => {
      const cell = prev[index]
      const existing = prev.findIndex((c) => c.streamId === streamId)

      if (existing >= 0 && existing !== index) {
        const next = [...prev]
        next[index] = { streamId, streamName }
        next[existing] = { ...cell }
        return next
      }

      const next = [...prev]
      next[index] = { streamId, streamName }
      return next
    })
  }, [])

  const handleRemove = useCallback((index: number) => {
    setCells((prev) => {
      const next = [...prev]
      next[index] = {}
      return next
    })
  }, [])

  // Cleanup all previews on unmount
  useEffect(() => {
    return () => {
      cellsRef.current.forEach((cell) => {
        if (cell.streamId) {
          api.streams.stopPreview(cell.streamId).catch(() => {})
        }
      })
    }
  }, [])

  const cols = gridCols[mode]

  return (
    <div className="space-y-4 h-full flex flex-col">
      <div className="flex items-center gap-2 flex-shrink-0">
        <div className="flex gap-1 bg-white/[0.04] rounded-lg p-1">
          {layoutModes.map((m) => (
            <Button
              key={m}
              size="sm"
              variant={mode === m ? 'default' : 'ghost'}
              onClick={() => handleModeChange(m)}
              className="min-w-[32px]"
            >
              {m}
            </Button>
          ))}
        </div>
        <div className="h-px flex-1 bg-gradient-to-r from-white/10 to-transparent" />
        <span className="text-xs text-white/30">拖拽左侧信号源到预览格子</span>
      </div>
      <div className="flex gap-4 flex-1 min-h-0">
        <PreviewSidebar streams={streams} groups={groups} />
        <div
          className="flex-1 grid gap-2 auto-rows-fr"
          style={{ gridTemplateColumns: `repeat(${cols}, 1fr)` }}
        >
          {cells.map((cell, idx) => (
            <PreviewCell
              key={idx}
              streamId={cell.streamId}
              streamName={cell.streamName}
              mediamtxUrl={mediamtxUrl}
              maxRetries={maxRetries}
              onDrop={(id, name) => handleDrop(idx, id, name)}
              onRemove={() => handleRemove(idx)}
            />
          ))}
        </div>
      </div>
    </div>
  )
}
```

- [ ] **Step 2: Verify TypeScript**

Run: `cd frontend && npx tsc --noEmit`
Expected: no errors

---

### Task 5: Add route and nav link

**Files:**
- Modify: `frontend/src/App.tsx`
- Modify: `frontend/src/components/Layout.tsx`

- [ ] **Step 1: Update App.tsx**

```typescript
import Preview from '@/pages/Preview'

// In the Routes section, add:
<Route path="/preview" element={<Preview />} />
```

Full edit: insert after the Groups route.

- [ ] **Step 2: Update Layout.tsx**

Add `Monitor` or `Grid3x3` icon import and a nav item:
```typescript
import { LayoutDashboard, Radio, Layers, FileText, Monitor, Settings } from 'lucide-react'

const navItems = [
  { path: '/', label: '仪表盘', icon: LayoutDashboard },
  { path: '/streams', label: '流媒体', icon: Radio },
  { path: '/preview', label: '预览', icon: Monitor },
  { path: '/groups', label: '分组', icon: Layers },
  { path: '/logs', label: '日志', icon: FileText },
  { path: '/settings', label: '设置', icon: Settings },
]
```

- [ ] **Step 3: Verify TypeScript**

Run: `cd frontend && npx tsc --noEmit`
Expected: no errors

---

### Task 6: Refactor StreamPreview to use shared hook

**Files:**
- Modify: `frontend/src/components/StreamPreview.tsx`

- [ ] **Step 1: Rewrite StreamPreview to use useWebRTC**

```typescript
import { useWebRTC } from '@/hooks/useWebRTC'
import { X } from 'lucide-react'

interface StreamPreviewProps {
  streamId: number
  mediamtxUrl: string
  maxRetries?: number
  onClose: () => void
}

export function StreamPreview({ streamId, mediamtxUrl, maxRetries = 60, onClose }: StreamPreviewProps) {
  const { videoRef, status } = useWebRTC(streamId, mediamtxUrl, maxRetries)

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
```

- [ ] **Step 2: Verify TypeScript**

Run: `cd frontend && npx tsc --noEmit`
Expected: no errors

---

### Task 7: Build and verify

- [ ] **Step 1: Full build**

Run:
```bash
cd frontend && npm run build
rm -rf backend/embed/dist && cp -r frontend/dist backend/embed/dist
cd backend && GOOS=linux GOARCH=amd64 go build -o ../build/living-recorder-linux .
GOOS=windows GOARCH=amd64 go build -o ../build/living-recorder.exe .
```
Expected: all compiled with no errors
