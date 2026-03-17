import { useState, useEffect } from 'react'
import { squadAPI } from '../lib/api'
import { Users, Plus, Play, XCircle, ChevronDown, ChevronRight, Loader2, Swords, Target, UserPlus, Trash2, Clock, CheckCircle2, AlertCircle, Zap } from 'lucide-react'

interface Squad {
  id: string
  name: string
  description: string
  captain_node: string
  status: string
  max_members: number
  tags: string
  created_at: string
}

interface SquadMember {
  id: string
  squad_id: string
  node_id: string
  role: string
  specialty: string
  status: string
  joined_at: string
}

interface Mission {
  id: string
  squad_id: string
  title: string
  goal: string
  status: string
  total_steps: number
  done_steps: number
  created_at: string
  completed_at: string | null
}

interface MissionStep {
  id: string
  mission_id: string
  target_node: string
  target_agent: string
  task: string
  output: string
  status: string
  error_msg: string
  sequence: number
}

const statusColors: Record<string, string> = {
  forming: 'bg-yellow-500/10 text-yellow-400 border-yellow-500/20',
  active: 'bg-green-500/10 text-green-400 border-green-500/20',
  disbanded: 'bg-gray-500/10 text-gray-400 border-gray-500/20',
  planning: 'bg-blue-500/10 text-blue-400 border-blue-500/20',
  executing: 'bg-amber-500/10 text-amber-400 border-amber-500/20',
  reviewing: 'bg-purple-500/10 text-purple-400 border-purple-500/20',
  completed: 'bg-green-500/10 text-green-400 border-green-500/20',
  failed: 'bg-red-500/10 text-red-400 border-red-500/20',
  pending: 'bg-gray-500/10 text-gray-400 border-gray-500/20',
  dispatched: 'bg-blue-500/10 text-blue-400 border-blue-500/20',
  running: 'bg-amber-500/10 text-amber-400 border-amber-500/20',
  done: 'bg-green-500/10 text-green-400 border-green-500/20',
  online: 'bg-green-500/10 text-green-400',
  offline: 'bg-gray-500/10 text-gray-500',
  busy: 'bg-amber-500/10 text-amber-400',
}

const statusIcon = (status: string) => {
  switch (status) {
    case 'completed': case 'done': return <CheckCircle2 className="w-3.5 h-3.5" />
    case 'failed': return <AlertCircle className="w-3.5 h-3.5" />
    case 'executing': case 'running': return <Loader2 className="w-3.5 h-3.5 animate-spin" />
    case 'planning': return <Clock className="w-3.5 h-3.5" />
    default: return null
  }
}

