import { useEffect, useState } from 'react';
import { useSearchParams, useNavigate } from 'react-router-dom';
import { setAuth } from '../lib/auth';
// Token-based auto-login for Claw nodes
import { Fingerprint, CheckCircle2, AlertCircle, Loader2 as Spinner } from 'lucide-react';

export function ClawLoginPage() {
  const [searchParams] = useSearchParams();
  const navigate = useNavigate();
  const [status, setStatus] = useState<'loading' | 'success' | 'error'>('loading');
  const [msg, setMsg] = useState('正在通过 Claw 身份认证...');

  useEffect(() => {
    const token = searchParams.get('token');
    if (!token) {
      setStatus('error');
      setMsg('缺少认证凭证');
      return;
    }

    // Verify token and get user profile
    (async () => {
      try {
        const res = await fetch(`${import.meta.env.VITE_API_URL || ''}/v1/user/profile`, {
          headers: { Authorization: `Bearer ${token}` },
        });
        if (!res.ok) throw new Error('凭证无效或已过期');
        const data = await res.json();
        const user = data.data || data;

        setAuth(token, {
          id: user.id,
          email: user.email || '',
          nickname: user.nickname || '',
          phone: user.phone || '',
          avatar: user.avatar || '',
          role: user.role || 'user',
        });

        setStatus('success');
        setMsg('认证成功，正在进入虫群门户...');
        const redirect = searchParams.get('redirect') || '/dashboard';
        setTimeout(() => navigate(redirect), 1000);
      } catch (e: any) {
        setStatus('error');
        setMsg(e.message || '认证失败');
      }
    })();
  }, []);

  return (
    <div className="min-h-screen bg-gradient-to-br from-indigo-50 via-white to-purple-50 flex items-center justify-center">
      <div className="bg-white rounded-2xl shadow-lg p-10 max-w-sm w-full text-center">
        <div className="w-16 h-16 mx-auto mb-6 rounded-full bg-indigo-50 flex items-center justify-center">
          {status === 'loading' && <Spinner className="w-8 h-8 text-indigo-500 animate-spin" />}
          {status === 'success' && <CheckCircle2 className="w-8 h-8 text-green-500" />}
          {status === 'error' && <AlertCircle className="w-8 h-8 text-red-500" />}
        </div>
        <div className="flex items-center justify-center gap-2 mb-3">
          <Fingerprint className="w-5 h-5 text-indigo-400" />
          <h1 className="text-lg font-semibold text-gray-800">Claw 身份认证</h1>
        </div>
        <p className={`text-sm ${status === 'error' ? 'text-red-500' : 'text-gray-500'}`}>{msg}</p>
        {status === 'error' && (
          <button
            onClick={() => navigate('/auth')}
            className="mt-4 px-4 py-2 text-sm text-indigo-600 border border-indigo-200 rounded-lg hover:bg-indigo-50"
          >
            返回登录
          </button>
        )}
      </div>
    </div>
  );
}
