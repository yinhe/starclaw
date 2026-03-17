import { useEffect, useState } from 'react'
import { api } from '../api'
import { Users, TrendingUp, Award, MapPin, Plus, X } from 'lucide-react'

interface PartnerPerf {
  id: string
  name: string
  type: string
  region: string
  status: string
  total_revenue: number
  total_commission: number
  active_clients: number
  deal_count: number
  level: string
  comm_rate: number
  claw_id?: string
}

const LEVEL_MAP: Record<string, { label: string; color: string }> = {
  partner: { label: '合伙人', color: 'text-gray-400 bg-gray-500/10' },
  senior: { label: '高级合伙人', color: 'text-blue-400 bg-blue-500/10' },
  director: { label: '合伙人总监', color: 'text-purple-400 bg-purple-500/10' },
}

const STATUS_MAP: Record<string, { label: string; color: string }> = {
  active: { label: '活跃', color: 'text-green-400' },
  approved: { label: '已审批', color: 'text-green-400' },
  pending: { label: '待审核', color: 'text-amber-400' },
  suspended: { label: '暂停', color: 'text-red-400' },
  rejected: { label: '已拒绝', color: 'text-gray-500' },
  terminated: { label: '已终止', color: 'text-gray-500' },
}

function fen2yuan(fen: number): string {
  return `¥${(fen / 100).toLocaleString('zh-CN', { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`
}

