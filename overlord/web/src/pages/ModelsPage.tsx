import { useState, useEffect } from 'react'
import { Loader2, Server, Cpu, AlertCircle } from 'lucide-react'
import { api } from '../api/client'

interface ClawModel {
  id: string
  provider: string
  model_name: string
  display_name: string
  max_tokens: number
  temperature: number
  is_platform: boolean
}

interface NodeModels {
  node_id: string
  node_name: string
  address: string
  models: ClawModel[]
  error?: string
}

export default function ModelsPage() {
  const [nodes, setNodes] = useState<NodeModels[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    api.listModels()
      .then(res => setNodes(res.nodes || []))
      .catch(() => {})
      .finally(() => setLoading(false))
  }, [])

  const totalModels = nodes.reduce((sum, n) => sum + (n.models?.length || 0), 0)

  return (
    <div className="h-full overflow-auto bg-gray-950">
      <div className="max-w-4xl mx-auto px-4 py-6">
        <div className="flex items-center gap-3 mb-6">
          <div className="w-10 h-10 rounded-xl bg-brand-600/20 flex items-center justify-center">
            <Cpu className="w-5 h-5 text-brand-400" />
          </div>
          <div>
            <h1 className="text-lg font-bold text-white">模型管理</h1>
            <p className="text-xs text-gray-500">
              {loading ? '加载中...' : `${nodes.length} 个节点 · ${totalModels} 个模型`}
            </p>
          </div>
        </div>

        {loading ? (
          <div className="flex items-center justify-center py-20 text-gray-500 text-sm">
            <Loader2 className="w-4 h-4 animate-spin mr-2" /> 正在从 Claw 节点获取模型...
          </div>
        ) : nodes.length === 0 ? (
          <div className="text-center py-20 text-gray-500 text-sm">
            没有在线节点
          </div>
        ) : (
          <div className="space-y-4">
            {nodes.map(node => (
              <div key={node.node_id} className="bg-gray-900 rounded-xl border border-gray-800 overflow-hidden">
                <div className="px-4 py-3 border-b border-gray-800 flex items-center gap-3">
                  <Server className="w-4 h-4 text-gray-400" />
                  <div className="flex-1 min-w-0">
                    <span className="text-sm font-medium text-white">{node.node_name}</span>
                    <span className="text-xs text-gray-500 ml-2">{node.address}</span>
                  </div>
                  {node.error ? (
                    <div className="flex items-center gap-1 text-xs text-red-400">
                      <AlertCircle className="w-3.5 h-3.5" />
                      连接失败
                    </div>
                  ) : (
                    <span className="text-xs text-gray-500">{node.models?.length || 0} 个模型</span>
                  )}
                </div>

                {node.error ? (
                  <div className="px-4 py-3 text-xs text-red-400/80">{node.error}</div>
                ) : node.models?.length > 0 ? (
                  <div className="divide-y divide-gray-800/50">
                    {node.models.map(m => (
                      <div key={m.id} className="px-4 py-2.5 flex items-center gap-3">
                        <div className="flex-1 min-w-0">
                          <div className="text-sm text-white font-mono">{m.model_name}</div>
                          <div className="text-[11px] text-gray-500">
                            {m.provider}
                            {m.display_name && m.display_name !== m.model_name && ` · ${m.display_name}`}
                          </div>
                        </div>
                        <div className="text-[11px] text-gray-500 tabular-nums shrink-0">
                          {m.max_tokens.toLocaleString()} tokens
                        </div>
                        {m.is_platform && (
                          <span className="text-[10px] bg-brand-600/20 text-brand-400 px-1.5 py-0.5 rounded">
                            平台
                          </span>
                        )}
                      </div>
                    ))}
                  </div>
                ) : (
                  <div className="px-4 py-3 text-xs text-gray-500">无可用模型</div>
                )}
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  )
}
