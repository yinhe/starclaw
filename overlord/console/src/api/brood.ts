const BASE = '/brood'
const TOKEN_KEY = 'overlord_token'
const USER_KEY = 'overlord_user'

export function getStoredToken(): string | null {
  return localStorage.getItem(TOKEN_KEY)
}

export function getStoredUser(): { username: string; role: string } | null {
  try { return JSON.parse(localStorage.getItem(USER_KEY) || 'null') } catch { return null }
}

export function storeAuth(token: string, user: { username: string; role: string }) {
  localStorage.setItem(TOKEN_KEY, token)
  localStorage.setItem(USER_KEY, JSON.stringify(user))
  // Sync to web app keys so /app/ auto-authenticates
  localStorage.setItem('web_token', token)
  localStorage.setItem('web_user', JSON.stringify(user))
}

export function clearAuth() {
  localStorage.removeItem(TOKEN_KEY)
  localStorage.removeItem(USER_KEY)
  localStorage.removeItem('web_token')
  localStorage.removeItem('web_user')
}

async function request<T = unknown>(path: string, opts?: RequestInit): Promise<T> {
  const token = getStoredToken()
  const headers: Record<string, string> = { 'Content-Type': 'application/json' }
  if (token) headers['X-Admin-Token'] = token
  if (opts?.headers) Object.assign(headers, opts.headers)

  const res = await fetch(`${BASE}${path}`, { ...opts, headers })
  if (res.status === 401) {
    clearAuth()
    window.location.reload()
    throw new Error('Unauthorized')
  }
  if (!res.ok) throw new Error(`${res.status} ${res.statusText}`)
  return res.json()
}

export async function login(username: string, password: string) {
  const res = await fetch(`${BASE}/auth/login`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ username, password }),
  })
  if (!res.ok) {
    const data = await res.json().catch(() => ({}))
    throw new Error(data.error || 'Login failed')
  }
  const data = await res.json()
  storeAuth(data.token, { username: data.user?.username || username, role: data.user?.role || 'viewer' })
  return data
}

export interface ClawNode {
  id: string
  name: string
  address: string
  version: string
  claw_id: string
  status: 'online' | 'feral' | 'offline'
  team: string
  tags: string
  max_concurrent: number
  max_tokens_day: number
  cpu_percent: number
  mem_percent: number
  tasks_running: number
  tasks_queued: number
  tokens_today: number
  error_rate: number
  avg_latency_ms: number
  last_heartbeat: string
  registered_at: string
}

export interface BroodStats {
  total: number
  online: number
  feral: number
  offline: number
  avg_cpu: number
  avg_mem: number
  total_tasks: number
  total_tokens: number
  teams: { team: string; count: number }[]
}

export interface AuditLogEntry {
  id: number
  actor: string
  action: string
  target_id: string
  detail: string
  created_at: string
}

// --- Admin Users ---
export interface AdminUser {
  id: string
  username: string
  role: string
  team_id: string
  email: string
  created_at: string
}

// --- Teams ---
export interface Team {
  id: string
  name: string
  display_name: string
  max_nodes: number
  max_tokens: number
  status: string
  created_at: string
  node_count?: number
}

// --- Nydus Tunnels ---
export interface NydusTunnel {
  id: string
  claw_node_id: string
  claw_name: string
  team: string
  local_port: number
  remote_port: number
  protocol: string
  mode: string
  status: string
  bytes_in: number
  bytes_out: number
  connections: number
  last_error: string
  created_at: string
}

// --- Molt Releases ---
export interface MoltRelease {
  id: string
  version: string
  channel: string
  title: string
  release_notes: string
  download_url: string
  checksum: string
  status: string
  submitted_by: string
  reviewed_by: string
  reviewed_at: string | null
  review_note: string
  target_team: string
  rollout_pct: number
  max_failures: number
  total_nodes: number
  updated_nodes: number
  failed_nodes: number
  created_at: string
}

