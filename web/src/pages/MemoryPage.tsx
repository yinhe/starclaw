import { useState, useEffect, useCallback } from 'react'
import { Brain, Search, Plus, Trash2, Pencil, X, Star, Bot, Pin, Lightbulb, FileText, Wrench, MessageSquare, Filter, RefreshCw } from 'lucide-react'
import { memoryAPI, agentAPI } from '../lib/api'

interface Memory {
  id: string
  user_id: string
  agent_id: string
  key: string
  content: string
  category: string
  source: string
  importance: number
  access_count: number
  last_access_at: string
  created_at: string
  updated_at: string
}

interface Agent {
  id: string
  name: string
}

const categoryConfig: Record<string, { label: string; icon: typeof Pin; color: string; bg: string }> = {
  instruct:   { label: '指令', icon: Pin,        color: 'text-red-600',    bg: 'bg-red-50 dark:bg-red-900/20' },
  fact:       { label: '事实', icon: FileText,    color: 'text-blue-600',   bg: 'bg-blue-50 dark:bg-blue-900/20' },
  preference: { label: '偏好', icon: Lightbulb,   color: 'text-amber-600',  bg: 'bg-amber-50 dark:bg-amber-900/20' },
  skill:      { label: '技能', icon: Wrench,      color: 'text-emerald-600', bg: 'bg-emerald-50 dark:bg-emerald-900/20' },
  context:    { label: '上下文', icon: MessageSquare, color: 'text-violet-600', bg: 'bg-violet-50 dark:bg-violet-900/20' },
}

const categories = ['instruct', 'fact', 'preference', 'skill', 'context']

