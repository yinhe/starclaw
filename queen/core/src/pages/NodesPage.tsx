import { useEffect, useState } from 'react'
import { api, type SwarmNode } from '../api'
import { Trash2, RefreshCw } from 'lucide-react'

function statusBadge(status: string) {
  const map: Record<string, string> = {
    online: 'bg-green-500/20 text-green-400',
    offline: 'bg-gray-500/20 text-gray-400',
    feral: 'bg-red-500/20 text-red-400',
  }
  return map[status] || 'bg-gray-500/20 text-gray-400'
}

function timeAgo(ts: string): string {
  const diff = (Date.now() - new Date(ts).getTime()) / 1000
  if (diff < 60) return `${Math.floor(diff)}秒前`
  if (diff < 3600) return `${Math.floor(diff / 60)}分钟前`
  if (diff < 86400) return `${Math.floor(diff / 3600)}小时前`
  return `${Math.floor(diff / 86400)}天前`
}

export default function NodesPage() {
  const [nodes, setNodes] = useState<SwarmNode[]>([])
  const [loading, setLoading] = useState(true)

  const fetchNodes = () => {
    setLoading(true)
    api.get<{ nodes: SwarmNode[] }>('/v1/admin/nodes')
      .then((d) => setNodes(d.nodes || []))
      .catch(() => {})
      .finally(() => setLoading(false))
  }

  useEffect(() => { fetchNodes() }, [])

  const handleRemove = async (id: string, name: string) => {
    if (!confirm(`确认移除节点 "${name}"？`)) return
    await api.delete(`/v1/admin/nodes/${id}`)
    fetchNodes()
  }

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <h2 className="text-xl font-bold">节点管理</h2>
        <button
          onClick={fetchNodes}
          className="flex items-center gap-2 px-3 py-2 bg-gray-800 hover:bg-gray-700 rounded-lg text-sm text-gray-300 transition-colors"
        >
          <RefreshCw size={14} />
          刷新
        </button>
      </div>

      {loading ? (
        <div className="text-gray-500 text-center py-20">加载中...</div>
      ) : nodes.length === 0 ? (
        <div className="text-gray-500 text-center py-20">暂无节点</div>
      ) : (
        <div className="bg-gray-900 border border-gray-800 rounded-xl overflow-hidden">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-gray-800 text-gray-400 text-left">
                <th className="px-4 py-3 font-medium">名称</th>
                <th className="px-4 py-3 font-medium">角色</th>
                <th className="px-4 py-3 font-medium">状态</th>
                <th className="px-4 py-3 font-medium">版本</th>
                <th className="px-4 py-3 font-medium">地区</th>
                <th className="px-4 py-3 font-medium">CPU</th>
                <th className="px-4 py-3 font-medium">内存</th>
                <th className="px-4 py-3 font-medium">最后心跳</th>
                <th className="px-4 py-3 font-medium">操作</th>
              </tr>
            </thead>
            <tbody>
              {nodes.map((node) => (
                <tr key={node.id} className="border-b border-gray-800/50 hover:bg-gray-800/30">
                  <td className="px-4 py-3 text-gray-100 font-medium">{node.name}</td>
                  <td className="px-4 py-3 text-gray-400">{node.role}</td>
                  <td className="px-4 py-3">
                    <span className={`px-2 py-0.5 rounded-full text-xs ${statusBadge(node.status)}`}>
                      {node.status}
                    </span>
                  </td>
                  <td className="px-4 py-3 text-gray-400">{node.version}</td>
                  <td className="px-4 py-3 text-gray-400">{node.region || '--'}</td>
                  <td className="px-4 py-3 text-gray-400">{node.cpu_percent?.toFixed(1)}%</td>
                  <td className="px-4 py-3 text-gray-400">{node.mem_percent?.toFixed(1)}%</td>
                  <td className="px-4 py-3 text-gray-500 text-xs">
                    {node.last_heartbeat ? timeAgo(node.last_heartbeat) : '--'}
                  </td>
                  <td className="px-4 py-3">
                    <button
                      onClick={() => handleRemove(node.id, node.name)}
                      className="p-1.5 rounded hover:bg-red-500/20 text-gray-500 hover:text-red-400 transition-colors"
                      title="移除节点"
                    >
                      <Trash2 size={14} />
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}
