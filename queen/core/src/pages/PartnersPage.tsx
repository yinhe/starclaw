import { useEffect, useState } from 'react'
import { api } from '../api'
import { Users, TrendingUp, Award, MapPin } from 'lucide-react'

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

  useEffect(() => {
    api.get<{ partners: PartnerPerf[] }>('/v1/admin/partners/performance')
      .then(r => setPartners(r.partners || []))
      .catch(() => {})
      .finally(() => setLoading(false))
  }, [])

  const filtered = partners.filter(p => !typeFilter || p.type === typeFilter)
  const coreCount = partners.filter(p => p.type === 'core').length
  const cityCount = partners.filter(p => p.type === 'city').length
  const totalComm = partners.reduce((s, p) => s + p.total_commission, 0)
  const totalRev = partners.filter(p => p.type === 'core').reduce((s, p) => s + p.total_revenue, 0)

  if (loading) return <div className="text-gray-500 text-center py-20">加载中...</div>

  return (
    <div>
      <h2 className="text-xl font-bold mb-6">合伙人管理</h2>

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
              <tr><td colSpan={9} className="px-4 py-8 text-center text-gray-600">暂无合伙人数据</td></tr>
            )}
          </tbody>
        </table>
      </div>
    </div>
  )
}
