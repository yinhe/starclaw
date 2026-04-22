import { Handle, Position, type NodeProps } from '@xyflow/react'
import { Cpu } from 'lucide-react'

interface LLMNodeData {
  label?: string
  model?: string
  prompt?: string
  [key: string]: unknown
}

export default function LLMNode({ data }: NodeProps) {
  const d = data as LLMNodeData
  return (
    <div className="bg-white border-2 border-primary-400 rounded-xl shadow-sm min-w-[200px] cursor-grab active:cursor-grabbing touch-manipulation">
      <Handle type="target" position={Position.Top} className="!bg-primary-500 !w-3.5 !h-3.5 !-top-1.5" />
      <div className="px-4 py-2.5 border-b border-primary-100 bg-primary-50 rounded-t-xl">
        <div className="flex items-center gap-2">
          <Cpu className="w-4 h-4 text-primary-600" />
          <span className="text-sm font-medium text-primary-700">{d.label || 'LLM'}</span>
        </div>
      </div>
      <div className="px-4 py-3 space-y-2">
        <div className="text-xs text-gray-500">
          <span className="font-medium">模型:</span>{' '}
          <span className="text-gray-700">{d.model || '未配置'}</span>
        </div>
        {d.prompt && (
          <div className="text-xs text-gray-400 truncate max-w-[160px]">
            {d.prompt}
          </div>
        )}
      </div>
      <Handle type="source" position={Position.Bottom} className="!bg-primary-500 !w-3.5 !h-3.5 !-bottom-1.5" />
    </div>
  )
}
