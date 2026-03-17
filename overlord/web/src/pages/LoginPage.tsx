import { useState } from 'react'
import { Loader2 } from 'lucide-react'
import { api, setAuth } from '../api/client'

export default function LoginPage({ onLogin }: { onLogin: () => void }) {
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    setLoading(true)
    try {
      const data = await api.login(username, password)
      setAuth(data.token, data.user)
      onLogin()
    } catch (err: any) {
      setError(err.message || '登录失败')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="min-h-screen flex items-center justify-center bg-gray-950">
      <div className="w-full max-w-sm mx-auto">
        <div className="text-center mb-8">
          <div className="text-4xl mb-3">🦞</div>
          <h1 className="text-xl font-bold text-white">StarClaw AI</h1>
          <p className="text-sm text-gray-500 mt-1">企业 AI 智能体工作台</p>
        </div>

        <form onSubmit={handleSubmit} className="bg-gray-900 rounded-xl border border-gray-800 p-6 space-y-4">
          {error && (
            <div className="text-sm text-red-400 bg-red-400/10 rounded-lg px-3 py-2">{error}</div>
          )}
          <div>
            <label className="block text-xs text-gray-400 mb-1.5">用户名</label>
            <input
              type="text"
              value={username}
              onChange={e => setUsername(e.target.value)}
              className="w-full px-3 py-2.5 bg-gray-800 border border-gray-700 rounded-lg text-sm text-white placeholder-gray-500 focus:outline-none focus:border-brand-500 transition"
              placeholder="请输入用户名"
              autoFocus
            />
          </div>
          <div>
            <label className="block text-xs text-gray-400 mb-1.5">密码</label>
            <input
              type="password"
              value={password}
              onChange={e => setPassword(e.target.value)}
              className="w-full px-3 py-2.5 bg-gray-800 border border-gray-700 rounded-lg text-sm text-white placeholder-gray-500 focus:outline-none focus:border-brand-500 transition"
              placeholder="请输入密码"
            />
          </div>
          <button
            type="submit"
            disabled={loading || !username || !password}
            className="w-full py-2.5 bg-brand-600 hover:bg-brand-500 disabled:bg-gray-700 disabled:text-gray-500 text-white text-sm font-medium rounded-lg transition flex items-center justify-center gap-2"
          >
            {loading && <Loader2 className="w-4 h-4 animate-spin" />}
            登录
          </button>
        </form>

        <p className="text-center text-[10px] text-gray-600 mt-6">
          Powered by{' '}
          <a
            href="https://starclaw.net?ref=overlord"
            target="_blank"
            rel="noopener noreferrer"
            className="text-brand-400/60 hover:text-brand-300 transition"
          >
            StarClaw
          </a>
          {' '}&middot; 企业 AI 智能体
        </p>
      </div>
    </div>
  )
}
