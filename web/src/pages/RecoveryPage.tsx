import { useState, useEffect } from 'react'
import { Shield, Key, Phone, Cloud, Copy, Check, Eye, EyeOff, AlertTriangle, ChevronRight, Loader2 } from 'lucide-react'
import { recoveryAPI } from '../lib/api'

interface RecoveryStatus {
  mnemonic_saved: boolean
  phone_bound: boolean
  phone?: string
  backup_exists: boolean
  backup_time?: string
}

interface AddressInfo {
  node_id: string
  fingerprint: string
  public_key: string
  hot_address: string
  hd_path: string
}

export default function RecoveryPage() {
  const [status, setStatus] = useState<RecoveryStatus | null>(null)
  const [address, setAddress] = useState<AddressInfo | null>(null)
  const [activeStep, setActiveStep] = useState<'mnemonic' | 'phone' | 'backup' | null>(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    loadData()
  }, [])

  async function loadData() {
    try {
      const [statusRes, addrRes] = await Promise.all([
        recoveryAPI.getStatus(),
        recoveryAPI.getAddress(),
      ])
      setStatus(statusRes.data.status)
      setAddress(addrRes.data)
    } catch (e) {
      console.error('Failed to load recovery data', e)
    } finally {
      setLoading(false)
    }
  }

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <Loader2 className="w-6 h-6 animate-spin text-gray-400" />
      </div>
    )
  }

  const steps = [
    { key: 'mnemonic' as const, label: '保存助记词', icon: Key, done: status?.mnemonic_saved, desc: '24 个英文单词，可恢复您的 Claw 身份' },
    { key: 'phone' as const, label: '绑定手机', icon: Phone, done: status?.phone_bound, desc: status?.phone || '恢复时通过短信验证身份' },
    { key: 'backup' as const, label: '云端备份', icon: Cloud, done: status?.backup_exists, desc: status?.backup_time ? `最近备份: ${status.backup_time}` : '加密备份 Agent 和配置到云端' },
  ]

  const completedCount = steps.filter(s => s.done).length

  return (
    <div className="max-w-2xl mx-auto p-6 space-y-6">
      {/* Header */}
      <div>
        <h1 className="text-2xl font-bold flex items-center gap-2">
          <Shield className="w-6 h-6 text-primary-600" />
          身份恢复
        </h1>
        <p className="text-sm text-gray-500 dark:text-gray-400 mt-1">
          保护您的 Claw 地址，在新设备上恢复所有数据
        </p>
      </div>

      {/* Address Card */}
      {address && (
        <div className="bg-gray-50 dark:bg-gray-800/50 rounded-xl p-4 border border-gray-200 dark:border-gray-700">
          <div className="text-xs text-gray-500 mb-1">Claw 地址</div>
          <div className="font-mono text-sm break-all">{address.node_id}</div>
          <div className="text-xs text-gray-400 mt-2">指纹: {address.fingerprint}</div>
        </div>
      )}

      {/* Progress */}
      <div className="flex items-center gap-2">
        <div className="flex-1 bg-gray-200 dark:bg-gray-700 rounded-full h-2">
          <div
            className="bg-primary-600 h-2 rounded-full transition-all"
            style={{ width: `${(completedCount / 3) * 100}%` }}
          />
        </div>
        <span className="text-sm text-gray-500">{completedCount}/3</span>
      </div>

      {/* Steps */}
      <div className="space-y-3">
        {steps.map(step => (
          <button
            key={step.key}
            onClick={() => setActiveStep(activeStep === step.key ? null : step.key)}
            className={`w-full text-left p-4 rounded-xl border transition-all ${
              step.done
                ? 'border-green-200 dark:border-green-800 bg-green-50 dark:bg-green-900/20'
                : 'border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800 hover:border-primary-300'
            }`}
          >
            <div className="flex items-center gap-3">
              <div className={`w-10 h-10 rounded-full flex items-center justify-center ${
                step.done ? 'bg-green-100 dark:bg-green-800' : 'bg-gray-100 dark:bg-gray-700'
              }`}>
                {step.done ? (
                  <Check className="w-5 h-5 text-green-600" />
                ) : (
                  <step.icon className="w-5 h-5 text-gray-500" />
                )}
              </div>
              <div className="flex-1">
                <div className="font-medium text-sm">{step.label}</div>
                <div className="text-xs text-gray-500">{step.desc}</div>
              </div>
              <ChevronRight className={`w-4 h-4 text-gray-400 transition-transform ${
                activeStep === step.key ? 'rotate-90' : ''
              }`} />
            </div>
          </button>
        ))}
      </div>

      {/* Step Content */}
      {activeStep === 'mnemonic' && <MnemonicStep onDone={loadData} />}
      {activeStep === 'phone' && <PhoneStep onDone={loadData} />}
      {activeStep === 'backup' && <BackupStep onDone={loadData} />}

      {/* Warning */}
      <div className="bg-amber-50 dark:bg-amber-900/20 border border-amber-200 dark:border-amber-800 rounded-xl p-4 flex gap-3">
        <AlertTriangle className="w-5 h-5 text-amber-600 flex-shrink-0 mt-0.5" />
        <div className="text-sm text-amber-800 dark:text-amber-200">
          <div className="font-medium">安全提示</div>
          <ul className="mt-1 space-y-1 text-xs">
            <li>• 助记词是恢复身份的唯一凭证，请抄写在纸上妥善保管</li>
            <li>• 切勿截图、拍照或以任何数字方式存储助记词</li>
            <li>• 任何持有助记词的人都可以完全控制您的 Claw</li>
          </ul>
        </div>
      </div>
    </div>
  )
}

