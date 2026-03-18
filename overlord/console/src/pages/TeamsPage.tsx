import { useEffect, useState } from 'react'
import { Users, Plus, Trash2, Settings, UserPlus, X } from 'lucide-react'
import { broodAPI, type Team, type AdminUser } from '../api/brood'

const inputCls = 'w-full bg-gray-800 border border-gray-700 rounded-lg px-3 py-2 text-white text-sm focus:outline-none focus:border-overlord-500'
const roleBadge: Record<string, string> = {
  superadmin: 'bg-red-600/10 text-red-400',
  admin: 'bg-amber-600/10 text-amber-400',
  operator: 'bg-blue-600/10 text-blue-400',
  viewer: 'bg-gray-600/10 text-gray-400',
}

export default function TeamsPage() {
  const [teams, setTeams] = useState<(Team & { node_count: number })[]>([])
  const [admins, setAdmins] = useState<AdminUser[]>([])
  const [loading, setLoading] = useState(true)
  const [showCreate, setShowCreate] = useState(false)
  const [showAddUser, setShowAddUser] = useState(false)
  const [form, setForm] = useState({ name: '', display_name: '', max_nodes: 0, max_tokens: 0 })
  const [userForm, setUserForm] = useState({ username: '', password: '', role: 'viewer', team_id: '', email: '' })

  const load = async () => {
    try {
      const [teamData, adminData] = await Promise.all([broodAPI.listTeams(), broodAPI.listAdmins()])
      setTeams(teamData.teams || [])
      setAdmins(adminData.users || [])
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

  const handleAddUser = async () => {
    if (!userForm.username.trim() || !userForm.password.trim()) return
    try {
      await broodAPI.createAdmin(userForm)
      setShowAddUser(false)
      setUserForm({ username: '', password: '', role: 'viewer', team_id: '', email: '' })
      load()
    } catch (e: any) { alert(e.message || 'failed') }
  }

  const handleDeleteUser = async (id: string, name: string) => {
    if (!confirm(`确定删除账号 "${name}" 吗？`)) return
    try { await broodAPI.deleteAdmin(id); load() } catch { /* */ }
  }

  return (
    <div className="p-8">
      {/* ── Teams Section ── */}
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
                placeholder="team-alpha" className={inputCls} />
            </div>
            <div>
              <label className="block text-xs text-gray-400 mb-1">显示名称</label>
              <input value={form.display_name} onChange={e => setForm({ ...form, display_name: e.target.value })}
                placeholder="Alpha 团队" className={inputCls} />
            </div>
            <div>
              <label className="block text-xs text-gray-400 mb-1">最大节点数 (0=不限)</label>
              <input type="number" value={form.max_nodes} onChange={e => setForm({ ...form, max_nodes: +e.target.value })}
                className={inputCls} />
            </div>
            <div>
              <label className="block text-xs text-gray-400 mb-1">每日 Token 上限 (0=不限)</label>
              <input type="number" value={form.max_tokens} onChange={e => setForm({ ...form, max_tokens: +e.target.value })}
                className={inputCls} />
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
                  <span>{admins.filter(a => a.team_id === team.id).length} 成员</span>
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

      {/* ── Admin Users Section ── */}
      <div className="flex items-center justify-between mt-12 mb-6">
        <div>
          <h2 className="text-xl font-bold text-white">员工账号</h2>
          <p className="text-sm text-gray-500 mt-1">管理控制台和员工工作台的登录账号</p>
        </div>
        <button
          onClick={() => setShowAddUser(!showAddUser)}
          className="flex items-center gap-2 px-4 py-2 bg-overlord-600 text-white rounded-lg text-sm hover:bg-overlord-500 transition"
        >
          <UserPlus className="w-4 h-4" /> 添加账号
        </button>
      </div>

      {showAddUser && (
        <div className="bg-gray-900 border border-gray-800 rounded-xl p-5 mb-6">
          <div className="flex items-center justify-between mb-4">
            <h3 className="text-sm font-medium text-white">创建账号</h3>
            <button onClick={() => setShowAddUser(false)} className="text-gray-500 hover:text-gray-300"><X className="w-4 h-4" /></button>
          </div>
          <div className="grid grid-cols-2 gap-4 mb-4">
            <div>
              <label className="block text-xs text-gray-400 mb-1">用户名 *</label>
              <input value={userForm.username} onChange={e => setUserForm({ ...userForm, username: e.target.value })}
                placeholder="zhangsan" className={inputCls} />
            </div>
            <div>
              <label className="block text-xs text-gray-400 mb-1">密码 *</label>
              <input type="password" value={userForm.password} onChange={e => setUserForm({ ...userForm, password: e.target.value })}
                placeholder="至少6位" className={inputCls} />
            </div>
            <div>
              <label className="block text-xs text-gray-400 mb-1">角色</label>
              <select value={userForm.role} onChange={e => setUserForm({ ...userForm, role: e.target.value })}
                className={inputCls}>
                <option value="viewer">viewer — 只读</option>
                <option value="operator">operator — 操作员</option>
                <option value="admin">admin — 管理员</option>
                <option value="superadmin">superadmin — 超级管理员</option>
              </select>
            </div>
            <div>
              <label className="block text-xs text-gray-400 mb-1">所属团队</label>
              <select value={userForm.team_id} onChange={e => setUserForm({ ...userForm, team_id: e.target.value })}
                className={inputCls}>
                <option value="">无（全局）</option>
                {teams.map(t => <option key={t.id} value={t.id}>{t.display_name || t.name}</option>)}
              </select>
            </div>
            <div className="col-span-2">
              <label className="block text-xs text-gray-400 mb-1">邮箱（可选）</label>
              <input value={userForm.email} onChange={e => setUserForm({ ...userForm, email: e.target.value })}
                placeholder="zhangsan@example.com" className={inputCls} />
            </div>
          </div>
          <div className="flex gap-2">
            <button onClick={handleAddUser} className="px-4 py-2 bg-overlord-600 text-white text-sm rounded-lg hover:bg-overlord-500 transition">创建</button>
            <button onClick={() => setShowAddUser(false)} className="px-4 py-2 bg-gray-800 text-gray-300 text-sm rounded-lg hover:bg-gray-700 transition">取消</button>
          </div>
        </div>
      )}

      {admins.length === 0 ? (
        <div className="bg-gray-900 border border-gray-800 border-dashed rounded-xl p-8 text-center">
          <UserPlus className="w-8 h-8 text-gray-600 mx-auto mb-2" />
          <p className="text-gray-400 text-sm">暂无员工账号，点击上方「添加账号」创建</p>
        </div>
      ) : (
        <div className="space-y-2">
          {admins.map(user => {
            const team = teams.find(t => t.id === user.team_id)
            return (
              <div key={user.id} className="bg-gray-900 border border-gray-800 rounded-xl p-4 flex items-center gap-4 hover:border-gray-700 transition group">
                <div className="w-10 h-10 rounded-full bg-overlord-600/10 flex items-center justify-center shrink-0">
                  <span className="text-sm font-bold text-overlord-400">{user.username.charAt(0).toUpperCase()}</span>
                </div>
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-2">
                    <span className="text-sm font-semibold text-white">{user.username}</span>
                    <span className={`text-[10px] px-1.5 py-0.5 rounded ${roleBadge[user.role] || roleBadge.viewer}`}>
                      {user.role}
                    </span>
                    {team && (
                      <span className="text-[10px] px-1.5 py-0.5 rounded bg-gray-800 text-gray-400">
                        {team.display_name || team.name}
                      </span>
                    )}
                  </div>
                  {user.email && <p className="text-xs text-gray-500 mt-0.5">{user.email}</p>}
                </div>
                <div className="opacity-0 group-hover:opacity-100 transition">
                  <button onClick={() => handleDeleteUser(user.id, user.username)} className="p-2 text-gray-500 hover:text-red-400 transition" title="删除">
                    <Trash2 className="w-4 h-4" />
                  </button>
                </div>
              </div>
            )
          })}
        </div>
      )}
    </div>
  )
}
