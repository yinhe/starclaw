import { useRef, useState, useEffect } from 'react'
import { Handle, Position, type NodeProps } from '@xyflow/react'
import { CheckCircle2, Clock, PlayCircle, Pause, Film, RefreshCw, X, Volume2, VolumeX, Maximize2 } from 'lucide-react'
import type { Take } from './episodeTypes'
import VideoPreviewModal from './VideoPreviewModal'

interface SceneStepData {
  sceneId: string
  label: string
  duration: number
  hasClip: boolean
  isPicked: boolean
  thumbnail?: string
  videoUrl?: string     // picked take 视频（用于对齐高亮）
  isFinal?: boolean
  takes?: Take[]
  pickedTakeId?: string
  prompt?: string
  // 归档路径信息（用于在 TOS 过期时 fallback 到本地文件）
  // 约定：archive 端点把每个 take 存到 docs/<project>/production/<epKey>/clips_v2/<scene>_<take>.mp4
  archiveProject?: string   // 例如 'swarm-universe'
  archiveEpKey?: string     // 例如 'ep05' / 'sp01'
  // 回调（WorkflowPage 注入）
  onRerun?: (sceneId: string) => void
  onPickTake?: (sceneId: string, takeId: string) => void
  onUpdatePrompt?: (sceneId: string, prompt: string) => void
  onSelectScene?: (sceneId: string) => void
}

// 根据归档约定派生 local URL。Archive 端点统一把 take 写到
//   /app/docs/<project>/production/<epKey>/clips_v2/<scene>_<take>.mp4
// 静态路由 /v1/projects/:project/*filepath 会对应返回。
function deriveArchivedLocalUrl(
  project: string | undefined,
  epKey: string | undefined,
  sceneId: string,
  takeId: string | undefined,
): string {
  if (!project || !epKey || !takeId) return ''
  return `/v1/projects/${project}/production/${epKey}/clips_v2/${sceneId}_${takeId}.mp4`
}

