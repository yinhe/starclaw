import { useState, useEffect, useCallback } from 'react'
import { Bot, Plus, Zap, Target, XCircle, ChevronRight, Code, Megaphone, Headphones, BarChart3, Loader2, Users, TrendingUp, ShoppingCart, Film, Crosshair, Shield } from 'lucide-react'
import { broodAPI, TeamAgentTemplate, TeamInstance, TeamMission, TeamAgentStats, ClawNode } from '../api/brood'
import { useTeamAgentWS } from '../hooks/useTeamAgentWS'

const statusColors: Record<string, string> = {
  forming: 'bg-yellow-500/15 text-yellow-400',
  ready: 'bg-blue-500/15 text-blue-400',
  running: 'bg-green-500/15 text-green-400',
  paused: 'bg-orange-500/15 text-orange-400',
  maintenance: 'bg-purple-500/15 text-purple-400',
  completed: 'bg-gray-500/15 text-gray-400',
  disbanded: 'bg-red-500/15 text-red-400',
}

const statusLabels: Record<string, string> = {
  forming: '组建中',
  ready: '就绪',
  running: '运行中',
  paused: '已暂停',
  maintenance: '维护中',
  completed: '已完成',
  disbanded: '已解散',
}

const missionStatusLabels: Record<string, string> = {
  planning: '规划中',
  confirming: '确认中',
  executing: '执行中',
  reviewing: '审查中',
  completed: '已完成',
  failed: '失败',
  cancelled: '已取消',
}

const categoryIcons: Record<string, typeof Code> = {
  development: Code,
  marketing: Megaphone,
  support: Headphones,
  data: BarChart3,
  finance: TrendingUp,
  ecommerce: ShoppingCart,
  content: Film,
  sales: Crosshair,
  ops: Shield,
}

type View = 'overview' | 'templates' | 'detail'

