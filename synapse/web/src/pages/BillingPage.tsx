import { useEffect, useState } from 'react';
import { ExternalLink, Clock, X, QrCode, RefreshCw } from 'lucide-react';
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
  const [balance, setBalance] = useState<{ balance_cents: number; free_quota: number; star_energy_display?: number; star_status?: string }>({ balance_cents: 0, free_quota: 0 });
  const [loading, setLoading] = useState('');
  const [wxLoading, setWxLoading] = useState('');
  const [error, setError] = useState('');
  const [qrCode, setQrCode] = useState<{ orderNo: string; codeUrl: string; amount: string } | null>(null);
  const [syncing, setSyncing] = useState(false);

  const refreshData = () => {
    dash.orders().then(r => setOrders(r.orders || [])).catch(console.error);
    dash.balance().then(setBalance).catch(console.error);
  };

  const syncPending = async () => {
    setSyncing(true);
    try {
      const res = await dash.syncOrders();
      if (res.synced > 0) refreshData();
    } catch { /* ignore */ }
    setSyncing(false);
  };

  useEffect(() => {
    dash.packages().then(r => setPackages(r.packages)).catch(console.error);
    refreshData();
    // Auto-sync pending orders on page load
    syncPending();
    // Also sync when tab becomes visible (user returns from payment page)
    const onFocus = () => syncPending();
    window.addEventListener('focus', onFocus);
    return () => window.removeEventListener('focus', onFocus);
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

  const payWechat = async (pkgId: string) => {
    setWxLoading(pkgId);
    setError('');
    try {
      const res = await dash.payWechat(pkgId);
      if (res.code_url) {
        setQrCode({ orderNo: res.order_no, codeUrl: res.code_url, amount: res.amount });
      }
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'WeChat Pay failed');
    } finally {
      setWxLoading('');
    }
  };

  // Poll order status when QR modal is open (active query)
  useEffect(() => {
    if (!qrCode) return;
    const interval = setInterval(async () => {
      try {
        const res = await dash.queryOrder(qrCode.orderNo);
        if (res.status === 'paid') {
          setQrCode(null);
          refreshData();
        }
      } catch { /* ignore */ }
    }, 3000);
    return () => clearInterval(interval);
  }, [qrCode]);

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
          <div className="text-sm text-amber-400 mb-1">{balance.star_energy_display != null ? '星能余额' : '当前余额'}</div>
          <div className="text-3xl font-bold text-white">
            {balance.star_energy_display != null
              ? `${balance.star_energy_display.toFixed(1)} ⚡`
              : fmt(balance.balance_cents)}
          </div>
          <div className="text-xs text-gray-400 mt-1">
            {balance.star_energy_display != null
              ? `≈ ¥${(balance.star_energy_display / 100).toFixed(2)}`
              : `免费额度: ${fmt(balance.free_quota)}`}
          </div>
        </div>
        <button
          onClick={syncPending}
          disabled={syncing}
          className="flex items-center gap-1.5 px-3 py-1.5 text-xs text-gray-400 hover:text-white bg-gray-800 hover:bg-gray-700 rounded-lg transition-colors disabled:opacity-50 cursor-pointer"
          title="同步支付状态"
        >
          <RefreshCw className={`w-3.5 h-3.5 ${syncing ? 'animate-spin' : ''}`} />
          {syncing ? '同步中...' : '刷新状态'}
        </button>
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
                  onClick={() => payWechat(pkg.id)}
                  disabled={wxLoading === pkg.id}
                  className="flex-1 bg-green-600 hover:bg-green-500 disabled:opacity-50 text-white font-medium py-2 rounded-lg text-sm transition-colors flex items-center justify-center gap-1.5 cursor-pointer"
                >
                  {wxLoading === pkg.id ? '...' : <>微信支付 <QrCode className="w-3 h-3" /></>}
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
                <th className="px-5 py-3 font-medium">订单号</th>
                <th className="px-5 py-3 font-medium">渠道</th>
                <th className="px-5 py-3 font-medium text-right">金额</th>
                <th className="px-5 py-3 font-medium text-right">到账</th>
                <th className="px-5 py-3 font-medium">状态</th>
              </tr>
            </thead>
            <tbody>
              {orders.length === 0 ? (
                <tr>
                  <td colSpan={6} className="px-5 py-12 text-center text-gray-500">
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
                      <td className="px-5 py-3 text-gray-500 font-mono text-xs">{o.order_no}</td>
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
      {/* WeChat QR Code Modal */}
      {qrCode && (
        <div className="fixed inset-0 bg-black/60 flex items-center justify-center z-50" onClick={() => setQrCode(null)}>
          <div className="bg-gray-900 border border-gray-700 rounded-2xl p-6 max-w-sm w-full mx-4 space-y-4" onClick={e => e.stopPropagation()}>
            <div className="flex items-center justify-between">
              <h3 className="text-lg font-semibold text-white">微信扫码支付</h3>
              <button onClick={() => setQrCode(null)} className="text-gray-500 hover:text-white cursor-pointer">
                <X className="w-5 h-5" />
              </button>
            </div>
            <div className="text-center space-y-3">
              <div className="text-2xl font-bold text-green-400">¥{qrCode.amount}</div>
              <div className="bg-white rounded-xl p-4 inline-block">
                <img
                  src={`https://api.qrserver.com/v1/create-qr-code/?size=200x200&data=${encodeURIComponent(qrCode.codeUrl)}`}
                  alt="WeChat Pay QR Code"
                  className="w-48 h-48"
                />
              </div>
              <p className="text-gray-400 text-sm">请用微信扫描二维码完成支付</p>
              <p className="text-gray-500 text-xs">支付成功后页面会自动刷新</p>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
