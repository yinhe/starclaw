import { useState, useEffect, useCallback } from 'react'
import { Zap, Globe, Shield, Clock, CheckCircle, AlertCircle, Loader2, ArrowRight, Github, Server, Cpu, Database } from 'lucide-react'

const DOMAIN = 'starclaw.me'
const SLUG_RE = /^[a-z][a-z0-9-]{1,28}[a-z0-9]$/

type Status = 'idle' | 'checking' | 'available' | 'taken' | 'invalid' | 'creating' | 'done' | 'error'

interface CreateResult {
  id: string
  slug: string
  url: string
  status: string
  message: string
}

interface Plan {
  id: string
  display_name: string
  deploy_mode: string
  price_daily: number
  price_monthly: number
  cpu: number
  memory_mb: number
  storage_gb: number
  bandwidth_mb: number
  custom_domain: boolean
  ssl_included: boolean
  backup_daily: boolean
  expire_days: number
}

function formatEnergy(amount: number): string {
  if (amount === 0) return '免费'
  const fen = amount / 10000
  if (fen >= 100) return `¥${(fen / 100).toFixed(0)}/月`
  return `¥${(fen / 100).toFixed(2)}/月`
}

function SlugInput({ slug, setSlug, status }: { slug: string; setSlug: (s: string) => void; status: Status }) {
  const border = {
    idle: 'border-zinc-700',
    checking: 'border-yellow-500',
    available: 'border-emerald-500',
    taken: 'border-red-500',
    invalid: 'border-red-500',
    creating: 'border-purple-500',
    done: 'border-emerald-500',
    error: 'border-red-500',
  }[status]

  return (
    <div className={`flex items-center rounded-xl border-2 ${border} bg-zinc-900/80 backdrop-blur transition-colors overflow-hidden`}>
      <span className="text-zinc-500 pl-4 pr-1 text-lg select-none whitespace-nowrap">https://</span>
      <input
        type="text"
        value={slug}
        onChange={e => setSlug(e.target.value.toLowerCase().replace(/[^a-z0-9-]/g, ''))}
        placeholder="my-claw"
        maxLength={30}
        className="bg-transparent text-white text-lg font-mono py-3.5 outline-none w-36 sm:w-48 placeholder:text-zinc-600"
        disabled={status === 'creating' || status === 'done'}
      />
      <span className="text-zinc-500 pr-4 text-lg select-none whitespace-nowrap">.{DOMAIN}</span>
    </div>
  )
}

function StatusBadge({ status }: { status: Status }) {
  if (status === 'idle') return null
  const cfg: Record<string, { icon: React.ReactNode; text: string; color: string }> = {
    checking: { icon: <Loader2 className="w-4 h-4 animate-spin" />, text: '检查中...', color: 'text-yellow-400' },
    available: { icon: <CheckCircle className="w-4 h-4" />, text: '可用', color: 'text-emerald-400' },
    taken: { icon: <AlertCircle className="w-4 h-4" />, text: '已被占用', color: 'text-red-400' },
    invalid: { icon: <AlertCircle className="w-4 h-4" />, text: '格式无效 (3-30位小写字母/数字/连字符)', color: 'text-red-400' },
    creating: { icon: <Loader2 className="w-4 h-4 animate-spin" />, text: '正在创建...', color: 'text-purple-400' },
    done: { icon: <CheckCircle className="w-4 h-4" />, text: '创建成功!', color: 'text-emerald-400' },
    error: { icon: <AlertCircle className="w-4 h-4" />, text: '创建失败', color: 'text-red-400' },
  }
  const c = cfg[status]
  if (!c) return null
  return (
    <div className={`flex items-center gap-1.5 text-sm ${c.color}`}>
      {c.icon} {c.text}
    </div>
  )
}