export default function TeamAgentPage() {
  const [view, setView] = useState<View>('overview')
  const [stats, setStats] = useState<TeamAgentStats | null>(null)
  const [templates, setTemplates] = useState<TeamAgentTemplate[]>([])
  const [instances, setInstances] = useState<TeamInstance[]>([])
  const [selectedInstance, setSelectedInstance] = useState<TeamInstance | null>(null)
  const [missions, setMissions] = useState<TeamMission[]>([])
  const [loading, setLoading] = useState(true)

  // Create modal
  const [showCreate, setShowCreate] = useState(false)
  const [createTmplId, setCreateTmplId] = useState('')
  const [createName, setCreateName] = useState('')
  const [createGoal, setCreateGoal] = useState('')
  const [createNodeId, setCreateNodeId] = useState('')
  const [createBudget, setCreateBudget] = useState(5000)
  const [nodes, setNodes] = useState<ClawNode[]>([])
  const [creating, setCreating] = useState(false)

  // New mission modal
  const [showMission, setShowMission] = useState(false)
  const [missionGoal, setMissionGoal] = useState('')
  const [creatingMission, setCreatingMission] = useState(false)

  // Real-time WS updates — refresh missions when status changes
  const onMissionUpdate = useCallback((data: { mission_id?: string; instance_id?: string; status?: string }) => {
    if (selectedInstance && data.instance_id === selectedInstance.id) {
      broodAPI.listTeamMissions(selectedInstance.id).then(r => setMissions(r.missions || [])).catch(() => {})
    }
    // Refresh stats on status change
    if (data.status === 'completed' || data.status === 'failed') {
      broodAPI.teamAgentStats().then(setStats).catch(() => {})
    }
  }, [selectedInstance])
  useTeamAgentWS(undefined, onMissionUpdate)

  useEffect(() => { load() }, [])

  async function load() {
    setLoading(true)
    try {
      const [s, t, i] = await Promise.all([
        broodAPI.teamAgentStats(),
        broodAPI.listTeamTemplates(),
        broodAPI.listTeamInstances(),
      ])
      setStats(s)
      setTemplates(t.templates || [])
      setInstances(i.instances || [])
    } catch { /* ignore */ }
    setLoading(false)
  }

  async function openCreate(tmplId: string) {
    setCreateTmplId(tmplId)
    setCreateName('')
    setCreateGoal('')
    setCreateNodeId('')
    setCreateBudget(5000)
    setShowCreate(true)
    try {
      const res = await broodAPI.listClaws({ status: 'online' })
      setNodes(res.claws || [])
      if (res.claws?.length) setCreateNodeId(res.claws[0].id)
    } catch { /* ignore */ }
  }

  async function handleCreate() {
    if (!createTmplId || !createNodeId || !createName) return
    setCreating(true)
    try {
      await broodAPI.createTeamInstance({
        template_id: createTmplId,
        claw_node_id: createNodeId,
        name: createName,
        goal: createGoal,
        energy_budget: createBudget,
      })
      setShowCreate(false)
      load()
    } catch { /* ignore */ }
    setCreating(false)
  }

  async function selectInstance(inst: TeamInstance) {
    setSelectedInstance(inst)
    setView('detail')
    try {
      const res = await broodAPI.listTeamMissions(inst.id)
      setMissions(res.missions || [])
    } catch { /* ignore */ }
  }

  async function handleDisband(id: string) {
    if (!confirm('确定解散此团队？')) return
    try {
      await broodAPI.disbandTeamInstance(id)
      setView('overview')
      load()
    } catch { /* ignore */ }
  }

  async function handleCreateMission() {
    if (!selectedInstance || !missionGoal.trim()) return
    setCreatingMission(true)
    try {
      await broodAPI.createTeamMission(selectedInstance.id, { goal: missionGoal })
      setShowMission(false)
      setMissionGoal('')
      const res = await broodAPI.listTeamMissions(selectedInstance.id)
      setMissions(res.missions || [])
      load()
    } catch { /* ignore */ }
    setCreatingMission(false)
  }

  function parseRoles(rolesJSON: string): { code: string; name: string; max_instances: number }[] {
    try { return JSON.parse(rolesJSON) } catch { return [] }
  }

  if (loading) return <div className="flex items-center justify-center h-full"><Loader2 className="w-6 h-6 animate-spin text-overlord-400" /></div>

  // ─── Detail View ───
  if (view === 'detail' && selectedInstance) {
    const inst = selectedInstance
    const tmpl = templates.find(t => t.id === inst.template_id)
    const roles = tmpl ? parseRoles(tmpl.roles) : []
    const progress = inst.energy_budget > 0 ? Math.min(100, Math.round(inst.energy_used / inst.energy_budget * 100)) : 0

    return (
      <div className="p-6 space-y-6">
        {/* Header */}
        <div className="flex items-center gap-3">
          <button onClick={() => { setView('overview'); setSelectedInstance(null) }} className="text-gray-400 hover:text-white transition text-sm">← 返回</button>
          <div className="flex-1" />
          <button onClick={() => setShowMission(true)} className="flex items-center gap-2 px-4 py-2 bg-overlord-600 hover:bg-overlord-500 text-white rounded-lg text-sm transition">
            <Plus className="w-4 h-4" /> 新建任务
          </button>
          {inst.status !== 'disbanded' && (
            <button onClick={() => handleDisband(inst.id)} className="flex items-center gap-2 px-4 py-2 bg-red-600/20 hover:bg-red-600/30 text-red-400 rounded-lg text-sm transition">
              <XCircle className="w-4 h-4" /> 解散
            </button>
          )}
        </div>

        {/* Team info card */}
        <div className="bg-gray-800/50 border border-gray-700/50 rounded-xl p-6">
          <div className="flex items-start gap-4">
            <div className="w-12 h-12 rounded-xl bg-overlord-600/20 flex items-center justify-center shrink-0">
              <Bot className="w-6 h-6 text-overlord-400" />
            </div>
            <div className="flex-1 min-w-0">
              <div className="flex items-center gap-3">
                <h2 className="text-xl font-bold text-white">{inst.name}</h2>
                <span className={`px-2 py-0.5 rounded text-xs font-medium ${statusColors[inst.status] || 'bg-gray-600 text-gray-300'}`}>
                  {statusLabels[inst.status] || inst.status}
                </span>
              </div>
              <div className="text-sm text-gray-400 mt-1">{inst.template_name} · {inst.goal || '无目标描述'}</div>
              <div className="text-xs text-gray-500 mt-1">创建于 {new Date(inst.created_at).toLocaleString()}</div>
            </div>
          </div>

          {/* Stats row */}
          <div className="grid grid-cols-4 gap-4 mt-6">
            <div className="bg-gray-900/50 rounded-lg p-3 text-center">
              <div className="text-2xl font-bold text-white">{inst.mission_count}</div>
              <div className="text-xs text-gray-500">任务数</div>
            </div>
            <div className="bg-gray-900/50 rounded-lg p-3 text-center">
              <div className="text-2xl font-bold text-overlord-400">{inst.avg_score > 0 ? inst.avg_score.toFixed(1) : '-'}</div>
              <div className="text-xs text-gray-500">平均评分</div>
            </div>
            <div className="bg-gray-900/50 rounded-lg p-3 text-center">
              <div className="text-2xl font-bold text-yellow-400">{inst.energy_used}⚡</div>
              <div className="text-xs text-gray-500">已消耗星能</div>
            </div>
            <div className="bg-gray-900/50 rounded-lg p-3 text-center">
              <div className="text-2xl font-bold text-white">{inst.energy_budget}⚡</div>
              <div className="text-xs text-gray-500">星能预算</div>
            </div>
          </div>

          {/* Energy progress bar */}
          {inst.energy_budget > 0 && (
            <div className="mt-4">
              <div className="flex justify-between text-xs text-gray-500 mb-1">
                <span>星能消耗</span>
                <span>{progress}%</span>
              </div>
              <div className="h-2 bg-gray-700 rounded-full overflow-hidden">
                <div className={`h-full rounded-full transition-all ${progress > 80 ? 'bg-red-500' : progress > 60 ? 'bg-yellow-500' : 'bg-overlord-500'}`} style={{ width: `${progress}%` }} />
              </div>
            </div>
          )}
        </div>

        {/* Roles */}
        {roles.length > 0 && (
          <div>
            <h3 className="text-sm font-semibold text-gray-300 mb-3">团队角色</h3>
            <div className="flex flex-wrap gap-2">
              {roles.map(r => (
                <div key={r.code} className="flex items-center gap-2 bg-gray-800/50 border border-gray-700/50 rounded-lg px-3 py-2">
                  <Users className="w-3.5 h-3.5 text-overlord-400" />
                  <span className="text-sm text-white">{r.name}</span>
                  {r.max_instances > 1 && <span className="text-xs text-gray-500">×{r.max_instances}</span>}
                </div>
              ))}
            </div>
          </div>
        )}

        {/* Missions */}
        <div>
          <h3 className="text-sm font-semibold text-gray-300 mb-3">任务列表</h3>
          {missions.length === 0 ? (
            <div className="bg-gray-800/30 border border-gray-700/30 rounded-xl p-8 text-center">
              <Target className="w-8 h-8 text-gray-600 mx-auto mb-2" />
              <div className="text-sm text-gray-500">暂无任务，点击「新建任务」开始</div>
            </div>
          ) : (
            <div className="space-y-2">
              {missions.map(m => (
                <div key={m.id} className="bg-gray-800/50 border border-gray-700/50 rounded-lg p-4 flex items-center gap-4">
                  <Target className="w-5 h-5 text-overlord-400 shrink-0" />
                  <div className="flex-1 min-w-0">
                    <div className="text-sm font-medium text-white truncate">{m.title}</div>
                    <div className="text-xs text-gray-500 mt-0.5">
                      {missionStatusLabels[m.status] || m.status}
                      {m.total_steps > 0 && ` · ${m.done_steps}/${m.total_steps} 步骤`}
                      {m.review_score > 0 && ` · 评分 ${m.review_score.toFixed(1)}`}
                      {m.energy_used > 0 && ` · ${m.energy_used}⚡`}
                    </div>
                  </div>
                  {m.preview_url && (
                    <a href={m.preview_url} target="_blank" rel="noopener noreferrer" className="text-xs text-overlord-400 hover:text-overlord-300">预览</a>
                  )}
                </div>
              ))}
            </div>
          )}
        </div>

        {/* New Mission Modal */}
        {showMission && (
          <div className="fixed inset-0 bg-black/60 flex items-center justify-center z-50" onClick={() => setShowMission(false)}>
            <div className="bg-gray-800 border border-gray-700 rounded-xl w-full max-w-lg p-6" onClick={e => e.stopPropagation()}>
              <h3 className="text-lg font-bold text-white mb-4">新建任务</h3>
              <textarea
                className="w-full bg-gray-900 border border-gray-700 rounded-lg p-3 text-sm text-white placeholder-gray-500 focus:border-overlord-500 focus:outline-none"
                rows={4}
                placeholder="描述你希望 AI 团队完成的任务..."
                value={missionGoal}
                onChange={e => setMissionGoal(e.target.value)}
              />
              <div className="flex justify-end gap-3 mt-4">
                <button onClick={() => setShowMission(false)} className="px-4 py-2 text-sm text-gray-400 hover:text-white transition">取消</button>
                <button onClick={handleCreateMission} disabled={creatingMission || !missionGoal.trim()} className="px-4 py-2 bg-overlord-600 hover:bg-overlord-500 text-white rounded-lg text-sm transition disabled:opacity-50">
                  {creatingMission ? '创建中...' : '创建任务'}
                </button>
              </div>
            </div>
          </div>
        )}
      </div>
    )
  }

  // ─── Overview ───
  return (
    <div className="p-6 space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-xl font-bold text-white flex items-center gap-2">
            <Bot className="w-5 h-5 text-overlord-400" />
            AI 团队智能体
          </h1>
          <p className="text-sm text-gray-500 mt-1">给你的企业雇一支永远在线的 AI 团队</p>
        </div>
      </div>

      {/* Stats cards */}
      {stats && (
        <div className="grid grid-cols-5 gap-4">
          {[
            { label: '模板数', value: stats.template_count, color: 'text-overlord-400' },
            { label: '总实例', value: stats.total_instances, color: 'text-blue-400' },
            { label: '活跃实例', value: stats.active_instances, color: 'text-green-400' },
            { label: '总任务', value: stats.total_missions, color: 'text-purple-400' },
            { label: '总星能', value: `${stats.total_energy}⚡`, color: 'text-yellow-400' },
          ].map(({ label, value, color }) => (
            <div key={label} className="bg-gray-800/50 border border-gray-700/50 rounded-xl p-4 text-center">
              <div className={`text-2xl font-bold ${color}`}>{value}</div>
              <div className="text-xs text-gray-500 mt-1">{label}</div>
            </div>
          ))}
        </div>
      )}

      {/* Templates */}
      <div>
        <h2 className="text-sm font-semibold text-gray-300 mb-3">官方模板</h2>
        <div className="grid grid-cols-3 gap-4">
          {templates.map(t => {
            const Icon = categoryIcons[t.category] || Bot
            const roles = parseRoles(t.roles)
            return (
              <div key={t.id} className="bg-gray-800/50 border border-gray-700/50 rounded-xl p-5 hover:border-overlord-600/50 transition group">
                <div className="flex items-start gap-3">
                  <div className="w-10 h-10 rounded-lg bg-overlord-600/15 flex items-center justify-center shrink-0">
                    <Icon className="w-5 h-5 text-overlord-400" />
                  </div>
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2">
                      <span className="text-sm font-bold text-white">{t.name}</span>
                      {t.is_official && <span className="text-[10px] bg-overlord-600/20 text-overlord-400 px-1.5 py-0.5 rounded">官方</span>}
                      <span className="text-[10px] text-gray-600">{t.version}</span>
                    </div>
                    <div className="text-xs text-gray-400 mt-1 line-clamp-2">{t.description}</div>
                    <div className="flex flex-wrap gap-1 mt-2">
                      {roles.map(r => (
                        <span key={r.code} className="text-[10px] bg-gray-700/50 text-gray-400 px-1.5 py-0.5 rounded">
                          {r.name}{r.max_instances > 1 ? `×${r.max_instances}` : ''}
                        </span>
                      ))}
                    </div>
                  </div>
                  <button
                    onClick={() => openCreate(t.id)}
                    className="opacity-0 group-hover:opacity-100 transition px-3 py-1.5 bg-overlord-600 hover:bg-overlord-500 text-white rounded-lg text-xs flex items-center gap-1 shrink-0"
                  >
                    <Plus className="w-3 h-3" /> 创建
                  </button>
                </div>
              </div>
            )
          })}
        </div>
      </div>

      {/* Instances */}
      <div>
        <h2 className="text-sm font-semibold text-gray-300 mb-3">团队实例 ({instances.length})</h2>
        {instances.length === 0 ? (
          <div className="bg-gray-800/30 border border-gray-700/30 rounded-xl p-12 text-center">
            <Bot className="w-10 h-10 text-gray-600 mx-auto mb-3" />
            <div className="text-sm text-gray-500">暂无团队实例</div>
            <div className="text-xs text-gray-600 mt-1">选择上方模板创建你的第一支 AI 团队</div>
          </div>
        ) : (
          <div className="space-y-2">
            {instances.map(inst => (
              <div
                key={inst.id}
                onClick={() => selectInstance(inst)}
                className="bg-gray-800/50 border border-gray-700/50 rounded-xl p-4 hover:border-overlord-600/40 transition cursor-pointer flex items-center gap-4"
              >
                <div className="w-10 h-10 rounded-lg bg-overlord-600/15 flex items-center justify-center shrink-0">
                  <Bot className="w-5 h-5 text-overlord-400" />
                </div>
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-2">
                    <span className="text-sm font-medium text-white">{inst.name}</span>
                    <span className={`px-1.5 py-0.5 rounded text-[10px] font-medium ${statusColors[inst.status] || 'bg-gray-600 text-gray-300'}`}>
                      {statusLabels[inst.status] || inst.status}
                    </span>
                    <span className="text-[10px] text-gray-600">{inst.template_name}</span>
                  </div>
                  <div className="text-xs text-gray-500 mt-0.5 truncate">
                    {inst.goal || '无目标描述'}
                    {inst.mission_count > 0 && ` · ${inst.mission_count} 个任务`}
                    {inst.energy_used > 0 && ` · ${inst.energy_used}⚡`}
                  </div>
                </div>
                <ChevronRight className="w-4 h-4 text-gray-600" />
              </div>
            ))}
          </div>
        )}
      </div>

      {/* Create Modal */}
      {showCreate && (
        <div className="fixed inset-0 bg-black/60 flex items-center justify-center z-50" onClick={() => setShowCreate(false)}>
          <div className="bg-gray-800 border border-gray-700 rounded-xl w-full max-w-lg p-6" onClick={e => e.stopPropagation()}>
            <h3 className="text-lg font-bold text-white mb-4">创建 AI 团队</h3>
            <div className="space-y-4">
              <div>
                <label className="text-xs text-gray-400 mb-1 block">团队名称</label>
                <input className="w-full bg-gray-900 border border-gray-700 rounded-lg px-3 py-2 text-sm text-white placeholder-gray-500 focus:border-overlord-500 focus:outline-none" placeholder="如: DevClaw-电商项目" value={createName} onChange={e => setCreateName(e.target.value)} />
              </div>
              <div>
                <label className="text-xs text-gray-400 mb-1 block">目标描述</label>
                <textarea className="w-full bg-gray-900 border border-gray-700 rounded-lg px-3 py-2 text-sm text-white placeholder-gray-500 focus:border-overlord-500 focus:outline-none" rows={3} placeholder="描述这支 AI 团队要做什么..." value={createGoal} onChange={e => setCreateGoal(e.target.value)} />
              </div>
              <div>
                <label className="text-xs text-gray-400 mb-1 block">部署节点</label>
                <select className="w-full bg-gray-900 border border-gray-700 rounded-lg px-3 py-2 text-sm text-white focus:border-overlord-500 focus:outline-none" value={createNodeId} onChange={e => setCreateNodeId(e.target.value)}>
                  {nodes.length === 0 && <option value="">无在线节点</option>}
                  {nodes.map(n => <option key={n.id} value={n.id}>{n.name || n.claw_id} ({n.status})</option>)}
                </select>
              </div>
              <div>
                <label className="text-xs text-gray-400 mb-1 block">星能预算</label>
                <div className="flex items-center gap-2">
                  <input type="number" className="w-32 bg-gray-900 border border-gray-700 rounded-lg px-3 py-2 text-sm text-white focus:border-overlord-500 focus:outline-none" value={createBudget} onChange={e => setCreateBudget(Number(e.target.value))} min={0} step={1000} />
                  <Zap className="w-4 h-4 text-yellow-400" />
                  <span className="text-xs text-gray-500">0 = 不限</span>
                </div>
              </div>
            </div>
            <div className="flex justify-end gap-3 mt-6">
              <button onClick={() => setShowCreate(false)} className="px-4 py-2 text-sm text-gray-400 hover:text-white transition">取消</button>
              <button onClick={handleCreate} disabled={creating || !createName || !createNodeId} className="px-4 py-2 bg-overlord-600 hover:bg-overlord-500 text-white rounded-lg text-sm transition disabled:opacity-50">
                {creating ? '创建中...' : '创建团队'}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
