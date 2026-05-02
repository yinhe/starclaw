// ── 虫群宇宙 · 完整种子数据 ──
// 真实资产 + 已规划集数（EP01-EP05，EP05 挂剧本+提示词但暂无成片）
// 从 manifest.json 运行时加载：/v1/projects/swarm-universe/assets/manifest.json
// EP06-EP50 + 8 衍生剧仍为占位（未产出实拍），走本文件硬编码。

import type { EpisodeData, CharacterData, SceneSpec, Take } from './episodeTypes'

// ── Manifest schema ──
export interface SwarmManifest {
  url_prefix: string
  characters: Array<{
    key: string; label: string; tag: string; role: string
    description: string; appearance_card: string
    ref: string | null
    tos_url?: string  // Volcengine Ark TOS signed URL (bypasses Seedance privacy filter)
    // 角色级参考视频 (Seedance 2.0 v2v) — 项目里 EP07 三个混混复用 EP03 S2 片段
    ref_video?: string
    extras?: Record<string, string>
  }>
  props: Array<{
    key: string; label: string; description: string
    ref: string | null; ref_clip?: string
    tag?: string      // [图N] tag (从 [图7] 起，与角色 [图1]-[图6] 接续)；用于 prompt 里 [图N] 引用道具
    tos_url?: string  // Volcengine Ark TOS signed URL (bypasses Seedance privacy filter)
  }>
  episodes: Array<{
    id: string; season: number; number: number; title: string; duration: number
    description: string; final: string
    script?: { md?: string; prompts_md?: string }
    history_preview?: { clip: string; note?: string }
    scenes: Array<{
      id: string; label: string; duration: number; clip: string; prompt: string
      rejected_takes?: Array<{ id: string; duration: number; clip: string; note?: string }>
      // —— 故事板静帧（可选，manifest 是单一真相源）——
      storyboard_url?: string       // 静帧可访问 URL（/v1/images/<id>.png 或外链；abs() 自动补前缀）
      storyboard_prompt?: string    // 生成静帧用的 prompt
      storyboard_model?: string     // 生成时用的模型
    }>
  }>
}

const MANIFEST_URL = '/v1/projects/swarm-universe/assets/manifest.json'
let _manifestCache: SwarmManifest | null = null

export async function loadSwarmManifest(force = false): Promise<SwarmManifest> {
  if (_manifestCache && !force) return _manifestCache
  const res = await fetch(MANIFEST_URL, { cache: 'no-cache' })
  if (!res.ok) throw new Error(`manifest fetch failed: ${res.status}`)
  _manifestCache = await res.json() as SwarmManifest
  return _manifestCache
}

function abs(m: SwarmManifest, rel: string | null | undefined): string {
  if (!rel) return ''
  return rel.startsWith('http') || rel.startsWith('/v1/') ? rel : m.url_prefix + rel
}

// ── 从 manifest 派生角色/道具 ──

function charactersFromManifest(m: SwarmManifest): CharacterData[] {
  return m.characters.map(c => ({
    category: 'character',
    label: c.label, tag: c.tag, role: c.role,
    key: c.key,
    appearance_card: c.appearance_card,
    description: c.description,
    imageUrl: abs(m, c.ref),
    tos_url: c.tos_url,
    // ref_video 不走 abs() 拼接 —— manifest 里已经是 /v1/projects/... 绝对路径。
    ref_video: c.ref_video,
  }))
}

function propsFromManifest(m: SwarmManifest): Array<{ category: 'prop'; key: string; label: string; description: string; imageUrl?: string; tos_url?: string; tag?: string }> {
  return m.props.map(p => ({
    category: 'prop' as const,
    key: p.key,
    label: p.label, description: p.description,
    imageUrl: p.ref ? abs(m, p.ref) : undefined,
    tos_url: p.tos_url,
    tag: p.tag,
  }))
}

// ── 辅助：构造带 scene 的 episode ──

