import { useEffect, useState } from 'react'
import { api, Status } from '@/lib/api'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Radio, Video } from 'lucide-react'

export default function Dashboard() {
  const [status, setStatus] = useState<Status | null>(null)

  useEffect(() => {
    api.status.get().then(setStatus)
  }, [])

  const stats = [
    {
      title: '总流媒体数',
      value: status?.total_streams ?? '-',
      icon: Radio,
      gradient: 'from-iridescent-blue/20 to-iridescent-cyan/20',
      iconColor: 'text-iridescent-blue',
    },
    {
      title: '正在录制',
      value: status?.active_recordings ?? '-',
      icon: Video,
      gradient: 'from-green-400/20 to-emerald-400/20',
      iconColor: 'text-green-400',
    },
  ]

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-3">
        <h2 className="text-2xl font-bold text-white/90">仪表盘</h2>
        <div className="h-px flex-1 bg-gradient-to-r from-white/10 to-transparent" />
      </div>
      <div className="grid grid-cols-2 gap-4">
        {stats.map((stat) => {
          const Icon = stat.icon
          return (
            <Card key={stat.title}>
              <CardHeader className="flex flex-row items-center justify-between pb-2">
                <CardTitle className="text-sm font-medium text-white/60">{stat.title}</CardTitle>
                <div className={`p-2 rounded-lg bg-gradient-to-br ${stat.gradient}`}>
                  <Icon className={`h-4 w-4 ${stat.iconColor}`} />
                </div>
              </CardHeader>
              <CardContent>
                <p className="text-3xl font-bold text-white/90">{stat.value}</p>
              </CardContent>
            </Card>
          )
        })}
      </div>
    </div>
  )
}
