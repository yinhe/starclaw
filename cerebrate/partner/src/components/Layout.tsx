import { NavLink, Outlet, useNavigate } from 'react-router-dom'
import { LayoutDashboard, Briefcase, Server, Users, Coins, TrendingUp, Rocket, LogOut, Ticket, Gem } from 'lucide-react'
import { clearToken } from '../lib/api'

const nav = [
  { to: '/dashboard', icon: LayoutDashboard, label: '个人大盘' },
  { to: '/deals', icon: Briefcase, label: '客户 CRM' },
  { to: '/nodes', icon: Server, label: '节点管理' },
  { to: '/invites', icon: Ticket, label: '邀请码管理' },
  { to: '/cities', icon: Users, label: '城市合伙人' },
  { to: '/commissions', icon: Coins, label: '佣金明细' },
  { to: '/equity', icon: TrendingUp, label: '期权归属' },
  { to: '/option', icon: Gem, label: '合伙人期权' },
  { to: '/deploy', icon: Rocket, label: '一键部署' },
]

export default function Layout() {
  const navigate = useNavigate()

  const logout = () => {
    clearToken()
    navigate('/login')
  }

  return (
    <div className="flex h-screen">
      <aside className="w-56 bg-gray-900 border-r border-white/10 flex flex-col">
        <div className="p-5 border-b border-white/10">
          <div className="text-lg font-bold tracking-tight">
            <span className="text-claw-500">Star</span>Claw
          </div>
          <div className="text-xs text-gray-500 mt-0.5">Partner Hub</div>
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
          <button
            onClick={logout}
            className="flex items-center gap-3 px-3 py-2.5 rounded-lg text-sm text-gray-400 hover:text-red-400 hover:bg-red-500/5 transition-colors w-full"
          >
            <LogOut size={18} />
            退出
          </button>
        </div>
      </aside>

      <main className="flex-1 overflow-y-auto p-8">
        <Outlet />
      </main>
    </div>
  )
}
