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
      node_id?: string          // media 节点 id，修复后用于 onPatchNode 回写
    }
  | {
      kind: 'stale_tos'
      // entity_kind 决定 /v1/projects/:p/manifest/:kind/:key 里的 :kind 段。
      //   'characters' —— 老字段兼容分支，key 从 character_key 取
      //   'props'      —— 新分支，道具 tos_url 也要求必须在位
      entity_kind?: 'characters' | 'props'
      entity_key?: string
      character_key?: string   // 老字段：entity_kind='characters' 时用（兼容老代码）
      character_label: string  // UI 展示名（角色/道具都靠这个）
      tos_url: string          // 可能为空（完全没 tos_url 时）
      // 该实体 manifest 里的本地 ref URL (/v1/projects/.../sheets/xxx.png)。
      // 优先已验证 200 OK。handleFix 会把它作为 fallbackSource 传给 refreshTOS，
      // 让 promote/launder 走本地原图——老的 TOS URL 可能早就死了，
      // 没这个字段 refreshTOS 只能去 fetch 过期的老 URL 拿 403。
      local_ref_url?: string
      node_id?: string          // 同上，stale_tos 刷新成功后直接 onPatchNode(nodeId, {tos_url: new})
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
    characters: Array<{ tag: string; label: string; key?: string; url?: string; source: 'tos' | 'local' | 'v2v' | 'missing' }>
    // 剧本里按 label 点名提到、manifest 里有 ref 的道具。
    // 和 characters 一样列到派单摘要区，让用户一眼看到这集 Seedance 真正会喂进去的 prop 参考图。
    props: Array<{ key: string; label: string; url?: string; source: 'tos' | 'local' | 'missing' }>
    // 每个片段的尾帧链信息：scene id → 上一场 picked_take 的尾帧 URL（或空）
    tail_frames: Array<{ scene_id: string; prev_scene_id: string; lastframe_url?: string; video_url?: string; status: 'ready' | 'missing' }>
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
  const props = collectPropsFromNodes(nodes)
  const nodeIdByTag = buildNodeIdByTag(nodes)   // tag -> media nodeId。修复 fixer 时走这个映射回写节点。
  const nodeIdByPropKey = buildNodeIdByPropKey(nodes)

  // 先解析这一集实际用到的 [图N] tag 集合（来自 scene.prompt 中的引用）
  const usedTags = extractTagsFromScenes(episode)
  // 剧本里按 label 提到的道具（只考虑有 ref 的道具 —— 没 ref 的道具 Seedance
  // 也没法上参考图，不强求 tos_url）
  const usedProps = extractUsedProps(episode, props)

  // 并发跑所有检查
  const [
    tagMatch,
    refReachability,
    refVideoCheck,
    tosMandatory,
    tosFreshness,
    tosVsLocalDiff,
    durationCheck,
    promptLengthCheck,
    upstreamCheck,
    summary,
  ] = await Promise.all([
    checkAllTagsMatched(usedTags, characters, project),
    checkAllRefsReachable(usedTags, characters, nodeIdByTag),
    checkRefVideosReachable(usedTags, characters, episode),
    Promise.resolve(checkTOSMandatory(usedTags, characters, usedProps, nodeIdByTag, nodeIdByPropKey)),
    Promise.resolve(checkTOSFreshness(usedTags, characters, usedProps, nodeIdByTag, nodeIdByPropKey)),
    checkTOSvsLocalDiff(usedTags, characters),
    Promise.resolve(checkSceneDurations(episode)),
    Promise.resolve(checkPromptLengths(episode)),
    Promise.resolve(checkUpstreamFrames(episode)),
    Promise.resolve(buildSummary(episode, characters, usedTags, usedProps, model, resolution)),
  ])

  const checks: PreflightCheck[] = [
    ...tagMatch,
    ...refReachability,
    ...refVideoCheck,
    ...tosMandatory,
    ...tosFreshness,
    ...tosVsLocalDiff,
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
    summary: buildSummaryObject(episode, characters, usedTags, usedProps, model, resolution),
  }
}

