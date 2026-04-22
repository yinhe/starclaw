// ── Shared types for short-drama episode workflow ──

export interface Take {
  take_id: string          // e.g. "t1", "t2"
  status: 'pending' | 'running' | 'succeeded' | 'failed'
  video_url?: string
  lastframe_url?: string
  task_id?: string
  thumbnail_url?: string
  note?: string            // reject/pick reason
  created_at?: string
}

export interface SceneSpec {
  id: string               // "S1"
  label: string            // "清晨醒来"
  duration: number         // seconds
  prompt?: string
  takes: Take[]
  picked_take?: string     // take_id reference
}

export interface Composition {
  picked_clips: string[]   // ["S1.t2","S2.t3",...]
  bgm_id?: string
  bgm_url?: string
  final_video_url?: string
  status: 'pending' | 'generating' | 'ready' | 'published'
  updated_at?: string
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
  // production
  scenes?: SceneSpec[]
  composition?: Composition
}

export interface CharacterData {
  category: 'character'
  label: string            // "林见月"
  tag?: string             // "[图1]"
  appearance_card?: string // 外观卡文案
  imageUrl?: string        // 参考图/三视图
  tos_url?: string         // Seedance TOS URL (24h)
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
