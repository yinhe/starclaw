import { useEffect, useState } from 'react'
import { api, type ContentReport, type ReportStats } from '../api'
import { Flag, Clock, CheckCircle, XCircle, AlertTriangle, ChevronLeft, ChevronRight } from 'lucide-react'

const reasonLabels: Record<string, string> = {
  spam: '垃圾信息', abuse: '辱骂攻击', nsfw: '不当内容', illegal: '违法违规', other: '其他',
}
const statusBadge: Record<string, { label: string; cls: string }> = {
  pending: { label: '待审核', cls: 'bg-amber-600/20 text-amber-400' },
  reviewed: { label: '已审核', cls: 'bg-blue-600/20 text-blue-400' },
  resolved: { label: '已处理', cls: 'bg-emerald-600/20 text-emerald-400' },
  dismissed: { label: '已驳回', cls: 'bg-gray-600/20 text-gray-400' },
}

export default function ReportsPage() {
  const [reports, setReports] = useState<ContentReport[]>([])
  const [stats, setStats] = useState<ReportStats | null>(null)
  const [page, setPage] = useState(1)
  const [total, setTotal] = useState(0)
  const [statusFilter, setStatusFilter] = useState('pending')
  const [expanded, setExpanded] = useState<string | null>(null)
  const [reviewNote, setReviewNote] = useState('')
  const size = 15

  const load = async () => {
    try {
      let path = `/v1/admin/reports?page=${page}&size=${size}`
      if (statusFilter) path += `&status=${statusFilter}`
      const [rData, sData] = await Promise.all([
        api.get<{ data: { reports: ContentReport[]; total: number } }>(path),
        api.get<{ data: ReportStats }>('/v1/admin/reports/stats'),
      ])
      setReports(rData.data?.reports || [])
      setTotal(rData.data?.total || 0)
      setStats(sData.data || null)
    } catch { /* ignore */ }
  }

  useEffect(() => { load() }, [page, statusFilter])

  const handleReview = async (id: string, status: string, resolution?: string) => {
    try {
      await api.put(`/v1/admin/reports/${id}`, { status, resolution, review_note: reviewNote })
      setReviewNote('')
      setExpanded(null)
      load()
    } catch { /* ignore */ }
  }

  const handleAction = async (id: string, action: string) => {
    if (!confirm(`确定执行「${action}」操作？`)) return
    try {
      await api.post(`/v1/admin/reports/${id}/action`, { action })
      load()
    } catch { /* ignore */ }
  }

  const totalPages = Math.ceil(total / size)

  return (
    <div>
      <h2 className="text-xl font-bold mb-6">内容审核</h2>

      {/* Stats */}
      {stats && (
        <div className="grid grid-cols-5 gap-3 mb-6">
          {[
            { label: '总举报', value: stats.total, icon: Flag, color: 'text-purple-400' },
            { label: '待审核', value: stats.pending, icon: Clock, color: 'text-amber-400' },
            { label: '已审核', value: stats.reviewed, icon: AlertTriangle, color: 'text-blue-400' },
            { label: '已处理', value: stats.resolved, icon: CheckCircle, color: 'text-emerald-400' },
            { label: '已驳回', value: stats.dismissed, icon: XCircle, color: 'text-gray-400' },
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
        {['', 'pending', 'reviewed', 'resolved', 'dismissed'].map(s => (
          <button key={s} onClick={() => { setStatusFilter(s); setPage(1) }}
            className={`px-3 py-1.5 rounded-lg text-xs transition ${
              statusFilter === s ? 'bg-purple-600/20 text-purple-400' : 'text-gray-500 hover:bg-gray-800'
            }`}
          >
            {s === '' ? '全部' : statusBadge[s]?.label || s}
          </button>
        ))}
      </div>

      {/* Report list */}
      <div className="space-y-3">
        {reports.length === 0 ? (
          <div className="bg-gray-900 border border-gray-800 rounded-xl p-8 text-center text-gray-600">
            暂无举报记录
          </div>
        ) : reports.map(r => {
          const sb = statusBadge[r.status] || statusBadge.pending
          const isExpanded = expanded === r.id
          return (
            <div key={r.id} className="bg-gray-900 border border-gray-800 rounded-xl overflow-hidden">
              <div className="flex items-center gap-4 px-5 py-4 cursor-pointer hover:bg-gray-800/30 transition"
                onClick={() => setExpanded(isExpanded ? null : r.id)}>
                <Flag size={14} className="text-red-400 shrink-0" />
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-2">
                    <span className="text-sm text-white font-medium truncate">
                      [{r.target_type}] {r.target_title || r.target_id}
                    </span>
                    <span className={`text-[10px] px-1.5 py-0.5 rounded ${sb.cls}`}>{sb.label}</span>
                  </div>
                  <div className="text-xs text-gray-500 mt-0.5">
                    {reasonLabels[r.reason] || r.reason} · {new Date(r.created_at).toLocaleString('zh-CN')}
                  </div>
                </div>
              </div>

              {isExpanded && (
                <div className="px-5 pb-4 border-t border-gray-800 pt-3">
                  {r.detail && <p className="text-sm text-gray-300 mb-3">{r.detail}</p>}
                  <div className="grid grid-cols-3 gap-3 text-xs text-gray-500 mb-4">
                    <div>举报人: {r.reporter_id.slice(0, 8)}…</div>
                    <div>被举报: {r.author_id ? r.author_id.slice(0, 8) + '…' : '—'}</div>
                    <div>目标 ID: {r.target_id.slice(0, 8)}…</div>
                  </div>

                  {r.status === 'pending' && (
                    <div className="space-y-2">
                      <input
                        value={reviewNote}
                        onChange={e => setReviewNote(e.target.value)}
                        placeholder="审核备注（可选）"
                        className="w-full bg-gray-800 border border-gray-700 rounded-lg px-3 py-2 text-sm text-white placeholder-gray-600"
                      />
                      <div className="flex gap-2">
                        <button onClick={() => handleReview(r.id, 'resolved', 'hide')}
                          className="px-3 py-1.5 bg-amber-600/10 text-amber-400 rounded-lg text-xs hover:bg-amber-600/20 transition">
                          隐藏内容
                        </button>
                        <button onClick={() => handleReview(r.id, 'resolved', 'delete')}
                          className="px-3 py-1.5 bg-red-600/10 text-red-400 rounded-lg text-xs hover:bg-red-600/20 transition">
                          删除内容
                        </button>
                        <button onClick={() => handleAction(r.id, 'ban_author')}
                          className="px-3 py-1.5 bg-red-600/10 text-red-400 rounded-lg text-xs hover:bg-red-600/20 transition">
                          封禁作者
                        </button>
                        <button onClick={() => handleReview(r.id, 'dismissed', 'none')}
                          className="px-3 py-1.5 bg-gray-600/10 text-gray-400 rounded-lg text-xs hover:bg-gray-600/20 transition ml-auto">
                          驳回
                        </button>
                      </div>
                    </div>
                  )}
                  {r.status !== 'pending' && r.review_note && (
                    <p className="text-xs text-gray-500 mt-2">审核备注: {r.review_note}</p>
                  )}
                </div>
              )}
            </div>
          )
        })}
      </div>

      {totalPages > 1 && (
        <div className="flex items-center justify-between mt-4">
          <span className="text-xs text-gray-500">共 {total} 条举报</span>
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
