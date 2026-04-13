import { useEffect, useMemo, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { ArrowLeft, Bot, CheckCircle2, GitBranch, Rocket, ShieldCheck } from 'lucide-react'
import { agentAPI, taskAPI, teamAPI } from '../lib/api'
import { formatAgentDisplayName } from '../lib/agentDisplay'

interface TeamMember {
  role: string
  agentName: string
  responsibility: string
}

interface AgentLite {
  id: string
  name: string
  description: string
}

const TEAM_DATA: Record<string, { name: string; description: string; members: TeamMember[]; flow: string[] }> = {
  'team-devops-rnd': {
    name: '研发DevOps团队',
    description: '面向软件全生命周期交付：需求拆解、研发实现、测试验收、部署上线、监控回滚。',
    members: [
      { role: 'PM', agentName: '产品经理Agent', responsibility: '需求澄清、任务拆解、验收标准定义' },
      { role: 'Frontend', agentName: '前端研发Agent', responsibility: 'UI实现、交互细节、页面性能优化' },
      { role: 'Backend', agentName: '后端研发Agent', responsibility: 'API设计、业务逻辑、数据模型落地' },
      { role: 'QA', agentName: '测试Agent', responsibility: '测试用例、回归验证、风险检查' },
      { role: 'DevOps', agentName: '运维部署Agent', responsibility: '构建发布、域名绑定、线上验证与回滚' },
    ],
    flow: [
      '需求澄清与任务拆分',
      '前后端并行开发',
      '联调 + 测试回归',
      '生产部署（需审批）',
      '域名绑定（需审批）',
      '上线巡检与交付报告',
    ],
  },
}

export default function TeamDetailPage() {
  const navigate = useNavigate()
  const { id } = useParams<{ id: string }>()
  const [backendAgents, setBackendAgents] = useState<AgentLite[]>([])
  const [orchestratorFromBackend, setOrchestratorFromBackend] = useState<AgentLite | null>(null)
  const [taskGoal, setTaskGoal] = useState('请研发DevOps团队从需求到上线完成官网交付，发布前和 DNS 变更前都需要审批确认。')
  const [submitting, setSubmitting] = useState(false)
  const [taskStatus, setTaskStatus] = useState('')

  const team = useMemo(() => {
    if (!id) return null
    return TEAM_DATA[id] || null
  }, [id])

  useEffect(() => {
    const loadAgents = async () => {
      try {
        const res = await agentAPI.list()
        setBackendAgents(res.data.agents || [])
      } catch {
        setBackendAgents([])
      }
    }
    loadAgents()
  }, [])

  useEffect(() => {
    if (!id) return
    const loadOrchestrator = async () => {
      try {
        const res = await teamAPI.get(id)
        const team = res.data?.team
        if (team?.coordinator_id) {
          const coordMember = team.members?.find((m: any) => m.role === 'coordinator')
          setOrchestratorFromBackend(coordMember?.agent || null)
        } else {
          setOrchestratorFromBackend(null)
        }
      } catch {
        setOrchestratorFromBackend(null)
      }
    }
    loadOrchestrator()
  }, [id])

  const resolveAgentName = (fallbackName: string) => {
    const exact = backendAgents.find((a) => a.name === fallbackName)
    if (exact) return formatAgentDisplayName(exact.name)

    const keyword = fallbackName.replace('Agent', '').replace('团队', '').trim().toLowerCase()
    const fuzzy = backendAgents.find((a) => a.name.toLowerCase().includes(keyword))
    return fuzzy ? formatAgentDisplayName(fuzzy.name) : `${formatAgentDisplayName(fallbackName)}（待创建）`
  }

  const resolveTeamOrchestratorAgent = () => {
    if (orchestratorFromBackend) return orchestratorFromBackend

    const preferredKeywords = ['全能助手', '编程Agent', '研发', 'devops', 'orchestrator']
    for (const keyword of preferredKeywords) {
      const hit = backendAgents.find((a) => a.name.toLowerCase().includes(keyword.toLowerCase()))
      if (hit) return hit
    }
    return backendAgents[0] || null
  }

  const createTeamTask = async () => {
    if (!taskGoal.trim()) return
    setSubmitting(true)
    setTaskStatus('')
    const orchestrator = resolveTeamOrchestratorAgent()
    if (!orchestrator) {
      setTaskStatus('未找到可用编排代理，请先创建“全能助手”或“编程Agent”。')
      setSubmitting(false)
      return
    }
    try {
      await taskAPI.create({
        title: `${team?.name || '团队'}协作任务`,
        goal: taskGoal.trim(),
        agent_id: orchestrator.id,
        priority: 'high',
      })
      setTaskStatus(`团队任务已创建，执行代理：${formatAgentDisplayName(orchestrator.name)}。可到「自主任务」查看进度。`)
    } catch {
      setTaskStatus('创建任务失败，请检查登录状态或稍后重试。')
    } finally {
      setSubmitting(false)
    }
  }

  if (!team) {
    return (
      <div className="h-full overflow-y-auto">
        <div className="max-w-5xl mx-auto p-8">
          <button
            onClick={() => navigate('/agents')}
            className="inline-flex items-center gap-2 text-sm text-gray-600 hover:text-gray-900 mb-4"
          >
            <ArrowLeft className="w-4 h-4" /> 返回 Agents
          </button>
          <div className="bg-white border rounded-xl p-8 text-center text-gray-500">团队不存在</div>
        </div>
      </div>
    )
  }

  return (
    <div className="h-full overflow-y-auto">
      <div className="max-w-5xl mx-auto p-8 space-y-6">
        <button
          onClick={() => navigate('/agents')}
          className="inline-flex items-center gap-2 text-sm text-gray-600 hover:text-gray-900"
        >
          <ArrowLeft className="w-4 h-4" /> 返回 Agents
        </button>

        <div className="bg-white border rounded-xl p-6">
          <div className="flex items-start justify-between gap-4">
            <div>
              <h1 className="text-2xl font-bold text-gray-900">{team.name}</h1>
              <p className="text-gray-600 mt-2">{team.description}</p>
              <div className="flex items-center gap-2 mt-3">
                <span className="px-2 py-0.5 bg-indigo-50 text-indigo-600 text-xs rounded-full">官方</span>
                <span className="px-2 py-0.5 bg-purple-50 text-purple-600 text-xs rounded-full">团队代理</span>
              </div>
            </div>
            <div className="w-12 h-12 rounded-xl bg-purple-100 flex items-center justify-center">
              <Bot className="w-6 h-6 text-purple-600" />
            </div>
          </div>
        </div>

        <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
          <section className="bg-white border rounded-xl p-6">
            <h2 className="text-sm font-semibold text-gray-900 mb-4 flex items-center gap-2">
              <ShieldCheck className="w-4 h-4 text-emerald-600" /> 团队成员角色
            </h2>
            <div className="space-y-3">
              {team.members.map((member) => (
                <div key={member.role} className="border rounded-lg p-3">
                  <div className="flex items-center justify-between">
                    <span className="text-xs px-2 py-0.5 rounded bg-gray-100 text-gray-600">{member.role}</span>
                    <span className="text-sm font-medium text-gray-900">{resolveAgentName(member.agentName)}</span>
                  </div>
                  <p className="text-sm text-gray-600 mt-2">{member.responsibility}</p>
                </div>
              ))}
            </div>
          </section>

          <section className="bg-white border rounded-xl p-6">
            <h2 className="text-sm font-semibold text-gray-900 mb-4 flex items-center gap-2">
              <GitBranch className="w-4 h-4 text-primary-600" /> 协作流程
            </h2>
            <ol className="space-y-3">
              {team.flow.map((step, idx) => (
                <li key={step} className="flex items-start gap-3 text-sm text-gray-700">
                  <span className="mt-0.5 inline-flex w-5 h-5 items-center justify-center rounded-full bg-primary-100 text-primary-700 text-xs font-semibold">
                    {idx + 1}
                  </span>
                  <span>{step}</span>
                </li>
              ))}
            </ol>
          </section>
        </div>

        <section className="bg-white border rounded-xl p-6">
          <h2 className="text-sm font-semibold text-gray-900 mb-3 flex items-center gap-2">
            <Rocket className="w-4 h-4 text-orange-600" /> 快速测试建议
          </h2>
          <div className="text-sm text-gray-700 space-y-2">
            <p>1. 在对话中输入："请研发DevOps团队从需求到上线完成一个官网交付"</p>
            <p>2. 观察任务拆分是否包含：研发、测试、部署、验证</p>
            <p>3. 在发布前和 DNS 变更前，确认是否触发审批提示</p>
            <p className="inline-flex items-center gap-1 text-emerald-700">
              <CheckCircle2 className="w-4 h-4" /> 通过后即可作为团队模板复用
            </p>
          </div>
        </section>

        <section className="bg-white border rounded-xl p-6">
          <h2 className="text-sm font-semibold text-gray-900 mb-3">创建团队任务</h2>
          <p className="text-sm text-gray-600 mb-2">
            当前编排代理：{formatAgentDisplayName(resolveTeamOrchestratorAgent()?.name || '未匹配')}
          </p>
          <p className="text-sm text-gray-600 mb-3">
            当前已连接后端代理：{backendAgents.length} 个。点击下方按钮会创建一个高优先级团队协作任务。
          </p>
          <textarea
            value={taskGoal}
            onChange={(e) => setTaskGoal(e.target.value)}
            rows={4}
            className="w-full px-3 py-2 border rounded-lg text-sm outline-none focus:ring-2 focus:ring-primary-500 resize-none"
          />
          <div className="mt-3 flex items-center gap-3">
            <button
              onClick={createTeamTask}
              disabled={submitting || !taskGoal.trim()}
              className="px-4 py-2 text-sm bg-primary-600 text-white rounded-lg hover:bg-primary-700 disabled:opacity-50 transition-colors"
            >
              {submitting ? '创建中...' : '创建团队任务'}
            </button>
            <button
              onClick={() => navigate('/tasks')}
              className="px-4 py-2 text-sm border border-gray-300 text-gray-700 rounded-lg hover:bg-gray-50 transition-colors"
            >
              查看任务列表
            </button>
          </div>
          {taskStatus && <p className="mt-3 text-sm text-gray-600">{taskStatus}</p>}
        </section>
      </div>
    </div>
  )
}
