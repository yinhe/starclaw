import { useEffect, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { ArrowLeft, Crown, Users, Loader2, GitBranch, FolderSync, RefreshCw, ChevronRight, MessageSquare } from 'lucide-react'
import { teamAPI, workflowAPI } from '../lib/api'
import { formatAgentDisplayName } from '../lib/agentDisplay'

interface Agent { id: string; name: string; description: string }
interface Member { id: string; team_id: string; agent_id: string; role: string; specialty: string; order: number; agent?: Agent }
interface Team { id: string; name: string; description: string; icon: string; coordinator_id: string; topology: string; status: string; template_id: string; members: Member[]; created_at: string }
interface Workflow { id: string; name: string; agent_id: string; created_at: string; updated_at: string }

export default function TeamDetailPage() {
  const navigate = useNavigate()
  const { id } = useParams<{ id: string }>()
  const [team, setTeam] = useState<Team | null>(null)
  const [workflows, setWorkflows] = useState<Workflow[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  // Sync project
  const [showSyncModal, setShowSyncModal] = useState(false)
  const [projects, setProjects] = useState<{ name: string; has_bible: boolean; has_drama: boolean }[]>([])
  const [syncing, setSyncing] = useState(false)
  const [syncStats, setSyncStats] = useState<Record<string, number> | null>(null)

  useEffect(() => {
    if (id) load()
  }, [id])

  const load = async () => {
    setLoading(true)
    setError('')
    try {
      const [tRes, wRes] = await Promise.all([
        teamAPI.get(id!),
        workflowAPI.list(),
      ])
      const t = tRes.data?.team
      if (!t) { setError('团队不存在'); setLoading(false); return }
      setTeam(t)
      // Filter workflows belonging to any member of this team
      const memberIds = new Set((t.members || []).map((m: Member) => m.agent_id))
      const allWf = wRes.data?.workflows || []
      setWorkflows(allWf.filter((w: Workflow) => memberIds.has(w.agent_id)))
    } catch {
      setError('加载失败')
    }
    setLoading(false)
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
      const res = await workflowAPI.syncProject({ project_name: projectName, agent_id: team?.coordinator_id })
      setSyncStats(res.data.stats)
      load() // reload workflows
      if (res.data.workflow?.id) {
        setTimeout(() => {
          setShowSyncModal(false)
          navigate(`/workflows/editor?id=${res.data.workflow.id}`)
        }, 1500)
      }
    } catch { alert('同步失败') }
    setSyncing(false)
  }

  const isDramaTeam = team && (team.name.includes('短剧') || team.name.includes('影视') || ['short-drama', 'video'].includes(team.template_id || ''))

  if (loading) return <div className="flex items-center justify-center h-full"><Loader2 className="w-6 h-6 animate-spin text-gray-400" /></div>

  if (error || !team) {
    return (
      <div className="h-full overflow-y-auto">
        <div className="max-w-5xl mx-auto p-8">
          <button onClick={() => navigate('/teams')} className="inline-flex items-center gap-2 text-sm text-gray-600 hover:text-gray-900 mb-4">
            <ArrowLeft className="w-4 h-4" /> 返回团队
          </button>
          <div className="bg-white dark:bg-gray-800 border dark:border-gray-700 rounded-xl p-8 text-center text-gray-500">{error || '团队不存在'}</div>
        </div>
      </div>
    )
  }

  return (
    <div className="h-full overflow-y-auto">
      <div className="max-w-5xl mx-auto p-8 space-y-6">
        {/* Header */}
        <button onClick={() => navigate('/teams')} className="inline-flex items-center gap-2 text-sm text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-white">
          <ArrowLeft className="w-4 h-4" /> 返回团队
        </button>

        <div className="bg-white dark:bg-gray-800 border dark:border-gray-700 rounded-xl p-6">
          <div className="flex items-start justify-between gap-4">
            <div>
              <h1 className="text-2xl font-bold text-gray-900 dark:text-white">{team.name}</h1>
              <p className="text-gray-600 dark:text-gray-400 mt-2">{team.description}</p>
              <div className="flex items-center gap-2 mt-3">
                <span className={`text-[10px] px-2 py-0.5 rounded-full ${team.status === 'active' ? 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400' : 'bg-gray-100 text-gray-500'}`}>
                  {team.status === 'active' ? '运行中' : '已暂停'}
                </span>
                <span className="text-[10px] px-2 py-0.5 rounded-full bg-purple-100 text-purple-700 dark:bg-purple-900/30 dark:text-purple-400">
                  {team.topology === 'sequential' ? '顺序执行' : team.topology === 'parallel' ? '并行执行' : team.topology}
                </span>
                <span className="text-[10px] px-2 py-0.5 rounded-full bg-gray-100 text-gray-600 dark:bg-gray-700 dark:text-gray-300">
                  {team.members?.length || 0} 成员
                </span>
              </div>
            </div>
            <div className="flex items-center gap-2">
              <a href={`/chat?team=${team.id}`}
                className="flex items-center gap-1.5 px-4 py-2 text-sm bg-primary-600 text-white rounded-lg hover:bg-primary-700 transition-colors">
                <MessageSquare className="w-4 h-4" /> 团队对话
              </a>
              {isDramaTeam && (
                <button onClick={() => { setShowSyncModal(true); loadProjects() }}
                  className="flex items-center gap-1.5 px-4 py-2 text-sm bg-violet-600 text-white rounded-lg hover:bg-violet-700 transition-colors">
                  <FolderSync className="w-4 h-4" /> 同步剧本项目
                </button>
              )}
            </div>
          </div>
        </div>

        {/* Members */}
        <section className="bg-white dark:bg-gray-800 border dark:border-gray-700 rounded-xl p-6">
          <h2 className="text-sm font-semibold text-gray-900 dark:text-white mb-4 flex items-center gap-2">
            <Users className="w-4 h-4 text-violet-600" /> 团队成员 ({team.members?.length || 0})
          </h2>
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-3">
            {(team.members || []).map(m => (
              <div key={m.id}
                onClick={() => navigate(`/agents/${m.agent_id}`)}
                className="border dark:border-gray-700 rounded-xl p-4 cursor-pointer hover:border-primary-300 hover:bg-primary-50 dark:hover:bg-primary-900/10 transition-all group">
                <div className="flex items-center gap-3">
                  <div className={`w-10 h-10 rounded-xl flex items-center justify-center ${m.role === 'coordinator' ? 'bg-amber-100 dark:bg-amber-900/30' : 'bg-gray-100 dark:bg-gray-700'}`}>
                    {m.role === 'coordinator' ? <Crown className="w-5 h-5 text-amber-600" /> : <Users className="w-5 h-5 text-gray-500" />}
                  </div>
                  <div className="flex-1 min-w-0">
                    <div className="font-medium text-gray-900 dark:text-white text-sm truncate group-hover:text-primary-600">
                      {formatAgentDisplayName(m.agent?.name || '未知')}
                    </div>
                    <div className="text-xs text-gray-500 truncate">{m.specialty || (m.role === 'coordinator' ? '团长' : '成员')}</div>
                  </div>
                  <ChevronRight className="w-4 h-4 text-gray-300 group-hover:text-primary-500" />
                </div>
              </div>
            ))}
          </div>
        </section>

        {/* Workflows */}
        <section className="bg-white dark:bg-gray-800 border dark:border-gray-700 rounded-xl p-6">
          <h2 className="text-sm font-semibold text-gray-900 dark:text-white mb-4 flex items-center gap-2">
            <GitBranch className="w-4 h-4 text-cyan-600" /> 团队工作流 ({workflows.length})
          </h2>
          {workflows.length === 0 ? (
            <div className="text-center py-8 text-gray-400 text-sm">
              {isDramaTeam ? '还没有工作流，点击上方「同步剧本项目」自动生成' : '暂无工作流'}
            </div>
          ) : (
            <div className="space-y-2">
              {workflows.map(wf => (
                <div key={wf.id}
                  onClick={() => navigate(`/workflows/editor?id=${wf.id}`)}
                  className="flex items-center justify-between p-4 border dark:border-gray-700 rounded-xl cursor-pointer hover:border-cyan-300 hover:bg-cyan-50 dark:hover:bg-cyan-900/10 transition-all group">
                  <div className="flex items-center gap-3">
                    <GitBranch className="w-5 h-5 text-cyan-500" />
                    <div>
                      <div className="font-medium text-gray-900 dark:text-white text-sm group-hover:text-cyan-600">{wf.name}</div>
                      <div className="text-xs text-gray-400 mt-0.5">更新于 {new Date(wf.updated_at).toLocaleString('zh-CN')}</div>
                    </div>
                  </div>
                  <ChevronRight className="w-4 h-4 text-gray-300 group-hover:text-cyan-500" />
                </div>
              ))}
            </div>
          )}
        </section>
      </div>

      {/* Sync Project Modal */}
      {showSyncModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 backdrop-blur-sm" onClick={() => setShowSyncModal(false)}>
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
                    className="w-full flex items-center justify-between p-4 rounded-xl border dark:border-gray-700 hover:border-violet-300 hover:bg-violet-50 dark:hover:bg-violet-900/20 transition-colors disabled:opacity-50">
                    <div className="text-left">
                      <div className="font-semibold text-gray-900 dark:text-gray-100">{p.name}</div>
                      <div className="text-xs text-gray-500 mt-0.5 flex gap-2">
                        {p.has_bible && <span className="text-violet-500">bible.md</span>}
                        {p.has_drama && <span className="text-cyan-500">drama/</span>}
                      </div>
                    </div>
                    {syncing ? <Loader2 className="w-5 h-5 text-violet-500 animate-spin" /> : <FolderSync className="w-5 h-5 text-gray-400" />}
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
    </div>
  )
}
