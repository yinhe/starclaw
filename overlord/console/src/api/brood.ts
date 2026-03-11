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
}

export function clearAuth() {
  localStorage.removeItem(TOKEN_KEY)
  localStorage.removeItem(USER_KEY)
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
}
