import { useState, useEffect } from 'react'
import { User, Shield, LogOut, Users, Zap, Target } from 'lucide-react'
import { getUser, clearAuth, api } from '../api/client'

interface TeamStats {
  total_instances: number; active_instances: number; total_missions: number; total_energy: number; template_count: number
}

export default function ProfilePage() {
  const user = getUser()
  const [stats, setStats] = useState<TeamStats | null>(null)

  useEffect(() => {
    api.teamStats().then(setStats).catch(() => {})
  }, [])

  const handleLogout = () => {
    clearAuth()
    window.location.reload()
  }

  return (
    <div className="p-4 md:p-6 max-w-lg mx-auto">
      <div className="mb-4 md:mb-6">
        <h1 className="text-lg md:text-xl font-bold text-white">我的</h1>
      </div>

      {/* Profile card */}
      <div className="bg-gray-900 border border-gray-800 rounded-xl p-4 md:p-5 mb-4">
        <div className="flex items-center gap-4">
          <div className="w-14 h-14 rounded-full bg-brand-600/20 flex items-center justify-center">
            <User className="w-7 h-7 text-brand-400" />
          </div>
          <div className="flex-1">
            <h2 className="text-lg font-semibold text-white">{user?.username || '用户'}</h2>
            <div className="flex items-center gap-3 mt-1">
              <span className="flex items-center gap-1 text-xs text-gray-400">
                <Shield className="w-3 h-3" />
                {user?.role || 'viewer'}
              </span>
              {user?.team_id && (
                <span className="text-xs text-gray-500">团队: {user.team_id}</span>
              )}
            </div>
          </div>
        </div>
      </div>

      {/* Real team stats */}
      {stats && (
        <>
          <h2 className="text-sm font-semibold text-gray-400 mb-3">团队概览</h2>
          <div className="grid grid-cols-2 gap-2 mb-4">
            <div className="bg-gray-900 border border-gray-800 rounded-xl p-3.5">
              <div className="flex items-center gap-2 mb-1.5">
                <Users className="w-4 h-4 text-brand-400" />
                <span className="text-xs text-gray-400">AI 团队</span>
              </div>
              <div className="text-lg font-bold text-white tabular-nums">{stats.total_instances}</div>
              <div className="text-[10px] text-gray-500">{stats.active_instances} 运行中</div>
            </div>
            <div className="bg-gray-900 border border-gray-800 rounded-xl p-3.5">
              <div className="flex items-center gap-2 mb-1.5">
                <Target className="w-4 h-4 text-green-400" />
                <span className="text-xs text-gray-400">任务</span>
              </div>
              <div className="text-lg font-bold text-white tabular-nums">{stats.total_missions}</div>
              <div className="text-[10px] text-gray-500">{stats.template_count} 种模板</div>
            </div>
          </div>
        </>
      )}

      {/* About */}
      <h2 className="text-sm font-semibold text-gray-400 mb-3">关于</h2>
      <div className="bg-gray-900 border border-gray-800 rounded-xl divide-y divide-gray-800 mb-4">
        <div className="px-4 py-3.5 flex items-center justify-between">
          <span className="text-sm text-gray-300">平台</span>
          <span className="text-sm text-gray-500">StarClaw Team Agent</span>
        </div>
        <div className="px-4 py-3.5 flex items-center justify-between">
          <span className="text-sm text-gray-300">引擎</span>
          <span className="text-sm text-gray-500">Claw (MIT 开源)</span>
        </div>
        <div className="px-4 py-3.5 flex items-center justify-between">
          <span className="text-sm text-gray-300">核心能力</span>
          <span className="text-sm text-gray-500">Squad · RAG · MCP · P2P</span>
        </div>
      </div>

      {/* Logout */}
      <button
        onClick={handleLogout}
        className="w-full flex items-center justify-center gap-2 py-3 bg-gray-900 border border-gray-800 rounded-xl text-sm text-red-400 hover:bg-red-400/10 transition active:scale-[0.98]"
      >
        <LogOut className="w-4 h-4" />
        退出登录
      </button>
    </div>
  )
}
