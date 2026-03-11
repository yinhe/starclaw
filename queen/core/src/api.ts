const API_BASE = import.meta.env.VITE_API_URL || ''

function getToken(): string | null {
  return localStorage.getItem('queen_token')
}

export function setToken(token: string) {
  localStorage.setItem('queen_token', token)
}

export function clearToken() {
  localStorage.removeItem('queen_token')
}

export function isLoggedIn(): boolean {
  return !!getToken()
}

async function request<T>(path: string, options: RequestInit = {}): Promise<T> {
  const token = getToken()
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    ...(options.headers as Record<string, string> || {}),
  }
  if (token) {
    headers['Authorization'] = `Bearer ${token}`
  }

  const resp = await fetch(`${API_BASE}${path}`, {
    ...options,
    headers,
  })

  if (resp.status === 401) {
    clearToken()
    window.location.href = '/login'
    throw new Error('Unauthorized')
  }

  const data = await resp.json()
  if (!resp.ok) {
    throw new Error(data.error || `HTTP ${resp.status}`)
  }
  return data as T
}

export const api = {
  get: <T>(path: string) => request<T>(path),
  post: <T>(path: string, body?: unknown) =>
    request<T>(path, { method: 'POST', body: body ? JSON.stringify(body) : undefined }),
  put: <T>(path: string, body?: unknown) =>
    request<T>(path, { method: 'PUT', body: body ? JSON.stringify(body) : undefined }),
  delete: <T>(path: string) => request<T>(path, { method: 'DELETE' }),
}

// ---- API types ----

export interface BillingStats {
  total_revenue: number
  total_orders: number
  today_revenue: number
  today_orders: number
  month_revenue: number
  total_users: number
  total_balance: number
  total_consumed: number
}

export interface RechargeOrder {
  id: string
  order_no: string
  user_id: string
  amount: number
  bonus_amount: number
  pay_method: string
  status: string
  created_at: string
  paid_at: string | null
}

export interface UserBalance {
  id: string
  user_id: string
  balance: number
  total_in: number
  total_out: number
}

export interface RechargePackage {
  id: string
  name: string
  amount: number
  bonus_amount: number
  bonus_rate: number
  sort_order: number
  enabled: boolean
}

export interface SwarmStats {
  total_nodes: number
  online_nodes: number
  offline_nodes: number
  feral_nodes: number
  claw_nodes: number
  overlord_nodes: number
}

export interface SwarmNode {
  id: string
  name: string
  role: string
  status: string
  version: string
  address: string
  region: string
  cpu_percent: number
  mem_percent: number
  tasks_running: number
  last_heartbeat: string
}

export interface QueenUser {
  id: string
  email: string
  phone: string
  nickname: string
  avatar: string
  bio: string
  role: string
  status: string
  oauth_provider: string
  created_at: string
  updated_at: string
}

export interface UserStats {
  total: number
  active: number
  banned: number
  admins: number
  developers: number
}

export interface ContentReport {
  id: string
  reporter_id: string
  target_type: string
  target_id: string
  target_title: string
  author_id: string
  reason: string
  detail: string
  status: string
  resolution: string
  reviewer_id: string
  review_note: string
  reviewed_at: string | null
  created_at: string
}

export interface ReportStats {
  total: number
  pending: number
  reviewed: number
  resolved: number
  dismissed: number
}

export interface ServiceStats {
  [key: string]: number | string
}
