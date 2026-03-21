import { useState, useEffect } from 'react'
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

const statusLabel: Record<string, string> = {
  forming: '组建中', ready: '就绪', running: '运行中', paused: '暂停',
  maintenance: '维护', completed: '已完成', disbanded: '已解散',
}
const mStatusLabel: Record<string, string> = {
  planning: '规划中', confirming: '确认中', executing: '执行中',
  reviewing: '审查中', completed: '已完成', failed: '失败', cancelled: '已取消',
}
const statusDot: Record<string, string> = {
  forming: 'bg-yellow-400', ready: 'bg-blue-400', running: 'bg-green-400',
  paused: 'bg-orange-400', maintenance: 'bg-purple-400', completed: 'bg-gray-400', disbanded: 'bg-red-400',
}

export default function TeamPage() {
  const [instances, setInstances] = useState<TeamInstance[]>([])
  const [selected, setSelected] = useState<TeamInstance | null>(null)
  const [missions, setMissions] = useState<TeamMission[]>([])
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

  async function selectTeam(inst: TeamInstance) {
    setSelected(inst)
    try {
      const res = await api.teamMissions(inst.id)
      setMissions(res.missions || [])
    } catch {}
  }

  async function submitMission() {
    if (!selected || !goalInput.trim()) return
    setSubmitting(true)
    try {
      await api.createTeamMission(selected.id, goalInput.trim())
      setGoalInput('')
      const res = await api.teamMissions(selected.id)
      setMissions(res.missions || [])
      loadInstances()
    } catch {}
    setSubmitting(false)
  }

  if (loading) return <div className="flex items-center justify-center h-full text-gray-500 text-sm">加载中...</div>

  // ── Detail view ──
  if (selected) {
    const progress = selected.energy_budget > 0 ? Math.min(100, Math.round(selected.energy_used / selected.energy_budget * 100)) : 0
    return (
      <div className="h-full flex flex-col">
        {/* Header */}
        <div className="px-5 py-4 border-b border-gray-800 flex items-center gap-3">
          <button onClick={() => setSelected(null)} className="text-gray-400 hover:text-white text-sm">←</button>
          <div className={`w-2.5 h-2.5 rounded-full ${statusDot[selected.status] || 'bg-gray-500'}`} />
          <div className="flex-1 min-w-0">
            <div className="text-sm font-bold text-white truncate">{selected.name}</div>
            <div className="text-[11px] text-gray-500">{selected.template_name} · {statusLabel[selected.status] || selected.status}</div>
          </div>
          <div className="text-xs text-gray-500">{selected.energy_used}/{selected.energy_budget}⚡</div>
        </div>

        {/* Energy bar */}
        {selected.energy_budget > 0 && (
          <div className="px-5 py-2 border-b border-gray-800/50">
            <div className="h-1.5 bg-gray-800 rounded-full overflow-hidden">
              <div className={`h-full rounded-full ${progress > 80 ? 'bg-red-500' : progress > 60 ? 'bg-yellow-500' : 'bg-brand-500'}`} style={{ width: `${progress}%` }} />
            </div>
          </div>
        )}

        {/* Missions list */}
        <div className="flex-1 overflow-auto px-5 py-4 space-y-2">
          {missions.length === 0 ? (
            <div className="text-center py-12 text-gray-600 text-sm">暂无任务</div>
          ) : missions.map(m => (
            <div key={m.id} className="bg-gray-800/40 border border-gray-700/40 rounded-lg p-3">
              <div className="flex items-center gap-2">
                <div className="text-sm font-medium text-white truncate flex-1">{m.title}</div>
                <span className="text-[10px] px-1.5 py-0.5 rounded bg-gray-700 text-gray-300">{mStatusLabel[m.status] || m.status}</span>
              </div>
              <div className="text-xs text-gray-500 mt-1">
                {m.total_steps > 0 && `${m.done_steps}/${m.total_steps} 步骤`}
                {m.review_score > 0 && ` · 评分 ${m.review_score.toFixed(1)}`}
                {m.energy_used > 0 && ` · ${m.energy_used}⚡`}
              </div>
              {m.preview_url && (
                <a href={m.preview_url} target="_blank" rel="noopener noreferrer" className="text-xs text-brand-400 hover:underline mt-1 inline-block">预览 →</a>
              )}
            </div>
          ))}
        </div>

        {/* Submit mission */}
        <div className="px-4 py-3 border-t border-gray-800 flex gap-2">
          <input
            className="flex-1 bg-gray-800 border border-gray-700 rounded-lg px-3 py-2 text-sm text-white placeholder-gray-500 focus:border-brand-500 focus:outline-none"
            placeholder="描述任务需求..."
            value={goalInput}
            onChange={e => setGoalInput(e.target.value)}
            onKeyDown={e => e.key === 'Enter' && !e.shiftKey && submitMission()}
          />
          <button
            onClick={submitMission}
            disabled={submitting || !goalInput.trim()}
            className="px-4 py-2 bg-brand-600 hover:bg-brand-500 text-white rounded-lg text-sm transition disabled:opacity-50 shrink-0"
          >
            {submitting ? '...' : '提交'}
          </button>
        </div>
      </div>
    )
  }

  // ── List view ──
  return (
    <div className="h-full flex flex-col">
      <div className="px-5 py-4 border-b border-gray-800">
        <div className="text-base font-bold text-white">AI 团队</div>
        <div className="text-xs text-gray-500 mt-0.5">你的 AI 团队列表，点击进入查看任务</div>
      </div>
      <div className="flex-1 overflow-auto px-5 py-4">
        {instances.length === 0 ? (
          <div className="text-center py-16">
            <div className="text-3xl mb-2">🤖</div>
            <div className="text-sm text-gray-500">暂无分配的 AI 团队</div>
            <div className="text-xs text-gray-600 mt-1">请联系管理员在控制台创建</div>
          </div>
        ) : (
          <div className="space-y-2">
            {instances.map(inst => (
              <button
                key={inst.id}
                onClick={() => selectTeam(inst)}
                className="w-full text-left bg-gray-800/40 border border-gray-700/40 rounded-xl p-4 hover:border-brand-600/40 transition"
              >
                <div className="flex items-center gap-3">
                  <div className={`w-2.5 h-2.5 rounded-full shrink-0 ${statusDot[inst.status] || 'bg-gray-500'}`} />
                  <div className="flex-1 min-w-0">
                    <div className="text-sm font-medium text-white truncate">{inst.name}</div>
                    <div className="text-xs text-gray-500 mt-0.5 truncate">{inst.template_name} · {inst.goal || '无描述'}</div>
                  </div>
                  <div className="text-right shrink-0">
                    <div className="text-xs text-gray-400">{inst.mission_count} 任务</div>
                    <div className="text-[10px] text-gray-600">{inst.energy_used}⚡</div>
                  </div>
                </div>
              </button>
            ))}
          </div>
        )}
      </div>
    </div>
  )
}
