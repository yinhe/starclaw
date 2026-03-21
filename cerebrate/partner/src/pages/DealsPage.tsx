import { useEffect, useState } from 'react'
import { Plus, Search } from 'lucide-react'
import { partner, type CRMDeal } from '../lib/api'

const STAGES = ['', 'lead', 'opportunity', 'negotiation', 'signed', 'delivery', 'active', 'renewal', 'churned'] as const
const STAGE_LABELS: Record<string, { text: string; color: string }> = {
  lead: { text: '线索', color: 'text-gray-400 bg-gray-500/10' },
  opportunity: { text: '商机', color: 'text-blue-400 bg-blue-500/10' },
  negotiation: { text: '谈判', color: 'text-amber-400 bg-amber-500/10' },
  signed: { text: '签约', color: 'text-green-400 bg-green-500/10' },
  delivery: { text: '交付', color: 'text-purple-400 bg-purple-500/10' },
  active: { text: '活跃', color: 'text-cyan-400 bg-cyan-500/10' },
  renewal: { text: '续费', color: 'text-orange-400 bg-orange-500/10' },
  churned: { text: '流失', color: 'text-red-400 bg-red-500/10' },
}

const NEXT_STAGE: Record<string, string> = {
  lead: 'opportunity', opportunity: 'negotiation', negotiation: 'signed',
  signed: 'delivery', delivery: 'active', active: 'renewal',
}

export default function DealsPage() {
  const [deals, setDeals] = useState<CRMDeal[]>([])
  const [stage, setStage] = useState('')
  const [search, setSearch] = useState('')
  const [showAdd, setShowAdd] = useState(false)
  const [form, setForm] = useState({ company_name: '', contact_name: '', contact_info: '', industry: '', deal_value: 0, plan: '', source: '' })

  const load = () => {
    partner.listDeals({ stage: stage || undefined, q: search || undefined }).then(r => setDeals(r.deals || [])).catch(console.error)
  }
  useEffect(() => { load() }, [stage, search])

  const handleAdd = async () => {
    if (!form.company_name) return
    await partner.createDeal(form)
    setForm({ company_name: '', contact_name: '', contact_info: '', industry: '', deal_value: 0, plan: '', source: '' })
    setShowAdd(false)
    load()
  }

  const advance = async (id: string, currentStage: string) => {
    const next = NEXT_STAGE[currentStage]
    if (!next) return
    await partner.updateDeal(id, { stage: next } as Partial<CRMDeal>)
    load()
  }

  const fmt = (cents: number) => cents > 0 ? `¥${(cents / 100).toLocaleString()}` : '-'

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold text-white">客户 CRM</h1>
        <button onClick={() => setShowAdd(!showAdd)}
          className="inline-flex items-center gap-2 rounded-lg bg-claw-600 px-4 py-2 text-sm font-medium text-white hover:bg-claw-500">
          <Plus size={16} /> 新增商机
        </button>
      </div>

      {showAdd && (
        <div className="rounded-xl border border-white/10 bg-white/[0.02] p-5 space-y-3">
          <div className="grid grid-cols-2 gap-3">
            <input placeholder="公司名称 *" value={form.company_name} onChange={e => setForm({ ...form, company_name: e.target.value })}
              className="rounded-lg border border-white/10 bg-gray-900 px-3 py-2 text-sm text-white focus:outline-none focus:border-claw-500" />
            <input placeholder="联系人" value={form.contact_name} onChange={e => setForm({ ...form, contact_name: e.target.value })}
              className="rounded-lg border border-white/10 bg-gray-900 px-3 py-2 text-sm text-white focus:outline-none focus:border-claw-500" />
            <input placeholder="联系方式" value={form.contact_info} onChange={e => setForm({ ...form, contact_info: e.target.value })}
              className="rounded-lg border border-white/10 bg-gray-900 px-3 py-2 text-sm text-white focus:outline-none focus:border-claw-500" />
            <input placeholder="行业" value={form.industry} onChange={e => setForm({ ...form, industry: e.target.value })}
              className="rounded-lg border border-white/10 bg-gray-900 px-3 py-2 text-sm text-white focus:outline-none focus:border-claw-500" />
          </div>
          <button onClick={handleAdd} className="rounded-lg bg-claw-600 px-4 py-2 text-sm text-white hover:bg-claw-500">确认添加</button>
        </div>
      )}

      {/* Search + filter */}
      <div className="flex items-center gap-3">
        <div className="relative flex-1 max-w-xs">
          <Search size={14} className="absolute left-3 top-1/2 -translate-y-1/2 text-gray-500" />
          <input placeholder="搜索公司/联系人" value={search} onChange={e => setSearch(e.target.value)}
            className="w-full rounded-lg border border-white/10 bg-gray-900 pl-9 pr-3 py-2 text-sm text-white focus:outline-none focus:border-claw-500" />
        </div>
        <div className="flex gap-1.5 flex-wrap">
          {STAGES.map(s => (
            <button key={s} onClick={() => setStage(s)}
              className={`px-2.5 py-1 rounded text-xs transition-colors ${stage === s ? 'bg-claw-500/10 text-claw-400' : 'text-gray-400 hover:text-white hover:bg-white/5'}`}>
              {s === '' ? '全部' : STAGE_LABELS[s]?.text || s}
            </button>
          ))}
        </div>
      </div>

      {/* Deals table */}
      <div className="rounded-xl border border-white/10 overflow-hidden">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-white/10 text-gray-400 text-left">
              <th className="px-4 py-3 font-medium">公司</th>
              <th className="px-4 py-3 font-medium">联系人</th>
              <th className="px-4 py-3 font-medium">阶段</th>
              <th className="px-4 py-3 font-medium text-right">预期年值</th>
              <th className="px-4 py-3 font-medium">套餐</th>
              <th className="px-4 py-3 font-medium">下一步</th>
              <th className="px-4 py-3 font-medium">操作</th>
            </tr>
          </thead>
          <tbody>
            {deals.map(d => {
              const st = STAGE_LABELS[d.stage] || { text: d.stage, color: 'text-gray-400 bg-gray-500/10' }
              return (
                <tr key={d.id} className="border-b border-white/5 hover:bg-white/[0.02]">
                  <td className="px-4 py-3 text-white font-medium">{d.company_name}</td>
                  <td className="px-4 py-3 text-gray-400">{d.contact_name || '-'}</td>
                  <td className="px-4 py-3"><span className={`text-xs px-2 py-0.5 rounded ${st.color}`}>{st.text}</span></td>
                  <td className="px-4 py-3 text-gray-300 text-right">{fmt(d.deal_value)}</td>
                  <td className="px-4 py-3 text-gray-400">{d.plan || '-'}</td>
                  <td className="px-4 py-3 text-gray-500 text-xs max-w-[150px] truncate">{d.next_action || '-'}</td>
                  <td className="px-4 py-3">
                    {NEXT_STAGE[d.stage] && (
                      <button onClick={() => advance(d.id, d.stage)}
                        className="text-xs text-claw-400 hover:text-claw-300">
                        → {STAGE_LABELS[NEXT_STAGE[d.stage]]?.text}
                      </button>
                    )}
                  </td>
                </tr>
              )
            })}
            {deals.length === 0 && (
              <tr><td colSpan={7} className="px-4 py-12 text-center text-gray-500">暂无商机</td></tr>
            )}
          </tbody>
        </table>
      </div>
    </div>
  )
}
