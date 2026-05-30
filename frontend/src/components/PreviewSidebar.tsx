import { useState, useEffect } from 'react'
import { Stream, Group, buildTree, api } from '@/lib/api'
import { ChevronRight, ChevronDown } from 'lucide-react'

interface PreviewSidebarProps {
  streams: Stream[]
  groups: Group[]
}

export function PreviewSidebar({ streams, groups }: PreviewSidebarProps) {
  const [expanded, setExpanded] = useState<Set<number>>(() => {
    return new Set(groups.map((g) => g.id))
  })
  const [reachable, setReachable] = useState<Record<number, boolean>>({})

  useEffect(() => {
    if (streams.length === 0) return
    setReachable({})
    streams.forEach((s) => {
      api.streams.probe(s.id).then((r) => {
        setReachable((prev) => ({ ...prev, [s.id]: r.reachable }))
      })
    })
  }, [streams])

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

  const countGroupStreams = (node: Group): number => {
    const direct = streamsByGroup.get(node.id)?.length ?? 0
    const sub = (node.children ?? []).reduce((sum, child) => sum + countGroupStreams(child), 0)
    return direct + sub
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
          <span className="text-[10px] text-white/20 ml-auto">({countGroupStreams(node)})</span>
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
                <div className={`h-2 w-2 rounded-full flex-shrink-0 ${reachable[s.id] === undefined ? 'bg-white/20' : reachable[s.id] ? 'bg-green-500' : 'bg-red-500'}`} />
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
              <div className={`h-2 w-2 rounded-full flex-shrink-0 ${reachable[s.id] === undefined ? 'bg-white/20' : reachable[s.id] ? 'bg-green-500' : 'bg-red-500'}`} />
              <span className="truncate">{s.name}</span>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