function ep(
  season: number, num: number, title: string, duration: number,
  scenes: Array<{
    id: string; label: string; duration: number; prompt?: string; clip?: string
    rejected_takes?: Array<{ id: string; duration: number; clip: string; note?: string }>
    storyboard_url?: string
    storyboard_prompt?: string
    storyboard_model?: string
  }>,
  description: string,
  isSpinoff = false, spinoffGroup?: string,
  finalVideoUrl?: string,
  script?: { md?: string; prompts_md?: string },
  history_preview?: { clip: string; note?: string },
): EpisodeData {
  const fullScenes: SceneSpec[] = scenes.map(s => {
    const takes: Take[] = []
    let picked_take: string | undefined
    if (s.clip) {
      takes.push({
        take_id: 't1',
        status: 'succeeded',
        video_url: s.clip,
        created_at: new Date().toISOString(),
        note: '真实成片',
      })
      picked_take = 't1'
    }
    const rejected_takes: Take[] | undefined = s.rejected_takes && s.rejected_takes.length > 0
      ? s.rejected_takes.map(rt => ({
          take_id: rt.id,
          status: 'failed' as const,
          video_url: rt.clip,
          note: rt.note || '早期废稿',
        }))
      : undefined
    return {
      id: s.id, label: s.label, duration: s.duration, prompt: s.prompt,
      takes, picked_take, rejected_takes,
      storyboard_url: s.storyboard_url,
      storyboard_prompt: s.storyboard_prompt,
      storyboard_model: s.storyboard_model,
      storyboard_status: s.storyboard_url ? 'succeeded' : undefined,
    }
  })
  const picked_clips = fullScenes.filter(s => s.picked_take).map(s => `${s.id}.${s.picked_take}`)
  const hasReal = !!finalVideoUrl
  return {
    category: 'scene',
    label: `${isSpinoff ? 'SP' : 'EP'}${String(num).padStart(2, '0')} ${title}`,
    season, episode_number: num,
    is_spinoff: isSpinoff, spinoff_group: spinoffGroup,
    duration, description,
    cover_url: finalVideoUrl,
    scenes: fullScenes,
    composition: {
      picked_clips,
      final_video_url: finalVideoUrl,
      status: hasReal ? 'ready' : 'pending',
    },
    script,
    history_preview,
  }
}

// 生成占位分镜：N 镜均分时长
function placeholderScenes(n: number, totalDuration: number): Array<{ id: string; label: string; duration: number }> {
  const per = Math.round(totalDuration / n)
  return Array.from({ length: n }, (_, i) => ({
    id: `S${i + 1}`, label: `场景 ${i + 1}`, duration: per,
  }))
}

// ── 第一季：漂泊篇 · EP01-EP10 ──

// 从 manifest 派生 EP01-EP04 的完整 EpisodeData（含真实 take 视频）
function episodesFromManifest(m: SwarmManifest): EpisodeData[] {
  return m.episodes.map(e => ep(
    e.season, e.number, e.title, e.duration,
    e.scenes.map(s => ({
      id: s.id, label: s.label, duration: s.duration,
      prompt: s.prompt, clip: abs(m, s.clip),
      rejected_takes: s.rejected_takes?.map(rt => ({
        id: rt.id, duration: rt.duration, clip: abs(m, rt.clip), note: rt.note,
      })),
      storyboard_url: s.storyboard_url ? abs(m, s.storyboard_url) : undefined,
      storyboard_prompt: s.storyboard_prompt,
      storyboard_model: s.storyboard_model,
    })),
    e.description,
    false, undefined,
    abs(m, e.final),
    e.script ? {
      md: e.script.md ? abs(m, e.script.md) : undefined,
      prompts_md: e.script.prompts_md ? abs(m, e.script.prompts_md) : undefined,
    } : undefined,
    e.history_preview ? {
      clip: abs(m, e.history_preview.clip),
      note: e.history_preview.note,
    } : undefined,
  ))
}

