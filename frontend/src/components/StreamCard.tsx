import { useState, useEffect } from 'react'
import { Stream } from '@/lib/api'
import { StatusBadge } from './StatusBadge'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Play, Square, Pencil, Trash2, Video, Eye, EyeOff } from 'lucide-react'
import { StreamPreview } from './StreamPreview'
import { api } from '@/lib/api'

interface StreamCardProps {
  stream: Stream
  groupPath?: string
  onStart: (id: number) => void
  onStop: (id: number) => void
  onEdit: (stream: Stream) => void
  onDelete: (id: number) => void
}

export function StreamCard({ stream, groupPath, onStart, onStop, onEdit, onDelete }: StreamCardProps) {
  const [previewing, setPreviewing] = useState(false)
  const [reachable, setReachable] = useState<boolean | null>(null)
  const isRecording = stream.status === 'recording'

  useEffect(() => {
    let cancelled = false
    api.streams.probe(stream.id).then((res) => {
      if (!cancelled) setReachable(res.reachable)
    }).catch(() => {
      if (!cancelled) setReachable(false)
    })
    return () => { cancelled = true }
  }, [stream.id])

  const handlePreview = () => setPreviewing(true)

  const handleStopPreview = () => setPreviewing(false)

  return (
    <>
      <Card>
        <CardHeader className="flex flex-row items-center justify-between">
          <div className="flex items-center gap-2">
            <Video className="h-4 w-4 text-iridescent-blue" />
            <CardTitle className="text-base">{stream.name}</CardTitle>
            <div
              className={`h-2 w-2 rounded-full flex-shrink-0 transition-colors ${
                reachable === null ? 'bg-white/20' : reachable ? 'bg-green-500' : 'bg-red-500'
              }`}
              title={reachable === null ? '检测中...' : reachable ? '信号源可达' : '信号源不可达'}
            />
          </div>
          <StatusBadge status={stream.status} />
        </CardHeader>
        <CardContent className="space-y-3">
          <div className="text-sm text-white/50 truncate font-mono">{stream.url}</div>
          <div className="flex items-center gap-2 flex-wrap">
            <span className="text-xs rounded-md bg-iridescent-blue/10 text-iridescent-blue border border-iridescent-blue/20 px-2 py-0.5">
              {stream.protocol.toUpperCase()}
            </span>
            {stream.group && groupPath && (
              <span className="text-xs rounded-md bg-iridescent-purple/10 text-iridescent-purple border border-iridescent-purple/20 px-2 py-0.5 max-w-[200px] truncate" title={groupPath}>
                {groupPath}
              </span>
            )}
            <span className="text-xs text-white/40">ID：{stream.id}</span>
          </div>
          <div className="flex gap-2 pt-1 flex-wrap">
            {isRecording ? (
              <Button size="sm" variant="destructive" onClick={() => onStop(stream.id)}>
                <Square className="h-3 w-3 mr-1" />
                停止
              </Button>
            ) : (
              <Button size="sm" variant="default" onClick={() => onStart(stream.id)}>
                <Play className="h-3 w-3 mr-1" />
                录制
              </Button>
            )}
            <Button size="sm" variant="outline" onClick={() => onEdit(stream)}>
              <Pencil className="h-3 w-3 mr-1" />
              编辑
            </Button>
            {previewing ? (
              <Button size="sm" variant="secondary" onClick={handleStopPreview}>
                <EyeOff className="h-3 w-3 mr-1" />
                关闭预览
              </Button>
            ) : (
              <Button size="sm" variant="secondary" onClick={handlePreview}>
                <Eye className="h-3 w-3 mr-1" />
                预览
              </Button>
            )}
            <Button size="sm" variant="ghost" onClick={() => onDelete(stream.id)}>
              <Trash2 className="h-3 w-3 mr-1" />
              删除
            </Button>
          </div>
        </CardContent>
      </Card>
      {previewing && (
        <StreamPreview
          streamId={stream.id}
          onClose={handleStopPreview}
        />
      )}
    </>
  )
}
