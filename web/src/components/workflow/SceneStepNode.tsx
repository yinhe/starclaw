import { useState } from 'react'
import { Handle, Position, type NodeProps } from '@xyflow/react'
import { CheckCircle2, Clock, PlayCircle, Film, RefreshCw, X } from 'lucide-react'
import type { Take } from './episodeTypes'

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
  // 回调（WorkflowPage 注入）
  onRerun?: (sceneId: string) => void
  onPickTake?: (sceneId: string, takeId: string) => void
}

export default function SceneStepNode({ data, selected }: NodeProps) {
  const d = data as unknown as SceneStepData
  const [hovering, setHovering] = useState(false)
  const [showTakes, setShowTakes] = useState(false)

  if (d.isFinal) {
    return (
      <div className={`rounded-xl shadow-lg border-2 overflow-hidden transition-all ${
        selected ? 'border-emerald-300 ring-2 ring-emerald-400/50' : 'border-emerald-500'
      }`} style={{ width: 160 }}>
        <Handle type="target" position={Position.Left} className="!bg-white/60 !w-3 !h-3" />
        <div className="bg-gradient-to-br from-emerald-800 to-cyan-900 p-3 flex flex-col items-center gap-1.5">
          <Film className="w-6 h-6 text-emerald-300" />
          <div className="text-xs font-semibold text-emerald-100">合成成片</div>
          <div className="text-[10px] text-emerald-300/80">Final Cut</div>
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

  return (
    <div className="relative"
         onMouseEnter={() => setHovering(true)}
         onMouseLeave={() => { setHovering(false); setShowTakes(false) }}>
      <div className={`rounded-lg shadow-md border-2 ${borderCls} overflow-hidden bg-gray-900 cursor-pointer transition-all`}
           style={{ width: 170 }}>
        <Handle type="target" position={Position.Left} className="!bg-white/60 !w-3 !h-3" />

        <div className="relative w-full h-[80px] bg-gray-950 flex items-center justify-center overflow-hidden">
          {d.thumbnail ? (
            <img src={d.thumbnail} alt="" className="w-full h-full object-cover"
                 onError={(e) => { (e.target as HTMLImageElement).style.display = 'none' }} />
          ) : d.videoUrl ? (
            <video src={d.videoUrl} className="w-full h-full object-cover" muted preload="metadata" />
          ) : (
            <Film className="w-7 h-7 text-gray-700" />
          )}
          <div className="absolute top-1 left-1 px-1.5 py-0.5 rounded bg-black/70 text-[10px] font-mono font-bold text-cyan-300">
            {d.sceneId}
          </div>
          <div className="absolute top-1 right-1">
            {d.isPicked ? <CheckCircle2 className="w-4 h-4 text-emerald-400" />
              : d.hasClip ? <PlayCircle className="w-4 h-4 text-amber-400" />
              : <Clock className="w-4 h-4 text-gray-600" />}
          </div>
          {/* Hover 动作区：takes 数量 + 重拍 */}
          {hovering && (
            <div className="absolute inset-x-0 bottom-0 flex items-center justify-between gap-1 px-1 py-1 bg-gradient-to-t from-black/90 to-transparent nodrag">
              {hasTakes ? (
                <button onClick={(e) => { e.stopPropagation(); setShowTakes(v => !v) }}
                        title={`查看 ${takes.length} 个 take`}
                        className="flex items-center gap-0.5 px-1.5 py-0.5 rounded bg-cyan-600/80 hover:bg-cyan-500 text-white text-[10px] font-medium transition">
                  <Film className="w-3 h-3" /> {takes.length}
                </button>
              ) : <span />}
              {d.onRerun && (
                <button onClick={(e) => { e.stopPropagation(); d.onRerun?.(d.sceneId) }}
                        title={d.hasClip ? '重拍这一镜' : '生成这一镜'}
                        className="flex items-center gap-0.5 px-1.5 py-0.5 rounded bg-violet-600/90 hover:bg-violet-500 text-white text-[10px] font-medium transition">
                  <RefreshCw className="w-3 h-3" /> {d.hasClip ? '重拍' : '生成'}
                </button>
              )}
            </div>
          )}
        </div>

        <div className="p-2 space-y-0.5">
          <div className="text-[11px] font-medium text-gray-200 truncate leading-tight">{d.label}</div>
          <div className={`text-[10px] font-mono text-${statusColor}-400`}>
            {d.duration}s · {d.isPicked ? '已选' : d.hasClip ? `待选 (${takes.length})` : '未拍'}
          </div>
        </div>

        <Handle type="source" position={Position.Right} className="!bg-white/60 !w-3 !h-3" />
      </div>

      {/* Takes 缩略图墙（hover 弹出） */}
      {showTakes && hasTakes && (
        <div className="absolute left-1/2 -translate-x-1/2 bottom-full mb-2 z-50 bg-gray-900 border-2 border-cyan-500 rounded-lg shadow-2xl p-2 nodrag"
             style={{ width: 320 }}
             onClick={(e) => e.stopPropagation()}>
          <div className="flex items-center justify-between mb-1.5 px-1">
            <div className="text-[11px] font-semibold text-cyan-300">{d.sceneId} · {takes.length} takes</div>
            <button onClick={() => setShowTakes(false)} className="text-gray-500 hover:text-white">
              <X className="w-3.5 h-3.5" />
            </button>
          </div>
          <div className="grid grid-cols-3 gap-1.5">
            {takes.map(t => {
              const isThisPicked = d.pickedTakeId === t.take_id
              return (
                <button key={t.take_id}
                        onClick={() => { d.onPickTake?.(d.sceneId, t.take_id); setShowTakes(false) }}
                        className={`relative rounded border-2 overflow-hidden aspect-video hover:scale-105 transition-transform ${
                          isThisPicked ? 'border-emerald-400' : 'border-gray-700 hover:border-cyan-400'
                        }`}>
                  {(t.local_url || t.video_url) ? (
                    <video src={t.local_url || t.video_url} className="w-full h-full object-cover" muted preload="metadata" />
                  ) : (
                    <div className="w-full h-full bg-gray-800 flex items-center justify-center">
                      <Film className="w-4 h-4 text-gray-600" />
                    </div>
                  )}
                  <div className="absolute top-0 left-0 right-0 flex items-center justify-between px-1 py-0.5 bg-black/70 text-[9px] font-mono">
                    <span className="text-gray-300">{t.take_id}</span>
                    <span className={
                      t.status === 'succeeded' ? 'text-emerald-400'
                      : t.status === 'running' ? 'text-amber-400'
                      : t.status === 'failed' ? 'text-red-400'
                      : 'text-gray-500'
                    }>{t.status}</span>
                  </div>
                  {isThisPicked && (
                    <div className="absolute bottom-0.5 right-0.5 bg-emerald-500 rounded-full p-0.5">
                      <CheckCircle2 className="w-2.5 h-2.5 text-white" />
                    </div>
                  )}
                </button>
              )
            })}
          </div>
          <div className="mt-1.5 text-[9px] text-gray-500 text-center">点击缩略图切换 picked take</div>
        </div>
      )}
    </div>
  )
}
