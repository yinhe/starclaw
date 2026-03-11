import { useEffect, useState, useRef, useCallback } from 'react'
import { Link } from 'react-router-dom'
import { Server, Wifi, WifiOff, AlertTriangle, Cpu, MemoryStick, Activity, Coins, RefreshCw, Clock, ArrowRight } from 'lucide-react'
import { broodAPI, type BroodStats, type ClawNode } from '../api/brood'

const REFRESH_INTERVAL = 15

export default function DashboardPage() {
  const [stats, setStats] = useState<BroodStats | null>(null)
  const [recentAlerts, setRecentAlerts] = useState<ClawNode[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [lastRefresh, setLastRefresh] = useState<Date | null>(null)
  const [countdown, setCountdown] = useState(REFRESH_INTERVAL)
  const [refreshing, setRefreshing] = useState(false)
  const countdownRef = useRef<ReturnType<typeof setInterval>>()

  const load = useCallback(async (manual = false) => {
    try {
      if (manual) setRefreshing(true)
      setError('')
      const [statsData, alertData] = await Promise.all([
        broodAPI.stats(),
        broodAPI.listClaws({ status: 'feral' }).catch(() => ({ claws: [] as ClawNode[], total: 0 })),
      ])
      setStats(statsData)
      setRecentAlerts(alertData.claws?.slice(0, 5) || [])
      setLastRefresh(new Date())
      setCountdown(REFRESH_INTERVAL)
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to load')
    } finally {
      setLoading(false)
      setRefreshing(false)
    }
  }, [])

  useEffect(() => {
    load()
    const timer = setInterval(() => load(), REFRESH_INTERVAL * 1000)
    countdownRef.current = setInterval(() => setCountdown(c => Math.max(0, c - 1)), 1000)
    return () => { clearInterval(timer); clearInterval(countdownRef.current) }
  }, [load])

  if (loading && !stats) {
    return (
      <div className="flex items-center justify-center h-full">
        <div className="animate-spin w-8 h-8 border-2 border-overlord-500 border-t-transparent rounded-full" />
      </div>
    )
  }

  if (error && !stats) {
    return (
      <div className="flex flex-col items-center justify-center h-full gap-4">
        <AlertTriangle className="w-12 h-12 text-red-400" />
        <p className="text-red-400">{error}</p>
        <button onClick={() => load(true)} className="px-4 py-2 bg-overlord-600 text-white rounded-lg text-sm hover:bg-overlord-500 transition">
          重试
        </button>
      </div>
    )
  }

  if (!stats) return null

  const healthPct = stats.total > 0 ? (stats.online / stats.total) * 100 : 0
  const healthColor = healthPct >= 90 ? 'text-emerald-400' : healthPct >= 50 ? 'text-amber-400' : 'text-red-400'

  const statCards = [
    { label: '总节点', value: stats.total, icon: Server, color: 'text-overlord-400', bg: 'bg-overlord-600/10', link: '/claws' },
    { label: '在线', value: stats.online, icon: Wifi, color: 'text-emerald-400', bg: 'bg-emerald-600/10', link: '/claws?status=online' },
    { label: '失控', value: stats.feral, icon: AlertTriangle, color: 'text-amber-400', bg: 'bg-amber-600/10', link: '/claws?status=feral' },
    { label: '离线', value: stats.offline, icon: WifiOff, color: 'text-gray-400', bg: 'bg-gray-600/10', link: '/claws?status=offline' },
  ]

  const metricCards = [
    { label: '平均 CPU', value: `${stats.avg_cpu?.toFixed(1) ?? 0}%`, icon: Cpu, color: 'text-blue-400', warn: (stats.avg_cpu ?? 0) > 80 },
    { label: '平均内存', value: `${stats.avg_mem?.toFixed(1) ?? 0}%`, icon: MemoryStick, color: 'text-purple-400', warn: (stats.avg_mem ?? 0) > 80 },
    { label: '运行任务', value: stats.total_tasks ?? 0, icon: Activity, color: 'text-cyan-400', warn: false },
    { label: '今日 Tokens', value: stats.total_tokens?.toLocaleString() ?? 0, icon: Coins, color: 'text-yellow-400', warn: false },
  ]

  return (
    <div className="p-8">
      {/* Header with live status */}
      <div className="flex items-center justify-between mb-8">
        <div>
          <div className="flex items-center gap-3">
            <h1 className="text-2xl font-bold text-white">虫群总览</h1>
            <div className="flex items-center gap-1.5">
              <span className="relative flex h-2.5 w-2.5">
                <span className={`animate-ping absolute inline-flex h-full w-full rounded-full opacity-75 ${healthPct >= 90 ? 'bg-emerald-400' : healthPct >= 50 ? 'bg-amber-400' : 'bg-red-400'}`}></span>
                <span className={`relative inline-flex rounded-full h-2.5 w-2.5 ${healthPct >= 90 ? 'bg-emerald-500' : healthPct >= 50 ? 'bg-amber-500' : 'bg-red-500'}`}></span>
              </span>
              <span className={`text-xs font-medium ${healthColor}`}>{healthPct.toFixed(0)}% 健康</span>
            </div>
          </div>
          <div className="flex items-center gap-3 mt-1">
            <p className="text-sm text-gray-500">Brood Cluster Dashboard</p>
            {lastRefresh && (
              <span className="flex items-center gap-1 text-[10px] text-gray-600">
                <Clock className="w-3 h-3" />
                {lastRefresh.toLocaleTimeString('zh-CN')} · {countdown}s
              </span>
            )}
          </div>
        </div>
        <button
          onClick={() => load(true)}
          disabled={refreshing}
          className="flex items-center gap-2 px-4 py-2 bg-gray-800 text-gray-300 rounded-lg text-sm hover:bg-gray-700 disabled:opacity-50 transition"
        >
          <RefreshCw className={`w-4 h-4 ${refreshing ? 'animate-spin' : ''}`} />
          刷新
        </button>
      </div>

      {/* Node status cards — clickable */}
      <div className="grid grid-cols-4 gap-4 mb-6">
        {statCards.map(({ label, value, icon: Icon, color, bg, link }) => (
          <Link key={label} to={link} className="bg-gray-900 border border-gray-800 rounded-xl p-5 hover:border-gray-700 transition group">
            <div className="flex items-center justify-between mb-3">
              <span className="text-sm text-gray-400">{label}</span>
              <div className={`w-8 h-8 rounded-lg ${bg} flex items-center justify-center`}>
                <Icon className={`w-4 h-4 ${color}`} />
              </div>
            </div>
            <div className="flex items-end justify-between">
              <div className="text-3xl font-bold text-white">{value}</div>
              <ArrowRight className="w-4 h-4 text-gray-700 group-hover:text-gray-400 transition" />
            </div>
          </Link>
        ))}
      </div>

      {/* Metric cards with warnings */}
      <div className="grid grid-cols-4 gap-4 mb-6">
        {metricCards.map(({ label, value, icon: Icon, color, warn }) => (
          <div key={label} className={`bg-gray-900 border rounded-xl p-5 ${warn ? 'border-amber-600/30' : 'border-gray-800'}`}>
            <div className="flex items-center gap-2 mb-2">
              <Icon className={`w-4 h-4 ${color}`} />
              <span className="text-sm text-gray-400">{label}</span>
              {warn && <span className="text-[10px] px-1.5 py-0.5 rounded bg-amber-600/10 text-amber-400 ml-auto">高</span>}
            </div>
            <div className="text-2xl font-semibold text-white">{value}</div>
          </div>
        ))}
      </div>

      <div className="grid grid-cols-3 gap-4">
        {/* Feral alert panel */}
        <div className="bg-gray-900 border border-gray-800 rounded-xl p-5">
          <div className="flex items-center justify-between mb-3">
            <h3 className="text-sm font-medium text-white">失控节点告警</h3>
            {stats.feral > 0 && (
              <Link to="/claws?status=feral" className="text-[10px] text-overlord-400 hover:text-overlord-300 transition">
                查看全部 →
              </Link>
            )}
          </div>
          {recentAlerts.length === 0 ? (
            <div className="text-xs text-gray-600 py-4 text-center">
              <Wifi className="w-5 h-5 mx-auto mb-1 text-emerald-600" />
              全部节点正常
            </div>
          ) : (
            <div className="space-y-2">
              {recentAlerts.map(n => (
                <Link key={n.id} to={`/claws/${n.id}`} className="flex items-center gap-2 text-xs bg-amber-600/5 border border-amber-600/10 rounded-lg px-3 py-2 hover:bg-amber-600/10 transition">
                  <AlertTriangle className="w-3.5 h-3.5 text-amber-400 shrink-0" />
                  <span className="text-white font-medium truncate">{n.name}</span>
                  <span className="text-gray-500 ml-auto shrink-0">{n.team || '—'}</span>
                </Link>
              ))}
            </div>
          )}
        </div>

        {/* Team breakdown */}
        <div className="col-span-2 bg-gray-900 border border-gray-800 rounded-xl p-5">
          <h3 className="text-sm font-medium text-white mb-3">团队分布</h3>
          {stats.teams && stats.teams.length > 0 ? (
            <div className="space-y-2.5">
              {stats.teams.map((t) => {
                const pct = stats.total > 0 ? (t.count / stats.total) * 100 : 0
                return (
                  <div key={t.team}>
                    <div className="flex items-center justify-between mb-1">
                      <Link
                        to={`/claws?team=${encodeURIComponent(t.team)}`}
                        className="text-sm text-overlord-300 hover:text-overlord-200 transition"
                      >
                        {t.team}
                      </Link>
                      <span className="text-xs text-gray-500">{t.count} 节点 · {pct.toFixed(0)}%</span>
                    </div>
                    <div className="h-1.5 bg-gray-800 rounded-full overflow-hidden">
                      <div
                        className="h-full bg-overlord-500 rounded-full transition-all"
                        style={{ width: `${pct}%` }}
                      />
                    </div>
                  </div>
                )
              })}
            </div>
          ) : (
            <p className="text-xs text-gray-600 py-4 text-center">暂无团队数据</p>
          )}
        </div>
      </div>

      {stats.total === 0 && (
        <div className="bg-gray-900 border border-gray-800 border-dashed rounded-xl p-12 text-center mt-6">
          <Server className="w-12 h-12 text-gray-600 mx-auto mb-4" />
          <h3 className="text-lg font-medium text-gray-300 mb-2">暂无 Claw 节点</h3>
          <p className="text-sm text-gray-500">通过 Claw 配置中的 Overlord URL 注册节点到此虫群</p>
        </div>
      )}
    </div>
  )
}
