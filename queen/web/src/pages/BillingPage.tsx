import { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { Navbar } from '../components/Navbar';
import { Footer } from '../components/Footer';
import { billingAPI, type RechargePackage, type BalanceInfo, type BalanceTransaction, type RechargeOrder } from '../lib/api';
import { isLoggedIn } from '../lib/auth';
import { Wallet, CreditCard, ArrowUpRight, ArrowDownRight, Receipt, Clock, CheckCircle, XCircle, Sparkles, Lock } from 'lucide-react';

function formatAmount(amount: number) {
  return `¥${(amount / 100).toFixed(2)}`;
}

function timeAgo(dateStr: string) {
  const d = new Date(dateStr);
  const now = new Date();
  const diff = Math.floor((now.getTime() - d.getTime()) / 1000);
  if (diff < 60) return '刚刚';
  if (diff < 3600) return `${Math.floor(diff / 60)} 分钟前`;
  if (diff < 86400) return `${Math.floor(diff / 3600)} 小时前`;
  if (diff < 604800) return `${Math.floor(diff / 86400)} 天前`;
  return d.toLocaleDateString('zh-CN');
}

type Tab = 'recharge' | 'transactions' | 'orders';

export function BillingPage() {
  const navigate = useNavigate();
  const [tab, setTab] = useState<Tab>('recharge');
  const [balance, setBalance] = useState<BalanceInfo | null>(null);
  const [packages, setPackages] = useState<RechargePackage[]>([]);
  const [transactions, setTransactions] = useState<BalanceTransaction[]>([]);
  const [orders, setOrders] = useState<RechargeOrder[]>([]);
  const [loading, setLoading] = useState(true);

  // Recharge state
  const [selectedPkg, setSelectedPkg] = useState<string>('');
  const [payMethod, setPayMethod] = useState<string>('alipay');
  const [payMethods, setPayMethods] = useState<string[]>([]);
  const [creating, setCreating] = useState(false);
  const [payUrl, setPayUrl] = useState('');

  useEffect(() => {
    if (!isLoggedIn()) { navigate('/auth?redirect=/billing'); return; }
    Promise.all([
      billingAPI.balance().then(setBalance).catch(() => {}),
      billingAPI.packages().then(r => setPackages(r.packages || [])).catch(() => {}),
      billingAPI.methods().then(r => { setPayMethods(r.methods || []); if (r.methods?.length) setPayMethod(r.methods[0]); }).catch(() => {}),
    ]).then(() => setLoading(false));
  }, [navigate]);

  useEffect(() => {
    if (tab === 'transactions') {
      billingAPI.transactions().then(r => setTransactions(r.transactions || [])).catch(() => {});
    } else if (tab === 'orders') {
      billingAPI.orders().then(r => setOrders(r.orders || [])).catch(() => {});
    }
  }, [tab]);

  async function handleRecharge() {
    if (!selectedPkg) return;
    setCreating(true);
    try {
      const r = await billingAPI.createOrder({ package_id: selectedPkg, pay_method: payMethod });
      if (r.pay_url) {
        setPayUrl(r.pay_url);
        window.open(r.pay_url, '_blank');
      }
    } catch { /* ignore */ }
    setCreating(false);
  }

  const PAY_LABELS: Record<string, string> = { alipay: '支付宝', wechatpay: '微信支付' };

  if (loading) {
    return (
      <div className="min-h-screen bg-gray-50">
        <Navbar />
        <div className="text-center pt-32 text-gray-400">加载中...</div>
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-gray-50">
      <Navbar />
      <div className="max-w-4xl mx-auto px-6 pt-24 pb-16">
        {/* Header */}
        <div className="mb-8">
          <div className="flex items-center gap-3 mb-2">
            <div className="w-10 h-10 rounded-xl bg-gradient-to-br from-amber-500 to-orange-500 flex items-center justify-center">
              <Wallet className="w-5 h-5 text-white" />
            </div>
            <h1 className="text-3xl font-bold text-gray-900">充值 & 账单</h1>
          </div>
        </div>

        {/* Balance Card */}
        {balance && (
          <div className="bg-gradient-to-r from-indigo-500 to-purple-600 rounded-2xl p-6 mb-8 text-white">
            <p className="text-sm opacity-80 mb-1">当前余额</p>
            <p className="text-4xl font-bold">{formatAmount(balance.balance)}</p>
            <div className="flex flex-wrap gap-6 mt-4 text-sm opacity-80">
              {balance.frozen > 0 && (
                <span className="flex items-center gap-1"><Lock className="w-4 h-4" />冻结中 {formatAmount(balance.frozen)}</span>
              )}
              <span className="flex items-center gap-1"><ArrowUpRight className="w-4 h-4" />累计充值 {formatAmount(balance.total_recharged)}</span>
              <span className="flex items-center gap-1"><ArrowDownRight className="w-4 h-4" />累计消费 {formatAmount(balance.total_consumed)}</span>
            </div>
          </div>
        )}

        {/* Tabs */}
        <div className="flex gap-1 mb-6 bg-gray-100 rounded-lg p-1 w-fit">
          {([
            { key: 'recharge' as Tab, label: '充值', icon: CreditCard },
            { key: 'transactions' as Tab, label: '账单流水', icon: Receipt },
            { key: 'orders' as Tab, label: '充值记录', icon: Clock },
          ]).map(t => (
            <button key={t.key} onClick={() => setTab(t.key)}
              className={`px-4 py-2 rounded-md text-sm font-medium transition flex items-center gap-1.5 ${tab === t.key ? 'bg-white text-gray-900 shadow-sm' : 'text-gray-500 hover:text-gray-700'}`}>
              <t.icon className="w-4 h-4" />{t.label}
            </button>
          ))}
        </div>

        {/* ─── Recharge Tab ─── */}
        {tab === 'recharge' && (
          <div>
            <h3 className="font-bold text-lg mb-4">选择充值套餐</h3>
            <div className="grid grid-cols-2 md:grid-cols-3 gap-4 mb-6">
              {packages.filter(p => p.is_active).map(pkg => (
                <button key={pkg.id} onClick={() => setSelectedPkg(pkg.id)}
                  className={`relative rounded-xl border-2 p-5 text-left transition ${selectedPkg === pkg.id ? 'border-indigo-500 bg-indigo-50/50 shadow-sm' : 'border-gray-200 bg-white hover:border-indigo-300'}`}>
                  {pkg.bonus_percent > 0 && (
                    <span className="absolute -top-2 -right-2 px-2 py-0.5 rounded-full bg-amber-400 text-white text-[10px] font-bold flex items-center gap-0.5">
                      <Sparkles className="w-3 h-3" />+{pkg.bonus_percent}%
                    </span>
                  )}
                  <p className="text-2xl font-bold text-gray-900">{pkg.price_display || formatAmount(pkg.amount)}</p>
                  <p className="text-sm text-gray-500 mt-1">{pkg.name}</p>
                  {pkg.bonus_percent > 0 && (
                    <p className="text-xs text-amber-600 mt-1">赠送 {pkg.bonus_percent}%</p>
                  )}
                </button>
              ))}
            </div>

            {/* Pay method */}
            <h3 className="font-bold text-lg mb-3">支付方式</h3>
            <div className="flex gap-3 mb-6">
              {payMethods.map(m => (
                <button key={m} onClick={() => setPayMethod(m)}
                  className={`px-5 py-3 rounded-xl border-2 text-sm font-medium transition ${payMethod === m ? 'border-indigo-500 bg-indigo-50/50' : 'border-gray-200 bg-white hover:border-indigo-300'}`}>
                  {PAY_LABELS[m] || m}
                </button>
              ))}
            </div>

            <button onClick={handleRecharge} disabled={!selectedPkg || creating}
              className="px-8 py-3 rounded-xl bg-indigo-500 text-white font-semibold hover:bg-indigo-600 disabled:opacity-50 transition shadow-lg shadow-indigo-500/25">
              {creating ? '创建订单...' : '立即充值'}
            </button>

            {payUrl && (
              <div className="mt-4 p-4 bg-green-50 border border-green-200 rounded-lg text-sm text-green-700">
                支付页面已在新窗口打开。如未跳转，<a href={payUrl} target="_blank" rel="noreferrer" className="underline">点击这里</a>。
              </div>
            )}
          </div>
        )}

        {/* ─── Transactions Tab ─── */}
        {tab === 'transactions' && (
          <div className="bg-white rounded-xl border border-gray-200 overflow-hidden">
            {transactions.length === 0 ? (
              <p className="text-center py-12 text-gray-400">暂无账单记录</p>
            ) : (
              <table className="w-full">
                <thead>
                  <tr className="border-b border-gray-100 text-sm text-gray-500">
                    <th className="text-left px-6 py-3 font-medium">时间</th>
                    <th className="text-left px-6 py-3 font-medium">类型</th>
                    <th className="text-left px-6 py-3 font-medium">说明</th>
                    <th className="text-right px-6 py-3 font-medium">金额</th>
                    <th className="text-right px-6 py-3 font-medium">余额</th>
                  </tr>
                </thead>
                <tbody>
                  {transactions.map(tx => (
                    <tr key={tx.id} className="border-b border-gray-50 hover:bg-gray-50/50">
                      <td className="px-6 py-3 text-sm text-gray-500">{timeAgo(tx.created_at)}</td>
                      <td className="px-6 py-3">
                        <span className={`inline-flex items-center gap-1 text-xs font-medium ${
                          tx.type === 'recharge' || tx.type === 'bonus' || tx.type === 'unfreeze' || tx.type === 'bounty_earn' ? 'text-emerald-600' :
                          tx.type === 'consume' || tx.type === 'freeze' || tx.type === 'bounty_pay' ? 'text-red-500' :
                          'text-blue-600'
                        }`}>
                          {tx.amount >= 0 ? <ArrowUpRight className="w-3 h-3" /> : <ArrowDownRight className="w-3 h-3" />}
                          {{
                            recharge: '充值', consume: '消费', bonus: '赠送',
                            freeze: '冻结', unfreeze: '解冻',
                            bounty_pay: '赏金支出', bounty_earn: '赏金收入',
                            admin_adjust: '调整',
                          }[tx.type] || tx.type}
                        </span>
                      </td>
                      <td className="px-6 py-3 text-sm text-gray-600">{tx.description}</td>
                      <td className={`px-6 py-3 text-sm text-right font-medium ${tx.amount >= 0 ? 'text-emerald-600' : 'text-red-500'}`}>
                        {tx.amount >= 0 ? '+' : ''}{formatAmount(tx.amount)}
                      </td>
                      <td className="px-6 py-3 text-sm text-right text-gray-500">{formatAmount(tx.balance_after)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </div>
        )}

        {/* ─── Orders Tab ─── */}
        {tab === 'orders' && (
          <div className="bg-white rounded-xl border border-gray-200 overflow-hidden">
            {orders.length === 0 ? (
              <p className="text-center py-12 text-gray-400">暂无充值记录</p>
            ) : (
              <table className="w-full">
                <thead>
                  <tr className="border-b border-gray-100 text-sm text-gray-500">
                    <th className="text-left px-6 py-3 font-medium">订单号</th>
                    <th className="text-left px-6 py-3 font-medium">金额</th>
                    <th className="text-left px-6 py-3 font-medium">支付方式</th>
                    <th className="text-left px-6 py-3 font-medium">状态</th>
                    <th className="text-left px-6 py-3 font-medium">时间</th>
                  </tr>
                </thead>
                <tbody>
                  {orders.map(order => (
                    <tr key={order.id} className="border-b border-gray-50 hover:bg-gray-50/50">
                      <td className="px-6 py-3 text-sm font-mono text-gray-600">{order.order_no}</td>
                      <td className="px-6 py-3 text-sm font-medium text-gray-900">{formatAmount(order.amount)}</td>
                      <td className="px-6 py-3 text-sm text-gray-600">{PAY_LABELS[order.pay_method] || order.pay_method}</td>
                      <td className="px-6 py-3">
                        <span className={`inline-flex items-center gap-1 text-xs font-medium ${
                          order.status === 'paid' ? 'text-green-600' :
                          order.status === 'pending' ? 'text-amber-600' :
                          'text-gray-400'
                        }`}>
                          {order.status === 'paid' ? <CheckCircle className="w-3 h-3" /> :
                           order.status === 'pending' ? <Clock className="w-3 h-3" /> :
                           <XCircle className="w-3 h-3" />}
                          {order.status === 'paid' ? '已支付' :
                           order.status === 'pending' ? '待支付' :
                           order.status === 'failed' ? '失败' : '已过期'}
                        </span>
                      </td>
                      <td className="px-6 py-3 text-sm text-gray-500">{timeAgo(order.created_at)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </div>
        )}
      </div>
      <Footer />
    </div>
  );
}
