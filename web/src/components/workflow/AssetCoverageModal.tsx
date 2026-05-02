// ── AssetCoverageModal · 全集物料对齐总览 ──
//
// 这个 modal 是「物料对齐」按钮打开的全景视图：
//   1. 顶部基础健康（角色 6/6 ref ✓ TOS 5/6 fresh）
//   2. 各集状态表（每行点击展开 PreflightModal 详情）
//   3. 复用 PreflightModal 看单集详情，不重新做 UI

import { useEffect, useMemo, useState } from 'react'
import type { Node } from '@xyflow/react'
import {
  CheckCircle2, ChevronRight, CircleX, Film, Loader2, RefreshCw, Sparkles, X,
} from 'lucide-react'

import { sweepManifest, type SweepResult, type BasicHealthEntry, type EpisodeSweepRow } from './manifestSweep'
import PreflightModal from './PreflightModal'

interface Props {
  open: boolean
  onClose: () => void
  nodes: Node[]
  project?: string
  /** 修复器需要回写的 patch；如果省略仅做只读展示。*/
  onPatchNode?: (nodeId: string, patch: Record<string, unknown>) => void
}

export default function AssetCoverageModal({ open, onClose, nodes, project = 'swarm-universe', onPatchNode }: Props) {
  const [result, setResult] = useState<SweepResult | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [drilldown, setDrilldown] = useState<EpisodeSweepRow | null>(null)
  const [nonce, setNonce] = useState(0)

  useEffect(() => {
    if (!open) return
    let cancelled = false
    setLoading(true); setError(null)
    sweepManifest({ nodes, project })
      .then(r => { if (!cancelled) setResult(r) })
      .catch(e => { if (!cancelled) setError(e instanceof Error ? e.message : String(e)) })
      .finally(() => { if (!cancelled) setLoading(false) })
    return () => { cancelled = true }
  }, [open, nodes, project, nonce])

  if (!open) return null

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/70 backdrop-blur-sm" onClick={onClose}>
      <div className="relative w-[min(1100px,94vw)] max-h-[92vh] overflow-hidden flex flex-col rounded-xl border border-gray-700 bg-gray-950 shadow-2xl"
        onClick={e => e.stopPropagation()}>
        <Header loading={loading} elapsed={result?.elapsed_ms} onRefresh={() => setNonce(n => n + 1)} onClose={onClose} />

        <div className="flex-1 overflow-auto px-5 py-4 space-y-5">
          {error && (
            <div className="rounded border border-rose-700/50 bg-rose-900/20 text-rose-200 text-xs px-3 py-2">
              扫描失败：{error}
            </div>
          )}
          {loading && !result && (
            <div className="flex items-center justify-center py-16 text-gray-500 text-xs gap-2">
              <Loader2 className="w-4 h-4 animate-spin" /> 正在并发扫描所有集……
            </div>
          )}
          {result && (
            <>
              <BasicHealthSection
                title="角色"
                entries={result.basic.characters}
                summary={`${result.basic.summary.chars_total} 个 · ref ${result.basic.summary.chars_with_ref}/${result.basic.summary.chars_total} · TOS 新鲜 ${result.basic.summary.chars_tos_fresh}/${result.basic.summary.chars_total} · v2v ref_video ${result.basic.summary.chars_with_ref_video}/${result.basic.summary.chars_total}`}
              />
              <BasicHealthSection
                title="道具"
                entries={result.basic.props}
                summary={`${result.basic.summary.props_total} 个 · ref ${result.basic.summary.props_with_ref}/${result.basic.summary.props_total} · TOS 新鲜 ${result.basic.summary.props_tos_fresh}/${result.basic.summary.props_total}`}
              />
              <EpisodeTable rows={result.episodes} onDrill={setDrilldown} />
            </>
          )}
        </div>

        <div className="flex items-center gap-2 px-5 py-3 border-t border-gray-800 bg-gray-950">
          <div className="text-[11px] text-gray-500">
            {result && `扫描 ${result.episodes.length} 集 · 用时 ${(result.elapsed_ms / 1000).toFixed(1)}s · 生成 ${new Date(result.generated_at).toLocaleTimeString()}`}
          </div>
          <div className="flex-1" />
          <button onClick={onClose}
            className="px-3 py-1.5 text-xs rounded border border-gray-700 text-gray-300 hover:bg-gray-800">
            关闭
          </button>
        </div>
      </div>

      {/* 单集 drilldown 复用 PreflightModal */}
      {drilldown && (
        <PreflightModal
          episode={drilldown.episode}
          nodes={nodes}
          project={project}
          onClose={() => setDrilldown(null)}
          onProceed={() => setDrilldown(null)}
          onPatchNode={onPatchNode}
        />
      )}
    </div>
  )
}

