const BASE = '/api';

function getToken(): string | null {
  return localStorage.getItem('partner_token');
}

export function setToken(token: string) {
  localStorage.setItem('partner_token', token);
}

export function clearToken() {
  localStorage.removeItem('partner_token');
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

// Types
export interface CorePartner {
  id: string; name: string; phone: string; email: string; region: string;
  level: string; status: string; base_salary: number;
  direct_comm_rate: number; manage_fee_rate: number;
  total_revenue: number; total_commission: number;
  active_clients: number; managed_cities: number;
  joined_at: string;
}

export interface CRMDeal {
  id: string; partner_id: string; company_name: string; contact_name: string;
  contact_info: string; industry: string; stage: string; deal_value: number;
  plan: string; source: string; priority: string; notes: string;
  next_action: string; next_date: string | null;
  signed_at: string | null; delivered_at: string | null; renew_at: string | null;
  created_at: string; updated_at: string;
}

export interface PartnerComm {
  id: string; deal_id: string; city_id: string; type: string;
  amount: number; rate: number; base_amount: number;
  month: string; status: string; remark: string; created_at: string;
}

export interface EquityGrant {
  id: string; total_shares: number; vested_shares: number;
  cliff_months: number; vesting_months: number;
  grant_date: string; cliff_date: string; full_vest_date: string;
  strike_price: number; current_value: number; status: string;
}

export interface Deployment {
  id: string; deal_id: string; client_name: string; type: string;
  region: string; domain: string; admin_email: string; version: string;
  status: string; health_url: string; started_at: string | null; created_at: string;
}

export interface CityPartner {
  id: string; name: string; company: string; city: string; phone: string;
  email: string; ref_code: string; comm_rate: number; status: string;
  total_earned: number; total_clients: number; created_at: string;
}

interface MonthTypeBreakdown { month: string; type: string; total: number }
interface FunnelItem { stage: string; count: number; value: number }

export interface SwarmNode {
  id: string; name: string; role: string; status: string;
  version: string; address: string; region: string;
  cpu_percent: number; mem_percent: number;
  tasks_running: number; last_heartbeat: string;
}

export interface PartnerInvite {
  id: string; code: string; alias: string; display_code: string;
  type: string; creator_id: string; creator_type: string; creator_name: string;
  label: string; max_uses: number; used_count: number;
  region: string; comm_rate: number; level: string; base_salary: number;
  preset_name: string; preset_phone: string; preset_email: string;
  expires_at: string | null; status: string;
  join_url: string; created_at: string; updated_at: string;
}

// Option Pool Types
export interface OptionRoundSummary {
  round: string; total_amount: number; total_shares: number; comm_rate: number;
}

export interface OptionInfo {
  partner_id: string; partner_type: string; partner_name: string;
  effective_rate: number; option_rate: number;
  in_transition: boolean; legacy_rate: number;
  total_invested: number; total_shares: number;
  rounds: OptionRoundSummary[];
  current_round: { round: string; invested: number; max: number; remaining: number };
}

export interface PurchaseResult {
  message: string; investment_id: string; partner_id: string; partner_type: string;
  round: string; amount: number; shares: number; price: number; new_comm_rate: number;
}

// Option Pool API
export const option = {
  myOptions: () =>
    request<OptionInfo>('GET', '/partner/option/me'),

  purchase: (amount: number) =>
    request<PurchaseResult>('POST', '/partner/option/purchase', { amount }),
};

// Partner Hub API
export const partner = {
  dashboard: () =>
    request<{
      partner: CorePartner; month: string; month_commission: number;
      funnel: FunnelItem[]; urgent_actions: number; city_partners: number;
      equity: EquityGrant;
    }>('GET', '/partner/dashboard'),

  listDeals: (params?: { stage?: string; priority?: string; q?: string }) => {
    const qs = new URLSearchParams();
    if (params?.stage) qs.set('stage', params.stage);
    if (params?.priority) qs.set('priority', params.priority);
    if (params?.q) qs.set('q', params.q);
    const s = qs.toString();
    return request<{ deals: CRMDeal[] }>('GET', `/partner/deals${s ? '?' + s : ''}`);
  },

  createDeal: (data: Partial<CRMDeal>) =>
    request<{ deal: CRMDeal }>('POST', '/partner/deals', data),

  getDeal: (id: string) =>
    request<{ deal: CRMDeal }>('GET', `/partner/deals/${id}`),

  updateDeal: (id: string, data: Partial<CRMDeal>) =>
    request<{ deal: CRMDeal }>('PUT', `/partner/deals/${id}`, data),

  listCityPartners: (status?: string) =>
    request<{ city_partners: CityPartner[] }>('GET', `/partner/city-partners${status ? '?status=' + status : ''}`),

  reviewCityPartner: (id: string, data: { status: string; comm_rate?: number }) =>
    request<{ message: string }>('PUT', `/partner/city-partners/${id}`, data),

  listNodes: (params?: { role?: string; status?: string }) => {
    const qs = new URLSearchParams();
    if (params?.role) qs.set('role', params.role);
    if (params?.status) qs.set('status', params.status);
    const s = qs.toString();
    return request<{ nodes: SwarmNode[]; total: number }>('GET', `/partner/nodes${s ? '?' + s : ''}`);
  },

  listMyNodes: (params?: { status?: string }) => {
    const qs = new URLSearchParams();
    if (params?.status) qs.set('status', params.status);
    const s = qs.toString();
    return request<{ nodes: SwarmNode[]; total: number }>('GET', `/partner/nodes/my${s ? '?' + s : ''}`);
  },

  getNode: (id: string) =>
    request<SwarmNode>('GET', `/partner/nodes/${id}`),

  listCommissions: (params?: { month?: string; type?: string }) => {
    const qs = new URLSearchParams();
    if (params?.month) qs.set('month', params.month);
    if (params?.type) qs.set('type', params.type);
    const s = qs.toString();
    return request<{ commissions: PartnerComm[]; breakdown: MonthTypeBreakdown[] }>(
      'GET', `/partner/commissions${s ? '?' + s : ''}`
    );
  },

  getEquity: () =>
    request<{ grants: EquityGrant[] }>('GET', '/partner/equity'),

  listDeployments: () =>
    request<{ deployments: Deployment[] }>('GET', '/partner/deployments'),

  createDeployment: (data: { deal_id?: string; client_name: string; type: string; region?: string; domain?: string; admin_email: string }) =>
    request<{ deployment: Deployment }>('POST', '/partner/deployments', data),

  getDeployment: (id: string) =>
    request<{ deployment: Deployment }>('GET', `/partner/deployments/${id}`),

  stopDeployment: (id: string) =>
    request<{ message: string }>('POST', `/partner/deployments/${id}/stop`),

  addCityPartnerClaw: (data: { claw_id: string; name: string; company?: string; city?: string; phone?: string; email?: string }) =>
    request<{ city_partner: CityPartner }>('POST', '/partner/city-partners/claw', data),

  removeCityPartnerClaw: (id: string) =>
    request<{ message: string }>('DELETE', `/partner/city-partners/${id}/claw`),

  listInvites: () =>
    request<{ invites: PartnerInvite[]; total: number }>('GET', '/partner/invites'),

  createInvite: (data: { alias?: string; label?: string; max_uses?: number; region?: string; comm_rate?: number; preset_name?: string; preset_phone?: string; preset_email?: string; expires_at?: string }) =>
    request<{ invite: PartnerInvite }>('POST', '/partner/invites', data),

  revokeInvite: (id: string) =>
    request<{ message: string }>('DELETE', `/partner/invites/${id}`),

  inviteStats: () =>
    request<{ total_invites: number; total_uses: number; active_invites: number; conversion_rate: string }>('GET', '/partner/invite-stats'),
};
