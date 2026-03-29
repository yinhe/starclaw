import { useState, useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { Navbar } from '../components/Navbar'
import { Footer } from '../components/Footer'
import { cloudAPI, type HivePlan, type ClawInstance } from '../lib/api'
import { isLoggedIn } from '../lib/auth'

const modeLabel: Record<string, string> = { lite: '🔥 Spark', hive: '⚡ Pulse', ecs: '🚀 Surge' }
const modeColor: Record<string, string> = {
  lite: 'border-amber-500/50 bg-amber-900/10',
  hive: 'border-blue-500/50 bg-blue-900/10',
  ecs: 'border-purple-500/50 bg-purple-900/10',
}
const statusColor: Record<string, string> = {
  running: 'bg-emerald-500', creating: 'bg-amber-500 animate-pulse', stopped: 'bg-gray-500',
  error: 'bg-red-500', destroying: 'bg-red-500 animate-pulse', destroyed: 'bg-gray-700',
}
const statusLabel: Record<string, string> = {
  running: '运行中', creating: '创建中', stopped: '已停止',
  error: '异常', destroying: '销毁中', destroyed: '已销毁',
}

export function CloudPage() {
  const [tab, setTab] = useState<'plans' | 'instances'>('plans')
  const [plans, setPlans] = useState<HivePlan[]>([])
  const [instances, setInstances] = useState<ClawInstance[]>([])
  const [loading, setLoading] = useState(true)
  const [creating, setCreating] = useState(false)
  const [slug, setSlug] = useState('')
  const [displayName, setDisplayName] = useState('')
  const [selectedPlan, setSelectedPlan] = useState('')
  const [clawId, setClawId] = useState('')
  const [result, setResult] = useState<{ url?: string; message?: string; error?: string } | null>(null)

  useEffect(() => {
    Promise.all([
      cloudAPI.plans().catch(() => ({ plans: [] })),
      cloudAPI.list().catch(() => ({ instances: [] })),
    ]).then(([p, i]) => {
      setPlans(p.plans || [])
      setInstances(i.instances || [])
      setLoading(false)
    })
  }, [])

  const refreshInstances = async () => {
    try {
      const res = await cloudAPI.list()
      setInstances(res.instances || [])
    } catch {}
  }

  const handleCreate = async () => {
    if (!slug.trim()) return
    setCreating(true)
    setResult(null)
    try {
      const res = await cloudAPI.create({
        slug: slug.trim().toLowerCase(),
        display_name: displayName.trim() || undefined,
        plan_id: selectedPlan || undefined,
        claw_id: clawId.trim() || undefined,
      })
      setResult({ url: res.url, message: res.message })
      setSlug('')
      setDisplayName('')
      setTab('instances')
      setTimeout(refreshInstances, 3000)
    } catch (e: any) {
      setResult({ error: e.message })
    }
    setCreating(false)
  }

  const handleAction = async (instSlug: string, action: 'stop' | 'start' | 'restart' | 'destroy') => {
    if (action === 'destroy' && !confirm(`确认销毁实例 ${instSlug}？此操作不可撤销。`)) return
    try {
      if (action === 'destroy') await cloudAPI.destroy(instSlug)
      else if (action === 'stop') await cloudAPI.stop(instSlug)
      else if (action === 'start') await cloudAPI.start(instSlug)
      else await cloudAPI.restart(instSlug)
      setTimeout(refreshInstances, 1500)
    } catch (e: any) {
      alert(e.message)
    }
  }

  const navigate = useNavigate()

  useEffect(() => {
    if (!isLoggedIn()) navigate('/auth')
  }, [navigate])

  if (loading) return <><Navbar /><div className="min-h-screen bg-gray-950 text-center py-20 text-gray-400">加载中...</div><Footer /></>

  return (
    <>
      <Navbar />
      <div className="min-h-screen bg-gray-950">
      <div className="max-w-5xl mx-auto px-4 py-8">
        <div className="flex items-center justify-between mb-6">
          <div>
            <h1 className="text-2xl font-bold text-white">🐝 云船队</h1>
            <p className="text-sm text-gray-400 mt-1">一键创建你的 AI 智能体节点</p>
          </div>
          <div className="flex gap-1 bg-gray-900 rounded-lg p-1 border border-gray-700">
            <button onClick={() => setTab('plans')}
              className={`px-4 py-1.5 rounded text-sm font-medium transition ${tab === 'plans' ? 'bg-gray-700 text-white' : 'text-gray-400 hover:text-white'}`}>
              套餐
            </button>
            <button onClick={() => setTab('instances')}
              className={`px-4 py-1.5 rounded text-sm font-medium transition ${tab === 'instances' ? 'bg-gray-700 text-white' : 'text-gray-400 hover:text-white'}`}>
              我的实例 ({instances.length})
            </button>
          </div>
        </div>

        {result && (
          <div className={`rounded-lg p-4 mb-6 border ${result.error ? 'border-red-600/50 bg-red-900/20 text-red-300' : 'border-emerald-600/50 bg-emerald-900/20 text-emerald-300'}`}>
            {result.error ? `❌ ${result.error}` : (
              <div>
                <div className="font-medium">✅ {result.message}</div>
                {result.url && <a href={result.url} target="_blank" rel="noreferrer" className="text-sm text-blue-400 hover:underline mt-1 block">{result.url}</a>}
              </div>
            )}
          </div>
        )}

        {tab === 'plans' && (
          <div className="space-y-6">
            {/* Plan cards */}
            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
              {plans.filter(p => p.is_active).map(plan => (
                <div key={plan.id}
                  onClick={() => setSelectedPlan(plan.id)}
                  className={`rounded-xl p-5 border-2 cursor-pointer transition-all ${
                    selectedPlan === plan.id ? 'border-blue-500 ring-2 ring-blue-500/30' : modeColor[plan.deploy_mode] || 'border-gray-700'
                  } hover:border-blue-400/50`}>
                  <div className="flex items-center justify-between mb-3">
                    <span className="text-lg font-bold text-white">{plan.display_name}</span>
                    <span className="text-xs text-gray-500">{modeLabel[plan.deploy_mode] || plan.deploy_mode}</span>
                  </div>
                  <div className="text-2xl font-bold mb-3">
                    {plan.price_monthly > 0 ? (
                      <span className="text-blue-400">⚡{plan.price_monthly}<span className="text-xs text-gray-500 font-normal">/月</span></span>
                    ) : (
                      <span className="text-emerald-400">免费</span>
                    )}
                  </div>
                  <div className="space-y-1.5 text-xs text-gray-400">
                    <div>CPU: {plan.cpu} 核</div>
                    <div>内存: {plan.memory_mb}MB</div>
                    <div>存储: {plan.storage_gb}GB</div>
                    {plan.bandwidth_mb > 0 && <div>带宽: {plan.bandwidth_mb}Mbps</div>}
                    <div>团队智能体: {plan.max_teams > 0 ? `${plan.max_teams} 个` : '无限'}</div>
                    {plan.expire_days > 0 && <div className="text-amber-400">有效期: {plan.expire_days} 天</div>}
                  </div>
                  {selectedPlan === plan.id && (
                    <div className="mt-3 text-center text-xs text-blue-400 font-medium">✓ 已选择</div>
                  )}
                </div>
              ))}
            </div>

            {/* Create form */}
            <div className="bg-gray-900/80 rounded-xl p-6 border border-gray-700/50">
              <h3 className="text-lg font-bold text-white mb-4">创建 Claw 实例</h3>
              <div className="grid grid-cols-1 md:grid-cols-2 gap-4 mb-4">
                <div>
                  <label className="text-xs text-gray-400 block mb-1">子域名 *</label>
                  <div className="flex items-center gap-0">
                    <input value={slug} onChange={e => setSlug(e.target.value)}
                      placeholder="my-claw" maxLength={30}
                      className="flex-1 bg-gray-800 border border-gray-600 rounded-l-lg px-3 py-2 text-white text-sm focus:border-blue-500 focus:outline-none" />
                    <span className="bg-gray-700 border border-gray-600 border-l-0 rounded-r-lg px-3 py-2 text-xs text-gray-400">.starclaw.me</span>
                  </div>
                </div>
                <div>
                  <label className="text-xs text-gray-400 block mb-1">显示名称</label>
                  <input value={displayName} onChange={e => setDisplayName(e.target.value)}
                    placeholder="我的小龙虾"
                    className="w-full bg-gray-800 border border-gray-600 rounded-lg px-3 py-2 text-white text-sm focus:border-blue-500 focus:outline-none" />
                </div>
              </div>
              {selectedPlan && plans.find(p => p.id === selectedPlan)?.price_monthly! > 0 && (
                <div className="mb-4">
                  <label className="text-xs text-gray-400 block mb-1">Claw 地址（用于扣费）</label>
                  <input value={clawId} onChange={e => setClawId(e.target.value)}
                    placeholder="claw_xxxx..."
                    className="w-full bg-gray-800 border border-gray-600 rounded-lg px-3 py-2 text-white text-sm focus:border-blue-500 focus:outline-none" />
                </div>
              )}
              <button onClick={handleCreate} disabled={creating || !slug.trim()}
                className="bg-blue-600 hover:bg-blue-500 disabled:opacity-50 disabled:cursor-not-allowed text-white px-6 py-2.5 rounded-lg font-medium transition">
                {creating ? '创建中...' : '🚀 创建实例'}
              </button>
              <p className="text-[10px] text-gray-500 mt-2">
                {selectedPlan ? `选择套餐: ${plans.find(p => p.id === selectedPlan)?.display_name}` : '默认使用免费 Spark 套餐'}
              </p>
            </div>
          </div>
        )}

        {tab === 'instances' && (
          <div className="space-y-3">
            {instances.length === 0 ? (
              <div className="text-center py-16 text-gray-500">
                <div className="text-4xl mb-3">🐝</div>
                <div className="text-lg mb-2">还没有实例</div>
                <button onClick={() => setTab('plans')} className="text-blue-400 hover:underline text-sm">去创建一个</button>
              </div>
            ) : instances.map(inst => (
              <div key={inst.id} className="bg-gray-900/80 rounded-xl p-5 border border-gray-700/50">
                <div className="flex items-start justify-between">
                  <div className="flex items-center gap-3">
                    <div className={`w-2.5 h-2.5 rounded-full ${statusColor[inst.status] || 'bg-gray-500'}`} />
                    <div>
                      <div className="flex items-center gap-2">
                        <span className="text-white font-medium">{inst.display_name || inst.slug}</span>
                        <span className="text-xs text-gray-500">{modeLabel[inst.deploy_mode] || inst.deploy_mode}</span>
                      </div>
                      <a href={`https://${inst.slug}.starclaw.me`} target="_blank" rel="noreferrer"
                        className="text-xs text-blue-400 hover:underline">{inst.slug}.starclaw.me</a>
                    </div>
                  </div>
                  <span className={`text-xs px-2 py-0.5 rounded ${
                    inst.status === 'running' ? 'bg-emerald-600/30 text-emerald-400' :
                    inst.status === 'error' ? 'bg-red-600/30 text-red-400' :
                    'bg-gray-600/30 text-gray-400'
                  }`}>{statusLabel[inst.status] || inst.status}</span>
                </div>

                <div className="flex items-center gap-4 mt-3 text-xs text-gray-500">
                  <span>CPU {inst.cpu_limit}核</span>
                  <span>内存 {Math.round(inst.memory_limit / 1024 / 1024)}MB</span>
                  {inst.expires_at && <span>到期 {new Date(inst.expires_at).toLocaleDateString()}</span>}
                  {inst.last_active_at && <span>活跃 {new Date(inst.last_active_at).toLocaleDateString()}</span>}
                </div>

                {inst.status !== 'destroyed' && inst.status !== 'destroying' && (
                  <div className="flex gap-2 mt-3">
                    {inst.status === 'running' && (
                      <>
                        <ActionBtn label="停止" onClick={() => handleAction(inst.slug, 'stop')} color="gray" />
                        <ActionBtn label="重启" onClick={() => handleAction(inst.slug, 'restart')} color="blue" />
                      </>
                    )}
                    {inst.status === 'stopped' && (
                      <ActionBtn label="启动" onClick={() => handleAction(inst.slug, 'start')} color="emerald" />
                    )}
                    <ActionBtn label="销毁" onClick={() => handleAction(inst.slug, 'destroy')} color="red" />
                  </div>
                )}
              </div>
            ))}
          </div>
        )}
      </div>
      </div>
      <Footer />
    </>
  )
}

function ActionBtn({ label, onClick, color }: { label: string; onClick: () => void; color: string }) {
  const colors: Record<string, string> = {
    gray: 'bg-gray-700 hover:bg-gray-600 text-gray-300',
    blue: 'bg-blue-700/50 hover:bg-blue-600/50 text-blue-300',
    emerald: 'bg-emerald-700/50 hover:bg-emerald-600/50 text-emerald-300',
    red: 'bg-red-700/50 hover:bg-red-600/50 text-red-300',
  }
  return (
    <button onClick={onClick} className={`text-xs px-3 py-1 rounded-lg transition ${colors[color] || colors.gray}`}>
      {label}
    </button>
  )
}
