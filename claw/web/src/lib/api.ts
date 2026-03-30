import axios from 'axios'

const api = axios.create({
  baseURL: '/v1',
  headers: { 'Content-Type': 'application/json' },
})

api.interceptors.request.use((config) => {
  const token = localStorage.getItem('starclaw_token')
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

api.interceptors.response.use(
  (res) => res,
  (err) => {
    if (err.response?.status === 401) {
      const path = window.location.pathname
      // Don't redirect if already on setup or login page
      if (path !== '/setup' && path !== '/login') {
        localStorage.removeItem('starclaw_token')
        window.location.href = '/login'
      }
    }
    return Promise.reject(err)
  }
)

// Auth
export const authAPI = {
  register: (data: { email: string; username?: string; password: string }) =>
    api.post('/auth/register', data),
  login: (data: { email: string; password: string }) =>
    api.post('/auth/login', data),
  phoneRegister: (data: { phone: string; password: string; username?: string }) =>
    api.post('/auth/phone/register', data),
  phoneLogin: (data: { phone: string; password: string }) =>
    api.post('/auth/phone/login', data),
  tokenLogin: (data: { token: string; device_id: string; device_name?: string }) =>
    api.post('/auth/token/login', data),
  getAPIToken: () => api.get('/auth/token'),
  regenerateToken: () => api.post('/auth/token/regenerate'),
  listDevices: () => api.get('/auth/devices'),
  revokeDevice: (deviceID: string) => api.post(`/auth/devices/${deviceID}/revoke`),
  oauthProviders: () => api.get('/auth/oauth/providers'),
  oauthGitHub: (code: string) => api.post('/auth/oauth/github', { code }),
  oauthGoogle: (code: string) => api.post('/auth/oauth/google', { code }),
}

// Version
export const versionAPI = {
  check: () => api.get('/version'),
}

// System (update, swarm, bounty)
export const systemAPI = {
  getUpdate: () => api.get('/system/update'),
  triggerUpdate: () => api.post('/system/update'),
  forceCheck: () => api.post('/system/update/check'),
  getUpdateLog: () => api.get('/system/update-log'),
  getSwarm: () => api.get('/system/swarm'),
  joinSwarm: (data: { queen_url: string; node_name?: string; region?: string; invite_code?: string }) => api.post('/system/swarm/join', data),
  leaveSwarm: () => api.post('/system/swarm/leave'),
  getCredits: (params?: { refresh?: boolean }) => api.get('/system/credits', { params }),
  getCreditTransactions: (params?: { page?: number; page_size?: number; type?: string }) => api.get('/system/credits/transactions', { params }),
  getBounty: () => api.get('/system/bounty'),
  getBridge: () => api.get('/system/bridge'),
  stopBridge: () => api.post('/system/bridge/stop'),
  getOverlord: () => api.get('/system/overlord'),
  joinOverlord: (data: { overlord_url: string; node_name?: string; region?: string; invite_code?: string }) => api.post('/system/overlord/join', data),
  leaveOverlord: () => api.post('/system/overlord/leave'),
  getMining: () => api.get('/system/mining'),
  toggleMining: (enabled: boolean) => api.post('/system/mining/toggle', { enabled }),
}

// Node Identity & Peer Networking
export const nodeAPI = {
  getInfo: () => api.get('/node/info'),
  updateConfig: (data: { address?: string; name?: string; region?: string }) => api.put('/node/config', data),
  autoSetup: (data: { use_public_ip: boolean; port?: string; name?: string }) => api.post('/node/auto-setup', data),
}

export const peerAPI = {
  list: () => api.get('/peers'),
  add: (data: { address: string }) => api.post('/peers', data),
  remove: (id: string) => api.delete(`/peers/${id}`),
  ping: (id: string) => api.post(`/peers/${id}/ping`),
  resolve: (nodeId: string) => api.get(`/peers/resolve?node_id=${encodeURIComponent(nodeId)}`),
}

// Agents
export const agentAPI = {
  list: () => api.get('/agents'),
  create: (data: Record<string, unknown>) => api.post('/agents', data),
  get: (id: string) => api.get(`/agents/${id}`),
  update: (id: string, data: Record<string, unknown>) => api.put(`/agents/${id}`, data),
  delete: (id: string) => api.delete(`/agents/${id}`),
  export: (id: string) => api.get(`/agents/${id}/export`),
  getWorkflow: (id: string) => api.get(`/agents/${id}/workflow`),
  import: (data: Record<string, unknown>) => api.post('/agents/import', data),
  marketplace: () => api.get('/agents/marketplace'),
  clone: (id: string) => api.post(`/agents/${id}/clone`),
  installedSourceIDs: () => api.get('/agents/installed-source-ids'),
  installFromMarketplace: (data: Record<string, unknown>) => api.post('/agents/install-marketplace', data),
  uninstallBySourceID: (sourceId: string) => api.delete(`/agents/uninstall/${sourceId}`),
}

// Chat
export const chatAPI = {
  send: (data: { agent_id: string; conversation_id?: string; message: string; stream?: boolean }) =>
    api.post('/chat/completions', data),
  listConversations: () => api.get('/conversations'),
  getMessages: (conversationId: string) => api.get(`/conversations/${conversationId}/messages`),
}

// Models
export const modelAPI = {
  list: () => api.get('/models'),
  available: () => api.get('/models/available'),
  create: (data: Record<string, unknown>) => api.post('/models', data),
  update: (id: string, data: Record<string, unknown>) => api.put(`/models/${id}`, data),
  delete: (id: string) => api.delete(`/models/${id}`),
}

// Tools
export const toolAPI = {
  list: () => api.get('/tools'),
  skills: () => api.get('/skills'),
}

// Videos
export const videoAPI = {
  list: () => api.get('/videos'),
  delete: (id: string) => api.delete(`/videos/${id}`),
  cancel: (id: string) => api.post(`/videos/${id}/cancel`),
  retry: (id: string) => api.post(`/videos/${id}/retry`),
  regenerate: (id: string) => api.post(`/videos/${id}/regenerate`),
  remerge: (id: string) => api.post(`/videos/${id}/remerge`),
  dub: (id: string, text: string, voice: string, subtitleStyle?: string) =>
    api.post(`/videos/${id}/dub`, { text, voice, subtitle_style: subtitleStyle || 'auto' }),
  voices: () => api.get('/videos/voices'),
  addMusic: (id: string, musicId: string, lyricsSrt?: string) =>
    api.post(`/videos/${id}/add-music`, { music_id: musicId, lyrics_srt: lyricsSrt || '' }),
}

// Images
export const imageAPI = {
  list: () => api.get('/images'),
  delete: (id: string) => api.delete(`/images/${id}`),
}

// Music
export const musicAPI = {
  list: () => api.get('/music'),
  delete: (id: string) => api.delete(`/music/${id}`),
}

// Config (public, no auth)
export const configAPI = {
  get: () => axios.get('/v1/config'),
}

// Setup (single-user Owner mode, public endpoints)
export const setupAPI = {
  status: () => axios.get('/v1/setup/status'),
  setup: (data?: { password?: string; username?: string }) =>
    axios.post('/v1/setup', data || {}),
  getToken: () => axios.get('/v1/setup/token'),
  ownerLogin: (data: { password: string; device_id?: string; device_name?: string }) =>
    axios.post('/v1/auth/owner-login', data),
}

// Device Management
export const deviceAPI = {
  list: () => api.get('/devices'),
  approve: (id: string) => api.post(`/devices/${id}/approve`),
  reject: (id: string) => api.post(`/devices/${id}/reject`),
  revoke: (id: string) => api.post(`/devices/${id}/revoke`),
}

// Auth Requests (MetaMask-style login approval)
export const authRequestAPI = {
  list: () => api.get('/identity/auth-requests'),
  approve: (id: string) => api.post(`/identity/auth-request/${id}/approve`),
  reject: (id: string) => api.post(`/identity/auth-request/${id}/reject`),
}

// Queen Account Linking
export const queenAPI = {
  getStatus: () => api.get('/queen/status'),
  link: (data: { email?: string; phone?: string; password: string }) => api.post('/queen/link', data),
  linkWithClaw: () => api.post('/queen/link-claw'),
  autoRegister: (data?: { ref_code?: string; invite_code?: string; nickname?: string }) => api.post('/queen/auto-register', data || {}),
  unlink: () => api.post('/queen/unlink'),
}

// Billing & Tenant
export const billingAPI = {
  listPlans: () => api.get('/billing/plans'),
  getCurrentPlan: () => api.get('/billing/plan'),
  recharge: (planId: string) => api.post('/billing/recharge', { plan_id: planId }),
  getUsageHistory: () => api.get('/billing/usage'),
  getDailyUsage: (month?: string) => api.get('/billing/usage/daily', { params: month ? { month } : {} }),
  listTransactions: () => api.get('/billing/transactions'),
}

export const tenantAPI = {
  get: () => api.get('/tenant'),
  update: (data: { name: string }) => api.put('/tenant', data),
  addMember: (email: string, role?: string) => api.post('/tenant/members', { email, role }),
  removeMember: (userId: string) => api.delete(`/tenant/members/${userId}`),
  updateMemberRole: (userId: string, role: string) => api.put(`/tenant/members/${userId}/role`, { role }),
}

// Documents (workspace files)
export const documentAPI = {
  list: (conversationId?: string) => api.get('/documents', { params: conversationId ? { conversation_id: conversationId } : {} }),
  delete: (workspace: string, filepath: string) => api.delete(`/documents/${workspace}/${filepath}`),
  getURL: (workspace: string, filepath: string) => `/v1/documents/${workspace}/${filepath}`,
}

// Document Folders (tree browsing)
export const docFolderAPI = {
  listFolders: () => api.get('/doc-folders'),
  listFiles: (convId: string, page = 1, pageSize = 50) =>
    api.get(`/doc-folders/${convId}`, { params: { page, page_size: pageSize } }),
  deleteFolder: (convId: string) => api.delete(`/doc-folders/${convId}`),
  lockFolder: (convId: string) => api.post(`/doc-folders/${convId}/lock`),
  unlockFolder: (convId: string) => api.post(`/doc-folders/${convId}/unlock`),
}

// Knowledge Bases
export const knowledgeBaseAPI = {
  list: () => api.get('/knowledge-bases'),
  create: (data: Record<string, unknown>) => api.post('/knowledge-bases', data),
  get: (id: string) => api.get(`/knowledge-bases/${id}`),
  delete: (id: string) => api.delete(`/knowledge-bases/${id}`),
  uploadFile: (id: string, file: File) => {
    const formData = new FormData()
    formData.append('file', file)
    return api.post(`/knowledge-bases/${id}/documents`, formData, {
      headers: { 'Content-Type': 'multipart/form-data' },
    })
  },
  uploadText: (id: string, data: { name: string; content: string }) =>
    api.post(`/knowledge-bases/${id}/documents/text`, data),
  deleteDocument: (kbId: string, docId: string) =>
    api.delete(`/knowledge-bases/${kbId}/documents/${docId}`),
  search: (id: string, data: { query: string; top_k?: number }) =>
    api.post(`/knowledge-bases/${id}/search`, data),
}

// Conversations
export const conversationAPI = {
  list: () => api.get('/conversations'),
  rename: (id: string, title: string) => api.put(`/conversations/${id}`, { title }),
  delete: (id: string) => api.delete(`/conversations/${id}`),
  export: (id: string) => api.get(`/conversations/${id}/export`),
  messages: (id: string) => api.get(`/conversations/${id}/messages`),
  feedback: (convId: string, msgId: string, feedback: number) =>
    api.put(`/conversations/${convId}/messages/${msgId}/feedback`, { feedback }),
  pin: (id: string) => api.post(`/conversations/${id}/pin`),
  batchDelete: (ids: string[]) => api.post('/conversations/batch-delete', { ids }),
  context: (id: string) => api.get(`/conversations/${id}/context`),
  truncateMessages: (convId: string, msgId: string) =>
    api.post(`/conversations/${convId}/messages/${msgId}/truncate`),
}

// Dashboard
export const dashboardAPI = {
  stats: () => api.get('/dashboard/stats'),
}

// Settings
export const settingsAPI = {
  getProfile: () => api.get('/settings/profile'),
  updateProfile: (data: { username?: string; email?: string; phone?: string }) => api.put('/settings/profile', data),
  changePassword: (data: { old_password: string; new_password: string }) => api.put('/settings/password', data),
  getAPIKeys: () => api.get('/settings/api-keys'),
}

// Integrations (messaging platforms: Feishu, DingTalk, Slack, etc.)
export const integrationAPI = {
  list: (type?: string) => api.get('/integrations', { params: type ? { type } : {} }),
  create: (data: { type: string; name: string; config: Record<string, string> }) =>
    api.post('/integrations', data),
  update: (id: string, data: { name?: string; config?: Record<string, string>; enabled?: boolean }) =>
    api.put(`/integrations/${id}`, data),
  delete: (id: string) => api.delete(`/integrations/${id}`),
  test: (id: string) => api.post(`/integrations/${id}/test`),
}

// MCP Servers
export const mcpAPI = {
  listServers: () => api.get('/mcp/servers'),
  addServer: (data: { name: string; base_url: string; api_key?: string }) =>
    api.post('/mcp/servers', data),
  deleteServer: (id: string) => api.delete(`/mcp/servers/${id}`),
  testServer: (id: string) => api.post(`/mcp/servers/${id}/test`),
}

// Multi-Agent
export const multiAgentAPI = {
  run: (data: { agent_ids: string[]; orchestrator_id?: string; mode: string; input: string; max_rounds?: number }) =>
    api.post('/multi-agent/run', data),
}

// Teams
export const teamAPI = {
  getOrchestrator: (teamId: string) => api.get(`/teams/${teamId}/orchestrator`),
}

// Workflows
export const workflowAPI = {
  list: () => api.get('/workflows'),
  create: (data: Record<string, unknown>) => api.post('/workflows', data),
  get: (id: string) => api.get(`/workflows/${id}`),
  update: (id: string, data: Record<string, unknown>) => api.put(`/workflows/${id}`, data),
  delete: (id: string) => api.delete(`/workflows/${id}`),
  run: (id: string, data: { input: string }) => api.post(`/workflows/${id}/run`, data),
  listRuns: (id: string) => api.get(`/workflows/${id}/runs`),
  enableWebhook: (id: string) => api.post(`/workflows/${id}/webhook/enable`),
  disableWebhook: (id: string) => api.post(`/workflows/${id}/webhook/disable`),
}

// Workflow Templates (Marketplace)
export const workflowTemplateAPI = {
  list: (category?: string) => api.get('/workflow-templates', { params: category ? { category } : {} }),
  publish: (data: { workflow_id: string; name: string; description?: string; category?: string }) =>
    api.post('/workflow-templates', data),
  clone: (id: string) => api.post(`/workflow-templates/${id}/clone`),
  delete: (id: string) => api.delete(`/workflow-templates/${id}`),
}

// Agent Templates (Creep Marketplace)
export const templateAPI = {
  list: (params?: { category?: string; q?: string; featured?: string }) =>
    api.get('/templates', { params }),
  get: (id: string) => api.get(`/templates/${id}`),
  categories: () => api.get('/templates/categories'),
  publish: (data: { agent_id: string; category?: string; tags?: string; icon?: string; description?: string }) =>
    api.post('/templates', data),
  install: (id: string) => api.post(`/templates/${id}/install`),
  installRemote: (data: { name: string; description?: string; system_prompt?: string; tools?: string; config?: string; source?: string; source_id?: string }) =>
    api.post('/templates/install-remote', data),
  rate: (id: string, rating: number) => api.post(`/templates/${id}/rate`, { rating }),
}

// Community Marketplace (proxied through local Claw backend to avoid CORS)
export const queenMarketplaceAPI = {
  list: (params?: { q?: string }) =>
    api.get('/templates/community', { params }),
  get: (id: string) =>
    api.get(`/templates/community/${id}`),
}

// Schedules (Cron)
export const scheduleAPI = {
  list: () => api.get('/schedules'),
  create: (data: { workflow_id: string; cron_expr: string; input?: string }) =>
    api.post('/schedules', data),
  toggle: (id: string) => api.post(`/schedules/${id}/toggle`),
  delete: (id: string) => api.delete(`/schedules/${id}`),
}

// Activities (Instinct — proactive behavior system)
export const activityAPI = {
  list: (type?: string) => api.get('/activities', { params: type ? { type } : {} }),
  get: (id: string) => api.get(`/activities/${id}`),
  create: (data: any) => api.post('/activities', data),
  update: (id: string, data: any) => api.put(`/activities/${id}`, data),
  toggle: (id: string) => api.post(`/activities/${id}/toggle`),
  delete: (id: string) => api.delete(`/activities/${id}`),
  logs: (id: string) => api.get(`/activities/${id}/logs`),
  templates: () => api.get('/activities/templates'),
  seed: (agentId?: string) => api.post('/activities/seed', null, { params: agentId ? { agent_id: agentId } : {} }),
  fireEvent: (event: string) => api.post(`/activities/events/${event}`),
}

// Long-term Memory
export const memoryAPI = {
  list: (params?: { agent_id?: string; category?: string; search?: string }) =>
    api.get('/memories', { params }),
  create: (data: { agent_id: string; key: string; content: string; category?: string; importance?: number }) =>
    api.post('/memories', data),
  update: (id: string, data: { content?: string; importance?: number }) => api.put(`/memories/${id}`, data),
  delete: (id: string) => api.delete(`/memories/${id}`),
  clear: (agentId?: string) => api.delete('/memories', { params: agentId ? { agent_id: agentId } : {} }),
  stats: () => api.get('/memories/stats'),
  recall: (agentId: string) => api.get(`/memories/recall/${agentId}`),
}

// Agent Evaluation
export const evalAPI = {
  listCases: (agentId?: string) => api.get('/eval/test-cases', { params: agentId ? { agent_id: agentId } : {} }),
  createCase: (data: { agent_id: string; name: string; input: string; expected_output?: string; tags?: string }) =>
    api.post('/eval/test-cases', data),
  deleteCase: (id: string) => api.delete(`/eval/test-cases/${id}`),
  runCase: (id: string) => api.post(`/eval/test-cases/${id}/run`),
  listRuns: (agentId?: string) => api.get('/eval/runs', { params: agentId ? { agent_id: agentId } : {} }),
}

// Admin (requires admin role)
export const adminAPI = {
  listUsers: () => api.get('/admin/users'),
  updateRole: (id: string, role: string) => api.put(`/admin/users/${id}/role`, { role }),
  deleteUser: (id: string) => api.delete(`/admin/users/${id}`),
  stats: () => api.get('/admin/stats'),
}

// Audit Logs
export const auditAPI = {
  list: () => api.get('/audit-logs'),
}

// Multimodal (image upload, STT, TTS)
export const multimodalAPI = {
  uploadImage: (file: File) => {
    const form = new FormData()
    form.append('file', file)
    return api.post('/multimodal/upload-image', form, {
      headers: { 'Content-Type': 'multipart/form-data' },
    })
  },
  stt: (file: Blob) => {
    const form = new FormData()
    form.append('file', file, 'recording.webm')
    return api.post('/multimodal/stt', form, {
      headers: { 'Content-Type': 'multipart/form-data' },
    })
  },
  tts: (text: string, voice?: string) =>
    api.post('/multimodal/tts', { text, voice: voice || 'alloy' }, { responseType: 'blob' }),
}

// File upload (general: documents, audio, video, code, etc.)
export const fileAPI = {
  upload: (file: File) => {
    const form = new FormData()
    form.append('file', file)
    return api.post('/upload', form, {
      headers: { 'Content-Type': 'multipart/form-data' },
    })
  },
}

// Coding Agent
// Super Agent (auto-create)
export const superAgentAPI = {
  ensure: () => api.post('/agents/super-agent'),
}

export const codingAPI = {
  listFiles: (workspaceId: string, path?: string) =>
    api.get(`/coding/workspace/${workspaceId}/files`, { params: { path: path || '.' } }),
  readFile: (workspaceId: string, path: string) =>
    api.get(`/coding/workspace/${workspaceId}/file`, { params: { path } }),
  executeCode: (language: string, code: string, timeout?: number) =>
    api.post('/coding/execute', { language, code, timeout: timeout || 15 }),
  runFile: (workspaceId: string, filePath: string, conversationId?: string, timeout?: number) =>
    api.post('/coding/run-file', { workspace_id: workspaceId, file_path: filePath, conversation_id: conversationId || '', timeout: timeout || 30 }),
  runCommand: (command: string, workspaceId?: string, conversationId?: string, timeout?: number) =>
    api.post('/coding/run-command', { command, workspace_id: workspaceId || '', conversation_id: conversationId || '', timeout: timeout || 30 }),
  stop: () => api.post('/coding/stop'),
  previewUrl: (workspaceId: string, filePath: string) =>
    `/v1/preview/${workspaceId}/${filePath}`,
}

// Tasks (autonomous background execution)
export const taskAPI = {
  list: (status?: string) => api.get('/tasks', { params: status ? { status } : {} }),
  get: (id: string) => api.get(`/tasks/${id}`),
  create: (data: { title: string; goal: string; agent_id?: string; priority?: string; scheduled_at?: string }) =>
    api.post('/tasks', data),
  cancel: (id: string) => api.post(`/tasks/${id}/cancel`),
  pause: (id: string) => api.post(`/tasks/${id}/pause`),
  resume: (id: string) => api.post(`/tasks/${id}/resume`),
  visualization: (conversationId?: string) => api.get('/tasks/visualization', { params: conversationId ? { conversation_id: conversationId } : {} }),
  workerPause: () => api.post('/tasks/worker/pause'),
  workerResume: () => api.post('/tasks/worker/resume'),
  workerStop: () => api.post('/tasks/worker/stop'),
  workerStatus: () => api.get('/tasks/worker/status'),
}

// Notifications
export const notificationAPI = {
  list: (unread?: boolean) => api.get('/notifications', { params: unread ? { unread: 'true' } : {} }),
  markRead: (ids?: string[]) => api.post('/notifications/read', { ids }),
  unreadCount: () => api.get('/notifications/unread-count'),
}

// ── P8: Observability ──
export const observeAPI = {
  stats: () => api.get('/observe/stats'),
  getTrace: (traceId: string) => api.get(`/observe/traces/${traceId}`),
  querySpans: (params?: { service?: string; kind?: string; status?: string }) =>
    api.get('/observe/spans', { params }),
  queryLogs: (params?: { level?: string; service?: string; trace_id?: string; page?: number; page_size?: number }) =>
    api.get('/observe/logs', { params }),
  listAlertRules: () => api.get('/observe/alerts/rules'),
  createAlertRule: (data: Record<string, unknown>) => api.post('/observe/alerts/rules', data),
  updateAlertRule: (id: string, data: Record<string, unknown>) => api.put(`/observe/alerts/rules/${id}`, data),
  toggleAlertRule: (id: string) => api.post(`/observe/alerts/rules/${id}/toggle`),
  deleteAlertRule: (id: string) => api.delete(`/observe/alerts/rules/${id}`),
  listAlertHistory: (params?: { rule_id?: string; resolved?: string }) =>
    api.get('/observe/alerts/history', { params }),
  resolveAlert: (id: string) => api.post(`/observe/alerts/history/${id}/resolve`),
}

// ── P8: Webhooks ──
export const webhookAPI = {
  listRules: () => api.get('/webhooks/rules'),
  createRule: (data: Record<string, unknown>) => api.post('/webhooks/rules', data),
  updateRule: (id: string, data: Record<string, unknown>) => api.put(`/webhooks/rules/${id}`, data),
  toggleRule: (id: string) => api.post(`/webhooks/rules/${id}/toggle`),
  deleteRule: (id: string) => api.delete(`/webhooks/rules/${id}`),
  listLogs: (params?: { status?: string; event_type?: string; page?: number; page_size?: number }) =>
    api.get('/webhooks/logs', { params }),
  retryDeadLetter: (id: string) => api.post(`/webhooks/logs/${id}/retry`),
  stats: () => api.get('/webhooks/stats'),
  eventTypes: () => api.get('/webhooks/event-types'),
  test: (data: { event_type: string; data?: Record<string, unknown> }) => api.post('/webhooks/test', data),
}

// ── P9: Developer Portal ──
export const developerAPI = {
  openApiSpec: () => api.get('/developer/openapi.json'),
  listPlugins: (params?: { category?: string; q?: string; sort?: string; page?: number }) =>
    api.get('/developer/plugins', { params }),
  getPlugin: (id: string) => api.get(`/developer/plugins/${id}`),
  publishPlugin: (data: Record<string, unknown>) => api.post('/developer/plugins', data),
  myPlugins: () => api.get('/developer/plugins/mine'),
  installPlugin: (id: string) => api.post(`/developer/plugins/${id}/install`),
  uninstallPlugin: (id: string) => api.delete(`/developer/plugins/${id}/install`),
  myInstalled: () => api.get('/developer/plugins/installed'),
  ratePlugin: (id: string, data: { score: number; comment?: string }) =>
    api.post(`/developer/plugins/${id}/rate`, data),
  categories: () => api.get('/developer/plugins/categories'),
  playgroundExecute: (data: { method: string; path: string; body?: string; headers?: Record<string, string> }) =>
    api.post('/developer/playground/execute', data),
  playgroundHistory: (limit?: number) => api.get('/developer/playground/history', { params: limit ? { limit } : {} }),
  stats: () => api.get('/developer/stats'),
}

// ── P9: Security ──
export const securityAPI = {
  overview: () => api.get('/security/overview'),
  encryption: () => api.get('/security/encryption'),
  auditQuery: (params?: { action?: string; actor?: string; severity?: string; page?: number; page_size?: number }) =>
    api.get('/security/audit', { params }),
  auditVerify: () => api.get('/security/audit/verify'),
  auditExport: (since?: string) => api.get('/security/audit/export', { params: since ? { since } : {}, responseType: 'blob' as const }),
  auditStats: () => api.get('/security/audit/stats'),
  gdprExport: () => api.get('/security/gdpr/export', { responseType: 'blob' as const }),
  gdprDelete: (confirm: boolean) => api.post('/security/gdpr/delete', { confirm }),
  gdprConsent: () => api.get('/security/gdpr/consent'),
  compliance: (framework?: string) => api.get('/security/compliance', { params: framework ? { framework } : {} }),
}

// ── P10: Goals (Proactive Agent) ──
export const goalAPI = {
  list: (params?: { status?: string; page?: number; page_size?: number }) =>
    api.get('/goals', { params }),
  create: (data: Record<string, unknown>) => api.post('/goals', data),
  get: (id: string) => api.get(`/goals/${id}`),
  activate: (id: string) => api.post(`/goals/${id}/activate`),
  cancel: (id: string) => api.post(`/goals/${id}/cancel`),
  stats: () => api.get('/goals/stats'),
  decompositionPrompt: () => api.get('/goals/decomposition-prompt'),
}

// ── P10: Collaboration ──
export const collaborationAPI = {
  list: () => api.get('/collaborations'),
  create: (data: Record<string, unknown>) => api.post('/collaborations', data),
  join: (id: string, data: { agent_id: string; role?: string; capabilities?: string }) =>
    api.post(`/collaborations/${id}/join`, data),
  members: (id: string) => api.get(`/collaborations/${id}/members`),
  messages: (id: string) => api.get(`/collaborations/${id}/messages`),
  sendMessage: (id: string, data: { agent_id: string; message_type: string; content: string }) =>
    api.post(`/collaborations/${id}/messages`, data),
  vote: (id: string, data: { agent_id: string; vote: string }) =>
    api.post(`/collaborations/${id}/vote`, data),
}

// ── P10: Fine-tuning ──
export const fineTuneAPI = {
  listAdapters: (params?: { page?: number; page_size?: number }) =>
    api.get('/finetune/adapters', { params }),
  createAdapter: (data: Record<string, unknown>) => api.post('/finetune/adapters', data),
  getAdapter: (id: string) => api.get(`/finetune/adapters/${id}`),
  deleteAdapter: (id: string) => api.delete(`/finetune/adapters/${id}`),
  startTraining: (id: string) => api.post(`/finetune/adapters/${id}/train`),
  exportSamples: (id: string) => api.get(`/finetune/adapters/${id}/export`, { responseType: 'blob' as const }),
  listSamples: (id: string, params?: { page?: number; page_size?: number }) =>
    api.get(`/finetune/adapters/${id}/samples`, { params }),
  addSample: (id: string, data: { input: string; output: string; system?: string }) =>
    api.post(`/finetune/adapters/${id}/samples`, data),
  addSamplesBatch: (id: string, samples: { input: string; output: string; system?: string }[]) =>
    api.post(`/finetune/adapters/${id}/samples/batch`, { samples }),
  deleteSample: (sampleId: string) => api.delete(`/finetune/samples/${sampleId}`),
  listDistillation: (params?: { page?: number }) => api.get('/finetune/distillation', { params }),
  createDistillation: (data: Record<string, unknown>) => api.post('/finetune/distillation', data),
  getDistillation: (id: string) => api.get(`/finetune/distillation/${id}`),
  cancelDistillation: (id: string) => api.post(`/finetune/distillation/${id}/cancel`),
  distillationPrompt: () => api.get('/finetune/distillation/prompt'),
  stats: () => api.get('/finetune/stats'),
}

// ── Squad (multi-node team collaboration) ──
export const squadAPI = {
  list: () => api.get('/squads'),
  create: (data: { name: string; description?: string; max_members?: number; is_public?: boolean; tags?: string[] }) =>
    api.post('/squads', data),
  get: (id: string) => api.get(`/squads/${id}`),
  update: (id: string, data: Record<string, unknown>) => api.put(`/squads/${id}`, data),
  delete: (id: string) => api.delete(`/squads/${id}`),
  invite: (id: string, data: { node_id: string; specialty?: string }) =>
    api.post(`/squads/${id}/invite`, data),
  members: (id: string) => api.get(`/squads/${id}/members`),
  removeMember: (id: string, nodeId: string) => api.delete(`/squads/${id}/members/${nodeId}`),
  createMission: (id: string, data: { title: string; goal: string }) =>
    api.post(`/squads/${id}/missions`, data),
  listMissions: (id: string) => api.get(`/squads/${id}/missions`),
  getMission: (id: string) => api.get(`/missions/${id}`),
  startMission: (id: string) => api.post(`/missions/${id}/start`),
  cancelMission: (id: string) => api.post(`/missions/${id}/cancel`),
  missions: () => api.get('/missions'),
  missionSteps: (missionId: string) => api.get(`/missions/${missionId}/steps`),
  sprints: (missionId: string) => api.get(`/missions/${missionId}/sprints`),
  submitFeedback: (missionId: string, feedback: string) => api.post(`/missions/${missionId}/feedback`, { feedback }),
  reviews: (missionId: string) => api.get(`/missions/${missionId}/reviews`),
}

// ── Node Growth System (one pet per Claw node) ──
export const growthAPI = {
  getGrowth: () => api.get('/growth'),
  getMilestones: () => api.get('/growth/milestones'),
  getNewMilestones: () => api.get('/growth/milestones/new'),
  getDailyReport: (date?: string) =>
    api.get('/growth/daily-report', { params: date ? { date } : {} }),
  getGrowthCurve: (days?: number) =>
    api.get('/growth/curve', { params: { days: days || 7 } }),
  getAssets: () => api.get('/assets/overview'),
  // v2: Path & Realm choice
  choosePath: (path: string) => api.post('/growth/choose-path', { path }),
  chooseRealm: (realm: string) => api.post('/growth/choose-realm', { realm }),
  // v2: Endgame
  awaken: () => api.post('/growth/awaken'),
  fuse: (targetPath: string) => api.post('/growth/fuse', { target_path: targetPath }),
  rebirth: (newPath: string) => api.post('/growth/rebirth', { new_path: newPath }),
}

// ── Swarm System (Agent → 虫群) ──
export const swarmAPI = {
  list: () => api.get('/swarm'),
  get: (id: string) => api.get(`/swarm/${id}`),
  invest: (id: string, stat: string, amount: number) => api.post(`/swarm/${id}/invest`, { stat, amount }),
}

// ── Stardust Economy (星尘) ──
export const stardustAPI = {
  balance: () => api.get('/stardust'),
  transactions: () => api.get('/stardust/transactions'),
  enhanceHero: (stat: string, amount: number) => api.post('/stardust/enhance-hero', { stat, amount }),
  hatch: (type?: string) => api.post('/stardust/hatch', type ? { type } : {}),
}

// ── Arena PK Battle System (proxied through Queen) ──
export const arenaAPI = {
  registerFighter: (data: { claw_id: string; name: string; level: number; evolution_path: string; form_code: string; base_hp: number; base_atk: number; base_def: number; base_spd: number }) =>
    api.post('/arena/pk/register', data),
  getFighter: (clawId: string) => api.get(`/arena/pk/fighter/${clawId}`),
  challenge: (challengerClawId: string, opponentClawId: string) =>
    api.post('/arena/pk/challenge', { challenger_claw_id: challengerClawId, opponent_claw_id: opponentClawId }),
  getHistory: (clawId: string) => api.get(`/arena/pk/history/${clawId}`),
  getLeaderboard: () => api.get('/arena/pk/leaderboard'),
  getShop: () => api.get('/arena/pk/shop'),
  buyEquipment: (clawId: string, defId: string) =>
    api.post('/arena/pk/shop/buy', { claw_id: clawId, def_id: defId }),
  equip: (clawId: string, itemId: string) =>
    api.post('/arena/pk/equip', { claw_id: clawId, item_id: itemId }),
  unequip: (clawId: string, slot: string) =>
    api.post('/arena/pk/unequip', { claw_id: clawId, slot }),
  getInventory: (clawId: string) => api.get(`/arena/pk/inventory/${clawId}`),
  // Stardust
  getStardust: (clawId: string) => api.get(`/arena/pk/stardust/${clawId}`),
  // Season
  getSeason: () => api.get('/arena/pk/season'),
  getSeasonRecord: (clawId: string) => api.get(`/arena/pk/season/record/${clawId}`),
  // Crafting
  getMaterials: () => api.get('/arena/pk/craft/materials'),
  getMyMaterials: (clawId: string) => api.get(`/arena/pk/craft/inventory/${clawId}`),
  getRecipes: () => api.get('/arena/pk/craft/recipes'),
  craft: (clawId: string, recipeId: string) =>
    api.post('/arena/pk/craft', { claw_id: clawId, recipe_id: recipeId }),
  collectMaterials: (clawId: string) =>
    api.post('/arena/pk/craft/collect', { claw_id: clawId }),
  // Mutations
  getMutations: (clawId: string) => api.get(`/arena/pk/mutations/${clawId}`),
  triggerMutation: (clawId: string) =>
    api.post('/arena/pk/mutations/trigger', { claw_id: clawId }),
}

// ── Identity Recovery ──
export const recoveryAPI = {
  getStatus: () => api.get('/recovery/status'),
  getMnemonic: () => api.get('/recovery/mnemonic'),
  confirmMnemonic: (mnemonic: string) => api.post('/recovery/confirm-mnemonic', { mnemonic }),
  bindPhone: (phone: string) => api.post('/recovery/bind-phone', { phone }),
  verifyPhone: (phone: string, code: string) => api.post('/recovery/verify-phone', { phone, code }),
  backup: () => api.post('/recovery/backup'),
  verifyMnemonic: (mnemonic: string) => api.post('/recovery/verify-mnemonic', { mnemonic }),
  restore: (mnemonic: string) => api.post('/recovery/restore', { mnemonic }),
  getAddress: () => api.get('/recovery/address'),
}

export default api
