import { useState } from 'react';
import { useNavigate, Link } from 'react-router-dom';
import { Zap } from 'lucide-react';
import { auth, setToken } from '../lib/api';

export default function LoginPage() {
  const navigate = useNavigate();
  const [mode, setMode] = useState<'phone' | 'email'>('phone');
  const [account, setAccount] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');
    setLoading(true);
    try {
      const payload = mode === 'phone'
        ? { phone: account, password }
        : { email: account, password };
      const res = await auth.login(payload);
      setToken(res.token);
      navigate('/dashboard');
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Login failed');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="min-h-screen bg-gray-950 flex items-center justify-center px-4">
      <div className="w-full max-w-sm">
        <div className="text-center mb-8">
          <div className="inline-flex items-center gap-3 mb-2">
            <div className="w-10 h-10 bg-gradient-to-br from-amber-400 to-orange-500 rounded-xl flex items-center justify-center shadow-lg shadow-amber-500/20">
              <Zap className="w-6 h-6 text-white" />
            </div>
            <span className="text-2xl font-bold tracking-tight text-white">Star<span className="bg-gradient-to-r from-amber-400 to-orange-400 bg-clip-text text-transparent">AI</span></span>
          </div>
          <p className="text-gray-400 text-sm">AI 算力平台</p>
        </div>

        <form onSubmit={handleSubmit} className="bg-gray-900 border border-gray-800 rounded-xl p-6 space-y-4">
          <h2 className="text-lg font-semibold text-white">登录</h2>

          <div className="flex gap-2 bg-gray-800 rounded-lg p-0.5">
            <button type="button" onClick={() => { setMode('phone'); setAccount(''); }} className={`flex-1 py-1.5 text-sm rounded-md transition-colors cursor-pointer ${mode === 'phone' ? 'bg-gray-700 text-white' : 'text-gray-400 hover:text-gray-300'}`}>手机号</button>
            <button type="button" onClick={() => { setMode('email'); setAccount(''); }} className={`flex-1 py-1.5 text-sm rounded-md transition-colors cursor-pointer ${mode === 'email' ? 'bg-gray-700 text-white' : 'text-gray-400 hover:text-gray-300'}`}>邮箱</button>
          </div>

          {error && (
            <div className="bg-red-500/10 border border-red-500/20 text-red-400 text-sm px-3 py-2 rounded-lg">
              {error}
            </div>
          )}

          <div>
            <label className="block text-sm text-gray-400 mb-1">{mode === 'phone' ? '手机号' : '邮箱'}</label>
            <input
              type={mode === 'phone' ? 'tel' : 'email'}
              value={account}
              onChange={e => setAccount(e.target.value)}
              required
              className="w-full bg-gray-800 border border-gray-700 rounded-lg px-3 py-2 text-white text-sm focus:outline-none focus:border-amber-500 transition-colors"
              placeholder={mode === 'phone' ? '13800138000' : 'you@example.com'}
            />
          </div>

          <div>
            <label className="block text-sm text-gray-400 mb-1">密码</label>
            <input
              type="password"
              value={password}
              onChange={e => setPassword(e.target.value)}
              required
              className="w-full bg-gray-800 border border-gray-700 rounded-lg px-3 py-2 text-white text-sm focus:outline-none focus:border-amber-500 transition-colors"
              placeholder="••••••"
            />
          </div>

          <button
            type="submit"
            disabled={loading}
            className="w-full bg-amber-500 hover:bg-amber-400 disabled:opacity-50 text-gray-900 font-medium py-2.5 rounded-lg text-sm transition-colors cursor-pointer"
          >
            {loading ? '登录中...' : '登录'}
          </button>

          <p className="text-center text-sm text-gray-500">
            还没有账号？{' '}
            <Link to="/register" className="text-amber-400 hover:text-amber-300">
              注册
            </Link>
          </p>
        </form>
      </div>
    </div>
  );
}
