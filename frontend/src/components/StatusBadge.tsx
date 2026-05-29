import { cn } from '@/lib/utils'

const statusStyles: Record<string, string> = {
  idle: 'border-gray-500/30 bg-gray-500/10 text-gray-400',
  recording: 'border-green-400/30 bg-green-400/10 text-green-400',
  error: 'border-red-400/30 bg-red-400/10 text-red-400',
  stopping: 'border-yellow-400/30 bg-yellow-400/10 text-yellow-400',
}

const statusLabels: Record<string, string> = {
  idle: '空闲',
  recording: '录制中',
  error: '错误',
  stopping: '停止中',
}

export function StatusBadge({ status }: { status: string }) {
  return (
    <span
      className={cn(
        'inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-xs font-medium border',
        statusStyles[status] || 'border-white/10 bg-white/[0.06] text-white/60',
      )}
    >
      <span className={cn('status-dot', status)} />
      {statusLabels[status] || status}
    </span>
  )
}
