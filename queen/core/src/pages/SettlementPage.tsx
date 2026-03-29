import { useEffect, useState } from 'react'
import { api } from '../api'
import { Calculator, CheckCircle, XCircle, Banknote, FileText, ArrowLeft, Settings, Save } from 'lucide-react'

interface ProfitConfig {
  city_comm_rate: number
  team_direct_rate: number
  team_mgmt_rate: number
  investor_pool_rate: number
  upstream_cost_pct: number
}

interface SettlementBill {
  id: string
  partner_id: string
  partner_type: string
  partner_name: string
  month: string
  total_amount: number
  salary_amount: number
  direct_amount: number
  manage_amount: number
  city_amount: number
  item_count: number
  status: string
  reviewed_by: string
  reviewed_at: string | null
  review_note: string
  paid_at: string | null
  pay_method: string
  pay_ref: string
  created_at: string
}

interface LineItem {
  id: string
  source_type: string
  client_name: string
  base_amount: number
  rate: number
  amount: number
  description: string
}

interface SettlementStats {
  monthly: { month: string; total_amount: number; bill_count: number; paid_count: number }[]
  by_status: { status: string; count: number; amount: number }[]
  total_paid: number
  pending_amount: number
}

const STATUS_MAP: Record<string, { label: string; color: string }> = {
  draft: { label: '草稿', color: 'text-gray-400 bg-gray-500/10' },
  pending_review: { label: '待审批', color: 'text-amber-400 bg-amber-500/10' },
  approved: { label: '已审批', color: 'text-blue-400 bg-blue-500/10' },
  paid: { label: '已打款', color: 'text-green-400 bg-green-500/10' },
  rejected: { label: '已驳回', color: 'text-red-400 bg-red-500/10' },
}

function fen2yuan(fen: number): string {
  return `¥${(fen / 100).toLocaleString('zh-CN', { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`
}

