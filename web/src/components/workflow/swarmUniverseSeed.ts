// ── OSS stub: no proprietary project seed data ──
// The private monorepo version contains project-specific episodes and character data.
// This stub provides the same export surface with empty data so the build succeeds.

import type { EpisodeData, SeasonMeta } from './episodeTypes'

export const HAS_SEED_PROJECT = false
export const PROJECT_LABEL = ''

// Generic season definitions (no proprietary subtitles)
export const SEASONS: SeasonMeta[] = [
  { number: 1, title: '第一季', subtitle: '', arc: '',
    episode_range: 'EP01-10', duration_hint: '45s/集',
    color: 'cyan', gradient: 'from-cyan-500 to-blue-500' },
  { number: 2, title: '第二季', subtitle: '', arc: '',
    episode_range: 'EP11-20', duration_hint: '90s-2min/集',
    color: 'violet', gradient: 'from-violet-500 to-purple-500' },
  { number: 3, title: '第三季', subtitle: '', arc: '',
    episode_range: 'EP21-30', duration_hint: '2-3min/集',
    color: 'amber', gradient: 'from-amber-500 to-orange-500' },
  { number: 4, title: '第四季', subtitle: '', arc: '',
    episode_range: 'EP31-40', duration_hint: '3-4min/集',
    color: 'rose', gradient: 'from-rose-500 to-pink-500' },
  { number: 5, title: '第五季', subtitle: '', arc: '',
    episode_range: 'EP41-50', duration_hint: '4-5min/集',
    color: 'emerald', gradient: 'from-emerald-500 to-teal-500' },
]

export const SPINOFF_GROUPS = [
  { key: '自定义', title: '自定义衍生', desc: '衍生剧集' },
]

// ── Manifest schema (kept for type compatibility) ──
export interface SwarmManifest {
  url_prefix: string
  characters: Array<{
    key: string; label: string; tag: string; role: string
    description: string; appearance_card: string
    appearance_cards?: Record<string, string>
    appearance_form?: string
    ref: string | null
    tos_url?: string
    ref_video?: string
    extras?: Record<string, string>
  }>
  props: Array<{
    key: string; label: string; description: string
    ref: string | null; ref_clip?: string
    tag?: string
    tos_url?: string
  }>
  episodes: Array<{
    id: string; season: number; number: number; title: string; duration: number
    description: string; final: string
    script?: { md?: string; prompts_md?: string }
    history_preview?: { clip: string; note?: string }
    scenes: Array<{
      id: string; label: string; duration: number; clip: string; prompt: string
      rejected_takes?: Array<{ id: string; duration: number; clip: string; note?: string }>
      storyboard_url?: string
      storyboard_prompt?: string
      storyboard_model?: string
    }>
  }>
}

export async function loadSwarmManifest(_force = false): Promise<SwarmManifest> {
  throw new Error('No project manifest configured (OSS build)')
}

export const STUB_EPISODES: EpisodeData[] = []
export const SEED_EPISODES = STUB_EPISODES

export interface SeedLoadOptions {
  startIdCounter: number
}

export async function buildSeedNodes(opts: SeedLoadOptions) {
  return { nodes: [] as Array<{ id: string; type: string; position: { x: number; y: number }; data: Record<string, unknown> }>, nextId: opts.startIdCounter }
}
