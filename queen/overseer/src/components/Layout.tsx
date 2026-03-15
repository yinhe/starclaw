import { NavLink, Outlet } from 'react-router-dom'
import { LayoutDashboard, Server, Activity, Zap, AlertTriangle, Eye } from 'lucide-react'

const nav = [
  { to: '/', icon: LayoutDashboard, label: 'Dashboard' },
  { to: '/nodes', icon: Server, label: 'Nodes' },
  { to: '/services', icon: Activity, label: 'Services' },
  { to: '/energy', icon: Zap, label: 'Star Energy' },
  { to: '/alerts', icon: AlertTriangle, label: 'Alerts' },
]

export default function Layout() {
  return (
    <div className="flex h-screen bg-[#0f1117]">
      {/* Sidebar */}
      <aside className="w-56 border-r border-gray-800 flex flex-col">
        <div className="p-4 flex items-center gap-2 border-b border-gray-800">
          <Eye className="w-6 h-6 text-purple-400" />
          <span className="text-lg font-bold text-white">Overseer</span>
        </div>
        <nav className="flex-1 p-2 space-y-1">
          {nav.map((item) => (
            <NavLink
              key={item.to}
              to={item.to}
              end={item.to === '/'}
              className={({ isActive }) =>
                `flex items-center gap-3 px-3 py-2 rounded-lg text-sm transition-colors ${
                  isActive
                    ? 'bg-purple-500/15 text-purple-400'
                    : 'text-gray-400 hover:text-gray-200 hover:bg-gray-800/50'
                }`
              }
            >
              <item.icon className="w-4 h-4" />
              {item.label}
            </NavLink>
          ))}
        </nav>
        <div className="p-3 border-t border-gray-800 text-xs text-gray-600">
          StarClaw Overseer v0.1
        </div>
      </aside>

      {/* Main content */}
      <main className="flex-1 overflow-auto">
        <Outlet />
      </main>
    </div>
  )
}
