import { useState, useEffect, useRef } from 'react';
import { investorAPI, clawAuthAPI, clawNodeRequest, type InvestorPoolInfo, type InvestorProfile, type DailyEarning, type EquityGrant, type DiamondOrderResult } from '../lib/api';
import { isLoggedIn, setAuth, clearAuth, getUser } from '../lib/auth';
import {
  Diamond, Wallet, ArrowRight, CheckCircle, ExternalLink, LogOut, CreditCard, Smartphone,
  FileText, BarChart3, Gem, Zap, Lock, Unlock, Calendar, Fingerprint, Shield,
} from 'lucide-react';

function fmt(yuan: number) { return `¥${yuan.toLocaleString('zh-CN', { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`; }
function fmtFen(fen: number) { return fmt(fen / 100); }
function pct(n: number, d: number) { return d > 0 ? `${(n / d * 100).toFixed(2)}%` : '0%'; }

export function InvestPage() {
  // ─── Auth ───
  const [authed, setAuthed] = useState(isLoggedIn());
  const [clawUrl, setClawUrl] = useState('');
  const [loginErr, setLoginErr] = useState('');
  const [loggingIn, setLoggingIn] = useState(false);
  const [loginStep, setLoginStep] = useState<'input' | 'connecting' | 'waiting' | 'verifying' | 'done' | 'error'>('input');
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null);

  // ─── Data ───
  const [pool, setPool] = useState<InvestorPoolInfo | null>(null);
  const [profile, setProfile] = useState<InvestorProfile | null>(null);
  const [earnings, setEarnings] = useState<{ today: number; total30d: number; percent: string; daily: DailyEarning[] } | null>(null);
  const [equity, setEquity] = useState<EquityGrant[]>([]);
  const [loading, setLoading] = useState(true);
  const [msg, setMsg] = useState('');
  const [err, setErr] = useState('');

  // Purchase (Alipay / WeChat)
  const [purchaseYuan, setPurchaseYuan] = useState('');
  const [purchasing, setPurchasing] = useState(false);
  const [payMethod, setPayMethod] = useState<'alipay' | 'wechatpay'>('alipay');
  const [orderResult, setOrderResult] = useState<DiamondOrderResult | null>(null);
  const [wxQr, setWxQr] = useState<{ orderNo: string; codeUrl: string; amount: string } | null>(null);

  // Agreement
  const [agreeTerm, setAgreeTerm] = useState(3);
  const [agreeing, setAgreeing] = useState(false);

  // ─── Auto-login from URL ?token=xxx ───
  useEffect(() => {
    const params = new URLSearchParams(window.location.search);
    const token = params.get('token');
    if (token) {
      window.history.replaceState({}, '', window.location.pathname);
      verifyAndLogin(token);
    }
  }, []);

  // ─── Load all data when authed ───
  useEffect(() => {
    if (authed) loadAll();
  }, [authed]);

  async function handleClawLogin() {
    if (!clawUrl.trim()) { setLoginErr('请输入 Claw 节点地址'); return; }
    setLoginStep('connecting'); setLoginErr(''); setLoggingIn(true);
    const baseUrl = new URL(clawUrl.includes('://') ? clawUrl : `https://${clawUrl}`).origin;
    try {
      await clawNodeRequest<{ node_id: string }>(baseUrl, '/v1/identity/info');
      const { challenge } = await clawAuthAPI.challenge();
      const reqRes = await clawNodeRequest<{ id: string }>(
        baseUrl, '/v1/identity/auth-request',
        { method: 'POST', body: JSON.stringify({ challenge, origin: window.location.hostname }) }
      );
      setLoginStep('waiting');

      await new Promise<void>((resolve, reject) => {
        let attempts = 0;
        pollRef.current = setInterval(async () => {
          attempts++;
          if (attempts > 100) { clearInterval(pollRef.current!); reject(new Error('授权超时')); return; }
          try {
            const s = await clawNodeRequest<{
              status: string; node_id?: string; public_key?: string; signature?: string; challenge?: string;
            }>(baseUrl, `/v1/identity/auth-request/${reqRes.id}`);
            if (s.status === 'approved') {
              clearInterval(pollRef.current!);
              setLoginStep('verifying');
              const data = await clawAuthAPI.verify({
                challenge: s.challenge!, node_id: s.node_id!,
                public_key: s.public_key!, signature: s.signature!,
              });
              setAuth(data.token, data.user);
              setLoginStep('done');
              setAuthed(true);
              resolve();
            } else if (s.status === 'rejected') {
              clearInterval(pollRef.current!); reject(new Error('授权被拒绝'));
            }
          } catch (e: any) {
            if (e.message?.includes('expired') || e.message?.includes('not found')) {
              clearInterval(pollRef.current!); reject(new Error('请求已过期'));
            }
          }
        }, 3000);
      });
    } catch (e: any) {
      if (pollRef.current) clearInterval(pollRef.current);
      setLoginStep('error');
      setLoginErr(e.message || '连接失败');
    }
    setLoggingIn(false);
  }

  async function verifyAndLogin(token: string) {
    setLoggingIn(true); setLoginErr('');
    try {
      const apiBase = import.meta.env.VITE_API_BASE || '/api';
      const res = await fetch(`${apiBase}/user/profile`, {
        headers: { Authorization: `Bearer ${token}` },
      });
      if (!res.ok) throw new Error('令牌无效或已过期');
      const raw = await res.json();
      const u = raw.data?.user || raw.user || raw;
      setAuth(token, { id: u.id, email: u.email || '', nickname: u.nickname || '', phone: u.phone || '', avatar: u.avatar || '', role: u.role || 'user' });
      setAuthed(true);
    } catch (e: any) { setLoginErr(e.message); }
    setLoggingIn(false);
  }

  function handleLogout() {
    clearAuth();
    setAuthed(false);
    setPool(null); setProfile(null); setEarnings(null); setEquity([]);
  }

  async function loadAll() {
    setLoading(true);
    await Promise.all([loadPool(), loadProfile(), loadEarnings(), loadEquity()]);
    setLoading(false);
  }

  async function loadPool() {
    try { setPool(await investorAPI.poolInfo()); } catch { /* not init */ }
  }
  async function loadProfile() {
    try { setProfile(await investorAPI.myProfile()); } catch { /* not investor */ }
  }
  async function loadEarnings() {
    try {
      const e = await investorAPI.dailyEarnings();
      setEarnings({ today: e.today_earning, total30d: e.last_30d_total, percent: e.share_percent, daily: e.daily_earnings || [] });
    } catch { /* no data */ }
  }
  async function loadEquity() {
    try { const e = await investorAPI.equity(); setEquity(e.grants || []); } catch { /* no grants */ }
  }

  async function handleRegister() {
    setErr(''); setMsg('');
    try { const r = await investorAPI.register(); setMsg(r.message); loadProfile(); } catch (e: any) { setErr(e.message); }
  }

  async function handleAgree() {
    setErr(''); setMsg(''); setAgreeing(true);
    try { const r = await investorAPI.signAgreement(agreeTerm); setMsg(r.message); loadProfile(); } catch (e: any) { setErr(e.message); }
    setAgreeing(false);
  }

  const myMaxInvest = profile?.max_invest_yuan ?? pool?.max_invest_yuan ?? 50000;
  const myMinInvest = pool?.min_invest_yuan ?? 10000;
  const tierLabel = profile?.tier_label || '合伙人';

  async function handlePurchase() {
    const yuan = parseFloat(purchaseYuan);
    if (!pool) return;
    if (!yuan || yuan < myMinInvest) { setErr(`${pool.current_round_label} 最低购买 ${fmt(myMinInvest)}`); return; }
    if (yuan > myMaxInvest) { setErr(`${tierLabel}单笔上限 ${fmt(myMaxInvest)}`); return; }
    setErr(''); setMsg(''); setPurchasing(true);
    try {
      const fen = Math.round(yuan * 100);
      const r = await investorAPI.purchase(fen, payMethod, 'pc');
      setOrderResult(r);
      // Redirect to payment
      if (r.pay_url) {
        window.open(r.pay_url, '_blank');
        setMsg(`订单已创建 (${r.order_no})，请在新窗口完成支付`);
      } else if (r.code_url) {
        setWxQr({ orderNo: r.order_no, codeUrl: r.code_url, amount: fmt(r.amount_yuan) });
        setMsg(`微信支付二维码已生成 (${r.order_no})`);
      }
      setPurchaseYuan('');
    } catch (e: any) { setErr(e.message); }
    setPurchasing(false);
  }

  async function pollOrder(orderNo: string) {
    try {
      const o = await investorAPI.queryOrder(orderNo);
      if (o.status === 'paid') {
        setMsg('支付成功！星钻已到账');
        setOrderResult(null);
        loadProfile(); loadPool();
      }
    } catch { /* ignore */ }
  }

  const inv = profile?.investor;
  const isInvestor = !!inv;
  const hasSigned = inv && inv.agreement_term > 0;
  const isActivated = inv?.activated;
  const user = getUser();

  // ═══════════════════════════════════════════
  // LOGIN GATE
  // ═══════════════════════════════════════════
  if (!authed) return (
    <div className="min-h-screen bg-[#0a0a14] flex items-center justify-center relative overflow-hidden">
      <div className="absolute inset-0 bg-[radial-gradient(ellipse_at_center,rgba(139,92,246,0.08),transparent_60%)]" />
      <div className="absolute top-1/4 left-1/2 -translate-x-1/2 -translate-y-1/2 w-[600px] h-[600px] bg-purple-500/5 rounded-full blur-[120px]" />
      <div className="relative z-10 w-full max-w-md px-6">
        <div className="text-center mb-10">
          <div className="w-16 h-16 mx-auto mb-5 rounded-2xl bg-purple-500/15 border border-purple-500/20 flex items-center justify-center">
            <Diamond className="w-8 h-8 text-purple-400" />
          </div>
          <h1 className="text-2xl font-bold bg-gradient-to-r from-purple-300 to-fuchsia-300 bg-clip-text text-transparent mb-2">星钻合伙人</h1>
          <p className="text-sm text-gray-500">Star Diamond Partnership</p>
        </div>
        <div className="rounded-2xl border border-purple-500/15 bg-white/[0.02] backdrop-blur-sm p-8">
          <div className="flex items-center gap-2 mb-6">
            <Fingerprint className="w-5 h-5 text-purple-400" />
            <span className="text-sm font-medium text-gray-300">Claw 节点认证</span>
          </div>

          {loginStep === 'input' && (
            <>
              {loginErr && <div className="mb-4 p-3 rounded-lg bg-red-500/10 border border-red-500/30 text-red-400 text-sm">{loginErr}</div>}
              <div className="mb-2">
                <label className="block text-xs text-gray-500 mb-1.5">Claw 节点地址</label>
                <input
                  type="url"
                  value={clawUrl}
                  onChange={e => setClawUrl(e.target.value)}
                  onKeyDown={e => e.key === 'Enter' && handleClawLogin()}
                  placeholder="如 https://app.starclaw.me"
                  className="w-full px-4 py-3 bg-white/5 border border-white/10 rounded-xl text-white placeholder-gray-500 focus:outline-none focus:border-purple-500/50"
                />
              </div>
              <div className="flex items-start gap-2 p-3 bg-purple-500/[0.06] rounded-xl mb-4">
                <Shield className="w-4 h-4 text-purple-400 mt-0.5 flex-none" />
                <p className="text-xs text-purple-300/70 leading-relaxed">
                  安全回签：请求会发送到你的 Claw 节点，你需要在 Claw 界面确认授权后才能登录。
                </p>
              </div>
              <button onClick={handleClawLogin} disabled={loggingIn}
                className="w-full py-3 bg-purple-600 hover:bg-purple-500 text-white rounded-xl font-medium transition disabled:opacity-50 flex items-center justify-center gap-2">
                <Fingerprint size={16} />发送认证请求
              </button>
            </>
          )}

          {loginStep === 'connecting' && (
            <p className="text-sm text-gray-400 text-center py-6">连接 Claw 节点...</p>
          )}

          {loginStep === 'waiting' && (
            <div className="text-center py-6">
              <p className="text-sm text-purple-400 font-medium">请在你的 Claw 界面确认授权登录</p>
              <p className="text-xs text-gray-500 mt-2">在 Claw 界面点击「授权登录」</p>
            </div>
          )}

          {loginStep === 'verifying' && (
            <p className="text-sm text-gray-400 text-center py-6">验证身份中...</p>
          )}

          {loginStep === 'done' && (
            <p className="text-sm text-green-400 text-center py-6">认证成功</p>
          )}

          {loginStep === 'error' && (
            <div className="space-y-3 py-2">
              <p className="text-sm text-red-400 text-center">{loginErr}</p>
              <button onClick={() => { setLoginStep('input'); setLoginErr(''); }}
                className="w-full py-2.5 rounded-xl border border-white/10 text-sm text-gray-400 hover:bg-white/5 transition">
                重试
              </button>
            </div>
          )}

          <p className="mt-5 pt-5 border-t border-white/5 text-center text-xs text-gray-600">
            没有 Claw 节点？<a href="https://starclaw.me/create" target="_blank" rel="noopener noreferrer" className="text-purple-400 hover:underline">免费部署一个</a>
          </p>
        </div>
      </div>
    </div>
  );

  // ═══════════════════════════════════════════
  // LOADING
  // ═══════════════════════════════════════════
  if (loading) return (
    <div className="min-h-screen bg-[#0a0a14] text-white flex items-center justify-center">
      <div className="animate-spin w-6 h-6 border-2 border-purple-500 border-t-transparent rounded-full" />
    </div>
  );

  return (
    <div className="min-h-screen bg-[#0a0a14] text-white">
      {/* ═══ Header ═══ */}
      <header className="relative border-b border-purple-500/10 bg-gradient-to-r from-[#0d0b1a] via-[#130f24] to-[#0d0b1a] sticky top-0 z-50">
        <div className="absolute inset-0 bg-[radial-gradient(ellipse_at_center,rgba(139,92,246,0.06),transparent_70%)]" />
        <div className="relative mx-auto max-w-7xl px-4 sm:px-6 h-16 flex items-center justify-between">
          <div className="flex items-center gap-3">
            <div className="w-9 h-9 rounded-xl bg-purple-500/15 border border-purple-500/20 flex items-center justify-center">
              <Diamond className="w-4.5 h-4.5 text-purple-400" />
            </div>
            <div>
              <h1 className="text-base font-bold leading-tight bg-gradient-to-r from-purple-300 to-fuchsia-300 bg-clip-text text-transparent">星钻合伙人</h1>
              <p className="text-[11px] text-gray-500 leading-tight">Star Diamond Partnership</p>
            </div>
          </div>
          <div className="flex items-center gap-3">
            <div className={`text-xs px-2.5 py-1 rounded-full border ${
              isActivated ? 'border-green-500/30 bg-green-500/10 text-green-400' :
              isInvestor ? 'border-purple-500/30 bg-purple-500/10 text-purple-400' :
              'border-white/10 bg-white/5 text-gray-500'
            }`}>{isInvestor ? (isActivated ? '✧ 分润已激活' : '○ 待激活') : '○ 未注册'}</div>
            <div className="text-xs text-gray-500 hidden sm:block">{user?.nickname || user?.email || ''}</div>
            <button onClick={handleLogout} className="p-1.5 rounded-lg hover:bg-red-500/10 text-gray-500 hover:text-red-400 transition" title="退出登录">
              <LogOut size={16} />
            </button>
          </div>
        </div>
      </header>

      <div className="mx-auto max-w-7xl px-4 sm:px-6 py-10 space-y-10">

        {/* Messages */}
        {msg && <div className="p-3 rounded-lg bg-green-500/10 border border-green-500/30 text-green-400 text-sm">{msg}</div>}
        {err && <div className="p-3 rounded-lg bg-red-500/10 border border-red-500/30 text-red-400 text-sm">{err}</div>}

        {pool && !pool.payment_available && (
          <div className="p-3 rounded-lg bg-amber-500/10 border border-amber-500/30 text-amber-400 text-sm">
            支付通道配置中，购买功能暂未开放。如需购买星钻请联系管理员。
          </div>
        )}

        {pool && (<>

        {/* ═══ Section 1: Pool Stats Hero ═══ */}
        <div className="relative rounded-2xl border border-purple-500/20 bg-gradient-to-br from-purple-500/[0.06] to-transparent p-8 overflow-hidden">
          <div className="absolute top-0 right-0 w-40 h-40 bg-purple-500/5 rounded-full blur-3xl -translate-y-1/2 translate-x-1/4" />
          <div className="relative grid grid-cols-2 md:grid-cols-4 gap-6 text-center">
            <div>
              <div className="text-sm text-gray-400 mb-1">当前价格</div>
              <div className="text-2xl font-bold text-white">{fmt(pool.price_yuan)}<span className="text-sm text-gray-400">/份</span></div>
              <div className="text-xs text-purple-400 mt-0.5">{pool.price_driver}驱动</div>
            </div>
            <div>
              <div className="text-sm text-gray-400 mb-1">NAV 净值</div>
              <div className="text-2xl font-bold text-white">{fmt(pool.nav_yuan)}</div>
            </div>
            <div>
              <div className="text-sm text-gray-400 mb-1">当前期</div>
              <div className="text-2xl font-bold text-purple-400">{pool.current_round_label || pool.current_round}</div>
            </div>
            <div>
              <div className="text-sm text-gray-400 mb-1">活跃合伙人</div>
              <div className="text-2xl font-bold text-white">{pool.active_investors}</div>
            </div>
          </div>
        </div>

        {/* ═══ Section 2: Supply Overview ═══ */}
        <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
          <div className="rounded-xl border border-purple-500/10 bg-white/[0.02] p-5">
            <div className="text-sm text-gray-500 mb-1">本期释放</div>
            <div className="text-xl font-bold text-white">{(pool.round_supply / 10000).toLocaleString()}万 <span className="text-sm font-normal text-gray-500">份</span></div>
            <div className="text-xs text-gray-600 mt-0.5">总供应 {(pool.total_supply / 10000).toLocaleString()}万份</div>
          </div>
          <div className="rounded-xl border border-purple-500/10 bg-white/[0.02] p-5">
            <div className="text-sm text-gray-500 mb-1">已孵化</div>
            <div className="text-xl font-bold text-white">{pool.round_issued.toLocaleString()} <span className="text-sm font-normal text-gray-500">份</span></div>
            <div className="text-xs text-purple-400/60 mt-0.5">{pct(pool.round_issued, pool.round_supply)}</div>
          </div>
          <div className="rounded-xl border border-purple-500/10 bg-white/[0.02] p-5">
            <div className="text-sm text-gray-500 mb-1">利润池余额</div>
            <div className="text-xl font-bold text-purple-400">{fmt(pool.pool_balance_yuan)}</div>
          </div>
        </div>

        {/* ═══ Section 3: Join / Agreement / Purchase ═══ */}
        {!isInvestor && (
          <div className="rounded-2xl border border-purple-500/15 bg-gradient-to-b from-purple-500/[0.04] to-transparent p-12 text-center">
            <Diamond className="w-14 h-14 text-purple-400 mx-auto mb-5" />
            <h2 className="text-xl font-bold mb-2">加入虫巢 · 成为合伙人</h2>
            <p className="text-gray-400 text-sm mb-8 max-w-md mx-auto">签署合伙人协议 → 购买星钻 → 分享虫巢利润。成为 StarClaw 生态的团队合伙人。</p>
            <button onClick={handleRegister} className="px-8 py-3 bg-purple-600 hover:bg-purple-500 text-white rounded-xl font-medium transition inline-flex items-center gap-2">
              <ArrowRight size={18} /> 立即注册
            </button>
          </div>
        )}

        {isInvestor && !hasSigned && (
          <div className="rounded-2xl border border-purple-500/20 bg-purple-500/[0.04] p-8">
            <div className="flex items-start gap-4">
              <FileText className="w-8 h-8 text-purple-400 shrink-0 mt-1" />
              <div className="flex-1">
                <h2 className="text-lg font-bold mb-2">签署合伙人协议</h2>
                <p className="text-gray-400 text-sm mb-5">请选择合伙期限并签署《星钻合伙人协议》，签署后可购买星钻。</p>
                <div className="flex gap-3 mb-5">
                  {[1, 3, 5].map(t => (
                    <button key={t} onClick={() => setAgreeTerm(t)}
                      className={`px-5 py-2.5 rounded-xl text-sm font-medium transition border ${agreeTerm === t ? 'border-purple-400 bg-purple-500/10 text-purple-400' : 'border-white/10 text-gray-400 hover:border-white/20'}`}>
                      {t} 年期
                    </button>
                  ))}
                </div>
                <button onClick={handleAgree} disabled={agreeing}
                  className="px-8 py-2.5 bg-purple-600 hover:bg-purple-500 text-white rounded-xl font-medium transition disabled:opacity-50">
                  {agreeing ? '签署中...' : `签署 ${agreeTerm} 年协议`}
                </button>
              </div>
            </div>
          </div>
        )}

        {isInvestor && hasSigned && (
          <div className="rounded-2xl border border-purple-500/15 bg-white/[0.02] p-8">
            <h2 className="text-lg font-bold mb-5 flex items-center gap-2">
              <Zap className="w-5 h-5 text-purple-400" />购买星钻
            </h2>
            <div className="grid grid-cols-1 lg:grid-cols-5 gap-6">
              <div className="lg:col-span-3">
                <div className="grid grid-cols-3 gap-3 mb-4 text-sm">
                  <div className="rounded-lg border border-purple-500/10 bg-white/[0.02] p-3 text-center">
                    <div className="text-gray-500 text-xs mb-0.5">当前价格</div>
                    <div className="text-white font-bold">{fmt(pool.price_yuan)}<span className="text-gray-500 font-normal">/份</span></div>
                  </div>
                  <div className="rounded-lg border border-purple-500/10 bg-white/[0.02] p-3 text-center">
                    <div className="text-gray-500 text-xs mb-0.5">单笔限额 ({tierLabel})</div>
                    <div className="text-purple-400 font-bold">{fmt(myMinInvest)} - {fmt(myMaxInvest)}</div>
                  </div>
                  <div className="rounded-lg border border-purple-500/10 bg-white/[0.02] p-3 text-center">
                    <div className="text-gray-500 text-xs mb-0.5">当前期</div>
                    <div className="text-purple-400 font-bold">{pool.current_round_label || pool.current_round}</div>
                  </div>
                </div>

                <div className="flex gap-2 mb-3">
                  <button onClick={() => setPayMethod('alipay')}
                    className={`flex items-center gap-1.5 px-4 py-2 rounded-lg text-sm font-medium transition border ${payMethod === 'alipay' ? 'border-blue-400 bg-blue-500/10 text-blue-400' : 'border-white/10 text-gray-400 hover:border-white/20'}`}>
                    <CreditCard size={14} />支付宝
                  </button>
                  <button onClick={() => setPayMethod('wechatpay')}
                    className={`flex items-center gap-1.5 px-4 py-2 rounded-lg text-sm font-medium transition border ${payMethod === 'wechatpay' ? 'border-green-400 bg-green-500/10 text-green-400' : 'border-white/10 text-gray-400 hover:border-white/20'}`}>
                    <Smartphone size={14} />微信支付
                  </button>
                </div>

                <div className="flex gap-2 mb-3">
                  <input type="number" value={purchaseYuan} onChange={e => setPurchaseYuan(e.target.value)}
                    placeholder={`购买金额 (${fmt(myMinInvest)} - ${fmt(myMaxInvest)})`}
                    className="flex-1 px-4 py-2.5 bg-white/5 border border-white/10 rounded-lg text-white placeholder-gray-500 focus:outline-none focus:border-purple-500/50" />
                  <button onClick={handlePurchase} disabled={purchasing || !pool.payment_available}
                    className="px-6 py-2.5 bg-purple-600 hover:bg-purple-500 text-white rounded-lg text-sm font-medium transition disabled:opacity-50 shrink-0">
                    {purchasing ? '创建订单...' : !pool.payment_available ? '支付通道配置中' : payMethod === 'alipay' ? '支付宝购买' : '微信购买'}
                  </button>
                </div>

                {orderResult && (
                  <div className="rounded-lg border border-purple-500/10 bg-white/[0.03] p-3 mb-3">
                    <div className="flex items-center justify-between text-sm mb-2">
                      <span className="text-gray-400">订单 {orderResult.order_no}</span>
                      <span className="text-yellow-400 text-xs">待支付</span>
                    </div>
                    <div className="text-xs text-gray-500 mb-2">
                      {orderResult.shares} 份 × {fmt(orderResult.price_yuan)} = {fmt(orderResult.amount_yuan)}
                    </div>
                    {orderResult.code_url && (
                      <div className="flex flex-col items-center gap-2 py-3">
                        <img
                          src={`https://api.qrserver.com/v1/create-qr-code/?size=200x200&data=${encodeURIComponent(orderResult.code_url)}`}
                          alt="微信支付二维码"
                          className="w-48 h-48 rounded-lg bg-white p-2"
                        />
                        <span className="text-xs text-green-400">请用微信扫码支付</span>
                      </div>
                    )}
                    <div className="flex gap-2">
                      {orderResult.pay_url && (
                        <a href={orderResult.pay_url} target="_blank" rel="noopener noreferrer"
                          className="inline-flex items-center gap-1 text-xs text-blue-400 hover:text-blue-300">
                          <ExternalLink size={11} />打开支付页面
                        </a>
                      )}
                      <button onClick={() => pollOrder(orderResult.order_no)}
                        className="text-xs text-purple-400 hover:text-purple-300 transition">
                        刷新支付状态
                      </button>
                    </div>
                  </div>
                )}

                <div className="text-xs text-gray-500">
                  通过 StarAI 支付通道完成支付，支付成功后星钻自动到账
                </div>
              </div>
              <div className="lg:col-span-2 space-y-3 rounded-lg border border-purple-500/10 bg-white/[0.02] p-4">
                <div className="flex justify-between text-sm"><span className="text-gray-400">已投资</span><span className="text-white font-medium">{fmtFen(inv!.total_invested)}</span></div>
                <div className="flex justify-between text-sm"><span className="text-gray-400">持有星钻</span><span className="text-white font-medium">{inv!.shares.toLocaleString()} 份</span></div>
                <div className="flex justify-between text-sm"><span className="text-gray-400">协议期限</span><span className="text-white">{inv!.agreement_term} 年 (至 {inv!.agreement_expires_at?.slice(0, 10)})</span></div>

                {/* T+1 分润状态 */}
                <div className="pt-3 mt-1 border-t border-purple-500/10">
                  <div className="flex items-center justify-between mb-2">
                    <span className="text-sm text-gray-400">分润状态</span>
                    <span className={`text-xs px-2 py-0.5 rounded-full ${isActivated ? 'bg-green-500/10 border border-green-500/30 text-green-400' : 'bg-gray-500/10 border border-white/10 text-gray-500'}`}>
                      {isActivated ? 'T+1 每日结算' : '未激活'}
                    </span>
                  </div>

                  {!isActivated && profile?.settlement && (
                    <div className="space-y-2">
                      <div className="h-2 bg-white/5 rounded-full overflow-hidden">
                        <div className="h-full bg-gradient-to-r from-purple-500 to-fuchsia-400 rounded-full transition-all"
                          style={{ width: `${(profile.settlement.activation_progress * 100).toFixed(1)}%` }} />
                      </div>
                      <div className="flex justify-between text-xs">
                        <span className="text-gray-500">{fmtFen(inv!.total_invested)} / {fmt(pool.activation_yuan)}</span>
                        <span className="text-purple-400">{(profile.settlement.activation_progress * 100).toFixed(1)}%</span>
                      </div>
                      <div className="text-xs text-gray-600">虫后每日 T+1 结算利润池，累投 ≥ {fmt(pool.activation_yuan)} 激活分润</div>
                    </div>
                  )}

                  {isActivated && profile?.settlement && (
                    <div className="space-y-1.5">
                      <div className="flex justify-between text-sm"><span className="text-gray-500">昨日分润</span><span className="text-green-400 font-medium">{fmt(profile.settlement.yesterday_earning)}</span></div>
                      <div className="flex justify-between text-sm"><span className="text-gray-500">今日预估</span><span className="text-yellow-400/80">{fmt(profile.settlement.today_estimate)} <span className="text-xs text-gray-600">明日结算</span></span></div>
                      <div className="flex justify-between text-sm"><span className="text-gray-500">本月累计</span><span className="text-white">{fmt(profile.settlement.month_earning)}</span></div>
                      <div className="flex justify-between text-sm"><span className="text-gray-500">累计分润</span><span className="text-purple-400 font-medium">{fmt(profile.settlement.total_earning)}</span></div>
                      <div className="flex justify-between text-sm"><span className="text-gray-500">我的占比</span><span className="text-white">{profile.settlement.share_percent}</span></div>
                      <div className="pt-1.5 mt-1 border-t border-purple-500/5 text-xs text-gray-600">
                        {profile.settlement.next_settlement_label}
                      </div>
                    </div>
                  )}
                </div>
              </div>
            </div>
          </div>
        )}

        {/* ═══ Section 4: Portfolio ═══ */}
        {isInvestor && (
          <div className="space-y-6">
            <h2 className="text-lg font-bold flex items-center gap-2"><Wallet className="w-5 h-5 text-purple-400" />我的持仓</h2>
            <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
              <div className="rounded-xl border border-purple-500/10 bg-white/[0.02] p-5 text-center">
                <div className="text-sm text-gray-400 mb-1">持有星钻</div>
                <div className="text-2xl font-bold text-white">{inv!.shares.toLocaleString()}</div>
                <div className="text-xs text-gray-500">份</div>
              </div>
              <div className="rounded-xl border border-purple-500/10 bg-white/[0.02] p-5 text-center">
                <div className="text-sm text-gray-400 mb-1">持仓价值</div>
                <div className="text-2xl font-bold text-purple-400">{fmt(profile!.portfolio_yuan)}</div>
              </div>
              <div className="rounded-xl border border-purple-500/10 bg-white/[0.02] p-5 text-center">
                <div className="text-sm text-gray-400 mb-1">占比</div>
                <div className="text-2xl font-bold text-white">{profile!.share_percent}</div>
              </div>
              <div className="rounded-xl border border-purple-500/10 bg-white/[0.02] p-5 text-center">
                <div className="text-sm text-gray-400 mb-1">累计分红</div>
                <div className="text-2xl font-bold text-green-400">{fmtFen(inv!.total_dividends)}</div>
              </div>
            </div>

            <div className="rounded-xl border border-purple-500/10 bg-white/[0.02] p-6">
              <h3 className="text-sm font-semibold text-gray-300 mb-4">合伙人信息</h3>
              <div className="grid grid-cols-2 md:grid-cols-3 gap-4 text-sm">
                <div className="flex justify-between"><span className="text-gray-500">姓名</span><span className="text-white">{inv!.name || '-'}</span></div>
                <div className="flex justify-between"><span className="text-gray-500">来源</span><span className="text-white">{inv!.source}</span></div>
                <div className="flex justify-between"><span className="text-gray-500">总投资</span><span className="text-white font-medium">{fmtFen(inv!.total_invested)}</span></div>
                <div className="flex justify-between"><span className="text-gray-500">分润</span><span className={isActivated ? 'text-green-400' : 'text-gray-500'}>{isActivated ? '已激活' : '未激活'}</span></div>
                <div className="flex justify-between"><span className="text-gray-500">协议</span><span className="text-white">{inv!.agreement_term > 0 ? `${inv!.agreement_term}年期` : '未签署'}</span></div>
                <div className="flex justify-between"><span className="text-gray-500">加入时间</span><span className="text-white">{inv!.joined_at?.slice(0, 10)}</span></div>
              </div>
            </div>

            {profile!.transactions?.length > 0 && (
              <div className="rounded-xl border border-purple-500/10 bg-white/[0.02] p-6">
                <h3 className="text-sm font-semibold text-gray-300 mb-4">交易记录</h3>
                <div className="space-y-2">
                  {profile!.transactions.map((tx: any) => (
                    <div key={tx.id} className="flex items-center justify-between py-2 border-b border-purple-500/5 text-sm">
                      <div>
                        <span className={`inline-block px-2 py-0.5 rounded text-xs mr-2 ${tx.type === 'seed_grant' ? 'bg-purple-500/10 text-purple-400' : tx.type === 'recharge' ? 'bg-green-500/10 text-green-400' : 'bg-gray-500/10 text-gray-400'}`}>{tx.type}</span>
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

            {profile!.dividends?.length > 0 && (
              <div className="rounded-xl border border-purple-500/10 bg-white/[0.02] p-6">
                <h3 className="text-sm font-semibold text-gray-300 mb-4">分红记录</h3>
                <div className="space-y-2">
                  {profile!.dividends.map((d: any) => (
                    <div key={d.id} className="flex items-center justify-between py-2 border-b border-purple-500/5 text-sm">
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
          </div>
        )}

        {/* ═══ Section 5: Earnings ═══ */}
        {isInvestor && earnings && (
          <div className="space-y-6">
            <h2 className="text-lg font-bold flex items-center gap-2"><BarChart3 className="w-5 h-5 text-purple-400" />收益明细</h2>
            <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
              <div className="rounded-xl border border-purple-500/20 bg-purple-500/[0.04] p-6 text-center">
                <div className="text-sm text-gray-400 mb-1">今日预估收益</div>
                <div className="text-3xl font-bold text-purple-400">{fmt(earnings.today)}</div>
              </div>
              <div className="rounded-xl border border-purple-500/10 bg-white/[0.02] p-6 text-center">
                <div className="text-sm text-gray-400 mb-1">近 30 天累计</div>
                <div className="text-3xl font-bold text-white">{fmt(earnings.total30d)}</div>
              </div>
              <div className="rounded-xl border border-purple-500/10 bg-white/[0.02] p-6 text-center">
                <div className="text-sm text-gray-400 mb-1">份额占比</div>
                <div className="text-3xl font-bold text-white">{earnings.percent}</div>
              </div>
            </div>
            {earnings.daily.length > 0 && (
              <div className="rounded-xl border border-purple-500/10 bg-white/[0.02] p-6">
                <h3 className="text-sm font-semibold text-gray-300 mb-4">每日收益 (近30天)</h3>
                <div className="space-y-1.5">
                  {earnings.daily.slice(0, 14).map(d => {
                    const maxVal = Math.max(...earnings!.daily.map(e => e.my_yuan), 1);
                    const w = Math.max(2, (d.my_yuan / maxVal) * 100);
                    return (
                      <div key={d.date} className="flex items-center gap-3 text-sm">
                        <span className="text-gray-500 w-20 shrink-0 text-xs">{d.date.slice(5)}</span>
                        <div className="flex-1 h-5 bg-white/5 rounded-full overflow-hidden">
                          <div className="h-full bg-purple-500/30 rounded-full" style={{ width: `${w}%` }} />
                        </div>
                        <span className="text-white font-medium w-24 text-right text-xs">{fmt(d.my_yuan)}</span>
                      </div>
                    );
                  })}
                </div>
              </div>
            )}
          </div>
        )}

        {/* ═══ Section 6: Equity ═══ */}
        {isInvestor && (
          <div className="space-y-6">
            <h2 className="text-lg font-bold flex items-center gap-2"><Gem className="w-5 h-5 text-purple-400" />合伙人期权</h2>
            {equity.length === 0 ? (
              <div className="rounded-xl border border-purple-500/10 bg-white/[0.02] p-10 text-center">
                <Lock className="w-8 h-8 text-gray-600 mx-auto mb-3" />
                <div className="text-sm text-gray-500">暂无期权授予</div>
                <div className="text-xs text-gray-600 mt-1">成为团队合伙人后由管理员授予</div>
              </div>
            ) : (
              <div className="space-y-4">
                {equity.map(g => {
                  const vestPct = g.total_shares > 0 ? (g.vested_shares / g.total_shares * 100) : 0;
                  const isCliff = new Date() < new Date(g.cliff_date);
                  return (
                    <div key={g.id} className="rounded-xl border border-purple-500/10 bg-white/[0.03] p-6">
                      <div className="flex items-start justify-between mb-4">
                        <div>
                          <div className="flex items-center gap-2 mb-1">
                            <Gem className="w-4 h-4 text-purple-400" />
                            <span className="text-white font-semibold">{g.total_shares.toLocaleString()} 份期权</span>
                            <span className={`px-2 py-0.5 rounded text-xs ${g.status === 'active' ? 'bg-green-500/10 text-green-400' : g.status === 'exercised' ? 'bg-blue-500/10 text-blue-400' : 'bg-gray-500/10 text-gray-400'}`}>
                              {g.status === 'active' ? '生效中' : g.status === 'exercised' ? '已行权' : '已取消'}
                            </span>
                          </div>
                          <div className="text-xs text-gray-500">授予于 {g.grant_date?.slice(0, 10)} · Cliff {g.cliff_months}个月 · 归属 {g.vesting_months}个月</div>
                        </div>
                        {g.strike_price > 0 && (
                          <div className="text-right">
                            <div className="text-xs text-gray-400">行权价</div>
                            <div className="text-white font-medium">{fmt(g.strike_price)}</div>
                          </div>
                        )}
                      </div>
                      <div className="mb-3">
                        <div className="flex justify-between text-xs mb-1">
                          <span className="text-gray-400">已归属 {g.vested_shares.toLocaleString()} / {g.total_shares.toLocaleString()}</span>
                          <span className="text-purple-400 font-medium">{vestPct.toFixed(1)}%</span>
                        </div>
                        <div className="h-2 bg-white/5 rounded-full overflow-hidden">
                          <div className="h-full bg-gradient-to-r from-purple-500 to-fuchsia-400 rounded-full transition-all" style={{ width: `${vestPct}%` }} />
                        </div>
                      </div>
                      <div className="flex items-center gap-4 text-xs">
                        <div className="flex items-center gap-1 text-gray-500"><Calendar size={12} /><span>Cliff: {g.cliff_date?.slice(0, 10)}</span>{isCliff ? <Lock size={10} className="text-red-400" /> : <Unlock size={10} className="text-green-400" />}</div>
                        <div className="flex items-center gap-1 text-gray-500"><CheckCircle size={12} /><span>全归属: {g.full_vest_date?.slice(0, 10)}</span></div>
                      </div>
                    </div>
                  );
                })}
              </div>
            )}
          </div>
        )}

        </>)}
      </div>

      {/* Footer */}
      <div className="border-t border-purple-500/10 mt-16 py-6 text-center text-xs text-gray-600">
        <span className="text-purple-500/40">◆</span> StarClaw · Star Diamond <span className="text-purple-500/40">◆</span>
      </div>
      {/* WeChat QR Code Modal */}
      {wxQr && (
        <div className="fixed inset-0 bg-black/70 flex items-center justify-center z-50" onClick={() => setWxQr(null)}>
          <div className="bg-gray-900 border border-gray-700 rounded-2xl p-6 max-w-sm w-full mx-4 space-y-4" onClick={e => e.stopPropagation()}>
            <div className="flex items-center justify-between">
              <h3 className="text-lg font-semibold text-white">微信扫码支付</h3>
              <button onClick={() => setWxQr(null)} className="text-gray-500 hover:text-white cursor-pointer text-xl">&times;</button>
            </div>
            <div className="text-center space-y-3">
              <div className="text-2xl font-bold text-green-400">¥{wxQr.amount}</div>
              <div className="bg-white rounded-xl p-4 inline-block">
                <img
                  src={`https://api.qrserver.com/v1/create-qr-code/?size=200x200&data=${encodeURIComponent(wxQr.codeUrl)}`}
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
