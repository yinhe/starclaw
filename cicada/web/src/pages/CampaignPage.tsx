import { useEffect, useState } from 'react'
import { Plus, Play, Pause, Square, RefreshCw } from 'lucide-react'
import { getCampaigns, createCampaign, startCampaign, pauseCampaign, stopCampaign, getCampaignProgress } from '../api'

const STATUS_LABEL: Record<string, string> = {
  pending: '待启动', running: '运行中', paused: '已暂停', completed: '已完成',
}
const STATUS_COLOR: Record<string, string> = {
  pending: 'bg-stone-600', running: 'bg-cicada-500', paused: 'bg-yellow-500', completed: 'bg-blue-500',
}

export default function CampaignPage() {
  const [campaigns, setCampaigns] = useState<any[]>([])
  const [progress, setProgress] = useState<any>(null)
  const [showCreate, setShowCreate] = useState(false)
  const [form, setForm] = useState({ name: '', industry: 'general', daily_limit: 800 })

  const load = () => {
    getCampaigns().then(r => setCampaigns(r.campaigns || [])).catch(() => {})
    getCampaignProgress().then(setProgress).catch(() => {})
  }
  useEffect(() => { load(); const t = setInterval(load, 5000); return () => clearInterval(t) }, [])

  const handleCreate = async () => {
    await createCampaign(form)
    setShowCreate(false)
    setForm({ name: '', industry: 'general', daily_limit: 800 })
    load()
  }

  const handleStart = async (id: number) => {
    await startCampaign(id, { display_num: '', script_industry: 'general' })
    load()
  }

  return (
    <div className="p-6 space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold">外呼任务</h1>
        <div className="flex gap-2">
          <button onClick={load} className="p-2 rounded-lg bg-stone-800 hover:bg-stone-700 transition">
            <RefreshCw className="w-4 h-4" />
          </button>
          <button onClick={() => setShowCreate(true)} className="flex items-center gap-1.5 px-3 py-2 rounded-lg bg-cicada-600 hover:bg-cicada-500 text-white text-sm font-medium transition">
            <Plus className="w-4 h-4" /> 新建任务
          </button>
        </div>
      </div>

      {/* Live Progress */}
      {progress?.running && (
        <div className="bg-cicada-500/10 border border-cicada-500/30 rounded-xl p-4">
          <div className="flex items-center gap-3">
            <div className="w-2 h-2 rounded-full bg-cicada-400 animate-pulse" />
            <span className="text-sm font-medium text-cicada-400">正在外呼</span>
            <span className="text-sm text-stone-400 ml-auto">
              今日 {progress.today_called}/{progress.daily_limit} | 并发 {progress.active_calls}/{progress.max_concurrent}
            </span>
          </div>
          <div className="mt-2 h-1.5 bg-stone-800 rounded-full overflow-hidden">
            <div className="h-full bg-cicada-500 rounded-full transition-all" style={{ width: `${(progress.today_called / progress.daily_limit) * 100}%` }} />
          </div>
        </div>
      )}

      {/* Create Modal */}
      {showCreate && (
        <div className="bg-stone-900 border border-stone-700 rounded-xl p-5 space-y-4">
          <h3 className="font-medium">新建外呼任务</h3>
          <div className="grid grid-cols-3 gap-4">
            <div>
              <label className="text-xs text-stone-500 block mb-1">任务名称</label>
              <input value={form.name} onChange={e => setForm({ ...form, name: e.target.value })}
                className="w-full bg-stone-800 border border-stone-700 rounded-lg px-3 py-2 text-sm" placeholder="例：Q2房产外呼" />
            </div>
            <div>
              <label className="text-xs text-stone-500 block mb-1">行业</label>
              <select value={form.industry} onChange={e => setForm({ ...form, industry: e.target.value })}
                className="w-full bg-stone-800 border border-stone-700 rounded-lg px-3 py-2 text-sm">
                <option value="real_estate">房产</option>
                <option value="education">教育</option>
                <option value="finance">金融</option>
                <option value="general">通用</option>
              </select>
            </div>
            <div>
              <label className="text-xs text-stone-500 block mb-1">日呼上限</label>
              <input type="number" value={form.daily_limit} onChange={e => setForm({ ...form, daily_limit: +e.target.value })}
                className="w-full bg-stone-800 border border-stone-700 rounded-lg px-3 py-2 text-sm" />
            </div>
          </div>
          <div className="flex gap-2 justify-end">
            <button onClick={() => setShowCreate(false)} className="px-3 py-1.5 text-sm rounded-lg bg-stone-800 hover:bg-stone-700">取消</button>
            <button onClick={handleCreate} disabled={!form.name} className="px-3 py-1.5 text-sm rounded-lg bg-cicada-600 hover:bg-cicada-500 text-white disabled:opacity-50">创建</button>
          </div>
        </div>
      )}

      {/* Campaign List */}
      <div className="bg-stone-900 border border-stone-800 rounded-xl overflow-hidden">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-stone-800 text-stone-500 text-xs">
              <th className="text-left px-4 py-3 font-medium">任务名称</th>
              <th className="text-left px-4 py-3 font-medium">行业</th>
              <th className="text-center px-4 py-3 font-medium">状态</th>
              <th className="text-center px-4 py-3 font-medium">已呼/总数</th>
              <th className="text-center px-4 py-3 font-medium">接通</th>
              <th className="text-center px-4 py-3 font-medium">A类</th>
              <th className="text-center px-4 py-3 font-medium">B类</th>
              <th className="text-right px-4 py-3 font-medium">操作</th>
            </tr>
          </thead>
          <tbody>
            {campaigns.length === 0 ? (
              <tr><td colSpan={8} className="text-center py-8 text-stone-600">暂无外呼任务</td></tr>
            ) : campaigns.map((c: any) => (
              <tr key={c.id} className="border-b border-stone-800/50 hover:bg-stone-800/30">
                <td className="px-4 py-3 font-medium">{c.name}</td>
                <td className="px-4 py-3 text-stone-400">{c.industry}</td>
                <td className="px-4 py-3 text-center">
                  <span className={`inline-block px-2 py-0.5 rounded text-xs text-white ${STATUS_COLOR[c.status] || 'bg-stone-600'}`}>
                    {STATUS_LABEL[c.status] || c.status}
                  </span>
                </td>
                <td className="px-4 py-3 text-center text-stone-400">{c.called ?? 0}/{c.total ?? 0}</td>
                <td className="px-4 py-3 text-center text-stone-400">{c.connected ?? 0}</td>
                <td className="px-4 py-3 text-center text-red-400 font-medium">{c.intent_a ?? 0}</td>
                <td className="px-4 py-3 text-center text-orange-400">{c.intent_b ?? 0}</td>
                <td className="px-4 py-3 text-right">
                  <div className="flex gap-1 justify-end">
                    {c.status === 'pending' && (
                      <button onClick={() => handleStart(c.id)} className="p-1.5 rounded bg-cicada-600/20 hover:bg-cicada-600/40 text-cicada-400" title="启动">
                        <Play className="w-3.5 h-3.5" />
                      </button>
                    )}
                    {c.status === 'running' && (
                      <>
                        <button onClick={() => pauseCampaign().then(load)} className="p-1.5 rounded bg-yellow-600/20 hover:bg-yellow-600/40 text-yellow-400" title="暂停">
                          <Pause className="w-3.5 h-3.5" />
                        </button>
                        <button onClick={() => stopCampaign().then(load)} className="p-1.5 rounded bg-red-600/20 hover:bg-red-600/40 text-red-400" title="停止">
                          <Square className="w-3.5 h-3.5" />
                        </button>
                      </>
                    )}
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  )
}