export default function SceneStepNode({ data, selected }: NodeProps) {
  const d = data as unknown as SceneStepData
  const [hovering, setHovering] = useState(false)
  const [showTakes, setShowTakes] = useState(false)
  const [showPrompt, setShowPrompt] = useState(false)
  const popupRef = useRef<HTMLDivElement | null>(null)
  const promptRef = useRef<HTMLDivElement | null>(null)

  // Click-outside for takes popup
  useEffect(() => {
    if (!showTakes) return
    const handler = (e: MouseEvent) => {
      if (popupRef.current && !popupRef.current.contains(e.target as Node)) {
        setShowTakes(false)
      }
    }
    document.addEventListener('mousedown', handler)
    return () => document.removeEventListener('mousedown', handler)
  }, [showTakes])

  // Click-outside for prompt editor
  useEffect(() => {
    if (!showPrompt) return
    const handler = (e: MouseEvent) => {
      if (promptRef.current && !promptRef.current.contains(e.target as Node)) {
        const ta = promptRef.current.querySelector('textarea')
        if (ta) {
          const val = ta.value.trim()
          if (val !== (d.prompt || '')) d.onUpdatePrompt?.(d.sceneId, val)
        }
        setShowPrompt(false)
      }
    }
    document.addEventListener('mousedown', handler)
    return () => document.removeEventListener('mousedown', handler)
  }, [showPrompt, d.prompt, d.sceneId, d.onUpdatePrompt])

  // 画布内联播放：单击 video 区域 = 就地带声播放 / 再点一次暂停；双击 = 打开全屏预览
  const videoRef = useRef<HTMLVideoElement | null>(null)
  const [playing, setPlaying] = useState(false)
  const [muted, setMuted] = useState(true)  // 默认静音防止 autoplay 被浏览器拒；点击时解除

  // 全屏预览 modal
  const [preview, setPreview] = useState<{ src: string; title?: string; subtitle?: string } | null>(null)

  if (d.isFinal) {
    const ready = !!d.videoUrl
    return (
      <div className={`rounded-xl shadow-lg border-2 overflow-hidden transition-all cursor-pointer hover:shadow-emerald-500/40 hover:scale-[1.02] ${
        selected ? 'border-emerald-300 ring-2 ring-emerald-400/50' : ready ? 'border-emerald-400' : 'border-emerald-500'
      }`} style={{ width: 160 }}
        onClick={(e) => { e.stopPropagation(); d.onSelectScene?.('__FINAL__') }}
        title={ready ? '已合成 · 点击查看成片' : '点击打开合成面板·一键合成成片'}>
        <Handle type="target" position={Position.Left} className="!bg-white/60 !w-3 !h-3" />
        <div className="bg-gradient-to-br from-emerald-800 to-cyan-900 p-3 flex flex-col items-center gap-1.5">
          <Film className="w-6 h-6 text-emerald-300" />
          <div className="text-xs font-semibold text-emerald-100">合成成片</div>
          <div className="text-[10px] text-emerald-300/80">{ready ? '✓ 已合成' : 'Final Cut'}</div>
        </div>
      </div>
    )
  }

  const statusColor = d.isPicked ? 'emerald' : d.hasClip ? 'amber' : 'gray'
  const borderCls = selected ? 'border-cyan-300 ring-2 ring-cyan-400/50'
    : d.isPicked ? 'border-emerald-500'
    : d.hasClip ? 'border-amber-500'
    : 'border-gray-600'

  const takes = d.takes || []
  const hasTakes = takes.length > 0

  // 候选 URL 列表 —— 顺序尝试，video onError 时跳到下一个。
  // 关键场景：
  //   · picked take 的归档文件 t-编号 在磁盘上不存在（rerun 后编号变了 / 部分归档）
  //   · TOS video_url 24h 过期
  //   → 把所有 succeeded take 的 local_url + 派生 archive 路径 + TOS 都列出来当退路。
  const pickedTake = takes.find(t => t.take_id === d.pickedTakeId)
  const succeededTakes = takes.filter(t => t.status === 'succeeded')
  const orderedTakes = pickedTake
    ? [pickedTake, ...succeededTakes.filter(t => t.take_id !== pickedTake.take_id)]
    : succeededTakes
  const candidateUrls: string[] = []
  const pushUnique = (u: string | undefined | null) => {
    if (u && !candidateUrls.includes(u)) candidateUrls.push(u)
  }
  if (d.videoUrl) pushUnique(d.videoUrl)
  for (const t of orderedTakes) {
    pushUnique(t.local_url)
    pushUnique(deriveArchivedLocalUrl(d.archiveProject, d.archiveEpKey, d.sceneId, t.take_id))
  }
  for (const t of orderedTakes) {
    pushUnique(t.video_url)
  }
  const [candidateIdx, setCandidateIdx] = useState(0)
  // 当 takes/picked 变化时复位到第一个候选
  useEffect(() => { setCandidateIdx(0) }, [d.pickedTakeId, takes.length])
  const playableUrl = candidateUrls[candidateIdx] || ''

  const togglePlay = () => {
    const v = videoRef.current
    if (!v || !playableUrl) return
    if (v.paused) {
      v.muted = false
      setMuted(false)
      v.play().catch(() => {
        // 浏览器 autoplay policy 拦了（非用户手势? 不太可能，但降级保底）
        v.muted = true
        setMuted(true)
        void v.play().catch(() => { /* 放弃 */ })
      })
      setPlaying(true)
    } else {
      v.pause()
      setPlaying(false)
    }
  }

  const toggleMute = (e: React.MouseEvent) => {
    e.stopPropagation()
    const v = videoRef.current
    if (!v) return
    v.muted = !v.muted
    setMuted(v.muted)
    if (!v.muted && v.paused) void v.play().catch(() => { /* noop */ })
  }

  return (
    <div className="relative"
         onMouseEnter={() => setHovering(true)}
         onMouseLeave={() => setHovering(false)}>
      <div className={`rounded-lg shadow-md border-2 ${borderCls} overflow-hidden bg-gray-900 cursor-pointer transition-all`}
           style={{ width: 170 }}>
        <Handle type="target" position={Position.Left} className="!bg-white/60 !w-3 !h-3" />

        {/* 视频/缩略图区域：单击播放/暂停带声，双击放大预览。nodrag 防止 React Flow 吞事件。 */}
        <div className="relative w-full h-[80px] bg-gray-950 flex items-center justify-center overflow-hidden nodrag"
             onClick={(e) => { e.stopPropagation(); if (playableUrl) togglePlay() }}
             onDoubleClick={(e) => {
               e.stopPropagation()
               if (playableUrl) setPreview({ src: playableUrl, title: d.label, subtitle: `${d.sceneId} · ${d.duration}s` })
             }}
             title={playableUrl ? '单击播放/暂停（带声）· 双击放大预览' : ''}>
          {playableUrl ? (
            <video key={playableUrl} ref={videoRef} src={playableUrl}
                   className="w-full h-full object-cover"
                   preload="metadata"
                   playsInline
                   onPlay={() => setPlaying(true)}
                   onPause={() => setPlaying(false)}
                   onEnded={() => setPlaying(false)}
                   onError={() => {
                     // 当前候选失败（404 / 过期 / 解码错误）→ 尝试下一个候选 URL。
                     if (candidateIdx < candidateUrls.length - 1) setCandidateIdx(i => i + 1)
                   }} />
          ) : d.thumbnail ? (
            <img src={d.thumbnail} alt="" className="w-full h-full object-cover"
                 onError={(e) => { (e.target as HTMLImageElement).style.display = 'none' }} />
          ) : (
            <Film className="w-7 h-7 text-gray-700" />
          )}

          {/* 场景ID */}
          <div className="absolute top-1 left-1 px-1.5 py-0.5 rounded bg-black/70 text-[10px] font-mono font-bold text-cyan-300 pointer-events-none">
            {d.sceneId}
          </div>

          {/* 状态图标：暂停时显示状态，正在播则显示 Pause icon */}
          <div className="absolute top-1 right-1 pointer-events-none">
            {playing ? <Pause className="w-4 h-4 text-white drop-shadow" />
              : d.isPicked ? <CheckCircle2 className="w-4 h-4 text-emerald-400" />
              : d.hasClip ? <PlayCircle className="w-4 h-4 text-amber-400" />
              : <Clock className="w-4 h-4 text-gray-600" />}
          </div>

          {/* 中央播放按钮叠层：未播放且 hover 时提示可以点 */}
          {!playing && hovering && playableUrl && (
            <div className="absolute inset-0 flex items-center justify-center pointer-events-none">
              <PlayCircle className="w-9 h-9 text-white/80 drop-shadow-lg" />
            </div>
          )}

          {/* Hover 动作区：静音开关 + 放大预览 + takes 列表 + 重拍 */}
          {hovering && (
            <div className="absolute inset-x-0 bottom-0 flex items-center justify-between gap-1 px-1 py-1 bg-gradient-to-t from-black/90 to-transparent nodrag nopan nowheel"
                 onMouseDown={(e) => e.stopPropagation()} onPointerDown={(e) => e.stopPropagation()}>
              <div className="flex items-center gap-1">
                {playableUrl && (
                  <>
                    <button onClick={toggleMute}
                            title={muted ? '当前静音，点击开声' : '静音'}
                            className="p-0.5 rounded bg-black/60 hover:bg-black/80 text-white transition">
                      {muted ? <VolumeX className="w-3 h-3" /> : <Volume2 className="w-3 h-3" />}
                    </button>
                    <button onClick={(e) => {
                              e.stopPropagation()
                              setPreview({ src: playableUrl, title: d.label, subtitle: `${d.sceneId} · ${d.duration}s` })
                            }}
                            title="放大预览"
                            className="p-0.5 rounded bg-black/60 hover:bg-black/80 text-white transition">
                      <Maximize2 className="w-3 h-3" />
                    </button>
                  </>
                )}
                {hasTakes && (
                  <button onClick={(e) => { e.stopPropagation(); setShowTakes(v => !v) }}
                          title={`查看 ${takes.length} 个 take`}
                          className="flex items-center gap-0.5 px-1.5 py-0.5 rounded bg-cyan-600/80 hover:bg-cyan-500 text-white text-[10px] font-medium transition">
                    <Film className="w-3 h-3" /> {takes.length}
                  </button>
                )}
              </div>
              {d.onRerun && (
                <button onClick={(e) => { e.stopPropagation(); d.onRerun?.(d.sceneId) }}
                        onMouseDown={(e) => e.stopPropagation()}
                        onPointerDown={(e) => e.stopPropagation()}
                        title={d.hasClip ? '重拍这一镜' : '生成这一镜'}
                        className="flex items-center gap-0.5 px-1.5 py-0.5 rounded bg-violet-600/90 hover:bg-violet-500 text-white text-[10px] font-medium transition">
                  <RefreshCw className="w-3 h-3" /> {d.hasClip ? '重拍' : '生成'}
                </button>
              )}
            </div>
          )}
        </div>

        <div className="p-2 space-y-0.5 nodrag cursor-pointer" onClick={(e) => { e.stopPropagation(); if (!d.isFinal) { setShowPrompt(v => !v); d.onSelectScene?.(d.sceneId) } }}>
          <div className="text-[11px] font-medium text-gray-200 truncate leading-tight">{d.label}</div>
          <div className={`text-[10px] font-mono text-${statusColor}-400`}>
            {d.duration}s · {d.isPicked ? `已选 #${takes.findIndex(t => t.take_id === d.pickedTakeId) + 1}` : d.hasClip ? `待选 (${takes.length})` : '未拍'}
          </div>
        </div>

        <Handle type="source" position={Position.Right} className="!bg-white/60 !w-3 !h-3" />
      </div>

      {/* 点击标签区域切换 prompt 编辑 */}
      {showPrompt && !d.isFinal && (
        <div ref={promptRef} className="mt-1 rounded-lg border border-cyan-700 bg-gray-900/95 p-2 nodrag nowheel" style={{ width: 320 }}>
          <textarea
            ref={(el) => { if (el) el.focus() }}
            className="w-full bg-gray-800 text-[11px] text-gray-200 rounded px-2 py-1.5 resize-none outline-none focus:ring-1 focus:ring-cyan-500 leading-relaxed cursor-text overflow-y-auto"
            style={{ maxHeight: 160 }}
            rows={6}
            defaultValue={d.prompt || ''}
            placeholder="输入场景提示词…"
            onBlur={(e) => {
              const val = e.target.value.trim()
              if (val !== (d.prompt || '')) d.onUpdatePrompt?.(d.sceneId, val)
            }}
            onClick={(e) => e.stopPropagation()}
            onWheel={(e) => e.stopPropagation()}
          />
          <div className="flex justify-end mt-1">
            <button
              className="px-2 py-0.5 rounded bg-cyan-600 hover:bg-cyan-500 text-white text-[10px] font-medium transition"
              onClick={(e) => {
                e.stopPropagation()
                const ta = (e.target as HTMLElement).closest('div')?.querySelector('textarea')
                if (ta) {
                  const val = ta.value.trim()
                  if (val !== (d.prompt || '')) d.onUpdatePrompt?.(d.sceneId, val)
                }
                setShowPrompt(false)
              }}
            >
              保存
            </button>
          </div>
        </div>
      )}

      {/* Takes 缩略图墙（hover 弹出）—— 每格子：单击就地播放带声，双击放大预览，专门的"选"按钮 pick */}
      {showTakes && hasTakes && (
        <div ref={popupRef} className="absolute left-1/2 -translate-x-1/2 bottom-full z-50 nodrag nopan nowheel cursor-default"
             style={{ width: 360, paddingBottom: 8 }}
             onClick={(e) => e.stopPropagation()}
             onMouseDown={(e) => e.stopPropagation()}
             onPointerDown={(e) => e.stopPropagation()}>
          <div className="bg-gray-900 border-2 border-cyan-500 rounded-lg shadow-2xl p-2">
          <div className="flex items-center justify-between mb-1.5 px-1">
            <div className="text-[11px] font-semibold text-cyan-300">{d.sceneId} · {takes.length} takes</div>
            <button onClick={() => setShowTakes(false)} className="text-gray-500 hover:text-white">
              <X className="w-3.5 h-3.5" />
            </button>
          </div>
          <div className="grid grid-cols-3 gap-1.5">
            {takes.map((t, idx) => {
              const isThisPicked = d.pickedTakeId === t.take_id
              // 候选列表：local_url → 派生 archive 路径 → TOS video_url
              const derived = deriveArchivedLocalUrl(d.archiveProject, d.archiveEpKey, d.sceneId, t.take_id)
              const urls = [t.local_url, derived, t.video_url].filter((u): u is string => !!u)
              const seqNum = idx + 1
              const isNewest = idx === takes.length - 1
              return (
                <TakeThumb key={t.take_id}
                           take={t}
                           urls={urls}
                           isPicked={isThisPicked}
                           seqNum={seqNum}
                           isNewest={isNewest}
                           onPick={() => d.onPickTake?.(d.sceneId, t.take_id)}
                           onPreview={() => setPreview({ src: urls[0] || '', title: `${d.sceneId} · #${seqNum}`, subtitle: d.label })} />
              )
            })}
          </div>
          <div className="mt-1.5 text-[9px] text-gray-500 text-center">单击播放（带声）· 双击放大 · 点「选」切换 picked</div>
          </div>
        </div>
      )}

      {/* 全屏预览 modal */}
      {preview && (
        <VideoPreviewModal
          open
          src={preview.src}
          title={preview.title}
          subtitle={preview.subtitle}
          onClose={() => setPreview(null)}
        />
      )}
    </div>
  )
}

