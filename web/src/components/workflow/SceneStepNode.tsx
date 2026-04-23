import { Handle, Position, type NodeProps } from '@xyflow/react'
import { CheckCircle2, Clock, PlayCircle, Film } from 'lucide-react'

interface SceneStepData {
  sceneId: string        // e.g. "S1"
  label: string          // e.g. "天空坠落"
  duration: number       // seconds
  hasClip: boolean       // any successful take exists
  isPicked: boolean      // picked_take present
  thumbnail?: string     // first frame (optional)
  videoUrl?: string      // picked clip url
  isFinal?: boolean      // 终点合成节点
}

export default function SceneStepNode({ data, selected }: NodeProps) {
  const d = data as unknown as SceneStepData

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

  return (
    <div className={`rounded-lg shadow-md border-2 ${borderCls} overflow-hidden bg-gray-900 cursor-grab active:cursor-grabbing transition-all`}
         style={{ width: 170 }}>
      <Handle type="target" position={Position.Left} className="!bg-white/60 !w-3 !h-3" />

      {/* Thumbnail / placeholder */}
      <div className="relative w-full h-[80px] bg-gray-950 flex items-center justify-center overflow-hidden">
        {d.thumbnail ? (
          <img src={d.thumbnail} alt="" className="w-full h-full object-cover"
               onError={(e) => { (e.target as HTMLImageElement).style.display = 'none' }} />
        ) : d.videoUrl ? (
          <video src={d.videoUrl} className="w-full h-full object-cover" muted preload="metadata" />
        ) : (
          <Film className="w-7 h-7 text-gray-700" />
        )}
        {/* Scene ID badge */}
        <div className="absolute top-1 left-1 px-1.5 py-0.5 rounded bg-black/70 text-[10px] font-mono font-bold text-cyan-300">
          {d.sceneId}
        </div>
        {/* Status badge */}
        <div className="absolute top-1 right-1">
          {d.isPicked ? (
            <CheckCircle2 className="w-4 h-4 text-emerald-400" />
          ) : d.hasClip ? (
            <PlayCircle className="w-4 h-4 text-amber-400" />
          ) : (
            <Clock className="w-4 h-4 text-gray-600" />
          )}
        </div>
      </div>

      {/* Info */}
      <div className="p-2 space-y-0.5">
        <div className="text-[11px] font-medium text-gray-200 truncate leading-tight">{d.label}</div>
        <div className={`text-[10px] font-mono text-${statusColor}-400`}>
          {d.duration}s · {d.isPicked ? '已选' : d.hasClip ? '待选' : '未拍'}
        </div>
      </div>

      <Handle type="source" position={Position.Right} className="!bg-white/60 !w-3 !h-3" />
    </div>
  )
}
