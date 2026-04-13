import { useState, useEffect } from 'react'
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
  const [loginMode, setLoginMode] = useState<'token' | 'owner'>('owner')
  const [password, setPassword] = useState('')
  const [apiToken, setApiToken] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)
  const [showPwd, setShowPwd] = useState(false)
  const [pendingApproval, setPendingApproval] = useState(false)
  const [pendingMessage, setPendingMessage] = useState('')
  const [searchParams] = useSearchParams()
  const navigate = useNavigate()
  const setAuth = useAuthStore((s) => s.setAuth)

  // Auto-login via ?token= param (from Spore setup completion link)
  useEffect(() => {
    const tokenParam = searchParams.get('token')
    if (tokenParam) {
      setLoading(true)
      authAPI.tokenLogin({ token: tokenParam, device_id: getDeviceID(), device_name: getDeviceName() })
        .then(res => {
          if (res.data?.token) {
            setAuth(res.data.owner_token || res.data.token, res.data.user)
            navigate('/', { replace: true })
          }
        })
        .catch(() => {
          // Token might be owner_token format — store directly and try
          setAuth(tokenParam, { id: 0, username: 'Owner', role: 'owner' } as any)
          navigate('/', { replace: true })
        })
        .finally(() => setLoading(false))
      return
    }
  }, [searchParams, setAuth, navigate])

  // Detect deploy mode and set appropriate login mode
  // Both opensource and hosted modes use the same Setup → Owner Token flow.
  // hosted mode only differs in blocking localhost-only endpoints (reset-token, get-token).
  useEffect(() => {
    setupAPI.status().then(async (res) => {
      const mode = res.data.deploy_mode || 'opensource'
      setDeployMode(mode)
      // No owner yet → redirect to setup page (all modes)
      if (!res.data.setup_completed) {
        navigate('/setup', { replace: true })
        return
      }
      setLoginMode('owner')
      // Auto-login from localhost (opensource/spore only, not hosted/hive)
      if (mode === 'opensource') {
        const host = window.location.hostname
        if (host === 'localhost' || host === '127.0.0.1' || host === '::1') {
          try {
            const tokenRes = await setupAPI.getToken()
            if (tokenRes.data?.owner_token) {
              setAuth(tokenRes.data.owner_token, { id: 0, username: tokenRes.data.username || 'Owner', role: 'owner' } as any)
              navigate('/', { replace: true })
              return
            }
          } catch {
            // Not localhost or token not available — show normal login
          }
        }
      }
    }).catch(() => setDeployMode('opensource'))
  }, [navigate, setAuth])

  // Poll for device approval when pending
  useEffect(() => {
    if (!pendingApproval || !apiToken) return
    const interval = setInterval(async () => {
      try {
        const res = await authAPI.tokenLogin({ token: apiToken, device_id: getDeviceID(), device_name: getDeviceName() })
        if (res.status === 200 && res.data?.token) {
          clearInterval(interval)
          setPendingApproval(false)
          setAuth(res.data.token, res.data.user)
          navigate('/chat')
        }
      } catch {
        // Still pending or error — keep polling
      }
    }, 3000)
    return () => clearInterval(interval)
  }, [pendingApproval, apiToken, setAuth, navigate])

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    setLoading(true)

    try {
      if (loginMode === 'owner') {
        // Owner password login — returns owner_token + auto-approves device
        const res = await setupAPI.ownerLogin({ password, device_id: getDeviceID(), device_name: getDeviceName() })
        setAuth(res.data.owner_token, res.data.user)
        navigate('/', { replace: true })
        return
      }
      // Token login
      const res = await authAPI.tokenLogin({ token: apiToken, device_id: getDeviceID(), device_name: getDeviceName() })
      // Handle pending approval (HTTP 202)
      if (res.status === 202 && res.data?.status === 'pending_approval') {
        setPendingApproval(true)
        setPendingMessage(res.data.approve_cmd || res.data.message || '新设备等待审批中...')
        setLoading(false)
        return
      }
      setAuth(res.data.token, res.data.user)
      navigate('/', { replace: true })
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
          {pendingApproval ? (
            <div className="text-center py-8">
              <div className="w-12 h-12 border-3 border-amber-500 border-t-transparent rounded-full animate-spin mx-auto mb-4" />
              <h2 className="text-xl font-semibold text-gray-800 mb-2">等待设备审批</h2>
              <p className="text-gray-500 text-sm mb-4">{pendingMessage}</p>
              <div className="bg-gray-50 rounded-lg p-3 text-left text-xs text-gray-600 space-y-1">
                <p className="font-medium text-gray-700">审批方式：</p>
                <p>1. 在已登录设备的 <b>设置 → 设备管理</b> 中审批</p>
                <p>2. 在服务器项目目录执行：</p>
                <code className="block bg-gray-200 px-2 py-1 rounded mt-1 select-all">{pendingMessage}</code>
              </div>
              <button
                type="button"
                onClick={() => { setPendingApproval(false); setPendingMessage(''); setError('') }}
                className="mt-4 text-sm text-gray-500 hover:text-gray-700 underline"
              >
                返回登录
              </button>
            </div>
          ) : (
          <>
          <h2 className="text-2xl font-semibold mb-6">登录</h2>

          <form onSubmit={handleSubmit} className="space-y-4">
            {/* Loading state */}
            {deployMode === null ? (
              <div className="flex justify-center py-8">
                <div className="w-6 h-6 border-2 border-primary-600 border-t-transparent rounded-full animate-spin" />
              </div>
            ) : (
              <>
                {deployMode === 'opensource' && (window.location.hostname === 'localhost' || window.location.hostname === '127.0.0.1') && (
                  <button
                    type="button"
                    onClick={async () => {
                      setError('')
                      setLoading(true)
                      try {
                        const res = await setupAPI.getToken()
                        if (res.data?.owner_token) {
                          setAuth(res.data.owner_token, { id: 0, username: res.data.username || 'Owner', role: 'owner' } as any)
                          navigate('/', { replace: true })
                          return
                        }
                        setError('未找到 Token，请先完成初始化')
                      } catch {
                        setError('自动登录失败，请使用密码或 Token 登录')
                      } finally {
                        setLoading(false)
                      }
                    }}
                    disabled={loading}
                    className="w-full mb-4 py-3 bg-gradient-to-r from-green-500 to-emerald-600 text-white font-medium rounded-lg hover:from-green-400 hover:to-emerald-500 disabled:opacity-50 disabled:cursor-not-allowed transition-all shadow-md flex items-center justify-center gap-2"
                  >
                    {loading ? (
                      <div className="w-5 h-5 border-2 border-white/30 border-t-white rounded-full animate-spin" />
                    ) : (
                      <>🖥️ 本机自动登录</>
                    )}
                  </button>
                )}
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
                    {error && (
                      <div className="p-3 bg-red-50 border border-red-200 rounded-lg text-red-600 text-sm">
                        {error}
                      </div>
                    )}
                    <button
                      type="submit"
                      disabled={loading}
                      className="w-full py-2.5 bg-primary-600 text-white rounded-lg font-medium hover:bg-primary-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
                    >
                      {loading ? '验证中...' : '登录'}
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
                    {error && (
                      <div className="p-3 bg-red-50 border border-red-200 rounded-lg text-red-600 text-sm">
                        {error}
                      </div>
                    )}
                    <button
                      type="submit"
                      disabled={loading}
                      className="w-full py-2.5 bg-primary-600 text-white rounded-lg font-medium hover:bg-primary-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
                    >
                      {loading ? '验证中...' : '登录'}
                    </button>
                    <div className="bg-blue-50 border border-blue-200 rounded-lg p-3">
                      <p className="text-xs text-blue-800">
                        Token 丢失？用初始化时设置的密码找回。
                        <br />
                        未设密码请通过 CLI 查看：<code className="bg-blue-100 px-1 rounded">spore token</code>
                      </p>
                    </div>
                  </>
                )}
              </>
            )}
          </form>
          </>
          )}
        </div>
      </div>
    </div>
  )
}
