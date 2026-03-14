import axios from 'axios'

// ── Auth state ──
const AUTH_KEY = 'nydus_secret'

export function getSecret(): string {
  return localStorage.getItem(AUTH_KEY) || ''
}

export function setSecret(secret: string) {
  if (secret) localStorage.setItem(AUTH_KEY, secret)
  else localStorage.removeItem(AUTH_KEY)
}

export function isAuthenticated(): boolean {
  return !!getSecret()
}

// ── Axios instances ──
// Public: /v1/* (no auth, only public repos)
const pubAPI = axios.create({ baseURL: '/v1' })

// Private: /api/* (with X-Nydus-Secret, all repos)
const privAPI = axios.create({ baseURL: '/api' })
privAPI.interceptors.request.use((cfg) => {
  const secret = getSecret()
  if (secret) cfg.headers['X-Nydus-Secret'] = secret
  return cfg
})

// Pick the right client based on auth state
function api() { return isAuthenticated() ? privAPI : pubAPI }

// Verify a secret is valid by calling /api/repos
export async function verifySecret(secret: string): Promise<boolean> {
  try {
    await axios.get('/api/repos', { headers: { 'X-Nydus-Secret': secret } })
    return true
  } catch {
    return false
  }
}

export interface LastCommit {
  hash: string
  short_hash: string
  message: string
  author: string
  time_ago: string
  date?: string
}

export interface Repo {
  name: string
  description: string
  targets: number
  initialized: boolean
  ssh_url: string
  https_url?: string
  head?: string
  branches?: number
  tags?: number
  commit_count?: number
  last_commit?: LastCommit
}

export interface Commit {
  hash: string
  message: string
  time: string
  author?: string
}

export interface Deploy {
  repo: string
  branch: string
  rev: string
  target: string
  status: string
  message: string
  timestamp: string
}

export interface Release {
  tag_name: string
  name: string
  body: string
  html_url: string
  commit: string
  source: string
  source_url: string
  git_clone: string
}

export interface TreeItem {
  name: string
  type: 'tree' | 'blob'
  size?: string
  message: string
  time_ago: string
}

export interface Branch {
  name: string
  head: string
  updated: string
}

export interface Tag {
  name: string
  hash: string
  date: string
}

export interface ReleaseItem {
  tag_name: string
  name: string
  body: string
  commit: string
  html_url: string
  latest: boolean
}

export interface ServerStats {
  repos: number
  targets: number
  total_commits: number
  total_tags: number
}

export const nydusAPI = {
  repos: () => api().get<{ repos: Repo[] }>('/repos'),
  repo: (name: string) => api().get<Repo>(`/repos/${name}`),
  repoTree: (name: string, path = '', ref = 'HEAD') =>
    api().get<{ items: TreeItem[]; path: string; ref: string }>(`/repos/${name}/tree`, { params: { path, ref } }),
  repoReadme: (name: string) =>
    api().get<{ name: string; content: string }>(`/repos/${name}/readme`),
  repoBranches: (name: string) =>
    api().get<{ branches: Branch[] }>(`/repos/${name}/branches`),
  repoTags: (name: string) =>
    api().get<{ tags: Tag[]; count: number }>(`/repos/${name}/tags`),
  commits: (repo = 'claw', limit = 20) =>
    api().get<{ commits: Commit[] }>('/commits', { params: { repo, limit } }),
  deploys: () => api().get<{ deploys: Deploy[] }>('/deploys'),
  release: () => pubAPI.get<Release>('/releases/latest'),
  releases: () => api().get<{ releases: ReleaseItem[] }>('/releases'),
  stats: () => api().get<ServerStats>('/stats'),
}

export const healthAPI = {
  check: () => axios.get('/health'),
}
