import { useEffect, useState } from 'react'
import { Zap, ArrowUpRight, ArrowDownRight, TrendingUp } from 'lucide-react'
import { PieChart, Pie, Cell, BarChart, Bar, XAxis, YAxis, Tooltip, ResponsiveContainer } from 'recharts'
import { overseerAPI, type EnergyData } from '../lib/api'

const HP_COLORS: Record<string, string> = {
  full: '#22c55e',
  healthy: '#3b82f6',
  low: '#eab308',
  critical: '#f97316',
  hibernated: '#6b7280',
}

const TX_COLORS: Record<string, string> = {
  grant: '#22c55e',
  consume: '#f97316',
  transfer: '#8b5cf6',
  mining_reward: '#06b6d4',
  bounty: '#eab308',
  freeze: '#6366f1',
  unfreeze: '#14b8a6',
  settle: '#ec4899',
}

export default function EnergyPage() {
  const [data, setData] = useState<EnergyData | null>(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    overseerAPI.energy().then(setData).catch(() => {}).finally(() => setLoading(false))
  }, [])

  if (loading) return <div className="p-6 text-gray-500">加载中...</div>
  if (!data) return <div className="p-6 text-red-400">加载失败</div>

  const hpData = (data.hp_distribution || []).map((b) => ({
    name: b.status,
    value: b.count,
  }))

  const typeData = (data.type_stats || []).map((t) => ({
    name: t.type,
    count: t.count,
    total: Math.round(t.total / 10000),
  }))

  return (
    <div className="p-6 space-y-6">
      <h1 className="text-xl font-bold text-white flex items-center gap-2">
        <Zap className="w-5 h-5 text-yellow-400" />
        星能经济
      </h1>

      {/* HP Distribution + TX Type Chart */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
        {/* HP Pie */}
        <div className="rounded-xl border border-gray-800 bg-gray-900/50 p-4">
          <h2 className="text-sm text-gray-400 mb-3">HP 分布</h2>
          {hpData.length > 0 ? (
            <div className="flex items-center gap-4">
              <ResponsiveContainer width="50%" height={180}>
                <PieChart>
                  <Pie data={hpData} cx="50%" cy="50%" innerRadius={40} outerRadius={70} dataKey="value">
                    {hpData.map((entry) => (
                      <Cell key={entry.name} fill={HP_COLORS[entry.name] || '#6b7280'} />
                    ))}
                  </Pie>
                  <Tooltip
                    contentStyle={{ background: '#1f2937', border: '1px solid #374151', borderRadius: 8, fontSize: 12 }}
                    itemStyle={{ color: '#e5e7eb' }}
                  />
                </PieChart>
              </ResponsiveContainer>
              <div className="space-y-1">
                {hpData.map((d) => (
                  <div key={d.name} className="flex items-center gap-2 text-xs">
                    <div className="w-2 h-2 rounded-full" style={{ background: HP_COLORS[d.name] || '#6b7280' }} />
                    <span className="text-gray-400 w-20">{d.name}</span>
                    <span className="text-white font-medium">{d.value}</span>
                  </div>
                ))}
              </div>
            </div>
          ) : (
            <div className="text-gray-600 text-sm">暂无数据</div>
          )}
        </div>

        {/* TX Type Bar */}
        <div className="rounded-xl border border-gray-800 bg-gray-900/50 p-4">
          <h2 className="text-sm text-gray-400 mb-3">交易类型分布 (⚡)</h2>
          {typeData.length > 0 ? (
            <ResponsiveContainer width="100%" height={180}>
              <BarChart data={typeData} layout="vertical">
                <XAxis type="number" tick={{ fill: '#6b7280', fontSize: 10 }} />
                <YAxis type="category" dataKey="name" tick={{ fill: '#9ca3af', fontSize: 11 }} width={80} />
                <Tooltip
                  contentStyle={{ background: '#1f2937', border: '1px solid #374151', borderRadius: 8, fontSize: 12 }}
                  itemStyle={{ color: '#e5e7eb' }}
                  formatter={(v) => [`${v} ⚡`, '总量']}
                />
                <Bar dataKey="total" radius={[0, 4, 4, 0]}>
                  {typeData.map((entry) => (
                    <Cell key={entry.name} fill={TX_COLORS[entry.name] || '#8b5cf6'} />
                  ))}
                </Bar>
              </BarChart>
            </ResponsiveContainer>
          ) : (
            <div className="text-gray-600 text-sm">暂无数据</div>
          )}
        </div>
      </div>

      {/* Top accounts */}
      <div className="rounded-xl border border-gray-800 bg-gray-900/50 p-4">
        <h2 className="text-sm text-gray-400 mb-3 flex items-center gap-2">
          <TrendingUp className="w-4 h-4" />
          Top 20 账户
        </h2>
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="text-xs text-gray-500 border-b border-gray-800">
                <th className="text-left py-2 px-3">#</th>
                <th className="text-left py-2 px-3">Claw ID</th>
                <th className="text-right py-2 px-3">余额 ⚡</th>
                <th className="text-right py-2 px-3">冻结</th>
                <th className="text-right py-2 px-3">累计收入</th>
                <th className="text-right py-2 px-3">累计支出</th>
                <th className="text-left py-2 px-3">状态</th>
              </tr>
            </thead>
            <tbody>
              {(data.top_accounts || []).map((acc, i) => (
                <tr key={acc.id} className="border-b border-gray-800/50 hover:bg-gray-800/30">
                  <td className="py-2 px-3 text-gray-500">{i + 1}</td>
                  <td className="py-2 px-3 font-mono text-xs text-gray-300">
                    {acc.claw_id ? `${acc.claw_id.slice(0, 16)}...` : '—'}
                  </td>
                  <td className="py-2 px-3 text-right text-yellow-400 font-medium">
                    {(acc.balance / 10000).toFixed(1)}
                  </td>
                  <td className="py-2 px-3 text-right text-gray-500">
                    {acc.frozen > 0 ? (acc.frozen / 10000).toFixed(1) : '—'}
                  </td>
                  <td className="py-2 px-3 text-right text-green-400 text-xs flex items-center justify-end gap-1">
                    <ArrowDownRight className="w-3 h-3" />
                    {(acc.total_in / 10000).toFixed(1)}
                  </td>
                  <td className="py-2 px-3 text-right text-orange-400 text-xs">
                    <span className="flex items-center justify-end gap-1">
                      <ArrowUpRight className="w-3 h-3" />
                      {(acc.total_out / 10000).toFixed(1)}
                    </span>
                  </td>
                  <td className="py-2 px-3">
                    <span className={`px-1.5 py-0.5 rounded text-xs ${
                      acc.status === 'active' ? 'bg-green-500/15 text-green-400' : 'bg-gray-700 text-gray-400'
                    }`}>
                      {acc.status}
                    </span>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>

      {/* Recent transactions */}
      <div className="rounded-xl border border-gray-800 bg-gray-900/50 p-4">
        <h2 className="text-sm text-gray-400 mb-3">最近交易</h2>
        <div className="space-y-1">
          {(data.recent_tx || []).slice(0, 20).map((tx) => (
            <div key={tx.id} className="flex items-center gap-3 text-xs py-1.5 border-b border-gray-800/30">
              <span className={`px-1.5 py-0.5 rounded font-medium ${
                tx.type === 'grant' ? 'bg-green-500/15 text-green-400' :
                tx.type === 'consume' ? 'bg-orange-500/15 text-orange-400' :
                tx.type === 'transfer' ? 'bg-purple-500/15 text-purple-400' :
                'bg-gray-700 text-gray-400'
              }`}>
                {tx.type}
              </span>
              <span className="text-gray-500 font-mono">{tx.from_claw?.slice(0, 10) || 'system'}</span>
              <span className="text-gray-600">→</span>
              <span className="text-gray-500 font-mono">{tx.to_claw?.slice(0, 10) || '—'}</span>
              <span className="ml-auto text-yellow-400 font-medium">{(tx.amount / 10000).toFixed(2)} ⚡</span>
              <span className="text-gray-600 w-24 text-right">
                {new Date(tx.created_at).toLocaleString('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' })}
              </span>
            </div>
          ))}
        </div>
      </div>
    </div>
  )
}
