import { useEffect, useState } from 'react'
import { api } from '@/lib/api'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Settings2, Save } from 'lucide-react'

export default function SettingsPage() {
  const [config, setConfig] = useState<Record<string, unknown>>({})
  const [dirty, setDirty] = useState<Record<string, unknown>>({})

  useEffect(() => {
    api.config.get().then(setConfig)
  }, [])

  const handleSave = async () => {
    await api.config.update(dirty)
    setDirty({})
  }

  const makeField = (key: string, label: string, type = 'text') => (
    <div key={key} className="space-y-2">
      <Label>{label}</Label>
      <Input
        type={type}
        defaultValue={String(config[key] ?? '')}
        onChange={(e) =>
          setDirty({
            ...dirty,
            [key]: type === 'number' ? Number(e.target.value) : e.target.value,
          })
        }
      />
    </div>
  )

  return (
    <div className="max-w-xl space-y-6">
      <div className="flex items-center gap-3">
        <div className="p-2 rounded-lg bg-iridescent-purple/10 border border-iridescent-purple/20">
          <Settings2 className="h-5 w-5 text-iridescent-purple" />
        </div>
        <h2 className="text-2xl font-bold text-white/90">设置</h2>
        <div className="h-px flex-1 bg-gradient-to-r from-white/10 to-transparent" />
      </div>
      <Card>
        <CardHeader>
          <div className="flex items-center gap-2">
            <Settings2 className="h-4 w-4 text-iridescent-blue" />
            <CardTitle>全局配置</CardTitle>
          </div>
        </CardHeader>
        <CardContent className="space-y-4">
          {makeField('ffmpeg_path', 'FFmpeg 路径')}
          {makeField('max_parallel', '最大并行录制数', 'number')}
          {makeField('restart_on_failure', '失败重启次数', 'number')}
          {makeField('health_check_interval', '健康检查间隔（秒）', 'number')}
          <Button
            onClick={handleSave}
            disabled={Object.keys(dirty).length === 0}
            className="w-full"
          >
            <Save className="h-4 w-4 mr-1" />
            保存更改
          </Button>
        </CardContent>
      </Card>
    </div>
  )
}
