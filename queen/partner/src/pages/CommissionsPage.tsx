import { useEffect, useState } from 'react'
import { Coins } from 'lucide-react'
import { partner, type PartnerComm } from '../lib/api'

const TYPE_LABELS: Record<string, string> = {
  salary: '底薪', direct: '直签佣金', manage_fee: '管理费',
}

export default function CommissionsPage() {
  const [comms, setComms] = useState<PartnerComm[]>([])
  const [breakdown, setBreakdown] = useState<{ month: string; type: string; total: number }[]>([])
  const [month, setMonth] = useState('')
  const [type, setType] = useState('')

  useEffect(() => {
    partner.listCommissions({ month: month || undefined, type: type || undefined }).then(r => {
      setComms(r.commissions || [])
      setBreakdown(r.breakdown || [])
    }).catch(console.error)
  }, [month, type])

  const fmt = (cents: number) => `¥${(cents / 100).toLocaleString('zh-CN', { minimumFractionDigits: 2 })}`

  // Unique months from breakdown
  const months = [...new Set(breakdown.map(b => b.month))].sort().reverse()

  const statusColor: Record<string, string> = {
    pending: 'text-yellow-400 bg-yellow-500/10',
    approved: 'text-blue-400 bg-blue-500/10',
    paid: 'text-green-400 bg-green-500/10',
  }

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold text-white">佣金明细</h1>

      {/* Monthly breakdown by type */}
      {breakdown.length > 0 && (
        <div className="rounded-xl border border-white/10 bg-white/[0.02] p-5">
          <h3 className="text-sm font-medium text-white mb-4">双轨佣金趋势</h3>
          <div className="space-y-2">
            {months.slice(0, 6).map(m => {
              const items = breakdown.filter(b => b.month === m)
              const total = items.reduce((s, i) => s + i.total, 0)
              return (
                <div key={m} className="flex items-center gap-3">
                  <span className="text-xs text-gray-500 w-14">{m.slice(5)}</span>
                  <div className="flex-1 flex h-5 rounded overflow-hidden bg-white/5">
                    {items.map(i => {
                      const pct = total > 0 ? (i.total / total) * 100 : 0
                      const colors: Record<string, string> = {
                        salary: 'bg-blue-500/60', direct: 'bg-green-500/60', manage_fee: 'bg-amber-500/60',
                      }
                      return (
                        <div key={i.type} className={`${colors[i.type] || 'bg-gray-500/60'}`}
                          style={{ width: `${pct}%` }} title={`${TYPE_LABELS[i.type] || i.type}: ${fmt(i.total)}`} />
                      )
                    })}
                  </div>
                  <span className="text-xs text-gray-300 w-24 text-right">{fmt(total)}</span>
                </div>
              )
            })}
          </div>
          <div className="flex gap-4 mt-3">
            {Object.entries(TYPE_LABELS).map(([k, v]) => (
              <div key={k} className="flex items-center gap-1.5 text-[10px] text-gray-400">
                <div className={`w-2 h-2 rounded-sm ${k === 'salary' ? 'bg-blue-500/60' : k === 'direct' ? 'bg-green-500/60' : 'bg-amber-500/60'}`} />
                {v}
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Filters */}
      <div className="flex items-center gap-4">
        <select value={month} onChange={e => setMonth(e.target.value)}
          className="bg-gray-900 border border-white/10 rounded-lg px-3 py-1.5 text-sm text-white focus:outline-none focus:border-claw-500">
          <option value="">全部月份</option>
          {months.map(m => <option key={m} value={m}>{m}</option>)}
        </select>
        <div className="flex gap-1.5">
          {['', 'salary', 'direct', 'manage_fee'].map(t => (
            <button key={t} onClick={() => setType(t)}
              className={`px-2.5 py-1 rounded text-xs transition-colors ${type === t ? 'bg-claw-500/10 text-claw-400' : 'text-gray-400 hover:text-white hover:bg-white/5'}`}>
              {t === '' ? '全部' : TYPE_LABELS[t] || t}
            </button>
          ))}
        </div>
      </div>

      {/* Table */}
      {comms.length === 0 ? (
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
                <th className="px-4 py-3 font-medium text-right">基础金额</th>
                <th className="px-4 py-3 font-medium text-right">比例</th>
                <th className="px-4 py-3 font-medium text-right">佣金</th>
                <th className="px-4 py-3 font-medium">备注</th>
                <th className="px-4 py-3 font-medium">状态</th>
              </tr>
            </thead>
            <tbody>
              {comms.map(c => (
                <tr key={c.id} className="border-b border-white/5 hover:bg-white/[0.02]">
                  <td className="px-4 py-3 text-gray-400">{c.month}</td>
                  <td className="px-4 py-3 text-white">{TYPE_LABELS[c.type] || c.type}</td>
                  <td className="px-4 py-3 text-gray-300 text-right">{c.base_amount > 0 ? fmt(c.base_amount) : '-'}</td>
                  <td className="px-4 py-3 text-gray-400 text-right">{c.rate > 0 ? `${(c.rate * 100).toFixed(0)}%` : '-'}</td>
                  <td className="px-4 py-3 text-green-400 text-right font-medium">{fmt(c.amount)}</td>
                  <td className="px-4 py-3 text-gray-500 text-xs max-w-[200px] truncate">{c.remark || '-'}</td>
                  <td className="px-4 py-3">
                    <span className={`text-xs px-2 py-0.5 rounded ${statusColor[c.status] || 'text-gray-400 bg-gray-500/10'}`}>
                      {c.status === 'pending' ? '待审' : c.status === 'approved' ? '已审' : c.status === 'paid' ? '已付' : c.status}
                    </span>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}
