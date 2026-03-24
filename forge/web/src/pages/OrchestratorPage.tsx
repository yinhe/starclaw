import { useEffect, useState } from 'react'
import { Activity, Cpu, Plus, RefreshCw, Wifi, WifiOff } from 'lucide-react'
import { api } from '../api'

interface Agent {
  id: string
  name: string
  type: string
  capabilities: string
  services: string
  status: string
  current_issue: string
  last_seen_at: string
  registered_at: string
}

export default function OrchestratorPage() {
  const [status, setStatus] = useState<any>(null)
  const [agents, setAgents] = useState<Agent[]>([])
  const [loading, setLoading] = useState(true)
  const [showRegister, setShowRegister] = useState(false)
  const [regForm, setRegForm] = useState({ name: '', type: 'windsurf', capabilities: '["go","react"]', services: '["claw/api"]' })

  const load = async () => {
    try {
      const [s, a] = await Promise.all([api.orchestratorStatus(), api.listAgents()])
      setStatus(s)
      setAgents(a.agents || [])
    } catch (e) {
      console.error(e)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    load()
    const timer = setInterval(load, 10000)
    return () => clearInterval(timer)
  }, [])

  const handleRegister = async () => {
    if (!regForm.name.trim()) return
    try {
      await api.registerAgent(regForm)
      setShowRegister(false)
      setRegForm({ name: '', type: 'windsurf', capabilities: '["go","react"]', services: '["claw/api"]' })
      await load()
    } catch (e: any) {
      alert(e.message || '注册失败')
    }
  }

  if (loading) {
    return <div className="flex items-center justify-center h-full text-stone-600">加载中...</div>
  }

  return (
    <div className="p-6 space-y-6 max-w-5xl">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <Activity className="w-6 h-6 text-forge-400" />
          <h1 className="text-xl font-bold">Orchestrator 调度中心</h1>
        </div>
        <div className="flex gap-2">
          <button onClick={load} className="flex items-center gap-1.5 px-3 py-1.5 text-sm text-stone-400 hover:text-stone-200 bg-stone-800 hover:bg-stone-700 rounded-lg transition-colors">
            <RefreshCw className="w-3.5 h-3.5" />
            刷新
          </button>
          <button
            onClick={() => setShowRegister(!showRegister)}
            className="flex items-center gap-1.5 px-4 py-1.5 text-sm bg-forge-600 hover:bg-forge-500 text-white rounded-lg transition-colors"
          >
            <Plus className="w-3.5 h-3.5" />
            注册 Agent
          </button>
        </div>
      </div>

      {/* Status Cards */}
      {status && (
        <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
          <MiniCard label="活跃 Sprint" value={status.active_sprints} color="text-yellow-400" />
          <MiniCard label="执行中 Issue" value={status.dispatched_issues} color="text-blue-400" />
          <MiniCard label="忙碌 Agent" value={status.busy_agents} color="text-orange-400" />
          <MiniCard label="空闲 Agent" value={status.idle_agents} color="text-green-400" />
        </div>
      )}

      {/* Register Form */}
      {showRegister && (
        <div className="glass rounded-xl p-4 space-y-3">
          <h3 className="text-sm font-medium text-stone-300">注册新 Agent</h3>
          <div className="grid grid-cols-2 gap-3">
            <input
              value={regForm.name}
              onChange={(e) => setRegForm({ ...regForm, name: e.target.value })}
              placeholder="名称 (windsurf-1)"
              className="bg-stone-800 border border-stone-700 rounded-lg px-3 py-2 text-sm focus:outline-none focus:border-forge-500"
            />
            <select
              value={regForm.type}
              onChange={(e) => setRegForm({ ...regForm, type: e.target.value })}
              className="bg-stone-800 border border-stone-700 rounded-lg px-3 py-2 text-sm"
            >
              <option value="windsurf">Windsurf</option>
              <option value="cursor">Cursor</option>
              <option value="vscode">VS Code</option>
              <option value="devclaw">DevClaw</option>
            </select>
            <input
              value={regForm.capabilities}
              onChange={(e) => setRegForm({ ...regForm, capabilities: e.target.value })}
              placeholder='能力 JSON: ["go","react"]'
              className="bg-stone-800 border border-stone-700 rounded-lg px-3 py-2 text-sm focus:outline-none focus:border-forge-500"
            />
            <input
              value={regForm.services}
              onChange={(e) => setRegForm({ ...regForm, services: e.target.value })}
              placeholder='服务 JSON: ["claw/api"]'
              className="bg-stone-800 border border-stone-700 rounded-lg px-3 py-2 text-sm focus:outline-none focus:border-forge-500"
            />
          </div>
          <div className="flex justify-end">
            <button onClick={handleRegister} className="px-4 py-2 bg-forge-600 hover:bg-forge-500 text-white rounded-lg text-sm">
              注册
            </button>
          </div>
        </div>
      )}

      {/* Agent Pool */}
      <div className="glass rounded-xl p-5">
        <div className="flex items-center gap-2 mb-4">
          <Cpu className="w-5 h-5 text-stone-400" />
          <h2 className="font-semibold">Agent 池</h2>
          <span className="text-xs text-stone-600 ml-auto">{agents.length} 个注册</span>
        </div>

        {agents.length === 0 ? (
          <div className="text-center py-8">
            <Cpu className="w-10 h-10 text-stone-700 mx-auto mb-2" />
            <p className="text-sm text-stone-500">无注册 Agent</p>
            <p className="text-xs text-stone-600 mt-1">点击上方「注册 Agent」添加编辑器会话</p>
          </div>
        ) : (
          <div className="space-y-3">
            {agents.map((agent) => (
              <div key={agent.id} className="flex items-center gap-4 p-3 rounded-lg bg-stone-800/50 hover:bg-stone-800 transition-colors">
                <div className={`w-3 h-3 rounded-full shrink-0 ${
                  agent.status === 'idle' ? 'bg-green-400 animate-pulse-slow' :
                  agent.status === 'busy' ? 'bg-yellow-400' :
                  'bg-stone-600'
                }`} />
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-2">
                    <span className="font-mono text-sm text-stone-200">{agent.name}</span>
                    <span className="text-[10px] px-1.5 py-0.5 rounded bg-stone-700 text-stone-400">{agent.type}</span>
                    <span className={`text-[10px] px-1.5 py-0.5 rounded ${
                      agent.status === 'idle' ? 'bg-green-500/10 text-green-400' :
                      agent.status === 'busy' ? 'bg-yellow-500/10 text-yellow-400' :
                      'bg-stone-700 text-stone-500'
                    }`}>
                      {agent.status === 'idle' ? '空闲' : agent.status === 'busy' ? '忙碌' : '离线'}
                    </span>
                  </div>
                  <div className="flex gap-3 mt-1 text-xs text-stone-500">
                    {agent.capabilities && <span>能力: {tryFormatJSON(agent.capabilities)}</span>}
                    {agent.services && <span>服务: {tryFormatJSON(agent.services)}</span>}
                  </div>
                  {agent.current_issue && (
                    <p className="text-xs text-yellow-400/70 mt-1">当前任务: {agent.current_issue}</p>
                  )}
                </div>
                <div className="text-right shrink-0">
                  {agent.status === 'idle' ? (
                    <Wifi className="w-4 h-4 text-green-500" />
                  ) : agent.status === 'offline' ? (
                    <WifiOff className="w-4 h-4 text-stone-600" />
                  ) : (
                    <Activity className="w-4 h-4 text-yellow-400 animate-pulse" />
                  )}
                </div>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  )
}

function MiniCard({ label, value, color }: { label: string; value: number; color: string }) {
  return (
    <div className="glass rounded-xl p-4 text-center">
      <p className={`text-2xl font-bold ${color}`}>{value ?? 0}</p>
      <p className="text-xs text-stone-500 mt-1">{label}</p>
    </div>
  )
}

function tryFormatJSON(s: string): string {
  try {
    const arr = JSON.parse(s)
    if (Array.isArray(arr)) return arr.join(', ')
    return s
  } catch {
    return s
  }
}
