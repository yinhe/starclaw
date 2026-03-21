import { useState } from 'react'
import { Routes, Route, NavLink, useLocation } from 'react-router-dom'
import { LayoutDashboard, Server, ScrollText, Search, Eye, Users, Network, Package, Bell, LogOut, Shield, CreditCard, BarChart3, Paintbrush, ShieldCheck, Bot, ChevronDown, ChevronRight } from 'lucide-react'
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
import BillingPage from './pages/BillingPage'
import AnalyticsPage from './pages/AnalyticsPage'
import BrandPage from './pages/BrandPage'
import CompliancePage from './pages/CompliancePage'
import TeamAgentPage from './pages/TeamAgentPage'

const primaryNav = [
  { to: '/', icon: LayoutDashboard, label: '总览' },
  { to: '/claws', icon: Server, label: '节点管理' },
  { to: '/team-agent', icon: Bot, label: 'AI 团队' },
  { to: '/teams', icon: Users, label: '团队管理' },
  { to: '/billing', icon: CreditCard, label: '计费管理' },
  { to: '/analytics', icon: BarChart3, label: '用量分析' },
  { to: '/audit', icon: ScrollText, label: '审计日志' },
]

const moreNav = [
  { to: '/tunnels', icon: Network, label: 'Nydus 隧道' },
  { to: '/molt', icon: Package, label: 'Molt 更新' },
  { to: '/webhooks', icon: Bell, label: 'Webhook' },
  { to: '/brand', icon: Paintbrush, label: '白牌 & 许可证' },
  { to: '/compliance', icon: ShieldCheck, label: '合规中心' },
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
  const [moreOpen, setMoreOpen] = useState(() => {
    // Auto-expand if current route is in the "more" group
    return moreNav.some(n => location.pathname.startsWith(n.to))
  })
  const user = getStoredUser()

  const handleLogout = () => {
    clearAuth()
    setAuthed(false)
  }

  if (!authed) {
    return <LoginPage onLogin={() => setAuthed(true)} />
  }

  const renderNavLink = ({ to, icon: Icon, label }: { to: string; icon: React.ComponentType<{ className?: string }>; label: string }) => {
    const active = to === '/' ? location.pathname === '/' : location.pathname.startsWith(to)
    return (
      <NavLink
        key={to}
        to={to}
        className={`flex items-center gap-3 px-3 py-2 rounded-lg text-sm transition-colors ${
          active
            ? 'bg-overlord-600/15 text-overlord-300 font-medium'
            : 'text-gray-400 hover:bg-gray-800 hover:text-gray-200'
        }`}
      >
        <Icon className="w-4 h-4" />
        {label}
      </NavLink>
    )
  }

  return (
    <div className="flex h-screen">
      {/* Sidebar */}
      <aside className="w-56 bg-gray-900 border-r border-gray-800 flex flex-col">
        <div className="px-4 py-4 flex items-center gap-3 border-b border-gray-800">
          <div className="w-8 h-8 rounded-lg bg-overlord-600/20 flex items-center justify-center">
            <Eye className="w-4.5 h-4.5 text-overlord-400" />
          </div>
          <div>
            <div className="text-sm font-bold text-white">Overlord</div>
            <div className="text-[10px] text-gray-500">企业管控台</div>
          </div>
        </div>

        <nav className="flex-1 px-3 py-3 space-y-0.5 overflow-auto">
          {primaryNav.map(renderNavLink)}

          {/* Collapsible "More" section */}
          <div className="pt-2">
            <button
              onClick={() => setMoreOpen(!moreOpen)}
              className="flex items-center gap-3 px-3 py-2 w-full rounded-lg text-sm text-gray-500 hover:bg-gray-800 hover:text-gray-300 transition-colors"
            >
              {moreOpen ? <ChevronDown className="w-4 h-4" /> : <ChevronRight className="w-4 h-4" />}
              更多功能
            </button>
            {moreOpen && (
              <div className="mt-0.5 space-y-0.5 pl-1">
                {moreNav.map(renderNavLink)}
              </div>
            )}
          </div>
        </nav>

        <div className="px-3 py-3 border-t border-gray-800">
          {user && (
            <div className="flex items-center gap-2 px-3 py-2 mb-1">
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
        </div>
      </aside>

      {/* Main content */}
      <main className="flex-1 overflow-auto">
        <Routes>
          <Route path="/" element={<DashboardPage />} />
          <Route path="/claws" element={<ClawsPage />} />
          <Route path="/claws/:id" element={<ClawDetailPage />} />
          <Route path="/teams" element={<TeamsPage />} />
          <Route path="/team-agent" element={<TeamAgentPage />} />
          <Route path="/tunnels" element={<TunnelsPage />} />
          <Route path="/molt" element={<MoltPage />} />
          <Route path="/webhooks" element={<WebhooksPage />} />
          <Route path="/billing" element={<BillingPage />} />
          <Route path="/analytics" element={<AnalyticsPage />} />
          <Route path="/audit" element={<AuditPage />} />
          <Route path="/brand" element={<BrandPage />} />
          <Route path="/compliance" element={<CompliancePage />} />
          <Route path="/resolve" element={<ResolvePage />} />
        </Routes>
      </main>
    </div>
  )
}
