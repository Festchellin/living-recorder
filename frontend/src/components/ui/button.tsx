import * as React from 'react'
import { cva, type VariantProps } from 'class-variance-authority'
import { cn } from '@/lib/utils'

const buttonVariants = cva(
  'inline-flex items-center justify-center whitespace-nowrap rounded-lg text-sm font-medium transition-all duration-300 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background disabled:pointer-events-none disabled:opacity-50 cursor-pointer',
  {
    variants: {
      variant: {
        default:
          'bg-gradient-to-r from-iridescent-blue to-iridescent-purple text-white shadow-lg shadow-iridescent-blue/20 hover:shadow-iridescent-blue/30 hover:scale-[1.02] active:scale-[0.98]',
        destructive:
          'bg-destructive/90 text-destructive-foreground shadow-lg hover:bg-destructive active:scale-[0.98]',
        outline:
          'glass glass-hover text-white/80 hover:text-white',
        secondary:
          'bg-white/[0.06] text-white/70 shadow-sm hover:bg-white/[0.1] hover:text-white/90',
        ghost:
          'text-white/60 hover:bg-white/[0.06] hover:text-white/80',
        link:
          'text-iridescent-blue underline-offset-4 hover:underline',
      },
      size: {
        default: 'h-9 px-4 py-2',
        sm: 'h-8 rounded-md px-3 text-xs',
        lg: 'h-10 rounded-md px-8',
        icon: 'h-9 w-9',
      },
    },
    defaultVariants: { variant: 'default', size: 'default' },
  },
)

export interface ButtonProps
  extends React.ButtonHTMLAttributes<HTMLButtonElement>,
    VariantProps<typeof buttonVariants> {}

const Button = React.forwardRef<HTMLButtonElement, ButtonProps>(
  ({ className, variant, size, ...props }, ref) => (
    <button className={cn(buttonVariants({ variant, size, className }))} ref={ref} {...props} />
  ),
)
Button.displayName = 'Button'
export { Button, buttonVariants }
