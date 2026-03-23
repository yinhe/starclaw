import { useEffect, useState } from 'react'
import { GitBranch, GitPullRequest, Globe, Lock, Shield, Server, RefreshCw } from 'lucide-react'
import { getStoredToken } from '../api/brood'

const BASE = '/brood'

async function forgeAPI<T = any>(path: string): Promise<T> {
  const token = getStoredToken()
  const headers: Record<string, string> = { 'Content-Type': 'application/json' }
  if (token) headers['X-Admin-Token'] = token
  const res = await fetch(`${BASE}/forge${path}`, { headers })
  if (!res.ok) throw new Error(`${res.status}`)
  return res.json()
}

interface Repo {
  id: string
  name: string
  description: string
  owner: string
  team_id: string
  public: boolean
  source: string
  forked_from: string
  initialized: boolean
  branches: number
  tags: number
  commit_count: number
  targets: number
  ssh_url: string
  https_url: string
  head: string
  last_commit: any
  created_at: string
}

interface NydusNode {
  id: string
  node_id: string
  name: string
  role: string
  team_id: string
  registered_at: string
  last_seen_at: string
}

interface PR {
  id: string
  number: number
  title: string
  status: string
  source_branch: string
  target_branch: string
  author_node_id: string
  created_at: string
}

const statusBadge: Record<string, string> = {
  open: 'bg-emerald-600/10 text-emerald-400 border-emerald-600/20',
  merged: 'bg-purple-600/10 text-purple-400 border-purple-600/20',
  closed: 'bg-gray-600/10 text-gray-400 border-gray-600/20',
}

