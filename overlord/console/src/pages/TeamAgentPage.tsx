import { useState, useEffect, useCallback } from 'react'
import { Bot, Plus, Zap, Target, XCircle, ChevronRight, Code, Megaphone, Headphones, BarChart3, Loader2, Users, TrendingUp, ShoppingCart, Film, Crosshair, Shield, Cpu, Wrench, ArrowRight, Server, HeartPulse } from 'lucide-react'
import { broodAPI, TeamAgentTemplate, TeamInstance, TeamMission, TeamAgentStats, ClawNode, ClawModel, ClawSkill, ClawAgentTemplate, AgentSandboxResp, EmployeeUsage, ProvisionResult } from '../api/brood'
import { useTeamAgentWS } from '../hooks/useTeamAgentWS'

const statusColors: Record<string, string> = {
  forming: 'bg-yellow-500/15 text-yellow-400',
  ready: 'bg-blue-500/15 text-blue-400',
  running: 'bg-green-500/15 text-green-400',
  paused: 'bg-orange-500/15 text-orange-400',
  maintenance: 'bg-purple-500/15 text-purple-400',
  completed: 'bg-gray-500/15 text-gray-400',
  disbanded: 'bg-red-500/15 text-red-400',
}

const statusLabels: Record<string, string> = {
  forming: '组建中',
  ready: '就绪',
  running: '运行中',
  paused: '已暂停',
  maintenance: '维护中',
  completed: '已完成',
  disbanded: '已解散',
}

const missionStatusLabels: Record<string, string> = {
  planning: '规划中',
  confirming: '确认中',
  executing: '执行中',
  reviewing: '审查中',
  completed: '已完成',
  failed: '失败',
  cancelled: '已取消',
}

const categoryIcons: Record<string, typeof Code> = {
  development: Code,
  marketing: Megaphone,
  support: Headphones,
  data: BarChart3,
  finance: TrendingUp,
  ecommerce: ShoppingCart,
  content: Film,
  sales: Crosshair,
  ops: Shield,
  medical: HeartPulse,
}

type View = 'overview' | 'templates' | 'detail'

