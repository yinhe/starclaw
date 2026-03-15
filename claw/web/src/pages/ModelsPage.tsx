import { useState, useEffect } from 'react'
import { Cpu, Plus, Trash2, X, Check, Key, Globe, ChevronDown, ChevronUp, Pencil, Save } from 'lucide-react'
import { modelAPI } from '../lib/api'

interface ModelConfig {
  id: string
  provider: string
  model_name: string
  display_name: string
  base_url: string
  max_tokens: number
  temperature: number
  is_platform: boolean
  is_enabled: boolean
}

interface EditForm {
  api_key: string
  base_url: string
}

const PROVIDERS = [
  { value: 'star-ai', label: 'Star AI', desc: '聚合 OpenAI / Claude / Gemini / DeepSeek / Qwen 等 60+ 模型，Claw 身份免密', icon: '⚡', base_url: 'https://star-ai.net/v1' },
  { value: 'qwen', label: '通义千问 (Qwen)', desc: '阿里云百炼，180+ 模型，文本/图像/视频/语音全覆盖', icon: '🤖', base_url: 'https://dashscope.aliyuncs.com/compatible-mode/v1' },
  { value: 'openai', label: 'OpenAI', desc: 'GPT-4o, o1, DALL-E 等', icon: '🟢', base_url: 'https://api.openai.com/v1' },
  { value: 'anthropic', label: 'Anthropic', desc: 'Claude 4, Claude 3.5 Sonnet 等', icon: '🟠', base_url: 'https://api.anthropic.com' },
  { value: 'google', label: 'Google', desc: 'Gemini 2.0, Gemini Pro 等', icon: '🔵', base_url: 'https://generativelanguage.googleapis.com/v1beta/openai' },
  { value: 'deepseek', label: 'DeepSeek', desc: 'DeepSeek V3, R1 推理模型', icon: '🐋', base_url: 'https://api.deepseek.com/v1' },
  { value: 'ollama', label: 'Ollama (本地)', desc: '本地部署开源模型', icon: '🏠', base_url: 'http://localhost:11434' },
  { value: 'openrouter', label: 'OpenRouter', desc: '聚合多家模型的统一接口', icon: '🔀', base_url: 'https://openrouter.ai/api/v1' },
  { value: 'fal', label: 'fal.ai', desc: 'Llama, Mistral, DeepSeek 等开源模型快速推理', icon: '⚡', base_url: 'https://fal.run/fal-ai/any-llm/v1' },
  { value: 'grok', label: 'Grok (xAI)', desc: 'Grok-3, Grok-2 等 xAI 模型', icon: '𝕏', base_url: 'https://api.x.ai/v1' },
  { value: 'minimax', label: 'MiniMax', desc: 'M2.5 旗舰、Hailuo 视频、语音合成、音乐生成', icon: '🐚', base_url: 'https://api.minimax.io/v1' },
  { value: 'zhipu', label: '智谱 (GLM)', desc: 'GLM-4 系列', icon: '💎', base_url: 'https://open.bigmodel.cn/api/paas/v4' },
  { value: 'moonshot', label: 'Moonshot (Kimi)', desc: 'Kimi 长文本模型', icon: '🌙', base_url: 'https://api.moonshot.cn/v1' },
  { value: 'custom', label: '自定义 (OpenAI 兼容)', desc: '任何兼容 OpenAI API 的服务', icon: '⚙️', base_url: '' },
]

const QWEN_REGIONS = [
  { value: 'https://dashscope.aliyuncs.com/compatible-mode/v1', label: '华北2（北京）' },
  { value: 'https://dashscope-intl.aliyuncs.com/compatible-mode/v1', label: '新加坡' },
  { value: 'https://dashscope-us.aliyuncs.com/compatible-mode/v1', label: '美国（弗吉尼亚）' },
]

const PROVIDER_LABELS: Record<string, string> = {
  'star-ai': 'Star AI',
  qwen: '通义千问 (Qwen)',
  openai: 'OpenAI',
  anthropic: 'Anthropic',
  google: 'Google',
  deepseek: 'DeepSeek',
  ollama: 'Ollama',
  openrouter: 'OpenRouter',
  fal: 'fal.ai',
  grok: 'Grok (xAI)',
  zhipu: '智谱 (GLM)',
  moonshot: 'Moonshot',
  custom: '自定义',
}

const REGION_LABELS: Record<string, string> = {
  'https://dashscope.aliyuncs.com/compatible-mode/v1': '北京',
  'https://dashscope-intl.aliyuncs.com/compatible-mode/v1': '新加坡',
  'https://dashscope-us.aliyuncs.com/compatible-mode/v1': '美国',
}

