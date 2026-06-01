import { useEffect, useState } from 'react'
import { api } from '@/lib/api'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Settings2, Save, HardDrive, Cloud, CheckCircle2, AlertCircle } from 'lucide-react'

interface ConfigData {
  ffmpeg_path: string
  storage_default: string
  storage_local_path: string
  storage_s3_endpoint: string
  storage_s3_bucket: string
  storage_s3_region: string
  storage_s3_configured: boolean
  max_parallel: number
  restart_on_failure: number
  health_check_interval: number
}

export default function SettingsPage() {
  const [config, setConfig] = useState<ConfigData | null>(null)
  const [dirty, setDirty] = useState<Record<string, unknown>>({})
  const [saving, setSaving] = useState(false)
  const [testResult, setTestResult] = useState<{ ok: boolean; message: string } | null>(null)

  useEffect(() => {
    api.config.get()
      .then((data) => setConfig(data as unknown as ConfigData))
      .catch((err) => console.error('Failed to load config:', err))
  }, [])

  const handleSave = async () => {
    setSaving(true)
    try {
      await api.config.update(dirty)
      setDirty({})
      const fresh = await api.config.get()
      setConfig(fresh as unknown as ConfigData)
    } finally {
      setSaving(false)
    }
  }

  const handleTestS3 = async () => {
    setTestResult(null)
    try {
      const res = await api.config.testS3({
        endpoint: dirty.storage_s3_endpoint ?? config?.storage_s3_endpoint ?? '',
        access_key: dirty.storage_s3_access_key ?? '',
        secret_key: dirty.storage_s3_secret_key ?? '',
        bucket: dirty.storage_s3_bucket ?? config?.storage_s3_bucket ?? '',
        region: dirty.storage_s3_region ?? config?.storage_s3_region ?? '',
      })
      setTestResult({ ok: true, message: res.message })
    } catch (e) {
      setTestResult({ ok: false, message: String(e) })
    }
  }

  const setField = (key: string, value: unknown) => {
    setDirty({ ...dirty, [key]: value })
  }

  const val = (key: string): string => {
    if (key in dirty) return String(dirty[key])
    return String((config as unknown as Record<string, unknown>)?.[key] ?? '')
  }

  const storageType = val('storage_default')

  if (!config) return null

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
            <HardDrive className="h-4 w-4 text-iridescent-blue" />
            <CardTitle>存储设置</CardTitle>
          </div>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="flex gap-4">
            <label
              className={`flex items-center gap-3 flex-1 p-3 rounded-lg border cursor-pointer transition-colors ${
                storageType === 'local'
                  ? 'border-iridescent-blue bg-iridescent-blue/10'
                  : 'border-white/10 bg-white/5 hover:bg-white/10'
              }`}
            >
              <input
                type="radio"
                name="storage_type"
                value="local"
                checked={storageType === 'local'}
                onChange={() => setField('storage_default', 'local')}
                className="sr-only"
              />
              <HardDrive className="h-5 w-5 text-white/70" />
              <span className="text-sm text-white/80">本地存储</span>
            </label>
            <label
              className={`flex items-center gap-3 flex-1 p-3 rounded-lg border cursor-pointer transition-colors ${
                storageType === 's3'
                  ? 'border-iridescent-blue bg-iridescent-blue/10'
                  : 'border-white/10 bg-white/5 hover:bg-white/10'
              }`}
            >
              <input
                type="radio"
                name="storage_type"
                value="s3"
                checked={storageType === 's3'}
                onChange={() => setField('storage_default', 's3')}
                className="sr-only"
              />
              <Cloud className="h-5 w-5 text-white/70" />
              <span className="text-sm text-white/80">S3 对象存储</span>
            </label>
          </div>

          {storageType === 's3' && (
            <div className="space-y-4 p-4 rounded-lg bg-white/5 border border-white/10">
              <div className="space-y-2">
                <Label>Endpoint</Label>
                <Input
                  value={val('storage_s3_endpoint')}
                  onChange={(e) => setField('storage_s3_endpoint', e.target.value)}
                  placeholder="s3.amazonaws.com"
                />
              </div>
              <div className="space-y-2">
                <Label>Access Key</Label>
                <Input
                  value={val('storage_s3_access_key')}
                  onChange={(e) => setField('storage_s3_access_key', e.target.value)}
                  placeholder="AKIAIOSFODNN7EXAMPLE"
                />
              </div>
              <div className="space-y-2">
                <Label>Secret Key</Label>
                <Input
                  type="password"
                  value={val('storage_s3_secret_key')}
                  onChange={(e) => setField('storage_s3_secret_key', e.target.value)}
                  placeholder="wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
                />
              </div>
              <div className="space-y-2">
                <Label>Bucket</Label>
                <Input
                  value={val('storage_s3_bucket')}
                  onChange={(e) => setField('storage_s3_bucket', e.target.value)}
                  placeholder="living-recorder"
                />
              </div>
              <div className="space-y-2">
                <Label>Region</Label>
                <Input
                  value={val('storage_s3_region')}
                  onChange={(e) => setField('storage_s3_region', e.target.value)}
                  placeholder="us-east-1"
                />
              </div>

              <Button
                variant="outline"
                onClick={handleTestS3}
                className="w-full"
              >
                <Cloud className="h-4 w-4 mr-1" />
                测试连接
              </Button>

              {testResult && (
                <div
                  className={`flex items-center gap-2 p-3 rounded-lg text-sm ${
                    testResult.ok
                      ? 'bg-green-500/10 text-green-400 border border-green-500/20'
                      : 'bg-red-500/10 text-red-400 border border-red-500/20'
                  }`}
                >
                  {testResult.ok ? (
                    <CheckCircle2 className="h-4 w-4 flex-shrink-0" />
                  ) : (
                    <AlertCircle className="h-4 w-4 flex-shrink-0" />
                  )}
                  {testResult.message}
                </div>
              )}
            </div>
          )}
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <div className="flex items-center gap-2">
            <Settings2 className="h-4 w-4 text-iridescent-blue" />
            <CardTitle>全局配置</CardTitle>
          </div>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="space-y-2">
            <Label>FFmpeg 路径</Label>
            <Input
              defaultValue={String(config.ffmpeg_path ?? '')}
              onChange={(e) => setField('ffmpeg_path', e.target.value)}
            />
          </div>
          <div className="space-y-2">
            <Label>最大并行录制数</Label>
            <Input
              type="number"
              defaultValue={String(config.max_parallel ?? '')}
              onChange={(e) => setField('max_parallel', Number(e.target.value))}
            />
          </div>
          <div className="space-y-2">
            <Label>失败重启次数</Label>
            <Input
              type="number"
              defaultValue={String(config.restart_on_failure ?? '')}
              onChange={(e) => setField('restart_on_failure', Number(e.target.value))}
            />
          </div>
          <div className="space-y-2">
            <Label>健康检查间隔（秒）</Label>
            <Input
              type="number"
              defaultValue={String(config.health_check_interval ?? '')}
              onChange={(e) => setField('health_check_interval', Number(e.target.value))}
            />
          </div>
          <Button
            onClick={handleSave}
            disabled={Object.keys(dirty).length === 0 || saving}
            className="w-full"
          >
            <Save className="h-4 w-4 mr-1" />
            {saving ? '保存中...' : '保存更改'}
          </Button>
        </CardContent>
      </Card>
    </div>
  )
}
