import { useEffect, useState } from 'react'
import { api, RecordTask } from '@/lib/api'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogTrigger } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'

export default function Tasks() {
  const [tasks, setTasks] = useState<RecordTask[]>([])
  const [editing, setEditing] = useState<Partial<RecordTask>>({})
  const [open, setOpen] = useState(false)
  const [streams, setStreams] = useState<{ id: number; name: string }[]>([])

  const load = () => {
    api.tasks.list().then(setTasks)
    api.streams.list().then((ss) => setStreams(ss.map((s) => ({ id: s.id, name: s.name }))))
  }

  useEffect(() => { load() }, [])

  const handleSave = async () => {
    if (editing.id) {
      await api.tasks.update(editing.id, editing)
    } else {
      await api.tasks.create(editing)
    }
    setOpen(false)
    setEditing({})
    load()
  }

  return (
    <div className="space-y-4">
      <div className="flex justify-between items-center">
        <h2 className="text-2xl font-bold">Record Tasks</h2>
        <Dialog open={open} onOpenChange={setOpen}>
          <DialogTrigger asChild>
            <Button onClick={() => setEditing({ video_codec: 'copy', audio_codec: 'copy', storage_type: 'local' })}>
              Add Task
            </Button>
          </DialogTrigger>
          <DialogContent className="max-w-lg">
            <DialogHeader><DialogTitle>{editing.id ? 'Edit Task' : 'Add Task'}</DialogTitle></DialogHeader>
            <div className="grid grid-cols-2 gap-4">
              <div className="col-span-2">
                <Label>Stream</Label>
                <Select value={String(editing.stream_id || '')} onValueChange={(v) => setEditing({ ...editing, stream_id: Number(v) })}>
                  <SelectTrigger><SelectValue placeholder="Select stream" /></SelectTrigger>
                  <SelectContent>
                    {streams.map((s) => (
                      <SelectItem key={s.id} value={String(s.id)}>{s.name}</SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              <div>
                <Label>Output Template</Label>
                <Input value={editing.output_template || '{name}/{date}_{time}.mp4'} onChange={(e) => setEditing({ ...editing, output_template: e.target.value })} />
              </div>
              <div>
                <Label>Segment (sec, 0=off)</Label>
                <Input type="number" value={editing.segment_sec ?? 0} onChange={(e) => setEditing({ ...editing, segment_sec: Number(e.target.value) })} />
              </div>
              <div>
                <Label>Video Codec</Label>
                <Select value={editing.video_codec || 'copy'} onValueChange={(v) => setEditing({ ...editing, video_codec: v })}>
                  <SelectTrigger><SelectValue /></SelectTrigger>
                  <SelectContent>
                    <SelectItem value="copy">Copy</SelectItem>
                    <SelectItem value="libx264">H.264</SelectItem>
                    <SelectItem value="h264_nvenc">H.264 (NVENC)</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <div>
                <Label>Video Bitrate</Label>
                <Input value={editing.video_bitrate || ''} onChange={(e) => setEditing({ ...editing, video_bitrate: e.target.value })} placeholder="2000k" />
              </div>
              <div>
                <Label>Framerate (0=auto)</Label>
                <Input type="number" value={editing.framerate ?? 0} onChange={(e) => setEditing({ ...editing, framerate: Number(e.target.value) })} />
              </div>
              <div>
                <Label>Resolution</Label>
                <Input value={editing.resolution || ''} onChange={(e) => setEditing({ ...editing, resolution: e.target.value })} placeholder="1920x1080" />
              </div>
              <div>
                <Label>Audio Codec</Label>
                <Select value={editing.audio_codec || 'copy'} onValueChange={(v) => setEditing({ ...editing, audio_codec: v })}>
                  <SelectTrigger><SelectValue /></SelectTrigger>
                  <SelectContent>
                    <SelectItem value="copy">Copy</SelectItem>
                    <SelectItem value="aac">AAC</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <div>
                <Label>Audio Bitrate</Label>
                <Input value={editing.audio_bitrate || ''} onChange={(e) => setEditing({ ...editing, audio_bitrate: e.target.value })} placeholder="128k" />
              </div>
              <div>
                <Label>Schedule Cron</Label>
                <Input value={editing.schedule_cron || ''} onChange={(e) => setEditing({ ...editing, schedule_cron: e.target.value })} placeholder="0 0 * * * *" />
              </div>
              <div>
                <Label>Duration (sec, 0=unlimited)</Label>
                <Input type="number" value={editing.schedule_duration ?? 0} onChange={(e) => setEditing({ ...editing, schedule_duration: Number(e.target.value) })} />
              </div>
              <div className="col-span-2">
                <Label>Storage Type</Label>
                <Select value={editing.storage_type || 'local'} onValueChange={(v) => setEditing({ ...editing, storage_type: v })}>
                  <SelectTrigger><SelectValue /></SelectTrigger>
                  <SelectContent>
                    <SelectItem value="local">Local</SelectItem>
                    <SelectItem value="s3">S3 / MinIO</SelectItem>
                  </SelectContent>
                </Select>
              </div>
            </div>
            <Button onClick={handleSave} className="w-full">{editing.id ? 'Update' : 'Create'}</Button>
          </DialogContent>
        </Dialog>
      </div>
      <div className="space-y-3">
        {tasks.map((task) => (
          <Card key={task.id}>
            <CardHeader><CardTitle className="text-sm">
              {task.stream?.name || `Stream #${task.stream_id}`}
            </CardTitle></CardHeader>
            <CardContent className="text-sm text-muted-foreground space-y-1">
              <div>Codec: {task.video_codec} / {task.audio_codec}</div>
              <div>Segment: {task.segment_sec > 0 ? `${task.segment_sec}s` : 'off'}</div>
              <div>Cron: {task.schedule_cron || 'manual only'}</div>
              <div className="flex gap-2 pt-1">
                <Button size="sm" variant="outline" onClick={() => { setEditing(task); setOpen(true) }}>Edit</Button>
                <Button size="sm" variant="ghost" onClick={async () => { await api.tasks.delete(task.id); load() }}>Delete</Button>
              </div>
            </CardContent>
          </Card>
        ))}
      </div>
    </div>
  )
}
