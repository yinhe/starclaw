import { Routes, Route, NavLink, useLocation, Navigate } from 'react-router-dom'
import {
  LayoutDashboard,
  KanbanSquare,
  FileText,
  Zap,
  Activity,
  LogOut,
  Flame,
} from 'lucide-react'
import DashboardPage from './pages/DashboardPage'
import BoardPage from './pages/BoardPage'
import PRDPage from './pages/PRDPage'
import SprintPage from './pages/SprintPage'
import OrchestratorPage from './pages/OrchestratorPage'
import LoginPage from './pages/LoginPage'
import { isLoggedIn, clearToken, getNodeId } from './api'

const nav = [
  { to: '/', icon: LayoutDashboard, label: '大屏' },
  { to: '/board', icon: KanbanSquare, label: '看板' },
  { to: '/prd', icon: FileText, label: 'PRD' },
  { to: '/sprints', icon: Zap, label: 'Sprint' },
  { to: '/orchestrator', icon: Activity, label: '调度' },
]

export default function App() {
  const loc = useLocation()

  // Login page — no auth guard
  if (loc.pathname === '/login') {
    return (
      <Routes>
        <Route path="/login" element={<LoginPage />} />
      </Routes>
    )
  }

  // Auth guard — redirect to login if not authenticated
  if (!isLoggedIn()) {
    return <Navigate to="/login" replace />
  }

  const handleLogout = () => {
    clearToken()
    window.location.href = '/login'
  }

  return (
    <div className="flex h-screen overflow-hidden">
      {/* Sidebar */}
      <aside className="w-16 flex flex-col items-center py-4 gap-1 bg-stone-950 border-r border-stone-800/50">
        <div className="mb-4 p-2">
          <Flame className="w-7 h-7 text-forge-500" />
        </div>
        {nav.map((n) => {
          const active = n.to === '/' ? loc.pathname === '/' : loc.pathname.startsWith(n.to)
          return (
            <NavLink
              key={n.to}
              to={n.to}
              className={`flex flex-col items-center justify-center w-12 h-12 rounded-lg text-xs gap-0.5 transition-all ${
                active
                  ? 'bg-forge-500/20 text-forge-400'
                  : 'text-stone-500 hover:text-stone-300 hover:bg-stone-800/50'
              }`}
            >
              <n.icon className="w-5 h-5" />
              <span className="text-[10px]">{n.label}</span>
            </NavLink>
          )
        })}
        <div className="flex-1" />
        <div className="text-center mb-1">
          <span className="text-[9px] text-stone-600 font-mono">{getNodeId()}</span>
        </div>
        <button
          onClick={handleLogout}
          className="w-12 h-12 flex items-center justify-center text-stone-600 hover:text-red-400 rounded-lg hover:bg-stone-800/50 transition-all"
          title="退出登录"
        >
          <LogOut className="w-5 h-5" />
        </button>
      </aside>

      {/* Main content */}
      <main className="flex-1 overflow-auto">
        <Routes>
          <Route path="/" element={<DashboardPage />} />
          <Route path="/board" element={<BoardPage />} />
          <Route path="/prd" element={<PRDPage />} />
          <Route path="/sprints" element={<SprintPage />} />
          <Route path="/orchestrator" element={<OrchestratorPage />} />
          <Route path="*" element={<Navigate to="/" replace />} />
        </Routes>
      </main>
    </div>
  )
}
