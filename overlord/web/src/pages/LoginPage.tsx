import { useState } from 'react'
import { Loader2, Server } from 'lucide-react'
import { api, setAuth } from '../api/client'

type LoginTab = 'password' | 'node'

export default function LoginPage({ onLogin }: { onLogin: () => void }) {
  const [tab, setTab] = useState<LoginTab>('password')
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [nodeAddress, setNodeAddress] = useState('')
  const [clawToken, setClawToken] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  const handlePasswordLogin = async (e: React.FormEvent) => {
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

  const handleNodeLogin = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    setLoading(true)
    try {
      const data = await api.nodeLogin(nodeAddress, clawToken)
      setAuth(data.token, data.user)
      onLogin()
    } catch (err: any) {
      setError(err.message || '节点登录失败')
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

        {/* Tab switcher */}
        <div className="flex mb-4 bg-gray-900 rounded-lg border border-gray-800 p-1">
          <button
            onClick={() => { setTab('password'); setError('') }}
            className={`flex-1 py-2 text-xs font-medium rounded-md transition ${
              tab === 'password' ? 'bg-gray-800 text-white' : 'text-gray-500 hover:text-gray-300'
            }`}
          >
            账号密码
          </button>
          <button
            onClick={() => { setTab('node'); setError('') }}
            className={`flex-1 py-2 text-xs font-medium rounded-md transition flex items-center justify-center gap-1.5 ${
              tab === 'node' ? 'bg-gray-800 text-white' : 'text-gray-500 hover:text-gray-300'
            }`}
          >
            <Server className="w-3.5 h-3.5" />
            节点登录
          </button>
        </div>

        {tab === 'password' ? (
          <form onSubmit={handlePasswordLogin} className="bg-gray-900 rounded-xl border border-gray-800 p-6 space-y-4">
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
        ) : (
          <form onSubmit={handleNodeLogin} className="bg-gray-900 rounded-xl border border-gray-800 p-6 space-y-4">
            {error && (
              <div className="text-sm text-red-400 bg-red-400/10 rounded-lg px-3 py-2">{error}</div>
            )}
            <div>
              <label className="block text-xs text-gray-400 mb-1.5">节点地址</label>
              <input
                type="text"
                value={nodeAddress}
                onChange={e => setNodeAddress(e.target.value)}
                className="w-full px-3 py-2.5 bg-gray-800 border border-gray-700 rounded-lg text-sm text-white placeholder-gray-500 focus:outline-none focus:border-brand-500 transition"
                placeholder="例如: https://starclaw.me:8080"
                autoFocus
              />
            </div>
            <div>
              <label className="block text-xs text-gray-400 mb-1.5">Claw Token</label>
              <input
                type="password"
                value={clawToken}
                onChange={e => setClawToken(e.target.value)}
                className="w-full px-3 py-2.5 bg-gray-800 border border-gray-700 rounded-lg text-sm text-white placeholder-gray-500 focus:outline-none focus:border-brand-500 transition"
                placeholder="从 Claw 节点获取的 JWT Token"
              />
            </div>
            <p className="text-[11px] text-gray-500 leading-relaxed">
              使用已注册 Claw 节点的 Token 登录。节点需在 Overlord 注册并处于在线状态。
            </p>
            <button
              type="submit"
              disabled={loading || !nodeAddress || !clawToken}
              className="w-full py-2.5 bg-brand-600 hover:bg-brand-500 disabled:bg-gray-700 disabled:text-gray-500 text-white text-sm font-medium rounded-lg transition flex items-center justify-center gap-2"
            >
              {loading && <Loader2 className="w-4 h-4 animate-spin" />}
              节点登录
            </button>
          </form>
        )}

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