// ── 顶部 Header ──

function Header({ loading, elapsed, onRefresh, onClose }: {
  loading: boolean; elapsed?: number; onRefresh: () => void; onClose: () => void
}) {
  return (
    <div className="flex items-center gap-3 px-5 py-3 border-b border-gray-800">
      <div className="w-9 h-9 rounded-full flex items-center justify-center flex-none bg-violet-900/50">
        {loading ? <Loader2 className="w-4 h-4 text-violet-400 animate-spin" /> : <Sparkles className="w-4 h-4 text-violet-300" />}
      </div>
      <div className="flex-1 min-w-0">
        <div className="text-sm font-semibold text-gray-100">物料对齐 · 全集扫描</div>
        <div className="text-[11px] text-gray-500">
          {loading ? '并发跑每集 preflight 中…' : `基础健康 + 跨集自检（${elapsed ? `${(elapsed / 1000).toFixed(1)}s` : '完成'}）`}
        </div>
      </div>
      <button onClick={onRefresh} disabled={loading} title="重新扫描"
        className="p-1.5 rounded hover:bg-gray-800 text-gray-400 hover:text-gray-200 disabled:opacity-40">
        <RefreshCw className={`w-4 h-4 ${loading ? 'animate-spin' : ''}`} />
      </button>
      <button onClick={onClose} title="关闭"
        className="p-1.5 rounded hover:bg-gray-800 text-gray-400 hover:text-gray-200">
        <X className="w-4 h-4" />
      </button>
    </div>
  )
}

// ── 基础健康（角色/道具）──

function BasicHealthSection({ title, entries, summary }: {
  title: string; entries: BasicHealthEntry[]; summary: string
}) {
  const okCount = entries.filter(e => e.status === 'ok').length
  const warnCount = entries.filter(e => e.status === 'warn').length
  const errCount = entries.filter(e => e.status === 'error').length
  return (
    <section className="rounded-lg border border-gray-800 bg-gray-900/40">
      <header className="px-3 py-2 border-b border-gray-800/80 flex items-center gap-3">
        <span className="text-[11px] font-medium text-gray-300 uppercase tracking-wider">{title}</span>
        <span className="text-[10.5px] text-gray-500 flex-1">{summary}</span>
        <div className="flex items-center gap-2 text-[10.5px]">
          <span className="text-emerald-400">✓ {okCount}</span>
          {warnCount > 0 && <span className="text-amber-400">⚠ {warnCount}</span>}
          {errCount > 0 && <span className="text-rose-400">✗ {errCount}</span>}
        </div>
      </header>
      {entries.length === 0 ? (
        <div className="px-3 py-3 text-xs text-gray-600 italic">未加载（先点「虫群宇宙」加载资产）</div>
      ) : (
        <div className="grid grid-cols-2 lg:grid-cols-3 gap-1.5 p-2">
          {entries.map(e => <BasicHealthCard key={`${e.kind}-${e.key || e.label}`} entry={e} />)}
        </div>
      )}
    </section>
  )
}

function BasicHealthCard({ entry }: { entry: BasicHealthEntry }) {
  const tone = entry.status === 'ok' ? 'border-emerald-800/40 bg-emerald-900/10'
    : entry.status === 'warn' ? 'border-amber-800/50 bg-amber-900/10'
    : 'border-rose-800/50 bg-rose-900/10'
  return (
    <div className={`flex items-center gap-2 px-2 py-1.5 rounded border ${tone}`}>
      <div className="w-9 h-9 flex-none bg-gray-950 rounded overflow-hidden border border-gray-800/60">
        {entry.imageUrl ? (
          <img src={entry.imageUrl} alt={entry.label} className="w-full h-full object-cover" loading="lazy"
            onError={e => { (e.target as HTMLImageElement).style.opacity = '0.2' }} />
        ) : (
          <div className="w-full h-full flex items-center justify-center text-rose-400">
            <CircleX className="w-3 h-3" />
          </div>
        )}
      </div>
      <div className="flex-1 min-w-0">
        <div className="text-[11.5px] text-gray-200 truncate">
          {entry.tag && <span className="text-gray-500 mr-1">{entry.tag}</span>}
          {entry.label}
        </div>
        <div className="text-[10px] text-gray-500 flex items-center gap-1.5">
          {entry.status === 'ok' && <span className="text-emerald-400">本地+TOS✓</span>}
          {entry.status === 'warn' && (
            <span className="text-amber-400">
              {!entry.tos_url ? 'TOS 缺' : (entry.tos_minutes_left === 0 ? 'TOS 过期' : 'TOS 异常')}
            </span>
          )}
          {entry.status === 'error' && <span className="text-rose-400">无 ref</span>}
          {entry.tos_fresh && entry.tos_minutes_left !== undefined && (
            <span className="text-gray-600">剩 {entry.tos_minutes_left}m</span>
          )}
          {entry.ref_video && (
            <a
              href={entry.ref_video}
              target="_blank"
              rel="noreferrer"
              onClick={e => e.stopPropagation()}
              title={`v2v 参考视频\n${entry.ref_video}`}
              className="inline-flex items-center gap-0.5 px-1 rounded bg-violet-900/30 border border-violet-700/40 text-violet-300 hover:text-white hover:bg-violet-800/50">
              <Film className="w-2.5 h-2.5" /> v2v
            </a>
          )}
          <span className="text-gray-600 ml-auto">{entry.used_by.length} 集用到</span>
        </div>
      </div>
    </div>
  )
}

