import { useState } from 'react'
import { Routes, Route, NavLink, useLocation } from 'react-router-dom'
import { MessageSquare, Bot, Wrench, User, LogOut, Users } from 'lucide-react'
import { getToken, getUser, clearAuth } from './api/client'
import { getBrand } from './lib/brand'
import LoginPage from './pages/LoginPage'
import ChatPage from './pages/ChatPage'
import AgentsPage from './pages/AgentsPage'
import ToolsPage from './pages/ToolsPage'
import ProfilePage from './pages/ProfilePage'
import TeamPage from './pages/TeamPage'

const navItems = [
  { to: '/', icon: MessageSquare, label: '对话' },
  { to: '/agents', icon: Bot, label: '智能体' },
  { to: '/team', icon: Users, label: '团队' },
  { to: '/tools', icon: Wrench, label: '工具' },
  { to: '/profile', icon: User, label: '我的' },
]

export default function App() {
  const location = useLocation()
  const [authed, setAuthed] = useState(() => !!getToken())
  const user = getUser()
  const brand = getBrand()

  const handleLogout = () => {
    clearAuth()
    setAuthed(false)
  }

  if (!authed) {
    return <LoginPage onLogin={() => setAuthed(true)} />
  }

  return (
    <div className="flex flex-col md:flex-row h-[100dvh]">
      {/* Desktop Sidebar — hidden on mobile */}
      <aside className="hidden md:flex w-16 bg-gray-900 border-r border-gray-800 flex-col items-center py-4 shrink-0">
        <div className="w-9 h-9 rounded-lg bg-brand-600/20 flex items-center justify-center mb-6" title={brand.brand_name}>
          {brand.enabled && brand.logo_url ? (
            <img src={brand.logo_url} alt={brand.brand_name} className="w-7 h-7 object-contain" />
          ) : (
            <span className="text-lg">🦞</span>
          )}
        </div>

        <nav className="flex-1 flex flex-col items-center gap-1">
          {navItems.map(({ to, icon: Icon, label }) => {
            const active = to === '/' ? location.pathname === '/' : location.pathname.startsWith(to)
            return (
              <NavLink
                key={to}
                to={to}
                title={label}
                className={`w-11 h-11 flex items-center justify-center rounded-xl transition-colors ${
                  active
                    ? 'bg-brand-600/20 text-brand-400'
                    : 'text-gray-500 hover:bg-gray-800 hover:text-gray-300'
                }`}
              >
                <Icon className="w-5 h-5" />
              </NavLink>
            )
          })}
        </nav>

        <div className="flex flex-col items-center gap-2">
          {user && (
            <div
              className="w-8 h-8 rounded-full bg-brand-600/20 flex items-center justify-center"
              title={user.username}
            >
              <span className="text-xs font-bold text-brand-400">
                {user.username.charAt(0).toUpperCase()}
              </span>
            </div>
          )}
          <button
            onClick={handleLogout}
            title="退出登录"
            className="w-11 h-11 flex items-center justify-center text-gray-600 hover:text-red-400 hover:bg-gray-800 rounded-xl transition"
          >
            <LogOut className="w-4 h-4" />
          </button>
          {brand.powered_by && (
            <div className="mt-1">
              <a
                href="https://starclaw.net?ref=overlord"
                target="_blank"
                rel="noopener noreferrer"
                className="text-[8px] text-gray-700 hover:text-brand-400/60 transition"
                title="Powered by StarClaw"
              >
                SC
              </a>
            </div>
          )}
        </div>
      </aside>

      {/* Main content */}
      <main className="flex-1 overflow-hidden min-h-0">
        <Routes>
          <Route path="/" element={<ChatPage />} />
          <Route path="/agents" element={<AgentsPage />} />
          <Route path="/team" element={<TeamPage />} />
          <Route path="/tools" element={<ToolsPage />} />
          <Route path="/profile" element={<ProfilePage />} />
        </Routes>
      </main>

      {/* Mobile Bottom Tab Bar — hidden on desktop */}
      <nav className="md:hidden flex items-center justify-around bg-gray-900 border-t border-gray-800 shrink-0 pb-[env(safe-area-inset-bottom)]">
        {navItems.map(({ to, icon: Icon, label }) => {
          const active = to === '/' ? location.pathname === '/' : location.pathname.startsWith(to)
          return (
            <NavLink
              key={to}
              to={to}
              className={`flex flex-col items-center gap-0.5 py-2 px-3 min-w-[56px] transition-colors ${
                active ? 'text-brand-400' : 'text-gray-500 active:text-gray-300'
              }`}
            >
              <Icon className="w-5 h-5" />
              <span className="text-[10px] leading-tight">{label}</span>
            </NavLink>
          )
        })}
      </nav>
    </div>
  )
}
