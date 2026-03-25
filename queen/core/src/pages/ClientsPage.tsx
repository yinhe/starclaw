import { useEffect, useState } from 'react'
import { api } from '../api'
import { Building2, Search } from 'lucide-react'

interface ClientRow {
  id: string
  name: string
  source: string
  partner_name: string
  plan: string
  stage: string
  mrr: number
  deal_value: number
  signed_at: string | null
  renew_at: string | null
  created_at: string
}

const STAGE_MAP: Record<string, { label: string; color: string }> = {
  lead: { label: '线索', color: 'text-gray-400 bg-gray-500/10' },
  trial: { label: '试用', color: 'text-blue-400 bg-blue-500/10' },
  opportunity: { label: '商机', color: 'text-cyan-400 bg-cyan-500/10' },
  negotiation: { label: '谈判', color: 'text-amber-400 bg-amber-500/10' },
  signed: { label: '已签约', color: 'text-green-400 bg-green-500/10' },
  delivery: { label: '交付中', color: 'text-purple-400 bg-purple-500/10' },
  active: { label: '活跃', color: 'text-emerald-400 bg-emerald-500/10' },
  renewal: { label: '续费', color: 'text-blue-400 bg-blue-500/10' },
  churned: { label: '流失', color: 'text-red-400 bg-red-500/10' },
}

const PLAN_MAP: Record<string, string> = {
  starter: 'Starter', pro: 'Pro', enterprise: 'Enterprise',
  unlimited: 'Unlimited', whitelabel: 'White-Label', community: 'Community',
}

function fen2yuan(fen: number): string {
  if (!fen) return '-'
  return `¥${(fen / 100).toLocaleString('zh-CN', { minimumFractionDigits: 0, maximumFractionDigits: 0 })}`
}

