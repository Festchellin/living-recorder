import { useEffect, useState, useRef, useCallback } from 'react'
import { api, Group, Stream, buildTree, flattenTree, getGroupPath } from '@/lib/api'
import { StreamCard } from '@/components/StreamCard'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogTrigger } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { FloatingStatus, StatusMsg } from '@/components/FloatingStatus'
import { Plus, Play, Square } from 'lucide-react'

const protocols = ['rtsp', 'rtmp', 'flv', 'hls']

let msgIdCounter = 0

async function mapConcurrent<T, R>(
  items: T[],
  fn: (item: T) => Promise<R>,
  concurrency: number,
): Promise<R[]> {
  const results: R[] = []
  let idx = 0
  const workers = Array.from({ length: Math.min(concurrency, items.length) }, async () => {
    while (idx < items.length) {
      const i = idx++
      results[i] = await fn(items[i])
    }
  })
  await Promise.all(workers)
  return results
}

export default function Streams() {
  const [streams, setStreams] = useState<Stream[]>([])
  const [groups, setGroups] = useState<Group[]>([])
  const [filterGroupId, setFilterGroupId] = useState('all')
  const [editing, setEditing] = useState<Partial<Stream>>({})
  const [open, setOpen] = useState(false)
  const [maxParallel, setMaxParallel] = useState(3)
  const [statusMsgs, setStatusMsgs] = useState<StatusMsg[]>([])
  const busyRef = useRef(false)

  const addMsg = useCallback((text: string, type: StatusMsg['type'] = 'loading') => {
    const id = ++msgIdCounter
    setStatusMsgs((prev) => [...prev, { id, text, type }])
    return id
  }, [])

  const updateMsg = useCallback((id: number, text: string, type: StatusMsg['type']) => {
    setStatusMsgs((prev) => prev.map((m) => (m.id === id ? { ...m, text, type } : m)))
  }, [])

  useEffect(() => {
    if (statusMsgs.length === 0) return
    const timer = setTimeout(() => {
      setStatusMsgs((prev) => prev.filter((m) => m.type === 'loading'))
    }, 6000)
    return () => clearTimeout(timer)
  }, [statusMsgs])

  const loadStreams = () => api.streams.list().then(setStreams)
  const refreshStreams = async () => {
    const updated = await api.streams.list()
    setStreams(updated)
  }
  const loadGroups = () => api.groups.list().then(setGroups)

  useEffect(() => {
    loadStreams()
    loadGroups()
  }, [])

  useEffect(() => {
    api.config.get().then((cfg) => {
      if (cfg.max_parallel) setMaxParallel(cfg.max_parallel as number)
    })
  }, [])

  const filteredStreams = filterGroupId !== 'all'
    ? streams.filter((s) => s.group_id === Number(filterGroupId))
    : streams

  const handleSave = async () => {
    if (editing.id) {
      await api.streams.update(editing.id, editing)
    } else {
      await api.streams.create(editing)
    }
    setOpen(false)
    setEditing({})
    loadStreams()
    loadGroups()
  }

  const handleStartAll = async () => {
    if (busyRef.current) return
    busyRef.current = true
    try {
      const allStreams = await api.streams.list()
      const idle = allStreams.filter(s => s.status !== 'recording')
      let started = 0
      let skipped = 0

      const results = await Promise.allSettled(
        idle.map(async (stream) => {
          const msgId = addMsg(`正在探测 ${stream.name}...`)
          try {
            const probe = await api.streams.probe(stream.id)
            if (probe.reachable) {
              return { stream, msgId }
            } else {
              updateMsg(msgId, `${stream.name} 不可达，已跳过`, 'error')
              skipped++
              return null
            }
          } catch {
            updateMsg(msgId, `${stream.name} 探测失败`, 'error')
            return null
          }
        }),
      )

      const reachable: { stream: Stream; msgId: number }[] = []
      for (const r of results) {
        if (r.status === 'fulfilled' && r.value) reachable.push(r.value)
      }

      await mapConcurrent(reachable, async ({ stream, msgId }) => {
        updateMsg(msgId, `正在启动 ${stream.name}...`, 'loading')
        try {
          await api.streams.start(stream.id)
          updateMsg(msgId, `${stream.name} 录制中`, 'success')
          started++
        } catch {
          updateMsg(msgId, `${stream.name} 启动失败`, 'error')
        }
      }, maxParallel)

      if (started > 0 || skipped > 0) {
        addMsg(`全部启动完成: ${started}个录制中, ${skipped}个跳过`, 'success')
      }
      await refreshStreams()
    } finally {
      busyRef.current = false
    }
  }

  const handleStopAll = async () => {
    if (busyRef.current) return
    busyRef.current = true
    try {
      const allStreams = await api.streams.list()
      const recording = allStreams.filter(s => s.status === 'recording')
      let stopped = 0

      await Promise.allSettled(
        recording.map(async (stream) => {
          const msgId = addMsg(`正在停止 ${stream.name}...`)
          try {
            await api.streams.stop(stream.id)
            updateMsg(msgId, `${stream.name} 已停止`, 'success')
            stopped++
          } catch {
            updateMsg(msgId, `${stream.name} 停止失败`, 'error')
          }
        }),
      )

      if (stopped > 0) {
        addMsg(`全部停止完成: ${stopped}个已停止`, 'success')
      }
      await refreshStreams()
    } finally {
      busyRef.current = false
    }
  }

  return (
    <div className="space-y-4">
      <div className="flex justify-between items-center">
        <div className="flex items-center gap-3">
          <h2 className="text-2xl font-bold text-white/90">流媒体</h2>
          <div className="h-px flex-1 bg-gradient-to-r from-white/10 to-transparent" />
        </div>
        <div className="flex gap-2 flex-wrap">
          <Select value={filterGroupId} onValueChange={setFilterGroupId}>
            <SelectTrigger className="w-36">
              <SelectValue placeholder="全部分组" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">全部分组</SelectItem>
              {buildTree(groups).flatMap((node) =>
                flattenTree([node]).map((g) => (
                  <SelectItem key={g.id} value={String(g.id)}>
                    {'\u00A0\u00A0'.repeat(g.depth)}{g.name}
                  </SelectItem>
                ))
              )}
            </SelectContent>
          </Select>
          <Button variant="outline" onClick={handleStartAll} disabled={busyRef.current}>
            <Play className="h-4 w-4 mr-1" />
            全部启动
          </Button>
          <Button variant="destructive" onClick={handleStopAll} disabled={busyRef.current}>
            <Square className="h-4 w-4 mr-1" />
            全部停止
          </Button>
          <Dialog open={open} onOpenChange={setOpen}>
            <DialogTrigger asChild>
              <Button onClick={() => setEditing({ protocol: 'rtsp' })}>
                <Plus className="h-4 w-4 mr-1" />
                添加流媒体
              </Button>
            </DialogTrigger>
            <DialogContent>
              <DialogHeader>
                <DialogTitle>{editing.id ? '编辑流媒体' : '添加流媒体'}</DialogTitle>
              </DialogHeader>
              <div className="space-y-4">
                <div className="space-y-2">
                  <Label>名称</Label>
                  <Input
                    value={editing.name || ''}
                    onChange={(e) => setEditing({ ...editing, name: e.target.value })}
                  />
                </div>
                <div className="space-y-2">
                  <Label>URL</Label>
                  <Input
                    value={editing.url || ''}
                    onChange={(e) => setEditing({ ...editing, url: e.target.value })}
                  />
                </div>
                <div className="space-y-2">
                  <Label>协议</Label>
                  <Select
                    value={editing.protocol}
                    onValueChange={(v) => setEditing({ ...editing, protocol: v })}
                  >
                    <SelectTrigger>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      {protocols.map((p) => (
                        <SelectItem key={p} value={p}>
                          {p.toUpperCase()}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
                <div className="space-y-2">
                  <Label>分组</Label>
                  <Select
                    value={editing.group_id ? String(editing.group_id) : '_none'}
                    onValueChange={(v) => setEditing({ ...editing, group_id: v === '_none' ? null : Number(v) })}
                  >
                    <SelectTrigger>
                      <SelectValue placeholder="未分组" />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="_none">未分组</SelectItem>
                      {buildTree(groups).flatMap((node) =>
                        flattenTree([node]).map((g) => (
                          <SelectItem key={g.id} value={String(g.id)}>
                            {'\u00A0\u00A0'.repeat(g.depth)}{g.name}
                          </SelectItem>
                        ))
                      )}
                    </SelectContent>
                  </Select>
                </div>
                <Button onClick={handleSave} className="w-full">
                  {editing.id ? '更新' : '创建'}
                </Button>
              </div>
            </DialogContent>
          </Dialog>
        </div>
      </div>
      <div className="grid grid-cols-3 gap-4">
        {filteredStreams.map((s) => (
          <StreamCard
            key={s.id}
            stream={s}
            groupPath={getGroupPath(groups, s.group_id)}
            onStart={async (id) => {
              const stream = streams.find((st) => st.id === id)
              if (!stream) return
              const msgId = addMsg(`正在探测 ${stream.name}...`)
              try {
                const probe = await api.streams.probe(id)
                if (probe.reachable) {
                  updateMsg(msgId, `正在录制 ${stream.name}...`, 'loading')
                  await api.streams.start(id)
                  updateMsg(msgId, `${stream.name} 录制中`, 'success')
                } else {
                  updateMsg(msgId, `${stream.name} 不可达`, 'error')
                }
              } catch {
                updateMsg(msgId, `${stream.name} 操作失败`, 'error')
              }
              await refreshStreams()
            }}
            onStop={async (id) => {
              const stream = streams.find((st) => st.id === id)
              if (!stream) return
              const msgId = addMsg(`正在停止 ${stream.name}...`)
              try {
                await api.streams.stop(id)
                updateMsg(msgId, `${stream.name} 已停止`, 'success')
              } catch {
                updateMsg(msgId, `${stream.name} 停止失败`, 'error')
              }
              await refreshStreams()
            }}
            onEdit={(stream) => {
              setEditing(stream)
              setOpen(true)
            }}
            onDelete={async (id) => {
              await api.streams.delete(id)
              loadStreams()
            }}
          />
        ))}
      </div>
      <FloatingStatus messages={statusMsgs} />
    </div>
  )
}
