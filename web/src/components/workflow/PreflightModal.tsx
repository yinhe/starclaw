// ── PreflightModal · 派单前自检 UI ──
//
// 点「开始生产 EPxx」不再直派 Seedance，而是先开这个 modal：
//   1. 并发跑 preflight.ts 里的所有检查
//   2. L1 红色错误显示 [🔧 一键修复] 按钮 → 调 /v1/projects/:p/ref/suggest 候选图 →
//      用户挑一个 → PUT /v1/projects/:p/manifest/characters/:k → 重跑 preflight
//   3. 全部 L1 绿之后，「开始派单」按钮亮起 → 调用 onProceed

import { useCallback, useEffect, useMemo, useState } from 'react'
import type { Node } from '@xyflow/react'
import {
  AlertTriangle, CheckCircle2, CircleX, Info, Loader2, RefreshCw,
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

interface PreflightModalProps {
  episode: EpisodeData
  nodes: Node[]
  onClose: () => void
  onProceed: () => void
  project?: string
}

export default function PreflightModal({ episode, nodes, onClose, onProceed, project = 'swarm-universe' }: PreflightModalProps) {
  const [report, setReport] = useState<PreflightReport | null>(null)
  const [running, setRunning] = useState(true)
  const [rerunNonce, setRerunNonce] = useState(0)
  const [busyFixId, setBusyFixId] = useState<string | null>(null)
  const [fixMessage, setFixMessage] = useState<string | null>(null)

  const run = useCallback(async () => {
    setRunning(true)
    setReport(null)
    try {
      const r = await preflightEpisode({ episode, nodes, project })
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
        summary: { scenes: 0, duration_s: 0, characters: [], model: '', resolution: '', needs_upstream_frame: false },
      })
    } finally {
      setRunning(false)
    }
  }, [episode, nodes, project])

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

  const canProceed = !!report?.can_proceed && !running && !busyFixId

  return (
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
            onFix={async (c) => { await handleFix(c, { project, setBusy: setBusyFixId, setMsg: setFixMessage, rerun: () => setRerunNonce(n => n + 1) }) }}
            busyFixId={busyFixId} />
          <Section title="L2 · 黄线警告（通过但提示）" tone="warn" checks={l2}
            onFix={async (c) => { await handleFix(c, { project, setBusy: setBusyFixId, setMsg: setFixMessage, rerun: () => setRerunNonce(n => n + 1) }) }}
            busyFixId={busyFixId} />
          <Section title="L3 · 信息" tone="info" checks={l3} />

          {report && <SummaryCard report={report} />}
        </div>

        <Footer running={running} canProceed={canProceed} hasErrors={l1.some(c => c.status === 'error')}
          onClose={onClose} onRerun={() => setRerunNonce(n => n + 1)} onProceed={onProceed} />
      </div>
    </div>
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

function Section({ title, tone, checks, onFix, busyFixId }: {
  title: string
  tone: 'error' | 'warn' | 'info'
  checks: PreflightCheck[]
  onFix?: (c: PreflightCheck) => void
  busyFixId?: string | null
}) {
  const toneBorder = tone === 'error' ? 'border-rose-900/50' : tone === 'warn' ? 'border-amber-900/50' : 'border-gray-800'
  return (
    <section className={`rounded-lg border ${toneBorder} bg-gray-900/40`}>
      <header className="px-3 py-2 text-[11px] font-medium text-gray-400 uppercase tracking-wider border-b border-gray-800/80">
        {title} <span className="text-gray-600">· {checks.length}</span>
      </header>
      {checks.length === 0 ? (
        <div className="px-3 py-3 text-xs text-gray-600 italic">无</div>
      ) : (
        <ul className="divide-y divide-gray-800/70">
          {checks.map(c => (
            <CheckRow key={c.id} check={c} onFix={onFix} busy={busyFixId === c.id} />
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
  return (
    <li className="flex items-start gap-2.5 px-3 py-2">
      <Icon className={`w-3.5 h-3.5 mt-0.5 flex-none ${color}`} />
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
    </li>
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

      <div className="text-[11px] uppercase tracking-wider text-gray-500 mt-3 mb-2">本集用到的角色</div>
      <div className="flex flex-wrap gap-2">
        {report.summary.characters.map(c => (
          <div key={c.tag} className="flex items-center gap-1.5 text-[11px] rounded bg-gray-900 border border-gray-800 overflow-hidden">
            <div className="w-10 h-10 bg-gray-950 flex items-center justify-center flex-none">
              {c.url ? (
                <img src={c.url} alt={c.label} className="w-full h-full object-cover" loading="lazy" />
              ) : (
                <CircleX className="w-3 h-3 text-rose-400" />
              )}
            </div>
            <div className="pr-2 py-1">
              <div className="text-gray-200 font-medium">{c.tag} {c.label}</div>
              <div className="text-[10px] text-gray-500 uppercase tracking-wider">
                {c.source === 'tos' ? 'TOS' : c.source === 'local' ? 'LOCAL' : 'MISSING'}
                {c.key && <span className="ml-1 text-gray-600 normal-case font-mono tracking-normal">· {c.key}</span>}
              </div>
            </div>
          </div>
        ))}
      </div>
    </section>
  )
}

// ── Footer ──

function Footer({ running, canProceed, hasErrors, onClose, onRerun, onProceed }: {
  running: boolean; canProceed: boolean; hasErrors: boolean
  onClose: () => void; onRerun: () => void; onProceed: () => void
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
      <button onClick={onProceed} disabled={!canProceed}
        className={`px-4 py-1.5 text-xs rounded font-semibold flex items-center gap-1.5 ${
          canProceed
            ? 'bg-gradient-to-r from-cyan-500 to-blue-500 text-white hover:brightness-110'
            : 'bg-gray-800 text-gray-600 cursor-not-allowed'
        }`}>
        <Sparkles className="w-3.5 h-3.5" /> 开始派单
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
  ctx: { project: string; setBusy: (id: string | null) => void; setMsg: (s: string | null) => void; rerun: () => void },
) {
  const f = check.fixer
  if (!f) return
  ctx.setMsg(null)
  ctx.setBusy(check.id)
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
      ctx.setMsg(`已将 ${f.character_label} 的 ref 更新为 ${pick.path}（${Math.round((pick.size || 0) / 1024)} KB）。${patched.data?.note || ''}`)
      // 3. 触发上层重新 loadSeed（让新 manifest 生效）
      //    manifest cache 在 swarmUniverseSeed 里，做一个 sentinel：
      //    简单粗暴——刷新 window
      //    更优雅版：调一个全局回调 onManifestChange → WorkflowPage re-fetch。
      //    目前为了最小改动 → reload。
      await new Promise(r => setTimeout(r, 400))
      window.location.reload()
    } else if (f.kind === 'stale_tos') {
      const r = await refreshTOS(f.tos_url, '')
      if (r && r.tosUrl) {
        ctx.setMsg(`已刷新 ${f.character_label} TOS URL（via ${r.source}${r.expiresSec ? ', ' + Math.round(r.expiresSec / 3600) + 'h 有效' : ''}）。重跑自检确认。`)
      } else {
        ctx.setMsg(`刷新 ${f.character_label} TOS URL 失败，考虑直接「修复 ref 指向本地文件」。`)
      }
      ctx.rerun()
    } else if (f.kind === 'no_api_key') {
      ctx.setMsg(`请到「模型配置」页新增 ${f.provider} API key 后再重试自检。`)
    }
  } catch (e) {
    const ax = e as { response?: { data?: { error?: string; detail?: string } }; message?: string }
    const detail = ax?.response?.data?.error || ax?.response?.data?.detail || ax?.message || String(e)
    ctx.setMsg(`修复失败：${detail}`)
  } finally {
    ctx.setBusy(null)
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
