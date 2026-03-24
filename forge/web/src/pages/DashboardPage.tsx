import { useEffect, useState } from 'react'
import {
  Activity,
  CheckCircle2,
  CircleDot,
  Cpu,
  Flame,
  GitBranch,
  Layers,
  Server,
  Timer,
  Zap,
} from 'lucide-react'
import { api } from '../api'

interface ServiceInfo {
  name: string
  status: string
  latency_ms: number
}

interface ActivityItem {
  id: string
  type: string
  actor: string
  summary: string
  service: string
  source: string
  created_at: string
}

interface AgentInfo {
  id: string
  name: string
  type: string
  status: string
  current_issue: string
}

export default function DashboardPage() {
  const [data, setData] = useState<any>(null)
  const [services, setServices] = useState<ServiceInfo[]>([])
  const [loading, setLoading] = useState(true)

  const load = async () => {
    try {
      const [d, s] = await Promise.all([api.dashboard(), api.services()])
      setData(d)
      setServices(s.services || [])
    } catch (e) {
      console.error(e)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    load()
    const timer = setInterval(load, 15000)
    return () => clearInterval(timer)
  }, [])

  if (loading) {
    return (
      <div className="flex items-center justify-center h-full">
        <Flame className="w-8 h-8 text-forge-500 animate-pulse" />
      </div>
    )
  }

  const activities: ActivityItem[] = data?.activities || []
  const agents: AgentInfo[] = data?.agents || []
  const sprint = data?.active_sprint

  return (
    <div className="p-6 space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <Flame className="w-8 h-8 text-forge-500" />
          <div>
            <h1 className="text-2xl font-bold text-stone-100">Forge 研发管控大屏</h1>
            <p className="text-sm text-stone-500">StarClaw 全局开发进度 · 实时监控</p>
          </div>
        </div>
        <div className="text-sm text-stone-500">
          {new Date().toLocaleString('zh-CN')}
        </div>
      </div>

      {/* Stats row */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
        <StatCard icon={Layers} label="项目" value={data?.projects ?? 0} color="text-blue-400" />
        <StatCard icon={CircleDot} label="待办 Issues" value={data?.open_issues ?? 0} color="text-yellow-400" />
        <StatCard icon={CheckCircle2} label="已完成" value={data?.done_issues ?? 0} color="text-green-400" />
        <StatCard icon={Cpu} label="Agent 池" value={agents.length} color="text-purple-400" />
      </div>

      {/* Active Sprint */}
      {sprint && (
        <div className="glass rounded-xl p-5">
          <div className="flex items-center gap-3 mb-4">
            <Zap className="w-5 h-5 text-forge-400" />
            <h2 className="text-lg font-semibold">{sprint.sprint?.name || 'Active Sprint'}</h2>
            <span className="ml-auto text-sm text-stone-400">
              {sprint.done_issues}/{sprint.total_issues} issues · {sprint.progress}%
            </span>
          </div>
          <div className="w-full bg-stone-800 rounded-full h-3 overflow-hidden">
            <div
              className="bg-gradient-to-r from-forge-600 to-forge-400 h-full rounded-full transition-all duration-700"
              style={{ width: `${sprint.progress}%` }}
            />
          </div>
        </div>
      )}

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* Services Health */}
        <div className="glass rounded-xl p-5">
          <div className="flex items-center gap-2 mb-4">
            <Server className="w-5 h-5 text-stone-400" />
            <h2 className="font-semibold">服务健康</h2>
          </div>
          <div className="space-y-2">
            {services.map((svc) => (
              <div key={svc.name} className="flex items-center justify-between text-sm py-1.5 px-2 rounded-lg hover:bg-stone-800/50 transition-colors">
                <div className="flex items-center gap-2">
                  <div className={`w-2 h-2 rounded-full ${svc.status === 'healthy' ? 'bg-green-400' : svc.status === 'unhealthy' ? 'bg-red-400' : 'bg-stone-600'}`} />
                  <span className="text-stone-300">{svc.name}</span>
                </div>
                <span className={`text-xs ${svc.status === 'healthy' ? 'text-stone-500' : 'text-red-400'}`}>
                  {svc.status === 'healthy' ? `${svc.latency_ms}ms` : svc.status}
                </span>
              </div>
            ))}
            {services.length === 0 && <p className="text-sm text-stone-600">无服务数据</p>}
          </div>
        </div>

        {/* Activity Feed */}
        <div className="glass rounded-xl p-5">
          <div className="flex items-center gap-2 mb-4">
            <Activity className="w-5 h-5 text-stone-400" />
            <h2 className="font-semibold">活动流</h2>
          </div>
          <div className="space-y-2 max-h-80 overflow-auto">
            {activities.slice(0, 15).map((a) => (
              <div key={a.id} className="text-sm py-1.5 px-2 rounded-lg hover:bg-stone-800/50 transition-colors">
                <div className="flex items-start gap-2">
                  <ActivityIcon type={a.type} />
                  <div className="flex-1 min-w-0">
                    <p className="text-stone-300 truncate">{a.summary}</p>
                    <p className="text-xs text-stone-600 mt-0.5">
                      {a.actor} · {a.source} · {timeAgo(a.created_at)}
                    </p>
                  </div>
                </div>
              </div>
            ))}
            {activities.length === 0 && <p className="text-sm text-stone-600">暂无活动</p>}
          </div>
        </div>

        {/* Agent Pool */}
        <div className="glass rounded-xl p-5">
          <div className="flex items-center gap-2 mb-4">
            <Cpu className="w-5 h-5 text-stone-400" />
            <h2 className="font-semibold">Agent 池</h2>
          </div>
          <div className="space-y-2">
            {agents.map((agent) => (
              <div key={agent.id} className="flex items-center justify-between text-sm py-2 px-3 rounded-lg glass-hover">
                <div className="flex items-center gap-2">
                  <div className={`w-2 h-2 rounded-full ${agent.status === 'idle' ? 'bg-green-400 animate-pulse-slow' : agent.status === 'busy' ? 'bg-yellow-400' : 'bg-stone-600'}`} />
                  <span className="text-stone-300 font-mono text-xs">{agent.name}</span>
                </div>
                <span className={`text-xs px-2 py-0.5 rounded-full ${
                  agent.status === 'idle' ? 'bg-green-400/10 text-green-400' :
                  agent.status === 'busy' ? 'bg-yellow-400/10 text-yellow-400' :
                  'bg-stone-700 text-stone-500'
                }`}>
                  {agent.status}
                </span>
              </div>
            ))}
            {agents.length === 0 && (
              <div className="text-center py-6">
                <Cpu className="w-8 h-8 text-stone-700 mx-auto mb-2" />
                <p className="text-sm text-stone-600">无注册 Agent</p>
                <p className="text-xs text-stone-700 mt-1">POST /api/orchestrator/register</p>
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  )
}

function StatCard({ icon: Icon, label, value, color }: { icon: any; label: string; value: number; color: string }) {
  return (
    <div className="glass rounded-xl p-4 flex items-center gap-4">
      <div className={`p-2.5 rounded-lg bg-stone-800/80 ${color}`}>
        <Icon className="w-5 h-5" />
      </div>
      <div>
        <p className="text-2xl font-bold text-stone-100">{value}</p>
        <p className="text-xs text-stone-500">{label}</p>
      </div>
    </div>
  )
}

function ActivityIcon({ type }: { type: string }) {
  switch (type) {
    case 'commit': return <GitBranch className="w-3.5 h-3.5 text-blue-400 mt-0.5 shrink-0" />
    case 'pr': return <GitBranch className="w-3.5 h-3.5 text-purple-400 mt-0.5 shrink-0" />
    case 'issue': return <CircleDot className="w-3.5 h-3.5 text-green-400 mt-0.5 shrink-0" />
    case 'deploy': return <Zap className="w-3.5 h-3.5 text-yellow-400 mt-0.5 shrink-0" />
    case 'ci': return <Timer className="w-3.5 h-3.5 text-orange-400 mt-0.5 shrink-0" />
    default: return <Activity className="w-3.5 h-3.5 text-stone-500 mt-0.5 shrink-0" />
  }
}

function timeAgo(dateStr: string): string {
  if (!dateStr) return ''
  const diff = Date.now() - new Date(dateStr).getTime()
  const mins = Math.floor(diff / 60000)
  if (mins < 1) return '刚刚'
  if (mins < 60) return `${mins}分钟前`
  const hours = Math.floor(mins / 60)
  if (hours < 24) return `${hours}小时前`
  const days = Math.floor(hours / 24)
  return `${days}天前`
}
