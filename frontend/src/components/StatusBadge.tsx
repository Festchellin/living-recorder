import { cn } from '@/lib/utils'

const statusColors: Record<string, string> = {
  idle: 'bg-gray-500',
  recording: 'bg-green-500 animate-pulse',
  error: 'bg-red-500',
  stopping: 'bg-yellow-500',
}

export function StatusBadge({ status }: { status: string }) {
  return (
    <span className="inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-xs font-medium">
      <span className={cn('w-2 h-2 rounded-full', statusColors[status] || 'bg-gray-500')} />
      {status}
    </span>
  )
}