export default function ModelsPage() {
  const [models, setModels] = useState<ModelConfig[]>([])
  const [showModal, setShowModal] = useState(false)
  const [expandedId, setExpandedId] = useState<string | null>(null)
  const [editingId, setEditingId] = useState<string | null>(null)
  const [editForm, setEditForm] = useState<EditForm>({ api_key: '', base_url: '' })
  const [availableModels, setAvailableModels] = useState<Record<string, string[]>>({})
  const [form, setForm] = useState({
    provider: 'qwen',
    api_key: '',
    base_url: QWEN_REGIONS[0].value,
  })

  useEffect(() => {
    loadModels()
  }, [])

  const loadModels = async () => {
    try {
      const [listRes, availRes] = await Promise.all([modelAPI.list(), modelAPI.available()])
      setModels(listRes.data.models || [])
      const avail: Record<string, string[]> = {}
      for (const p of availRes.data.providers || []) {
        avail[p.config_id] = p.models
      }
      setAvailableModels(avail)
    } catch { /* ignore */ }
  }

  const handleCreate = async () => {
    try {
      await modelAPI.create({
        provider: form.provider,
        api_key: form.provider === 'star-ai' ? 'claw-identity' : form.api_key,
        base_url: form.base_url,
      })
      setShowModal(false)
      setForm({ provider: 'qwen', api_key: '', base_url: QWEN_REGIONS[0].value })
      loadModels()
    } catch { /* ignore */ }
  }

  const handleDelete = async (id: string) => {
    if (!confirm('确定要删除这个模型提供商配置吗？')) return
    try {
      await modelAPI.delete(id)
      loadModels()
    } catch { /* ignore */ }
  }

  const startEdit = (m: ModelConfig) => {
    setEditingId(m.id)
    setEditForm({ api_key: '', base_url: m.base_url })
    setExpandedId(m.id)
  }

  const handleUpdate = async (id: string, provider: string) => {
    try {
      const data: Record<string, unknown> = {
        provider,
        base_url: editForm.base_url,
      }
      if (editForm.api_key) {
        data.api_key = editForm.api_key
      }
      await modelAPI.update(id, data)
      setEditingId(null)
      loadModels()
    } catch { /* ignore */ }
  }

  const needsCustomUrl = !['star-ai', 'qwen', 'openai', 'anthropic', 'deepseek', 'google', 'zhipu', 'moonshot', 'fal'].includes(form.provider)
  const isStarAI = form.provider === 'star-ai'

  return (
    <div className="h-full overflow-y-auto">
      <div className="max-w-4xl mx-auto p-8">
        <div className="flex items-center justify-between mb-8">
          <div>
            <h1 className="text-2xl font-bold text-gray-900">模型提供商</h1>
            <p className="text-gray-500 text-sm mt-1">添加提供商的 API Key，即可使用其全部模型</p>
          </div>
          <button
            onClick={() => setShowModal(true)}
            className="flex items-center gap-2 px-4 py-2 bg-primary-600 text-white rounded-lg text-sm hover:bg-primary-700 transition-colors"
          >
            <Plus className="w-4 h-4" />
            添加提供商
          </button>
        </div>

        {models.length === 0 ? (
          <div className="text-center py-20 text-gray-400">
            <Cpu className="w-12 h-12 mx-auto mb-3 opacity-50" />
            <p className="mb-1">还没有配置模型提供商</p>
            <p className="text-xs">添加提供商并填入 API Key，所有模型即刻可用</p>
          </div>
        ) : (
          <div className="space-y-4">
            {models.map((m) => {
              const modelCount = availableModels[m.id]?.length || 0
              const isExpanded = expandedId === m.id
              return (
                <div key={m.id} className="bg-white rounded-xl border overflow-hidden">
                  <div
                    className="flex items-center justify-between px-5 py-4 cursor-pointer hover:bg-gray-50 transition-colors"
                    onClick={() => setExpandedId(isExpanded ? null : m.id)}
                  >
                    <div className="flex items-center gap-4">
                      <div className="w-10 h-10 rounded-lg bg-primary-50 flex items-center justify-center text-xl">
                        {PROVIDERS.find(p => p.value === m.provider)?.icon || '🤖'}
                      </div>
                      <div>
                        <div className="font-semibold text-gray-900 flex items-center gap-2">
                          {PROVIDER_LABELS[m.provider] || m.provider}
                          {m.is_platform && <span className="text-[10px] px-1.5 py-0.5 rounded-full bg-emerald-100 text-emerald-700 font-medium">平台</span>}
                        </div>
                        <div className="text-xs text-gray-400 flex items-center gap-3 mt-0.5">
                          <span className="flex items-center gap-1">
                            <Key className="w-3 h-3" />
                            {m.is_platform ? '平台共享 Key' : 'API Key 已配置'}
                          </span>
                          {m.base_url && REGION_LABELS[m.base_url] && (
                            <span className="flex items-center gap-1">
                              <Globe className="w-3 h-3" />
                              {REGION_LABELS[m.base_url]}
                            </span>
                          )}
                          <span className="text-primary-500 font-medium">{modelCount} 个模型可用</span>
                        </div>
                      </div>
                    </div>
                    <div className="flex items-center gap-3">
                      <span className={`px-2 py-0.5 rounded-full text-xs ${m.is_enabled ? 'bg-green-50 text-green-600' : 'bg-gray-100 text-gray-400'}`}>
                        {m.is_enabled ? '已启用' : '已停用'}
                      </span>
                      {!m.is_platform && <>
                        <button
                          onClick={(e) => { e.stopPropagation(); startEdit(m) }}
                          className="p-1.5 text-gray-300 hover:text-primary-500 transition-colors"
                          title="编辑"
                        >
                          <Pencil className="w-4 h-4" />
                        </button>
                        <button
                          onClick={(e) => { e.stopPropagation(); handleDelete(m.id) }}
                          className="p-1.5 text-gray-300 hover:text-red-500 transition-colors"
                          title="删除"
                        >
                          <Trash2 className="w-4 h-4" />
                        </button>
                      </>}
                      {isExpanded ? <ChevronUp className="w-4 h-4 text-gray-400" /> : <ChevronDown className="w-4 h-4 text-gray-400" />}
                    </div>
                  </div>
                  {isExpanded && (
                    <div className="border-t">
                      {/* Edit form */}
                      {editingId === m.id && (
                        <div className="px-5 py-4 bg-primary-50/50 space-y-3">
                          <p className="text-sm font-medium text-gray-700">编辑提供商配置</p>
                          <div>
                            <label className="block text-xs text-gray-500 mb-1">API Key（留空则不修改）</label>
                            <input
                              type="password"
                              value={editForm.api_key}
                              onChange={(e) => setEditForm({ ...editForm, api_key: e.target.value })}
                              className="w-full px-3 py-2 border rounded-lg text-sm outline-none focus:ring-2 focus:ring-primary-500 bg-white"
                              placeholder="留空保持原 Key 不变"
                            />
                          </div>
                          <div>
                            {m.provider === 'qwen' ? (
                              <>
                                <label className="block text-xs text-gray-500 mb-1">地域</label>
                                <select
                                  value={editForm.base_url}
                                  onChange={(e) => setEditForm({ ...editForm, base_url: e.target.value })}
                                  className="w-full px-3 py-2 border rounded-lg text-sm outline-none focus:ring-2 focus:ring-primary-500 bg-white"
                                >
                                  {QWEN_REGIONS.map((r) => (
                                    <option key={r.value} value={r.value}>{r.label}</option>
                                  ))}
                                </select>
                                <p className="text-xs text-gray-400 mt-1">Base URL: {editForm.base_url}</p>
                              </>
                            ) : (
                              <>
                                <label className="block text-xs text-gray-500 mb-1">Base URL</label>
                                <input
                                  value={editForm.base_url}
                                  onChange={(e) => setEditForm({ ...editForm, base_url: e.target.value })}
                                  className="w-full px-3 py-2 border rounded-lg text-sm outline-none focus:ring-2 focus:ring-primary-500 bg-white"
                                  placeholder={PROVIDERS.find(p => p.value === m.provider)?.base_url || '留空使用默认地址'}
                                />
                              </>
                            )}
                          </div>
                          <div className="flex gap-2 pt-1">
                            <button
                              onClick={() => handleUpdate(m.id, m.provider)}
                              className="flex items-center gap-1.5 px-3 py-1.5 bg-primary-600 text-white rounded-lg text-sm hover:bg-primary-700 transition-colors"
                            >
                              <Save className="w-3.5 h-3.5" /> 保存
                            </button>
                            <button
                              onClick={() => setEditingId(null)}
                              className="px-3 py-1.5 text-gray-500 hover:bg-gray-100 rounded-lg text-sm transition-colors"
                            >
                              取消
                            </button>
                          </div>
                        </div>
                      )}
                      {/* Available models */}
                      {availableModels[m.id] && (
                        <div className="px-5 py-4 bg-gray-50/50">
                          <p className="text-xs text-gray-500 mb-3">可用模型（{availableModels[m.id].length} 个）：</p>
                          <div className="flex flex-wrap gap-1.5 max-h-48 overflow-y-auto">
                            {availableModels[m.id].map((name) => (
                              <span key={name} className="px-2 py-0.5 bg-white border rounded text-xs text-gray-600">
                                {name}
                              </span>
                            ))}
                          </div>
                        </div>
                      )}
                    </div>
                  )}
                </div>
              )
            })}
          </div>
        )}
      </div>

      {/* Add Provider Modal */}
      {showModal && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
          <div className="bg-white rounded-2xl w-full max-w-lg mx-4 p-6">
            <div className="flex items-center justify-between mb-5">
              <h2 className="text-lg font-semibold">添加模型提供商</h2>
              <button onClick={() => setShowModal(false)} className="text-gray-400 hover:text-gray-600">
                <X className="w-5 h-5" />
              </button>
            </div>

            <div className="space-y-4">
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-2">选择提供商</label>
                <div className="grid grid-cols-2 gap-2 max-h-64 overflow-y-auto">
                  {PROVIDERS.map((p) => (
                    <button
                      key={p.value}
                      onClick={() => setForm({
                        ...form,
                        provider: p.value,
                        base_url: p.value === 'qwen' ? QWEN_REGIONS[0].value : (p.base_url || ''),
                      })}
                      className={`text-left px-3 py-2.5 rounded-lg border text-sm transition-all ${
                        form.provider === p.value
                          ? 'border-primary-500 bg-primary-50 ring-1 ring-primary-500'
                          : 'border-gray-200 hover:border-gray-300'
                      }`}
                    >
                      <div className="flex items-center gap-2">
                        <span>{p.icon}</span>
                        <span className="font-medium text-gray-800">{p.label}</span>
                        {form.provider === p.value && <Check className="w-3.5 h-3.5 text-primary-600 ml-auto" />}
                      </div>
                      <p className="text-xs text-gray-400 mt-0.5 ml-6">{p.desc}</p>
                    </button>
                  ))}
                </div>
              </div>

              {isStarAI ? (
                <div className="bg-emerald-50 rounded-lg px-4 py-3 text-xs text-emerald-700">
                  Star AI 使用 Claw 节点身份认证，无需 API Key。添加后即可直接使用 60+ 模型。
                </div>
              ) : (
                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-1">API Key</label>
                  <input
                    type="password"
                    value={form.api_key}
                    onChange={(e) => setForm({ ...form, api_key: e.target.value })}
                    className="w-full px-3 py-2 border rounded-lg text-sm outline-none focus:ring-2 focus:ring-primary-500"
                    placeholder="sk-..."
                  />
                </div>
              )}

              {form.provider === 'qwen' ? (
                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-1">地域</label>
                  <select
                    value={form.base_url}
                    onChange={(e) => setForm({ ...form, base_url: e.target.value })}
                    className="w-full px-3 py-2 border rounded-lg text-sm outline-none focus:ring-2 focus:ring-primary-500"
                  >
                    {QWEN_REGIONS.map((r) => (
                      <option key={r.value} value={r.value}>{r.label}</option>
                    ))}
                  </select>
                  <p className="text-xs text-gray-400 mt-1">Base URL: {form.base_url}</p>
                </div>
              ) : (
                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-1">
                    Base URL{!needsCustomUrl && '（可选，留空使用默认地址）'}
                  </label>
                  <input
                    value={form.base_url}
                    onChange={(e) => setForm({ ...form, base_url: e.target.value })}
                    className="w-full px-3 py-2 border rounded-lg text-sm outline-none focus:ring-2 focus:ring-primary-500"
                    placeholder={PROVIDERS.find(p => p.value === form.provider)?.base_url || 'https://api.example.com/v1'}
                  />
                </div>
              )}

              <div className="bg-blue-50 rounded-lg px-4 py-3 text-xs text-blue-700">
                添加后，该提供商的所有模型都会自动可用。创建 Agent 时可选择任意模型。
              </div>
            </div>

            <div className="flex justify-end gap-3 mt-6">
              <button
                onClick={() => setShowModal(false)}
                className="px-4 py-2 text-sm text-gray-600 hover:bg-gray-100 rounded-lg transition-colors"
              >
                取消
              </button>
              <button
                onClick={handleCreate}
                disabled={!isStarAI && !form.api_key}
                className="px-4 py-2 text-sm bg-primary-600 text-white rounded-lg hover:bg-primary-700 disabled:opacity-50 transition-colors"
              >
                添加提供商
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
