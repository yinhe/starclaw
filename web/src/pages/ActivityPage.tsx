import { useState, useEffect } from 'react'
import { Brain, Clock, Heart, Shield, BookOpen, Zap, ToggleLeft, ToggleRight, Trash2, Plus, ChevronDown, ChevronRight, History, Sparkles, Play, PauseCircle, MessageSquare } from 'lucide-react'
import { activityAPI, scheduleAPI } from '../lib/api'

interface Activity {
  id: string
  name: string
  title: string
  description: string
  type: string
  trigger: string
  condition: string
  action: string
  channel: string
  cooldown: string
  template: string
  config: string
  enabled: boolean
  last_run_at: string | null
  next_run_at: string | null
  last_result: string
  total_runs: number
  success_runs: number
  consec_fails: number
  pending_tasks: number
  created_at: string
}

interface ActivityLog {
  id: string
  activity_id: string
  task_id: string
  status: string
  result: string
  error: string
  created_at: string
}

interface Schedule {
  id: string
  title: string
  goal: string
  cron_expr: string
  enabled: boolean
  conversation_id: string
  conversation_title: string
  agent_id: string
  last_run_at: string | null
  next_run_at: string | null
  created_at: string
}

const typeConfig: Record<string, { label: string; icon: typeof Heart; color: string }> = {
  care:     { label: '关怀本能', icon: Heart,    color: 'text-pink-600 bg-pink-50 border-pink-200' },
  schedule: { label: '时间本能', icon: Clock,    color: 'text-blue-600 bg-blue-50 border-blue-200' },
  monitor:  { label: '监控本能', icon: Shield,   color: 'text-orange-600 bg-orange-50 border-orange-200' },
  event:    { label: '事件本能', icon: Zap,      color: 'text-purple-600 bg-purple-50 border-purple-200' },
  learn:    { label: '学习本能', icon: BookOpen,  color: 'text-green-600 bg-green-50 border-green-200' },
}

