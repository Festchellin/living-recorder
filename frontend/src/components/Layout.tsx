import { Link, useLocation } from 'react-router-dom'
import { Button } from '@/components/ui/button'
import { LayoutDashboard, Radio, Layers, FileText, Monitor, Settings } from 'lucide-react'

const navItems = [
  { path: '/', label: '仪表盘', icon: LayoutDashboard },
  { path: '/streams', label: '流媒体', icon: Radio },
  { path: '/preview', label: '预览', icon: Monitor },
  { path: '/groups', label: '分组', icon: Layers },
  { path: '/logs', label: '日志', icon: FileText },
  { path: '/settings', label: '设置', icon: Settings },
]

export function Layout({ children }: { children: React.ReactNode }) {
  const location = useLocation()
  return (
    <div className="flex h-screen relative">
      <div className="animated-bg" aria-hidden="true">
        <div className="orb" />
        <div className="orb" />
        <div className="orb" />
      </div>
      <nav className="w-56 glass m-3 rounded-xl flex flex-col gap-1 p-3 border-white/[0.06] z-10 h-[calc(100vh-1.5rem)]">
        <div className="px-3 py-4 mb-2">
          <h1 className="text-lg font-bold text-gradient">Living Recorder</h1>
          <p className="text-xs text-white/40 mt-0.5">流媒体管理</p>
        </div>
        {navItems.map((item) => {
          const Icon = item.icon
          const isActive = location.pathname === item.path
          return (
            <Link key={item.path} to={item.path}>
              <Button
                variant={isActive ? 'default' : 'ghost'}
                className="w-full justify-start gap-3"
              >
                <Icon className="h-4 w-4" />
                {item.label}
              </Button>
            </Link>
          )
        })}
      </nav>
      <main className="flex-1 overflow-auto p-6 relative z-10">{children}</main>
    </div>
  )
}
