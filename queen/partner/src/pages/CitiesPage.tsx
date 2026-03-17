import { useEffect, useState } from 'react'
import { MapPin, CheckCircle, XCircle } from 'lucide-react'
import { partner, type CityPartner } from '../lib/api'

const STATUS_MAP: Record<string, { text: string; color: string }> = {
  pending: { text: '待审核', color: 'text-yellow-400 bg-yellow-500/10' },
  approved: { text: '已通过', color: 'text-green-400 bg-green-500/10' },
  rejected: { text: '已拒绝', color: 'text-red-400 bg-red-500/10' },
  suspended: { text: '已暂停', color: 'text-gray-400 bg-gray-500/10' },
}

export default function CitiesPage() {
  const [cities, setCities] = useState<CityPartner[]>([])
  const [filter, setFilter] = useState('')

  const load = () => {
    partner.listCityPartners(filter || undefined).then(r => setCities(r.city_partners || [])).catch(console.error)
  }
  useEffect(() => { load() }, [filter])

  const review = async (id: string, status: string) => {
    await partner.reviewCityPartner(id, { status })
    load()
  }

  const fmt = (cents: number) => `¥${(cents / 100).toLocaleString('zh-CN', { minimumFractionDigits: 2 })}`

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold text-white">城市合伙人</h1>
        <p className="text-sm text-gray-400 mt-1">管理你负责区域的城市合伙人</p>
      </div>

      <div className="flex gap-2">
        {['', 'pending', 'approved', 'rejected', 'suspended'].map(s => (
          <button key={s} onClick={() => setFilter(s)}
            className={`px-3 py-1.5 rounded-lg text-xs transition-colors ${filter === s ? 'bg-claw-500/10 text-claw-400' : 'text-gray-400 hover:text-white hover:bg-white/5'}`}>
            {s === '' ? '全部' : STATUS_MAP[s]?.text || s}
          </button>
        ))}
      </div>

      {cities.length === 0 ? (
        <div className="rounded-xl border border-white/10 border-dashed p-12 text-center">
          <MapPin className="w-10 h-10 text-gray-600 mx-auto mb-3" />
          <p className="text-gray-500">暂无城市合伙人</p>
        </div>
      ) : (
        <div className="rounded-xl border border-white/10 overflow-hidden">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-white/10 text-gray-400 text-left">
                <th className="px-4 py-3 font-medium">姓名</th>
                <th className="px-4 py-3 font-medium">公司</th>
                <th className="px-4 py-3 font-medium">城市</th>
                <th className="px-4 py-3 font-medium">推荐码</th>
                <th className="px-4 py-3 font-medium text-right">佣金率</th>
                <th className="px-4 py-3 font-medium text-right">总客户</th>
                <th className="px-4 py-3 font-medium text-right">累计佣金</th>
                <th className="px-4 py-3 font-medium">状态</th>
                <th className="px-4 py-3 font-medium">操作</th>
              </tr>
            </thead>
            <tbody>
              {cities.map(cp => {
                const st = STATUS_MAP[cp.status] || { text: cp.status, color: 'text-gray-400' }
                return (
                  <tr key={cp.id} className="border-b border-white/5 hover:bg-white/[0.02]">
                    <td className="px-4 py-3 text-white font-medium">{cp.name}</td>
                    <td className="px-4 py-3 text-gray-400">{cp.company || '-'}</td>
                    <td className="px-4 py-3 text-gray-400">{cp.city}</td>
                    <td className="px-4 py-3 text-claw-400 font-mono text-xs">{cp.ref_code}</td>
                    <td className="px-4 py-3 text-gray-300 text-right">{(cp.comm_rate * 100).toFixed(0)}%</td>
                    <td className="px-4 py-3 text-gray-300 text-right">{cp.total_clients}</td>
                    <td className="px-4 py-3 text-green-400 text-right">{fmt(cp.total_earned)}</td>
                    <td className="px-4 py-3"><span className={`text-xs px-2 py-0.5 rounded ${st.color}`}>{st.text}</span></td>
                    <td className="px-4 py-3">
                      <div className="flex gap-1.5">
                        {cp.status === 'pending' && (
                          <>
                            <button onClick={() => review(cp.id, 'approved')} className="text-green-400 hover:text-green-300" title="通过">
                              <CheckCircle size={16} />
                            </button>
                            <button onClick={() => review(cp.id, 'rejected')} className="text-red-400 hover:text-red-300" title="拒绝">
                              <XCircle size={16} />
                            </button>
                          </>
                        )}
                        {cp.status === 'approved' && (
                          <button onClick={() => review(cp.id, 'suspended')} className="text-gray-400 hover:text-gray-300 text-xs">暂停</button>
                        )}
                        {cp.status === 'suspended' && (
                          <button onClick={() => review(cp.id, 'approved')} className="text-green-400 hover:text-green-300 text-xs">恢复</button>
                        )}
                      </div>
                    </td>
                  </tr>
                )
              })}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}
