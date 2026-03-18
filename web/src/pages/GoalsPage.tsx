import { useState, useEffect } from 'react'
import { ChevronRight, Flag, Play, Plus, RefreshCw, Target, Users, XCircle } from 'lucide-react'
import { goalAPI, collaborationAPI, agentAPI } from '../lib/api'

type Tab = 'goals' | 'collaborations'

export default function GoalsPage() {
  const [tab, setTab] = useState<Tab>('goals')
  const [goals, setGoals] = useState<any[]>([])
  const [collabs, setCollabs] = useState<any[]>([])
  const [stats, setStats] = useState<any>(null)
  const [agents, setAgents] = useState<any[]>([])
  const [loading, setLoading] = useState(false)
  const [showCreate, setShowCreate] = useState(false)
  const [statusFilter, setStatusFilter] = useState('')
  const [selectedGoal, setSelectedGoal] = useState<any>(null)
  const [goalDetail, setGoalDetail] = useState<any>(null)

  const [newGoal, setNewGoal] = useState({ agent_id: '', title: '', description: '', priority: 5, trigger_type: 'manual', max_steps: 20 })
  const [showCreateCollab, setShowCreateCollab] = useState(false)
  const [newCollab, setNewCollab] = useState({ title: '', protocol: 'consensus', max_agents: 5 })

  useEffect(() => { agentAPI.list().then(r => setAgents(r.data || [])).catch(() => {}) }, [])
  useEffect(() => { loadData() }, [tab, statusFilter])

  const loadData = async () => {
    setLoading(true)
    try {
      if (tab === 'goals') {
        const [g, s] = await Promise.all([
          goalAPI.list({ status: statusFilter || undefined, page_size: 50 }),
          goalAPI.stats(),
        ])
        setGoals(g.data?.items || [])
        setStats(s.data)
      } else {
        const r = await collaborationAPI.list()
        setCollabs(r.data || [])
      }
    } catch {}
    setLoading(false)
  }

  const createGoal = async () => {
    await goalAPI.create(newGoal)
    setShowCreate(false)
    setNewGoal({ agent_id: '', title: '', description: '', priority: 5, trigger_type: 'manual', max_steps: 20 })
    loadData()
  }

  const activateGoal = async (id: string) => { await goalAPI.activate(id); loadData() }
  const cancelGoal = async (id: string) => { await goalAPI.cancel(id); loadData() }

  const viewGoal = async (id: string) => {
    const r = await goalAPI.get(id)
    setGoalDetail(r.data)
    setSelectedGoal(id)
  }

  const createCollab = async () => {
    await collaborationAPI.create(newCollab)
    setShowCreateCollab(false)
    setNewCollab({ title: '', protocol: 'consensus', max_agents: 5 })
    loadData()
  }

  const statusColors: Record<string, string> = {
    pending: 'bg-gray-100 text-gray-600', active: 'bg-blue-100 text-blue-700',
    completed: 'bg-green-100 text-green-700', failed: 'bg-red-100 text-red-700',
    cancelled: 'bg-gray-200 text-gray-500',
  }

  return (
    <div className="p-6 max-w-7xl mx-auto">
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-2xl font-bold text-gray-900 dark:text-white flex items-center gap-2"><Target className="w-6 h-6 text-primary-600" /> 自主目标</h1>
          <p className="text-sm text-gray-500 mt-1">目标驱动的自主执行 + 多 Agent 协作</p>
        </div>
        <button onClick={loadData} className="flex items-center gap-2 px-3 py-2 text-sm bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg hover:bg-gray-50">
          <RefreshCw className={`w-4 h-4 ${loading ? 'animate-spin' : ''}`} />
        </button>
      </div>

      <div className="flex gap-1 mb-6 bg-gray-100 dark:bg-gray-800 p-1 rounded-lg w-fit">
        <button onClick={() => setTab('goals')} className={`flex items-center gap-1.5 px-3 py-1.5 text-sm rounded-md transition ${tab === 'goals' ? 'bg-white dark:bg-gray-700 shadow text-primary-600 font-medium' : 'text-gray-500'}`}>
          <Flag className="w-4 h-4" /> 目标
        </button>
        <button onClick={() => setTab('collaborations')} className={`flex items-center gap-1.5 px-3 py-1.5 text-sm rounded-md transition ${tab === 'collaborations' ? 'bg-white dark:bg-gray-700 shadow text-primary-600 font-medium' : 'text-gray-500'}`}>
          <Users className="w-4 h-4" /> 多 Agent 协作
        </button>
      </div>

      {/* Goals Tab */}
      {tab === 'goals' && (
        <div>
          {/* Stats */}
          {stats && (
            <div className="grid grid-cols-5 gap-3 mb-4">
              {[
                { label: '总计', value: stats.total_goals ?? 0 },
                { label: '等待中', value: stats.pending ?? 0 },
                { label: '进行中', value: stats.active ?? 0 },
                { label: '已完成', value: stats.completed ?? 0 },
                { label: '完成率', value: `${((stats.completion_rate ?? 0) * 100).toFixed(0)}%` },
              ].map(s => (
                <div key={s.label} className="bg-white dark:bg-gray-800 rounded-lg p-3 border border-gray-200 dark:border-gray-700 text-center">
                  <p className="text-xs text-gray-500">{s.label}</p>
                  <p className="text-xl font-bold text-gray-900 dark:text-white">{s.value}</p>
                </div>
              ))}
            </div>
          )}

          <div className="flex justify-between items-center mb-4">
            <select value={statusFilter} onChange={e => setStatusFilter(e.target.value)} className="px-3 py-2 text-sm bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg">
              <option value="">全部状态</option>
              <option value="pending">等待中</option><option value="active">进行中</option><option value="completed">已完成</option><option value="failed">失败</option>
            </select>
            <button onClick={() => setShowCreate(true)} className="flex items-center gap-1.5 px-4 py-2 text-sm bg-primary-600 text-white rounded-lg hover:bg-primary-700"><Plus className="w-4 h-4" /> 新建目标</button>
          </div>

          {showCreate && (
            <div className="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-4 mb-4 space-y-3">
              <select value={newGoal.agent_id} onChange={e => setNewGoal({ ...newGoal, agent_id: e.target.value })} className="w-full px-3 py-2 text-sm bg-gray-50 dark:bg-gray-900 border border-gray-200 dark:border-gray-700 rounded-lg">
                <option value="">选择执行 Agent</option>
                {agents.map((a: any) => <option key={a.id} value={a.id}>{a.name}</option>)}
              </select>
              <input value={newGoal.title} onChange={e => setNewGoal({ ...newGoal, title: e.target.value })} placeholder="目标标题" className="w-full px-3 py-2 text-sm bg-gray-50 dark:bg-gray-900 border border-gray-200 dark:border-gray-700 rounded-lg" />
              <textarea value={newGoal.description} onChange={e => setNewGoal({ ...newGoal, description: e.target.value })} placeholder="详细描述（可选）" rows={2} className="w-full px-3 py-2 text-sm bg-gray-50 dark:bg-gray-900 border border-gray-200 dark:border-gray-700 rounded-lg" />
              <div className="flex gap-2">
                <select value={newGoal.trigger_type} onChange={e => setNewGoal({ ...newGoal, trigger_type: e.target.value })} className="px-3 py-2 text-sm bg-gray-50 dark:bg-gray-900 border border-gray-200 dark:border-gray-700 rounded-lg">
                  <option value="manual">手动触发</option><option value="schedule">定时触发</option><option value="event">事件触发</option>
                </select>
                <input type="number" value={newGoal.priority} onChange={e => setNewGoal({ ...newGoal, priority: parseInt(e.target.value) || 5 })} min={1} max={10} className="w-20 px-3 py-2 text-sm bg-gray-50 dark:bg-gray-900 border border-gray-200 dark:border-gray-700 rounded-lg" placeholder="优先级" />
                <input type="number" value={newGoal.max_steps} onChange={e => setNewGoal({ ...newGoal, max_steps: parseInt(e.target.value) || 20 })} min={1} max={100} className="w-24 px-3 py-2 text-sm bg-gray-50 dark:bg-gray-900 border border-gray-200 dark:border-gray-700 rounded-lg" placeholder="最大步数" />
              </div>
              <div className="flex gap-2 justify-end">
                <button onClick={() => setShowCreate(false)} className="px-3 py-1.5 text-sm text-gray-500">取消</button>
                <button onClick={createGoal} disabled={!newGoal.title || !newGoal.agent_id} className="px-4 py-1.5 text-sm bg-primary-600 text-white rounded-lg hover:bg-primary-700 disabled:opacity-50">创建</button>
              </div>
            </div>
          )}

          {/* Goal List */}
          <div className="space-y-2">
            {goals.map((g: any) => (
              <div key={g.id} className="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-4">
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-3 min-w-0">
                    <span className={`px-2 py-0.5 text-xs rounded shrink-0 ${statusColors[g.status] || 'bg-gray-100 text-gray-600'}`}>{g.status}</span>
                    <div className="min-w-0">
                      <p className="font-medium text-gray-900 dark:text-white truncate">{g.title}</p>
                      <p className="text-xs text-gray-500">优先级 {g.priority} · 步数 {g.steps_used}/{g.max_steps} · {g.trigger_type}</p>
                    </div>
                  </div>
                  <div className="flex items-center gap-2 shrink-0">
                    {g.status === 'pending' && <button onClick={() => activateGoal(g.id)} className="p-1.5 hover:bg-green-50 rounded text-green-600" title="激活"><Play className="w-4 h-4" /></button>}
                    {(g.status === 'pending' || g.status === 'active') && <button onClick={() => cancelGoal(g.id)} className="p-1.5 hover:bg-red-50 rounded text-red-500" title="取消"><XCircle className="w-4 h-4" /></button>}
                    <button onClick={() => viewGoal(g.id)} className="p-1.5 hover:bg-gray-100 dark:hover:bg-gray-700 rounded text-gray-400"><ChevronRight className="w-4 h-4" /></button>
                  </div>
                </div>
                {/* Progress bar */}
                <div className="mt-2 w-full bg-gray-100 dark:bg-gray-700 rounded-full h-1.5">
                  <div className="bg-primary-600 h-1.5 rounded-full transition-all" style={{ width: `${(g.progress || 0) * 100}%` }} />
                </div>
              </div>
            ))}
            {goals.length === 0 && <p className="text-center text-gray-400 py-8">暂无目标</p>}
          </div>

          {/* Goal Detail Modal */}
          {selectedGoal && goalDetail && (
            <div className="fixed inset-0 bg-black/50 z-50 flex items-center justify-center p-4" onClick={() => setSelectedGoal(null)}>
              <div className="bg-white dark:bg-gray-800 rounded-2xl max-w-lg w-full max-h-[80vh] overflow-y-auto p-6" onClick={e => e.stopPropagation()}>
                <h3 className="text-lg font-bold text-gray-900 dark:text-white mb-2">{goalDetail.goal?.title}</h3>
                <p className="text-sm text-gray-500 mb-4">{goalDetail.goal?.description}</p>
                <p className="text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">执行步骤</p>
                <div className="space-y-2">
                  {(goalDetail.steps || []).map((s: any) => (
                    <div key={s.id} className="flex items-start gap-2 p-2 bg-gray-50 dark:bg-gray-900 rounded-lg">
                      <span className={`mt-0.5 w-5 h-5 rounded-full flex items-center justify-center text-xs shrink-0 ${s.status === 'done' ? 'bg-green-100 text-green-600' : s.status === 'error' ? 'bg-red-100 text-red-600' : 'bg-gray-100 text-gray-400'}`}>{s.step_number}</span>
                      <div className="min-w-0">
                        <p className="text-sm text-gray-700 dark:text-gray-300">{s.description || s.action}</p>
                        {s.output && <p className="text-xs text-gray-400 mt-0.5 truncate">{s.output}</p>}
                      </div>
                    </div>
                  ))}
                  {(goalDetail.steps || []).length === 0 && <p className="text-sm text-gray-400 text-center py-4">暂无执行步骤</p>}
                </div>
                <button onClick={() => setSelectedGoal(null)} className="mt-4 w-full py-2 text-sm text-gray-500 border border-gray-200 dark:border-gray-700 rounded-lg hover:bg-gray-50">关闭</button>
              </div>
            </div>
          )}
        </div>
      )}

      {/* Collaborations Tab */}
      {tab === 'collaborations' && (
        <div>
          <div className="flex justify-end mb-4">
            <button onClick={() => setShowCreateCollab(true)} className="flex items-center gap-1.5 px-4 py-2 text-sm bg-primary-600 text-white rounded-lg hover:bg-primary-700"><Plus className="w-4 h-4" /> 新建协作</button>
          </div>
          {showCreateCollab && (
            <div className="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-4 mb-4 space-y-3">
              <input value={newCollab.title} onChange={e => setNewCollab({ ...newCollab, title: e.target.value })} placeholder="协作标题" className="w-full px-3 py-2 text-sm bg-gray-50 dark:bg-gray-900 border border-gray-200 dark:border-gray-700 rounded-lg" />
              <div className="flex gap-2">
                <select value={newCollab.protocol} onChange={e => setNewCollab({ ...newCollab, protocol: e.target.value })} className="px-3 py-2 text-sm bg-gray-50 dark:bg-gray-900 border border-gray-200 dark:border-gray-700 rounded-lg">
                  <option value="consensus">共识投票</option><option value="delegation">委派分工</option><option value="auction">拍卖竞标</option><option value="voting">多数表决</option>
                </select>
                <input type="number" value={newCollab.max_agents} onChange={e => setNewCollab({ ...newCollab, max_agents: parseInt(e.target.value) || 5 })} min={2} max={20} className="w-24 px-3 py-2 text-sm bg-gray-50 dark:bg-gray-900 border border-gray-200 dark:border-gray-700 rounded-lg" placeholder="最大Agent数" />
              </div>
              <div className="flex gap-2 justify-end">
                <button onClick={() => setShowCreateCollab(false)} className="px-3 py-1.5 text-sm text-gray-500">取消</button>
                <button onClick={createCollab} disabled={!newCollab.title} className="px-4 py-1.5 text-sm bg-primary-600 text-white rounded-lg hover:bg-primary-700 disabled:opacity-50">创建</button>
              </div>
            </div>
          )}
          <div className="space-y-2">
            {collabs.map((c: any) => (
              <div key={c.id} className="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-4 flex items-center justify-between">
                <div>
                  <div className="flex items-center gap-2">
                    <Users className="w-4 h-4 text-primary-500" />
                    <p className="font-medium text-gray-900 dark:text-white">{c.title}</p>
                    <span className={`px-2 py-0.5 text-xs rounded ${c.status === 'active' ? 'bg-blue-100 text-blue-700' : c.status === 'completed' ? 'bg-green-100 text-green-700' : 'bg-gray-100 text-gray-600'}`}>{c.status}</span>
                  </div>
                  <p className="text-xs text-gray-500 mt-0.5">协议: {c.protocol} · 最大 {c.max_agents} 个 Agent</p>
                </div>
              </div>
            ))}
            {collabs.length === 0 && <p className="text-center text-gray-400 py-8">暂无协作会话</p>}
          </div>
        </div>
      )}
    </div>
  )
}
