import { useState } from 'react'
import { Eye, EyeOff, LogIn } from 'lucide-react'
import { login } from '../api/brood'

export default function LoginPage({ onLogin }: { onLogin: () => void }) {
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [showPw, setShowPw] = useState(false)
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!username.trim() || !password.trim()) return
    setLoading(true)
    setError('')
    try {
      await login(username, password)
      onLogin()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Login failed')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="min-h-screen flex items-center justify-center bg-gray-950">
      <div className="w-full max-w-sm">
        <div className="text-center mb-8">
          <div className="w-16 h-16 rounded-2xl bg-overlord-600/20 flex items-center justify-center mx-auto mb-4">
            <svg className="w-8 h-8 text-overlord-400" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
              <circle cx="12" cy="12" r="3" />
              <path d="M12 1v2M12 21v2M4.22 4.22l1.42 1.42M18.36 18.36l1.42 1.42M1 12h2M21 12h2M4.22 19.78l1.42-1.42M18.36 5.64l1.42-1.42" />
            </svg>
          </div>
          <h1 className="text-xl font-bold text-white">Overlord Console</h1>
          <p className="text-sm text-gray-500 mt-1">虫群企业管理平台</p>
        </div>

        <form onSubmit={handleSubmit} className="bg-gray-900 border border-gray-800 rounded-2xl p-6 space-y-4">
          {error && (
            <div className="bg-red-600/10 border border-red-600/20 rounded-lg px-3 py-2 text-sm text-red-400">
              {error}
            </div>
          )}

          <div>
            <label className="block text-xs text-gray-400 mb-1.5">用户名</label>
            <input
              type="text"
              value={username}
              onChange={e => setUsername(e.target.value)}
              placeholder="admin"
              autoFocus
              className="w-full bg-gray-800 border border-gray-700 rounded-lg px-3 py-2.5 text-white text-sm placeholder-gray-600 focus:outline-none focus:border-overlord-500 transition"
            />
          </div>

          <div>
            <label className="block text-xs text-gray-400 mb-1.5">密码</label>
            <div className="relative">
              <input
                type={showPw ? 'text' : 'password'}
                value={password}
                onChange={e => setPassword(e.target.value)}
                placeholder="••••••••"
                className="w-full bg-gray-800 border border-gray-700 rounded-lg px-3 py-2.5 pr-10 text-white text-sm placeholder-gray-600 focus:outline-none focus:border-overlord-500 transition"
              />
              <button
                type="button"
                onClick={() => setShowPw(!showPw)}
                className="absolute right-2.5 top-1/2 -translate-y-1/2 text-gray-500 hover:text-gray-300 transition"
              >
                {showPw ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
              </button>
            </div>
          </div>

          <button
            type="submit"
            disabled={loading || !username.trim() || !password.trim()}
            className="w-full flex items-center justify-center gap-2 py-2.5 bg-overlord-600 text-white text-sm font-medium rounded-lg hover:bg-overlord-500 disabled:opacity-50 disabled:cursor-not-allowed transition"
          >
            {loading ? (
              <div className="animate-spin w-4 h-4 border-2 border-white border-t-transparent rounded-full" />
            ) : (
              <>
                <LogIn className="w-4 h-4" />
                登录
              </>
            )}
          </button>
        </form>

        <p className="text-center text-[10px] text-gray-700 mt-6">
          StarClaw Overlord &middot; Enterprise Brood Manager
        </p>
      </div>
    </div>
  )
}
