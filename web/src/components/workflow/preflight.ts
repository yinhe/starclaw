// ── 工作流派单前自检 · Preflight Engine ──
//
// 设计目标：在点「开始生产 EPxx」真正派 Seedance 之前，对这一集用到的
// 所有资源做一次地毯式检查，把可以本地发现的问题挡在派单之前，避免
// "等 3 分钟才知道 404" 这种烂情况。
//
// 三个层级（按阻断力排序）：
//   L1 · 致命（error）—— 不通过 不允许 派单
//   L2 · 黄线（warn） —— 通过但高亮警告，仍可派
//   L3 · 信息（info） —— 只展示，帮助判断
//
// 所有检查尽量并发执行 (Promise.all)，在 UI 上同时呈现进度。

import type { Node } from '@xyflow/react'
import type { EpisodeData, CharacterData } from './episodeTypes'
import { parseTOSFreshness } from './tosUrlUtils'

// ── 数据结构 ───────────────────────────────────────────

export type CheckStatus = 'ok' | 'warn' | 'error' | 'running'
export type CheckLevel = 1 | 2 | 3

export type CheckFixer =
  | {
      kind: 'broken_ref'
      project: string
      character_key: string
      character_label: string
      character_tag?: string
      broken_ref: string
      current_url?: string
    }
  | {
      kind: 'stale_tos'
      character_key?: string
      character_label: string
      tos_url: string
    }
  | {
      kind: 'no_api_key'
      provider: string
    }
  | null

export interface PreflightCheck {
  id: string
  level: CheckLevel
  status: CheckStatus
  label: string
  detail?: string
  fixer?: CheckFixer
  data?: Record<string, unknown>
}

export interface PreflightReport {
  generated_at: string
  episode_label: string
  episode_key: string          // "ep05" / "sp01"
  project: string              // "swarm-universe"
  checks: PreflightCheck[]
  can_proceed: boolean         // 无 L1 error
  summary: {
    scenes: number
    duration_s: number
    characters: Array<{ tag: string; label: string; key?: string; url?: string; source: 'tos' | 'local' | 'missing' }>
    model: string
    resolution: string
    needs_upstream_frame: boolean  // 有镜依赖 S(i-1).picked_take
  }
}

// ── 公开入口 ─────────────────────────────────────────

export interface PreflightArgs {
  episode: EpisodeData
  nodes: Node[]
  project?: string             // 默认 "swarm-universe"
  model?: string               // 默认 "doubao-seedance-2-0-260128"
  resolution?: string          // 默认 "720*1280"
}

export async function preflightEpisode(args: PreflightArgs): Promise<PreflightReport> {
  const project = args.project ?? 'swarm-universe'
  const model = args.model ?? 'doubao-seedance-2-0-260128'
  const resolution = args.resolution ?? '720*1280'

  const { episode, nodes } = args
  const episodeKey = deriveEpisodeKey(episode)

  const characters = collectCharactersFromNodes(nodes)

  // 先解析这一集实际用到的 [图N] tag 集合（来自 scene.prompt 中的引用）
  const usedTags = extractTagsFromScenes(episode)

  // 并发跑所有检查
  const [
    tagMatch,
    refReachability,
    tosFreshness,
    durationCheck,
    promptLengthCheck,
    upstreamCheck,
    summary,
  ] = await Promise.all([
    checkAllTagsMatched(usedTags, characters, project),
    checkAllRefsReachable(usedTags, characters),
    checkTOSFreshness(usedTags, characters),
    Promise.resolve(checkSceneDurations(episode)),
    Promise.resolve(checkPromptLengths(episode)),
    Promise.resolve(checkUpstreamFrames(episode)),
    Promise.resolve(buildSummary(episode, characters, usedTags, model, resolution)),
  ])

  const checks: PreflightCheck[] = [
    ...tagMatch,
    ...refReachability,
    ...tosFreshness,
    ...durationCheck,
    ...promptLengthCheck,
    ...upstreamCheck,
    ...summary,
  ]

  const can_proceed = !checks.some(c => c.level === 1 && c.status === 'error')

  return {
    generated_at: new Date().toISOString(),
    episode_label: episode.label,
    episode_key: episodeKey,
    project,
    checks,
    can_proceed,
    summary: buildSummaryObject(episode, characters, usedTags, model, resolution),
  }
}

// ── 内部：角色收集 & tag 解析 ──────────────────────

