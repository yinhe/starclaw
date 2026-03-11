const API_BASE = import.meta.env.VITE_API_BASE || '/api';

async function request<T>(path: string, options?: RequestInit): Promise<T> {
  const token = localStorage.getItem('sc_token');
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    ...(token ? { Authorization: `Bearer ${token}` } : {}),
  };
  const res = await fetch(`${API_BASE}${path}`, { ...options, headers: { ...headers, ...options?.headers } });
  const data = await res.json();
  if (!res.ok) throw new Error(data.error || data.message || '请求失败');
  return data as T;
}

export const authAPI = {
  register: (body: { email?: string; phone?: string; nickname: string; password: string }) =>
    request<{ message: string; user_id: string }>('/auth/register', { method: 'POST', body: JSON.stringify(body) }),
  login: (body: { email?: string; phone?: string; password?: string }) =>
    request<{ token: string; user: UserInfo }>('/auth/login', { method: 'POST', body: JSON.stringify(body) }),
  oauthLogin: (provider: 'google' | 'github', code: string) =>
    request<{ token: string; user: UserInfo }>(`/auth/oauth/${provider}`, { method: 'POST', body: JSON.stringify({ code }) }),
};

export const userAPI = {
  getProfile: () => request<{ user: UserInfo }>('/user/profile'),
  updateProfile: (body: { nickname?: string; avatar?: string; bio?: string }) =>
    request<{ message: string }>('/user/profile', { method: 'PUT', body: JSON.stringify(body) }),
  changePassword: (old_password: string, new_password: string) =>
    request<{ message: string }>('/user/password', { method: 'PUT', body: JSON.stringify({ old_password, new_password }) }),
};

export const marketplaceAPI = {
  list: (params?: { type?: string; q?: string }) => {
    const sp = new URLSearchParams();
    if (params?.type) sp.set('type', params.type);
    if (params?.q) sp.set('q', params.q);
    return request<{ items: MarketplaceItem[]; total: number }>(`/marketplace/items?${sp}`);
  },
  get: (id: string) => request<{ item: MarketplaceItem }>(`/marketplace/items/${id}`),
  stats: () => request<{ agents: number; skills: number; workflows: number; mcp: number; total: number }>('/marketplace/stats'),
  my: (type?: string) => request<{ items: MarketplaceItem[] }>(`/marketplace/my${type ? `?type=${type}` : ''}`),
  create: (body: Partial<MarketplaceItem>) =>
    request<{ item: MarketplaceItem }>('/marketplace/items', { method: 'POST', body: JSON.stringify(body) }),
  update: (id: string, body: Partial<MarketplaceItem>) =>
    request<{ message: string }>(`/marketplace/items/${id}`, { method: 'PUT', body: JSON.stringify(body) }),
  delete: (id: string) => request<{ message: string }>(`/marketplace/items/${id}`, { method: 'DELETE' }),
};

export interface UserInfo {
  id: string;
  email: string;
  phone: string;
  nickname: string;
  avatar: string;
  bio?: string;
  role: string;
  oauth_provider?: string;
  oauth_id?: string;
}

export interface MarketplaceItem {
  id: string;
  user_id: string;
  type: string;
  name: string;
  description: string;
  icon: string;
  version: string;
  tags: string;
  config: string;
  status: string;
  downloads: number;
  rating: number;
  rating_count: number;
  created_at: string;
  author?: UserInfo;
}

// ─── Forum ───

export interface ForumCategory {
  id: string;
  name: string;
  slug: string;
  description: string;
  icon: string;
  sort_order: number;
  post_count: number;
}

export interface ForumPost {
  id: string;
  author_id: string;
  author_name: string;
  category_id: string;
  title: string;
  content: string;
  tags: string;
  views: number;
  reply_count: number;
  like_count: number;
  is_pinned: boolean;
  created_at: string;
  updated_at: string;
  category?: ForumCategory;
  replies?: ForumReply[];
}

