import { useEffect, useState } from 'react'
import { TrendingUp, Users, Coins, UserPlus, Copy, Check, Wallet, Zap } from 'lucide-react'
import { city } from '../lib/api'

export default function DashboardPage() {
  const [data, setData] = useState<Awaited<ReturnType<typeof city.dashboard>> | null>(null)
  const [copied, setCopied] = useState(false)

  useEffect(() => {
    city.dashboard().then(setData).catch(console.error)
  }, [])

  const copyRef = () => {
    if (!data) return
    navigator.clipboard.writeText(data.ref_url)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  if (!data) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="animate-spin w-8 h-8 border-2 border-claw-500 border-t-transparent rounded-full" />
      </div>
    )
  }

  const fmt = (cents: number) => `¥${(cents / 100).toLocaleString('zh-CN', { minimumFractionDigits: 2 })}`

  const fmtEnergy = (units: number) => units >= 10000 ? `${(units / 10000).toFixed(1)} ⭐` : `${units.toLocaleString()} ⚡`

  const cards = [
    { label: '本月佣金', value: fmt(data.month_commission), icon: TrendingUp, color: 'text-green-400 bg-green-500/10' },
    { label: '累计佣金', value: fmt(data.total_earned), icon: Coins, color: 'text-amber-400 bg-amber-500/10' },
    { label: '待结算', value: fmt(data.pending_commission), icon: Coins, color: 'text-blue-400 bg-blue-500/10' },
    { label: '下游本月充值', value: fmt(data.month_recharge || 0), icon: Wallet, color: 'text-emerald-400 bg-emerald-500/10' },
    { label: '下游累计充值', value: fmt(data.total_recharge || 0), icon: Wallet, color: 'text-teal-400 bg-teal-500/10' },
    { label: '本月星能消耗', value: fmtEnergy(data.month_energy || 0), icon: Zap, color: 'text-orange-400 bg-orange-500/10' },
    { label: '总客户数', value: String(data.total_clients), icon: Users, color: 'text-purple-400 bg-purple-500/10' },
    { label: '活跃客户', value: String(data.active_clients), icon: Users, color: 'text-claw-400 bg-claw-500/10' },
    { label: '本月新增', value: String(data.month_new_clients), icon: UserPlus, color: 'text-cyan-400 bg-cyan-500/10' },
  ]

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold text-white">我的大盘</h1>
        <p className="text-sm text-gray-400 mt-1">
          {data.partner.name} · {data.partner.city} · {data.month}
        </p>
      </div>

      {/* Stats cards */}
      <div className="grid grid-cols-2 lg:grid-cols-3 xl:grid-cols-3 gap-4">
        {cards.map(card => (
          <div key={card.label} className="rounded-xl border border-white/10 bg-white/[0.02] p-5">
            <div className="flex items-center gap-3 mb-3">
              <div className={`w-9 h-9 rounded-lg flex items-center justify-center ${card.color}`}>
                <card.icon size={18} />
              </div>
              <span className="text-sm text-gray-400">{card.label}</span>
            </div>
            <div className="text-2xl font-bold text-white">{card.value}</div>
          </div>
        ))}
      </div>

      {/* Referral link */}
      <div className="rounded-xl border border-white/10 bg-white/[0.02] p-5">
        <h3 className="text-sm font-medium text-white mb-3">推广链接</h3>
        <div className="flex items-center gap-3">
          <code className="flex-1 bg-gray-900 rounded-lg px-4 py-2.5 text-sm text-claw-400 overflow-x-auto">
            {data.ref_url}
          </code>
          <button
            onClick={copyRef}
            className="shrink-0 rounded-lg border border-white/10 px-3 py-2.5 text-sm text-gray-400 hover:text-white hover:bg-white/5 transition-colors"
          >
            {copied ? <Check size={16} className="text-green-400" /> : <Copy size={16} />}
          </button>
        </div>
        <p className="text-xs text-gray-500 mt-2">
          推荐码: {data.partner.ref_code} · 佣金比例: {(data.partner.comm_rate * 100).toFixed(0)}%
        </p>
      </div>

      {/* Partner info */}
      <div className="rounded-xl border border-white/10 bg-white/[0.02] p-5">
        <h3 className="text-sm font-medium text-white mb-3">合伙人信息</h3>
        <div className="grid grid-cols-2 gap-4 text-sm">
          <div><span className="text-gray-500">姓名:</span> <span className="text-white">{data.partner.name}</span></div>
          <div><span className="text-gray-500">公司:</span> <span className="text-white">{data.partner.company || '-'}</span></div>
          <div><span className="text-gray-500">城市:</span> <span className="text-white">{data.partner.city}</span></div>
          <div><span className="text-gray-500">电话:</span> <span className="text-white">{data.partner.phone}</span></div>
          <div><span className="text-gray-500">邮箱:</span> <span className="text-white">{data.partner.email}</span></div>
          <div><span className="text-gray-500">状态:</span> <span className="text-green-400">{data.partner.status}</span></div>
        </div>
      </div>
    </div>
  )
}