function collectCharactersFromNodes(nodes: Node[]): CharacterData[] {
  const chars: CharacterData[] = []
  for (const n of nodes) {
    const d = (n.data || {}) as Record<string, unknown>
    if (d.category === 'character') {
      chars.push({
        category: 'character',
        label: String(d.label || ''),
        tag: d.tag ? String(d.tag) : undefined,
        key: d.key ? String(d.key) : undefined,
        appearance_card: d.appearance_card ? String(d.appearance_card) : undefined,
        imageUrl: d.imageUrl ? String(d.imageUrl) : undefined,
        tos_url: d.tos_url ? String(d.tos_url) : undefined,
        description: d.description ? String(d.description) : undefined,
        role: d.role ? String(d.role) : undefined,
      })
    }
  }
  return chars
}

function extractTagsFromScenes(ep: EpisodeData): string[] {
  const tags = new Set<string>()
  for (const s of ep.scenes || []) {
    const text = `${s.prompt || ''} ${s.label || ''}`
    const re = /\[图[0-9]+\]/g
    let m: RegExpExecArray | null
    while ((m = re.exec(text)) !== null) tags.add(m[0])
  }
  return Array.from(tags).sort()
}

function deriveEpisodeKey(ep: EpisodeData): string {
  const n = ep.episode_number ?? 0
  const prefix = ep.is_spinoff ? 'sp' : 'ep'
  return `${prefix}${String(n).padStart(2, '0')}`
}

// ── L1.1：所有 [图N] 都能在 character 节点里找到匹配 ──

function checkAllTagsMatched(tags: string[], chars: CharacterData[], project: string): PreflightCheck[] {
  const byTag = new Map<string, CharacterData>()
  for (const c of chars) {
    if (c.tag) byTag.set(c.tag, c)
  }
  return tags.map(tag => {
    const c = byTag.get(tag)
    if (!c) {
      return {
        id: `tag-${tag}`,
        level: 1 as const,
        status: 'error' as const,
        label: `剧本引用 ${tag} 但找不到对应角色`,
        detail: '前端 characters 节点里没有 tag=' + tag + ' 的角色。请补 manifest.characters[].tag，或修正 scene.prompt。',
      }
    }
    return {
      id: `tag-${tag}`,
      level: 1 as const,
      status: 'ok' as const,
      label: `${tag} → ${c.label}`,
      detail: c.key ? `key=${c.key}${project === 'swarm-universe' ? '' : ` (${project})`}` : undefined,
      data: { character: c },
    }
  })
}

// ── L1.3：每个实际会派出去的 ref URL HTTP 200 ──
//   优先级：有效的 tos_url > imageUrl（本地 /v1/projects/...）
//   HEAD 请求超时 5s，任何非 2xx 视为 error

async function checkAllRefsReachable(tags: string[], chars: CharacterData[]): Promise<PreflightCheck[]> {
  const byTag = new Map<string, CharacterData>()
  for (const c of chars) if (c.tag) byTag.set(c.tag, c)

  const tasks = tags.map(async tag => {
    const c = byTag.get(tag)
    if (!c) {
      // tag 未匹配由 checkAllTagsMatched 报，这里跳过
      return null
    }
    const useTOS = !!c.tos_url && !isTOSStale(c.tos_url)
    const url = useTOS ? c.tos_url! : (c.imageUrl || '')
    const source: 'tos' | 'local' | 'missing' = useTOS ? 'tos' : (url ? 'local' : 'missing')

    if (!url) {
      return {
        id: `ref-${tag}`,
        level: 1 as const,
        status: 'error' as const,
        label: `${tag} ${c.label} 没有任何可用的 ref URL`,
        detail: 'manifest.ref 和 tos_url 都为空。',
        fixer: {
          kind: 'broken_ref',
          project: 'swarm-universe',
          character_key: c.key || '',
          character_label: c.label,
          character_tag: c.tag,
          broken_ref: '',
        } as CheckFixer,
      }
    }

    const probe = await headOrGet(url)
    if (probe.ok) {
      return {
        id: `ref-${tag}`,
        level: 1 as const,
        status: 'ok' as const,
        label: `${tag} ${c.label} ref 可访问 (${probe.sizeKB ? `${probe.sizeKB} KB` : source.toUpperCase()})`,
        detail: url,
        data: { source, url, size: probe.size, content_type: probe.contentType },
      }
    }
    // 不可访问 → 根据 source 提供不同 fixer
    if (source === 'tos') {
      return {
        id: `ref-${tag}`,
        level: 1 as const,
        status: 'error' as const,
        label: `${tag} ${c.label} TOS URL 不可访问（${probe.status || 'network'}）`,
        detail: url,
        fixer: {
          kind: 'stale_tos',
          character_key: c.key,
          character_label: c.label,
          tos_url: url,
        } as CheckFixer,
      }
    }
    return {
      id: `ref-${tag}`,
      level: 1 as const,
      status: 'error' as const,
      label: `${tag} ${c.label} 本地 ref 404（${probe.status || 'network'}）`,
      detail: url,
      fixer: {
        kind: 'broken_ref',
        project: 'swarm-universe',
        character_key: c.key || '',
        character_label: c.label,
        character_tag: c.tag,
        broken_ref: url,
      } as CheckFixer,
    }
  })

  const results = (await Promise.all(tasks)).filter(Boolean) as PreflightCheck[]
  return results
}

