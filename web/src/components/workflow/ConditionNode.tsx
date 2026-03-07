import { Handle, Position, type NodeProps } from '@xyflow/react'
import { GitBranch } from 'lucide-react'

interface ConditionNodeData {
  label?: string
  expression?: string
  [key: string]: unknown
}

export default function ConditionNode({ data }: NodeProps) {
  const d = data as ConditionNodeData
  return (
    <div className="bg-white border-2 border-purple-400 rounded-xl shadow-sm min-w-[180px]">
      <Handle type="target" position={Position.Top} className="!bg-purple-500 !w-3 !h-3" />
      <div className="px-4 py-2.5 border-b border-purple-100 bg-purple-50 rounded-t-xl">
        <div className="flex items-center gap-2">
          <GitBranch className="w-4 h-4 text-purple-600" />
          <span className="text-sm font-medium text-purple-700">{d.label || '条件分支'}</span>
        </div>
      </div>
      <div className="px-4 py-3">
        <div className="text-xs text-gray-500">
          <span className="font-medium">条件:</span>{' '}
          <span className="text-gray-700">{d.expression || '未配置'}</span>
        </div>
      </div>
      <Handle
        type="source"
        position={Position.Bottom}
        id="true"
        style={{ left: '30%' }}
        className="!bg-green-500 !w-3 !h-3"
      />
      <Handle
        type="source"
        position={Position.Bottom}
        id="false"
        style={{ left: '70%' }}
        className="!bg-red-500 !w-3 !h-3"
      />
    </div>
  )
}
