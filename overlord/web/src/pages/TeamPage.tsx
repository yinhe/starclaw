import { useState, useEffect, useCallback, useRef } from 'react'
import { api } from '../api/client'

interface TeamInstance {
  id: string; name: string; template_name: string; status: string; goal: string
  energy_budget: number; energy_used: number; mission_count: number; avg_score: number
  created_at: string
}

interface TeamMission {
  id: string; title: string; goal: string; status: string
  total_steps: number; done_steps: number; review_score: number
  energy_used: number; preview_url: string; created_at: string
}

interface DashboardData {
  team_id: string; team_name: string; template_name: string; status: string
  mission_id: string; mission_title: string; total_steps: number; done_steps: number; progress: number
  roles: { code: string; name: string }[]
  energy_budget: number; energy_used: number; energy_rate: number
  mission_count: number; avg_score: number; missions: TeamMission[]
}

const statusLabel: Record<string, string> = {
  forming: '组建中', ready: '就绪', running: '运行中', paused: '暂停',
  maintenance: '维护', completed: '已完成', disbanded: '已解散',
}
const mStatusLabel: Record<string, string> = {
  planning: '规划中', confirming: '确认中', executing: '执行中',
  reviewing: '审查中', completed: '已完成', failed: '失败', cancelled: '已取消',
}
const mStatusColor: Record<string, string> = {
  planning: 'text-yellow-400 bg-yellow-400/10', confirming: 'text-blue-400 bg-blue-400/10',
  executing: 'text-green-400 bg-green-400/10', reviewing: 'text-purple-400 bg-purple-400/10',
  completed: 'text-gray-400 bg-gray-400/10', failed: 'text-red-400 bg-red-400/10',
  cancelled: 'text-gray-500 bg-gray-500/10',
}
const statusDot: Record<string, string> = {
  forming: 'bg-yellow-400', ready: 'bg-blue-400', running: 'bg-green-400',
  paused: 'bg-orange-400', maintenance: 'bg-purple-400', completed: 'bg-gray-400', disbanded: 'bg-red-400',
}
const roleIcons: Record<string, string> = {
  architect: '🏗️', drone: '⚙️', tester: '🧪', reviewer: '🔍', docbot: '📝',
  strategist: '🎯', copywriter: '✍️', designer: '🎨', analyst: '📊',
  dispatcher: '📡', responder: '💬', escalator: '🚨',
  etl_bot: '🔄', reporter: '📋', researcher: '🔬',
}

function useTeamWS(onUpdate: () => void) {
  const wsRef = useRef<WebSocket | null>(null)
  const reconnectRef = useRef<ReturnType<typeof setTimeout>>()

  useEffect(() => {
    function connect() {
      const proto = location.protocol === 'https:' ? 'wss:' : 'ws:'
      const ws = new WebSocket(`${proto}//${location.host}/ws/team-agent?team_id=global`)
      wsRef.current = ws
      ws.onmessage = () => onUpdate()
      ws.onclose = () => { reconnectRef.current = setTimeout(connect, 5000) }
      ws.onerror = () => ws.close()
    }
    connect()
    return () => {
      clearTimeout(reconnectRef.current)
      wsRef.current?.close()
    }
  }, [onUpdate])
}

