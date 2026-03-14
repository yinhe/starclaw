import { useState, useEffect, useRef } from 'react';
import { useSearchParams, useNavigate, Link } from 'react-router-dom';
import { LogoMark } from '../components/Logo';
import { authAPI, clawAuthAPI, clawNodeRequest } from '../lib/api';
import { isLoggedIn, setAuth, clearAuth, getUserDisplayName } from '../lib/auth';
import { Github, Fingerprint, CheckCircle2, AlertCircle, Loader2 as Spinner } from 'lucide-react';

type Tab = 'login' | 'register';
type Method = 'email' | 'phone';

const GOOGLE_CLIENT_ID = import.meta.env.VITE_GOOGLE_CLIENT_ID || '';
const GITHUB_CLIENT_ID = import.meta.env.VITE_GITHUB_CLIENT_ID || '';

export function AuthPage() {
  const [searchParams] = useSearchParams();
  const navigate = useNavigate();
  const [tab, setTab] = useState<Tab>(searchParams.get('tab') === 'register' ? 'register' : 'login');
  const [msg, setMsg] = useState<{ text: string; error: boolean } | null>(null);
  const [loading, setLoading] = useState(false);
  const [loginMethod, setLoginMethod] = useState<Method>('email');
  const [regMethod, setRegMethod] = useState<Method>('email');
  const logged = isLoggedIn();

  // Login form
  const [loginEmail, setLoginEmail] = useState('');
  const [loginPhone, setLoginPhone] = useState('');
  const [loginPassword, setLoginPassword] = useState('');

  // Register form
  const [regEmail, setRegEmail] = useState('');
  const [regPhone, setRegPhone] = useState('');
  const [regNickname, setRegNickname] = useState('');
  const [regPassword, setRegPassword] = useState('');

  const handleLogin = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);
    setMsg(null);
    try {
      const body: Record<string, string> = { password: loginPassword };
      if (loginMethod === 'email') {
        if (!loginEmail) { setMsg({ text: '请输入邮箱', error: true }); return; }
        body.email = loginEmail;
      } else {
        if (!loginPhone) { setMsg({ text: '请输入手机号', error: true }); return; }
        body.phone = loginPhone;
      }
      if (!loginPassword) { setMsg({ text: '请输入密码', error: true }); return; }
      const data = await authAPI.login(body);
      setAuth(data.token, data.user);
      setMsg({ text: '登录成功，正在跳转...', error: false });
      const redirect = searchParams.get('redirect');
      setTimeout(() => navigate(redirect || '/dashboard'), 800);
    } catch (e: any) {
      setMsg({ text: e.message, error: true });
    } finally {
      setLoading(false);
    }
  };

  const handleRegister = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);
    setMsg(null);
    try {
      const body: Record<string, string> = { nickname: regNickname, password: regPassword };
      if (regMethod === 'email') {
        if (!regEmail) { setMsg({ text: '请输入邮箱', error: true }); return; }
        body.email = regEmail;
      } else {
        if (!regPhone) { setMsg({ text: '请输入手机号', error: true }); return; }
        body.phone = regPhone;
      }
      if (!regNickname) { setMsg({ text: '请输入昵称', error: true }); return; }
      if (!regPassword || regPassword.length < 6) { setMsg({ text: '密码至少 6 位', error: true }); return; }
      await authAPI.register(body as any);
      setMsg({ text: '注册成功！正在自动登录...', error: false });

      const loginBody: Record<string, string> = { password: regPassword };
      if (body.email) loginBody.email = body.email; else loginBody.phone = body.phone;
      try {
        const ld = await authAPI.login(loginBody);
        setAuth(ld.token, ld.user);
        setTimeout(() => navigate('/dashboard'), 1000);
      } catch {
        setTimeout(() => setTab('login'), 1500);
      }
    } catch (e: any) {
      setMsg({ text: e.message, error: true });
    } finally {
      setLoading(false);
    }
  };

  // ---- Claw Login ----
  const [clawMode, setClawMode] = useState(false);
  const [clawUrl, setClawUrl] = useState('http://localhost:8080');
  const [clawStep, setClawStep] = useState<'input' | 'connecting' | 'signing' | 'verifying' | 'done' | 'error'>('input');
  const [clawNodeInfo, setClawNodeInfo] = useState<{ node_id: string; public_key: string } | null>(null);

  async function handleClawLogin() {
    setClawStep('connecting');
    setMsg(null);
    try {
      // Step 1: Get node identity
      const info = await clawNodeRequest<{ node_id: string; public_key: string }>(
        clawUrl.replace(/\/$/, ''), '/v1/identity/info'
      );
      setClawNodeInfo(info);

      // Step 2: Get challenge from Queen
      setClawStep('signing');
      const { challenge } = await clawAuthAPI.challenge();

      // Step 3: Ask Claw to sign the challenge
      const signed = await clawNodeRequest<{ node_id: string; public_key: string; signature: string; challenge: string }>(
        clawUrl.replace(/\/$/, ''), '/v1/identity/sign-challenge',
        { method: 'POST', body: JSON.stringify({ challenge }) }
      );

      // Step 4: Verify with Queen
      setClawStep('verifying');
      const data = await clawAuthAPI.verify({
        challenge: signed.challenge,
        node_id: signed.node_id,
        public_key: signed.public_key,
        signature: signed.signature,
      });

      // Success!
      setClawStep('done');
      setAuth(data.token, data.user);
      setMsg({ text: `Claw ${info.node_id.slice(0, 14)}... 登录成功！`, error: false });
      setTimeout(() => navigate(searchParams.get('redirect') || '/dashboard'), 1200);
    } catch (e: any) {
      setClawStep('error');
      setMsg({ text: e.message || '连接 Claw 节点失败', error: true });
    }
  }

  // ---- OAuth ----
  function loginWithGoogle() {
    if (!GOOGLE_CLIENT_ID) { setMsg({ text: 'Google 登录尚未配置', error: true }); return; }
    const redirect = `${window.location.origin}/auth`;
    const url = `https://accounts.google.com/o/oauth2/v2/auth?client_id=${GOOGLE_CLIENT_ID}&redirect_uri=${encodeURIComponent(redirect)}&response_type=code&scope=openid%20email%20profile&state=google`;
    window.location.href = url;
  }

  function loginWithGithub() {
    if (!GITHUB_CLIENT_ID) { setMsg({ text: 'GitHub 登录尚未配置', error: true }); return; }
    const redirect = `${window.location.origin}/auth`;
    const url = `https://github.com/login/oauth/authorize?client_id=${GITHUB_CLIENT_ID}&redirect_uri=${encodeURIComponent(redirect)}&scope=user:email&state=github`;
    window.location.href = url;
  }

  // Handle OAuth callback
  const oauthHandled = useRef(false);
  useEffect(() => {
    const code = searchParams.get('code');
    const state = searchParams.get('state');
    if (!code || !state || oauthHandled.current || logged) return;
    oauthHandled.current = true;
    const provider = state as 'google' | 'github';
    setLoading(true);
    setMsg({ text: `正在通过 ${provider === 'google' ? 'Google' : 'GitHub'} 登录...`, error: false });
    authAPI.oauthLogin(provider, code)
      .then(data => {
        setAuth(data.token, data.user);
        setMsg({ text: '登录成功，正在跳转...', error: false });
        setTimeout(() => navigate('/dashboard'), 800);
      })
      .catch((e: any) => {
        setMsg({ text: e.message, error: true });
        setLoading(false);
      });
  }, [searchParams, logged, navigate]);

  const TAB_ON = 'flex-1 py-4 text-sm font-semibold text-indigo-600 border-b-2 border-indigo-600 transition';
  const TAB_OFF = 'flex-1 py-4 text-sm font-medium text-gray-400 border-b-2 border-transparent hover:text-gray-600 transition';
  const BTN_ON = 'flex-1 py-2 text-xs font-medium rounded-md bg-white text-gray-900 shadow-sm transition';
  const BTN_OFF = 'flex-1 py-2 text-xs font-medium rounded-md text-gray-500 transition';
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
                <svg className="w-8 h-8 text-green-500" fill="none" stroke="currentColor" strokeWidth="2" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" d="M5 13l4 4L19 7" /></svg>
              </div>
              <h2 className="text-lg font-bold mb-1">已登录</h2>
              <p className="text-sm text-gray-500 mb-6">欢迎回来，{getUserDisplayName()}</p>
              <div className="space-y-3">
                <Link to="/dashboard" className="block w-full py-3 rounded-xl bg-indigo-600 text-white font-semibold text-sm hover:bg-indigo-700 transition text-center">进入用户后台</Link>
                <button onClick={() => { clearAuth(); window.location.reload(); }} className="w-full py-2.5 rounded-xl text-sm text-gray-400 hover:text-red-500 transition">退出登录</button>
              </div>
            </div>
          ) : (
            <div className="bg-white rounded-2xl shadow-xl shadow-gray-200/50 border border-gray-100 overflow-hidden">
              {/* Tabs */}
              <div className="flex border-b border-gray-100">
                <button onClick={() => { setTab('login'); setMsg(null); }} className={tab === 'login' ? TAB_ON : TAB_OFF}>登录</button>
                <button onClick={() => { setTab('register'); setMsg(null); }} className={tab === 'register' ? TAB_ON : TAB_OFF}>注册</button>
              </div>

              {/* Login Form */}
              {tab === 'login' && (
                <form className="p-8 space-y-5" onSubmit={handleLogin}>
                  <div className="flex bg-gray-100 rounded-lg p-0.5 mb-1">
                    <button type="button" onClick={() => setLoginMethod('email')} className={loginMethod === 'email' ? BTN_ON : BTN_OFF}>邮箱登录</button>
                    <button type="button" onClick={() => setLoginMethod('phone')} className={loginMethod === 'phone' ? BTN_ON : BTN_OFF}>手机登录</button>
                  </div>
                  {loginMethod === 'email' ? (
                    <div>
                      <label className="block text-sm font-medium text-gray-700 mb-1.5">邮箱</label>
                      <input type="email" value={loginEmail} onChange={e => setLoginEmail(e.target.value)} className={INPUT} placeholder="you@example.com" />
                    </div>
                  ) : (
                    <div>
                      <label className="block text-sm font-medium text-gray-700 mb-1.5">手机号</label>
                      <input type="tel" value={loginPhone} onChange={e => setLoginPhone(e.target.value)} className={INPUT} placeholder="13800138000" />
                    </div>
                  )}
                  <div>
                    <label className="block text-sm font-medium text-gray-700 mb-1.5">密码</label>
                    <input type="password" value={loginPassword} onChange={e => setLoginPassword(e.target.value)} className={INPUT} placeholder="••••••••" />
                  </div>
                  {msg && <p className={`text-sm ${msg.error ? 'text-red-500' : 'text-green-600'}`}>{msg.text}</p>}
                  <button type="submit" disabled={loading}
                    className="w-full py-3 rounded-xl bg-indigo-600 text-white font-semibold text-sm hover:bg-indigo-700 transition shadow-lg shadow-indigo-500/25 disabled:opacity-50">
                    {loading ? '登录中...' : '登录'}
                  </button>

                  {/* Divider */}
                  <div className="relative my-2">
                    <div className="absolute inset-0 flex items-center"><div className="w-full border-t border-gray-200" /></div>
                    <div className="relative flex justify-center text-xs"><span className="bg-white px-3 text-gray-400">或</span></div>
                  </div>

                  {/* Sign-In with Claw */}
                  {!clawMode ? (
                    <>
                      <button type="button" onClick={() => setClawMode(true)}
                        className="w-full flex items-center justify-center gap-2.5 py-3 rounded-xl bg-gradient-to-r from-orange-500 to-red-500 text-white font-semibold text-sm hover:from-orange-600 hover:to-red-600 transition shadow-lg shadow-orange-500/25">
                        <Fingerprint className="w-5 h-5" />
                        使用 Claw 节点登录
                      </button>

                      {/* OAuth buttons */}
                      <div className="grid grid-cols-2 gap-3">
                        <button type="button" onClick={loginWithGoogle}
                          className="flex items-center justify-center gap-2 py-2.5 rounded-xl border border-gray-200 text-sm font-medium text-gray-700 hover:bg-gray-50 hover:border-gray-300 transition">
                          <GoogleIcon />
                          Google
                        </button>
                        <button type="button" onClick={loginWithGithub}
                          className="flex items-center justify-center gap-2 py-2.5 rounded-xl border border-gray-200 text-sm font-medium text-gray-700 hover:bg-gray-50 hover:border-gray-300 transition">
                          <Github className="w-4 h-4" />
                          GitHub
                        </button>
                      </div>
                    </>
                  ) : (
                    <div className="space-y-4 rounded-xl border-2 border-orange-200 bg-orange-50/50 p-4">
                      <div className="flex items-center justify-between">
                        <div className="flex items-center gap-2 text-sm font-semibold text-orange-700">
                          <Fingerprint className="w-4 h-4" />
                          使用 Claw 节点登录
                        </div>
                        <button type="button" onClick={() => { setClawMode(false); setClawStep('input'); setMsg(null); }}
                          className="text-xs text-gray-400 hover:text-gray-600">返回</button>
                      </div>

                      {clawStep === 'input' && (
                        <>
                          <div>
                            <label className="block text-xs font-medium text-gray-600 mb-1">Claw 节点地址</label>
                            <input type="url" value={clawUrl} onChange={e => setClawUrl(e.target.value)}
                              className="w-full px-3 py-2.5 rounded-lg border border-orange-200 text-sm focus:outline-none focus:ring-2 focus:ring-orange-400 bg-white"
                              placeholder="http://localhost:8080" />
                          </div>
                          <p className="text-[11px] text-gray-400">输入你的 Claw 节点地址，我们会通过 Ed25519 签名验证你的身份，无需密码。</p>
                          <button type="button" onClick={handleClawLogin}
                            className="w-full py-2.5 rounded-lg bg-orange-500 text-white text-sm font-semibold hover:bg-orange-600 transition">
                            连接并登录
                          </button>
                        </>
                      )}

                      {clawStep !== 'input' && clawStep !== 'error' && (
                        <div className="space-y-2">
                          <StepIndicator done={clawStep !== 'connecting'} active={clawStep === 'connecting'} label="连接 Claw 节点" sub={clawNodeInfo ? clawNodeInfo.node_id.slice(0, 20) + '...' : ''} />
                          <StepIndicator done={clawStep === 'verifying' || clawStep === 'done'} active={clawStep === 'signing'} label="签名挑战码" />
                          <StepIndicator done={clawStep === 'done'} active={clawStep === 'verifying'} label="验证身份" />
                          {clawStep === 'done' && (
                            <div className="flex items-center gap-2 text-green-600 text-sm font-medium pt-1">
                              <CheckCircle2 className="w-4 h-4" /> 登录成功，正在跳转...
                            </div>
                          )}
                        </div>
                      )}

                      {clawStep === 'error' && (
                        <div className="space-y-2">
                          <div className="flex items-center gap-2 text-red-500 text-sm">
                            <AlertCircle className="w-4 h-4" /> {msg?.text || '连接失败'}
                          </div>
                          <button type="button" onClick={() => { setClawStep('input'); setMsg(null); }}
                            className="w-full py-2 rounded-lg border border-orange-300 text-orange-600 text-sm font-medium hover:bg-orange-50 transition">
                            重试
                          </button>
                        </div>
                      )}
                    </div>
                  )}

                  <p className="text-center text-xs text-gray-400">登录后可管理你的开发者资源和市场发布</p>
                </form>
              )}

              {/* Register Form */}
              {tab === 'register' && (
                <form className="p-8 space-y-5" onSubmit={handleRegister}>
                  <div className="flex bg-gray-100 rounded-lg p-0.5 mb-1">
                    <button type="button" onClick={() => setRegMethod('email')} className={regMethod === 'email' ? BTN_ON : BTN_OFF}>邮箱注册</button>
                    <button type="button" onClick={() => setRegMethod('phone')} className={regMethod === 'phone' ? BTN_ON : BTN_OFF}>手机注册</button>
                  </div>
                  {regMethod === 'email' ? (
                    <div>
                      <label className="block text-sm font-medium text-gray-700 mb-1.5">邮箱</label>
                      <input type="email" value={regEmail} onChange={e => setRegEmail(e.target.value)} className={INPUT} placeholder="you@example.com" />
                    </div>
                  ) : (
                    <div>
                      <label className="block text-sm font-medium text-gray-700 mb-1.5">手机号</label>
                      <input type="tel" value={regPhone} onChange={e => setRegPhone(e.target.value)} className={INPUT} placeholder="13800138000" />
                    </div>
                  )}
                  <div>
                    <label className="block text-sm font-medium text-gray-700 mb-1.5">昵称</label>
                    <input type="text" value={regNickname} onChange={e => setRegNickname(e.target.value)} className={INPUT} placeholder="你的昵称" required />
                  </div>
                  <div>
                    <label className="block text-sm font-medium text-gray-700 mb-1.5">密码</label>
                    <input type="password" value={regPassword} onChange={e => setRegPassword(e.target.value)} className={INPUT} placeholder="至少 6 位" required minLength={6} />
                  </div>
                  {msg && <p className={`text-sm ${msg.error ? 'text-red-500' : 'text-green-600'}`}>{msg.text}</p>}
                  <button type="submit" disabled={loading}
                    className="w-full py-3 rounded-xl bg-indigo-600 text-white font-semibold text-sm hover:bg-indigo-700 transition shadow-lg shadow-indigo-500/25 disabled:opacity-50">
                    {loading ? '注册中...' : '创建账号'}
                  </button>

                  {/* OAuth divider */}
                  <div className="relative my-2">
                    <div className="absolute inset-0 flex items-center"><div className="w-full border-t border-gray-200" /></div>
                    <div className="relative flex justify-center text-xs"><span className="bg-white px-3 text-gray-400">或使用第三方账号注册</span></div>
                  </div>

                  {/* OAuth buttons */}
                  <div className="grid grid-cols-2 gap-3">
                    <button type="button" onClick={loginWithGoogle}
                      className="flex items-center justify-center gap-2 py-2.5 rounded-xl border border-gray-200 text-sm font-medium text-gray-700 hover:bg-gray-50 hover:border-gray-300 transition">
                      <GoogleIcon />
                      Google
                    </button>
                    <button type="button" onClick={loginWithGithub}
                      className="flex items-center justify-center gap-2 py-2.5 rounded-xl border border-gray-200 text-sm font-medium text-gray-700 hover:bg-gray-50 hover:border-gray-300 transition">
                      <Github className="w-4 h-4" />
                      GitHub
                    </button>
                  </div>

                  <p className="text-center text-xs text-gray-400">
                    注册即同意 <a href="#" className="text-indigo-500 hover:underline">服务条款</a> 和 <a href="#" className="text-indigo-500 hover:underline">隐私政策</a>
                  </p>
                </form>
              )}
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
        <Spinner className="w-4 h-4 text-orange-500 animate-spin flex-none" />
      ) : (
        <div className="w-4 h-4 rounded-full border-2 border-gray-200 flex-none" />
      )}
      <span className={done ? 'text-green-700' : active ? 'text-orange-700 font-medium' : 'text-gray-400'}>{label}</span>
      {sub && <span className="text-[10px] text-gray-400 font-mono">{sub}</span>}
    </div>
  );
}

function GoogleIcon() {
  return (
    <svg className="w-4 h-4" viewBox="0 0 24 24">
      <path d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92a5.06 5.06 0 01-2.2 3.32v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.1z" fill="#4285F4"/>
      <path d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z" fill="#34A853"/>
      <path d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.07H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.93l2.85-2.22.81-.62z" fill="#FBBC05"/>
      <path d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.07l3.66 2.84c.87-2.6 3.3-4.53 6.16-4.53z" fill="#EA4335"/>
    </svg>
  );
}
