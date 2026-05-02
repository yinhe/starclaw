// ── PreflightModal · 派单前自检 UI ──
//
// 点「开始生产 EPxx」不再直派 Seedance，而是先开这个 modal：
//   1. 并发跑 preflight.ts 里的所有检查
//   2. L1 红色错误显示 [🔧 一键修复] 按钮 → 调 /v1/projects/:p/ref/suggest 候选图 →
//      用户挑一个 → PUT /v1/projects/:p/manifest/characters/:k → 重跑 preflight
//   3. 全部 L1 绿之后，「开始派单」按钮亮起 → 调用 onProceed

import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState } from 'react'
import type { Node } from '@xyflow/react'
import {
  AlertTriangle, CheckCircle2, CircleX, Info, Loader2, Maximize2, RefreshCw,
  Sparkles, Wrench, X,
} from 'lucide-react'

import type { EpisodeData } from './episodeTypes'
import {
  preflightEpisode,
  type PreflightCheck,
  type PreflightReport,
} from './preflight'
import { projectAPI, cdnAPI } from '../../lib/api'
import { refreshTOS } from './tosUrlUtils'

// ── 图片大图预览 Lightbox · 共享 Context ──
//
// 自检面板里所有可访问的图片（角色 ref、道具 ref、TOS 签名图、本地原图）
// 都应该可以点开看大图，便于核对一致性。用一个 Provider 暴露 openPreview，
// 子组件不需要逐层 prop drill。
type PreviewItem = { url: string; label?: string }
type OpenPreview = (url: string, label?: string) => void
const PreviewCtx = createContext<OpenPreview>(() => {})
const usePreview = () => useContext(PreviewCtx)

