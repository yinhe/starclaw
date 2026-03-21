import { useEffect, useState } from 'react'
import { Coins } from 'lucide-react'
import { city, type CommissionItem, type MonthlySummary, type PayoutItem } from '../lib/api'

export default function CommissionsPage() {
  const [commissions, setCommissions] = useState<CommissionItem[]>([])
  const [summaries, setSummaries] = useState<MonthlySummary[]>([])
  const [payouts, setPayouts] = useState<PayoutItem[]>([])
  const [month, setMonth] = useState('')

  useEffect(() => {
    city.listCommissions(month || undefined).then(r => {
      setCommissions(r.commissions || [])
      setSummaries(r.monthly_summary || [])
    }).catch(console.error)
    city.listPayouts().then(r => setPayouts(r.payouts || [])).catch(console.error)
  }, [month])

  const fmt = (cents: number) => `¥${(cents / 100).toLocaleString('zh-CN', { minimumFractionDigits: 2 })}`

  const statusLabel: Record<string, { text: string; color: string }> = {
    pending: { text: '待审核', color: 'text-yellow-400 bg-yellow-500/10' },
    approved: { text: '已审核', color: 'text-blue-400 bg-blue-500/10' },
    paid: { text: '已支付', color: 'text-green-400 bg-green-500/10' },
    rejected: { text: '已拒绝', color: 'text-red-400 bg-red-500/10' },
  }

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold text-white">我的佣金</h1>

      {/* Monthly summary chart */}
      {summaries.length > 0 && (
        <div className="rounded-xl border border-white/10 bg-white/[0.02] p-5">
          <h3 className="text-sm font-medium text-white mb-4">月度趋势</h3>
          <div className="flex items-end gap-2 h-32">
            {summaries.slice(0, 6).reverse().map(s => {
              const max = Math.max(...summaries.map(x => x.total), 1)
              const h = Math.max((s.total / max) * 100, 4)
              return (
                <div key={s.month} className="flex-1 flex flex-col items-center gap-1">
                  <span className="text-[10px] text-gray-400">{fmt(s.total)}</span>
                  <div
                    className="w-full rounded-t bg-claw-500/60 transition-all"
                    style={{ height: `${h}%` }}
                  />
                  <span className="text-[10px] text-gray-500">{s.month.slice(5)}</span>
                </div>
              )
            })}
          </div>
        </div>
      )}

      {/* Filter by month */}
      <div className="flex items-center gap-3">
        <span className="text-sm text-gray-400">筛选月份:</span>
        <select
          value={month}
          onChange={e => setMonth(e.target.value)}
          className="bg-gray-900 border border-white/10 rounded-lg px-3 py-1.5 text-sm text-white focus:outline-none focus:border-claw-500"
        >
          <option value="">全部</option>
          {summaries.map(s => (
            <option key={s.month} value={s.month}>{s.month}</option>
          ))}
        </select>
      </div>

      {/* Commission table */}
      {commissions.length === 0 ? (
        <div className="rounded-xl border border-white/10 border-dashed p-12 text-center">
          <Coins className="w-10 h-10 text-gray-600 mx-auto mb-3" />
          <p className="text-gray-500">暂无佣金记录</p>
        </div>
      ) : (
        <div className="rounded-xl border border-white/10 overflow-hidden">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-white/10 text-gray-400 text-left">
                <th className="px-4 py-3 font-medium">月份</th>
                <th className="px-4 py-3 font-medium">类型</th>
                <th className="px-4 py-3 font-medium text-right">订单金额</th>
                <th className="px-4 py-3 font-medium text-right">比例</th>
                <th className="px-4 py-3 font-medium text-right">佣金</th>
                <th className="px-4 py-3 font-medium">状态</th>
              </tr>
            </thead>
            <tbody>
              {commissions.map(c => {
                const st = statusLabel[c.status] || { text: c.status, color: 'text-gray-400' }
                return (
                  <tr key={c.id} className="border-b border-white/5 hover:bg-white/[0.02]">
                    <td className="px-4 py-3 text-gray-400">{c.month}</td>
                    <td className="px-4 py-3 text-white">{c.type}</td>
                    <td className="px-4 py-3 text-gray-300 text-right">{fmt(c.base_amount)}</td>
                    <td className="px-4 py-3 text-gray-400 text-right">{(c.rate * 100).toFixed(0)}%</td>
                    <td className="px-4 py-3 text-green-400 text-right font-medium">{fmt(c.amount)}</td>
                    <td className="px-4 py-3">
                      <span className={`text-xs px-2 py-0.5 rounded ${st.color}`}>{st.text}</span>
                    </td>
                  </tr>
                )
              })}
            </tbody>
          </table>
        </div>
      )}

      {/* Payouts */}
      {payouts.length > 0 && (
        <div>
          <h2 className="text-lg font-bold text-white mb-4">提现记录</h2>
          <div className="rounded-xl border border-white/10 overflow-hidden">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-white/10 text-gray-400 text-left">
                  <th className="px-4 py-3 font-medium">月份</th>
                  <th className="px-4 py-3 font-medium text-right">金额</th>
                  <th className="px-4 py-3 font-medium">方式</th>
                  <th className="px-4 py-3 font-medium">状态</th>
                  <th className="px-4 py-3 font-medium">支付时间</th>
                </tr>
              </thead>
              <tbody>
                {payouts.map(p => (
                  <tr key={p.id} className="border-b border-white/5">
                    <td className="px-4 py-3 text-gray-400">{p.month}</td>
                    <td className="px-4 py-3 text-white text-right font-medium">{fmt(p.amount)}</td>
                    <td className="px-4 py-3 text-gray-400">{p.method}</td>
                    <td className="px-4 py-3">
                      <span className={`text-xs px-2 py-0.5 rounded ${
                        p.status === 'completed' ? 'text-green-400 bg-green-500/10' : 'text-yellow-400 bg-yellow-500/10'
                      }`}>
                        {p.status === 'completed' ? '已完成' : p.status === 'processing' ? '处理中' : '待处理'}
                      </span>
                    </td>
                    <td className="px-4 py-3 text-gray-500">{p.paid_at ? new Date(p.paid_at).toLocaleDateString() : '-'}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}
    </div>
  )
}
