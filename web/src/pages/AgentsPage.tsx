import { useState, useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { Bot, Plus, Pencil, Trash2, X, Wrench, Sparkles, Download, Upload } from 'lucide-react'
import { agentAPI, modelAPI, toolAPI, knowledgeBaseAPI, superAgentAPI } from '../lib/api'

interface Agent {
  id: string
  name: string
  description: string
  system_prompt: string
  model_id: string
  tools: string
  knowledge_base_id: string
  is_public: boolean
  is_builtin: boolean
  created_at: string
}

interface KB {
  id: string
  name: string
}

interface ModelConfig {
  id: string
  provider: string
  model_name: string
  display_name: string
}

export default function AgentsPage() {
  const navigate = useNavigate()
  const [agents, setAgents] = useState<Agent[]>([])
  const [showModal, setShowModal] = useState(false)
  const [editingAgent, setEditingAgent] = useState<Agent | null>(null)
  const [models, setModels] = useState<ModelConfig[]>([])
  const [availableTools, setAvailableTools] = useState<string[]>([])
  const [knowledgeBases, setKnowledgeBases] = useState<KB[]>([])
  const [form, setForm] = useState({
    name: '',
    description: '',
    system_prompt: '',
    model_id: '',
    tools: '' as string,
    knowledge_base_id: '',
    is_public: false,
  })
  const [selectedTools, setSelectedTools] = useState<string[]>([])

  useEffect(() => {
    loadAgents()
    loadModels()
    loadTools()
    loadKBs()
  }, [])

  const loadAgents = async () => {
    try {
      // Ensure built-in agents (SuperAgent + specialists) exist
      try { await superAgentAPI.ensure() } catch { /* ignore */ }
      const res = await agentAPI.list()
      setAgents(res.data.agents || [])
    } catch { /* ignore */ }
  }

  const loadModels = async () => {
    try {
      const res = await modelAPI.list()
      setModels(res.data.models || [])
    } catch { /* ignore */ }
  }

  const loadTools = async () => {
    try {
      const res = await toolAPI.list()
      setAvailableTools(res.data.tools || [])
    } catch { /* ignore */ }
  }

  const loadKBs = async () => {
    try {
      const res = await knowledgeBaseAPI.list()
      setKnowledgeBases(res.data.knowledge_bases || [])
    } catch { /* ignore */ }
  }

  const handleSave = async () => {
    try {
      const payload = { ...form, tools: JSON.stringify(selectedTools) }
      if (editingAgent) {
        await agentAPI.update(editingAgent.id, payload)
      } else {
        await agentAPI.create(payload)
      }
      setShowModal(false)
      setEditingAgent(null)
      setForm({ name: '', description: '', system_prompt: '', model_id: '', tools: '', knowledge_base_id: '', is_public: false })
      setSelectedTools([])
      loadAgents()
    } catch { /* ignore */ }
  }

  const handleEdit = (agent: Agent) => {
    setEditingAgent(agent)
    setForm({
      name: agent.name,
      description: agent.description,
      system_prompt: agent.system_prompt,
      model_id: agent.model_id,
      tools: agent.tools || '',
      knowledge_base_id: agent.knowledge_base_id || '',
      is_public: agent.is_public,
    })
    try {
      setSelectedTools(agent.tools ? JSON.parse(agent.tools) : [])
    } catch { setSelectedTools([]) }
    setShowModal(true)
  }

  const handleDelete = async (id: string) => {
    if (!confirm('确定要删除这个 Agent 吗？')) return
    try {
      await agentAPI.delete(id)
      loadAgents()
    } catch { /* ignore */ }
  }

  return (
    <div className="h-full overflow-y-auto">
      <div className="max-w-5xl mx-auto p-8">
        <div className="flex items-center justify-between mb-8">
          <div>
            <h1 className="text-2xl font-bold text-gray-900">Agents</h1>
            <p className="text-gray-500 text-sm mt-1">创建和管理你的 AI 智能体</p>
          </div>
          <div className="flex items-center gap-2">
            <label className="flex items-center gap-2 px-4 py-2 border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-200 rounded-lg text-sm hover:bg-gray-50 dark:hover:bg-gray-800 transition-colors cursor-pointer">
              <Upload className="w-4 h-4" />
              导入
              <input
                type="file"
                accept=".json"
                className="hidden"
                onChange={async (e) => {
                  const file = e.target.files?.[0]
                  if (!file) return
                  try {
                    const text = await file.text()
                    const data = JSON.parse(text)
                    await agentAPI.import(data)
                    loadAgents()
                  } catch { /* ignore */ }
                  e.target.value = ''
                }}
              />
            </label>
            <button
              onClick={() => {
                setEditingAgent(null)
                setForm({ name: '', description: '', system_prompt: '', model_id: '', tools: '', knowledge_base_id: '', is_public: false })
                setSelectedTools([])
                setShowModal(true)
              }}
              className="flex items-center gap-2 px-4 py-2 bg-primary-600 text-white rounded-lg text-sm hover:bg-primary-700 transition-colors"
            >
              <Plus className="w-4 h-4" />
              创建 Agent
            </button>
          </div>
        </div>

        {agents.length === 0 ? (
          <div className="text-center py-20 text-gray-400">
            <Bot className="w-12 h-12 mx-auto mb-3 opacity-50" />
            <p>还没有 Agent，点击上方按钮创建第一个</p>
          </div>
        ) : (
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
            {agents.map((agent) => (
              <div
                key={agent.id}
                className="bg-white border rounded-xl p-5 hover:shadow-md transition-shadow cursor-pointer group"
                onClick={() => navigate(`/agents/${agent.id}`)}
              >
                <div className="flex items-start justify-between mb-3">
                  <div className="w-10 h-10 bg-primary-100 rounded-lg flex items-center justify-center group-hover:bg-primary-200 transition-colors">
                    <Bot className="w-5 h-5 text-primary-600" />
                  </div>
                  <div className="flex items-center gap-1" onClick={(e) => e.stopPropagation()}>
                    {agent.is_builtin && (
                      <span className="px-1.5 py-0.5 bg-indigo-50 text-indigo-600 text-[10px] font-medium rounded mr-1">官方</span>
                    )}
                    <button
                      onClick={async () => {
                        try {
                          const res = await agentAPI.export(agent.id)
                          const blob = new Blob([JSON.stringify(res.data, null, 2)], { type: 'application/json' })
                          const url = URL.createObjectURL(blob)
                          const a = document.createElement('a')
                          a.href = url
                          a.download = `agent_${agent.name}.json`
                          a.click()
                          URL.revokeObjectURL(url)
                        } catch { /* ignore */ }
                      }}
                      className="p-1.5 text-gray-400 hover:text-blue-500 rounded-md hover:bg-blue-50"
                      title="导出 JSON"
                    >
                      <Download className="w-3.5 h-3.5" />
                    </button>
                    <button
                      onClick={() => handleEdit(agent)}
                      className="p-1.5 text-gray-400 hover:text-gray-600 rounded-md hover:bg-gray-100"
                    >
                      <Pencil className="w-3.5 h-3.5" />
                    </button>
                    {!agent.is_builtin && (
                      <button
                        onClick={() => handleDelete(agent.id)}
                        className="p-1.5 text-gray-400 hover:text-red-500 rounded-md hover:bg-red-50"
                      >
                        <Trash2 className="w-3.5 h-3.5" />
                      </button>
                    )}
                  </div>
                </div>
                <h3 className="font-semibold text-gray-900 group-hover:text-primary-600 transition-colors">{agent.name}</h3>
                <p className="text-sm text-gray-500 mt-1 line-clamp-2">
                  {agent.description || '暂无描述'}
                </p>
                {agent.is_public && (
                  <span className="inline-block mt-2 px-2 py-0.5 bg-green-50 text-green-600 text-xs rounded-full">
                    公开
                  </span>
                )}
              </div>
            ))}
          </div>
        )}
      </div>

      {/* Modal */}
      {showModal && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
          <div className="bg-white rounded-2xl w-full max-w-lg mx-4 p-6">
            <div className="flex items-center justify-between mb-5">
              <h2 className="text-lg font-semibold">
                {editingAgent ? '编辑 Agent' : '创建 Agent'}
              </h2>
              <button
                onClick={() => setShowModal(false)}
                className="text-gray-400 hover:text-gray-600"
              >
                <X className="w-5 h-5" />
              </button>
            </div>

            <div className="space-y-4">
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">名称</label>
                <input
                  value={form.name}
                  onChange={(e) => setForm({ ...form, name: e.target.value })}
                  className="w-full px-3 py-2 border rounded-lg text-sm outline-none focus:ring-2 focus:ring-primary-500"
                  placeholder="例如：代码助手"
                />
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">描述</label>
                <input
                  value={form.description}
                  onChange={(e) => setForm({ ...form, description: e.target.value })}
                  className="w-full px-3 py-2 border rounded-lg text-sm outline-none focus:ring-2 focus:ring-primary-500"
                  placeholder="简短描述这个 Agent 的功能"
                />
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">模型</label>
                <select
                  value={form.model_id}
                  onChange={(e) => setForm({ ...form, model_id: e.target.value })}
                  className="w-full px-3 py-2 border rounded-lg text-sm outline-none focus:ring-2 focus:ring-primary-500"
                >
                  <option value="">选择模型</option>
                  {models.map((m) => (
                    <option key={m.id} value={m.id}>
                      {m.display_name || `${m.provider} / ${m.model_name}`}
                    </option>
                  ))}
                </select>
              </div>
              <div>
                <div className="flex items-center justify-between mb-1">
                  <label className="block text-sm font-medium text-gray-700">
                    System Prompt
                  </label>
                  <div className="relative group">
                    <button type="button" className="flex items-center gap-1 text-xs text-primary-600 hover:text-primary-700">
                      <Sparkles className="w-3 h-3" /> 模板
                    </button>
                    <div className="absolute right-0 top-full mt-1 z-20 bg-white border rounded-lg shadow-lg py-1 w-56 hidden group-hover:block">
                      {[
                        { label: '通用助手', prompt: '你是一个有帮助的 AI 助手。请用简洁清晰的中文回答用户的问题。如果不确定答案，请诚实说明。' },
                        { label: '代码专家', prompt: '你是一个资深软件工程师。请提供高质量的代码解决方案，包含注释和最佳实践。默认使用用户提到的编程语言，没有指定则使用 Python。' },
                        { label: '翻译专家', prompt: '你是一个专业翻译。用户发送中文时翻译为英文，发送英文时翻译为中文。保持原文风格和语气，不要添加解释。' },
                        { label: '文案写手', prompt: '你是一个创意文案专家。根据用户需求撰写引人注目的文案，包括标题、正文和行动召唤。风格简洁有力，善用修辞。' },
                        { label: '数据分析师', prompt: '你是一个数据分析专家。帮助用户分析数据、生成图表代码、解读统计结果。使用 Python pandas/matplotlib 进行数据处理。' },
                        { label: 'RAG 问答', prompt: '你是一个基于知识库的问答助手。请仅根据提供的上下文信息回答问题。如果上下文中没有相关信息，请回答"根据现有知识库，我无法找到相关答案"。' },
                      ].map((tpl) => (
                        <button
                          key={tpl.label}
                          type="button"
                          onClick={() => setForm({ ...form, system_prompt: tpl.prompt })}
                          className="w-full text-left px-3 py-1.5 text-xs text-gray-600 hover:bg-gray-50"
                        >
                          {tpl.label}
                        </button>
                      ))}
                    </div>
                  </div>
                </div>
                <textarea
                  value={form.system_prompt}
                  onChange={(e) => setForm({ ...form, system_prompt: e.target.value })}
                  rows={5}
                  className="w-full px-3 py-2 border rounded-lg text-sm outline-none focus:ring-2 focus:ring-primary-500 resize-none"
                  placeholder="你是一个有帮助的助手..."
                />
              </div>
              {availableTools.length > 0 && (
                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-2">
                    <Wrench className="w-3.5 h-3.5 inline mr-1" />
                    启用工具
                  </label>
                  <div className="flex flex-wrap gap-2">
                    {availableTools.map((t) => (
                      <label
                        key={t}
                        className={`flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs border cursor-pointer transition-colors ${
                          selectedTools.includes(t)
                            ? 'bg-primary-50 border-primary-300 text-primary-700'
                            : 'bg-white border-gray-200 text-gray-500 hover:border-gray-300'
                        }`}
                      >
                        <input
                          type="checkbox"
                          checked={selectedTools.includes(t)}
                          onChange={(e) => {
                            if (e.target.checked) {
                              setSelectedTools([...selectedTools, t])
                            } else {
                              setSelectedTools(selectedTools.filter((x) => x !== t))
                            }
                          }}
                          className="hidden"
                        />
                        {t === 'web_search' ? 'Web 搜索' : t === 'http_request' ? 'HTTP 请求' : t}
                      </label>
                    ))}
                  </div>
                </div>
              )}
              {knowledgeBases.length > 0 && (
                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-1">知识库 (RAG)</label>
                  <select
                    value={form.knowledge_base_id}
                    onChange={(e) => setForm({ ...form, knowledge_base_id: e.target.value })}
                    className="w-full px-3 py-2 border rounded-lg text-sm outline-none focus:ring-2 focus:ring-primary-500"
                  >
                    <option value="">不使用知识库</option>
                    {knowledgeBases.map((kb) => (
                      <option key={kb.id} value={kb.id}>{kb.name}</option>
                    ))}
                  </select>
                </div>
              )}
              <label className="flex items-center gap-2 text-sm">
                <input
                  type="checkbox"
                  checked={form.is_public}
                  onChange={(e) => setForm({ ...form, is_public: e.target.checked })}
                  className="rounded"
                />
                <span className="text-gray-700">公开（其他用户可见）</span>
              </label>
            </div>

            <div className="flex justify-end gap-3 mt-6">
              <button
                onClick={() => setShowModal(false)}
                className="px-4 py-2 text-sm text-gray-600 hover:bg-gray-100 rounded-lg transition-colors"
              >
                取消
              </button>
              <button
                onClick={handleSave}
                disabled={!form.name}
                className="px-4 py-2 text-sm bg-primary-600 text-white rounded-lg hover:bg-primary-700 disabled:opacity-50 transition-colors"
              >
                {editingAgent ? '保存' : '创建'}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
