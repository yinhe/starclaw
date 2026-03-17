import { useEffect, useState } from 'react'
import { api } from '../api'
import { Server, Wifi, WifiOff, Activity, Zap, AlertTriangle, RefreshCw } from 'lucide-react'

interface OverseerDashboard {
  nodes_total: number
  nodes_online: number
  total_users: number
  total_revenue: number
  active_alerts: number
  services: { name: string; status: string; latency_ms: number }[]
}

interface OverseerNode {
  id: string
  name: string
  role: string
  status: string
  version: string
  address: string
  region: string
  cpu_percent: number
  mem_percent: number
  tasks_running: number
  last_heartbeat: string
}

interface OverseerService {
  name: string
  status: string
  url: string
  latency_ms: number
  version: string
  uptime: string
}

interface EnergyStats {
  total_accounts: number
  total_balance: number
  total_consumed: number
  today_consumed: number
}

export default function OverseerPage() {
  const [tab, setTab] = useState<'dashboard' | 'nodes' | 'services' | 'energy'>('dashboard')
  const [dashboard, setDashboard] = useState<OverseerDashboard | null>(null)
  const [nodes, setNodes] = useState<OverseerNode[]>([])
  const [services, setServices] = useState<OverseerService[]>([])
  const [energy, setEnergy] = useState<EnergyStats | null>(null)
  const [alerts, setAlerts] = useState<{ id: string; name: string; state: string; severity: string; summary: string }[]>([])
  const [loading, setLoading] = useState(true)
  const [refreshing, setRefreshing] = useState(false)

  const load = async (manual = false) => {
    if (manual) setRefreshing(true)
    try {
      const [d, n, s, e, a] = await Promise.all([
        api.get<OverseerDashboard>('/v1/admin/overseer/dashboard').catch(() => null),
        api.get<{ nodes: OverseerNode[] }>('/v1/admin/overseer/nodes').catch(() => ({ nodes: [] })),
        api.get<{ services: OverseerService[] }>('/v1/admin/overseer/services').catch(() => ({ services: [] })),
        api.get<EnergyStats>('/v1/admin/overseer/energy').catch(() => null),
        api.get<{ alerts: { id: string; name: string; state: string; severity: string; summary: string }[] }>('/v1/admin/overseer/alerts').catch(() => ({ alerts: [] })),
      ])
      setDashboard(d)
      setNodes(n.nodes || [])
      setServices(s.services || [])
      setEnergy(e)
      setAlerts(a.alerts || [])
    } catch {}
    setLoading(false)
    setRefreshing(false)
  }

  useEffect(() => { load() }, [])

  if (loading) return <div className="text-gray-500 text-center py-20">加载中...</div>

  const onlineNodes = nodes.filter(n => n.status === 'online').length

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <h2 className="text-xl font-bold">Overseer 监控</h2>
        <button onClick={() => load(true)} disabled={refreshing}
          className="flex items-center gap-1.5 px-3 py-1.5 bg-gray-800 text-gray-400 rounded-lg text-xs hover:bg-gray-700 disabled:opacity-50 transition">
          <RefreshCw size={12} className={refreshing ? 'animate-spin' : ''} /> 刷新
        </button>
      </div>

      {/* Top stats */}
      <div className="grid grid-cols-5 gap-4 mb-6">
        <div className="bg-gray-900 border border-gray-800 rounded-xl p-4">
          <div className="flex items-center gap-2 text-gray-500 text-xs mb-2"><Server size={14} /> 总节点</div>
          <div className="text-2xl font-bold text-white">{dashboard?.nodes_total ?? nodes.length}</div>
        </div>
        <div className="bg-gray-900 border border-gray-800 rounded-xl p-4">
          <div className="flex items-center gap-2 text-gray-500 text-xs mb-2"><Wifi size={14} /> 在线</div>
          <div className="text-2xl font-bold text-green-400">{dashboard?.nodes_online ?? onlineNodes}</div>
        </div>
        <div className="bg-gray-900 border border-gray-800 rounded-xl p-4">
          <div className="flex items-center gap-2 text-gray-500 text-xs mb-2"><Activity size={14} /> 服务</div>
          <div className="text-2xl font-bold text-blue-400">{services.length}</div>
        </div>
        <div className="bg-gray-900 border border-gray-800 rounded-xl p-4">
          <div className="flex items-center gap-2 text-gray-500 text-xs mb-2"><Zap size={14} /> 星能消耗</div>
          <div className="text-2xl font-bold text-amber-400">{energy ? energy.today_consumed.toLocaleString() : '--'}</div>
        </div>
        <div className="bg-gray-900 border border-gray-800 rounded-xl p-4">
          <div className="flex items-center gap-2 text-gray-500 text-xs mb-2"><AlertTriangle size={14} /> 告警</div>
          <div className={`text-2xl font-bold ${alerts.length > 0 ? 'text-red-400' : 'text-green-400'}`}>{alerts.length}</div>
        </div>
      </div>

      {/* Alerts banner */}
      {alerts.length > 0 && (
        <div className="rounded-xl bg-red-500/10 border border-red-500/20 p-4 mb-6">
          <h3 className="text-sm font-medium text-red-400 mb-2">活跃告警 ({alerts.length})</h3>
          <div className="space-y-1.5">
            {alerts.slice(0, 5).map(a => (
              <div key={a.id || a.name} className="flex items-center gap-3 text-xs">
                <span className={`w-2 h-2 rounded-full ${a.severity === 'critical' ? 'bg-red-500' : 'bg-amber-500'}`} />
                <span className="text-white font-medium">{a.name}</span>
                <span className="text-gray-400 flex-1 truncate">{a.summary}</span>
                <span className={`px-1.5 py-0.5 rounded ${a.state === 'firing' ? 'text-red-400 bg-red-500/10' : 'text-amber-400 bg-amber-500/10'}`}>{a.state}</span>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Tabs */}
      <div className="flex gap-1 border-b border-gray-800 pb-px mb-6">
        {([
          { key: 'nodes', label: '节点' },
          { key: 'services', label: '服务' },
          { key: 'energy', label: '星能' },
        ] as const).map(t => (
          <button key={t.key} onClick={() => setTab(t.key)}
            className={`px-4 py-2.5 text-sm rounded-t-lg transition-colors ${
              tab === t.key ? 'bg-gray-800 text-white font-medium' : 'text-gray-400 hover:text-gray-200 hover:bg-gray-800/50'
            }`}>
            {t.label}
          </button>
        ))}
      </div>

      {/* Nodes */}
      {tab === 'nodes' && (
        <div className="bg-gray-900 border border-gray-800 rounded-xl overflow-hidden">
          <table className="w-full text-xs">
            <thead>
              <tr className="border-b border-gray-800 text-gray-400 text-left">
                <th className="px-4 py-3 font-medium">名称</th>
                <th className="px-4 py-3 font-medium">角色</th>
                <th className="px-4 py-3 font-medium">状态</th>
                <th className="px-4 py-3 font-medium">版本</th>
                <th className="px-4 py-3 font-medium">地区</th>
                <th className="px-4 py-3 font-medium text-right">CPU</th>
                <th className="px-4 py-3 font-medium text-right">内存</th>
                <th className="px-4 py-3 font-medium text-right">任务</th>
                <th className="px-4 py-3 font-medium">心跳</th>
              </tr>
            </thead>
            <tbody>
              {nodes.map(n => (
                <tr key={n.id} className="border-b border-gray-800/50 hover:bg-gray-800/30">
                  <td className="px-4 py-3 text-white font-medium">{n.name || n.id.slice(0, 8)}</td>
                  <td className="px-4 py-3 text-gray-400">{n.role}</td>
                  <td className="px-4 py-3">
                    <span className={`flex items-center gap-1.5 ${n.status === 'online' ? 'text-green-400' : 'text-red-400'}`}>
                      {n.status === 'online' ? <Wifi size={12} /> : <WifiOff size={12} />}
                      {n.status}
                    </span>
                  </td>
                  <td className="px-4 py-3 text-gray-400 font-mono">{n.version || '-'}</td>
                  <td className="px-4 py-3 text-gray-400">{n.region || '-'}</td>
                  <td className="px-4 py-3 text-right">
                    <span className={n.cpu_percent > 80 ? 'text-red-400' : 'text-gray-300'}>{n.cpu_percent.toFixed(1)}%</span>
                  </td>
                  <td className="px-4 py-3 text-right">
                    <span className={n.mem_percent > 80 ? 'text-red-400' : 'text-gray-300'}>{n.mem_percent.toFixed(1)}%</span>
                  </td>
                  <td className="px-4 py-3 text-right text-gray-300">{n.tasks_running}</td>
                  <td className="px-4 py-3 text-gray-500">{n.last_heartbeat ? new Date(n.last_heartbeat).toLocaleTimeString() : '-'}</td>
                </tr>
              ))}
              {nodes.length === 0 && (
                <tr><td colSpan={9} className="px-4 py-8 text-center text-gray-600">暂无节点数据</td></tr>
              )}
            </tbody>
          </table>
        </div>
      )}

      {/* Services */}
      {tab === 'services' && (
        <div className="grid grid-cols-2 gap-4">
          {services.length > 0 ? services.map(s => (
            <div key={s.name} className="bg-gray-900 border border-gray-800 rounded-xl p-4 flex items-center gap-4">
              <div className={`w-3 h-3 rounded-full ${s.status === 'ok' || s.status === 'healthy' ? 'bg-green-500' : s.status === 'degraded' ? 'bg-amber-500' : 'bg-red-500'}`} />
              <div className="flex-1 min-w-0">
                <div className="text-sm text-white font-medium">{s.name}</div>
                <div className="text-xs text-gray-500 truncate">{s.url || '-'}</div>
              </div>
              <div className="text-right shrink-0">
                <div className="text-xs text-gray-300">{s.latency_ms > 0 ? `${s.latency_ms}ms` : '-'}</div>
                {s.version && <div className="text-[10px] text-gray-600">{s.version}</div>}
              </div>
            </div>
          )) : (
            <div className="col-span-2 bg-gray-900 border border-gray-800 border-dashed rounded-xl p-8 text-center text-gray-600">
              暂无服务数据
            </div>
          )}
        </div>
      )}

      {/* Energy */}
      {tab === 'energy' && (
        <div>
          {energy ? (
            <div className="grid grid-cols-4 gap-4">
              <div className="bg-gray-900 border border-gray-800 rounded-xl p-5">
                <div className="text-xs text-gray-500 mb-2">星能账户</div>
                <div className="text-2xl font-bold text-white">{energy.total_accounts.toLocaleString()}</div>
              </div>
              <div className="bg-gray-900 border border-gray-800 rounded-xl p-5">
                <div className="text-xs text-gray-500 mb-2">总余额</div>
                <div className="text-2xl font-bold text-amber-400">{energy.total_balance.toLocaleString()}</div>
              </div>
              <div className="bg-gray-900 border border-gray-800 rounded-xl p-5">
                <div className="text-xs text-gray-500 mb-2">累计消耗</div>
                <div className="text-2xl font-bold text-purple-400">{energy.total_consumed.toLocaleString()}</div>
              </div>
              <div className="bg-gray-900 border border-gray-800 rounded-xl p-5">
                <div className="text-xs text-gray-500 mb-2">今日消耗</div>
                <div className="text-2xl font-bold text-blue-400">{energy.today_consumed.toLocaleString()}</div>
              </div>
            </div>
          ) : (
            <div className="bg-gray-900 border border-gray-800 border-dashed rounded-xl p-8 text-center text-gray-600">
              暂无星能数据
            </div>
          )}
        </div>
      )}
    </div>
  )
}
