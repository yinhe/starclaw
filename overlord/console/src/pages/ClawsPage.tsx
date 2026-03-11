import { useEffect, useState } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { Server, Wifi, WifiOff, AlertTriangle, Trash2 } from 'lucide-react'
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
    </div>
  )
}
