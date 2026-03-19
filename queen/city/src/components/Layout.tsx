import { NavLink, Outlet, useNavigate } from 'react-router-dom'
import { LayoutDashboard, Users, Coins, FileBox, LogOut, Link2, BarChart3 } from 'lucide-react'
import { clearToken } from '../lib/api'

const nav = [
  { to: '/dashboard', icon: LayoutDashboard, label: '我的大盘' },
  { to: '/clients', icon: Users, label: '我的客户' },
  { to: '/commissions', icon: Coins, label: '我的佣金' },
  { to: '/client-stats', icon: BarChart3, label: '消费统计' },
  { to: '/materials', icon: FileBox, label: '营销工具' },
]

export default function Layout() {
  const navigate = useNavigate()

  const logout = () => {
    clearToken()
    navigate('/login')
  }

  return (
    <div className="flex h-screen">
      {/* Sidebar */}
      <aside className="w-56 bg-gray-900 border-r border-white/10 flex flex-col">
        <div className="p-5 border-b border-white/10">
          <div className="text-lg font-bold tracking-tight">
            <span className="text-claw-500">Star</span>Claw
          </div>
          <div className="text-xs text-gray-500 mt-0.5">City Partner Portal</div>
        </div>

        <nav className="flex-1 p-3 space-y-1">
          {nav.map(n => (
            <NavLink
              key={n.to}
              to={n.to}
              className={({ isActive }) =>
                `flex items-center gap-3 px-3 py-2.5 rounded-lg text-sm transition-colors ${
                  isActive
                    ? 'bg-claw-500/10 text-claw-400'
                    : 'text-gray-400 hover:text-white hover:bg-white/5'
                }`
              }
            >
              <n.icon size={18} />
              {n.label}
            </NavLink>
          ))}
        </nav>

        <div className="p-3 border-t border-white/10">
          <NavLink
            to="/ref-link"
            className="flex items-center gap-3 px-3 py-2.5 rounded-lg text-sm text-gray-400 hover:text-white hover:bg-white/5 transition-colors"
          >
            <Link2 size={18} />
            推广链接
          </NavLink>
          <button
            onClick={logout}
            className="flex items-center gap-3 px-3 py-2.5 rounded-lg text-sm text-gray-400 hover:text-red-400 hover:bg-red-500/5 transition-colors w-full mt-1"
          >
            <LogOut size={18} />
            退出
          </button>
        </div>
      </aside>

      {/* Main */}
      <main className="flex-1 overflow-y-auto p-8">
        <Outlet />
      </main>
    </div>
  )
}
