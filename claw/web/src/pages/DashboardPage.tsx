import { useState, useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import {
  Bot, MessageSquare, GitBranch, BookOpen, Plug, FileText, Zap, Clock,
  ArrowRight, Users, BarChart3, TrendingUp,
} from 'lucide-react'
import { dashboardAPI, modelAPI } from '../lib/api'
import OnboardingWizard from '../components/OnboardingWizard'

interface Stats {
  agents: number
  conversations: number
  workflows: number
  knowledge_bases: number
  mcp_servers: number
  documents: number
  tokens_30d: number
  messages_today: number
}

interface RecentConversation {
  id: string
  title: string
  updated_at: string
}

interface AgentUsage {
  agent_id: string
  agent_name: string
  tokens: number
  msg_count: number
}

interface DailyUsage {
  date: string
  tokens: number
  msgs: number
}

export default function DashboardPage() {
  const [stats, setStats] = useState<Stats | null>(null)
  const [recentConvos, setRecentConvos] = useState<RecentConversation[]>([])
  const [agentUsage, setAgentUsage] = useState<AgentUsage[]>([])
  const [dailyUsage, setDailyUsage] = useState<DailyUsage[]>([])
  const [showOnboarding, setShowOnboarding] = useState(false)
  const navigate = useNavigate()

  useEffect(() => {
    loadStats()
    // Check if first run (no models configured and not previously dismissed)
    if (!localStorage.getItem('starclaw_onboarded')) {
      modelAPI.list().then(res => {
        if (!res.data.models || res.data.models.length === 0) {
          setShowOnboarding(true)
        } else {
          localStorage.setItem('starclaw_onboarded', '1')
        }
      }).catch(() => {})
    }
  }, [])

  const loadStats = async () => {
    try {
      const res = await dashboardAPI.stats()
      setStats(res.data.stats)
      setRecentConvos(res.data.recent_conversations || [])
      setAgentUsage(res.data.agent_usage || [])
      setDailyUsage(res.data.daily_usage || [])
    } catch { /* ignore */ }
  }

  const statCards = stats
    ? [
        { label: 'Agents', value: stats.agents, icon: Bot, color: 'bg-blue-100 text-blue-600', to: '/agents' },
        { label: '对话', value: stats.conversations, icon: MessageSquare, color: 'bg-green-100 text-green-600', to: '/chat' },
        { label: '工作流', value: stats.workflows, icon: GitBranch, color: 'bg-purple-100 text-purple-600', to: '/workflows' },
        { label: '知识库', value: stats.knowledge_bases, icon: BookOpen, color: 'bg-amber-100 text-amber-600', to: '/knowledge' },
        { label: 'MCP 服务器', value: stats.mcp_servers, icon: Plug, color: 'bg-cyan-100 text-cyan-600', to: '/mcp' },
        { label: '文档', value: stats.documents, icon: FileText, color: 'bg-pink-100 text-pink-600', to: '/knowledge' },
      ]
    : []

  const formatDate = (d: string) => {
    try {
      return new Date(d).toLocaleString('zh-CN', { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' })
    } catch { return d }
  }

  return (
    <div className="h-full overflow-y-auto">
      {showOnboarding && <OnboardingWizard onComplete={() => setShowOnboarding(false)} />}
      <div className="max-w-6xl mx-auto p-8">
        {/* Header */}
        <div className="mb-8">
          <h1 className="text-2xl font-bold text-gray-900 dark:text-white">仪表盘</h1>
          <p className="text-gray-500 dark:text-gray-400 text-sm mt-1">StarClaw AI Agent Platform 总览</p>
        </div>

        {/* Token / Message highlights */}
        {stats && (
          <div className="grid grid-cols-1 md:grid-cols-2 gap-4 mb-6">
            <div className="bg-gradient-to-r from-primary-500 to-primary-600 rounded-xl p-5 text-white">
              <div className="flex items-center gap-3">
                <Zap className="w-8 h-8 opacity-80" />
                <div>
                  <p className="text-sm opacity-80">近 30 天 Token 用量</p>
                  <p className="text-2xl font-bold">{stats.tokens_30d.toLocaleString()}</p>
                </div>
              </div>
            </div>
            <div className="bg-gradient-to-r from-green-500 to-emerald-600 rounded-xl p-5 text-white">
              <div className="flex items-center gap-3">
                <MessageSquare className="w-8 h-8 opacity-80" />
                <div>
                  <p className="text-sm opacity-80">今日消息</p>
                  <p className="text-2xl font-bold">{stats.messages_today}</p>
                </div>
              </div>
            </div>
          </div>
        )}

        {/* Stat Cards */}
        <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-6 gap-4 mb-8">
          {statCards.map((card) => (
            <div
              key={card.label}
              onClick={() => navigate(card.to)}
              className="bg-white dark:bg-gray-800 border dark:border-gray-700 rounded-xl p-4 cursor-pointer hover:shadow-md transition-shadow"
            >
              <div className={`w-9 h-9 rounded-lg flex items-center justify-center ${card.color} mb-3`}>
                <card.icon className="w-4.5 h-4.5" />
              </div>
              <p className="text-2xl font-bold text-gray-900 dark:text-white">{card.value}</p>
              <p className="text-xs text-gray-500 mt-0.5">{card.label}</p>
            </div>
          ))}
        </div>

        {/* Usage Charts */}
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-6 mb-6">
          {/* Daily Usage Bar Chart */}
          <div className="bg-white dark:bg-gray-800 border rounded-xl p-5">
            <h3 className="text-sm font-semibold text-gray-700 dark:text-gray-200 flex items-center gap-2 mb-4">
              <TrendingUp className="w-4 h-4" /> 近 7 天用量
            </h3>
            {dailyUsage.length === 0 ? (
              <p className="text-sm text-gray-400 text-center py-8">暂无数据</p>
            ) : (
              <div className="flex items-end gap-2 h-32">
                {dailyUsage.map((d) => {
                  const maxTokens = Math.max(...dailyUsage.map((x) => x.tokens), 1)
                  const h = Math.max((d.tokens / maxTokens) * 100, 4)
                  return (
                    <div key={d.date} className="flex-1 flex flex-col items-center gap-1">
                      <span className="text-xs text-gray-500">{d.tokens > 0 ? d.tokens.toLocaleString() : ''}</span>
                      <div
                        className="w-full bg-primary-500 rounded-t-md transition-all"
                        style={{ height: `${h}%` }}
                        title={`${d.tokens} tokens, ${d.msgs} 消息`}
                      />
                      <span className="text-xs text-gray-400">{d.date.slice(5)}</span>
                    </div>
                  )
                })}
              </div>
            )}
          </div>

          {/* Agent Token Ranking */}
          <div className="bg-white dark:bg-gray-800 border rounded-xl p-5">
            <h3 className="text-sm font-semibold text-gray-700 dark:text-gray-200 flex items-center gap-2 mb-4">
              <BarChart3 className="w-4 h-4" /> Agent 用量排行
            </h3>
            {agentUsage.length === 0 ? (
              <p className="text-sm text-gray-400 text-center py-8">暂无数据</p>
            ) : (
              <div className="space-y-2.5">
                {agentUsage.slice(0, 5).map((a, i) => {
                  const maxTokens = Math.max(...agentUsage.map((x) => x.tokens), 1)
                  const pct = (a.tokens / maxTokens) * 100
                  return (
                    <div key={a.agent_id}>
                      <div className="flex items-center justify-between text-xs mb-1">
                        <span className="text-gray-700 dark:text-gray-300 font-medium">
                          <span className="text-gray-400 mr-1">#{i + 1}</span>
                          {a.agent_name}
                        </span>
                        <span className="text-gray-500">{a.tokens.toLocaleString()} tokens · {a.msg_count} 消息</span>
                      </div>
                      <div className="w-full bg-gray-100 dark:bg-gray-700 rounded-full h-1.5">
                        <div className="bg-primary-500 h-1.5 rounded-full transition-all" style={{ width: `${pct}%` }} />
                      </div>
                    </div>
                  )
                })}
              </div>
            )}
          </div>
        </div>

        <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
          {/* Recent Conversations */}
          <div className="bg-white dark:bg-gray-800 border rounded-xl p-5">
            <div className="flex items-center justify-between mb-4">
              <h3 className="text-sm font-semibold text-gray-700 dark:text-gray-200">最近对话</h3>
              <button
                onClick={() => navigate('/chat')}
                className="text-xs text-primary-600 hover:text-primary-700 flex items-center gap-1"
              >
                查看全部 <ArrowRight className="w-3 h-3" />
              </button>
            </div>
            {recentConvos.length === 0 ? (
              <p className="text-sm text-gray-400 py-4 text-center">暂无对话</p>
            ) : (
              <div className="space-y-2">
                {recentConvos.map((conv) => (
                  <div
                    key={conv.id}
                    onClick={() => navigate(`/chat/${conv.id}`)}
                    className="flex items-center justify-between p-3 rounded-lg hover:bg-gray-50 dark:hover:bg-gray-700 cursor-pointer transition-colors"
                  >
                    <div className="flex items-center gap-3">
                      <MessageSquare className="w-4 h-4 text-gray-400" />
                      <span className="text-sm text-gray-800 dark:text-gray-200 truncate max-w-[200px]">{conv.title}</span>
                    </div>
                    <div className="flex items-center gap-1 text-xs text-gray-400">
                      <Clock className="w-3 h-3" />
                      {formatDate(conv.updated_at)}
                    </div>
                  </div>
                ))}
              </div>
            )}
          </div>

          {/* Quick Actions */}
          <div className="bg-white dark:bg-gray-800 border dark:border-gray-700 rounded-xl p-5">
            <h3 className="text-sm font-semibold text-gray-700 dark:text-gray-200 mb-4">快捷操作</h3>
            <div className="grid grid-cols-2 gap-3">
              {[
                { label: '新建对话', icon: MessageSquare, to: '/chat', color: 'text-green-600 bg-green-50' },
                { label: '创建 Agent', icon: Bot, to: '/agents', color: 'text-blue-600 bg-blue-50' },
                { label: '创建工作流', icon: GitBranch, to: '/workflows/editor', color: 'text-purple-600 bg-purple-50' },
                { label: '管理知识库', icon: BookOpen, to: '/knowledge', color: 'text-amber-600 bg-amber-50' },
                { label: '多 Agent 协作', icon: Users, to: '/multi-agent', color: 'text-indigo-600 bg-indigo-50' },
                { label: '添加 MCP 工具', icon: Plug, to: '/mcp', color: 'text-cyan-600 bg-cyan-50' },
              ].map((action) => (
                <button
                  key={action.label}
                  onClick={() => navigate(action.to)}
                  className="flex items-center gap-3 p-3 rounded-lg border dark:border-gray-700 hover:border-gray-300 dark:hover:border-gray-600 transition-colors text-left"
                >
                  <div className={`w-8 h-8 rounded-lg flex items-center justify-center ${action.color}`}>
                    <action.icon className="w-4 h-4" />
                  </div>
                  <span className="text-sm text-gray-700 dark:text-gray-300">{action.label}</span>
                </button>
              ))}
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}
