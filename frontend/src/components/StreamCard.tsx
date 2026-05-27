import { Stream } from '@/lib/api'
import { StatusBadge } from './StatusBadge'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'

interface StreamCardProps {
  stream: Stream
  onStart: (id: number) => void
  onStop: (id: number) => void
  onEdit: (stream: Stream) => void
  onDelete: (id: number) => void
}

export function StreamCard({ stream, onStart, onStop, onEdit, onDelete }: StreamCardProps) {
  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between">
        <CardTitle className="text-base">{stream.name}</CardTitle>
        <StatusBadge status={stream.status} />
      </CardHeader>
      <CardContent className="space-y-2">
        <div className="text-sm text-muted-foreground truncate">{stream.url}</div>
        <div className="flex items-center gap-2">
          <span className="text-xs bg-muted px-2 py-0.5 rounded">{stream.protocol.toUpperCase()}</span>
          <span className="text-xs text-muted-foreground">ID: {stream.id}</span>
        </div>
        <div className="flex gap-2 pt-2">
          {stream.status === 'recording' ? (
            <Button size="sm" variant="destructive" onClick={() => onStop(stream.id)}>
              Stop
            </Button>
          ) : (
            <Button size="sm" variant="default" onClick={() => onStart(stream.id)}>
              Record
            </Button>
          )}
          <Button size="sm" variant="outline" onClick={() => onEdit(stream)}>
            Edit
          </Button>
          <Button size="sm" variant="ghost" onClick={() => onDelete(stream.id)}>
            Delete
          </Button>
        </div>
      </CardContent>
    </Card>
  )
}
