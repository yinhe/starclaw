import { Handle, Position, type NodeProps } from '@xyflow/react'
import { Play } from 'lucide-react'

export default function StartNode({ data }: NodeProps) {
  return (
    <div className="bg-green-50 border-2 border-green-400 rounded-xl px-6 py-4 shadow-sm min-w-[140px] text-center touch-manipulation">
      <div className="flex items-center justify-center gap-2">
        <Play className="w-4 h-4 text-green-600" />
        <span className="text-sm font-medium text-green-700">{(data as { label?: string }).label || '开始'}</span>
      </div>
      <Handle type="source" position={Position.Bottom} className="!bg-green-500 !w-3.5 !h-3.5 !-bottom-1.5" />
    </div>
  )
}
