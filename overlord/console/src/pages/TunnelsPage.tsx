import { useEffect, useState } from 'react'
import { Network, Plus, Trash2, ArrowRightLeft, AlertCircle } from 'lucide-react'
import { broodAPI, type NydusTunnel } from '../api/brood'

const statusColors: Record<string, string> = {
  active: 'bg-emerald-600/10 text-emerald-400',
  pending: 'bg-yellow-600/10 text-yellow-400',
  error: 'bg-red-600/10 text-red-400',
  closed: 'bg-gray-600/10 text-gray-400',
}

function fmtBytes(b: number): string {
  if (b < 1024) return b + ' B'
  if (b < 1048576) return (b / 1024).toFixed(1) + ' KB'
  if (b < 1073741824) return (b / 1048576).toFixed(1) + ' MB'
  return (b / 1073741824).toFixed(2) + ' GB'
}

export default function TunnelsPage() {
  const [tunnels, setTunnels] = useState<NydusTunnel[]>([])
  const [loading, setLoading] = useState(true)
  const [filter, setFilter] = useState('')
  const [showCreate, setShowCreate] = useState(false)
  const [form, setForm] = useState({ claw_node_id: '', local_port: 0, remote_port: 0, protocol: 'tcp', mode: 'forward' })

  const load = async () => {
    try {
      const data = await broodAPI.listTunnels({ status: filter || undefined })
      setTunnels(data.tunnels || [])
    } catch { /* */ }
    finally { setLoading(false) }
  }

  useEffect(() => { setLoading(true); load() }, [filter])

  const handleCreate = async () => {
    if (!form.claw_node_id || !form.local_port || !form.remote_port) return
    try {
      await broodAPI.createTunnel(form)
      setShowCreate(false)
      setForm({ claw_node_id: '', local_port: 0, remote_port: 0, protocol: 'tcp', mode: 'forward' })
      load()
    } catch { /* */ }
  }

  const handleDelete = async (id: string) => {
    if (!confirm('确定删除此隧道吗？')) return
    try { await broodAPI.deleteTunnel(id); load() } catch { /* */ }
  }

  return (
    <div className="p-8">
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-2xl font-bold text-white">Nydus 隧道</h1>
          <p className="text-sm text-gray-500 mt-1">管理 Overlord ↔ Claw 之间的网络隧道</p>
        </div>
        <div className="flex gap-2">
          <select value={filter} onChange={e => setFilter(e.target.value)}
            className="bg-gray-800 text-gray-300 text-sm px-3 py-2 rounded-lg border border-gray-700 focus:outline-none focus:border-overlord-500">
            <option value="">全部状态</option>
            <option value="active">活跃</option>
            <option value="pending">等待</option>
            <option value="error">错误</option>
            <option value="closed">关闭</option>
          </select>
          <button onClick={() => setShowCreate(!showCreate)}
            className="flex items-center gap-2 px-4 py-2 bg-overlord-600 text-white rounded-lg text-sm hover:bg-overlord-500 transition">
            <Plus className="w-4 h-4" /> 新建隧道
          </button>
        </div>
      </div>

      {showCreate && (
        <div className="bg-gray-900 border border-gray-800 rounded-xl p-5 mb-6">
          <h3 className="text-sm font-medium text-white mb-4">创建隧道</h3>
          <div className="grid grid-cols-3 gap-4 mb-4">
            <div className="col-span-3">
              <label className="block text-xs text-gray-400 mb-1">Claw 节点 ID *</label>
              <input value={form.claw_node_id} onChange={e => setForm({ ...form, claw_node_id: e.target.value })}
                placeholder="节点 UUID" className="w-full bg-gray-800 border border-gray-700 rounded-lg px-3 py-2 text-white text-sm font-mono focus:outline-none focus:border-overlord-500" />
            </div>
            <div>
              <label className="block text-xs text-gray-400 mb-1">本地端口 *</label>
              <input type="number" value={form.local_port || ''} onChange={e => setForm({ ...form, local_port: +e.target.value })}
                placeholder="8080" className="w-full bg-gray-800 border border-gray-700 rounded-lg px-3 py-2 text-white text-sm focus:outline-none focus:border-overlord-500" />
            </div>
            <div>
              <label className="block text-xs text-gray-400 mb-1">远程端口 *</label>
              <input type="number" value={form.remote_port || ''} onChange={e => setForm({ ...form, remote_port: +e.target.value })}
                placeholder="8080" className="w-full bg-gray-800 border border-gray-700 rounded-lg px-3 py-2 text-white text-sm focus:outline-none focus:border-overlord-500" />
            </div>
            <div className="flex gap-2">
              <div className="flex-1">
                <label className="block text-xs text-gray-400 mb-1">协议</label>
                <select value={form.protocol} onChange={e => setForm({ ...form, protocol: e.target.value })}
                  className="w-full bg-gray-800 border border-gray-700 rounded-lg px-3 py-2 text-white text-sm focus:outline-none focus:border-overlord-500">
                  <option value="tcp">TCP</option>
                  <option value="udp">UDP</option>
                </select>
              </div>
              <div className="flex-1">
                <label className="block text-xs text-gray-400 mb-1">模式</label>
                <select value={form.mode} onChange={e => setForm({ ...form, mode: e.target.value })}
                  className="w-full bg-gray-800 border border-gray-700 rounded-lg px-3 py-2 text-white text-sm focus:outline-none focus:border-overlord-500">
                  <option value="forward">正向</option>
                  <option value="reverse">反向</option>
                </select>
              </div>
            </div>
          </div>
          <div className="flex gap-2">
            <button onClick={handleCreate} className="px-4 py-2 bg-overlord-600 text-white text-sm rounded-lg hover:bg-overlord-500 transition">创建</button>
            <button onClick={() => setShowCreate(false)} className="px-4 py-2 bg-gray-800 text-gray-300 text-sm rounded-lg hover:bg-gray-700 transition">取消</button>
          </div>
        </div>
      )}

      {loading ? (
        <div className="flex justify-center py-12">
          <div className="animate-spin w-6 h-6 border-2 border-overlord-500 border-t-transparent rounded-full" />
        </div>
      ) : tunnels.length === 0 ? (
        <div className="bg-gray-900 border border-gray-800 border-dashed rounded-xl p-12 text-center">
          <Network className="w-10 h-10 text-gray-600 mx-auto mb-3" />
          <p className="text-gray-400">暂无隧道</p>
        </div>
      ) : (
        <div className="space-y-2">
          {tunnels.map(t => (
            <div key={t.id} className="bg-gray-900 border border-gray-800 rounded-xl p-4 flex items-center gap-4 hover:border-gray-700 transition group">
              <div className="w-10 h-10 rounded-lg bg-cyan-600/10 flex items-center justify-center shrink-0">
                <ArrowRightLeft className="w-5 h-5 text-cyan-400" />
              </div>
              <div className="flex-1 min-w-0">
                <div className="flex items-center gap-2">
                  <span className="text-sm font-semibold text-white">{t.claw_name || t.claw_node_id.slice(0, 8)}</span>
                  <span className={`text-[10px] px-1.5 py-0.5 rounded ${statusColors[t.status] || statusColors.closed}`}>{t.status}</span>
                  <span className="text-[10px] px-1.5 py-0.5 rounded bg-gray-800 text-gray-400 uppercase">{t.protocol} {t.mode}</span>
                  {t.team && <span className="text-[10px] px-1.5 py-0.5 rounded bg-gray-800 text-gray-400">{t.team}</span>}
                </div>
                <div className="flex items-center gap-4 mt-1 text-xs text-gray-500">
                  <span className="font-mono">:{t.local_port} → :{t.remote_port}</span>
                  <span>↑ {fmtBytes(t.bytes_out)}</span>
                  <span>↓ {fmtBytes(t.bytes_in)}</span>
                  <span>{t.connections} 连接</span>
                </div>
                {t.last_error && (
                  <div className="flex items-center gap-1 mt-1 text-xs text-red-400">
                    <AlertCircle className="w-3 h-3" />
                    <span className="truncate">{t.last_error}</span>
                  </div>
                )}
              </div>
              <button onClick={() => handleDelete(t.id)}
                className="opacity-0 group-hover:opacity-100 p-2 text-gray-500 hover:text-red-400 transition" title="删除">
                <Trash2 className="w-4 h-4" />
              </button>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
