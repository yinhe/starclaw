import { useState, useEffect } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { Bot, ArrowLeft, MessageSquare, Dna, Wrench, Zap, Plug, GitBranch, Brain, Clock, ChevronDown, ChevronUp, Sparkles, ExternalLink, Search, Film, Code, FileText, Settings, Users, Monitor, Headset } from 'lucide-react'
import { agentAPI, memoryAPI, workflowAPI, mcpAPI, activityAPI } from '../lib/api'
import { formatAgentDisplayName } from '../lib/agentDisplay'

// ── Types ──

interface AgentSkill {
  id: string
  skill_name: string
  skill_spec: string
  version: string
  ability_type: string
  instinct_category: string
  enabled: boolean
}

interface Agent {
  id: string
  name: string
  description: string
  system_prompt: string
  model_id: string
  model_name: string
  tools: string
  config: string
  knowledge_base_id: string
  is_public: boolean
  is_builtin: boolean
  gene: string
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

interface Memory {
  id: string
  key: string
  content: string
  category: string
  importance: number
  updated_at: string
}

interface Workflow {
  id: string
  name: string
  description: string
  nodes: string
  edges: string
}

interface MCPServer {
  id: string
  name: string
  base_url: string
  status: string
  tools?: string[]
}

interface Activity {
  id: string
  agent_id?: string
  name: string
  title: string
  description: string
  type: string
  trigger: string
  cooldown: string
  enabled: boolean
  last_run_at: string | null
  next_run_at: string | null
  total_runs: number
}

// ── Constants ──

type HexadTab = 'gene' | 'skill' | 'instinct' | 'mcp' | 'workflow' | 'memory'

const hexadTabs: { key: HexadTab; label: string; icon: typeof Dna; color: string; desc: string }[] = [
  { key: 'gene', label: '基因', icon: Dna, color: 'text-rose-500', desc: '人设与模型' },
  { key: 'skill', label: '技能', icon: Wrench, color: 'text-blue-500', desc: '你让我做' },
  { key: 'instinct', label: '本能', icon: Zap, color: 'text-amber-500', desc: '我自己做' },
  { key: 'mcp', label: '外接服务', icon: Plug, color: 'text-purple-500', desc: '第三方扩展' },
  { key: 'workflow', label: '工作流', icon: GitBranch, color: 'text-emerald-500', desc: '流程编排' },
  { key: 'memory', label: '记忆', icon: Brain, color: 'text-pink-500', desc: '长期记忆' },
]

// Skill grouping: tools organized by capability domain
interface SkillGroup {
  key: string
  label: string
  icon: typeof Search
  color: string
  tools: string[]
  desc: string
}

const skillGroups: SkillGroup[] = [
  { key: 'info', label: '信息获取', icon: Search, color: 'text-amber-500', tools: ['web_search', 'browser', 'http_request'], desc: '搜索、浏览、抓取互联网信息' },
  { key: 'create', label: '内容创作', icon: Film, color: 'text-blue-500', tools: ['video_generation', 'dubbing', 'mv_production', 'comic_production', 'music_generation', 'image_generation', 'audio_analysis'], desc: '视频/音乐/图片/漫剧全链路创作' },
  { key: 'dev', label: '编程开发', icon: Code, color: 'text-emerald-500', tools: ['code'], desc: '编写代码、运行调试、部署应用' },
  { key: 'doc', label: '文档处理', icon: FileText, color: 'text-cyan-500', tools: ['document'], desc: '对话总结、Word文档导出' },
  { key: 'cs', label: '客服沟通', icon: Headset, color: 'text-teal-500', tools: ['wechat_cs', 'feishu'], desc: '微信/飞书客服自动回复与监控' },
  { key: 'desktop', label: '桌面操控', icon: Monitor, color: 'text-indigo-500', tools: ['desktop'], desc: '截图、点击、输入，控制本地桌面应用' },
  { key: 'sys', label: '系统管理', icon: Settings, color: 'text-rose-500', tools: ['system'], desc: 'Agent编排、任务调度、委派' },
]

const toolMeta: Record<string, { label: string; icon: string; desc: string }> = {
  web_search: { label: '网页搜索', icon: '🔍', desc: '搜索互联网信息' },
  browser: { label: '浏览器操控', icon: '🌐', desc: '打开网页、点击、截图、提取内容' },
  http_request: { label: 'HTTP 请求', icon: '📡', desc: '发送请求，调用第三方 API' },
  video_generation: { label: '视频生成', icon: '🎬', desc: 'wan/veo3/sora2/kling/luma 多模型' },
  dubbing: { label: '配音字幕', icon: '🎙️', desc: 'TTS配音 + 字幕烧录' },
  mv_production: { label: 'MV 合成', icon: '🎵', desc: '节拍同步剪辑 + 专业转场' },
  comic_production: { label: '漫剧制作', icon: '📖', desc: '漫画图片 + 多角色配音组装' },
  music_generation: { label: '音乐创作', icon: '🎶', desc: 'ACE-Step/MiniMax/DiffRhythm' },
  image_generation: { label: '图片生成', icon: '🎨', desc: 'Flux/DALL-E 等 AI 绘画' },
  audio_analysis: { label: '音频分析', icon: '🔊', desc: 'BPM/能量分析、节拍检测' },
  code: { label: '代码执行', icon: '💻', desc: '14种语言、Web应用部署' },
  document: { label: '文档总结', icon: '📄', desc: '对话摘要 + Word 导出' },
  feishu: { label: '飞书', icon: '📱', desc: '消息发送与应用集成' },
  desktop: { label: '桌面操控', icon: '🖥️', desc: '截图/点击/输入桌面应用' },
  wechat_cs: { label: '微信客服', icon: '💬', desc: '自动回复/监控/转人工/消息分类' },
  system: { label: '系统管理', icon: '⚙️', desc: 'Agent编排、任务调度、委派' },
}

const skillIcons: Record<string, string> = {
  '个股诊断': '📊', '持仓复盘': '📋', '风险排查': '🛡️', '全市场扫描': '🔍', '市场解读': '📈',
  '盘前分析': '🌅', '自动扫描选股': '🔄', '持仓实时监控': '👁️', '日终复盘': '📝',
}

const activityTypeLabels: Record<string, { label: string; color: string; icon: string }> = {
  care: { label: '关怀', color: 'bg-pink-100 text-pink-600', icon: '💝' },
  schedule: { label: '定时', color: 'bg-blue-100 text-blue-600', icon: '⏰' },
  monitor: { label: '监控', color: 'bg-green-100 text-green-600', icon: '🛡️' },
  learn: { label: '学习', color: 'bg-purple-100 text-purple-600', icon: '📚' },
  event: { label: '事件', color: 'bg-amber-100 text-amber-600', icon: '⚡' },
}

const instinctCategoryLabels: Record<string, { label: string; color: string }> = {
  care: { label: '关怀', color: 'bg-pink-100 text-pink-600' },
  time: { label: '定时', color: 'bg-blue-100 text-blue-600' },
  monitor: { label: '监控', color: 'bg-green-100 text-green-600' },
  event: { label: '事件', color: 'bg-amber-100 text-amber-600' },
}

// ── Main Component ──

export default function AgentDetailPage() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const [agent, setAgent] = useState<Agent | null>(null)
  const [loading, setLoading] = useState(true)
  const [activeTab, setActiveTab] = useState<HexadTab>('gene')
  const [memories, setMemories] = useState<Memory[]>([])
  const [memoryCount, setMemoryCount] = useState(0)
  const [workflow, setWorkflow] = useState<Workflow | null>(null)
  const [mcpServers, setMcpServers] = useState<MCPServer[]>([])
  const [activities, setActivities] = useState<Activity[]>([])

