import { useEffect, useState } from 'react'
import { ScrollText, User, Clock } from 'lucide-react'
import { broodAPI, type AuditLogEntry } from '../api/brood'

const actionColors: Record<string, string> = {
  register: 'bg-emerald-600/15 text-emerald-400',
  remove: 'bg-red-600/15 text-red-400',
  update_quota: 'bg-blue-600/15 text-blue-400',
  assign_task: 'bg-purple-600/15 text-purple-400',
}

export default function AuditPage() {
  const [logs, setLogs] = useState<AuditLogEntry[]>([])
  const [loading, setLoading] = useState(true)

  const load = async () => {
    try {
      const data = await broodAPI.auditLogs()
      setLogs(data.logs || [])
    } catch {
      /* ignore */
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => { load() }, [])

  const formatTime = (dateStr: string) => {
    return new Date(dateStr).toLocaleString('zh-CN', {
      month: '2-digit',
      day: '2-digit',
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit',
    })
  }

  return (
    <div className="p-8">
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-2xl font-bold text-white">审计日志</h1>
          <p className="text-sm text-gray-500 mt-1">所有管理操作记录</p>
        </div>
        <button
          onClick={load}
          className="px-4 py-2 bg-gray-800 text-gray-300 rounded-lg text-sm hover:bg-gray-700 transition"
        >
          刷新
        </button>
      </div>

      {loading ? (
        <div className="flex justify-center py-12">
          <div className="animate-spin w-6 h-6 border-2 border-overlord-500 border-t-transparent rounded-full" />
        </div>
      ) : logs.length === 0 ? (
        <div className="bg-gray-900 border border-gray-800 border-dashed rounded-xl p-12 text-center">
          <ScrollText className="w-10 h-10 text-gray-600 mx-auto mb-3" />
          <p className="text-gray-400">暂无审计日志</p>
        </div>
      ) : (
        <div className="space-y-1">
          {logs.map((log) => {
            const color = actionColors[log.action] || 'bg-gray-600/15 text-gray-400'
            return (
              <div
                key={log.id}
                className="bg-gray-900 border border-gray-800 rounded-lg px-4 py-3 flex items-center gap-4"
              >
                <span className={`text-xs px-2 py-1 rounded font-medium shrink-0 ${color}`}>
                  {log.action}
                </span>
                <div className="flex-1 min-w-0">
                  <p className="text-sm text-gray-200 truncate">{log.detail}</p>
                  {log.target_id && (
                    <p className="text-xs text-gray-600 font-mono mt-0.5 truncate">{log.target_id}</p>
                  )}
                </div>
                <div className="flex items-center gap-4 text-xs text-gray-500 shrink-0">
                  <div className="flex items-center gap-1">
                    <User className="w-3 h-3" />
                    <span>{log.actor}</span>
                  </div>
                  <div className="flex items-center gap-1">
                    <Clock className="w-3 h-3" />
                    <span>{formatTime(log.created_at)}</span>
                  </div>
                </div>
              </div>
            )
          })}
        </div>
      )}
    </div>
  )
}
