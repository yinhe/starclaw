import { useState, useEffect } from 'react'
import { Users, Play, Loader2 } from 'lucide-react'
import { agentAPI, multiAgentAPI } from '../lib/api'

interface Agent {
  id: string
  name: string
  description: string
}

export default function MultiAgentPage() {
  const [agents, setAgents] = useState<Agent[]>([])
  const [selectedAgents, setSelectedAgents] = useState<string[]>([])
  const [orchestratorId, setOrchestratorId] = useState('')
  const [mode, setMode] = useState<'sequential' | 'parallel' | 'orchestrated'>('sequential')
  const [input, setInput] = useState('')
  const [running, setRunning] = useState(false)
  const [result, setResult] = useState<{ output: string; agent_outputs: Record<string, string> } | null>(null)

  useEffect(() => {
    loadAgents()
  }, [])

  const loadAgents = async () => {
    try {
      const res = await agentAPI.list()
      setAgents(res.data.agents || [])
    } catch { /* ignore */ }
  }

  const toggleAgent = (id: string) => {
    setSelectedAgents((prev) =>
      prev.includes(id) ? prev.filter((a) => a !== id) : [...prev, id],
    )
  }

  const handleRun = async () => {
    if (selectedAgents.length === 0 || !input.trim()) return
    setRunning(true)
    setResult(null)
    try {
      const res = await multiAgentAPI.run({
        agent_ids: selectedAgents,
        orchestrator_id: mode === 'orchestrated' ? orchestratorId : undefined,
        mode,
        input,
      })
      setResult(res.data)
    } catch (e: unknown) {
      const err = e as { response?: { data?: { error?: string } } }
      setResult({ output: '运行失败: ' + (err.response?.data?.error || '未知错误'), agent_outputs: {} })
    }
    setRunning(false)
  }

  return (
    <div className="h-full overflow-y-auto">
      <div className="max-w-5xl mx-auto p-8">
        <div className="mb-8">
          <h1 className="text-2xl font-bold text-gray-900">Multi-Agent 协作</h1>
          <p className="text-gray-500 text-sm mt-1">选择多个 Agent 协同完成复杂任务</p>
        </div>

        <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
          {/* Left: Config */}
          <div className="space-y-6">
            {/* Mode */}
            <div className="bg-white border rounded-xl p-5">
              <h3 className="text-sm font-semibold text-gray-700 mb-3">协作模式</h3>
              <div className="grid grid-cols-3 gap-2">
                {(['sequential', 'parallel', 'orchestrated'] as const).map((m) => (
                  <button
                    key={m}
                    onClick={() => setMode(m)}
                    className={`py-2 px-3 rounded-lg text-xs font-medium border transition-colors ${
                      mode === m
                        ? 'bg-primary-50 border-primary-300 text-primary-700'
                        : 'bg-white border-gray-200 text-gray-500 hover:border-gray-300'
                    }`}
                  >
                    {m === 'sequential' ? '顺序执行' : m === 'parallel' ? '并行执行' : '编排执行'}
                  </button>
                ))}
              </div>
              <p className="text-xs text-gray-400 mt-2">
                {mode === 'sequential' && '每个 Agent 的输出作为下一个的输入，依次执行'}
                {mode === 'parallel' && '所有 Agent 同时处理同一输入，汇总结果'}
                {mode === 'orchestrated' && '编排 Agent 分析任务并委托子 Agent 执行'}
              </p>
            </div>

            {/* Agent Selection */}
            <div className="bg-white border rounded-xl p-5">
              <h3 className="text-sm font-semibold text-gray-700 mb-3">
                选择 Agent（{selectedAgents.length} 已选）
              </h3>
              {agents.length === 0 ? (
                <p className="text-sm text-gray-400">请先创建 Agent</p>
              ) : (
                <div className="space-y-2 max-h-64 overflow-y-auto">
                  {agents.map((ag) => (
                    <label
                      key={ag.id}
                      className={`flex items-center gap-3 p-3 rounded-lg border cursor-pointer transition-colors ${
                        selectedAgents.includes(ag.id)
                          ? 'bg-primary-50 border-primary-300'
                          : 'bg-white border-gray-200 hover:border-gray-300'
                      }`}
                    >
                      <input
                        type="checkbox"
                        checked={selectedAgents.includes(ag.id)}
                        onChange={() => toggleAgent(ag.id)}
                        className="rounded"
                      />
                      <div>
                        <p className="text-sm font-medium text-gray-800">{ag.name}</p>
                        <p className="text-xs text-gray-400">{ag.description || '暂无描述'}</p>
                      </div>
                    </label>
                  ))}
                </div>
              )}
            </div>

            {/* Orchestrator (only for orchestrated mode) */}
            {mode === 'orchestrated' && (
              <div className="bg-white border rounded-xl p-5">
                <h3 className="text-sm font-semibold text-gray-700 mb-3">编排 Agent（管理者）</h3>
                <select
                  value={orchestratorId}
                  onChange={(e) => setOrchestratorId(e.target.value)}
                  className="w-full px-3 py-2 border rounded-lg text-sm outline-none focus:ring-2 focus:ring-primary-500"
                >
                  <option value="">选择编排 Agent</option>
                  {agents.map((ag) => (
                    <option key={ag.id} value={ag.id}>{ag.name}</option>
                  ))}
                </select>
                <p className="text-xs text-gray-400 mt-2">编排 Agent 负责分析任务，将子任务委派给其他 Agent</p>
              </div>
            )}

            {/* Input */}
            <div className="bg-white border rounded-xl p-5">
              <h3 className="text-sm font-semibold text-gray-700 mb-3">输入</h3>
              <textarea
                value={input}
                onChange={(e) => setInput(e.target.value)}
                rows={4}
                className="w-full px-3 py-2 border rounded-lg text-sm outline-none focus:ring-2 focus:ring-primary-500 resize-none"
                placeholder="输入要协同完成的任务..."
              />
              <button
                onClick={handleRun}
                disabled={running || selectedAgents.length === 0 || !input.trim()}
                className="mt-3 w-full flex items-center justify-center gap-2 px-4 py-2.5 bg-primary-600 text-white rounded-lg text-sm hover:bg-primary-700 disabled:opacity-50 transition-colors"
              >
                {running ? (
                  <><Loader2 className="w-4 h-4 animate-spin" /> 运行中...</>
                ) : (
                  <><Play className="w-4 h-4" /> 开始协作</>
                )}
              </button>
            </div>
          </div>

          {/* Right: Result */}
          <div>
            <div className="bg-white border rounded-xl p-5 min-h-[400px]">
              <h3 className="text-sm font-semibold text-gray-700 mb-4">执行结果</h3>
              {!result && !running && (
                <div className="text-center py-16 text-gray-400">
                  <Users className="w-10 h-10 mx-auto mb-3 opacity-50" />
                  <p className="text-sm">配置好 Agent 和输入后点击运行</p>
                </div>
              )}
              {running && (
                <div className="text-center py-16 text-gray-400">
                  <Loader2 className="w-8 h-8 mx-auto mb-3 animate-spin" />
                  <p className="text-sm">Agent 协作中...</p>
                </div>
              )}
              {result && (
                <div className="space-y-4">
                  <div>
                    <h4 className="text-xs font-medium text-gray-500 mb-2">最终输出</h4>
                    <div className="bg-gray-50 rounded-lg p-4 text-sm text-gray-800 whitespace-pre-wrap max-h-96 overflow-y-auto">
                      {result.output}
                    </div>
                  </div>
                  {Object.keys(result.agent_outputs).length > 0 && (
                    <div>
                      <h4 className="text-xs font-medium text-gray-500 mb-2">各 Agent 输出</h4>
                      <div className="space-y-2">
                        {Object.entries(result.agent_outputs).map(([id, output]) => {
                          const ag = agents.find((a) => a.id === id)
                          return (
                            <details key={id} className="bg-gray-50 rounded-lg">
                              <summary className="px-4 py-2 text-sm font-medium text-gray-700 cursor-pointer">
                                {ag?.name || id}
                              </summary>
                              <div className="px-4 pb-3 text-sm text-gray-600 whitespace-pre-wrap">
                                {output}
                              </div>
                            </details>
                          )
                        })}
                      </div>
                    </div>
                  )}
                </div>
              )}
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}
