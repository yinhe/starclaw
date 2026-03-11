import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { Shield } from 'lucide-react';
import { auth, admin, setToken, clearToken } from '../lib/api';

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
      // Verify admin access
      try {
        await admin.me();
      } catch {
        clearToken();
        throw new Error('该账号没有管理后台权限');
      }
      navigate('/');
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Login failed');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="min-h-screen bg-gray-950 flex items-center justify-center px-4">
      <div className="w-full max-w-sm">
        <div className="flex flex-col items-center mb-8">
          <div className="w-12 h-12 bg-gradient-to-br from-rose-500 to-pink-600 rounded-xl flex items-center justify-center mb-4">
            <Shield className="w-6 h-6 text-white" />
          </div>
          <h1 className="text-2xl font-bold text-white">Star-AI Admin</h1>
          <p className="text-gray-500 text-sm mt-1">管理后台登录</p>
        </div>

        <form onSubmit={handleSubmit} className="space-y-4">
          <div className="flex gap-2 bg-gray-900 rounded-lg p-0.5">
            <button type="button" onClick={() => { setMode('phone'); setAccount(''); }} className={`flex-1 py-1.5 text-sm rounded-md transition-colors cursor-pointer ${mode === 'phone' ? 'bg-gray-800 text-white' : 'text-gray-500 hover:text-gray-300'}`}>手机号</button>
            <button type="button" onClick={() => { setMode('email'); setAccount(''); }} className={`flex-1 py-1.5 text-sm rounded-md transition-colors cursor-pointer ${mode === 'email' ? 'bg-gray-800 text-white' : 'text-gray-500 hover:text-gray-300'}`}>邮箱</button>
          </div>

          {error && (
            <div className="bg-red-500/10 border border-red-500/20 rounded-lg px-4 py-3 text-red-400 text-sm">
              {error}
            </div>
          )}

          <div>
            <label className="block text-sm text-gray-400 mb-1.5">{mode === 'phone' ? '手机号' : '邮箱'}</label>
            <input
              type={mode === 'phone' ? 'tel' : 'email'}
              value={account}
              onChange={(e) => setAccount(e.target.value)}
              className="w-full bg-gray-900 border border-gray-800 rounded-lg px-4 py-2.5 text-white placeholder-gray-600 focus:outline-none focus:border-rose-500/50 transition-colors"
              placeholder={mode === 'phone' ? '13800138000' : 'admin@star-ai.net'}
              required
            />
          </div>

          <div>
            <label className="block text-sm text-gray-400 mb-1.5">密码</label>
            <input
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              className="w-full bg-gray-900 border border-gray-800 rounded-lg px-4 py-2.5 text-white placeholder-gray-600 focus:outline-none focus:border-rose-500/50 transition-colors"
              placeholder="••••••••"
              required
            />
          </div>

          <button
            type="submit"
            disabled={loading}
            className="w-full bg-gradient-to-r from-rose-500 to-pink-600 text-white py-2.5 rounded-lg font-medium hover:from-rose-600 hover:to-pink-700 transition-all disabled:opacity-50 cursor-pointer"
          >
            {loading ? '登录中...' : '登录'}
          </button>
        </form>
      </div>
    </div>
  );
}