interface ProbeResult { ok: boolean; status?: number; size?: number; sizeKB?: number; contentType?: string }

async function headOrGet(url: string, timeoutMs = 5000): Promise<ProbeResult> {
  const ctrl = new AbortController()
  const timer = setTimeout(() => ctrl.abort(), timeoutMs)
  try {
    // HEAD 优先；部分 CDN 对 HEAD 返 405，fallback 到 GET+Range: bytes=0-0
    const h = await fetch(url, { method: 'HEAD', signal: ctrl.signal, credentials: 'same-origin' })
    if (h.ok) {
      const size = Number(h.headers.get('content-length') || 0)
      return {
        ok: true,
        status: h.status,
        size,
        sizeKB: size ? Math.round(size / 1024) : undefined,
        contentType: h.headers.get('content-type') || undefined,
      }
    }
    if (h.status === 405 || h.status === 501) {
      const g = await fetch(url, {
        method: 'GET',
        signal: ctrl.signal,
        headers: { Range: 'bytes=0-0' },
        credentials: 'same-origin',
      })
      return {
        ok: g.ok || g.status === 206,
        status: g.status,
        contentType: g.headers.get('content-type') || undefined,
      }
    }
    return { ok: false, status: h.status }
  } catch (_e) {
    return { ok: false, status: 0 }
  } finally {
    clearTimeout(timer)
  }
}

// ── L2.1：TOS URL 新鲜度 < 1 小时 ──

function checkTOSFreshness(tags: string[], chars: CharacterData[]): PreflightCheck[] {
  const byTag = new Map<string, CharacterData>()
  for (const c of chars) if (c.tag) byTag.set(c.tag, c)

  const out: PreflightCheck[] = []
  for (const tag of tags) {
    const c = byTag.get(tag)
    if (!c || !c.tos_url) continue
    const fr = parseTOSFreshness(c.tos_url)
    if (!fr.parsed) continue
    const mins = Math.round(fr.remainingSec / 60)
    if (!fr.valid) {
      out.push({
        id: `tos-${tag}`,
        level: 2,
        status: 'warn',
        label: `${tag} ${c.label} TOS URL 已过期`,
        detail: `过期 ${Math.abs(mins)} 分钟。系统会在派单时尝试 resign/launder，但失败则派到本地 ref。`,
        fixer: { kind: 'stale_tos', character_key: c.key, character_label: c.label, tos_url: c.tos_url } as CheckFixer,
      })
    } else if (mins < 60) {
      out.push({
        id: `tos-${tag}`,
        level: 2,
        status: 'warn',
        label: `${tag} ${c.label} TOS URL 剩 ${mins} 分钟`,
        detail: '建议派单前提前刷新，避免生成中途过期。',
        fixer: { kind: 'stale_tos', character_key: c.key, character_label: c.label, tos_url: c.tos_url } as CheckFixer,
      })
    }
  }
  return out
}

function isTOSStale(url: string): boolean {
  const fr = parseTOSFreshness(url)
  if (!fr.parsed) return false
  return !fr.valid
}

// ── L2.2：scene.duration 在 [3, 12] 范围 ──