export default function ForgePage() {
  const [repos, setRepos] = useState<Repo[]>([])
  const [nodes, setNodes] = useState<NydusNode[]>([])
  const [selectedRepo, setSelectedRepo] = useState<string | null>(null)
  const [prs, setPRs] = useState<PR[]>([])
  const [loading, setLoading] = useState(true)
  const [tab, setTab] = useState<'repos' | 'nodes'>('repos')

  const load = async () => {
    setLoading(true)
    try {
      const data = await forgeAPI('/summary')
      const repoData = (data as any)?.repos
      const nodeData = (data as any)?.nodes
      setRepos(repoData?.repos || [])
      setNodes(nodeData?.nodes || [])
    } catch { /* */ }
    finally { setLoading(false) }
  }

  const loadPRs = async (repoName: string) => {
    setSelectedRepo(repoName)
    try {
      const data = await forgeAPI(`/repos/${repoName}/pulls?status=all`)
      setPRs((data as any)?.pull_requests || [])
    } catch { setPRs([]) }
  }

  useEffect(() => { load() }, [])

  const totalCommits = repos.reduce((s, r) => s + (r.commit_count || 0), 0)
  const totalBranches = repos.reduce((s, r) => s + (r.branches || 0), 0)
  const publicCount = repos.filter(r => r.public).length

  return (
    <div className="p-8">
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-2xl font-bold text-white">Forge — 代码协作</h1>
          <p className="text-sm text-gray-500 mt-1">Nydus 仓库、Pull Requests、节点注册总览</p>
        </div>
        <button onClick={load} className="flex items-center gap-2 px-3 py-2 bg-gray-800 text-gray-300 rounded-lg hover:bg-gray-700 text-sm">
          <RefreshCw className="w-4 h-4" /> 刷新
        </button>
      </div>

      {/* Stats */}
      <div className="grid grid-cols-5 gap-4 mb-6">
        {[
          { label: '仓库', value: repos.length, icon: GitBranch, color: 'text-blue-400' },
          { label: '注册节点', value: nodes.length, icon: Server, color: 'text-emerald-400' },
          { label: '总提交', value: totalCommits, icon: GitPullRequest, color: 'text-purple-400' },
          { label: '总分支', value: totalBranches, icon: GitBranch, color: 'text-orange-400' },
          { label: '公开仓库', value: publicCount, icon: Globe, color: 'text-cyan-400' },
        ].map(s => (
          <div key={s.label} className="bg-gray-900 border border-gray-800 rounded-xl p-4">
            <div className="flex items-center gap-2 mb-2">
              <s.icon className={`w-4 h-4 ${s.color}`} />
              <span className="text-xs text-gray-500">{s.label}</span>
            </div>
            <div className="text-2xl font-bold text-white">{s.value}</div>
          </div>
        ))}
      </div>

      {/* Tabs */}
      <div className="flex gap-1 mb-4">
        {(['repos', 'nodes'] as const).map(t => (
          <button key={t} onClick={() => setTab(t)}
            className={`px-4 py-2 rounded-lg text-sm font-medium transition ${tab === t ? 'bg-overlord-600/15 text-overlord-300' : 'text-gray-400 hover:bg-gray-800'}`}>
            {t === 'repos' ? '仓库列表' : '注册节点'}
          </button>
        ))}
      </div>

      {loading ? (
        <div className="text-center text-gray-500 py-12">加载中…</div>
      ) : tab === 'repos' ? (
        <div className="grid grid-cols-1 gap-3">
          {repos.map(repo => (
            <div key={repo.name} className="bg-gray-900 border border-gray-800 rounded-xl p-4 hover:border-gray-700 transition">
              <div className="flex items-center justify-between mb-2">
                <div className="flex items-center gap-3">
                  <GitBranch className="w-5 h-5 text-blue-400" />
                  <button onClick={() => loadPRs(repo.name)} className="text-lg font-semibold text-white hover:text-overlord-400 transition">
                    {repo.name}
                  </button>
                  {repo.public ? (
                    <span className="flex items-center gap-1 text-xs px-2 py-0.5 rounded-full bg-cyan-600/10 text-cyan-400 border border-cyan-600/20">
                      <Globe className="w-3 h-3" /> 公开
                    </span>
                  ) : (
                    <span className="flex items-center gap-1 text-xs px-2 py-0.5 rounded-full bg-gray-600/10 text-gray-400 border border-gray-600/20">
                      <Lock className="w-3 h-3" /> 私有
                    </span>
                  )}
                  {repo.source === 'system' && (
                    <span className="text-xs px-2 py-0.5 rounded-full bg-yellow-600/10 text-yellow-400 border border-yellow-600/20">system</span>
                  )}
                </div>
                <div className="flex items-center gap-4 text-xs text-gray-500">
                  <span>HEAD: <code className="text-gray-400">{repo.head || '—'}</code></span>
                  <span>{repo.branches || 0} 分支</span>
                  <span>{repo.tags || 0} 标签</span>
                  <span>{repo.commit_count || 0} 提交</span>
                  {repo.targets > 0 && <span className="text-emerald-400">{repo.targets} 部署目标</span>}
                </div>
              </div>
              {repo.description && <p className="text-sm text-gray-500 ml-8">{repo.description}</p>}
              <div className="ml-8 mt-1 text-xs text-gray-600">
                <code>{repo.ssh_url}</code>
              </div>

              {/* PRs panel */}
              {selectedRepo === repo.name && (
                <div className="mt-3 ml-8 border-t border-gray-800 pt-3">
                  <h4 className="text-sm font-medium text-gray-400 mb-2">Pull Requests</h4>
                  {prs.length === 0 ? (
                    <p className="text-xs text-gray-600">暂无 PR</p>
                  ) : (
                    <div className="space-y-1">
                      {prs.map(pr => (
                        <div key={pr.id} className="flex items-center justify-between py-1.5 px-3 bg-gray-800/50 rounded-lg">
                          <div className="flex items-center gap-2">
                            <GitPullRequest className="w-3.5 h-3.5 text-purple-400" />
                            <span className="text-sm text-white">#{pr.number} {pr.title}</span>
                          </div>
                          <div className="flex items-center gap-3 text-xs">
                            <code className="text-gray-500">{pr.source_branch} → {pr.target_branch}</code>
                            <span className={`px-2 py-0.5 rounded-full border ${statusBadge[pr.status] || statusBadge.closed}`}>
                              {pr.status}
                            </span>
                          </div>
                        </div>
                      ))}
                    </div>
                  )}
                </div>
              )}
            </div>
          ))}
          {repos.length === 0 && <p className="text-center text-gray-600 py-8">Nydus 未连接或无仓库</p>}
        </div>
      ) : (
        /* Nodes tab */
        <div className="bg-gray-900 border border-gray-800 rounded-xl overflow-hidden">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-gray-800 text-gray-500 text-left">
                <th className="px-4 py-3 font-medium">节点 ID</th>
                <th className="px-4 py-3 font-medium">名称</th>
                <th className="px-4 py-3 font-medium">角色</th>
                <th className="px-4 py-3 font-medium">团队</th>
                <th className="px-4 py-3 font-medium">注册时间</th>
                <th className="px-4 py-3 font-medium">最近活跃</th>
              </tr>
            </thead>
            <tbody>
              {nodes.map(n => (
                <tr key={n.id} className="border-b border-gray-800/50 hover:bg-gray-800/30">
                  <td className="px-4 py-3">
                    <div className="flex items-center gap-2">
                      <Shield className="w-3.5 h-3.5 text-overlord-400" />
                      <code className="text-xs text-gray-300">{n.node_id}</code>
                    </div>
                  </td>
                  <td className="px-4 py-3 text-white">{n.name || '—'}</td>
                  <td className="px-4 py-3">
                    <span className={`px-2 py-0.5 rounded-full text-xs ${n.role === 'owner' ? 'bg-purple-600/10 text-purple-400' : n.role === 'admin' ? 'bg-blue-600/10 text-blue-400' : 'bg-gray-600/10 text-gray-400'}`}>
                      {n.role}
                    </span>
                  </td>
                  <td className="px-4 py-3 text-gray-500 text-xs">{n.team_id || '—'}</td>
                  <td className="px-4 py-3 text-gray-500 text-xs">{n.registered_at ? new Date(n.registered_at).toLocaleDateString() : '—'}</td>
                  <td className="px-4 py-3 text-gray-500 text-xs">{n.last_seen_at ? new Date(n.last_seen_at).toLocaleString() : '从未'}</td>
                </tr>
              ))}
              {nodes.length === 0 && (
                <tr><td colSpan={6} className="text-center py-8 text-gray-600">暂无注册节点</td></tr>
              )}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}
