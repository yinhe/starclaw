import { useState, useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { ListTodo, Clock, CheckCircle2, XCircle, Loader2, PauseCircle, ChevronDown, ChevronRight, Plus, Ban, Pause, Play, MessageSquare } from 'lucide-react'
import { taskAPI } from '../lib/api'

interface Task {
  id: string
  title: string
  description: string
  goal: string
  status: string
  priority: string
  progress: number
  progress_note: string
  result: string
  error_msg: string
  agent_id: string
  conversation_id: string
  parent_task_id: string
  scheduled_at: string | null
  started_at: string | null
  completed_at: string | null
  created_at: string
}

const statusConfig: Record<string, { label: string; color: string; icon: typeof CheckCircle2 }> = {
  pending: { label: '排队中', color: 'text-yellow-600 bg-yellow-50 border-yellow-200', icon: Clock },
  running: { label: '执行中', color: 'text-blue-600 bg-blue-50 border-blue-200', icon: Loader2 },
  completed: { label: '已完成', color: 'text-green-600 bg-green-50 border-green-200', icon: CheckCircle2 },
  failed: { label: '失败', color: 'text-red-600 bg-red-50 border-red-200', icon: XCircle },
  cancelled: { label: '已取消', color: 'text-gray-500 bg-gray-50 border-gray-200', icon: Ban },
  waiting: { label: '等待执行', color: 'text-purple-600 bg-purple-50 border-purple-200', icon: PauseCircle },
}

const priorityColors: Record<string, string> = {
  urgent: 'text-red-700 bg-red-100',
  high: 'text-orange-700 bg-orange-100',
  normal: 'text-blue-700 bg-blue-100',
  low: 'text-gray-600 bg-gray-100',
}

export default function TasksPage() {
  const navigate = useNavigate()
  const [tasks, setTasks] = useState<Task[]>([])
  const [loading, setLoading] = useState(true)
  const [filter, setFilter] = useState<string>('')
  const [convFilter, setConvFilter] = useState<string>('')
  const [conversations, setConversations] = useState<{ conversation_id: string; title: string; count: number }[]>([])
  const [expandedId, setExpandedId] = useState<string | null>(null)
  const [showCreate, setShowCreate] = useState(false)
  const [newTask, setNewTask] = useState({ title: '', goal: '', priority: 'normal' })
  const [creating, setCreating] = useState(false)

  const loadTasks = async () => {
    try {
      const res = await taskAPI.list(filter || undefined, convFilter || undefined)
      setTasks(res.data.tasks || [])
      if (res.data.conversations) setConversations(res.data.conversations)
    } catch (e) {
      console.error(e)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    loadTasks()
    const interval = setInterval(loadTasks, 10000)
    return () => clearInterval(interval)
  }, [filter, convFilter])

  const handleBatchCancel = async () => {
    if (!confirm(`确定取消所有排队中的任务？`)) return
    try {
      await taskAPI.batchCancel(convFilter || undefined)
      loadTasks()
    } catch (e) {
      console.error(e)
    }
  }

  const handleCancel = async (id: string) => {
    try {
      await taskAPI.cancel(id)
      loadTasks()
    } catch (e) {
      console.error(e)
    }
  }

  const handlePause = async (id: string) => {
    try {
      await taskAPI.pause(id)
      loadTasks()
    } catch (e) {
      console.error(e)
    }
  }

  const handleResume = async (id: string) => {
    try {
      await taskAPI.resume(id)
      loadTasks()
    } catch (e) {
      console.error(e)
    }
  }

  const handleCreate = async () => {
    if (!newTask.title || !newTask.goal) return
    setCreating(true)
    try {
      await taskAPI.create(newTask)
      setNewTask({ title: '', goal: '', priority: 'normal' })
      setShowCreate(false)
      loadTasks()
    } catch (e) {
      console.error(e)
    } finally {
      setCreating(false)
    }
  }

  const formatTime = (t: string | null) => {
    if (!t) return '-'
    return new Date(t).toLocaleString('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' })
  }

  const runningCount = tasks.filter(t => t.status === 'running').length
  const pendingCount = tasks.filter(t => t.status === 'pending' || t.status === 'waiting').length

  return (
    <div className="h-full flex flex-col bg-gray-50 dark:bg-gray-900">
      {/* Header */}
      <div className="flex-none border-b border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800 px-6 py-4">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-3">
            <ListTodo className="w-6 h-6 text-violet-600" />
            <h1 className="text-xl font-semibold text-gray-900 dark:text-white">自主任务</h1>
            <div className="flex gap-2 ml-4">
              {runningCount > 0 && (
                <span className="flex items-center gap-1 text-xs px-2 py-1 rounded-full bg-blue-100 text-blue-700">
                  <Loader2 className="w-3 h-3 animate-spin" /> {runningCount} 执行中
                </span>
              )}
              {pendingCount > 0 && (
                <span className="flex items-center gap-1 text-xs px-2 py-1 rounded-full bg-yellow-100 text-yellow-700">
                  <Clock className="w-3 h-3" /> {pendingCount} 排队
                </span>
              )}
            </div>
          </div>
          <div className="flex items-center gap-3">
            {/* Conversation filter */}
            {conversations.length > 0 && (
              <select
                value={convFilter}
                onChange={e => setConvFilter(e.target.value)}
                className="text-sm border border-gray-300 dark:border-gray-600 rounded-lg px-3 py-1.5 bg-white dark:bg-gray-700 text-gray-700 dark:text-gray-200 max-w-[200px] truncate"
              >
                <option value="">全部对话</option>
                {conversations.map(c => (
                  <option key={c.conversation_id} value={c.conversation_id}>
                    {c.title || c.conversation_id.slice(0, 8)} ({c.count})
                  </option>
                ))}
              </select>
            )}
            {/* Status filter */}
            <select
              value={filter}
              onChange={e => setFilter(e.target.value)}
              className="text-sm border border-gray-300 dark:border-gray-600 rounded-lg px-3 py-1.5 bg-white dark:bg-gray-700 text-gray-700 dark:text-gray-200"
            >
              <option value="">全部状态</option>
              <option value="running">执行中</option>
              <option value="pending">排队中</option>
              <option value="waiting">等待执行</option>
              <option value="completed">已完成</option>
              <option value="failed">失败</option>
              <option value="cancelled">已取消</option>
            </select>
            {pendingCount > 0 && (
              <button
                onClick={handleBatchCancel}
                className="flex items-center gap-1.5 px-3 py-1.5 bg-red-50 text-red-600 border border-red-200 rounded-lg hover:bg-red-100 text-sm font-medium dark:bg-red-900/20 dark:text-red-400 dark:border-red-800 dark:hover:bg-red-900/40"
              >
                <Ban className="w-3.5 h-3.5" /> 全部取消排队
              </button>
            )}
            <button
              onClick={() => setShowCreate(!showCreate)}
              className="flex items-center gap-1.5 px-3 py-1.5 bg-violet-600 text-white rounded-lg hover:bg-violet-700 text-sm font-medium"
            >
              <Plus className="w-4 h-4" /> 新建任务
            </button>
          </div>
        </div>

        {/* Create task form */}
        {showCreate && (
          <div className="mt-4 p-4 border border-violet-200 dark:border-violet-800 rounded-lg bg-violet-50 dark:bg-violet-900/20">
            <div className="grid grid-cols-1 md:grid-cols-2 gap-3 mb-3">
              <input
                placeholder="任务标题"
                value={newTask.title}
                onChange={e => setNewTask({ ...newTask, title: e.target.value })}
                className="px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg text-sm bg-white dark:bg-gray-700 text-gray-900 dark:text-white"
              />
              <select
                value={newTask.priority}
                onChange={e => setNewTask({ ...newTask, priority: e.target.value })}
                className="px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg text-sm bg-white dark:bg-gray-700 text-gray-900 dark:text-white"
              >
                <option value="urgent">紧急</option>
                <option value="high">高</option>
                <option value="normal">普通</option>
                <option value="low">低</option>
              </select>
            </div>
            <textarea
              placeholder="详细的任务目标和指令（越详细越好）"
              value={newTask.goal}
              onChange={e => setNewTask({ ...newTask, goal: e.target.value })}
              rows={3}
              className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg text-sm bg-white dark:bg-gray-700 text-gray-900 dark:text-white mb-3"
            />
            <div className="flex justify-end gap-2">
              <button onClick={() => setShowCreate(false)} className="px-3 py-1.5 text-sm text-gray-600 dark:text-gray-300 hover:bg-gray-200 dark:hover:bg-gray-600 rounded-lg">取消</button>
              <button onClick={handleCreate} disabled={creating} className="px-4 py-1.5 text-sm bg-violet-600 text-white rounded-lg hover:bg-violet-700 disabled:opacity-50">
                {creating ? <Loader2 className="w-4 h-4 animate-spin" /> : '创建'}
              </button>
            </div>
          </div>
        )}
      </div>

      {/* Task list */}
      <div className="flex-1 overflow-y-auto p-6">
        {loading ? (
          <div className="flex items-center justify-center h-40">
            <Loader2 className="w-6 h-6 animate-spin text-violet-500" />
          </div>
        ) : tasks.length === 0 ? (
          <div className="text-center py-20 text-gray-500 dark:text-gray-400">
            <ListTodo className="w-12 h-12 mx-auto mb-3 opacity-30" />
            <p className="text-lg font-medium">暂无任务</p>
            <p className="text-sm mt-1">在对话中让 AI 创建后台任务，或点击上方"新建任务"</p>
          </div>
        ) : (
          <div className="space-y-3 max-w-4xl mx-auto">
            {tasks.map(task => {
              const cfg = statusConfig[task.status] || statusConfig.pending
              const StatusIcon = cfg.icon
              const isExpanded = expandedId === task.id
              return (
                <div key={task.id} className="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-xl overflow-hidden shadow-sm">
                  {/* Task header */}
                  <div
                    className="flex items-center gap-3 px-5 py-4 cursor-pointer hover:bg-gray-50 dark:hover:bg-gray-750"
                    onClick={() => setExpandedId(isExpanded ? null : task.id)}
                  >
                    {isExpanded ? <ChevronDown className="w-4 h-4 text-gray-400 flex-none" /> : <ChevronRight className="w-4 h-4 text-gray-400 flex-none" />}
                    <StatusIcon className={`w-5 h-5 flex-none ${task.status === 'running' ? 'animate-spin text-blue-500' : cfg.color.split(' ')[0]}`} />
                    <div className="flex-1 min-w-0">
                      <div className="flex items-center gap-2">
                        <span className="font-medium text-gray-900 dark:text-white truncate">{task.title}</span>
                        <span className={`text-xs px-2 py-0.5 rounded-full ${priorityColors[task.priority] || priorityColors.normal}`}>
                          {task.priority}
                        </span>
                        <span className={`text-xs px-2 py-0.5 rounded-full border ${cfg.color}`}>
                          {cfg.label}
                        </span>
                        {task.conversation_id && (
                          <button
                            onClick={(e) => { e.stopPropagation(); navigate(`/chat/${task.conversation_id}`) }}
                            className="flex items-center gap-0.5 px-1.5 py-0.5 bg-blue-50 dark:bg-blue-900/30 text-blue-500 rounded-full text-[10px] hover:bg-blue-100 dark:hover:bg-blue-900/50 transition-colors"
                            title="跳转到来源对话"
                          >
                            <MessageSquare className="w-2.5 h-2.5" />对话创建
                          </button>
                        )}
                      </div>
                      {task.progress_note && (
                        <p className="text-xs text-gray-500 dark:text-gray-400 mt-0.5 truncate">{task.progress_note}</p>
                      )}
                    </div>
                    <div className="flex items-center gap-1.5 flex-none">
                      {/* Pause/Resume button */}
                      {(task.status === 'running' || task.status === 'pending') && (
                        <button
                          onClick={(e) => { e.stopPropagation(); handlePause(task.id) }}
                          className="p-1.5 rounded-lg text-yellow-600 hover:bg-yellow-50 dark:hover:bg-yellow-900/20 transition-colors"
                          title="暂停任务"
                        >
                          <Pause className="w-4 h-4" />
                        </button>
                      )}
                      {task.status === 'waiting' && (
                        <button
                          onClick={(e) => { e.stopPropagation(); handleResume(task.id) }}
                          className="p-1.5 rounded-lg text-green-600 hover:bg-green-50 dark:hover:bg-green-900/20 transition-colors"
                          title="恢复任务"
                        >
                          <Play className="w-4 h-4" />
                        </button>
                      )}
                      {/* Cancel button */}
                      {(task.status === 'running' || task.status === 'pending' || task.status === 'waiting') && (
                        <button
                          onClick={(e) => { e.stopPropagation(); handleCancel(task.id) }}
                          className="p-1.5 rounded-lg text-red-500 hover:bg-red-50 dark:hover:bg-red-900/20 transition-colors"
                          title="取消任务"
                        >
                          <Ban className="w-4 h-4" />
                        </button>
                      )}
                      <span className="text-xs text-gray-400 ml-1">{formatTime(task.created_at)}</span>
                    </div>
                  </div>

                  {/* Progress bar */}
                  {(task.status === 'running' || task.progress > 0) && task.progress < 100 && (
                    <div className="px-5 pb-2">
                      <div className="w-full bg-gray-200 dark:bg-gray-700 rounded-full h-1.5">
                        <div className="bg-blue-500 h-1.5 rounded-full transition-all duration-500" style={{ width: `${task.progress}%` }} />
                      </div>
                    </div>
                  )}

                  {/* Expanded details */}
                  {isExpanded && (
                    <div className="px-5 pb-4 border-t border-gray-100 dark:border-gray-700 pt-3 space-y-3">
                      <div className="grid grid-cols-2 md:grid-cols-4 gap-3 text-sm">
                        <div>
                          <span className="text-gray-500 dark:text-gray-400 text-xs">进度</span>
                          <p className="font-mono text-gray-900 dark:text-white">{task.progress}%</p>
                        </div>
                        <div>
                          <span className="text-gray-500 dark:text-gray-400 text-xs">开始时间</span>
                          <p className="text-gray-900 dark:text-white">{formatTime(task.started_at)}</p>
                        </div>
                        <div>
                          <span className="text-gray-500 dark:text-gray-400 text-xs">完成时间</span>
                          <p className="text-gray-900 dark:text-white">{formatTime(task.completed_at)}</p>
                        </div>
                        <div>
                          <span className="text-gray-500 dark:text-gray-400 text-xs">定时执行</span>
                          <p className="text-gray-900 dark:text-white">{formatTime(task.scheduled_at)}</p>
                        </div>
                      </div>

                      {task.goal && (
                        <div>
                          <span className="text-xs text-gray-500 dark:text-gray-400">任务目标</span>
                          <p className="text-sm text-gray-700 dark:text-gray-300 mt-1 whitespace-pre-wrap bg-gray-50 dark:bg-gray-900 p-3 rounded-lg max-h-40 overflow-y-auto">{task.goal}</p>
                        </div>
                      )}

                      {task.result && (
                        <div>
                          <span className="text-xs text-green-600">执行结果</span>
                          <p className="text-sm text-gray-700 dark:text-gray-300 mt-1 whitespace-pre-wrap bg-green-50 dark:bg-green-900/20 p-3 rounded-lg max-h-60 overflow-y-auto">{task.result}</p>
                        </div>
                      )}

                      {task.error_msg && (
                        <div>
                          <span className="text-xs text-red-600">错误信息</span>
                          <p className="text-sm text-red-700 dark:text-red-400 mt-1 whitespace-pre-wrap bg-red-50 dark:bg-red-900/20 p-3 rounded-lg">{task.error_msg}</p>
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
    </div>
  )
}
