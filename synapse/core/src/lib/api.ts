const BASE = '';

function getToken(): string | null {
  return localStorage.getItem('admin_token');
}

export function setToken(token: string) {
  localStorage.setItem('admin_token', token);
}

export function clearToken() {
  localStorage.removeItem('admin_token');
}

export function isLoggedIn(): boolean {
  return !!getToken();
}

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  const headers: Record<string, string> = { 'Content-Type': 'application/json' };
  const token = getToken();
  if (token) headers['Authorization'] = `Bearer ${token}`;

  const res = await fetch(`${BASE}${path}`, {
    method,
    headers,
    body: body ? JSON.stringify(body) : undefined,
  });

  if (res.status === 401 || res.status === 403) {
    clearToken();
    window.location.href = '/login';
    throw new Error('Unauthorized');
  }

  const data = await res.json();
  if (!res.ok) throw new Error(data.error || `HTTP ${res.status}`);
  return data as T;
}

// Auth (reuses same /auth/login endpoint)
export const auth = {
  login: (data: { email?: string; phone?: string; password: string }) =>
    request<{ token: string; user: { id: string; email: string; phone: string; name: string } }>(
      'POST', '/auth/login', data
    ),
};

// Admin API
export interface User {
  id: string;
  email: string;
  name: string;
  balance: number;
  free_quota: number;
  is_admin: boolean;
  status: string;
  created_at: string;
}

export interface UsageLog {
  id: string;
  user_id: string;
  model: string;
  endpoint: string;
  provider: string;
  prompt_tokens: number;
  completion_tokens: number;
  total_tokens: number;
  cost_cents: number;
  upstream_cost: number;
  duration: number;
  status: string;
  error_msg: string;
  via: string;
  created_at: string;
}

export interface Order {
  id: string;
  user_id: string;
  order_no: string;
  channel: string;
  amount_cents: number;
  bonus_cents: number;
  total_cents: number;
  status: string;
  trade_no: string;
  paid_at: string | null;
  created_at: string;
}

export interface ProviderInfo {
  slug: string;
  model_count: number;
}

export interface ModelInfo {
  name: string;
  provider: string;
  type: string;
  context_length: number;
  input_price: number;
  output_price: number;
}

export interface Paginated<T> {
  total: number;
  page: number;
  page_size: number;
  pages: number;
  items: T[];
}

export interface Role {
  id: string;
  name: string;
  description: string;
  permissions: Permission[];
  created_at: string;
}

export interface Permission {
  id: string;
  name: string;
  description: string;
}

export interface AdminMeResponse {
  user: { id: string; email: string; name: string; is_admin: boolean };
  roles: string[];
  permissions: string[];
}

export const admin = {
  overview: () =>
    request<{ users: number; api_keys: number; paid_orders: number; total_revenue: number; total_requests: number; today_requests: number; today_users: number }>('GET', '/admin/overview'),

  users: (params?: { page?: number; page_size?: number; q?: string; status?: string }) => {
    const q = new URLSearchParams();
    if (params?.page) q.set('page', String(params.page));
    if (params?.page_size) q.set('page_size', String(params.page_size));
    if (params?.q) q.set('q', params.q);
    if (params?.status) q.set('status', params.status);
    const qs = q.toString();
    return request<{ users: User[]; total: number; page: number; page_size: number; pages: number }>('GET', `/admin/users${qs ? '?' + qs : ''}`);
  },

  getUser: (id: string) =>
    request<{ user: User; api_key_count: number; request_count: number; order_count: number }>('GET', `/admin/users/${id}`),

  updateUser: (id: string, data: { status?: string; balance?: number; free_quota?: number; is_admin?: boolean; name?: string }) =>
    request<{ message: string }>('PUT', `/admin/users/${id}`, data),

  logs: (params?: { page?: number; page_size?: number; user_id?: string; model?: string; status?: string; provider?: string; from?: string; to?: string }) => {
    const q = new URLSearchParams();
    if (params?.page) q.set('page', String(params.page));
    if (params?.page_size) q.set('page_size', String(params.page_size));
    if (params?.user_id) q.set('user_id', params.user_id);
    if (params?.model) q.set('model', params.model);
    if (params?.status) q.set('status', params.status);
    if (params?.provider) q.set('provider', params.provider);
    if (params?.from) q.set('from', params.from);
    if (params?.to) q.set('to', params.to);
    const qs = q.toString();
    return request<{ logs: UsageLog[]; total: number; page: number; page_size: number; pages: number }>('GET', `/admin/logs${qs ? '?' + qs : ''}`);
  },

  orders: (params?: { page?: number; page_size?: number; user_id?: string; status?: string; channel?: string }) => {
    const q = new URLSearchParams();
    if (params?.page) q.set('page', String(params.page));
    if (params?.page_size) q.set('page_size', String(params.page_size));
    if (params?.user_id) q.set('user_id', params.user_id);
    if (params?.status) q.set('status', params.status);
    if (params?.channel) q.set('channel', params.channel);
    const qs = q.toString();
    return request<{ orders: Order[]; total: number; page: number; page_size: number; pages: number }>('GET', `/admin/orders${qs ? '?' + qs : ''}`);
  },

  providers: () =>
    request<{ providers: ProviderInfo[]; models: ModelInfo[] }>('GET', '/admin/providers'),

  // RBAC
  me: () => request<AdminMeResponse>('GET', '/admin/me'),

  roles: () => request<{ roles: Role[] }>('GET', '/admin/roles'),

  getRole: (id: string) => request<{ role: Role; users: User[] }>('GET', `/admin/roles/${id}`),

  permissions: () => request<{ permissions: Permission[] }>('GET', '/admin/permissions'),

  assignRole: (userId: string, roleId: string) =>
    request<{ message: string }>('POST', `/admin/users/${userId}/roles`, { role_id: roleId }),

  revokeRole: (userId: string, roleId: string) =>
    request<{ message: string }>('DELETE', `/admin/users/${userId}/roles/${roleId}`),
};