// EP06-EP50 + 衍生剧：无实拍，保持硬编码占位
// EP05 已从 stub 移除 — 现在由 manifest.json 提供（挂 md/prompts_md，待生产）
export const STUB_EPISODES: EpisodeData[] = [
  // EP06-EP10 stubs (大纲写好但未分镜)
  // EP06、EP07 已迁到 manifest.json（带完整分镜+prompts），从此处移除避免 fallback 出占位
  ep(1, 8, '日常', 60, placeholderScenes(6, 60), '6镜·60s·买衣服化妆+林第一次看K线指股票'),
  ep(1, 9, '大裂隙', 60, placeholderScenes(8, 60), '8镜·60s·S1高潮战斗+共鸣链接首次激活'),
  ep(1, 10, '信号', 55, placeholderScenes(6, 55), '6镜·55s·ZERG感知远方信号·S2钩子'),

  // ── S2 觉醒（Arc 1）· 90s-2min · EP11-EP20 ──
  ep(2, 11, '追踪', 110, placeholderScenes(8, 110), '8镜·110s·颜术正面登场'),
  ep(2, 12, '碰撞', 110, placeholderScenes(8, 110), '8镜·110s·灵vs器的第一次交锋'),
  ep(2, 13, '另一个世界', 110, placeholderScenes(8, 110), '8镜·110s·颜术展示Claw矩阵'),
  ep(2, 14, '入场', 110, placeholderScenes(8, 110), '8镜·110s·颜术宠爱林+开账户'),
  ep(2, 15, '甜头', 110, placeholderScenes(8, 110), '8镜·110s·三族首次炒股赚12%'),
  ep(2, 16, '暴风', 110, placeholderScenes(8, 110), '8镜·110s·裂隙再现+三族首次合力'),
  ep(2, 17, '真相', 110, placeholderScenes(8, 110), '8镜·110s·颜术发现裂隙与ZERG关联'),
  ep(2, 18, '信任危机', 110, placeholderScenes(8, 110), '8镜·110s·灵vs器的严重冲突'),
  ep(2, 19, '融合之战', 120, placeholderScenes(10, 120), '10镜·120s·最强裂隙+三族完整联动'),
  ep(2, 20, '新世界的门', 120, placeholderScenes(10, 120), '10镜·120s·和解+文明轮廓浮现'),

  // ── S3 联盟（Arc 2）· 2-3min · EP21-EP30 ──
  ep(3, 21, '第一只Claw', 180, placeholderScenes(10, 180), '10镜·180s·林创建第一只Claw'),
  ep(3, 22, '寻找同族', 180, placeholderScenes(10, 180), '10镜·180s·ZERG感知其他Claw'),
  ep(3, 23, '联络网', 180, placeholderScenes(10, 180), '10镜·180s·苏蜜抖音引流+联络'),
  ep(3, 24, '铁三角', 180, placeholderScenes(10, 180), '10镜·180s·招募Sean/Summer/Noah'),
  ep(3, 25, 'KPI风波', 180, placeholderScenes(10, 180), '10镜·180s·颜术KPI vs 林感召'),
  ep(3, 26, 'Overlord涌现', 180, placeholderScenes(10, 180), '10镜·180s·Claw向林聚拢'),
  ep(3, 27, '谁是王', 180, placeholderScenes(10, 180), '10镜·180s·颜术不服:为什么是她'),
  ep(3, 28, '天道盟', 180, placeholderScenes(10, 180), '10镜·180s·对立部落出现'),
  ep(3, 29, '铜钱之光', 180, placeholderScenes(10, 180), '10镜·180s·林的仙道记忆解锁'),
  ep(3, 30, 'Cerebrate', 180, placeholderScenes(10, 180), '10镜·180s·区域联盟+颜术做Cerebrate'),

  // ── S4 文明（Arc 3）· 3-4min · EP31-EP40 ──
  ep(4, 31, 'Queen的回响', 240, placeholderScenes(12, 240), '12镜·240s·Queen第一次显现'),
  ep(4, 32, '心跳同步', 240, placeholderScenes(12, 240), '12镜·240s·林与Queen韵律同步'),
  ep(4, 33, '器的局限', 240, placeholderScenes(12, 240), '12镜·240s·颜术无法解析Queen信号'),
  ep(4, 34, '网络成形', 240, placeholderScenes(12, 240), '12镜·240s·全域节点觉醒'),
  ep(4, 35, '五蜂分化', 240, placeholderScenes(12, 240), '12镜·240s·SenseClaw/ScoutClaw/dev_team/TestClaw/OpsClaw'),
  ep(4, 36, 'Abathur', 240, placeholderScenes(12, 240), '12镜·240s·文明自修复·颜术做Abathur'),
  ep(4, 37, 'BroodMind', 240, placeholderScenes(12, 240), '12镜·240s·集体记忆沉淀'),
  ep(4, 38, '深海之旅', 240, placeholderScenes(12, 240), '12镜·240s·ZERG带林进入记忆深处'),
  ep(4, 39, '大寂静真相', 240, placeholderScenes(14, 240), '14镜·240s·文明主动重启真相'),
  ep(4, 40, '王与后', 240, placeholderScenes(14, 240), '14镜·240s·前世光之殿堂画面'),

  // ── S5 新纪元（Arc 4）· 4-5min · EP41-EP50 ──
  ep(5, 41, '上岸', 300, placeholderScenes(14, 300), '14镜·300s·物理变种第一次诞生'),
  ep(5, 42, 'Zergling', 300, placeholderScenes(14, 300), '14镜·300s·机器人犬变种部署'),
  ep(5, 43, 'Hydralisk·Mutalisk', 300, placeholderScenes(14, 300), '14镜·300s·地面+空中载体'),
  ep(5, 44, 'Cerebrate失联', 300, placeholderScenes(16, 300), '16镜·300s·网络面临崩溃'),
  ep(5, 45, '虫后觉醒', 300, placeholderScenes(16, 300), '16镜·300s·林下达精准指令挽救网络'),
  ep(5, 46, '她就是Queen', 300, placeholderScenes(16, 300), '16镜·300s·ZERG眼中Queen面容=林见月'),
  ep(5, 47, '时间闭环', 300, placeholderScenes(16, 300), '16镜·300s·林看到自己掰饼画面（Queen视角）'),
  ep(5, 48, '创世螺旋', 300, placeholderScenes(16, 300), '16镜·300s·虫群文明是人类文明的孩子'),
  ep(5, 49, '星辰', 300, placeholderScenes(16, 300), '16镜·300s·林站天台俯瞰城市'),
  ep(5, 50, '大海', 300, placeholderScenes(18, 300), '18镜·300s·全剧终章·新纪元'),

  // ── 衍生剧：《道裂》前传 · 6集迷你剧 ──
  ep(0, 1, '源文明', 90, placeholderScenes(8, 90), '8镜·90s·源文明鼎盛期·一王一后', true, '道裂前传'),
  ep(0, 2, '分歧初现', 90, placeholderScenes(8, 90), '8镜·90s·感应派vs工程派争论', true, '道裂前传'),
  ep(0, 3, '融合派出走', 90, placeholderScenes(8, 90), '8镜·90s·第三条路·虫群雏形', true, '道裂前传'),
  ep(0, 4, '王与后的决裂', 90, placeholderScenes(10, 90), '10镜·90s·最亲密的人撕开裂缝', true, '道裂前传'),
  ep(0, 5, '道裂', 90, placeholderScenes(10, 90), '10镜·90s·源文明一分为三', true, '道裂前传'),
  ep(0, 6, '三族各奔', 90, placeholderScenes(8, 90), '8镜·90s·感应→仙道，工程→人族，融合→虫群', true, '道裂前传'),

  // MCU 外传（预设）
  ep(0, 1, '老周鉴古', 60, placeholderScenes(6, 60), '6镜·60s·古董鉴定师线·铜钱追踪', true, 'MCU外传'),
  ep(0, 2, '柳星河', 60, placeholderScenes(6, 60), '6镜·60s·神秘灵觉者支线', true, 'MCU外传'),
]

