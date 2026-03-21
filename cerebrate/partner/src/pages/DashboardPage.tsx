import { useEffect, useState } from 'react'
import { TrendingUp, Users, Coins, Briefcase, AlertTriangle, MapPin } from 'lucide-react'
import { partner } from '../lib/api'

export default function DashboardPage() {
  const [data, setData] = useState<Awaited<ReturnType<typeof partner.dashboard>> | null>(null)

  useEffect(() => {
    partner.dashboard().then(setData).catch(console.error)
  }, [])

  if (!data) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="animate-spin w-8 h-8 border-2 border-claw-500 border-t-transparent rounded-full" />
      </div>
    )
  }

  const fmt = (cents: number) => `¥${(cents / 100).toLocaleString('zh-CN', { minimumFractionDigits: 2 })}`

  const cards = [
    { label: '本月佣金', value: fmt(data.month_commission), icon: Coins, color: 'text-green-400 bg-green-500/10' },
    { label: '累计佣金', value: fmt(data.partner.total_commission), icon: TrendingUp, color: 'text-amber-400 bg-amber-500/10' },
    { label: '活跃客户', value: String(data.partner.active_clients), icon: Users, color: 'text-blue-400 bg-blue-500/10' },
    { label: '待处理事项', value: String(data.urgent_actions), icon: AlertTriangle, color: 'text-red-400 bg-red-500/10' },
    { label: '城市合伙人', value: String(data.city_partners), icon: MapPin, color: 'text-purple-400 bg-purple-500/10' },
    { label: '累计营收', value: fmt(data.partner.total_revenue), icon: Briefcase, color: 'text-cyan-400 bg-cyan-500/10' },
  ]

  const STAGE_ORDER = ['lead', 'opportunity', 'negotiation', 'signed', 'delivery', 'active', 'renewal', 'churned']
  const STAGE_LABELS: Record<string, string> = {
    lead: '线索', opportunity: '商机', negotiation: '谈判', signed: '签约',
    delivery: '交付', active: '活跃', renewal: '续费', churned: '流失',
  }

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold text-white">个人大盘</h1>
        <p className="text-sm text-gray-400 mt-1">
          {data.partner.name} · {data.partner.level} · {data.partner.region || '全国'} · {data.month}
        </p>
      </div>

      {/* Stats */}
      <div className="grid grid-cols-2 lg:grid-cols-3 gap-4">
        {cards.map(c => (
          <div key={c.label} className="rounded-xl border border-white/10 bg-white/[0.02] p-5">
            <div className="flex items-center gap-3 mb-3">
              <div className={`w-9 h-9 rounded-lg flex items-center justify-center ${c.color}`}>
                <c.icon size={18} />
              </div>
              <span className="text-sm text-gray-400">{c.label}</span>
            </div>
            <div className="text-2xl font-bold text-white">{c.value}</div>
          </div>
        ))}
      </div>

      {/* Pipeline funnel */}
      {data.funnel && data.funnel.length > 0 && (
        <div className="rounded-xl border border-white/10 bg-white/[0.02] p-5">
          <h3 className="text-sm font-medium text-white mb-4">管道漏斗</h3>
          <div className="flex items-end gap-3 h-28">
            {STAGE_ORDER.map(stage => {
              const item = data.funnel.find(f => f.stage === stage)
              const maxCount = Math.max(...data.funnel.map(f => f.count), 1)
              const h = item ? Math.max((item.count / maxCount) * 100, 8) : 4
              return (
                <div key={stage} className="flex-1 flex flex-col items-center gap-1">
                  {item && <span className="text-[10px] text-gray-400">{item.count}</span>}
                  <div
                    className={`w-full rounded-t transition-all ${item && item.count > 0 ? 'bg-claw-500/60' : 'bg-white/5'}`}
                    style={{ height: `${h}%` }}
                  />
                  <span className="text-[10px] text-gray-500">{STAGE_LABELS[stage] || stage}</span>
                </div>
              )
            })}
          </div>
        </div>
      )}

      {/* Equity snapshot */}
      {data.equity && data.equity.id && (
        <div className="rounded-xl border border-white/10 bg-white/[0.02] p-5">
          <h3 className="text-sm font-medium text-white mb-3">期权概况</h3>
          <div className="grid grid-cols-3 gap-4 text-sm">
            <div>
              <span className="text-gray-500">总授予:</span>{' '}
              <span className="text-white font-medium">{data.equity.total_shares.toLocaleString()} 股</span>
            </div>
            <div>
              <span className="text-gray-500">已归属:</span>{' '}
              <span className="text-green-400 font-medium">{data.equity.vested_shares.toLocaleString()} 股</span>
            </div>
            <div>
              <span className="text-gray-500">估值:</span>{' '}
              <span className="text-amber-400 font-medium">
                ¥{(data.equity.vested_shares * data.equity.current_value).toLocaleString()}
              </span>
            </div>
          </div>
          {/* Vesting progress bar */}
          <div className="mt-3">
            <div className="h-2 bg-white/5 rounded-full overflow-hidden">
              <div
                className="h-full bg-gradient-to-r from-claw-600 to-claw-400 rounded-full transition-all"
                style={{ width: `${Math.min((data.equity.vested_shares / data.equity.total_shares) * 100, 100)}%` }}
              />
            </div>
            <div className="flex justify-between text-[10px] text-gray-500 mt-1">
              <span>Cliff: {new Date(data.equity.cliff_date).toLocaleDateString()}</span>
              <span>Full vest: {new Date(data.equity.full_vest_date).toLocaleDateString()}</span>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
