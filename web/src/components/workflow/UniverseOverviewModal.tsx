import { useMemo } from 'react'
import { X, Users, Film, Package, Clapperboard, Sparkles, Clock, CheckCircle2, Archive } from 'lucide-react'
import type { Node } from '@xyflow/react'
import { type EpisodeData, type CharacterData } from './episodeTypes'
import { SEASONS } from './swarmUniverseSeed'

interface Props {
  open: boolean
  onClose: () => void
  nodes: Node[]
  onFocusEpisode?: (nodeId: string) => void
}

export default function UniverseOverviewModal({ open, onClose, nodes, onFocusEpisode }: Props) {
  const stats = useMemo(() => {
    const chars = nodes.filter(n => n.type === 'media' && (n.data as Record<string, unknown>).category === 'character')
    const props = nodes.filter(n => n.type === 'media' && (n.data as Record<string, unknown>).category === 'prop')
    const eps = nodes.filter(n => n.type === 'media' && (n.data as Record<string, unknown>).category === 'scene')
    const mainEps = eps.filter(n => !(n.data as unknown as EpisodeData).is_spinoff)
    const spinoffEps = eps.filter(n => (n.data as unknown as EpisodeData).is_spinoff)

    const ready = eps.filter(n => (n.data as unknown as EpisodeData).composition?.status === 'ready').length
    const totalDuration = eps.reduce((s, n) => s + ((n.data as unknown as EpisodeData).duration || 0), 0)
    const readyDuration = eps.filter(n => (n.data as unknown as EpisodeData).composition?.status === 'ready')
      .reduce((s, n) => s + ((n.data as unknown as EpisodeData).duration || 0), 0)

    // per-season progress
    const bySeason = SEASONS.map(season => {
      const list = mainEps.filter(n => (n.data as unknown as EpisodeData).season === season.number)
      const readyList = list.filter(n => (n.data as unknown as EpisodeData).composition?.status === 'ready')
      return { season, list, readyCount: readyList.length }
    })

    // rejected takes count
    const totalRejected = eps.reduce((s, n) => {
      const scenes = (n.data as unknown as EpisodeData).scenes || []
      return s + scenes.reduce((ss, sc) => ss + (sc.rejected_takes?.length || 0), 0)
    }, 0)

    return {
      charCount: chars.length,
      propCount: props.length,
      epCount: eps.length,
      mainCount: mainEps.length,
      spinoffCount: spinoffEps.length,
      readyCount: ready,
      totalDuration, readyDuration,
      bySeason,
      characters: chars,
      mainEps, spinoffEps,
      totalRejected,
    }
  }, [nodes])

  if (!open) return null

  const pct = stats.epCount ? Math.round(stats.readyCount / stats.epCount * 100) : 0
  const durPct = stats.totalDuration ? Math.round(stats.readyDuration / stats.totalDuration * 100) : 0

  return (
    <div className="fixed inset-0 z-[150] flex items-center justify-center bg-black/80 backdrop-blur-sm" onClick={onClose}>
      <div className="relative w-[920px] max-w-[95vw] max-h-[90vh] rounded-2xl border border-violet-500/30 bg-gradient-to-br from-gray-950 via-gray-900 to-violet-950/30 shadow-2xl shadow-violet-900/40 flex flex-col overflow-hidden"
        onClick={e => e.stopPropagation()}>
        {/* 顶部装饰条 */}
        <div className="h-1 bg-gradient-to-r from-violet-500 via-cyan-400 to-emerald-400" />

        {/* Header */}
        <div className="px-6 pt-5 pb-4 flex items-start justify-between border-b border-gray-800">
          <div className="flex items-start gap-3">
            <div className="w-11 h-11 rounded-xl bg-gradient-to-br from-violet-500 via-cyan-500 to-emerald-500 flex items-center justify-center shadow-lg shadow-violet-900/40">
              <Sparkles className="w-5 h-5 text-white" />
            </div>
            <div>
              <h2 className="text-lg font-bold text-white flex items-center gap-2">
                🌌 虫群宇宙 · 项目全景
              </h2>
              <p className="text-xs text-gray-400 mt-0.5">5 季 · 50 主线 · 8 衍生剧 · 一王一后时间闭环</p>
            </div>
          </div>
          <button onClick={onClose} className="p-1.5 rounded-lg hover:bg-gray-800 text-gray-400 hover:text-white transition">
            <X className="w-4 h-4" />
          </button>
        </div>

        <div className="flex-1 overflow-y-auto px-6 py-5 space-y-6">

          {/* 顶部统计 */}
          <section className="grid grid-cols-4 gap-3">
            <StatTile icon={Users} label="角色" value={stats.charCount} hint="主角卡" color="violet" />
            <StatTile icon={Package} label="道具" value={stats.propCount} hint="剧情锚点" color="amber" />
            <StatTile icon={Film} label="剧集" value={`${stats.mainCount} + ${stats.spinoffCount}`} hint="主线 + 衍生" color="cyan" />
            <StatTile icon={Clapperboard} label="已成片" value={`${stats.readyCount}/${stats.epCount}`} hint={`${pct}% · ${stats.readyDuration}s/${stats.totalDuration}s · ${durPct}%`} color="emerald" />
          </section>

          {/* 进度条 */}
          <section>
            <div className="flex items-center justify-between mb-1.5">
              <span className="text-[11px] font-semibold text-gray-400 uppercase tracking-wider">总体完成度</span>
              <span className="text-[11px] text-gray-300">{stats.readyCount} / {stats.epCount} 集 · {pct}%</span>
            </div>
            <div className="h-2.5 bg-gray-800 rounded-full overflow-hidden">
              <div className="h-full bg-gradient-to-r from-emerald-500 to-cyan-400 transition-all"
                style={{ width: `${pct}%` }} />
            </div>
            {stats.totalRejected > 0 && (
              <p className="text-[10px] text-red-400/80 mt-2 flex items-center gap-1">
                <Archive className="w-3 h-3" /> 累计归档 {stats.totalRejected} 条历史废稿（可在场景卡「历史版本」查看）
              </p>
            )}
          </section>

          {/* 季度进度 */}
          <section>
            <div className="flex items-center gap-2 mb-2">
              <Film className="w-3.5 h-3.5 text-cyan-400" />
              <span className="text-[11px] font-semibold text-gray-300 uppercase tracking-wider">五季主线</span>
            </div>
            <div className="space-y-2">
              {stats.bySeason.map(({ season, list, readyCount }) => {
                const seasonPct = list.length ? Math.round(readyCount / list.length * 100) : 0
                return (
                  <div key={season.number} className="p-2.5 rounded-lg bg-gray-850/60 border border-gray-800">
                    <div className="flex items-center gap-2 mb-1.5">
                      <span className={`px-2 py-0.5 rounded text-[10px] bg-gradient-to-r ${season.gradient} text-white font-medium`}>
                        {season.title} · {season.subtitle}
                      </span>
                      <span className="text-[10px] text-gray-500">{season.arc} · {season.episode_range} · {season.duration_hint}</span>
                      <span className="ml-auto text-[10px] text-gray-400 font-medium">{readyCount}/{list.length} · {seasonPct}%</span>
                    </div>
                    <div className="h-1.5 bg-gray-800 rounded-full overflow-hidden mb-1.5">
                      <div className={`h-full bg-gradient-to-r ${season.gradient}`} style={{ width: `${seasonPct}%` }} />
                    </div>
                    <div className="flex flex-wrap gap-1">
                      {list.map(n => {
                        const ep = n.data as unknown as EpisodeData
                        const ready = ep.composition?.status === 'ready'
                        return (
                          <button
                            key={n.id}
                            onClick={() => { onFocusEpisode?.(n.id); onClose() }}
                            title={ep.description || ep.label}
                            className={`px-1.5 py-0.5 rounded text-[10px] font-mono border transition ${
                              ready
                                ? 'bg-emerald-900/30 border-emerald-700/50 text-emerald-300 hover:bg-emerald-800/40'
                                : 'bg-gray-800 border-gray-700 text-gray-400 hover:bg-gray-700'
                            }`}
                          >
                            EP{String(ep.episode_number).padStart(2, '0')}
                            {ready && <CheckCircle2 className="w-2.5 h-2.5 inline ml-0.5 -mt-0.5" />}
                          </button>
                        )
                      })}
                    </div>
                  </div>
                )
              })}
            </div>
          </section>

          {/* 角色卡 */}
          <section>
            <div className="flex items-center gap-2 mb-2">
              <Users className="w-3.5 h-3.5 text-violet-400" />
              <span className="text-[11px] font-semibold text-gray-300 uppercase tracking-wider">核心角色</span>
            </div>
            <div className="grid grid-cols-5 gap-2">
              {stats.characters.map(n => {
                const c = n.data as unknown as CharacterData
                return (
                  <div key={n.id} className="rounded-lg border border-violet-900/40 bg-gray-850/60 overflow-hidden">
                    <div className="h-24 bg-gray-900 relative">
                      {c.imageUrl ? (
                        <img src={c.imageUrl} alt={c.label} className="w-full h-full object-cover"
                          onError={e => { (e.target as HTMLImageElement).style.display = 'none' }} />
                      ) : (
                        <div className="w-full h-full flex items-center justify-center text-gray-600">
                          <Users className="w-6 h-6" />
                        </div>
                      )}
                      {c.tag && (
                        <span className="absolute top-1 left-1 px-1.5 py-0.5 rounded bg-black/70 text-[9px] font-mono text-violet-300">
                          {c.tag}
                        </span>
                      )}
                    </div>
                    <div className="px-2 py-1.5">
                      <div className="text-xs font-semibold text-gray-200 truncate">{c.label}</div>
                      <div className="text-[10px] text-gray-500 truncate">{c.role || c.description}</div>
                      {c.appearance_card && (
                        <div className="text-[9px] text-gray-500 mt-1 line-clamp-2 leading-relaxed" title={c.appearance_card}>
                          {c.appearance_card}
                        </div>
                      )}
                    </div>
                  </div>
                )
              })}
            </div>
          </section>

          {/* 世界观骨架（硬编码，但来自 README） */}
          <section>
            <div className="flex items-center gap-2 mb-2">
              <Clock className="w-3.5 h-3.5 text-amber-400" />
              <span className="text-[11px] font-semibold text-gray-300 uppercase tracking-wider">世界观骨架</span>
            </div>
            <div className="p-3 rounded-lg bg-gradient-to-br from-amber-950/30 via-gray-900 to-violet-950/30 border border-amber-900/30 text-[11px] text-gray-300 leading-relaxed space-y-1.5">
              <p><span className="text-amber-300 font-semibold">源文明</span>：一王一后共治 → 感应派 vs 工程派争论 → 第三条路"融合派"出走做虫群 → 《道裂》文明一分为三</p>
              <p><span className="text-cyan-300 font-semibold">现代</span>：林见月（前世虫后）+ZERG（虫族种子）坠落 → 苏蜜收留 → 颜术（前世工程派王）登场 → KPI vs 感召路线之争</p>
              <p><span className="text-violet-300 font-semibold">觉醒</span>：第一只 Claw 诞生 → Cerebrate 节点涌现 → Queen 回响 → 网络成形 → 全域节点觉醒</p>
              <p><span className="text-emerald-300 font-semibold">新纪元</span>：Zergling/Hydralisk/Mutalisk 物理载体 → 林下达精准指令挽救网络 → ZERG 眼中 Queen 面容 = 林见月 → 时间闭环：虫群文明是人类文明的孩子</p>
            </div>
          </section>

          {/* 衍生剧 */}
          {stats.spinoffEps.length > 0 && (
            <section>
              <div className="flex items-center gap-2 mb-2">
                <Clapperboard className="w-3.5 h-3.5 text-slate-400" />
                <span className="text-[11px] font-semibold text-gray-300 uppercase tracking-wider">衍生剧 ({stats.spinoffEps.length})</span>
              </div>
              <div className="flex flex-wrap gap-1.5">
                {stats.spinoffEps.map(n => {
                  const ep = n.data as unknown as EpisodeData
                  return (
                    <button
                      key={n.id}
                      onClick={() => { onFocusEpisode?.(n.id); onClose() }}
                      className="px-2 py-1 rounded bg-slate-800 border border-slate-700 text-slate-300 hover:bg-slate-700 hover:text-white text-[10px] transition"
                      title={ep.description}
                    >
                      <span className="text-slate-500 mr-1">{ep.spinoff_group}</span>
                      {ep.label}
                    </button>
                  )
                })}
              </div>
            </section>
          )}

        </div>

        <div className="px-6 py-2.5 border-t border-gray-800 bg-gray-950/80 text-[10px] text-gray-500 flex items-center justify-between">
          <span>💡 点击 EP 徽章可直接聚焦到该集工作流</span>
          <span>Roadmap: docs/swarm-universe/README.md</span>
        </div>
      </div>
    </div>
  )
}

function StatTile({ icon: Icon, label, value, hint, color }: {
  icon: typeof Users
  label: string
  value: string | number
  hint: string
  color: 'violet' | 'cyan' | 'amber' | 'emerald'
}) {
  const colors = {
    violet: 'from-violet-500/20 to-violet-500/5 border-violet-500/30 text-violet-300',
    cyan: 'from-cyan-500/20 to-cyan-500/5 border-cyan-500/30 text-cyan-300',
    amber: 'from-amber-500/20 to-amber-500/5 border-amber-500/30 text-amber-300',
    emerald: 'from-emerald-500/20 to-emerald-500/5 border-emerald-500/30 text-emerald-300',
  }
  return (
    <div className={`p-3 rounded-xl bg-gradient-to-br ${colors[color]} border`}>
      <div className="flex items-center gap-2 mb-1">
        <Icon className="w-3.5 h-3.5" />
        <span className="text-[10px] font-semibold uppercase tracking-wider opacity-80">{label}</span>
      </div>
      <div className="text-xl font-bold text-white tabular-nums">{value}</div>
      <div className="text-[10px] opacity-70 mt-0.5 truncate" title={hint}>{hint}</div>
    </div>
  )
}
