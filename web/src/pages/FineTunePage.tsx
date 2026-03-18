import { useState, useEffect } from 'react'
import { Beaker, Brain, Database, Download, FlaskConical, Plus, RefreshCw, Play, Trash2 } from 'lucide-react'
import { fineTuneAPI } from '../lib/api'

type Tab = 'adapters' | 'distillation' | 'stats'

export default function FineTunePage() {
  const [tab, setTab] = useState<Tab>('adapters')
  const [adapters, setAdapters] = useState<any[]>([])
  const [jobs, setJobs] = useState<any[]>([])
  const [stats, setStats] = useState<any>(null)
  const [loading, setLoading] = useState(false)

  const [showCreateAdapter, setShowCreateAdapter] = useState(false)
  const [newAdapter, setNewAdapter] = useState({ name: '', base_model: 'llama-3-8b', rank: 16, alpha: 32, training_epochs: 3, learning_rate: 0.0002, batch_size: 4 })

  const [showCreateJob, setShowCreateJob] = useState(false)
  const [newJob, setNewJob] = useState({ name: '', teacher_model: 'gpt-4o', student_model: 'llama-3-8b', target_count: 1000 })

  const [selectedAdapter, setSelectedAdapter] = useState<string | null>(null)
  const [samples, setSamples] = useState<any[]>([])
  const [showAddSample, setShowAddSample] = useState(false)
  const [newSample, setNewSample] = useState({ input: '', output: '', system: '' })

  useEffect(() => { loadData() }, [tab])

  const loadData = async () => {
    setLoading(true)
    try {
      if (tab === 'adapters') {
        const r = await fineTuneAPI.listAdapters({})
        setAdapters(r.data?.items || [])
      } else if (tab === 'distillation') {
        const r = await fineTuneAPI.listDistillation({})
        setJobs(r.data?.items || [])
      } else if (tab === 'stats') {
        const r = await fineTuneAPI.stats()
        setStats(r.data)
      }
    } catch {}
    setLoading(false)
  }

  const createAdapter = async () => {
    await fineTuneAPI.createAdapter(newAdapter)
    setShowCreateAdapter(false)
    setNewAdapter({ name: '', base_model: 'llama-3-8b', rank: 16, alpha: 32, training_epochs: 3, learning_rate: 0.0002, batch_size: 4 })
    loadData()
  }

  const deleteAdapter = async (id: string) => {
    if (!confirm('确认删除此适配器？')) return
    await fineTuneAPI.deleteAdapter(id)
    loadData()
  }

  const startTraining = async (id: string) => {
    try {
      await fineTuneAPI.startTraining(id)
      loadData()
    } catch (e: any) {
      alert(e.response?.data?.error || '启动训练失败')
    }
  }

  const exportSamples = async (id: string) => {
    try {
      const r = await fineTuneAPI.exportSamples(id)
      const url = URL.createObjectURL(new Blob([r.data as any]))
      const a = document.createElement('a'); a.href = url; a.download = `training-data-${id}.jsonl`; a.click()
    } catch {}
  }

  const loadSamples = async (id: string) => {
    setSelectedAdapter(id)
    const r = await fineTuneAPI.listSamples(id, { page_size: 50 })
    setSamples(r.data?.items || [])
  }

  const addSample = async () => {
    if (!selectedAdapter) return
    await fineTuneAPI.addSample(selectedAdapter, newSample)
    setShowAddSample(false)
    setNewSample({ input: '', output: '', system: '' })
    loadSamples(selectedAdapter)
  }

  const deleteSample = async (sampleId: string) => {
    await fineTuneAPI.deleteSample(sampleId)
    if (selectedAdapter) loadSamples(selectedAdapter)
  }

  const createJob = async () => {
    await fineTuneAPI.createDistillation(newJob)
    setShowCreateJob(false)
    setNewJob({ name: '', teacher_model: 'gpt-4o', student_model: 'llama-3-8b', target_count: 1000 })
    loadData()
  }

  const cancelJob = async (id: string) => {
    await fineTuneAPI.cancelDistillation(id)
    loadData()
  }

  const statusColors: Record<string, string> = {
    pending: 'bg-gray-100 text-gray-600', training: 'bg-blue-100 text-blue-700',
    ready: 'bg-green-100 text-green-700', failed: 'bg-red-100 text-red-700',
    generating: 'bg-purple-100 text-purple-700', completed: 'bg-green-100 text-green-700',
  }

  return (
    <div className="p-6 max-w-7xl mx-auto">
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-2xl font-bold text-gray-900 dark:text-white flex items-center gap-2"><Brain className="w-6 h-6 text-primary-600" /> 微调 & 蒸馏</h1>
          <p className="text-sm text-gray-500 mt-1">LoRA 适配器管理、训练数据、知识蒸馏管线</p>
        </div>
        <button onClick={loadData} className="flex items-center gap-2 px-3 py-2 text-sm bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg hover:bg-gray-50">
          <RefreshCw className={`w-4 h-4 ${loading ? 'animate-spin' : ''}`} />
        </button>
      </div>

      <div className="flex gap-1 mb-6 bg-gray-100 dark:bg-gray-800 p-1 rounded-lg w-fit">
        {([['adapters', 'LoRA 适配器', <Database className="w-4 h-4" key="a" />], ['distillation', '知识蒸馏', <FlaskConical className="w-4 h-4" key="d" />], ['stats', '统计', <Beaker className="w-4 h-4" key="s" />]] as [Tab, string, React.ReactNode][]).map(([k, l, i]) => (
          <button key={k} onClick={() => { setTab(k); setSelectedAdapter(null) }}
            className={`flex items-center gap-1.5 px-3 py-1.5 text-sm rounded-md transition ${tab === k ? 'bg-white dark:bg-gray-700 shadow text-primary-600 font-medium' : 'text-gray-500 hover:text-gray-700'}`}>
            {i} {l}
          </button>
        ))}
      </div>

      {/* Adapters */}
      {tab === 'adapters' && !selectedAdapter && (
        <div>
          <div className="flex justify-end mb-4">
            <button onClick={() => setShowCreateAdapter(true)} className="flex items-center gap-1.5 px-4 py-2 text-sm bg-primary-600 text-white rounded-lg hover:bg-primary-700"><Plus className="w-4 h-4" /> 新建适配器</button>
          </div>
          {showCreateAdapter && (
            <div className="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-4 mb-4 space-y-3">
              <input value={newAdapter.name} onChange={e => setNewAdapter({ ...newAdapter, name: e.target.value })} placeholder="适配器名称" className="w-full px-3 py-2 text-sm bg-gray-50 dark:bg-gray-900 border border-gray-200 dark:border-gray-700 rounded-lg" />
              <div className="grid grid-cols-3 gap-2">
                <select value={newAdapter.base_model} onChange={e => setNewAdapter({ ...newAdapter, base_model: e.target.value })} className="px-3 py-2 text-sm bg-gray-50 dark:bg-gray-900 border border-gray-200 dark:border-gray-700 rounded-lg">
                  <option value="llama-3-8b">Llama 3 8B</option><option value="llama-3-70b">Llama 3 70B</option>
                  <option value="mistral-7b">Mistral 7B</option><option value="qwen-2-7b">Qwen2 7B</option>
                </select>
                <div className="flex gap-1">
                  <input type="number" value={newAdapter.rank} onChange={e => setNewAdapter({ ...newAdapter, rank: parseInt(e.target.value) || 16 })} className="w-full px-3 py-2 text-sm bg-gray-50 dark:bg-gray-900 border border-gray-200 dark:border-gray-700 rounded-lg" placeholder="Rank" />
                  <input type="number" value={newAdapter.alpha} onChange={e => setNewAdapter({ ...newAdapter, alpha: parseInt(e.target.value) || 32 })} className="w-full px-3 py-2 text-sm bg-gray-50 dark:bg-gray-900 border border-gray-200 dark:border-gray-700 rounded-lg" placeholder="Alpha" />
                </div>
                <input type="number" value={newAdapter.training_epochs} onChange={e => setNewAdapter({ ...newAdapter, training_epochs: parseInt(e.target.value) || 3 })} className="px-3 py-2 text-sm bg-gray-50 dark:bg-gray-900 border border-gray-200 dark:border-gray-700 rounded-lg" placeholder="Epochs" />
              </div>
              <div className="flex gap-2 justify-end">
                <button onClick={() => setShowCreateAdapter(false)} className="px-3 py-1.5 text-sm text-gray-500">取消</button>
                <button onClick={createAdapter} disabled={!newAdapter.name} className="px-4 py-1.5 text-sm bg-primary-600 text-white rounded-lg hover:bg-primary-700 disabled:opacity-50">创建</button>
              </div>
            </div>
          )}
          <div className="space-y-2">
            {adapters.map((a: any) => (
              <div key={a.id} className="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-4 flex items-center justify-between">
                <div className="cursor-pointer" onClick={() => loadSamples(a.id)}>
                  <div className="flex items-center gap-2">
                    <p className="font-medium text-gray-900 dark:text-white">{a.name}</p>
                    <span className={`px-2 py-0.5 text-xs rounded ${statusColors[a.status] || 'bg-gray-100 text-gray-600'}`}>{a.status}</span>
                  </div>
                  <p className="text-xs text-gray-500 mt-0.5">{a.base_model} · r={a.rank} α={a.alpha} · {a.training_samples} 样本 · {a.training_epochs} epochs</p>
                </div>
                <div className="flex items-center gap-2">
                  {a.status === 'pending' && <button onClick={() => startTraining(a.id)} className="p-1.5 hover:bg-green-50 rounded text-green-600" title="开始训练"><Play className="w-4 h-4" /></button>}
                  <button onClick={() => exportSamples(a.id)} className="p-1.5 hover:bg-gray-100 dark:hover:bg-gray-700 rounded text-gray-400" title="导出 JSONL"><Download className="w-4 h-4" /></button>
                  <button onClick={() => deleteAdapter(a.id)} className="p-1.5 hover:bg-red-50 rounded text-red-500"><Trash2 className="w-4 h-4" /></button>
                </div>
              </div>
            ))}
            {adapters.length === 0 && <p className="text-center text-gray-400 py-8">暂无 LoRA 适配器</p>}
          </div>
        </div>
      )}

      {/* Adapter Samples Detail */}
      {tab === 'adapters' && selectedAdapter && (
        <div>
          <button onClick={() => setSelectedAdapter(null)} className="mb-4 text-sm text-primary-600 hover:underline">← 返回适配器列表</button>
          <div className="flex justify-between items-center mb-4">
            <h3 className="font-medium text-gray-900 dark:text-white">训练样本</h3>
            <button onClick={() => setShowAddSample(true)} className="flex items-center gap-1.5 px-3 py-1.5 text-sm bg-primary-600 text-white rounded-lg hover:bg-primary-700"><Plus className="w-4 h-4" /> 添加样本</button>
          </div>
          {showAddSample && (
            <div className="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-4 mb-4 space-y-3">
              <textarea value={newSample.system} onChange={e => setNewSample({ ...newSample, system: e.target.value })} placeholder="System prompt (可选)" rows={2} className="w-full px-3 py-2 text-sm bg-gray-50 dark:bg-gray-900 border border-gray-200 dark:border-gray-700 rounded-lg" />
              <textarea value={newSample.input} onChange={e => setNewSample({ ...newSample, input: e.target.value })} placeholder="用户输入 (必填)" rows={3} className="w-full px-3 py-2 text-sm bg-gray-50 dark:bg-gray-900 border border-gray-200 dark:border-gray-700 rounded-lg" />
              <textarea value={newSample.output} onChange={e => setNewSample({ ...newSample, output: e.target.value })} placeholder="期望输出 (必填)" rows={3} className="w-full px-3 py-2 text-sm bg-gray-50 dark:bg-gray-900 border border-gray-200 dark:border-gray-700 rounded-lg" />
              <div className="flex gap-2 justify-end">
                <button onClick={() => setShowAddSample(false)} className="px-3 py-1.5 text-sm text-gray-500">取消</button>
                <button onClick={addSample} disabled={!newSample.input || !newSample.output} className="px-4 py-1.5 text-sm bg-primary-600 text-white rounded-lg hover:bg-primary-700 disabled:opacity-50">添加</button>
              </div>
            </div>
          )}
          <div className="space-y-2">
            {samples.map((s: any) => (
              <div key={s.id} className="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-3">
                <div className="flex justify-between items-start">
                  <div className="min-w-0 flex-1 space-y-1">
                    {s.system && <p className="text-xs text-purple-500 bg-purple-50 dark:bg-purple-900/20 px-2 py-0.5 rounded inline-block">system: {s.system.slice(0, 60)}</p>}
                    <p className="text-sm text-gray-700 dark:text-gray-300"><span className="text-xs font-medium text-blue-600 mr-1">IN:</span>{s.input.slice(0, 100)}</p>
                    <p className="text-sm text-gray-500"><span className="text-xs font-medium text-green-600 mr-1">OUT:</span>{s.output.slice(0, 100)}</p>
                  </div>
                  <button onClick={() => deleteSample(s.id)} className="p-1 hover:bg-red-50 rounded text-red-400 shrink-0 ml-2"><Trash2 className="w-3.5 h-3.5" /></button>
                </div>
              </div>
            ))}
            {samples.length === 0 && <p className="text-center text-gray-400 py-8">暂无训练样本，点击"添加样本"开始</p>}
          </div>
        </div>
      )}

      {/* Distillation */}
      {tab === 'distillation' && (
        <div>
          <div className="flex justify-end mb-4">
            <button onClick={() => setShowCreateJob(true)} className="flex items-center gap-1.5 px-4 py-2 text-sm bg-primary-600 text-white rounded-lg hover:bg-primary-700"><Plus className="w-4 h-4" /> 新建蒸馏任务</button>
          </div>
          {showCreateJob && (
            <div className="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-4 mb-4 space-y-3">
              <input value={newJob.name} onChange={e => setNewJob({ ...newJob, name: e.target.value })} placeholder="任务名称" className="w-full px-3 py-2 text-sm bg-gray-50 dark:bg-gray-900 border border-gray-200 dark:border-gray-700 rounded-lg" />
              <div className="grid grid-cols-3 gap-2">
                <select value={newJob.teacher_model} onChange={e => setNewJob({ ...newJob, teacher_model: e.target.value })} className="px-3 py-2 text-sm bg-gray-50 dark:bg-gray-900 border border-gray-200 dark:border-gray-700 rounded-lg">
                  <option value="gpt-4o">GPT-4o (Teacher)</option><option value="claude-3-opus">Claude 3 Opus</option><option value="gemini-2.0-flash">Gemini 2.0 Flash</option>
                </select>
                <select value={newJob.student_model} onChange={e => setNewJob({ ...newJob, student_model: e.target.value })} className="px-3 py-2 text-sm bg-gray-50 dark:bg-gray-900 border border-gray-200 dark:border-gray-700 rounded-lg">
                  <option value="llama-3-8b">Llama 3 8B (Student)</option><option value="mistral-7b">Mistral 7B</option><option value="qwen-2-7b">Qwen2 7B</option>
                </select>
                <input type="number" value={newJob.target_count} onChange={e => setNewJob({ ...newJob, target_count: parseInt(e.target.value) || 1000 })} className="px-3 py-2 text-sm bg-gray-50 dark:bg-gray-900 border border-gray-200 dark:border-gray-700 rounded-lg" placeholder="目标样本数" />
              </div>
              <div className="flex gap-2 justify-end">
                <button onClick={() => setShowCreateJob(false)} className="px-3 py-1.5 text-sm text-gray-500">取消</button>
                <button onClick={createJob} disabled={!newJob.name} className="px-4 py-1.5 text-sm bg-primary-600 text-white rounded-lg hover:bg-primary-700 disabled:opacity-50">创建</button>
              </div>
            </div>
          )}
          <div className="space-y-2">
            {jobs.map((j: any) => (
              <div key={j.id} className="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-4 flex items-center justify-between">
                <div>
                  <div className="flex items-center gap-2">
                    <p className="font-medium text-gray-900 dark:text-white">{j.name}</p>
                    <span className={`px-2 py-0.5 text-xs rounded ${statusColors[j.status] || 'bg-gray-100 text-gray-600'}`}>{j.status}</span>
                  </div>
                  <p className="text-xs text-gray-500 mt-0.5">{j.teacher_model} → {j.student_model} · {j.generated_count}/{j.target_count} 样本 · {((j.progress || 0) * 100).toFixed(0)}%</p>
                  <div className="mt-1.5 w-48 bg-gray-100 dark:bg-gray-700 rounded-full h-1">
                    <div className="bg-primary-600 h-1 rounded-full transition-all" style={{ width: `${(j.progress || 0) * 100}%` }} />
                  </div>
                </div>
                {(j.status === 'pending' || j.status === 'generating') && (
                  <button onClick={() => cancelJob(j.id)} className="px-3 py-1 text-xs text-red-500 border border-red-200 rounded hover:bg-red-50">取消</button>
                )}
              </div>
            ))}
            {jobs.length === 0 && <p className="text-center text-gray-400 py-8">暂无蒸馏任务</p>}
          </div>
        </div>
      )}

      {/* Stats */}
      {tab === 'stats' && stats && (
        <div className="grid grid-cols-2 md:grid-cols-3 gap-4">
          {[
            { label: '总适配器', value: stats.total_adapters ?? 0 },
            { label: '就绪适配器', value: stats.ready_adapters ?? 0 },
            { label: '训练中', value: stats.training_adapters ?? 0 },
            { label: '总样本数', value: stats.total_samples ?? 0 },
            { label: '蒸馏任务', value: stats.total_jobs ?? 0 },
            { label: '活跃任务', value: stats.active_jobs ?? 0 },
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