// ── 内部：角色收集 & tag 解析 ──────────────────────

function buildNodeIdByTag(nodes: Node[]): Map<string, string> {
  const m = new Map<string, string>()
  for (const n of nodes) {
    const d = (n.data || {}) as Record<string, unknown>
    if (d.category === 'character' && d.tag) {
      m.set(String(d.tag), n.id)
    }
  }
  return m
}

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
        // 关键：v2v-only 角色（如 EP07 三个混混）的 ref_video 需要从节点 data
        // 透传到 preflight，否则下游 c.ref_video 永远是 undefined，短路分支不会触发。
        ref_video: d.ref_video ? String(d.ref_video) : undefined,
        description: d.description ? String(d.description) : undefined,
        role: d.role ? String(d.role) : undefined,
      })
    }
  }
  return chars
}

// ── 道具（props）──────────────────────────────────────
//
// 道具在 script 里按 label 提及（例：「古铜钱发烫」），没有 [图N] tag。
// 我们只把 ref != null 的道具纳入检查——没 ref 的纯概念道具（手机/台灯）
// Seedance 也上不了参考图，不强求 tos_url。

interface PropRef {
  label: string
  key: string
  imageUrl?: string      // 本地 /v1/projects/.../sheets/sheet.png
  tos_url?: string
}

function collectPropsFromNodes(nodes: Node[]): PropRef[] {
  const out: PropRef[] = []
  for (const n of nodes) {
    const d = (n.data || {}) as Record<string, unknown>
    if (d.category !== 'prop') continue
    const label = String(d.label || '').trim()
    const key = String(d.key || '').trim()
    const imageUrl = d.imageUrl ? String(d.imageUrl) : undefined
    const tos_url = d.tos_url ? String(d.tos_url) : undefined
    if (!label || !key) continue
    // 只收录有 ref（imageUrl）的道具——没 ref 的道具就算 label 在 script 出现
    // 也无法派参考图，跳过
    if (!imageUrl) continue
    out.push({ label, key, imageUrl, tos_url })
  }
  return out
}

function buildNodeIdByPropKey(nodes: Node[]): Map<string, string> {
  const m = new Map<string, string>()
  for (const n of nodes) {
    const d = (n.data || {}) as Record<string, unknown>
    if (d.category === 'prop' && d.key) {
      m.set(String(d.key), n.id)
    }
  }
  return m
}

