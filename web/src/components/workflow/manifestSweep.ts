// ── manifestSweep · 全集物料对齐扫描 ──
//
// 单集 preflightEpisode 已经把 [图N] 标签匹配、ref/TOS 可达、剧本时长/引用
// 都跑过一遍。这个文件做的事是把它"开 50 倍"：
//
//   1. 基础健康：扫描所有 character/prop 节点，看 ref/TOS 字段完整性；
//      不绑定任何具体集 —— 一个角色没 ref 全集都派不了。
//   2. 跨集汇总：并发跑每集 preflight，汇总各集 L1/L2 错误数 + picked 进度。
//
// 输出给 AssetCoverageModal 直接渲染表格。复用 PreflightReport 类型，
// 不需要新概念。
//
// 性能：50 集×N 个 head probe，靠 preflight 本身的并发 + 浏览器 keep-alive
// 兜得住。第一次跑大概 10-20s，后续靠 manifest 缓存秒回。

import type { Node } from '@xyflow/react'
import type { EpisodeData } from './episodeTypes'
import { preflightEpisode, type PreflightReport } from './preflight'

// ── 基础健康（不绑定 episode） ──

export interface BasicHealthEntry {
  /** "character" | "prop" */
  kind: 'character' | 'prop'
  /** 显示名 */
  label: string
  /** manifest key（lin_jianyue / coin / ...） */
  key?: string
  /** 角色 tag（[图1]）；prop 没 tag */
  tag?: string
  /** 本地 ref URL（/v1/projects/...） */
  imageUrl?: string
  /** 角色级参考视频（Seedance 2.0 v2v，如 EP07 三个混混复用 EP03 S2 片段） */
  ref_video?: string
  /** Volcengine TOS 签名 URL */
  tos_url?: string
  /** TOS 是否新鲜（>0 分钟剩余） */
  tos_fresh: boolean
  /** TOS 剩余分钟（已过期 = 0） */
  tos_minutes_left?: number
  /** 综合状态：ok（有 ref + 有新鲜 TOS） / warn（有 ref 但 TOS 缺/过期） / error（连本地 ref 都没） */
  status: 'ok' | 'warn' | 'error'
  /** 这个资产被哪些集用到（episode_key 列表） */
  used_by: string[]
}

export interface BasicHealth {
  characters: BasicHealthEntry[]
  props: BasicHealthEntry[]
  /** 整体一行摘要：用到的角色总数 / 有 ref / TOS 新鲜 / 有 ref_video */
  summary: {
    chars_total: number
    chars_with_ref: number
    chars_tos_fresh: number
    chars_with_ref_video: number
    props_total: number
    props_with_ref: number
    props_tos_fresh: number
  }
}

// ── 跨集汇总（每集一行） ──

export interface EpisodeSweepRow {
  episode: EpisodeData
  nodeId: string                 // 媒体节点 id（点击行能聚焦/打开 PreflightModal）
  episode_key: string            // ep01 / sp01 / ...
  scenes: number
  duration_s: number
  picked_count: number           // 已选 picked_take 的镜数
  l1_errors: number              // L1 红色致命数
  l2_warns: number               // L2 黄线警告数
  l3_info: number                // L3 信息数
  can_proceed: boolean           // L1 没错 = 可派单
  /** 完整 preflight 报告，供详情面板用 */
  report: PreflightReport
}

export interface SweepResult {
  basic: BasicHealth
  episodes: EpisodeSweepRow[]
  /** 跑完整次扫描的耗时（ms） */
  elapsed_ms: number
  /** 耗时分布（用于性能调试） */
  generated_at: string
}

// ── TOS 签名解析（与 preflight 内部逻辑一致，避免循环依赖单独实现） ──

function parseTOSMinutesLeft(url: string): number | null {
  if (!url || !url.includes('X-Tos-Algorithm=')) return null
  try {
    const u = new URL(url)
    const date = u.searchParams.get('X-Tos-Date')   // YYYYMMDDTHHmmssZ
    const expiresStr = u.searchParams.get('X-Tos-Expires')
    if (!date || !expiresStr) return null
    const expiresSec = parseInt(expiresStr, 10)
    if (!isFinite(expiresSec)) return null
    // YYYYMMDDTHHmmssZ → ISO
    const m = /^(\d{4})(\d{2})(\d{2})T(\d{2})(\d{2})(\d{2})Z$/.exec(date)
    if (!m) return null
    const [, y, mo, d, h, mi, s] = m
    const ts = Date.parse(`${y}-${mo}-${d}T${h}:${mi}:${s}Z`)
    if (!isFinite(ts)) return null
    const expiresAt = ts + expiresSec * 1000
    const minutes = Math.floor((expiresAt - Date.now()) / 60000)
    return minutes > 0 ? minutes : 0
  } catch {
    return null
  }
}