export default function ActivityPage() {
  const [activities, setActivities] = useState<Activity[]>([])
  const [loading, setLoading] = useState(true)
  const [expandedId, setExpandedId] = useState<string | null>(null)
  const [logs, setLogs] = useState<ActivityLog[]>([])
  const [logsLoading, setLogsLoading] = useState(false)
  const [showCreate, setShowCreate] = useState(false)
  const [filterType, setFilterType] = useState<string>('')
  const [seeding, setSeeding] = useState(false)
  const [schedules, setSchedules] = useState<Schedule[]>([])
  const [selectedSchIds, setSelectedSchIds] = useState<Set<string>>(new Set())

  useEffect(() => { load() }, [filterType])

  async function load() {
    setLoading(true)
    try {
      const [actRes, schRes] = await Promise.all([
        activityAPI.list(filterType || undefined),
        scheduleAPI.list(),
      ])
      setActivities(actRes.data.activities || [])
      setSchedules(schRes.data.schedules || [])
    } catch { /* ignore */ }
    setLoading(false)
  }

  async function toggleActivity(id: string) {
    try {
      const res = await activityAPI.toggle(id)
      setActivities(prev => prev.map(a => a.id === id ? { ...a, enabled: res.data.enabled } : a))
    } catch { /* ignore */ }
  }

  async function deleteActivity(id: string) {
    if (!confirm('确定删除此活动？')) return
    try {
      await activityAPI.delete(id)
      setActivities(prev => prev.filter(a => a.id !== id))
    } catch { /* ignore */ }
  }

  async function loadLogs(id: string) {
    if (expandedId === id) { setExpandedId(null); return }
    setExpandedId(id)
    setLogsLoading(true)
    try {
      const res = await activityAPI.logs(id)
      setLogs(res.data.logs || [])
    } catch { setLogs([]) }
    setLogsLoading(false)
  }

  async function seedBuiltins() {
    setSeeding(true)
    try {
      await activityAPI.seed()
      await load()
    } catch { /* ignore */ }
    setSeeding(false)
  }

  async function batchDisable() {
    if (!confirm('确定暂停所有活动？暂停后不会再触发新任务。')) return
    try {
      await activityAPI.batchDisable()
      await load()
    } catch { /* ignore */ }
  }

  async function toggleSchedule(id: string) {
    try {
      const res = await scheduleAPI.toggle(id)
      setSchedules(prev => prev.map(s => s.id === id ? { ...s, enabled: res.data.enabled } : s))
    } catch { /* ignore */ }
  }

  async function deleteSchedule(id: string) {
    if (!confirm('确定删除此定时任务？')) return
    try {
      await scheduleAPI.delete(id)
      setSchedules(prev => prev.filter(s => s.id !== id))
    } catch { /* ignore */ }
  }

  async function batchDeleteSchedules(ids?: string[]) {
    const count = ids ? ids.length : schedules.length
    const label = ids ? `选中的 ${count}` : `全部 ${count}`
    if (!confirm(`确定删除${label}个会话定时任务？删除后不可恢复。`)) return
    try {
      await scheduleAPI.batchDelete(ids)
      if (ids) {
        const idSet = new Set(ids)
        setSchedules(prev => prev.filter(s => !idSet.has(s.id)))
        setSelectedSchIds(new Set())
      } else {
        setSchedules([])
        setSelectedSchIds(new Set())
      }
    } catch { /* ignore */ }
  }

  function toggleSelectSchedule(id: string) {
    setSelectedSchIds(prev => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id); else next.add(id)
      return next
    })
  }

  function toggleSelectAll() {
    if (selectedSchIds.size === schedules.length) {
      setSelectedSchIds(new Set())
    } else {
      setSelectedSchIds(new Set(schedules.map(s => s.id)))
    }
  }

  const enabledCount = activities.filter(a => a.enabled).length
  const totalRuns = activities.reduce((sum, a) => sum + a.total_runs, 0)
  const totalPending = activities.reduce((sum, a) => sum + (a.pending_tasks || 0), 0)

  return (
    <div className="min-h-screen bg-gray-50 p-6">
      <div className="max-w-5xl mx-auto">
        {/* Header */}
        <div className="flex items-center justify-between mb-6">
          <div className="flex items-center gap-3">
            <Brain className="w-8 h-8 text-indigo-600" />
            <div>
              <h1 className="text-2xl font-bold text-gray-900">本能系统</h1>
              <p className="text-sm text-gray-500">
                Instinct — 主动行为引擎 · {enabledCount} 个活跃 · 累计执行 {totalRuns} 次{totalPending > 0 ? ` · ${totalPending} 个任务排队中` : ''}
              </p>
            </div>
          </div>
          <div className="flex gap-2">
            {enabledCount > 0 && (
              <button onClick={batchDisable}
                className="px-3 py-2 text-sm bg-red-50 text-red-600 border border-red-200 rounded-lg hover:bg-red-100 flex items-center gap-1">
                <PauseCircle className="w-4 h-4" /> 全部暂停
              </button>
            )}
            <button onClick={seedBuiltins} disabled={seeding}
              className="px-3 py-2 text-sm bg-indigo-50 text-indigo-700 rounded-lg hover:bg-indigo-100 flex items-center gap-1">
              <Sparkles className="w-4 h-4" />
              {seeding ? '初始化中...' : '初始化内置活动'}
            </button>
            <button onClick={() => setShowCreate(!showCreate)}
              className="px-3 py-2 text-sm bg-indigo-600 text-white rounded-lg hover:bg-indigo-700 flex items-center gap-1">
              <Plus className="w-4 h-4" /> 新建活动
            </button>
          </div>
        </div>

        {/* Type filter tabs */}
        <div className="flex gap-2 mb-4 flex-wrap">
          <button onClick={() => setFilterType('')}
            className={`px-3 py-1.5 text-sm rounded-full border ${!filterType ? 'bg-gray-900 text-white border-gray-900' : 'bg-white text-gray-600 border-gray-200 hover:bg-gray-50'}`}>
            全部
          </button>
          {Object.entries(typeConfig).map(([key, cfg]) => (
            <button key={key} onClick={() => setFilterType(key)}
              className={`px-3 py-1.5 text-sm rounded-full border flex items-center gap-1 ${filterType === key ? 'bg-gray-900 text-white border-gray-900' : `${cfg.color}`}`}>
              <cfg.icon className="w-3.5 h-3.5" /> {cfg.label}
            </button>
          ))}
        </div>

        {/* Create form */}
        {showCreate && <CreateActivityForm onCreated={() => { setShowCreate(false); load() }} />}

        {/* Activity list */}
        {loading ? (
          <div className="text-center py-12 text-gray-400">加载中...</div>
        ) : activities.length === 0 ? (
          <div className="text-center py-16 bg-white rounded-xl border">
            <Brain className="w-12 h-12 text-gray-300 mx-auto mb-3" />
            <p className="text-gray-500">还没有活动</p>
            <p className="text-sm text-gray-400 mt-1">点击「初始化内置活动」添加默认活动模板</p>
          </div>
        ) : (
          <div className="space-y-3">
            {activities.map(act => {
              const cfg = typeConfig[act.type] || typeConfig.schedule
              const Icon = cfg.icon
              const isExpanded = expandedId === act.id
              return (
                <div key={act.id} className="bg-white rounded-xl border shadow-sm overflow-hidden">
                  <div className="flex items-center gap-3 p-4">
                    {/* Type icon */}
                    <div className={`w-10 h-10 rounded-lg flex items-center justify-center ${cfg.color}`}>
                      <Icon className="w-5 h-5" />
                    </div>

                    {/* Info */}
                    <div className="flex-1 min-w-0">
                      <div className="flex items-center gap-2">
                        <h3 className="font-medium text-gray-900 truncate">{act.title}</h3>
                        {act.template && (
                          <span className="text-xs px-1.5 py-0.5 bg-gray-100 text-gray-500 rounded">内置</span>
                        )}
                      </div>
                      <div className="flex items-center gap-3 text-xs text-gray-400 mt-0.5">
                        <span className="font-mono">{act.trigger}</span>
                        {act.condition && <span>· 条件: {act.condition}</span>}
                        <span>· 冷却: {act.cooldown || '24h'}</span>
                        {act.total_runs > 0 && <span>· 已执行 {act.total_runs} 次</span>}
                        {act.pending_tasks > 0 && (
                          <span className="text-orange-600 font-medium">· {act.pending_tasks} 个任务排队</span>
                        )}
                        {act.next_run_at && (
                          <span>· 下次: {new Date(act.next_run_at).toLocaleString('zh-CN', { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' })}</span>
                        )}
                      </div>
                    </div>

                    {/* Actions */}
                    <div className="flex items-center gap-2">
                      <button onClick={() => loadLogs(act.id)} className="p-1.5 text-gray-400 hover:text-gray-600" title="执行记录">
                        {isExpanded ? <ChevronDown className="w-4 h-4" /> : <ChevronRight className="w-4 h-4" />}
                      </button>
                      <button onClick={() => toggleActivity(act.id)}
                        className={`p-1.5 rounded-lg ${act.enabled ? 'text-green-600 hover:bg-green-50' : 'text-gray-400 hover:bg-gray-50'}`}
                        title={act.enabled ? '已启用，点击关闭' : '已关闭，点击启用'}>
                        {act.enabled ? <ToggleRight className="w-5 h-5" /> : <ToggleLeft className="w-5 h-5" />}
                      </button>
                      <button onClick={() => deleteActivity(act.id)} className="p-1.5 text-gray-400 hover:text-red-500" title="删除">
                        <Trash2 className="w-4 h-4" />
                      </button>
                    </div>
                  </div>

                  {/* Description */}
                  {act.description && (
                    <div className="px-4 pb-3 text-sm text-gray-500">{act.description}</div>
                  )}

                  {/* Expanded: logs */}
                  {isExpanded && (
                    <div className="border-t bg-gray-50 p-4">
                      <div className="flex items-center gap-2 mb-2 text-sm font-medium text-gray-700">
                        <History className="w-4 h-4" /> 执行记录
                      </div>
                      {logsLoading ? (
                        <div className="text-sm text-gray-400">加载中...</div>
                      ) : logs.length === 0 ? (
                        <div className="text-sm text-gray-400">暂无执行记录</div>
                      ) : (
                        <div className="space-y-1.5">
                          {logs.slice(0, 10).map(log => (
                            <div key={log.id} className="flex items-center gap-2 text-sm">
                              <span className={`w-2 h-2 rounded-full ${log.status === 'ok' ? 'bg-green-400' : log.status === 'failed' ? 'bg-red-400' : 'bg-gray-300'}`} />
                              <span className="text-gray-500">{new Date(log.created_at).toLocaleString('zh-CN')}</span>
                              <span className={log.status === 'ok' ? 'text-green-600' : 'text-red-600'}>
                                {log.status === 'ok' ? '成功' : log.status === 'failed' ? '失败' : log.status}
                              </span>
                              {log.error && <span className="text-red-400 truncate">— {log.error}</span>}
                            </div>
                          ))}
                        </div>
                      )}
                    </div>
                  )}
                </div>
              )
            })}
          </div>
        )}

        {/* Conversation Schedules section */}
        {schedules.length > 0 && (
          <div className="mt-8">
            <div className="flex items-center gap-2 mb-4">
              <MessageSquare className="w-5 h-5 text-blue-600" />
              <h2 className="text-lg font-bold text-gray-900">会话定时任务</h2>
              <span className="text-sm text-gray-400">由对话中的 Agent 创建的 cron 定时任务</span>
              <div className="ml-auto flex items-center gap-2">
                <label className="flex items-center gap-1.5 text-sm text-gray-500 cursor-pointer select-none">
                  <input type="checkbox" checked={selectedSchIds.size === schedules.length && schedules.length > 0}
                    onChange={toggleSelectAll} className="rounded border-gray-300" />
                  全选
                </label>
                {selectedSchIds.size > 0 && (
                  <button onClick={() => batchDeleteSchedules(Array.from(selectedSchIds))}
                    className="px-3 py-1.5 text-sm bg-red-50 text-red-600 border border-red-200 rounded-lg hover:bg-red-100 flex items-center gap-1">
                    <Trash2 className="w-3.5 h-3.5" /> 删除选中 ({selectedSchIds.size})
                  </button>
                )}
                <button onClick={() => batchDeleteSchedules()}
                  className="px-3 py-1.5 text-sm bg-red-50 text-red-600 border border-red-200 rounded-lg hover:bg-red-100 flex items-center gap-1">
                  <Trash2 className="w-3.5 h-3.5" /> 全部删除 ({schedules.length})
                </button>
              </div>
            </div>
            <div className="space-y-3">
              {schedules.map(sch => (
                <div key={sch.id} className={`bg-white rounded-xl border shadow-sm p-4 flex items-center gap-3 ${selectedSchIds.has(sch.id) ? 'ring-2 ring-blue-300' : ''}`}>
                  <input type="checkbox" checked={selectedSchIds.has(sch.id)}
                    onChange={() => toggleSelectSchedule(sch.id)}
                    className="rounded border-gray-300 flex-shrink-0" />
                  <div className="w-10 h-10 rounded-lg flex items-center justify-center text-blue-600 bg-blue-50 border border-blue-200">
                    <Clock className="w-5 h-5" />
                  </div>
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2">
                      <h3 className="font-medium text-gray-900 truncate">{sch.title || sch.goal?.slice(0, 60)}</h3>
                    </div>
                    <div className="flex items-center gap-3 text-xs text-gray-400 mt-0.5 flex-wrap">
                      <span className="font-mono">{sch.cron_expr}</span>
                      {sch.conversation_id && (
                        <span className="text-blue-600 font-medium">
                          · 来自对话: {sch.conversation_title || sch.conversation_id.slice(0, 8)}
                        </span>
                      )}
                      {sch.last_run_at && (
                        <span>· 上次: {new Date(sch.last_run_at).toLocaleString('zh-CN', { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' })}</span>
                      )}
                      {sch.next_run_at && (
                        <span>· 下次: {new Date(sch.next_run_at).toLocaleString('zh-CN', { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' })}</span>
                      )}
                    </div>
                    {sch.goal && sch.title && (
                      <div className="text-xs text-gray-400 mt-1 truncate">{sch.goal.slice(0, 120)}</div>
                    )}
                  </div>
                  <div className="flex items-center gap-2">
                    <button onClick={() => toggleSchedule(sch.id)}
                      className={`p-1.5 rounded-lg ${sch.enabled ? 'text-green-600 hover:bg-green-50' : 'text-gray-400 hover:bg-gray-50'}`}
                      title={sch.enabled ? '已启用，点击关闭' : '已关闭，点击启用'}>
                      {sch.enabled ? <ToggleRight className="w-5 h-5" /> : <ToggleLeft className="w-5 h-5" />}
                    </button>
                    <button onClick={() => deleteSchedule(sch.id)} className="p-1.5 text-gray-400 hover:text-red-500" title="删除">
                      <Trash2 className="w-4 h-4" />
                    </button>
                  </div>
                </div>
              ))}
            </div>
          </div>
        )}
      </div>
    </div>
  )
}

function CreateActivityForm({ onCreated }: { onCreated: () => void }) {
  const [form, setForm] = useState({
    name: '', title: '', description: '', type: 'schedule',
    trigger: '0 9 * * *', condition: '', action: '',
    channel: '', cooldown: '24h', enabled: true,
  })
  const [submitting, setSubmitting] = useState(false)

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    setSubmitting(true)
    try {
      await activityAPI.create(form)
      onCreated()
    } catch { /* ignore */ }
    setSubmitting(false)
  }

  return (
    <form onSubmit={handleSubmit} className="bg-white rounded-xl border p-5 mb-4 space-y-3">
      <h3 className="font-medium text-gray-900 flex items-center gap-2">
        <Plus className="w-4 h-4" /> 新建自定义活动
      </h3>
      <div className="grid grid-cols-2 gap-3">
        <input placeholder="活动名称 (英文标识)" value={form.name}
          onChange={e => setForm({ ...form, name: e.target.value })} required
          className="px-3 py-2 border rounded-lg text-sm" />
        <input placeholder="活动标题" value={form.title}
          onChange={e => setForm({ ...form, title: e.target.value })} required
          className="px-3 py-2 border rounded-lg text-sm" />
      </div>
      <input placeholder="描述" value={form.description}
        onChange={e => setForm({ ...form, description: e.target.value })}
        className="w-full px-3 py-2 border rounded-lg text-sm" />
      <div className="grid grid-cols-3 gap-3">
        <select value={form.type} onChange={e => setForm({ ...form, type: e.target.value })}
          className="px-3 py-2 border rounded-lg text-sm">
          <option value="schedule">时间本能</option>
          <option value="care">关怀本能</option>
          <option value="monitor">监控本能</option>
          <option value="event">事件本能</option>
          <option value="learn">学习本能</option>
        </select>
        <input placeholder="Cron 表达式" value={form.trigger}
          onChange={e => setForm({ ...form, trigger: e.target.value })} required
          className="px-3 py-2 border rounded-lg text-sm font-mono" />
        <input placeholder="冷却时间 (如 24h)" value={form.cooldown}
          onChange={e => setForm({ ...form, cooldown: e.target.value })}
          className="px-3 py-2 border rounded-lg text-sm" />
      </div>
      <input placeholder="条件 (可选, 如 user.birthday == today)" value={form.condition}
        onChange={e => setForm({ ...form, condition: e.target.value })}
        className="w-full px-3 py-2 border rounded-lg text-sm" />
      <textarea placeholder="执行动作 (发送给 Agent 的指令)" value={form.action}
        onChange={e => setForm({ ...form, action: e.target.value })} required rows={3}
        className="w-full px-3 py-2 border rounded-lg text-sm" />
      <div className="flex justify-end gap-2">
        <button type="button" onClick={onCreated} className="px-4 py-2 text-sm text-gray-600 hover:bg-gray-50 rounded-lg">取消</button>
        <button type="submit" disabled={submitting}
          className="px-4 py-2 text-sm bg-indigo-600 text-white rounded-lg hover:bg-indigo-700 flex items-center gap-1">
          <Play className="w-4 h-4" /> {submitting ? '创建中...' : '创建活动'}
        </button>
      </div>
    </form>
  )
}
