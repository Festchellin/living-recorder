import { useEffect, useState } from 'react'
import { api, Status, RecordLog } from '@/lib/api'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'

export default function Dashboard() {
  const [status, setStatus] = useState<Status | null>(null)
  const [recentLogs, setRecentLogs] = useState<RecordLog[]>([])

  useEffect(() => {
    api.status.get().then(setStatus)
    api.streams.list().then((streams) => {
      if (streams.length > 0) {
        api.streams.logs(streams[0].id).then(setRecentLogs)
      }
    })
  }, [])

  return (
    <div className="space-y-6">
      <h2 className="text-2xl font-bold">Dashboard</h2>
      <div className="grid grid-cols-4 gap-4">
        <Card>
          <CardHeader><CardTitle className="text-sm">Total Streams</CardTitle></CardHeader>
          <CardContent><p className="text-2xl font-bold">{status?.total_streams ?? '-'}</p></CardContent>
        </Card>
        <Card>
          <CardHeader><CardTitle className="text-sm">Active Recordings</CardTitle></CardHeader>
          <CardContent><p className="text-2xl font-bold text-green-600">{status?.active_recordings ?? '-'}</p></CardContent>
        </Card>
        <Card>
          <CardHeader><CardTitle className="text-sm">Total Recordings</CardTitle></CardHeader>
          <CardContent><p className="text-2xl font-bold">{status?.total_recordings ?? '-'}</p></CardContent>
        </Card>
        <Card>
          <CardHeader><CardTitle className="text-sm">Storage Used</CardTitle></CardHeader>
          <CardContent>
            <p className="text-2xl font-bold">
              {status ? `${(status.storage_used / 1024 / 1024 / 1024).toFixed(2)} GB` : '-'}
            </p>
          </CardContent>
        </Card>
      </div>
      <div>
        <h3 className="text-lg font-semibold mb-2">Recent Recordings</h3>
        {recentLogs.length === 0 ? (
          <p className="text-muted-foreground">No recordings yet</p>
        ) : (
          <div className="space-y-2">
            {recentLogs.slice(0, 10).map((log) => (
              <div key={log.id} className="flex justify-between items-center p-2 border rounded">
                <span>{log.file_path}</span>
                <span className="text-sm text-muted-foreground">{log.duration}s</span>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  )
}
