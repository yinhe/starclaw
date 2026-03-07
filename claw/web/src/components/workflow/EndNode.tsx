import { Handle, Position, type NodeProps } from '@xyflow/react'
import { Square } from 'lucide-react'

export default function EndNode({ data }: NodeProps) {
  return (
    <div className="bg-red-50 border-2 border-red-400 rounded-xl px-5 py-3 shadow-sm min-w-[120px] text-center">
      <Handle type="target" position={Position.Top} className="!bg-red-500 !w-3 !h-3" />
      <div className="flex items-center justify-center gap-2">
        <Square className="w-4 h-4 text-red-600" />
        <span className="text-sm font-medium text-red-700">{(data as { label?: string }).label || '结束'}</span>
      </div>
    </div>
  )
}
