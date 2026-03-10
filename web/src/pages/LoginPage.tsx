import { useState, useEffect, useCallback } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
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
import { authAPI, setupAPI } from '../lib/api'
import { useAuthStore } from '../stores/authStore'

interface OAuthProvider {
  name: string
  client_id: string
}

function getDeviceID(): string {
  let id = localStorage.getItem('starclaw_device_id')
  if (!id) {
    id = crypto.randomUUID()
    localStorage.setItem('starclaw_device_id', id)
  }
  return id
}

function getDeviceName(): string {
  const ua = navigator.userAgent
  if (ua.includes('Windows')) return 'Windows'
  if (ua.includes('Mac')) return 'macOS'
  if (ua.includes('Linux')) return 'Linux'
  if (ua.includes('iPhone') || ua.includes('iPad')) return 'iOS'
  if (ua.includes('Android')) return 'Android'
  return 'Unknown'
}

export default function LoginPage() {
  const [deployMode, setDeployMode] = useState<string | null>(null)
  const [isRegister, setIsRegister] = useState(false)
  const [loginMode, setLoginMode] = useState<'email' | 'phone' | 'token' | 'owner'>('email')
  const [email, setEmail] = useState('')
  const [phone, setPhone] = useState('')
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [apiToken, setApiToken] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)
  const [showPwd, setShowPwd] = useState(false)
  const [rememberMe, setRememberMe] = useState(true)
  const [oauthProviders, setOauthProviders] = useState<OAuthProvider[]>([])
  const [searchParams] = useSearchParams()
  const navigate = useNavigate()
  const setAuth = useAuthStore((s) => s.setAuth)

  // Detect deploy mode and set appropriate login mode
  useEffect(() => {
    setupAPI.status().then(res => {
      const mode = res.data.deploy_mode || 'opensource'
      setDeployMode(mode)
      if (mode === 'opensource') {
        setLoginMode('owner')
      }
    }).catch(() => setDeployMode('opensource'))
  }, [])

  // Fetch available OAuth providers (hosted mode only)
  useEffect(() => {
    if (deployMode === 'hosted') {
      authAPI.oauthProviders().then(res => {
        setOauthProviders(res.data.providers || [])
      }).catch(() => {})
    }
  }, [deployMode])

  // Handle OAuth callback (code in URL params)
  const handleOAuthCode = useCallback(async (provider: string, code: string) => {
    setLoading(true)
    setError('')
    try {
      const res = provider === 'github'
        ? await authAPI.oauthGitHub(code)
        : await authAPI.oauthGoogle(code)
      setAuth(res.data.token, res.data.user)
      navigate('/chat')
    } catch (err: unknown) {
      const axiosErr = err as { response?: { data?: { error?: string } } }
      setError(axiosErr.response?.data?.error || 'OAuth login failed')
    } finally {
      setLoading(false)
    }
  }, [setAuth, navigate])

  useEffect(() => {
    const code = searchParams.get('code')
    const state = searchParams.get('state') // 'github' or 'google'
    if (code && state) {
      handleOAuthCode(state, code)
    }
  }, [searchParams, handleOAuthCode])

  const startOAuth = (provider: OAuthProvider) => {
    const redirectUri = `${window.location.origin}/login?state=${provider.name}`
    if (provider.name === 'github') {
      window.location.href = `https://github.com/login/oauth/authorize?client_id=${provider.client_id}&redirect_uri=${encodeURIComponent(redirectUri)}&scope=read:user%20user:email&state=${provider.name}`
    } else if (provider.name === 'google') {
      window.location.href = `https://accounts.google.com/o/oauth2/v2/auth?client_id=${provider.client_id}&redirect_uri=${encodeURIComponent(redirectUri)}&response_type=code&scope=openid%20email%20profile&state=${provider.name}`
    }
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    setLoading(true)

    try {
      let res
      if (loginMode === 'owner') {
        // Owner password login (opensource mode) — returns owner_token
        res = await setupAPI.ownerLogin({ password })
        setAuth(res.data.owner_token, res.data.user)
        navigate('/', { replace: true })
        return
      } else if (loginMode === 'token') {
        res = await authAPI.tokenLogin({ token: apiToken, device_id: getDeviceID(), device_name: getDeviceName() })
      } else if (loginMode === 'phone') {
        res = isRegister
          ? await authAPI.phoneRegister({ phone, password, username: username || undefined })
          : await authAPI.phoneLogin({ phone, password })
      } else {
        res = isRegister
          ? await authAPI.register({ email, username: username || undefined, password })
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
            {deployMode === 'opensource' ? '登录' : isRegister ? '创建账号' : '登录'}
          </h2>

          {error && (
            <div className="mb-4 p-3 bg-red-50 border border-red-200 rounded-lg text-red-600 text-sm">
              {error}
            </div>
          )}

          <form onSubmit={handleSubmit} className="space-y-4">
            {/* Opensource mode: password or token login */}
            {deployMode === 'opensource' ? (
              <>
                <div className="flex rounded-lg bg-gray-100 p-1 mb-2">
                  <button
                    type="button"
                    onClick={() => { setLoginMode('owner'); setError('') }}
                    className={`flex-1 py-2 text-sm font-medium rounded-md transition-all ${loginMode === 'owner' ? 'bg-white text-primary-600 shadow-sm' : 'text-gray-500 hover:text-gray-700'}`}
                  >
                    密码
                  </button>
                  <button
                    type="button"
                    onClick={() => { setLoginMode('token'); setError('') }}
                    className={`flex-1 py-2 text-sm font-medium rounded-md transition-all ${loginMode === 'token' ? 'bg-white text-primary-600 shadow-sm' : 'text-gray-500 hover:text-gray-700'}`}
                  >
                    Auth Token
                  </button>
                </div>

                {loginMode === 'owner' ? (
                  <>
                    <div className="bg-blue-50 border border-blue-200 rounded-lg p-3 mb-2">
                      <p className="text-xs text-blue-800">
                        Token 丢失？用初始化时设置的密码找回。
                        <br />
                        未设密码请通过 CLI 重置：<code className="bg-blue-100 px-1 rounded">starclaw reset-token</code>
                      </p>
                    </div>
                    <div>
                      <label className="block text-sm font-medium text-gray-700 mb-1">密码</label>
                      <div className="relative">
                        <input
                          type={showPwd ? 'text' : 'password'}
                          value={password}
                          onChange={(e) => setPassword(e.target.value)}
                          className="w-full px-4 py-2.5 pr-10 border border-gray-300 rounded-lg focus:ring-2 focus:ring-primary-500 focus:border-transparent outline-none transition-all"
                          placeholder="输入你的密码"
                          required
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
                    <button
                      type="submit"
                      disabled={loading}
                      className="w-full py-2.5 bg-primary-600 text-white rounded-lg font-medium hover:bg-primary-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
                    >
                      {loading ? '验证中...' : '找回 Token'}
                    </button>
                  </>
                ) : (
                  <>
                    <div>
                      <label className="block text-sm font-medium text-gray-700 mb-1">Auth Token</label>
                      <input
                        type="password"
                        value={apiToken}
                        onChange={(e) => setApiToken(e.target.value)}
                        className="w-full px-4 py-2.5 border border-gray-300 rounded-lg focus:ring-2 focus:ring-primary-500 focus:border-transparent outline-none transition-all font-mono"
                        placeholder="粘贴你的 Auth Token"
                        required
                      />
                      <p className="mt-1.5 text-xs text-gray-400">初始化时获取的 Token，或在设置页面复制</p>
                    </div>
                    <button
                      type="submit"
                      disabled={loading}
                      className="w-full py-2.5 bg-primary-600 text-white rounded-lg font-medium hover:bg-primary-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
                    >
                      {loading ? '验证中...' : '登录'}
                    </button>
                  </>
                )}
              </>
            ) : (
            <>
            {/* Email / Phone / Token mode toggle */}
            {!isRegister && (
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
                <button
                  type="button"
                  onClick={() => { setLoginMode('token'); setError('') }}
                  className={`flex-1 py-2 text-sm font-medium rounded-md transition-all ${loginMode === 'token' ? 'bg-white text-primary-600 shadow-sm' : 'text-gray-500 hover:text-gray-700'}`}
                >
                  Token
                </button>
              </div>
            )}

            {isRegister && (
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
            )}

            {loginMode === 'token' ? (
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">Auth Token</label>
                <input
                  type="password"
                  value={apiToken}
                  onChange={(e) => setApiToken(e.target.value)}
                  className="w-full px-4 py-2.5 border border-gray-300 rounded-lg focus:ring-2 focus:ring-primary-500 focus:border-transparent outline-none transition-all font-mono"
                  placeholder="粘贴你的 Token"
                  required
                />
                <p className="mt-1.5 text-xs text-gray-400">在设置页面获取你的 Auth Token</p>
              </div>
            ) : (
              <>
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
                      用户名（可选）
                    </label>
                    <input
                      type="text"
                      value={username}
                      onChange={(e) => setUsername(e.target.value)}
                      className="w-full px-4 py-2.5 border border-gray-300 rounded-lg focus:ring-2 focus:ring-primary-500 focus:border-transparent outline-none transition-all"
                      placeholder="留空自动生成 Claw#xxxx"
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
              </>
            )}

            <button
              type="submit"
              disabled={loading}
              className="w-full py-2.5 bg-primary-600 text-white rounded-lg font-medium hover:bg-primary-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
            >
              {loading ? '处理中...' : isRegister ? '注册' : '登录'}
            </button>
            </>
            )}
          </form>

          {deployMode === 'hosted' && oauthProviders.length > 0 && (
            <div className="mt-6">
              <div className="relative mb-4">
                <div className="absolute inset-0 flex items-center"><div className="w-full border-t border-gray-200" /></div>
                <div className="relative flex justify-center text-xs"><span className="bg-white px-3 text-gray-400">or</span></div>
              </div>
              <div className="flex gap-3">
                {oauthProviders.map((p) => (
                  <button
                    key={p.name}
                    onClick={() => startOAuth(p)}
                    disabled={loading}
                    className="flex-1 flex items-center justify-center gap-2 py-2.5 border border-gray-300 rounded-lg text-sm font-medium text-gray-700 hover:bg-gray-50 disabled:opacity-50 transition-colors"
                  >
                    {p.name === 'github' && (
                      <svg className="w-5 h-5" viewBox="0 0 24 24" fill="currentColor"><path d="M12 0C5.37 0 0 5.37 0 12c0 5.31 3.435 9.795 8.205 11.385.6.105.825-.255.825-.57 0-.285-.015-1.23-.015-2.235-3.015.555-3.795-.735-4.035-1.41-.135-.345-.72-1.41-1.23-1.695-.42-.225-1.02-.78-.015-.795.945-.015 1.62.87 1.845 1.23 1.08 1.815 2.805 1.305 3.495.99.105-.78.42-1.305.765-1.605-2.67-.3-5.46-1.335-5.46-5.925 0-1.305.465-2.385 1.23-3.225-.12-.3-.54-1.53.12-3.18 0 0 1.005-.315 3.3 1.23.96-.27 1.98-.405 3-.405s2.04.135 3 .405c2.295-1.56 3.3-1.23 3.3-1.23.66 1.65.24 2.88.12 3.18.765.84 1.23 1.905 1.23 3.225 0 4.605-2.805 5.625-5.475 5.925.435.375.81 1.095.81 2.22 0 1.605-.015 2.895-.015 3.3 0 .315.225.69.825.57A12.02 12.02 0 0024 12c0-6.63-5.37-12-12-12z"/></svg>
                    )}
                    {p.name === 'google' && (
                      <svg className="w-5 h-5" viewBox="0 0 24 24"><path d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92a5.06 5.06 0 01-2.2 3.32v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.1z" fill="#4285F4"/><path d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z" fill="#34A853"/><path d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.07H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.93l2.85-2.22.81-.62z" fill="#FBBC05"/><path d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.07l3.66 2.84c.87-2.6 3.3-4.53 6.16-4.53z" fill="#EA4335"/></svg>
                    )}
                    {p.name === 'github' ? 'GitHub' : 'Google'}
                  </button>
                ))}
              </div>
            </div>
          )}

          {deployMode === 'hosted' && loginMode !== 'token' && (
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
          )}
        </div>
      </div>
    </div>
  )
}
