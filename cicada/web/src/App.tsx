import { Routes, Route, NavLink, Navigate } from 'react-router-dom'
import {
  LayoutDashboard, Megaphone, Users, PhoneCall,
  FileText, Settings, Bug,
} from 'lucide-react'
import DashboardPage from './pages/DashboardPage'
import CampaignPage from './pages/CampaignPage'
import CustomerPage from './pages/CustomerPage'
import RecordPage from './pages/RecordPage'
import ScriptPage from './pages/ScriptPage'
import SettingsPage from './pages/SettingsPage'

const nav = [
  { to: '/dashboard', icon: LayoutDashboard, label: '数据总览' },
  { to: '/campaigns', icon: Megaphone, label: '外呼任务' },
  { to: '/customers', icon: Users, label: '客户管理' },
  { to: '/records', icon: PhoneCall, label: '通话记录' },
  { to: '/scripts', icon: FileText, label: '话术管理' },
  { to: '/settings', icon: Settings, label: '系统设置' },
]

export default function App() {
  return (
    <div className="flex h-screen">
      {/* Sidebar */}
      <aside className="w-56 shrink-0 bg-stone-900 border-r border-stone-800 flex flex-col">
        <div className="h-14 flex items-center gap-2 px-4 border-b border-stone-800">
          <Bug className="w-6 h-6 text-cicada-400 animate-ring" />
          <span className="font-bold text-lg">Cicada</span>
          <span className="text-xs text-stone-500 ml-auto">v1.0</span>
        </div>
        <nav className="flex-1 py-2 space-y-0.5 px-2">
          {nav.map(n => (
            <NavLink
              key={n.to}
              to={n.to}
              className={({ isActive }) =>
                `flex items-center gap-2.5 px-3 py-2 rounded-lg text-sm transition-colors ${
                  isActive
                    ? 'bg-cicada-500/15 text-cicada-400 font-medium'
                    : 'text-stone-400 hover:text-stone-200 hover:bg-stone-800'
                }`
              }
            >
              <n.icon className="w-4 h-4" />
              {n.label}
            </NavLink>
          ))}
        </nav>
        <div className="p-3 border-t border-stone-800 text-xs text-stone-600 text-center">
          StarClaw / Cicada
        </div>
      </aside>

      {/* Main */}
      <main className="flex-1 overflow-auto">
        <Routes>
          <Route path="/dashboard" element={<DashboardPage />} />
          <Route path="/campaigns" element={<CampaignPage />} />
          <Route path="/customers" element={<CustomerPage />} />
          <Route path="/records" element={<RecordPage />} />
          <Route path="/scripts" element={<ScriptPage />} />
          <Route path="/settings" element={<SettingsPage />} />
          <Route path="*" element={<Navigate to="/dashboard" replace />} />
        </Routes>
      </main>
    </div>
  )
}
