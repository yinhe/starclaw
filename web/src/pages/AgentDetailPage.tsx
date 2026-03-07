import { useState, useEffect } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { Bot, ArrowLeft, Wrench, MessageSquare, GitBranch } from 'lucide-react'
import { agentAPI } from '../lib/api'

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

              {/* Tools */}
              {tools.length > 0 && (
                <div className="flex flex-wrap gap-2 mt-4">
                  {tools.map(t => (
                    <span key={t} className={`px-2.5 py-1 text-xs rounded-lg border font-medium ${toolColors[t] || 'bg-gray-100 text-gray-600 border-gray-200'}`}>
                      <Wrench className="w-3 h-3 inline mr-1" />
                      {toolLabels[t] || t}
                    </span>
                  ))}
                </div>
              )}

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
