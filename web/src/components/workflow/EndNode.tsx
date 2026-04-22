import { Handle, Position, type NodeProps } from '@xyflow/react'
import { Square } from 'lucide-react'

export default function EndNode({ data }: NodeProps) {
  return (
    <div className="bg-red-50 border-2 border-red-400 rounded-xl px-6 py-4 shadow-sm min-w-[140px] text-center touch-manipulation">
      <Handle type="target" position={Position.Top} className="!bg-red-500 !w-3.5 !h-3.5 !-top-1.5" />
      <div className="flex items-center justify-center gap-2">
        <Square className="w-4 h-4 text-red-600" />
        <span className="text-sm font-medium text-red-700">{(data as { label?: string }).label || '结束'}</span>
      </div>
    </div>
  )
}
