import { useEffect, useState } from 'react'
import { api } from '@/lib/api'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'

export default function Settings() {
  const [config, setConfig] = useState<Record<string, unknown>>({})
  const [dirty, setDirty] = useState<Record<string, unknown>>({})

  useEffect(() => { api.config.get().then(setConfig) }, [])

  const handleSave = async () => {
    await api.config.update(dirty)
    setDirty({})
  }

  const makeField = (key: string, label: string, type = 'text') => (
    <div key={key}>
      <Label>{label}</Label>
      <Input
        type={type}
        defaultValue={String(config[key] ?? '')}
        onChange={(e) => setDirty({ ...dirty, [key]: type === 'number' ? Number(e.target.value) : e.target.value })}
      />
    </div>
  )

  return (
    <div className="max-w-xl space-y-6">
      <h2 className="text-2xl font-bold">Settings</h2>
      <Card>
        <CardHeader><CardTitle>Global Config</CardTitle></CardHeader>
        <CardContent className="space-y-4">
          {makeField('ffmpeg_path', 'FFmpeg Path')}
          {makeField('max_parallel', 'Max Parallel Recordings', 'number')}
          {makeField('restart_on_failure', 'Restart on Failure (count)', 'number')}
          {makeField('health_check_interval', 'Health Check Interval (sec)', 'number')}
          <Button onClick={handleSave} disabled={Object.keys(dirty).length === 0}>Save</Button>
        </CardContent>
      </Card>
    </div>
  )
}