  useEffect(() => {
    if (!id) return
    setLoading(true)
    agentAPI.get(id).then(res => {
      setAgent(res.data.agent || res.data)
    }).catch(() => {
      navigate('/agents')
    }).finally(() => setLoading(false))
  }, [id, navigate])

  // Load tab-specific data lazily
  useEffect(() => {
    if (!id || !agent) return
    if (activeTab === 'memory') {
      memoryAPI.list({ agent_id: id }).then(res => {
        const list = res.data.memories || []
        setMemories(list.slice(0, 20))
        setMemoryCount(list.length)
      }).catch(() => {})
    }
    if (activeTab === 'workflow') {
      agentAPI.getWorkflow(id).then(res => {
        const wfId = res.data.workflow_id
        if (wfId) {
          workflowAPI.get(wfId).then(r => setWorkflow(r.data.workflow || r.data)).catch(() => {})
        }
      }).catch(() => {})
    }
    if (activeTab === 'mcp') {
      mcpAPI.listServers().then(res => {
        setMcpServers(res.data.servers || [])
      }).catch(() => {})
    }
    if (activeTab === 'instinct') {
      activityAPI.list().then(res => {
        const acts = (res.data.activities || []) as Activity[]
        // For builtin SuperAgent, show ALL user activities (it's the default agent)
        // For other agents, filter by agent_id
        if (agent.is_builtin) {
          setActivities(acts)
        } else {
          setActivities(acts.filter((a: Activity) => a.agent_id === id))
        }
      }).catch(() => {})
    }
  }, [activeTab, id, agent])

  const tools: string[] = agent?.tools ? (() => { try { return JSON.parse(agent.tools) } catch { return [] } })() : []
  const builtinTools = tools.filter(t => !t.startsWith('mcp_'))
  const mcpTools = tools.filter(t => t.startsWith('mcp_'))
  const skills = (agent?.skills || []).filter(s => s.ability_type !== 'instinct')
  const instincts = (agent?.skills || []).filter(s => s.ability_type === 'instinct')

