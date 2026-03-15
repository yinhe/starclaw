import { useState, useEffect } from 'react'
import { Cpu, Zap, Power, PowerOff, Wifi, WifiOff, Loader2, RefreshCw, ChevronRight, Layers } from 'lucide-react'
import { systemAPI } from '../lib/api'

interface MiningData {
  enabled: boolean
  connected: boolean
  is_contributing?: boolean
  models?: string[]
  gpu_info?: string
  balance?: number
  balance_energy?: number
  hp_status?: string
}

export default function MiningPage() {
  const [data, setData] = useState<MiningData | null>(null)
  const [loading, setLoading] = useState(true)
  const [toggling, setToggling] = useState(false)

  const fetchStatus = async () => {
    try {
      const res = await systemAPI.getMining()
      setData(res.data)
    } catch {
      setData(null)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => { fetchStatus() }, [])

  // Auto-refresh every 30s
  useEffect(() => {
    const iv = setInterval(fetchStatus, 30000)
    return () => clearInterval(iv)
  }, [])

  const handleToggle = async () => {
    if (!data) return
    setToggling(true)
    try {
      await systemAPI.toggleMining(!data.enabled)
      await fetchStatus()
    } catch { /* ignore */ }
    setToggling(false)
  }

  if (loading) {
    return (
      <div className="flex items-center justify-center py-32">
        <Loader2 className="w-6 h-6 text-indigo-500 animate-spin" />
      </div>
    )
  }

  const isActive = data?.is_contributing && data?.connected

  return (
    <div className="max-w-3xl mx-auto px-4 py-8 space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <div className={`w-10 h-10 rounded-xl flex items-center justify-center ${isActive ? 'bg-gradient-to-br from-indigo-500 to-purple-600 shadow-lg shadow-indigo-500/30' : 'bg-gray-100 dark:bg-gray-800'}`}>
            <Cpu className={`w-5 h-5 ${isActive ? 'text-white' : 'text-gray-400'}`} />
          </div>
          <div>
            <h1 className="text-xl font-bold text-gray-900 dark:text-white">算力共享</h1>
            <p className="text-sm text-gray-500">贡献 GPU 算力，赚取星能 ⚡</p>
          </div>
        </div>
        <button onClick={fetchStatus} className="p-2 rounded-lg hover:bg-gray-100 dark:hover:bg-gray-800 text-gray-400 transition">
          <RefreshCw className="w-4 h-4" />
        </button>
      </div>

      {/* Status Card */}
      <div className={`rounded-2xl border p-6 ${isActive ? 'bg-gradient-to-br from-indigo-50 to-purple-50 dark:from-indigo-950/30 dark:to-purple-950/30 border-indigo-200 dark:border-indigo-800/40' : 'bg-white dark:bg-gray-900 border-gray-200 dark:border-gray-800'}`}>
        <div className="flex items-center justify-between mb-5">
          <div className="flex items-center gap-3">
            {isActive ? (
              <div className="relative">
                <div className="w-3 h-3 rounded-full bg-green-400 animate-pulse" />
                <div className="absolute inset-0 w-3 h-3 rounded-full bg-green-400 animate-ping opacity-50" />
              </div>
            ) : (
              <div className="w-3 h-3 rounded-full bg-gray-300 dark:bg-gray-600" />
            )}
            <span className={`text-sm font-semibold ${isActive ? 'text-green-600 dark:text-green-400' : 'text-gray-500'}`}>
              {isActive ? '正在贡献算力' : data?.enabled ? '已启用（等待连接）' : '未启用'}
            </span>
          </div>

          {/* Toggle */}
          <button
            onClick={handleToggle}
            disabled={toggling}
            className={`flex items-center gap-2 px-4 py-2 rounded-xl text-sm font-medium transition-all cursor-pointer ${
              data?.enabled
                ? 'bg-red-50 dark:bg-red-950/30 text-red-600 dark:text-red-400 border border-red-200 dark:border-red-800/40 hover:bg-red-100 dark:hover:bg-red-950/50'
                : 'bg-indigo-600 text-white hover:bg-indigo-700 shadow-lg shadow-indigo-500/25'
            }`}
          >
            {toggling ? (
              <Loader2 className="w-4 h-4 animate-spin" />
            ) : data?.enabled ? (
              <PowerOff className="w-4 h-4" />
            ) : (
              <Power className="w-4 h-4" />
            )}
            {data?.enabled ? '关闭' : '开启算力共享'}
          </button>
        </div>

        {/* Stats Grid */}
        <div className="grid grid-cols-3 gap-4">
          <StatCard
            icon={<Wifi className="w-4 h-4" />}
            label="连接状态"
            value={data?.connected ? '已连接' : '未连接'}
            color={data?.connected ? 'text-green-600 dark:text-green-400' : 'text-gray-400'}
          />
          <StatCard
            icon={<Zap className="w-4 h-4" />}
            label="星能余额"
            value={data?.balance_energy != null ? `${data.balance_energy.toFixed(1)} ⚡` : '—'}
            color="text-amber-600 dark:text-amber-400"
          />
          <StatCard
            icon={<Layers className="w-4 h-4" />}
            label="贡献模型数"
            value={data?.models?.length?.toString() || '0'}
            color="text-indigo-600 dark:text-indigo-400"
          />
        </div>
      </div>

      {/* GPU Info */}
      {data?.gpu_info && (
        <div className="bg-white dark:bg-gray-900 border border-gray-200 dark:border-gray-800 rounded-xl p-5">
          <h3 className="text-sm font-semibold text-gray-700 dark:text-gray-200 mb-2 flex items-center gap-2">
            <Cpu className="w-4 h-4 text-gray-400" /> GPU 信息
          </h3>
          <p className="text-sm text-gray-500 font-mono">{data.gpu_info}</p>
        </div>
      )}

      {/* Contributing Models */}
      {data?.models && data.models.length > 0 && (
        <div className="bg-white dark:bg-gray-900 border border-gray-200 dark:border-gray-800 rounded-xl p-5">
          <h3 className="text-sm font-semibold text-gray-700 dark:text-gray-200 mb-3 flex items-center gap-2">
            <Layers className="w-4 h-4 text-indigo-400" /> 贡献中的模型
          </h3>
          <div className="space-y-2">
            {data.models.map((model) => (
              <div key={model} className="flex items-center gap-2 px-3 py-2 bg-gray-50 dark:bg-gray-800 rounded-lg">
                <ChevronRight className="w-3.5 h-3.5 text-indigo-400 flex-none" />
                <span className="text-sm text-gray-700 dark:text-gray-300 font-mono">{model}</span>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Explainer */}
      <div className="bg-white dark:bg-gray-900 border border-gray-200 dark:border-gray-800 rounded-xl p-5">
        <h3 className="text-sm font-semibold text-gray-700 dark:text-gray-200 mb-3 flex items-center gap-2">
          <Zap className="w-4 h-4 text-amber-500" fill="currentColor" /> 算力共享计划
        </h3>
        <div className="text-xs text-gray-500 dark:text-gray-400 space-y-2 leading-relaxed">
          <p>开启后，你的本地 GPU/模型将被虫群网络中其他节点用于推理请求。每次成功推理都会获得星能 ⚡ 奖励。</p>
          <div className="grid grid-cols-2 gap-3 mt-3">
            {[
              { title: '保底奖励', desc: '在线即得 0.1⚡/10分钟' },
              { title: '推理奖励', desc: '按 token 计费，90% 归你' },
              { title: 'GPU 加成', desc: '高端 GPU 获得更高奖励' },
              { title: '信任提升', desc: '持续稳定贡献提升信任等级' },
            ].map(item => (
              <div key={item.title} className="p-2.5 bg-amber-50 dark:bg-amber-950/20 rounded-lg border border-amber-100 dark:border-amber-900/30">
                <p className="font-medium text-amber-800 dark:text-amber-300 text-[11px]">{item.title}</p>
                <p className="text-[10px] text-amber-600/70 dark:text-amber-400/60 mt-0.5">{item.desc}</p>
              </div>
            ))}
          </div>
        </div>
      </div>

      {/* Not Connected Warning */}
      {data?.enabled && !data?.connected && (
        <div className="flex items-start gap-3 p-4 bg-yellow-50 dark:bg-yellow-950/20 border border-yellow-200 dark:border-yellow-800/30 rounded-xl">
          <WifiOff className="w-5 h-5 text-yellow-500 flex-none mt-0.5" />
          <div>
            <p className="text-sm font-medium text-yellow-800 dark:text-yellow-300">未连接虫群</p>
            <p className="text-xs text-yellow-600 dark:text-yellow-400/70 mt-1">
              算力共享需要连接到虫群网络。请在「设置 → 虫群」中加入虫群，或确保你的 Ollama 有可用模型。
            </p>
          </div>
        </div>
      )}
    </div>
  )
}

function StatCard({ icon, label, value, color }: { icon: React.ReactNode; label: string; value: string; color: string }) {
  return (
    <div className="text-center p-3 bg-white/60 dark:bg-gray-800/40 rounded-xl">
      <div className="flex items-center justify-center text-gray-400 mb-1.5">{icon}</div>
      <p className={`text-lg font-bold ${color}`}>{value}</p>
      <p className="text-[10px] text-gray-400 mt-0.5">{label}</p>
    </div>
  )
}
