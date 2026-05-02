// ── Shared types for short-drama episode workflow ──

export interface Take {
  take_id: string          // e.g. "t1", "t2"
  status: 'pending' | 'running' | 'succeeded' | 'failed' | 'cancelled'
  video_url?: string
  lastframe_url?: string
  task_id?: string
  thumbnail_url?: string
  note?: string            // reject/pick reason / error message
  created_at?: string
  // —— 日志字段（详细记录 Seedance 调用上下文，供「日志」tab 复盘） ——
  prompt?: string          // 实际发给 Seedance 的完整 prompt
  ref_image_url?: string   // 本 take 使用的角色参考图
  ref_video_url?: string   // 本 take 使用的上一场 picked_take 视频（尾帧链）
  ref_video_id?: string    // 本 take 使用的上一场 VideoRecord.id
  model?: string           // e.g. doubao-seedance-2-0-260128
  duration?: number        // 视频时长（秒）
  finished_at?: string     // 结束时间
  // —— 归档字段（本地持久化，不依赖 24h 过期的 TOS URL） ——
  // 成功后由 /v1/videos/archive 下载 TOS mp4 到
  // docs/<project>/production/<ep>/clips_v2/<scene>_<take>.mp4
  local_path?: string      // "/production/ep05/clips_v2/S1_t1.mp4"
  local_url?: string       // "/v1/projects/swarm-universe/production/ep05/clips_v2/S1_t1.mp4"
  // —— API 请求记录（POST /v1/videos/generate 的请求体，便于日志复盘） ——
  request_body?: Record<string, unknown>
}

export interface SceneSpec {
  id: string               // "S1"
  label: string            // "清晨醒来"
  duration: number         // seconds
  prompt?: string
  takes: Take[]
  picked_take?: string     // take_id reference
  rejected_takes?: Take[]  // 历史废稿（早期版本 / 已弃用）
  deleted_task_ids?: string[] // 手动删除过的 task_id，防止 backfill 再次回灌
  // —— 故事板静帧（GPT Image 2 / Nano Banana 等生成的一张 720×1280 静帧） ——
  // 作为 Seedance i2v 首帧锚定镜头构图、角色动作、光影。
  //   storyboard_url     静帧的可访问 URL（/v1/images/<id>.png 或外链）
  //   storyboard_prompt  生成静帧用的视觉 prompt（独立于 scene.prompt —— 后者是给 Seedance 的运镜/动态描述）
  //   storyboard_model   生成时用的模型（gpt-image-2 / nano-banana-2 / flux-pro 等）
  //   storyboard_status  pending / running / succeeded / failed
  //   storyboard_task_id 生成任务 ID（fal.ai request_id），用于回轮询
  storyboard_url?: string
  storyboard_prompt?: string
  storyboard_model?: string
  storyboard_status?: 'pending' | 'running' | 'succeeded' | 'failed'
  storyboard_task_id?: string
  // 是否把 storyboard_url 作为 Seedance i2v 首帧 ref 注入。默认 false（仅作为人工预览/构图参考）。
  // 即便勾选，运行时还会校验 URL 必须 https:// 公网可达——本地 /v1/images/... Seedance 抓不到。
  storyboard_use_as_ref?: boolean
  // 候选缩略图画廊：一次批量生成 N 张，用户挑一张 promote 到 storyboard_url。
  //   每张 = { url, image_id?, model? }；storyboard_url 代表当前"入选"那张。
  storyboard_candidates?: Array<{ url: string; image_id?: string; model?: string }>
  // —— 场景级额外参考资产（独立于 Take 调用时传入的 ref）——
  // ref_video_url、ref_image_extra 不覆盖角色 ref，是叠加上去的。
  // 比如某场主体是“金链子混混被击飞”，场景本身可以额外指一个 EP03 S2 的动作参考视频。
  // 运行时 mergeRefs(scene, characters_in_scene) 负责去重+上传 TOS。
  ref_video_url?: string
  ref_image_extra?: string
}

export interface EpisodeScript {
  md?: string              // 剧本 Markdown 相对路径（或绝对 /v1/... URL）
  prompts_md?: string      // 提示词总稿 Markdown
}

export interface EpisodeHistoryPreview {
  clip: string             // 早期整集合成废稿视频 URL
  note?: string
}

export interface Composition {
  picked_clips: string[]   // ["S1.t2","S2.t3",...]
  bgm_id?: string
  bgm_url?: string
  final_video_url?: string
  status: 'pending' | 'generating' | 'ready' | 'published'
  updated_at?: string
  // 用户反馈：生成的发布文案刷新就没了。原来 promo 只存在 React local state，
  // 不写回 workflow definition。这里把成功生成的 PromoResponse 落到 comp 里，
  // 由 WorkflowPage 的常规持久化路径写进 workflow JSON，刷新页面自动恢复。
  // 故意用 Record<string, unknown> 避免 episodeTypes 反向依赖 lib/api 的类型，
  // 持有方在使用时再 cast 成 PromoResponse。
  promo?: Record<string, unknown>
}

export interface EpisodeData {
  // discriminator
  category: 'scene'
  label: string            // "EP01 穿越"
  // meta
  season: number           // 1-5 (main series), 0 = spinoff
  episode_number?: number  // within season
  is_spinoff?: boolean
  spinoff_group?: string   // "道裂前传" | "MCU外传" etc.
  duration?: number        // target seconds
  description?: string
  cover_url?: string
  // Seedance 2.0 视频规格（per-episode, 应用到所有 scene）
  //   resolution: "480p" | "720p" | "1080p"     默认 "1080p"
  //   ratio:      "9:16" | "16:9" | "1:1" 等    默认 "9:16"
  // 后端会把这两个字段直接透传给 Seedance，覆盖掉旧的 size 推导逻辑。
  video_resolution?: string
  video_ratio?: string
  // production
  scenes?: SceneSpec[]
  composition?: Composition
  // 扩展：剧本 + 历史整集合成废稿
  script?: EpisodeScript
  history_preview?: EpisodeHistoryPreview
}

