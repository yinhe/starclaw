import { useState, useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { GitBranch, Trash2, Clock, History, CheckCircle, XCircle, Loader2, Webhook, Timer, Copy, Check, MessageSquare } from 'lucide-react'
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
  conversation_id?: string
  webhook_token?: string
  updated_at: string
}

export default function WorkflowListPage() {
  const [workflows, setWorkflows] = useState<Workflow[]>([])
  const [selectedRuns, setSelectedRuns] = useState<{ wfId: string; runs: WorkflowRun[] } | null>(null)
  const [webhookPanel, setWebhookPanel] = useState<string | null>(null)
  const [copiedToken, setCopiedToken] = useState(false)
  const [schedules, setSchedules] = useState<Schedule[]>([])
  const [showScheduleModal, setShowScheduleModal] = useState<string | null>(null)
  const [cronForm, setCronForm] = useState({ cron_expr: '0 9 * * *', input: '' })
  const navigate = useNavigate()

  useEffect(() => {
    loadWorkflows()
  }, [])

  const loadWorkflows = async () => {
    try {
      const res = await workflowAPI.list()
      setWorkflows(res.data.workflows || [])
    } catch { /* ignore */ }
  }

  const handleDelete = async (id: string) => {
    if (!confirm('确定要删除这个工作流吗？')) return
    try {
      await workflowAPI.delete(id)
      loadWorkflows()
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

  return (
    <div className="h-full overflow-y-auto">
      <div className="max-w-5xl mx-auto p-8">
        <div className="flex items-center justify-between mb-8">
          <div>
            <h1 className="text-2xl font-bold text-gray-900 dark:text-gray-100">工作流</h1>
            <p className="text-gray-500 text-sm mt-1">智能体绑定的多步自动化管道</p>
          </div>
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
                  <h3 className="font-semibold text-gray-900 dark:text-gray-100">{wf.name}</h3>
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
  )
}
