import { useRef, useState } from 'react'
import { Handle, Position, type NodeProps } from '@xyflow/react'
import { Image, User, Clapperboard, Gem, Shirt, PlayCircle, Pause } from 'lucide-react'

interface MediaNodeData {
  label?: string
  description?: string
  imageUrl?: string
  category?: 'character' | 'scene' | 'prop' | 'costume' | 'reference'
  [key: string]: unknown
}

const categoryConfig: Record<string, { border: string; bg: string; text: string; icon: typeof Image }> = {
  character: { border: 'border-violet-400', bg: 'bg-violet-900/80', text: 'text-violet-200', icon: User },
  scene:     { border: 'border-cyan-400',   bg: 'bg-cyan-900/80',   text: 'text-cyan-200',   icon: Clapperboard },
  prop:      { border: 'border-amber-400',  bg: 'bg-amber-900/80',  text: 'text-amber-200',  icon: Gem },
  costume:   { border: 'border-rose-400',   bg: 'bg-rose-900/80',   text: 'text-rose-200',   icon: Shirt },
  reference: { border: 'border-slate-400',  bg: 'bg-slate-900/80',  text: 'text-slate-200',  icon: Image },
}

export default function MediaNode({ data }: NodeProps) {
  const d = data as MediaNodeData
  const cat = d.category || 'reference'
  const cfg = categoryConfig[cat] || categoryConfig.reference
  const Icon = cfg.icon
  const [imgFailed, setImgFailed] = useState(false)

  const tosUrl = d.tos_url as string | undefined
  const localUrl = d.imageUrl
  const cdnUrl = d.cdn_url as string | undefined
  // Episode cover：优先用 final_video_url / cover_url；都没有就从 picked take 派生本地归档路径。
  // 这样 EP05 这种「场景全选完但还没合成成片」的卡片也能立即显示首镜缩略图。
  const comp = d.composition as { final_video_url?: string } | undefined
  type EpScene = { id: string; picked_take?: string; takes?: Array<{ take_id?: string; local_url?: string; video_url?: string }> }
  const scenesArr = (d.scenes as EpScene[] | undefined) || []
  let derivedEpisodeCover = ''
  let publishedFinal = ''
  if (cat === 'scene' && scenesArr.length) {
    const epNum = (d.episode_number as number | undefined) || 0
    const isSpinoff = !!(d.is_spinoff as boolean | undefined)
    const epKey = epNum > 0 ? `${isSpinoff ? 'sp' : 'ep'}${String(epNum).padStart(2, '0')}` : ''
    // 已发布的 final.mp4（合成成片回到 episodes/<ep>/final.mp4）—— 这是最高优先级。
    if (epKey) publishedFinal = `/v1/projects/swarm-universe/episodes/${epKey}/final.mp4`
    const firstPicked = scenesArr.find(s => s.picked_take && s.takes?.length)
    if (firstPicked) {
      const t = firstPicked.takes?.find(tk => tk.take_id === firstPicked.picked_take)
      if (t) {
        derivedEpisodeCover = t.local_url
          || (epKey ? `/v1/projects/swarm-universe/production/${epKey}/clips_v2/${firstPicked.id}_${t.take_id}.mp4` : '')
          || t.video_url
          || ''
      }
    }
  }
  // 顺序：显式 cover_url → composition.final_video_url → 已发布 final.mp4 → 派生首镜
  const coverUrl = (d.cover_url as string) || comp?.final_video_url || publishedFinal || derivedEpisodeCover
  // 场景（剧集）节点：如果有派生视频封面，优先用视频；否则保持原来的图片优先级。
  // 这样 EP05 这种「全选完未合成」的卡片也能在画布直接看到首镜，而不是默认那张人脸 ref。
  const isSceneEp = cat === 'scene'
  const coverIsVideo = !!coverUrl && coverUrl.endsWith('.mp4')
  const primarySrc = isSceneEp && coverIsVideo
    ? (coverUrl)
    : (cdnUrl || tosUrl || localUrl || coverUrl)
  const fallbackSrc = isSceneEp && coverIsVideo
    ? (cdnUrl || tosUrl || localUrl)
    : (localUrl || coverUrl)
  const isVideo = primarySrc?.endsWith('.mp4') || (imgFailed && fallbackSrc?.endsWith('.mp4'))
  const activeSrc = imgFailed && fallbackSrc ? fallbackSrc : primarySrc

  // 单击播放/暂停（带声）·双击放大（todo: hook to existing modal in future）
  const videoRef = useRef<HTMLVideoElement | null>(null)
  const [playing, setPlaying] = useState(false)
  const [hover, setHover] = useState(false)
  const togglePlay = (e: React.MouseEvent) => {
    e.stopPropagation()
    const v = videoRef.current
    if (!v || !activeSrc || !isVideo) return
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

  return (
    <div className={`rounded-xl shadow-lg border-2 ${cfg.border} overflow-hidden cursor-grab active:cursor-grabbing touch-manipulation`}
         style={{ width: 200 }}>
      <Handle type="target" position={Position.Top} className="!bg-white/60 !w-3.5 !h-3.5 !-top-1.5" />

      {/* Image/Video area */}
      <div className="relative w-full h-[140px] bg-gray-900 flex items-center justify-center overflow-hidden nodrag"
           onMouseEnter={() => setHover(true)}
           onMouseLeave={() => setHover(false)}
           onClick={isVideo && activeSrc ? togglePlay : undefined}
           title={isVideo && activeSrc ? '单击播放/暂停（带声）' : ''}>
        {activeSrc && isVideo ? (
          <>
            <video
              key={activeSrc}
              ref={videoRef}
              src={activeSrc}
              className="w-full h-full object-cover"
              preload="metadata"
              muted
              playsInline
              onPlay={() => setPlaying(true)}
              onPause={() => setPlaying(false)}
              onEnded={() => setPlaying(false)}
              onError={() => {
                if (!imgFailed && fallbackSrc && fallbackSrc !== primarySrc) setImgFailed(true)
              }}
            />
            {/* 播放/暂停叠层（hover 显示 / 播放中显示 Pause） */}
            {(!playing && hover) && (
              <div className="absolute inset-0 flex items-center justify-center pointer-events-none">
                <PlayCircle className="w-12 h-12 text-white/85 drop-shadow-lg" />
              </div>
            )}
            {playing && hover && (
              <div className="absolute top-1.5 right-1.5 pointer-events-none">
                <Pause className="w-5 h-5 text-white drop-shadow" />
              </div>
            )}
          </>
        ) : activeSrc ? (
          <img
            src={activeSrc}
            alt={d.label || ''}
            className="w-full h-full object-cover"
            onError={() => {
              if (!imgFailed && fallbackSrc && fallbackSrc !== primarySrc) {
                setImgFailed(true)
              }
            }}
          />
        ) : (
          <Icon className="w-12 h-12 text-gray-600" />
        )}
      </div>

      {/* Label bar */}
      <div className={`px-3 py-2.5 ${cfg.bg}`}>
        <div className={`text-sm font-semibold ${cfg.text} truncate`}>
          {d.label || '素材'}
        </div>
        {d.description && (
          <div className="text-xs text-gray-400 truncate mt-0.5">
            {d.description}
          </div>
        )}
      </div>

      <Handle type="source" position={Position.Bottom} className="!bg-white/60 !w-3.5 !h-3.5 !-bottom-1.5" />
    </div>
  )
}
