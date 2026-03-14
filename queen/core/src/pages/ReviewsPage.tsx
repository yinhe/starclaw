import { useEffect, useState } from 'react'
import { api } from '../api'
import { Clock, CheckCircle, XCircle, Package, ChevronLeft, ChevronRight, Eye, Trash2 } from 'lucide-react'

interface MarketplaceItem {
  id: string
  user_id: string
  type: string
  name: string
  description: string
  icon: string
  version: string
  tags: string
  config: string
  status: string
  downloads: number
  rating: number
  review_status: string
  reviewer_id: string
  review_note: string
  reviewed_at: string | null
  submitted_at: string | null
  created_at: string
  author?: { id: string; nickname: string; email: string }
}

interface ReviewStats {
  pending: number
  approved: number
  rejected: number
  total: number
}

const statusBadge: Record<string, { label: string; cls: string }> = {
  pending_review: { label: '待审核', cls: 'bg-amber-600/20 text-amber-400' },
  approved: { label: '已通过', cls: 'bg-emerald-600/20 text-emerald-400' },
  rejected: { label: '已拒绝', cls: 'bg-red-600/20 text-red-400' },
  published: { label: '已发布', cls: 'bg-blue-600/20 text-blue-400' },
  removed: { label: '已下架', cls: 'bg-gray-600/20 text-gray-400' },
  draft: { label: '草稿', cls: 'bg-gray-600/20 text-gray-500' },
}

const typeLabel: Record<string, string> = {
  agent: 'Agent',
  skill: '技能',
  workflow: '工作流',
  mcp: 'MCP',
}

