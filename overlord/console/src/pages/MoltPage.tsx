import { useEffect, useState } from 'react'
import { Package, Plus, CheckCircle, XCircle, Rocket, Clock, AlertTriangle, ChevronDown, ChevronUp } from 'lucide-react'
import { broodAPI, type MoltRelease, type MoltNodeStatus } from '../api/brood'

const statusConfig: Record<string, { color: string; label: string }> = {
  pending: { color: 'bg-yellow-600/10 text-yellow-400', label: '待审批' },
  approved: { color: 'bg-emerald-600/10 text-emerald-400', label: '已批准' },
  rejected: { color: 'bg-red-600/10 text-red-400', label: '已拒绝' },
  rolling: { color: 'bg-blue-600/10 text-blue-400', label: '滚动中' },
  completed: { color: 'bg-overlord-600/10 text-overlord-400', label: '已完成' },
}

export default function MoltPage() {
  const [releases, setReleases] = useState<MoltRelease[]>([])
  const [loading, setLoading] = useState(true)
  const [filter, setFilter] = useState('')
  const [showCreate, setShowCreate] = useState(false)
  const [expanded, setExpanded] = useState<string | null>(null)
  const [nodeStatuses, setNodeStatuses] = useState<MoltNodeStatus[]>([])
  const [form, setForm] = useState({ version: '', channel: 'stable', title: '', release_notes: '', download_url: '', target_team: '', rollout_pct: 100, max_failures: 1 })

  const load = async () => {
    try {
      const data = await broodAPI.listReleases({ status: filter || undefined })
      setReleases(data.releases || [])
    } catch { /* */ }
    finally { setLoading(false) }
  }

  useEffect(() => { setLoading(true); load() }, [filter])

  const handleCreate = async () => {
    if (!form.version.trim()) return
    try {
      await broodAPI.createRelease(form)
      setShowCreate(false)
      setForm({ version: '', channel: 'stable', title: '', release_notes: '', download_url: '', target_team: '', rollout_pct: 100, max_failures: 1 })
      load()
    } catch { /* */ }
  }

  const handleReview = async (id: string, action: 'approve' | 'reject') => {
    const note = action === 'reject' ? prompt('拒绝原因:') || '' : ''
    try { await broodAPI.reviewRelease(id, { action, note }); load() } catch { /* */ }
  }

  const handleRollout = async (id: string) => {
    if (!confirm('确定开始滚动更新？')) return
    try { await broodAPI.startRollout(id); load() } catch { /* */ }
  }

  const toggleDetail = async (id: string) => {
    if (expanded === id) { setExpanded(null); return }
    setExpanded(id)
    try {
      const data = await broodAPI.getRelease(id)
      setNodeStatuses(data.node_statuses || [])
    } catch { setNodeStatuses([]) }
  }

  return (
    <div className="p-8">
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-2xl font-bold text-white">Molt 更新管理</h1>
          <p className="text-sm text-gray-500 mt-1">管理 Claw 节点的版本更新与审批</p>
        </div>
        <div className="flex gap-2">
          <select value={filter} onChange={e => setFilter(e.target.value)}
            className="bg-gray-800 text-gray-300 text-sm px-3 py-2 rounded-lg border border-gray-700 focus:outline-none focus:border-overlord-500">
            <option value="">全部状态</option>
            <option value="pending">待审批</option>
            <option value="approved">已批准</option>
            <option value="rolling">滚动中</option>
            <option value="completed">已完成</option>
            <option value="rejected">已拒绝</option>
          </select>
          <button onClick={() => setShowCreate(!showCreate)}
            className="flex items-center gap-2 px-4 py-2 bg-overlord-600 text-white rounded-lg text-sm hover:bg-overlord-500 transition">
            <Plus className="w-4 h-4" /> 提交版本
          </button>
        </div>
      </div>

      {showCreate && (
        <div className="bg-gray-900 border border-gray-800 rounded-xl p-5 mb-6">
          <h3 className="text-sm font-medium text-white mb-4">提交新版本</h3>
          <div className="grid grid-cols-3 gap-4 mb-4">
            <div>
              <label className="block text-xs text-gray-400 mb-1">版本号 *</label>
              <input value={form.version} onChange={e => setForm({ ...form, version: e.target.value })}
                placeholder="v1.2.3" className="w-full bg-gray-800 border border-gray-700 rounded-lg px-3 py-2 text-white text-sm focus:outline-none focus:border-overlord-500" />
            </div>
            <div>
              <label className="block text-xs text-gray-400 mb-1">通道</label>
              <select value={form.channel} onChange={e => setForm({ ...form, channel: e.target.value })}
                className="w-full bg-gray-800 border border-gray-700 rounded-lg px-3 py-2 text-white text-sm focus:outline-none focus:border-overlord-500">
                <option value="stable">stable</option>
                <option value="beta">beta</option>
                <option value="nightly">nightly</option>
              </select>
            </div>
            <div>
              <label className="block text-xs text-gray-400 mb-1">标题</label>
              <input value={form.title} onChange={e => setForm({ ...form, title: e.target.value })}
                placeholder="安全修复" className="w-full bg-gray-800 border border-gray-700 rounded-lg px-3 py-2 text-white text-sm focus:outline-none focus:border-overlord-500" />
            </div>
            <div className="col-span-3">
              <label className="block text-xs text-gray-400 mb-1">更新说明</label>
              <textarea value={form.release_notes} onChange={e => setForm({ ...form, release_notes: e.target.value })} rows={3}
                className="w-full bg-gray-800 border border-gray-700 rounded-lg px-3 py-2 text-white text-sm focus:outline-none focus:border-overlord-500 resize-none" />
            </div>
            <div>
              <label className="block text-xs text-gray-400 mb-1">下载 URL</label>
              <input value={form.download_url} onChange={e => setForm({ ...form, download_url: e.target.value })}
                className="w-full bg-gray-800 border border-gray-700 rounded-lg px-3 py-2 text-white text-sm focus:outline-none focus:border-overlord-500" />
            </div>
            <div>
              <label className="block text-xs text-gray-400 mb-1">滚动比例 (%)</label>
              <input type="number" value={form.rollout_pct} onChange={e => setForm({ ...form, rollout_pct: +e.target.value })}
                className="w-full bg-gray-800 border border-gray-700 rounded-lg px-3 py-2 text-white text-sm focus:outline-none focus:border-overlord-500" />
            </div>
            <div>
              <label className="block text-xs text-gray-400 mb-1">最大容错</label>
              <input type="number" value={form.max_failures} onChange={e => setForm({ ...form, max_failures: +e.target.value })}
                className="w-full bg-gray-800 border border-gray-700 rounded-lg px-3 py-2 text-white text-sm focus:outline-none focus:border-overlord-500" />
            </div>
          </div>
          <div className="flex gap-2">
            <button onClick={handleCreate} className="px-4 py-2 bg-overlord-600 text-white text-sm rounded-lg hover:bg-overlord-500 transition">提交</button>
            <button onClick={() => setShowCreate(false)} className="px-4 py-2 bg-gray-800 text-gray-300 text-sm rounded-lg hover:bg-gray-700 transition">取消</button>
          </div>
        </div>
      )}

      {loading ? (
        <div className="flex justify-center py-12">
          <div className="animate-spin w-6 h-6 border-2 border-overlord-500 border-t-transparent rounded-full" />
        </div>
      ) : releases.length === 0 ? (
        <div className="bg-gray-900 border border-gray-800 border-dashed rounded-xl p-12 text-center">
          <Package className="w-10 h-10 text-gray-600 mx-auto mb-3" />
          <p className="text-gray-400">暂无版本发布</p>
        </div>
      ) : (
        <div className="space-y-2">
          {releases.map(r => {
            const sc = statusConfig[r.status] || statusConfig.pending
            const isExpanded = expanded === r.id
            const progress = r.total_nodes > 0 ? ((r.updated_nodes + r.failed_nodes) / r.total_nodes) * 100 : 0
            return (
              <div key={r.id} className="bg-gray-900 border border-gray-800 rounded-xl overflow-hidden hover:border-gray-700 transition">
                <div className="p-4 flex items-center gap-4 cursor-pointer" onClick={() => toggleDetail(r.id)}>
                  <div className="w-10 h-10 rounded-lg bg-purple-600/10 flex items-center justify-center shrink-0">
                    <Package className="w-5 h-5 text-purple-400" />
                  </div>
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2">
                      <span className="text-sm font-bold text-white font-mono">{r.version}</span>
                      <span className={`text-[10px] px-1.5 py-0.5 rounded ${sc.color}`}>{sc.label}</span>
                      <span className="text-[10px] px-1.5 py-0.5 rounded bg-gray-800 text-gray-400">{r.channel}</span>
                      {r.title && <span className="text-xs text-gray-400">{r.title}</span>}
                    </div>
                    <div className="flex items-center gap-4 mt-1 text-xs text-gray-500">
                      <span>提交: {r.submitted_by}</span>
                      {r.reviewed_by && <span>审批: {r.reviewed_by}</span>}
                      {r.target_team && <span>团队: {r.target_team}</span>}
                      {r.status === 'rolling' && <span>{r.updated_nodes}/{r.total_nodes} 完成, {r.failed_nodes} 失败</span>}
                    </div>
                    {r.status === 'rolling' && (
                      <div className="h-1.5 bg-gray-800 rounded-full overflow-hidden mt-2 max-w-sm">
                        <div className="h-full bg-blue-500 rounded-full transition-all" style={{ width: `${progress}%` }} />
                      </div>
                    )}
                  </div>
                  <div className="flex items-center gap-1 shrink-0">
                    {r.status === 'pending' && (
                      <>
                        <button onClick={e => { e.stopPropagation(); handleReview(r.id, 'approve') }}
                          className="p-2 text-emerald-500 hover:bg-emerald-500/10 rounded-lg transition" title="批准">
                          <CheckCircle className="w-4 h-4" />
                        </button>
                        <button onClick={e => { e.stopPropagation(); handleReview(r.id, 'reject') }}
                          className="p-2 text-red-500 hover:bg-red-500/10 rounded-lg transition" title="拒绝">
                          <XCircle className="w-4 h-4" />
                        </button>
                      </>
                    )}
                    {r.status === 'approved' && (
                      <button onClick={e => { e.stopPropagation(); handleRollout(r.id) }}
                        className="p-2 text-blue-400 hover:bg-blue-500/10 rounded-lg transition" title="开始滚动">
                        <Rocket className="w-4 h-4" />
                      </button>
                    )}
                    {isExpanded ? <ChevronUp className="w-4 h-4 text-gray-500" /> : <ChevronDown className="w-4 h-4 text-gray-500" />}
                  </div>
                </div>

                {isExpanded && (
                  <div className="border-t border-gray-800 p-4">
                    {r.release_notes && <p className="text-sm text-gray-300 mb-4 whitespace-pre-wrap">{r.release_notes}</p>}
                    {r.review_note && (
                      <div className="text-xs text-gray-500 mb-4 flex items-center gap-1">
                        <AlertTriangle className="w-3 h-3" /> 审批备注: {r.review_note}
                      </div>
                    )}
                    {nodeStatuses.length > 0 ? (
                      <div className="space-y-1">
                        <h4 className="text-xs font-medium text-gray-400 mb-2">节点更新状态</h4>
                        {nodeStatuses.map(ns => (
                          <div key={ns.id} className="flex items-center gap-3 text-xs bg-gray-800/50 rounded-lg px-3 py-2">
                            <span className="text-white font-medium w-32 truncate">{ns.claw_name}</span>
                            <span className="text-gray-500 font-mono">{ns.old_version} →</span>
                            <span className={`px-1.5 py-0.5 rounded ${
                              ns.status === 'completed' ? 'bg-emerald-600/10 text-emerald-400'
                              : ns.status === 'failed' ? 'bg-red-600/10 text-red-400'
                              : 'bg-yellow-600/10 text-yellow-400'
                            }`}>{ns.status}</span>
                            {ns.error_detail && <span className="text-red-400 truncate">{ns.error_detail}</span>}
                            {ns.completed_at && <span className="text-gray-600 ml-auto"><Clock className="w-3 h-3 inline" /> {new Date(ns.completed_at).toLocaleTimeString('zh-CN')}</span>}
                          </div>
                        ))}
                      </div>
                    ) : (
                      <p className="text-xs text-gray-600">暂无节点状态数据</p>
                    )}
                  </div>
                )}
              </div>
            )
          })}
        </div>
      )}
    </div>
  )
}
