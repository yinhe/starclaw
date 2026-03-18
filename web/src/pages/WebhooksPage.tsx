import { useState, useEffect } from 'react'
import { Bell, CheckCircle2, Clock, Inbox, Play, Plus, RefreshCw, ToggleLeft, ToggleRight, Trash2 } from 'lucide-react'
import { webhookAPI } from '../lib/api'

type Tab = 'rules' | 'logs' | 'stats'

export default function WebhooksPage() {
  const [tab, setTab] = useState<Tab>('rules')
  const [rules, setRules] = useState<any[]>([])
  const [logs, setLogs] = useState<any[]>([])
  const [stats, setStats] = useState<any>(null)
  const [eventTypes, setEventTypes] = useState<any[]>([])
  const [loading, setLoading] = useState(false)
  const [logStatus, setLogStatus] = useState('')
  const [showCreate, setShowCreate] = useState(false)
  const [newRule, setNewRule] = useState({ name: '', event_type: '', actions: '', condition: '' })

  useEffect(() => { loadData() }, [tab])
  useEffect(() => { webhookAPI.eventTypes().then(r => setEventTypes(r.data || [])).catch(() => {}) }, [])

  const loadData = async () => {
    setLoading(true)
    try {
      if (tab === 'rules') { const r = await webhookAPI.listRules(); setRules(r.data || []) }
      if (tab === 'logs') { const r = await webhookAPI.listLogs({ status: logStatus || undefined, page_size: 50 }); setLogs(r.data?.items || []) }
      if (tab === 'stats') { const r = await webhookAPI.stats(); setStats(r.data) }
    } catch {}
    setLoading(false)
  }

  const toggleRule = async (id: string) => { await webhookAPI.toggleRule(id); loadData() }
  const deleteRule = async (id: string) => { if (!confirm('确认删除？')) return; await webhookAPI.deleteRule(id); loadData() }
  const retryDL = async (id: string) => { await webhookAPI.retryDeadLetter(id); loadData() }

  const createRule = async () => {
    const actions = newRule.actions || '[{"type":"webhook","url":"https://example.com","method":"POST"}]'
    await webhookAPI.createRule({ name: newRule.name, event_type: newRule.event_type, actions, condition: newRule.condition || '{}' })
    setShowCreate(false)
    setNewRule({ name: '', event_type: '', actions: '', condition: '' })
    loadData()
  }

  const testEvent = async (eventType: string) => {
    await webhookAPI.test({ event_type: eventType, data: { test: true, timestamp: new Date().toISOString() } })
  }

  const statusColors: Record<string, string> = {
    success: 'bg-green-100 text-green-700', failed: 'bg-red-100 text-red-700',
    retrying: 'bg-yellow-100 text-yellow-700', pending: 'bg-blue-100 text-blue-700',
    dead_letter: 'bg-gray-800 text-white',
  }

  return (
    <div className="p-6 max-w-7xl mx-auto">
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-2xl font-bold text-gray-900 dark:text-white">Webhook 编排</h1>
          <p className="text-sm text-gray-500 mt-1">事件驱动规则引擎、执行日志、死信队列</p>
        </div>
        <button onClick={loadData} className="flex items-center gap-2 px-3 py-2 text-sm bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg hover:bg-gray-50 dark:hover:bg-gray-700">
          <RefreshCw className={`w-4 h-4 ${loading ? 'animate-spin' : ''}`} /> 刷新
        </button>
      </div>

      <div className="flex gap-1 mb-6 bg-gray-100 dark:bg-gray-800 p-1 rounded-lg w-fit">
        {([['rules', '事件规则', <Bell className="w-4 h-4" key="r" />], ['logs', '执行日志', <Inbox className="w-4 h-4" key="l" />], ['stats', '统计概览', <CheckCircle2 className="w-4 h-4" key="s" />]] as [Tab, string, React.ReactNode][]).map(([k, l, i]) => (
          <button key={k} onClick={() => setTab(k)}
            className={`flex items-center gap-1.5 px-3 py-1.5 text-sm rounded-md transition ${tab === k ? 'bg-white dark:bg-gray-700 shadow text-primary-600 font-medium' : 'text-gray-500 hover:text-gray-700'}`}>
            {i} {l}
          </button>
        ))}
      </div>

      {/* Rules */}
      {tab === 'rules' && (
        <div>
          <div className="flex justify-end mb-4">
            <button onClick={() => setShowCreate(true)} className="flex items-center gap-1.5 px-4 py-2 text-sm bg-primary-600 text-white rounded-lg hover:bg-primary-700"><Plus className="w-4 h-4" /> 新建规则</button>
          </div>
          {showCreate && (
            <div className="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-4 mb-4 space-y-3">
              <input value={newRule.name} onChange={e => setNewRule({ ...newRule, name: e.target.value })} placeholder="规则名称" className="w-full px-3 py-2 text-sm bg-gray-50 dark:bg-gray-900 border border-gray-200 dark:border-gray-700 rounded-lg" />
              <select value={newRule.event_type} onChange={e => setNewRule({ ...newRule, event_type: e.target.value })} className="w-full px-3 py-2 text-sm bg-gray-50 dark:bg-gray-900 border border-gray-200 dark:border-gray-700 rounded-lg">
                <option value="">选择事件类型</option>
                {eventTypes.map((t: any) => <option key={t.type} value={t.type}>{t.type} — {t.description}</option>)}
              </select>
              <textarea value={newRule.actions} onChange={e => setNewRule({ ...newRule, actions: e.target.value })} placeholder='Actions JSON, 例如: [{"type":"webhook","url":"https://...","method":"POST"}]' rows={3} className="w-full px-3 py-2 text-sm bg-gray-50 dark:bg-gray-900 border border-gray-200 dark:border-gray-700 rounded-lg font-mono" />
              <div className="flex gap-2 justify-end">
                <button onClick={() => setShowCreate(false)} className="px-3 py-1.5 text-sm text-gray-500">取消</button>
                <button onClick={createRule} disabled={!newRule.name || !newRule.event_type} className="px-4 py-1.5 text-sm bg-primary-600 text-white rounded-lg hover:bg-primary-700 disabled:opacity-50">创建</button>
              </div>
            </div>
          )}
          <div className="space-y-2">
            {rules.map((r: any) => (
              <div key={r.id} className="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-4 flex items-center justify-between">
                <div>
                  <div className="flex items-center gap-2">
                    <p className="font-medium text-gray-900 dark:text-white">{r.name}</p>
                    <span className="px-2 py-0.5 text-xs bg-blue-100 text-blue-700 rounded">{r.event_type}</span>
                  </div>
                  <p className="text-xs text-gray-500 mt-0.5">触发 {r.fired_count ?? 0} 次 · 重试 {r.retry_count}x · 延迟 {r.retry_delay}s</p>
                </div>
                <div className="flex items-center gap-2">
                  <button onClick={() => testEvent(r.event_type)} className="p-1.5 hover:bg-gray-100 dark:hover:bg-gray-700 rounded" title="发送测试事件"><Play className="w-4 h-4 text-gray-400" /></button>
                  <button onClick={() => toggleRule(r.id)} className="p-1.5 hover:bg-gray-100 dark:hover:bg-gray-700 rounded">
                    {r.enabled ? <ToggleRight className="w-5 h-5 text-green-500" /> : <ToggleLeft className="w-5 h-5 text-gray-400" />}
                  </button>
                  <button onClick={() => deleteRule(r.id)} className="p-1.5 hover:bg-red-50 dark:hover:bg-red-900/20 rounded text-red-500"><Trash2 className="w-4 h-4" /></button>
                </div>
              </div>
            ))}
            {rules.length === 0 && <p className="text-center text-gray-400 py-8">暂无事件规则</p>}
          </div>
        </div>
      )}

      {/* Logs */}
      {tab === 'logs' && (
        <div>
          <div className="flex gap-2 mb-4">
            <select value={logStatus} onChange={e => { setLogStatus(e.target.value); setTimeout(loadData, 0) }} className="px-3 py-2 text-sm bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg">
              <option value="">全部状态</option>
              <option value="success">成功</option><option value="failed">失败</option><option value="retrying">重试中</option><option value="dead_letter">死信</option>
            </select>
          </div>
          <div className="space-y-2">
            {logs.length === 0 ? <p className="text-center text-gray-400 py-8">暂无日志</p> : logs.map((l: any) => (
              <div key={l.id} className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-3 flex items-center justify-between">
                <div className="min-w-0 flex-1">
                  <div className="flex items-center gap-2">
                    <span className={`px-1.5 py-0.5 rounded text-xs font-medium ${statusColors[l.status] || 'bg-gray-100 text-gray-600'}`}>{l.status}</span>
                    <span className="text-sm font-medium text-gray-900 dark:text-white truncate">{l.rule_name || l.rule_id?.slice(0, 8)}</span>
                    <span className="text-xs text-gray-400">{l.event_type}</span>
                  </div>
                  <p className="text-xs text-gray-500 mt-0.5 flex items-center gap-2">
                    {l.action_type} → {l.action_url?.slice(0, 40)} · 尝试 {l.attempts}/{l.max_retries}
                    <Clock className="w-3 h-3" />{new Date(l.created_at).toLocaleString()}
                  </p>
                  {l.error && <p className="text-xs text-red-500 mt-0.5">{l.error}</p>}
                </div>
                {l.status === 'dead_letter' && (
                  <button onClick={() => retryDL(l.id)} className="px-3 py-1 text-xs bg-yellow-600 text-white rounded hover:bg-yellow-700 ml-2 shrink-0">重试</button>
                )}
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Stats */}
      {tab === 'stats' && stats && (
        <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
          {[
            { label: '总规则数', value: stats.total_rules ?? 0 },
            { label: '启用规则', value: stats.enabled_rules ?? 0 },
            { label: '今日触发', value: stats.fired_today ?? 0 },
            { label: '今日成功', value: stats.success_today ?? 0 },
            { label: '今日失败', value: stats.failed_today ?? 0 },
            { label: '死信数量', value: stats.dead_letters ?? 0 },
            { label: '等待重试', value: stats.pending_retries ?? 0 },
            { label: '队列大小', value: stats.queue_size ?? 0 },
          ].map(s => (
            <div key={s.label} className="bg-white dark:bg-gray-800 rounded-xl p-5 border border-gray-200 dark:border-gray-700">
              <p className="text-sm text-gray-500">{s.label}</p>
              <p className="text-3xl font-bold mt-1 text-gray-900 dark:text-white">{s.value}</p>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
