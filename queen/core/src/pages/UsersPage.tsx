import { useEffect, useState } from 'react'
import { api, type QueenUser, type UserStats } from '../api'
import { Users, Shield, Ban, Search, ChevronLeft, ChevronRight } from 'lucide-react'

const roleBadge: Record<string, { label: string; cls: string }> = {
  admin: { label: '管理员', cls: 'bg-purple-600/20 text-purple-400' },
  developer: { label: '开发者', cls: 'bg-blue-600/20 text-blue-400' },
  user: { label: '用户', cls: 'bg-gray-600/20 text-gray-400' },
}

const statusBadge: Record<string, { label: string; cls: string }> = {
  active: { label: '正常', cls: 'bg-emerald-600/20 text-emerald-400' },
  banned: { label: '封禁', cls: 'bg-red-600/20 text-red-400' },
}

export default function UsersPage() {
  const [users, setUsers] = useState<QueenUser[]>([])
  const [stats, setStats] = useState<UserStats | null>(null)
  const [page, setPage] = useState(1)
  const [total, setTotal] = useState(0)
  const [search, setSearch] = useState('')
  const [roleFilter, setRoleFilter] = useState('')
  const [statusFilter, setStatusFilter] = useState('')
  const [loading, setLoading] = useState(true)
  const size = 20

  const load = async () => {
    setLoading(true)
    try {
      let path = `/v1/admin/users?page=${page}&size=${size}`
      if (search) path += `&search=${encodeURIComponent(search)}`
      if (roleFilter) path += `&role=${roleFilter}`
      if (statusFilter) path += `&status=${statusFilter}`

      const [userData, statsData] = await Promise.all([
        api.get<{ data: { users: QueenUser[]; total: number } }>(path),
        api.get<{ data: UserStats }>('/v1/admin/users/stats'),
      ])
      setUsers(userData.data?.users || [])
      setTotal(userData.data?.total || 0)
      setStats(statsData.data || null)
    } catch { /* ignore */ }
    setLoading(false)
  }

  useEffect(() => { load() }, [page, roleFilter, statusFilter])

  const handleSearch = () => { setPage(1); load() }

  const updateRole = async (id: string, role: string) => {
    try {
      await api.put(`/v1/admin/users/${id}/role`, { role })
      load()
    } catch { /* ignore */ }
  }

  const toggleStatus = async (id: string, current: string) => {
    const newStatus = current === 'active' ? 'banned' : 'active'
    try {
      await api.put(`/v1/admin/users/${id}/status`, { status: newStatus })
      load()
    } catch { /* ignore */ }
  }

  const totalPages = Math.ceil(total / size)

  return (
    <div>
      <h2 className="text-xl font-bold mb-6">用户管理</h2>

      {/* Stats */}
      {stats && (
        <div className="grid grid-cols-5 gap-3 mb-6">
          {[
            { label: '总用户', value: stats.total, icon: Users, color: 'text-purple-400' },
            { label: '活跃', value: stats.active, icon: Shield, color: 'text-emerald-400' },
            { label: '封禁', value: stats.banned, icon: Ban, color: 'text-red-400' },
            { label: '管理员', value: stats.admins, icon: Shield, color: 'text-amber-400' },
            { label: '开发者', value: stats.developers, icon: Shield, color: 'text-blue-400' },
          ].map(s => (
            <div key={s.label} className="bg-gray-900 border border-gray-800 rounded-lg p-4">
              <div className="flex items-center gap-2 mb-1">
                <s.icon size={14} className={s.color} />
                <span className="text-xs text-gray-500">{s.label}</span>
              </div>
              <div className="text-xl font-bold text-white">{s.value}</div>
            </div>
          ))}
        </div>
      )}

      {/* Filters */}
      <div className="flex gap-3 mb-4">
        <div className="flex-1 relative">
          <Search size={14} className="absolute left-3 top-1/2 -translate-y-1/2 text-gray-500" />
          <input
            value={search}
            onChange={e => setSearch(e.target.value)}
            onKeyDown={e => e.key === 'Enter' && handleSearch()}
            placeholder="搜索邮箱/昵称/手机号…"
            className="w-full pl-9 pr-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-sm text-white placeholder-gray-600 focus:outline-none focus:border-purple-500"
          />
        </div>
        <select value={roleFilter} onChange={e => { setRoleFilter(e.target.value); setPage(1) }}
          className="bg-gray-800 border border-gray-700 rounded-lg px-3 py-2 text-sm text-gray-300">
          <option value="">全部角色</option>
          <option value="admin">管理员</option>
          <option value="developer">开发者</option>
          <option value="user">普通用户</option>
        </select>
        <select value={statusFilter} onChange={e => { setStatusFilter(e.target.value); setPage(1) }}
          className="bg-gray-800 border border-gray-700 rounded-lg px-3 py-2 text-sm text-gray-300">
          <option value="">全部状态</option>
          <option value="active">正常</option>
          <option value="banned">封禁</option>
        </select>
      </div>

      {/* Table */}
      <div className="bg-gray-900 border border-gray-800 rounded-xl overflow-hidden">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-gray-800 text-gray-500 text-xs uppercase">
              <th className="text-left px-4 py-3">用户</th>
              <th className="text-left px-4 py-3">邮箱</th>
              <th className="text-left px-4 py-3">角色</th>
              <th className="text-left px-4 py-3">状态</th>
              <th className="text-left px-4 py-3">注册时间</th>
              <th className="text-right px-4 py-3">操作</th>
            </tr>
          </thead>
          <tbody>
            {loading ? (
              <tr><td colSpan={6} className="text-center py-8 text-gray-600">加载中…</td></tr>
            ) : users.length === 0 ? (
              <tr><td colSpan={6} className="text-center py-8 text-gray-600">暂无用户</td></tr>
            ) : users.map(u => {
              const rb = roleBadge[u.role] || roleBadge.user
              const sb = statusBadge[u.status] || statusBadge.active
              return (
                <tr key={u.id} className="border-b border-gray-800/50 hover:bg-gray-800/30">
                  <td className="px-4 py-3">
                    <div className="flex items-center gap-2">
                      <div className="w-7 h-7 rounded-full bg-gray-700 flex items-center justify-center text-xs text-gray-400">
                        {(u.nickname || u.email)?.[0]?.toUpperCase() || '?'}
                      </div>
                      <span className="text-white font-medium">{u.nickname || '未设置'}</span>
                    </div>
                  </td>
                  <td className="px-4 py-3 text-gray-400">{u.email}</td>
                  <td className="px-4 py-3">
                    <select
                      value={u.role}
                      onChange={e => updateRole(u.id, e.target.value)}
                      className={`text-xs px-2 py-1 rounded ${rb.cls} bg-transparent border-none cursor-pointer`}
                    >
                      <option value="user">用户</option>
                      <option value="developer">开发者</option>
                      <option value="admin">管理员</option>
                    </select>
                  </td>
                  <td className="px-4 py-3">
                    <span className={`text-xs px-2 py-0.5 rounded ${sb.cls}`}>{sb.label}</span>
                  </td>
                  <td className="px-4 py-3 text-gray-500 text-xs">
                    {new Date(u.created_at).toLocaleDateString('zh-CN')}
                  </td>
                  <td className="px-4 py-3 text-right">
                    <button
                      onClick={() => toggleStatus(u.id, u.status)}
                      className={`text-xs px-2.5 py-1 rounded transition ${
                        u.status === 'active'
                          ? 'text-red-400 hover:bg-red-600/10'
                          : 'text-emerald-400 hover:bg-emerald-600/10'
                      }`}
                    >
                      {u.status === 'active' ? '封禁' : '解封'}
                    </button>
                  </td>
                </tr>
              )
            })}
          </tbody>
        </table>
      </div>

      {/* Pagination */}
      {totalPages > 1 && (
        <div className="flex items-center justify-between mt-4">
          <span className="text-xs text-gray-500">共 {total} 个用户</span>
          <div className="flex items-center gap-2">
            <button onClick={() => setPage(p => Math.max(1, p - 1))} disabled={page <= 1}
              className="p-1.5 rounded bg-gray-800 text-gray-400 hover:text-white disabled:opacity-30 transition">
              <ChevronLeft size={14} />
            </button>
            <span className="text-xs text-gray-400">{page} / {totalPages}</span>
            <button onClick={() => setPage(p => Math.min(totalPages, p + 1))} disabled={page >= totalPages}
              className="p-1.5 rounded bg-gray-800 text-gray-400 hover:text-white disabled:opacity-30 transition">
              <ChevronRight size={14} />
            </button>
          </div>
        </div>
      )}
    </div>
  )
}