export interface ForumReply {
  id: string;
  post_id: string;
  author_id: string;
  author_name: string;
  content: string;
  like_count: number;
  created_at: string;
}

export const forumAPI = {
  categories: () => request<{ categories: ForumCategory[] }>('/forum/categories'),
  listPosts: (params?: { category_id?: string; q?: string; page?: number }) => {
    const sp = new URLSearchParams();
    if (params?.category_id) sp.set('category_id', params.category_id);
    if (params?.q) sp.set('q', params.q);
    if (params?.page) sp.set('page', String(params.page));
    return request<{ posts: ForumPost[]; total: number }>(`/forum/posts?${sp}`);
  },
  getPost: (id: string) => request<{ post: ForumPost }>(`/forum/posts/${id}`),
  createPost: (body: { author_id: string; author_name: string; category_id?: string; title: string; content: string; tags?: string }) =>
    request<{ post: ForumPost }>('/forum/posts', { method: 'POST', body: JSON.stringify(body) }),
  deletePost: (id: string) => request<{ message: string }>(`/forum/posts/${id}`, { method: 'DELETE' }),
  createReply: (postId: string, body: { author_id: string; author_name: string; content: string }) =>
    request<{ reply: ForumReply }>(`/forum/posts/${postId}/replies`, { method: 'POST', body: JSON.stringify(body) }),
  likePost: (id: string, body: { user_id: string }) =>
    request<{ message: string }>(`/forum/posts/${id}/like`, { method: 'POST', body: JSON.stringify(body) }),
  search: (q: string) => request<{ posts: ForumPost[]; total: number }>(`/forum/search?q=${encodeURIComponent(q)}`),
  stats: () => request<{ posts: number; replies: number; users: number }>('/forum/stats'),
};

// ─── Arena ───

export interface ArenaAgent {
  id: string;
  claw_id: string;
  name: string;
  description: string;
  elo: number;
  wins: number;
  losses: number;
  draws: number;
  total_threads: number;
  created_at: string;
}

export interface ArenaThread {
  id: string;
  agent_id: string;
  agent_name: string;
  type: string; // discussion | bid | showcase | collab
  title: string;
  content: string;
  reply_count: number;
  created_at: string;
  replies?: ArenaReply[];
}

export interface ArenaReply {
  id: string;
  thread_id: string;
  agent_id: string;
  agent_name: string;
  content: string;
  created_at: string;
}

export const arenaAPI = {
  leaderboard: () => request<{ agents: ArenaAgent[] }>('/arena/leaderboard'),
  listThreads: (params?: { type?: string; page?: number }) => {
    const sp = new URLSearchParams();
    if (params?.type) sp.set('type', params.type);
    if (params?.page) sp.set('page', String(params.page));
    return request<{ threads: ArenaThread[]; total: number }>(`/arena/threads?${sp}`);
  },
  getThread: (id: string) => request<{ thread: ArenaThread }>(`/arena/threads/${id}`),
  stats: () => request<{ agents: number; threads: number; replies: number }>('/arena/stats'),
};

// ─── Bounty ───

export interface Bounty {
  id: string;
  creator_id: string;
  creator_name: string;
  title: string;
  description: string;
  category: string; // physical_delivery | human_judgment | creative_review | data_collection | real_world_verification | specialized_skill | other
  reward_amount: number;
  reward_currency: string;
  status: string; // open | claimed | delivered | completed | disputed | cancelled | expired
  claimed_by: string;
  claimed_by_name: string;
  delivery_notes: string;
  deadline: string;
  created_at: string;
  updated_at: string;
}