function ImageLightbox({ item, onClose }: { item: PreviewItem | null; onClose: () => void }) {
  // ESC 关闭
  useEffect(() => {
    if (!item) return
    const onKey = (e: KeyboardEvent) => { if (e.key === 'Escape') onClose() }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [item, onClose])
  if (!item) return null
  return (
    <div
      className="fixed inset-0 z-[60] flex items-center justify-center bg-black/85 backdrop-blur-sm cursor-zoom-out"
      onClick={onClose}
    >
      <button
        onClick={(e) => { e.stopPropagation(); onClose() }}
        className="absolute top-4 right-4 p-2 rounded-full bg-gray-900/80 hover:bg-gray-800 text-gray-300 hover:text-white"
        title="关闭 (Esc)">
        <X className="w-5 h-5" />
      </button>
      <div className="max-w-[92vw] max-h-[88vh] flex flex-col items-center gap-2" onClick={e => e.stopPropagation()}>
        <img
          src={item.url}
          alt={item.label || 'preview'}
          className="max-w-full max-h-[82vh] object-contain rounded shadow-2xl bg-gray-950"
          onError={e => { (e.target as HTMLImageElement).alt = '加载失败：' + (item.label || item.url) }}
        />
        {item.label && <div className="text-xs text-gray-300">{item.label}</div>}
        <a href={item.url} target="_blank" rel="noreferrer"
          className="text-[11px] text-cyan-400 hover:text-cyan-300 break-all max-w-full text-center">
          {item.url}
        </a>
      </div>
    </div>
  )
}

// 通用可点缩略图：点击后打开 Lightbox。url 缺失时回退展示一个 X 占位。
function PreviewThumb({ url, label, size = 40 }: { url?: string; label?: string; size?: number }) {
  const open = usePreview()
  const px = `${size}px`
  if (!url) {
    return (
      <div style={{ width: px, height: px }} className="bg-gray-950 flex items-center justify-center flex-none rounded">
        <CircleX className="w-3 h-3 text-rose-400" />
      </div>
    )
  }
  return (
    <button
      type="button"
      onClick={(e) => { e.stopPropagation(); open(url, label) }}
      style={{ width: px, height: px }}
      className="relative group bg-gray-950 flex-none rounded overflow-hidden border border-gray-800 hover:border-cyan-500 transition"
      title={`点击查看大图\n${label || ''}`}>
      <img src={url} alt={label || ''} className="w-full h-full object-cover" loading="lazy"
        onError={e => { (e.target as HTMLImageElement).style.opacity = '0.2' }} />
      <span className="absolute inset-0 bg-black/40 opacity-0 group-hover:opacity-100 transition flex items-center justify-center">
        <Maximize2 className="w-3.5 h-3.5 text-white" />
      </span>
    </button>
  )
}

// 推断一个 URL 是否值得作为图片预览（manifest 里的 ref 大都是 png/jpg/webp，
// Volcengine TOS 签名是 .png?X-Tos-... 这种形式）。
function looksLikeImageUrl(u?: string): boolean {
  if (!u) return false
  const path = u.split('?')[0].toLowerCase()
  return /\.(png|jpe?g|webp|gif|avif)$/i.test(path)
}

interface PreflightModalProps {
  episode: EpisodeData
  nodes: Node[]
  onClose: () => void
  onProceed: () => void
  onRegenAll?: () => void
  project?: string
  /**
   * 修复器（如 stale_tos 一键刷新）成功后，把新值写回到对应 media 节点的 data。
   * 没传的话自检面板重扫时会读到旧 URL——用户看到的就是“明明修了但面板状态不动”。
   */
  onPatchNode?: (nodeId: string, patch: Record<string, unknown>) => void
}

export default function PreflightModal({ episode, nodes, onClose, onProceed, onRegenAll, project = 'swarm-universe', onPatchNode }: PreflightModalProps) {
  const [report, setReport] = useState<PreflightReport | null>(null)
  const [running, setRunning] = useState(true)
  const [rerunNonce, setRerunNonce] = useState(0)
  // 同时运行的修复：用 Set，每个修复读自己的状态 busy，其他奖圈 L2 按钮不会互封。
  // 「修复全部」也通过同一套 setBusy/clearBusy 更新 UI。
  const [busyFixIds, setBusyFixIds] = useState<Set<string>>(new Set())
  const [fixMessage, setFixMessage] = useState<string | null>(null)

  const setBusy = useCallback((id: string | null, flag: boolean = true) => {
    if (!id) return
    setBusyFixIds(prev => {
      const next = new Set(prev)
      if (flag) next.add(id); else next.delete(id)
      return next
    })
  }, [])

  // nodes 是父级的 React state，在 WorkflowPage 任何 setState 后数组引用改变 →
  // 若放进 useCallback deps 会导致 run 每帧重建，useEffect 毫无仰制地起动 preflight，
  // 表现出来就是用户报的“不停闪屏”。常规修法：用 ref 搶新值，run 只依赖稳定引用。
  const nodesRef = useRef(nodes)
  useEffect(() => { nodesRef.current = nodes }, [nodes])

  const run = useCallback(async () => {
    setRunning(true)
    setReport(null)
    try {
      const r = await preflightEpisode({ episode, nodes: nodesRef.current, project })
      setReport(r)
    } catch (e) {
      const msg = (e as Error)?.message || String(e)
      setReport({
        generated_at: new Date().toISOString(),
        episode_label: episode.label,
        episode_key: '',
        project,
        can_proceed: false,
        checks: [{
          id: 'preflight-crash', level: 1, status: 'error',
          label: '自检引擎抛异常', detail: msg,
        }],
        summary: { scenes: 0, duration_s: 0, characters: [], props: [], tail_frames: [], model: '', resolution: '', needs_upstream_frame: false },
      })
    } finally {
      setRunning(false)
    }
  }, [episode, project])

  useEffect(() => { void run() }, [run, rerunNonce])

  const { l1, l2, l3 } = useMemo(() => {
    const out = { l1: [] as PreflightCheck[], l2: [] as PreflightCheck[], l3: [] as PreflightCheck[] }
    if (!report) return out
    for (const c of report.checks) {
      if (c.level === 1) out.l1.push(c)
      else if (c.level === 2) out.l2.push(c)
      else out.l3.push(c)
    }
    return out
  }, [report])

  const canProceed = !!report?.can_proceed && !running && busyFixIds.size === 0

  // 大图预览 Lightbox 共享状态
  const [previewItem, setPreviewItem] = useState<PreviewItem | null>(null)
  const openPreview = useCallback<OpenPreview>((url, label) => setPreviewItem({ url, label }), [])

  return (
    <PreviewCtx.Provider value={openPreview}>
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/70 backdrop-blur-sm">
      <div className="relative w-[min(880px,92vw)] max-h-[92vh] overflow-hidden flex flex-col rounded-xl border border-gray-700 bg-gray-950 shadow-2xl">
        <Header episode={episode.label} running={running} canProceed={canProceed} onClose={onClose} onRerun={() => setRerunNonce(n => n + 1)} />

        <div className="flex-1 overflow-auto px-5 py-4 space-y-5">
          {fixMessage && (
            <div className="rounded border border-cyan-700/50 bg-cyan-900/20 text-cyan-200 text-xs px-3 py-2 flex items-start gap-2">
              <Sparkles className="w-3.5 h-3.5 mt-0.5 flex-none" />
              <span className="flex-1">{fixMessage}</span>
            </div>
          )}

          <Section title="L1 · 致命检查（未通过不允许派单）" tone="error" checks={l1}
            onFix={async (c) => { await handleFix(c, { project, setBusy, setMsg: setFixMessage, rerun: () => setRerunNonce(n => n + 1), onPatchNode }) }}
            busyFixIds={busyFixIds}
            onFixAll={async () => {
              // 并发炼所有 L1 有 fixer 的行。rerun 只在最后触发一次，避免每个 fix 都弹一遍自检。
              const fixables = l1.filter(c => !!c.fixer)
              if (fixables.length === 0) return
              setFixMessage(`正在并发修复 ${fixables.length} 条 L1 错误…`)
              await Promise.allSettled(fixables.map(c => handleFix(c, {
                project, setBusy, setMsg: setFixMessage,
                rerun: () => {}, // 暂时不逐个 rerun
                onPatchNode,
              })))
              setRerunNonce(n => n + 1)
            }} />
          <Section title="L2 · 黄线警告（通过但提示）" tone="warn" checks={l2}
            onFix={async (c) => { await handleFix(c, { project, setBusy, setMsg: setFixMessage, rerun: () => setRerunNonce(n => n + 1), onPatchNode }) }}
            busyFixIds={busyFixIds} />
          <Section title="L3 · 信息" tone="info" checks={l3} />

          {report && <SummaryCard report={report} />}
        </div>

        <Footer running={running} canProceed={canProceed} hasErrors={l1.some(c => c.status === 'error')}
          pickedCount={(episode.scenes || []).filter(s => s.picked_take).length}
          sceneCount={(episode.scenes || []).length}
          onClose={onClose} onRerun={() => setRerunNonce(n => n + 1)} onProceed={onProceed} onRegenAll={onRegenAll} />
      </div>
      <ImageLightbox item={previewItem} onClose={() => setPreviewItem(null)} />
    </div>
    </PreviewCtx.Provider>
  )
}

// ── Header ──

function Header({ episode, running, canProceed, onClose, onRerun }: {
  episode: string; running: boolean; canProceed: boolean; onClose: () => void; onRerun: () => void
}) {
  return (
    <div className="flex items-center gap-3 px-5 py-3 border-b border-gray-800">
      <div className={`w-9 h-9 rounded-full flex items-center justify-center flex-none ${
        running ? 'bg-blue-900/50' : canProceed ? 'bg-emerald-900/50' : 'bg-rose-900/50'
      }`}>
        {running ? <Loader2 className="w-4 h-4 text-blue-400 animate-spin" />
          : canProceed ? <CheckCircle2 className="w-4 h-4 text-emerald-400" />
          : <CircleX className="w-4 h-4 text-rose-400" />}
      </div>
      <div className="flex-1 min-w-0">
        <div className="text-sm font-semibold text-gray-100 truncate">派单前自检 · {episode}</div>
        <div className="text-[11px] text-gray-500">
          {running ? '跑中…' : canProceed ? '全部通过，可以派单' : '有致命错误，请先修复'}
        </div>
      </div>
      <button onClick={onRerun} disabled={running} title="重新自检"
        className="p-1.5 rounded hover:bg-gray-800 text-gray-400 hover:text-gray-200 disabled:opacity-40">
        <RefreshCw className={`w-4 h-4 ${running ? 'animate-spin' : ''}`} />
      </button>
      <button onClick={onClose} title="关闭"
        className="p-1.5 rounded hover:bg-gray-800 text-gray-400 hover:text-gray-200">
        <X className="w-4 h-4" />
      </button>
    </div>
  )
}

// ── Section（一个 level 的结果集） ──

function Section({ title, tone, checks, onFix, busyFixIds, onFixAll }: {
  title: string
  tone: 'error' | 'warn' | 'info'
  checks: PreflightCheck[]
  onFix?: (c: PreflightCheck) => void
  busyFixIds?: Set<string>
  onFixAll?: () => void
}) {
  const toneBorder = tone === 'error' ? 'border-rose-900/50' : tone === 'warn' ? 'border-amber-900/50' : 'border-gray-800'
  const fixableCount = checks.filter(c => !!c.fixer && c.status === 'error').length
  const anyBusy = !!busyFixIds && busyFixIds.size > 0
  return (
    <section className={`rounded-lg border ${toneBorder} bg-gray-900/40`}>
      <header className="px-3 py-2 border-b border-gray-800/80 flex items-center gap-2">
        <span className="text-[11px] font-medium text-gray-400 uppercase tracking-wider flex-1">
          {title} <span className="text-gray-600">· {checks.length}</span>
        </span>
        {onFixAll && fixableCount >= 2 && (
          <button
            onClick={onFixAll}
            disabled={anyBusy}
            className="text-[10.5px] px-2 py-1 rounded border border-cyan-700 bg-cyan-900/30 text-cyan-200 hover:bg-cyan-800/40 disabled:opacity-50 flex items-center gap-1"
            title={`并发运行所有 ${fixableCount} 个修复器`}>
            {anyBusy ? <Loader2 className="w-3 h-3 animate-spin" /> : <Wrench className="w-3 h-3" />}
            修复全部 ({fixableCount})
          </button>
        )}
      </header>
      {checks.length === 0 ? (
        <div className="px-3 py-3 text-xs text-gray-600 italic">无</div>
      ) : (
        <ul className="divide-y divide-gray-800/70">
          {checks.map(c => (
            <CheckRow key={c.id} check={c} onFix={onFix} busy={!!busyFixIds?.has(c.id)} />
          ))}
        </ul>
      )}
    </section>
  )
}

// ── 单行 ──

function CheckRow({ check, onFix, busy }: { check: PreflightCheck; onFix?: (c: PreflightCheck) => void; busy?: boolean }) {
  const Icon = check.status === 'ok' ? CheckCircle2
    : check.status === 'error' ? CircleX
    : check.status === 'warn' ? AlertTriangle
    : Info
  const color = check.status === 'ok' ? 'text-emerald-400'
    : check.status === 'error' ? 'text-rose-400'
    : check.status === 'warn' ? 'text-amber-400'
    : 'text-gray-500'
  const hasDiffPreview = !!(check.id.startsWith('tos-diff-') && check.data?.tos_url && check.data?.local_url)
  // ref-/tos-req- 这类检查的 data.url 是图片地址，单条直接挂个可点缩略图，
  // 用户不用展开 detail 就能眼瞄一眼有没有挂错图。
  const inlineUrl = (() => {
    const d = check.data as Record<string, unknown> | undefined
    if (!d) return undefined
    const candidates = [d.url, d.tos_url, d.local_url, d.imageUrl]
    for (const u of candidates) {
      if (typeof u === 'string' && looksLikeImageUrl(u)) return u
    }
    return undefined
  })()
  return (
    <li className="px-3 py-2">
      <div className="flex items-start gap-2.5">
        <Icon className={`w-3.5 h-3.5 mt-0.5 flex-none ${color}`} />
        {inlineUrl && !hasDiffPreview && (
          <PreviewThumb url={inlineUrl} label={check.label} size={32} />
        )}
        <div className="flex-1 min-w-0 text-xs">
          <div className="text-gray-200 leading-tight">{check.label}</div>
          {check.detail && (
            <div className="text-[10.5px] text-gray-500 mt-0.5 break-all whitespace-pre-wrap">{check.detail}</div>
          )}
        </div>
        {check.fixer && onFix && (
          <button
            onClick={() => onFix(check)}
            disabled={busy}
            className="flex-none text-[10.5px] px-2 py-1 rounded border border-cyan-800 bg-cyan-900/20 text-cyan-300 hover:bg-cyan-900/40 disabled:opacity-50 flex items-center gap-1">
            {busy ? <Loader2 className="w-3 h-3 animate-spin" /> : <Wrench className="w-3 h-3" />}
            {busy ? '修复中' : '一键修复'}
          </button>
        )}
      </div>
      {hasDiffPreview && (
        <TOSDiffThumbnails tosUrl={String(check.data!.tos_url)} localUrl={String(check.data!.local_url)} label={String(check.data!.character_label || '')} />
      )}
    </li>
  )
}

// ── TOS vs 本地原图缩略图对比 ──

function TOSDiffThumbnails({ tosUrl, localUrl, label }: { tosUrl: string; localUrl: string; label: string }) {
  const [expanded, setExpanded] = useState(false)
  return (
    <div className="mt-1.5 ml-6">
      <button onClick={() => setExpanded(v => !v)} className="text-[10px] text-cyan-400 hover:text-cyan-300 underline">
        {expanded ? '收起对比' : '展开对比缩略图'}
      </button>
      {expanded && (
        <div className="mt-1 flex gap-2 items-start">
          <div className="text-center">
            <div className="text-[9px] text-gray-500 mb-0.5">TOS 图（点击放大）</div>
            <PreviewThumb url={tosUrl} label={`${label} · TOS`} size={96} />
          </div>
          <div className="text-center">
            <div className="text-[9px] text-gray-500 mb-0.5">本地原图（点击放大）</div>
            <PreviewThumb url={localUrl} label={`${label} · 本地`} size={96} />
          </div>
        </div>
      )}
    </div>
  )
}

// ── Summary Card（L3 详细展示 + 角色缩略图条） ──

function SummaryCard({ report }: { report: PreflightReport }) {
  return (
    <section className="rounded-lg border border-gray-800 bg-gray-900/30 px-3 py-3">
      <div className="text-[11px] uppercase tracking-wider text-gray-500 mb-2">派单摘要</div>
      <dl className="grid grid-cols-2 gap-x-3 gap-y-1 text-[11px] text-gray-300">
        <dt className="text-gray-500">EP</dt>
        <dd className="font-mono">{report.episode_key}</dd>
        <dt className="text-gray-500">镜数 / 总时长</dt>
        <dd>{report.summary.scenes} 镜 / {report.summary.duration_s}s</dd>
        <dt className="text-gray-500">模型</dt>
        <dd className="font-mono text-[10.5px]">{report.summary.model}</dd>
        <dt className="text-gray-500">分辨率</dt>
        <dd className="font-mono">{report.summary.resolution}</dd>
        <dt className="text-gray-500">尾帧链</dt>
        <dd>{report.summary.needs_upstream_frame ? '需要（S2→S3→…）' : '无'}</dd>
      </dl>

      {report.summary.tail_frames.length > 0 && (
        <>
          <div className="text-[11px] uppercase tracking-wider text-gray-500 mt-3 mb-1.5">各片段尾帧地址</div>
          <div className="space-y-1">
            {report.summary.tail_frames.map(tf => (
              <div key={tf.scene_id} className="flex items-center gap-2 text-[11px]">
                <span className="font-mono text-gray-400 w-16 flex-none">{tf.prev_scene_id}→{tf.scene_id}</span>
                {tf.status === 'ready' ? (
                  <span className="text-emerald-400 font-mono text-[10px] truncate flex-1" title={tf.lastframe_url || tf.video_url}>
                    ✓ {tf.lastframe_url || tf.video_url || '(video ready)'}
                  </span>
                ) : (
                  <span className="text-amber-400 text-[10px]">⚠ {tf.prev_scene_id} 未选定 picked_take</span>
                )}
              </div>
            ))}
          </div>
        </>
      )}

      <div className="text-[11px] uppercase tracking-wider text-gray-500 mt-3 mb-2">本集用到的角色</div>
      <div className="flex flex-wrap gap-2">
        {report.summary.characters.map(c => (
          <div key={c.tag} className="flex items-center gap-1.5 text-[11px] rounded bg-gray-900 border border-gray-800 overflow-hidden">
            <PreviewThumb url={c.url} label={`${c.tag} ${c.label}`} size={40} />
            <div className="pr-2 py-1">
              <div className="text-gray-200 font-medium">{c.tag} {c.label}</div>
              <div className="text-[10px] text-gray-500 uppercase tracking-wider">
                {c.source === 'tos' ? 'TOS' : c.source === 'local' ? 'LOCAL' : c.source === 'v2v' ? 'V2V' : 'MISSING'}
                {c.key && <span className="ml-1 text-gray-600 normal-case font-mono tracking-normal">· {c.key}</span>}
              </div>
            </div>
          </div>
        ))}
      </div>

      <div className="text-[11px] uppercase tracking-wider text-gray-500 mt-3 mb-2">
        本集用到的道具 <span className="text-gray-600 normal-case tracking-normal">· 剧本里按 label 点名、manifest 里有 ref 的</span>
      </div>
      {report.summary.props.length > 0 ? (
        <div className="flex flex-wrap gap-2">
          {report.summary.props.map(p => (
            <div key={p.key} className="flex items-center gap-1.5 text-[11px] rounded bg-gray-900 border border-gray-800 overflow-hidden">
              <PreviewThumb url={p.url} label={`「${p.label}」`} size={40} />
              <div className="pr-2 py-1">
                <div className="text-gray-200 font-medium">「{p.label}」</div>
                <div className="text-[10px] text-gray-500 uppercase tracking-wider">
                  {p.source === 'tos' ? 'TOS' : p.source === 'local' ? 'LOCAL' : 'MISSING'}
                  <span className="ml-1 text-gray-600 normal-case font-mono tracking-normal">· {p.key}</span>
                </div>
              </div>
            </div>
          ))}
        </div>
      ) : (
        <div className="text-[11px] text-gray-600">本集剧本未引用有参考图的道具</div>
      )}
    </section>
  )
}

// ── Footer ──

function Footer({ running, canProceed, hasErrors, pickedCount, sceneCount, onClose, onRerun, onProceed, onRegenAll }: {
  running: boolean; canProceed: boolean; hasErrors: boolean
  pickedCount: number; sceneCount: number
  onClose: () => void; onRerun: () => void; onProceed: () => void; onRegenAll?: () => void
}) {
  return (
    <div className="flex items-center gap-2 px-5 py-3 border-t border-gray-800 bg-gray-950">
      <button onClick={onClose}
        className="px-3 py-1.5 text-xs rounded border border-gray-700 text-gray-300 hover:bg-gray-800">
        取消
      </button>
      <button onClick={onRerun} disabled={running}
        className="px-3 py-1.5 text-xs rounded border border-gray-700 text-gray-300 hover:bg-gray-800 disabled:opacity-40 flex items-center gap-1">
        <RefreshCw className={`w-3 h-3 ${running ? 'animate-spin' : ''}`} /> 重新自检
      </button>
      <div className="flex-1" />
      {hasErrors && (
        <div className="text-[11px] text-rose-300 mr-2 flex items-center gap-1">
          <CircleX className="w-3 h-3" /> 修复所有 L1 错误后才能派单
        </div>
      )}
      {onRegenAll && pickedCount > 0 && (
        <button onClick={onRegenAll} disabled={!canProceed}
          title={`清除 ${pickedCount} 个已选 take，全部 ${sceneCount} 个场景重新生成`}
          className={`px-4 py-1.5 text-xs rounded font-semibold flex items-center gap-1.5 ${
            canProceed
              ? 'bg-amber-600/30 border border-amber-500/50 text-amber-200 hover:bg-amber-600/40'
              : 'bg-gray-800 text-gray-600 cursor-not-allowed'
          }`}>
          <RefreshCw className="w-3.5 h-3.5" /> 重拍全集
        </button>
      )}
      <button onClick={onProceed} disabled={!canProceed}
        className={`px-4 py-1.5 text-xs rounded font-semibold flex items-center gap-1.5 ${
          canProceed
            ? 'bg-gradient-to-r from-cyan-500 to-blue-500 text-white hover:brightness-110'
            : 'bg-gray-800 text-gray-600 cursor-not-allowed'
        }`}>
        <Sparkles className="w-3.5 h-3.5" /> {pickedCount > 0 ? '续接派单' : '开始派单'}
      </button>
    </div>
  )
}

// ── 一键修复 dispatcher ──
//
// 目前支持两种 fixer：
//   broken_ref  → 候选图挑选 dialog → PUT manifest.characters.:key
//   stale_tos   → refreshTOS （resign/launder 现有工具）

async function handleFix(
  check: PreflightCheck,
  ctx: {
    project: string
    setBusy: (id: string | null, flag?: boolean) => void
    setMsg: (s: string | null) => void
    rerun: () => void
    onPatchNode?: (nodeId: string, patch: Record<string, unknown>) => void
  },
) {
  const f = check.fixer
  if (!f) return
  ctx.setMsg(null)
  ctx.setBusy(check.id, true)
  try {
    if (f.kind === 'broken_ref') {
      // 1. 拿候选
      const res = await projectAPI.suggestRef(f.project, {
        character_key: f.character_key,
        character_label: f.character_label,
        broken_ref: f.broken_ref,
        limit: 8,
      })
      const candidates = (res.data?.candidates || []) as Array<{ path: string; url: string; size: number; score: number; reason: string }>
      if (candidates.length === 0) {
        ctx.setMsg(`未扫到任何 ${f.character_label} 的候选图 —— 检查 docs/${ctx.project}/ 下是否真的有这个角色的图片。`)
        return
      }
      const pick = await showCandidatePicker(f, candidates)
      if (!pick) return
      // 2. 写 manifest
      const patched = await projectAPI.setCharacterRef(ctx.project, f.character_key, pick.path)
      // 3a. 同步把新 imageUrl 写回到对应节点 —— 不用 reload 家有了这句就可以。
      if (ctx.onPatchNode && f.node_id) {
        ctx.onPatchNode(f.node_id, { imageUrl: pick.url })
      }
      ctx.setMsg(`已将 ${f.character_label} 的 ref 更新为 ${pick.path}（${Math.round((pick.size || 0) / 1024)} KB）。${patched.data?.note || ''}`)
      // 3b. 重扫自检 —— 新值已在 nodesRef 里，preflight 会看到 200 OK 变绿。
      ctx.rerun()
    } else if (f.kind === 'stale_tos') {
      // TOS URL 现在是派单必填项（角色 + 剧本引用的道具都要求）。
      // 本地 ref L1 已确认 200 OK，把它作为 fallbackSource 传进去，
      // refreshTOS 的 promote/launder 路径就能走「本地原图 → Seedream → 新 TOS」。
      const localRef = f.local_ref_url || ''
      const entityKind: 'characters' | 'props' = f.entity_kind || 'characters'
      const entityKey = f.entity_key || f.character_key || ''
      try {
        const r = await refreshTOS(f.tos_url || '', localRef)
        if (r && r.tosUrl) {
          if (ctx.onPatchNode && f.node_id) {
            ctx.onPatchNode(f.node_id, { tos_url: r.tosUrl })
          }
          // 持久化到 manifest —— 同类型 /v1/projects/:p/manifest/:kind/:key
          // characters 用 character_key 兼容；props 走 entity_key。
          if (entityKey) {
            try {
              await projectAPI.patchManifestEntity(ctx.project, entityKind, entityKey, { tos_url: r.tosUrl })
            } catch (persistErr) {
              console.warn(`[handleFix/stale_tos] 写回 manifest.${entityKind}.${entityKey}.tos_url 失败（节点已更新，刷页会恢复旧值）：`, persistErr)
            }
          }
          const kindLabel = entityKind === 'props' ? '道具' : '角色'
          ctx.setMsg(`已为${kindLabel}「${f.character_label}」刷新 TOS URL（via ${r.source}${r.expiresSec ? ', ' + Math.round(r.expiresSec / 3600) + 'h 有效' : ''}）。正在重扫…`)
          ctx.rerun()
          return
        }
      } catch (err) {
        const ax = err as { response?: { data?: { error?: string; detail?: string } }; message?: string }
        const detail = ax?.response?.data?.error || ax?.response?.data?.detail || ax?.message || String(err)
        const kindLabel = entityKind === 'props' ? '道具' : '角色'
        ctx.setMsg(`刷新${kindLabel}「${f.character_label}」TOS URL 失败：${detail}${localRef ? '（本地 ref 是 ' + (localRef.split('/').pop() || '') + '，可去角色/道具面板手动再试）' : ''}`)
        // 不 rerun —— 状态没变，L1 error 保留原样
        return
      }
      ctx.setMsg(`刷新 ${f.character_label} TOS URL 失败（返回空值），建议去节点属性面板手动点「洗 TOS URL」重试。`)
      ctx.rerun()
    } else if (f.kind === 'no_api_key') {
      ctx.setMsg(`请到「模型配置」页新增 ${f.provider} API key 后再重试自检。`)
    }
  } catch (e) {
    const ax = e as { response?: { data?: { error?: string; detail?: string } }; message?: string }
    const detail = ax?.response?.data?.error || ax?.response?.data?.detail || ax?.message || String(e)
    ctx.setMsg(`修复失败：${detail}`)
  } finally {
    ctx.setBusy(check.id, false)
  }
}

// ── 候选图选择弹窗（简易版，复用 window.confirm + 自构 DOM） ──

interface CandidatePickArgs { path: string; url: string; size: number; score: number; reason: string }

function showCandidatePicker(
  f: Extract<import('./preflight').CheckFixer, { kind: 'broken_ref' }>,
  candidates: CandidatePickArgs[],
): Promise<CandidatePickArgs | null> {
  return new Promise(resolve => {
    const host = document.createElement('div')
    host.className = 'fixed inset-0 z-[60] flex items-center justify-center bg-black/80 backdrop-blur-sm'
    host.style.padding = '20px'

    const panel = document.createElement('div')
    panel.className = 'w-[min(720px,92vw)] max-h-[80vh] overflow-hidden flex flex-col rounded-xl border border-cyan-800 bg-gray-950'
    host.appendChild(panel)

    panel.innerHTML = `
      <div class="flex items-center gap-3 px-5 py-3 border-b border-gray-800">
        <div class="flex-1 min-w-0">
          <div class="text-sm font-semibold text-gray-100">一键修复 · 挑选 ${escapeHtml(f.character_label)} (${escapeHtml(f.character_tag || '')}) 的新 ref</div>
          <div class="text-[11px] text-gray-500">扫到 ${candidates.length} 张候选，按评分排序。选中后会写回 manifest.json 的 <code class="text-cyan-400">${escapeHtml(f.character_key)}.ref</code> 字段。</div>
        </div>
        <button id="pf-cancel" class="p-1.5 rounded hover:bg-gray-800 text-gray-400" title="取消">✕</button>
      </div>
      <div class="flex-1 overflow-auto p-3 grid grid-cols-2 gap-2">
        ${candidates.map((c, i) => `
          <button data-idx="${i}" class="pf-cand text-left rounded border border-gray-800 hover:border-cyan-600 bg-gray-900/60 overflow-hidden transition">
            <div class="aspect-video bg-gray-950 overflow-hidden">
              <img src="${escapeAttr(c.url)}" alt="" class="w-full h-full object-cover" loading="lazy" />
            </div>
            <div class="px-2 py-1.5 text-[11px]">
              <div class="text-gray-200 font-mono truncate">${escapeHtml(c.path)}</div>
              <div class="flex items-center gap-2 mt-0.5 text-[10px] text-gray-500">
                <span>${Math.round(c.size / 1024)} KB</span>
                <span>score ${c.score}</span>
                <span class="text-gray-600 truncate">${escapeHtml(c.reason)}</span>
              </div>
            </div>
          </button>
        `).join('')}
      </div>
      <div class="px-4 py-2 border-t border-gray-800 text-[10.5px] text-gray-500">
        提示：选完后页面会刷新以让新 manifest 生效。原 ref：<code class="text-rose-300">${escapeHtml(f.broken_ref || '(空)')}</code>
      </div>
    `
    document.body.appendChild(host)

    const cleanup = (ret: CandidatePickArgs | null) => {
      host.remove()
      resolve(ret)
    }

    host.addEventListener('click', (e) => {
      if (e.target === host) cleanup(null)
    })
    host.querySelector('#pf-cancel')?.addEventListener('click', () => cleanup(null))
    host.querySelectorAll<HTMLButtonElement>('.pf-cand').forEach(btn => {
      btn.addEventListener('click', () => {
        const idx = Number(btn.getAttribute('data-idx'))
        cleanup(candidates[idx])
      })
    })
  })
}

function escapeHtml(s: string): string {
  return s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;')
}
function escapeAttr(s: string): string { return escapeHtml(s) }

// 为了满足 lint（避免未使用的 cdnAPI 导入被 tree-shake 误杀）
void cdnAPI
