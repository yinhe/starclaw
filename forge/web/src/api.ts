const BASE = '/api'

// ── Token management ──

export function getToken(): string | null {
  return localStorage.getItem('forge_token')
}

export function setToken(token: string, nodeId: string) {
  localStorage.setItem('forge_token', token)
  localStorage.setItem('forge_node_id', nodeId)
}

export function clearToken() {
  localStorage.removeItem('forge_token')
  localStorage.removeItem('forge_node_id')
}

export function getNodeId(): string | null {
  return localStorage.getItem('forge_node_id')
}

export function isLoggedIn(): boolean {
  return !!getToken()
}

// ── Request helpers with auth ──

async function request<T>(path: string, opts?: RequestInit): Promise<T> {
  const token = getToken()
  const headers: Record<string, string> = { 'Content-Type': 'application/json' }
  if (token) {
    headers['Authorization'] = `Bearer ${token}`
  }

  const res = await fetch(`${BASE}${path}`, {
    headers,
    ...opts,
  })

  if (res.status === 401) {
    clearToken()
    window.location.href = '/login'
    throw new Error('认证已过期')
  }

  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: res.statusText }))
    throw new Error(err.error || res.statusText)
  }
  return res.json()
}

function get<T>(path: string) {
  return request<T>(path)
}

function post<T>(path: string, body?: unknown) {
  return request<T>(path, { method: 'POST', body: body ? JSON.stringify(body) : undefined })
}

function put<T>(path: string, body?: unknown) {
  return request<T>(path, { method: 'PUT', body: body ? JSON.stringify(body) : undefined })
}

// SSE streaming helper — returns an async generator of chunks
export interface StreamChunk {
  type: 'thinking' | 'content' | 'done' | 'error' | 'result'
  text: string
}

async function* streamSSE(path: string, body: unknown): AsyncGenerator<StreamChunk> {
  const token = getToken()
  const headers: Record<string, string> = { 'Content-Type': 'application/json' }
  if (token) headers['Authorization'] = `Bearer ${token}`

  const res = await fetch(`${BASE}${path}`, {
    method: 'POST',
    headers,
    body: JSON.stringify(body),
  })

  if (res.status === 401) {
    clearToken()
    window.location.href = '/login'
    return
  }

  if (!res.ok || !res.body) {
    const err = await res.json().catch(() => ({ error: res.statusText }))
    yield { type: 'error', text: err.error || res.statusText }
    return
  }

  const reader = res.body.getReader()
  const decoder = new TextDecoder()
  let buffer = ''

  while (true) {
    const { done, value } = await reader.read()
    if (done) break

    buffer += decoder.decode(value, { stream: true })
    const lines = buffer.split('\n')
    buffer = lines.pop() || ''

    for (const line of lines) {
      if (!line.startsWith('data: ')) continue
      const data = line.slice(6).trim()
      if (!data) continue
      try {
        yield JSON.parse(data) as StreamChunk
      } catch { /* skip */ }
    }
  }
}

export const api = {
  // Auth
  login: (nodeId: string, password: string) => post<any>('/auth/login', { node_id: nodeId, password }),
  me: () => get<any>('/auth/me'),

  // Dashboard
  dashboard: () => get<any>('/dashboard'),
  services: () => get<any>('/dashboard/services'),
  activity: (params?: string) => get<any>(`/dashboard/activity${params ? `?${params}` : ''}`),
  stats: () => get<any>('/dashboard/stats'),
  devclaws: () => get<any>('/dashboard/devclaws'),
  heatmap: (repo?: string, days?: number) => get<any>(`/dashboard/heatmap?repo=${repo || 'starclaw'}&days=${days || 30}`),
  commits: (repo?: string, limit?: number) => get<any>(`/dashboard/commits?repo=${repo || 'starclaw'}&limit=${limit || 30}`),
  deploys: () => get<any>('/dashboard/deploys'),

  // Projects
  listProjects: () => get<any>('/projects'),
  getProject: (id: string) => get<any>(`/projects/${id}`),
  createProject: (data: any) => post<any>('/projects', data),

  // Issues
  listIssues: (projectId: string, params?: string) => get<any>(`/projects/${projectId}/issues${params ? `?${params}` : ''}`),
  getIssue: (id: string) => get<any>(`/issues/${id}`),
  getIssueByKey: (key: string) => get<any>(`/issues/key/${key}`),
  createIssue: (projectId: string, data: any) => post<any>(`/projects/${projectId}/issues`, data),
  updateIssue: (id: string, data: any) => put<any>(`/issues/${id}`, data),
  transitionIssue: (id: string, status: string) => post<any>(`/issues/${id}/transition`, { status }),
  completeIssue: (id: string) => post<any>(`/issues/${id}/complete`),
  addComment: (id: string, data: any) => post<any>(`/issues/${id}/comments`, data),
  board: (projectId: string, sprintId?: string) => get<any>(`/projects/${projectId}/board${sprintId ? `?sprint_id=${sprintId}` : ''}`),

  // Sprints
  listSprints: (projectId: string) => get<any>(`/projects/${projectId}/sprints`),
  createSprint: (projectId: string, data: any) => post<any>(`/projects/${projectId}/sprints`, data),
  startSprint: (sprintId: string) => post<any>(`/sprints/${sprintId}/start`),
  burndown: (sprintId: string) => get<any>(`/sprints/${sprintId}/burndown`),

  // PRD
  generatePRD: (projectId: string, prompt: string) => post<any>('/prd/generate', { project_id: projectId, prompt }),
  importPRD: (data: any) => post<any>('/prd/import', data),
  getPRD: (id: string) => get<any>(`/prd/${id}`),
  confirmPRD: (id: string) => post<any>(`/prd/${id}/confirm`),
  planPRD: (id: string) => post<any>(`/prd/${id}/plan`),

  // Streaming endpoints
  generatePRDStream: (projectId: string, prompt: string) =>
    streamSSE('/prd/generate/stream', { project_id: projectId, prompt }),
  planPRDStream: (prdId: string) =>
    streamSSE(`/prd/${prdId}/plan/stream`, {}),

  // Orchestrator
  orchestratorStatus: () => get<any>('/orchestrator/status'),
  listAgents: () => get<any>('/orchestrator/agents'),
  registerAgent: (data: any) => post<any>('/orchestrator/register', data),
}