function extractUsedProps(ep: EpisodeData, props: PropRef[]): PropRef[] {
  // 聚合所有 scene.prompt + scene.label 作为 haystack。label 当作子串匹配——
  // "古铜钱" 出现在 S3/S4 的 prompt 里就算用到，不管上下文。
  const haystack = (ep.scenes || [])
    .map(s => `${s.prompt || ''} ${s.label || ''}`)
    .join(' ')
  const used: PropRef[] = []
  for (const p of props) {
    if (haystack.includes(p.label)) used.push(p)
  }
  return used
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

async function checkAllRefsReachable(tags: string[], chars: CharacterData[], nodeIdByTag: Map<string, string>): Promise<PreflightCheck[]> {
  const byTag = new Map<string, CharacterData>()
  for (const c of chars) if (c.tag) byTag.set(c.tag, c)

  const tasks = tags.map(async tag => {
    const c = byTag.get(tag)
    if (!c) {
      // tag 未匹配由 checkAllTagsMatched 报，这里跳过
      return null
    }
    const nodeId = nodeIdByTag.get(tag)

    // v2v-only 角色：只有 ref_video，没 imageUrl/tos_url。Seedance 2.0 可以纯走
    // v2v（参考视频带人物一致性 + 首帧从参考视频抽），这种角色不需要静帧参考图。
    // EP07 三个混混就是这种模式 — 不报 L1，走可达性检查由 checkRefVideosReachable 负责。
    if (!c.tos_url && !c.imageUrl && c.ref_video) {
      return {
        id: `ref-${tag}`,
        level: 3 as const,
        status: 'ok' as const,
        label: `${tag} ${c.label} v2v 模式（仅 ref_video）`,
        detail: c.ref_video,
        data: { source: 'v2v_only', url: c.ref_video, character_label: c.label },
      }
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
        detail: 'manifest.ref 和 tos_url 都为空，且未提供 ref_video。',
        fixer: {
          kind: 'broken_ref',
          project: 'swarm-universe',
          character_key: c.key || '',
          character_label: c.label,
          character_tag: c.tag,
          broken_ref: '',
          node_id: nodeId,
        } as CheckFixer,
      }
    }

    // 新鲜的 Ark TOS URL 浏览器 HEAD 会被 CORS 挡（volces.com 不允许跨域 HEAD），
    // 导致这里永远报 "network"。Ark 服务端本身对 Seedance 是可达的，
    // 所以我们信任已通过 parseTOSFreshness 的签名：过期时间未到 → 判定为 OK。
    if (useTOS && isFreshArkTOS(url)) {
      return {
        id: `ref-${tag}`,
        level: 1 as const,
        status: 'ok' as const,
        label: `${tag} ${c.label} ref 可访问 (TOS 签名有效)`,
        detail: url,
        data: { source, url, skipped_probe: 'ark-tos-cors' },
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
          local_ref_url: c.imageUrl,
          node_id: nodeId,
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
        node_id: nodeId,
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

// ── L2：参考视频可达性（Seedance 2.0 v2v）──
//
// 角色级 ref_video（manifest.characters[].ref_video）+ 场景级 ref_video_url
// 都是 v2v 一致性参考。缺/404 不阻断派单（L2 黄线），但要让用户看到。
//
// 路径形态：通常是 /v1/projects/swarm-universe/production/.../*.mp4，
// 浏览器 HEAD 探测 Same-Origin 即可，无需 CORS。

async function checkRefVideosReachable(
  tags: string[],
  chars: CharacterData[],
  episode: EpisodeData,
): Promise<PreflightCheck[]> {
  const out: PreflightCheck[] = []
  const byTag = new Map<string, CharacterData>()
  for (const c of chars) if (c.tag) byTag.set(c.tag, c)

  // 1. 角色级 ref_video（仅检查本集实际用到的角色）
  const charTasks = tags.map(async (tag): Promise<PreflightCheck | null> => {
    const c = byTag.get(tag)
    if (!c) return null
    if (!c.ref_video) {
      // 没填 ref_video 不算错——只是没用 v2v 而已，info 级别静默
      return null
    }
    const probe = await headOrGet(c.ref_video)
    const id = `ref-video-${tag}`
    if (probe.ok) {
      const sizeMB = probe.size ? (probe.size / 1024 / 1024).toFixed(1) : '?'
      return {
        id, level: 3, status: 'ok',
        label: `${tag} ${c.label} ref_video 可访问 (${sizeMB} MB)`,
        detail: c.ref_video,
        data: { url: c.ref_video, character_label: c.label, kind: 'character_ref_video' },
      }
    }
    return {
      id, level: 2, status: 'warn',
      label: `${tag} ${c.label} ref_video 不可访问（${probe.status || 'network'}）`,
      detail: c.ref_video,
      data: { url: c.ref_video, character_label: c.label, kind: 'character_ref_video' },
    }
  })

  // 2. 场景级 ref_video_url 覆盖（每集独立）
  const sceneTasks: Array<Promise<PreflightCheck | null>> = []
  for (const s of episode.scenes || []) {
    if (!s.ref_video_url) continue
    sceneTasks.push((async (): Promise<PreflightCheck | null> => {
      const probe = await headOrGet(s.ref_video_url!)
      const id = `ref-video-scene-${s.id}`
      if (probe.ok) {
        const sizeMB = probe.size ? (probe.size / 1024 / 1024).toFixed(1) : '?'
        return {
          id, level: 3, status: 'ok',
          label: `${s.id} 场景 ref_video 可访问 (${sizeMB} MB)`,
          detail: s.ref_video_url,
          data: { url: s.ref_video_url, scene_id: s.id, kind: 'scene_ref_video' },
        }
      }
      return {
        id, level: 2, status: 'warn',
        label: `${s.id} 场景 ref_video 不可访问（${probe.status || 'network'}）`,
        detail: s.ref_video_url,
        data: { url: s.ref_video_url, scene_id: s.id, kind: 'scene_ref_video' },
      }
    })())
  }

  const all = await Promise.all([...charTasks, ...sceneTasks])
  for (const r of all) if (r) out.push(r)
  return out
}

// ── L1.4：TOS URL 必填 + 未过期 ──
//
// 用户要求：剧本里用到的所有 3 个角色 + 所有被点名的道具（有 ref 的）都必须
// 有有效的 TOS URL 才能派单。没有 tos_url、或者 tos_url 已过期 都是 L1 error，
// 阻断派单。本地 ref 只是兜底 fallback，不再默认接受。

function checkTOSMandatory(
  tags: string[],
  chars: CharacterData[],
  usedProps: PropRef[],
  nodeIdByTag: Map<string, string>,
  nodeIdByPropKey: Map<string, string>,
): PreflightCheck[] {
  const out: PreflightCheck[] = []
  const byTag = new Map<string, CharacterData>()
  for (const c of chars) if (c.tag) byTag.set(c.tag, c)

  // 角色 —— 每个剧本里引用到的 [图N] tag
  for (const tag of tags) {
    const c = byTag.get(tag)
    if (!c) continue   // tag 未匹配由 checkAllTagsMatched 报，这里不重复
    const nodeId = nodeIdByTag.get(tag)
    const id = `tos-req-${tag}`
    // v2v-only 角色不要求 TOS——参考视频本身就是人物错参考。
    // 未来如果需要 v2v 走 TOS，另加一个 checkVideoTOSMandatory，现阶段 Seedance 2.0
    // 可以直接吃 same-origin URL。
    if (!c.tos_url && !c.imageUrl && c.ref_video) continue
    if (!c.tos_url) {
      out.push({
        id,
        level: 1,
        status: 'error',
        label: `${tag} ${c.label} 没有 TOS URL（角色必须）`,
        detail: '派单前请先把角色三视图洗成 Volcengine Ark TOS 签名 URL（Seedance 隐私滤镜绕过）。点「一键修复」会从本地 ref 自动 launder 生成。',
        fixer: {
          kind: 'stale_tos',
          entity_kind: 'characters', entity_key: c.key,
          character_key: c.key, character_label: c.label,
          tos_url: '', local_ref_url: c.imageUrl, node_id: nodeId,
        } as CheckFixer,
      })
      continue
    }
    const fr = parseTOSFreshness(c.tos_url)
    if (fr.parsed && !fr.valid) {
      const mins = Math.round(fr.remainingSec / 60)
      out.push({
        id,
        level: 1,
        status: 'error',
        label: `${tag} ${c.label} TOS URL 已过期（${Math.abs(mins)} 分钟前）`,
        detail: '已过期的 TOS URL 会被 Seedance 拒绝。点「一键修复」走 resign/launder 拿新 24h 签名。',
        fixer: {
          kind: 'stale_tos',
          entity_kind: 'characters', entity_key: c.key,
          character_key: c.key, character_label: c.label,
          tos_url: c.tos_url, local_ref_url: c.imageUrl, node_id: nodeId,
        } as CheckFixer,
      })
    }
    // fr.valid === true 则通过，不输出 check 行（保持面板简洁）
  }

  // 道具 —— 只看 script 里按 label 提到、且 manifest.ref 存在的
  for (const p of usedProps) {
    const nodeId = nodeIdByPropKey.get(p.key)
    const id = `tos-req-prop-${p.key}`
    if (!p.tos_url) {
      out.push({
        id,
        level: 1,
        status: 'error',
        label: `道具「${p.label}」没有 TOS URL（剧本引用必须）`,
        detail: '剧本里提到这个道具，但 manifest.props 里没有 tos_url。点「一键修复」从本地 sheet.png launder 生成。',
        fixer: {
          kind: 'stale_tos',
          entity_kind: 'props', entity_key: p.key,
          character_label: p.label,   // UI 当 label 用
          tos_url: '', local_ref_url: p.imageUrl, node_id: nodeId,
        } as CheckFixer,
      })
      continue
    }
    const fr = parseTOSFreshness(p.tos_url)
    if (fr.parsed && !fr.valid) {
      const mins = Math.round(fr.remainingSec / 60)
      out.push({
        id,
        level: 1,
        status: 'error',
        label: `道具「${p.label}」TOS URL 已过期（${Math.abs(mins)} 分钟前）`,
        detail: '已过期的 TOS URL 会被 Seedance 拒绝。点「一键修复」刷新。',
        fixer: {
          kind: 'stale_tos',
          entity_kind: 'props', entity_key: p.key,
          character_label: p.label,
          tos_url: p.tos_url, local_ref_url: p.imageUrl, node_id: nodeId,
        } as CheckFixer,
      })
    }
  }
  return out
}

// ── L2.0：TOS 图 vs 本地原图差异检测 ──
//
// 对比 TOS 签名图和本地原始三视图的文件大小（字节），差异 > 30% 时黄线警告。
// 这能发现 TOS launder 后图被压缩/替换的情况，避免角色外观偏移浪费 token。
// 同时把两个 URL 存到 check.data 里，供 PreflightModal 展示缩略图对比。

async function checkTOSvsLocalDiff(tags: string[], chars: CharacterData[]): Promise<PreflightCheck[]> {
  const byTag = new Map<string, CharacterData>()
  for (const c of chars) if (c.tag) byTag.set(c.tag, c)

  const tasks = tags.map(async (tag): Promise<PreflightCheck | null> => {
    const c = byTag.get(tag)
    if (!c) return null
    // 需要同时有 tos_url 和 imageUrl（本地 ref）才能对比
    if (!c.tos_url || !c.imageUrl) return null
    // 跳过已过期的 TOS（由 L1 检查拦截）
    if (isTOSStale(c.tos_url)) return null

    const [tosProbe, localProbe] = await Promise.all([
      // Ark TOS 有 CORS 限制，HEAD 会失败 → 跳过大小比较，仅输出预览数据
      isFreshArkTOS(c.tos_url) ? Promise.resolve({ ok: true } as ProbeResult) : headOrGet(c.tos_url, 5000),
      headOrGet(c.imageUrl, 5000),
    ])

    // 如果任一探测失败，不做大小比较（由其他检查报错）
    if (!tosProbe.ok || !localProbe.ok) return null

    // Ark TOS CORS 限制 → 无法拿到大小，仅输出预览信息
    if (!tosProbe.size || !localProbe.size) {
      return {
        id: `tos-diff-${tag}`,
        level: 3 as const,
        status: 'ok' as const,
        label: `${tag} ${c.label} TOS vs 本地原图（请目视对比）`,
        detail: '无法自动比较文件大小（CORS 限制），请在下方缩略图目视确认一致性。',
        data: { tos_url: c.tos_url, local_url: c.imageUrl, character_label: c.label, tag },
      }
    }

    const sizeDiffPct = Math.abs(tosProbe.size - localProbe.size) / Math.max(localProbe.size, 1) * 100
    const tosKB = Math.round(tosProbe.size / 1024)
    const localKB = Math.round(localProbe.size / 1024)

    if (sizeDiffPct > 30) {
      return {
        id: `tos-diff-${tag}`,
        level: 2 as const,
        status: 'warn' as const,
        label: `${tag} ${c.label} TOS 图 vs 本地原图大小差异 ${Math.round(sizeDiffPct)}%`,
        detail: `TOS: ${tosKB} KB vs 本地: ${localKB} KB — 差距较大，TOS 图可能被压缩或替换，建议目视对比确认角色外观一致。`,
        data: { tos_url: c.tos_url, local_url: c.imageUrl, tos_size: tosProbe.size, local_size: localProbe.size, diff_pct: sizeDiffPct, character_label: c.label, tag },
      }
    }
    return {
      id: `tos-diff-${tag}`,
      level: 3 as const,
      status: 'ok' as const,
      label: `${tag} ${c.label} TOS 图 ≈ 本地原图（${tosKB} KB / ${localKB} KB）`,
      data: { tos_url: c.tos_url, local_url: c.imageUrl, tos_size: tosProbe.size, local_size: localProbe.size, diff_pct: sizeDiffPct, character_label: c.label, tag },
    }
  })

  return (await Promise.all(tasks)).filter(Boolean) as PreflightCheck[]
}

// ── L2.1：TOS URL 即将过期（< 60 min 剩余）提前预警 ──
//
// 已过期 / 不存在的情况由 L1 checkTOSMandatory 拦截，这里只负责黄线提醒
// 「快过期了可以现在刷一下」，让用户避免派单中途签名到期。

function checkTOSFreshness(
  tags: string[],
  chars: CharacterData[],
  usedProps: PropRef[],
  nodeIdByTag: Map<string, string>,
  nodeIdByPropKey: Map<string, string>,
): PreflightCheck[] {
  const out: PreflightCheck[] = []
  const byTag = new Map<string, CharacterData>()
  for (const c of chars) if (c.tag) byTag.set(c.tag, c)

  for (const tag of tags) {
    const c = byTag.get(tag)
    if (!c || !c.tos_url) continue
    const fr = parseTOSFreshness(c.tos_url)
    if (!fr.parsed || !fr.valid) continue
    const mins = Math.round(fr.remainingSec / 60)
    if (mins < 60) {
      const nodeId = nodeIdByTag.get(tag)
      out.push({
        id: `tos-${tag}`,
        level: 2,
        status: 'warn',
        label: `${tag} ${c.label} TOS URL 剩 ${mins} 分钟`,
        detail: '建议派单前提前刷新，避免生成中途过期。',
        fixer: {
          kind: 'stale_tos',
          entity_kind: 'characters', entity_key: c.key,
          character_key: c.key, character_label: c.label,
          tos_url: c.tos_url, local_ref_url: c.imageUrl, node_id: nodeId,
        } as CheckFixer,
      })
    }
  }

  for (const p of usedProps) {
    if (!p.tos_url) continue
    const fr = parseTOSFreshness(p.tos_url)
    if (!fr.parsed || !fr.valid) continue
    const mins = Math.round(fr.remainingSec / 60)
    if (mins < 60) {
      const nodeId = nodeIdByPropKey.get(p.key)
      out.push({
        id: `tos-prop-${p.key}`,
        level: 2,
        status: 'warn',
        label: `道具「${p.label}」TOS URL 剩 ${mins} 分钟`,
        detail: '建议派单前提前刷新，避免生成中途过期。',
        fixer: {
          kind: 'stale_tos',
          entity_kind: 'props', entity_key: p.key,
          character_label: p.label,
          tos_url: p.tos_url, local_ref_url: p.imageUrl, node_id: nodeId,
        } as CheckFixer,
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

// isFreshArkTOS: 是不是一条 Volcengine Ark TOS 签名 URL，并且签名未过期？
// 用于 L1.3 决定是否跳过 CORS 会失败的浏览器 HEAD 探测。
function isFreshArkTOS(url: string): boolean {
  if (!url.includes('X-Tos-Algorithm=')) return false
  const fr = parseTOSFreshness(url)
  return fr.parsed && fr.valid
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

function buildSummary(ep: EpisodeData, chars: CharacterData[], tags: string[], usedProps: PropRef[], model: string, resolution: string): PreflightCheck[] {
  const s = buildSummaryObject(ep, chars, tags, usedProps, model, resolution)
  const propSuffix = s.props.length > 0 ? ` · ${s.props.length} 道具` : ''
  return [
    {
      id: 'info-summary',
      level: 3,
      status: 'ok',
      label: `${s.scenes} 镜 · 总 ${s.duration_s}s · ${s.characters.length} 角色${propSuffix}`,
      detail: `${s.model} · ${s.resolution}${s.needs_upstream_frame ? ' · 需要尾帧链' : ''}`,
      data: { summary: s as unknown as Record<string, unknown> },
    },
  ]
}

function buildSummaryObject(ep: EpisodeData, chars: CharacterData[], tags: string[], usedProps: PropRef[], model: string, resolution: string): PreflightReport['summary'] {
  const byTag = new Map<string, CharacterData>()
  for (const c of chars) if (c.tag) byTag.set(c.tag, c)
  const summaryChars: PreflightReport['summary']['characters'] = tags.map(tag => {
    const c = byTag.get(tag)
    if (!c) return { tag, label: '<未匹配>', source: 'missing' as const }
    const useTOS = !!c.tos_url && !isTOSStale(c.tos_url)
    const url = useTOS ? c.tos_url : c.imageUrl
    // v2v-only：没有静帧 ref，但有 ref_video。在摘要里显示 V2V，避免误标 MISSING。
    if (!url && c.ref_video) {
      return { tag, label: c.label, key: c.key, url: c.ref_video, source: 'v2v' as const }
    }
    return {
      tag,
      label: c.label,
      key: c.key,
      url,
      source: useTOS ? ('tos' as const) : (url ? ('local' as const) : ('missing' as const)),
    }
  })
  // 道具用的源逻辑和角色一致：有效的 tos_url 走 TOS，否则 fallback 到 imageUrl (本地 sheet)，再没就 missing。
  const summaryProps: PreflightReport['summary']['props'] = usedProps.map(p => {
    const useTOS = !!p.tos_url && !isTOSStale(p.tos_url)
    const url = useTOS ? p.tos_url : p.imageUrl
    return {
      key: p.key,
      label: p.label,
      url,
      source: useTOS ? ('tos' as const) : (url ? ('local' as const) : ('missing' as const)),
    }
  })
  // 尾帧链：S2 需要 S1 的 picked_take 尾帧，S3 需要 S2 的……
  const scenes = ep.scenes || []
  const tailFrames: PreflightReport['summary']['tail_frames'] = []
  for (let i = 1; i < scenes.length; i++) {
    const prev = scenes[i - 1]
    const picked = prev.picked_take
    const pickedTake = picked ? (prev.takes || []).find(t => t.take_id === picked) : undefined
    tailFrames.push({
      scene_id: scenes[i].id,
      prev_scene_id: prev.id,
      lastframe_url: pickedTake?.lastframe_url,
      video_url: pickedTake?.video_url,
      status: pickedTake?.video_url ? 'ready' : 'missing',
    })
  }

  return {
    scenes: scenes.length,
    duration_s: scenes.reduce((a, s) => a + (s.duration || 0), 0),
    characters: summaryChars,
    props: summaryProps,
    tail_frames: tailFrames,
    model,
    resolution,
    needs_upstream_frame: scenes.length > 1,
  }
}

// ── 导出辅助：按 level 分组 ──

export function groupChecksByLevel(report: PreflightReport) {
  const byLevel: Record<CheckLevel, PreflightCheck[]> = { 1: [], 2: [], 3: [] }
  for (const c of report.checks) byLevel[c.level].push(c)
  return byLevel
}
