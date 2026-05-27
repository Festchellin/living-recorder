import { useEffect, useState } from 'react'
import { useParams } from 'react-router-dom'
import { api, Stream, RecordLog } from '@/lib/api'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { StatusBadge } from '@/components/StatusBadge'

export default function StreamDetail() {
  const { id } = useParams<{ id: string }>()
  const [stream, setStream] = useState<Stream | null>(null)
  const [logs, setLogs] = useState<RecordLog[]>([])

  useEffect(() => {
    if (!id) return
    api.streams.get(Number(id)).then(setStream)
    api.streams.logs(Number(id)).then(setLogs)
  }, [id])

  if (!stream) return <p>Loading...</p>

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-4">
        <h2 className="text-2xl font-bold">{stream.name}</h2>
        <StatusBadge status={stream.status} />
      </div>
      <Card>
        <CardHeader><CardTitle>Info</CardTitle></CardHeader>
        <CardContent className="space-y-2">
          <div><span className="text-muted-foreground">URL:</span> {stream.url}</div>
          <div><span className="text-muted-foreground">Protocol:</span> {stream.protocol.toUpperCase()}</div>
          <div><span className="text-muted-foreground">Created:</span> {stream.created_at}</div>
        </CardContent>
      </Card>
      <div>
        <h3 className="text-lg font-semibold mb-2">Recording Logs</h3>
        <div className="space-y-2">
          {logs.map((log) => (
            <div key={log.id} className="flex justify-between items-center p-3 border rounded">
              <div>
                <div className="text-sm">{log.file_path}</div>
                <div className="text-xs text-muted-foreground">
                  {new Date(log.started_at).toLocaleString()} - {log.duration}s
                </div>
              </div>
              <div className="text-right">
                <div className="text-sm">{(log.file_size / 1024 / 1024).toFixed(2)} MB</div>
                <StatusBadge status={log.status} />
              </div>
            </div>
          ))}
        </div>
      </div>
    </div>
  )
}
