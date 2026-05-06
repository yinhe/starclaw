import { useState, useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { GitBranch, Trash2, Clock, History, CheckCircle, XCircle, Loader2, Webhook, Timer, Copy, Check, MessageSquare, Tag, Plus } from 'lucide-react'
import { workflowAPI, scheduleAPI } from '../lib/api'

interface Schedule {
  id: string
  workflow_id: string
  cron_expr: string
  input: string
  enabled: boolean
}

interface WorkflowRun {
  id: string
  status: string
  input: string
  output: string
  error: string
  duration_ms: number
  created_at: string
}

interface Workflow {
  id: string
  name: string
  description: string
  category?: string
  conversation_id?: string
  webhook_token?: string
  updated_at: string
}

const CATEGORY_PRESETS = [
  { key: 'marketing', label: '广告宣传', color: 'indigo', icon: '📺' },
  { key: 'creative', label: '创意制作', color: 'purple', icon: '🎬' },
  { key: 'content', label: '内容运营', color: 'pink', icon: '📱' },
  { key: 'coding', label: '编程开发', color: 'emerald', icon: '�' },
  { key: 'finance', label: '金融财务', color: 'amber', icon: '💰' },
  { key: 'research', label: '调研分析', color: 'cyan', icon: '�' },
  { key: 'assistant', label: '通用助手', color: 'gray', icon: '🤖' },
] as const

const getCategoryLabel = (key: string) => CATEGORY_PRESETS.find(c => c.key === key)?.label || key
const getCategoryColor = (key: string) => {
  const colors: Record<string, string> = {
    marketing: 'bg-indigo-100 text-indigo-700 dark:bg-indigo-900/30 dark:text-indigo-300',
    creative: 'bg-purple-100 text-purple-700 dark:bg-purple-900/30 dark:text-purple-300',
    content: 'bg-pink-100 text-pink-700 dark:bg-pink-900/30 dark:text-pink-300',
    coding: 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300',
    finance: 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300',
    research: 'bg-cyan-100 text-cyan-700 dark:bg-cyan-900/30 dark:text-cyan-300',
    assistant: 'bg-gray-100 text-gray-600 dark:bg-gray-700 dark:text-gray-300',
  }
  return colors[key] || 'bg-gray-100 text-gray-600 dark:bg-gray-700 dark:text-gray-300'
}
const getCategoryIcon = (key: string) => CATEGORY_PRESETS.find(c => c.key === key)?.icon || '📁'

export default function WorkflowListPage() {
  const [workflows, setWorkflows] = useState<Workflow[]>([])
  const [allWorkflows, setAllWorkflows] = useState<Workflow[]>([])
  const [categories, setCategories] = useState<string[]>([])
  const [activeCategory, setActiveCategory] = useState<string>('')
  const [categoryDropdown, setCategoryDropdown] = useState<string | null>(null)
  const [selectedRuns, setSelectedRuns] = useState<{ wfId: string; runs: WorkflowRun[] } | null>(null)
  const [webhookPanel, setWebhookPanel] = useState<string | null>(null)
  const [copiedToken, setCopiedToken] = useState(false)
  const [schedules, setSchedules] = useState<Schedule[]>([])
  const [showScheduleModal, setShowScheduleModal] = useState<string | null>(null)
  const [cronForm, setCronForm] = useState({ cron_expr: '0 9 * * *', input: '' })
  const navigate = useNavigate()

  const loadWorkflows = async () => {
    try {
      const res = await workflowAPI.list(activeCategory || undefined)
      setWorkflows(res.data.workflows || [])
      setCategories(res.data.categories || [])
    } catch { /* ignore */ }
  }

  const loadAllWorkflows = async () => {
    try {
      const res = await workflowAPI.list()
      setAllWorkflows(res.data.workflows || [])
    } catch { /* ignore */ }
  }

  useEffect(() => {
    loadAllWorkflows()
  }, [])

  useEffect(() => {
    loadWorkflows()
  }, [activeCategory])

  const handleSetCategory = async (wfId: string, category: string) => {
    try {
      await workflowAPI.update(wfId, { category })
      setCategoryDropdown(null)
      loadWorkflows()
      loadAllWorkflows()
    } catch { /* ignore */ }
  }

  const handleDelete = async (id: string) => {
    if (!confirm('确定要删除这个工作流吗？')) return
    try {
      await workflowAPI.delete(id)
      loadWorkflows()
      loadAllWorkflows()
    } catch { /* ignore */ }
  }

  const createWorkflow = async () => {
    try {
      const defaultDef = JSON.stringify({
        nodes: [
          { id: 'start-1', type: 'start', position: { x: 250, y: 50 }, data: { label: '开始' } },
          { id: 'end-1', type: 'end', position: { x: 250, y: 400 }, data: { label: '结束' } },
        ],
        edges: [],
      })
      const res = await workflowAPI.create({
        name: '新建工作流',
        description: '',
        category: activeCategory || '',
        definition: defaultDef,
      })
      const newId = res.data?.workflow?.id
      if (newId) navigate(`/workflows/editor?id=${newId}`)
    } catch { /* ignore */ }
  }

  const loadSchedules = async () => {
    try {
      const res = await scheduleAPI.list()
      setSchedules(res.data.schedules || [])
    } catch { /* ignore */ }
  }

  useEffect(() => { loadSchedules() }, [])

  const handleEnableWebhook = async (e: React.MouseEvent, wfId: string) => {
    e.stopPropagation()
    try {
      const res = await workflowAPI.enableWebhook(wfId)
      setWebhookPanel(wfId)
      loadWorkflows()
      navigator.clipboard.writeText(window.location.origin + res.data.webhook_url)
      setCopiedToken(true)
      setTimeout(() => setCopiedToken(false), 2000)
    } catch { /* ignore */ }
  }

  const handleDisableWebhook = async (e: React.MouseEvent, wfId: string) => {
    e.stopPropagation()
    try {
      await workflowAPI.disableWebhook(wfId)
      setWebhookPanel(null)
      loadWorkflows()
    } catch { /* ignore */ }
  }

  const handleCreateSchedule = async (wfId: string) => {
    try {
      await scheduleAPI.create({ workflow_id: wfId, cron_expr: cronForm.cron_expr, input: cronForm.input })
      setShowScheduleModal(null)
      setCronForm({ cron_expr: '0 9 * * *', input: '' })
      loadSchedules()
    } catch { /* ignore */ }
  }

  const handleToggleSchedule = async (id: string) => {
    try {
      await scheduleAPI.toggle(id)
      loadSchedules()
    } catch { /* ignore */ }
  }

  const handleDeleteSchedule = async (id: string) => {
    try {
      await scheduleAPI.delete(id)
      loadSchedules()
    } catch { /* ignore */ }
  }

  const handleViewRuns = async (e: React.MouseEvent, wfId: string) => {
    e.stopPropagation()
    if (selectedRuns?.wfId === wfId) {
      setSelectedRuns(null)
      return
    }
    try {
      const res = await workflowAPI.listRuns(wfId)
      setSelectedRuns({ wfId, runs: res.data.runs || [] })
    } catch { /* ignore */ }
  }

  const formatDate = (d: string) => {
    try {
      return new Date(d).toLocaleString('zh-CN', { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' })
    } catch { return d }
  }

  const statusIcon = (s: string) => {
    if (s === 'success') return <CheckCircle className="w-3.5 h-3.5 text-green-500" />
    if (s === 'error') return <XCircle className="w-3.5 h-3.5 text-red-500" />
    return <Loader2 className="w-3.5 h-3.5 text-yellow-500 animate-spin" />
  }

  const workflowCounts = allWorkflows.reduce<Record<string, number>>((acc, wf) => {
    const cat = wf.category || '_uncategorized'
    acc[cat] = (acc[cat] || 0) + 1
    return acc
  }, {})

  return (
    <div className="h-full flex">
      {/* Left sidebar */}
      <div className="w-56 flex-shrink-0 border-r border-gray-200 dark:border-gray-700 bg-gray-50/50 dark:bg-gray-900/50 overflow-y-auto">
        <div className="p-4">
          <h2 className="text-xs font-semibold text-gray-400 uppercase tracking-wider mb-3">分类</h2>
          <div className="space-y-0.5">
            <button
              onClick={() => setActiveCategory('')}
              className={`w-full flex items-center gap-2.5 px-3 py-2 rounded-lg text-sm transition-colors ${
                activeCategory === '' ? 'bg-gray-900 text-white dark:bg-gray-100 dark:text-gray-900 font-medium' : 'text-gray-600 hover:bg-gray-200/70 dark:text-gray-400 dark:hover:bg-gray-800'
              }`}
            >
              <span className="text-base">📋</span>
              <span className="flex-1 text-left">全部</span>
              <span className={`text-xs tabular-nums ${activeCategory === '' ? 'text-gray-300 dark:text-gray-600' : 'text-gray-400'}`}>{allWorkflows.length}</span>
            </button>
            {CATEGORY_PRESETS.map((cat) => {
              const count = workflowCounts[cat.key] || 0
              if (count === 0 && !categories.includes(cat.key)) return null
              return (
                <button
                  key={cat.key}
                  onClick={() => setActiveCategory(activeCategory === cat.key ? '' : cat.key)}
                  className={`w-full flex items-center gap-2.5 px-3 py-2 rounded-lg text-sm transition-colors ${
                    activeCategory === cat.key ? 'bg-gray-900 text-white dark:bg-gray-100 dark:text-gray-900 font-medium' : 'text-gray-600 hover:bg-gray-200/70 dark:text-gray-400 dark:hover:bg-gray-800'
                  }`}
                >
                  <span className="text-base">{cat.icon}</span>
                  <span className="flex-1 text-left">{cat.label}</span>
                  <span className={`text-xs tabular-nums ${activeCategory === cat.key ? 'text-gray-300 dark:text-gray-600' : 'text-gray-400'}`}>{count}</span>
                </button>
              )
            })}
            {categories.filter(c => !CATEGORY_PRESETS.some(p => p.key === c)).map((cat) => (
              <button
                key={cat}
                onClick={() => setActiveCategory(activeCategory === cat ? '' : cat)}
                className={`w-full flex items-center gap-2.5 px-3 py-2 rounded-lg text-sm transition-colors ${
                  activeCategory === cat ? 'bg-gray-900 text-white dark:bg-gray-100 dark:text-gray-900 font-medium' : 'text-gray-600 hover:bg-gray-200/70 dark:text-gray-400 dark:hover:bg-gray-800'
                }`}
              >
                <span className="text-base">📁</span>
                <span className="flex-1 text-left">{cat}</span>
                <span className={`text-xs tabular-nums ${activeCategory === cat ? 'text-gray-300 dark:text-gray-600' : 'text-gray-400'}`}>{workflowCounts[cat] || 0}</span>
              </button>
            ))}
          </div>
        </div>
      </div>

      {/* Right content */}
      <div className="flex-1 overflow-y-auto">
        <div className="max-w-5xl mx-auto p-8">
          <div className="flex items-center justify-between mb-6">
            <div>
              <h1 className="text-2xl font-bold text-gray-900 dark:text-gray-100">
                {activeCategory ? (getCategoryIcon(activeCategory) + ' ' + getCategoryLabel(activeCategory)) : '工作流'}
              </h1>
              <p className="text-gray-500 text-sm mt-1">
                {activeCategory ? `${workflows.length} 个工作流` : '智能体绑定的多步自动化管道'}
              </p>
            </div>
            <button
              onClick={createWorkflow}
              className="flex items-center gap-2 px-4 py-2 bg-primary-600 hover:bg-primary-700 text-white rounded-lg text-sm font-medium transition-colors shadow-sm"
            >
              <Plus className="w-4 h-4" />
              新建工作流
            </button>
          </div>

          {workflows.length === 0 ? (
            <div className="text-center py-20 text-gray-400">
              <GitBranch className="w-12 h-12 mx-auto mb-3 opacity-50" />
              <p>还没有工作流</p>
              <p className="text-xs mt-1">工作流跟随智能体安装，或在智能体详情页中创建</p>
            </div>
          ) : (
            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
            {workflows.map((wf) => (
              <div key={wf.id} className="space-y-0">
                <div
                  className="bg-white dark:bg-gray-800 border rounded-xl p-5 hover:shadow-md transition-shadow cursor-pointer group"
                  onClick={() => navigate(`/workflows/editor?id=${wf.id}`)}
                >
                  <div className="flex items-start justify-between mb-3">
                    <div className="w-10 h-10 bg-purple-100 dark:bg-purple-900/30 rounded-lg flex items-center justify-center">
                      <GitBranch className="w-5 h-5 text-purple-600 dark:text-purple-400" />
                    </div>
                    <div className="flex items-center gap-1">
                      <button
                        onClick={(e) => handleViewRuns(e, wf.id)}
                        className="p-1.5 text-gray-400 hover:text-primary-600 rounded-md hover:bg-primary-50 dark:hover:bg-primary-900/30 opacity-0 group-hover:opacity-100 transition-opacity"
                        title="执行历史"
                      >
                        <History className="w-3.5 h-3.5" />
                      </button>
                      <button
                        onClick={(e) => { e.stopPropagation(); setWebhookPanel(webhookPanel === wf.id ? null : wf.id) }}
                        className={`p-1.5 rounded-md opacity-0 group-hover:opacity-100 transition-opacity ${wf.webhook_token ? 'text-green-500' : 'text-gray-400 hover:text-blue-500 hover:bg-blue-50 dark:hover:bg-blue-900/30'}`}
                        title="Webhook"
                      >
                        <Webhook className="w-3.5 h-3.5" />
                      </button>
                      <button
                        onClick={(e) => { e.stopPropagation(); setShowScheduleModal(showScheduleModal === wf.id ? null : wf.id) }}
                        className="p-1.5 text-gray-400 hover:text-amber-500 rounded-md hover:bg-amber-50 dark:hover:bg-amber-900/30 opacity-0 group-hover:opacity-100 transition-opacity"
                        title="定时任务"
                      >
                        <Timer className="w-3.5 h-3.5" />
                      </button>
                      <button
                        onClick={(e) => { e.stopPropagation(); handleDelete(wf.id) }}
                        className="p-1.5 text-gray-400 hover:text-red-500 rounded-md hover:bg-red-50 dark:hover:bg-red-900/30 opacity-0 group-hover:opacity-100 transition-opacity"
                      >
                        <Trash2 className="w-3.5 h-3.5" />
                      </button>
                    </div>
                  </div>
                  <div className="flex items-center gap-2">
                    <h3 className="font-semibold text-gray-900 dark:text-gray-100 flex-1 truncate">{wf.name}</h3>
                    <div className="relative">
                      {wf.category ? (
                        <button
                          onClick={(e) => { e.stopPropagation(); setCategoryDropdown(categoryDropdown === wf.id ? null : wf.id) }}
                          className={`px-1.5 py-0.5 rounded text-[10px] font-medium ${getCategoryColor(wf.category)}`}
                        >
                          {getCategoryLabel(wf.category)}
                        </button>
                      ) : (
                        <button
                          onClick={(e) => { e.stopPropagation(); setCategoryDropdown(categoryDropdown === wf.id ? null : wf.id) }}
                          className="p-1 text-gray-300 hover:text-gray-500 rounded opacity-0 group-hover:opacity-100 transition-opacity"
                          title="设置分类"
                        >
                          <Tag className="w-3 h-3" />
                        </button>
                      )}
                      {categoryDropdown === wf.id && (
                        <div className="absolute right-0 top-full mt-1 bg-white dark:bg-gray-800 border rounded-lg shadow-lg py-1 z-10 min-w-[100px]" onClick={(e) => e.stopPropagation()}>
                          {CATEGORY_PRESETS.map((cat) => (
                            <button
                              key={cat.key}
                              onClick={() => handleSetCategory(wf.id, cat.key)}
                              className={`block w-full text-left px-3 py-1.5 text-xs hover:bg-gray-50 dark:hover:bg-gray-700 ${wf.category === cat.key ? 'font-bold' : ''}`}
                            >
                              {cat.label}
                            </button>
                          ))}
                          {wf.category && (
                            <button
                              onClick={() => handleSetCategory(wf.id, '')}
                              className="block w-full text-left px-3 py-1.5 text-xs text-red-500 hover:bg-red-50 dark:hover:bg-red-900/20 border-t"
                            >
                              清除分类
                            </button>
                          )}
                        </div>
                      )}
                    </div>
                  </div>
                  <p className="text-sm text-gray-500 mt-1 line-clamp-2">
                    {wf.description || '暂无描述'}
                  </p>
                  <div className="flex items-center gap-2 mt-3 text-xs text-gray-400">
                    <span className="flex items-center gap-1"><Clock className="w-3 h-3" />{formatDate(wf.updated_at)}</span>
                    {wf.conversation_id && (
                      <button
                        onClick={(e) => { e.stopPropagation(); navigate(`/chat/${wf.conversation_id}`) }}
                        className="flex items-center gap-0.5 px-1.5 py-0.5 bg-blue-50 dark:bg-blue-900/30 text-blue-500 rounded-full text-[10px] hover:bg-blue-100 dark:hover:bg-blue-900/50 transition-colors"
                        title="跳转到来源对话"
                      >
                        <MessageSquare className="w-2.5 h-2.5" />对话创建
                      </button>
                    )}
                  </div>
                </div>

                {/* Run history panel */}
                {selectedRuns?.wfId === wf.id && (
                  <div className="bg-gray-50 dark:bg-gray-800/50 border border-t-0 rounded-b-xl px-4 py-3 -mt-1">
                    <p className="text-xs font-medium text-gray-500 mb-2">执行历史</p>
                    {selectedRuns.runs.length === 0 ? (
                      <p className="text-xs text-gray-400">暂无执行记录</p>
                    ) : (
                      <div className="space-y-1.5 max-h-48 overflow-y-auto">
                        {selectedRuns.runs.slice(0, 10).map((run) => (
                          <div key={run.id} className="flex items-center justify-between text-xs">
                            <div className="flex items-center gap-1.5">
                              {statusIcon(run.status)}
                              <span className="text-gray-600 dark:text-gray-300 truncate max-w-[120px]">
                                {run.input || '(无输入)'}
                              </span>
                            </div>
                            <div className="flex items-center gap-2 text-gray-400">
                              {run.duration_ms > 0 && <span>{(run.duration_ms / 1000).toFixed(1)}s</span>}
                              <span>{formatDate(run.created_at)}</span>
                            </div>
                          </div>
                        ))}
                      </div>
                    )}
                  </div>
                )}

                {/* Webhook panel */}
                {webhookPanel === wf.id && (
                  <div className="bg-blue-50 dark:bg-blue-900/20 border border-t-0 rounded-b-xl px-4 py-3 -mt-1" onClick={(e) => e.stopPropagation()}>
                    <p className="text-xs font-medium text-gray-600 dark:text-gray-300 mb-2">Webhook 触发</p>
                    {wf.webhook_token ? (
                      <div className="space-y-2">
                        <div className="flex items-center gap-2">
                          <code className="flex-1 text-xs bg-white dark:bg-gray-800 px-2 py-1.5 rounded border truncate">
                            POST /v1/webhooks/workflow/{wf.webhook_token.slice(0, 12)}...
                          </code>
                          <button
                            onClick={() => {
                              navigator.clipboard.writeText(window.location.origin + '/v1/webhooks/workflow/' + wf.webhook_token)
                              setCopiedToken(true)
                              setTimeout(() => setCopiedToken(false), 2000)
                            }}
                            className="p-1.5 text-xs text-blue-600 hover:bg-blue-100 rounded"
                          >
                            {copiedToken ? <Check className="w-3.5 h-3.5" /> : <Copy className="w-3.5 h-3.5" />}
                          </button>
                        </div>
                        <button onClick={(e) => handleDisableWebhook(e, wf.id)} className="text-xs text-red-500 hover:underline">
                          禁用 Webhook
                        </button>
                      </div>
                    ) : (
                      <button onClick={(e) => handleEnableWebhook(e, wf.id)} className="text-xs px-3 py-1.5 bg-blue-600 text-white rounded-lg hover:bg-blue-700">
                        启用 Webhook
                      </button>
                    )}
                  </div>
                )}

                {/* Schedule panel */}
                {showScheduleModal === wf.id && (
                  <div className="bg-amber-50 dark:bg-amber-900/20 border border-t-0 rounded-b-xl px-4 py-3 -mt-1" onClick={(e) => e.stopPropagation()}>
                    <p className="text-xs font-medium text-gray-600 dark:text-gray-300 mb-2">定时任务</p>
                    {schedules.filter((s) => s.workflow_id === wf.id).map((s) => (
                      <div key={s.id} className="flex items-center justify-between text-xs mb-1.5">
                        <div className="flex items-center gap-2">
                          <code className="bg-white dark:bg-gray-800 px-1.5 py-0.5 rounded border">{s.cron_expr}</code>
                          <span className={s.enabled ? 'text-green-600' : 'text-gray-400'}>{s.enabled ? '运行中' : '已暂停'}</span>
                        </div>
                        <div className="flex items-center gap-1">
                          <button onClick={() => handleToggleSchedule(s.id)} className="text-blue-500 hover:underline">{s.enabled ? '暂停' : '恢复'}</button>
                          <button onClick={() => handleDeleteSchedule(s.id)} className="text-red-500 hover:underline ml-1">删除</button>
                        </div>
                      </div>
                    ))}
                    <div className="flex items-center gap-2 mt-2">
                      <input
                        value={cronForm.cron_expr}
                        onChange={(e) => setCronForm({ ...cronForm, cron_expr: e.target.value })}
                        placeholder="Cron 表达式"
                        className="flex-1 px-2 py-1 border rounded text-xs bg-white dark:bg-gray-800 dark:text-gray-200 outline-none"
                      />
                      <button onClick={() => handleCreateSchedule(wf.id)} className="px-2 py-1 bg-amber-600 text-white rounded text-xs hover:bg-amber-700">
                        添加
                      </button>
                    </div>
                    <p className="text-xs text-gray-400 mt-1">示例: 0 9 * * * (每天9点), */30 * * * * (每30分钟)</p>
                  </div>
                )}
              </div>
            ))}
          </div>
        )}
        </div>
      </div>
    </div>
  )
}
