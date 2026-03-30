import { useState } from 'react'
import { useNavigate, Link } from 'react-router-dom'
import { ArrowLeft, Globe2 } from 'lucide-react'
import { type Lang, t, getBrowserLang } from '../i18n'

interface Props {
  onLogin: (token: string) => void
}

export default function Login({ onLogin }: Props) {
  const navigate = useNavigate()
  const [lang, setLang] = useState<Lang>(getBrowserLang())
  const L = (zh: string, en: string) => t(zh, en, lang)

  const [clawAddr, setClawAddr] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    setLoading(true)

    try {
      const addr = clawAddr.trim().replace(/\/+$/, '')
      const resp = await fetch('/api/v1/node/auth', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ claw_url: addr }),
      })

      if (!resp.ok) {
        const data = await resp.json().catch(() => ({}))
        throw new Error(data.error || L('认证失败', 'Authentication failed'))
      }

      const data = await resp.json()
      onLogin(data.token || addr)
      navigate('/dashboard')
    } catch (err: any) {
      setError(err.message || L('连接失败', 'Connection failed'))
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="min-h-screen bg-gray-950 flex flex-col items-center justify-center px-4">
      {/* Top controls */}
      <div className="w-full max-w-md flex items-center justify-between mb-8">
        <Link to="/" className="inline-flex items-center gap-1.5 text-sm text-gray-500 hover:text-gray-300 transition">
          <ArrowLeft className="w-4 h-4" /> {L('返回', 'Back')}
        </Link>
        <button onClick={() => setLang(lang === 'zh' ? 'en' : 'zh')} className="flex items-center gap-1 text-sm text-gray-400 hover:text-white border border-gray-700 rounded-lg px-2.5 py-1 transition">
          <Globe2 className="w-3.5 h-3.5" /> {lang === 'zh' ? 'EN' : '中文'}
        </button>
      </div>

      {/* Brand */}
      <div className="text-center mb-10">
        <h1 className="text-4xl font-bold">
          <span className="text-red-500">Q8</span><span className="text-white">bot</span>
        </h1>
        <p className="text-gray-500 mt-2 text-sm">{L('AI量化智能体 · 投资人门户', 'AI Quantitative Agent · Investor Portal')}</p>
      </div>

      {/* Login card */}
      <div className="w-full max-w-md bg-gray-900 border border-gray-800 rounded-2xl p-8">
        <p className="text-center text-gray-300 mb-6">
          {L('使用你的 Claw 节点地址登录', 'Login with your Claw node address')}
        </p>

        <form onSubmit={handleSubmit} className="space-y-5">
          <div>
            <label className="block text-sm font-medium text-gray-400 mb-1.5">
              {L('Claw 节点地址', 'Claw Node Address')}
            </label>
            <input
              type="text"
              value={clawAddr}
              onChange={(e) => setClawAddr(e.target.value)}
              placeholder={L('请输入你的 Claw 节点地址', 'Enter your Claw node address')}
              className="w-full bg-gray-950 border border-gray-700 rounded-lg px-4 py-3 text-white placeholder-gray-600 focus:outline-none focus:border-red-500 transition"
              required
            />
          </div>

          {error && (
            <div className="bg-red-500/10 border border-red-500/20 rounded-lg px-4 py-2.5 text-sm text-red-400">
              {error}
            </div>
          )}

          <button
            type="submit"
            disabled={loading}
            className="w-full bg-red-600 hover:bg-red-500 disabled:bg-gray-700 disabled:text-gray-500 text-white font-semibold py-3 rounded-lg transition text-base"
          >
            {loading ? L('认证中...', 'Authenticating...') : L('发送认证请求', 'Send Authentication Request')}
          </button>
        </form>
      </div>

      <p className="text-xs text-gray-600 mt-6 text-center max-w-md">
        {L('Claw 地址需由管理员添加白名单后方可登录', 'Claw address must be whitelisted by an administrator before login')}
      </p>
    </div>
  )
}
