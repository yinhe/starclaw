import { useEffect, useState } from 'react'
import { Server, Wifi, WifiOff, Cpu } from 'lucide-react'
import { overseerAPI, type NodeInfo } from '../lib/api'

export default function NodesPage() {
  const [nodes, setNodes] = useState<NodeInfo[]>([])
  const [total, setTotal] = useState(0)
  const [filter, setFilter] = useState('')
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    setLoading(true)
    overseerAPI.nodes(1, 100, filter).then((r) => {
      setNodes(r.nodes || [])
      setTotal(r.total || 0)
    }).catch(() => {}).finally(() => setLoading(false))
  }, [filter])

  const timeAgo = (ts: string) => {
    if (!ts) return '—'
    const diff = Date.now() - new Date(ts).getTime()
    if (diff < 60000) return `${Math.floor(diff / 1000)}s ago`
    if (diff < 3600000) return `${Math.floor(diff / 60000)}m ago`
    if (diff < 86400000) return `${Math.floor(diff / 3600000)}h ago`
    return `${Math.floor(diff / 86400000)}d ago`
  }

  return (
    <div className="p-6 space-y-4">
      <div className="flex items-center justify-between">
        <h1 className="text-xl font-bold text-white flex items-center gap-2">
          <Server className="w-5 h-5 text-purple-400" />
          节点管理
          <span className="text-sm font-normal text-gray-500 ml-2">共 {total} 个</span>
        </h1>
        <div className="flex gap-2">
          {['', 'online', 'offline'].map((s) => (
            <button
              key={s}
              onClick={() => setFilter(s)}
              className={`px-3 py-1 text-xs rounded-lg border transition-colors ${
                filter === s
                  ? 'border-purple-500/50 bg-purple-500/15 text-purple-400'
                  : 'border-gray-700 text-gray-400 hover:border-gray-600'
              }`}
            >
              {s === '' ? '全部' : s === 'online' ? '在线' : '离线'}
            </button>
          ))}
        </div>
      </div>

      {loading ? (
        <div className="text-gray-500 text-sm">加载中...</div>
      ) : nodes.length === 0 ? (
        <div className="text-gray-500 text-sm">暂无节点</div>
      ) : (
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="text-xs text-gray-500 border-b border-gray-800">
                <th className="text-left py-2 px-3">状态</th>
                <th className="text-left py-2 px-3">名称</th>
                <th className="text-left py-2 px-3">Claw ID</th>
                <th className="text-left py-2 px-3">版本</th>
                <th className="text-left py-2 px-3">地区</th>
                <th className="text-left py-2 px-3">算力</th>
                <th className="text-left py-2 px-3">最后心跳</th>
              </tr>
            </thead>
            <tbody>
              {nodes.map((node) => (
                <tr key={node.id} className="border-b border-gray-800/50 hover:bg-gray-800/30 transition-colors">
                  <td className="py-2 px-3">
                    {node.status === 'online' ? (
                      <Wifi className="w-4 h-4 text-green-400" />
                    ) : (
                      <WifiOff className="w-4 h-4 text-gray-600" />
                    )}
                  </td>
                  <td className="py-2 px-3 text-white font-medium">{node.name || '—'}</td>
                  <td className="py-2 px-3 text-gray-400 font-mono text-xs">
                    {node.claw_id ? `${node.claw_id.slice(0, 12)}...` : '—'}
                  </td>
                  <td className="py-2 px-3">
                    <span className="px-1.5 py-0.5 bg-gray-800 rounded text-xs text-gray-300">
                      {node.version || '—'}
                    </span>
                  </td>
                  <td className="py-2 px-3 text-gray-400">{node.region || '—'}</td>
                  <td className="py-2 px-3">
                    {node.is_contributor ? (
                      <span className="flex items-center gap-1 text-cyan-400 text-xs">
                        <Cpu className="w-3 h-3" /> 贡献中
                      </span>
                    ) : (
                      <span className="text-gray-600 text-xs">—</span>
                    )}
                  </td>
                  <td className="py-2 px-3 text-gray-500 text-xs">{timeAgo(node.last_heartbeat)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}
