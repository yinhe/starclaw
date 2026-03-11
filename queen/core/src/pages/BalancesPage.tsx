import { useEffect, useState } from 'react'
import { api, type UserBalance } from '../api'

function fen2yuan(fen: number): string {
  return `¥${(fen / 100).toFixed(2)}`
}

export default function BalancesPage() {
  const [balances, setBalances] = useState<UserBalance[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [loading, setLoading] = useState(true)
  const [adjustModal, setAdjustModal] = useState<{ userId: string; show: boolean }>({ userId: '', show: false })
  const [adjustForm, setAdjustForm] = useState({ amount: '', remark: '' })
  const size = 20

  const fetchBalances = () => {
    setLoading(true)
    api.get<{ balances: UserBalance[]; total: number }>(`/v1/admin/billing/balances?page=${page}&size=${size}`)
      .then((d) => { setBalances(d.balances || []); setTotal(d.total) })
      .catch(() => {})
      .finally(() => setLoading(false))
  }

  useEffect(() => { fetchBalances() }, [page])

  const handleAdjust = async () => {
    const amountFen = Math.round(parseFloat(adjustForm.amount) * 100)
    if (!amountFen || !adjustForm.remark) return
    await api.post('/v1/admin/billing/adjust', {
      user_id: adjustModal.userId,
      amount: amountFen,
      remark: adjustForm.remark,
    })
    setAdjustModal({ userId: '', show: false })
    setAdjustForm({ amount: '', remark: '' })
    fetchBalances()
  }

  const totalPages = Math.ceil(total / size)

  return (
    <div>
      <h2 className="text-xl font-bold mb-6">用户余额</h2>

      {loading ? (
        <div className="text-gray-500 text-center py-20">加载中...</div>
      ) : (
        <>
          <div className="bg-gray-900 border border-gray-800 rounded-xl overflow-hidden">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-gray-800 text-gray-400 text-left">
                  <th className="px-4 py-3 font-medium">用户ID</th>
                  <th className="px-4 py-3 font-medium">余额</th>
                  <th className="px-4 py-3 font-medium">累计充值</th>
                  <th className="px-4 py-3 font-medium">累计消费</th>
                  <th className="px-4 py-3 font-medium">操作</th>
                </tr>
              </thead>
              <tbody>
                {balances.map((b) => (
                  <tr key={b.id} className="border-b border-gray-800/50 hover:bg-gray-800/30">
                    <td className="px-4 py-3 text-gray-300 font-mono text-xs">{b.user_id.slice(0, 12)}...</td>
                    <td className="px-4 py-3 text-gray-100 font-medium">{fen2yuan(b.balance)}</td>
                    <td className="px-4 py-3 text-green-400">{fen2yuan(b.total_in)}</td>
                    <td className="px-4 py-3 text-red-400">{fen2yuan(b.total_out)}</td>
                    <td className="px-4 py-3">
                      <button
                        onClick={() => setAdjustModal({ userId: b.user_id, show: true })}
                        className="px-2 py-1 bg-purple-600/20 text-purple-400 rounded text-xs hover:bg-purple-600/30"
                      >
                        调账
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>

          {totalPages > 1 && (
            <div className="flex items-center justify-center gap-2 mt-4">
              <button onClick={() => setPage(Math.max(1, page - 1))} disabled={page === 1} className="px-3 py-1.5 bg-gray-800 rounded text-sm disabled:opacity-30">上一页</button>
              <span className="text-sm text-gray-400">{page} / {totalPages}</span>
              <button onClick={() => setPage(Math.min(totalPages, page + 1))} disabled={page === totalPages} className="px-3 py-1.5 bg-gray-800 rounded text-sm disabled:opacity-30">下一页</button>
            </div>
          )}
        </>
      )}

      {/* Adjust modal */}
      {adjustModal.show && (
        <div className="fixed inset-0 bg-black/60 flex items-center justify-center z-50">
          <div className="bg-gray-900 border border-gray-700 rounded-xl p-6 w-96">
            <h3 className="text-lg font-bold mb-4">调整余额</h3>
            <p className="text-xs text-gray-500 mb-4">用户: {adjustModal.userId.slice(0, 12)}...</p>
            <div className="space-y-3">
              <div>
                <label className="block text-sm text-gray-400 mb-1">金额（元，正数加/负数减）</label>
                <input
                  type="number"
                  step="0.01"
                  value={adjustForm.amount}
                  onChange={(e) => setAdjustForm({ ...adjustForm, amount: e.target.value })}
                  className="w-full bg-gray-800 border border-gray-700 rounded-lg px-3 py-2 text-sm focus:outline-none focus:border-purple-500"
                  placeholder="例: 10 或 -5"
                />
              </div>
              <div>
                <label className="block text-sm text-gray-400 mb-1">备注</label>
                <input
                  type="text"
                  value={adjustForm.remark}
                  onChange={(e) => setAdjustForm({ ...adjustForm, remark: e.target.value })}
                  className="w-full bg-gray-800 border border-gray-700 rounded-lg px-3 py-2 text-sm focus:outline-none focus:border-purple-500"
                  placeholder="调整原因"
                />
              </div>
            </div>
            <div className="flex gap-2 mt-5">
              <button onClick={() => setAdjustModal({ userId: '', show: false })} className="flex-1 px-3 py-2 bg-gray-800 rounded-lg text-sm">取消</button>
              <button onClick={handleAdjust} className="flex-1 px-3 py-2 bg-purple-600 rounded-lg text-sm text-white">确认</button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
