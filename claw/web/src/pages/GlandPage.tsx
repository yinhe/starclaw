import { useState, useEffect } from 'react'
import { Droplets, Plus, Trash2, Eye, EyeOff, Save, Search, Bot, Lock, Gauge, ToggleLeft, Globe, Settings2 } from 'lucide-react'
import { glandAPI, agentAPI } from '../lib/api'

interface Gland {
  id: string
  agent_id: string
  key: string
  value: string
  category: string
  encrypted: boolean
  required: boolean
  label: string
  help_text: string
  sort_order: number
}

interface Agent {
  id: string
  name: string
}

const CATEGORY_META: Record<string, { label: string; icon: React.ComponentType<{ className?: string }>; color: string }> = {
  credential: { label: '凭证', icon: Lock, color: 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400' },
  threshold:  { label: '阈值', icon: Gauge, color: 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400' },
  toggle:     { label: '开关', icon: ToggleLeft, color: 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400' },
  endpoint:   { label: '端点', icon: Globe, color: 'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400' },
  general:    { label: '通用', icon: Settings2, color: 'bg-gray-100 text-gray-700 dark:bg-gray-800 dark:text-gray-400' },
}

export default function GlandPage() {
  const [glands, setGlands] = useState<Gland[]>([])
  const [agents, setAgents] = useState<Agent[]>([])
  const [loading, setLoading] = useState(true)
  const [selectedAgent, setSelectedAgent] = useState('')
  const [search, setSearch] = useState('')
  const [showAdd, setShowAdd] = useState(false)
  const [editingId, setEditingId] = useState<string | null>(null)
  const [editValue, setEditValue] = useState('')
  const [revealedIds, setRevealedIds] = useState<Set<string>>(new Set())
  const [revealedValues, setRevealedValues] = useState<Record<string, string>>({})

  // New gland form
  const [form, setForm] = useState({ agent_id: '', key: '', value: '', category: 'general', encrypted: false, required: false, label: '', help_text: '' })

  useEffect(() => {
    agentAPI.list().then(r => setAgents(r.data.agents || [])).catch(() => {})
  }, [])

  useEffect(() => { load() }, [selectedAgent])

  const load = async () => {
    setLoading(true)
    try {
      const res = await glandAPI.list(selectedAgent || undefined)
      setGlands(res.data.glands || [])
    } catch {}
    setLoading(false)
  }

  const handleCreate = async () => {
    if (!form.agent_id || !form.key) return
    try {
      await glandAPI.create(form)
      setShowAdd(false)
      setForm({ agent_id: selectedAgent, key: '', value: '', category: 'general', encrypted: false, required: false, label: '', help_text: '' })
      load()
    } catch {}
  }

  const handleDelete = async (id: string) => {
    if (!confirm('确定删除此配置项？')) return
    try {
      await glandAPI.delete(id)
      load()
    } catch {}
  }

  const handleSaveEdit = async (id: string) => {
    try {
      await glandAPI.update(id, { value: editValue })
      setEditingId(null)
      setEditValue('')
      load()
    } catch {}
  }

  const handleReveal = async (g: Gland) => {
    if (revealedIds.has(g.id)) {
      setRevealedIds(prev => { const n = new Set(prev); n.delete(g.id); return n })
      return
    }
    try {
      const res = await glandAPI.decrypt(g.agent_id, g.key)
      setRevealedValues(prev => ({ ...prev, [g.id]: res.data.value }))
      setRevealedIds(prev => new Set(prev).add(g.id))
    } catch {}
  }

  const getAgentName = (agentId: string) => agents.find(a => a.id === agentId)?.name || agentId.slice(0, 8)

  const filtered = glands.filter(g => {
    if (search) {
      const q = search.toLowerCase()
      if (!g.key.toLowerCase().includes(q) && !g.label.toLowerCase().includes(q) && !g.category.toLowerCase().includes(q)) return false
    }
    return true
  })

  // Group by agent
  const grouped = filtered.reduce<Record<string, Gland[]>>((acc, g) => {
    const key = g.agent_id
    if (!acc[key]) acc[key] = []
    acc[key].push(g)
    return acc
  }, {})

  const totalCount = glands.length
  const credentialCount = glands.filter(g => g.category === 'credential').length
  const requiredEmpty = glands.filter(g => g.required && (!g.value || g.value === '••••' || g.value === '••••••••')).length

  return (
    <div className="h-full overflow-y-auto bg-gray-50 dark:bg-gray-900">
      <div className="max-w-5xl mx-auto px-6 py-8">
        {/* Header */}
        <div className="flex items-center justify-between mb-6">
          <div className="flex items-center gap-3">
            <div className="p-2 bg-emerald-100 dark:bg-emerald-900/30 rounded-lg">
              <Droplets className="w-6 h-6 text-emerald-600 dark:text-emerald-400" />
            </div>
            <div>
              <h1 className="text-2xl font-bold text-gray-900 dark:text-white">腺体</h1>
              <p className="text-sm text-gray-500 dark:text-gray-400">智能体运行配置 — 凭证、参数、开关</p>
            </div>
          </div>
          <button onClick={() => { setShowAdd(true); setForm(f => ({ ...f, agent_id: selectedAgent })) }} className="flex items-center gap-2 px-4 py-2 bg-emerald-600 text-white rounded-lg hover:bg-emerald-700 transition-colors">
            <Plus className="w-4 h-4" /> 添加配置
          </button>
        </div>

        {/* Stats */}
        <div className="grid grid-cols-3 gap-4 mb-6">
          <div className="bg-white dark:bg-gray-800 rounded-lg p-4 border border-gray-200 dark:border-gray-700">
            <div className="text-2xl font-bold text-gray-900 dark:text-white">{totalCount}</div>
            <div className="text-sm text-gray-500">配置总数</div>
          </div>
          <div className="bg-white dark:bg-gray-800 rounded-lg p-4 border border-gray-200 dark:border-gray-700">
            <div className="text-2xl font-bold text-red-600">{credentialCount}</div>
            <div className="text-sm text-gray-500">加密凭证</div>
          </div>
          <div className="bg-white dark:bg-gray-800 rounded-lg p-4 border border-gray-200 dark:border-gray-700">
            <div className="text-2xl font-bold text-amber-600">{requiredEmpty}</div>
            <div className="text-sm text-gray-500">待填必填项</div>
          </div>
        </div>

        {/* Filters */}
        <div className="flex gap-3 mb-6">
          <div className="relative flex-1">
            <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-gray-400" />
            <input value={search} onChange={e => setSearch(e.target.value)} placeholder="搜索配置项..." className="w-full pl-10 pr-4 py-2 bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-emerald-500 text-gray-900 dark:text-gray-100" />
          </div>
          <select value={selectedAgent} onChange={e => setSelectedAgent(e.target.value)} className="px-3 py-2 bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg text-sm text-gray-700 dark:text-gray-300">
            <option value="">全部智能体</option>
            {agents.map(a => <option key={a.id} value={a.id}>{a.name}</option>)}
          </select>
        </div>

        {/* Add form */}
        {showAdd && (
          <div className="bg-white dark:bg-gray-800 rounded-lg border border-emerald-200 dark:border-emerald-800 p-5 mb-6">
            <h3 className="text-sm font-semibold text-gray-900 dark:text-white mb-4">添加配置项</h3>
            <div className="grid grid-cols-2 gap-4">
              <div>
                <label className="text-xs text-gray-500 mb-1 block">智能体 *</label>
                <select value={form.agent_id} onChange={e => setForm(f => ({ ...f, agent_id: e.target.value }))} className="w-full px-3 py-2 bg-gray-50 dark:bg-gray-900 border border-gray-200 dark:border-gray-700 rounded-lg text-sm text-gray-900 dark:text-gray-100">
                  <option value="">选择智能体</option>
                  {agents.map(a => <option key={a.id} value={a.id}>{a.name}</option>)}
                </select>
              </div>
              <div>
                <label className="text-xs text-gray-500 mb-1 block">配置键 *</label>
                <input value={form.key} onChange={e => setForm(f => ({ ...f, key: e.target.value }))} placeholder="e.g. api_key" className="w-full px-3 py-2 bg-gray-50 dark:bg-gray-900 border border-gray-200 dark:border-gray-700 rounded-lg text-sm text-gray-900 dark:text-gray-100" />
              </div>
              <div>
                <label className="text-xs text-gray-500 mb-1 block">显示名</label>
                <input value={form.label} onChange={e => setForm(f => ({ ...f, label: e.target.value }))} placeholder="e.g. API 密钥" className="w-full px-3 py-2 bg-gray-50 dark:bg-gray-900 border border-gray-200 dark:border-gray-700 rounded-lg text-sm text-gray-900 dark:text-gray-100" />
              </div>
              <div>
                <label className="text-xs text-gray-500 mb-1 block">分类</label>
                <select value={form.category} onChange={e => setForm(f => ({ ...f, category: e.target.value }))} className="w-full px-3 py-2 bg-gray-50 dark:bg-gray-900 border border-gray-200 dark:border-gray-700 rounded-lg text-sm text-gray-900 dark:text-gray-100">
                  {Object.entries(CATEGORY_META).map(([k, v]) => <option key={k} value={k}>{v.label}</option>)}
                </select>
              </div>
              <div className="col-span-2">
                <label className="text-xs text-gray-500 mb-1 block">值</label>
                <input type={form.encrypted ? 'password' : 'text'} value={form.value} onChange={e => setForm(f => ({ ...f, value: e.target.value }))} placeholder="配置值" className="w-full px-3 py-2 bg-gray-50 dark:bg-gray-900 border border-gray-200 dark:border-gray-700 rounded-lg text-sm text-gray-900 dark:text-gray-100" />
              </div>
              <div>
                <label className="text-xs text-gray-500 mb-1 block">帮助文本</label>
                <input value={form.help_text} onChange={e => setForm(f => ({ ...f, help_text: e.target.value }))} placeholder="提示信息" className="w-full px-3 py-2 bg-gray-50 dark:bg-gray-900 border border-gray-200 dark:border-gray-700 rounded-lg text-sm text-gray-900 dark:text-gray-100" />
              </div>
              <div className="flex items-end gap-6">
                <label className="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300 cursor-pointer">
                  <input type="checkbox" checked={form.encrypted} onChange={e => setForm(f => ({ ...f, encrypted: e.target.checked }))} className="rounded" />
                  加密存储
                </label>
                <label className="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300 cursor-pointer">
                  <input type="checkbox" checked={form.required} onChange={e => setForm(f => ({ ...f, required: e.target.checked }))} className="rounded" />
                  必填
                </label>
              </div>
            </div>
            <div className="flex justify-end gap-3 mt-4">
              <button onClick={() => setShowAdd(false)} className="px-4 py-2 text-sm text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-200">取消</button>
              <button onClick={handleCreate} disabled={!form.agent_id || !form.key} className="px-4 py-2 text-sm bg-emerald-600 text-white rounded-lg hover:bg-emerald-700 disabled:opacity-50 disabled:cursor-not-allowed">创建</button>
            </div>
          </div>
        )}

        {/* Gland list grouped by agent */}
        {loading ? (
          <div className="text-center py-20 text-gray-400">加载中...</div>
        ) : Object.keys(grouped).length === 0 ? (
          <div className="text-center py-20">
            <Droplets className="w-12 h-12 mx-auto mb-4 text-gray-300 dark:text-gray-600" />
            <p className="text-gray-500 dark:text-gray-400">暂无配置项</p>
            <p className="text-sm text-gray-400 dark:text-gray-500 mt-1">从市场安装智能体后，可在此配置运行参数</p>
          </div>
        ) : (
          <div className="space-y-6">
            {Object.entries(grouped).map(([agentId, items]) => (
              <div key={agentId} className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 overflow-hidden">
                <div className="px-5 py-3 bg-gray-50 dark:bg-gray-800/80 border-b border-gray-200 dark:border-gray-700 flex items-center gap-2">
                  <Bot className="w-4 h-4 text-gray-400" />
                  <span className="font-medium text-sm text-gray-900 dark:text-white">{getAgentName(agentId)}</span>
                  <span className="text-xs text-gray-400 ml-auto">{items.length} 项配置</span>
                </div>
                <div className="divide-y divide-gray-100 dark:divide-gray-700/50">
                  {items.sort((a, b) => a.sort_order - b.sort_order).map(g => {
                    const meta = CATEGORY_META[g.category] || CATEGORY_META.general
                    const CatIcon = meta.icon
                    const isRevealed = revealedIds.has(g.id)
                    const isEditing = editingId === g.id

                    return (
                      <div key={g.id} className="px-5 py-3 flex items-center gap-4 hover:bg-gray-50 dark:hover:bg-gray-700/30 transition-colors">
                        {/* Category badge */}
                        <span className={`inline-flex items-center gap-1 px-2 py-0.5 rounded text-xs font-medium ${meta.color}`}>
                          <CatIcon className="w-3 h-3" />
                          {meta.label}
                        </span>

                        {/* Key + Label */}
                        <div className="min-w-0 flex-1">
                          <div className="flex items-center gap-2">
                            <code className="text-sm font-mono text-gray-900 dark:text-gray-100">{g.key}</code>
                            {g.required && <span className="text-[10px] text-red-500 font-semibold">必填</span>}
                            {g.encrypted && <Lock className="w-3 h-3 text-red-400" />}
                          </div>
                          {g.label && <div className="text-xs text-gray-400 mt-0.5">{g.label}{g.help_text ? ` — ${g.help_text}` : ''}</div>}
                        </div>

                        {/* Value */}
                        <div className="flex items-center gap-2 min-w-[200px] justify-end">
                          {isEditing ? (
                            <>
                              <input type={g.encrypted ? 'password' : 'text'} value={editValue} onChange={e => setEditValue(e.target.value)} className="w-48 px-2 py-1 text-sm bg-gray-50 dark:bg-gray-900 border border-gray-300 dark:border-gray-600 rounded text-gray-900 dark:text-gray-100" autoFocus />
                              <button onClick={() => handleSaveEdit(g.id)} className="p-1 text-emerald-600 hover:text-emerald-700"><Save className="w-4 h-4" /></button>
                              <button onClick={() => setEditingId(null)} className="p-1 text-gray-400 hover:text-gray-600 text-xs">取消</button>
                            </>
                          ) : (
                            <>
                              <span className="text-sm text-gray-500 dark:text-gray-400 font-mono truncate max-w-[180px]">
                                {g.encrypted ? (isRevealed ? (revealedValues[g.id] || '') : '••••••••') : (g.value || <span className="text-gray-300 italic">空</span>)}
                              </span>
                              {g.encrypted && (
                                <button onClick={() => handleReveal(g)} className="p-1 text-gray-400 hover:text-gray-600" title={isRevealed ? '隐藏' : '显示'}>
                                  {isRevealed ? <EyeOff className="w-3.5 h-3.5" /> : <Eye className="w-3.5 h-3.5" />}
                                </button>
                              )}
                              <button onClick={() => { setEditingId(g.id); setEditValue('') }} className="p-1 text-gray-400 hover:text-blue-600" title="编辑">
                                <Settings2 className="w-3.5 h-3.5" />
                              </button>
                              <button onClick={() => handleDelete(g.id)} className="p-1 text-gray-400 hover:text-red-600" title="删除">
                                <Trash2 className="w-3.5 h-3.5" />
                              </button>
                            </>
                          )}
                        </div>
                      </div>
                    )
                  })}
                </div>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  )
}