export default function PartnersPage() {
  const [partners, setPartners] = useState<PartnerPerf[]>([])
  const [typeFilter, setTypeFilter] = useState<string>('')
  const [loading, setLoading] = useState(true)
  const [showAdd, setShowAdd] = useState(false)
  const [addForm, setAddForm] = useState({ claw_id: '', name: '', region: '', email: '', phone: '' })
  const [addError, setAddError] = useState('')
  const [addLoading, setAddLoading] = useState(false)

  const loadPartners = () => {
    api.get<{ partners: PartnerPerf[] }>('/v1/admin/partners/performance')
      .then(r => setPartners(r.partners || []))
      .catch(() => {})
      .finally(() => setLoading(false))
  }

  useEffect(() => { loadPartners() }, [])

  const handleAddPartner = async () => {
    if (!addForm.claw_id || !addForm.name) { setAddError('Claw 地址和姓名为必填'); return }
    setAddError('')
    setAddLoading(true)
    try {
      await api.post('/v1/admin/partners', addForm)
      setShowAdd(false)
      setAddForm({ claw_id: '', name: '', region: '', email: '', phone: '' })
      loadPartners()
    } catch (err) {
      setAddError(err instanceof Error ? err.message : '添加失败')
    } finally {
      setAddLoading(false)
    }
  }

  const filtered = partners.filter(p => !typeFilter || p.type === typeFilter)
  const coreCount = partners.filter(p => p.type === 'core').length
  const cityCount = partners.filter(p => p.type === 'city').length
  const totalComm = partners.reduce((s, p) => s + p.total_commission, 0)
  const totalRev = partners.filter(p => p.type === 'core').reduce((s, p) => s + p.total_revenue, 0)

  if (loading) return <div className="text-gray-500 text-center py-20">加载中...</div>

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <h2 className="text-xl font-bold">合伙人管理</h2>
        <button onClick={() => setShowAdd(true)}
          className="flex items-center gap-1.5 px-3 py-1.5 text-xs rounded-lg bg-purple-600 text-white hover:bg-purple-500 transition-colors">
          <Plus size={14} /> 添加核心合伙人
        </button>
      </div>

      {/* Add Core Partner Modal */}
      {showAdd && (
        <div className="fixed inset-0 bg-black/60 flex items-center justify-center z-50 px-4">
          <div className="bg-gray-900 border border-gray-800 rounded-xl p-6 w-full max-w-md space-y-4">
            <div className="flex items-center justify-between">
              <h3 className="text-lg font-semibold text-white">添加核心合伙人</h3>
              <button onClick={() => setShowAdd(false)} className="text-gray-500 hover:text-gray-300"><X size={18} /></button>
            </div>
            {addError && <div className="bg-red-500/10 border border-red-500/20 text-red-400 text-sm px-3 py-2 rounded-lg">{addError}</div>}
            <div>
              <label className="block text-xs text-gray-400 mb-1">Claw 地址 *</label>
              <input value={addForm.claw_id} onChange={e => setAddForm({ ...addForm, claw_id: e.target.value })}
                className="w-full bg-gray-800 border border-gray-700 rounded-lg px-3 py-2 text-sm text-white focus:outline-none focus:border-purple-500"
                placeholder="claw:xxxxxxxxxxxxxxxx" />
            </div>
            <div>
              <label className="block text-xs text-gray-400 mb-1">姓名 *</label>
              <input value={addForm.name} onChange={e => setAddForm({ ...addForm, name: e.target.value })}
                className="w-full bg-gray-800 border border-gray-700 rounded-lg px-3 py-2 text-sm text-white focus:outline-none focus:border-purple-500"
                placeholder="合伙人姓名" />
            </div>
            <div className="grid grid-cols-2 gap-3">
              <div>
                <label className="block text-xs text-gray-400 mb-1">区域</label>
                <input value={addForm.region} onChange={e => setAddForm({ ...addForm, region: e.target.value })}
                  className="w-full bg-gray-800 border border-gray-700 rounded-lg px-3 py-2 text-sm text-white focus:outline-none focus:border-purple-500"
                  placeholder="华东" />
              </div>
              <div>
                <label className="block text-xs text-gray-400 mb-1">手机号</label>
                <input value={addForm.phone} onChange={e => setAddForm({ ...addForm, phone: e.target.value })}
                  className="w-full bg-gray-800 border border-gray-700 rounded-lg px-3 py-2 text-sm text-white focus:outline-none focus:border-purple-500"
                  placeholder="13800138000" />
              </div>
            </div>
            <div>
              <label className="block text-xs text-gray-400 mb-1">邮箱</label>
              <input value={addForm.email} onChange={e => setAddForm({ ...addForm, email: e.target.value })}
                className="w-full bg-gray-800 border border-gray-700 rounded-lg px-3 py-2 text-sm text-white focus:outline-none focus:border-purple-500"
                placeholder="partner@example.com" />
            </div>
            <button onClick={handleAddPartner} disabled={addLoading}
              className="w-full bg-purple-600 hover:bg-purple-500 disabled:opacity-50 text-white rounded-lg py-2.5 text-sm font-medium transition-colors">
              {addLoading ? '添加中...' : '添加'}
            </button>
          </div>
        </div>
      )}

      {/* Stats */}
      <div className="grid grid-cols-4 gap-4 mb-6">
        <div className="bg-gray-900 border border-gray-800 rounded-xl p-4">
          <div className="flex items-center gap-2 text-gray-500 text-xs mb-2"><Users size={14} /> 核心合伙人</div>
          <div className="text-2xl font-bold text-white">{coreCount}</div>
        </div>
        <div className="bg-gray-900 border border-gray-800 rounded-xl p-4">
          <div className="flex items-center gap-2 text-gray-500 text-xs mb-2"><MapPin size={14} /> 城市合伙人</div>
          <div className="text-2xl font-bold text-white">{cityCount}</div>
        </div>
        <div className="bg-gray-900 border border-gray-800 rounded-xl p-4">
          <div className="flex items-center gap-2 text-gray-500 text-xs mb-2"><TrendingUp size={14} /> 累计 GMV</div>
          <div className="text-2xl font-bold text-green-400">{fen2yuan(totalRev)}</div>
        </div>
        <div className="bg-gray-900 border border-gray-800 rounded-xl p-4">
          <div className="flex items-center gap-2 text-gray-500 text-xs mb-2"><Award size={14} /> 累计佣金</div>
          <div className="text-2xl font-bold text-purple-400">{fen2yuan(totalComm)}</div>
        </div>
      </div>

      {/* Filters */}
      <div className="flex items-center gap-3 mb-4">
        {[
          { value: '', label: '全部' },
          { value: 'core', label: '核心合伙人' },
          { value: 'city', label: '城市合伙人' },
        ].map(f => (
          <button key={f.value} onClick={() => setTypeFilter(f.value)}
            className={`px-3 py-1.5 text-xs rounded-lg transition-colors ${
              typeFilter === f.value ? 'bg-purple-600/20 text-purple-400' : 'bg-gray-800 text-gray-400 hover:bg-gray-700'
            }`}>
            {f.label}
          </button>
        ))}
        <span className="text-xs text-gray-600 ml-2">共 {filtered.length} 人</span>
      </div>

      {/* Table */}
      <div className="bg-gray-900 border border-gray-800 rounded-xl overflow-hidden">
        <table className="w-full text-xs">
          <thead>
            <tr className="border-b border-gray-800 text-gray-400 text-left">
              <th className="px-4 py-3 font-medium">姓名</th>
              <th className="px-4 py-3 font-medium">Claw 地址</th>
              <th className="px-4 py-3 font-medium">类型</th>
              <th className="px-4 py-3 font-medium">等级</th>
              <th className="px-4 py-3 font-medium">区域</th>
              <th className="px-4 py-3 font-medium">状态</th>
              <th className="px-4 py-3 font-medium text-right">GMV</th>
              <th className="px-4 py-3 font-medium text-right">佣金</th>
              <th className="px-4 py-3 font-medium text-right">客户</th>
              <th className="px-4 py-3 font-medium text-right">佣金率</th>
            </tr>
          </thead>
          <tbody>
            {filtered.map(p => {
              const st = STATUS_MAP[p.status] || STATUS_MAP.active
              const lv = LEVEL_MAP[p.level] || { label: p.level || '-', color: 'text-gray-400' }
              return (
                <tr key={p.id} className="border-b border-gray-800/50 hover:bg-gray-800/30">
                  <td className="px-4 py-3 text-white font-medium">{p.name}</td>
                  <td className="px-4 py-3 text-gray-500 font-mono text-[10px]">{p.claw_id ? (p.claw_id.length > 20 ? p.claw_id.slice(0, 20) + '…' : p.claw_id) : '-'}</td>
                  <td className="px-4 py-3">
                    <span className={`px-1.5 py-0.5 rounded ${p.type === 'core' ? 'text-purple-400 bg-purple-500/10' : 'text-blue-400 bg-blue-500/10'}`}>
                      {p.type === 'core' ? '核心' : '城市'}
                    </span>
                  </td>
                  <td className="px-4 py-3"><span className={`px-1.5 py-0.5 rounded ${lv.color}`}>{lv.label}</span></td>
                  <td className="px-4 py-3 text-gray-400">{p.region || '-'}</td>
                  <td className="px-4 py-3"><span className={st.color}>{st.label}</span></td>
                  <td className="px-4 py-3 text-right text-gray-300">{p.total_revenue > 0 ? fen2yuan(p.total_revenue) : '-'}</td>
                  <td className="px-4 py-3 text-right text-green-400">{fen2yuan(p.total_commission)}</td>
                  <td className="px-4 py-3 text-right text-gray-300">{p.active_clients}</td>
                  <td className="px-4 py-3 text-right text-gray-400">{(p.comm_rate * 100).toFixed(0)}%</td>
                </tr>
              )
            })}
            {filtered.length === 0 && (
              <tr><td colSpan={10} className="px-4 py-8 text-center text-gray-600">暂无合伙人数据</td></tr>
            )}
          </tbody>
        </table>
      </div>
    </div>
  )
}
