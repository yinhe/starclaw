import { useState } from 'react';
import { useSearchParams, useNavigate, Link } from 'react-router-dom';
import { LogoMark } from '../components/Logo';
import { authAPI, clawAuthAPI, clawNodeRequest } from '../lib/api';
import { isLoggedIn, setAuth, clearAuth, getUserDisplayName } from '../lib/auth';
import { Fingerprint, CheckCircle2, AlertCircle, Loader2 as Spinner, Shield, Mail, Lock, UserPlus } from 'lucide-react';

const isInvestDomain = window.location.hostname === 'invest.starclaw.net';
const isQueenDomain = !isInvestDomain;

export function AuthPage() {
  const [searchParams] = useSearchParams();
  const navigate = useNavigate();
  const logged = isLoggedIn();
  const initTab = searchParams.get('tab') || (isInvestDomain ? 'login' : 'claw');
  const [mode, setMode] = useState<'login' | 'register' | 'claw'>(isQueenDomain ? 'claw' : initTab as any);
  const defaultRedirect = isInvestDomain ? '/' : '/dashboard';

  // Email/password login state
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [nickname, setNickname] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [authMsg, setAuthMsg] = useState<{ text: string; error: boolean } | null>(null);

  async function handleEmailLogin() {
    setAuthMsg(null); setSubmitting(true);
    try {
      const body: any = { password };
      if (email.includes('@')) body.email = email; else body.phone = email;
      const data = await authAPI.login(body);
      setAuth(data.token, data.user);
      setAuthMsg({ text: '登录成功', error: false });
      setTimeout(() => navigate(defaultRedirect), 500);
    } catch (e: any) { setAuthMsg({ text: e.message || '登录失败', error: true }); }
    setSubmitting(false);
  }

  async function handleRegister() {
    setAuthMsg(null); setSubmitting(true);
    try {
      const body: any = { nickname, password };
      if (email.includes('@')) body.email = email; else body.phone = email;
      await authAPI.register(body);
      setAuthMsg({ text: '注册成功，请登录', error: false });
      setMode('login');
    } catch (e: any) { setAuthMsg({ text: e.message || '注册失败', error: true }); }
    setSubmitting(false);
  }

  // Claw node login state
  const [clawUrl, setClawUrl] = useState('');
  const [clawStep, setClawStep] = useState<'input' | 'connecting' | 'waiting' | 'verifying' | 'done' | 'error'>('input');
  const [clawNodeInfo, setClawNodeInfo] = useState<{ node_id: string; public_key: string } | null>(null);
  const [msg, setMsg] = useState<{ text: string; error: boolean } | null>(null);
  const pollRef = { current: null as ReturnType<typeof setInterval> | null };

  async function handleClawLogin() {
    setClawStep('connecting');
    setMsg(null);
    const baseUrl = clawUrl.replace(/\/$/, '');
    try {
      // Step 1: Get node identity (public)
      const info = await clawNodeRequest<{ node_id: string; public_key: string }>(
        baseUrl, '/v1/identity/info'
      );
      setClawNodeInfo(info);

      // Step 2: Get challenge from Queen
      const { challenge } = await clawAuthAPI.challenge();

      // Step 3: Send auth-request to Claw (public endpoint — creates pending request)
      const reqRes = await clawNodeRequest<{ id: string; status: string }>(
        baseUrl, '/v1/identity/auth-request',
        { method: 'POST', body: JSON.stringify({ challenge, origin: window.location.hostname }) }
      );
      const requestId = reqRes.id;

      // Step 4: Poll for approval — user must approve on their Claw UI
      setClawStep('waiting');
      setMsg({ text: '请在你的 Claw 界面确认授权登录', error: false });

      await new Promise<void>((resolve, reject) => {
        let attempts = 0;
        pollRef.current = setInterval(async () => {
          attempts++;
          if (attempts > 100) { // 5min timeout (100 * 3s)
            if (pollRef.current) clearInterval(pollRef.current);
            reject(new Error('授权超时，请重试'));
            return;
          }
          try {
            const status = await clawNodeRequest<{
              id: string; status: string;
              node_id?: string; public_key?: string; signature?: string; challenge?: string;
            }>(baseUrl, `/v1/identity/auth-request/${requestId}`);

            if (status.status === 'approved') {
              if (pollRef.current) clearInterval(pollRef.current);

              // Step 5: Submit signature to Queen for verification
              setClawStep('verifying');
              const data = await clawAuthAPI.verify({
                challenge: status.challenge!,
                node_id: status.node_id!,
                public_key: status.public_key!,
                signature: status.signature!,
              });

              setClawStep('done');
              setAuth(data.token, data.user);
              setMsg({ text: `${info.node_id.slice(0, 18)}... 身份验证成功`, error: false });
              setTimeout(() => navigate(defaultRedirect), 1200);
              resolve();
            } else if (status.status === 'rejected') {
              if (pollRef.current) clearInterval(pollRef.current);
              reject(new Error('授权被拒绝'));
            }
          } catch (e: any) {
            if (e.message?.includes('expired') || e.message?.includes('not found')) {
              if (pollRef.current) clearInterval(pollRef.current);
              reject(new Error('请求已过期，请重试'));
            }
          }
        }, 3000);
      });
    } catch (e: any) {
      if (pollRef.current) clearInterval(pollRef.current);
      setClawStep('error');
      setMsg({ text: e.message || '连接 Claw 节点失败', error: true });
    }
  }

  const INPUT = 'w-full px-4 py-3 rounded-xl border border-gray-200 text-sm focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:border-transparent transition';

  return (
    <div className="min-h-screen bg-gray-50 antialiased">
      <div className="fixed inset-0 bg-gradient-to-br from-indigo-500/5 via-purple-500/5 to-pink-500/5" />
      <div className="relative min-h-screen flex flex-col items-center justify-center px-4">

        {/* Logo */}
        <Link to="/" className="flex items-center gap-2.5 mb-8">
          <LogoMark className="w-10 h-10 shadow-lg shadow-indigo-500/25" />
          <span className="text-2xl font-bold text-gray-900">StarClaw</span>
        </Link>

        <div className="w-full max-w-md">
          {logged ? (
            <div className="bg-white rounded-2xl shadow-xl shadow-gray-200/50 border border-gray-100 p-8 text-center">
              <div className="w-16 h-16 rounded-full bg-green-50 flex items-center justify-center mx-auto mb-4">
                <CheckCircle2 className="w-8 h-8 text-green-500" />
              </div>
              <h2 className="text-lg font-bold mb-1">已登录</h2>
              <p className="text-sm text-gray-500 mb-6">欢迎回来，{getUserDisplayName()}</p>
              <div className="space-y-3">
                <Link to="/dashboard" className="block w-full py-3 rounded-xl bg-indigo-600 text-white font-semibold text-sm hover:bg-indigo-700 transition text-center">进入虫群门户</Link>
                <button onClick={() => { clearAuth(); window.location.reload(); }} className="w-full py-2.5 rounded-xl text-sm text-gray-400 hover:text-red-500 transition">退出登录</button>
              </div>
            </div>
          ) : (
            <div className="bg-white rounded-2xl shadow-xl shadow-gray-200/50 border border-gray-100 overflow-hidden">
              {/* Mode Tabs — only show on invest domain (queen domain = Claw only) */}
              {isInvestDomain && (
                <div className="flex border-b border-gray-100">
                  <button onClick={() => { setMode('login'); setAuthMsg(null); }}
                    className={`flex-1 py-3 text-sm font-medium transition ${mode === 'login' ? 'text-indigo-600 border-b-2 border-indigo-600' : 'text-gray-400 hover:text-gray-600'}`}>
                    <Mail className="w-4 h-4 inline mr-1.5 -mt-0.5" />登录
                  </button>
                  <button onClick={() => { setMode('register'); setAuthMsg(null); }}
                    className={`flex-1 py-3 text-sm font-medium transition ${mode === 'register' ? 'text-indigo-600 border-b-2 border-indigo-600' : 'text-gray-400 hover:text-gray-600'}`}>
                    <UserPlus className="w-4 h-4 inline mr-1.5 -mt-0.5" />注册
                  </button>
                  <button onClick={() => { setMode('claw'); setAuthMsg(null); }}
                    className={`flex-1 py-3 text-sm font-medium transition ${mode === 'claw' ? 'text-indigo-600 border-b-2 border-indigo-600' : 'text-gray-400 hover:text-gray-600'}`}>
                    <Fingerprint className="w-4 h-4 inline mr-1.5 -mt-0.5" />Claw
                  </button>
                </div>
              )}

              <div className="p-8 space-y-5">

                {/* ── Email Login ── */}
                {mode === 'login' && (
                  <>
                    <div className="text-center mb-2">
                      <h2 className="text-lg font-bold text-gray-900">账号登录</h2>
                      <p className="text-sm text-gray-400 mt-1">使用邮箱或手机号登录</p>
                    </div>
                    <div>
                      <label className="block text-sm font-medium text-gray-700 mb-1.5">邮箱 / 手机号</label>
                      <input type="text" value={email} onChange={e => setEmail(e.target.value)}
                        className={INPUT} placeholder="请输入邮箱或手机号"
                        onKeyDown={e => e.key === 'Enter' && handleEmailLogin()} />
                    </div>
                    <div>
                      <label className="block text-sm font-medium text-gray-700 mb-1.5">密码</label>
                      <input type="password" value={password} onChange={e => setPassword(e.target.value)}
                        className={INPUT} placeholder="请输入密码"
                        onKeyDown={e => e.key === 'Enter' && handleEmailLogin()} />
                    </div>
                    {authMsg && <p className={`text-sm ${authMsg.error ? 'text-red-500' : 'text-green-600'}`}>{authMsg.text}</p>}
                    <button type="button" onClick={handleEmailLogin} disabled={submitting}
                      className="w-full flex items-center justify-center gap-2 py-3 rounded-xl bg-indigo-600 text-white font-semibold text-sm hover:bg-indigo-700 transition shadow-lg shadow-indigo-500/25 disabled:opacity-50">
                      <Lock className="w-4 h-4" />
                      {submitting ? '登录中...' : '登录'}
                    </button>
                  </>
                )}

                {/* ── Register ── */}
                {mode === 'register' && (
                  <>
                    <div className="text-center mb-2">
                      <h2 className="text-lg font-bold text-gray-900">注册账号</h2>
                      <p className="text-sm text-gray-400 mt-1">创建 StarClaw 账号</p>
                    </div>
                    <div>
                      <label className="block text-sm font-medium text-gray-700 mb-1.5">昵称</label>
                      <input type="text" value={nickname} onChange={e => setNickname(e.target.value)}
                        className={INPUT} placeholder="请输入昵称" />
                    </div>
                    <div>
                      <label className="block text-sm font-medium text-gray-700 mb-1.5">邮箱 / 手机号</label>
                      <input type="text" value={email} onChange={e => setEmail(e.target.value)}
                        className={INPUT} placeholder="请输入邮箱或手机号" />
                    </div>
                    <div>
                      <label className="block text-sm font-medium text-gray-700 mb-1.5">密码</label>
                      <input type="password" value={password} onChange={e => setPassword(e.target.value)}
                        className={INPUT} placeholder="请设置密码（至少6位）"
                        onKeyDown={e => e.key === 'Enter' && handleRegister()} />
                    </div>
                    {authMsg && <p className={`text-sm ${authMsg.error ? 'text-red-500' : 'text-green-600'}`}>{authMsg.text}</p>}
                    <button type="button" onClick={handleRegister} disabled={submitting}
                      className="w-full flex items-center justify-center gap-2 py-3 rounded-xl bg-indigo-600 text-white font-semibold text-sm hover:bg-indigo-700 transition shadow-lg shadow-indigo-500/25 disabled:opacity-50">
                      <UserPlus className="w-4 h-4" />
                      {submitting ? '注册中...' : '注册'}
                    </button>
                  </>
                )}

                {/* ── Claw Node Login ── */}
                {mode === 'claw' && (
                  <>
                    <div className="text-center mb-2">
                      <div className="w-14 h-14 rounded-full bg-indigo-50 flex items-center justify-center mx-auto mb-4">
                        <Fingerprint className="w-7 h-7 text-indigo-500" />
                      </div>
                      <h2 className="text-lg font-bold text-gray-900">Claw 节点认证</h2>
                      <p className="text-sm text-gray-400 mt-1">输入节点地址，在你的 Claw 上确认授权</p>
                    </div>

                    {clawStep === 'input' && (
                      <>
                        <div>
                          <label className="block text-sm font-medium text-gray-700 mb-1.5">Claw 节点地址</label>
                          <input type="url" value={clawUrl} onChange={e => setClawUrl(e.target.value)}
                            className={INPUT} placeholder="请输入你的 Claw 节点地址"
                            onKeyDown={e => e.key === 'Enter' && handleClawLogin()} />
                        </div>

                        <div className="flex items-start gap-2 p-3 bg-indigo-50/70 rounded-xl">
                          <Shield className="w-4 h-4 text-indigo-400 mt-0.5 flex-none" />
                          <p className="text-xs text-indigo-600/70 leading-relaxed">
                            安全回签：请求会发送到你的 Claw 节点，你需要在 Claw 界面确认授权后才能登录，无人可冒充。
                          </p>
                        </div>

                        {msg && <p className={`text-sm ${msg.error ? 'text-red-500' : 'text-green-600'}`}>{msg.text}</p>}

                        <button type="button" onClick={handleClawLogin}
                          className="w-full flex items-center justify-center gap-2.5 py-3 rounded-xl bg-indigo-600 text-white font-semibold text-sm hover:bg-indigo-700 transition shadow-lg shadow-indigo-500/25">
                          <Fingerprint className="w-5 h-5" />
                          发送认证请求
                        </button>
                      </>
                    )}

                    {clawStep !== 'input' && clawStep !== 'error' && (
                      <div className="space-y-3 py-2">
                        <StepIndicator done={clawStep !== 'connecting'} active={clawStep === 'connecting'}
                          label="连接 Claw 节点" sub={clawNodeInfo ? clawNodeInfo.node_id.slice(0, 20) + '...' : ''} />
                        <StepIndicator done={clawStep === 'verifying' || clawStep === 'done'} active={clawStep === 'waiting'}
                          label="等待 Claw 授权确认" sub={clawStep === 'waiting' ? '请在 Claw 界面点击「授权登录」' : ''} />
                        <StepIndicator done={clawStep === 'done'} active={clawStep === 'verifying'}
                          label="Queen 验证身份" />
                        {clawStep === 'done' && msg && (
                          <div className="flex items-center gap-2 text-green-600 text-sm font-medium pt-2">
                            <CheckCircle2 className="w-4 h-4" /> {msg.text}
                          </div>
                        )}
                      </div>
                    )}

                    {clawStep === 'error' && (
                      <div className="space-y-3">
                        <div className="flex items-center gap-2 text-red-500 text-sm">
                          <AlertCircle className="w-4 h-4 flex-none" />
                          <span>{msg?.text || '连接失败'}</span>
                        </div>
                        <button type="button" onClick={() => { setClawStep('input'); setMsg(null); }}
                          className="w-full py-2.5 rounded-xl border border-gray-200 text-gray-600 text-sm font-medium hover:bg-gray-50 transition">
                          重试
                        </button>
                      </div>
                    )}

                    <p className="text-center text-xs text-gray-400 pt-2">
                      没有 Claw 节点？<a href="https://github.com/yinhe/starclaw" target="_blank" rel="noopener noreferrer" className="text-indigo-500 hover:underline">免费部署一个</a>
                    </p>
                  </>
                )}

              </div>
            </div>
          )}
        </div>

        <p className="mt-8 text-xs text-gray-400">
          <Link to="/" className="hover:text-gray-600">官网</Link>
          <span className="mx-2">&middot;</span>
          <Link to="/docs" className="hover:text-gray-600">文档</Link>
          <span className="mx-2">&middot;</span>
          <a href="https://github.com/yinhe/starclaw" className="hover:text-gray-600">GitHub</a>
        </p>
      </div>
    </div>
  );
}

function StepIndicator({ done, active, label, sub }: { done: boolean; active: boolean; label: string; sub?: string }) {
  return (
    <div className="flex items-center gap-2.5 text-sm">
      {done ? (
        <CheckCircle2 className="w-4 h-4 text-green-500 flex-none" />
      ) : active ? (
        <Spinner className="w-4 h-4 text-indigo-500 animate-spin flex-none" />
      ) : (
        <div className="w-4 h-4 rounded-full border-2 border-gray-200 flex-none" />
      )}
      <span className={done ? 'text-green-700' : active ? 'text-indigo-700 font-medium' : 'text-gray-400'}>{label}</span>
      {sub && <span className="text-[10px] text-gray-400 font-mono">{sub}</span>}
    </div>
  );
}