function checkSceneDurations(ep: EpisodeData): PreflightCheck[] {
  const out: PreflightCheck[] = []
  for (const s of ep.scenes || []) {
    const d = s.duration ?? 0
    if (d < 3) {
      out.push({ id: `dur-${s.id}`, level: 2, status: 'warn',
        label: `${s.id} duration=${d}s 偏短`,
        detail: 'Seedance 2.0 最佳 5-10s，短于 3s 容易被模型忽略。' })
    } else if (d > 12) {
      out.push({ id: `dur-${s.id}`, level: 2, status: 'warn',
        label: `${s.id} duration=${d}s 接近/超过上限`,
        detail: 'Seedance 2.0 单次生成建议 ≤ 12s，超长容易失败或被截断。' })
    }
  }
  return out
}

// ── L2.3：prompt 长度 < 150 或 > 3500 ──

function checkPromptLengths(ep: EpisodeData): PreflightCheck[] {
  const out: PreflightCheck[] = []
  for (const s of ep.scenes || []) {
    const len = (s.prompt || '').length
    if (len < 150) {
      out.push({ id: `plen-${s.id}`, level: 2, status: 'warn',
        label: `${s.id} prompt 仅 ${len} 字`,
        detail: '偏短可能描写不足。派单时会自动从 PROMPTS.md 合并镜别细则（若存在）扩充到 ~1-2K。' })
    } else if (len > 3500) {
      out.push({ id: `plen-${s.id}`, level: 2, status: 'warn',
        label: `${s.id} prompt 达 ${len} 字`,
        detail: '派单时会被截到 4000 字，确认主要描写在前半段。' })
    }
  }
  return out
}

// ── L2.4：S(i>=2) 需要的上一场尾帧存在 ──

function checkUpstreamFrames(ep: EpisodeData): PreflightCheck[] {
  const scenes = ep.scenes || []
  const out: PreflightCheck[] = []
  for (let i = 1; i < scenes.length; i++) {
    const prev = scenes[i - 1]
    const picked = prev.picked_take
    const pickedTake = picked ? (prev.takes || []).find(t => t.take_id === picked) : undefined
    const hasUpstream = !!(pickedTake && pickedTake.video_url)
    if (!hasUpstream) {
      out.push({
        id: `up-${scenes[i].id}`,
        level: 2,
        status: 'warn',
        label: `${scenes[i].id} 无上一场 (${prev.id}) picked_take 尾帧可用`,
        detail: 'Seedance 会回退到纯首帧 i2v 生成；若追求连贯动作链可先把 ' + prev.id + ' 跑成功再起这镜。',
      })
    }
  }
  return out
}

// ── L3：摘要信息 ──

function buildSummary(ep: EpisodeData, chars: CharacterData[], tags: string[], model: string, resolution: string): PreflightCheck[] {
  const s = buildSummaryObject(ep, chars, tags, model, resolution)
  return [
    {
      id: 'info-summary',
      level: 3,
      status: 'ok',
      label: `${s.scenes} 镜 · 总 ${s.duration_s}s · ${s.characters.length} 角色`,
      detail: `${s.model} · ${s.resolution}${s.needs_upstream_frame ? ' · 需要尾帧链' : ''}`,
      data: { summary: s as unknown as Record<string, unknown> },
    },
  ]
}

function buildSummaryObject(ep: EpisodeData, chars: CharacterData[], tags: string[], model: string, resolution: string): PreflightReport['summary'] {
  const byTag = new Map<string, CharacterData>()
  for (const c of chars) if (c.tag) byTag.set(c.tag, c)
  const summaryChars: PreflightReport['summary']['characters'] = tags.map(tag => {
    const c = byTag.get(tag)
    if (!c) return { tag, label: '<未匹配>', source: 'missing' as const }
    const useTOS = !!c.tos_url && !isTOSStale(c.tos_url)
    const url = useTOS ? c.tos_url : c.imageUrl
    return {
      tag,
      label: c.label,
      key: c.key,
      url,
      source: useTOS ? ('tos' as const) : (url ? ('local' as const) : ('missing' as const)),
    }
  })
  return {
    scenes: (ep.scenes || []).length,
    duration_s: (ep.scenes || []).reduce((a, s) => a + (s.duration || 0), 0),
    characters: summaryChars,
    model,
    resolution,
    needs_upstream_frame: (ep.scenes || []).length > 1,
  }
}

// ── 导出辅助：按 level 分组 ──

export function groupChecksByLevel(report: PreflightReport) {
  const byLevel: Record<CheckLevel, PreflightCheck[]> = { 1: [], 2: [], 3: [] }
  for (const c of report.checks) byLevel[c.level].push(c)
  return byLevel
}
