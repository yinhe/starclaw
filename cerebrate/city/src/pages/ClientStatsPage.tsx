import { useEffect, useState } from 'react'
import { Activity, Zap, TrendingUp, Wallet, Users, BarChart3 } from 'lucide-react'
import { city, type ClientStat } from '../lib/api'

const STATUS_LABELS: Record<string, { label: string; color: string }> = {
  lead: { label: '线索', color: 'text-gray-400 bg-gray-500/10' },
  trial: { label: '试用', color: 'text-blue-400 bg-blue-500/10' },
  active: { label: '活跃', color: 'text-green-400 bg-green-500/10' },
  churned: { label: '流失', color: 'text-red-400 bg-red-500/10' },
}

export default function ClientStatsPage() {
  const [data, setData] = useState<{
    clients: ClientStat[]
    total_clients: number
    total_recharge: number
    month_recharge: number
    total_energy: number
    month_energy: number
  } | null>(null)
  const [error, setError] = useState('')

  useEffect(() => {
    city.clientStats().then(setData).catch(e => setError(e.message))
  }, [])

  if (error) {
    return (
      <div className="rounded-xl border border-red-500/20 bg-red-500/5 p-8 text-center">
        <p className="text-red-400">{error}</p>
      </div>
    )
  }

  if (!data) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="animate-spin w-8 h-8 border-2 border-claw-500 border-t-transparent rounded-full" />
      </div>
    )
  }

  const fmtCNY = (cents: number) => `¥${(cents / 100).toLocaleString('zh-CN', { minimumFractionDigits: 2 })}`
  const fmtEnergy = (units: number) => {
    if (units >= 10000) return `${(units / 10000).toFixed(1)} ⭐`
    return `${units.toLocaleString()} ⚡`
  }

  const cards = [
    { label: '客户总数', value: String(data.total_clients), icon: Users, color: 'text-blue-400 bg-blue-500/10' },
    { label: '累计充值', value: fmtCNY(data.total_recharge), icon: Wallet, color: 'text-green-400 bg-green-500/10' },
    { label: '本月充值', value: fmtCNY(data.month_recharge), icon: TrendingUp, color: 'text-emerald-400 bg-emerald-500/10' },
    { label: '累计消耗', value: fmtEnergy(data.total_energy), icon: Zap, color: 'text-amber-400 bg-amber-500/10' },
    { label: '本月消耗', value: fmtEnergy(data.month_energy), icon: Activity, color: 'text-orange-400 bg-orange-500/10' },
  ]

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold text-white">客户消费统计</h1>
        <p className="text-sm text-gray-400 mt-1">下游用户充值、星能消耗和活跃度概览</p>
      </div>

      {/* Summary cards */}
      <div className="grid grid-cols-2 lg:grid-cols-5 gap-4">
        {cards.map(c => (
          <div key={c.label} className="rounded-xl border border-white/10 bg-white/[0.02] p-4">
            <div className="flex items-center gap-2 mb-2">
              <div className={`w-8 h-8 rounded-lg flex items-center justify-center ${c.color}`}>
                <c.icon size={16} />
              </div>
              <span className="text-xs text-gray-400">{c.label}</span>
            </div>
            <div className="text-xl font-bold text-white">{c.value}</div>
          </div>
        ))}
      </div>

      {/* Client detail table */}
      {data.clients.length === 0 ? (
        <div className="rounded-xl border border-white/10 border-dashed p-12 text-center">
          <BarChart3 className="w-10 h-10 text-gray-600 mx-auto mb-3" />
          <p className="text-gray-500">暂无客户消费数据</p>
        </div>
      ) : (
        <div className="rounded-xl border border-white/10 overflow-hidden">
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-white/10 text-gray-400 text-left">
                  <th className="px-4 py-3 font-medium">客户</th>
                  <th className="px-4 py-3 font-medium">状态</th>
                  <th className="px-4 py-3 font-medium text-right">累计充值</th>
                  <th className="px-4 py-3 font-medium text-right">本月充值</th>
                  <th className="px-4 py-3 font-medium text-right">累计消耗</th>
                  <th className="px-4 py-3 font-medium text-right">本月消耗</th>
                  <th className="px-4 py-3 font-medium text-right">星能余额</th>
                  <th className="px-4 py-3 font-medium">最近活跃</th>
                </tr>
              </thead>
              <tbody>
                {data.clients.map(cl => {
                  const st = STATUS_LABELS[cl.status] || { label: cl.status, color: 'text-gray-400 bg-gray-500/10' }
                  return (
                    <tr key={cl.id} className="border-b border-white/5 hover:bg-white/[0.02]">
                      <td className="px-4 py-3">
                        <div className="text-white font-medium">{cl.client_name}</div>
                        {cl.user_id && <div className="text-[10px] text-gray-500 font-mono">{cl.user_id.slice(0, 8)}...</div>}
                      </td>
                      <td className="px-4 py-3">
                        <span className={`text-xs px-2 py-0.5 rounded ${st.color}`}>{st.label}</span>
                      </td>
                      <td className="px-4 py-3 text-right text-gray-300">{cl.total_recharge > 0 ? fmtCNY(cl.total_recharge) : '-'}</td>
                      <td className="px-4 py-3 text-right text-gray-300">{cl.month_recharge > 0 ? fmtCNY(cl.month_recharge) : '-'}</td>
                      <td className="px-4 py-3 text-right text-amber-400">{cl.total_energy > 0 ? fmtEnergy(cl.total_energy) : '-'}</td>
                      <td className="px-4 py-3 text-right text-amber-400">{cl.month_energy > 0 ? fmtEnergy(cl.month_energy) : '-'}</td>
                      <td className="px-4 py-3 text-right text-emerald-400">{cl.energy_balance > 0 ? fmtEnergy(cl.energy_balance) : '-'}</td>
                      <td className="px-4 py-3 text-gray-500 text-xs">{cl.last_active || '-'}</td>
                    </tr>
                  )
                })}
              </tbody>
            </table>
          </div>
        </div>
      )}
    </div>
  )
}
