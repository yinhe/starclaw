import { useState, useRef, useEffect } from 'react';
import { useNavigate, Link, useSearchParams } from 'react-router-dom';
import { Zap, Fingerprint, CheckCircle2, AlertCircle, Loader2, Shield } from 'lucide-react';
import { clawAuth, clawNodeRequest, setToken } from '../lib/api';

type Step = 'input' | 'connecting' | 'waiting' | 'verifying' | 'done' | 'error';

export default function LoginPage() {
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const paramUrl = searchParams.get('claw_url');
  const [clawUrl, setClawUrl] = useState(paramUrl || 'http://localhost:8080');
  const autoStarted = useRef(false);
  const [step, setStep] = useState<Step>('input');
  const [nodeInfo, setNodeInfo] = useState<{ node_id: string } | null>(null);
  const [msg, setMsg] = useState<{ text: string; error: boolean } | null>(null);
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null);

  async function handleClawLogin() {
    setStep('connecting');
    setMsg(null);
    const baseUrl = clawUrl.replace(/\/$/, '');
    try {
      const info = await clawNodeRequest<{ node_id: string; public_key: string }>(
        baseUrl, '/v1/identity/info'
      );
      setNodeInfo(info);

      const { challenge } = await clawAuth.challenge();

      const reqRes = await clawNodeRequest<{ id: string; status: string }>(
        baseUrl, '/v1/identity/auth-request',
        { method: 'POST', body: JSON.stringify({ challenge, origin: window.location.hostname }) }
      );
      const requestId = reqRes.id;

      setStep('waiting');
      setMsg({ text: '请在你的 Claw 界面确认授权登录', error: false });

      await new Promise<void>((resolve, reject) => {
        let attempts = 0;
        pollRef.current = setInterval(async () => {
          attempts++;
          if (attempts > 100) {
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
              setStep('verifying');
              try {
                const data = await clawAuth.verify({
                  challenge: status.challenge!,
                  node_id: status.node_id!,
                  public_key: status.public_key!,
                  signature: status.signature!,
                });
                setStep('done');
                setToken(data.token);
                setMsg({ text: `${info.node_id.slice(0, 18)}... 身份验证成功`, error: false });
                setTimeout(() => navigate('/dashboard'), 1200);
                resolve();
              } catch (verifyErr: any) {
                reject(new Error(verifyErr.message || 'StarAI 验证身份失败'));
              }
              return;
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
      setStep('error');
      setMsg({ text: e.message || '连接 Claw 节点失败', error: true });
    }
  }

  // Auto-start auth if claw_url provided via URL params (e.g. from Claw wallet)
  useEffect(() => {
    if (paramUrl && !autoStarted.current && step === 'input') {
      autoStarted.current = true;
      handleClawLogin();
    }
  }, [paramUrl]); // eslint-disable-line react-hooks/exhaustive-deps

  const INPUT = 'w-full bg-gray-800 border border-gray-700 rounded-lg px-3 py-2.5 text-white text-sm focus:outline-none focus:border-amber-500 transition-colors';

  return (
    <div className="min-h-screen bg-gray-950 flex items-center justify-center px-4">
      <div className="w-full max-w-sm">
        {/* Logo */}
        <div className="text-center mb-8">
          <div className="inline-flex items-center gap-3 mb-2">
            <div className="w-10 h-10 bg-gradient-to-br from-amber-400 to-orange-500 rounded-xl flex items-center justify-center shadow-lg shadow-amber-500/20">
              <Zap className="w-6 h-6 text-white" />
            </div>
            <span className="text-2xl font-bold tracking-tight text-white">Star<span className="bg-gradient-to-r from-amber-400 to-orange-400 bg-clip-text text-transparent">AI</span></span>
          </div>
          <p className="text-gray-400 text-sm">AI 算力平台</p>
        </div>

        <div className="bg-gray-900 border border-gray-800 rounded-xl p-6 space-y-5">
          {/* Header */}
          <div className="text-center">
            <div className="w-12 h-12 rounded-full bg-amber-500/10 flex items-center justify-center mx-auto mb-3">
              <Fingerprint className="w-6 h-6 text-amber-400" />
            </div>
            <h2 className="text-lg font-semibold text-white">Claw 节点认证</h2>
            <p className="text-sm text-gray-500 mt-1">输入节点地址，在你的 Claw 上确认授权</p>
          </div>

          {/* Node URL Input */}
          {step === 'input' && (
            <>
              <div>
                <label className="block text-sm text-gray-400 mb-1.5">Claw 节点地址</label>
                <input type="url" value={clawUrl} onChange={e => setClawUrl(e.target.value)}
                  className={INPUT} placeholder="http://localhost:8080"
                  onKeyDown={e => e.key === 'Enter' && handleClawLogin()} />
              </div>

              <div className="flex items-start gap-2 p-3 bg-amber-500/5 border border-amber-500/10 rounded-lg">
                <Shield className="w-4 h-4 text-amber-400 mt-0.5 flex-none" />
                <p className="text-xs text-amber-300/70 leading-relaxed">
                  安全双向认证：请求会发送到你的 Claw 节点，你需要在 Claw 界面确认授权后才能登录，无人可冒充。
                </p>
              </div>

              {msg && <p className={`text-sm ${msg.error ? 'text-red-400' : 'text-green-400'}`}>{msg.text}</p>}

              <button type="button" onClick={handleClawLogin}
                className="w-full flex items-center justify-center gap-2.5 py-2.5 rounded-lg bg-amber-500 hover:bg-amber-400 text-gray-900 font-semibold text-sm transition-colors cursor-pointer shadow-lg shadow-amber-500/20">
                <Fingerprint className="w-5 h-5" />
                发送认证请求
              </button>
            </>
          )}

          {/* Progress Steps */}
          {step !== 'input' && step !== 'error' && (
            <div className="space-y-3 py-2">
              <StepLine done={step !== 'connecting'} active={step === 'connecting'}
                label="连接 Claw 节点" sub={nodeInfo ? nodeInfo.node_id.slice(0, 20) + '...' : ''} />
              <StepLine done={step === 'verifying' || step === 'done'} active={step === 'waiting'}
                label="等待 Claw 授权确认" sub={step === 'waiting' ? '请在 Claw 界面点击「授权登录」' : ''} />
              <StepLine done={step === 'done'} active={step === 'verifying'}
                label="StarAI 验证身份" />
              {step === 'done' && msg && (
                <div className="flex items-center gap-2 text-green-400 text-sm font-medium pt-2">
                  <CheckCircle2 className="w-4 h-4" /> {msg.text}
                </div>
              )}
            </div>
          )}

          {/* Error */}
          {step === 'error' && (
            <div className="space-y-3">
              <div className="flex items-center gap-2 text-red-400 text-sm">
                <AlertCircle className="w-4 h-4 flex-none" />
                <span>{msg?.text || '连接失败'}</span>
              </div>
              <button type="button" onClick={() => { setStep('input'); setMsg(null); }}
                className="w-full py-2.5 rounded-lg border border-gray-700 text-gray-400 text-sm font-medium hover:bg-gray-800 transition cursor-pointer">
                重试
              </button>
            </div>
          )}

          <p className="text-center text-xs text-gray-600 pt-1">
            没有 Claw 节点？<a href="https://starclaw.net/download" className="text-amber-500 hover:underline">免费安装一个</a>
          </p>
        </div>

        <p className="mt-6 text-center text-xs text-gray-600">
          <Link to="/" className="hover:text-gray-400">首页</Link>
          <span className="mx-2">&middot;</span>
          <Link to="/terms" className="hover:text-gray-400">服务条款</Link>
          <span className="mx-2">&middot;</span>
          <Link to="/privacy" className="hover:text-gray-400">隐私政策</Link>
        </p>
      </div>
    </div>
  );
}

function StepLine({ done, active, label, sub }: { done: boolean; active: boolean; label: string; sub?: string }) {
  return (
    <div className="flex items-center gap-2.5 text-sm">
      {done ? (
        <CheckCircle2 className="w-4 h-4 text-green-400 flex-none" />
      ) : active ? (
        <Loader2 className="w-4 h-4 text-amber-400 animate-spin flex-none" />
      ) : (
        <div className="w-4 h-4 rounded-full border-2 border-gray-700 flex-none" />
      )}
      <span className={done ? 'text-green-400' : active ? 'text-amber-300 font-medium' : 'text-gray-600'}>{label}</span>
      {sub && <span className="text-[10px] text-gray-600 font-mono">{sub}</span>}
    </div>
  );
}
