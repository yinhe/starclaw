import { useState } from 'react'
import { Routes, Route, NavLink, useLocation } from 'react-router-dom'
import { LayoutDashboard, Server, ScrollText, Search, Eye, Users, Network, Package, Bell, LogOut, Shield } from 'lucide-react'
import { getStoredToken, getStoredUser, clearAuth } from './api/brood'
import LoginPage from './pages/LoginPage'
import DashboardPage from './pages/DashboardPage'
import ClawsPage from './pages/ClawsPage'
import ClawDetailPage from './pages/ClawDetailPage'
import AuditPage from './pages/AuditPage'
import ResolvePage from './pages/ResolvePage'
import TeamsPage from './pages/TeamsPage'
import TunnelsPage from './pages/TunnelsPage'
import MoltPage from './pages/MoltPage'
import WebhooksPage from './pages/WebhooksPage'

const navItems = [
  { to: '/', icon: LayoutDashboard, label: '总览' },
  { to: '/claws', icon: Server, label: '节点管理' },
  { to: '/teams', icon: Users, label: '团队管理' },
  { to: '/tunnels', icon: Network, label: 'Nydus 隧道' },
  { to: '/molt', icon: Package, label: 'Molt 更新' },
  { to: '/webhooks', icon: Bell, label: 'Webhook' },
  { to: '/audit', icon: ScrollText, label: '审计日志' },
  { to: '/resolve', icon: Search, label: '地址解析' },
]

const roleLabels: Record<string, string> = {
  superadmin: '超级管理员',
  admin: '管理员',
  operator: '操作员',
  viewer: '只读',
}

export default function App() {
  const location = useLocation()
  const [authed, setAuthed] = useState(() => !!getStoredToken())
  const user = getStoredUser()

  const handleLogout = () => {
    clearAuth()
    setAuthed(false)
  }

  if (!authed) {
    return <LoginPage onLogin={() => setAuthed(true)} />
  }

  return (
    <div className="flex h-screen">
      {/* Sidebar */}
      <aside className="w-60 bg-gray-900 border-r border-gray-800 flex flex-col">
        <div className="px-5 py-5 flex items-center gap-3 border-b border-gray-800">
          <div className="w-9 h-9 rounded-lg bg-overlord-600/20 flex items-center justify-center">
            <Eye className="w-5 h-5 text-overlord-400" />
          </div>
          <div>
            <div className="text-sm font-bold text-white">Overlord Console</div>
            <div className="text-[10px] text-gray-500">虫群企业管理</div>
          </div>
        </div>

        <nav className="flex-1 px-3 py-4 space-y-1">
          {navItems.map(({ to, icon: Icon, label }) => {
            const active = to === '/' ? location.pathname === '/' : location.pathname.startsWith(to)
            return (
              <NavLink
                key={to}
                to={to}
                className={`flex items-center gap-3 px-3 py-2.5 rounded-lg text-sm transition-colors ${
                  active
                    ? 'bg-overlord-600/15 text-overlord-300 font-medium'
                    : 'text-gray-400 hover:bg-gray-800 hover:text-gray-200'
                }`}
              >
                <Icon className="w-4 h-4" />
                {label}
              </NavLink>
            )
          })}
        </nav>

        <div className="px-3 py-3 border-t border-gray-800">
          {user && (
            <div className="flex items-center gap-2 px-3 py-2 mb-2">
              <div className="w-7 h-7 rounded-full bg-overlord-600/20 flex items-center justify-center shrink-0">
                <Shield className="w-3.5 h-3.5 text-overlord-400" />
              </div>
              <div className="min-w-0">
                <div className="text-xs font-medium text-white truncate">{user.username}</div>
                <div className="text-[10px] text-gray-500">{roleLabels[user.role] || user.role}</div>
              </div>
            </div>
          )}
          <button
            onClick={handleLogout}
            className="flex items-center gap-2 w-full px-3 py-2 text-sm text-gray-500 hover:text-red-400 hover:bg-gray-800 rounded-lg transition"
          >
            <LogOut className="w-4 h-4" />
            退出登录
          </button>
          <div className="mt-2 px-3 py-1.5 text-center">
            <span className="text-[9px] text-gray-700">
              Powered by{' '}
              <a
                href="https://starclaw.net?ref=overlord"
                target="_blank"
                rel="noopener noreferrer"
                className="text-overlord-400/60 hover:text-overlord-300 transition"
              >
                StarClaw
              </a>
            </span>
          </div>
        </div>
      </aside>

      {/* Main content */}
      <main className="flex-1 overflow-auto">
        <Routes>
          <Route path="/" element={<DashboardPage />} />
          <Route path="/claws" element={<ClawsPage />} />
          <Route path="/claws/:id" element={<ClawDetailPage />} />
          <Route path="/teams" element={<TeamsPage />} />
          <Route path="/tunnels" element={<TunnelsPage />} />
          <Route path="/molt" element={<MoltPage />} />
          <Route path="/webhooks" element={<WebhooksPage />} />
          <Route path="/audit" element={<AuditPage />} />
          <Route path="/resolve" element={<ResolvePage />} />
        </Routes>
      </main>
    </div>
  )
}
