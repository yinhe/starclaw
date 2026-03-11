import { useEffect, useState } from 'react'
import { api, type RechargeOrder } from '../api'

function fen2yuan(fen: number): string {
  return `¥${(fen / 100).toFixed(2)}`
}

function statusBadge(status: string) {
  const map: Record<string, string> = {
    paid: 'bg-green-500/20 text-green-400',
    pending: 'bg-amber-500/20 text-amber-400',
    expired: 'bg-gray-500/20 text-gray-400',
    refunded: 'bg-red-500/20 text-red-400',
  }
  return map[status] || 'bg-gray-500/20 text-gray-400'
}

const statusLabel: Record<string, string> = {
  paid: '已支付',
  pending: '待支付',
  expired: '已过期',
  refunded: '已退款',
}

export default function OrdersPage() {
  const [orders, setOrders] = useState<RechargeOrder[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [filter, setFilter] = useState('')
  const [loading, setLoading] = useState(true)
  const size = 20

  const fetchOrders = () => {
    setLoading(true)
    let path = `/v1/admin/billing/orders?page=${page}&size=${size}`
    if (filter) path += `&status=${filter}`
    api.get<{ orders: RechargeOrder[]; total: number }>(path)
      .then((d) => { setOrders(d.orders || []); setTotal(d.total) })
      .catch(() => {})
      .finally(() => setLoading(false))
  }

  useEffect(() => { fetchOrders() }, [page, filter])

  const totalPages = Math.ceil(total / size)

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <h2 className="text-xl font-bold">订单管理</h2>
        <div className="flex gap-2">
          {['', 'paid', 'pending', 'expired'].map((s) => (
            <button
              key={s}
              onClick={() => { setFilter(s); setPage(1) }}
              className={`px-3 py-1.5 rounded-lg text-xs ${
                filter === s ? 'bg-purple-600 text-white' : 'bg-gray-800 text-gray-400 hover:bg-gray-700'
              }`}
            >
              {s === '' ? '全部' : statusLabel[s] || s}
            </button>
          ))}
        </div>
      </div>

      {loading ? (
        <div className="text-gray-500 text-center py-20">加载中...</div>
      ) : (
        <>
          <div className="bg-gray-900 border border-gray-800 rounded-xl overflow-hidden">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-gray-800 text-gray-400 text-left">
                  <th className="px-4 py-3 font-medium">订单号</th>
                  <th className="px-4 py-3 font-medium">用户ID</th>
                  <th className="px-4 py-3 font-medium">金额</th>
                  <th className="px-4 py-3 font-medium">赠送</th>
                  <th className="px-4 py-3 font-medium">方式</th>
                  <th className="px-4 py-3 font-medium">状态</th>
                  <th className="px-4 py-3 font-medium">创建时间</th>
                  <th className="px-4 py-3 font-medium">支付时间</th>
                </tr>
              </thead>
              <tbody>
                {orders.map((o) => (
                  <tr key={o.id} className="border-b border-gray-800/50 hover:bg-gray-800/30">
                    <td className="px-4 py-3 text-gray-300 font-mono text-xs">{o.order_no}</td>
                    <td className="px-4 py-3 text-gray-400 text-xs">{o.user_id.slice(0, 8)}...</td>
                    <td className="px-4 py-3 text-gray-100">{fen2yuan(o.amount)}</td>
                    <td className="px-4 py-3 text-green-400">{o.bonus_amount > 0 ? fen2yuan(o.bonus_amount) : '--'}</td>
                    <td className="px-4 py-3 text-gray-400">{o.pay_method}</td>
                    <td className="px-4 py-3">
                      <span className={`px-2 py-0.5 rounded-full text-xs ${statusBadge(o.status)}`}>
                        {statusLabel[o.status] || o.status}
                      </span>
                    </td>
                    <td className="px-4 py-3 text-gray-500 text-xs">{new Date(o.created_at).toLocaleString('zh-CN')}</td>
                    <td className="px-4 py-3 text-gray-500 text-xs">{o.paid_at ? new Date(o.paid_at).toLocaleString('zh-CN') : '--'}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>

          {/* Pagination */}
          {totalPages > 1 && (
            <div className="flex items-center justify-center gap-2 mt-4">
              <button
                onClick={() => setPage(Math.max(1, page - 1))}
                disabled={page === 1}
                className="px-3 py-1.5 bg-gray-800 rounded text-sm disabled:opacity-30"
              >
                上一页
              </button>
              <span className="text-sm text-gray-400">{page} / {totalPages}</span>
              <button
                onClick={() => setPage(Math.min(totalPages, page + 1))}
                disabled={page === totalPages}
                className="px-3 py-1.5 bg-gray-800 rounded text-sm disabled:opacity-30"
              >
                下一页
              </button>
            </div>
          )}
        </>
      )}
    </div>
  )
}
