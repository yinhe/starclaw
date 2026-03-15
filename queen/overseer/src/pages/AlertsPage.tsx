import { useEffect, useState } from 'react'
import { AlertTriangle, RefreshCw, CheckCircle, Bell } from 'lucide-react'
import { overseerAPI, type Alert } from '../lib/api'

export default function AlertsPage() {
  const [alerts, setAlerts] = useState<Alert[]>([])
  const [loading, setLoading] = useState(true)

  const refresh = () => {
    setLoading(true)
    overseerAPI.alerts().then((r) => {
      setAlerts(r?.data?.alerts || [])
    }).catch(() => {}).finally(() => setLoading(false))
  }

  useEffect(() => { refresh() }, [])

  const firing = alerts.filter((a) => a.state === 'firing')
  const pending = alerts.filter((a) => a.state === 'pending')
  const inactive = alerts.filter((a) => a.state === 'inactive')

  return (
    <div className="p-6 space-y-4">
      <div className="flex items-center justify-between">
        <h1 className="text-xl font-bold text-white flex items-center gap-2">
          <AlertTriangle className="w-5 h-5 text-yellow-400" />
          告警中心
        </h1>
        <button
          onClick={refresh}
          disabled={loading}
          className="flex items-center gap-1 px-3 py-1 text-xs rounded-lg border border-gray-700 text-gray-400 hover:border-purple-500/50 hover:text-purple-400 transition-colors disabled:opacity-50"
        >
          <RefreshCw className={`w-3 h-3 ${loading ? 'animate-spin' : ''}`} />
          刷新
        </button>
      </div>

      {/* Summary */}
      <div className="flex gap-3">
        <div className={`px-3 py-1.5 rounded-lg text-sm flex items-center gap-2 ${
          firing.length > 0 ? 'bg-red-500/10 border border-red-500/30 text-red-400' : 'bg-gray-800 text-gray-500'
        }`}>
          <Bell className="w-4 h-4" />
          触发中 {firing.length}
        </div>
        <div className="px-3 py-1.5 rounded-lg text-sm bg-yellow-500/10 border border-yellow-500/30 text-yellow-400 flex items-center gap-2">
          Pending {pending.length}
        </div>
        <div className="px-3 py-1.5 rounded-lg text-sm bg-gray-800 text-gray-500 flex items-center gap-2">
          <CheckCircle className="w-4 h-4" />
          正常 {inactive.length}
        </div>
      </div>

      {/* Firing alerts */}
      {firing.length > 0 && (
        <div className="space-y-2">
          <h2 className="text-sm font-medium text-red-400">🔥 触发中</h2>
          {firing.map((a, i) => (
            <AlertCard key={i} alert={a} severity="firing" />
          ))}
        </div>
      )}

      {/* Pending alerts */}
      {pending.length > 0 && (
        <div className="space-y-2">
          <h2 className="text-sm font-medium text-yellow-400">⏳ Pending</h2>
          {pending.map((a, i) => (
            <AlertCard key={i} alert={a} severity="pending" />
          ))}
        </div>
      )}

      {/* All clear */}
      {firing.length === 0 && pending.length === 0 && !loading && (
        <div className="rounded-xl border border-green-500/30 bg-green-500/5 p-6 text-center">
          <CheckCircle className="w-8 h-8 text-green-400 mx-auto mb-2" />
          <div className="text-green-400 font-medium">全部正常</div>
          <div className="text-xs text-gray-500 mt-1">无活跃告警</div>
        </div>
      )}

      {/* Inactive rules */}
      {inactive.length > 0 && (
        <div className="space-y-2">
          <h2 className="text-sm font-medium text-gray-500">✅ 正常规则 ({inactive.length})</h2>
          <div className="grid grid-cols-1 md:grid-cols-2 gap-2">
            {inactive.map((a, i) => (
              <div key={i} className="rounded-lg border border-gray-800 bg-gray-900/30 px-3 py-2 text-xs text-gray-500 flex items-center gap-2">
                <CheckCircle className="w-3 h-3 text-green-600" />
                {a.labels.alertname}
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  )
}

function AlertCard({ alert, severity }: { alert: Alert; severity: 'firing' | 'pending' }) {
  const colors = severity === 'firing'
    ? 'border-red-500/30 bg-red-500/5'
    : 'border-yellow-500/30 bg-yellow-500/5'

  return (
    <div className={`rounded-xl border p-4 ${colors}`}>
      <div className="flex items-center justify-between mb-2">
        <span className={`font-medium ${severity === 'firing' ? 'text-red-400' : 'text-yellow-400'}`}>
          {alert.labels.alertname}
        </span>
        <span className={`text-xs px-2 py-0.5 rounded ${
          alert.labels.severity === 'critical' ? 'bg-red-500/20 text-red-400' :
          alert.labels.severity === 'warning' ? 'bg-yellow-500/20 text-yellow-400' :
          'bg-gray-700 text-gray-400'
        }`}>
          {alert.labels.severity || 'info'}
        </span>
      </div>
      {alert.annotations?.summary && (
        <div className="text-sm text-gray-300 mb-1">{alert.annotations.summary}</div>
      )}
      {alert.annotations?.description && (
        <div className="text-xs text-gray-500">{alert.annotations.description}</div>
      )}
      <div className="mt-2 flex gap-3 text-xs text-gray-600">
        {alert.labels.job && <span>job: {alert.labels.job}</span>}
        {alert.labels.instance && <span>instance: {alert.labels.instance}</span>}
        {alert.activeAt && <span>since: {new Date(alert.activeAt).toLocaleString('zh-CN')}</span>}
      </div>
    </div>
  )
}
