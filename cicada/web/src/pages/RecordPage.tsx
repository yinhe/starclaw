import { useEffect, useState } from 'react'
import { PhoneIncoming, PhoneOff, Clock, ChevronLeft, ChevronRight, Play } from 'lucide-react'
import { getCalls } from '../api'

const STATUS_LABEL: Record<string, string> = {
  dialing: '拨号中', ringing: '振铃', connected: '已接通', talking: '通话中',
  hangup: '已挂断', failed: '失败', no_answer: '未接听', rejected: '拒接',
  recording_saved: '已存录音', error: '异常',
}

export default function RecordPage() {
  const [calls, setCalls] = useState<any[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [expanded, setExpanded] = useState<number | null>(null)
  const pageSize = 20

  const load = () => {
    getCalls({ page, page_size: pageSize })
      .then(r => { setCalls(r.calls || []); setTotal(r.total || 0) })
      .catch(() => {})
  }
  useEffect(() => { load() }, [page])

  const totalPages = Math.ceil(total / pageSize)

  const formatDuration = (s: number) => {
    if (!s) return '-'
    const m = Math.floor(s / 60)
    const sec = s % 60
    return m > 0 ? `${m}分${sec}秒` : `${sec}秒`
  }

  return (
    <div className="p-6 space-y-4">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold">通话记录</h1>
        <span className="text-xs text-stone-500">共 {total} 条</span>
      </div>

      <div className="space-y-2">
        {calls.length === 0 ? (
          <div className="text-center py-12 text-stone-600">暂无通话记录</div>
        ) : calls.map((c: any) => (
          <div key={c.id} className="bg-stone-900 border border-stone-800 rounded-xl overflow-hidden">
            {/* Header */}
            <div className="flex items-center gap-3 px-4 py-3 cursor-pointer hover:bg-stone-800/50"
              onClick={() => setExpanded(expanded === c.id ? null : c.id)}>
              <div className={`w-8 h-8 rounded-full flex items-center justify-center ${c.duration > 0 ? 'bg-cicada-500/20' : 'bg-stone-800'}`}>
                {c.duration > 0 ? <PhoneIncoming className="w-4 h-4 text-cicada-400" /> : <PhoneOff className="w-4 h-4 text-stone-500" />}
              </div>
              <div className="flex-1 min-w-0">
                <div className="flex items-center gap-2">
                  <span className="font-medium text-sm">{c.callee_number || '-'}</span>
                  <span className={`text-xs px-1.5 py-0.5 rounded ${c.intent_level === 'A' ? 'bg-red-500/20 text-red-400' : c.intent_level === 'B' ? 'bg-orange-500/20 text-orange-400' : 'bg-stone-800 text-stone-500'}`}>
                    {c.intent_level || '-'}
                  </span>
                </div>
                <div className="text-xs text-stone-500">{STATUS_LABEL[c.status] || c.status}</div>
              </div>
              <div className="flex items-center gap-4 text-xs text-stone-500">
                <span className="flex items-center gap-1"><Clock className="w-3 h-3" />{formatDuration(c.duration)}</span>
                <span>{c.created_at ? new Date(c.created_at).toLocaleString('zh-CN') : '-'}</span>
              </div>
              {c.recording_path && (
                <button className="p-1.5 rounded bg-cicada-600/20 text-cicada-400 hover:bg-cicada-600/40" title="播放录音"
                  onClick={e => { e.stopPropagation(); window.open(`/calls/${c.id}/recording`) }}>
                  <Play className="w-3.5 h-3.5" />
                </button>
              )}
            </div>

            {/* Expanded details */}
            {expanded === c.id && (
              <div className="border-t border-stone-800 px-4 py-3 space-y-2 text-sm">
                {c.summary && <div className="text-stone-300"><span className="text-stone-500">摘要:</span> {c.summary}</div>}
                {c.transcript && (
                  <div className="bg-stone-950 rounded-lg p-3 max-h-48 overflow-auto text-xs font-mono text-stone-400 whitespace-pre-wrap">
                    {c.transcript}
                  </div>
                )}
                {c.ai_analysis && (
                  <div className="text-xs text-stone-500">
                    <span>意向评分: {c.intent_score}</span>
                  </div>
                )}
              </div>
            )}
          </div>
        ))}
      </div>

      {/* Pagination */}
      {totalPages > 1 && (
        <div className="flex items-center justify-center gap-2">
          <button onClick={() => setPage(p => Math.max(1, p - 1))} disabled={page <= 1}
            className="p-1.5 rounded bg-stone-800 hover:bg-stone-700 disabled:opacity-30">
            <ChevronLeft className="w-4 h-4" />
          </button>
          <span className="text-sm text-stone-400">{page} / {totalPages}</span>
          <button onClick={() => setPage(p => Math.min(totalPages, p + 1))} disabled={page >= totalPages}
            className="p-1.5 rounded bg-stone-800 hover:bg-stone-700 disabled:opacity-30">
            <ChevronRight className="w-4 h-4" />
          </button>
        </div>
      )}
    </div>
  )
}
