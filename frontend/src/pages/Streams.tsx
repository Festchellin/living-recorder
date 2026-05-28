import { useEffect, useState } from 'react'
import { api, Stream } from '@/lib/api'
import { StreamCard } from '@/components/StreamCard'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogTrigger } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'

const protocols = ['rtsp', 'rtmp', 'flv', 'hls']

export default function Streams() {
  const [streams, setStreams] = useState<Stream[]>([])
  const [editing, setEditing] = useState<Partial<Stream>>({})
  const [open, setOpen] = useState(false)

  const load = () => api.streams.list().then(setStreams)

  useEffect(() => { load() }, [])

  const handleSave = async () => {
    if (editing.id) {
      await api.streams.update(editing.id, editing)
    } else {
      await api.streams.create(editing)
    }
    setOpen(false)
    setEditing({})
    load()
  }

  const handleStartAll = async () => {
    try {
      await api.streams.startAll()
    } catch (e) {
      console.error('Start all failed', e)
    }
    load()
  }

  const handleStopAll = async () => {
    try {
      await api.streams.stopAll()
    } catch (e) {
      console.error('Stop all failed', e)
    }
    load()
  }

  return (
    <div className="space-y-4">
      <div className="flex justify-between items-center">
        <h2 className="text-2xl font-bold">Streams</h2>
        <div className="flex gap-2">
          <Button variant="default" onClick={handleStartAll}>▶ Start All</Button>
          <Button variant="destructive" onClick={handleStopAll}>■ Stop All</Button>
          <Dialog open={open} onOpenChange={setOpen}>
          <DialogTrigger asChild>
            <Button onClick={() => setEditing({ protocol: 'rtsp' })}>Add Stream</Button>
          </DialogTrigger>
          <DialogContent>
            <DialogHeader><DialogTitle>{editing.id ? 'Edit Stream' : 'Add Stream'}</DialogTitle></DialogHeader>
            <div className="space-y-4">
              <div>
                <Label>Name</Label>
                <Input value={editing.name || ''} onChange={(e) => setEditing({ ...editing, name: e.target.value })} />
              </div>
              <div>
                <Label>URL</Label>
                <Input value={editing.url || ''} onChange={(e) => setEditing({ ...editing, url: e.target.value })} />
              </div>
              <div>
                <Label>Protocol</Label>
                <Select value={editing.protocol} onValueChange={(v) => setEditing({ ...editing, protocol: v })}>
                  <SelectTrigger><SelectValue /></SelectTrigger>
                  <SelectContent>
                    {protocols.map((p) => (
                      <SelectItem key={p} value={p}>{p.toUpperCase()}</SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              <Button onClick={handleSave} className="w-full">
                {editing.id ? 'Update' : 'Create'}
              </Button>
            </div>
          </DialogContent>
        </Dialog>
        </div>
      </div>
      <div className="grid grid-cols-3 gap-4">
        {streams.map((s) => (
          <StreamCard
            key={s.id}
            stream={s}
            onStart={async (id) => { await api.streams.start(id); load() }}
            onStop={async (id) => { await api.streams.stop(id); load() }}
            onEdit={(stream) => { setEditing(stream); setOpen(true) }}
            onDelete={async (id) => { await api.streams.delete(id); load() }}
          />
        ))}
      </div>
    </div>
  )
}
