import { useEffect, useState } from 'react'
import { api, type RechargePackage } from '../api'

function fen2yuan(fen: number): string {
  return `¥${(fen / 100).toFixed(2)}`
}

export default function PackagesPage() {
  const [packages, setPackages] = useState<RechargePackage[]>([])
  const [loading, setLoading] = useState(true)
  const [editing, setEditing] = useState<RechargePackage | null>(null)

  const fetchPackages = () => {
    setLoading(true)
    api.get<{ packages: RechargePackage[] }>('/v1/admin/billing/packages')
      .then((d) => setPackages(d.packages || []))
      .catch(() => {})
      .finally(() => setLoading(false))
  }

  useEffect(() => { fetchPackages() }, [])

  const handleToggle = async (pkg: RechargePackage) => {
    await api.put(`/v1/admin/billing/packages/${pkg.id}`, { enabled: !pkg.enabled })
    fetchPackages()
  }

  const handleSave = async () => {
    if (!editing) return
    await api.put(`/v1/admin/billing/packages/${editing.id}`, {
      name: editing.name,
      amount: editing.amount,
      bonus_amount: editing.bonus_amount,
      bonus_rate: editing.bonus_rate,
      sort_order: editing.sort_order,
    })
    setEditing(null)
    fetchPackages()
  }

  return (
    <div>
      <h2 className="text-xl font-bold mb-6">套餐管理</h2>

      {loading ? (
        <div className="text-gray-500 text-center py-20">加载中...</div>
      ) : (
        <div className="bg-gray-900 border border-gray-800 rounded-xl overflow-hidden">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-gray-800 text-gray-400 text-left">
                <th className="px-4 py-3 font-medium">名称</th>
                <th className="px-4 py-3 font-medium">金额</th>
                <th className="px-4 py-3 font-medium">赠送</th>
                <th className="px-4 py-3 font-medium">赠送比例</th>
                <th className="px-4 py-3 font-medium">排序</th>
                <th className="px-4 py-3 font-medium">状态</th>
                <th className="px-4 py-3 font-medium">操作</th>
              </tr>
            </thead>
            <tbody>
              {packages.map((pkg) => (
                <tr key={pkg.id} className="border-b border-gray-800/50 hover:bg-gray-800/30">
                  <td className="px-4 py-3 text-gray-100 font-medium">{pkg.name}</td>
                  <td className="px-4 py-3 text-gray-300">{fen2yuan(pkg.amount)}</td>
                  <td className="px-4 py-3 text-green-400">{fen2yuan(pkg.bonus_amount)}</td>
                  <td className="px-4 py-3 text-gray-400">{(pkg.bonus_rate * 100).toFixed(0)}%</td>
                  <td className="px-4 py-3 text-gray-400">{pkg.sort_order}</td>
                  <td className="px-4 py-3">
                    <button
                      onClick={() => handleToggle(pkg)}
                      className={`px-2 py-0.5 rounded-full text-xs ${
                        pkg.enabled ? 'bg-green-500/20 text-green-400' : 'bg-gray-500/20 text-gray-400'
                      }`}
                    >
                      {pkg.enabled ? '启用' : '停用'}
                    </button>
                  </td>
                  <td className="px-4 py-3">
                    <button
                      onClick={() => setEditing({ ...pkg })}
                      className="px-2 py-1 bg-purple-600/20 text-purple-400 rounded text-xs hover:bg-purple-600/30"
                    >
                      编辑
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {/* Edit modal */}
      {editing && (
        <div className="fixed inset-0 bg-black/60 flex items-center justify-center z-50">
          <div className="bg-gray-900 border border-gray-700 rounded-xl p-6 w-96">
            <h3 className="text-lg font-bold mb-4">编辑套餐</h3>
            <div className="space-y-3">
              <div>
                <label className="block text-sm text-gray-400 mb-1">名称</label>
                <input
                  value={editing.name}
                  onChange={(e) => setEditing({ ...editing, name: e.target.value })}
                  className="w-full bg-gray-800 border border-gray-700 rounded-lg px-3 py-2 text-sm focus:outline-none focus:border-purple-500"
                />
              </div>
              <div>
                <label className="block text-sm text-gray-400 mb-1">金额（分）</label>
                <input
                  type="number"
                  value={editing.amount}
                  onChange={(e) => setEditing({ ...editing, amount: parseInt(e.target.value) || 0 })}
                  className="w-full bg-gray-800 border border-gray-700 rounded-lg px-3 py-2 text-sm focus:outline-none focus:border-purple-500"
                />
              </div>
              <div>
                <label className="block text-sm text-gray-400 mb-1">赠送金额（分）</label>
                <input
                  type="number"
                  value={editing.bonus_amount}
                  onChange={(e) => setEditing({ ...editing, bonus_amount: parseInt(e.target.value) || 0 })}
                  className="w-full bg-gray-800 border border-gray-700 rounded-lg px-3 py-2 text-sm focus:outline-none focus:border-purple-500"
                />
              </div>
              <div>
                <label className="block text-sm text-gray-400 mb-1">赠送比例 (0~1)</label>
                <input
                  type="number"
                  step="0.01"
                  value={editing.bonus_rate}
                  onChange={(e) => setEditing({ ...editing, bonus_rate: parseFloat(e.target.value) || 0 })}
                  className="w-full bg-gray-800 border border-gray-700 rounded-lg px-3 py-2 text-sm focus:outline-none focus:border-purple-500"
                />
              </div>
              <div>
                <label className="block text-sm text-gray-400 mb-1">排序</label>
                <input
                  type="number"
                  value={editing.sort_order}
                  onChange={(e) => setEditing({ ...editing, sort_order: parseInt(e.target.value) || 0 })}
                  className="w-full bg-gray-800 border border-gray-700 rounded-lg px-3 py-2 text-sm focus:outline-none focus:border-purple-500"
                />
              </div>
            </div>
            <div className="flex gap-2 mt-5">
              <button onClick={() => setEditing(null)} className="flex-1 px-3 py-2 bg-gray-800 rounded-lg text-sm">取消</button>
              <button onClick={handleSave} className="flex-1 px-3 py-2 bg-purple-600 rounded-lg text-sm text-white">保存</button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