function App() {
  const [slug, setSlug] = useState('')
  const [email, setEmail] = useState('')
  const [clawId, setClawId] = useState('')
  const [status, setStatus] = useState<Status>('idle')
  const [result, setResult] = useState<CreateResult | null>(null)
  const [errorMsg, setErrorMsg] = useState('')
  const [plans, setPlans] = useState<Plan[]>([])
  const [selectedPlan, setSelectedPlan] = useState('free')

  // Fetch plans on mount
  useEffect(() => {
    fetch('/hive/plans').then(r => r.json()).then(d => {
      if (d.plans) setPlans(d.plans)
    }).catch(() => {})
  }, [])

  // Debounced slug availability check
  useEffect(() => {
    if (!slug) { setStatus('idle'); return }
    if (!SLUG_RE.test(slug)) { setStatus('invalid'); return }

    setStatus('checking')
    const timer = setTimeout(async () => {
      try {
        const res = await fetch(`/hive/claws/${slug}`)
        setStatus(res.status === 404 ? 'available' : 'taken')
      } catch {
        setStatus('available') // assume available if API unreachable
      }
    }, 400)
    return () => clearTimeout(timer)
  }, [slug])

  const handleCreate = useCallback(async () => {
    if (status !== 'available') return
    setStatus('creating')
    setErrorMsg('')
    try {
      const res = await fetch('/hive/claws', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          slug,
          display_name: slug,
          owner_email: email,
          plan_id: selectedPlan,
          claw_id: clawId || undefined,
        }),
      })
      const data = await res.json()
      if (!res.ok) {
        setErrorMsg(data.error || '创建失败')
        setStatus('error')
        return
      }
      setResult(data)
      setStatus('done')
    } catch (e) {
      setErrorMsg('网络错误，请重试')
      setStatus('error')
    }
  }, [slug, email, status])

  return (
    <div className="min-h-screen bg-zinc-950 text-white">
      {/* Hero */}
      <header className="relative overflow-hidden">
        <div className="absolute inset-0 bg-gradient-to-br from-purple-900/30 via-zinc-950 to-emerald-900/20" />
        <div className="absolute top-0 left-1/2 -translate-x-1/2 w-[800px] h-[800px] rounded-full bg-purple-600/10 blur-3xl" />

        <nav className="relative z-10 flex items-center justify-between px-6 py-5 max-w-6xl mx-auto">
          <div className="flex items-center gap-2">
            <Zap className="w-7 h-7 text-purple-400" />
            <span className="text-xl font-bold tracking-tight">StarClaw</span>
          </div>
          <div className="flex items-center gap-4">
            <a href="https://github.com/yinheai/starclaw" target="_blank" className="text-zinc-400 hover:text-white transition-colors">
              <Github className="w-5 h-5" />
            </a>
            <a href="https://docs.starclaw.me" target="_blank" className="text-zinc-400 hover:text-white text-sm transition-colors">
              文档
            </a>
          </div>
        </nav>

        <div className="relative z-10 max-w-3xl mx-auto px-6 pt-16 pb-24 text-center">
          <div className="inline-flex items-center gap-2 px-3 py-1 rounded-full bg-purple-500/10 border border-purple-500/20 text-purple-300 text-sm mb-8">
            <Zap className="w-3.5 h-3.5" /> 10 秒创建你的 AI 智能体平台
          </div>

          <h1 className="text-4xl sm:text-5xl lg:text-6xl font-bold tracking-tight mb-6 bg-gradient-to-r from-white via-purple-200 to-emerald-200 bg-clip-text text-transparent">
            一键创建 Claw
          </h1>
          <p className="text-lg text-zinc-400 mb-12 max-w-xl mx-auto">
            输入你想要的子域名，立即获得一个完整的 AI 智能体管理平台。
            <br className="hidden sm:block" />
            免费体验，无需配置。
          </p>

          {/* Create Form */}
          <div className="flex flex-col items-center gap-4">
            <SlugInput slug={slug} setSlug={setSlug} status={status} />
            <StatusBadge status={status} />

            {status !== 'done' && (
              <div className="flex flex-col items-center gap-3 w-full max-w-md">
                {selectedPlan !== 'free' && (
                  <div className="flex items-center gap-2 text-sm text-purple-300 bg-purple-500/10 rounded-lg px-3 py-2 w-full">
                    <Zap className="w-4 h-4" />
                    已选择 <span className="font-medium">{plans.find(p => p.id === selectedPlan)?.display_name || selectedPlan}</span> 套餐
                    <button onClick={() => setSelectedPlan('free')} className="ml-auto text-zinc-400 hover:text-white text-xs cursor-pointer">切换</button>
                  </div>
                )}
                <div className="flex flex-col sm:flex-row items-center gap-3 w-full">
                  <input
                    type="email"
                    value={email}
                    onChange={e => setEmail(e.target.value)}
                    placeholder="邮箱（可选，用于找回）"
                    className="bg-zinc-900/80 border border-zinc-700 rounded-lg px-4 py-3 text-white text-sm w-full outline-none focus:border-purple-500 transition-colors placeholder:text-zinc-600"
                    disabled={status === 'creating'}
                  />
                  <button
                    onClick={handleCreate}
                    disabled={status !== 'available'}
                    className="flex items-center gap-2 px-6 py-3 rounded-lg bg-purple-600 hover:bg-purple-500 disabled:bg-zinc-800 disabled:text-zinc-600 text-white font-medium transition-all whitespace-nowrap cursor-pointer disabled:cursor-not-allowed"
                  >
                    {status === 'creating' ? (
                      <><Loader2 className="w-4 h-4 animate-spin" /> 创建中...</>
                    ) : (
                      <>创建 <ArrowRight className="w-4 h-4" /></>
                    )}
                  </button>
                </div>
                {selectedPlan !== 'free' && (
                  <input
                    type="text"
                    value={clawId}
                    onChange={e => setClawId(e.target.value)}
                    placeholder="StarAI 账户 ID（claw:xxx，付费套餐必填）"
                    className="bg-zinc-900/80 border border-zinc-700 rounded-lg px-4 py-3 text-white text-sm w-full outline-none focus:border-purple-500 transition-colors placeholder:text-zinc-600 font-mono"
                    disabled={status === 'creating'}
                  />
                )}
              </div>
            )}

            {errorMsg && <p className="text-red-400 text-sm">{errorMsg}</p>}

            {/* Success Card */}
            {status === 'done' && result && (
              <div className="mt-4 p-6 rounded-2xl bg-emerald-500/10 border border-emerald-500/30 max-w-md w-full text-left">
                <div className="flex items-center gap-2 text-emerald-400 font-medium mb-3">
                  <CheckCircle className="w-5 h-5" /> Claw 创建成功!
                </div>
                <p className="text-zinc-300 text-sm mb-4">{result.message}</p>
                <a
                  href={result.url}
                  target="_blank"
                  className="inline-flex items-center gap-2 px-5 py-2.5 rounded-lg bg-emerald-600 hover:bg-emerald-500 text-white font-medium transition-colors"
                >
                  <Globe className="w-4 h-4" /> 访问 {result.url.replace('https://', '')}
                </a>
              </div>
            )}
          </div>
        </div>
      </header>

      {/* Features */}
      <section className="max-w-5xl mx-auto px-6 py-20">
        <h2 className="text-2xl font-bold text-center mb-12">开箱即用的 AI 能力</h2>
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-6">
          {[
            { icon: <Cpu className="w-6 h-6 text-purple-400" />, title: '多模型支持', desc: '接入 OpenAI、通义千问、DeepSeek 等主流大模型' },
            { icon: <Server className="w-6 h-6 text-emerald-400" />, title: 'MCP 工具集成', desc: '连接外部工具服务器，扩展智能体能力边界' },
            { icon: <Shield className="w-6 h-6 text-blue-400" />, title: '端到端加密', desc: 'Ed25519 身份密钥 + AES-256 数据加密' },
            { icon: <Database className="w-6 h-6 text-orange-400" />, title: '独立数据库', desc: '每个 Claw 独享 MySQL 数据库，数据完全隔离' },
          ].map((f, i) => (
            <div key={i} className="p-5 rounded-xl bg-zinc-900/50 border border-zinc-800 hover:border-zinc-700 transition-colors">
              <div className="mb-3">{f.icon}</div>
              <h3 className="font-semibold text-white mb-1.5">{f.title}</h3>
              <p className="text-sm text-zinc-400">{f.desc}</p>
            </div>
          ))}
        </div>
      </section>

      {/* How it works */}
      <section className="max-w-3xl mx-auto px-6 pb-20">
        <h2 className="text-2xl font-bold text-center mb-12">三步开始</h2>
        <div className="flex flex-col sm:flex-row gap-8">
          {[
            { step: '1', title: '选择子域名', desc: '输入你喜欢的名称，系统自动检测可用性', icon: <Globe className="w-5 h-5" /> },
            { step: '2', title: '一键创建', desc: '10 秒自动部署完整的 Claw 实例', icon: <Zap className="w-5 h-5" /> },
            { step: '3', title: '开始使用', desc: '访问你的专属域名，配置 AI 模型，创建智能体', icon: <ArrowRight className="w-5 h-5" /> },
          ].map((s, i) => (
            <div key={i} className="flex-1 text-center">
              <div className="w-10 h-10 rounded-full bg-purple-600/20 border border-purple-500/30 flex items-center justify-center mx-auto mb-3 text-purple-400">
                {s.icon}
              </div>
              <h3 className="font-semibold text-white mb-1">{s.title}</h3>
              <p className="text-sm text-zinc-400">{s.desc}</p>
            </div>
          ))}
        </div>
      </section>

      {/* Pricing Plans */}
      <section className="border-t border-zinc-800" id="pricing">
        <div className="max-w-5xl mx-auto px-6 py-16">
          <h2 className="text-2xl font-bold text-center mb-3">选择套餐</h2>
          <p className="text-center text-zinc-400 text-sm mb-10">免费体验或按月付费，随时升级</p>
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-5">
            {(plans.length > 0 ? plans : [
              { id: 'free', display_name: '免费体验', deploy_mode: 'hive', price_monthly: 0, cpu: 0.5, memory_mb: 512, storage_gb: 2, bandwidth_mb: 0, backup_daily: false, custom_domain: false, expire_days: 7 } as Plan,
            ]).map(p => {
              const selected = selectedPlan === p.id
              const isPaid = p.price_monthly > 0
              const highlight = p.id === 'pro'
              return (
                <button
                  key={p.id}
                  onClick={() => setSelectedPlan(p.id)}
                  className={`relative p-5 rounded-xl border-2 text-left transition-all cursor-pointer ${
                    selected
                      ? 'border-purple-500 bg-purple-500/10'
                      : highlight
                        ? 'border-purple-500/30 bg-zinc-900/80'
                        : 'border-zinc-800 bg-zinc-900/50 hover:border-zinc-700'
                  }`}
                >
                  {highlight && (
                    <span className="absolute -top-2.5 left-4 px-2 py-0.5 rounded-full bg-purple-600 text-white text-xs font-medium">
                      推荐
                    </span>
                  )}
                  <div className="text-white font-semibold mb-1">{p.display_name}</div>
                  <div className="text-2xl font-bold text-white mb-3">{formatEnergy(p.price_monthly)}</div>
                  <div className="space-y-1.5 text-sm text-zinc-400">
                    <div>{p.cpu} 核 CPU</div>
                    <div>{p.memory_mb >= 1024 ? `${p.memory_mb / 1024} GB` : `${p.memory_mb} MB`} 内存</div>
                    <div>{p.storage_gb} GB 存储</div>
                    {p.bandwidth_mb > 0 && <div>{p.bandwidth_mb} Mbps 带宽</div>}
                    <div className="text-zinc-500 text-xs pt-1">
                      {p.deploy_mode === 'ecs' ? '独立云服务器' : '共享容器'}
                      {p.backup_daily && ' · 每日备份'}
                      {p.custom_domain && ' · 自定义域名'}
                    </div>
                    {p.expire_days > 0 && (
                      <div className="flex items-center gap-1 text-yellow-500/80 text-xs">
                        <Clock className="w-3 h-3" /> {p.expire_days} 天有效期
                      </div>
                    )}
                    {isPaid && !p.expire_days && (
                      <div className="text-emerald-500/80 text-xs">无限期 · 按月续费</div>
                    )}
                  </div>
                  {selected && (
                    <div className="absolute top-4 right-4">
                      <CheckCircle className="w-5 h-5 text-purple-400" />
                    </div>
                  )}
                </button>
              )
            })}
          </div>
        </div>
      </section>

      {/* Footer */}
      <footer className="border-t border-zinc-800 py-8">
        <div className="max-w-5xl mx-auto px-6 flex flex-col sm:flex-row items-center justify-between gap-4 text-sm text-zinc-500">
          <span>&copy; {new Date().getFullYear()} StarClaw &mdash; AI Agent Platform</span>
          <div className="flex items-center gap-4">
            <a href="https://github.com/yinheai/starclaw" target="_blank" className="hover:text-white transition-colors">GitHub</a>
            <a href="https://docs.starclaw.me" target="_blank" className="hover:text-white transition-colors">文档</a>
          </div>
        </div>
      </footer>
    </div>
  )
}

export default App