export default function MemoryPage() {
  const [memories, setMemories] = useState<Memory[]>([])
  const [agents, setAgents] = useState<Agent[]>([])
  const [stats, setStats] = useState<{ total: number; categories: Record<string, number> }>({ total: 0, categories: {} })
  const [loading, setLoading] = useState(true)
  const [search, setSearch] = useState('')
  const [filterCategory, setFilterCategory] = useState('')
  const [filterAgent, setFilterAgent] = useState('')
  const [showCreate, setShowCreate] = useState(false)
  const [editingId, setEditingId] = useState<string | null>(null)
  const [editContent, setEditContent] = useState('')
  const [editImportance, setEditImportance] = useState(0.5)

  // Create form
  const [createForm, setCreateForm] = useState({ agent_id: '', key: '', content: '', category: 'fact', importance: 0.5 })

  const loadData = useCallback(async () => {
    setLoading(true)
    try {
      const params: Record<string, string> = {}
      if (filterCategory) params.category = filterCategory
      if (filterAgent) params.agent_id = filterAgent
      if (search) params.search = search

      const [memRes, statsRes, agentRes] = await Promise.all([
        memoryAPI.list(params),
        memoryAPI.stats(),
        agentAPI.list(),
      ])
      setMemories(memRes.data.memories || [])
      setStats(statsRes.data)
      setAgents((agentRes.data.agents || agentRes.data || []).map((a: any) => ({ id: a.id, name: a.name })))
    } catch (e) {
      console.error('Failed to load memories', e)
    } finally {
      setLoading(false)
    }
  }, [filterCategory, filterAgent, search])

  useEffect(() => { loadData() }, [loadData])

  const handleCreate = async () => {
    if (!createForm.agent_id || !createForm.key || !createForm.content) return
    try {
      await memoryAPI.create(createForm)
      setShowCreate(false)
      setCreateForm({ agent_id: '', key: '', content: '', category: 'fact', importance: 0.5 })
      loadData()
    } catch (e) {
      console.error('Failed to create memory', e)
    }
  }

  const handleUpdate = async (id: string) => {
    try {
      await memoryAPI.update(id, { content: editContent, importance: editImportance })
      setEditingId(null)
      loadData()
    } catch (e) {
      console.error('Failed to update memory', e)
    }
  }

  const handleDelete = async (id: string) => {
    if (!confirm('确定删除这条记忆？')) return
    try {
      await memoryAPI.delete(id)
      loadData()
    } catch (e) {
      console.error('Failed to delete memory', e)
    }
  }

  const getAgentName = (agentId: string) => {
    return agents.find(a => a.id === agentId)?.name || agentId.slice(0, 8)
  }

  const formatTime = (t: string) => {
    if (!t) return ''
    const d = new Date(t)
    const now = new Date()
    const diff = now.getTime() - d.getTime()
    if (diff < 60000) return '刚刚'
    if (diff < 3600000) return `${Math.floor(diff / 60000)}分钟前`
    if (diff < 86400000) return `${Math.floor(diff / 3600000)}小时前`
    if (diff < 604800000) return `${Math.floor(diff / 86400000)}天前`
    return d.toLocaleDateString()
  }

  const renderImportanceStars = (importance: number) => {
    const stars = Math.round(importance * 5)
    return (
      <span className="flex items-center gap-0.5">
        {[1, 2, 3, 4, 5].map(i => (
          <Star key={i} className={`w-3 h-3 ${i <= stars ? 'text-amber-400 fill-amber-400' : 'text-gray-300'}`} />
        ))}
      </span>
    )
  }

  return (
    <div className="p-6 max-w-6xl mx-auto space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <div className="w-10 h-10 rounded-xl bg-gradient-to-br from-violet-500 to-purple-600 flex items-center justify-center">
            <Brain className="w-5 h-5 text-white" />
          </div>
          <div>
            <h1 className="text-xl font-bold text-gray-800 dark:text-gray-100">记忆中心</h1>
            <p className="text-sm text-gray-500">Cerebrate — Claw 的跨会话长期记忆</p>
          </div>
        </div>
        <div className="flex items-center gap-2">
          <button onClick={loadData} className="p-2 text-gray-400 hover:text-gray-600 hover:bg-gray-100 dark:hover:bg-gray-800 rounded-lg transition-colors">
            <RefreshCw className="w-4 h-4" />
          </button>
          <button
            onClick={() => setShowCreate(true)}
            className="flex items-center gap-1.5 px-3 py-2 bg-violet-600 text-white rounded-lg hover:bg-violet-700 text-sm font-medium transition-colors"
          >
            <Plus className="w-4 h-4" />
            新建记忆
          </button>
        </div>
      </div>

      {/* Stats cards */}
      <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-6 gap-3">
        <div className="bg-white dark:bg-gray-800 rounded-xl border p-3 text-center">
          <div className="text-2xl font-bold text-gray-800 dark:text-gray-100">{stats.total}</div>
          <div className="text-xs text-gray-500">总记忆</div>
        </div>
        {categories.map(cat => {
          const cfg = categoryConfig[cat]
          const Icon = cfg.icon
          return (
            <div key={cat} className={`${cfg.bg} rounded-xl border p-3 text-center cursor-pointer hover:ring-2 hover:ring-violet-300 transition-all ${filterCategory === cat ? 'ring-2 ring-violet-500' : ''}`}
              onClick={() => setFilterCategory(filterCategory === cat ? '' : cat)}>
              <div className="flex items-center justify-center gap-1 mb-0.5">
                <Icon className={`w-3.5 h-3.5 ${cfg.color}`} />
                <span className={`text-2xl font-bold ${cfg.color}`}>{stats.categories?.[cat] || 0}</span>
              </div>
              <div className="text-xs text-gray-500">{cfg.label}</div>
            </div>
          )
        })}
      </div>

      {/* Search & filters */}
      <div className="flex flex-col sm:flex-row gap-3">
        <div className="relative flex-1">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-gray-400" />
          <input
            value={search}
            onChange={e => setSearch(e.target.value)}
            placeholder="搜索记忆内容..."
            className="w-full pl-9 pr-4 py-2 border rounded-lg bg-white dark:bg-gray-800 text-sm focus:ring-2 focus:ring-violet-500 focus:border-transparent"
          />
        </div>
        <select
          value={filterAgent}
          onChange={e => setFilterAgent(e.target.value)}
          className="px-3 py-2 border rounded-lg bg-white dark:bg-gray-800 text-sm min-w-[140px]"
        >
          <option value="">全部 Agent</option>
          {agents.map(a => (
            <option key={a.id} value={a.id}>{a.name}</option>
          ))}
        </select>
        {(filterCategory || filterAgent || search) && (
          <button
            onClick={() => { setFilterCategory(''); setFilterAgent(''); setSearch('') }}
            className="flex items-center gap-1 px-3 py-2 text-sm text-gray-500 hover:text-gray-700 hover:bg-gray-100 dark:hover:bg-gray-800 rounded-lg"
          >
            <Filter className="w-3.5 h-3.5" />
            清除筛选
          </button>
        )}
      </div>

      {/* Memory list */}
      {loading ? (
        <div className="flex items-center justify-center py-20">
          <div className="w-6 h-6 border-2 border-violet-500 border-t-transparent rounded-full animate-spin" />
        </div>
      ) : memories.length === 0 ? (
        <div className="text-center py-20">
          <Brain className="w-12 h-12 text-gray-300 mx-auto mb-3" />
          <p className="text-gray-500">暂无记忆</p>
          <p className="text-sm text-gray-400 mt-1">与 Claw 对话后会自动提取和学习记忆</p>
        </div>
      ) : (
        <div className="space-y-3">
          {memories.map(mem => {
            const cfg = categoryConfig[mem.category] || categoryConfig.context
            const Icon = cfg.icon
            const isEditing = editingId === mem.id

            return (
              <div key={mem.id} className="bg-white dark:bg-gray-800 rounded-xl border hover:border-violet-300 transition-colors p-4">
                <div className="flex items-start gap-3">
                  <div className={`w-8 h-8 rounded-lg ${cfg.bg} flex items-center justify-center flex-shrink-0 mt-0.5`}>
                    <Icon className={`w-4 h-4 ${cfg.color}`} />
                  </div>
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2 mb-1">
                      <span className="text-sm font-semibold text-gray-700 dark:text-gray-200 font-mono">{mem.key}</span>
                      <span className={`px-1.5 py-0.5 text-[10px] font-medium rounded ${cfg.bg} ${cfg.color}`}>
                        {cfg.label}
                      </span>
                      {mem.source === 'auto_extract' && (
                        <span className="px-1.5 py-0.5 text-[10px] font-medium rounded bg-gray-100 dark:bg-gray-700 text-gray-500">自动</span>
                      )}
                      {mem.source === 'user_explicit' && (
                        <span className="px-1.5 py-0.5 text-[10px] font-medium rounded bg-green-50 dark:bg-green-900/20 text-green-600">手动</span>
                      )}
                    </div>

                    {isEditing ? (
                      <div className="space-y-2 mt-2">
                        <textarea
                          value={editContent}
                          onChange={e => setEditContent(e.target.value)}
                          rows={2}
                          className="w-full px-3 py-2 border rounded-lg text-sm bg-gray-50 dark:bg-gray-900 focus:ring-2 focus:ring-violet-500"
                        />
                        <div className="flex items-center gap-3">
                          <label className="text-xs text-gray-500">重要度:</label>
                          <input
                            type="range"
                            min="0" max="1" step="0.1"
                            value={editImportance}
                            onChange={e => setEditImportance(parseFloat(e.target.value))}
                            className="flex-1 accent-violet-500"
                          />
                          <span className="text-xs text-gray-500 w-8">{editImportance.toFixed(1)}</span>
                          <button onClick={() => handleUpdate(mem.id)} className="px-3 py-1 bg-violet-600 text-white text-xs rounded-lg hover:bg-violet-700">保存</button>
                          <button onClick={() => setEditingId(null)} className="px-3 py-1 text-gray-500 text-xs hover:bg-gray-100 dark:hover:bg-gray-700 rounded-lg">取消</button>
                        </div>
                      </div>
                    ) : (
                      <p className="text-sm text-gray-600 dark:text-gray-300 leading-relaxed">{mem.content}</p>
                    )}

                    <div className="flex items-center gap-4 mt-2 text-[11px] text-gray-400">
                      {renderImportanceStars(mem.importance)}
                      <span className="flex items-center gap-1">
                        <Bot className="w-3 h-3" />
                        {getAgentName(mem.agent_id)}
                      </span>
                      <span>访问 {mem.access_count} 次</span>
                      <span>{formatTime(mem.updated_at)}</span>
                    </div>
                  </div>

                  {!isEditing && (
                    <div className="flex items-center gap-1 flex-shrink-0">
                      <button
                        onClick={() => { setEditingId(mem.id); setEditContent(mem.content); setEditImportance(mem.importance) }}
                        className="p-1.5 text-gray-400 hover:text-violet-600 hover:bg-violet-50 dark:hover:bg-violet-900/20 rounded-lg transition-colors"
                        title="编辑"
                      >
                        <Pencil className="w-3.5 h-3.5" />
                      </button>
                      <button
                        onClick={() => handleDelete(mem.id)}
                        className="p-1.5 text-gray-400 hover:text-red-600 hover:bg-red-50 dark:hover:bg-red-900/20 rounded-lg transition-colors"
                        title="删除"
                      >
                        <Trash2 className="w-3.5 h-3.5" />
                      </button>
                    </div>
                  )}
                </div>
              </div>
            )
          })}
        </div>
      )}

      {/* Create modal */}
      {showCreate && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50" onClick={() => setShowCreate(false)}>
          <div className="bg-white dark:bg-gray-800 rounded-2xl shadow-2xl w-full max-w-lg mx-4 p-6" onClick={e => e.stopPropagation()}>
            <div className="flex items-center justify-between mb-4">
              <h3 className="text-lg font-semibold text-gray-800 dark:text-gray-100">新建记忆</h3>
              <button onClick={() => setShowCreate(false)} className="p-1 text-gray-400 hover:text-gray-600 rounded">
                <X className="w-5 h-5" />
              </button>
            </div>
            <div className="space-y-4">
              <div>
                <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Agent</label>
                <select
                  value={createForm.agent_id}
                  onChange={e => setCreateForm({ ...createForm, agent_id: e.target.value })}
                  className="w-full px-3 py-2 border rounded-lg bg-white dark:bg-gray-900 text-sm"
                >
                  <option value="">选择 Agent...</option>
                  {agents.map(a => <option key={a.id} value={a.id}>{a.name}</option>)}
                </select>
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Key (英文标识)</label>
                <input
                  value={createForm.key}
                  onChange={e => setCreateForm({ ...createForm, key: e.target.value })}
                  placeholder="如: preferred_language, project_name"
                  className="w-full px-3 py-2 border rounded-lg bg-white dark:bg-gray-900 text-sm"
                />
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">内容</label>
                <textarea
                  value={createForm.content}
                  onChange={e => setCreateForm({ ...createForm, content: e.target.value })}
                  placeholder="记忆内容..."
                  rows={3}
                  className="w-full px-3 py-2 border rounded-lg bg-white dark:bg-gray-900 text-sm"
                />
              </div>
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">分类</label>
                  <select
                    value={createForm.category}
                    onChange={e => setCreateForm({ ...createForm, category: e.target.value })}
                    className="w-full px-3 py-2 border rounded-lg bg-white dark:bg-gray-900 text-sm"
                  >
                    {categories.map(cat => (
                      <option key={cat} value={cat}>{categoryConfig[cat].label}</option>
                    ))}
                  </select>
                </div>
                <div>
                  <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">重要度: {createForm.importance.toFixed(1)}</label>
                  <input
                    type="range"
                    min="0" max="1" step="0.1"
                    value={createForm.importance}
                    onChange={e => setCreateForm({ ...createForm, importance: parseFloat(e.target.value) })}
                    className="w-full accent-violet-500 mt-2"
                  />
                </div>
              </div>
            </div>
            <div className="flex justify-end gap-2 mt-6">
              <button onClick={() => setShowCreate(false)} className="px-4 py-2 text-sm text-gray-600 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-lg">取消</button>
              <button
                onClick={handleCreate}
                disabled={!createForm.agent_id || !createForm.key || !createForm.content}
                className="px-4 py-2 text-sm bg-violet-600 text-white rounded-lg hover:bg-violet-700 disabled:opacity-50 disabled:cursor-not-allowed"
              >
                创建
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
