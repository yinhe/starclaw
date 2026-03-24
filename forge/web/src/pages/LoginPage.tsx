import { useState } from 'react'
import { Flame, LogIn, Loader2, ShieldAlert } from 'lucide-react'
import { api, setToken } from '../api'
import { useNavigate } from 'react-router-dom'

export default function LoginPage() {
  const [nodeId, setNodeId] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)
  const navigate = useNavigate()

  const handleLogin = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!nodeId.trim() || !password.trim()) return
    setLoading(true)
    setError('')
    try {
      const r = await api.login(nodeId.trim(), password.trim())
      setToken(r.token, r.node_id)
      navigate('/')
    } catch (err: any) {
      setError(err.message || '登录失败')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="min-h-screen flex items-center justify-center bg-stone-950 px-4">
      <div className="w-full max-w-sm">
        {/* Logo */}
        <div className="text-center mb-8">
          <div className="inline-flex items-center justify-center w-16 h-16 rounded-2xl bg-forge-500/10 border border-forge-500/20 mb-4">
            <Flame className="w-8 h-8 text-forge-500" />
          </div>
          <h1 className="text-2xl font-bold text-stone-100">Forge</h1>
          <p className="text-sm text-stone-500 mt-1">StarClaw 研发管控大屏</p>
        </div>

        {/* Login Form */}
        <form onSubmit={handleLogin} className="glass rounded-xl p-6 space-y-4">
          <div>
            <label className="block text-xs font-medium text-stone-400 mb-1.5">节点 ID</label>
            <input
              type="text"
              value={nodeId}
              onChange={(e) => setNodeId(e.target.value)}
              placeholder="输入节点 ID"
              autoFocus
              className="w-full bg-stone-800 border border-stone-700 rounded-lg px-3 py-2.5 text-sm text-stone-200 placeholder:text-stone-600 focus:outline-none focus:border-forge-500 transition-colors"
            />
          </div>
          <div>
            <label className="block text-xs font-medium text-stone-400 mb-1.5">密码</label>
            <input
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              placeholder="输入密码"
              className="w-full bg-stone-800 border border-stone-700 rounded-lg px-3 py-2.5 text-sm text-stone-200 placeholder:text-stone-600 focus:outline-none focus:border-forge-500 transition-colors"
            />
          </div>

          {error && (
            <div className="flex items-center gap-2 text-sm text-red-400 bg-red-500/10 border border-red-500/20 rounded-lg px-3 py-2">
              <ShieldAlert className="w-4 h-4 shrink-0" />
              {error}
            </div>
          )}

          <button
            type="submit"
            disabled={loading || !nodeId.trim() || !password.trim()}
            className="w-full flex items-center justify-center gap-2 px-4 py-2.5 bg-forge-600 hover:bg-forge-500 disabled:bg-stone-700 disabled:text-stone-500 text-white rounded-lg text-sm font-medium transition-colors"
          >
            {loading ? <Loader2 className="w-4 h-4 animate-spin" /> : <LogIn className="w-4 h-4" />}
            {loading ? '登录中...' : '登录'}
          </button>
        </form>

        <p className="text-center text-xs text-stone-700 mt-6">
          仅限白名单节点访问
        </p>
      </div>
    </div>
  )
}
