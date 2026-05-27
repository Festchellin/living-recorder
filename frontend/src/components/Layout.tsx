import { Link, useLocation } from 'react-router-dom'
import { Button } from '@/components/ui/button'

const navItems = [
  { path: '/', label: 'Dashboard', icon: '📊' },
  { path: '/streams', label: 'Streams', icon: '📡' },
  { path: '/tasks', label: 'Tasks', icon: '⏰' },
  { path: '/settings', label: 'Settings', icon: '⚙️' },
]

export function Layout({ children }: { children: React.ReactNode }) {
  const location = useLocation()
  return (
    <div className="flex h-screen">
      <nav className="w-56 border-r bg-card p-4 flex flex-col gap-2">
        <h1 className="text-lg font-bold mb-4">Living Recorder</h1>
        {navItems.map((item) => (
          <Link key={item.path} to={item.path}>
            <Button
              variant={location.pathname === item.path ? 'default' : 'ghost'}
              className="w-full justify-start"
            >
              <span className="mr-2">{item.icon}</span>
              {item.label}
            </Button>
          </Link>
        ))}
      </nav>
      <main className="flex-1 overflow-auto p-6">{children}</main>
    </div>
  )
}
