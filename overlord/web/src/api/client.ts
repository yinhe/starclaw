const API_BASE = '/brood'

// Shared auth: check web_token first, fallback to overlord_token (console SSO)
let _token = localStorage.getItem('web_token') || localStorage.getItem('overlord_token') || ''
let _user: { username: string; role: string; team_id: string } | null = null

try {
  const raw = localStorage.getItem('web_user') || localStorage.getItem('overlord_user')
  if (raw) _user = JSON.parse(raw)
} catch {}

export function getToken() { return _token }
export function getUser() { return _user }

export function setAuth(token: string, user: any) {
  _token = token
  _user = user
  // Write to both key sets so console stays in sync
  localStorage.setItem('web_token', token)
  localStorage.setItem('web_user', JSON.stringify(user))
  localStorage.setItem('overlord_token', token)
  localStorage.setItem('overlord_user', JSON.stringify(user))
}

export function clearAuth() {
  _token = ''
  _user = null
  localStorage.removeItem('web_token')
  localStorage.removeItem('web_user')
  localStorage.removeItem('overlord_token')
  localStorage.removeItem('overlord_user')
}

async function request(method: string, path: string, body?: any) {
  const headers: Record<string, string> = { 'Content-Type': 'application/json' }
  if (_token) headers['X-Admin-Token'] = _token

  const res = await fetch(API_BASE + path, {
    method,
    headers,
    body: body ? JSON.stringify(body) : undefined,
  })
  const data = await res.json()
  if (!res.ok) throw new Error(data.error || `HTTP ${res.status}`)
  return data
}

export const api = {
  login: (username: string, password: string) =>
    request('POST', '/auth/login', { username, password }),

  // Claws (for task routing)
  listClaws: (team?: string) =>
    request('GET', '/claws' + (team ? `?team=${team}` : '')),
  assignTask: (taskId: string, team?: string) =>
    request('POST', '/task/assign', { task_id: taskId, team }),
  resolve: (clawId: string) =>
    request('GET', `/resolve?claw_id=${clawId}`),

  // Stats
  stats: () => request('GET', '/stats'),

  // Team Agent
  teamTemplates: () => request('GET', '/team-agent/templates'),
  teamInstances: (status?: string) =>
    request('GET', '/team-agent/instances' + (status ? `?status=${status}` : '')),
  teamInstance: (id: string) => request('GET', `/team-agent/instances/${id}`),
  teamDashboard: (id: string) => request('GET', `/team-agent/instances/${id}/dashboard`),
  teamMissions: (id: string) => request('GET', `/team-agent/instances/${id}/missions`),
  createTeamMission: (id: string, goal: string) =>
    request('POST', `/team-agent/instances/${id}/missions`, { goal }),
  teamStats: () => request('GET', '/team-agent/stats'),

  // Chat (instance-based)
  sendChat: (instanceId: string, message: string) =>
    request('POST', `/team-agent/instances/${instanceId}/chat`, { message }),
  chatHistory: (instanceId: string) =>
    request('GET', `/team-agent/instances/${instanceId}/chat`),

  // Direct Chat (no instance/template needed)
  directChat: (message: string) =>
    request('POST', '/chat', { message }),
  directChatHistory: () =>
    request('GET', '/chat/history'),
}

export function isEmployee(): boolean {
  return _user?.role === 'viewer'
}

export function isAdmin(): boolean {
  return _user?.role === 'superadmin' || _user?.role === 'admin' || _user?.role === 'operator'
}
