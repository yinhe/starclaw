import { useEffect, useState } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { Server, Wifi, WifiOff, AlertTriangle, Trash2, Plus, ExternalLink } from 'lucide-react'
import { broodAPI, type ClawNode } from '../api/brood'

const statusIcon = { online: Wifi, feral: AlertTriangle, offline: WifiOff }
const statusColor = {
  online: 'text-emerald-400 bg-emerald-600/10',
  feral: 'text-amber-400 bg-amber-600/10',
  offline: 'text-gray-400 bg-gray-600/10',
}
const statusLabel = { online: '在线', feral: '失控', offline: '离线' }

export default function ClawsPage() {
  const [searchParams, setSearchParams] = useSearchParams()
  const [claws, setClaws] = useState<ClawNode[]>([])
  const [loading, setLoading] = useState(true)
  const [filter, setFilter] = useState(searchParams.get('status') || '')
  const [teamFilter, setTeamFilter] = useState(searchParams.get('team') || '')

  // Add node modal
  const [showAdd, setShowAdd] = useState(false)
  const [addForm, setAddForm] = useState({ name: '', address: '', team: '' })
  const [adding, setAdding] = useState(false)
  const [addError, setAddError] = useState('')

  const handleAdd = async () => {
    if (!addForm.name || !addForm.address) return
    setAdding(true)
    setAddError('')
    try {
      await broodAPI.registerClaw({ name: addForm.name, address: addForm.address, team: addForm.team || undefined })
      setShowAdd(false)
      setAddForm({ name: '', address: '', team: '' })
      load()
    } catch (e: unknown) {
      setAddError(e instanceof Error ? e.message : '添加失败')
    }
    setAdding(false)
  }

  const load = async () => {
    try {
      const data = await broodAPI.listClaws({
        status: filter || undefined,
        team: teamFilter || undefined,
      })
      setClaws(data.claws || [])
    } catch {
      /* ignore */
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    setLoading(true)
    load()
  }, [filter, teamFilter])

  const handleRemove = async (id: string, name: string) => {
    if (!confirm(`确定移除节点 "${name}" 吗？`)) return
    try {
      await broodAPI.removeClaw(id)
      load()
    } catch {
      /* ignore */
    }
  }

  const teams = [...new Set(claws.map((c) => c.team).filter(Boolean))]

  return (
    <div className="p-8">
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-2xl font-bold text-white">节点管理</h1>
          <p className="text-sm text-gray-500 mt-1">管理虫群中的 Claw 节点</p>
        </div>
        <div className="flex gap-2">
          <select
            value={filter}
            onChange={(e) => { setFilter(e.target.value); setSearchParams(p => { e.target.value ? p.set('status', e.target.value) : p.delete('status'); return p }) }}
            className="bg-gray-800 text-gray-300 text-sm px-3 py-2 rounded-lg border border-gray-700 focus:outline-none focus:border-overlord-500"
          >
            <option value="">全部状态</option>
            <option value="online">在线</option>
            <option value="feral">失控</option>
            <option value="offline">离线</option>
          </select>
          {teams.length > 0 && (
            <select
              value={teamFilter}
              onChange={(e) => { setTeamFilter(e.target.value); setSearchParams(p => { e.target.value ? p.set('team', e.target.value) : p.delete('team'); return p }) }}
              className="bg-gray-800 text-gray-300 text-sm px-3 py-2 rounded-lg border border-gray-700 focus:outline-none focus:border-overlord-500"
            >
              <option value="">全部团队</option>
              {teams.map((t) => (
                <option key={t} value={t}>{t}</option>
              ))}
            </select>
          )}
          <button
            onClick={() => setShowAdd(true)}
            className="flex items-center gap-2 px-4 py-2 bg-overlord-600 hover:bg-overlord-500 text-white rounded-lg text-sm transition"
          >
            <Plus className="w-4 h-4" /> 添加节点
          </button>
        </div>
      </div>

      {loading ? (
        <div className="flex justify-center py-12">
          <div className="animate-spin w-6 h-6 border-2 border-overlord-500 border-t-transparent rounded-full" />
        </div>
      ) : claws.length === 0 ? (
        <div className="bg-gray-900 border border-gray-800 border-dashed rounded-xl p-12 text-center">
          <Server className="w-10 h-10 text-gray-600 mx-auto mb-3" />
          <p className="text-gray-400">没有匹配的节点</p>
        </div>
      ) : (
        <div className="space-y-2">
          {claws.map((claw) => {
            const status = claw.status as keyof typeof statusIcon
            const Icon = statusIcon[status] || WifiOff
            const colors = statusColor[status] || statusColor.offline
            const label = statusLabel[status] || status
            return (
              <div
                key={claw.id}
                className="bg-gray-900 border border-gray-800 rounded-xl p-4 flex items-center gap-4 hover:border-gray-700 transition group"
              >
                <div className={`w-10 h-10 rounded-lg ${colors} flex items-center justify-center shrink-0`}>
                  <Icon className="w-5 h-5" />
                </div>
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-2">
                    <Link
                      to={`/claws/${claw.id}`}
                      className="text-sm font-semibold text-white hover:text-overlord-300 transition"
                    >
                      {claw.name || '未命名'}
                    </Link>
                    <span className={`text-[10px] px-1.5 py-0.5 rounded ${colors}`}>{label}</span>
                    {claw.team && (
                      <span className="text-[10px] px-1.5 py-0.5 rounded bg-gray-800 text-gray-400">{claw.team}</span>
                    )}
                    {claw.version && (
                      <span className="text-[10px] text-gray-600">{claw.version}</span>
                    )}
                  </div>
                  <div className="flex items-center gap-4 mt-1 text-xs text-gray-500">
                    {claw.claw_id && <span className="font-mono truncate max-w-[200px]">{claw.claw_id}</span>}
                    <span>{claw.address}</span>
                  </div>
                </div>
                <div className="flex items-center gap-6 text-xs text-gray-400 shrink-0">
                  <div className="text-center">
                    <div className="text-white font-medium">{claw.cpu_percent.toFixed(0)}%</div>
                    <div>CPU</div>
                  </div>
                  <div className="text-center">
                    <div className="text-white font-medium">{claw.mem_percent.toFixed(0)}%</div>
                    <div>内存</div>
                  </div>
                  <div className="text-center">
                    <div className="text-white font-medium">{claw.tasks_running}</div>
                    <div>任务</div>
                  </div>
                </div>
                {(claw.web_url || claw.address) && (
                  <a
                    href={claw.web_url || claw.address}
                    target="_blank"
                    rel="noopener noreferrer"
                    onClick={(e) => e.stopPropagation()}
                    className="p-2 text-gray-500 hover:text-overlord-400 transition"
                    title={`打开节点管理界面 (${claw.web_url || claw.address})`}
                  >
                    <ExternalLink className="w-4 h-4" />
                  </a>
                )}
                <button
                  onClick={(e) => { e.preventDefault(); handleRemove(claw.id, claw.name) }}
                  className="opacity-0 group-hover:opacity-100 p-2 text-gray-500 hover:text-red-400 transition"
                  title="移除节点"
                >
                  <Trash2 className="w-4 h-4" />
                </button>
              </div>
            )
          })}
        </div>
      )}

      {/* Add Node Modal */}
      {showAdd && (
        <div className="fixed inset-0 bg-black/60 flex items-center justify-center z-50" onClick={() => setShowAdd(false)}>
          <div className="bg-gray-800 border border-gray-700 rounded-xl w-full max-w-md p-6" onClick={e => e.stopPropagation()}>
            <h3 className="text-lg font-bold text-white mb-4">添加节点</h3>
            <div className="space-y-4">
              <div>
                <label className="text-xs text-gray-400 mb-1 block">节点名称 *</label>
                <input
                  className="w-full bg-gray-900 border border-gray-700 rounded-lg px-3 py-2 text-sm text-white placeholder-gray-500 focus:border-overlord-500 focus:outline-none"
                  placeholder="如: claw-prod-01"
                  value={addForm.name}
                  onChange={e => setAddForm({ ...addForm, name: e.target.value })}
                />
              </div>
              <div>
                <label className="text-xs text-gray-400 mb-1 block">节点地址 *</label>
                <input
                  className="w-full bg-gray-900 border border-gray-700 rounded-lg px-3 py-2 text-sm text-white placeholder-gray-500 focus:border-overlord-500 focus:outline-none"
                  placeholder="如: https://claw.example.com:8080"
                  value={addForm.address}
                  onChange={e => setAddForm({ ...addForm, address: e.target.value })}
                />
                <div className="text-[10px] text-gray-600 mt-1">Claw 节点的可访问地址（含协议和端口）</div>
              </div>
              <div>
                <label className="text-xs text-gray-400 mb-1 block">所属团队</label>
                <input
                  className="w-full bg-gray-900 border border-gray-700 rounded-lg px-3 py-2 text-sm text-white placeholder-gray-500 focus:border-overlord-500 focus:outline-none"
                  placeholder="留空表示全局"
                  value={addForm.team}
                  onChange={e => setAddForm({ ...addForm, team: e.target.value })}
                />
              </div>
              {addError && <div className="text-xs text-red-400">{addError}</div>}
            </div>
            <div className="flex justify-end gap-3 mt-6">
              <button onClick={() => setShowAdd(false)} className="px-4 py-2 text-sm text-gray-400 hover:text-white transition">取消</button>
              <button
                onClick={handleAdd}
                disabled={adding || !addForm.name || !addForm.address}
                className="px-4 py-2 bg-overlord-600 hover:bg-overlord-500 text-white rounded-lg text-sm transition disabled:opacity-50"
              >
                {adding ? '添加中...' : '添加节点'}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