export default function ClientsPage() {
  const [clients, setClients] = useState<ClientRow[]>([])
  const [loading, setLoading] = useState(true)
  const [search, setSearch] = useState('')
  const [stageFilter, setStageFilter] = useState('')
  const [sourceFilter, setSourceFilter] = useState('')

  useEffect(() => {
    api.get<{ clients: ClientRow[] }>('/v1/admin/clients')
      .then(r => setClients(r.clients || []))
      .catch(() => {})
      .finally(() => setLoading(false))
  }, [])

  const filtered = clients.filter(c => {
    if (search && !c.name.toLowerCase().includes(search.toLowerCase()) && !c.partner_name?.toLowerCase().includes(search.toLowerCase())) return false
    if (stageFilter && c.stage !== stageFilter) return false
    if (sourceFilter && c.source !== sourceFilter) return false
    return true
  })

  const activeCount = clients.filter(c => c.stage === 'active').length
  const totalMRR = clients.filter(c => c.stage === 'active').reduce((s, c) => s + (c.mrr || 0), 0)
  const totalDealValue = clients.reduce((s, c) => s + (c.deal_value || 0), 0)

  if (loading) return <div className="text-gray-500 text-center py-20">加载中...</div>

  return (
    <div>
      <h2 className="text-xl font-bold mb-6">客户总览</h2>

      {/* Stats */}
      <div className="grid grid-cols-4 gap-4 mb-6">
        <div className="bg-gray-900 border border-gray-800 rounded-xl p-4">
          <div className="text-xs text-gray-500 mb-2">总客户</div>
          <div className="text-2xl font-bold text-white">{clients.length}</div>
        </div>
        <div className="bg-gray-900 border border-gray-800 rounded-xl p-4">
          <div className="text-xs text-gray-500 mb-2">活跃客户</div>
          <div className="text-2xl font-bold text-green-400">{activeCount}</div>
        </div>
        <div className="bg-gray-900 border border-gray-800 rounded-xl p-4">
          <div className="text-xs text-gray-500 mb-2">月度 MRR</div>
          <div className="text-2xl font-bold text-blue-400">{fen2yuan(totalMRR)}</div>
        </div>
        <div className="bg-gray-900 border border-gray-800 rounded-xl p-4">
          <div className="text-xs text-gray-500 mb-2">合同总额</div>
          <div className="text-2xl font-bold text-purple-400">{fen2yuan(totalDealValue)}</div>
        </div>
      </div>

      {/* Filters */}
      <div className="flex items-center gap-3 mb-4 flex-wrap">
        <div className="relative">
          <Search size={14} className="absolute left-3 top-1/2 -translate-y-1/2 text-gray-500" />
          <input value={search} onChange={e => setSearch(e.target.value)} placeholder="搜索客户 / 合伙人"
            className="pl-8 pr-3 py-1.5 text-xs rounded-lg bg-gray-800 border border-gray-700 text-white focus:outline-none focus:border-purple-500 w-48" />
        </div>
        <select value={stageFilter} onChange={e => setStageFilter(e.target.value)}
          className="bg-gray-800 border border-gray-700 rounded-lg px-3 py-1.5 text-xs text-white">
          <option value="">全部阶段</option>
          {Object.entries(STAGE_MAP).map(([k, v]) => <option key={k} value={k}>{v.label}</option>)}
        </select>
        <select value={sourceFilter} onChange={e => setSourceFilter(e.target.value)}
          className="bg-gray-800 border border-gray-700 rounded-lg px-3 py-1.5 text-xs text-white">
          <option value="">全部来源</option>
          <option value="crm">团队合伙人 CRM</option>
          <option value="city">城市合伙人</option>
        </select>
        <span className="text-xs text-gray-600 ml-2">共 {filtered.length} 条</span>
      </div>

      {/* Table */}
      <div className="bg-gray-900 border border-gray-800 rounded-xl overflow-hidden">
        <table className="w-full text-xs">
          <thead>
            <tr className="border-b border-gray-800 text-gray-400 text-left">
              <th className="px-4 py-3 font-medium">客户名称</th>
              <th className="px-4 py-3 font-medium">来源</th>
              <th className="px-4 py-3 font-medium">合伙人</th>
              <th className="px-4 py-3 font-medium">套餐</th>
              <th className="px-4 py-3 font-medium">阶段</th>
              <th className="px-4 py-3 font-medium text-right">合同金额</th>
              <th className="px-4 py-3 font-medium text-right">MRR</th>
              <th className="px-4 py-3 font-medium">签约日期</th>
              <th className="px-4 py-3 font-medium">续费日期</th>
            </tr>
          </thead>
          <tbody>
            {filtered.map(c => {
              const st = STAGE_MAP[c.stage] || { label: c.stage, color: 'text-gray-400 bg-gray-500/10' }
              return (
                <tr key={c.id} className="border-b border-gray-800/50 hover:bg-gray-800/30">
                  <td className="px-4 py-3">
                    <div className="flex items-center gap-2">
                      <Building2 size={13} className="text-gray-600" />
                      <span className="text-white">{c.name}</span>
                    </div>
                  </td>
                  <td className="px-4 py-3">
                    <span className={`px-1.5 py-0.5 rounded ${c.source === 'crm' ? 'text-purple-400 bg-purple-500/10' : 'text-blue-400 bg-blue-500/10'}`}>
                      {c.source === 'crm' ? 'CRM' : '城市'}
                    </span>
                  </td>
                  <td className="px-4 py-3 text-gray-400">{c.partner_name || '-'}</td>
                  <td className="px-4 py-3 text-gray-300">{PLAN_MAP[c.plan] || c.plan || '-'}</td>
                  <td className="px-4 py-3"><span className={`px-1.5 py-0.5 rounded ${st.color}`}>{st.label}</span></td>
                  <td className="px-4 py-3 text-right text-gray-300">{fen2yuan(c.deal_value)}</td>
                  <td className="px-4 py-3 text-right text-blue-400">{c.mrr ? fen2yuan(c.mrr) : '-'}</td>
                  <td className="px-4 py-3 text-gray-500">{c.signed_at ? new Date(c.signed_at).toLocaleDateString() : '-'}</td>
                  <td className="px-4 py-3 text-gray-500">{c.renew_at ? new Date(c.renew_at).toLocaleDateString() : '-'}</td>
                </tr>
              )
            })}
            {filtered.length === 0 && (
              <tr><td colSpan={9} className="px-4 py-8 text-center text-gray-600">暂无客户数据</td></tr>
            )}
          </tbody>
        </table>
      </div>
    </div>
  )
}
