import { useState, useEffect } from 'react'
import { Activity, AlertTriangle, Bell, CheckCircle2, Clock, Eye, FileText, RefreshCw, Search, ToggleLeft, ToggleRight, Trash2, XCircle } from 'lucide-react'
import { observeAPI } from '../lib/api'

type Tab = 'overview' | 'traces' | 'logs' | 'alerts' | 'history'

export default function ObservePage() {
  const [tab, setTab] = useState<Tab>('overview')
  const [stats, setStats] = useState<any>(null)
  const [spans, setSpans] = useState<any[]>([])
  const [logs, setLogs] = useState<any[]>([])
  const [alertRules, setAlertRules] = useState<any[]>([])
  const [alertHistory, setAlertHistory] = useState<any[]>([])
  const [loading, setLoading] = useState(false)
  const [traceId, setTraceId] = useState('')
  const [logLevel, setLogLevel] = useState('')

  const [showCreateAlert, setShowCreateAlert] = useState(false)
  const [newAlert, setNewAlert] = useState({ name: '', metric: 'error_rate', operator: 'gt', threshold: '0.1', cooldown_minutes: '5' })

  useEffect(() => { loadData() }, [tab])

  const loadData = async () => {
    setLoading(true)
    try {
      if (tab === 'overview') {
        const res = await observeAPI.stats()
        setStats(res.data)
      } else if (tab === 'traces') {
        const res = await observeAPI.querySpans({})
        setSpans(res.data || [])
      } else if (tab === 'logs') {
        const res = await observeAPI.queryLogs({ level: logLevel || undefined, page_size: 50 })
        setLogs(res.data?.items || [])
      } else if (tab === 'alerts') {
        const res = await observeAPI.listAlertRules()
        setAlertRules(res.data || [])
      } else if (tab === 'history') {
        const res = await observeAPI.listAlertHistory({})
        setAlertHistory(res.data || [])
      }
    } catch {}
    setLoading(false)
  }

  const searchTrace = async () => {
    if (!traceId.trim()) return
    try {
      const res = await observeAPI.getTrace(traceId.trim())
      setSpans(res.data || [])
    } catch {}
  }

  const toggleAlert = async (id: string) => {
    await observeAPI.toggleAlertRule(id)
    loadData()
  }

  const deleteAlert = async (id: string) => {
    if (!confirm('确认删除此告警规则？')) return
    await observeAPI.deleteAlertRule(id)
    loadData()
  }

  const resolveAlert = async (id: string) => {
    await observeAPI.resolveAlert(id)
    loadData()
  }

  const createAlert = async () => {
    await observeAPI.createAlertRule({
      name: newAlert.name,
      metric: newAlert.metric,
      operator: newAlert.operator,
      threshold: parseFloat(newAlert.threshold),
      cooldown_minutes: parseInt(newAlert.cooldown_minutes),
    })
    setShowCreateAlert(false)
    setNewAlert({ name: '', metric: 'error_rate', operator: 'gt', threshold: '0.1', cooldown_minutes: '5' })
    loadData()
  }

  const tabs: { key: Tab; label: string; icon: React.ReactNode }[] = [
    { key: 'overview', label: '总览', icon: <Eye className="w-4 h-4" /> },
    { key: 'traces', label: '链路追踪', icon: <Activity className="w-4 h-4" /> },
    { key: 'logs', label: '结构化日志', icon: <FileText className="w-4 h-4" /> },
    { key: 'alerts', label: '告警规则', icon: <Bell className="w-4 h-4" /> },
    { key: 'history', label: '告警历史', icon: <AlertTriangle className="w-4 h-4" /> },
  ]

  return (
    <div className="p-6 max-w-7xl mx-auto">
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-2xl font-bold text-gray-900 dark:text-white">可观测性</h1>
          <p className="text-sm text-gray-500 mt-1">链路追踪、告警规则、结构化日志</p>
        </div>
        <button onClick={loadData} className="flex items-center gap-2 px-3 py-2 text-sm bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg hover:bg-gray-50 dark:hover:bg-gray-700">
          <RefreshCw className={`w-4 h-4 ${loading ? 'animate-spin' : ''}`} /> 刷新
        </button>
      </div>

      {/* Tabs */}
      <div className="flex gap-1 mb-6 bg-gray-100 dark:bg-gray-800 p-1 rounded-lg w-fit">
        {tabs.map(t => (
          <button key={t.key} onClick={() => setTab(t.key)}
            className={`flex items-center gap-1.5 px-3 py-1.5 text-sm rounded-md transition ${tab === t.key ? 'bg-white dark:bg-gray-700 shadow text-primary-600 font-medium' : 'text-gray-500 hover:text-gray-700 dark:hover:text-gray-300'}`}>
            {t.icon} {t.label}
          </button>
        ))}
      </div>

      {/* Overview */}
      {tab === 'overview' && stats && (
        <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
          {[
            { label: 'Trace 总数', value: stats.total_traces ?? 0, color: 'blue' },
            { label: '今日 Span', value: stats.spans_today ?? 0, color: 'green' },
            { label: '活跃告警', value: stats.active_alerts ?? 0, color: 'red' },
            { label: '今日日志', value: stats.logs_today ?? 0, color: 'purple' },
          ].map(s => (
            <div key={s.label} className="bg-white dark:bg-gray-800 rounded-xl p-5 border border-gray-200 dark:border-gray-700">
              <p className="text-sm text-gray-500">{s.label}</p>
              <p className="text-3xl font-bold mt-1 text-gray-900 dark:text-white">{s.value}</p>
            </div>
          ))}
        </div>
      )}

      {/* Traces */}
      {tab === 'traces' && (
        <div>
          <div className="flex gap-2 mb-4">
            <div className="relative flex-1 max-w-md">
              <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-gray-400" />
              <input value={traceId} onChange={e => setTraceId(e.target.value)} onKeyDown={e => e.key === 'Enter' && searchTrace()}
                placeholder="输入 Trace ID 搜索..." className="w-full pl-9 pr-3 py-2 text-sm bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg" />
            </div>
            <button onClick={searchTrace} className="px-4 py-2 text-sm bg-primary-600 text-white rounded-lg hover:bg-primary-700">搜索</button>
          </div>
          <div className="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 overflow-hidden">
            <table className="w-full text-sm">
              <thead className="bg-gray-50 dark:bg-gray-900 text-gray-500">
                <tr>
                  <th className="px-4 py-3 text-left font-medium">Trace ID</th>
                  <th className="px-4 py-3 text-left font-medium">服务</th>
                  <th className="px-4 py-3 text-left font-medium">操作</th>
                  <th className="px-4 py-3 text-left font-medium">状态</th>
                  <th className="px-4 py-3 text-left font-medium">耗时</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-gray-100 dark:divide-gray-700">
                {spans.length === 0 ? (
                  <tr><td colSpan={5} className="px-4 py-8 text-center text-gray-400">暂无数据</td></tr>
                ) : spans.map((s: any) => (
                  <tr key={s.id} className="hover:bg-gray-50 dark:hover:bg-gray-700/50">
                    <td className="px-4 py-3 font-mono text-xs">{s.trace_id?.slice(0, 12)}...</td>
                    <td className="px-4 py-3">{s.service_name}</td>
                    <td className="px-4 py-3">{s.operation}</td>
                    <td className="px-4 py-3"><span className={`px-2 py-0.5 rounded text-xs ${s.status === 'ok' ? 'bg-green-100 text-green-700' : 'bg-red-100 text-red-700'}`}>{s.status}</span></td>
                    <td className="px-4 py-3 text-gray-500">{s.duration_ms}ms</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {/* Logs */}
      {tab === 'logs' && (
        <div>
          <div className="flex gap-2 mb-4">
            <select value={logLevel} onChange={e => { setLogLevel(e.target.value); setTimeout(loadData, 0) }}
              className="px-3 py-2 text-sm bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg">
              <option value="">全部级别</option>
              <option value="debug">Debug</option>
              <option value="info">Info</option>
              <option value="warn">Warn</option>
              <option value="error">Error</option>
            </select>
          </div>
          <div className="space-y-2">
            {logs.length === 0 ? <p className="text-center text-gray-400 py-8">暂无日志</p> : logs.map((l: any) => (
              <div key={l.id} className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-3 text-sm">
                <div className="flex items-center gap-2 mb-1">
                  <span className={`px-1.5 py-0.5 rounded text-xs font-medium ${l.level === 'error' ? 'bg-red-100 text-red-700' : l.level === 'warn' ? 'bg-yellow-100 text-yellow-700' : l.level === 'info' ? 'bg-blue-100 text-blue-700' : 'bg-gray-100 text-gray-600'}`}>{l.level?.toUpperCase()}</span>
                  <span className="text-gray-400 text-xs">{l.service_name}</span>
                  <span className="text-gray-300 text-xs ml-auto"><Clock className="w-3 h-3 inline mr-1" />{new Date(l.created_at).toLocaleString()}</span>
                </div>
                <p className="text-gray-700 dark:text-gray-300">{l.message}</p>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Alert Rules */}
      {tab === 'alerts' && (
        <div>
          <div className="flex justify-end mb-4">
            <button onClick={() => setShowCreateAlert(true)} className="px-4 py-2 text-sm bg-primary-600 text-white rounded-lg hover:bg-primary-700">+ 新建规则</button>
          </div>
          {showCreateAlert && (
            <div className="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-4 mb-4 space-y-3">
              <input value={newAlert.name} onChange={e => setNewAlert({ ...newAlert, name: e.target.value })} placeholder="规则名称" className="w-full px-3 py-2 text-sm bg-gray-50 dark:bg-gray-900 border border-gray-200 dark:border-gray-700 rounded-lg" />
              <div className="flex gap-2">
                <select value={newAlert.metric} onChange={e => setNewAlert({ ...newAlert, metric: e.target.value })} className="px-3 py-2 text-sm bg-gray-50 dark:bg-gray-900 border border-gray-200 dark:border-gray-700 rounded-lg">
                  <option value="error_rate">错误率</option><option value="p99_latency">P99 延迟</option><option value="p95_latency">P95 延迟</option>
                  <option value="agent_failures">Agent 失败数</option><option value="error_count">错误计数</option><option value="avg_latency">平均延迟</option>
                </select>
                <select value={newAlert.operator} onChange={e => setNewAlert({ ...newAlert, operator: e.target.value })} className="px-3 py-2 text-sm bg-gray-50 dark:bg-gray-900 border border-gray-200 dark:border-gray-700 rounded-lg">
                  <option value="gt">&gt;</option><option value="gte">&ge;</option><option value="lt">&lt;</option><option value="lte">&le;</option><option value="eq">=</option>
                </select>
                <input value={newAlert.threshold} onChange={e => setNewAlert({ ...newAlert, threshold: e.target.value })} placeholder="阈值" className="w-24 px-3 py-2 text-sm bg-gray-50 dark:bg-gray-900 border border-gray-200 dark:border-gray-700 rounded-lg" />
                <input value={newAlert.cooldown_minutes} onChange={e => setNewAlert({ ...newAlert, cooldown_minutes: e.target.value })} placeholder="冷却(分钟)" className="w-28 px-3 py-2 text-sm bg-gray-50 dark:bg-gray-900 border border-gray-200 dark:border-gray-700 rounded-lg" />
              </div>
              <div className="flex gap-2 justify-end">
                <button onClick={() => setShowCreateAlert(false)} className="px-3 py-1.5 text-sm text-gray-500 hover:text-gray-700">取消</button>
                <button onClick={createAlert} disabled={!newAlert.name} className="px-4 py-1.5 text-sm bg-primary-600 text-white rounded-lg hover:bg-primary-700 disabled:opacity-50">创建</button>
              </div>
            </div>
          )}
          <div className="space-y-2">
            {alertRules.map((r: any) => (
              <div key={r.id} className="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-4 flex items-center justify-between">
                <div>
                  <p className="font-medium text-gray-900 dark:text-white">{r.name}</p>
                  <p className="text-xs text-gray-500 mt-0.5">{r.metric} {r.operator} {r.threshold} · 冷却 {r.cooldown_minutes}min · 触发 {r.fired_count ?? 0} 次</p>
                </div>
                <div className="flex items-center gap-2">
                  <button onClick={() => toggleAlert(r.id)} className="p-1.5 hover:bg-gray-100 dark:hover:bg-gray-700 rounded">
                    {r.enabled ? <ToggleRight className="w-5 h-5 text-green-500" /> : <ToggleLeft className="w-5 h-5 text-gray-400" />}
                  </button>
                  <button onClick={() => deleteAlert(r.id)} className="p-1.5 hover:bg-red-50 dark:hover:bg-red-900/20 rounded text-red-500"><Trash2 className="w-4 h-4" /></button>
                </div>
              </div>
            ))}
            {alertRules.length === 0 && <p className="text-center text-gray-400 py-8">暂无告警规则</p>}
          </div>
        </div>
      )}

      {/* Alert History */}
      {tab === 'history' && (
        <div className="space-y-2">
          {alertHistory.length === 0 ? <p className="text-center text-gray-400 py-8">暂无告警历史</p> : alertHistory.map((h: any) => (
            <div key={h.id} className="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-4 flex items-center justify-between">
              <div>
                <div className="flex items-center gap-2">
                  {h.resolved ? <CheckCircle2 className="w-4 h-4 text-green-500" /> : <XCircle className="w-4 h-4 text-red-500" />}
                  <p className="font-medium text-gray-900 dark:text-white">{h.rule_name || h.rule_id?.slice(0, 8)}</p>
                </div>
                <p className="text-xs text-gray-500 mt-0.5">{h.metric_value} · {new Date(h.created_at).toLocaleString()}</p>
              </div>
              {!h.resolved && (
                <button onClick={() => resolveAlert(h.id)} className="px-3 py-1.5 text-xs bg-green-600 text-white rounded-lg hover:bg-green-700">标记已解决</button>
              )}
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