// ── Step 1: Mnemonic ──

function MnemonicStep({ onDone }: { onDone: () => void }) {
  const [words, setWords] = useState<string[]>([])
  const [visible, setVisible] = useState(false)
  const [copied, setCopied] = useState(false)
  const [confirmInput, setConfirmInput] = useState('')
  const [confirming, setConfirming] = useState(false)
  const [showConfirm, setShowConfirm] = useState(false)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  useEffect(() => {
    recoveryAPI.getMnemonic().then(res => {
      setWords(res.data.words || [])
      setLoading(false)
    }).catch(() => setLoading(false))
  }, [])

  async function handleCopy() {
    await navigator.clipboard.writeText(words.join(' '))
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  async function handleConfirm() {
    setConfirming(true)
    setError('')
    try {
      await recoveryAPI.confirmMnemonic(confirmInput.trim())
      onDone()
    } catch (e: any) {
      setError(e.response?.data?.error || '验证失败')
    } finally {
      setConfirming(false)
    }
  }

  if (loading) return <div className="text-center py-4"><Loader2 className="w-5 h-5 animate-spin mx-auto" /></div>

  return (
    <div className="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-5 space-y-4">
      <div className="flex items-center justify-between">
        <h3 className="font-medium">您的 24 位助记词</h3>
        <div className="flex gap-2">
          <button onClick={() => setVisible(!visible)} className="text-xs flex items-center gap-1 text-gray-500 hover:text-gray-700">
            {visible ? <EyeOff className="w-3.5 h-3.5" /> : <Eye className="w-3.5 h-3.5" />}
            {visible ? '隐藏' : '显示'}
          </button>
          <button onClick={handleCopy} className="text-xs flex items-center gap-1 text-gray-500 hover:text-gray-700">
            {copied ? <Check className="w-3.5 h-3.5 text-green-500" /> : <Copy className="w-3.5 h-3.5" />}
            {copied ? '已复制' : '复制'}
          </button>
        </div>
      </div>

      <div className="grid grid-cols-4 gap-2">
        {words.map((word, i) => (
          <div key={i} className="bg-gray-50 dark:bg-gray-700/50 rounded-lg px-3 py-2 text-center">
            <span className="text-xs text-gray-400 mr-1">{i + 1}.</span>
            <span className={`text-sm font-mono ${visible ? '' : 'blur-sm select-none'}`}>{word}</span>
          </div>
        ))}
      </div>

      {!showConfirm ? (
        <button
          onClick={() => setShowConfirm(true)}
          className="w-full py-2 bg-primary-600 text-white rounded-lg text-sm font-medium hover:bg-primary-700 transition-colors"
        >
          我已抄写完毕，确认保存
        </button>
      ) : (
        <div className="space-y-3">
          <p className="text-xs text-gray-500">请输入完整的 24 个助记词以确认您已正确记录：</p>
          <textarea
            value={confirmInput}
            onChange={e => setConfirmInput(e.target.value)}
            placeholder="输入 24 个单词，用空格分隔..."
            className="w-full p-3 border rounded-lg text-sm font-mono bg-gray-50 dark:bg-gray-700 dark:border-gray-600 resize-none"
            rows={3}
          />
          {error && <p className="text-xs text-red-500">{error}</p>}
          <button
            onClick={handleConfirm}
            disabled={confirming || confirmInput.trim().split(/\s+/).length !== 24}
            className="w-full py-2 bg-primary-600 text-white rounded-lg text-sm font-medium hover:bg-primary-700 transition-colors disabled:opacity-50"
          >
            {confirming ? '验证中...' : '确认'}
          </button>
        </div>
      )}
    </div>
  )
}

// ── Step 2: Phone Binding ──

function PhoneStep({ onDone }: { onDone: () => void }) {
  const [phone, setPhone] = useState('')
  const [code, setCode] = useState('')
  const [codeSent, setCodeSent] = useState(false)
  const [countdown, setCountdown] = useState(0)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => {
    if (countdown > 0) {
      const t = setTimeout(() => setCountdown(countdown - 1), 1000)
      return () => clearTimeout(t)
    }
  }, [countdown])

  async function sendCode() {
    setLoading(true)
    setError('')
    try {
      await recoveryAPI.bindPhone(phone)
      setCodeSent(true)
      setCountdown(60)
    } catch (e: any) {
      setError(e.response?.data?.error || '发送失败')
    } finally {
      setLoading(false)
    }
  }

  async function verify() {
    setLoading(true)
    setError('')
    try {
      await recoveryAPI.verifyPhone(phone, code)
      onDone()
    } catch (e: any) {
      setError(e.response?.data?.error || '验证失败')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-5 space-y-4">
      <h3 className="font-medium">绑定手机号</h3>
      <p className="text-xs text-gray-500">恢复身份时将通过短信验证码进行二次验证</p>

      <div className="flex gap-2">
        <input
          type="tel"
          value={phone}
          onChange={e => setPhone(e.target.value)}
          placeholder="输入手机号"
          className="flex-1 px-3 py-2 border rounded-lg text-sm bg-gray-50 dark:bg-gray-700 dark:border-gray-600"
        />
        <button
          onClick={sendCode}
          disabled={loading || phone.length < 11 || countdown > 0}
          className="px-4 py-2 bg-primary-600 text-white rounded-lg text-sm font-medium hover:bg-primary-700 disabled:opacity-50 whitespace-nowrap"
        >
          {countdown > 0 ? `${countdown}s` : '发送验证码'}
        </button>
      </div>

      {codeSent && (
        <div className="flex gap-2">
          <input
            type="text"
            value={code}
            onChange={e => setCode(e.target.value)}
            placeholder="输入 6 位验证码"
            maxLength={6}
            className="flex-1 px-3 py-2 border rounded-lg text-sm bg-gray-50 dark:bg-gray-700 dark:border-gray-600 font-mono tracking-widest"
          />
          <button
            onClick={verify}
            disabled={loading || code.length !== 6}
            className="px-4 py-2 bg-primary-600 text-white rounded-lg text-sm font-medium hover:bg-primary-700 disabled:opacity-50"
          >
            {loading ? '验证中...' : '绑定'}
          </button>
        </div>
      )}

      {error && <p className="text-xs text-red-500">{error}</p>}
    </div>
  )
}

// ── Step 3: Cloud Backup ──

function BackupStep({ onDone }: { onDone: () => void }) {
  const [loading, setLoading] = useState(false)
  const [result, setResult] = useState<{ agents: number; backup_size: number } | null>(null)
  const [error, setError] = useState('')

  async function handleBackup() {
    setLoading(true)
    setError('')
    try {
      const res = await recoveryAPI.backup()
      setResult(res.data)
      onDone()
    } catch (e: any) {
      setError(e.response?.data?.error || '备份失败')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-5 space-y-4">
      <h3 className="font-medium">云端加密备份</h3>
      <p className="text-xs text-gray-500">
        将您的 Agent、配置等数据加密后上传到 Queen 服务器。
        数据使用助记词派生的密钥加密，Queen 无法解密您的数据。
      </p>

      {result ? (
        <div className="bg-green-50 dark:bg-green-900/20 rounded-lg p-3 text-sm text-green-800 dark:text-green-200">
          <Check className="w-4 h-4 inline mr-1" />
          备份成功！共 {result.agents} 个 Agent，大小 {(result.backup_size / 1024).toFixed(1)} KB
        </div>
      ) : (
        <button
          onClick={handleBackup}
          disabled={loading}
          className="w-full py-2 bg-primary-600 text-white rounded-lg text-sm font-medium hover:bg-primary-700 transition-colors disabled:opacity-50 flex items-center justify-center gap-2"
        >
          {loading ? (
            <>
              <Loader2 className="w-4 h-4 animate-spin" />
              备份中...
            </>
          ) : (
            <>
              <Cloud className="w-4 h-4" />
              立即备份
            </>
          )}
        </button>
      )}

      {error && <p className="text-xs text-red-500">{error}</p>}
    </div>
  )
}