export default function ReviewsPage() {
  const [items, setItems] = useState<MarketplaceItem[]>([])
  const [stats, setStats] = useState<ReviewStats | null>(null)
  const [page, setPage] = useState(1)
  const [total, setTotal] = useState(0)
  const [statusFilter, setStatusFilter] = useState('pending_review')
  const [expanded, setExpanded] = useState<string | null>(null)
  const [note, setNote] = useState('')
  const size = 15

  const load = async () => {
    try {
      const [iData, sData] = await Promise.all([
        api.get<{ items: MarketplaceItem[]; total: number }>(
          `/v1/admin/marketplace/pending?status=${statusFilter}&page=${page}&size=${size}`
        ),
        api.get<ReviewStats>('/v1/admin/marketplace/stats'),
      ])
      setItems(iData.items || [])
      setTotal(iData.total || 0)
      setStats(sData || null)
    } catch { /* ignore */ }
  }

  useEffect(() => { load() }, [page, statusFilter])

  const handleApprove = async (id: string) => {
    try {
      await api.put(`/v1/admin/marketplace/items/${id}/approve`, { note })
      setNote('')
      setExpanded(null)
      load()
    } catch { /* ignore */ }
  }

  const handleReject = async (id: string) => {
    if (!note.trim()) { alert('请填写拒绝原因'); return }
    try {
      await api.put(`/v1/admin/marketplace/items/${id}/reject`, { note })
      setNote('')
      setExpanded(null)
      load()
    } catch { /* ignore */ }
  }

  const handleRemove = async (id: string) => {
    if (!note.trim()) { alert('请填写下架原因'); return }
    if (!confirm('确定下架该商品？')) return
    try {
      await api.put(`/v1/admin/marketplace/items/${id}/remove`, { note })
      setNote('')
      setExpanded(null)
      load()
    } catch { /* ignore */ }
  }

  const totalPages = Math.ceil(total / size)

  return (
    <div>
      <h2 className="text-xl font-bold mb-6">开发者中心 · 审核管理</h2>

      {/* Stats */}
      {stats && (
        <div className="grid grid-cols-4 gap-3 mb-6">
          {[
            { label: '待审核', value: stats.pending, icon: Clock, color: 'text-amber-400' },
            { label: '已通过', value: stats.approved, icon: CheckCircle, color: 'text-emerald-400' },
            { label: '已拒绝', value: stats.rejected, icon: XCircle, color: 'text-red-400' },
            { label: '总提交', value: stats.total, icon: Package, color: 'text-purple-400' },
          ].map(s => (
            <div key={s.label} className="bg-gray-900 border border-gray-800 rounded-lg p-4">
              <div className="flex items-center gap-2 mb-1">
                <s.icon size={14} className={s.color} />
                <span className="text-xs text-gray-500">{s.label}</span>
              </div>
              <div className="text-xl font-bold text-white">{s.value}</div>
            </div>
          ))}
        </div>
      )}

      {/* Filter */}
      <div className="flex gap-2 mb-4">
        {['pending_review', 'approved', 'rejected', 'removed', 'published'].map(s => (
          <button key={s} onClick={() => { setStatusFilter(s); setPage(1) }}
            className={`px-3 py-1.5 rounded-lg text-xs transition ${
              statusFilter === s ? 'bg-purple-600/20 text-purple-400' : 'text-gray-500 hover:bg-gray-800'
            }`}
          >
            {statusBadge[s]?.label || s}
          </button>
        ))}
      </div>

      {/* Item list */}
      <div className="space-y-3">
        {items.length === 0 ? (
          <div className="bg-gray-900 border border-gray-800 rounded-xl p-8 text-center text-gray-600">
            暂无提交记录
          </div>
        ) : items.map(item => {
          const sb = statusBadge[item.status] || statusBadge.pending_review
          const isExpanded = expanded === item.id
          return (
            <div key={item.id} className="bg-gray-900 border border-gray-800 rounded-xl overflow-hidden">
              <div className="flex items-center gap-4 px-5 py-4 cursor-pointer hover:bg-gray-800/30 transition"
                onClick={() => { setExpanded(isExpanded ? null : item.id); setNote('') }}>
                <Package size={16} className="text-purple-400 shrink-0" />
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-2">
                    <span className="text-sm text-white font-medium truncate">{item.name}</span>
                    <span className="text-[10px] px-1.5 py-0.5 rounded bg-gray-800 text-gray-400">
                      {typeLabel[item.type] || item.type}
                    </span>
                    <span className={`text-[10px] px-1.5 py-0.5 rounded ${sb.cls}`}>{sb.label}</span>
                    <span className="text-[10px] text-gray-600">v{item.version}</span>
                  </div>
                  <div className="text-xs text-gray-500 mt-0.5 truncate">
                    {item.description || '无描述'}
                    <span className="mx-1">·</span>
                    {item.author?.nickname || item.author?.email || item.user_id.slice(0, 8) + '…'}
                    <span className="mx-1">·</span>
                    {item.submitted_at
                      ? new Date(item.submitted_at).toLocaleString('zh-CN')
                      : new Date(item.created_at).toLocaleString('zh-CN')}
                  </div>
                </div>
              </div>

              {isExpanded && (
                <div className="px-5 pb-4 border-t border-gray-800 pt-3">
                  {/* Description & tags */}
                  <p className="text-sm text-gray-300 mb-3">{item.description || '无描述'}</p>
                  {item.tags && (
                    <div className="flex flex-wrap gap-1.5 mb-3">
                      {item.tags.split(',').map(t => (
                        <span key={t} className="text-[10px] px-2 py-0.5 rounded-full bg-gray-800 text-gray-400">
                          {t.trim()}
                        </span>
                      ))}
                    </div>
                  )}

                  <div className="grid grid-cols-4 gap-3 text-xs text-gray-500 mb-4">
                    <div>类型: {typeLabel[item.type] || item.type}</div>
                    <div>版本: {item.version}</div>
                    <div>下载量: {item.downloads}</div>
                    <div>评分: {item.rating > 0 ? item.rating.toFixed(1) : '—'}</div>
                  </div>

                  {/* Previous review note */}
                  {item.review_note && item.status !== 'pending_review' && (
                    <div className="text-xs text-gray-500 mb-3 p-2 bg-gray-800/50 rounded">
                      审核备注: {item.review_note}
                      {item.reviewed_at && (
                        <span className="ml-2">({new Date(item.reviewed_at).toLocaleString('zh-CN')})</span>
                      )}
                    </div>
                  )}

                  {/* Review actions */}
                  {item.status === 'pending_review' && (
                    <div className="space-y-2">
                      <input
                        value={note}
                        onChange={e => setNote(e.target.value)}
                        placeholder="审核备注（拒绝时必填）"
                        className="w-full bg-gray-800 border border-gray-700 rounded-lg px-3 py-2 text-sm text-white placeholder-gray-600"
                      />
                      <div className="flex gap-2">
                        <button onClick={() => handleApprove(item.id)}
                          className="flex items-center gap-1.5 px-3 py-1.5 bg-emerald-600/10 text-emerald-400 rounded-lg text-xs hover:bg-emerald-600/20 transition">
                          <CheckCircle size={12} /> 通过
                        </button>
                        <button onClick={() => handleReject(item.id)}
                          className="flex items-center gap-1.5 px-3 py-1.5 bg-red-600/10 text-red-400 rounded-lg text-xs hover:bg-red-600/20 transition">
                          <XCircle size={12} /> 拒绝
                        </button>
                      </div>
                    </div>
                  )}

                  {/* Remove action for approved/published items */}
                  {(item.status === 'approved' || item.status === 'published') && (
                    <div className="space-y-2">
                      <input
                        value={note}
                        onChange={e => setNote(e.target.value)}
                        placeholder="下架原因（必填）"
                        className="w-full bg-gray-800 border border-gray-700 rounded-lg px-3 py-2 text-sm text-white placeholder-gray-600"
                      />
                      <div className="flex gap-2">
                        <button onClick={() => handleRemove(item.id)}
                          className="flex items-center gap-1.5 px-3 py-1.5 bg-red-600/10 text-red-400 rounded-lg text-xs hover:bg-red-600/20 transition">
                          <Trash2 size={12} /> 下架
                        </button>
                        <a href={`/marketplace/${item.id}`} target="_blank" rel="noreferrer"
                          className="flex items-center gap-1.5 px-3 py-1.5 bg-gray-600/10 text-gray-400 rounded-lg text-xs hover:bg-gray-600/20 transition">
                          <Eye size={12} /> 预览
                        </a>
                      </div>
                    </div>
                  )}
                </div>
              )}
            </div>
          )
        })}
      </div>

      {totalPages > 1 && (
        <div className="flex items-center justify-between mt-4">
          <span className="text-xs text-gray-500">共 {total} 条提交</span>
          <div className="flex items-center gap-2">
            <button onClick={() => setPage(p => Math.max(1, p - 1))} disabled={page <= 1}
              className="p-1.5 rounded bg-gray-800 text-gray-400 hover:text-white disabled:opacity-30 transition">
              <ChevronLeft size={14} />
            </button>
            <span className="text-xs text-gray-400">{page} / {totalPages}</span>
            <button onClick={() => setPage(p => Math.min(totalPages, p + 1))} disabled={page >= totalPages}
              className="p-1.5 rounded bg-gray-800 text-gray-400 hover:text-white disabled:opacity-30 transition">
              <ChevronRight size={14} />
            </button>
          </div>
        </div>
      )}
    </div>
  )
}