// ── 各集表 ──

function EpisodeTable({ rows, onDrill }: { rows: EpisodeSweepRow[]; onDrill: (r: EpisodeSweepRow) => void }) {
  // 按状态分组：错误的在最上面
  const sorted = useMemo(() => {
    return [...rows].sort((a, b) => {
      const sa = a.l1_errors > 0 ? 0 : (a.l2_warns > 0 ? 1 : 2)
      const sb = b.l1_errors > 0 ? 0 : (b.l2_warns > 0 ? 1 : 2)
      if (sa !== sb) return sa - sb
      // 同状态保留原（season+number）顺序
      return 0
    })
  }, [rows])

  if (rows.length === 0) {
    return (
      <section className="rounded-lg border border-gray-800 bg-gray-900/40 px-3 py-6 text-xs text-gray-600 italic text-center">
        画布上没有剧集节点 —— 先点「虫群宇宙」加载。
      </section>
    )
  }

  return (
    <section className="rounded-lg border border-gray-800 bg-gray-900/40">
      <header className="px-3 py-2 border-b border-gray-800/80 flex items-center gap-3">
        <span className="text-[11px] font-medium text-gray-300 uppercase tracking-wider">各集状态 · {rows.length} 集</span>
        <span className="text-[10.5px] text-gray-500 flex-1">
          致命 {rows.filter(r => r.l1_errors > 0).length} · 黄线 {rows.filter(r => r.l2_warns > 0 && r.l1_errors === 0).length} · 全绿 {rows.filter(r => r.l1_errors === 0 && r.l2_warns === 0).length}
        </span>
      </header>
      <div className="divide-y divide-gray-800/60">
        {sorted.map(r => <EpisodeRow key={r.nodeId} row={r} onDrill={() => onDrill(r)} />)}
      </div>
    </section>
  )
}

function EpisodeRow({ row, onDrill }: { row: EpisodeSweepRow; onDrill: () => void }) {
  const statusColor = row.l1_errors > 0 ? 'text-rose-400'
    : row.l2_warns > 0 ? 'text-amber-400'
    : 'text-emerald-400'
  const StatusIcon = row.l1_errors > 0 ? CircleX
    : row.l2_warns > 0 ? Sparkles
    : CheckCircle2
  const pickedColor = row.picked_count === row.scenes ? 'text-emerald-400'
    : row.picked_count === 0 ? 'text-gray-500'
    : 'text-amber-300'
  return (
    <button onClick={onDrill}
      className="w-full flex items-center gap-3 px-3 py-2 text-left hover:bg-gray-800/60 transition group">
      <StatusIcon className={`w-3.5 h-3.5 flex-none ${statusColor}`} />
      <span className="font-mono text-[11px] text-gray-300 w-12 flex-none">{row.episode_key}</span>
      <span className="text-xs text-gray-200 flex-1 truncate">{row.episode.label}</span>
      <span className="text-[10.5px] text-gray-500 w-20 flex-none text-right">{row.scenes}镜·{row.duration_s}s</span>
      <span className={`text-[10.5px] w-16 flex-none text-right ${pickedColor}`}>已选 {row.picked_count}/{row.scenes}</span>
      <span className="w-20 flex-none text-right text-[10.5px]">
        {row.l1_errors > 0 && <span className="text-rose-400">L1×{row.l1_errors}</span>}
        {row.l1_errors > 0 && row.l2_warns > 0 && <span className="text-gray-600 mx-1">·</span>}
        {row.l2_warns > 0 && <span className="text-amber-400">L2×{row.l2_warns}</span>}
        {row.l1_errors === 0 && row.l2_warns === 0 && <span className="text-gray-500">—</span>}
      </span>
      <ChevronRight className="w-3.5 h-3.5 flex-none text-gray-600 group-hover:text-gray-300" />
    </button>
  )
}
