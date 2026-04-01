import { useEffect, useState } from 'react'
import {
  Phone, PhoneCall, PhoneOff, UserCheck, TrendingUp,
  Clock, BarChart3, Activity,
} from 'lucide-react'
import { getStats, getOverview } from '../api'

const INTENT_COLORS: Record<string, string> = {
  A: 'bg-red-500', B: 'bg-orange-500', C: 'bg-yellow-500',
  D: 'bg-green-500', E: 'bg-blue-500', F: 'bg-stone-500',
}

export default function DashboardPage() {
  const [stats, setStats] = useState<any>(null)
  const [overview, setOverview] = useState<any>(null)
  const [error, setError] = useState('')

  const load = () => {
    getStats().then(setStats).catch(e => setError(e.message))
    getOverview().then(setOverview).catch(() => {})
  }

  useEffect(() => { load(); const t = setInterval(load, 10000); return () => clearInterval(t) }, [])

  const sched = stats?.scheduler || {}
  const eng = stats?.engine || {}
  const disk = stats?.storage || {}
  const ov = overview || {}

  const cards = [
    { label: '今日已呼', value: sched.today_called ?? '-', icon: Phone, color: 'text-cicada-400' },
    { label: '活跃通话', value: eng.active_calls ?? 0, icon: PhoneCall, color: 'text-yellow-400' },
    { label: '日上限', value: sched.daily_limit ?? '-', icon: TrendingUp, color: 'text-stone-400' },
    { label: '最大并发', value: eng.max_concurrent ?? '-', icon: Activity, color: 'text-blue-400' },
    { label: '总客户数', value: ov.total_customers ?? '-', icon: UserCheck, color: 'text-green-400' },
    { label: '总通话数', value: ov.total_calls ?? '-', icon: PhoneOff, color: 'text-orange-400' },
    { label: '录音容量', value: `${disk.total_size_mb ?? 0} MB`, icon: BarChart3, color: 'text-purple-400' },
    { label: '调度状态', value: sched.running ? '运行中' : '已停止', icon: Clock, color: sched.running ? 'text-cicada-400' : 'text-stone-500' },
  ]

  return (
    <div className="p-6 space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold">数据总览</h1>
        <span className={`text-xs px-2 py-1 rounded ${sched.is_calling_time ? 'bg-cicada-500/20 text-cicada-400' : 'bg-stone-800 text-stone-500'}`}>
          {sched.is_calling_time ? '● 外呼时段' : '○ 非外呼时段'}
        </span>
      </div>

      {error && <div className="bg-red-500/10 text-red-400 px-4 py-2 rounded">{error}</div>}

      {/* Stat Cards */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
        {cards.map(c => (
          <div key={c.label} className="bg-stone-900 border border-stone-800 rounded-xl p-4">
            <div className="flex items-center justify-between mb-2">
              <span className="text-xs text-stone-500">{c.label}</span>
              <c.icon className={`w-4 h-4 ${c.color}`} />
            </div>
            <div className="text-2xl font-bold">{c.value}</div>
          </div>
        ))}
      </div>

      {/* Intent Distribution */}
      {ov.intent_distribution && (
        <div className="bg-stone-900 border border-stone-800 rounded-xl p-5">
          <h2 className="text-sm font-medium text-stone-400 mb-4">意向分布</h2>
          <div className="flex gap-2 h-8">
            {Object.entries(ov.intent_distribution as Record<string, number>).map(([level, count]) => {
              const total = Object.values(ov.intent_distribution as Record<string, number>).reduce((a: number, b: number) => a + b, 0)
              const pct = total > 0 ? (count / total) * 100 : 0
              return pct > 0 ? (
                <div
                  key={level}
                  className={`${INTENT_COLORS[level] || 'bg-stone-600'} rounded flex items-center justify-center text-xs font-bold text-white`}
                  style={{ width: `${Math.max(pct, 5)}%` }}
                  title={`${level}类: ${count}人 (${pct.toFixed(1)}%)`}
                >
                  {level}
                </div>
              ) : null
            })}
          </div>
          <div className="flex gap-4 mt-3 text-xs text-stone-500">
            {Object.entries(ov.intent_distribution as Record<string, number>).map(([level, count]) => (
              <span key={level}>{level}类: {count}</span>
            ))}
          </div>
        </div>
      )}

      {/* Campaign Overview */}
      {ov.campaigns && (
        <div className="bg-stone-900 border border-stone-800 rounded-xl p-5">
          <h2 className="text-sm font-medium text-stone-400 mb-3">活跃任务</h2>
          {(ov.campaigns as any[]).length === 0 ? (
            <p className="text-stone-600 text-sm">暂无活跃任务</p>
          ) : (
            <div className="space-y-2">
              {(ov.campaigns as any[]).slice(0, 5).map((c: any) => (
                <div key={c.id} className="flex items-center gap-3 text-sm">
                  <span className={`w-2 h-2 rounded-full ${c.status === 'running' ? 'bg-cicada-400' : 'bg-stone-600'}`} />
                  <span className="flex-1 truncate">{c.name}</span>
                  <span className="text-stone-500">{c.called}/{c.total}</span>
                  <span className="text-stone-500">{c.connected_rate}%</span>
                </div>
              ))}
            </div>
          )}
        </div>
      )}
    </div>
  )
}
