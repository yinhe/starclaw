import { useEffect, useState } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { ArrowLeft, Wifi, WifiOff, AlertTriangle, Cpu, MemoryStick, Activity, Coins, Clock, Save, Trash2 } from 'lucide-react'
import { broodAPI, type ClawNode } from '../api/brood'

export default function ClawDetailPage() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const [claw, setClaw] = useState<ClawNode | null>(null)
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [maxConcurrent, setMaxConcurrent] = useState(10)
  const [maxTokensDay, setMaxTokensDay] = useState(0)
  const [msg, setMsg] = useState('')

  const load = async () => {
    if (!id) return
    try {
      const data = await broodAPI.getClaw(id)
      setClaw(data.claw)
      setMaxConcurrent(data.claw.max_concurrent)
      setMaxTokensDay(data.claw.max_tokens_day)
    } catch {
      /* ignore */
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => { load() }, [id])

  const handleSaveQuota = async () => {
    if (!id) return
    setSaving(true)
    setMsg('')
    try {
      await broodAPI.updateQuota(id, { max_concurrent: maxConcurrent, max_tokens_day: maxTokensDay })
      setMsg('配额已更新')
      load()
    } catch {
      setMsg('更新失败')
    }
    setSaving(false)
  }

  const handleRemove = async () => {
    if (!id || !claw) return
    if (!confirm(`确定移除节点 "${claw.name}" 吗？`)) return
    try {
      await broodAPI.removeClaw(id)
      navigate('/claws')
    } catch {
      /* ignore */
    }
  }

  if (loading) {
    return (
      <div className="flex items-center justify-center h-full">
        <div className="animate-spin w-8 h-8 border-2 border-overlord-500 border-t-transparent rounded-full" />
      </div>
    )
  }

  if (!claw) {
    return (
      <div className="flex flex-col items-center justify-center h-full gap-3">
        <p className="text-gray-400">节点未找到</p>
        <button onClick={() => navigate('/claws')} className="text-sm text-overlord-400 hover:text-overlord-300">
          返回列表
        </button>
      </div>
    )
  }

  const statusConfig = {
    online: { icon: Wifi, color: 'text-emerald-400', bg: 'bg-emerald-600/10', label: '在线' },
    feral: { icon: AlertTriangle, color: 'text-amber-400', bg: 'bg-amber-600/10', label: '失控' },
    offline: { icon: WifiOff, color: 'text-gray-400', bg: 'bg-gray-600/10', label: '离线' },
  }
  const sc = statusConfig[claw.status] || statusConfig.offline
  const StatusIcon = sc.icon

  const metrics = [
    { label: 'CPU 使用率', value: `${claw.cpu_percent.toFixed(1)}%`, icon: Cpu, color: 'text-blue-400', pct: claw.cpu_percent },
    { label: '内存使用率', value: `${claw.mem_percent.toFixed(1)}%`, icon: MemoryStick, color: 'text-purple-400', pct: claw.mem_percent },
    { label: '运行任务', value: claw.tasks_running, icon: Activity, color: 'text-cyan-400', pct: claw.max_concurrent > 0 ? (claw.tasks_running / claw.max_concurrent) * 100 : 0 },
    { label: '今日 Tokens', value: claw.tokens_today.toLocaleString(), icon: Coins, color: 'text-yellow-400', pct: claw.max_tokens_day > 0 ? (claw.tokens_today / claw.max_tokens_day) * 100 : 0 },
  ]

  const timeSince = (dateStr: string) => {
    const d = new Date(dateStr)
    const diff = Math.floor((Date.now() - d.getTime()) / 1000)
    if (diff < 60) return `${diff}秒前`
    if (diff < 3600) return `${Math.floor(diff / 60)}分钟前`
    if (diff < 86400) return `${Math.floor(diff / 3600)}小时前`
    return `${Math.floor(diff / 86400)}天前`
  }

  return (
    <div className="p-8 max-w-4xl">
      {/* Header */}
      <button
        onClick={() => navigate('/claws')}
        className="flex items-center gap-2 text-sm text-gray-400 hover:text-gray-200 mb-6 transition"
      >
        <ArrowLeft className="w-4 h-4" /> 返回节点列表
      </button>

      <div className="flex items-start justify-between mb-8">
        <div className="flex items-center gap-4">
          <div className={`w-12 h-12 rounded-xl ${sc.bg} flex items-center justify-center`}>
            <StatusIcon className={`w-6 h-6 ${sc.color}`} />
          </div>
          <div>
            <h1 className="text-2xl font-bold text-white">{claw.name || '未命名'}</h1>
            <div className="flex items-center gap-3 mt-1 text-sm text-gray-500">
              <span className={`${sc.color} text-xs px-2 py-0.5 rounded ${sc.bg}`}>{sc.label}</span>
              {claw.team && <span className="bg-gray-800 px-2 py-0.5 rounded text-xs">{claw.team}</span>}
              {claw.version && <span>{claw.version}</span>}
            </div>
          </div>
        </div>
        <button
          onClick={handleRemove}
          className="flex items-center gap-2 px-3 py-2 text-sm text-red-400 hover:bg-red-500/10 rounded-lg transition"
        >
          <Trash2 className="w-4 h-4" /> 移除
        </button>
      </div>

      {/* Info */}
      <div className="grid grid-cols-2 gap-4 mb-8">
        <div className="bg-gray-900 border border-gray-800 rounded-xl p-4">
          <div className="text-xs text-gray-500 mb-1">地址</div>
          <div className="text-sm text-white font-mono">{claw.address}</div>
        </div>
        <div className="bg-gray-900 border border-gray-800 rounded-xl p-4">
          <div className="text-xs text-gray-500 mb-1">Claw ID</div>
          <div className="text-sm text-white font-mono truncate">{claw.claw_id || '—'}</div>
        </div>
        <div className="bg-gray-900 border border-gray-800 rounded-xl p-4">
          <div className="text-xs text-gray-500 mb-1 flex items-center gap-1">
            <Clock className="w-3 h-3" /> 最后心跳
          </div>
          <div className="text-sm text-white">{timeSince(claw.last_heartbeat)}</div>
        </div>
        <div className="bg-gray-900 border border-gray-800 rounded-xl p-4">
          <div className="text-xs text-gray-500 mb-1">注册时间</div>
          <div className="text-sm text-white">{new Date(claw.registered_at).toLocaleString('zh-CN')}</div>
        </div>
      </div>

      {/* Metrics */}
      <h2 className="text-lg font-semibold text-white mb-4">实时指标</h2>
      <div className="grid grid-cols-2 gap-4 mb-8">
        {metrics.map(({ label, value, icon: Icon, color, pct }) => (
          <div key={label} className="bg-gray-900 border border-gray-800 rounded-xl p-4">
            <div className="flex items-center justify-between mb-3">
              <div className="flex items-center gap-2">
                <Icon className={`w-4 h-4 ${color}`} />
                <span className="text-sm text-gray-400">{label}</span>
              </div>
              <span className="text-lg font-semibold text-white">{value}</span>
            </div>
            <div className="h-2 bg-gray-800 rounded-full overflow-hidden">
              <div
                className={`h-full rounded-full transition-all ${
                  pct > 80 ? 'bg-red-500' : pct > 60 ? 'bg-amber-500' : 'bg-overlord-500'
                }`}
                style={{ width: `${Math.min(pct, 100)}%` }}
              />
            </div>
          </div>
        ))}
      </div>

      {/* Additional metrics */}
      <div className="grid grid-cols-3 gap-4 mb-8">
        <div className="bg-gray-900 border border-gray-800 rounded-xl p-4">
          <div className="text-xs text-gray-500 mb-1">队列任务</div>
          <div className="text-xl font-semibold text-white">{claw.tasks_queued}</div>
        </div>
        <div className="bg-gray-900 border border-gray-800 rounded-xl p-4">
          <div className="text-xs text-gray-500 mb-1">错误率</div>
          <div className={`text-xl font-semibold ${claw.error_rate > 5 ? 'text-red-400' : 'text-white'}`}>
            {claw.error_rate.toFixed(2)}%
          </div>
        </div>
        <div className="bg-gray-900 border border-gray-800 rounded-xl p-4">
          <div className="text-xs text-gray-500 mb-1">平均延迟</div>
          <div className={`text-xl font-semibold ${claw.avg_latency_ms > 1000 ? 'text-amber-400' : 'text-white'}`}>
            {claw.avg_latency_ms}ms
          </div>
        </div>
      </div>

      {/* Quota management */}
      <h2 className="text-lg font-semibold text-white mb-4">配额管理</h2>
      <div className="bg-gray-900 border border-gray-800 rounded-xl p-6">
        <div className="grid grid-cols-2 gap-6">
          <div>
            <label className="block text-sm text-gray-400 mb-2">最大并发任务数</label>
            <input
              type="number"
              value={maxConcurrent}
              onChange={(e) => setMaxConcurrent(parseInt(e.target.value) || 0)}
              min={0}
              className="w-full bg-gray-800 border border-gray-700 rounded-lg px-3 py-2 text-white text-sm focus:outline-none focus:border-overlord-500"
            />
            <p className="text-xs text-gray-600 mt-1">0 = 不限制</p>
          </div>
          <div>
            <label className="block text-sm text-gray-400 mb-2">每日 Token 上限</label>
            <input
              type="number"
              value={maxTokensDay}
              onChange={(e) => setMaxTokensDay(parseInt(e.target.value) || 0)}
              min={0}
              className="w-full bg-gray-800 border border-gray-700 rounded-lg px-3 py-2 text-white text-sm focus:outline-none focus:border-overlord-500"
            />
            <p className="text-xs text-gray-600 mt-1">0 = 不限制</p>
          </div>
        </div>
        <div className="flex items-center gap-3 mt-4">
          <button
            onClick={handleSaveQuota}
            disabled={saving}
            className="flex items-center gap-2 px-4 py-2 bg-overlord-600 text-white text-sm rounded-lg hover:bg-overlord-500 disabled:opacity-50 transition"
          >
            <Save className="w-4 h-4" />
            {saving ? '保存中...' : '保存配额'}
          </button>
          {msg && <span className="text-sm text-overlord-300">{msg}</span>}
        </div>
      </div>
    </div>
  )
}
