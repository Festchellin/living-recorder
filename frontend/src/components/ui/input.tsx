import * as React from 'react'
import { cn } from '@/lib/utils'

export interface InputProps extends React.InputHTMLAttributes<HTMLInputElement> {}

const Input = React.forwardRef<HTMLInputElement, InputProps>(({ className, type, ...props }, ref) => {
  return (
    <input
      type={type}
      className={cn(
        'flex h-9 w-full rounded-lg bg-white/[0.06] border border-white/10 px-3 py-1 text-sm text-white/90 shadow-sm transition-all duration-300',
        'file:border-0 file:bg-transparent file:text-sm file:font-medium file:text-white/70',
        'placeholder:text-white/30',
        'focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:border-iridescent-blue/50',
        'hover:border-white/[0.18]',
        'disabled:cursor-not-allowed disabled:opacity-50',
        className,
      )}
      ref={ref}
      {...props}
    />
  )
})
Input.displayName = 'Input'
export { Input }
