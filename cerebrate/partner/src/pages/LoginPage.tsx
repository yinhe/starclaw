import { useState, useRef } from 'react'
import { useNavigate } from 'react-router-dom'
import { clawAuth, clawNodeRequest, setToken } from '../lib/api'

type Step = 'input' | 'connecting' | 'waiting' | 'verifying' | 'done' | 'error'

export default function LoginPage() {
  const navigate = useNavigate()
  const [clawUrl, setClawUrl] = useState('')
  const [step, setStep] = useState<Step>('input')
  const [msg, setMsg] = useState<{ text: string; error: boolean } | null>(null)
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null)

  async function handleClawLogin() {
    setStep('connecting')
    setMsg(null)
    const baseUrl = new URL(clawUrl.includes('://') ? clawUrl : `https://${clawUrl}`).origin
    try {
      await clawNodeRequest<{ node_id: string }>(baseUrl, '/v1/identity/info')
      const { challenge } = await clawAuth.challenge()
      const reqRes = await clawNodeRequest<{ id: string }>(
        baseUrl, '/v1/identity/auth-request',
        { method: 'POST', body: JSON.stringify({ challenge, origin: window.location.hostname }) }
      )
      setStep('waiting')
      setMsg({ text: '请在 Claw 界面确认授权', error: false })

      await new Promise<void>((resolve, reject) => {
        let attempts = 0
        pollRef.current = setInterval(async () => {
          attempts++
          if (attempts > 100) { clearInterval(pollRef.current!); reject(new Error('授权超时')); return }
          try {
            const s = await clawNodeRequest<{
              status: string; node_id?: string; public_key?: string; signature?: string; challenge?: string
            }>(baseUrl, `/v1/identity/auth-request/${reqRes.id}`)
            if (s.status === 'approved') {
              clearInterval(pollRef.current!)
              setStep('verifying')
              const data = await clawAuth.verify({
                challenge: s.challenge!, node_id: s.node_id!,
                public_key: s.public_key!, signature: s.signature!,
              })
              if (data.user.role !== 'partner' && data.user.role !== 'admin') {
                reject(new Error('此 Claw 地址不是团队合伙人，请联系管理员添加白名单'))
                return
              }
              setToken(data.token)
              setStep('done')
              setMsg({ text: '认证成功', error: false })
              setTimeout(() => navigate('/dashboard'), 800)
              resolve()
            } else if (s.status === 'rejected') {
              clearInterval(pollRef.current!); reject(new Error('授权被拒绝'))
            }
          } catch (e: any) {
            if (e.message?.includes('expired') || e.message?.includes('not found')) {
              clearInterval(pollRef.current!); reject(new Error('请求已过期'))
            }
          }
        }, 3000)
      })
    } catch (e: any) {
      if (pollRef.current) clearInterval(pollRef.current)
      setStep('error')
      setMsg({ text: e.message || '连接失败', error: true })
    }
  }

  const INPUT = 'w-full rounded-lg border border-white/10 bg-white/[0.02] px-4 py-2.5 text-white text-sm focus:outline-none focus:border-claw-500'

  return (
    <div className="min-h-screen flex items-center justify-center px-4">
      <div className="w-full max-w-sm">
        <div className="text-center mb-8">
          <h1 className="text-2xl font-bold">
            <span className="text-claw-500">Star</span>Claw
          </h1>
          <p className="text-sm text-gray-400 mt-1">团队合伙人门户</p>
        </div>

        <div className="rounded-xl border border-white/10 bg-white/[0.02] p-6 space-y-4">
          {step === 'input' && (
            <>
              <p className="text-sm text-gray-400 text-center">使用你的 Claw 节点地址登录</p>
              <div>
                <label className="block text-sm text-gray-400 mb-1.5">Claw 节点地址</label>
                <input type="url" value={clawUrl} onChange={e => setClawUrl(e.target.value)}
                  className={INPUT} placeholder="请输入你的 Claw 节点地址"
                  onKeyDown={e => e.key === 'Enter' && handleClawLogin()} />
              </div>
              {msg && <p className={`text-sm ${msg.error ? 'text-red-400' : 'text-green-400'}`}>{msg.text}</p>}
              <button type="button" onClick={handleClawLogin}
                className="w-full rounded-lg bg-claw-600 py-2.5 text-sm font-semibold text-white hover:bg-claw-500 transition-colors">
                发送认证请求
              </button>
            </>
          )}

          {step === 'connecting' && (
            <p className="text-sm text-gray-400 text-center py-4">连接 Claw 节点...</p>
          )}

          {step === 'waiting' && (
            <div className="text-center py-4">
              <p className="text-sm text-claw-400 font-medium">{msg?.text}</p>
              <p className="text-xs text-gray-500 mt-2">在 Claw 界面点击「授权登录」</p>
            </div>
          )}

          {step === 'verifying' && (
            <p className="text-sm text-gray-400 text-center py-4">验证身份中...</p>
          )}

          {step === 'done' && (
            <p className="text-sm text-green-400 text-center py-4">{msg?.text}</p>
          )}

          {step === 'error' && (
            <div className="space-y-3">
              <p className="text-sm text-red-400 text-center">{msg?.text}</p>
              <button type="button" onClick={() => { setStep('input'); setMsg(null) }}
                className="w-full rounded-lg border border-white/10 py-2.5 text-sm text-gray-400 hover:bg-white/5 transition-colors">
                重试
              </button>
            </div>
          )}
        </div>

        <p className="text-center text-xs text-gray-600 mt-4">
          Claw 地址需由管理员添加白名单后方可登录
        </p>
      </div>
    </div>
  )
}