export default function SquadPage() {
  const [squads, setSquads] = useState<Squad[]>([])
  const [selectedSquad, setSelectedSquad] = useState<Squad | null>(null)
  const [members, setMembers] = useState<SquadMember[]>([])
  const [missions, setMissions] = useState<Mission[]>([])
  const [expandedMission, setExpandedMission] = useState<string | null>(null)
  const [missionSteps, setMissionSteps] = useState<MissionStep[]>([])
  const [tab, setTab] = useState<'missions' | 'members'>('missions')
  const [showCreate, setShowCreate] = useState(false)
  const [showMission, setShowMission] = useState(false)
  const [showInvite, setShowInvite] = useState(false)
  const [loading, setLoading] = useState(false)

  // Create form
  const [newName, setNewName] = useState('')
  const [newDesc, setNewDesc] = useState('')
  // Mission form
  const [missionTitle, setMissionTitle] = useState('')
  const [missionGoal, setMissionGoal] = useState('')
  // Invite form
  const [inviteNodeId, setInviteNodeId] = useState('')
  const [inviteSpecialty, setInviteSpecialty] = useState('')

  useEffect(() => { loadSquads() }, [])

  const loadSquads = async () => {
    try {
      const res = await squadAPI.list()
      setSquads(res.data.squads || [])
    } catch { /* */ }
  }

  const selectSquad = async (squad: Squad) => {
    setSelectedSquad(squad)
    try {
      const [mRes, memRes] = await Promise.all([
        squadAPI.listMissions(squad.id),
        squadAPI.members(squad.id),
      ])
      setMissions(mRes.data.missions || [])
      setMembers(memRes.data.members || [])
    } catch { /* */ }
  }

  const createSquad = async () => {
    if (!newName.trim()) return
    setLoading(true)
    try {
      const res = await squadAPI.create({ name: newName, description: newDesc })
      setShowCreate(false)
      setNewName('')
      setNewDesc('')
      await loadSquads()
      selectSquad(res.data.squad)
    } catch { /* */ }
    setLoading(false)
  }

  const deleteSquad = async (id: string) => {
    if (!confirm('确定要解散这个战队吗？')) return
    try {
      await squadAPI.delete(id)
      setSelectedSquad(null)
      loadSquads()
    } catch { /* */ }
  }

  const inviteMember = async () => {
    if (!selectedSquad || !inviteNodeId.trim()) return
    setLoading(true)
    try {
      await squadAPI.invite(selectedSquad.id, { node_id: inviteNodeId, specialty: inviteSpecialty })
      setShowInvite(false)
      setInviteNodeId('')
      setInviteSpecialty('')
      const res = await squadAPI.members(selectedSquad.id)
      setMembers(res.data.members || [])
    } catch { /* */ }
    setLoading(false)
  }

  const createMission = async () => {
    if (!selectedSquad || !missionTitle.trim() || !missionGoal.trim()) return
    setLoading(true)
    try {
      await squadAPI.createMission(selectedSquad.id, { title: missionTitle, goal: missionGoal })
      setShowMission(false)
      setMissionTitle('')
      setMissionGoal('')
      const res = await squadAPI.listMissions(selectedSquad.id)
      setMissions(res.data.missions || [])
    } catch { /* */ }
    setLoading(false)
  }

  const startMission = async (id: string) => {
    try {
      await squadAPI.startMission(id)
      if (selectedSquad) {
        const res = await squadAPI.listMissions(selectedSquad.id)
        setMissions(res.data.missions || [])
      }
    } catch { /* */ }
  }

  const cancelMission = async (id: string) => {
    try {
      await squadAPI.cancelMission(id)
      if (selectedSquad) {
        const res = await squadAPI.listMissions(selectedSquad.id)
        setMissions(res.data.missions || [])
      }
    } catch { /* */ }
  }

  const toggleMissionSteps = async (missionId: string) => {
    if (expandedMission === missionId) {
      setExpandedMission(null)
      return
    }
    try {
      const res = await squadAPI.getMission(missionId)
      setMissionSteps(res.data.steps || [])
      setExpandedMission(missionId)
    } catch { /* */ }
  }

  // Auto-refresh executing missions
  useEffect(() => {
    if (!selectedSquad) return
    const hasExecuting = missions.some(m => m.status === 'executing' || m.status === 'reviewing')
    if (!hasExecuting) return
    const interval = setInterval(async () => {
      try {
        const res = await squadAPI.listMissions(selectedSquad.id)
        setMissions(res.data.missions || [])
        if (expandedMission) {
          const mRes = await squadAPI.getMission(expandedMission)
          setMissionSteps(mRes.data.steps || [])
        }
      } catch { /* */ }
    }, 5000)
    return () => clearInterval(interval)
  }, [selectedSquad, missions, expandedMission])

  return (
    <div className="flex h-full">
      {/* Squad list sidebar */}
      <div className="w-72 flex-shrink-0 border-r border-gray-200 dark:border-gray-700 bg-gray-50 dark:bg-gray-900 flex flex-col">
        <div className="p-4 border-b border-gray-200 dark:border-gray-700">
          <div className="flex items-center justify-between mb-3">
            <h2 className="font-semibold text-gray-800 dark:text-white flex items-center gap-2">
              <Swords className="w-5 h-5 text-primary-500" />
              战队
            </h2>
            <button
              onClick={() => setShowCreate(true)}
              className="p-1.5 rounded-lg bg-primary-600 text-white hover:bg-primary-700 transition-colors"
            >
              <Plus className="w-4 h-4" />
            </button>
          </div>
        </div>

        <div className="flex-1 overflow-y-auto p-2 space-y-1">
          {squads.length === 0 && (
            <div className="text-center py-12 text-gray-400 text-sm">
              <Users className="w-10 h-10 mx-auto mb-2 opacity-30" />
              暂无战队<br />点击 + 创建
            </div>
          )}
          {squads.map(s => (
            <button
              key={s.id}
              onClick={() => selectSquad(s)}
              className={`w-full text-left px-3 py-2.5 rounded-lg transition-colors ${
                selectedSquad?.id === s.id
                  ? 'bg-primary-100 dark:bg-primary-900/30 text-primary-700 dark:text-primary-300'
                  : 'text-gray-600 dark:text-gray-400 hover:bg-gray-100 dark:hover:bg-gray-800'
              }`}
            >
              <div className="font-medium text-sm truncate">{s.name}</div>
              <div className="flex items-center gap-2 mt-0.5">
                <span className={`text-[10px] px-1.5 py-0.5 rounded border ${statusColors[s.status] || ''}`}>
                  {s.status}
                </span>
                {s.description && <span className="text-xs text-gray-500 truncate">{s.description}</span>}
              </div>
            </button>
          ))}
        </div>
      </div>

      {/* Main content */}
      <div className="flex-1 min-w-0 flex flex-col">
        {!selectedSquad ? (
          <div className="flex-1 flex items-center justify-center text-gray-400">
            <div className="text-center">
              <Swords className="w-16 h-16 mx-auto mb-3 opacity-20" />
              <p className="text-lg font-medium">选择或创建一个战队</p>
              <p className="text-sm mt-1">多个 Claw 节点组队协作完成复杂任务</p>
            </div>
          </div>
        ) : (
          <>
            {/* Header */}
            <div className="px-6 py-4 border-b border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800">
              <div className="flex items-center justify-between">
                <div>
                  <h1 className="text-xl font-bold text-gray-800 dark:text-white">{selectedSquad.name}</h1>
                  <p className="text-sm text-gray-500 mt-0.5">
                    {members.length} 成员 · {missions.length} 任务 · 队长: {selectedSquad.captain_node.slice(0, 12)}...
                  </p>
                </div>
                <div className="flex items-center gap-2">
                  <button
                    onClick={() => setShowMission(true)}
                    className="flex items-center gap-1.5 px-3 py-1.5 bg-primary-600 text-white rounded-lg text-sm hover:bg-primary-700 transition-colors"
                  >
                    <Target className="w-4 h-4" /> 新建任务
                  </button>
                  <button
                    onClick={() => setShowInvite(true)}
                    className="flex items-center gap-1.5 px-3 py-1.5 bg-gray-200 dark:bg-gray-700 text-gray-700 dark:text-gray-300 rounded-lg text-sm hover:bg-gray-300 dark:hover:bg-gray-600 transition-colors"
                  >
                    <UserPlus className="w-4 h-4" /> 邀请
                  </button>
                  <button
                    onClick={() => deleteSquad(selectedSquad.id)}
                    className="p-1.5 text-gray-400 hover:text-red-500 transition-colors"
                    title="解散战队"
                  >
                    <Trash2 className="w-4 h-4" />
                  </button>
                </div>
              </div>

              {/* Tabs */}
              <div className="flex gap-4 mt-4">
                {(['missions', 'members'] as const).map(t => (
                  <button
                    key={t}
                    onClick={() => setTab(t)}
                    className={`pb-1 text-sm font-medium border-b-2 transition-colors ${
                      tab === t
                        ? 'border-primary-500 text-primary-600 dark:text-primary-400'
                        : 'border-transparent text-gray-500 hover:text-gray-700 dark:hover:text-gray-300'
                    }`}
                  >
                    {t === 'missions' ? `任务 (${missions.length})` : `成员 (${members.length})`}
                  </button>
                ))}
              </div>
            </div>

            {/* Content */}
            <div className="flex-1 overflow-y-auto p-6 space-y-3">
              {tab === 'missions' && (
                <>
                  {missions.length === 0 && (
                    <div className="text-center py-16 text-gray-400">
                      <Target className="w-10 h-10 mx-auto mb-2 opacity-30" />
                      <p>暂无任务，点击「新建任务」开始</p>
                    </div>
                  )}
                  {missions.map(m => (
                    <div key={m.id} className="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-xl overflow-hidden">
                      <div className="px-5 py-4">
                        <div className="flex items-center justify-between">
                          <div className="flex items-center gap-3 min-w-0">
                            <button onClick={() => toggleMissionSteps(m.id)} className="text-gray-400 hover:text-gray-600 flex-shrink-0">
                              {expandedMission === m.id ? <ChevronDown className="w-4 h-4" /> : <ChevronRight className="w-4 h-4" />}
                            </button>
                            <div className="min-w-0">
                              <h3 className="font-medium text-gray-800 dark:text-white truncate">{m.title}</h3>
                              <p className="text-xs text-gray-500 mt-0.5 truncate">{m.goal}</p>
                            </div>
                          </div>
                          <div className="flex items-center gap-2 flex-shrink-0 ml-3">
                            <span className={`inline-flex items-center gap-1 text-xs px-2 py-0.5 rounded-full border ${statusColors[m.status] || ''}`}>
                              {statusIcon(m.status)} {m.status}
                            </span>
                            {m.total_steps > 0 && (
                              <span className="text-xs text-gray-500">
                                {m.done_steps}/{m.total_steps}
                              </span>
                            )}
                            {m.status === 'planning' && (
                              <button onClick={() => startMission(m.id)} className="p-1 text-green-500 hover:text-green-600" title="开始执行">
                                <Play className="w-4 h-4" />
                              </button>
                            )}
                            {(m.status === 'executing' || m.status === 'planning') && (
                              <button onClick={() => cancelMission(m.id)} className="p-1 text-red-400 hover:text-red-500" title="取消">
                                <XCircle className="w-4 h-4" />
                              </button>
                            )}
                          </div>
                        </div>

                        {/* Progress bar */}
                        {m.total_steps > 0 && (
                          <div className="mt-3 h-1.5 bg-gray-200 dark:bg-gray-700 rounded-full overflow-hidden">
                            <div
                              className={`h-full transition-all duration-500 rounded-full ${m.status === 'failed' ? 'bg-red-500' : 'bg-primary-500'}`}
                              style={{ width: `${(m.done_steps / m.total_steps) * 100}%` }}
                            />
                          </div>
                        )}
                      </div>

                      {/* Expanded steps */}
                      {expandedMission === m.id && missionSteps.length > 0 && (
                        <div className="border-t border-gray-200 dark:border-gray-700 bg-gray-50 dark:bg-gray-900/50 px-5 py-3 space-y-2">
                          {missionSteps.map((s, i) => (
                            <div key={s.id} className="flex items-start gap-3 text-sm">
                              <div className="flex-shrink-0 w-6 h-6 rounded-full bg-gray-200 dark:bg-gray-700 flex items-center justify-center text-xs font-medium text-gray-600 dark:text-gray-400">
                                {i + 1}
                              </div>
                              <div className="min-w-0 flex-1">
                                <div className="flex items-center gap-2">
                                  <span className="font-medium text-gray-700 dark:text-gray-300 truncate">{s.task.slice(0, 80)}</span>
                                  <span className={`inline-flex items-center gap-0.5 text-[10px] px-1.5 py-0.5 rounded border flex-shrink-0 ${statusColors[s.status] || ''}`}>
                                    {statusIcon(s.status)} {s.status}
                                  </span>
                                </div>
                                <div className="text-xs text-gray-500 mt-0.5">
                                  → {s.target_node?.slice(0, 12) || '未分配'}... {s.target_agent && `(${s.target_agent})`}
                                </div>
                                {s.output && (
                                  <pre className="mt-1 text-xs text-gray-600 dark:text-gray-400 bg-white dark:bg-gray-800 rounded p-2 max-h-24 overflow-y-auto whitespace-pre-wrap break-words border">
                                    {s.output.slice(0, 500)}
                                  </pre>
                                )}
                                {s.error_msg && (
                                  <div className="mt-1 text-xs text-red-400">{s.error_msg}</div>
                                )}
                              </div>
                            </div>
                          ))}
                        </div>
                      )}
                    </div>
                  ))}
                </>
              )}

              {tab === 'members' && (
                <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3">
                  {members.map(m => (
                    <div key={m.id} className="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-xl p-4">
                      <div className="flex items-center justify-between">
                        <div className="flex items-center gap-2">
                          <div className={`w-2 h-2 rounded-full ${m.status === 'online' ? 'bg-green-500' : m.status === 'busy' ? 'bg-amber-500' : 'bg-gray-400'}`} />
                          <span className="font-mono text-sm text-gray-700 dark:text-gray-300">{m.node_id.slice(0, 16)}...</span>
                        </div>
                        <span className={`text-[10px] px-1.5 py-0.5 rounded ${statusColors[m.status] || ''}`}>{m.status}</span>
                      </div>
                      <div className="mt-2 flex items-center gap-2 text-xs text-gray-500">
                        {m.role === 'captain' && <span className="bg-amber-500/10 text-amber-500 px-1.5 py-0.5 rounded">队长</span>}
                        {m.specialty && <span className="bg-primary-500/10 text-primary-500 px-1.5 py-0.5 rounded">{m.specialty}</span>}
                        <span>{new Date(m.joined_at).toLocaleDateString()}</span>
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </div>
          </>
        )}
      </div>

      {/* Create Squad Modal */}
      {showCreate && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50" onClick={() => setShowCreate(false)}>
          <div className="bg-white dark:bg-gray-800 rounded-2xl p-6 max-w-md w-full mx-4 space-y-4" onClick={e => e.stopPropagation()}>
            <h3 className="text-lg font-semibold text-gray-800 dark:text-white">创建战队</h3>
            <input
              value={newName}
              onChange={e => setNewName(e.target.value)}
              placeholder="战队名称"
              className="w-full px-3 py-2 border rounded-lg text-sm bg-white dark:bg-gray-900 dark:text-white dark:border-gray-600 focus:ring-1 focus:ring-primary-500 outline-none"
              autoFocus
            />
            <textarea
              value={newDesc}
              onChange={e => setNewDesc(e.target.value)}
              placeholder="战队描述（可选）"
              rows={2}
              className="w-full px-3 py-2 border rounded-lg text-sm bg-white dark:bg-gray-900 dark:text-white dark:border-gray-600 focus:ring-1 focus:ring-primary-500 outline-none resize-none"
            />
            <div className="flex justify-end gap-2">
              <button onClick={() => setShowCreate(false)} className="px-4 py-2 text-sm text-gray-600 dark:text-gray-400 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-lg transition-colors">取消</button>
              <button onClick={createSquad} disabled={loading || !newName.trim()} className="px-4 py-2 text-sm bg-primary-600 text-white rounded-lg hover:bg-primary-700 disabled:opacity-50 transition-colors flex items-center gap-1.5">
                {loading ? <Loader2 className="w-3.5 h-3.5 animate-spin" /> : <Zap className="w-3.5 h-3.5" />} 创建
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Create Mission Modal */}
      {showMission && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50" onClick={() => setShowMission(false)}>
          <div className="bg-white dark:bg-gray-800 rounded-2xl p-6 max-w-lg w-full mx-4 space-y-4" onClick={e => e.stopPropagation()}>
            <h3 className="text-lg font-semibold text-gray-800 dark:text-white">新建任务</h3>
            <input
              value={missionTitle}
              onChange={e => setMissionTitle(e.target.value)}
              placeholder="任务标题"
              className="w-full px-3 py-2 border rounded-lg text-sm bg-white dark:bg-gray-900 dark:text-white dark:border-gray-600 focus:ring-1 focus:ring-primary-500 outline-none"
              autoFocus
            />
            <textarea
              value={missionGoal}
              onChange={e => setMissionGoal(e.target.value)}
              placeholder="详细描述任务目标，编排 Agent 会自动分解为子任务并分配给成员节点"
              rows={5}
              className="w-full px-3 py-2 border rounded-lg text-sm bg-white dark:bg-gray-900 dark:text-white dark:border-gray-600 focus:ring-1 focus:ring-primary-500 outline-none resize-none"
            />
            <div className="flex justify-end gap-2">
              <button onClick={() => setShowMission(false)} className="px-4 py-2 text-sm text-gray-600 dark:text-gray-400 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-lg transition-colors">取消</button>
              <button onClick={createMission} disabled={loading || !missionTitle.trim() || !missionGoal.trim()} className="px-4 py-2 text-sm bg-primary-600 text-white rounded-lg hover:bg-primary-700 disabled:opacity-50 transition-colors flex items-center gap-1.5">
                {loading ? <Loader2 className="w-3.5 h-3.5 animate-spin" /> : <Target className="w-3.5 h-3.5" />} 创建
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Invite Member Modal */}
      {showInvite && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50" onClick={() => setShowInvite(false)}>
          <div className="bg-white dark:bg-gray-800 rounded-2xl p-6 max-w-md w-full mx-4 space-y-4" onClick={e => e.stopPropagation()}>
            <h3 className="text-lg font-semibold text-gray-800 dark:text-white">邀请成员</h3>
            <input
              value={inviteNodeId}
              onChange={e => setInviteNodeId(e.target.value)}
              placeholder="对方的 claw: 地址"
              className="w-full px-3 py-2 border rounded-lg text-sm font-mono bg-white dark:bg-gray-900 dark:text-white dark:border-gray-600 focus:ring-1 focus:ring-primary-500 outline-none"
              autoFocus
            />
            <select
              value={inviteSpecialty}
              onChange={e => setInviteSpecialty(e.target.value)}
              className="w-full px-3 py-2 border rounded-lg text-sm bg-white dark:bg-gray-900 dark:text-white dark:border-gray-600 focus:ring-1 focus:ring-primary-500 outline-none"
            >
              <option value="">选择特长（可选）</option>
              <option value="coding">编程开发</option>
              <option value="design">设计创意</option>
              <option value="writing">文案写作</option>
              <option value="video">视频制作</option>
              <option value="testing">测试质检</option>
              <option value="sales">销售运营</option>
              <option value="general">通用</option>
            </select>
            <div className="flex justify-end gap-2">
              <button onClick={() => setShowInvite(false)} className="px-4 py-2 text-sm text-gray-600 dark:text-gray-400 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-lg transition-colors">取消</button>
              <button onClick={inviteMember} disabled={loading || !inviteNodeId.trim()} className="px-4 py-2 text-sm bg-primary-600 text-white rounded-lg hover:bg-primary-700 disabled:opacity-50 transition-colors flex items-center gap-1.5">
                {loading ? <Loader2 className="w-3.5 h-3.5 animate-spin" /> : <UserPlus className="w-3.5 h-3.5" />} 邀请
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
