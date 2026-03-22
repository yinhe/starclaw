import { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { investorAPI, type InvestorPoolInfo, type InvestorProfile, type DailyEarning, type EquityGrant, type DiamondOrderResult } from '../lib/api';
import { isLoggedIn } from '../lib/auth';
import {
  Diamond, Wallet, ArrowRight, CheckCircle, ArrowLeft,
  FileText, BarChart3, Gem, Zap, Lock, Unlock, Calendar, CreditCard, ShoppingCart,
} from 'lucide-react';

function fmt(yuan: number) { return `¥${yuan.toLocaleString('zh-CN', { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`; }
function fmtFen(fen: number) { return fmt(fen / 100); }
function pct(n: number, d: number) { return d > 0 ? `${(n / d * 100).toFixed(2)}%` : '0%'; }

type Tab = 'pool' | 'portfolio' | 'earnings' | 'equity';

export function InvestPage() {
  const navigate = useNavigate();
  const [tab, setTab] = useState<Tab>('pool');
  const [pool, setPool] = useState<InvestorPoolInfo | null>(null);
  const [profile, setProfile] = useState<InvestorProfile | null>(null);
  const [earnings, setEarnings] = useState<{ today: number; total30d: number; percent: string; daily: DailyEarning[] } | null>(null);
  const [equity, setEquity] = useState<EquityGrant[]>([]);
  const [loading, setLoading] = useState(true);
  const [msg, setMsg] = useState('');
  const [err, setErr] = useState('');

  // Recharge (balance-based)
  const [rechargeYuan, setRechargeYuan] = useState('');
  const [recharging, setRecharging] = useState(false);

  // Direct purchase (Alipay/WeChat)
  const [purchaseYuan, setPurchaseYuan] = useState('');
  const [payMethod, setPayMethod] = useState<'alipay' | 'wechatpay'>('alipay');
  const [purchasing, setPurchasing] = useState(false);
  const [purchaseResult, setPurchaseResult] = useState<DiamondOrderResult | null>(null);
  const [buyMode, setBuyMode] = useState<'pay' | 'balance'>('pay');

  // Agreement
  const [agreeTerm, setAgreeTerm] = useState(3);
  const [agreeing, setAgreeing] = useState(false);

  const loggedIn = isLoggedIn();

  useEffect(() => {
    if (!loggedIn) { navigate('/auth?redirect=' + encodeURIComponent(window.location.pathname)); return; }
    loadPool();
    loadProfile();
  }, []);

  async function loadPool() {
    try {
      const p = await investorAPI.poolInfo();
      setPool(p);
    } catch { /* pool not init */ }
    setLoading(false);
  }

  async function loadProfile() {
    try {
      const p = await investorAPI.myProfile();
      setProfile(p);
    } catch { /* not investor yet */ }
  }

  async function loadEarnings() {
    try {
      const e = await investorAPI.dailyEarnings();
      setEarnings({ today: e.today_earning, total30d: e.last_30d_total, percent: e.share_percent, daily: e.daily_earnings || [] });
    } catch { /* not investor */ }
  }

  async function loadEquity() {
    try {
      const e = await investorAPI.equity();
      setEquity(e.grants || []);
    } catch { /* not partner */ }
  }

  function switchTab(t: Tab) {
    setTab(t);
    if (t === 'earnings' && !earnings) loadEarnings();
    if (t === 'equity' && equity.length === 0) loadEquity();
    if (t === 'portfolio' && !profile) loadProfile();
  }

  async function handleRegister() {
    setErr(''); setMsg('');
    try {
      const r = await investorAPI.register();
      setMsg(r.message);
      loadProfile();
    } catch (e: any) { setErr(e.message); }
  }

  async function handleAgree() {
    setErr(''); setMsg(''); setAgreeing(true);
    try {
      const r = await investorAPI.signAgreement(agreeTerm);
      setMsg(r.message);
      loadProfile();
    } catch (e: any) { setErr(e.message); }
    setAgreeing(false);
  }

  async function handleRecharge() {
    const yuan = parseFloat(rechargeYuan);
    if (!yuan || yuan < 10000) { setErr('最低充值 ¥10,000'); return; }
    setErr(''); setMsg(''); setRecharging(true);
    try {
      const fen = Math.round(yuan * 100);
      const r = await investorAPI.recharge(fen);
      setMsg(r.message);
      setRechargeYuan('');
      loadProfile();
      loadPool();
    } catch (e: any) { setErr(e.message); }
    setRecharging(false);
  }

  async function handlePurchase() {
    const yuan = parseFloat(purchaseYuan);
    if (!yuan || yuan < 10000) { setErr('最低购买 ¥10,000'); return; }
    setErr(''); setMsg(''); setPurchasing(true); setPurchaseResult(null);
    try {
      const fen = Math.round(yuan * 100);
      const r = await investorAPI.purchase(fen, payMethod, 'pc');
      setPurchaseResult(r);
      setPurchaseYuan('');
      if (r.pay_url) {
        window.open(r.pay_url, '_blank');
      }
    } catch (e: any) { setErr(e.message); }
    setPurchasing(false);
  }

  const inv = profile?.investor;
  const isInvestor = !!inv;
  const hasSigned = inv && inv.agreement_term > 0;
  const isActivated = inv?.activated;

  const TABS: { key: Tab; label: string; icon: typeof Diamond }[] = [
    { key: 'pool', label: '星钻池', icon: Diamond },
    { key: 'portfolio', label: '我的持仓', icon: Wallet },
    { key: 'earnings', label: '收益明细', icon: BarChart3 },
    { key: 'equity', label: '合伙人期权', icon: Gem },
  ];

  if (loading) return (
    <div className="min-h-screen bg-gray-950 text-white flex items-center justify-center">
      <div className="animate-spin w-6 h-6 border-2 border-amber-400 border-t-transparent rounded-full" />
    </div>
  );

  return (
    <div className="min-h-screen bg-gray-950 text-white">
      {/* Dashboard header — standalone, no portal nav */}
      <header className="border-b border-white/10 bg-gray-950/80 backdrop-blur-sm sticky top-0 z-50">
        <div className="mx-auto max-w-5xl px-4 sm:px-6 h-14 flex items-center justify-between">
          <div className="flex items-center gap-3">
            <button onClick={() => navigate('/')} className="p-1.5 rounded-lg hover:bg-white/5 text-gray-400 hover:text-white transition" title="返回首页">
              <ArrowLeft size={18} />
            </button>
            <div className="w-8 h-8 rounded-lg bg-amber-500/10 flex items-center justify-center">
              <Diamond className="w-4 h-4 text-amber-400" />
            </div>
            <div>
              <h1 className="text-base font-bold leading-tight">星钻 · 合伙人期权</h1>
              <p className="text-[11px] text-gray-500 leading-tight">Star Diamond & Partner Equity</p>
            </div>
          </div>
          <div className="text-xs text-gray-500">{isInvestor ? (isActivated ? '✓ 已激活分润' : '待激活') : '未注册'}</div>
        </div>
      </header>

      <div className="mx-auto max-w-5xl px-4 sm:px-6 py-8">

        {/* Tabs */}
        <div className="flex gap-1 border-b border-white/10 mb-8">
          {TABS.map(t => (
            <button
              key={t.key}
              onClick={() => switchTab(t.key)}
              className={`flex items-center gap-2 px-4 py-2.5 text-sm font-medium border-b-2 transition-colors ${
                tab === t.key
                  ? 'border-amber-400 text-amber-400'
                  : 'border-transparent text-gray-500 hover:text-gray-300'
              }`}
            >
              <t.icon size={16} />
              {t.label}
            </button>
          ))}
        </div>

        {/* Messages */}
        {msg && <div className="mb-6 p-3 rounded-lg bg-green-500/10 border border-green-500/30 text-green-400 text-sm">{msg}</div>}
        {err && <div className="mb-6 p-3 rounded-lg bg-red-500/10 border border-red-500/30 text-red-400 text-sm">{err}</div>}

        {/* ═══ Pool Tab ═══ */}
        {tab === 'pool' && pool && (
          <div className="space-y-8">
            {/* Pool Stats */}
            <div className="rounded-2xl border border-amber-500/20 bg-amber-500/[0.04] p-8">
              <div className="grid grid-cols-2 md:grid-cols-4 gap-6 text-center">
                <div>
                  <div className="text-sm text-gray-400 mb-1">当前价格</div>
                  <div className="text-2xl font-bold text-white">{fmt(pool.price_yuan)}<span className="text-sm text-gray-400">/份</span></div>
                  <div className="text-xs text-amber-400 mt-0.5">{pool.price_driver}驱动</div>
                </div>
                <div>
                  <div className="text-sm text-gray-400 mb-1">NAV 净值</div>
                  <div className="text-2xl font-bold text-white">{fmt(pool.nav_yuan)}</div>
                </div>
                <div>
                  <div className="text-sm text-gray-400 mb-1">当前轮次</div>
                  <div className="text-2xl font-bold text-amber-400">{pool.current_round_label || pool.current_round}</div>
                </div>
                <div>
                  <div className="text-sm text-gray-400 mb-1">活跃合伙人</div>
                  <div className="text-2xl font-bold text-white">{pool.active_investors}</div>
                </div>
              </div>
            </div>

            {/* Supply */}
            <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
              <div className="rounded-xl border border-white/10 bg-white/[0.02] p-5">
                <div className="text-sm text-gray-400 mb-1">总供应量</div>
                <div className="text-xl font-bold text-white">{(pool.total_supply / 10000).toLocaleString()}万 份</div>
              </div>
              <div className="rounded-xl border border-white/10 bg-white/[0.02] p-5">
                <div className="text-sm text-gray-400 mb-1">已发行</div>
                <div className="text-xl font-bold text-white">{pool.issued.toLocaleString()} 份</div>
                <div className="text-xs text-gray-500">{pct(pool.issued, pool.total_supply)}</div>
              </div>
              <div className="rounded-xl border border-white/10 bg-white/[0.02] p-5">
                <div className="text-sm text-gray-400 mb-1">利润池余额</div>
                <div className="text-xl font-bold text-amber-400">{fmt(pool.pool_balance_yuan)}</div>
              </div>
            </div>

            {/* Actions for non-investors */}
            {!isInvestor && (
              <div className="rounded-xl border border-white/10 bg-white/[0.02] p-8 text-center">
                <Diamond className="w-10 h-10 text-amber-400 mx-auto mb-4" />
                <h3 className="text-lg font-semibold mb-2">注册为合伙人</h3>
                <p className="text-gray-400 text-sm mb-6">注册后签署合伙人协议 → 购买星钻 → 享受利润分成</p>
                <button onClick={handleRegister} className="px-6 py-2.5 bg-amber-600 hover:bg-amber-500 text-white rounded-lg text-sm font-medium transition inline-flex items-center gap-2">
                  <ArrowRight size={16} /> 立即注册
                </button>
              </div>
            )}

            {isInvestor && !hasSigned && (
              <div className="rounded-xl border border-amber-500/20 bg-amber-500/[0.04] p-8">
                <div className="flex items-start gap-4">
                  <FileText className="w-8 h-8 text-amber-400 shrink-0 mt-1" />
                  <div className="flex-1">
                    <h3 className="text-lg font-semibold mb-2">签署合伙人协议</h3>
                    <p className="text-gray-400 text-sm mb-4">请选择合伙期限并签署《星钻合伙人协议》，签署后可购买星钻。</p>
                    <div className="flex gap-3 mb-4">
                      {[1, 3, 5].map(t => (
                        <button
                          key={t}
                          onClick={() => setAgreeTerm(t)}
                          className={`px-4 py-2 rounded-lg text-sm font-medium transition border ${
                            agreeTerm === t
                              ? 'border-amber-400 bg-amber-500/10 text-amber-400'
                              : 'border-white/10 text-gray-400 hover:border-white/20'
                          }`}
                        >
                          {t} 年期
                        </button>
                      ))}
                    </div>
                    <button
                      onClick={handleAgree}
                      disabled={agreeing}
                      className="px-6 py-2.5 bg-amber-600 hover:bg-amber-500 text-white rounded-lg text-sm font-medium transition disabled:opacity-50"
                    >
                      {agreeing ? '签署中...' : `签署 ${agreeTerm} 年协议`}
                    </button>
                  </div>
                </div>
              </div>
            )}

            {isInvestor && hasSigned && (
              <div className="rounded-xl border border-white/10 bg-white/[0.02] p-8">
                <h3 className="text-lg font-semibold mb-4 flex items-center gap-2">
                  <Zap className="w-5 h-5 text-amber-400" />
                  购买星钻
                </h3>

                {/* Buy mode toggle */}
                <div className="flex gap-2 mb-6">
                  <button onClick={() => setBuyMode('pay')} className={`flex items-center gap-1.5 px-4 py-2 rounded-lg text-sm font-medium transition border ${
                    buyMode === 'pay' ? 'border-amber-400 bg-amber-500/10 text-amber-400' : 'border-white/10 text-gray-400 hover:border-white/20'
                  }`}><CreditCard size={14} />直接支付</button>
                  <button onClick={() => setBuyMode('balance')} className={`flex items-center gap-1.5 px-4 py-2 rounded-lg text-sm font-medium transition border ${
                    buyMode === 'balance' ? 'border-amber-400 bg-amber-500/10 text-amber-400' : 'border-white/10 text-gray-400 hover:border-white/20'
                  }`}><Wallet size={14} />余额购买</button>
                </div>

                <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
                  <div>
                    <div className="text-sm text-gray-400 mb-2">当前价格: <span className="text-white font-medium">{fmt(pool.price_yuan)}/份</span> · {pool.current_round_label || pool.current_round}</div>
                    <div className="text-sm text-gray-400 mb-4">最低购买: <span className="text-white font-medium">{fmt(pool.min_recharge_yuan)}</span></div>

                    {buyMode === 'pay' ? (
                      <>
                        {/* Payment method selector */}
                        <div className="flex gap-2 mb-3">
                          <button onClick={() => setPayMethod('alipay')} className={`flex-1 py-2 rounded-lg text-sm font-medium border transition ${
                            payMethod === 'alipay' ? 'border-blue-400 bg-blue-500/10 text-blue-400' : 'border-white/10 text-gray-400'
                          }`}>支付宝</button>
                          <button onClick={() => setPayMethod('wechatpay')} className={`flex-1 py-2 rounded-lg text-sm font-medium border transition ${
                            payMethod === 'wechatpay' ? 'border-green-400 bg-green-500/10 text-green-400' : 'border-white/10 text-gray-400'
                          }`}>微信支付</button>
                        </div>
                        <div className="flex gap-2">
                          <input type="number" value={purchaseYuan} onChange={e => setPurchaseYuan(e.target.value)} placeholder="购买金额 (元)"
                            className="flex-1 px-4 py-2.5 bg-white/5 border border-white/10 rounded-lg text-white placeholder-gray-500 focus:outline-none focus:border-amber-500/50" />
                          <button onClick={handlePurchase} disabled={purchasing}
                            className="px-6 py-2.5 bg-amber-600 hover:bg-amber-500 text-white rounded-lg text-sm font-medium transition disabled:opacity-50 shrink-0 flex items-center gap-1.5">
                            <ShoppingCart size={14} />{purchasing ? '创建中...' : '支付购买'}
                          </button>
                        </div>
                        {purchaseResult && (
                          <div className="mt-3 p-3 rounded-lg bg-blue-500/10 border border-blue-500/30 text-sm">
                            <div className="text-blue-400 font-medium mb-1">订单已创建: {purchaseResult.order_no}</div>
                            <div className="text-gray-400 text-xs mb-2">{purchaseResult.shares} 份 @ ¥{purchaseResult.price_yuan}/份 = ¥{purchaseResult.amount_yuan}</div>
                            {purchaseResult.pay_url && (
                              <a href={purchaseResult.pay_url} target="_blank" rel="noopener noreferrer"
                                className="inline-flex items-center gap-1 px-4 py-2 bg-blue-600 hover:bg-blue-500 text-white rounded-lg text-sm font-medium transition">
                                <ArrowRight size={14} />前往支付
                              </a>
                            )}
                          </div>
                        )}
                        <div className="text-xs text-gray-600 mt-2">通过{payMethod === 'alipay' ? '支付宝' : '微信'}直接支付，到账自动发放星钻。</div>
                      </>
                    ) : (
                      <>
                        <div className="flex gap-2">
                          <input type="number" value={rechargeYuan} onChange={e => setRechargeYuan(e.target.value)} placeholder="购买金额 (元)"
                            className="flex-1 px-4 py-2.5 bg-white/5 border border-white/10 rounded-lg text-white placeholder-gray-500 focus:outline-none focus:border-amber-500/50" />
                          <button onClick={handleRecharge} disabled={recharging}
                            className="px-6 py-2.5 bg-amber-600 hover:bg-amber-500 text-white rounded-lg text-sm font-medium transition disabled:opacity-50 shrink-0">
                            {recharging ? '购买中...' : '余额购买'}
                          </button>
                        </div>
                        <div className="text-xs text-gray-600 mt-2">从平台余额扣款。累计充值 ≥ {fmt(pool.activation_yuan)} 激活分润。</div>
                      </>
                    )}
                  </div>
                  <div className="space-y-2">
                    <div className="flex justify-between text-sm">
                      <span className="text-gray-400">已投资</span>
                      <span className="text-white font-medium">{fmtFen(inv!.total_invested)}</span>
                    </div>
                    <div className="flex justify-between text-sm">
                      <span className="text-gray-400">持有星钻</span>
                      <span className="text-white font-medium">{inv!.shares.toLocaleString()} 份</span>
                    </div>
                    <div className="flex justify-between text-sm">
                      <span className="text-gray-400">分润状态</span>
                      <span className={isActivated ? 'text-green-400 font-medium' : 'text-gray-500'}>
                        {isActivated ? '✓ 已激活' : `还需 ${fmtFen(Math.max(0, 10000000 - inv!.total_invested))}`}
                      </span>
                    </div>
                    <div className="flex justify-between text-sm">
                      <span className="text-gray-400">协议期限</span>
                      <span className="text-white">{inv!.agreement_term} 年 (至 {inv!.agreement_expires_at?.slice(0, 10)})</span>
                    </div>
                  </div>
                </div>
              </div>
            )}
          </div>
        )}

        {/* ═══ Portfolio Tab ═══ */}
        {tab === 'portfolio' && (
          <div className="space-y-6">
            {!isInvestor ? (
              <div className="rounded-xl border border-white/10 bg-white/[0.02] p-12 text-center">
                <Wallet className="w-10 h-10 text-gray-600 mx-auto mb-3" />
                <div className="text-gray-500">请先注册为合伙人</div>
              </div>
            ) : (
              <>
                {/* Summary cards */}
                <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
                  <div className="rounded-xl border border-white/10 bg-white/[0.02] p-5 text-center">
                    <div className="text-sm text-gray-400 mb-1">持有星钻</div>
                    <div className="text-2xl font-bold text-white">{inv!.shares.toLocaleString()}</div>
                    <div className="text-xs text-gray-500">份</div>
                  </div>
                  <div className="rounded-xl border border-white/10 bg-white/[0.02] p-5 text-center">
                    <div className="text-sm text-gray-400 mb-1">持仓价值</div>
                    <div className="text-2xl font-bold text-amber-400">{fmt(profile!.portfolio_yuan)}</div>
                  </div>
                  <div className="rounded-xl border border-white/10 bg-white/[0.02] p-5 text-center">
                    <div className="text-sm text-gray-400 mb-1">占比</div>
                    <div className="text-2xl font-bold text-white">{profile!.share_percent}</div>
                  </div>
                  <div className="rounded-xl border border-white/10 bg-white/[0.02] p-5 text-center">
                    <div className="text-sm text-gray-400 mb-1">累计分红</div>
                    <div className="text-2xl font-bold text-green-400">{fmtFen(inv!.total_dividends)}</div>
                  </div>
                </div>

                {/* Status */}
                <div className="rounded-xl border border-white/10 bg-white/[0.02] p-6">
                  <h3 className="text-sm font-semibold text-gray-300 mb-4">合伙人信息</h3>
                  <div className="grid grid-cols-2 gap-4 text-sm">
                    <div className="flex justify-between"><span className="text-gray-500">姓名</span><span className="text-white">{inv!.name || '-'}</span></div>
                    <div className="flex justify-between"><span className="text-gray-500">来源</span><span className="text-white">{inv!.source}</span></div>
                    <div className="flex justify-between"><span className="text-gray-500">总投资</span><span className="text-white font-medium">{fmtFen(inv!.total_invested)}</span></div>
                    <div className="flex justify-between"><span className="text-gray-500">分润</span><span className={isActivated ? 'text-green-400' : 'text-gray-500'}>{isActivated ? '已激活' : '未激活'}</span></div>
                    <div className="flex justify-between"><span className="text-gray-500">协议</span><span className="text-white">{inv!.agreement_term > 0 ? `${inv!.agreement_term}年期` : '未签署'}</span></div>
                    <div className="flex justify-between"><span className="text-gray-500">加入时间</span><span className="text-white">{inv!.joined_at?.slice(0, 10)}</span></div>
                  </div>
                </div>

                {/* Transactions */}
                {profile!.transactions?.length > 0 && (
                  <div className="rounded-xl border border-white/10 bg-white/[0.02] p-6">
                    <h3 className="text-sm font-semibold text-gray-300 mb-4">交易记录</h3>
                    <div className="space-y-2">
                      {profile!.transactions.map((tx: any) => (
                        <div key={tx.id} className="flex items-center justify-between py-2 border-b border-white/5 text-sm">
                          <div>
                            <span className={`inline-block px-2 py-0.5 rounded text-xs mr-2 ${
                              tx.type === 'seed_grant' ? 'bg-purple-500/10 text-purple-400' :
                              tx.type === 'recharge' ? 'bg-green-500/10 text-green-400' :
                              'bg-gray-500/10 text-gray-400'
                            }`}>{tx.type}</span>
                            <span className="text-gray-400">{tx.remark}</span>
                          </div>
                          <div className="text-right">
                            <div className="text-white font-medium">{tx.shares > 0 ? '+' : ''}{tx.shares.toLocaleString()} 份</div>
                            {tx.amount > 0 && <div className="text-xs text-gray-500">{fmtFen(tx.amount)}</div>}
                          </div>
                        </div>
                      ))}
                    </div>
                  </div>
                )}

                {/* Dividends */}
                {profile!.dividends?.length > 0 && (
                  <div className="rounded-xl border border-white/10 bg-white/[0.02] p-6">
                    <h3 className="text-sm font-semibold text-gray-300 mb-4">分红记录</h3>
                    <div className="space-y-2">
                      {profile!.dividends.map((d: any) => (
                        <div key={d.id} className="flex items-center justify-between py-2 border-b border-white/5 text-sm">
                          <div className="text-gray-400">{d.period}</div>
                          <div className="flex items-center gap-4">
                            <span className="text-gray-500 text-xs">占比 {(d.share_ratio * 100).toFixed(2)}%</span>
                            <span className="text-green-400 font-medium">+{fmtFen(d.amount)}</span>
                          </div>
                        </div>
                      ))}
                    </div>
                  </div>
                )}
              </>
            )}
          </div>
        )}

        {/* ═══ Earnings Tab ═══ */}
        {tab === 'earnings' && (
          <div className="space-y-6">
            {!isInvestor ? (
              <div className="rounded-xl border border-white/10 bg-white/[0.02] p-12 text-center">
                <BarChart3 className="w-10 h-10 text-gray-600 mx-auto mb-3" />
                <div className="text-gray-500">请先注册为合伙人</div>
              </div>
            ) : !earnings ? (
              <div className="flex items-center justify-center py-12">
                <div className="animate-spin w-6 h-6 border-2 border-amber-400 border-t-transparent rounded-full" />
              </div>
            ) : (
              <>
                {/* Earnings summary */}
                <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
                  <div className="rounded-xl border border-amber-500/20 bg-amber-500/[0.04] p-6 text-center">
                    <div className="text-sm text-gray-400 mb-1">今日预估收益</div>
                    <div className="text-3xl font-bold text-amber-400">{fmt(earnings.today)}</div>
                  </div>
                  <div className="rounded-xl border border-white/10 bg-white/[0.02] p-6 text-center">
                    <div className="text-sm text-gray-400 mb-1">近 30 天累计</div>
                    <div className="text-3xl font-bold text-white">{fmt(earnings.total30d)}</div>
                  </div>
                  <div className="rounded-xl border border-white/10 bg-white/[0.02] p-6 text-center">
                    <div className="text-sm text-gray-400 mb-1">份额占比</div>
                    <div className="text-3xl font-bold text-white">{earnings.percent}</div>
                  </div>
                </div>

                {/* Daily chart (simple bar) */}
                {earnings.daily.length > 0 && (
                  <div className="rounded-xl border border-white/10 bg-white/[0.02] p-6">
                    <h3 className="text-sm font-semibold text-gray-300 mb-4">每日收益 (近30天)</h3>
                    <div className="space-y-1.5">
                      {earnings.daily.slice(0, 14).map(d => {
                        const maxVal = Math.max(...earnings.daily.map(e => e.my_yuan), 1);
                        const w = Math.max(2, (d.my_yuan / maxVal) * 100);
                        return (
                          <div key={d.date} className="flex items-center gap-3 text-sm">
                            <span className="text-gray-500 w-20 shrink-0 text-xs">{d.date.slice(5)}</span>
                            <div className="flex-1 h-5 bg-white/5 rounded-full overflow-hidden">
                              <div className="h-full bg-amber-500/30 rounded-full" style={{ width: `${w}%` }} />
                            </div>
                            <span className="text-white font-medium w-24 text-right text-xs">{fmt(d.my_yuan)}</span>
                          </div>
                        );
                      })}
                    </div>
                  </div>
                )}

                {earnings.daily.length === 0 && (
                  <div className="rounded-xl border border-white/10 bg-white/[0.02] p-12 text-center text-gray-500 text-sm">
                    暂无收益数据
                  </div>
                )}
              </>
            )}
          </div>
        )}

        {/* ═══ Equity Tab ═══ */}
        {tab === 'equity' && (
          <div className="space-y-6">
            <div className="rounded-xl border border-white/10 bg-white/[0.02] p-6">
              <h3 className="text-sm font-semibold text-gray-300 mb-2">合伙人期权 (Equity Grant)</h3>
              <p className="text-xs text-gray-500 mb-6">核心合伙人期权授予记录。期权按月线性归属，设有 cliff 锁定期。</p>

              {equity.length === 0 ? (
                <div className="text-center py-8">
                  <Lock className="w-8 h-8 text-gray-600 mx-auto mb-3" />
                  <div className="text-sm text-gray-500">暂无期权授予</div>
                  <div className="text-xs text-gray-600 mt-1">成为核心合伙人后由管理员授予</div>
                </div>
              ) : (
                <div className="space-y-4">
                  {equity.map(g => {
                    const vestPct = g.total_shares > 0 ? (g.vested_shares / g.total_shares * 100) : 0;
                    const isCliff = new Date() < new Date(g.cliff_date);
                    return (
                      <div key={g.id} className="rounded-xl border border-white/10 bg-white/[0.03] p-6">
                        <div className="flex items-start justify-between mb-4">
                          <div>
                            <div className="flex items-center gap-2 mb-1">
                              <Gem className="w-4 h-4 text-amber-400" />
                              <span className="text-white font-semibold">{g.total_shares.toLocaleString()} 份期权</span>
                              <span className={`px-2 py-0.5 rounded text-xs ${
                                g.status === 'active' ? 'bg-green-500/10 text-green-400' :
                                g.status === 'exercised' ? 'bg-blue-500/10 text-blue-400' :
                                'bg-gray-500/10 text-gray-400'
                              }`}>
                                {g.status === 'active' ? '生效中' : g.status === 'exercised' ? '已行权' : '已取消'}
                              </span>
                            </div>
                            <div className="text-xs text-gray-500">
                              授予于 {g.grant_date?.slice(0, 10)} · Cliff {g.cliff_months}个月 · 归属 {g.vesting_months}个月
                            </div>
                          </div>
                          {g.strike_price > 0 && (
                            <div className="text-right">
                              <div className="text-xs text-gray-400">行权价</div>
                              <div className="text-white font-medium">{fmt(g.strike_price)}</div>
                            </div>
                          )}
                        </div>

                        {/* Vesting progress */}
                        <div className="mb-3">
                          <div className="flex justify-between text-xs mb-1">
                            <span className="text-gray-400">已归属 {g.vested_shares.toLocaleString()} / {g.total_shares.toLocaleString()}</span>
                            <span className="text-amber-400 font-medium">{vestPct.toFixed(1)}%</span>
                          </div>
                          <div className="h-2 bg-white/5 rounded-full overflow-hidden">
                            <div className="h-full bg-gradient-to-r from-amber-500 to-orange-400 rounded-full transition-all" style={{ width: `${vestPct}%` }} />
                          </div>
                        </div>

                        {/* Timeline */}
                        <div className="flex items-center gap-4 text-xs">
                          <div className="flex items-center gap-1 text-gray-500">
                            <Calendar size={12} />
                            <span>Cliff: {g.cliff_date?.slice(0, 10)}</span>
                            {isCliff ? <Lock size={10} className="text-red-400" /> : <Unlock size={10} className="text-green-400" />}
                          </div>
                          <div className="flex items-center gap-1 text-gray-500">
                            <CheckCircle size={12} />
                            <span>全归属: {g.full_vest_date?.slice(0, 10)}</span>
                          </div>
                        </div>
                      </div>
                    );
                  })}
                </div>
              )}
            </div>
          </div>
        )}
      </div>

      {/* Minimal footer */}
      <div className="border-t border-white/5 mt-12 py-6 text-center text-xs text-gray-600">
        StarClaw · 星钻合伙人系统
      </div>
    </div>
  );
}