  const agentConfig: Record<string, unknown> = agent?.config ? (() => { try { return JSON.parse(agent.config) } catch { return {} } })() : {}

  if (loading) {
    return (
      <div className="flex items-center justify-center h-full">
        <div className="animate-spin w-8 h-8 border-2 border-primary-500 border-t-transparent rounded-full" />
      </div>
    )
  }

  if (!agent) return null

  // Count badges for tabs
  const groupedSkillCount = skillGroups.reduce((n, g) => n + g.tools.filter(t => builtinTools.includes(t)).length, 0) + skills.length
  const tabCounts: Partial<Record<HexadTab, number>> = {
    skill: groupedSkillCount,
    instinct: instincts.length + activities.length + (builtinTools.includes('wechat_cs') ? 5 : 0),
    mcp: mcpTools.length + mcpServers.length,
  }

  return (
    <div className="h-full overflow-y-auto">
      <div className="max-w-5xl mx-auto p-8">
        {/* Back button */}
        <button
          onClick={() => navigate('/agents')}
          className="flex items-center gap-1.5 text-sm text-gray-500 hover:text-gray-700 mb-6 transition-colors"
        >
          <ArrowLeft className="w-4 h-4" />
          返回智能体
        </button>

        {/* Header Card */}
        <div className="bg-white border rounded-2xl p-6 mb-6">
          <div className="flex items-start gap-4">
            <div className={`w-14 h-14 rounded-xl flex items-center justify-center flex-shrink-0 ${
              agent.is_builtin ? 'bg-gradient-to-br from-primary-500 to-orange-500' : 'bg-primary-100'
            }`}>
              {agent.is_builtin
                ? <Sparkles className="w-7 h-7 text-white" />
                : <Bot className="w-7 h-7 text-primary-600" />
              }
            </div>
            <div className="flex-1 min-w-0">
              <div className="flex items-center gap-3">
                <h1 className="text-2xl font-bold text-gray-900">{formatAgentDisplayName(agent.name)}</h1>
                {agent.is_builtin && (
                  <span className="px-2 py-0.5 bg-primary-50 text-primary-600 text-xs rounded-full border border-primary-200 font-medium">内置</span>
                )}
                {agent.is_public && (
                  <span className="px-2 py-0.5 bg-green-50 text-green-600 text-xs rounded-full border border-green-200">公开</span>
                )}
              </div>
              <p className="text-gray-500 mt-1">{agent.description || '暂无描述'}</p>
              <div className="flex gap-3 mt-4">
                <button
                  onClick={() => navigate('/chat')}
                  className="flex items-center gap-2 px-4 py-2 bg-primary-600 text-white rounded-lg text-sm hover:bg-primary-700 transition-colors"
                >
                  <MessageSquare className="w-4 h-4" />
                  开始对话
                </button>
              </div>
            </div>
          </div>
        </div>

        {/* Hexad Tabs */}
        <div className="flex gap-1 mb-6 bg-gray-100 rounded-xl p-1 overflow-x-auto">
          {hexadTabs.map(tab => {
            const Icon = tab.icon
            const isActive = activeTab === tab.key
            const count = tabCounts[tab.key]
            return (
              <button
                key={tab.key}
                onClick={() => setActiveTab(tab.key)}
                className={`flex items-center gap-1.5 px-4 py-2.5 rounded-lg text-sm font-medium transition-all whitespace-nowrap ${
                  isActive
                    ? 'bg-white shadow-sm text-gray-900'
                    : 'text-gray-500 hover:text-gray-700 hover:bg-gray-50'
                }`}
              >
                <Icon className={`w-4 h-4 ${isActive ? tab.color : ''}`} />
                {tab.label}
                {count !== undefined && count > 0 && (
                  <span className={`text-[10px] px-1.5 py-0.5 rounded-full font-medium ${
                    isActive ? 'bg-gray-100 text-gray-600' : 'bg-gray-200 text-gray-500'
                  }`}>{count}</span>
                )}
              </button>
            )
          })}
        </div>

        {/* Tab Content */}
        <div className="min-h-[400px]">
          {activeTab === 'gene' && <GeneTab agent={agent} config={agentConfig} />}
          {activeTab === 'skill' && <SkillTab builtinTools={builtinTools} skills={skills} />}
          {activeTab === 'instinct' && <InstinctTab instincts={instincts} activities={activities} builtinTools={builtinTools} />}
          {activeTab === 'mcp' && <MCPTab mcpTools={mcpTools} mcpServers={mcpServers} />}
          {activeTab === 'workflow' && <WorkflowTab workflow={workflow} agentId={id || ''} navigate={navigate} isBuiltin={agent.is_builtin} builtinTools={builtinTools} />}
          {activeTab === 'memory' && <MemoryTab memories={memories} memoryCount={memoryCount} agentId={id || ''} navigate={navigate} />}
        </div>
      </div>
    </div>
  )
}

