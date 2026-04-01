import { useEffect, useState } from 'react'
import { Search, ChevronLeft, ChevronRight } from 'lucide-react'
import { getCustomers } from '../api'

const INTENT_BADGE: Record<string, { bg: string; label: string }> = {
  A: { bg: 'bg-red-500/20 text-red-400', label: 'A 强意向' },
  B: { bg: 'bg-orange-500/20 text-orange-400', label: 'B 较强' },
  C: { bg: 'bg-yellow-500/20 text-yellow-400', label: 'C 一般' },
  D: { bg: 'bg-green-500/20 text-green-400', label: 'D 弱' },
  E: { bg: 'bg-blue-500/20 text-blue-400', label: 'E 拒绝' },
  F: { bg: 'bg-stone-500/20 text-stone-400', label: 'F 无效' },
}

export default function CustomerPage() {
  const [customers, setCustomers] = useState<any[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [search, setSearch] = useState('')
  const [intentFilter, setIntentFilter] = useState('')
  const pageSize = 20

  const load = () => {
    getCustomers({ page, page_size: pageSize, search: search || undefined, intent_level: intentFilter || undefined })
      .then(r => { setCustomers(r.customers || []); setTotal(r.total || 0) })
      .catch(() => {})
  }
  useEffect(() => { load() }, [page, intentFilter])

  const totalPages = Math.ceil(total / pageSize)

  return (
    <div className="p-6 space-y-4">
      <h1 className="text-2xl font-bold">客户管理</h1>

      {/* Filters */}
      <div className="flex gap-3 items-center">
        <div className="relative flex-1 max-w-xs">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-stone-500" />
          <input value={search} onChange={e => setSearch(e.target.value)} onKeyDown={e => e.key === 'Enter' && load()}
            className="w-full pl-9 pr-3 py-2 bg-stone-800 border border-stone-700 rounded-lg text-sm" placeholder="搜索姓名/电话..." />
        </div>
        <div className="flex gap-1">
          {['', 'A', 'B', 'C', 'D', 'E', 'F'].map(lv => (
            <button key={lv} onClick={() => { setIntentFilter(lv); setPage(1) }}
              className={`px-2.5 py-1.5 rounded text-xs font-medium transition ${intentFilter === lv ? 'bg-cicada-600 text-white' : 'bg-stone-800 text-stone-400 hover:bg-stone-700'}`}>
              {lv || '全部'}
            </button>
          ))}
        </div>
        <span className="text-xs text-stone-500 ml-auto">共 {total} 条</span>
      </div>

      {/* Table */}
      <div className="bg-stone-900 border border-stone-800 rounded-xl overflow-hidden">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-stone-800 text-stone-500 text-xs">
              <th className="text-left px-4 py-3 font-medium">姓名</th>
              <th className="text-left px-4 py-3 font-medium">电话</th>
              <th className="text-center px-4 py-3 font-medium">意向</th>
              <th className="text-center px-4 py-3 font-medium">通话次数</th>
              <th className="text-left px-4 py-3 font-medium">行业</th>
              <th className="text-left px-4 py-3 font-medium">地区</th>
              <th className="text-left px-4 py-3 font-medium">最近备注</th>
              <th className="text-left px-4 py-3 font-medium">更新时间</th>
            </tr>
          </thead>
          <tbody>
            {customers.length === 0 ? (
              <tr><td colSpan={8} className="text-center py-8 text-stone-600">暂无客户数据</td></tr>
            ) : customers.map((c: any) => {
              const badge = INTENT_BADGE[c.intent_level] || INTENT_BADGE.F
              return (
                <tr key={c.id} className="border-b border-stone-800/50 hover:bg-stone-800/30">
                  <td className="px-4 py-3 font-medium">{c.name || '-'}</td>
                  <td className="px-4 py-3 text-stone-400 font-mono text-xs">{c.phone_masked || c.phone || '-'}</td>
                  <td className="px-4 py-3 text-center">
                    <span className={`inline-block px-2 py-0.5 rounded text-xs font-medium ${badge.bg}`}>{badge.label}</span>
                  </td>
                  <td className="px-4 py-3 text-center text-stone-400">{c.call_count ?? 0}</td>
                  <td className="px-4 py-3 text-stone-400">{c.industry || '-'}</td>
                  <td className="px-4 py-3 text-stone-400">{c.region || '-'}</td>
                  <td className="px-4 py-3 text-stone-500 truncate max-w-[200px]">{c.summary || '-'}</td>
                  <td className="px-4 py-3 text-stone-500 text-xs">{c.updated_at ? new Date(c.updated_at).toLocaleString('zh-CN') : '-'}</td>
                </tr>
              )
            })}
          </tbody>
        </table>
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
