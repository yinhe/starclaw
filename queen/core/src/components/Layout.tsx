import { Outlet, NavLink, useNavigate } from 'react-router-dom'
import { clearToken } from '../api'
import {
  LayoutDashboard,
  Server,
  CreditCard,
  Receipt,
  Wallet,
  Package,
  Users,
  Flag,
  ShoppingBag,
  Layers,
  LogOut,
  Handshake,
  Building2,
  Calculator,
  Activity,
  ExternalLink,
} from 'lucide-react'

const navItems = [
  { to: '/', icon: LayoutDashboard, label: '仪表盘', end: true },
  { to: '/nodes', icon: Server, label: '节点管理' },
  { to: '/users', icon: Users, label: '用户管理' },
  { to: '/reports', icon: Flag, label: '内容审核' },
  { to: '/reviews', icon: ShoppingBag, label: '开发者审核' },
  { to: '/services', icon: Layers, label: '服务概览' },
  { to: '/billing', icon: CreditCard, label: '收入统计' },
  { to: '/orders', icon: Receipt, label: '订单管理' },
  { to: '/balances', icon: Wallet, label: '用户余额' },
  { to: '/packages', icon: Package, label: '套餐管理' },
  { to: '/partners', icon: Handshake, label: '合伙人管理' },
  { to: '/clients', icon: Building2, label: '客户总览' },
  { to: '/settlement', icon: Calculator, label: '结算管理' },
]

export default function Layout() {
  const navigate = useNavigate()

  const handleLogout = () => {
    clearToken()
    navigate('/login')
  }

  return (
    <div className="flex h-screen">
      {/* Sidebar */}
      <aside className="w-60 bg-gray-900 border-r border-gray-800 flex flex-col">
        <div className="p-5 border-b border-gray-800">
          <h1 className="text-lg font-bold text-purple-400">StarClaw</h1>
          <p className="text-xs text-gray-500 mt-1">运营中心</p>
        </div>

        <nav className="flex-1 py-4 space-y-1 px-3">
          {navItems.map((item) => (
            <NavLink
              key={item.to}
              to={item.to}
              end={item.end}
              className={({ isActive }) =>
                `flex items-center gap-3 px-3 py-2.5 rounded-lg text-sm transition-colors ${
                  isActive
                    ? 'bg-purple-600/20 text-purple-400'
                    : 'text-gray-400 hover:bg-gray-800 hover:text-gray-200'
                }`
              }
            >
              <item.icon size={18} />
              {item.label}
            </NavLink>
          ))}
          <a
            href="/overseer"
            target="_blank"
            rel="noopener noreferrer"
            className="flex items-center gap-3 px-3 py-2.5 rounded-lg text-sm text-gray-400 hover:bg-gray-800 hover:text-gray-200 transition-colors"
          >
            <Activity size={18} />
            Overseer 监控
            <ExternalLink size={12} className="ml-auto opacity-40" />
          </a>
        </nav>

        <div className="p-3 border-t border-gray-800">
          <button
            onClick={handleLogout}
            className="flex items-center gap-3 px-3 py-2.5 rounded-lg text-sm text-gray-400 hover:bg-gray-800 hover:text-gray-200 w-full transition-colors"
          >
            <LogOut size={18} />
            退出登录
          </button>
        </div>
      </aside>

      {/* Main content */}
      <main className="flex-1 overflow-auto bg-gray-950">
        <div className="p-8">
          <Outlet />
        </div>
      </main>
    </div>
  )
}
