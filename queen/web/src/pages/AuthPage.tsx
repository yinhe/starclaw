import { useState } from 'react';
import { useSearchParams, useNavigate, Link } from 'react-router-dom';
import { LogoMark } from '../components/Logo';
import { clawAuthAPI, clawNodeRequest } from '../lib/api';
import { isLoggedIn, setAuth, clearAuth, getUserDisplayName } from '../lib/auth';
import { Fingerprint, CheckCircle2, AlertCircle, Loader2 as Spinner, Shield } from 'lucide-react';

export function AuthPage() {
  const [searchParams] = useSearchParams();
  const navigate = useNavigate();
  const logged = isLoggedIn();

  // Claw node login state
  const [clawUrl, setClawUrl] = useState('http://localhost:8080');
  const [clawPassword, setClawPassword] = useState('');
  const [clawStep, setClawStep] = useState<'input' | 'authenticating' | 'connecting' | 'signing' | 'verifying' | 'done' | 'error'>('input');
  const [clawNodeInfo, setClawNodeInfo] = useState<{ node_id: string; public_key: string } | null>(null);
  const [msg, setMsg] = useState<{ text: string; error: boolean } | null>(null);

  async function handleClawLogin() {
    if (!clawPassword) { setMsg({ text: '请输入 Claw 节点密码', error: true }); return; }
    setClawStep('authenticating');
    setMsg(null);
    const baseUrl = clawUrl.replace(/\/$/, '');
    try {
      // Step 1: Authenticate with Claw node (owner-login) to get a token
      const loginRes = await clawNodeRequest<{ token: string }>(
        baseUrl, '/v1/auth/owner-login',
        { method: 'POST', body: JSON.stringify({ password: clawPassword }) }
      );
      const clawToken = loginRes.token;

      // Step 2: Get node identity (public, just for display)
      setClawStep('connecting');
      const info = await clawNodeRequest<{ node_id: string; public_key: string }>(
        baseUrl, '/v1/identity/info'
      );
      setClawNodeInfo(info);

      // Step 3: Get challenge from Queen
      setClawStep('signing');
      const { challenge } = await clawAuthAPI.challenge();

      // Step 4: Send challenge to Claw node for counter-signature (PROTECTED — requires Claw token)
      const signed = await clawNodeRequest<{ node_id: string; public_key: string; signature: string; challenge: string }>(
        baseUrl, '/v1/identity/sign-challenge',
        { method: 'POST', body: JSON.stringify({ challenge }), token: clawToken }
      );

      // Step 5: Submit signature to Queen for verification
      setClawStep('verifying');
      const data = await clawAuthAPI.verify({
        challenge: signed.challenge,
        node_id: signed.node_id,
        public_key: signed.public_key,
        signature: signed.signature,
      });

      // Success — account created or logged in
      setClawStep('done');
      setAuth(data.token, data.user);
      setMsg({ text: `${info.node_id.slice(0, 18)}... 身份验证成功`, error: false });
      setTimeout(() => navigate(searchParams.get('redirect') || '/dashboard'), 1200);
    } catch (e: any) {
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
              <div className="p-8 space-y-5">
                {/* Header */}
                <div className="text-center mb-2">
                  <div className="w-14 h-14 rounded-full bg-indigo-50 flex items-center justify-center mx-auto mb-4">
                    <Fingerprint className="w-7 h-7 text-indigo-500" />
                  </div>
                  <h2 className="text-lg font-bold text-gray-900">Claw 节点认证</h2>
                  <p className="text-sm text-gray-400 mt-1">输入节点地址和密码，通过签名验证身份</p>
                </div>

                {/* Node URL + Password Input */}
                {clawStep === 'input' && (
                  <>
                    <div>
                      <label className="block text-sm font-medium text-gray-700 mb-1.5">Claw 节点地址</label>
                      <input type="url" value={clawUrl} onChange={e => setClawUrl(e.target.value)}
                        className={INPUT} placeholder="http://localhost:8080" />
                    </div>
                    <div>
                      <label className="block text-sm font-medium text-gray-700 mb-1.5">节点密码</label>
                      <input type="password" value={clawPassword} onChange={e => setClawPassword(e.target.value)}
                        className={INPUT} placeholder="你的 Claw 节点登录密码"
                        onKeyDown={e => e.key === 'Enter' && handleClawLogin()} />
                    </div>

                    <div className="flex items-start gap-2 p-3 bg-indigo-50/70 rounded-xl">
                      <Shield className="w-4 h-4 text-indigo-400 mt-0.5 flex-none" />
                      <p className="text-xs text-indigo-600/70 leading-relaxed">
                        安全回签：需要节点密码授权签名，防止他人用你的地址冒充登录。
                      </p>
                    </div>

                    {msg && <p className={`text-sm ${msg.error ? 'text-red-500' : 'text-green-600'}`}>{msg.text}</p>}

                    <button type="button" onClick={handleClawLogin}
                      className="w-full flex items-center justify-center gap-2.5 py-3 rounded-xl bg-indigo-600 text-white font-semibold text-sm hover:bg-indigo-700 transition shadow-lg shadow-indigo-500/25">
                      <Fingerprint className="w-5 h-5" />
                      连接节点并认证
                    </button>
                  </>
                )}

                {/* Progress Steps */}
                {clawStep !== 'input' && clawStep !== 'error' && (
                  <div className="space-y-3 py-2">
                    <StepIndicator done={clawStep !== 'authenticating'} active={clawStep === 'authenticating'}
                      label="验证节点密码" />
                    <StepIndicator done={!['authenticating', 'connecting'].includes(clawStep)} active={clawStep === 'connecting'}
                      label="连接 Claw 节点" sub={clawNodeInfo ? clawNodeInfo.node_id.slice(0, 20) + '...' : ''} />
                    <StepIndicator done={clawStep === 'verifying' || clawStep === 'done'} active={clawStep === 'signing'}
                      label="节点回签挑战码" sub="Ed25519 签名" />
                    <StepIndicator done={clawStep === 'done'} active={clawStep === 'verifying'}
                      label="Queen 验证身份" />
                    {clawStep === 'done' && msg && (
                      <div className="flex items-center gap-2 text-green-600 text-sm font-medium pt-2">
                        <CheckCircle2 className="w-4 h-4" /> {msg.text}
                      </div>
                    )}
                  </div>
                )}

                {/* Error */}
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
