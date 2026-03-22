import { useEffect, useState } from 'react';
import { admin, type Order } from '../lib/api';

export default function OrdersPage() {
  const [orders, setOrders] = useState<Order[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [pages, setPages] = useState(1);
  const [status, setStatus] = useState('');
  const [channel, setChannel] = useState('');

  useEffect(() => {
    admin.orders({ page, page_size: 30, status: status || undefined, channel: channel || undefined }).then((res) => {
      setOrders(res.orders || []);
      setTotal(res.total);
      setPages(res.pages);
    });
  }, [page, status, channel]);

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-2xl font-bold">订单管理 <span className="text-sm font-normal text-gray-500">({total})</span></h1>
        <div className="flex gap-2">
          <select value={status} onChange={(e) => { setStatus(e.target.value); setPage(1); }} className="bg-gray-900 border border-gray-800 rounded-lg px-3 py-2 text-sm text-white focus:outline-none focus:border-rose-500/50">
            <option value="">全部状态</option>
            <option value="pending">pending</option>
            <option value="paid">paid</option>
            <option value="failed">failed</option>
          </select>
          <select value={channel} onChange={(e) => { setChannel(e.target.value); setPage(1); }} className="bg-gray-900 border border-gray-800 rounded-lg px-3 py-2 text-sm text-white focus:outline-none focus:border-rose-500/50">
            <option value="">全部渠道</option>
            <option value="alipay">支付宝</option>
            <option value="wechat">微信</option>
          </select>
        </div>
      </div>

      <div className="bg-gray-900 border border-gray-800 rounded-xl overflow-x-auto">
        <table className="w-full text-sm whitespace-nowrap">
          <thead>
            <tr className="border-b border-gray-800 text-gray-400">
              <th className="text-left px-4 py-3 font-medium">订单号</th>
              <th className="text-left px-4 py-3 font-medium">用户ID</th>
              <th className="text-center px-4 py-3 font-medium">渠道</th>
              <th className="text-right px-4 py-3 font-medium">金额</th>
              <th className="text-right px-4 py-3 font-medium">赠送</th>
              <th className="text-right px-4 py-3 font-medium">到账</th>
              <th className="text-center px-4 py-3 font-medium">状态</th>
              <th className="text-left px-4 py-3 font-medium">创建时间</th>
              <th className="text-left px-4 py-3 font-medium">支付时间</th>
            </tr>
          </thead>
          <tbody>
            {orders.map((o) => (
              <tr key={o.id} className="border-b border-gray-800/50 hover:bg-gray-800/30">
                <td className="px-4 py-2.5 text-white font-mono text-xs">{o.order_no}</td>
                <td className="px-4 py-2.5 text-gray-500 font-mono text-xs">{o.user_id.slice(0, 8)}...</td>
                <td className="px-4 py-2.5 text-center">
                  <span className={`text-xs px-1.5 py-0.5 rounded ${o.channel === 'alipay' ? 'bg-blue-500/10 text-blue-400' : 'bg-green-500/10 text-green-400'}`}>{o.channel}</span>
                </td>
                <td className="px-4 py-2.5 text-right text-white">¥{(o.amount_cents / 100).toFixed(2)}</td>
                <td className="px-4 py-2.5 text-right text-amber-400">{o.bonus_cents > 0 ? `+¥${(o.bonus_cents / 100).toFixed(2)}` : '-'}</td>
                <td className="px-4 py-2.5 text-right text-emerald-400">¥{(o.total_cents / 100).toFixed(2)}</td>
                <td className="px-4 py-2.5 text-center">
                  <span className={`text-xs px-1.5 py-0.5 rounded ${
                    o.status === 'paid' ? 'bg-emerald-500/10 text-emerald-400' :
                    o.status === 'pending' ? 'bg-amber-500/10 text-amber-400' :
                    'bg-red-500/10 text-red-400'
                  }`}>{o.status}</span>
                </td>
                <td className="px-4 py-2.5 text-gray-400">{new Date(o.created_at).toLocaleString()}</td>
                <td className="px-4 py-2.5 text-gray-400">{o.paid_at ? new Date(o.paid_at).toLocaleString() : '-'}</td>
              </tr>
            ))}
            {orders.length === 0 && (
              <tr><td colSpan={9} className="px-4 py-8 text-center text-gray-500">暂无数据</td></tr>
            )}
          </tbody>
        </table>
      </div>

      {pages > 1 && (
        <div className="flex items-center justify-center gap-2 mt-4">
          <button onClick={() => setPage(Math.max(1, page - 1))} disabled={page <= 1} className="px-3 py-1.5 rounded bg-gray-800 text-gray-300 text-sm disabled:opacity-40 cursor-pointer">上一页</button>
          <span className="text-sm text-gray-500">{page} / {pages}</span>
          <button onClick={() => setPage(Math.min(pages, page + 1))} disabled={page >= pages} className="px-3 py-1.5 rounded bg-gray-800 text-gray-300 text-sm disabled:opacity-40 cursor-pointer">下一页</button>
        </div>
      )}
    </div>
  );
}
