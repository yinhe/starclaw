const BASE = ''

async function request<T>(url: string, opts?: RequestInit): Promise<T> {
  const res = await fetch(BASE + url, {
    headers: { 'Content-Type': 'application/json', ...opts?.headers },
    ...opts,
  })
  if (!res.ok) {
    const text = await res.text().catch(() => res.statusText)
    throw new Error(`${res.status}: ${text}`)
  }
  return res.json()
}

// ── Health & Stats ───────────────────────────────────
export const getHealth = () => request<any>('/health')
export const getStats = () => request<any>('/stats')

// ── Campaigns ────────────────────────────────────────
export const getCampaigns = (status?: string) =>
  request<any>(`/campaign${status ? `?status=${status}` : ''}`)
export const getCampaign = (id: number) => request<any>(`/campaign/${id}`)
export const createCampaign = (data: any) =>
  request<any>('/campaign', { method: 'POST', body: JSON.stringify(data) })
export const startCampaign = (id: number, data: any) =>
  request<any>(`/campaign/${id}/start`, { method: 'POST', body: JSON.stringify(data) })
export const pauseCampaign = () =>
  request<any>('/campaign/pause', { method: 'POST' })
export const stopCampaign = () =>
  request<any>('/campaign/stop', { method: 'POST' })
export const getCampaignProgress = () => request<any>('/campaign/progress')

// ── Customers ────────────────────────────────────────
export const getCustomers = (params: Record<string, any> = {}) => {
  const qs = new URLSearchParams()
  Object.entries(params).forEach(([k, v]) => { if (v != null && v !== '') qs.set(k, String(v)) })
  return request<any>(`/customers?${qs}`)
}
export const getCustomer = (id: number) => request<any>(`/customers/${id}`)
export const updateCustomer = (id: number, data: any) =>
  request<any>(`/customers/${id}`, { method: 'PUT', body: JSON.stringify(data) })

// ── Call Records ─────────────────────────────────────
export const getCalls = (params: Record<string, any> = {}) => {
  const qs = new URLSearchParams()
  Object.entries(params).forEach(([k, v]) => { if (v != null && v !== '') qs.set(k, String(v)) })
  return request<any>(`/calls?${qs}`)
}
export const getCall = (id: number) => request<any>(`/calls/${id}`)

// ── Active Calls ─────────────────────────────────────
export const getActiveCalls = () => request<any>('/call/active')

// ── Scripts ──────────────────────────────────────────
export const getScripts = (industry?: string) =>
  request<any>(`/scripts${industry ? `?industry=${industry}` : ''}`)
export const getBuiltinScripts = () => request<any>('/scripts/builtin')
export const getBuiltinScript = (industry: string) => request<any>(`/scripts/builtin/${industry}`)
export const createScript = (data: any) =>
  request<any>('/scripts', { method: 'POST', body: JSON.stringify(data) })

// ── Analytics ────────────────────────────────────────
export const getOverview = (campaignId?: number) =>
  request<any>(`/analytics/overview${campaignId ? `?campaign_id=${campaignId}` : ''}`)
export const getIntentLevels = () => request<any>('/analytics/intent-levels')

// ── Recordings ───────────────────────────────────────
export const getRecordings = (date?: string) =>
  request<any>(`/recordings${date ? `?date=${date}` : ''}`)
export const getRecordingsUsage = () => request<any>('/recordings/usage')

// ── Compliance ───────────────────────────────────────
export const getComplianceStats = () => request<any>('/compliance/stats')
