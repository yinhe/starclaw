import { useEffect, useState } from 'react'
import { Server, Users, Zap, ShoppingBag, Activity, AlertTriangle } from 'lucide-react'
import { overseerAPI, type DashboardData, type ServiceStatus } from '../lib/api'
import StatCard from '../components/StatCard'

export default function DashboardPage() {
  const [data, setData] = useState<DashboardData | null>(null)
  const [services, setServices] = useState<ServiceStatus[]>([])
  const [error, setError] = useState('')

  useEffect(() => {
    overseerAPI.dashboard().then(setData).catch((e) => setError(e.message))
    overseerAPI.services().then((r) => setServices(r.services)).catch(() => {})
  }, [])

  if (error) return <div className="p-6 text-red-400">加载失败: {error}</div>
  if (!data) return <div className="p-6 text-gray-500">加载中...</div>

  const upCount = services.filter((s) => s.status === 'up').length
  const downCount = services.filter((s) => s.status === 'down').length

  return (
    <div className="p-6 space-y-6">
      <h1 className="text-xl font-bold text-white">Dashboard</h1>

      {/* Stats grid */}
      <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
        <StatCard
          title="在线节点"
          value={data.nodes.online}
          subtitle={`共 ${data.nodes.total} 个节点`}
          icon={<Server className="w-5 h-5" />}
          color="green"
        />
        <StatCard
          title="用户数"
          value={data.users}
          icon={<Users className="w-5 h-5" />}
          color="blue"
        />
        <StatCard
          title="星能余额"
          value={`${data.energy.total_balance.toFixed(1)} ⚡`}
          subtitle={`${data.energy.total_accounts} 个账户`}
          icon={<Zap className="w-5 h-5" />}
          color="yellow"
        />
        <StatCard
          title="市场模板"
          value={data.marketplace}
          icon={<ShoppingBag className="w-5 h-5" />}
          color="purple"
        />
      </div>

      {/* Services status */}
      <div className="rounded-xl border border-gray-800 bg-gray-900/50 p-4">
        <h2 className="text-sm font-medium text-gray-400 mb-3 flex items-center gap-2">
          <Activity className="w-4 h-4" />
          服务状态
          <span className="ml-auto text-xs">
            <span className="text-green-400">{upCount} 在线</span>
            {downCount > 0 && <span className="text-red-400 ml-2">{downCount} 离线</span>}
          </span>
        </h2>
        <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-6 gap-2">
          {services.map((svc) => (
            <div
              key={svc.name}
              className={`rounded-lg px-3 py-2 text-center text-xs border ${
                svc.status === 'up'
                  ? 'bg-green-500/10 border-green-500/30 text-green-400'
                  : 'bg-red-500/10 border-red-500/30 text-red-400'
              }`}
            >
              <div className="font-medium">{svc.name}</div>
              <div className="mt-1 text-[10px] opacity-70">{svc.latency_ms}ms</div>
            </div>
          ))}
        </div>
      </div>

      {/* Energy economy summary */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        <div className="rounded-xl border border-gray-800 bg-gray-900/50 p-4">
          <div className="text-xs text-gray-400 mb-1">累计发放</div>
          <div className="text-lg font-bold text-green-400">{data.energy.total_granted.toFixed(1)} ⚡</div>
        </div>
        <div className="rounded-xl border border-gray-800 bg-gray-900/50 p-4">
          <div className="text-xs text-gray-400 mb-1">累计消耗</div>
          <div className="text-lg font-bold text-orange-400">{data.energy.total_consumed.toFixed(1)} ⚡</div>
        </div>
        <div className="rounded-xl border border-gray-800 bg-gray-900/50 p-4">
          <div className="text-xs text-gray-400 mb-1">流通余额</div>
          <div className="text-lg font-bold text-yellow-400">{data.energy.total_balance.toFixed(1)} ⚡</div>
        </div>
      </div>

      {/* Alerts preview */}
      <AlertsPreview />
    </div>
  )
}

function AlertsPreview() {
  const [alerts, setAlerts] = useState<Array<{ labels: Record<string, string>; state: string }>>([])

  useEffect(() => {
    overseerAPI.alerts().then((r) => {
      if (r?.data?.alerts) setAlerts(r.data.alerts)
    }).catch(() => {})
  }, [])

  const firing = alerts.filter((a) => a.state === 'firing')

  if (firing.length === 0) {
    return (
      <div className="rounded-xl border border-gray-800 bg-gray-900/50 p-4 text-sm text-green-400 flex items-center gap-2">
        <AlertTriangle className="w-4 h-4" />
        无活跃告警
      </div>
    )
  }

  return (
    <div className="rounded-xl border border-red-500/30 bg-red-500/5 p-4">
      <h2 className="text-sm font-medium text-red-400 mb-2 flex items-center gap-2">
        <AlertTriangle className="w-4 h-4" />
        活跃告警 ({firing.length})
      </h2>
      <div className="space-y-1">
        {firing.slice(0, 5).map((a, i) => (
          <div key={i} className="text-xs text-red-300 bg-red-500/10 rounded px-2 py-1">
            {a.labels.alertname} — {a.labels.job || a.labels.instance || ''}
          </div>
        ))}
      </div>
    </div>
  )
}
