const API_BASE = import.meta.env.VITE_API_URL || ''

function getToken() {
  return localStorage.getItem('queen_token') || ''
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, {
    ...init,
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${getToken()}`,
      ...init?.headers,
    },
  })
  if (!res.ok) throw new Error(`${res.status} ${res.statusText}`)
  return res.json()
}

export interface DashboardData {
  nodes: { total: number; online: number }
  energy: { total_accounts: number; total_balance: number; total_granted: number; total_consumed: number }
  users: number
  marketplace: number
}

export interface ServiceStatus {
  name: string
  status: 'up' | 'down'
  latency_ms: number
}

export interface NodeInfo {
  id: string
  claw_id: string
  name: string
  version: string
  region: string
  status: string
  ip: string
  last_heartbeat: string
  created_at: string
  is_contributor?: boolean
  gpu_info?: string
}

export interface CreditAccount {
  id: string
  claw_id: string
  balance: number
  frozen: number
  total_in: number
  total_out: number
  status: string
  trust_level: string
  created_at: string
}

export interface CreditTransaction {
  id: string
  from_claw: string
  to_claw: string
  amount: number
  fee: number
  type: string
  created_at: string
}

export interface TypeStat {
  type: string
  count: number
  total: number
}

export interface HPBucket {
  status: string
  count: number
}

export interface EnergyData {
  top_accounts: CreditAccount[]
  recent_tx: CreditTransaction[]
  type_stats: TypeStat[]
  hp_distribution: HPBucket[]
}

export interface PromResult {
  status: string
  data: {
    resultType: string
    result: Array<{
      metric: Record<string, string>
      value?: [number, string]
      values?: [number, string][]
    }>
  }
}

export interface Alert {
  labels: Record<string, string>
  annotations: Record<string, string>
  state: string
  activeAt: string
  value: string
}

export const overseerAPI = {
  dashboard: () => request<DashboardData>('/v1/admin/overseer/dashboard'),
  nodes: (page = 1, size = 50, status = '') =>
    request<{ nodes: NodeInfo[]; total: number }>(`/v1/admin/overseer/nodes?page=${page}&size=${size}&status=${status}`),
  nodeDetail: (id: string) => request<{ node: NodeInfo }>(`/v1/admin/overseer/nodes/${id}`),
  services: () => request<{ services: ServiceStatus[] }>('/v1/admin/overseer/services'),
  energy: () => request<EnergyData>('/v1/admin/overseer/energy'),
  alerts: () => request<{ status: string; data: { alerts: Alert[] } }>('/v1/admin/overseer/alerts'),
  metricsQuery: (query: string) =>
    request<PromResult>(`/v1/admin/overseer/metrics/query?query=${encodeURIComponent(query)}`),
  metricsRange: (query: string, start: number, end: number, step = 60) =>
    request<PromResult>(`/v1/admin/overseer/metrics/query_range?query=${encodeURIComponent(query)}&start=${start}&end=${end}&step=${step}`),
}

export const authAPI = {
  login: (email: string, password: string) =>
    request<{ token: string; user: { id: string; nickname: string; role: string } }>('/v1/auth/login', {
      method: 'POST',
      body: JSON.stringify({ email, password }),
    }),
}
