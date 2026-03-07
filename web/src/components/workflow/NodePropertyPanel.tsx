import { useState, useEffect } from 'react'
import { X } from 'lucide-react'
import type { Node } from '@xyflow/react'

interface Props {
  node: Node | null
  models: { id: string; provider: string; model_name: string; display_name: string }[]
  tools: string[]
  onUpdate: (id: string, data: Record<string, unknown>) => void
  onClose: () => void
}

export default function NodePropertyPanel({ node, models, tools, onUpdate, onClose }: Props) {
  const [localData, setLocalData] = useState<Record<string, unknown>>({})

  useEffect(() => {
    if (node) {
      setLocalData({ ...(node.data as Record<string, unknown>) })
    }
  }, [node])

  if (!node) return null

  const update = (key: string, value: unknown) => {
    const next = { ...localData, [key]: value }
    setLocalData(next)
    onUpdate(node.id, next)
  }

  const nodeType = node.type || ''

  return (
    <div className="w-72 border-l bg-white flex flex-col h-full">
      <div className="px-4 py-3 border-b flex items-center justify-between">
        <div>
          <h3 className="text-sm font-semibold text-gray-800">节点属性</h3>
          <p className="text-xs text-gray-400 mt-0.5">{(localData.label as string) || nodeType}</p>
        </div>
        <button onClick={onClose} className="text-gray-400 hover:text-gray-600">
          <X className="w-4 h-4" />
        </button>
      </div>

      <div className="flex-1 overflow-y-auto p-4 space-y-4">
        {/* Common: label */}
        <Field label="名称">
          <input
            value={(localData.label as string) || ''}
            onChange={(e) => update('label', e.target.value)}
            className="input-sm"
          />
        </Field>

        {/* LLM Node */}
        {nodeType === 'llm' && (
          <>
            <Field label="模型">
              <select
                value={(localData.model as string) || ''}
                onChange={(e) => update('model', e.target.value)}
                className="input-sm"
              >
                <option value="">选择模型</option>
                {models.map((m) => (
                  <option key={m.id} value={m.model_name}>
                    {m.display_name || `${m.provider} / ${m.model_name}`}
                  </option>
                ))}
              </select>
            </Field>
            <Field label="Prompt 模板">
              <textarea
                value={(localData.prompt as string) || ''}
                onChange={(e) => update('prompt', e.target.value)}
                rows={5}
                className="input-sm resize-none"
                placeholder="使用 {{input}} 引用上游输入"
              />
            </Field>
            <Field label="Temperature">
              <div className="flex items-center gap-2">
                <input
                  type="range"
                  min="0"
                  max="2"
                  step="0.1"
                  value={(localData.temperature as number) ?? 0.7}
                  onChange={(e) => update('temperature', parseFloat(e.target.value))}
                  className="flex-1"
                />
                <span className="text-xs text-gray-500 w-8 text-right">
                  {(localData.temperature as number)?.toFixed(1) ?? '0.7'}
                </span>
              </div>
            </Field>
            <Field label="Max Tokens">
              <input
                type="number"
                value={(localData.maxTokens as number) || 4096}
                onChange={(e) => update('maxTokens', parseInt(e.target.value) || 4096)}
                className="input-sm"
              />
            </Field>
          </>
        )}

        {/* Tool Node */}
        {nodeType === 'tool' && (
          <>
            <Field label="工具">
              <select
                value={(localData.toolName as string) || ''}
                onChange={(e) => update('toolName', e.target.value)}
                className="input-sm"
              >
                <option value="">选择工具</option>
                {tools.map((t) => (
                  <option key={t} value={t}>
                    {t === 'web_search' ? 'Web 搜索' : t === 'http_request' ? 'HTTP 请求' : t}
                  </option>
                ))}
              </select>
            </Field>
            <Field label="参数模板 (JSON)">
              <textarea
                value={(localData.argsTemplate as string) || ''}
                onChange={(e) => update('argsTemplate', e.target.value)}
                rows={4}
                className="input-sm resize-none font-mono text-xs"
                placeholder='{"query": "{{input}}"}'
              />
            </Field>
          </>
        )}

        {/* Condition Node */}
        {nodeType === 'condition' && (
          <>
            <Field label="条件表达式">
              <textarea
                value={(localData.expression as string) || ''}
                onChange={(e) => update('expression', e.target.value)}
                rows={3}
                className="input-sm resize-none font-mono text-xs"
                placeholder='input.contains("error")'
              />
            </Field>
            <div className="text-xs text-gray-400 bg-gray-50 rounded-lg p-3">
              <p className="font-medium text-gray-500 mb-1">说明</p>
              <p>表达式为 true 走绿色出口，false 走红色出口。</p>
              <p className="mt-1">可使用 <code className="bg-gray-200 px-1 rounded">input</code> 引用上游节点的输出。</p>
            </div>
          </>
        )}

        {/* Start Node */}
        {nodeType === 'start' && (
          <Field label="输入变量">
            <textarea
              value={(localData.inputVars as string) || ''}
              onChange={(e) => update('inputVars', e.target.value)}
              rows={3}
              className="input-sm resize-none font-mono text-xs"
              placeholder="user_message"
            />
          </Field>
        )}

        {/* End Node */}
        {nodeType === 'end' && (
          <Field label="输出映射">
            <textarea
              value={(localData.outputMapping as string) || ''}
              onChange={(e) => update('outputMapping', e.target.value)}
              rows={3}
              className="input-sm resize-none font-mono text-xs"
              placeholder="{{last_node_output}}"
            />
          </Field>
        )}

        {/* Node ID (read-only) */}
        <div className="pt-2 border-t">
          <p className="text-xs text-gray-400">
            ID: <code className="bg-gray-100 px-1 rounded">{node.id}</code>
          </p>
        </div>
      </div>
    </div>
  )
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div>
      <label className="block text-xs font-medium text-gray-600 mb-1.5">{label}</label>
      {children}
    </div>
  )
}
