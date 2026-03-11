import { useEffect, useState } from 'react'
import { Bell, Plus, Trash2, Play, ChevronDown, ChevronUp, CheckCircle, XCircle } from 'lucide-react'
import { broodAPI, type Webhook, type WebhookLog } from '../api/brood'

export default function WebhooksPage() {
  const [webhooks, setWebhooks] = useState<Webhook[]>([])
  const [loading, setLoading] = useState(true)
  const [showCreate, setShowCreate] = useState(false)
  const [expanded, setExpanded] = useState<string | null>(null)
  const [logs, setLogs] = useState<WebhookLog[]>([])
  const [testResult, setTestResult] = useState<{ id: string; ok: boolean; msg: string } | null>(null)
  const [form, setForm] = useState({ name: '', url: '', events: '*' })

  const load = async () => {
    try {
      const data = await broodAPI.listWebhooks()
      setWebhooks(data.webhooks || [])
    } catch { /* */ }
    finally { setLoading(false) }
  }

  useEffect(() => { load() }, [])

  const handleCreate = async () => {
    if (!form.name || !form.url) return
    try {
      const data = await broodAPI.createWebhook(form)
      setShowCreate(false)
      setForm({ name: '', url: '', events: '*' })
      alert(`Webhook Secret (仅显示一次): ${data.secret}`)
      load()
    } catch { /* */ }
  }

  const handleDelete = async (id: string) => {
    if (!confirm('确定删除此 Webhook？')) return
    try { await broodAPI.deleteWebhook(id); load() } catch { /* */ }
  }

  const handleTest = async (id: string) => {
    setTestResult(null)
    try {
      const res = await broodAPI.testWebhook(id)
      setTestResult({ id, ok: res.success, msg: res.success ? `${res.status_code} OK` : (res.error || `${res.status_code}`) })
    } catch (e) {
      setTestResult({ id, ok: false, msg: e instanceof Error ? e.message : 'failed' })
    }
  }

  const toggleDetail = async (id: string) => {
    if (expanded === id) { setExpanded(null); return }
    setExpanded(id)
    try {
      const data = await broodAPI.getWebhook(id)
      setLogs(data.recent_logs || [])
    } catch { setLogs([]) }
  }

  return (
    <div className="p-8">
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-2xl font-bold text-white">Webhook 通知</h1>
          <p className="text-sm text-gray-500 mt-1">配置事件回调通知端点</p>
        </div>
        <button onClick={() => setShowCreate(!showCreate)}
          className="flex items-center gap-2 px-4 py-2 bg-overlord-600 text-white rounded-lg text-sm hover:bg-overlord-500 transition">
          <Plus className="w-4 h-4" /> 新建 Webhook
        </button>
      </div>

      {showCreate && (
        <div className="bg-gray-900 border border-gray-800 rounded-xl p-5 mb-6">
          <h3 className="text-sm font-medium text-white mb-4">创建 Webhook</h3>
          <div className="grid grid-cols-3 gap-4 mb-4">
            <div>
              <label className="block text-xs text-gray-400 mb-1">名称 *</label>
              <input value={form.name} onChange={e => setForm({ ...form, name: e.target.value })}
                placeholder="生产告警" className="w-full bg-gray-800 border border-gray-700 rounded-lg px-3 py-2 text-white text-sm focus:outline-none focus:border-overlord-500" />
            </div>
            <div>
              <label className="block text-xs text-gray-400 mb-1">URL *</label>
              <input value={form.url} onChange={e => setForm({ ...form, url: e.target.value })}
                placeholder="https://hooks.example.com/..." className="w-full bg-gray-800 border border-gray-700 rounded-lg px-3 py-2 text-white text-sm focus:outline-none focus:border-overlord-500" />
            </div>
            <div>
              <label className="block text-xs text-gray-400 mb-1">事件过滤 (* = 全部)</label>
              <input value={form.events} onChange={e => setForm({ ...form, events: e.target.value })}
                placeholder="node.offline,molt.failed" className="w-full bg-gray-800 border border-gray-700 rounded-lg px-3 py-2 text-white text-sm focus:outline-none focus:border-overlord-500" />
            </div>
          </div>
          <div className="flex gap-2">
            <button onClick={handleCreate} className="px-4 py-2 bg-overlord-600 text-white text-sm rounded-lg hover:bg-overlord-500 transition">创建</button>
            <button onClick={() => setShowCreate(false)} className="px-4 py-2 bg-gray-800 text-gray-300 text-sm rounded-lg hover:bg-gray-700 transition">取消</button>
          </div>
        </div>
      )}

      {loading ? (
        <div className="flex justify-center py-12">
          <div className="animate-spin w-6 h-6 border-2 border-overlord-500 border-t-transparent rounded-full" />
        </div>
      ) : webhooks.length === 0 ? (
        <div className="bg-gray-900 border border-gray-800 border-dashed rounded-xl p-12 text-center">
          <Bell className="w-10 h-10 text-gray-600 mx-auto mb-3" />
          <p className="text-gray-400">暂无 Webhook</p>
        </div>
      ) : (
        <div className="space-y-2">
          {webhooks.map(wh => {
            const isExpanded = expanded === wh.id
            const tr = testResult?.id === wh.id ? testResult : null
            return (
              <div key={wh.id} className="bg-gray-900 border border-gray-800 rounded-xl overflow-hidden hover:border-gray-700 transition">
                <div className="p-4 flex items-center gap-4 cursor-pointer" onClick={() => toggleDetail(wh.id)}>
                  <div className={`w-10 h-10 rounded-lg flex items-center justify-center shrink-0 ${wh.status === 'active' ? 'bg-amber-600/10' : 'bg-gray-600/10'}`}>
                    <Bell className={`w-5 h-5 ${wh.status === 'active' ? 'text-amber-400' : 'text-gray-400'}`} />
                  </div>
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2">
                      <span className="text-sm font-semibold text-white">{wh.name}</span>
                      <span className={`text-[10px] px-1.5 py-0.5 rounded ${wh.status === 'active' ? 'bg-emerald-600/10 text-emerald-400' : 'bg-gray-600/10 text-gray-400'}`}>
                        {wh.status}
                      </span>
                      <span className="text-[10px] text-gray-600 font-mono truncate max-w-[300px]">{wh.url}</span>
                    </div>
                    <div className="flex items-center gap-4 mt-1 text-xs text-gray-500">
                      <span>事件: {wh.events}</span>
                      <span>发送: {wh.total_sent}</span>
                      <span>失败: {wh.total_failed}</span>
                      {wh.last_error && <span className="text-red-400 truncate max-w-[200px]">{wh.last_error}</span>}
                    </div>
                    {tr && (
                      <div className={`flex items-center gap-1 mt-1 text-xs ${tr.ok ? 'text-emerald-400' : 'text-red-400'}`}>
                        {tr.ok ? <CheckCircle className="w-3 h-3" /> : <XCircle className="w-3 h-3" />}
                        <span>测试: {tr.msg}</span>
                      </div>
                    )}
                  </div>
                  <div className="flex items-center gap-1 shrink-0">
                    <button onClick={e => { e.stopPropagation(); handleTest(wh.id) }}
                      className="p-2 text-gray-500 hover:text-blue-400 transition" title="发送测试">
                      <Play className="w-4 h-4" />
                    </button>
                    <button onClick={e => { e.stopPropagation(); handleDelete(wh.id) }}
                      className="p-2 text-gray-500 hover:text-red-400 transition" title="删除">
                      <Trash2 className="w-4 h-4" />
                    </button>
                    {isExpanded ? <ChevronUp className="w-4 h-4 text-gray-500" /> : <ChevronDown className="w-4 h-4 text-gray-500" />}
                  </div>
                </div>

                {isExpanded && (
                  <div className="border-t border-gray-800 p-4">
                    <h4 className="text-xs font-medium text-gray-400 mb-2">最近投递记录</h4>
                    {logs.length === 0 ? (
                      <p className="text-xs text-gray-600">暂无记录</p>
                    ) : (
                      <div className="space-y-1 max-h-48 overflow-auto">
                        {logs.map(l => (
                          <div key={l.id} className="flex items-center gap-3 text-xs bg-gray-800/50 rounded-lg px-3 py-2">
                            <span className={`px-1.5 py-0.5 rounded font-mono ${l.status_code < 400 ? 'bg-emerald-600/10 text-emerald-400' : 'bg-red-600/10 text-red-400'}`}>
                              {l.status_code || 'ERR'}
                            </span>
                            <span className="text-gray-400">{l.event}</span>
                            <span className="text-gray-600">{l.duration_ms}ms</span>
                            {l.error && <span className="text-red-400 truncate">{l.error}</span>}
                            <span className="text-gray-600 ml-auto">{new Date(l.created_at).toLocaleTimeString('zh-CN')}</span>
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
    </div>
  )
}
