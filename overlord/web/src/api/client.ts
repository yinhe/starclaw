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

  // Node login (Claw token exchange)
  nodeLogin: (nodeAddress: string, clawToken: string) =>
    request('POST', '/auth/node-login', { node_address: nodeAddress, claw_token: clawToken }),

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
  deleteMission: (instanceId: string, missionId: string) =>
    request('DELETE', `/team-agent/instances/${instanceId}/missions/${missionId}`),
  cancelMission: (instanceId: string, missionId: string) =>
    request('POST', `/team-agent/instances/${instanceId}/missions/${missionId}/cancel`),
  teamStats: () => request('GET', '/team-agent/stats'),

  // Models
  listModels: () => request('GET', '/models'),
  nodeModels: (nodeId: string) => request('GET', `/team-agent/node-models/${nodeId}`),

  // Chat (instance-based) — non-streaming fallback
  sendChat: (instanceId: string, message: string) =>
    request('POST', `/team-agent/instances/${instanceId}/chat`, { message }),
  chatHistory: (instanceId: string) =>
    request('GET', `/team-agent/instances/${instanceId}/chat`),

  // Direct Chat — non-streaming fallback
  directChat: (message: string) =>
    request('POST', '/chat', { message }),
  directChatHistory: () =>
    request('GET', '/chat/history'),

  // Conversations
  listConversations: (instanceId: string) =>
    request('GET', `/team-agent/instances/${instanceId}/conversations`),
  createConversation: (instanceId: string, title?: string, model?: string) =>
    request('POST', `/team-agent/instances/${instanceId}/conversations`, { title, model }),
  deleteConversation: (instanceId: string, convId: string) =>
    request('DELETE', `/team-agent/instances/${instanceId}/conversations/${convId}`),

  // Invite
  createInvite: (teamId?: string, role?: string, maxUses?: number) =>
    request('POST', '/admins/invite', { team_id: teamId, role, max_uses: maxUses }),
  registerWithInvite: (code: string, username: string, password: string) =>
    request('POST', '/auth/register', { code, username, password }),

  // Instance Access (employee binding)
  listInstanceAccess: (instanceId: string) =>
    request('GET', `/team-agent/instances/${instanceId}/access`),
  grantInstanceAccess: (instanceId: string, userId: string) =>
    request('POST', `/team-agent/instances/${instanceId}/access`, { user_id: userId }),
  revokeInstanceAccess: (instanceId: string, userId: string) =>
    request('DELETE', `/team-agent/instances/${instanceId}/access/${userId}`),
  listEmployees: () => request('GET', '/admins/employees'),
}

// SSE streaming chat — returns an AbortController and calls onChunk for each token
export async function streamChat(
  path: string,
  message: string,
  onChunk: (text: string) => void,
  onDone: () => void,
  onError: (err: string) => void,
): Promise<AbortController> {
  const controller = new AbortController()
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    'Accept': 'text/event-stream',
  }
  if (_token) headers['X-Admin-Token'] = _token

  try {
    const res = await fetch(API_BASE + path, {
      method: 'POST',
      headers,
      body: JSON.stringify({ message }),
      signal: controller.signal,
    })

    if (!res.ok) {
      const data = await res.json().catch(() => ({ error: `HTTP ${res.status}` }))
      onError(data.error || `HTTP ${res.status}`)
      return controller
    }

    const reader = res.body?.getReader()
    if (!reader) { onError('No response body'); return controller }

    const decoder = new TextDecoder()
    let buffer = ''

    while (true) {
      const { done, value } = await reader.read()
      if (done) break
      buffer += decoder.decode(value, { stream: true })

      const lines = buffer.split('\n')
      buffer = lines.pop() || ''

      for (const line of lines) {
        if (line.startsWith('data: ')) {
          const data = line.slice(6)
          if (data === '[DONE]') { onDone(); return controller }
          try {
            const chunk = JSON.parse(data)
            const content = chunk.choices?.[0]?.delta?.content
            if (content) onChunk(content)
          } catch {}
        }
      }
    }
    onDone()
  } catch (err: any) {
    if (err.name !== 'AbortError') {
      onError(err.message || 'Stream failed')
    }
  }
  return controller
}

export function isEmployee(): boolean {
  return _user?.role === 'viewer'
}

export function isAdmin(): boolean {
  return _user?.role === 'superadmin' || _user?.role === 'admin' || _user?.role === 'operator'
}
