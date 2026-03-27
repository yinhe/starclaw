import { useEffect, useState, useRef } from 'react'
import { api } from '../api'
import { Bug, Play, RefreshCw, Database, CheckCircle, XCircle, Loader2, Download, Zap } from 'lucide-react'

// Drone API base — same server as Queen (Server C), port 8110
const DRONE_API = '/drone'

interface HarvestRecord {
  id: string
  source: string
  status: string
  collected: number
  morphed: number
  imported: number
  duration: string
  error?: string
  started_at: string
}

interface DroneStats {
  total_runs: number
  completed: number
  failed: number
  running: string[]
  total_collected: number
  total_morphed: number
  total_imported: number
}

const SOURCES = [
  { id: 'gpt_prompts', name: 'GPT Prompts', desc: '完整 GPT system prompt (linexjlin/GPTs)', icon: '🧠', tier: 1 },
  { id: 'awesome_gpts', name: 'Awesome GPTs', desc: 'GitHub GPT 目录合集', icon: '⭐', tier: 1 },
  { id: 'clawhub', name: 'ClawHub.ai', desc: '开源 AgentSkills (MIT)', icon: '🦞', tier: 1 },
  { id: 'skillhub', name: 'SkillHub.club', desc: '7000+ Claude/Codex skills', icon: '🎯', tier: 1 },
  { id: 'coze', name: 'Coze 扣子', desc: '字节 Bot + Plugin 市场', icon: '🤖', tier: 2 },
  { id: 'dify', name: 'Dify', desc: 'Dify 模板市场', icon: '🔮', tier: 2 },
  { id: 'gpts_store', name: 'GPTs Store', desc: 'OpenAI GPTs (需 Scrapling)', icon: '🏪', tier: 3 },
]