export interface MoltNodeStatus {
  id: number
  release_id: string
  claw_node_id: string
  claw_name: string
  old_version: string
  status: string
  error_detail: string
  started_at: string | null
  completed_at: string | null
}

// --- Webhooks ---
export interface Webhook {
  id: string
  name: string
  url: string
  team_id: string
  events: string
  status: string
  total_sent: number
  total_failed: number
  last_sent_at: string | null
  last_error: string
  created_at: string
}

export interface WebhookLog {
  id: number
  webhook_id: string
  event: string
  payload: string
  status_code: number
  error: string
  duration_ms: number
  created_at: string
}

// --- Billing ---
export interface Plan {
  id: string
  name: string
  display_name: string
  price_monthly: number
  price_yearly: number
  max_nodes: number
  max_teams: number
  max_tokens_day: number
  features: string
  sort_order: number
  active: boolean
}

export interface Subscription {
  id: string
  team_id: string
  plan_id: string
  plan_name: string
  status: string
  billing_cycle: string
  current_period_start: string
  current_period_end: string
  cancelled_at: string | null
  created_at: string
}

export interface UsageDailySummary {
  id: number
  team_id: string
  date: string
  total_requests: number
  total_tokens: number
  input_tokens: number
  output_tokens: number
  total_cost_cents: number
  total_star_energy: number
  unique_users: number
  unique_models: number
  avg_latency_ms: number
}

export interface BillingOverview {
  subscription: Subscription
  plan: Plan
  month_usage: { total_requests: number; total_tokens: number; total_cost_cents: number }
  today_usage: { total_requests: number; total_tokens: number; total_cost_cents: number }
  active_alerts: number
}

export interface ModelUsage {
  model_name: string
  total_requests: number
  total_tokens: number
  total_cost_cents: number
  avg_latency_ms: number
}

export interface UserUsage {
  user_id: string
  total_requests: number
  total_tokens: number
  total_cost_cents: number
}

export interface BudgetAlert {
  id: string
  team_id: string
  name: string
  metric_type: string
  threshold_value: number
  period: string
  notify_email: string
  notify_webhook: string
  enabled: boolean
  last_triggered: string | null
  created_at: string
}