export default function SettlementPage() {
  const [stats, setStats] = useState<SettlementStats | null>(null)
  const [bills, setBills] = useState<SettlementBill[]>([])
  const [monthFilter, setMonthFilter] = useState('')
  const [statusFilter, setStatusFilter] = useState('')
  const [generating, setGenerating] = useState(false)
  const [genMonth, setGenMonth] = useState(() => {
    const d = new Date()
    return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}`
  })
  const [detail, setDetail] = useState<{ bill: SettlementBill; items: LineItem[] } | null>(null)
  const [loading, setLoading] = useState(true)
  const [showConfig, setShowConfig] = useState(false)
  const [profitCfg, setProfitCfg] = useState<ProfitConfig | null>(null)
  const [saving, setSaving] = useState(false)
  const [cfgMsg, setCfgMsg] = useState('')

  useEffect(() => { loadStats(); loadBills(); loadProfitConfig() }, [monthFilter, statusFilter])

  const loadProfitConfig = () => {
    api.get<ProfitConfig>('/v1/admin/settlement/profit-config').then(setProfitCfg).catch(() => {})
  }

  const saveProfitConfig = async () => {
    if (!profitCfg) return
    setSaving(true)
    setCfgMsg('')
    try {
      await api.put('/v1/admin/settlement/profit-config', profitCfg)
      setCfgMsg('已保存')
      setTimeout(() => setCfgMsg(''), 2000)
    } catch (e: any) {
      setCfgMsg(e.message || '保存失败')
    }
    setSaving(false)
  }

  const loadStats = () => {
    api.get<SettlementStats>('/v1/admin/settlement/stats').then(setStats).catch(() => {})
  }

  const loadBills = () => {
    const params = new URLSearchParams()
    if (monthFilter) params.set('month', monthFilter)
    if (statusFilter) params.set('status', statusFilter)
    api.get<{ bills: SettlementBill[] }>(`/v1/admin/settlement/bills?${params}`)
      .then(r => setBills(r.bills || []))
      .catch(() => {})
      .finally(() => setLoading(false))
  }

  const handleGenerate = async () => {
    if (!genMonth) return
    setGenerating(true)
    try {
      await api.post('/v1/admin/settlement/generate', { month: genMonth })
      loadBills()
      loadStats()
    } catch {}
    setGenerating(false)
  }

  const handleApprove = async (id: string) => {
    await api.post(`/v1/admin/settlement/bills/${id}/approve`, {})
    loadBills()
    loadStats()
    if (detail?.bill.id === id) loadDetail(id)
  }

  const handleReject = async (id: string) => {
    await api.post(`/v1/admin/settlement/bills/${id}/reject`, { note: '管理员驳回' })
    loadBills()
    loadStats()
    if (detail?.bill.id === id) loadDetail(id)
  }

  const handlePay = async (id: string) => {
    await api.post(`/v1/admin/settlement/bills/${id}/pay`, { pay_method: 'bank' })
    loadBills()
    loadStats()
    if (detail?.bill.id === id) loadDetail(id)
  }

  const handleDelete = async (id: string) => {
    await api.delete(`/v1/admin/settlement/bills/${id}`)
    loadBills()
    loadStats()
    if (detail?.bill.id === id) setDetail(null)
  }

  const loadDetail = async (id: string) => {
    const r = await api.get<{ bill: SettlementBill; items: LineItem[] }>(`/v1/admin/settlement/bills/${id}`)
    setDetail(r)
  }

  if (loading) return <div className="text-gray-500 text-center py-20">加载中...</div>

  // Detail view
  if (detail) {
    const { bill, items } = detail
    const st = STATUS_MAP[bill.status] || STATUS_MAP.draft
    return (
      <div>
        <button onClick={() => setDetail(null)} className="flex items-center gap-1.5 text-xs text-gray-400 hover:text-white mb-4">
          <ArrowLeft size={14} /> 返回列表
        </button>
        <div className="flex items-center justify-between mb-6">
          <div>
            <h2 className="text-xl font-bold text-white">{bill.partner_name}</h2>
            <div className="flex items-center gap-3 mt-1 text-sm text-gray-400">
              <span>{bill.month}</span>
              <span className={`px-1.5 py-0.5 rounded text-xs ${st.color}`}>{st.label}</span>
              <span>{bill.partner_type === 'core' ? '团队合伙人' : '城市合伙人'}</span>
            </div>
          </div>
          <div className="text-right">
            <div className="text-2xl font-bold text-white">{fen2yuan(bill.total_amount)}</div>
            <div className="text-xs text-gray-500 mt-1">{bill.item_count} 项明细</div>
          </div>
        </div>

        {/* Amount breakdown */}
        <div className="grid grid-cols-4 gap-4 mb-6">
          {bill.salary_amount > 0 && (
            <div className="bg-gray-900 border border-gray-800 rounded-xl p-4">
              <div className="text-xs text-gray-500 mb-1">底薪</div>
              <div className="text-lg font-bold text-white">{fen2yuan(bill.salary_amount)}</div>
            </div>
          )}
          {bill.direct_amount > 0 && (
            <div className="bg-gray-900 border border-gray-800 rounded-xl p-4">
              <div className="text-xs text-gray-500 mb-1">直签佣金</div>
              <div className="text-lg font-bold text-green-400">{fen2yuan(bill.direct_amount)}</div>
            </div>
          )}
          {bill.manage_amount > 0 && (
            <div className="bg-gray-900 border border-gray-800 rounded-xl p-4">
              <div className="text-xs text-gray-500 mb-1">管理费</div>
              <div className="text-lg font-bold text-purple-400">{fen2yuan(bill.manage_amount)}</div>
            </div>
          )}
          {bill.city_amount > 0 && (
            <div className="bg-gray-900 border border-gray-800 rounded-xl p-4">
              <div className="text-xs text-gray-500 mb-1">城市佣金</div>
              <div className="text-lg font-bold text-blue-400">{fen2yuan(bill.city_amount)}</div>
            </div>
          )}
        </div>

        {/* Actions */}
        <div className="flex gap-2 mb-6">
          {(bill.status === 'draft' || bill.status === 'pending_review') && (
            <>
              <button onClick={() => handleApprove(bill.id)}
                className="flex items-center gap-1.5 px-4 py-2 bg-green-600 text-white text-xs rounded-lg hover:bg-green-500">
                <CheckCircle size={14} /> 审批通过
              </button>
              <button onClick={() => handleReject(bill.id)}
                className="flex items-center gap-1.5 px-4 py-2 bg-red-600/20 text-red-400 text-xs rounded-lg hover:bg-red-600/30">
                <XCircle size={14} /> 驳回
              </button>
            </>
          )}
          {bill.status === 'approved' && (
            <button onClick={() => handlePay(bill.id)}
              className="flex items-center gap-1.5 px-4 py-2 bg-purple-600 text-white text-xs rounded-lg hover:bg-purple-500">
              <Banknote size={14} /> 标记打款
            </button>
          )}
          {bill.status === 'draft' && (
            <button onClick={() => handleDelete(bill.id)}
              className="flex items-center gap-1.5 px-4 py-2 bg-gray-800 text-gray-400 text-xs rounded-lg hover:bg-gray-700">
              删除
            </button>
          )}
        </div>

        {/* Line items */}
        <div className="bg-gray-900 border border-gray-800 rounded-xl overflow-hidden">
          <table className="w-full text-xs">
            <thead>
              <tr className="border-b border-gray-800 text-gray-400 text-left">
                <th className="px-4 py-3 font-medium">类型</th>
                <th className="px-4 py-3 font-medium">描述</th>
                <th className="px-4 py-3 font-medium">客户</th>
                <th className="px-4 py-3 font-medium text-right">基数</th>
                <th className="px-4 py-3 font-medium text-right">费率</th>
                <th className="px-4 py-3 font-medium text-right">金额</th>
              </tr>
            </thead>
            <tbody>
              {items.map(item => (
                <tr key={item.id} className="border-b border-gray-800/50">
                  <td className="px-4 py-3 text-gray-400">{item.source_type}</td>
                  <td className="px-4 py-3 text-white">{item.description}</td>
                  <td className="px-4 py-3 text-gray-400">{item.client_name || '-'}</td>
                  <td className="px-4 py-3 text-right text-gray-400">{item.base_amount ? fen2yuan(item.base_amount) : '-'}</td>
                  <td className="px-4 py-3 text-right text-gray-400">{item.rate ? `${(item.rate * 100).toFixed(0)}%` : '-'}</td>
                  <td className="px-4 py-3 text-right text-green-400 font-medium">{fen2yuan(item.amount)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>
    )
  }

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <h2 className="text-xl font-bold">结算管理</h2>
        <button onClick={() => setShowConfig(!showConfig)}
          className={`flex items-center gap-1.5 px-3 py-1.5 text-xs rounded-lg transition-colors ${showConfig ? 'bg-purple-600/20 text-purple-400' : 'bg-gray-800 text-gray-400 hover:bg-gray-700'}`}>
          <Settings size={14} /> 利润分配配置
        </button>
      </div>

      {/* Profit Config Panel */}
      {showConfig && profitCfg && (
        <div className="bg-gray-900 border border-gray-800 rounded-xl p-5 mb-6">
          <div className="flex items-center justify-between mb-4">
            <h3 className="text-sm font-bold text-white">利润分配比例</h3>
            <div className="flex items-center gap-2">
              {cfgMsg && <span className={`text-xs ${cfgMsg === '已保存' ? 'text-green-400' : 'text-red-400'}`}>{cfgMsg}</span>}
              <button onClick={saveProfitConfig} disabled={saving}
                className="flex items-center gap-1.5 px-3 py-1.5 bg-purple-600 text-white text-xs rounded-lg hover:bg-purple-500 disabled:opacity-50">
                <Save size={13} /> {saving ? '保存中...' : '保存'}
              </button>
            </div>
          </div>
          <p className="text-xs text-gray-500 mb-4">调整全局默认比例。个别合伙人如有自定义费率，将优先使用自定义值。</p>
          <div className="grid grid-cols-5 gap-4">
            {([
              { key: 'city_comm_rate' as const, label: '城市合伙人佣金', desc: '消费利润 × 比例', color: 'text-blue-400' },
              { key: 'team_direct_rate' as const, label: '团队直签佣金', desc: '消费利润 × 比例', color: 'text-green-400' },
              { key: 'team_mgmt_rate' as const, label: '团队管理费', desc: '城市佣金 × 比例', color: 'text-purple-400' },
              { key: 'investor_pool_rate' as const, label: '投资人池', desc: '消费利润 × 比例', color: 'text-amber-400' },
              { key: 'upstream_cost_pct' as const, label: '上游成本估算', desc: '充值额 × 比例', color: 'text-gray-400' },
            ]).map(item => (
              <div key={item.key} className="bg-gray-800/50 rounded-lg p-3">
                <div className={`text-xs font-medium mb-1 ${item.color}`}>{item.label}</div>
                <div className="text-[10px] text-gray-500 mb-2">{item.desc}</div>
                <div className="flex items-center gap-1">
                  <input
                    type="number" min="0" max="100" step="1"
                    value={Math.round(profitCfg[item.key] * 100)}
                    onChange={e => setProfitCfg({ ...profitCfg, [item.key]: Number(e.target.value) / 100 })}
                    className="w-16 bg-gray-900 border border-gray-700 rounded px-2 py-1 text-sm text-white text-right"
                  />
                  <span className="text-xs text-gray-500">%</span>
                </div>
              </div>
            ))}
          </div>
          <div className="mt-4 p-3 bg-gray-800/30 rounded-lg">
            <div className="text-[10px] text-gray-500 mb-1">利润分配示例（假设消费利润 ¥100）</div>
            <div className="flex gap-4 text-xs">
              <span className="text-blue-400">城市佣金 ¥{(profitCfg.city_comm_rate * 100).toFixed(0)}</span>
              <span className="text-green-400">直签佣金 ¥{(profitCfg.team_direct_rate * 100).toFixed(0)}</span>
              <span className="text-purple-400">管理费 ¥{(profitCfg.city_comm_rate * profitCfg.team_mgmt_rate * 100).toFixed(1)}</span>
              <span className="text-amber-400">投资池 ¥{(profitCfg.investor_pool_rate * 100).toFixed(0)}</span>
            </div>
          </div>
        </div>
      )}

      {/* Stats cards */}
      {stats && (
        <div className="grid grid-cols-4 gap-4 mb-6">
          <div className="bg-gray-900 border border-gray-800 rounded-xl p-4">
            <div className="text-xs text-gray-500 mb-2">累计结算</div>
            <div className="text-2xl font-bold text-green-400">{fen2yuan(stats.total_paid)}</div>
          </div>
          <div className="bg-gray-900 border border-gray-800 rounded-xl p-4">
            <div className="text-xs text-gray-500 mb-2">待处理</div>
            <div className="text-2xl font-bold text-amber-400">{fen2yuan(stats.pending_amount)}</div>
          </div>
          <div className="bg-gray-900 border border-gray-800 rounded-xl p-4">
            <div className="text-xs text-gray-500 mb-2">本期账单</div>
            <div className="text-2xl font-bold text-white">{bills.length}</div>
          </div>
          <div className="bg-gray-900 border border-gray-800 rounded-xl p-4">
            <div className="text-xs text-gray-500 mb-2">月度趋势</div>
            <div className="flex items-end gap-1 h-8 mt-1">
              {(stats.monthly || []).map(m => {
                const max = Math.max(...(stats.monthly || []).map(x => x.total_amount), 1)
                const h = Math.max((m.total_amount / max) * 100, 8)
                return (
                  <div key={m.month} className="flex-1 flex flex-col items-center">
                    <div className="w-full rounded-t bg-purple-500/40" style={{ height: `${h}%` }} />
                  </div>
                )
              })}
            </div>
          </div>
        </div>
      )}

      {/* Generate + Filters */}
      <div className="flex items-center justify-between mb-4">
        <div className="flex items-center gap-3">
          <input type="month" value={monthFilter} onChange={e => setMonthFilter(e.target.value)}
            className="bg-gray-800 border border-gray-700 rounded-lg px-3 py-1.5 text-xs text-white" />
          <select value={statusFilter} onChange={e => setStatusFilter(e.target.value)}
            className="bg-gray-800 border border-gray-700 rounded-lg px-3 py-1.5 text-xs text-white">
            <option value="">全部状态</option>
            {Object.entries(STATUS_MAP).map(([k, v]) => <option key={k} value={k}>{v.label}</option>)}
          </select>
        </div>
        <div className="flex items-center gap-2">
          <input type="month" value={genMonth} onChange={e => setGenMonth(e.target.value)}
            className="bg-gray-800 border border-gray-700 rounded-lg px-3 py-1.5 text-xs text-white" />
          <button onClick={handleGenerate} disabled={generating}
            className="flex items-center gap-1.5 px-4 py-1.5 bg-purple-600 text-white text-xs rounded-lg hover:bg-purple-500 disabled:opacity-50">
            <Calculator size={14} /> {generating ? '生成中...' : '生成账单'}
          </button>
        </div>
      </div>

      {/* Bills table */}
      <div className="bg-gray-900 border border-gray-800 rounded-xl overflow-hidden">
        <table className="w-full text-xs">
          <thead>
            <tr className="border-b border-gray-800 text-gray-400 text-left">
              <th className="px-4 py-3 font-medium">月份</th>
              <th className="px-4 py-3 font-medium">合伙人</th>
              <th className="px-4 py-3 font-medium">类型</th>
              <th className="px-4 py-3 font-medium">状态</th>
              <th className="px-4 py-3 font-medium text-right">金额</th>
              <th className="px-4 py-3 font-medium text-right">明细</th>
              <th className="px-4 py-3 font-medium">操作</th>
            </tr>
          </thead>
          <tbody>
            {bills.map(b => {
              const st = STATUS_MAP[b.status] || STATUS_MAP.draft
              return (
                <tr key={b.id} className="border-b border-gray-800/50 hover:bg-gray-800/30">
                  <td className="px-4 py-3 text-white font-mono">{b.month}</td>
                  <td className="px-4 py-3 text-white">{b.partner_name}</td>
                  <td className="px-4 py-3">
                    <span className={`px-1.5 py-0.5 rounded ${b.partner_type === 'core' ? 'text-purple-400 bg-purple-500/10' : 'text-blue-400 bg-blue-500/10'}`}>
                      {b.partner_type === 'core' ? '核心' : '城市'}
                    </span>
                  </td>
                  <td className="px-4 py-3"><span className={`px-1.5 py-0.5 rounded ${st.color}`}>{st.label}</span></td>
                  <td className="px-4 py-3 text-right text-green-400 font-medium">{fen2yuan(b.total_amount)}</td>
                  <td className="px-4 py-3 text-right text-gray-400">{b.item_count} 项</td>
                  <td className="px-4 py-3">
                    <button onClick={() => loadDetail(b.id)}
                      className="text-purple-400 hover:text-purple-300 flex items-center gap-1">
                      <FileText size={13} /> 详情
                    </button>
                  </td>
                </tr>
              )
            })}
            {bills.length === 0 && (
              <tr><td colSpan={7} className="px-4 py-8 text-center text-gray-600">暂无结算账单</td></tr>
            )}
          </tbody>
        </table>
      </div>
    </div>
  )
}
