import { useState, useEffect } from 'react'
import { X, Film, Clapperboard } from 'lucide-react'
import { makeEmptyEpisode, type EpisodeData } from './episodeTypes'
import { SEASONS, SPINOFF_GROUPS } from './swarmUniverseSeed'

interface Props {
  open: boolean
  defaultSeason?: number      // 0 for spinoff
  defaultSpinoffGroup?: string
  existingEpisodes: EpisodeData[]
  onClose: () => void
  onCreate: (data: EpisodeData) => void
}

export default function EpisodeCreatorModal({
  open, defaultSeason, defaultSpinoffGroup, existingEpisodes, onClose, onCreate,
}: Props) {
  const [season, setSeason] = useState(defaultSeason ?? 1)
  const [spinoffGroup, setSpinoffGroup] = useState(defaultSpinoffGroup ?? SPINOFF_GROUPS[0].key)
  const [episodeNum, setEpisodeNum] = useState(1)
  const [title, setTitle] = useState('')
  const [sceneCount, setSceneCount] = useState(6)
  const [duration, setDuration] = useState(48)

  const isSpinoff = season === 0

  // Auto-suggest next episode number when season changes
  useEffect(() => {
    const scope = existingEpisodes.filter(e =>
      isSpinoff
        ? (e.is_spinoff && e.spinoff_group === spinoffGroup)
        : (!e.is_spinoff && e.season === season)
    )
    const nums = scope.map(e => e.episode_number || 0)
    const next = nums.length ? Math.max(...nums) + 1 : 1
    setEpisodeNum(next)
  }, [season, spinoffGroup, existingEpisodes, isSpinoff])

  // Suggest duration per season
  useEffect(() => {
    if (isSpinoff) { setDuration(60); setSceneCount(6); return }
    const hints: Record<number, [number, number]> = {
      1: [48, 6],   // 45s, 6镜
      2: [110, 8],  // 90-120s
      3: [180, 10], // 2-3min
      4: [240, 12], // 3-4min
      5: [300, 14], // 4-5min
    }
    const [d, s] = hints[season] || [48, 6]
    setDuration(d); setSceneCount(s)
  }, [season, isSpinoff])

  if (!open) return null

  const seasonMeta = SEASONS.find(s => s.number === season)

  const submit = () => {
    if (!title.trim()) return
    const data = makeEmptyEpisode(
      isSpinoff ? 0 : season,
      episodeNum,
      title.trim(),
      sceneCount,
      duration,
      isSpinoff,
      isSpinoff ? spinoffGroup : undefined,
    )
    onCreate(data)
    setTitle('')
    onClose()
  }

  return (
    <div className="fixed inset-0 z-[100] flex items-center justify-center bg-black/60 backdrop-blur-sm p-4">
      <div className="w-full max-w-xl bg-gray-900 border border-gray-700 rounded-2xl shadow-2xl overflow-hidden">
        <div className="px-5 py-4 border-b border-gray-800 flex items-center justify-between bg-gradient-to-r from-cyan-900/40 to-gray-900">
          <div className="flex items-center gap-2">
            <Clapperboard className="w-4 h-4 text-cyan-400" />
            <h3 className="text-sm font-semibold text-gray-100">新建剧集</h3>
          </div>
          <button onClick={onClose} className="text-gray-500 hover:text-gray-200 transition-colors">
            <X className="w-4 h-4" />
          </button>
        </div>

        <div className="p-5 space-y-4">
          {/* Season selector */}
          <div>
            <label className="block text-[11px] font-medium text-gray-400 mb-2 uppercase tracking-wider">归属</label>
            <div className="grid grid-cols-3 gap-1.5">
              {SEASONS.map(s => (
                <button key={s.number} type="button" onClick={() => setSeason(s.number)}
                  className={`p-2 rounded-lg border text-left transition ${season === s.number
                    ? `bg-gradient-to-br ${s.gradient} border-transparent text-white shadow-lg`
                    : 'bg-gray-800 border-gray-700 text-gray-400 hover:border-gray-600'}`}>
                  <div className="text-xs font-semibold">{s.title} · {s.subtitle}</div>
                  <div className="text-[10px] opacity-80 mt-0.5">{s.episode_range}</div>
                </button>
              ))}
              <button type="button" onClick={() => setSeason(0)}
                className={`p-2 rounded-lg border text-left transition ${isSpinoff
                  ? 'bg-gradient-to-br from-slate-500 to-gray-500 border-transparent text-white shadow-lg'
                  : 'bg-gray-800 border-gray-700 text-gray-400 hover:border-gray-600'}`}>
                <div className="text-xs font-semibold">衍生剧</div>
                <div className="text-[10px] opacity-80 mt-0.5">前传/外传/联动</div>
              </button>
            </div>
            {seasonMeta && !isSpinoff && (
              <p className="mt-2 text-[10px] text-gray-500">
                {seasonMeta.arc} · 推荐 {seasonMeta.duration_hint}
              </p>
            )}
          </div>

          {isSpinoff && (
            <div>
              <label className="block text-[11px] font-medium text-gray-400 mb-1.5 uppercase tracking-wider">衍生剧分组</label>
              <div className="flex flex-wrap gap-1.5">
                {SPINOFF_GROUPS.map(g => (
                  <button key={g.key} type="button" onClick={() => setSpinoffGroup(g.key)}
                    className={`px-2.5 py-1 text-xs rounded-md border transition ${spinoffGroup === g.key
                      ? 'bg-slate-500/30 text-slate-200 border-slate-500/50'
                      : 'bg-gray-800 text-gray-400 border-gray-700 hover:border-gray-600'}`}>
                    {g.title}
                  </button>
                ))}
              </div>
            </div>
          )}

          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className="block text-[11px] font-medium text-gray-400 mb-1.5 uppercase tracking-wider">集号</label>
              <input type="number" min={1} value={episodeNum}
                onChange={e => setEpisodeNum(parseInt(e.target.value) || 1)}
                className="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-sm text-gray-200 focus:border-cyan-500 focus:outline-none" />
            </div>
            <div>
              <label className="block text-[11px] font-medium text-gray-400 mb-1.5 uppercase tracking-wider">集名</label>
              <input value={title} onChange={e => setTitle(e.target.value)}
                placeholder="例：穿越 / 半块面包"
                className="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-sm text-gray-200 placeholder-gray-600 focus:border-cyan-500 focus:outline-none"
                autoFocus />
            </div>
          </div>

          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className="block text-[11px] font-medium text-gray-400 mb-1.5 uppercase tracking-wider">目标时长 (秒)</label>
              <input type="number" min={15} max={600} value={duration}
                onChange={e => setDuration(parseInt(e.target.value) || 48)}
                className="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-sm text-gray-200 focus:border-cyan-500 focus:outline-none" />
            </div>
            <div>
              <label className="block text-[11px] font-medium text-gray-400 mb-1.5 uppercase tracking-wider">场景数</label>
              <input type="number" min={1} max={30} value={sceneCount}
                onChange={e => setSceneCount(parseInt(e.target.value) || 6)}
                className="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-sm text-gray-200 focus:border-cyan-500 focus:outline-none" />
            </div>
          </div>

          {/* Preview */}
          <div className="p-3 rounded-lg bg-gray-800/50 border border-gray-700/50">
            <div className="text-[10px] text-gray-500 uppercase tracking-wider mb-1">预览</div>
            <div className="flex items-center gap-2">
              <Film className="w-4 h-4 text-cyan-400" />
              <span className="text-sm font-semibold text-gray-200">
                {isSpinoff ? 'SP' : 'EP'}{String(episodeNum).padStart(2, '0')} {title || '(未命名)'}
              </span>
              <span className="text-[10px] text-gray-500">·</span>
              <span className="text-xs text-gray-400">{sceneCount}镜·{duration}s</span>
              {!isSpinoff && seasonMeta && (
                <span className={`ml-auto px-2 py-0.5 rounded text-[10px] bg-gradient-to-r ${seasonMeta.gradient} text-white`}>
                  {seasonMeta.title} {seasonMeta.subtitle}
                </span>
              )}
              {isSpinoff && (
                <span className="ml-auto px-2 py-0.5 rounded text-[10px] bg-slate-500/30 text-slate-200">
                  衍生·{spinoffGroup}
                </span>
              )}
            </div>
          </div>
        </div>

        <div className="px-5 py-3 border-t border-gray-800 bg-gray-950/50 flex items-center justify-end gap-2">
          <button onClick={onClose} className="px-3 py-1.5 text-xs text-gray-400 hover:text-gray-200 transition">取消</button>
          <button onClick={submit} disabled={!title.trim()}
            className="px-4 py-1.5 text-xs font-medium rounded-lg bg-cyan-600 text-white hover:bg-cyan-500 disabled:opacity-40 disabled:cursor-not-allowed transition">
            创建剧集
          </button>
        </div>
      </div>
    </div>
  )
}