export const broodAPI = {
  stats: () => request<BroodStats>('/stats'),

  listClaws: (params?: { team?: string; status?: string }) => {
    const q = new URLSearchParams()
    if (params?.team) q.set('team', params.team)
    if (params?.status) q.set('status', params.status)
    const qs = q.toString()
    return request<{ claws: ClawNode[]; total: number }>(`/claws${qs ? '?' + qs : ''}`)
  },

  getClaw: (id: string) => request<{ claw: ClawNode }>(`/claws/${id}`),

  registerClaw: (data: { name: string; address: string; team?: string }) =>
    request<{ node_id: string; token: string }>('/register', { method: 'POST', body: JSON.stringify(data) }),

  updateQuota: (id: string, data: { max_concurrent: number; max_tokens_day: number }) =>
    request(`/claws/${id}/quota`, { method: 'PUT', body: JSON.stringify(data) }),

  removeClaw: (id: string) =>
    request(`/claws/${id}`, { method: 'DELETE' }),

  assignTask: (taskId: string, team?: string) =>
    request<{ claw_id: string; claw_name: string; address: string; task_id: string }>(
      '/task/assign',
      { method: 'POST', body: JSON.stringify({ task_id: taskId, team }) },
    ),

  auditLogs: () => request<{ logs: AuditLogEntry[]; total: number }>('/audit'),

  resolve: (clawId: string) =>
    request<{ found: boolean; address?: string; name?: string; claw_id?: string; version?: string; status?: string; team?: string }>(
      `/resolve?claw_id=${encodeURIComponent(clawId)}`,
    ),

  // --- Admin Users ---
  listAdmins: () => request<{ users: AdminUser[]; total: number }>('/admins'),
  createAdmin: (data: { username: string; password: string; role?: string; team_id?: string; email?: string }) =>
    request<{ user: AdminUser }>('/admins', { method: 'POST', body: JSON.stringify(data) }),
  deleteAdmin: (id: string) => request(`/admins/${id}`, { method: 'DELETE' }),

  // --- Teams ---
  listTeams: () => request<{ teams: (Team & { node_count: number })[]; total: number }>('/teams'),
  getTeam: (id: string) => request<{ team: Team; node_count: number }>(`/teams/${id}`),
  createTeam: (data: { name: string; display_name?: string; max_nodes?: number; max_tokens?: number }) =>
    request<{ team: Team }>('/teams', { method: 'POST', body: JSON.stringify(data) }),
  updateTeam: (id: string, data: Record<string, unknown>) =>
    request(`/teams/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
  deleteTeam: (id: string) => request(`/teams/${id}`, { method: 'DELETE' }),

  // --- Nydus Tunnels ---
  listTunnels: (params?: { status?: string; claw_node_id?: string }) => {
    const q = new URLSearchParams()
    if (params?.status) q.set('status', params.status)
    if (params?.claw_node_id) q.set('claw_node_id', params.claw_node_id)
    const qs = q.toString()
    return request<{ tunnels: NydusTunnel[]; total: number }>(`/tunnels${qs ? '?' + qs : ''}`)
  },
  getTunnel: (id: string) => request<{ tunnel: NydusTunnel }>(`/tunnels/${id}`),
  createTunnel: (data: { claw_node_id: string; local_port: number; remote_port: number; protocol?: string; mode?: string }) =>
    request<{ tunnel: NydusTunnel }>('/tunnels', { method: 'POST', body: JSON.stringify(data) }),
  updateTunnelStatus: (id: string, data: { status: string; last_error?: string }) =>
    request(`/tunnels/${id}/status`, { method: 'PUT', body: JSON.stringify(data) }),
  deleteTunnel: (id: string) => request(`/tunnels/${id}`, { method: 'DELETE' }),

  // --- Molt Releases ---
  listReleases: (params?: { status?: string; channel?: string }) => {
    const q = new URLSearchParams()
    if (params?.status) q.set('status', params.status)
    if (params?.channel) q.set('channel', params.channel)
    const qs = q.toString()
    return request<{ releases: MoltRelease[]; total: number }>(`/molt/releases${qs ? '?' + qs : ''}`)
  },
  getRelease: (id: string) => request<{ release: MoltRelease; node_statuses: MoltNodeStatus[] }>(`/molt/releases/${id}`),
  createRelease: (data: Partial<MoltRelease>) =>
    request<{ release: MoltRelease }>('/molt/releases', { method: 'POST', body: JSON.stringify(data) }),
  reviewRelease: (id: string, data: { action: 'approve' | 'reject'; note?: string }) =>
    request(`/molt/releases/${id}/review`, { method: 'POST', body: JSON.stringify(data) }),
  startRollout: (id: string) =>
    request<{ message: string; total_nodes: number }>(`/molt/releases/${id}/rollout`, { method: 'POST' }),

  // --- Webhooks ---
  listWebhooks: () => request<{ webhooks: Webhook[]; total: number }>('/webhooks'),
  getWebhook: (id: string) => request<{ webhook: Webhook; recent_logs: WebhookLog[] }>(`/webhooks/${id}`),
  createWebhook: (data: { name: string; url: string; events?: string; team_id?: string }) =>
    request<{ webhook: Webhook; secret: string }>('/webhooks', { method: 'POST', body: JSON.stringify(data) }),
  updateWebhook: (id: string, data: Record<string, unknown>) =>
    request(`/webhooks/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
  deleteWebhook: (id: string) => request(`/webhooks/${id}`, { method: 'DELETE' }),
  testWebhook: (id: string) => request<{ success: boolean; status_code: number; error?: string }>(`/webhooks/${id}/test`, { method: 'POST' }),

  // --- Billing ---
  billingOverview: (teamId?: string) => {
    const q = teamId ? `?team_id=${teamId}` : ''
    return request<BillingOverview>(`/billing/overview${q}`)
  },
  listPlans: () => request<Plan[]>('/billing/plans?active_only=true'),
  listSubscriptions: (params?: { team_id?: string; status?: string }) => {
    const q = new URLSearchParams()
    if (params?.team_id) q.set('team_id', params.team_id)
    if (params?.status) q.set('status', params.status)
    const qs = q.toString()
    return request<Subscription[]>(`/billing/subscriptions${qs ? '?' + qs : ''}`)
  },
  createSubscription: (data: { team_id?: string; plan_id: string; billing_cycle?: string }) =>
    request<Subscription>('/billing/subscriptions', { method: 'POST', body: JSON.stringify(data) }),
  cancelSubscription: (id: string) =>
    request<Subscription>(`/billing/subscriptions/${id}/cancel`, { method: 'POST' }),

  // --- Usage Analytics ---
  usageStats: (params?: { team_id?: string; from?: string; to?: string }) => {
    const q = new URLSearchParams()
    if (params?.team_id) q.set('team_id', params.team_id)
    if (params?.from) q.set('from', params.from)
    if (params?.to) q.set('to', params.to)
    return request<{ from: string; to: string; totals: { total_requests: number; total_tokens: number; input_tokens: number; output_tokens: number; total_cost_cents: number; total_star_energy: number }; daily: UsageDailySummary[] }>(`/billing/usage/stats?${q}`)
  },
  usageByModel: (params?: { team_id?: string; from?: string; to?: string }) => {
    const q = new URLSearchParams()
    if (params?.team_id) q.set('team_id', params.team_id)
    if (params?.from) q.set('from', params.from)
    if (params?.to) q.set('to', params.to)
    return request<{ models: ModelUsage[] }>(`/billing/usage/by-model?${q}`)
  },
  usageByUser: (params?: { team_id?: string; from?: string; to?: string; limit?: number }) => {
    const q = new URLSearchParams()
    if (params?.team_id) q.set('team_id', params.team_id)
    if (params?.from) q.set('from', params.from)
    if (params?.to) q.set('to', params.to)
    if (params?.limit) q.set('limit', String(params.limit))
    return request<{ users: UserUsage[] }>(`/billing/usage/by-user?${q}`)
  },

  // --- Budget Alerts ---
  listAlerts: (teamId?: string) => {
    const q = teamId ? `?team_id=${teamId}` : ''
    return request<BudgetAlert[]>(`/billing/alerts${q}`)
  },
  createAlert: (data: Partial<BudgetAlert>) =>
    request<BudgetAlert>('/billing/alerts', { method: 'POST', body: JSON.stringify(data) }),
  updateAlert: (id: string, data: Partial<BudgetAlert>) =>
    request<BudgetAlert>(`/billing/alerts/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
  deleteAlert: (id: string) =>
    request(`/billing/alerts/${id}`, { method: 'DELETE' }),

  // --- Brand / White-Label ---
  getBrand: () => request<{ brand: BrandConfig }>('/brand'),
  updateBrand: (data: Partial<BrandConfig>) =>
    request<{ brand: BrandConfig }>('/brand/config', { method: 'PUT', body: JSON.stringify(data) }),

  // --- License ---
  getLicense: () => request<{ license: LicenseKeyInfo | null; tier: string; limits: TierLimits }>('/license'),
  activateLicense: (key: string, fingerprint?: string) =>
    request<{ license: LicenseKeyInfo; tier: string; limits: TierLimits; message: string }>(
      '/license/activate', { method: 'POST', body: JSON.stringify({ key, fingerprint }) }),
  createLicense: (data: Partial<LicenseKeyInfo>) =>
    request<{ license: LicenseKeyInfo }>('/license', { method: 'POST', body: JSON.stringify(data) }),
  revokeLicense: (id: string) =>
    request('/license/' + id + '/revoke', { method: 'POST' }),

  // --- Features ---
  listFeatures: () => request<{ features: FeatureWithAccess[]; current_tier: string }>('/features'),
  updateFeature: (id: string, data: { enabled?: boolean; min_tier?: string }) =>
    request<{ feature: FeatureToggle }>(`/features/${id}`, { method: 'PUT', body: JSON.stringify(data) }),

  // --- Compliance ---
  complianceStats: () => request<ComplianceStats>('/compliance/stats'),
  complianceLogs: (params?: { event_type?: string; severity?: string; from?: string; to?: string; resolved?: string; page?: number; size?: number }) => {
    const q = new URLSearchParams()
    if (params?.event_type) q.set('event_type', params.event_type)
    if (params?.severity) q.set('severity', params.severity)
    if (params?.from) q.set('from', params.from)
    if (params?.to) q.set('to', params.to)
    if (params?.resolved) q.set('resolved', params.resolved)
    if (params?.page) q.set('page', String(params.page))
    if (params?.size) q.set('size', String(params.size))
    const qs = q.toString()
    return request<{ logs: ComplianceLogEntry[]; total: number }>(`/compliance/logs${qs ? '?' + qs : ''}`)
  },
  resolveComplianceLog: (id: string) =>
    request(`/compliance/logs/${id}/resolve`, { method: 'POST' }),
  exportComplianceLogs: (from?: string, to?: string) => {
    const q = new URLSearchParams()
    if (from) q.set('from', from)
    if (to) q.set('to', to)
    return request<{ logs: ComplianceLogEntry[]; exported_by: string; exported_at: string }>(`/compliance/export?${q}`)
  },

  // Sensitive words
  listSensitiveWords: (category?: string) => {
    const q = category ? `?category=${category}` : ''
    return request<{ rules: SensitiveWordRule[]; total: number }>(`/compliance/words${q}`)
  },
  createSensitiveWord: (data: Partial<SensitiveWordRule>) =>
    request<{ rule: SensitiveWordRule }>('/compliance/words', { method: 'POST', body: JSON.stringify(data) }),
  updateSensitiveWord: (id: string, data: Partial<SensitiveWordRule>) =>
    request<{ rule: SensitiveWordRule }>(`/compliance/words/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
  deleteSensitiveWord: (id: string) =>
    request(`/compliance/words/${id}`, { method: 'DELETE' }),

  // Data flows
  listDataFlows: () => request<{ flows: DataFlowRecord[]; total: number }>('/compliance/flows'),
  createDataFlow: (data: Partial<DataFlowRecord>) =>
    request<{ flow: DataFlowRecord }>('/compliance/flows', { method: 'POST', body: JSON.stringify(data) }),
  updateDataFlow: (id: string, data: Partial<DataFlowRecord>) =>
    request<{ flow: DataFlowRecord }>(`/compliance/flows/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
  deleteDataFlow: (id: string) =>
    request(`/compliance/flows/${id}`, { method: 'DELETE' }),

  // --- Team Agent ---
  teamAgentStats: () => request<TeamAgentStats>('/team-agent/stats'),
  listTeamTemplates: (category?: string) => {
    const q = category ? `?category=${category}` : ''
    return request<{ templates: TeamAgentTemplate[]; total: number }>(`/team-agent/templates${q}`)
  },
  getTeamTemplate: (id: string) => request<{ template: TeamAgentTemplate }>(`/team-agent/templates/${id}`),
  listTeamInstances: (params?: { status?: string; template_id?: string }) => {
    const q = new URLSearchParams()
    if (params?.status) q.set('status', params.status)
    if (params?.template_id) q.set('template_id', params.template_id)
    const qs = q.toString()
    return request<{ instances: TeamInstance[]; total: number }>(`/team-agent/instances${qs ? '?' + qs : ''}`)
  },
  getTeamInstance: (id: string) => request<{ instance: TeamInstance }>(`/team-agent/instances/${id}`),
  createTeamInstance: (data: { template_id: string; claw_node_id: string; name: string; goal?: string; energy_budget?: number; default_model?: string }) =>
    request<{ instance: TeamInstance }>('/team-agent/instances', { method: 'POST', body: JSON.stringify(data) }),
  disbandTeamInstance: (id: string) =>
    request('/team-agent/instances/' + id + '/disband', { method: 'POST' }),
  getTeamDashboard: (id: string) => request<Record<string, unknown>>(`/team-agent/instances/${id}/dashboard`),
  listTeamMissions: (instanceId: string) =>
    request<{ missions: TeamMission[]; total: number }>(`/team-agent/instances/${instanceId}/missions`),
  createTeamMission: (instanceId: string, data: { goal: string; auto_confirm?: boolean }) =>
    request<{ mission: TeamMission }>(`/team-agent/instances/${instanceId}/missions`, { method: 'POST', body: JSON.stringify(data) }),
  updateInstanceRoles: (id: string, data: { role_overrides: Record<string, { model: string; system_prompt: string; tools: string[] }>; default_model?: string }) =>
    request<{ message: string; config: string }>(`/team-agent/instances/${id}/roles`, { method: 'PUT', body: JSON.stringify(data) }),
  nodeModels: (nodeId: string) =>
    request<{ models: ClawModel[]; total: number; node_name: string }>(`/team-agent/node-models/${nodeId}`),
  nodeSkills: (nodeId: string) =>
    request<{ skills: ClawSkill[]; plugins: ClawPlugin[]; mcp_servers: ClawMCP[]; total: number; node_name: string }>(`/team-agent/node-skills/${nodeId}`),
  nodeAgents: (nodeId: string) =>
    request<{ agents: ClawAgentTemplate[]; categories: { category: string; count: number }[]; total: number; node_name: string }>(`/team-agent/node-agents/${nodeId}`),
  agentSandbox: (instanceId: string, data: AgentSandboxReq) =>
    request<AgentSandboxResp>(`/team-agent/instances/${instanceId}/agent-sandbox`, { method: 'POST', body: JSON.stringify(data) }),
  agentPublish: (instanceId: string, data: AgentPublishReq) =>
    request<AgentPublishResp>(`/team-agent/instances/${instanceId}/agent-publish`, { method: 'POST', body: JSON.stringify(data) }),
  teamAgentUsageByUser: () =>
    request<{ users: EmployeeUsage[]; total: number }>('/team-agent/usage/by-user'),
  teamAgentChatHistory: (instanceId: string) =>
    request<{ messages: ChatMessageRecord[]; total: number }>(`/team-agent/instances/${instanceId}/chat`),

  // --- Provision (one-click Spark node) ---
  provisionNode: (name?: string) =>
    request<ProvisionResult>('/team-agent/provision-node', { method: 'POST', body: JSON.stringify({ name: name || '' }) }),
  provisionStatus: (slug: string) =>
    request<ProvisionResult>(`/team-agent/provision-status?slug=${encodeURIComponent(slug)}`),
}

// --- P4 Types ---

export interface BrandConfig {
  id: string
  brand_name: string
  logo_url: string
  favicon_url: string
  primary_color: string
  secondary_color: string
  bg_color: string
  accent_color: string
  domain: string
  login_title: string
  login_subtitle: string
  copyright_text: string
  icp_number: string
  support_email: string
  custom_css: string
  powered_by: boolean
  enabled: boolean
  created_at: string
  updated_at: string
}

export interface LicenseKeyInfo {
  id: string
  key: string
  tier: string
  holder: string
  email: string
  max_nodes: number
  max_teams: number
  issued_at: string
  expires_at: string | null
  status: string
  fingerprint: string
  created_at: string
}

export interface TierLimits {
  MaxNodes: number
  MaxTeams: number
  SSOEnabled: boolean
  AuditDays: number
  AdvancedUsage: boolean
  Compliance: boolean
  BrandCustom: boolean
  FeatureToggle: boolean
}

export interface FeatureToggle {
  id: string
  key: string
  name: string
  description: string
  category: string
  min_tier: string
  enabled: boolean
  sort_order: number
}

export interface FeatureWithAccess extends FeatureToggle {
  has_access: boolean
}

export interface ComplianceStats {
  total: number
  unresolved: number
  critical: number
  by_type: { event_type: string; count: number }[]
  by_severity: { severity: string; count: number }[]
  daily_7d: { date: string; count: number }[]
}

export interface ComplianceLogEntry {
  id: string
  team_id: string
  actor: string
  event_type: string
  severity: string
  resource: string
  detail: string
  ip_address: string
  resolved: boolean
  created_at: string
}

export interface SensitiveWordRule {
  id: string
  word: string
  category: string
  action: string
  enabled: boolean
  created_at: string
}

export interface DataFlowRecord {
  id: string
  source: string
  destination: string
  data_type: string
  encryption: string
  region: string
  cross_border: boolean
  description: string
}

// --- Team Agent ---
export interface TeamAgentTemplate {
  id: string
  name: string
  category: string
  description: string
  icon: string
  roles: string       // JSON
  topology: string    // JSON
  quality_gate: string
  escalation: string
  is_official: boolean
  version: string
  created_at: string
}

export interface TeamInstance {
  id: string
  template_id: string
  template_name: string
  team_id: string
  claw_node_id: string
  user_id: string
  name: string
  goal: string
  status: string
  published: boolean
  welcome_msg: string
  default_model: string
  role_map: string
  config: string
  energy_budget: number
  energy_used: number
  mission_count: number
  avg_score: number
  created_at: string
  updated_at: string
  disbanded_at: string | null
}

export interface ClawSkill {
  name: string
  description: string
  type: string  // builtin, plugin, mcp
  status: string
}

export interface ClawPlugin {
  id: string
  name: string
  display_name: string
  description: string
  category: string
  icon: string
  version: string
  pricing: string
}

export interface ClawMCP {
  name: string
  base_url: string
}

export interface ClawAgentTemplate {
  id: string
  name: string
  description: string
  category: string
  system_prompt: string
  model: string
  tools: string  // JSON array
  icon: string
  rating: number
  install_count: number
  is_official: boolean
}

export interface AgentSandboxReq {
  name: string
  system_prompt: string
  model?: string
  tools?: string
  config?: string
  test_messages: { role: string; content: string }[]
}

export interface AgentSandboxResult {
  input: string
  output: string
  verdict: string
  checks: Record<string, boolean>
  error?: string
}

export interface AgentSandboxResp {
  results: AgentSandboxResult[]
  overall_score: number
  pass_count: number
  total_tests: number
  ready_to_publish: boolean
}

export interface AgentPublishReq {
  name: string
  description?: string
  system_prompt: string
  model?: string
  tools?: string
  config?: string
  category?: string
  tags?: string
  icon?: string
  pricing?: string
}

export interface AgentPublishResp {
  template_id: string
  name: string
  category: string
  status: string
}

export interface ClawModel {
  id: string
  provider: string
  model_name: string
  display_name: string
  max_tokens: number
  temperature: number
  is_platform: boolean
}

export interface TeamMission {
  id: string
  instance_id: string
  claw_mission_id: string
  title: string
  goal: string
  status: string
  sprint_count: number
  total_steps: number
  done_steps: number
  review_score: number
  energy_used: number
  preview_url: string
  created_at: string
  completed_at: string | null
}

export interface ProvisionResult {
  status: 'ready' | 'provisioning'
  node_id?: string
  name?: string
  address?: string
  slug: string
  message?: string
}

export interface TeamAgentStats {
  total_instances: number
  active_instances: number
  total_missions: number
  total_energy: number
  template_count: number
}

export interface EmployeeUsage {
  user_id: string
  username: string
  message_count: number
  total_tokens: number
  input_tokens: number
  output_tokens: number
}

export interface ChatMessageRecord {
  id: string
  instance_id: string
  user_id: string
  role: string
  content: string
  model: string
  tokens_in: number
  tokens_out: number
  duration_ms: number
  created_at: string
}
