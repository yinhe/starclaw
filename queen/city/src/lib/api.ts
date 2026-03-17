const BASE = '/api';

function getToken(): string | null {
  return localStorage.getItem('city_token');
}

export function setToken(token: string) {
  localStorage.setItem('city_token', token);
}

export function clearToken() {
  localStorage.removeItem('city_token');
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
  if (!res.ok) throw new Error(data.message || data.error || `HTTP ${res.status}`);
  return data as T;
}

// Auth
export const auth = {
  login: (data: { email?: string; phone?: string; password: string }) =>
    request<{ token: string; user: { id: string; email: string; nickname: string; role: string } }>(
      'POST', '/auth/login', data
    ),
};

// City Partner Portal
export interface Partner {
  id: string;
  name: string;
  company: string;
  city: string;
  phone: string;
  email: string;
  ref_code: string;
  comm_rate: number;
  status: string;
  total_earned: number;
  total_clients: number;
  created_at: string;
}

export interface Client {
  id: string;
  partner_id: string;
  client_name: string;
  contact_info: string;
  plan: string;
  status: string;
  mrr: number;
  signed_at: string | null;
  renew_at: string | null;
  created_at: string;
}

export interface CommissionItem {
  id: string;
  client_id: string;
  order_no: string;
  type: string;
  amount: number;
  rate: number;
  base_amount: number;
  status: string;
  month: string;
  created_at: string;
}

export interface PayoutItem {
  id: string;
  amount: number;
  method: string;
  account: string;
  status: string;
  month: string;
  invoice_url: string;
  paid_at: string | null;
  created_at: string;
}

export interface Material {
  id: string;
  title: string;
  category: string;
  description: string;
  file_url: string;
  file_size: number;
  created_at: string;
}

export interface MonthlySummary {
  month: string;
  total: number;
  count: number;
}

export const city = {
  dashboard: () =>
    request<{
      partner: Partner;
      month: string;
      month_commission: number;
      month_new_clients: number;
      total_clients: number;
      active_clients: number;
      total_earned: number;
      pending_commission: number;
      ref_url: string;
    }>('GET', '/city/dashboard'),

  listClients: (status?: string) =>
    request<{ clients: Client[] }>('GET', `/city/clients${status ? `?status=${status}` : ''}`),

  addClient: (data: { client_name: string; contact_info?: string; plan?: string }) =>
    request<{ client: Client }>('POST', '/city/clients', data),

  updateClient: (id: string, data: { client_name?: string; contact_info?: string; plan?: string; status?: string }) =>
    request<{ client: Client }>('PUT', `/city/clients/${id}`, data),

  listCommissions: (month?: string) =>
    request<{ commissions: CommissionItem[]; monthly_summary: MonthlySummary[] }>(
      'GET', `/city/commissions${month ? `?month=${month}` : ''}`
    ),

  listPayouts: () =>
    request<{ payouts: PayoutItem[] }>('GET', '/city/payouts'),

  listMaterials: (category?: string) =>
    request<{ materials: Material[] }>('GET', `/city/materials${category ? `?category=${category}` : ''}`),

  refLink: () =>
    request<{ ref_code: string; ref_url: string; utm_url: string }>('GET', '/city/ref-link'),
};
