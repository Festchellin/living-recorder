import { useEffect, useState, useCallback } from 'react'
import { api, Stream, Group } from '@/lib/api'
import { PreviewCell } from '@/components/PreviewCell'
import { PreviewSidebar } from '@/components/PreviewSidebar'
import { Button } from '@/components/ui/button'

const layoutModes = [1, 2, 4, 8, 16, 32] as const
const gridLayouts: Record<number, { cols: number; rows: number }> = {
  1: { cols: 1, rows: 1 },
  2: { cols: 2, rows: 1 },
  4: { cols: 2, rows: 2 },
  8: { cols: 4, rows: 2 },
  16: { cols: 4, rows: 4 },
  32: { cols: 6, rows: 6 },
}

interface CellState {
  streamId?: number
  streamName?: string
}

export default function Preview() {
  const [mode, setMode] = useState<number>(4)
  const [cells, setCells] = useState<CellState[]>(() => Array(4).fill({}))
  const [streams, setStreams] = useState<Stream[]>([])
  const [groups, setGroups] = useState<Group[]>([])

  useEffect(() => {
    api.streams.list().then(setStreams)
    api.groups.list().then(setGroups)
  }, [])

  const handleModeChange = (newMode: number) => {
    setCells(Array(newMode).fill({}))
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

  const layout = gridLayouts[mode]

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
          className="flex-1 grid gap-2"
          style={{
            gridTemplateColumns: `repeat(${layout.cols}, 1fr)`,
            gridTemplateRows: `repeat(${layout.rows}, 1fr)`,
          }}
        >
          {cells.slice(0, layout.cols * layout.rows).map((cell, idx) => (
            <PreviewCell
              key={idx}
              streamId={cell.streamId}
              streamName={cell.streamName}
              onDrop={(id, name) => handleDrop(idx, id, name)}
              onRemove={() => handleRemove(idx)}
            />
          ))}
        </div>
      </div>
    </div>
  )
}
