import { useEffect, useState } from 'react'
import { useParams } from 'react-router-dom'
import { api, Group, Stream, RecordLog, getGroupPath } from '@/lib/api'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { StatusBadge } from '@/components/StatusBadge'
import { StreamPreview } from '@/components/StreamPreview'
import { Video, Info, FileText, Eye, EyeOff } from 'lucide-react'

export default function StreamDetail() {
  const { id } = useParams<{ id: string }>()
  const [stream, setStream] = useState<Stream | null>(null)
  const [logs, setLogs] = useState<RecordLog[]>([])
  const [allGroups, setAllGroups] = useState<Group[]>([])
  const [previewing, setPreviewing] = useState(false)

  useEffect(() => {
    if (!id) return
    api.streams.get(Number(id)).then(setStream)
    api.streams.logs(Number(id)).then(setLogs)
    api.groups.list().then(setAllGroups)
  }, [id])

  const handlePreview = () => setPreviewing(true)

  const handleStopPreview = () => setPreviewing(false)

  if (!stream) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="glass rounded-xl p-8 text-center">
          <div className="animate-pulse text-white/40">加载中...</div>
        </div>
      </div>
    )
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-3">
        <div className="p-2 rounded-lg bg-iridescent-blue/10 border border-iridescent-blue/20">
          <Video className="h-5 w-5 text-iridescent-blue" />
        </div>
        <h2 className="text-2xl font-bold text-white/90">{stream.name}</h2>
        <StatusBadge status={stream.status} />
        <div className="h-px flex-1 bg-gradient-to-r from-white/10 to-transparent" />
        <div className="flex gap-2">
          {previewing ? (
            <Button variant="secondary" size="sm" onClick={handleStopPreview}>
              <EyeOff className="h-4 w-4 mr-1" />
              关闭预览
            </Button>
          ) : (
            <Button variant="default" size="sm" onClick={handlePreview}>
              <Eye className="h-4 w-4 mr-1" />
              预览
            </Button>
          )}
        </div>
      </div>
      {previewing && (
        <StreamPreview
          streamId={Number(id)}
          onClose={handleStopPreview}
        />
      )}
      <Card>
        <CardHeader>
          <div className="flex items-center gap-2">
            <Info className="h-4 w-4 text-iridescent-blue" />
            <CardTitle>流媒体信息</CardTitle>
          </div>
        </CardHeader>
        <CardContent>
          <div className="grid grid-cols-4 gap-6">
            <div>
              <div className="text-xs text-white/40 mb-1">URL</div>
              <div className="text-sm text-white/70 font-mono break-all">{stream.url}</div>
            </div>
            <div>
              <div className="text-xs text-white/40 mb-1">协议</div>
              <div className="text-sm">
                <span className="rounded-md bg-iridescent-blue/10 text-iridescent-blue border border-iridescent-blue/20 px-2 py-0.5 text-xs">
                  {stream.protocol.toUpperCase()}
                </span>
              </div>
            </div>
            <div>
              <div className="text-xs text-white/40 mb-1">分组路径</div>
              <div className="text-sm text-white/70">
                {stream.group ? (
                  <span className="rounded-md bg-iridescent-purple/10 text-iridescent-purple border border-iridescent-purple/20 px-2 py-0.5 text-xs">
                    {getGroupPath(allGroups, stream.group_id)}
                  </span>
                ) : (
                  <span className="text-white/40">未分组</span>
                )}
              </div>
            </div>
            <div>
              <div className="text-xs text-white/40 mb-1">创建时间</div>
              <div className="text-sm text-white/70">{stream.created_at}</div>
            </div>
          </div>
        </CardContent>
      </Card>
      <div>
        <div className="flex items-center gap-2 mb-3">
          <FileText className="h-4 w-4 text-iridescent-purple" />
          <h3 className="text-lg font-semibold text-white/80">录制日志</h3>
        </div>
        <div className="space-y-2">
          {logs.length === 0 ? (
            <div className="glass rounded-xl p-8 text-center">
              <FileText className="h-8 w-8 text-white/20 mx-auto mb-2" />
              <p className="text-white/40">暂无录制日志</p>
            </div>
          ) : (
            logs.map((log) => (
              <div
                key={log.id}
                className="glass rounded-lg px-4 py-3 flex justify-between items-center glass-hover"
              >
                <div className="flex-1 min-w-0">
                  <div className="text-sm text-white/70 font-mono truncate">{log.file_path}</div>
                  <div className="text-xs text-white/40 mt-0.5">
                    {new Date(log.started_at).toLocaleString()} - {log.duration}秒
                  </div>
                </div>
                <div className="text-right ml-4 flex items-center gap-3">
                  <div className="text-sm text-white/50">
                    {(log.file_size / 1024 / 1024).toFixed(2)} MB
                  </div>
                  <StatusBadge status={log.status} />
                </div>
              </div>
            ))
          )}
        </div>
      </div>
    </div>
  )
}
