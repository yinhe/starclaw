import { useState, useEffect } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { Bot, ArrowLeft, Wrench, MessageSquare, GitBranch, Zap, MessageCircle, Plug, Clock, ChevronDown, ChevronUp } from 'lucide-react'
import { agentAPI } from '../lib/api'

interface AgentSkill {
  id: string
  skill_name: string
  skill_spec: string
  version: string
}

interface Agent {
  id: string
  name: string
  description: string
  system_prompt: string
  model_id: string
  model_name: string
  tools: string
  knowledge_base_id: string
  is_public: boolean
  created_at: string
  skills?: AgentSkill[]
}

interface SkillSpec {
  trigger: 'passive' | 'proactive'
  description?: string
  schedule?: string
  tools?: string[]
  example_triggers?: string[]
  auto_execute?: boolean
  notify?: boolean
}

const toolLabels: Record<string, string> = {
  video_generation: '视频生成',
  dubbing: '配音字幕',
  mv_production: 'MV合成',
  comic_production: '漫剧制作',
  music_generation: '音乐生成',
  image_generation: '图片生成',
  code: '代码执行',
  web_search: '网页搜索',
  browser: '浏览器',
  http_request: 'HTTP请求',
  system: '系统管理',
}

const toolColors: Record<string, string> = {
  video_generation: 'bg-blue-100 text-blue-700 border-blue-200',
  dubbing: 'bg-purple-100 text-purple-700 border-purple-200',
  mv_production: 'bg-fuchsia-100 text-fuchsia-700 border-fuchsia-200',
  comic_production: 'bg-orange-100 text-orange-700 border-orange-200',
  music_generation: 'bg-pink-100 text-pink-700 border-pink-200',
  image_generation: 'bg-cyan-100 text-cyan-700 border-cyan-200',
  code: 'bg-emerald-100 text-emerald-700 border-emerald-200',
  web_search: 'bg-amber-100 text-amber-700 border-amber-200',
  browser: 'bg-indigo-100 text-indigo-700 border-indigo-200',
  http_request: 'bg-teal-100 text-teal-700 border-teal-200',
  system: 'bg-rose-100 text-rose-700 border-rose-200',
}

