import { Routes, Route, NavLink, useLocation } from 'react-router-dom'
import {
  LayoutDashboard,
  KanbanSquare,
  FileText,
  Zap,
  Activity,
  Settings,
  Flame,
} from 'lucide-react'
import DashboardPage from './pages/DashboardPage'
import BoardPage from './pages/BoardPage'
import PRDPage from './pages/PRDPage'
import SprintPage from './pages/SprintPage'
import OrchestratorPage from './pages/OrchestratorPage'

const nav = [
  { to: '/', icon: LayoutDashboard, label: '大屏' },
  { to: '/board', icon: KanbanSquare, label: '看板' },
  { to: '/prd', icon: FileText, label: 'PRD' },
  { to: '/sprints', icon: Zap, label: 'Sprint' },
  { to: '/orchestrator', icon: Activity, label: '调度' },
]

export default function App() {
  const loc = useLocation()

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
        <button className="w-12 h-12 flex items-center justify-center text-stone-600 hover:text-stone-400 rounded-lg hover:bg-stone-800/50 transition-all">
          <Settings className="w-5 h-5" />
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
        </Routes>
      </main>
    </div>
  )
}