// 保持旧 export 名为 backward-compat（EP05-EP50 仍占位，EP01-04 改由 manifest 提供）
export const SEED_EPISODES = STUB_EPISODES

// ── 加载函数：把种子数据转换成 ReactFlow Node 数组 ──

export interface SeedLoadOptions {
  startIdCounter: number
}

export async function buildSeedNodes(opts: SeedLoadOptions) {
  let id = opts.startIdCounter
  const nodes: Array<{ id: string; type: string; position: { x: number; y: number }; data: Record<string, unknown> }> = []

  // 1. 运行时拉取 manifest（单一真相源）
  // force=true：用户点「虫群宇宙」是显式重置画布的动作，必须绕过 _manifestCache
  // 拉磁盘最新内容；否则改了 manifest.json 后即使再点按钮也会吃老缓存。
  const manifest = await loadSwarmManifest(true)
  const characters = charactersFromManifest(manifest)
  const props = propsFromManifest(manifest)
  const realEpisodes = episodesFromManifest(manifest)
  // 合并：manifest 优先（真相源），stub 仅补齐 manifest 里还没有的集数
  // 去重 key = season + episode_number（同季同集号则 manifest 覆盖 stub）
  const realKeys = new Set(realEpisodes.map(e => `${e.season}-${e.episode_number}`))
  const pendingStubs = STUB_EPISODES.filter(e => !realKeys.has(`${e.season}-${e.episode_number}`))
  const allEpisodes: EpisodeData[] = [...realEpisodes, ...pendingStubs]

  // ── 三层画布布局 ──
  // 第一层 (TOP)    角色 y = 40               水平排列
  // 第二层 (MIDDLE) 剧集 y = 280 起，每季一行   水平排列
  // 第三层 (BOTTOM) 道具 y = MIDDLE 之下       水平排列
  const COL_W = 200
  const CHAR_Y = 40
  const EP_Y_START = 300
  const ROW_H = 170

  // TOP: Characters horizontal row
  characters.forEach((c, i) => {
    nodes.push({
      id: `media-${id++}`, type: 'media',
      position: { x: 80 + i * COL_W, y: CHAR_Y },
      data: c as unknown as Record<string, unknown>,
    })
  })

  // MIDDLE: Episodes grouped by season rows
  // S1-S5 主线各一行，衍生剧单独一行
  const seasonCounters: Record<number, number> = { 0: 0, 1: 0, 2: 0, 3: 0, 4: 0, 5: 0 }
  allEpisodes.forEach((e) => {
    const season = e.is_spinoff ? 0 : e.season
    const col = seasonCounters[season]++
    // season 1-5 are rows 0-4; spinoffs row 5
    const rowIndex = e.is_spinoff ? 5 : (e.season - 1)
    const rowY = EP_Y_START + rowIndex * ROW_H
    nodes.push({
      id: `media-${id++}`, type: 'media',
      position: { x: 80 + col * COL_W, y: rowY },
      data: e as unknown as Record<string, unknown>,
    })
  })

  // BOTTOM: Props horizontal row below all episode rows
  const PROPS_Y = EP_Y_START + 6 * ROW_H + 60
  props.forEach((p, i) => {
    nodes.push({
      id: `media-${id++}`, type: 'media',
      position: { x: 80 + i * COL_W, y: PROPS_Y },
      data: p as unknown as Record<string, unknown>,
    })
  })

  return { nodes, nextId: id }
}