// ── Tab 1: 基因 (Gene) ──

function GeneTab({ agent, config }: { agent: Agent; config: Record<string, unknown> }) {
  const [promptCollapsed, setPromptCollapsed] = useState(false)

  return (
    <div className="space-y-6">
      {/* Identity */}
      <div className="bg-white border rounded-2xl p-6">
        <h3 className="text-sm font-semibold text-gray-500 uppercase tracking-wider mb-4 flex items-center gap-2">
          <Dna className="w-4 h-4 text-rose-500" />
          身份基因
        </h3>
        <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
          <div className="bg-gray-50 rounded-xl p-4">
            <div className="text-xs text-gray-400 mb-1">名称</div>
            <div className="font-semibold text-gray-900">{formatAgentDisplayName(agent.name)}</div>
          </div>
          <div className="bg-gray-50 rounded-xl p-4">
            <div className="text-xs text-gray-400 mb-1">模型</div>
            <div className="font-semibold text-gray-900">{agent.model_name || '使用默认模型'}</div>
          </div>
          <div className="bg-gray-50 rounded-xl p-4">
            <div className="text-xs text-gray-400 mb-1">温度</div>
            <div className="font-semibold text-gray-900">{(config.temperature as number) ?? 0.3}</div>
          </div>
          <div className="bg-gray-50 rounded-xl p-4">
            <div className="text-xs text-gray-400 mb-1">最大 Token</div>
            <div className="font-semibold text-gray-900">{(config.max_tokens as number) ?? 8192}</div>
          </div>
        </div>
        {agent.description && (
          <div className="mt-4 bg-gray-50 rounded-xl p-4">
            <div className="text-xs text-gray-400 mb-1">角色定位</div>
            <div className="text-sm text-gray-700">{agent.description}</div>
          </div>
        )}
      </div>

      {/* Rules (System Prompt) — default expanded */}
      <div className="bg-white border rounded-2xl p-6">
        <div className="flex items-center justify-between mb-3">
          <h3 className="text-sm font-semibold text-gray-500 uppercase tracking-wider">规则</h3>
          {agent.system_prompt && (
            <button
              onClick={() => setPromptCollapsed(!promptCollapsed)}
              className="flex items-center gap-1 text-xs text-primary-600 hover:text-primary-700"
            >
              {promptCollapsed ? '展开' : '收起'}
              {promptCollapsed ? <ChevronDown className="w-3 h-3" /> : <ChevronUp className="w-3 h-3" />}
            </button>
          )}
        </div>
        {!promptCollapsed && (
          <div className="bg-gray-50 rounded-xl p-4 max-h-[600px] overflow-y-auto">
            <pre className="text-sm text-gray-700 whitespace-pre-wrap font-sans leading-relaxed">
              {agent.system_prompt || '(未设置)'}
            </pre>
          </div>
        )}
      </div>
    </div>
  )
}

// ── Tab 2: 技能 (Skill) — grouped by capability domain ──

