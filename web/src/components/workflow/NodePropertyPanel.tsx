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

const MEDIA_CATEGORIES = [
  { value: 'character', label: '角色' },
  { value: 'scene', label: '场景' },
  { value: 'prop', label: '道具' },
  { value: 'costume', label: '服装' },
  { value: 'reference', label: '参考' },
]

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
    <div className="w-72 border-l border-gray-800 bg-gray-900 flex flex-col h-full">
      <div className="px-4 py-3 border-b border-gray-800 flex items-center justify-between">
        <div>
          <h3 className="text-sm font-semibold text-gray-200">节点属性</h3>
          <p className="text-xs text-gray-500 mt-0.5">{(localData.label as string) || nodeType}</p>
        </div>
        <button onClick={onClose} className="text-gray-500 hover:text-gray-300 transition-colors">
          <X className="w-4 h-4" />
        </button>
      </div>

      <div className="flex-1 overflow-y-auto p-4 space-y-4">
        <Field label="名称">
          <input
            value={(localData.label as string) || ''}
            onChange={(e) => update('label', e.target.value)}
            className="input-dark"
          />
        </Field>

        {nodeType === 'llm' && (
          <>
            <Field label="模型">
              <select value={(localData.model as string) || ''} onChange={(e) => update('model', e.target.value)} className="input-dark">
                <option value="">选择模型</option>
                {models.map((m) => (
                  <option key={m.id} value={m.model_name}>{m.display_name || `${m.provider} / ${m.model_name}`}</option>
                ))}
              </select>
            </Field>
            <Field label="描述">
              <textarea value={(localData.description as string) || ''} onChange={(e) => update('description', e.target.value)}
                rows={3} className="input-dark resize-none" placeholder="节点功能说明" />
            </Field>
            <Field label="Prompt 模板">
              <textarea value={(localData.prompt as string) || ''} onChange={(e) => update('prompt', e.target.value)}
                rows={5} className="input-dark resize-none font-mono text-xs" placeholder="使用 {{input}} 引用上游输入" />
            </Field>
            <Field label="Temperature">
              <div className="flex items-center gap-2">
                <input type="range" min="0" max="2" step="0.1"
                  value={(localData.temperature as number) ?? 0.7}
                  onChange={(e) => update('temperature', parseFloat(e.target.value))}
                  className="flex-1 accent-blue-500" />
                <span className="text-xs text-gray-400 w-8 text-right">{(localData.temperature as number)?.toFixed(1) ?? '0.7'}</span>
              </div>
            </Field>
          </>
        )}

        {nodeType === 'tool' && (
          <>
            <Field label="工具">
              <select value={(localData.toolName as string) || ''} onChange={(e) => update('toolName', e.target.value)} className="input-dark">
                <option value="">选择工具</option>
                {tools.map((t) => (<option key={t} value={t}>{t}</option>))}
              </select>
            </Field>
            <Field label="描述">
              <textarea value={(localData.description as string) || ''} onChange={(e) => update('description', e.target.value)}
                rows={2} className="input-dark resize-none" placeholder="工具调用说明" />
            </Field>
            <Field label="参数模板 (JSON)">
              <textarea value={(localData.argsTemplate as string) || ''} onChange={(e) => update('argsTemplate', e.target.value)}
                rows={4} className="input-dark resize-none font-mono text-xs" placeholder='{"query": "{{input}}"}' />
            </Field>
          </>
        )}

        {nodeType === 'condition' && (
          <>
            <Field label="条件表达式">
              <textarea value={(localData.expression as string) || ''} onChange={(e) => update('expression', e.target.value)}
                rows={3} className="input-dark resize-none font-mono text-xs" placeholder='input.contains("error")' />
            </Field>
            <Field label="描述">
              <textarea value={(localData.description as string) || ''} onChange={(e) => update('description', e.target.value)}
                rows={2} className="input-dark resize-none" placeholder="分支逻辑说明" />
            </Field>
          </>
        )}

        {nodeType === 'media' && (
          <>
            <Field label="类型">
              <select value={(localData.category as string) || 'character'} onChange={(e) => update('category', e.target.value)} className="input-dark">
                {MEDIA_CATEGORIES.map((c) => (<option key={c.value} value={c.value}>{c.label}</option>))}
              </select>
            </Field>
            <Field label="图片 URL">
              <input value={(localData.imageUrl as string) || ''} onChange={(e) => update('imageUrl', e.target.value)}
                className="input-dark font-mono text-xs" placeholder="/v1/images/xxx 或 https://..." />
            </Field>
            <Field label="描述">
              <textarea value={(localData.description as string) || ''} onChange={(e) => update('description', e.target.value)}
                rows={3} className="input-dark resize-none" placeholder="角色/场景/道具描述" />
            </Field>
            {(localData.imageUrl as string) && (
              <div className="rounded-lg overflow-hidden border border-gray-700">
                <img src={localData.imageUrl as string} alt="" className="w-full h-32 object-cover"
                  onError={(e) => { (e.target as HTMLImageElement).style.display = 'none' }} />
              </div>
            )}
          </>
        )}

        {nodeType === 'start' && (
          <Field label="输入变量">
            <textarea value={(localData.inputVars as string) || ''} onChange={(e) => update('inputVars', e.target.value)}
              rows={3} className="input-dark resize-none font-mono text-xs" placeholder="user_message" />
          </Field>
        )}

        {nodeType === 'end' && (
          <Field label="输出映射">
            <textarea value={(localData.outputMapping as string) || ''} onChange={(e) => update('outputMapping', e.target.value)}
              rows={3} className="input-dark resize-none font-mono text-xs" placeholder="{{last_node_output}}" />
          </Field>
        )}

        <div className="pt-2 border-t border-gray-800">
          <p className="text-xs text-gray-600">
            ID: <code className="bg-gray-800 px-1.5 py-0.5 rounded text-gray-400">{node.id}</code>
          </p>
        </div>
      </div>
    </div>
  )
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div>
      <label className="block text-[11px] font-medium text-gray-400 mb-1.5 uppercase tracking-wider">{label}</label>
      {children}
    </div>
  )
}
