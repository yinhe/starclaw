import { useEffect, useState } from 'react'
import { partner, type SwarmNode } from '../lib/api'
import { Server, Wifi, WifiOff, Cpu, HardDrive } from 'lucide-react'

const statusStyle: Record<string, string> = {
  online: 'bg-emerald-500/10 text-emerald-400',
  offline: 'bg-gray-500/10 text-gray-400',
  feral: 'bg-amber-500/10 text-amber-400',
}

const roleStyle: Record<string, string> = {
  claw: 'bg-claw-500/10 text-claw-400',
  overlord: 'bg-purple-500/10 text-purple-400',
}

export default function NodesPage() {
  const [nodes, setNodes] = useState<SwarmNode[]>([])
  const [loading, setLoading] = useState(true)
  const [filter, setFilter] = useState<{ role?: string; status?: string }>({})

  const load = async () => {
    try {
      const data = await partner.listNodes(filter)
      setNodes(data.nodes || [])
    } catch { /* */ }
    finally { setLoading(false) }
  }

  useEffect(() => { load() }, [filter.role, filter.status])

  const stats = {
    total: nodes.length,
    online: nodes.filter(n => n.status === 'online').length,
    offline: nodes.filter(n => n.status === 'offline').length,
    feral: nodes.filter(n => n.status === 'feral').length,
  }

  function timeAgo(ts: string) {
    if (!ts) return '--'
    const diff = (Date.now() - new Date(ts).getTime()) / 1000
    if (diff < 60) return `${Math.floor(diff)}s ago`
    if (diff < 3600) return `${Math.floor(diff / 60)}m ago`
    if (diff < 86400) return `${Math.floor(diff / 3600)}h ago`
    return `${Math.floor(diff / 86400)}d ago`
  }

  const tabs = [
    { label: '全部', value: undefined, count: stats.total },
    { label: '在线', value: 'online', count: stats.online },
    { label: '离线', value: 'offline', count: stats.offline },
    { label: '失联', value: 'feral', count: stats.feral },
  ]

  return (
    <div>
      <h1 className="text-2xl font-bold text-white mb-1">节点管理</h1>
      <p className="text-sm text-gray-500 mb-6">查看和管理所有 Claw 节点</p>

      {/* Stats cards */}
      <div className="grid grid-cols-4 gap-4 mb-6">
        <div className="bg-gray-900 border border-white/10 rounded-xl p-4">
          <div className="text-xs text-gray-500">总节点</div>
          <div className="text-2xl font-bold text-white mt-1">{stats.total}</div>
        </div>
        <div className="bg-gray-900 border border-white/10 rounded-xl p-4">
          <div className="text-xs text-gray-500">在线</div>
          <div className="text-2xl font-bold text-emerald-400 mt-1">{stats.online}</div>
        </div>
        <div className="bg-gray-900 border border-white/10 rounded-xl p-4">
          <div className="text-xs text-gray-500">离线</div>
          <div className="text-2xl font-bold text-gray-400 mt-1">{stats.offline}</div>
        </div>
        <div className="bg-gray-900 border border-white/10 rounded-xl p-4">
          <div className="text-xs text-gray-500">失联</div>
          <div className="text-2xl font-bold text-amber-400 mt-1">{stats.feral}</div>
        </div>
      </div>

      {/* Filters */}
      <div className="flex gap-2 mb-4">
        {tabs.map(t => (
          <button
            key={t.label}
            onClick={() => setFilter({ ...filter, status: t.value })}
            className={`px-3 py-1.5 rounded-lg text-sm transition-colors ${
              filter.status === t.value
                ? 'bg-claw-500/10 text-claw-400'
                : 'text-gray-400 hover:bg-white/5'
            }`}
          >
            {t.label} ({t.count})
          </button>
        ))}
      </div>

      {/* Table */}
      {loading ? (
        <div className="flex justify-center py-12">
          <div className="animate-spin w-6 h-6 border-2 border-claw-500 border-t-transparent rounded-full" />
        </div>
      ) : nodes.length === 0 ? (
        <div className="bg-gray-900 border border-white/10 border-dashed rounded-xl p-12 text-center">
          <Server className="w-10 h-10 text-gray-600 mx-auto mb-3" />
          <p className="text-gray-400">暂无节点</p>
        </div>
      ) : (
        <div className="bg-gray-900 border border-white/10 rounded-xl overflow-hidden">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-white/10 text-gray-500 text-left">
                <th className="px-4 py-3 font-medium">名称</th>
                <th className="px-4 py-3 font-medium">角色</th>
                <th className="px-4 py-3 font-medium">状态</th>
                <th className="px-4 py-3 font-medium">版本</th>
                <th className="px-4 py-3 font-medium">地区</th>
                <th className="px-4 py-3 font-medium">CPU</th>
                <th className="px-4 py-3 font-medium">内存</th>
                <th className="px-4 py-3 font-medium">任务</th>
                <th className="px-4 py-3 font-medium">心跳</th>
              </tr>
            </thead>
            <tbody>
              {nodes.map(node => (
                <tr key={node.id} className="border-b border-white/5 hover:bg-white/[0.02] transition-colors">
                  <td className="px-4 py-3">
                    <div className="flex items-center gap-2">
                      {node.status === 'online' ? (
                        <Wifi className="w-4 h-4 text-emerald-400 shrink-0" />
                      ) : (
                        <WifiOff className="w-4 h-4 text-gray-500 shrink-0" />
                      )}
                      <span className="text-white font-medium truncate max-w-[200px]">{node.name}</span>
                    </div>
                  </td>
                  <td className="px-4 py-3">
                    <span className={`text-xs px-1.5 py-0.5 rounded ${roleStyle[node.role] || 'bg-gray-500/10 text-gray-400'}`}>
                      {node.role}
                    </span>
                  </td>
                  <td className="px-4 py-3">
                    <span className={`text-xs px-1.5 py-0.5 rounded ${statusStyle[node.status] || statusStyle.offline}`}>
                      {node.status}
                    </span>
                  </td>
                  <td className="px-4 py-3 text-gray-400 font-mono text-xs">{node.version || '--'}</td>
                  <td className="px-4 py-3 text-gray-400">{node.region || '--'}</td>
                  <td className="px-4 py-3">
                    <div className="flex items-center gap-1.5">
                      <Cpu className="w-3.5 h-3.5 text-gray-500" />
                      <span className={node.cpu_percent > 80 ? 'text-red-400' : 'text-gray-400'}>
                        {node.cpu_percent?.toFixed(0) || 0}%
                      </span>
                    </div>
                  </td>
                  <td className="px-4 py-3">
                    <div className="flex items-center gap-1.5">
                      <HardDrive className="w-3.5 h-3.5 text-gray-500" />
                      <span className={node.mem_percent > 80 ? 'text-red-400' : 'text-gray-400'}>
                        {node.mem_percent?.toFixed(0) || 0}%
                      </span>
                    </div>
                  </td>
                  <td className="px-4 py-3 text-gray-400">{node.tasks_running || 0}</td>
                  <td className="px-4 py-3 text-gray-500 text-xs">{timeAgo(node.last_heartbeat)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}