// Seedance 2.0 支持的分辨率/宽高比选项 —— 面板下拉 + runEpisodeProduction 默认值共用
export const VIDEO_RESOLUTION_OPTIONS: { value: string; label: string; hint: string }[] = [
  { value: '480p',  label: '480p',  hint: '最便宜、快速预览' },
  { value: '720p',  label: '720p',  hint: '平衡画质/成本' },
  { value: '1080p', label: '1080p（默认）', hint: '短剧推荐，电影级画质' },
]
export const VIDEO_RATIO_OPTIONS: { value: string; label: string; hint: string }[] = [
  { value: '9:16', label: '9:16（默认）', hint: '抖音/TikTok 竖屏' },
  { value: '16:9', label: '16:9',         hint: 'B站/YouTube 横屏' },
  { value: '1:1',  label: '1:1',          hint: '朋友圈/Instagram' },
  { value: '3:4',  label: '3:4',          hint: '小幅竖屏' },
  { value: '4:3',  label: '4:3',          hint: '复古/老电视' },
  { value: '21:9', label: '21:9',         hint: '超宽银幕' },
]
// 短剧默认值 —— 对齐 style_guide.md 的「9:16 竖屏 · 抖音优先」+ 用户偏好 1080p
export const DEFAULT_VIDEO_RESOLUTION = '1080p'
export const DEFAULT_VIDEO_RATIO = '9:16'

export interface CharacterData {
  category: 'character'
  label: string            // "林见月"
  tag?: string             // "[图1]"
  key?: string             // manifest key, e.g. "lin_jianyue" / "sumi" / "zerg" — 自检一键修复要用
  appearance_card?: string // 外观卡文案
  imageUrl?: string        // 参考图/三视图（/v1/projects/... path）
  tos_url?: string         // Seedance TOS URL (24h)
  // 角色级参考视频（Seedance 2.0 v2v）——用于人物一致性。
  // 路径示例：/v1/projects/swarm-universe/production/ep03/clips_v2/ep03_S2_sumi_thugs.mp4
  // 项目里 EP07 三个混混复用 EP03 S2 的片段作为角色参考。
  ref_video?: string
  description?: string
  role?: string            // 女一 | 女二 | 男一 | 配角
}

// ── Season metadata (based on 虫群宇宙 Roadmap) ──

export interface SeasonMeta {
  number: number
  title: string
  subtitle: string
  arc: string
  episode_range: string
  duration_hint: string
  color: string           // tailwind color key
  gradient: string        // for badges
}

export const SEASONS: SeasonMeta[] = [
  { number: 1, title: '第一季', subtitle: '起源', arc: 'Arc 0',
    episode_range: 'EP01-10', duration_hint: '45s/集',
    color: 'cyan', gradient: 'from-cyan-500 to-blue-500' },
  { number: 2, title: '第二季', subtitle: '觉醒', arc: 'Arc 1',
    episode_range: 'EP11-20', duration_hint: '90s-2min/集',
    color: 'violet', gradient: 'from-violet-500 to-purple-500' },
  { number: 3, title: '第三季', subtitle: '联盟', arc: 'Arc 2',
    episode_range: 'EP21-30', duration_hint: '2-3min/集',
    color: 'amber', gradient: 'from-amber-500 to-orange-500' },
  { number: 4, title: '第四季', subtitle: '文明', arc: 'Arc 3',
    episode_range: 'EP31-40', duration_hint: '3-4min/集',
    color: 'rose', gradient: 'from-rose-500 to-pink-500' },
  { number: 5, title: '第五季', subtitle: '新纪元', arc: 'Arc 4',
    episode_range: 'EP41-50', duration_hint: '4-5min/集',
    color: 'emerald', gradient: 'from-emerald-500 to-teal-500' },
]

export const SPINOFF_GROUPS = [
  { key: '道裂前传', title: '《道裂》前传', desc: '源文明 6-8集迷你剧' },
  { key: 'MCU外传', title: 'MCU 外传', desc: '5部外传 + 5部落篇' },
  { key: '联动日历', title: '产品联动日历', desc: '8个里程碑' },
  { key: '自定义', title: '自定义衍生', desc: '其他衍生剧' },
]

// ── Helpers ──

export function makeEmptyEpisode(
  season: number,
  episode_number: number,
  title: string,
  scene_count = 6,
  duration = 48,
  isSpinoff = false,
  spinoffGroup?: string,
): EpisodeData {
  const scenes: SceneSpec[] = []
  const perScene = Math.max(6, Math.round(duration / scene_count))
  for (let i = 0; i < scene_count; i++) {
    scenes.push({
      id: `S${i + 1}`,
      label: `场景 ${i + 1}`,
      duration: perScene,
      takes: [],
    })
  }
  const seasonPrefix = isSpinoff ? 'SP' : 'EP'
  const epNum = String(episode_number).padStart(2, '0')
  return {
    category: 'scene',
    label: `${seasonPrefix}${epNum} ${title}`,
    season,
    episode_number,
    is_spinoff: isSpinoff,
    spinoff_group: spinoffGroup,
    duration,
    description: `${scene_count}镜·${duration}s`,
    scenes,
    composition: { picked_clips: [], status: 'pending' },
  }
}

export function sceneTakesSummary(scene: SceneSpec): { total: number; picked: boolean; running: number } {
  const total = scene.takes.length
  const running = scene.takes.filter(t => t.status === 'running' || t.status === 'pending').length
  return { total, picked: !!scene.picked_take, running }
}
