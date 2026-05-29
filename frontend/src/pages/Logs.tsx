import { useEffect, useState } from 'react'
import { api, RecordLog } from '@/lib/api'
import { Card, CardContent } from '@/components/ui/card'
import { FileText, AlertCircle, Info, AlertTriangle, Clock, HardDrive } from 'lucide-react'

const eventLabels: Record<string, string> = {
  recording_started: '开始录制',
  recording_stopped: '录制结束',
  recording_failed: '录制失败',
  preview_started: '开始预览',
  preview_stopped: '停止预览',
  stream_created: '创建流媒体',
  stream_updated: '更新流媒体',
  stream_deleted: '删除流媒体',
  stream_probe: '信号源探测',
  system_startup: '系统启动',
  config_changed: '配置变更',
}

function EventBadge({ eventType }: { eventType: string }) {
  const label = eventLabels[eventType] || eventType
  return (
    <span className="text-xs rounded-md bg-iridescent-blue/10 text-iridescent-blue border border-iridescent-blue/20 px-2 py-0.5">
      {label}
    </span>
  )
}

function LevelIcon({ level }: { level: string }) {
  switch (level) {
    case 'error':
      return <AlertCircle className="h-4 w-4 text-red-400" />
    case 'warning':
      return <AlertTriangle className="h-4 w-4 text-yellow-400" />
    default:
      return <Info className="h-4 w-4 text-blue-400" />
  }
}

export default function Logs() {
  const [logs, setLogs] = useState<RecordLog[]>([])

  useEffect(() => {
    api.logs.recent().then(setLogs)
  }, [])

  const formatDuration = (sec: number) => {
    if (sec < 60) return `${sec}秒`
    const m = Math.floor(sec / 60)
    const s = sec % 60
    return s > 0 ? `${m}分${s}秒` : `${m}分`
  }

  const formatFileSize = (bytes: number) => {
    if (bytes < 1024) return `${bytes}B`
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)}KB`
    return `${(bytes / 1024 / 1024).toFixed(2)}MB`
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center gap-3">
        <div className="p-2 rounded-lg bg-iridescent-pink/10 border border-iridescent-pink/20">
          <FileText className="h-5 w-5 text-iridescent-pink" />
        </div>
        <h2 className="text-2xl font-bold text-white/90">系统日志</h2>
        <div className="h-px flex-1 bg-gradient-to-r from-white/10 to-transparent" />
        <span className="text-xs text-white/30">最近 100 条</span>
      </div>

      {logs.length === 0 ? (
        <Card>
          <CardContent className="py-12 text-center">
            <FileText className="h-10 w-10 text-white/20 mx-auto mb-3" />
            <p className="text-white/40">暂无日志</p>
          </CardContent>
        </Card>
      ) : (
        <div className="space-y-2">
          {logs.map((log) => (
            <div
              key={log.id}
              className="glass rounded-lg px-4 py-3 flex items-start gap-3 glass-hover"
            >
              <div className="mt-0.5">
                <LevelIcon level={log.level} />
              </div>
              <div className="flex-1 min-w-0">
                <div className="flex items-center gap-2 flex-wrap">
                  <span className="text-sm font-medium text-white/80">
                    {log.stream?.name || (log.stream_id ? `流媒体 #${log.stream_id}` : '系统')}
                  </span>
                  <EventBadge eventType={log.event_type} />
                  {log.status === 'failed' && (
                    <span className="inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium border border-red-400/30 bg-red-400/10 text-red-400">
                      失败
                    </span>
                  )}
                </div>
                <div className="text-sm text-white/60 mt-0.5">
                  {log.message || log.error_msg}
                </div>
                {log.error_msg && log.message !== log.error_msg && (
                  <div className="text-xs text-red-400/80 mt-1 font-mono break-all">
                    {log.error_msg}
                  </div>
                )}
                <div className="flex items-center gap-3 mt-1.5 text-xs text-white/40 flex-wrap">
                  <span className="flex items-center gap-1">
                    <Clock className="h-3 w-3" />
                    {new Date(log.started_at).toLocaleString()}
                  </span>
                  {log.duration > 0 && <span className="flex items-center gap-1">时长：{formatDuration(log.duration)}</span>}
                  {log.file_size > 0 && (
                    <span className="flex items-center gap-1">
                      <HardDrive className="h-3 w-3" />
                      {formatFileSize(log.file_size)}
                    </span>
                  )}
                </div>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
