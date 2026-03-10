import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Eye, EyeOff, Copy, Check, Shield, ArrowRight, Sparkles } from 'lucide-react'
import { setupAPI } from '../lib/api'
import { useAuthStore } from '../stores/authStore'

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

export default function SetupPage() {
  const navigate = useNavigate()
  const setAuth = useAuthStore((s) => s.setAuth)

  const [step, setStep] = useState<'init' | 'done'>('init')
  const [password, setPassword] = useState('')
  const [username, setUsername] = useState('')
  const [showPwd, setShowPwd] = useState(false)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [ownerToken, setOwnerToken] = useState('')
  const [copied, setCopied] = useState(false)

  const handleSetup = async () => {
    setLoading(true)
    setError('')
    try {
      const res = await setupAPI.setup({
        password: password || undefined,
        username: username || undefined,
      })
      const { owner_token, user } = res.data
      setOwnerToken(owner_token)
      // Store owner_token as the auth token (permanent, no expiry)
      setAuth(owner_token, user)
      setStep('done')
    } catch (err: any) {
      setError(err.response?.data?.error || '初始化失败')
    } finally {
      setLoading(false)
    }
  }

  const handleCopy = () => {
    navigator.clipboard.writeText(ownerToken)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  const handleContinue = () => {
    navigate('/', { replace: true })
  }

  return (
    <div className="min-h-screen flex items-center justify-center bg-gradient-to-br from-gray-900 via-gray-800 to-gray-900 p-4">
      {/* Background decoration */}
      <div className="absolute inset-0 overflow-hidden pointer-events-none">
        <div className="absolute -top-40 -right-40 w-80 h-80 bg-primary-500/10 rounded-full blur-3xl" />
        <div className="absolute -bottom-40 -left-40 w-80 h-80 bg-blue-500/10 rounded-full blur-3xl" />
      </div>

      <div className="relative w-full max-w-md">
        {/* Logo */}
        <div className="text-center mb-8">
          <div className="inline-flex items-center gap-2 mb-2">
            <CrawfishIcon className="w-12 h-12 text-red-400" />
            <h1 className="text-4xl font-bold text-white">StarClaw</h1>
          </div>
          <p className="text-gray-400">
            {step === 'init' ? '初始化你的小龙虾' : '准备就绪'}
          </p>
        </div>

        <div className="bg-white/[0.03] backdrop-blur-xl border border-white/10 rounded-2xl shadow-2xl p-8">
          {step === 'init' ? (
            <>
              {/* Info badge */}
              <div className="flex items-start gap-3 bg-white/5 border border-white/10 rounded-xl p-4 mb-6">
                <Shield className="w-5 h-5 text-primary-400 mt-0.5 shrink-0" />
                <div>
                  <p className="text-sm text-gray-200 font-medium">单用户模式</p>
                  <p className="text-xs text-gray-400 mt-0.5">
                    初始化后生成永久 Owner Token，自动保存到浏览器。
                  </p>
                </div>
              </div>

              {/* Username (optional) */}
              <div className="mb-4">
                <label className="block text-sm font-medium text-gray-300 mb-1.5">
                  用户名 <span className="text-gray-500 font-normal">可选</span>
                </label>
                <input
                  type="text"
                  value={username}
                  onChange={(e) => setUsername(e.target.value)}
                  className="w-full px-4 py-2.5 bg-white/5 border border-white/10 rounded-lg text-white placeholder-gray-500 focus:ring-2 focus:ring-primary-500/50 focus:border-primary-500/50 outline-none transition-all"
                  placeholder="留空自动生成 Claw#XXXX"
                />
              </div>

              {/* Password (optional) */}
              <div className="mb-6">
                <label className="block text-sm font-medium text-gray-300 mb-1.5">
                  密码 <span className="text-gray-500 font-normal">可选</span>
                </label>
                <div className="relative">
                  <input
                    type={showPwd ? 'text' : 'password'}
                    value={password}
                    onChange={(e) => setPassword(e.target.value)}
                    className="w-full px-4 py-2.5 pr-10 bg-white/5 border border-white/10 rounded-lg text-white placeholder-gray-500 focus:ring-2 focus:ring-primary-500/50 focus:border-primary-500/50 outline-none transition-all"
                    placeholder="对外暴露时建议设置"
                    minLength={6}
                  />
                  <button
                    type="button"
                    onClick={() => setShowPwd(!showPwd)}
                    className="absolute right-3 top-1/2 -translate-y-1/2 text-gray-500 hover:text-gray-300 transition-colors"
                  >
                    {showPwd ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
                  </button>
                </div>
                <p className="text-xs text-gray-500 mt-1.5">
                  设了密码后，丢失 Token 可用密码找回
                </p>
              </div>

              {error && (
                <div className="bg-red-500/10 border border-red-500/20 text-red-400 text-sm rounded-lg p-3 mb-4">
                  {error}
                </div>
              )}

              <button
                onClick={handleSetup}
                disabled={loading}
                className="w-full py-3 bg-gradient-to-r from-primary-500 to-primary-600 text-white font-medium rounded-lg hover:from-primary-400 hover:to-primary-500 transition-all disabled:opacity-50 disabled:cursor-not-allowed shadow-lg shadow-primary-500/20 flex items-center justify-center gap-2"
              >
                {loading ? (
                  <div className="w-5 h-5 border-2 border-white/30 border-t-white rounded-full animate-spin" />
                ) : (
                  <>
                    <Sparkles className="w-4 h-4" />
                    开始使用
                  </>
                )}
              </button>
            </>
          ) : (
            <>
              {/* Success header */}
              <div className="text-center mb-6">
                <div className="inline-flex items-center justify-center w-14 h-14 bg-green-500/10 border border-green-500/20 rounded-full mb-3">
                  <Check className="w-7 h-7 text-green-400" />
                </div>
                <h2 className="text-xl font-semibold text-white">初始化完成</h2>
                <p className="text-sm text-gray-400 mt-1">
                  Token 已自动保存，建议额外备份
                </p>
              </div>

              {/* Token display */}
              <div className="mb-4">
                <label className="block text-xs font-medium text-gray-400 mb-2 uppercase tracking-wider">
                  Owner Token
                </label>
                <div className="bg-black/30 border border-white/10 rounded-lg p-3 font-mono text-xs text-primary-300 break-all select-all leading-relaxed">
                  {ownerToken}
                </div>
              </div>

              <button
                onClick={handleCopy}
                className="w-full mb-4 py-2.5 bg-white/5 border border-white/10 rounded-lg text-sm font-medium text-gray-300 hover:bg-white/10 hover:text-white transition-all flex items-center justify-center gap-2"
              >
                {copied ? (
                  <><Check className="w-4 h-4 text-green-400" /> 已复制</>
                ) : (
                  <><Copy className="w-4 h-4" /> 复制 Token</>
                )}
              </button>

              {/* Warning */}
              <div className="flex items-start gap-2.5 bg-amber-500/5 border border-amber-500/15 rounded-lg p-3 mb-6">
                <Shield className="w-4 h-4 text-amber-400 mt-0.5 shrink-0" />
                <p className="text-xs text-amber-300/80 leading-relaxed">
                  这是你访问此 Claw 的唯一凭证。
                  {password ? ' 已设密码，丢失 Token 可用密码找回。' : ' 未设密码，丢失 Token 需通过 CLI 重置。'}
                </p>
              </div>

              <button
                onClick={handleContinue}
                className="w-full py-3 bg-gradient-to-r from-primary-500 to-primary-600 text-white font-medium rounded-lg hover:from-primary-400 hover:to-primary-500 transition-all shadow-lg shadow-primary-500/20 flex items-center justify-center gap-2"
              >
                进入 StarClaw
                <ArrowRight className="w-4 h-4" />
              </button>
            </>
          )}
        </div>

        <p className="text-center text-xs text-gray-600 mt-6">
          StarClaw — 你的私人 AI 助手
        </p>
      </div>
    </div>
  )
}
