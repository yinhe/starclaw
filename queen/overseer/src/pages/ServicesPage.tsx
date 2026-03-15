import { useEffect, useState } from 'react'
import { Activity, RefreshCw, CheckCircle, XCircle } from 'lucide-react'
import { overseerAPI, type ServiceStatus } from '../lib/api'

export default function ServicesPage() {
  const [services, setServices] = useState<ServiceStatus[]>([])
  const [loading, setLoading] = useState(true)
  const [lastRefresh, setLastRefresh] = useState<Date | null>(null)

  const refresh = () => {
    setLoading(true)
    overseerAPI.services().then((r) => {
      setServices(r.services || [])
      setLastRefresh(new Date())
    }).catch(() => {}).finally(() => setLoading(false))
  }

  useEffect(() => { refresh() }, [])

  return (
    <div className="p-6 space-y-4">
      <div className="flex items-center justify-between">
        <h1 className="text-xl font-bold text-white flex items-center gap-2">
          <Activity className="w-5 h-5 text-purple-400" />
          服务健康
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

      {lastRefresh && (
        <div className="text-xs text-gray-600">
          最后检查: {lastRefresh.toLocaleTimeString()}
        </div>
      )}

      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
        {services.map((svc) => (
          <div
            key={svc.name}
            className={`rounded-xl border p-5 transition-all ${
              svc.status === 'up'
                ? 'border-green-500/30 bg-green-500/5'
                : 'border-red-500/30 bg-red-500/5'
            }`}
          >
            <div className="flex items-center justify-between mb-3">
              <span className="font-medium text-white">{svc.name}</span>
              {svc.status === 'up' ? (
                <CheckCircle className="w-5 h-5 text-green-400" />
              ) : (
                <XCircle className="w-5 h-5 text-red-400" />
              )}
            </div>
            <div className="flex items-baseline gap-2">
              <span className={`text-2xl font-bold ${svc.status === 'up' ? 'text-green-400' : 'text-red-400'}`}>
                {svc.status === 'up' ? 'UP' : 'DOWN'}
              </span>
              <span className="text-xs text-gray-500">{svc.latency_ms}ms</span>
            </div>
            <div className="mt-3 h-1 rounded-full bg-gray-800 overflow-hidden">
              <div
                className={`h-full rounded-full transition-all ${
                  svc.status === 'up' ? 'bg-green-500' : 'bg-red-500'
                }`}
                style={{ width: svc.status === 'up' ? '100%' : '0%' }}
              />
            </div>
          </div>
        ))}
      </div>
    </div>
  )
}