export default function AgentDetailPage() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const [agent, setAgent] = useState<Agent | null>(null)
  const [loading, setLoading] = useState(true)
  const [navigating, setNavigating] = useState(false)

  useEffect(() => {
    if (!id) return
    setLoading(true)
    agentAPI.get(id).then(res => {
      setAgent(res.data.agent || res.data)
    }).catch(() => {
      navigate('/agents')
    }).finally(() => setLoading(false))
  }, [id, navigate])

  const tools: string[] = agent?.tools ? (() => { try { return JSON.parse(agent.tools) } catch { return [] } })() : []

  const handleGoToWorkflow = async () => {
    if (!id) return
    setNavigating(true)
    try {
      const res = await agentAPI.getWorkflow(id)
      const wfId = res.data.workflow_id
      if (wfId) {
        navigate(`/workflows/editor?id=${wfId}`)
        return
      }
    } catch { /* ignore */ }
    // Fallback
    navigate('/workflows/editor')
    setNavigating(false)
  }

  if (loading) {
    return (
      <div className="flex items-center justify-center h-full">
        <div className="animate-spin w-8 h-8 border-2 border-primary-500 border-t-transparent rounded-full" />
      </div>
    )
  }

  if (!agent) return null

  return (
    <div className="h-full overflow-y-auto">
      <div className="max-w-5xl mx-auto p-8">
        {/* Header */}
        <button
          onClick={() => navigate('/agents')}
          className="flex items-center gap-1.5 text-sm text-gray-500 hover:text-gray-700 mb-6 transition-colors"
        >
          <ArrowLeft className="w-4 h-4" />
          返回 Agents
        </button>

        <div className="bg-white border rounded-2xl p-6 mb-6">
          <div className="flex items-start gap-4">
            <div className="w-14 h-14 bg-primary-100 rounded-xl flex items-center justify-center flex-shrink-0">
              <Bot className="w-7 h-7 text-primary-600" />
            </div>
            <div className="flex-1 min-w-0">
              <div className="flex items-center gap-3">
                <h1 className="text-2xl font-bold text-gray-900">{agent.name}</h1>
                {agent.is_public && (
                  <span className="px-2 py-0.5 bg-green-50 text-green-600 text-xs rounded-full border border-green-200">公开</span>
                )}
              </div>
              <p className="text-gray-500 mt-1">{agent.description || '暂无描述'}</p>

              {/* Quick actions */}
              <div className="flex gap-3 mt-5">
                <button
                  onClick={() => navigate('/chat')}
                  className="flex items-center gap-2 px-4 py-2 bg-primary-600 text-white rounded-lg text-sm hover:bg-primary-700 transition-colors"
                >
                  <MessageSquare className="w-4 h-4" />
                  开始对话
                </button>
                <button
                  onClick={handleGoToWorkflow}
                  disabled={navigating}
                  className="flex items-center gap-2 px-4 py-2 border-2 border-primary-300 text-primary-700 bg-primary-50 rounded-lg text-sm hover:bg-primary-100 transition-colors font-medium disabled:opacity-50"
                >
                  <GitBranch className="w-4 h-4" />
                  {navigating ? '加载中...' : '查看工作流'}
                </button>
              </div>
            </div>
          </div>
        </div>

        <SkillsSection skills={agent.skills || []} />

        <MCPSection tools={tools} />

        <ToolsSection tools={tools} />

        {/* System Prompt */}
        <div className="bg-white border rounded-2xl p-6">
          <h2 className="text-lg font-semibold text-gray-900 mb-3">System Prompt</h2>
          <div className="bg-gray-50 rounded-xl p-4 max-h-80 overflow-y-auto">
            <pre className="text-sm text-gray-700 whitespace-pre-wrap font-sans leading-relaxed">
              {agent.system_prompt || '(未设置)'}
            </pre>
          </div>
        </div>
      </div>

    </div>
  )
}

// ── Skills Section ──

const skillIcons: Record<string, string> = {
  '个股诊断': '📊', '持仓复盘': '📋', '风险排查': '🛡️', '全市场扫描': '🔍', '市场解读': '📈',
  '盘前分析': '🌅', '自动扫描选股': '🔄', '持仓实时监控': '👁️', '日终复盘': '📝',
}

function SkillsSection({ skills }: { skills: AgentSkill[] }) {
  if (!skills.length) return null

  const passive = skills.filter(s => {
    try { return JSON.parse(s.skill_spec).trigger === 'passive' } catch { return false }
  })
  const active = skills.filter(s => {
    try { return JSON.parse(s.skill_spec).trigger === 'proactive' } catch { return false }
  })

  return (
    <>
      {passive.length > 0 && (
        <div className="bg-white border rounded-2xl p-6 mb-6">
          <h2 className="text-lg font-semibold text-gray-900 mb-1 flex items-center gap-2">
            <MessageCircle className="w-5 h-5 text-blue-500" />
            被动技能
            <span className="text-xs font-normal text-gray-400">你问我答</span>
          </h2>
          <p className="text-xs text-gray-400 mb-4">用户提问时自动触发对应能力</p>
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3">
            {passive.map(s => {
              let spec: SkillSpec = { trigger: 'passive' }
              try { spec = JSON.parse(s.skill_spec) } catch {}
              return (
                <div key={s.id} className="border rounded-xl p-4 hover:border-blue-300 transition-colors bg-blue-50/30">
                  <div className="flex items-center gap-2 mb-1.5">
                    <span className="text-lg">{skillIcons[s.skill_name] || '⚡'}</span>
                    <span className="font-semibold text-gray-800 text-sm">{s.skill_name}</span>
                  </div>
                  <p className="text-xs text-gray-500 leading-relaxed">{spec.description || ''}</p>
                  {spec.example_triggers && spec.example_triggers.length > 0 && (
                    <div className="mt-2 flex flex-wrap gap-1">
                      {spec.example_triggers.map((t, i) => (
                        <span key={i} className="text-[10px] bg-blue-100 text-blue-600 px-1.5 py-0.5 rounded">"{t}"</span>
                      ))}
                    </div>
                  )}
                </div>
              )
            })}
          </div>
        </div>
      )}

      {active.length > 0 && (
        <div className="bg-white border rounded-2xl p-6 mb-6">
          <h2 className="text-lg font-semibold text-gray-900 mb-1 flex items-center gap-2">
            <Zap className="w-5 h-5 text-amber-500" />
            主动技能
            <span className="text-xs font-normal text-gray-400">自动执行</span>
          </h2>
          <p className="text-xs text-gray-400 mb-4">按计划自动运行，无需手动触发</p>
          <div className="space-y-2">
            {active.map(s => {
              let spec: SkillSpec = { trigger: 'proactive' }
              try { spec = JSON.parse(s.skill_spec) } catch {}
              return (
                <div key={s.id} className="flex items-center gap-4 border rounded-xl p-3 hover:border-amber-300 transition-colors bg-amber-50/30">
                  <span className="text-lg">{skillIcons[s.skill_name] || '⚡'}</span>
                  <div className="flex-1 min-w-0">
                    <div className="font-semibold text-gray-800 text-sm">{s.skill_name}</div>
                    <div className="text-xs text-gray-500">{spec.description || ''}</div>
                  </div>
                  {spec.schedule && (
                    <div className="flex items-center gap-1 text-xs text-amber-600 bg-amber-100 px-2 py-1 rounded-lg flex-shrink-0">
                      <Clock className="w-3 h-3" />
                      {spec.schedule}
                    </div>
                  )}
                </div>
              )
            })}
          </div>
        </div>
      )}
    </>
  )
}