/**
 * 单个 take 缩略格子：
 * - 单击 → 就地带声播放/暂停
 * - 双击 → 打开全屏 preview modal（由父组件控制）
 * - 右下角「选」按钮 → pick 这个 take
 */
function TakeThumb({ take, urls, isPicked, seqNum, isNewest, onPick, onPreview }: {
  take: Take
  urls: string[]      // 候选 URL（按优先级排序）；onError 时跳到下一个
  isPicked: boolean
  seqNum: number
  isNewest: boolean
  onPick: () => void
  onPreview: () => void
}) {
  const ref = useRef<HTMLVideoElement | null>(null)
  const [playing, setPlaying] = useState(false)
  const [idx, setIdx] = useState(0)
  const url = urls[idx] || ''

  const toggle = (e: React.MouseEvent) => {
    e.stopPropagation()
    const v = ref.current
    if (!v || !url) return
    if (v.paused) {
      v.muted = false
      v.play().catch(() => {
        v.muted = true
        void v.play().catch(() => { /* noop */ })
      })
      setPlaying(true)
    } else {
      v.pause()
      setPlaying(false)
    }
  }

  // 非 picked 的 succeeded take：单击 = pick（主要操作），双击 = 预览
  // 已 picked 的 take：单击 = 播放/暂停，双击 = 预览
  const canPick = !isPicked && take.status === 'succeeded'
  const handleClick = (e: React.MouseEvent) => {
    e.stopPropagation()
    if (canPick) { onPick(); return }
    toggle(e)
  }

  return (
    <div
      className={`group relative rounded border-2 overflow-hidden aspect-video transition-transform cursor-pointer ${
        isPicked ? 'border-emerald-400' : 'border-gray-700 hover:border-cyan-400'
      }`}
      onClick={handleClick}
      onMouseDown={(e) => e.stopPropagation()}
      onPointerDown={(e) => e.stopPropagation()}
      onDoubleClick={(e) => { e.stopPropagation(); if (url) onPreview() }}
      title={canPick ? '单击选中此 take' : url ? '单击播放（带声）· 双击放大预览' : ''}
    >
      {url ? (
        <video key={url} ref={ref} src={url}
               className="w-full h-full object-cover pointer-events-none"
               preload="metadata"
               playsInline
               onPlay={() => setPlaying(true)}
               onPause={() => setPlaying(false)}
               onEnded={() => setPlaying(false)}
               onError={() => {
                 if (idx < urls.length - 1) setIdx(i => i + 1)
               }} />
      ) : (
        <div className="w-full h-full bg-gray-800 flex items-center justify-center">
          <Film className="w-4 h-4 text-gray-600" />
        </div>
      )}
      <div className="absolute top-0 left-0 right-0 flex items-center justify-between px-1 py-0.5 bg-black/70 text-[9px] font-mono pointer-events-none">
        <span className="text-gray-300 flex items-center gap-1">{seqNum}{isNewest && <span className="px-1 rounded bg-cyan-500 text-white text-[7px] font-bold leading-tight">NEW</span>}</span>
        <span className={
          take.status === 'succeeded' ? 'text-emerald-400'
          : take.status === 'running' ? 'text-amber-400'
          : take.status === 'failed' ? 'text-red-400'
          : 'text-gray-500'
        }>{take.status}</span>
      </div>
      {isPicked ? (
        <>
          {!playing && url && (
            <div className="absolute inset-0 flex items-center justify-center pointer-events-none opacity-0 group-hover:opacity-100 transition">
              <PlayCircle className="w-6 h-6 text-white/80 drop-shadow" />
            </div>
          )}
          <div className="absolute bottom-0.5 right-0.5 bg-emerald-500 rounded-full p-0.5 pointer-events-none">
            <CheckCircle2 className="w-2.5 h-2.5 text-white" />
          </div>
        </>
      ) : canPick ? (
        <div className="absolute inset-0 flex items-center justify-center bg-black/30 opacity-0 group-hover:opacity-100 transition pointer-events-none">
          <span className="px-3 py-1.5 rounded-lg bg-emerald-600/90 text-white text-xs font-bold shadow-lg">选</span>
        </div>
      ) : null}
    </div>
  )
}
