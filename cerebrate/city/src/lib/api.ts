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

// Claw Auth
export const clawAuth = {
  challenge: () =>
    request<{ challenge: string; expires_in: number }>('POST', '/auth/claw/challenge'),
  verify: (body: { challenge: string; node_id: string; public_key: string; signature: string }) =>
    request<{ token: string; user: { id: string; role: string; claw_id: string } }>(
      'POST', '/auth/claw/verify', body
    ),
};

// Helper: call a Claw node's API (cross-origin)
export async function clawNodeRequest<T>(clawUrl: string, path: string, options?: RequestInit): Promise<T> {
  const headers: Record<string, string> = { 'Content-Type': 'application/json', ...options?.headers as Record<string, string> };
  const res = await fetch(`${clawUrl}${path}`, { ...options, headers });
  const data = await res.json();
  if (!res.ok) throw new Error(data.error || '连接 Claw 节点失败');
  return data as T;
}

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

export interface CityInvite {
  id: string; code: string; alias: string; display_code: string;
  type: string; creator_id: string; creator_type: string; creator_name: string;
  label: string; max_uses: number; used_count: number;
  region: string; comm_rate: number;
  preset_name: string; preset_phone: string; preset_email: string;
  expires_at: string | null; status: string;
  join_url: string; created_at: string; updated_at: string;
}

export interface ClientStat {
  id: string;
  client_name: string;
  user_id: string;
  status: string;
  total_recharge: number;
  month_recharge: number;
  total_energy: number;
  month_energy: number;
  energy_balance: number;
  last_active: string;
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
      total_recharge: number;
      month_recharge: number;
      total_energy: number;
      month_energy: number;
      ref_url: string;
    }>('GET', '/city/dashboard'),

  listClients: (status?: string) =>
    request<{ clients: Client[] }>('GET', `/city/clients${status ? `?status=${status}` : ''}`),

  addClient: (data: { client_name: string; contact_info?: string; plan?: string }) =>
    request<{ client: Client }>('POST', '/city/clients', data),

  updateClient: (id: string, data: { client_name?: string; contact_info?: string; plan?: string; status?: string }) =>
    request<{ client: Client }>('PUT', `/city/clients/${id}`, data),

  clientStats: () =>
    request<{
      clients: ClientStat[];
      total_clients: number;
      total_recharge: number;
      month_recharge: number;
      total_energy: number;
      month_energy: number;
    }>('GET', '/city/client-stats'),

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

  listInvites: () =>
    request<{ invites: CityInvite[]; total: number }>('GET', '/city/invites'),

  createInvite: (data: { alias?: string; label?: string; max_uses?: number }) =>
    request<{ invite: CityInvite }>('POST', '/city/invites', data),

  revokeInvite: (id: string) =>
    request<{ message: string }>('DELETE', `/city/invites/${id}`),
};
