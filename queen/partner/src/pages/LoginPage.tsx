import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { auth, setToken } from '../lib/api'

export default function LoginPage() {
  const navigate = useNavigate()
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    setLoading(true)
    try {
      const res = await auth.login({ email, password })
      if (res.user.role !== 'partner' && res.user.role !== 'admin') {
        setError('此账号不是核心合伙人账号')
        return
      }
      setToken(res.token)
      navigate('/dashboard')
    } catch (err) {
      setError(err instanceof Error ? err.message : '登录失败')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="min-h-screen flex items-center justify-center px-4">
      <div className="w-full max-w-sm">
        <div className="text-center mb-8">
          <h1 className="text-2xl font-bold">
            <span className="text-claw-500">Star</span>Claw
          </h1>
          <p className="text-sm text-gray-400 mt-1">核心合伙人门户</p>
        </div>
        <form onSubmit={handleSubmit} className="space-y-4">
          {error && (
            <div className="rounded-lg bg-red-500/10 border border-red-500/20 p-3 text-sm text-red-400">{error}</div>
          )}
          <div>
            <label className="block text-sm text-gray-400 mb-1.5">邮箱</label>
            <input type="email" required value={email} onChange={e => setEmail(e.target.value)}
              className="w-full rounded-lg border border-white/10 bg-white/[0.02] px-4 py-2.5 text-white text-sm focus:outline-none focus:border-claw-500" />
          </div>
          <div>
            <label className="block text-sm text-gray-400 mb-1.5">密码</label>
            <input type="password" required value={password} onChange={e => setPassword(e.target.value)}
              className="w-full rounded-lg border border-white/10 bg-white/[0.02] px-4 py-2.5 text-white text-sm focus:outline-none focus:border-claw-500" />
          </div>
          <button type="submit" disabled={loading}
            className="w-full rounded-lg bg-claw-600 py-2.5 text-sm font-semibold text-white hover:bg-claw-500 transition-colors disabled:opacity-50">
            {loading ? '登录中...' : '登录'}
          </button>
        </form>
      </div>
    </div>
  )
}
