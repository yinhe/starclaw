import { useEffect, useState, useCallback } from 'react'
import { BarChart3, Cpu, Users, Coins, Zap, TrendingUp, Clock, RefreshCw, Calendar } from 'lucide-react'
import { broodAPI, type UsageDailySummary, type ModelUsage, type UserUsage } from '../api/brood'

function formatTokens(n: number): string {
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`
  if (n >= 1_000) return `${(n / 1_000).toFixed(1)}K`
  return String(n)
}

function formatCents(cents: number): string {
  return `¥${(cents / 100).toFixed(2)}`
}

function daysAgo(n: number): string {
  const d = new Date()
  d.setDate(d.getDate() - n)
  return d.toISOString().slice(0, 10)
}

type DateRange = '7d' | '14d' | '30d' | '90d'

export default function AnalyticsPage() {
  const [range, setRange] = useState<DateRange>('30d')
  const [daily, setDaily] = useState<UsageDailySummary[]>([])
  const [totals, setTotals] = useState<{ total_requests: number; total_tokens: number; input_tokens: number; output_tokens: number; total_cost_cents: number; total_star_energy: number } | null>(null)
  const [models, setModels] = useState<ModelUsage[]>([])
  const [users, setUsers] = useState<UserUsage[]>([])
  const [loading, setLoading] = useState(true)
  const [refreshing, setRefreshing] = useState(false)

  const rangeDays: Record<DateRange, number> = { '7d': 7, '14d': 14, '30d': 30, '90d': 90 }

  const load = useCallback(async (manual = false) => {
    try {
      if (manual) setRefreshing(true)
      const from = daysAgo(rangeDays[range])
      const to = daysAgo(0)
      const [statsData, modelData, userData] = await Promise.all([
        broodAPI.usageStats({ from, to }),
        broodAPI.usageByModel({ from, to }),
        broodAPI.usageByUser({ from, to, limit: 20 }),
      ])
      setDaily(statsData.daily || [])
      setTotals(statsData.totals || null)
      setModels(modelData.models || [])
      setUsers(userData.users || [])
    } catch { /* */ }
    finally { setLoading(false); setRefreshing(false) }
  }, [range])

  useEffect(() => { setLoading(true); load() }, [load])

  // Simple bar chart renderer (pure CSS, no external chart lib)
  const maxTokens = Math.max(...daily.map(d => d.total_tokens), 1)
  const maxRequests = Math.max(...daily.map(d => d.total_requests), 1)

  if (loading && !totals) {
    return (
      <div className="flex items-center justify-center h-full">
        <div className="animate-spin w-8 h-8 border-2 border-overlord-500 border-t-transparent rounded-full" />
      </div>
    )
  }

  return (
    <div className="p-8">
      {/* Header */}
      <div className="flex items-center justify-between mb-8">
        <div>
          <h1 className="text-2xl font-bold text-white">用量分析</h1>
          <p className="text-sm text-gray-500 mt-1">Token 消耗、模型分布与用户用量排行</p>
        </div>
        <div className="flex items-center gap-3">
          <div className="flex bg-gray-800 rounded-lg p-0.5">
            {(['7d', '14d', '30d', '90d'] as DateRange[]).map(r => (
              <button
                key={r}
                onClick={() => setRange(r)}
                className={`px-3 py-1.5 text-xs rounded-md transition ${range === r ? 'bg-overlord-600 text-white' : 'text-gray-400 hover:text-white'}`}
              >
                {r}
              </button>
            ))}
          </div>
          <button
            onClick={() => load(true)}
            disabled={refreshing}
            className="flex items-center gap-1.5 px-3 py-1.5 bg-gray-800 text-gray-300 rounded-lg text-xs hover:bg-gray-700 disabled:opacity-50 transition"
          >
            <RefreshCw className={`w-3.5 h-3.5 ${refreshing ? 'animate-spin' : ''}`} />
            刷新
          </button>
        </div>
      </div>

      {/* Summary Cards */}
      {totals && (
        <div className="grid grid-cols-6 gap-3 mb-8">
          <div className="bg-gray-900 border border-gray-800 rounded-xl p-4">
            <div className="flex items-center gap-1.5 mb-1.5">
              <Zap className="w-3.5 h-3.5 text-yellow-400" />
              <span className="text-xs text-gray-400">总 Tokens</span>
            </div>
            <div className="text-lg font-bold text-white">{formatTokens(totals.total_tokens)}</div>
          </div>
          <div className="bg-gray-900 border border-gray-800 rounded-xl p-4">
            <div className="flex items-center gap-1.5 mb-1.5">
              <TrendingUp className="w-3.5 h-3.5 text-blue-400" />
              <span className="text-xs text-gray-400">输入 Tokens</span>
            </div>
            <div className="text-lg font-bold text-white">{formatTokens(totals.input_tokens)}</div>
          </div>
          <div className="bg-gray-900 border border-gray-800 rounded-xl p-4">
            <div className="flex items-center gap-1.5 mb-1.5">
              <TrendingUp className="w-3.5 h-3.5 text-purple-400" />
              <span className="text-xs text-gray-400">输出 Tokens</span>
            </div>
            <div className="text-lg font-bold text-white">{formatTokens(totals.output_tokens)}</div>
          </div>
          <div className="bg-gray-900 border border-gray-800 rounded-xl p-4">
            <div className="flex items-center gap-1.5 mb-1.5">
              <BarChart3 className="w-3.5 h-3.5 text-cyan-400" />
              <span className="text-xs text-gray-400">总请求</span>
            </div>
            <div className="text-lg font-bold text-white">{totals.total_requests.toLocaleString()}</div>
          </div>
          <div className="bg-gray-900 border border-gray-800 rounded-xl p-4">
            <div className="flex items-center gap-1.5 mb-1.5">
              <Coins className="w-3.5 h-3.5 text-emerald-400" />
              <span className="text-xs text-gray-400">总费用</span>
            </div>
            <div className="text-lg font-bold text-white">{formatCents(totals.total_cost_cents)}</div>
          </div>
          <div className="bg-gray-900 border border-gray-800 rounded-xl p-4">
            <div className="flex items-center gap-1.5 mb-1.5">
              <Zap className="w-3.5 h-3.5 text-orange-400" />
              <span className="text-xs text-gray-400">星能消耗</span>
            </div>
            <div className="text-lg font-bold text-white">{totals.total_star_energy.toLocaleString()}</div>
          </div>
        </div>
      )}

      {/* Daily Usage Chart (CSS bars) */}
      <div className="bg-gray-900 border border-gray-800 rounded-xl p-5 mb-6">
        <div className="flex items-center justify-between mb-4">
          <h3 className="text-sm font-medium text-white flex items-center gap-2">
            <Calendar className="w-4 h-4 text-overlord-400" />
            每日用量趋势
          </h3>
          <div className="flex items-center gap-4 text-[10px] text-gray-500">
            <span className="flex items-center gap-1"><span className="w-2.5 h-2.5 rounded-sm bg-overlord-500 inline-block" /> Tokens</span>
            <span className="flex items-center gap-1"><span className="w-2.5 h-2.5 rounded-sm bg-cyan-500 inline-block" /> 请求数</span>
          </div>
        </div>
        {daily.length === 0 ? (
          <div className="text-center text-gray-600 text-sm py-8">暂无用量数据</div>
        ) : (
          <div className="flex items-end gap-[2px] h-40">
            {daily.map((d, i) => {
              const tokenPct = (d.total_tokens / maxTokens) * 100
              const reqPct = (d.total_requests / maxRequests) * 100
              return (
                <div key={d.date || i} className="flex-1 flex flex-col items-center gap-[1px] group relative">
                  <div className="w-full flex gap-[1px]" style={{ height: '100%', alignItems: 'flex-end' }}>
                    <div
                      className="flex-1 bg-overlord-500/70 rounded-t-sm transition-all hover:bg-overlord-400"
                      style={{ height: `${Math.max(tokenPct, 2)}%` }}
                    />
                    <div
                      className="flex-1 bg-cyan-500/70 rounded-t-sm transition-all hover:bg-cyan-400"
                      style={{ height: `${Math.max(reqPct, 2)}%` }}
                    />
                  </div>
                  {/* Tooltip */}
                  <div className="absolute bottom-full mb-2 left-1/2 -translate-x-1/2 bg-gray-800 border border-gray-700 rounded-lg px-3 py-2 text-[10px] whitespace-nowrap opacity-0 group-hover:opacity-100 pointer-events-none transition z-10">
                    <div className="text-gray-300 font-medium mb-1">{d.date}</div>
                    <div className="text-overlord-300">Tokens: {d.total_tokens.toLocaleString()}</div>
                    <div className="text-cyan-300">请求: {d.total_requests.toLocaleString()}</div>
                    <div className="text-gray-400">用户: {d.unique_users} · 模型: {d.unique_models}</div>
                    <div className="text-gray-400">延迟: {d.avg_latency_ms}ms</div>
                  </div>
                  {/* Date label (show every N days) */}
                  {(i === 0 || i === daily.length - 1 || i % Math.max(Math.floor(daily.length / 6), 1) === 0) && (
                    <div className="text-[9px] text-gray-600 mt-1 whitespace-nowrap">{d.date?.slice(5)}</div>
                  )}
                </div>
              )
            })}
          </div>
        )}
      </div>

      <div className="grid grid-cols-2 gap-6">
        {/* Model Breakdown */}
        <div className="bg-gray-900 border border-gray-800 rounded-xl p-5">
          <h3 className="text-sm font-medium text-white flex items-center gap-2 mb-4">
            <Cpu className="w-4 h-4 text-purple-400" />
            模型用量分布
          </h3>
          {models.length === 0 ? (
            <div className="text-center text-gray-600 text-sm py-6">暂无模型数据</div>
          ) : (
            <div className="space-y-3">
              {models.map((m, i) => {
                const maxModelTokens = Math.max(...models.map(x => x.total_tokens), 1)
                const pct = (m.total_tokens / maxModelTokens) * 100
                const colors = ['bg-overlord-500', 'bg-blue-500', 'bg-purple-500', 'bg-cyan-500', 'bg-amber-500', 'bg-emerald-500']
                return (
                  <div key={m.model_name}>
                    <div className="flex items-center justify-between mb-1">
                      <span className="text-sm text-white font-medium">{m.model_name || '(unknown)'}</span>
                      <div className="flex items-center gap-3 text-xs text-gray-500">
                        <span>{formatTokens(m.total_tokens)} tokens</span>
                        <span>{m.total_requests.toLocaleString()} 次</span>
                        <span className="flex items-center gap-1"><Clock className="w-3 h-3" />{m.avg_latency_ms}ms</span>
                      </div>
                    </div>
                    <div className="h-2 bg-gray-800 rounded-full overflow-hidden">
                      <div className={`h-full rounded-full transition-all ${colors[i % colors.length]}`} style={{ width: `${pct}%` }} />
                    </div>
                  </div>
                )
              })}
            </div>
          )}
        </div>

        {/* User Ranking */}
        <div className="bg-gray-900 border border-gray-800 rounded-xl p-5">
          <h3 className="text-sm font-medium text-white flex items-center gap-2 mb-4">
            <Users className="w-4 h-4 text-cyan-400" />
            用户用量排行
          </h3>
          {users.length === 0 ? (
            <div className="text-center text-gray-600 text-sm py-6">暂无用户数据</div>
          ) : (
            <div className="space-y-1">
              {users.map((u, i) => (
                <div key={u.user_id} className="flex items-center gap-3 px-3 py-2 rounded-lg hover:bg-gray-800/50 transition">
                  <span className={`w-5 text-center text-xs font-bold ${i < 3 ? 'text-overlord-400' : 'text-gray-600'}`}>
                    {i + 1}
                  </span>
                  <div className="flex-1 min-w-0">
                    <span className="text-sm text-white truncate block">{u.user_id || '(anonymous)'}</span>
                  </div>
                  <div className="flex items-center gap-4 text-xs text-gray-500 shrink-0">
                    <span>{formatTokens(u.total_tokens)} tokens</span>
                    <span>{u.total_requests.toLocaleString()} 次</span>
                    <span className="text-emerald-400">{formatCents(u.total_cost_cents)}</span>
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
