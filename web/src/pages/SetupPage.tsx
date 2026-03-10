import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { setupAPI } from '../lib/api'
import { useAuthStore } from '../stores/authStore'

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
    <div className="min-h-screen bg-gradient-to-br from-gray-50 to-blue-50 flex items-center justify-center p-4">
      <div className="w-full max-w-md">
        {/* Logo */}
        <div className="text-center mb-8">
          <div className="inline-flex items-center justify-center w-16 h-16 bg-gradient-to-br from-primary-500 to-primary-700 rounded-2xl mb-4 shadow-lg">
            <span className="text-3xl">🦞</span>
          </div>
          <h1 className="text-2xl font-bold text-gray-900">StarClaw</h1>
          <p className="text-gray-500 mt-1">
            {step === 'init' ? '初始化你的小龙虾' : '初始化完成'}
          </p>
        </div>

        <div className="bg-white rounded-2xl shadow-xl p-8">
          {step === 'init' ? (
            <>
              {/* Info banner */}
              <div className="bg-blue-50 border border-blue-200 rounded-lg p-4 mb-6">
                <p className="text-sm text-blue-800">
                  <strong>单用户模式</strong> — 每个 Claw 只有一个主人。
                  初始化后会生成一个永久 Token，保存在浏览器中自动登录。
                </p>
              </div>

              {/* Username (optional) */}
              <div className="mb-4">
                <label className="block text-sm font-medium text-gray-700 mb-1">
                  用户名（可选）
                </label>
                <input
                  type="text"
                  value={username}
                  onChange={(e) => setUsername(e.target.value)}
                  className="w-full px-4 py-2.5 border border-gray-300 rounded-lg focus:ring-2 focus:ring-primary-500 focus:border-transparent outline-none transition-all"
                  placeholder="留空自动生成 Claw#XXXX"
                />
              </div>

              {/* Password (optional) */}
              <div className="mb-6">
                <label className="block text-sm font-medium text-gray-700 mb-1">
                  密码（可选）
                </label>
                <div className="relative">
                  <input
                    type={showPwd ? 'text' : 'password'}
                    value={password}
                    onChange={(e) => setPassword(e.target.value)}
                    className="w-full px-4 py-2.5 border border-gray-300 rounded-lg focus:ring-2 focus:ring-primary-500 focus:border-transparent outline-none transition-all pr-12"
                    placeholder="对外暴露时建议设置"
                    minLength={6}
                  />
                  <button
                    type="button"
                    onClick={() => setShowPwd(!showPwd)}
                    className="absolute right-3 top-1/2 -translate-y-1/2 text-gray-400 hover:text-gray-600"
                  >
                    {showPwd ? '🙈' : '👁️'}
                  </button>
                </div>
                <p className="text-xs text-gray-400 mt-1">
                  设了密码后，丢失 Token 可用密码找回
                </p>
              </div>

              {error && (
                <div className="bg-red-50 text-red-600 text-sm rounded-lg p-3 mb-4">
                  {error}
                </div>
              )}

              <button
                onClick={handleSetup}
                disabled={loading}
                className="w-full py-3 bg-gradient-to-r from-primary-500 to-primary-600 text-white font-medium rounded-lg hover:from-primary-600 hover:to-primary-700 transition-all disabled:opacity-50 disabled:cursor-not-allowed shadow-md"
              >
                {loading ? '初始化中...' : '🦞 开始使用'}
              </button>
            </>
          ) : (
            <>
              {/* Success state */}
              <div className="bg-green-50 border border-green-200 rounded-lg p-4 mb-6">
                <p className="text-sm text-green-800 font-medium mb-1">✅ 初始化成功</p>
                <p className="text-xs text-green-700">
                  Token 已自动保存到浏览器。建议额外备份以下 Token：
                </p>
              </div>

              {/* Token display */}
              <div className="mb-6">
                <label className="block text-sm font-medium text-gray-700 mb-2">
                  Owner Token
                </label>
                <div className="bg-gray-50 border border-gray-200 rounded-lg p-3 font-mono text-sm break-all select-all">
                  {ownerToken}
                </div>
                <button
                  onClick={handleCopy}
                  className="mt-2 w-full py-2 border border-gray-300 rounded-lg text-sm font-medium text-gray-700 hover:bg-gray-50 transition-all"
                >
                  {copied ? '✅ 已复制' : '📋 复制 Token'}
                </button>
              </div>

              {/* Warning */}
              <div className="bg-amber-50 border border-amber-200 rounded-lg p-4 mb-6">
                <p className="text-xs text-amber-800">
                  <strong>⚠️ 注意：</strong>这是你访问 Claw 的唯一凭证。
                  {password ? ' 你设了密码，丢失 Token 可用密码找回。' : ' 你未设密码，丢失 Token 需通过 CLI 重置。'}
                </p>
              </div>

              <button
                onClick={handleContinue}
                className="w-full py-3 bg-gradient-to-r from-primary-500 to-primary-600 text-white font-medium rounded-lg hover:from-primary-600 hover:to-primary-700 transition-all shadow-md"
              >
                进入 StarClaw →
              </button>
            </>
          )}
        </div>

        <p className="text-center text-xs text-gray-400 mt-6">
          StarClaw — 你的私人 AI 助手
        </p>
      </div>
    </div>
  )
}