export default function TeamPage() {
  const [instances, setInstances] = useState<TeamInstance[]>([])
  const [selected, setSelected] = useState<TeamInstance | null>(null)
  const [dash, setDash] = useState<DashboardData | null>(null)
  const [loading, setLoading] = useState(true)
  const [goalInput, setGoalInput] = useState('')
  const [submitting, setSubmitting] = useState(false)

  useEffect(() => { loadInstances() }, [])

  async function loadInstances() {
    setLoading(true)
    try {
      const res = await api.teamInstances()
      setInstances(res.instances || [])
    } catch {}
    setLoading(false)
  }

  const refreshDash = useCallback(() => {
    if (!selected) return
    api.teamDashboard(selected.id).then(setDash).catch(() => {})
  }, [selected])

  useTeamWS(refreshDash)

  // Auto-poll every 5s when viewing active mission
  useEffect(() => {
    if (!selected) return
    refreshDash()
    const t = setInterval(refreshDash, 5000)
    return () => clearInterval(t)
  }, [selected, refreshDash])

  async function selectTeam(inst: TeamInstance) {
    setSelected(inst)
    setDash(null)
  }

  async function submitMission() {
    if (!selected || !goalInput.trim()) return
    setSubmitting(true)
    try {
      await api.createTeamMission(selected.id, goalInput.trim())
      setGoalInput('')
      refreshDash()
      loadInstances()
    } catch {}
    setSubmitting(false)
  }

  if (loading) return <div className="flex items-center justify-center h-full text-gray-500 text-sm">加载中...</div>

  // ── Dashboard view ──
  if (selected) {
    const d = dash
    const energyPct = d && d.energy_budget > 0 ? Math.min(100, Math.round(d.energy_used / d.energy_budget * 100)) : 0
    const isActive = d?.mission_id && d.total_steps > 0
    const missions = d?.missions || []

    return (
      <div className="h-full flex flex-col bg-gray-950">
        {/* Header */}
        <div className="px-5 py-3 border-b border-gray-800 flex items-center gap-3 bg-gray-900/60">
          <button onClick={() => { setSelected(null); setDash(null) }} className="text-gray-400 hover:text-white text-lg leading-none">←</button>
          <div className={`w-2.5 h-2.5 rounded-full ${statusDot[selected.status] || 'bg-gray-500'} ${selected.status === 'running' ? 'animate-pulse' : ''}`} />
          <div className="flex-1 min-w-0">
            <div className="text-sm font-bold text-white truncate">{selected.name}</div>
            <div className="text-[11px] text-gray-500">{selected.template_name} · {statusLabel[selected.status] || selected.status}</div>
          </div>
          {d && <div className="text-xs text-gray-500 tabular-nums">{d.energy_used}/{d.energy_budget}⚡</div>}
        </div>

        <div className="flex-1 overflow-auto">
          {/* ── Active Mission Hero ── */}
          {isActive && d && (
            <div className="mx-4 mt-4 p-4 rounded-xl bg-gradient-to-br from-brand-600/10 to-purple-600/10 border border-brand-500/20">
              <div className="flex items-center gap-2 mb-3">
                <div className="w-2 h-2 rounded-full bg-green-400 animate-pulse" />
                <span className="text-xs font-medium text-green-400 uppercase tracking-wider">任务执行中</span>
              </div>
              <div className="text-sm font-medium text-white mb-1">{d.mission_title}</div>

              {/* Step progress bar */}
              <div className="mt-3">
                <div className="flex items-center justify-between text-[11px] text-gray-400 mb-1.5">
                  <span>步骤进度</span>
                  <span className="tabular-nums font-medium text-white">{d.done_steps}/{d.total_steps}</span>
                </div>
                <div className="h-2.5 bg-gray-800 rounded-full overflow-hidden">
                  <div
                    className="h-full rounded-full bg-gradient-to-r from-brand-500 to-green-500 transition-all duration-700 ease-out"
                    style={{ width: `${d.progress}%` }}
                  />
                </div>
                {/* Step dots */}
                {d.total_steps <= 12 && (
                  <div className="flex items-center gap-1 mt-2">
                    {Array.from({ length: d.total_steps }, (_, i) => {
                      const done = i < d.done_steps
                      const current = i === d.done_steps
                      return (
                        <div
                          key={i}
                          className={`h-1.5 flex-1 rounded-full transition-all duration-500 ${
                            done ? 'bg-green-500' : current ? 'bg-brand-400 animate-pulse' : 'bg-gray-700'
                          }`}
                        />
                      )
                    })}
                  </div>
                )}
              </div>
            </div>
          )}

          {/* ── AI Team Roles ── */}
          {d && d.roles.length > 0 && (
            <div className="px-4 mt-4">
              <div className="text-[11px] text-gray-500 uppercase tracking-wider mb-2 font-medium">团队成员</div>
              <div className="flex flex-wrap gap-2">
                {d.roles.map(r => (
                  <div key={r.code} className="flex items-center gap-1.5 px-2.5 py-1.5 rounded-lg bg-gray-800/60 border border-gray-700/40">
                    <span className="text-sm">{roleIcons[r.code] || '🤖'}</span>
                    <span className="text-xs text-gray-300">{r.name}</span>
                  </div>
                ))}
              </div>
            </div>
          )}

          {/* ── Energy Gauge ── */}
          {d && d.energy_budget > 0 && (
            <div className="px-4 mt-4">
              <div className="flex items-center justify-between text-[11px] text-gray-500 mb-1">
                <span>星能消耗</span>
                <span className="tabular-nums">{d.energy_used.toLocaleString()} / {d.energy_budget.toLocaleString()}⚡ ({energyPct}%)</span>
              </div>
              <div className="h-1.5 bg-gray-800 rounded-full overflow-hidden">
                <div
                  className={`h-full rounded-full transition-all duration-500 ${energyPct > 80 ? 'bg-red-500' : energyPct > 60 ? 'bg-yellow-500' : 'bg-brand-500'}`}
                  style={{ width: `${energyPct}%` }}
                />
              </div>
              {d.energy_rate > 0 && (
                <div className="text-[10px] text-gray-600 mt-1">消耗速率: {d.energy_rate.toFixed(1)}⚡/分钟</div>
              )}
            </div>
          )}

          {/* ── Mission Timeline ── */}
          <div className="px-4 mt-5 pb-4">
            <div className="text-[11px] text-gray-500 uppercase tracking-wider mb-3 font-medium">
              任务历史 {missions.length > 0 && <span className="text-gray-600">({missions.length})</span>}
            </div>
            {missions.length === 0 ? (
              <div className="text-center py-8 text-gray-600 text-sm">提交你的第一个任务，AI 团队将立即开始工作</div>
            ) : (
              <div className="space-y-2">
                {missions.map(m => {
                  const isRunning = ['planning', 'confirming', 'executing', 'reviewing'].includes(m.status)
                  const stepPct = m.total_steps > 0 ? Math.round(m.done_steps / m.total_steps * 100) : 0
                  return (
                    <div
                      key={m.id}
                      className={`rounded-xl p-3.5 border transition-all ${
                        isRunning
                          ? 'bg-gray-800/60 border-brand-500/30 shadow-lg shadow-brand-500/5'
                          : 'bg-gray-800/30 border-gray-700/30'
                      }`}
                    >
                      <div className="flex items-start gap-2">
                        <div className="flex-1 min-w-0">
                          <div className="text-sm font-medium text-white truncate">{m.title}</div>
                          <div className="flex items-center gap-2 mt-1.5">
                            <span className={`text-[10px] px-2 py-0.5 rounded-full font-medium ${mStatusColor[m.status] || 'text-gray-400 bg-gray-700'}`}>
                              {isRunning && <span className="inline-block w-1.5 h-1.5 rounded-full bg-current mr-1 animate-pulse" />}
                              {mStatusLabel[m.status] || m.status}
                            </span>
                            {m.total_steps > 0 && (
                              <span className="text-[10px] text-gray-500 tabular-nums">{m.done_steps}/{m.total_steps} 步骤</span>
                            )}
                            {m.review_score > 0 && (
                              <span className="text-[10px] text-yellow-500">★ {m.review_score.toFixed(1)}</span>
                            )}
                          </div>
                        </div>
                        {m.energy_used > 0 && (
                          <div className="text-[10px] text-gray-600 tabular-nums shrink-0">{m.energy_used}⚡</div>
                        )}
                      </div>

                      {/* Mini progress bar for active missions */}
                      {isRunning && m.total_steps > 0 && (
                        <div className="mt-2.5 h-1 bg-gray-700 rounded-full overflow-hidden">
                          <div
                            className="h-full rounded-full bg-brand-500 transition-all duration-700"
                            style={{ width: `${stepPct}%` }}
                          />
                        </div>
                      )}

                      {m.preview_url && (
                        <a href={m.preview_url} target="_blank" rel="noopener noreferrer"
                          className="inline-flex items-center gap-1 mt-2 text-xs text-brand-400 hover:text-brand-300 transition">
                          <span>查看预览</span>
                          <span className="text-[10px]">↗</span>
                        </a>
                      )}
                    </div>
                  )
                })}
              </div>
            )}
          </div>
        </div>

        {/* Submit mission */}
        <div className="px-4 py-3 border-t border-gray-800 bg-gray-900/60">
          <div className="flex gap-2">
            <input
              className="flex-1 bg-gray-800 border border-gray-700 rounded-xl px-3.5 py-2.5 text-sm text-white placeholder-gray-500 focus:border-brand-500 focus:outline-none transition"
              placeholder="描述任务需求，AI 团队将立即开始..."
              value={goalInput}
              onChange={e => setGoalInput(e.target.value)}
              onKeyDown={e => e.key === 'Enter' && !e.shiftKey && submitMission()}
            />
            <button
              onClick={submitMission}
              disabled={submitting || !goalInput.trim()}
              className="px-5 py-2.5 bg-brand-600 hover:bg-brand-500 text-white rounded-xl text-sm font-medium transition disabled:opacity-40 shrink-0"
            >
              {submitting ? (
                <span className="inline-block w-4 h-4 border-2 border-white/30 border-t-white rounded-full animate-spin" />
              ) : '提交任务'}
            </button>
          </div>
        </div>
      </div>
    )
  }

  // ── List view ──
  return (
    <div className="h-full flex flex-col bg-gray-950">
      <div className="px-5 py-4 border-b border-gray-800 bg-gray-900/60">
        <div className="text-base font-bold text-white">AI 团队</div>
        <div className="text-xs text-gray-500 mt-0.5">你的 AI 团队列表 · 点击进入实时看板</div>
      </div>
      <div className="flex-1 overflow-auto px-4 py-4">
        {instances.length === 0 ? (
          <div className="text-center py-16">
            <div className="text-4xl mb-3">�</div>
            <div className="text-sm text-gray-400 font-medium">暂无分配的 AI 团队</div>
            <div className="text-xs text-gray-600 mt-1">请联系管理员在控制台创建团队</div>
          </div>
        ) : (
          <div className="space-y-3">
            {instances.map(inst => {
              const isActive = ['forming', 'ready', 'running'].includes(inst.status)
              return (
                <button
                  key={inst.id}
                  onClick={() => selectTeam(inst)}
                  className={`w-full text-left rounded-xl p-4 border transition-all group ${
                    isActive
                      ? 'bg-gray-800/50 border-gray-700/50 hover:border-brand-500/40 hover:shadow-lg hover:shadow-brand-500/5'
                      : 'bg-gray-800/30 border-gray-700/30 hover:border-gray-600/50'
                  }`}
                >
                  <div className="flex items-center gap-3">
                    <div className={`w-3 h-3 rounded-full shrink-0 ${statusDot[inst.status] || 'bg-gray-500'} ${inst.status === 'running' ? 'animate-pulse' : ''}`} />
                    <div className="flex-1 min-w-0">
                      <div className="text-sm font-medium text-white truncate group-hover:text-brand-300 transition">{inst.name}</div>
                      <div className="text-xs text-gray-500 mt-0.5 truncate">
                        {inst.template_name} · {statusLabel[inst.status] || inst.status}
                        {inst.goal && <span className="text-gray-600"> · {inst.goal}</span>}
                      </div>
                    </div>
                    <div className="text-right shrink-0">
                      <div className="text-xs text-gray-400 tabular-nums">{inst.mission_count} 任务</div>
                      <div className="text-[10px] text-gray-600 tabular-nums">{inst.energy_used.toLocaleString()}⚡</div>
                    </div>
                  </div>
                  {/* Energy mini bar */}
                  {inst.energy_budget > 0 && (
                    <div className="mt-2.5 h-1 bg-gray-700/60 rounded-full overflow-hidden">
                      <div
                        className="h-full rounded-full bg-brand-500/60"
                        style={{ width: `${Math.min(100, Math.round(inst.energy_used / inst.energy_budget * 100))}%` }}
                      />
                    </div>
                  )}
                </button>
              )
            })}
          </div>
        )}
      </div>
    </div>
  )
}