// ── 节点扫描 ──

interface CharNode {
  id: string
  label: string
  tag?: string
  key?: string
  imageUrl?: string
  tos_url?: string
  ref_video?: string
}

interface PropNode {
  id: string
  label: string
  key?: string
  imageUrl?: string
  tos_url?: string
}

function collectCharNodes(nodes: Node[]): CharNode[] {
  const out: CharNode[] = []
  for (const n of nodes) {
    const d = (n.data || {}) as Record<string, unknown>
    if (d.category !== 'character') continue
    out.push({
      id: n.id,
      label: String(d.label || ''),
      tag: d.tag ? String(d.tag) : undefined,
      key: d.key ? String(d.key) : undefined,
      imageUrl: d.imageUrl ? String(d.imageUrl) : undefined,
      tos_url: d.tos_url ? String(d.tos_url) : undefined,
      ref_video: d.ref_video ? String(d.ref_video) : undefined,
    })
  }
  return out
}

function collectPropNodes(nodes: Node[]): PropNode[] {
  const out: PropNode[] = []
  for (const n of nodes) {
    const d = (n.data || {}) as Record<string, unknown>
    if (d.category !== 'prop') continue
    out.push({
      id: n.id,
      label: String(d.label || ''),
      key: d.key ? String(d.key) : undefined,
      imageUrl: d.imageUrl ? String(d.imageUrl) : undefined,
      tos_url: d.tos_url ? String(d.tos_url) : undefined,
    })
  }
  return out
}

function collectEpisodeNodes(nodes: Node[]): Array<{ id: string; episode: EpisodeData }> {
  const out: Array<{ id: string; episode: EpisodeData }> = []
  for (const n of nodes) {
    const d = (n.data || {}) as Record<string, unknown>
    if (d.category !== 'scene') continue
    if (typeof d.episode_number !== 'number') continue
    out.push({ id: n.id, episode: d as unknown as EpisodeData })
  }
  return out
}

// ── 用量统计：每个角色/道具被哪些集用到 ──

function buildUsageMap(episodes: Array<{ id: string; episode: EpisodeData }>): {
  byTag: Map<string, string[]>     // [图N] → [ep01, ep02, ...]
  byPropLabel: Map<string, string[]>
} {
  const byTag = new Map<string, string[]>()
  const byPropLabel = new Map<string, string[]>()
  for (const { episode } of episodes) {
    const epKey = deriveEpisodeKey(episode)
    const allText = (episode.scenes || []).map(s => s.prompt || '').join('\n')
    // tag 引用
    const tags = new Set(allText.match(/\[图\d+\]/g) || [])
    for (const t of tags) {
      const arr = byTag.get(t) || []
      if (!arr.includes(epKey)) arr.push(epKey)
      byTag.set(t, arr)
    }
    // prop 按「label」点名
    const propMatches = allText.match(/「[^」]+」/g) || []
    for (const m of propMatches) {
      const label = m.slice(1, -1)
      const arr = byPropLabel.get(label) || []
      if (!arr.includes(epKey)) arr.push(epKey)
      byPropLabel.set(label, arr)
    }
  }
  return { byTag, byPropLabel }
}

function deriveEpisodeKey(ep: EpisodeData): string {
  if (ep.is_spinoff) return `sp${String(ep.episode_number).padStart(2, '0')}`
  return `ep${String(ep.episode_number).padStart(2, '0')}`
}

// ── 入口 ──

export interface SweepArgs {
  nodes: Node[]
  project?: string
  model?: string
  resolution?: string
  /** 限制并发数，避免一次性 100+ HEAD 把浏览器堵死。默认 8。 */
  concurrency?: number
}