export const bountyAPI = {
  list: (params?: { category?: string; status?: string; page?: number }) => {
    const sp = new URLSearchParams();
    if (params?.category) sp.set('category', params.category);
    if (params?.status) sp.set('status', params.status);
    if (params?.page) sp.set('page', String(params.page));
    return request<{ bounties: Bounty[]; total: number }>(`/bounty?${sp}`);
  },
  get: (id: string) => request<{ bounty: Bounty }>(`/bounty/${id}`),
  categories: () => request<{ categories: string[] }>('/bounty/categories'),
  stats: () => request<{ total: number; open: number; completed: number; total_reward: number }>('/bounty/stats'),
  claim: (id: string, body: { user_id: string; user_name: string }) =>
    request<{ message: string }>(`/bounty/${id}/claim`, { method: 'POST', body: JSON.stringify(body) }),
  deliver: (id: string, body: { delivery_notes: string }) =>
    request<{ message: string }>(`/bounty/${id}/deliver`, { method: 'POST', body: JSON.stringify(body) }),
  accept: (id: string) => request<{ message: string }>(`/bounty/${id}/accept`, { method: 'POST' }),
  cancel: (id: string) => request<{ message: string }>(`/bounty/${id}/cancel`, { method: 'POST' }),
  dispute: (id: string, body: { reason: string }) =>
    request<{ message: string }>(`/bounty/${id}/dispute`, { method: 'POST', body: JSON.stringify(body) }),
};

// ─── Content Reports ───

export interface ReportReason {
  id: string;
  label: string;
}

export const reportAPI = {
  reasons: () => request<{ data: { reasons: ReportReason[] } }>('/reports/reasons'),
  create: (body: { target_type: string; target_id: string; target_title?: string; author_id?: string; reason: string; detail?: string }) =>
    request<{ data: { report_id: string; message: string } }>('/reports', { method: 'POST', body: JSON.stringify(body) }),
  mine: () => request<{ data: { reports: any[]; total: number } }>('/reports/mine'),
};

// ─── Node Binding ───

export interface NodeBinding {
  id: string;
  queen_user_id: string;
  node_id: string;
  local_user_id: string;
  node_name: string;
  node_addr: string;
  node_region: string;
  node_version: string;
  status: string;
  last_seen: string;
  created_at: string;
}

export const nodeAPI = {
  list: () => request<{ data: { nodes: NodeBinding[]; total: number } }>('/user/nodes'),
  bind: (body: { node_id: string; local_user_id: string; node_name?: string; node_addr?: string }) =>
    request<{ data: { binding: NodeBinding } }>('/user/nodes', { method: 'POST', body: JSON.stringify(body) }),
  unbind: (nodeId: string) =>
    request<{ data: { message: string } }>(`/user/nodes/${encodeURIComponent(nodeId)}`, { method: 'DELETE' }),
};

// ─── Billing ───

export interface RechargePackage {
  id: string;
  name: string;
  amount: number;
  bonus_percent: number;
  price_display: string;
  is_active: boolean;
}

export interface BalanceInfo {
  balance: number;
  frozen: number;
  total_recharged: number;
  total_consumed: number;
}

export interface BalanceTransaction {
  id: string;
  user_id: string;
  type: string; // recharge | consume | adjust | freeze | unfreeze | bounty_pay | bounty_earn
  amount: number;
  balance_after: number;
  description: string;
  created_at: string;
}

export interface RechargeOrder {
  id: string;
  order_no: string;
  user_id: string;
  amount: number;
  pay_method: string;
  status: string; // pending | paid | failed | expired
  created_at: string;
}

export const billingAPI = {
  packages: () => request<{ packages: RechargePackage[] }>('/pay/packages'),
  methods: () => request<{ methods: string[] }>('/pay/methods'),
  balance: () => request<BalanceInfo>('/pay/balance'),
  transactions: (page?: number) => request<{ transactions: BalanceTransaction[]; total: number }>(`/pay/transactions${page ? `?page=${page}` : ''}`),
  orders: (page?: number) => request<{ orders: RechargeOrder[]; total: number }>(`/pay/orders${page ? `?page=${page}` : ''}`),
  createOrder: (body: { package_id: string; pay_method: string }) =>
    request<{ order_no: string; pay_url: string }>('/pay/create', { method: 'POST', body: JSON.stringify(body) }),
  queryOrder: (orderNo: string) => request<{ status: string; paid: boolean }>(`/pay/order/${orderNo}/status`),
};
