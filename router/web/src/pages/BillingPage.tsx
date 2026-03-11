import { useEffect, useState } from 'react';
import { CreditCard, ExternalLink, Clock } from 'lucide-react';
import { dash } from '../lib/api';

interface Package {
  id: string;
  name: string;
  amount_cents: number;
  bonus_cents: number;
  total_cents: number;
}

interface Order {
  id: string;
  order_no: string;
  channel: string;
  amount_cents: number;
  bonus_cents: number;
  total_cents: number;
  status: string;
  created_at: string;
  paid_at: string | null;
}

export default function BillingPage() {
  const [packages, setPackages] = useState<Package[]>([]);
  const [orders, setOrders] = useState<Order[]>([]);
  const [balance, setBalance] = useState({ balance_cents: 0, free_quota: 0 });
  const [loading, setLoading] = useState('');
  const [error, setError] = useState('');

  useEffect(() => {
    dash.packages().then(r => setPackages(r.packages)).catch(console.error);
    dash.orders().then(r => setOrders(r.orders || [])).catch(console.error);
    dash.balance().then(setBalance).catch(console.error);
  }, []);

  const fmt = (cents: number) => `¥${(cents / 100).toFixed(2)}`;

  const payAlipay = async (pkgId: string) => {
    setLoading(pkgId);
    setError('');
    try {
      const res = await dash.payAlipay(pkgId);
      window.open(res.pay_url, '_blank');
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Payment failed');
    } finally {
      setLoading('');
    }
  };

  const statusMap: Record<string, { text: string; color: string }> = {
    pending: { text: '待支付', color: 'text-yellow-400' },
    paid: { text: '已支付', color: 'text-green-400' },
    failed: { text: '失败', color: 'text-red-400' },
    expired: { text: '已过期', color: 'text-gray-500' },
  };

  return (
    <div className="space-y-8">
      <div>
        <h1 className="text-2xl font-bold text-white">充值</h1>
        <p className="text-gray-400 text-sm mt-1">为你的账户充值余额</p>
      </div>

      {/* Current balance */}
      <div className="bg-gradient-to-r from-amber-500/10 to-orange-500/10 border border-amber-500/20 rounded-xl p-6 flex items-center justify-between">
        <div>
          <div className="text-sm text-amber-400 mb-1">当前余额</div>
          <div className="text-3xl font-bold text-white">{fmt(balance.balance_cents)}</div>
          <div className="text-xs text-gray-400 mt-1">免费额度: {fmt(balance.free_quota)}</div>
        </div>
        <CreditCard className="w-12 h-12 text-amber-400/30" />
      </div>

      {error && (
        <div className="bg-red-500/10 border border-red-500/20 text-red-400 text-sm px-4 py-3 rounded-lg">
          {error}
        </div>
      )}

      {/* Packages */}
      <div>
        <h2 className="text-lg font-semibold text-white mb-4">选择充值套餐</h2>
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
          {packages.map(pkg => (
            <div key={pkg.id} className="bg-gray-900 border border-gray-800 rounded-xl p-5 hover:border-amber-500/30 transition-colors">
              <div className="text-xl font-bold text-white">{fmt(pkg.amount_cents)}</div>
              {pkg.bonus_cents > 0 && (
                <div className="inline-block bg-green-500/10 text-green-400 text-xs px-2 py-0.5 rounded mt-2">
                  +{fmt(pkg.bonus_cents)} 赠送
                </div>
              )}
              <div className="text-gray-400 text-sm mt-2">
                到账: <span className="text-white font-medium">{fmt(pkg.total_cents)}</span>
              </div>
              <div className="flex gap-2 mt-4">
                <button
                  onClick={() => payAlipay(pkg.id)}
                  disabled={loading === pkg.id}
                  className="flex-1 bg-blue-500 hover:bg-blue-400 disabled:opacity-50 text-white font-medium py-2 rounded-lg text-sm transition-colors flex items-center justify-center gap-1.5 cursor-pointer"
                >
                  {loading === pkg.id ? '...' : <>支付宝 <ExternalLink className="w-3 h-3" /></>}
                </button>
                <button
                  disabled
                  className="flex-1 bg-green-600 opacity-50 text-white font-medium py-2 rounded-lg text-sm flex items-center justify-center gap-1.5 cursor-not-allowed"
                >
                  微信支付
                </button>
              </div>
            </div>
          ))}
        </div>
      </div>

      {/* Orders */}
      <div>
        <h2 className="text-lg font-semibold text-white mb-4">充值记录</h2>
        <div className="bg-gray-900 border border-gray-800 rounded-xl overflow-hidden">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-gray-800 text-gray-400 text-left">
                <th className="px-5 py-3 font-medium">时间</th>
                <th className="px-5 py-3 font-medium">渠道</th>
                <th className="px-5 py-3 font-medium text-right">金额</th>
                <th className="px-5 py-3 font-medium text-right">到账</th>
                <th className="px-5 py-3 font-medium">状态</th>
              </tr>
            </thead>
            <tbody>
              {orders.length === 0 ? (
                <tr>
                  <td colSpan={5} className="px-5 py-12 text-center text-gray-500">
                    <Clock className="w-8 h-8 mx-auto mb-2 opacity-30" />
                    暂无充值记录
                  </td>
                </tr>
              ) : (
                orders.map(o => {
                  const st = statusMap[o.status] || { text: o.status, color: 'text-gray-400' };
                  return (
                    <tr key={o.id} className="border-b border-gray-800/50 hover:bg-gray-800/30">
                      <td className="px-5 py-3 text-gray-400">{new Date(o.created_at).toLocaleString()}</td>
                      <td className="px-5 py-3 text-gray-300">{o.channel === 'alipay' ? '支付宝' : '微信'}</td>
                      <td className="px-5 py-3 text-white text-right">{fmt(o.amount_cents)}</td>
                      <td className="px-5 py-3 text-white text-right">{fmt(o.total_cents)}</td>
                      <td className={`px-5 py-3 ${st.color}`}>{st.text}</td>
                    </tr>
                  );
                })
              )}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  );
}
