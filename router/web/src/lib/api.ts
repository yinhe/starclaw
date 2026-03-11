const BASE = '';

function getToken(): string | null {
  return localStorage.getItem('token');
}

export function setToken(token: string) {
  localStorage.setItem('token', token);
}

export function clearToken() {
  localStorage.removeItem('token');
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

  if (res.status === 401) {
    clearToken();
    window.location.href = '/login';
    throw new Error('Unauthorized');
  }

  const data = await res.json();
  if (!res.ok) throw new Error(data.error || `HTTP ${res.status}`);
  return data as T;
}

// Auth
export const auth = {
  register: (data: { email?: string; phone?: string; password: string; name?: string }) =>
    request<{ token: string; user: { id: string; email: string; phone: string; name: string }; api_key: { key: string; key_prefix: string } }>(
      'POST', '/auth/register', data
    ),
  login: (data: { email?: string; phone?: string; password: string }) =>
    request<{ token: string; user: { id: string; email: string; phone: string; name: string } }>(
      'POST', '/auth/login', data
    ),
};

// Dashboard (JWT)
export const dash = {
  profile: () => request<{ user: { id: string; email: string; name: string; balance: number; free_quota: number; status: string; created_at: string }; api_key_count: number }>('GET', '/dash/profile'),
  keys: () => request<{ keys: { id: string; name: string; key_prefix: string; is_enabled: boolean; last_used: string | null; created_at: string }[] }>('GET', '/dash/keys'),
  createKey: (name: string) => request<{ key: string; api_key: { id: string; key_prefix: string } }>('POST', '/dash/keys', { name }),
  deleteKey: (id: string) => request<void>('DELETE', `/dash/keys/${id}`),
  usage: (days?: number) => request<{ records: { id: string; model: string; prompt_tokens: number; completion_tokens: number; total_tokens: number; cost_cents: number; created_at: string }[]; total_tokens: number; total_cost: number; total_requests: number; days: number }>('GET', `/dash/usage${days ? `?days=${days}` : ''}`),
  balance: () => request<{ balance_cents: number; free_quota: number }>('GET', '/dash/balance'),
  packages: () => request<{ packages: { id: string; name: string; amount_cents: number; bonus_cents: number; total_cents: number }[] }>('GET', '/dash/pay/packages'),
  payAlipay: (packageId: string) => request<{ order_no: string; pay_url: string }>('POST', '/dash/pay/alipay', { package_id: packageId }),
  payWechat: (packageId: string) => request<{ order_no: string }>('POST', '/dash/pay/wechat', { package_id: packageId }),
  orders: () => request<{ orders: { id: string; order_no: string; channel: string; amount_cents: number; bonus_cents: number; total_cents: number; status: string; created_at: string; paid_at: string | null }[] }>('GET', '/dash/pay/orders'),
  updateProfile: (data: { name?: string; email?: string }) => request<{ message: string }>('PUT', '/dash/profile', data),
  changePassword: (oldPassword: string, newPassword: string) => request<{ message: string }>('POST', '/dash/password', { old_password: oldPassword, new_password: newPassword }),
  logs: (params?: { page?: number; page_size?: number; model?: string; status?: string; from?: string; to?: string }) => {
    const q = new URLSearchParams();
    if (params?.page) q.set('page', String(params.page));
    if (params?.page_size) q.set('page_size', String(params.page_size));
    if (params?.model) q.set('model', params.model);
    if (params?.status) q.set('status', params.status);
    if (params?.from) q.set('from', params.from);
    if (params?.to) q.set('to', params.to);
    const qs = q.toString();
    return request<{ logs: { id: string; model: string; endpoint: string; provider: string; prompt_tokens: number; completion_tokens: number; total_tokens: number; cost_cents: number; upstream_cost: number; duration: number; status: string; error_msg: string; via: string; created_at: string }[]; total: number; page: number; page_size: number; pages: number }>('GET', `/dash/logs${qs ? '?' + qs : ''}`);
  },
};
