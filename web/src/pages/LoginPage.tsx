import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Eye, EyeOff } from 'lucide-react'

const CrawfishIcon = ({ className }: { className?: string }) => (
  <svg className={className} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
    <path d="M12 19c-2 0-4-1-5-3s-1-4 0-6c1-1.5 3-3 5-3s4 1.5 5 3c1 2 1 4 0 6s-3 3-5 3z" />
    <path d="M9 10c-2-2-4-3-6-2" />
    <path d="M15 10c2-2 4-3 6-2" />
    <path d="M8 7c-1-2-1-4 0-5" />
    <path d="M16 7c1-2 1-4 0-5" />
    <circle cx="10" cy="11" r="0.8" fill="currentColor" />
    <circle cx="14" cy="11" r="0.8" fill="currentColor" />
    <path d="M10 16c0.5 0.5 1.5 1 2 1s1.5-0.5 2-1" />
    <path d="M9 19l-1 3" />
    <path d="M15 19l1 3" />
    <path d="M11 19.5l-0.5 2.5" />
    <path d="M13 19.5l0.5 2.5" />
  </svg>
)
import { authAPI } from '../lib/api'
import { useAuthStore } from '../stores/authStore'

export default function LoginPage() {
  const [isRegister, setIsRegister] = useState(false)
  const [loginMode, setLoginMode] = useState<'email' | 'phone'>('email')
  const [email, setEmail] = useState('')
  const [phone, setPhone] = useState('')
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)
  const [showPwd, setShowPwd] = useState(false)
  const [rememberMe, setRememberMe] = useState(true)
  const navigate = useNavigate()
  const setAuth = useAuthStore((s) => s.setAuth)

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    setLoading(true)

    try {
      let res
      if (loginMode === 'phone') {
        res = isRegister
          ? await authAPI.phoneRegister({ phone, password, username: username || undefined })
          : await authAPI.phoneLogin({ phone, password })
      } else {
        res = isRegister
          ? await authAPI.register({ email, username, password })
          : await authAPI.login({ email, password })
      }

      setAuth(res.data.token, res.data.user)
      navigate('/chat')
    } catch (err: unknown) {
      const axiosErr = err as { response?: { data?: { error?: string } } }
      setError(axiosErr.response?.data?.error || '操作失败，请重试')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="min-h-screen flex items-center justify-center bg-gradient-to-br from-gray-900 via-gray-800 to-gray-900">
      <div className="w-full max-w-md">
        <div className="text-center mb-8">
          <div className="inline-flex items-center gap-2 mb-2">
            <CrawfishIcon className="w-12 h-12 text-red-400" />
            <h1 className="text-4xl font-bold text-white">StarClaw</h1>
          </div>
          <p className="text-gray-400">AI Agent 智能体平台</p>
        </div>

        <div className="bg-white rounded-2xl shadow-2xl p-8">
          <h2 className="text-2xl font-semibold mb-6">
            {isRegister ? '创建账号' : '登录'}
          </h2>

          {error && (
            <div className="mb-4 p-3 bg-red-50 border border-red-200 rounded-lg text-red-600 text-sm">
              {error}
            </div>
          )}

          <form onSubmit={handleSubmit} className="space-y-4">
            {/* Email / Phone mode toggle */}
            <div className="flex rounded-lg bg-gray-100 p-1">
              <button
                type="button"
                onClick={() => { setLoginMode('email'); setError('') }}
                className={`flex-1 py-2 text-sm font-medium rounded-md transition-all ${loginMode === 'email' ? 'bg-white text-primary-600 shadow-sm' : 'text-gray-500 hover:text-gray-700'}`}
              >
                邮箱
              </button>
              <button
                type="button"
                onClick={() => { setLoginMode('phone'); setError('') }}
                className={`flex-1 py-2 text-sm font-medium rounded-md transition-all ${loginMode === 'phone' ? 'bg-white text-primary-600 shadow-sm' : 'text-gray-500 hover:text-gray-700'}`}
              >
                手机号
              </button>
            </div>

            {loginMode === 'email' ? (
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">邮箱</label>
                <input
                  type="email"
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  className="w-full px-4 py-2.5 border border-gray-300 rounded-lg focus:ring-2 focus:ring-primary-500 focus:border-transparent outline-none transition-all"
                  placeholder="your@email.com"
                  required
                />
              </div>
            ) : (
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">手机号</label>
                <input
                  type="tel"
                  value={phone}
                  onChange={(e) => setPhone(e.target.value)}
                  className="w-full px-4 py-2.5 border border-gray-300 rounded-lg focus:ring-2 focus:ring-primary-500 focus:border-transparent outline-none transition-all"
                  placeholder="13800138000"
                  required
                />
              </div>
            )}

            {isRegister && (
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">
                  用户名{loginMode === 'phone' ? '（可选）' : ''}
                </label>
                <input
                  type="text"
                  value={username}
                  onChange={(e) => setUsername(e.target.value)}
                  className="w-full px-4 py-2.5 border border-gray-300 rounded-lg focus:ring-2 focus:ring-primary-500 focus:border-transparent outline-none transition-all"
                  placeholder="your_username"
                  required={loginMode === 'email'}
                  minLength={3}
                />
              </div>
            )}

            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">
                密码
              </label>
              <div className="relative">
                <input
                  type={showPwd ? 'text' : 'password'}
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  className="w-full px-4 py-2.5 pr-10 border border-gray-300 rounded-lg focus:ring-2 focus:ring-primary-500 focus:border-transparent outline-none transition-all"
                  placeholder="••••••"
                  required
                  minLength={6}
                />
                <button
                  type="button"
                  onClick={() => setShowPwd(!showPwd)}
                  className="absolute right-3 top-1/2 -translate-y-1/2 text-gray-400 hover:text-gray-600"
                >
                  {showPwd ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
                </button>
              </div>
            </div>

            {!isRegister && (
              <label className="flex items-center gap-2 text-sm text-gray-600 cursor-pointer">
                <input
                  type="checkbox"
                  checked={rememberMe}
                  onChange={(e) => setRememberMe(e.target.checked)}
                  className="rounded border-gray-300 text-primary-600 focus:ring-primary-500"
                />
                记住我
              </label>
            )}

            <button
              type="submit"
              disabled={loading}
              className="w-full py-2.5 bg-primary-600 text-white rounded-lg font-medium hover:bg-primary-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
            >
              {loading ? '处理中...' : isRegister ? '注册' : '登录'}
            </button>
          </form>

          <div className="mt-6 text-center text-sm text-gray-500">
            {isRegister ? '已有账号？' : '没有账号？'}
            <button
              onClick={() => {
                setIsRegister(!isRegister)
                setError('')
              }}
              className="text-primary-600 hover:text-primary-700 font-medium ml-1"
            >
              {isRegister ? '去登录' : '注册'}
            </button>
          </div>
        </div>
      </div>
    </div>
  )
}