export default function DronePage() {
  const [stats, setStats] = useState<DroneStats | null>(null)
  const [records, setRecords] = useState<HarvestRecord[]>([])
  const [triggering, setTriggering] = useState<string | null>(null)
  const [error, setError] = useState('')
  const pollRef = useRef<ReturnType<typeof setInterval>>(undefined)

  const fetchData = async () => {
    try {
      const s = await api.get<DroneStats>(DRONE_API + '/stats')
      const r = await api.get<{ records: HarvestRecord[] }>(DRONE_API + '/records')
      setStats(s)
      setRecords(r.records || [])
      setError('')
    } catch (e: any) {
      setError(e.message || 'Drone API 不可用')
    }
  }

  useEffect(() => {
    fetchData()
    pollRef.current = setInterval(fetchData, 10000) // poll every 10s
    return () => clearInterval(pollRef.current)
  }, [])

  const triggerHarvest = async (source: string) => {
    setTriggering(source)
    try {
      await api.post(`${DRONE_API}/harvest/${source}`)
      setTimeout(fetchData, 2000)
    } catch (e: any) {
      setError(e.message)
    } finally {
      setTriggering(null)
    }
  }

  const triggerAll = async () => {
    setTriggering('all')
    try {
      await api.post(`${DRONE_API}/harvest`)
      setTimeout(fetchData, 2000)
    } catch (e: any) {
      setError(e.message)
    } finally {
      setTriggering(null)
    }
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <div className="w-10 h-10 bg-amber-500/20 rounded-xl flex items-center justify-center">
            <Bug className="w-5 h-5 text-amber-400" />
          </div>
          <div>
            <h1 className="text-xl font-bold text-white">工蜂采集中心</h1>
            <p className="text-xs text-gray-500">Drone — Agent/Skill 采集 + 虫茧同化 + 市场导入</p>
          </div>
        </div>
        <div className="flex items-center gap-2">
          <button
            onClick={fetchData}
            className="flex items-center gap-1.5 px-3 py-1.5 text-xs bg-gray-800 text-gray-300 rounded-lg hover:bg-gray-700"
          >
            <RefreshCw className="w-3.5 h-3.5" /> 刷新
          </button>
          <button
            onClick={triggerAll}
            disabled={triggering !== null}
            className="flex items-center gap-1.5 px-4 py-1.5 text-xs bg-amber-600 text-white rounded-lg hover:bg-amber-500 disabled:opacity-50"
          >
            {triggering === 'all' ? <Loader2 className="w-3.5 h-3.5 animate-spin" /> : <Zap className="w-3.5 h-3.5" />}
            全部采集
          </button>
        </div>
      </div>

      {error && (
        <div className="bg-red-500/10 border border-red-500/20 rounded-lg px-4 py-3 text-sm text-red-400">
          {error}
        </div>
      )}

      {/* Stats Cards */}
      <div className="grid grid-cols-2 md:grid-cols-5 gap-3">
        <StatCard label="总采集" value={stats?.total_collected ?? 0} icon={<Download className="w-4 h-4 text-blue-400" />} />
        <StatCard label="同化成功" value={stats?.total_morphed ?? 0} icon={<Zap className="w-4 h-4 text-amber-400" />} />
        <StatCard label="已导入市场" value={stats?.total_imported ?? 0} icon={<Database className="w-4 h-4 text-green-400" />} />
        <StatCard label="成功/失败" value={`${stats?.completed ?? 0}/${stats?.failed ?? 0}`} icon={<CheckCircle className="w-4 h-4 text-emerald-400" />} />
        <StatCard
          label="运行中"
          value={stats?.running?.length ?? 0}
          icon={<Loader2 className={`w-4 h-4 text-purple-400 ${(stats?.running?.length ?? 0) > 0 ? 'animate-spin' : ''}`} />}
          detail={stats?.running?.join(', ')}
        />
      </div>

      {/* Data Sources */}
      <div className="bg-gray-900 border border-gray-800 rounded-xl p-5">
        <h2 className="text-sm font-semibold text-white mb-4">数据源</h2>
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-3">
          {SOURCES.map((src) => {
            const isRunning = stats?.running?.includes(src.id)
            const lastRun = records.find((r) => r.source === src.id)
            return (
              <div
                key={src.id}
                className={`border rounded-lg p-4 transition-colors ${
                  isRunning ? 'border-amber-500/40 bg-amber-500/5' : 'border-gray-800 bg-gray-800/30 hover:border-gray-700'
                }`}
              >
                <div className="flex items-start justify-between mb-2">
                  <div className="flex items-center gap-2">
                    <span className="text-lg">{src.icon}</span>
                    <div>
                      <div className="text-sm font-medium text-white">{src.name}</div>
                      <div className="text-xs text-gray-500">{src.desc}</div>
                    </div>
                  </div>
                  <span className={`text-[10px] px-1.5 py-0.5 rounded ${
                    src.tier === 1 ? 'bg-green-500/20 text-green-400' :
                    src.tier === 2 ? 'bg-blue-500/20 text-blue-400' :
                    'bg-orange-500/20 text-orange-400'
                  }`}>
                    Tier {src.tier}
                  </span>
                </div>
                {lastRun && (
                  <div className="text-xs text-gray-500 mb-2 space-y-0.5">
                    <div>上次: {lastRun.collected} 采集 → {lastRun.morphed} 同化 → {lastRun.imported} 导入</div>
                    <div className="flex items-center gap-1">
                      {lastRun.status === 'completed' ? (
                        <CheckCircle className="w-3 h-3 text-green-500" />
                      ) : lastRun.status === 'failed' ? (
                        <XCircle className="w-3 h-3 text-red-500" />
                      ) : (
                        <Loader2 className="w-3 h-3 text-amber-400 animate-spin" />
                      )}
                      <span>{lastRun.duration || lastRun.status}</span>
                    </div>
                  </div>
                )}
                <button
                  onClick={() => triggerHarvest(src.id)}
                  disabled={isRunning || triggering !== null}
                  className="w-full mt-1 flex items-center justify-center gap-1.5 px-3 py-1.5 text-xs bg-gray-700 text-gray-300 rounded-lg hover:bg-gray-600 disabled:opacity-40 transition-colors"
                >
                  {isRunning || triggering === src.id ? (
                    <><Loader2 className="w-3 h-3 animate-spin" /> 采集中...</>
                  ) : (
                    <><Play className="w-3 h-3" /> 开始采集</>
                  )}
                </button>
              </div>
            )
          })}
        </div>
      </div>

      {/* Recent Records */}
      <div className="bg-gray-900 border border-gray-800 rounded-xl p-5">
        <h2 className="text-sm font-semibold text-white mb-4">采集记录</h2>
        {records.length === 0 ? (
          <div className="text-center py-8 text-gray-500 text-sm">暂无采集记录</div>
        ) : (
          <div className="space-y-2">
            {records.slice(0, 20).map((r) => (
              <div
                key={r.id}
                className="flex items-center justify-between py-2.5 px-3 bg-gray-800/50 rounded-lg"
              >
                <div className="flex items-center gap-3">
                  {r.status === 'completed' ? (
                    <CheckCircle className="w-4 h-4 text-green-500 shrink-0" />
                  ) : r.status === 'failed' ? (
                    <XCircle className="w-4 h-4 text-red-500 shrink-0" />
                  ) : (
                    <Loader2 className="w-4 h-4 text-amber-400 animate-spin shrink-0" />
                  )}
                  <div>
                    <div className="text-sm text-white">{r.source}</div>
                    <div className="text-xs text-gray-500">
                      {r.collected} 采集 → {r.morphed} 同化 → {r.imported} 导入
                      {r.error && <span className="text-red-400 ml-2">{r.error.substring(0, 80)}</span>}
                    </div>
                  </div>
                </div>
                <div className="text-right shrink-0">
                  <div className="text-xs text-gray-400">{r.duration}</div>
                  <div className="text-[10px] text-gray-600">
                    {new Date(r.started_at).toLocaleString('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' })}
                  </div>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  )
}

function StatCard({ label, value, icon, detail }: { label: string; value: string | number; icon: React.ReactNode; detail?: string }) {
  return (
    <div className="bg-gray-900 border border-gray-800 rounded-xl p-4">
      <div className="flex items-center gap-2 mb-2">
        {icon}
        <span className="text-xs text-gray-500">{label}</span>
      </div>
      <div className="text-xl font-bold text-white">{typeof value === 'number' ? value.toLocaleString() : value}</div>
      {detail && <div className="text-[10px] text-gray-500 mt-1 truncate">{detail}</div>}
    </div>
  )
}