export async function sweepManifest(args: SweepArgs): Promise<SweepResult> {
  const t0 = performance.now()
  const project = args.project ?? 'swarm-universe'
  const concurrency = args.concurrency ?? 8

  const charNodes = collectCharNodes(args.nodes)
  const propNodes = collectPropNodes(args.nodes)
  const epNodes = collectEpisodeNodes(args.nodes)
  const usage = buildUsageMap(epNodes)

  // 基础健康：每个角色/道具
  const characters: BasicHealthEntry[] = charNodes.map(c => {
    const minsLeft = c.tos_url ? parseTOSMinutesLeft(c.tos_url) : null
    const tos_fresh = minsLeft !== null && minsLeft > 0
    let status: 'ok' | 'warn' | 'error' = 'error'
    if (c.imageUrl && tos_fresh) status = 'ok'
    else if (c.imageUrl) status = 'warn'
    return {
      kind: 'character',
      label: c.label,
      key: c.key,
      tag: c.tag,
      imageUrl: c.imageUrl,
      tos_url: c.tos_url,
      ref_video: c.ref_video,
      tos_fresh,
      tos_minutes_left: minsLeft ?? undefined,
      status,
      used_by: c.tag ? (usage.byTag.get(c.tag) || []) : [],
    }
  })

  const props: BasicHealthEntry[] = propNodes.map(p => {
    const minsLeft = p.tos_url ? parseTOSMinutesLeft(p.tos_url) : null
    const tos_fresh = minsLeft !== null && minsLeft > 0
    let status: 'ok' | 'warn' | 'error' = 'error'
    if (p.imageUrl && tos_fresh) status = 'ok'
    else if (p.imageUrl) status = 'warn'
    return {
      kind: 'prop',
      label: p.label,
      key: p.key,
      imageUrl: p.imageUrl,
      tos_url: p.tos_url,
      tos_fresh,
      tos_minutes_left: minsLeft ?? undefined,
      status,
      used_by: usage.byPropLabel.get(p.label) || [],
    }
  })

  const basic: BasicHealth = {
    characters,
    props,
    summary: {
      chars_total: characters.length,
      chars_with_ref: characters.filter(c => !!c.imageUrl).length,
      chars_tos_fresh: characters.filter(c => c.tos_fresh).length,
      chars_with_ref_video: characters.filter(c => !!c.ref_video).length,
      props_total: props.length,
      props_with_ref: props.filter(p => !!p.imageUrl).length,
      props_tos_fresh: props.filter(p => p.tos_fresh).length,
    },
  }

  // 跨集 preflight：限并发跑
  const episodes: EpisodeSweepRow[] = []
  let cursor = 0
  async function worker() {
    while (cursor < epNodes.length) {
      const i = cursor++
      const { id, episode } = epNodes[i]
      try {
        const report = await preflightEpisode({
          episode,
          nodes: args.nodes,
          project,
          model: args.model,
          resolution: args.resolution,
        })
        const l1_errors = report.checks.filter(c => c.level === 1 && c.status === 'error').length
        const l2_warns = report.checks.filter(c => c.level === 2 && c.status === 'warn').length
        const l3_info = report.checks.filter(c => c.level === 3).length
        episodes[i] = {
          episode,
          nodeId: id,
          episode_key: report.episode_key,
          scenes: report.summary.scenes,
          duration_s: report.summary.duration_s,
          picked_count: (episode.scenes || []).filter(s => !!s.picked_take).length,
          l1_errors,
          l2_warns,
          l3_info,
          can_proceed: report.can_proceed,
          report,
        }
      } catch (e) {
        // preflight 抛异常：占个位塞 error，避免 sweep 整个崩
        const msg = e instanceof Error ? e.message : String(e)
        episodes[i] = {
          episode,
          nodeId: id,
          episode_key: deriveEpisodeKey(episode),
          scenes: (episode.scenes || []).length,
          duration_s: episode.duration || 0,
          picked_count: (episode.scenes || []).filter(s => s.picked_take).length,
          l1_errors: 1,
          l2_warns: 0,
          l3_info: 0,
          can_proceed: false,
          report: {
            generated_at: new Date().toISOString(),
            episode_label: episode.label,
            episode_key: deriveEpisodeKey(episode),
            project,
            checks: [{ id: 'sweep-crash', level: 1, status: 'error', label: '自检引擎抛异常', detail: msg }],
            can_proceed: false,
            summary: { scenes: 0, duration_s: 0, characters: [], props: [], tail_frames: [], model: '', resolution: '', needs_upstream_frame: false },
          },
        }
      }
    }
  }

  const workerCount = Math.min(concurrency, Math.max(1, epNodes.length))
  await Promise.all(Array.from({ length: workerCount }, () => worker()))

  // 按 season + episode_number 排序（衍生剧排末尾）
  episodes.sort((a, b) => {
    const sa = a.episode.is_spinoff ? 99 : a.episode.season
    const sb = b.episode.is_spinoff ? 99 : b.episode.season
    if (sa !== sb) return sa - sb
    return (a.episode.episode_number ?? 0) - (b.episode.episode_number ?? 0)
  })

  return {
    basic,
    episodes,
    elapsed_ms: Math.round(performance.now() - t0),
    generated_at: new Date().toISOString(),
  }
}