// ── MCP Section ──

function MCPSection({ tools }: { tools: string[] }) {
  const mcpTools = tools.filter(t => t.startsWith('mcp_'))
  if (!mcpTools.length) return null

  return (
    <div className="bg-white border rounded-2xl p-6 mb-6">
      <h2 className="text-lg font-semibold text-gray-900 mb-1 flex items-center gap-2">
        <Plug className="w-5 h-5 text-purple-500" />
        MCP 外接服务
      </h2>
      <p className="text-xs text-gray-400 mb-4">通过 MCP 协议连接的外部工具服务</p>
      <div className="space-y-2">
        {mcpTools.map(t => (
          <div key={t} className="flex items-center gap-3 border rounded-xl p-3 bg-purple-50/30">
            <div className="w-2 h-2 rounded-full bg-green-500 flex-shrink-0" />
            <div className="flex-1">
              <div className="font-medium text-gray-800 text-sm">{t.replace('mcp_', '').replace(/_/g, ' ')}</div>
            </div>
            <span className="text-xs text-purple-500 bg-purple-100 px-2 py-0.5 rounded">MCP 外接</span>
          </div>
        ))}
      </div>
    </div>
  )
}

// ── Tools Section (collapsible) ──

function ToolsSection({ tools }: { tools: string[] }) {
  const [expanded, setExpanded] = useState(false)
  if (!tools.length) return null

  return (
    <div className="bg-white border rounded-2xl p-6 mb-6">
      <button
        onClick={() => setExpanded(!expanded)}
        className="flex items-center justify-between w-full"
      >
        <h2 className="text-lg font-semibold text-gray-900 flex items-center gap-2">
          <Wrench className="w-5 h-5 text-gray-400" />
          全部工具
          <span className="text-xs font-normal text-gray-400">({tools.length})</span>
        </h2>
        {expanded ? <ChevronUp className="w-4 h-4 text-gray-400" /> : <ChevronDown className="w-4 h-4 text-gray-400" />}
      </button>
      {expanded && (
        <div className="flex flex-wrap gap-2 mt-4">
          {tools.map(t => (
            <span key={t} className={`px-2.5 py-1 text-xs rounded-lg border font-medium ${
              t.startsWith('mcp_') ? 'bg-purple-50 text-purple-600 border-purple-200' :
              toolColors[t] || 'bg-gray-100 text-gray-600 border-gray-200'
            }`}>
              {toolLabels[t] || t}
            </span>
          ))}
        </div>
      )}
    </div>
  )
}
