import { useEffect, useState } from 'react'
import { api, type BillingStats, type ReportStats, type UserStats, type ServiceStats } from '../api'
import StatCard from '../components/StatCard'
import {
  DollarSign,
  TrendingUp,
  Users,
  Server,
  Wifi,
  ShoppingCart,
  Wallet,
  Flag,
  AlertTriangle,
  Trophy,
  MessageSquare,
  Swords,
  RefreshCw,
} from 'lucide-react'

function fen2yuan(fen: number): string {
  return `¥${(fen / 100).toFixed(2)}`
}

interface SwarmFull {
  total_nodes: number
  online_nodes: number
  claw_nodes: number
  overlord_nodes: number
  avg_cpu: number
  avg_mem: number
  total_tasks_running: number
  total_tokens_30d: number
  version_distribution: { version: string; count: number }[]
}

export default function DashboardPage() {
  const [billing, setBilling] = useState<BillingStats | null>(null)
  const [swarm, setSwarm] = useState<SwarmFull | null>(null)
  const [userStats, setUserStats] = useState<UserStats | null>(null)
  const [reportStats, setReportStats] = useState<ReportStats | null>(null)
  const [services, setServices] = useState<{ bounty: ServiceStats | null; forum: ServiceStats | null; arena: ServiceStats | null }>({ bounty: null, forum: null, arena: null })
  const [loading, setLoading] = useState(true)
  const [refreshing, setRefreshing] = useState(false)

  const load = async (manual = false) => {
    if (manual) setRefreshing(true)
    const [b, s, u, r, bounty, forum, arena] = await Promise.all([
      api.get<BillingStats>('/v1/admin/billing/stats').catch(() => null),
      api.get<SwarmFull>('/v1/admin/stats').catch(() => null),
      api.get<{ data: UserStats }>('/v1/admin/users/stats').catch(() => null),
      api.get<{ data: ReportStats }>('/v1/admin/reports/stats').catch(() => null),
      api.get<ServiceStats>('/v1/admin/bounty/stats').catch(() => null),
      api.get<ServiceStats>('/v1/admin/forum/stats').catch(() => null),
      api.get<ServiceStats>('/v1/admin/arena/stats').catch(() => null),
    ])
    setBilling(b)
    setSwarm(s)
    setUserStats(u?.data || null)
    setReportStats(r?.data || null)
    setServices({ bounty, forum, arena })
    setLoading(false)
    setRefreshing(false)
  }

  useEffect(() => { load() }, [])

  if (loading) {
    return <div className="text-gray-500 text-center py-20">加载中...</div>
  }

  const versions = swarm?.version_distribution || []
  const totalVersionNodes = versions.reduce((a, v) => a + v.count, 0)

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <h2 className="text-xl font-bold">仪表盘</h2>
        <button onClick={() => load(true)} disabled={refreshing}
          className="flex items-center gap-1.5 px-3 py-1.5 bg-gray-800 text-gray-400 rounded-lg text-xs hover:bg-gray-700 disabled:opacity-50 transition">
          <RefreshCw size={12} className={refreshing ? 'animate-spin' : ''} /> 刷新
        </button>
      </div>

      {/* Billing stats */}
      <h3 className="text-sm font-medium text-gray-400 uppercase tracking-wider mb-3">收入概况</h3>
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4 mb-6">
        <StatCard title="今日收入" value={billing ? fen2yuan(billing.today_revenue) : '--'} icon={DollarSign} sub={billing ? `${billing.today_orders} 笔订单` : ''} color="green" />
        <StatCard title="本月收入" value={billing ? fen2yuan(billing.month_revenue) : '--'} icon={TrendingUp} color="blue" />
        <StatCard title="累计收入" value={billing ? fen2yuan(billing.total_revenue) : '--'} icon={ShoppingCart} sub={billing ? `共 ${billing.total_orders} 笔` : ''} color="purple" />
        <StatCard title="用户总余额" value={billing ? fen2yuan(billing.total_balance) : '--'} icon={Wallet} sub={billing ? `${billing.total_users} 个用户` : ''} color="amber" />
      </div>

      {/* Swarm + Users */}
      <h3 className="text-sm font-medium text-gray-400 uppercase tracking-wider mb-3">集群 & 用户</h3>
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4 mb-6">
        <StatCard title="总节点" value={swarm?.total_nodes ?? '--'} icon={Server} sub={swarm ? `Claw ${swarm.claw_nodes} · Overlord ${swarm.overlord_nodes}` : ''} color="purple" />
        <StatCard title="在线节点" value={swarm?.online_nodes ?? '--'} icon={Wifi} color="green" />
        <StatCard title="总用户" value={userStats?.total ?? '--'} icon={Users} sub={userStats ? `活跃 ${userStats.active} · 封禁 ${userStats.banned}` : ''} color="cyan" />
        <StatCard title="待审举报" value={reportStats?.pending ?? '--'} icon={Flag} sub={reportStats ? `共 ${reportStats.total} 条` : ''} color="red" />
      </div>

      <div className="grid grid-cols-3 gap-4 mb-6">
        {/* Version distribution */}
        <div className="bg-gray-900 border border-gray-800 rounded-xl p-5">
          <h4 className="text-sm font-medium text-white mb-3">版本分布</h4>
          {versions.length === 0 ? (
            <p className="text-xs text-gray-600 py-4 text-center">暂无数据</p>
          ) : (
            <div className="space-y-2">
              {versions.slice(0, 6).map(v => {
                const pct = totalVersionNodes > 0 ? (v.count / totalVersionNodes) * 100 : 0
                return (
                  <div key={v.version}>
                    <div className="flex items-center justify-between mb-0.5">
                      <span className="text-xs text-gray-400 font-mono">{v.version || '未知'}</span>
                      <span className="text-[10px] text-gray-500">{v.count} ({pct.toFixed(0)}%)</span>
                    </div>
                    <div className="h-1.5 bg-gray-800 rounded-full overflow-hidden">
                      <div className="h-full bg-purple-500 rounded-full transition-all" style={{ width: `${pct}%` }} />
                    </div>
                  </div>
                )
              })}
            </div>
          )}
        </div>

        {/* Cluster metrics */}
        <div className="bg-gray-900 border border-gray-800 rounded-xl p-5">
          <h4 className="text-sm font-medium text-white mb-3">集群指标</h4>
          <div className="space-y-3">
            {[
              { label: '平均 CPU', value: `${(swarm?.avg_cpu ?? 0).toFixed(1)}%`, warn: (swarm?.avg_cpu ?? 0) > 80 },
              { label: '平均内存', value: `${(swarm?.avg_mem ?? 0).toFixed(1)}%`, warn: (swarm?.avg_mem ?? 0) > 80 },
              { label: '运行任务', value: swarm?.total_tasks_running ?? 0, warn: false },
              { label: '30 天 Tokens', value: (swarm?.total_tokens_30d ?? 0).toLocaleString(), warn: false },
            ].map(m => (
              <div key={m.label} className="flex items-center justify-between">
                <span className="text-xs text-gray-500">{m.label}</span>
                <div className="flex items-center gap-1.5">
                  {m.warn && <AlertTriangle size={10} className="text-amber-400" />}
                  <span className={`text-sm font-medium ${m.warn ? 'text-amber-400' : 'text-white'}`}>{m.value}</span>
                </div>
              </div>
            ))}
          </div>
        </div>

        {/* Service health */}
        <div className="bg-gray-900 border border-gray-800 rounded-xl p-5">
          <h4 className="text-sm font-medium text-white mb-3">服务状态</h4>
          <div className="space-y-2.5">
            {[
              { name: '赏金系统', icon: Trophy, ok: !!services.bounty, stat: services.bounty ? `${services.bounty.total_tasks ?? 0} 任务` : '' },
              { name: '用户社区', icon: MessageSquare, ok: !!services.forum, stat: services.forum ? `${services.forum.total_posts ?? 0} 帖子` : '' },
              { name: '龙虾竞技', icon: Swords, ok: !!services.arena, stat: services.arena ? `${services.arena.total_agents ?? 0} Agent` : '' },
              { name: '计费服务', icon: Wallet, ok: !!billing, stat: billing ? `${billing.total_users} 用户` : '' },
              { name: '虫群管理', icon: Server, ok: !!swarm, stat: swarm ? `${swarm.total_nodes} 节点` : '' },
            ].map(s => (
              <div key={s.name} className="flex items-center gap-2.5">
                <div className={`w-2 h-2 rounded-full ${s.ok ? 'bg-emerald-500' : 'bg-red-500'}`} />
                <s.icon size={13} className="text-gray-500" />
                <span className="text-xs text-gray-300 flex-1">{s.name}</span>
                <span className="text-[10px] text-gray-600">{s.ok ? s.stat : '不可达'}</span>
              </div>
            ))}
          </div>
        </div>
      </div>
    </div>
  )
}