function SkillTab({ builtinTools, skills }: { builtinTools: string[]; skills: AgentSkill[] }) {
  const activeGroups = skillGroups.filter(g => g.tools.some(t => builtinTools.includes(t)))

  return (
    <div className="space-y-6">
      {/* Grouped capability domains */}
      {activeGroups.map(group => {
        const GroupIcon = group.icon
        const groupTools = group.tools.filter(t => builtinTools.includes(t))
        return (
          <div key={group.key} className="bg-white border rounded-2xl p-6">
            <div className="flex items-center gap-2 mb-1">
              <GroupIcon className={`w-4 h-4 ${group.color}`} />
              <h3 className="text-sm font-semibold text-gray-700">{group.label}</h3>
              <span className="text-[10px] text-gray-400 ml-1">{group.desc}</span>
            </div>
            <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-2 mt-3">
              {groupTools.map(t => {
                const meta = toolMeta[t] || { label: t, icon: '⚡', desc: '' }
                return (
                  <div key={t} className="flex items-center gap-3 border rounded-lg p-3 bg-gray-50/50 hover:bg-gray-50 transition-colors">
                    <span className="text-lg flex-shrink-0">{meta.icon}</span>
                    <div className="min-w-0">
                      <div className="font-medium text-gray-800 text-sm">{meta.label}</div>
                      <div className="text-[11px] text-gray-400 truncate">{meta.desc}</div>
                    </div>
                  </div>
                )
              })}
            </div>
          </div>
        )
      })}

      {/* Installed Skills (from marketplace / plugins) */}
      {skills.length > 0 && (
        <div className="bg-white border rounded-2xl p-6">
          <div className="flex items-center gap-2 mb-1">
            <Sparkles className="w-4 h-4 text-blue-500" />
            <h3 className="text-sm font-semibold text-gray-700">已安装技能</h3>
            <span className="text-[10px] text-gray-400 ml-1">从市场安装的专业技能</span>
          </div>
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3 mt-3">
            {skills.map(s => {
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

      {activeGroups.length === 0 && skills.length === 0 && (
        <EmptyState icon={Wrench} text="暂无技能" desc="此智能体没有配置任何工具或技能" />
      )}
    </div>
  )
}

// ── Tab 3: 本能 (Instinct) — "我自己做" ──

function InstinctTab({ instincts, activities, builtinTools }: { instincts: AgentSkill[]; activities: Activity[]; builtinTools: string[] }) {
  const hasWechatCS = builtinTools.includes('wechat_cs')

  if (!instincts.length && !activities.length && !hasWechatCS) {
    return <EmptyState icon={Zap} text="暂无本能" desc="本能是智能体自动执行的能力，不需要用户触发" />
  }

  const wechatInstincts = [
    { id: 'auto_chat', icon: '🔄', title: '持续自动聊天', desc: '后台每5秒扫描微信未读消息，用 AI 理解上下文自动回复', trigger: '每 5 秒', type: 'monitor' },
    { id: 'reply_all', icon: '💬', title: '一键回复所有未读', desc: '遍历所有未读会话，逐个用 AI 生成回复并发送', trigger: '用户触发', type: 'event' },
    { id: 'watch', icon: '👁️', title: '群聊监控', desc: '定时截图对比微信窗口变化，检测到新消息自动创建客服任务', trigger: '每 20-30 秒', type: 'monitor' },
    { id: 'classify', icon: '🏷️', title: '消息智能分类', desc: '对客户消息做意图分类（投诉/报价/下单/进度/多媒体），高风险自动转人工', trigger: '收到消息时', type: 'event' },
    { id: 'handoff', icon: '🙋', title: '自动转人工', desc: '检测到投诉/退款/法律风险时自动标记转人工，附带优先级和原因', trigger: '高风险触发', type: 'event' },
  ]

  return (
    <div className="space-y-6">
      {/* WeChat CS built-in instincts */}
      {hasWechatCS && (
        <div className="bg-white border rounded-2xl p-6">
          <div className="flex items-center gap-2 mb-1">
            <Zap className="w-4 h-4 text-teal-500" />
            <h3 className="text-sm font-semibold text-gray-700">微信客服本能</h3>
            <span className="text-[10px] text-gray-400 ml-1">自动化客服能力，无需手动触发</span>
          </div>
          <div className="space-y-2 mt-3">
            {wechatInstincts.map(item => {
              const typeInfo = activityTypeLabels[item.type] || { label: item.type, color: 'bg-gray-100 text-gray-600', icon: '⚡' }
              return (
                <div key={item.id} className="flex items-center gap-3 border rounded-xl p-3.5 transition-colors bg-teal-50/30 hover:border-teal-300">
                  <span className="text-lg flex-shrink-0">{item.icon}</span>
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2">
                      <span className="font-medium text-gray-800 text-sm">{item.title}</span>
                      <span className={`text-[10px] px-1.5 py-0.5 rounded-full font-medium ${typeInfo.color}`}>{typeInfo.label}</span>
                    </div>
                    <div className="text-[11px] text-gray-500 mt-0.5">{item.desc}</div>
                  </div>
                  <div className="flex items-center gap-1 text-[11px] text-teal-600 bg-teal-100 px-2 py-0.5 rounded flex-shrink-0">
                    <Clock className="w-3 h-3" />
                    {item.trigger}
                  </div>
                </div>
              )
            })}
          </div>
        </div>
      )}

      {/* Activities (built-in instincts) */}
      {activities.length > 0 && (
        <div className="bg-white border rounded-2xl p-6">
          <div className="flex items-center gap-2 mb-1">
            <Zap className="w-4 h-4 text-amber-500" />
            <h3 className="text-sm font-semibold text-gray-700">自主行为</h3>
            <span className="text-[10px] text-gray-400 ml-1">系统预设，可单独开关</span>
          </div>
          <div className="space-y-2 mt-3">
            {activities.map(act => {
              const typeInfo = activityTypeLabels[act.type] || { label: act.type, color: 'bg-gray-100 text-gray-600', icon: '⚡' }
              return (
                <div key={act.id} className={`flex items-center gap-3 border rounded-xl p-3.5 transition-colors ${act.enabled ? 'bg-amber-50/30 hover:border-amber-300' : 'bg-gray-50 opacity-60'}`}>
                  <span className="text-lg flex-shrink-0">{typeInfo.icon}</span>
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2">
                      <span className="font-medium text-gray-800 text-sm">{act.title}</span>
                      <span className={`text-[10px] px-1.5 py-0.5 rounded-full font-medium ${typeInfo.color}`}>{typeInfo.label}</span>
                      {!act.enabled && (
                        <span className="text-[10px] px-1.5 py-0.5 rounded-full bg-gray-200 text-gray-500">未启用</span>
                      )}
                    </div>
                    <div className="text-[11px] text-gray-500 mt-0.5">{act.description}</div>
                  </div>
                  <div className="flex flex-col items-end gap-1 flex-shrink-0">
                    <div className="flex items-center gap-1 text-[11px] text-amber-600 bg-amber-100 px-2 py-0.5 rounded">
                      <Clock className="w-3 h-3" />
                      {act.trigger}
                    </div>
                    {act.total_runs > 0 && (
                      <span className="text-[10px] text-gray-400">已执行 {act.total_runs} 次</span>
                    )}
                  </div>
                </div>
              )
            })}
          </div>
        </div>
      )}

      {/* AgentSkill instincts (marketplace-installed) */}
      {instincts.length > 0 && (
        <div className="bg-white border rounded-2xl p-6">
          <div className="flex items-center gap-2 mb-1">
            <Sparkles className="w-4 h-4 text-amber-500" />
            <h3 className="text-sm font-semibold text-gray-700">已安装本能</h3>
          </div>
          <div className="space-y-2 mt-3">
            {instincts.map(s => {
              let spec: SkillSpec = { trigger: 'proactive' }
              try { spec = JSON.parse(s.skill_spec) } catch {}
              const cat = instinctCategoryLabels[s.instinct_category] || { label: s.instinct_category || '未分类', color: 'bg-gray-100 text-gray-600' }
              return (
                <div key={s.id} className={`flex items-center gap-3 border rounded-xl p-3.5 transition-colors ${s.enabled ? 'bg-amber-50/30 hover:border-amber-300' : 'bg-gray-50 opacity-60'}`}>
                  <span className="text-lg">{skillIcons[s.skill_name] || '⚡'}</span>
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2">
                      <span className="font-medium text-gray-800 text-sm">{s.skill_name}</span>
                      <span className={`text-[10px] px-1.5 py-0.5 rounded-full font-medium ${cat.color}`}>{cat.label}</span>
                    </div>
                    <div className="text-[11px] text-gray-500 mt-0.5">{spec.description || ''}</div>
                  </div>
                  {spec.schedule && (
                    <div className="flex items-center gap-1 text-[11px] text-amber-600 bg-amber-100 px-2 py-0.5 rounded flex-shrink-0">
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
    </div>
  )
}

// ── Tab 4: 外接服务 ──

function MCPTab({ mcpTools, mcpServers }: { mcpTools: string[]; mcpServers: MCPServer[] }) {
  if (!mcpTools.length && !mcpServers.length) {
    return <EmptyState icon={Plug} text="暂无外接服务" desc="通过 MCP 协议连接第三方工具和数据源，扩展智能体能力" />
  }

  return (
    <div className="space-y-6">
      {mcpTools.length > 0 && (
        <div className="bg-white border rounded-2xl p-6">
          <div className="flex items-center gap-2 mb-1">
            <Plug className="w-4 h-4 text-purple-500" />
            <h3 className="text-sm font-semibold text-gray-700">已绑定外接工具</h3>
          </div>
          <div className="space-y-2 mt-3">
            {mcpTools.map(t => {
              const name = t.replace(/^mcp_/, '').replace(/_/g, ' ')
              return (
                <div key={t} className="flex items-center gap-3 border rounded-xl p-3 bg-purple-50/30 hover:border-purple-300 transition-colors">
                  <div className="w-2 h-2 rounded-full bg-green-500 flex-shrink-0" />
                  <div className="flex-1">
                    <div className="font-medium text-gray-800 text-sm capitalize">{name}</div>
                  </div>
                  <span className="text-[10px] text-purple-500 bg-purple-100 px-2 py-0.5 rounded font-medium">外接</span>
                </div>
              )
            })}
          </div>
        </div>
      )}

      {mcpServers.length > 0 && (
        <div className="bg-white border rounded-2xl p-6">
          <div className="flex items-center gap-2 mb-1">
            <Users className="w-4 h-4 text-gray-500" />
            <h3 className="text-sm font-semibold text-gray-700">已连接服务</h3>
            <span className="text-[10px] text-gray-400 ml-1">当前节点的外接服务</span>
          </div>
          <div className="space-y-2 mt-3">
            {mcpServers.map(s => (
              <div key={s.id} className="flex items-center gap-3 border rounded-xl p-3 bg-gray-50">
                <div className={`w-2 h-2 rounded-full flex-shrink-0 ${s.status === 'connected' || s.status === 'active' ? 'bg-green-500' : 'bg-gray-300'}`} />
                <div className="flex-1 min-w-0">
                  <div className="font-medium text-gray-800 text-sm">{s.name}</div>
                  <div className="text-[11px] text-gray-400 truncate">{s.base_url}</div>
                </div>
                <span className={`text-[10px] px-2 py-0.5 rounded font-medium ${
                  s.status === 'connected' || s.status === 'active' ? 'bg-green-100 text-green-600' : 'bg-gray-200 text-gray-500'
                }`}>{s.status === 'connected' || s.status === 'active' ? '已连接' : s.status || '未知'}</span>
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  )
}

// ── Tab 5: 工作流 (Workflow) ──

function WorkflowTab({ workflow, agentId, navigate, isBuiltin, builtinTools }: { workflow: Workflow | null; agentId: string; navigate: (path: string) => void; isBuiltin: boolean; builtinTools: string[] }) {
  const handleGo = async () => {
    try {
      const res = await agentAPI.getWorkflow(agentId)
      const wfId = res.data.workflow_id
      if (wfId) {
        navigate(`/workflows/editor?id=${wfId}`)
        return
      }
    } catch {}
    navigate('/workflows/editor')
  }

  const hasWechatCS = builtinTools.includes('wechat_cs')

  // For WeChat CS agent, show customer service decision flow
  if (hasWechatCS && (!workflow || !workflow.nodes || workflow.nodes === '[]')) {
    return (
      <div className="space-y-6">
        <div className="bg-white border rounded-2xl p-6">
          <div className="flex items-center gap-2 mb-4">
            <GitBranch className="w-4 h-4 text-teal-500" />
            <h3 className="text-sm font-semibold text-gray-700">微信客服决策流程</h3>
          </div>
          <div className="space-y-3">
            {[
              { step: '1', label: '消息检测', desc: '定时截图/扫描微信窗口，对比 hash 判断是否有新消息', color: 'bg-teal-50 border-teal-200' },
              { step: '2', label: '上下文抓取', desc: '聚焦微信窗口，截图并用视觉模型 OCR 读取聊天内容', color: 'bg-blue-50 border-blue-200' },
              { step: '3', label: '意图分类', desc: '分析消息意图（投诉/报价/下单/进度/多媒体），评估风险等级', color: 'bg-amber-50 border-amber-200' },
              { step: '4', label: '路由决策', desc: '低风险 → AI 自动回复；高风险 → 转人工并通知', color: 'bg-purple-50 border-purple-200' },
              { step: '5', label: '执行回复', desc: '通过桌面自动化定位聊天窗口、输入回复内容并发送', color: 'bg-emerald-50 border-emerald-200' },
              { step: '6', label: '状态更新', desc: '更新监控 hash、记录回复日志、创建任务通知', color: 'bg-pink-50 border-pink-200' },
            ].map(item => (
              <div key={item.step} className={`flex items-center gap-4 border rounded-xl p-3.5 ${item.color}`}>
                <div className="w-8 h-8 rounded-full bg-white border flex items-center justify-center text-sm font-bold text-gray-600 flex-shrink-0">{item.step}</div>
                <div>
                  <div className="font-medium text-gray-800 text-sm">{item.label}</div>
                  <div className="text-[11px] text-gray-500">{item.desc}</div>
                </div>
              </div>
            ))}
          </div>
        </div>
      </div>
    )
  }

  // For builtin SuperAgent, show its decision flow even without workflow nodes
  if (isBuiltin && (!workflow || !workflow.nodes || workflow.nodes === '[]')) {
    return (
      <div className="space-y-6">
        <div className="bg-white border rounded-2xl p-6">
          <div className="flex items-center gap-2 mb-4">
            <GitBranch className="w-4 h-4 text-emerald-500" />
            <h3 className="text-sm font-semibold text-gray-700">总管决策流程</h3>
          </div>
          <div className="space-y-3">
            {[
              { step: '1', label: '接收指令', desc: '理解用户意图，识别任务类型', color: 'bg-blue-50 border-blue-200' },
              { step: '2', label: '路由决策', desc: '简单任务直接执行，复杂任务委派给专业 Agent', color: 'bg-amber-50 border-amber-200' },
              { step: '3', label: '调用工具', desc: '根据任务类型选择最合适的工具组合', color: 'bg-emerald-50 border-emerald-200' },
              { step: '4', label: '质量检查', desc: '验证执行结果，确保符合用户预期', color: 'bg-purple-50 border-purple-200' },
              { step: '5', label: '交付结果', desc: '向用户展示结果，提供后续建议', color: 'bg-pink-50 border-pink-200' },
              { step: '6', label: '记忆归档', desc: '提取对话中的关键信息存入长期记忆', color: 'bg-cyan-50 border-cyan-200' },
            ].map(item => (
              <div key={item.step} className={`flex items-center gap-4 border rounded-xl p-3.5 ${item.color}`}>
                <div className="w-8 h-8 rounded-full bg-white border flex items-center justify-center text-sm font-bold text-gray-600 flex-shrink-0">{item.step}</div>
                <div>
                  <div className="font-medium text-gray-800 text-sm">{item.label}</div>
                  <div className="text-[11px] text-gray-500">{item.desc}</div>
                </div>
              </div>
            ))}
          </div>
        </div>
        {workflow && (
          <div className="text-center">
            <button onClick={handleGo} className="inline-flex items-center gap-1.5 text-xs text-primary-600 hover:text-primary-700">
              打开工作流编辑器 <ExternalLink className="w-3 h-3" />
            </button>
          </div>
        )}
      </div>
    )
  }

  if (!workflow) {
    return (
      <div className="bg-white border rounded-2xl p-8 text-center">
        <GitBranch className="w-10 h-10 text-gray-300 mx-auto mb-3" />
        <p className="text-gray-500 text-sm mb-4">此智能体暂无关联工作流</p>
        <button
          onClick={handleGo}
          className="inline-flex items-center gap-2 px-4 py-2 bg-primary-600 text-white rounded-lg text-sm hover:bg-primary-700 transition-colors"
        >
          <GitBranch className="w-4 h-4" />
          创建工作流
        </button>
      </div>
    )
  }

  let nodeCount = 0
  try { nodeCount = JSON.parse(workflow.nodes || '[]').length } catch {}

  return (
    <div className="bg-white border rounded-2xl p-6">
      <div className="flex items-center justify-between mb-4">
        <h3 className="text-sm font-semibold text-gray-500 uppercase tracking-wider flex items-center gap-2">
          <GitBranch className="w-4 h-4 text-emerald-500" />
          关联工作流
        </h3>
        <button
          onClick={handleGo}
          className="flex items-center gap-1 text-xs text-primary-600 hover:text-primary-700"
        >
          打开编辑器 <ExternalLink className="w-3 h-3" />
        </button>
      </div>
      <div className="bg-emerald-50/50 border border-emerald-200 rounded-xl p-4">
        <div className="font-semibold text-gray-900">{workflow.name || '默认工作流'}</div>
        {workflow.description && <p className="text-xs text-gray-500 mt-1">{workflow.description}</p>}
        <div className="flex items-center gap-4 mt-3 text-xs text-gray-400">
          <span>{nodeCount} 个节点</span>
          <span>ID: {workflow.id.slice(0, 8)}...</span>
        </div>
      </div>
    </div>
  )
}

// ── Tab 6: 记忆 (Memory) ──

function MemoryTab({ memories, memoryCount, agentId, navigate }: { memories: Memory[]; memoryCount: number; agentId: string; navigate: (path: string) => void }) {
  const categoryColors: Record<string, string> = {
    fact: 'bg-blue-100 text-blue-600',
    preference: 'bg-pink-100 text-pink-600',
    instruct: 'bg-amber-100 text-amber-600',
    context: 'bg-green-100 text-green-600',
    skill: 'bg-purple-100 text-purple-600',
    summary: 'bg-gray-100 text-gray-600',
  }

  if (!memories.length) {
    return (
      <div className="bg-white border rounded-2xl p-8 text-center">
        <Brain className="w-10 h-10 text-gray-300 mx-auto mb-3" />
        <p className="text-gray-500 text-sm mb-2">暂无记忆</p>
        <p className="text-xs text-gray-400">对话过程中自动提取的记忆会显示在这里</p>
      </div>
    )
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <span className="text-sm text-gray-500">共 {memoryCount} 条记忆</span>
        <button
          onClick={() => navigate(`/memories?agent_id=${agentId}`)}
          className="flex items-center gap-1 text-xs text-primary-600 hover:text-primary-700"
        >
          查看全部 <ExternalLink className="w-3 h-3" />
        </button>
      </div>
      <div className="bg-white border rounded-2xl divide-y">
        {memories.map(m => (
          <div key={m.id} className="p-4">
            <div className="flex items-center gap-2 mb-1">
              <span className="font-medium text-gray-800 text-sm">{m.key}</span>
              <span className={`text-[10px] px-1.5 py-0.5 rounded-full font-medium ${categoryColors[m.category] || 'bg-gray-100 text-gray-600'}`}>
                {m.category}
              </span>
              <span className="text-[10px] text-gray-400 ml-auto">
                {'★'.repeat(Math.round(m.importance * 5))}{'☆'.repeat(5 - Math.round(m.importance * 5))}
              </span>
            </div>
            <p className="text-xs text-gray-500 line-clamp-2">{m.content}</p>
          </div>
        ))}
      </div>
    </div>
  )
}

// ── Helpers ──

function EmptyState({ icon: Icon, text, desc }: { icon: typeof Wrench; text: string; desc: string }) {
  return (
    <div className="bg-white border rounded-2xl p-10 text-center">
      <Icon className="w-10 h-10 text-gray-300 mx-auto mb-3" />
      <p className="text-gray-500 text-sm font-medium">{text}</p>
      <p className="text-xs text-gray-400 mt-1">{desc}</p>
    </div>
  )
}
