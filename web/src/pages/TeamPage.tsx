import { useState, useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { Plus, Trash2, Users, Crown, ChevronRight, Loader2, Zap, FolderSync, RefreshCw } from 'lucide-react'
import { teamAPI, agentAPI, workflowAPI } from '../lib/api'
import { formatAgentDisplayName } from '../lib/agentDisplay'

interface Agent { id: string; name: string; description: string }
interface TeamMember { id: string; team_id: string; agent_id: string; role: string; specialty: string; order: number; agent?: Agent }
interface Team { id: string; name: string; description: string; icon: string; coordinator_id: string; topology: string; status: string; template_id: string; members: TeamMember[]; created_at: string }
interface Template { id: string; name: string; description: string; icon: string; topology: string; roles: { specialty: string; role: string; agent_hint: string }[] }

export default function TeamPage() {
  const [teams, setTeams] = useState<Team[]>([])
  const [templates, setTemplates] = useState<Template[]>([])
  const [agents, setAgents] = useState<Agent[]>([])
  const [loading, setLoading] = useState(true)
  const [showCreate, setShowCreate] = useState(false)
  const [creating, setCreating] = useState(false)

  // Create form
  const [form, setForm] = useState({ name: '', description: '', coordinator_id: '', topology: 'sequential', template_id: '' })
  const [selectedMembers, setSelectedMembers] = useState<{ agent_id: string; specialty: string }[]>([])
  // Sync project
  const [showSyncModal, setShowSyncModal] = useState<string | null>(null) // team id
  const [projects, setProjects] = useState<{ name: string; has_bible: boolean; has_drama: boolean }[]>([])
  const [syncing, setSyncing] = useState(false)
  const [syncStats, setSyncStats] = useState<Record<string, number> | null>(null)
  const navigate = useNavigate()

  useEffect(() => { load() }, [])

  const load = async () => {
    setLoading(true)
    try {
      const [tRes, tmplRes, aRes] = await Promise.all([
        teamAPI.list(),
        teamAPI.templates(),
        agentAPI.list(),
      ])
      setTeams(tRes.data.teams || [])
      setTemplates(tmplRes.data.templates || [])
      setAgents(aRes.data.agents || [])
    } catch {}
    setLoading(false)
  }

  const handleCreate = async () => {
    if (!form.name || !form.coordinator_id) return
    setCreating(true)
    try {
      await teamAPI.create({
        name: form.name,
        description: form.description,
        coordinator_id: form.coordinator_id,
        topology: form.topology,
        template_id: form.template_id,
        members: selectedMembers.map((m, i) => ({ ...m, order: i + 1 })),
      })
      setShowCreate(false)
      setForm({ name: '', description: '', coordinator_id: '', topology: 'sequential', template_id: '' })
      setSelectedMembers([])
      load()
    } catch {}
    setCreating(false)
  }

  const handleDelete = async (id: string) => {
    if (!confirm('确定删除此团队？')) return
    try { await teamAPI.delete(id); load() } catch {}
  }

  const applyTemplate = (tmpl: Template) => {
    setForm({ ...form, name: tmpl.name, description: tmpl.description, topology: tmpl.topology, template_id: tmpl.id })
    // Auto-match agents to roles
    const coordRole = tmpl.roles.find(r => r.role === 'coordinator')
    const memberRoles = tmpl.roles.filter(r => r.role === 'member')

    if (coordRole) {
      const match = agents.find(a => a.name.includes(coordRole.agent_hint))
      if (match) setForm(f => ({ ...f, coordinator_id: match.id }))
    }

    const members: { agent_id: string; specialty: string }[] = []
    for (const role of memberRoles) {
      const match = agents.find(a => a.name.includes(role.agent_hint) && !members.some(m => m.agent_id === a.id))
      if (match) members.push({ agent_id: match.id, specialty: role.specialty })
    }
    setSelectedMembers(members)
    setShowCreate(true)
  }

  const loadProjects = async () => {
    try {
      const res = await workflowAPI.listProjects()
      setProjects(res.data.projects || [])
    } catch { /* */ }
  }

  const handleSyncProject = async (projectName: string) => {
    setSyncing(true)
    setSyncStats(null)
    try {
      const res = await workflowAPI.syncProject({ project_name: projectName })
      setSyncStats(res.data.stats)
      if (res.data.workflow?.id) {
        setTimeout(() => {
          setShowSyncModal(null)
          navigate(`/workflows/editor?id=${res.data.workflow.id}`)
        }, 1500)
      }
    } catch { alert('同步失败') }
    setSyncing(false)
  }

  const topologyLabel: Record<string, string> = {
    sequential: '顺序执行',
    parallel: '并行执行',
    round_robin: '轮询',
    free: '自由讨论',
  }

  if (loading) return <div className="flex items-center justify-center h-full"><Loader2 className="w-6 h-6 animate-spin text-gray-400" /></div>

  return (
    <div className="h-full overflow-y-auto p-6 max-w-5xl mx-auto space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-gray-900 dark:text-white flex items-center gap-2">
            <Users className="w-6 h-6 text-primary-500" /> 团队
          </h1>
          <p className="text-sm text-gray-500 mt-1">多个智能体协作，组成团队完成复杂任务</p>
        </div>
        <button onClick={() => setShowCreate(true)} className="flex items-center gap-1.5 px-4 py-2 bg-primary-600 text-white rounded-lg text-sm font-medium hover:bg-primary-700 transition">
          <Plus className="w-4 h-4" /> 创建团队
        </button>
      </div>

      {/* Templates */}
      {teams.length === 0 && (
        <div>
          <h3 className="text-sm font-semibold text-gray-500 dark:text-gray-400 mb-3">从模板快速创建</h3>
          <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
            {templates.map(t => (
              <button
                key={t.id}
                onClick={() => applyTemplate(t)}
                className="text-left p-4 rounded-xl border border-gray-200 dark:border-gray-700 hover:border-primary-300 dark:hover:border-primary-700 hover:shadow-md transition group"
              >
                <div className="text-lg mb-1">{t.icon === 'Code2' ? '💻' : t.icon === 'PenTool' ? '✍️' : t.icon === 'TrendingUp' ? '📈' : '🎬'}</div>
                <div className="font-medium text-sm text-gray-800 dark:text-gray-200 group-hover:text-primary-600">{t.name}</div>
                <div className="text-[11px] text-gray-400 mt-0.5">{t.description}</div>
                <div className="text-[10px] text-gray-400 mt-1">{t.roles.length} 个角色 · {topologyLabel[t.topology]}</div>
              </button>
            ))}
          </div>
        </div>
      )}

      {/* Team List */}
      {teams.length > 0 && (
        <div className="space-y-3">
          {teams.map(team => (
            <div key={team.id} className="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-4 hover:shadow-md transition">
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-3">
                  <div className="w-10 h-10 rounded-xl bg-primary-100 dark:bg-primary-900/30 flex items-center justify-center">
                    <Users className="w-5 h-5 text-primary-600" />
                  </div>
                  <div>
                    <h3 className="font-semibold text-gray-900 dark:text-white cursor-pointer hover:text-primary-600 transition-colors" onClick={() => navigate(`/teams/${team.id}`)}>{team.name}</h3>
                    <p className="text-xs text-gray-400">{team.description || topologyLabel[team.topology] || team.topology} · {team.members?.length || 0} 成员</p>
                  </div>
                </div>
                <div className="flex items-center gap-2">
                  <span className={`text-[10px] px-2 py-0.5 rounded-full ${team.status === 'active' ? 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400' : 'bg-gray-100 text-gray-500'}`}>
                    {team.status === 'active' ? '运行中' : '已暂停'}
                  </span>
                  <button onClick={() => handleDelete(team.id)} className="p-1.5 text-gray-400 hover:text-red-500 transition"><Trash2 className="w-4 h-4" /></button>
                </div>
              </div>
              {/* Members */}
              {team.members && team.members.length > 0 && (
                <div className="mt-3 flex flex-wrap gap-2">
                  {team.members.map(m => (
                    <div key={m.id} onClick={() => navigate(`/agents/${m.agent_id}`)}
                      className="flex items-center gap-1.5 px-2.5 py-1 bg-gray-50 dark:bg-gray-750 rounded-lg text-xs cursor-pointer hover:bg-primary-50 dark:hover:bg-primary-900/20 hover:border-primary-200 transition-colors">
                      {m.role === 'coordinator' && <Crown className="w-3 h-3 text-amber-500" />}
                      <span className="font-medium text-gray-700 dark:text-gray-300">{formatAgentDisplayName(m.agent?.name || '未知')}</span>
                      {m.specialty && <span className="text-gray-400">· {m.specialty}</span>}
                    </div>
                  ))}
                </div>
              )}
              <div className="mt-3 flex items-center gap-3">
                <a href={`/chat?team=${team.id}`} className="flex items-center gap-1 text-xs text-primary-600 hover:text-primary-700 font-medium">
                  <Zap className="w-3 h-3" /> 开始对话 <ChevronRight className="w-3 h-3" />
                </a>
                {(team.name.includes('短剧') || team.name.includes('影视') || ['short-drama', 'video'].includes(team.template_id || '')) && (
                  <button
                    onClick={() => { setShowSyncModal(team.id); loadProjects() }}
                    className="flex items-center gap-1 text-xs text-violet-600 hover:text-violet-700 font-medium"
                  >
                    <FolderSync className="w-3 h-3" /> 同步剧本项目 <ChevronRight className="w-3 h-3" />
                  </button>
                )}
              </div>
            </div>
          ))}
        </div>
      )}

      {teams.length === 0 && templates.length > 0 && (
        <div className="text-center py-8 text-gray-400 text-sm">
          还没有团队，从上方模板快速创建，或点击右上角自定义创建
        </div>
      )}

      {/* Sync Project Modal */}
      {showSyncModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 backdrop-blur-sm" onClick={() => setShowSyncModal(null)}>
          <div className="bg-white dark:bg-gray-800 rounded-2xl shadow-2xl w-[420px] max-h-[80vh] overflow-y-auto" onClick={e => e.stopPropagation()}>
            <div className="p-6 border-b dark:border-gray-700">
              <h2 className="text-lg font-bold text-gray-900 dark:text-white">同步剧本项目</h2>
              <p className="text-sm text-gray-500 mt-1">扫描项目目录，自动生成生产看板工作流</p>
            </div>
            <div className="p-6 space-y-3">
              {projects.length === 0 ? (
                <div className="text-center py-8 text-gray-400">
                  <RefreshCw className="w-8 h-8 mx-auto mb-2 animate-spin opacity-50" />
                  <p className="text-sm">扫描项目目录中...</p>
                </div>
              ) : (
                projects.map(p => (
                  <button key={p.name}
                    onClick={() => handleSyncProject(p.name)}
                    disabled={syncing}
                    className="w-full flex items-center justify-between p-4 rounded-xl border dark:border-gray-700 hover:border-violet-300 hover:bg-violet-50 dark:hover:bg-violet-900/20 transition-colors disabled:opacity-50"
                  >
                    <div className="text-left">
                      <div className="font-semibold text-gray-900 dark:text-gray-100">{p.name}</div>
                      <div className="text-xs text-gray-500 mt-0.5 flex gap-2">
                        {p.has_bible && <span className="text-violet-500">bible.md</span>}
                        {p.has_drama && <span className="text-cyan-500">drama/</span>}
                      </div>
                    </div>
                    {syncing ? (
                      <Loader2 className="w-5 h-5 text-violet-500 animate-spin" />
                    ) : (
                      <FolderSync className="w-5 h-5 text-gray-400" />
                    )}
                  </button>
                ))
              )}
              {syncStats && (
                <div className="mt-4 p-3 bg-green-50 dark:bg-green-900/20 rounded-lg text-sm text-green-700 dark:text-green-300">
                  ✓ 同步完成：{syncStats.characters} 角色 · {syncStats.episodes} 剧集 · {syncStats.images} 张图 · {syncStats.clips} 片段
                </div>
              )}
            </div>
          </div>
        </div>
      )}

      {/* Create Modal */}
      {showCreate && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 backdrop-blur-sm" onClick={() => setShowCreate(false)}>
          <div className="bg-white dark:bg-gray-800 rounded-2xl shadow-2xl p-6 w-full max-w-md mx-4" onClick={e => e.stopPropagation()}>
            <h3 className="text-lg font-bold text-gray-900 dark:text-white mb-4">创建团队</h3>
            <div className="space-y-3">
              <div>
                <label className="block text-xs font-medium text-gray-600 dark:text-gray-400 mb-1">团队名称</label>
                <input value={form.name} onChange={e => setForm({ ...form, name: e.target.value })} placeholder="例：开发团队"
                  className="w-full px-3 py-2 text-sm border border-gray-200 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white" />
              </div>
              <div>
                <label className="block text-xs font-medium text-gray-600 dark:text-gray-400 mb-1">描述</label>
                <input value={form.description} onChange={e => setForm({ ...form, description: e.target.value })} placeholder="团队目标与分工"
                  className="w-full px-3 py-2 text-sm border border-gray-200 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white" />
              </div>
              <div>
                <label className="block text-xs font-medium text-gray-600 dark:text-gray-400 mb-1">团长（协调 Agent）</label>
                <select value={form.coordinator_id} onChange={e => setForm({ ...form, coordinator_id: e.target.value })}
                  className="w-full px-3 py-2 text-sm border border-gray-200 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white">
                  <option value="">选择团长...</option>
                  {agents.map(a => <option key={a.id} value={a.id}>{formatAgentDisplayName(a.name)}</option>)}
                </select>
              </div>
              <div>
                <label className="block text-xs font-medium text-gray-600 dark:text-gray-400 mb-1">执行模式</label>
                <select value={form.topology} onChange={e => setForm({ ...form, topology: e.target.value })}
                  className="w-full px-3 py-2 text-sm border border-gray-200 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white">
                  <option value="sequential">顺序执行（团长分派→成员依次执行）</option>
                  <option value="parallel">并行执行（成员同时执行）</option>
                  <option value="round_robin">轮询（成员轮流发言）</option>
                </select>
              </div>

              {/* Members */}
              <div>
                <label className="block text-xs font-medium text-gray-600 dark:text-gray-400 mb-1">团队成员</label>
                {selectedMembers.map((m, i) => (
                  <div key={i} className="flex items-center gap-2 mb-1.5">
                    <select value={m.agent_id} onChange={e => { const v = [...selectedMembers]; v[i].agent_id = e.target.value; setSelectedMembers(v) }}
                      className="flex-1 px-2 py-1.5 text-xs border border-gray-200 dark:border-gray-600 rounded bg-white dark:bg-gray-700 text-gray-900 dark:text-white">
                      <option value="">选择 Agent...</option>
                      {agents.map(a => <option key={a.id} value={a.id}>{formatAgentDisplayName(a.name)}</option>)}
                    </select>
                    <input value={m.specialty} onChange={e => { const v = [...selectedMembers]; v[i].specialty = e.target.value; setSelectedMembers(v) }}
                      placeholder="角色特长" className="w-28 px-2 py-1.5 text-xs border border-gray-200 dark:border-gray-600 rounded bg-white dark:bg-gray-700 text-gray-900 dark:text-white" />
                    <button onClick={() => setSelectedMembers(selectedMembers.filter((_, j) => j !== i))} className="text-gray-400 hover:text-red-500"><Trash2 className="w-3 h-3" /></button>
                  </div>
                ))}
                <button onClick={() => setSelectedMembers([...selectedMembers, { agent_id: '', specialty: '' }])}
                  className="text-xs text-primary-600 hover:text-primary-700 font-medium mt-1">+ 添加成员</button>
              </div>
            </div>

            <div className="flex gap-3 mt-5">
              <button onClick={() => setShowCreate(false)} className="flex-1 py-2 rounded-lg border border-gray-200 dark:border-gray-600 text-sm text-gray-600 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-700">取消</button>
              <button onClick={handleCreate} disabled={creating || !form.name || !form.coordinator_id}
                className="flex-1 py-2 rounded-lg bg-primary-600 text-white text-sm font-medium hover:bg-primary-700 disabled:opacity-50">
                {creating ? '创建中...' : '创建团队'}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
