import { Handle, Position, type NodeProps } from '@xyflow/react'
import { Image, User, Clapperboard, Gem, Shirt } from 'lucide-react'

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

  return (
    <div className={`rounded-xl shadow-lg border-2 ${cfg.border} overflow-hidden cursor-grab active:cursor-grabbing touch-manipulation`}
         style={{ width: 200 }}>
      <Handle type="target" position={Position.Top} className="!bg-white/60 !w-3.5 !h-3.5 !-top-1.5" />

      {/* Image area */}
      <div className="relative w-full h-[140px] bg-gray-900 flex items-center justify-center overflow-hidden">
        {d.imageUrl ? (
          <img
            src={d.imageUrl}
            alt={d.label || ''}
            className="w-full h-full object-cover"
            onError={(e) => { (e.target as HTMLImageElement).style.display = 'none' }}
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
