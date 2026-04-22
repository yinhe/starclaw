import { Handle, Position, type NodeProps } from '@xyflow/react'
import { Wrench } from 'lucide-react'

interface ToolNodeData {
  label?: string
  toolName?: string
  [key: string]: unknown
}

export default function ToolNode({ data }: NodeProps) {
  const d = data as ToolNodeData
  return (
    <div className="bg-white border-2 border-amber-400 rounded-xl shadow-sm min-w-[200px] cursor-grab active:cursor-grabbing touch-manipulation">
      <Handle type="target" position={Position.Top} className="!bg-amber-500 !w-3.5 !h-3.5 !-top-1.5" />
      <div className="px-4 py-2.5 border-b border-amber-100 bg-amber-50 rounded-t-xl">
        <div className="flex items-center gap-2">
          <Wrench className="w-4 h-4 text-amber-600" />
          <span className="text-sm font-medium text-amber-700">{d.label || '工具'}</span>
        </div>
      </div>
      <div className="px-4 py-3">
        <div className="text-xs text-gray-500">
          <span className="font-medium">工具:</span>{' '}
          <span className="text-gray-700">{d.toolName || '未配置'}</span>
        </div>
      </div>
      <Handle type="source" position={Position.Bottom} className="!bg-amber-500 !w-3.5 !h-3.5 !-bottom-1.5" />
    </div>
  )
}
