import { useEffect, useState } from 'react'
import { Users, Plus, Trash2, Settings } from 'lucide-react'
import { broodAPI, type Team } from '../api/brood'

export default function TeamsPage() {
  const [teams, setTeams] = useState<(Team & { node_count: number })[]>([])
  const [loading, setLoading] = useState(true)
  const [showCreate, setShowCreate] = useState(false)
  const [form, setForm] = useState({ name: '', display_name: '', max_nodes: 0, max_tokens: 0 })

  const load = async () => {
    try {
      const data = await broodAPI.listTeams()
      setTeams(data.teams || [])
    } catch { /* */ }
    finally { setLoading(false) }
  }

  useEffect(() => { load() }, [])

  const handleCreate = async () => {
    if (!form.name.trim()) return
    try {
      await broodAPI.createTeam(form)
      setShowCreate(false)
      setForm({ name: '', display_name: '', max_nodes: 0, max_tokens: 0 })
      load()
    } catch { /* */ }
  }

  const handleDelete = async (id: string, name: string) => {
    if (!confirm(`确定删除团队 "${name}" 吗？`)) return
    try { await broodAPI.deleteTeam(id); load() } catch { /* */ }
  }

  return (
    <div className="p-8">
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-2xl font-bold text-white">团队管理</h1>
          <p className="text-sm text-gray-500 mt-1">管理企业内的多租户团队</p>
        </div>
        <button
          onClick={() => setShowCreate(!showCreate)}
          className="flex items-center gap-2 px-4 py-2 bg-overlord-600 text-white rounded-lg text-sm hover:bg-overlord-500 transition"
        >
          <Plus className="w-4 h-4" /> 新建团队
        </button>
      </div>

      {showCreate && (
        <div className="bg-gray-900 border border-gray-800 rounded-xl p-5 mb-6">
          <h3 className="text-sm font-medium text-white mb-4">创建团队</h3>
          <div className="grid grid-cols-2 gap-4 mb-4">
            <div>
              <label className="block text-xs text-gray-400 mb-1">团队标识 *</label>
              <input value={form.name} onChange={e => setForm({ ...form, name: e.target.value })}
                placeholder="team-alpha" className="w-full bg-gray-800 border border-gray-700 rounded-lg px-3 py-2 text-white text-sm focus:outline-none focus:border-overlord-500" />
            </div>
            <div>
              <label className="block text-xs text-gray-400 mb-1">显示名称</label>
              <input value={form.display_name} onChange={e => setForm({ ...form, display_name: e.target.value })}
                placeholder="Alpha 团队" className="w-full bg-gray-800 border border-gray-700 rounded-lg px-3 py-2 text-white text-sm focus:outline-none focus:border-overlord-500" />
            </div>
            <div>
              <label className="block text-xs text-gray-400 mb-1">最大节点数 (0=不限)</label>
              <input type="number" value={form.max_nodes} onChange={e => setForm({ ...form, max_nodes: +e.target.value })}
                className="w-full bg-gray-800 border border-gray-700 rounded-lg px-3 py-2 text-white text-sm focus:outline-none focus:border-overlord-500" />
            </div>
            <div>
              <label className="block text-xs text-gray-400 mb-1">每日 Token 上限 (0=不限)</label>
              <input type="number" value={form.max_tokens} onChange={e => setForm({ ...form, max_tokens: +e.target.value })}
                className="w-full bg-gray-800 border border-gray-700 rounded-lg px-3 py-2 text-white text-sm focus:outline-none focus:border-overlord-500" />
            </div>
          </div>
          <div className="flex gap-2">
            <button onClick={handleCreate} className="px-4 py-2 bg-overlord-600 text-white text-sm rounded-lg hover:bg-overlord-500 transition">创建</button>
            <button onClick={() => setShowCreate(false)} className="px-4 py-2 bg-gray-800 text-gray-300 text-sm rounded-lg hover:bg-gray-700 transition">取消</button>
          </div>
        </div>
      )}

      {loading ? (
        <div className="flex justify-center py-12">
          <div className="animate-spin w-6 h-6 border-2 border-overlord-500 border-t-transparent rounded-full" />
        </div>
      ) : teams.length === 0 ? (
        <div className="bg-gray-900 border border-gray-800 border-dashed rounded-xl p-12 text-center">
          <Users className="w-10 h-10 text-gray-600 mx-auto mb-3" />
          <p className="text-gray-400">暂无团队</p>
        </div>
      ) : (
        <div className="space-y-2">
          {teams.map(team => (
            <div key={team.id} className="bg-gray-900 border border-gray-800 rounded-xl p-4 flex items-center gap-4 hover:border-gray-700 transition group">
              <div className="w-10 h-10 rounded-lg bg-overlord-600/10 flex items-center justify-center shrink-0">
                <Users className="w-5 h-5 text-overlord-400" />
              </div>
              <div className="flex-1 min-w-0">
                <div className="flex items-center gap-2">
                  <span className="text-sm font-semibold text-white">{team.display_name || team.name}</span>
                  <span className="text-[10px] px-1.5 py-0.5 rounded bg-gray-800 text-gray-400 font-mono">{team.name}</span>
                  <span className={`text-[10px] px-1.5 py-0.5 rounded ${team.status === 'active' ? 'bg-emerald-600/10 text-emerald-400' : 'bg-red-600/10 text-red-400'}`}>
                    {team.status}
                  </span>
                </div>
                <div className="flex items-center gap-4 mt-1 text-xs text-gray-500">
                  <span>{team.node_count} 节点</span>
                  <span>上限: {team.max_nodes || '∞'} 节点</span>
                  <span>Token: {team.max_tokens ? team.max_tokens.toLocaleString() : '∞'}/天</span>
                </div>
              </div>
              <div className="flex items-center gap-1 opacity-0 group-hover:opacity-100 transition">
                <button className="p-2 text-gray-500 hover:text-overlord-400 transition" title="设置">
                  <Settings className="w-4 h-4" />
                </button>
                <button onClick={() => handleDelete(team.id, team.name)} className="p-2 text-gray-500 hover:text-red-400 transition" title="删除">
                  <Trash2 className="w-4 h-4" />
                </button>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
