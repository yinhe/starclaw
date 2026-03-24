import { useEffect, useState } from 'react'
import { Zap, Play, Pause, CheckCircle2, Clock, BarChart3 } from 'lucide-react'
import { api } from '../api'

interface Sprint {
  id: string
  name: string
  goal: string
  status: string
  seq_num: number
  start_date: string | null
  end_date: string | null
  velocity: number
  total_issues: number
  done_issues: number
  total_points: number
  done_points: number
}

export default function SprintPage() {
  const [projects, setProjects] = useState<any[]>([])
  const [selectedProject, setSelectedProject] = useState('')
  const [sprints, setSprints] = useState<Sprint[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    api.listProjects().then((r) => {
      const p = r.projects || []
      setProjects(p)
      if (p.length > 0) setSelectedProject(p[0].id)
    })
  }, [])

  const loadSprints = async () => {
    if (!selectedProject) return
    setLoading(true)
    try {
      const r = await api.listSprints(selectedProject)
      setSprints(r.sprints || [])
    } catch (e) {
      console.error(e)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => { loadSprints() }, [selectedProject])

  const handleStart = async (sprintId: string) => {
    try {
      await api.startSprint(sprintId)
      await loadSprints()
    } catch (e: any) {
      alert(e.message || '启动失败')
    }
  }

  return (
    <div className="p-6 space-y-6 max-w-5xl">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <Zap className="w-6 h-6 text-forge-400" />
          <h1 className="text-xl font-bold">Sprint 管理</h1>
        </div>
        {projects.length > 0 && (
          <select
            value={selectedProject}
            onChange={(e) => setSelectedProject(e.target.value)}
            className="bg-stone-800 border border-stone-700 rounded-lg px-3 py-1.5 text-sm text-stone-300 focus:outline-none focus:border-forge-500"
          >
            {projects.map((p: any) => (
              <option key={p.id} value={p.id}>{p.name}</option>
            ))}
          </select>
        )}
      </div>

      {loading ? (
        <div className="text-stone-600 text-center py-12">加载中...</div>
      ) : sprints.length === 0 ? (
        <div className="text-center py-16">
          <Zap className="w-12 h-12 text-stone-700 mx-auto mb-3" />
          <p className="text-stone-500">暂无 Sprint</p>
          <p className="text-sm text-stone-600 mt-1">通过 PRD 生成器创建 Sprint 计划</p>
        </div>
      ) : (
        <div className="space-y-4">
          {sprints.map((sprint) => {
            const progress = sprint.total_issues > 0 ? Math.round((sprint.done_issues / sprint.total_issues) * 100) : 0
            const pointProgress = sprint.total_points > 0 ? Math.round((sprint.done_points / sprint.total_points) * 100) : 0
            return (
              <div key={sprint.id} className="glass rounded-xl p-5">
                <div className="flex items-center justify-between mb-3">
                  <div className="flex items-center gap-3">
                    <StatusBadge status={sprint.status} />
                    <div>
                      <h3 className="font-semibold text-stone-200">{sprint.name}</h3>
                      {sprint.goal && <p className="text-xs text-stone-500 mt-0.5">{sprint.goal}</p>}
                    </div>
                  </div>
                  <div className="flex items-center gap-3">
                    {sprint.status === 'planned' && (
                      <button
                        onClick={() => handleStart(sprint.id)}
                        className="flex items-center gap-1.5 px-4 py-2 bg-forge-600 hover:bg-forge-500 text-white rounded-lg text-sm transition-colors"
                      >
                        <Play className="w-4 h-4" />
                        一键开始
                      </button>
                    )}
                    {sprint.status === 'completed' && sprint.velocity > 0 && (
                      <div className="flex items-center gap-1.5 text-sm text-green-400">
                        <BarChart3 className="w-4 h-4" />
                        Velocity: {sprint.velocity} pts
                      </div>
                    )}
                  </div>
                </div>

                {/* Progress bars */}
                <div className="space-y-2">
                  <div className="flex items-center gap-3">
                    <span className="text-xs text-stone-500 w-20">Issues</span>
                    <div className="flex-1 bg-stone-800 rounded-full h-2 overflow-hidden">
                      <div className="bg-forge-500 h-full rounded-full transition-all duration-500" style={{ width: `${progress}%` }} />
                    </div>
                    <span className="text-xs text-stone-400 w-24 text-right">{sprint.done_issues}/{sprint.total_issues} ({progress}%)</span>
                  </div>
                  <div className="flex items-center gap-3">
                    <span className="text-xs text-stone-500 w-20">Points</span>
                    <div className="flex-1 bg-stone-800 rounded-full h-2 overflow-hidden">
                      <div className="bg-blue-500 h-full rounded-full transition-all duration-500" style={{ width: `${pointProgress}%` }} />
                    </div>
                    <span className="text-xs text-stone-400 w-24 text-right">{sprint.done_points}/{sprint.total_points} ({pointProgress}%)</span>
                  </div>
                </div>

                {/* Dates */}
                <div className="flex gap-4 mt-3 text-xs text-stone-600">
                  {sprint.start_date && <span>开始: {new Date(sprint.start_date).toLocaleDateString('zh-CN')}</span>}
                  {sprint.end_date && <span>结束: {new Date(sprint.end_date).toLocaleDateString('zh-CN')}</span>}
                </div>
              </div>
            )
          })}
        </div>
      )}
    </div>
  )
}

function StatusBadge({ status }: { status: string }) {
  switch (status) {
    case 'active':
      return <span className="flex items-center gap-1 text-xs px-2 py-1 rounded-full bg-yellow-500/10 text-yellow-400"><Clock className="w-3 h-3" />进行中</span>
    case 'completed':
      return <span className="flex items-center gap-1 text-xs px-2 py-1 rounded-full bg-green-500/10 text-green-400"><CheckCircle2 className="w-3 h-3" />已完成</span>
    default:
      return <span className="flex items-center gap-1 text-xs px-2 py-1 rounded-full bg-stone-500/10 text-stone-400"><Pause className="w-3 h-3" />计划中</span>
  }
}