export default function TeamAgentPage() {
  const [view, setView] = useState<View>('overview')
  const [stats, setStats] = useState<TeamAgentStats | null>(null)
  const [templates, setTemplates] = useState<TeamAgentTemplate[]>([])
  const [instances, setInstances] = useState<TeamInstance[]>([])
  const [selectedInstance, setSelectedInstance] = useState<TeamInstance | null>(null)
  const [missions, setMissions] = useState<TeamMission[]>([])
  const [loading, setLoading] = useState(true)
  const [employeeUsage, setEmployeeUsage] = useState<EmployeeUsage[]>([])

  // Create modal (lobby)
  const [showCreate, setShowCreate] = useState(false)
  const [createStep, setCreateStep] = useState<1 | 2>(1)
  const [createTmplId, setCreateTmplId] = useState('')
  const [createName, setCreateName] = useState('')
  const [createGoal, setCreateGoal] = useState('')
  const [createBudget, setCreateBudget] = useState(5000)
  const [nodes, setNodes] = useState<ClawNode[]>([])
  const [roleNodeMap, setRoleNodeMap] = useState<Record<string, string>>({})
  const [creating, setCreating] = useState(false)

  // Provision node (one-click Spark deploy)
  const [provisioning, setProvisioning] = useState(false)
  const [provisionSlug, setProvisionSlug] = useState('')

  // New mission modal
  const [showMission, setShowMission] = useState(false)
  const [missionGoal, setMissionGoal] = useState('')
  const [creatingMission, setCreatingMission] = useState(false)

  // Role edit modal
  const [editingRole, setEditingRole] = useState<string | null>(null) // role code
  const [roleModels, setRoleModels] = useState<ClawModel[]>([])
  const [roleOverrides, setRoleOverrides] = useState<Record<string, { model: string; system_prompt: string; tools: string[] }>>({})
  const [roleModelLoading, setRoleModelLoading] = useState(false)
  const [savingRoles, setSavingRoles] = useState(false)
  const [instanceDefaultModel, setInstanceDefaultModel] = useState('')
  const [nodeSkills, setNodeSkills] = useState<ClawSkill[]>([])
  const [nodeAgents, setNodeAgents] = useState<ClawAgentTemplate[]>([])
  const [showAgentPicker, setShowAgentPicker] = useState(false)

  // Agent Dev Workshop
  const [showDevWorkshop, setShowDevWorkshop] = useState(false)
  const [devAgentName, setDevAgentName] = useState('')
  const [devAgentPrompt, setDevAgentPrompt] = useState('')
  const [devAgentModel, setDevAgentModel] = useState('')
  const [devAgentTools, setDevAgentTools] = useState<string[]>([])
  const [devAgentCategory, setDevAgentCategory] = useState('assistant')
  const [devAgentIcon, setDevAgentIcon] = useState('')
  const [devAgentDesc, setDevAgentDesc] = useState('')
  const [devTestInput, setDevTestInput] = useState('')
  const [devTestMessages, setDevTestMessages] = useState<{ role: string; content: string }[]>([])
  const [sandboxResult, setSandboxResult] = useState<AgentSandboxResp | null>(null)
  const [sandboxLoading, setSandboxLoading] = useState(false)
  const [publishLoading, setPublishLoading] = useState(false)
  const [publishResult, setPublishResult] = useState<string | null>(null)

  // Real-time WS updates — refresh missions when status changes
  const onMissionUpdate = useCallback((data: { mission_id?: string; instance_id?: string; status?: string }) => {
    if (selectedInstance && data.instance_id === selectedInstance.id) {
      broodAPI.listTeamMissions(selectedInstance.id).then(r => setMissions(r.missions || [])).catch(() => {})
    }
    // Refresh stats on status change
    if (data.status === 'completed' || data.status === 'failed') {
      broodAPI.teamAgentStats().then(setStats).catch(() => {})
    }
  }, [selectedInstance])
  useTeamAgentWS(undefined, onMissionUpdate)

  useEffect(() => { load() }, [])

  async function load() {
    setLoading(true)
    try {
      const [s, t, i, u] = await Promise.all([
        broodAPI.teamAgentStats(),
        broodAPI.listTeamTemplates(),
        broodAPI.listTeamInstances(),
        broodAPI.teamAgentUsageByUser().catch(() => ({ users: [], total: 0 })),
      ])
      setStats(s)
      setTemplates(t.templates || [])
      setInstances(i.instances || [])
      setEmployeeUsage(u.users || [])
    } catch { /* ignore */ }
    setLoading(false)
  }

  async function openCreate(tmplId: string) {
    setCreateTmplId(tmplId)
    setCreateStep(1)
    setCreateName('')
    setCreateGoal('')
    setCreateBudget(5000)
    setRoleNodeMap({})
    setShowCreate(true)
    try {
      const res = await broodAPI.listClaws({ status: 'online' })
      setNodes(res.claws || [])
      // Auto-assign first node to all roles
      const tmpl = templates.find(t => t.id === tmplId)
      if (tmpl && res.claws?.length) {
        const roles = parseRoles(tmpl.roles)
        const map: Record<string, string> = {}
        roles.forEach(r => { map[r.code] = res.claws[0].id })
        setRoleNodeMap(map)
      }
    } catch { /* ignore */ }
  }

  async function handleCreate() {
    const primaryNodeId = Object.values(roleNodeMap)[0] || ''
    if (!createTmplId || !primaryNodeId || !createName) return
    setCreating(true)
    try {
      await broodAPI.createTeamInstance({
        template_id: createTmplId,
        claw_node_id: primaryNodeId,
        name: createName,
        goal: createGoal,
        energy_budget: createBudget,
      })
      setShowCreate(false)
      load()
    } catch { /* ignore */ }
    setCreating(false)
  }

  async function handleProvision() {
    setProvisioning(true)
    try {
      const res = await broodAPI.provisionNode()
      setProvisionSlug(res.slug)
      if (res.status === 'ready' && res.node_id) {
        // Node is already online — refresh list
        const claws = await broodAPI.listClaws({ status: 'online' })
        setNodes(claws.claws || [])
        const tmpl = templates.find(t => t.id === createTmplId)
        if (tmpl && claws.claws?.length) {
          const roles = parseRoles(tmpl.roles)
          const map: Record<string, string> = {}
          roles.forEach(r => { map[r.code] = claws.claws[0].id })
          setRoleNodeMap(map)
        }
      } else {
        // Poll for node to come online (max 90s)
        for (let i = 0; i < 30; i++) {
          await new Promise(r => setTimeout(r, 3000))
          const status = await broodAPI.provisionStatus(res.slug)
          if (status.status === 'ready' && status.node_id) {
            const claws = await broodAPI.listClaws({ status: 'online' })
            setNodes(claws.claws || [])
            const tmpl = templates.find(t => t.id === createTmplId)
            if (tmpl && claws.claws?.length) {
              const roles = parseRoles(tmpl.roles)
              const map: Record<string, string> = {}
              roles.forEach(r => { map[r.code] = claws.claws[0].id })
              setRoleNodeMap(map)
            }
            break
          }
        }
      }
    } catch { /* ignore */ }
    setProvisioning(false)
  }

  function assignNode(roleCode: string, nodeId: string) {
    setRoleNodeMap(prev => ({ ...prev, [roleCode]: nodeId }))
  }

  const modelColors: Record<string, string> = {
    'gpt-4o': 'text-green-400 bg-green-500/10',
    'deepseek-chat': 'text-blue-400 bg-blue-500/10',
    'claude-3': 'text-orange-400 bg-orange-500/10',
  }

  const roleIcons: Record<string, string> = {
    architect: '🏗️', designer: '🏗️', drone: '⚡', coder: '⚡',
    tester: '🧪', reviewer: '🔍', docbot: '📝', reporter: '📊',
    strategist: '🎯', copywriter: '✍️', analyst: '📈',
    dispatcher: '📡', guardian: '🛡️', scout: '🔭',
  }

  async function openRoleEdit(inst: TeamInstance, roleCode: string) {
    setEditingRole(roleCode)
    setShowAgentPicker(false)
    setInstanceDefaultModel(inst.default_model || '')
    // Parse existing overrides from config
    try {
      const cfg = inst.config ? JSON.parse(inst.config) : {}
      setRoleOverrides(cfg.role_overrides || {})
    } catch { setRoleOverrides({}) }
    // Fetch models, skills, agents from the Claw node in parallel
    if (inst.claw_node_id) {
      setRoleModelLoading(true)
      try {
        const [modelsRes, skillsRes, agentsRes] = await Promise.all([
          broodAPI.nodeModels(inst.claw_node_id).catch(() => ({ models: [], total: 0, node_name: '' })),
          broodAPI.nodeSkills(inst.claw_node_id).catch(() => ({ skills: [], plugins: [], mcp_servers: [], total: 0, node_name: '' })),
          broodAPI.nodeAgents(inst.claw_node_id).catch(() => ({ agents: [], categories: [], total: 0, node_name: '' })),
        ])
        setRoleModels(modelsRes.models || [])
        setNodeSkills(skillsRes.skills || [])
        setNodeAgents(agentsRes.agents || [])
      } catch {
        setRoleModels([]); setNodeSkills([]); setNodeAgents([])
      }
      setRoleModelLoading(false)
    }
  }

  function updateRoleOverride(code: string, field: string, value: string | string[]) {
    setRoleOverrides(prev => ({
      ...prev,
      [code]: { ...(prev[code] || { model: '', system_prompt: '', tools: [] }), [field]: value },
    }))
  }

  // Agent Dev Workshop functions
  async function runSandboxTest() {
    if (!selectedInstance || devTestMessages.length === 0) return
    setSandboxLoading(true)
    setSandboxResult(null)
    try {
      const res = await broodAPI.agentSandbox(selectedInstance.id, {
        name: devAgentName || 'Untitled Agent',
        system_prompt: devAgentPrompt,
        model: devAgentModel || undefined,
        tools: devAgentTools.length > 0 ? JSON.stringify(devAgentTools) : undefined,
        test_messages: devTestMessages,
      })
      setSandboxResult(res)
    } catch (e: unknown) {
      alert('沙箱测试失败: ' + (e instanceof Error ? e.message : '未知错误'))
    }
    setSandboxLoading(false)
  }

  async function publishAgent() {
    if (!selectedInstance || !devAgentName || !devAgentPrompt) return
    setPublishLoading(true)
    setPublishResult(null)
    try {
      const res = await broodAPI.agentPublish(selectedInstance.id, {
        name: devAgentName,
        description: devAgentDesc,
        system_prompt: devAgentPrompt,
        model: devAgentModel || undefined,
        tools: devAgentTools.length > 0 ? JSON.stringify(devAgentTools) : undefined,
        category: devAgentCategory,
        icon: devAgentIcon,
      })
      setPublishResult(`上架成功! 模板 ID: ${res.template_id}`)
    } catch (e: unknown) {
      alert('发布失败: ' + (e instanceof Error ? e.message : '未知错误'))
    }
    setPublishLoading(false)
  }

  function addTestMessage() {
    if (!devTestInput.trim()) return
    setDevTestMessages(prev => [...prev, { role: 'user', content: devTestInput.trim() }])
    setDevTestInput('')
  }

  function openDevWorkshop() {
    setShowDevWorkshop(true)
    setSandboxResult(null)
    setPublishResult(null)
    // Fetch models and skills if not loaded
    if (selectedInstance?.claw_node_id && roleModels.length === 0) {
      broodAPI.nodeModels(selectedInstance.claw_node_id).then(r => setRoleModels(r.models || [])).catch(() => {})
      broodAPI.nodeSkills(selectedInstance.claw_node_id).then(r => setNodeSkills(r.skills || [])).catch(() => {})
    }
  }

  async function saveRoleOverrides() {
    if (!selectedInstance) return
    setSavingRoles(true)
    try {
      const res = await broodAPI.updateInstanceRoles(selectedInstance.id, {
        role_overrides: roleOverrides,
        default_model: instanceDefaultModel || undefined,
      })
      // Update local instance config
      setSelectedInstance({ ...selectedInstance, config: res.config, default_model: instanceDefaultModel })
      setEditingRole(null)
    } catch { /* ignore */ }
    setSavingRoles(false)
  }

  async function selectInstance(inst: TeamInstance) {
    setSelectedInstance(inst)
    setView('detail')
    // Load existing role overrides
    try {
      const cfg = inst.config ? JSON.parse(inst.config) : {}
      setRoleOverrides(cfg.role_overrides || {})
    } catch { setRoleOverrides({}) }
    setInstanceDefaultModel(inst.default_model || '')
    try {
      const res = await broodAPI.listTeamMissions(inst.id)
      setMissions(res.missions || [])
    } catch { /* ignore */ }
  }

  async function handleDisband(id: string) {
    if (!confirm('确定解散此团队？')) return
    try {
      await broodAPI.disbandTeamInstance(id)
      setView('overview')
      load()
    } catch { /* ignore */ }
  }

  async function handleCreateMission() {
    if (!selectedInstance || !missionGoal.trim()) return
    setCreatingMission(true)
    try {
      await broodAPI.createTeamMission(selectedInstance.id, { goal: missionGoal })
      setShowMission(false)
      setMissionGoal('')
      const res = await broodAPI.listTeamMissions(selectedInstance.id)
      setMissions(res.missions || [])
      load()
    } catch { /* ignore */ }
    setCreatingMission(false)
  }

  interface ParsedRole {
    code: string
    name: string
    system_prompt?: string
    model?: string
    tools?: string[]
    max_instances: number
  }

  function parseRoles(rolesJSON: string): ParsedRole[] {
    try { return JSON.parse(rolesJSON) } catch { return [] }
  }

  if (loading) return <div className="flex items-center justify-center h-full"><Loader2 className="w-6 h-6 animate-spin text-overlord-400" /></div>

  // ─── Detail View ───
  if (view === 'detail' && selectedInstance) {
    const inst = selectedInstance
    const tmpl = templates.find(t => t.id === inst.template_id)
    const roles = tmpl ? parseRoles(tmpl.roles) : []
    const progress = inst.energy_budget > 0 ? Math.min(100, Math.round(inst.energy_used / inst.energy_budget * 100)) : 0

    return (
      <div className="p-6 space-y-6">
        {/* Header */}
        <div className="flex items-center gap-3">
          <button onClick={() => { setView('overview'); setSelectedInstance(null) }} className="text-gray-400 hover:text-white transition text-sm">← 返回</button>
          <div className="flex-1" />
          <button onClick={() => setShowMission(true)} className="flex items-center gap-2 px-4 py-2 bg-overlord-600 hover:bg-overlord-500 text-white rounded-lg text-sm transition">
            <Plus className="w-4 h-4" /> 新建任务
          </button>
          {inst.status !== 'disbanded' && (
            <button onClick={() => handleDisband(inst.id)} className="flex items-center gap-2 px-4 py-2 bg-red-600/20 hover:bg-red-600/30 text-red-400 rounded-lg text-sm transition">
              <XCircle className="w-4 h-4" /> 解散
            </button>
          )}
        </div>

        {/* Team info card */}
        <div className="bg-gray-800/50 border border-gray-700/50 rounded-xl p-6">
          <div className="flex items-start gap-4">
            <div className="w-12 h-12 rounded-xl bg-overlord-600/20 flex items-center justify-center shrink-0">
              <Bot className="w-6 h-6 text-overlord-400" />
            </div>
            <div className="flex-1 min-w-0">
              <div className="flex items-center gap-3">
                <h2 className="text-xl font-bold text-white">{inst.name}</h2>
                <span className={`px-2 py-0.5 rounded text-xs font-medium ${statusColors[inst.status] || 'bg-gray-600 text-gray-300'}`}>
                  {statusLabels[inst.status] || inst.status}
                </span>
              </div>
              <div className="text-sm text-gray-400 mt-1">{inst.template_name} · {inst.goal || '无目标描述'}</div>
              <div className="text-xs text-gray-500 mt-1">创建于 {new Date(inst.created_at).toLocaleString()}</div>
            </div>
          </div>

          {/* Stats row */}
          <div className="grid grid-cols-4 gap-4 mt-6">
            <div className="bg-gray-900/50 rounded-lg p-3 text-center">
              <div className="text-2xl font-bold text-white">{inst.mission_count}</div>
              <div className="text-xs text-gray-500">任务数</div>
            </div>
            <div className="bg-gray-900/50 rounded-lg p-3 text-center">
              <div className="text-2xl font-bold text-overlord-400">{inst.avg_score > 0 ? inst.avg_score.toFixed(1) : '-'}</div>
              <div className="text-xs text-gray-500">平均评分</div>
            </div>
            <div className="bg-gray-900/50 rounded-lg p-3 text-center">
              <div className="text-2xl font-bold text-yellow-400">{inst.energy_used}⚡</div>
              <div className="text-xs text-gray-500">已消耗星能</div>
            </div>
            <div className="bg-gray-900/50 rounded-lg p-3 text-center">
              <div className="text-2xl font-bold text-white">{inst.energy_budget}⚡</div>
              <div className="text-xs text-gray-500">星能预算</div>
            </div>
          </div>

          {/* Energy progress bar */}
          {inst.energy_budget > 0 && (
            <div className="mt-4">
              <div className="flex justify-between text-xs text-gray-500 mb-1">
                <span>星能消耗</span>
                <span>{progress}%</span>
              </div>
              <div className="h-2 bg-gray-700 rounded-full overflow-hidden">
                <div className={`h-full rounded-full transition-all ${progress > 80 ? 'bg-red-500' : progress > 60 ? 'bg-yellow-500' : 'bg-overlord-500'}`} style={{ width: `${progress}%` }} />
              </div>
            </div>
          )}
        </div>

        {/* Roles */}
        {roles.length > 0 && (
          <div>
            <div className="flex items-center justify-between mb-3">
              <h3 className="text-sm font-semibold text-gray-300">团队角色</h3>
              <span className="text-[10px] text-gray-600">点击角色配置模型和技能</span>
            </div>
            <div className="grid grid-cols-2 lg:grid-cols-4 gap-3">
              {roles.map(r => {
                const ov = roleOverrides[r.code]
                const effectiveModel = ov?.model || r.model || ''
                return (
                  <div
                    key={r.code}
                    onClick={() => openRoleEdit(inst, r.code)}
                    className={`bg-gray-800/50 border rounded-xl p-3 cursor-pointer transition hover:border-overlord-600/50 hover:shadow-lg hover:shadow-overlord-600/5 ${
                      ov?.model ? 'border-overlord-600/30' : 'border-gray-700/50'
                    }`}
                  >
                    <div className="flex items-center gap-2 mb-2">
                      <span className="text-lg">{roleIcons[r.code] || '🤖'}</span>
                      <span className="text-sm font-bold text-white">{r.name}</span>
                      {r.max_instances > 1 && <span className="text-[10px] text-gray-500">×{r.max_instances}</span>}
                    </div>
                    {effectiveModel ? (
                      <div className="flex items-center gap-1 mb-1">
                        <Cpu className="w-3 h-3 text-overlord-400" />
                        <span className={`text-[10px] px-1.5 py-0.5 rounded ${modelColors[effectiveModel] || 'text-gray-400 bg-gray-700/50'}`}>{effectiveModel}</span>
                      </div>
                    ) : (
                      <div className="text-[10px] text-gray-600 mb-1">未配置模型</div>
                    )}
                    {(ov?.tools?.length || r.tools?.length) ? (
                      <div className="flex items-center gap-1">
                        <Wrench className="w-2.5 h-2.5 text-gray-500" />
                        <span className="text-[10px] text-gray-500">{(ov?.tools || r.tools || []).length} 个工具</span>
                      </div>
                    ) : null}
                  </div>
                )
              })}
            </div>
          </div>
        )}

        {/* Role Edit Modal */}
        {editingRole && (() => {
          const role = roles.find(r => r.code === editingRole)
          if (!role) return null
          const ov = roleOverrides[editingRole] || { model: role.model || '', system_prompt: role.system_prompt || '', tools: role.tools || [] }
          const selectedTools = new Set(ov.tools || [])
          const toggleTool = (name: string) => {
            const next = new Set(selectedTools)
            if (next.has(name)) next.delete(name); else next.add(name)
            updateRoleOverride(editingRole, 'tools', Array.from(next))
          }
          const applyAgentPreset = (agent: typeof nodeAgents[0]) => {
            const tools: string[] = []
            try { const parsed = JSON.parse(agent.tools || '[]'); if (Array.isArray(parsed)) tools.push(...parsed) } catch {}
            updateRoleOverride(editingRole, 'system_prompt', agent.system_prompt || '')
            updateRoleOverride(editingRole, 'model', agent.model || '')
            if (tools.length > 0) updateRoleOverride(editingRole, 'tools', tools)
            setShowAgentPicker(false)
          }
          const skillTypeColors: Record<string, string> = {
            builtin: 'bg-green-500/10 text-green-400 border-green-500/20',
            plugin: 'bg-blue-500/10 text-blue-400 border-blue-500/20',
            mcp: 'bg-purple-500/10 text-purple-400 border-purple-500/20',
          }
          return (
            <div className="fixed inset-0 bg-black/60 flex items-center justify-center z-50" onClick={() => setEditingRole(null)}>
              <div className="bg-gray-800 border border-gray-700 rounded-xl w-full max-w-2xl max-h-[90vh] flex flex-col" onClick={e => e.stopPropagation()}>
                <div className="px-5 py-4 border-b border-gray-700 flex items-center gap-3 shrink-0">
                  <span className="text-xl">{roleIcons[role.code] || '🤖'}</span>
                  <div className="flex-1">
                    <h3 className="text-base font-bold text-white">{role.name}</h3>
                    <p className="text-[11px] text-gray-500">配置此角色的模型、提示词和技能</p>
                  </div>
                  {nodeAgents.length > 0 && (
                    <button
                      onClick={() => setShowAgentPicker(!showAgentPicker)}
                      className="px-3 py-1.5 text-xs bg-overlord-600/20 text-overlord-400 hover:bg-overlord-600/30 rounded-lg transition flex items-center gap-1"
                    >
                      <Bot className="w-3 h-3" /> {showAgentPicker ? '返回编辑' : '从市场导入'}
                    </button>
                  )}
                </div>
                <div className="flex-1 overflow-y-auto p-5 space-y-4">
                  {showAgentPicker ? (
                    /* Agent Marketplace Picker */
                    <div>
                      <p className="text-xs text-gray-400 mb-3">选择一个智能体模板，将其提示词、模型和技能导入到当前角色</p>
                      <div className="grid grid-cols-2 gap-2">
                        {nodeAgents.map(agent => (
                          <div
                            key={agent.id}
                            onClick={() => applyAgentPreset(agent)}
                            className="bg-gray-900/60 border border-gray-700/50 rounded-lg p-3 cursor-pointer hover:border-overlord-600/50 transition"
                          >
                            <div className="flex items-center gap-2 mb-1.5">
                              <span className="text-sm">{agent.icon || '🤖'}</span>
                              <span className="text-xs font-bold text-white truncate">{agent.name}</span>
                              {agent.is_official && <span className="text-[9px] bg-overlord-600/20 text-overlord-400 px-1 py-0.5 rounded">官方</span>}
                            </div>
                            <p className="text-[10px] text-gray-500 line-clamp-2 mb-1.5">{agent.description}</p>
                            <div className="flex items-center gap-2 text-[10px] text-gray-600">
                              {agent.model && <span className="flex items-center gap-0.5"><Cpu className="w-2.5 h-2.5" />{agent.model}</span>}
                              {agent.rating > 0 && <span>{'⭐'.repeat(Math.round(agent.rating))}</span>}
                              <span>{agent.install_count} 次安装</span>
                            </div>
                          </div>
                        ))}
                      </div>
                    </div>
                  ) : (
                    /* Normal Edit View */
                    <>
                      {/* Model */}
                      <div>
                        <label className="text-xs text-gray-400 mb-1.5 block">AI 模型</label>
                        {roleModelLoading ? (
                          <div className="flex items-center gap-2 text-xs text-gray-500"><Loader2 className="w-3.5 h-3.5 animate-spin" /> 加载中...</div>
                        ) : (
                          <select
                            className="w-full bg-gray-900 border border-gray-700 rounded-lg px-3 py-2 text-sm text-white focus:border-overlord-500 focus:outline-none"
                            value={ov.model}
                            onChange={e => updateRoleOverride(editingRole, 'model', e.target.value)}
                          >
                            <option value="">使用默认模型</option>
                            {roleModels.map(m => (
                              <option key={m.id} value={m.model_name}>
                                {m.model_name} ({m.provider}{m.display_name && m.display_name !== m.model_name ? ` · ${m.display_name}` : ''})
                              </option>
                            ))}
                          </select>
                        )}
                      </div>
                      {/* System Prompt */}
                      <div>
                        <label className="text-xs text-gray-400 mb-1.5 block">系统提示词</label>
                        <textarea
                          className="w-full bg-gray-900 border border-gray-700 rounded-lg px-3 py-2 text-sm text-white placeholder-gray-500 focus:border-overlord-500 focus:outline-none font-mono"
                          rows={5}
                          placeholder="自定义此角色的行为指令..."
                          value={ov.system_prompt}
                          onChange={e => updateRoleOverride(editingRole, 'system_prompt', e.target.value)}
                        />
                      </div>
                      {/* Skills — Selectable Cards */}
                      <div>
                        <label className="text-xs text-gray-400 mb-1.5 block">技能 / 工具</label>
                        {nodeSkills.length > 0 ? (
                          <div className="flex flex-wrap gap-1.5">
                            {nodeSkills.map(s => {
                              const active = selectedTools.has(s.name)
                              return (
                                <button
                                  key={s.name}
                                  onClick={() => toggleTool(s.name)}
                                  title={s.description}
                                  className={`text-[11px] px-2 py-1 rounded-lg border transition flex items-center gap-1 ${
                                    active
                                      ? 'bg-overlord-600/20 text-overlord-300 border-overlord-500/40'
                                      : `${skillTypeColors[s.type] || 'bg-gray-700/30 text-gray-500 border-gray-700/50'} hover:border-gray-600`
                                  }`}
                                >
                                  <Wrench className="w-2.5 h-2.5" />
                                  {s.name}
                                  {active && <XCircle className="w-2.5 h-2.5 ml-0.5" />}
                                </button>
                              )
                            })}
                          </div>
                        ) : (
                          <input
                            className="w-full bg-gray-900 border border-gray-700 rounded-lg px-3 py-2 text-sm text-white placeholder-gray-500 focus:border-overlord-500 focus:outline-none"
                            placeholder="web_search, code_exec, file_read (逗号分隔)"
                            value={(ov.tools || []).join(', ')}
                            onChange={e => updateRoleOverride(editingRole, 'tools', e.target.value.split(',').map(s => s.trim()).filter(Boolean))}
                          />
                        )}
                        {selectedTools.size > 0 && (
                          <div className="text-[10px] text-gray-600 mt-1.5">已选 {selectedTools.size} 个技能</div>
                        )}
                      </div>
                      {/* Instance Default Model */}
                      <div className="pt-2 border-t border-gray-700/50">
                        <label className="text-xs text-gray-400 mb-1.5 block">实例默认聊天模型</label>
                        <select
                          className="w-full bg-gray-900 border border-gray-700 rounded-lg px-3 py-2 text-sm text-white focus:border-overlord-500 focus:outline-none"
                          value={instanceDefaultModel}
                          onChange={e => setInstanceDefaultModel(e.target.value)}
                        >
                          <option value="">跟随模板默认</option>
                          {roleModels.map(m => (
                            <option key={m.id} value={m.model_name}>{m.model_name} ({m.provider})</option>
                          ))}
                        </select>
                        <p className="text-[10px] text-gray-600 mt-1">员工发起聊天时使用的默认模型</p>
                      </div>
                    </>
                  )}
                </div>
                <div className="px-5 py-3 border-t border-gray-700 flex justify-end gap-3 shrink-0">
                  <button onClick={() => setEditingRole(null)} className="px-4 py-2 text-sm text-gray-400 hover:text-white transition">取消</button>
                  <button onClick={saveRoleOverrides} disabled={savingRoles} className="px-4 py-2 bg-overlord-600 hover:bg-overlord-500 text-white rounded-lg text-sm transition disabled:opacity-50">
                    {savingRoles ? '保存中...' : '保存配置'}
                  </button>
                </div>
              </div>
            </div>
          )
        })()}

        {/* Agent Dev Workshop Button — only for DevClaw instances */}
        {selectedInstance && templates.find(t => t.id === selectedInstance.template_id)?.name === 'DevClaw' && !showDevWorkshop && (
          <button
            onClick={openDevWorkshop}
            className="w-full bg-gradient-to-r from-overlord-600/20 to-purple-600/20 border border-overlord-500/30 rounded-xl p-4 flex items-center gap-3 hover:border-overlord-500/60 transition group"
          >
            <div className="w-10 h-10 bg-overlord-600/30 rounded-lg flex items-center justify-center">
              <Wrench className="w-5 h-5 text-overlord-400" />
            </div>
            <div className="flex-1 text-left">
              <div className="text-sm font-bold text-white">Agent 开发工坊</div>
              <div className="text-[11px] text-gray-400">在 DevClaw 中开发新的智能体、技能、工作流，测试后一键上架到市场</div>
            </div>
            <ArrowRight className="w-4 h-4 text-gray-500 group-hover:text-overlord-400 transition" />
          </button>
        )}

        {/* Agent Dev Workshop Panel */}
        {showDevWorkshop && selectedInstance && (
          <div className="bg-gray-800/50 border border-overlord-500/30 rounded-xl overflow-hidden">
            <div className="px-5 py-3 border-b border-gray-700/50 flex items-center justify-between">
              <div className="flex items-center gap-2">
                <Wrench className="w-4 h-4 text-overlord-400" />
                <span className="text-sm font-bold text-white">Agent 开发工坊</span>
                <span className="text-[10px] text-gray-500 bg-gray-700/50 px-1.5 py-0.5 rounded">DevClaw</span>
              </div>
              <button onClick={() => setShowDevWorkshop(false)} className="text-gray-500 hover:text-white transition">
                <XCircle className="w-4 h-4" />
              </button>
            </div>
            <div className="p-5 space-y-4">
              {/* Basic info row */}
              <div className="grid grid-cols-3 gap-3">
                <div>
                  <label className="text-[10px] text-gray-500 mb-1 block">智能体名称 *</label>
                  <input className="w-full bg-gray-900 border border-gray-700 rounded-lg px-3 py-1.5 text-sm text-white placeholder-gray-600 focus:border-overlord-500 focus:outline-none" placeholder="例: 药理虫" value={devAgentName} onChange={e => setDevAgentName(e.target.value)} />
                </div>
                <div>
                  <label className="text-[10px] text-gray-500 mb-1 block">分类</label>
                  <select className="w-full bg-gray-900 border border-gray-700 rounded-lg px-3 py-1.5 text-sm text-white focus:border-overlord-500 focus:outline-none" value={devAgentCategory} onChange={e => setDevAgentCategory(e.target.value)}>
                    <option value="assistant">通用助手</option>
                    <option value="coding">编程开发</option>
                    <option value="writing">写作创作</option>
                    <option value="data">数据分析</option>
                    <option value="medical">医疗健康</option>
                    <option value="finance">金融财务</option>
                    <option value="legal">法律合规</option>
                    <option value="education">教育培训</option>
                    <option value="creative">创意设计</option>
                  </select>
                </div>
                <div>
                  <label className="text-[10px] text-gray-500 mb-1 block">图标 + 模型</label>
                  <div className="flex gap-2">
                    <input className="w-16 bg-gray-900 border border-gray-700 rounded-lg px-2 py-1.5 text-sm text-center focus:border-overlord-500 focus:outline-none" placeholder="emoji" value={devAgentIcon} onChange={e => setDevAgentIcon(e.target.value)} />
                    <select className="flex-1 bg-gray-900 border border-gray-700 rounded-lg px-2 py-1.5 text-sm text-white focus:border-overlord-500 focus:outline-none" value={devAgentModel} onChange={e => setDevAgentModel(e.target.value)}>
                      <option value="">默认模型</option>
                      {roleModels.map(m => <option key={m.id} value={m.model_name}>{m.model_name}</option>)}
                    </select>
                  </div>
                </div>
              </div>
              {/* Description */}
              <div>
                <label className="text-[10px] text-gray-500 mb-1 block">一句话描述</label>
                <input className="w-full bg-gray-900 border border-gray-700 rounded-lg px-3 py-1.5 text-sm text-white placeholder-gray-600 focus:border-overlord-500 focus:outline-none" placeholder="这个智能体能做什么..." value={devAgentDesc} onChange={e => setDevAgentDesc(e.target.value)} />
              </div>
              {/* System Prompt */}
              <div>
                <label className="text-[10px] text-gray-500 mb-1 block">System Prompt *</label>
                <textarea className="w-full bg-gray-900 border border-gray-700 rounded-lg px-3 py-2 text-sm text-white placeholder-gray-600 focus:border-overlord-500 focus:outline-none font-mono" rows={6} placeholder="你是一个..." value={devAgentPrompt} onChange={e => setDevAgentPrompt(e.target.value)} />
              </div>
              {/* Skills selection */}
              {nodeSkills.length > 0 && (
                <div>
                  <label className="text-[10px] text-gray-500 mb-1 block">技能 / 工具</label>
                  <div className="flex flex-wrap gap-1">
                    {nodeSkills.map(s => {
                      const active = devAgentTools.includes(s.name)
                      return (
                        <button key={s.name} title={s.description} onClick={() => setDevAgentTools(prev => active ? prev.filter(t => t !== s.name) : [...prev, s.name])}
                          className={`text-[10px] px-1.5 py-0.5 rounded border transition ${active ? 'bg-overlord-600/20 text-overlord-300 border-overlord-500/40' : 'bg-gray-700/20 text-gray-500 border-gray-700/40 hover:border-gray-600'}`}
                        >
                          {s.name}{active && ' ✓'}
                        </button>
                      )
                    })}
                  </div>
                </div>
              )}
              {/* Test Messages */}
              <div className="border-t border-gray-700/50 pt-4">
                <label className="text-[10px] text-gray-500 mb-1.5 block">沙箱测试用例</label>
                {devTestMessages.length > 0 && (
                  <div className="space-y-1 mb-2">
                    {devTestMessages.map((tm, i) => (
                      <div key={i} className="flex items-center gap-2 text-xs">
                        <span className="text-gray-600 w-4">{i + 1}.</span>
                        <span className="flex-1 text-gray-300 truncate">{tm.content}</span>
                        <button onClick={() => setDevTestMessages(prev => prev.filter((_, j) => j !== i))} className="text-gray-600 hover:text-red-400"><XCircle className="w-3 h-3" /></button>
                      </div>
                    ))}
                  </div>
                )}
                <div className="flex gap-2">
                  <input className="flex-1 bg-gray-900 border border-gray-700 rounded-lg px-3 py-1.5 text-sm text-white placeholder-gray-600 focus:border-overlord-500 focus:outline-none" placeholder="输入测试问题..." value={devTestInput} onChange={e => setDevTestInput(e.target.value)} onKeyDown={e => e.key === 'Enter' && addTestMessage()} />
                  <button onClick={addTestMessage} className="px-3 py-1.5 bg-gray-700 hover:bg-gray-600 text-white rounded-lg text-xs transition">添加</button>
                </div>
              </div>
              {/* Sandbox Results */}
              {sandboxResult && (
                <div className="border-t border-gray-700/50 pt-4">
                  <div className="flex items-center gap-3 mb-3">
                    <span className="text-xs font-bold text-white">测试结果</span>
                    <span className={`text-[10px] px-2 py-0.5 rounded ${sandboxResult.ready_to_publish ? 'bg-green-500/15 text-green-400' : 'bg-red-500/15 text-red-400'}`}>
                      {sandboxResult.ready_to_publish ? '可以发布' : '需要改进'} · 得分 {sandboxResult.overall_score.toFixed(1)}
                    </span>
                    <span className="text-[10px] text-gray-500">{sandboxResult.pass_count}/{sandboxResult.total_tests} 通过</span>
                  </div>
                  <div className="space-y-2">
                    {sandboxResult.results.map((r, i) => (
                      <div key={i} className="bg-gray-900/50 rounded-lg p-3">
                        <div className="flex items-center gap-2 mb-1">
                          <span className={`text-[10px] px-1.5 py-0.5 rounded ${r.verdict === 'pass' ? 'bg-green-500/15 text-green-400' : r.verdict === 'warning' ? 'bg-yellow-500/15 text-yellow-400' : 'bg-red-500/15 text-red-400'}`}>{r.verdict}</span>
                          <span className="text-[10px] text-gray-400 truncate">{r.input}</span>
                        </div>
                        <div className="text-[11px] text-gray-300 line-clamp-3">{r.output || r.error}</div>
                      </div>
                    ))}
                  </div>
                </div>
              )}
              {/* Publish Result */}
              {publishResult && (
                <div className="bg-green-500/10 border border-green-500/20 rounded-lg p-3 text-xs text-green-400">{publishResult}</div>
              )}
              {/* Actions */}
              <div className="flex justify-end gap-3 pt-2">
                <button onClick={runSandboxTest} disabled={sandboxLoading || !devAgentPrompt || devTestMessages.length === 0}
                  className="px-4 py-2 bg-blue-600/20 hover:bg-blue-600/30 text-blue-400 rounded-lg text-xs transition disabled:opacity-30 flex items-center gap-1.5">
                  {sandboxLoading ? <><Loader2 className="w-3 h-3 animate-spin" /> 测试中...</> : <><Zap className="w-3 h-3" /> 沙箱测试</>}
                </button>
                <button onClick={publishAgent} disabled={publishLoading || !devAgentName || !devAgentPrompt}
                  className="px-4 py-2 bg-overlord-600 hover:bg-overlord-500 text-white rounded-lg text-xs transition disabled:opacity-30 flex items-center gap-1.5">
                  {publishLoading ? <><Loader2 className="w-3 h-3 animate-spin" /> 发布中...</> : <><ArrowRight className="w-3 h-3" /> 上架到市场</>}
                </button>
              </div>
            </div>
          </div>
        )}

        {/* Missions */}
        <div>
          <h3 className="text-sm font-semibold text-gray-300 mb-3">任务列表</h3>
          {missions.length === 0 ? (
            <div className="bg-gray-800/30 border border-gray-700/30 rounded-xl p-8 text-center">
              <Target className="w-8 h-8 text-gray-600 mx-auto mb-2" />
              <div className="text-sm text-gray-500">暂无任务，点击「新建任务」开始</div>
            </div>
          ) : (
            <div className="space-y-2">
              {missions.map(m => (
                <div key={m.id} className="bg-gray-800/50 border border-gray-700/50 rounded-lg p-4 flex items-center gap-4">
                  <Target className="w-5 h-5 text-overlord-400 shrink-0" />
                  <div className="flex-1 min-w-0">
                    <div className="text-sm font-medium text-white truncate">{m.title}</div>
                    <div className="text-xs text-gray-500 mt-0.5">
                      {missionStatusLabels[m.status] || m.status}
                      {m.total_steps > 0 && ` · ${m.done_steps}/${m.total_steps} 步骤`}
                      {m.review_score > 0 && ` · 评分 ${m.review_score.toFixed(1)}`}
                      {m.energy_used > 0 && ` · ${m.energy_used}⚡`}
                    </div>
                  </div>
                  {m.preview_url && (
                    <a href={m.preview_url} target="_blank" rel="noopener noreferrer" className="text-xs text-overlord-400 hover:text-overlord-300">预览</a>
                  )}
                </div>
              ))}
            </div>
          )}
        </div>

        {/* New Mission Modal */}
        {showMission && (
          <div className="fixed inset-0 bg-black/60 flex items-center justify-center z-50" onClick={() => setShowMission(false)}>
            <div className="bg-gray-800 border border-gray-700 rounded-xl w-full max-w-lg p-6" onClick={e => e.stopPropagation()}>
              <h3 className="text-lg font-bold text-white mb-4">新建任务</h3>
              <textarea
                className="w-full bg-gray-900 border border-gray-700 rounded-lg p-3 text-sm text-white placeholder-gray-500 focus:border-overlord-500 focus:outline-none"
                rows={4}
                placeholder="描述你希望 AI 团队完成的任务..."
                value={missionGoal}
                onChange={e => setMissionGoal(e.target.value)}
              />
              <div className="flex justify-end gap-3 mt-4">
                <button onClick={() => setShowMission(false)} className="px-4 py-2 text-sm text-gray-400 hover:text-white transition">取消</button>
                <button onClick={handleCreateMission} disabled={creatingMission || !missionGoal.trim()} className="px-4 py-2 bg-overlord-600 hover:bg-overlord-500 text-white rounded-lg text-sm transition disabled:opacity-50">
                  {creatingMission ? '创建中...' : '创建任务'}
                </button>
              </div>
            </div>
          </div>
        )}
      </div>
    )
  }

  // ─── Overview ───
  return (
    <div className="p-6 space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-xl font-bold text-white flex items-center gap-2">
            <Bot className="w-5 h-5 text-overlord-400" />
            团队智能体
          </h1>
          <p className="text-sm text-gray-500 mt-1">给你的企业雇一支永远在线的 AI 团队</p>
        </div>
      </div>

      {/* Stats cards */}
      {stats && (
        <div className="grid grid-cols-5 gap-4">
          {[
            { label: '模板数', value: stats.template_count, color: 'text-overlord-400' },
            { label: '总实例', value: stats.total_instances, color: 'text-blue-400' },
            { label: '活跃实例', value: stats.active_instances, color: 'text-green-400' },
            { label: '总任务', value: stats.total_missions, color: 'text-purple-400' },
            { label: '总星能', value: `${stats.total_energy}⚡`, color: 'text-yellow-400' },
          ].map(({ label, value, color }) => (
            <div key={label} className="bg-gray-800/50 border border-gray-700/50 rounded-xl p-4 text-center">
              <div className={`text-2xl font-bold ${color}`}>{value}</div>
              <div className="text-xs text-gray-500 mt-1">{label}</div>
            </div>
          ))}
        </div>
      )}

      {/* Templates */}
      <div>
        <h2 className="text-sm font-semibold text-gray-300 mb-3">官方模板</h2>
        <div className="grid grid-cols-3 gap-4">
          {templates.map(t => {
            const Icon = categoryIcons[t.category] || Bot
            const roles = parseRoles(t.roles)
            return (
              <div key={t.id} className="bg-gray-800/50 border border-gray-700/50 rounded-xl p-5 hover:border-overlord-600/50 transition group">
                <div className="flex items-start gap-3">
                  <div className="w-10 h-10 rounded-lg bg-overlord-600/15 flex items-center justify-center shrink-0">
                    <Icon className="w-5 h-5 text-overlord-400" />
                  </div>
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2">
                      <span className="text-sm font-bold text-white">{t.name}</span>
                      {t.is_official && <span className="text-[10px] bg-overlord-600/20 text-overlord-400 px-1.5 py-0.5 rounded">官方</span>}
                      <span className="text-[10px] text-gray-600">{t.version}</span>
                    </div>
                    <div className="text-xs text-gray-400 mt-1 line-clamp-2">{t.description}</div>
                    <div className="flex flex-wrap gap-1 mt-2">
                      {roles.map(r => (
                        <span key={r.code} className="text-[10px] bg-gray-700/50 text-gray-400 px-1.5 py-0.5 rounded">
                          {r.name}{r.max_instances > 1 ? `×${r.max_instances}` : ''}
                        </span>
                      ))}
                    </div>
                  </div>
                  <button
                    onClick={() => openCreate(t.id)}
                    className="opacity-0 group-hover:opacity-100 transition px-3 py-1.5 bg-overlord-600 hover:bg-overlord-500 text-white rounded-lg text-xs flex items-center gap-1 shrink-0"
                  >
                    <Plus className="w-3 h-3" /> 创建
                  </button>
                </div>
              </div>
            )
          })}
        </div>
      </div>

      {/* Instances */}
      <div>
        <h2 className="text-sm font-semibold text-gray-300 mb-3">团队实例 ({instances.length})</h2>
        {instances.length === 0 ? (
          <div className="bg-gray-800/30 border border-gray-700/30 rounded-xl p-12 text-center">
            <Bot className="w-10 h-10 text-gray-600 mx-auto mb-3" />
            <div className="text-sm text-gray-500">暂无团队实例</div>
            <div className="text-xs text-gray-600 mt-1">选择上方模板创建你的第一支 AI 团队</div>
          </div>
        ) : (
          <div className="space-y-2">
            {instances.map(inst => (
              <div
                key={inst.id}
                onClick={() => selectInstance(inst)}
                className="bg-gray-800/50 border border-gray-700/50 rounded-xl p-4 hover:border-overlord-600/40 transition cursor-pointer flex items-center gap-4"
              >
                <div className="w-10 h-10 rounded-lg bg-overlord-600/15 flex items-center justify-center shrink-0">
                  <Bot className="w-5 h-5 text-overlord-400" />
                </div>
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-2">
                    <span className="text-sm font-medium text-white">{inst.name}</span>
                    <span className={`px-1.5 py-0.5 rounded text-[10px] font-medium ${statusColors[inst.status] || 'bg-gray-600 text-gray-300'}`}>
                      {statusLabels[inst.status] || inst.status}
                    </span>
                    <span className="text-[10px] text-gray-600">{inst.template_name}</span>
                  </div>
                  <div className="text-xs text-gray-500 mt-0.5 truncate">
                    {inst.goal || '无目标描述'}
                    {inst.mission_count > 0 && ` · ${inst.mission_count} 个任务`}
                    {inst.energy_used > 0 && ` · ${inst.energy_used}⚡`}
                  </div>
                </div>
                <ChevronRight className="w-4 h-4 text-gray-600" />
              </div>
            ))}
          </div>
        )}
      </div>

      {/* Employee Usage */}
      {employeeUsage.length > 0 && (
        <div>
          <h2 className="text-sm font-semibold text-gray-300 mb-3 flex items-center gap-2">
            <Users className="w-4 h-4" /> 员工用量
          </h2>
          <div className="bg-gray-800/50 border border-gray-700/50 rounded-xl overflow-hidden">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-gray-700/50 text-gray-500 text-xs">
                  <th className="text-left px-4 py-2.5 font-medium">员工</th>
                  <th className="text-right px-4 py-2.5 font-medium">对话次数</th>
                  <th className="text-right px-4 py-2.5 font-medium">输入 Tokens</th>
                  <th className="text-right px-4 py-2.5 font-medium">输出 Tokens</th>
                  <th className="text-right px-4 py-2.5 font-medium">总 Tokens</th>
                </tr>
              </thead>
              <tbody>
                {employeeUsage.map(u => (
                  <tr key={u.user_id} className="border-b border-gray-700/30 hover:bg-gray-700/20 transition">
                    <td className="px-4 py-2.5 text-white font-medium">{u.username || u.user_id}</td>
                    <td className="px-4 py-2.5 text-right text-gray-400 tabular-nums">{u.message_count.toLocaleString()}</td>
                    <td className="px-4 py-2.5 text-right text-gray-400 tabular-nums">{u.input_tokens.toLocaleString()}</td>
                    <td className="px-4 py-2.5 text-right text-gray-400 tabular-nums">{u.output_tokens.toLocaleString()}</td>
                    <td className="px-4 py-2.5 text-right text-white font-medium tabular-nums">{u.total_tokens.toLocaleString()}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {/* Create Modal — Team Lobby */}
      {showCreate && (() => {
        const tmpl = templates.find(t => t.id === createTmplId)
        const roles = tmpl ? parseRoles(tmpl.roles) : []
        const TmplIcon = tmpl ? (categoryIcons[tmpl.category] || Bot) : Bot
        const allAssigned = roles.every(r => roleNodeMap[r.code])

        return (
          <div className="fixed inset-0 bg-black/70 flex items-center justify-center z-50 p-4" onClick={() => setShowCreate(false)}>
            <div className="bg-gray-800 border border-gray-700 rounded-2xl w-full max-w-4xl max-h-[90vh] overflow-hidden flex flex-col" onClick={e => e.stopPropagation()}>
              {/* Lobby Header */}
              <div className="bg-gray-900/80 border-b border-gray-700 px-6 py-4 flex items-center gap-4">
                <div className="w-10 h-10 rounded-xl bg-overlord-600/20 flex items-center justify-center">
                  <TmplIcon className="w-5 h-5 text-overlord-400" />
                </div>
                <div className="flex-1">
                  <div className="flex items-center gap-2">
                    <h3 className="text-lg font-bold text-white">组建团队智能体</h3>
                    {tmpl && <span className="text-xs bg-overlord-600/20 text-overlord-400 px-2 py-0.5 rounded">{tmpl.name}</span>}
                  </div>
                  <p className="text-xs text-gray-500 mt-0.5">{tmpl?.description}</p>
                </div>
                {/* Step indicator */}
                <div className="flex items-center gap-2">
                  <div className={`w-8 h-8 rounded-full flex items-center justify-center text-xs font-bold ${createStep === 1 ? 'bg-overlord-600 text-white' : 'bg-gray-700 text-gray-400'}`}>1</div>
                  <div className="w-6 h-px bg-gray-700" />
                  <div className={`w-8 h-8 rounded-full flex items-center justify-center text-xs font-bold ${createStep === 2 ? 'bg-overlord-600 text-white' : 'bg-gray-700 text-gray-400'}`}>2</div>
                </div>
              </div>

              {/* Lobby Body */}
              <div className="flex-1 overflow-y-auto p-6">
                {createStep === 1 ? (
                  /* Step 1: 选将配阵 */
                  <div>
                    <div className="flex items-center gap-2 mb-4">
                      <Users className="w-4 h-4 text-overlord-400" />
                      <h4 className="text-sm font-semibold text-white">选将配阵</h4>
                      <span className="text-xs text-gray-500">为每个角色分配 Claw 节点</span>
                    </div>
                    {/* Deploy Spark node — shown when no nodes or as option */}
                    {nodes.length === 0 && !provisioning && (
                      <div className="mb-4 bg-orange-500/5 border border-orange-500/20 rounded-xl p-4 flex items-center gap-4">
                        <div className="w-10 h-10 rounded-lg bg-orange-500/10 flex items-center justify-center shrink-0 text-xl">🔥</div>
                        <div className="flex-1">
                          <div className="text-sm font-medium text-white">没有可用节点</div>
                          <div className="text-xs text-gray-400 mt-0.5">一键部署免费 Spark 节点（3 秒就绪，永久免费）</div>
                        </div>
                        <button onClick={handleProvision} className="px-4 py-2 bg-orange-600 hover:bg-orange-500 text-white rounded-lg text-sm transition flex items-center gap-2 shrink-0">
                          <Zap className="w-4 h-4" /> 极速部署
                        </button>
                      </div>
                    )}
                    {provisioning && (
                      <div className="mb-4 bg-purple-500/5 border border-purple-500/20 rounded-xl p-4 flex items-center gap-4">
                        <Loader2 className="w-6 h-6 text-purple-400 animate-spin shrink-0" />
                        <div className="flex-1">
                          <div className="text-sm font-medium text-white">正在部署 Spark 节点...</div>
                          <div className="text-xs text-gray-400 mt-0.5">{provisionSlug ? `${provisionSlug}.starclaw.me` : 'Spore + SQLite 极速启动中'}</div>
                          <div className="mt-2 h-1.5 bg-gray-700 rounded-full overflow-hidden">
                            <div className="h-full bg-purple-500 rounded-full animate-pulse" style={{ width: '60%' }} />
                          </div>
                        </div>
                      </div>
                    )}
                    {nodes.length > 0 && !provisioning && (
                      <div className="mb-4 flex items-center justify-between">
                        <div />
                        <button onClick={handleProvision} className="text-xs text-orange-400 hover:text-orange-300 transition flex items-center gap-1">
                          <Zap className="w-3 h-3" /> 部署新的 Spark 节点
                        </button>
                      </div>
                    )}

                    <div className="grid grid-cols-2 gap-3">
                      {roles.map((role, idx) => {
                        const assigned = roleNodeMap[role.code]
                        const assignedNode = nodes.find(n => n.id === assigned)
                        return (
                          <div key={role.code} className={`bg-gray-900/60 border rounded-xl p-4 transition ${
                            assigned ? 'border-overlord-600/50 shadow-lg shadow-overlord-600/5' : 'border-gray-700/50'
                          }`}>
                            <div className="flex items-start gap-3">
                              {/* Role avatar */}
                              <div className="w-12 h-12 rounded-xl bg-gray-800 border border-gray-700 flex items-center justify-center text-xl shrink-0">
                                {roleIcons[role.code] || '🤖'}
                              </div>
                              <div className="flex-1 min-w-0">
                                <div className="flex items-center gap-2">
                                  <span className="text-sm font-bold text-white">{role.name}</span>
                                  {role.max_instances > 1 && (
                                    <span className="text-[10px] bg-yellow-500/10 text-yellow-400 px-1.5 py-0.5 rounded">×{role.max_instances}</span>
                                  )}
                                  <span className="text-[10px] text-gray-600">#{idx + 1}</span>
                                </div>
                                {/* Model + Tools */}
                                <div className="flex items-center gap-2 mt-1.5">
                                  {role.model && (
                                    <span className={`text-[10px] px-1.5 py-0.5 rounded flex items-center gap-1 ${modelColors[role.model] || 'text-gray-400 bg-gray-700/50'}`}>
                                      <Cpu className="w-2.5 h-2.5" />{role.model}
                                    </span>
                                  )}
                                  {role.tools && role.tools.length > 0 && (
                                    <span className="text-[10px] text-gray-500 flex items-center gap-1">
                                      <Wrench className="w-2.5 h-2.5" />{role.tools.length} 工具
                                    </span>
                                  )}
                                </div>
                                {/* System prompt snippet */}
                                {role.system_prompt && (
                                  <div className="text-[10px] text-gray-600 mt-1 line-clamp-1">{role.system_prompt.slice(0, 60)}...</div>
                                )}
                              </div>
                            </div>
                            {/* Node selector */}
                            <div className="mt-3 flex items-center gap-2">
                              <Server className="w-3.5 h-3.5 text-gray-500" />
                              <select
                                className="flex-1 bg-gray-800 border border-gray-700 rounded-lg px-2 py-1.5 text-xs text-white focus:border-overlord-500 focus:outline-none"
                                value={assigned || ''}
                                onChange={e => assignNode(role.code, e.target.value)}
                              >
                                <option value="">选择节点...</option>
                                {nodes.map(n => (
                                  <option key={n.id} value={n.id}>{n.name || n.claw_id?.slice(0, 12)} ({n.status})</option>
                                ))}
                              </select>
                              {assigned && (
                                <div className="w-2 h-2 rounded-full bg-green-500 animate-pulse" title="已分配" />
                              )}
                            </div>
                          </div>
                        )
                      })}
                    </div>
                    {/* Topology hint */}
                    {tmpl?.topology && (() => {
                      try {
                        const topo = JSON.parse(tmpl.topology)
                        if (topo.flow?.length) {
                          return (
                            <div className="mt-4 bg-gray-900/40 border border-gray-700/30 rounded-lg p-3">
                              <div className="text-[10px] text-gray-500 mb-2">协作拓扑</div>
                              <div className="flex items-center gap-1 flex-wrap">
                                {topo.flow.map((f: { from: string; to: string; type: string }, i: number) => {
                                  const fromRole = roles.find(r => r.code === f.from)
                                  const toRole = roles.find(r => r.code === f.to)
                                  return (
                                    <div key={i} className="flex items-center gap-1">
                                      <span className="text-[10px] text-overlord-400">{fromRole?.name || f.from}</span>
                                      <ArrowRight className="w-3 h-3 text-gray-600" />
                                      <span className="text-[10px] text-overlord-400">{toRole?.name || f.to}</span>
                                      {i < topo.flow.length - 1 && <span className="text-gray-700 mx-1">·</span>}
                                    </div>
                                  )
                                })}
                              </div>
                            </div>
                          )
                        }
                      } catch { /* ignore */ }
                      return null
                    })()}
                  </div>
                ) : (
                  /* Step 2: 团队设置 */
                  <div className="max-w-lg mx-auto space-y-4">
                    <div className="flex items-center gap-2 mb-4">
                      <Bot className="w-4 h-4 text-overlord-400" />
                      <h4 className="text-sm font-semibold text-white">团队设置</h4>
                    </div>
                    <div>
                      <label className="text-xs text-gray-400 mb-1 block">团队名称 *</label>
                      <input className="w-full bg-gray-900 border border-gray-700 rounded-lg px-3 py-2 text-sm text-white placeholder-gray-500 focus:border-overlord-500 focus:outline-none" placeholder={`如: ${tmpl?.name || 'My'}-电商项目`} value={createName} onChange={e => setCreateName(e.target.value)} />
                    </div>
                    <div>
                      <label className="text-xs text-gray-400 mb-1 block">目标描述</label>
                      <textarea className="w-full bg-gray-900 border border-gray-700 rounded-lg px-3 py-2 text-sm text-white placeholder-gray-500 focus:border-overlord-500 focus:outline-none" rows={3} placeholder="描述这支 AI 团队要做什么..." value={createGoal} onChange={e => setCreateGoal(e.target.value)} />
                    </div>
                    <div>
                      <label className="text-xs text-gray-400 mb-1 block">星能预算</label>
                      <div className="flex items-center gap-2">
                        <input type="number" className="w-40 bg-gray-900 border border-gray-700 rounded-lg px-3 py-2 text-sm text-white focus:border-overlord-500 focus:outline-none" value={createBudget} onChange={e => setCreateBudget(Number(e.target.value))} min={0} step={1000} />
                        <Zap className="w-4 h-4 text-yellow-400" />
                        <span className="text-xs text-gray-500">0 = 不限</span>
                      </div>
                    </div>
                    {/* Role summary */}
                    <div className="bg-gray-900/40 border border-gray-700/30 rounded-lg p-3">
                      <div className="text-xs text-gray-500 mb-2">阵容确认</div>
                      <div className="flex flex-wrap gap-2">
                        {roles.map(r => {
                          const node = nodes.find(n => n.id === roleNodeMap[r.code])
                          return (
                            <div key={r.code} className="flex items-center gap-1.5 bg-gray-800/60 rounded-lg px-2 py-1">
                              <span className="text-sm">{roleIcons[r.code] || '🤖'}</span>
                              <span className="text-xs text-white">{r.name}</span>
                              <span className="text-[10px] text-gray-500">→</span>
                              <span className="text-[10px] text-overlord-400">{node?.name || '未分配'}</span>
                            </div>
                          )
                        })}
                      </div>
                    </div>
                  </div>
                )}
              </div>

              {/* Lobby Footer */}
              <div className="bg-gray-900/60 border-t border-gray-700 px-6 py-4 flex items-center justify-between">
                <div className="text-xs text-gray-500">
                  {roles.length} 个角色 · {Object.values(roleNodeMap).filter(Boolean).length} 已分配
                </div>
                <div className="flex items-center gap-3">
                  <button onClick={() => setShowCreate(false)} className="px-4 py-2 text-sm text-gray-400 hover:text-white transition">取消</button>
                  {createStep === 1 ? (
                    <button
                      onClick={() => setCreateStep(2)}
                      disabled={!allAssigned}
                      className="px-5 py-2 bg-overlord-600 hover:bg-overlord-500 text-white rounded-lg text-sm transition disabled:opacity-50 flex items-center gap-2"
                    >
                      下一步 <ArrowRight className="w-4 h-4" />
                    </button>
                  ) : (
                    <>
                      <button onClick={() => setCreateStep(1)} className="px-4 py-2 text-sm text-gray-400 hover:text-white transition">上一步</button>
                      <button
                        onClick={handleCreate}
                        disabled={creating || !createName}
                        className="px-5 py-2 bg-overlord-600 hover:bg-overlord-500 text-white rounded-lg text-sm transition disabled:opacity-50 flex items-center gap-2"
                      >
                        {creating ? <Loader2 className="w-4 h-4 animate-spin" /> : <Bot className="w-4 h-4" />}
                        {creating ? '组建中...' : '组建团队'}
                      </button>
                    </>
                  )}
                </div>
              </div>
            </div>
          </div>
        )
      })()}
    </div>
  )
}
