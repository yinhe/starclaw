import { useEffect, useState } from 'react'
import { FileText, Sparkles, Check, ListTree, Loader2 } from 'lucide-react'
import { api } from '../api'

export default function PRDPage() {
  const [projects, setProjects] = useState<any[]>([])
  const [selectedProject, setSelectedProject] = useState('')
  const [prompt, setPrompt] = useState('')
  const [prd, setPrd] = useState<any>(null)
  const [plan, setPlan] = useState<any>(null)
  const [generating, setGenerating] = useState(false)
  const [planning, setPlanning] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => {
    api.listProjects().then((r) => {
      const p = r.projects || []
      setProjects(p)
      if (p.length > 0) setSelectedProject(p[0].id)
    })
  }, [])

  const handleGenerate = async () => {
    if (!prompt.trim() || !selectedProject) return
    setGenerating(true)
    setError('')
    setPrd(null)
    setPlan(null)
    try {
      const r = await api.generatePRD(selectedProject, prompt)
      setPrd(r.prd)
    } catch (e: any) {
      setError(e.message || '生成失败')
    } finally {
      setGenerating(false)
    }
  }

  const handleConfirm = async () => {
    if (!prd) return
    await api.confirmPRD(prd.id)
    setPrd({ ...prd, status: 'confirmed' })
  }

  const handlePlan = async () => {
    if (!prd) return
    setPlanning(true)
    setError('')
    try {
      const r = await api.planPRD(prd.id)
      setPlan(r)
    } catch (e: any) {
      setError(e.message || '拆分失败')
    } finally {
      setPlanning(false)
    }
  }

  return (
    <div className="p-6 space-y-6 max-w-4xl">
      {/* Header */}
      <div className="flex items-center gap-3">
        <FileText className="w-6 h-6 text-forge-400" />
        <h1 className="text-xl font-bold">PRD 生成器</h1>
        <span className="text-sm text-stone-500">自然语言 → 结构化需求文档 → Sprint 拆分</span>
      </div>

      {/* Input */}
      <div className="glass rounded-xl p-5 space-y-4">
        <div className="flex items-center gap-3">
          <select
            value={selectedProject}
            onChange={(e) => setSelectedProject(e.target.value)}
            className="bg-stone-800 border border-stone-700 rounded-lg px-3 py-2 text-sm text-stone-300 focus:outline-none focus:border-forge-500"
          >
            {projects.map((p: any) => (
              <option key={p.id} value={p.id}>{p.name}</option>
            ))}
          </select>
          <span className="text-sm text-stone-500">目标项目</span>
        </div>
        <textarea
          value={prompt}
          onChange={(e) => setPrompt(e.target.value)}
          placeholder="描述你想要实现的功能...&#10;例如: 实现记忆系统向量化，支持语义搜索，对接 Carapace 加密存储"
          rows={4}
          className="w-full bg-stone-800 border border-stone-700 rounded-lg px-4 py-3 text-sm text-stone-200 placeholder:text-stone-600 focus:outline-none focus:border-forge-500 resize-none"
        />
        <div className="flex justify-end">
          <button
            onClick={handleGenerate}
            disabled={generating || !prompt.trim()}
            className="flex items-center gap-2 px-5 py-2.5 bg-forge-600 hover:bg-forge-500 disabled:bg-stone-700 disabled:text-stone-500 text-white rounded-lg text-sm transition-colors"
          >
            {generating ? <Loader2 className="w-4 h-4 animate-spin" /> : <Sparkles className="w-4 h-4" />}
            {generating ? 'AI 生成中...' : '生成 PRD'}
          </button>
        </div>
      </div>

      {error && (
        <div className="bg-red-500/10 border border-red-500/30 rounded-lg px-4 py-3 text-sm text-red-400">
          {error}
        </div>
      )}

      {/* PRD Result */}
      {prd && (
        <div className="glass rounded-xl p-5 space-y-4">
          <div className="flex items-center justify-between">
            <h2 className="text-lg font-semibold text-stone-200">{prd.title || 'PRD'}</h2>
            <span className={`text-xs px-2 py-1 rounded-full ${
              prd.status === 'draft' ? 'bg-yellow-500/10 text-yellow-400' :
              prd.status === 'confirmed' ? 'bg-green-500/10 text-green-400' :
              prd.status === 'planned' ? 'bg-blue-500/10 text-blue-400' :
              'bg-stone-700 text-stone-400'
            }`}>
              {prd.status}
            </span>
          </div>

          {prd.objective && (
            <div>
              <h3 className="text-xs font-medium text-stone-500 mb-1">目标</h3>
              <p className="text-sm text-stone-300">{prd.objective}</p>
            </div>
          )}

          {prd.features && (
            <div>
              <h3 className="text-xs font-medium text-stone-500 mb-2">功能清单</h3>
              <div className="space-y-1.5">
                {safeParseJSON(prd.features).map((f: any, i: number) => (
                  <div key={i} className="flex items-start gap-2 text-sm">
                    <span className="text-forge-400 font-mono text-xs mt-0.5">{f.id || `F${i+1}`}</span>
                    <div>
                      <span className="text-stone-200">{f.title || f}</span>
                      {f.desc && <p className="text-xs text-stone-500 mt-0.5">{f.desc}</p>}
                      {f.service && <span className="text-[10px] px-1.5 py-0.5 rounded bg-stone-800 text-stone-400 ml-1">{f.service}</span>}
                    </div>
                  </div>
                ))}
              </div>
            </div>
          )}

          {prd.acceptance_criteria && (
            <div>
              <h3 className="text-xs font-medium text-stone-500 mb-2">验收标准</h3>
              <ul className="space-y-1">
                {safeParseJSON(prd.acceptance_criteria).map((c: string, i: number) => (
                  <li key={i} className="flex items-start gap-2 text-sm text-stone-400">
                    <Check className="w-3.5 h-3.5 text-green-500 mt-0.5 shrink-0" />
                    {c}
                  </li>
                ))}
              </ul>
            </div>
          )}

          <div className="text-xs text-stone-600">预估 Sprint 数: {prd.estimated_sprints}</div>

          {/* Actions */}
          <div className="flex gap-3 pt-2 border-t border-stone-800">
            {prd.status === 'draft' && (
              <button onClick={handleConfirm} className="flex items-center gap-2 px-4 py-2 bg-green-600/80 hover:bg-green-600 text-white rounded-lg text-sm transition-colors">
                <Check className="w-4 h-4" />
                确认 PRD
              </button>
            )}
            {(prd.status === 'confirmed' || prd.status === 'draft') && (
              <button
                onClick={handlePlan}
                disabled={planning}
                className="flex items-center gap-2 px-4 py-2 bg-blue-600/80 hover:bg-blue-600 disabled:bg-stone-700 text-white rounded-lg text-sm transition-colors"
              >
                {planning ? <Loader2 className="w-4 h-4 animate-spin" /> : <ListTree className="w-4 h-4" />}
                {planning ? '拆分中...' : '拆分为 Sprint'}
              </button>
            )}
          </div>
        </div>
      )}

      {/* Plan Result */}
      {plan && (
        <div className="glass rounded-xl p-5 space-y-4">
          <div className="flex items-center gap-2">
            <ListTree className="w-5 h-5 text-blue-400" />
            <h2 className="text-lg font-semibold">Sprint 计划</h2>
            <span className="text-sm text-stone-500 ml-auto">
              {plan.total_sprints} Sprint · {plan.total_issues} Issues
            </span>
          </div>
          {(plan.sprints || []).map((s: any, si: number) => (
            <div key={si} className="bg-stone-800/50 rounded-lg p-4">
              <h3 className="font-medium text-stone-200 mb-1">Sprint {si + 1}: {s.name}</h3>
              {s.goal && <p className="text-xs text-stone-500 mb-3">{s.goal}</p>}
            </div>
          ))}
          {(plan.issues || []).map((iss: any, ii: number) => (
            <div key={ii} className="flex items-center gap-3 text-sm py-1.5 px-3 rounded-lg hover:bg-stone-800/50">
              <span className="font-mono text-xs text-stone-500 w-16">{iss.key}</span>
              <span className="text-stone-300 flex-1">{iss.title}</span>
              <span className="text-[10px] px-1.5 py-0.5 rounded bg-stone-800 text-stone-400">{iss.service}</span>
              <span className="text-[10px] px-1.5 py-0.5 rounded bg-forge-500/10 text-forge-400">{iss.story_points}pt</span>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}

function safeParseJSON(s: string): any[] {
  if (!s) return []
  try {
    const parsed = JSON.parse(s)
    return Array.isArray(parsed) ? parsed : []
  } catch {
    return []
  }
}
