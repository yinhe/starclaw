import { useState, useEffect, useCallback } from 'react'
import { nydusAPI, healthAPI, isAuthenticated, setSecret, verifySecret } from './lib/api'
import type { Repo, Commit, Deploy, Release, ReleaseItem, TreeItem, ServerStats } from './lib/api'
import {
  GitBranch, Tag, Clock, Server, Download, ExternalLink,
  CheckCircle2, XCircle, Activity, RefreshCw,
  Folder, FileText, ChevronRight, GitCommit, Database,
  Rocket, ArrowLeft, Lock, Unlock, KeyRound,
} from 'lucide-react'

// ── Page type ──
type Page = { view: 'home' } | { view: 'repo'; name: string }

function parseHash(): Page {
  const h = window.location.hash.replace(/^#\/?/, '')
  if (h.startsWith('repo/')) {
    const name = h.slice(5)
    if (name) return { view: 'repo', name }
  }
  return { view: 'home' }
}

export default function App() {
  const [page, setPage] = useState<Page>(parseHash)
  const [repos, setRepos] = useState<Repo[]>([])
  const [deploys, setDeploys] = useState<Deploy[]>([])
  const [release, setRelease] = useState<Release | null>(null)
  const [releases, setReleases] = useState<ReleaseItem[]>([])
  const [healthy, setHealthy] = useState<boolean | null>(null)
  const [loading, setLoading] = useState(true)
  const [serverStats, setServerStats] = useState<ServerStats | null>(null)
  const [authed, setAuthed] = useState(isAuthenticated())
  const [showLogin, setShowLogin] = useState(false)

  // Repo-detail state
  const [commits, setCommits] = useState<Commit[]>([])
  const [treeItems, setTreeItems] = useState<TreeItem[]>([])
  const [treePath, setTreePath] = useState('')
  const [readme, setReadme] = useState('')

  const handleLogin = async (secret: string) => {
    const ok = await verifySecret(secret)
    if (ok) {
      setSecret(secret)
      setAuthed(true)
      setShowLogin(false)
      // Refresh data with new auth
      setPage({ view: 'home' })
    }
    return ok
  }

  const handleLogout = () => {
    setSecret('')
    setAuthed(false)
    setPage({ view: 'home' })
  }

  // ── Fetch home data ──
  // eslint-disable-next-line react-hooks/exhaustive-deps
  const fetchHome = useCallback(async () => {
    setLoading(true)
    try {
      const [repoRes, deployRes, healthRes, statsRes] = await Promise.allSettled([
        nydusAPI.repos(),
        nydusAPI.deploys(),
        healthAPI.check(),
        nydusAPI.stats(),
      ])
      if (repoRes.status === 'fulfilled') setRepos(repoRes.value.data.repos || [])
      if (deployRes.status === 'fulfilled') setDeploys(deployRes.value.data.deploys || [])
      if (healthRes.status === 'fulfilled') setHealthy(healthRes.value.data.status === 'ok')
      else setHealthy(false)
      if (statsRes.status === 'fulfilled') setServerStats(statsRes.value.data)
    } catch { /* ignore */ }
    try {
      const r = await nydusAPI.release()
      setRelease(r.data)
    } catch { /* no release */ }
    try {
      const rels = await nydusAPI.releases()
      setReleases(rels.data.releases || [])
    } catch { /* no releases */ }
    setLoading(false)
  }, [])

  // ── Fetch repo detail ──
  const fetchRepo = useCallback(async (name: string, path: string) => {
    setLoading(true)
    try {
      const [commitRes, treeRes, readmeRes] = await Promise.allSettled([
        nydusAPI.commits(name, 20),
        nydusAPI.repoTree(name, path),
        path === '' ? nydusAPI.repoReadme(name) : Promise.reject('skip'),
      ])
      if (commitRes.status === 'fulfilled') setCommits(commitRes.value.data.commits || [])
      if (treeRes.status === 'fulfilled') setTreeItems(treeRes.value.data.items || [])
      else setTreeItems([])
      if (readmeRes.status === 'fulfilled') setReadme(readmeRes.value.data.content || '')
      else if (path !== '') { /* keep readme from root */ }
      else setReadme('')
    } catch { /* ignore */ }
    setLoading(false)
  }, [])

  // ── Navigate ──
  const goHome = useCallback(() => {
    window.location.hash = '/'
    setPage({ view: 'home' })
    setTreePath('')
    setTreeItems([])
    setReadme('')
    setCommits([])
  }, [])

  const goRepo = useCallback((name: string) => {
    window.location.hash = `/repo/${name}`
    setPage({ view: 'repo', name })
    setTreePath('')
  }, [])

  // ── Hash change listener (browser back/forward) ──
  useEffect(() => {
    const onHash = () => {
      const p = parseHash()
      setPage(p)
      if (p.view === 'home') {
        setTreePath('')
        setTreeItems([])
        setReadme('')
        setCommits([])
      } else {
        setTreePath('')
      }
    }
    window.addEventListener('hashchange', onHash)
    return () => window.removeEventListener('hashchange', onHash)
  }, [])

  // ── Effects ──
  useEffect(() => {
    if (page.view === 'home') fetchHome()
  }, [page, fetchHome, authed])

  useEffect(() => {
    if (page.view === 'repo') fetchRepo(page.name, treePath)
  }, [page, treePath, fetchRepo, authed])

  const currentRepo = page.view === 'repo' ? repos.find(r => r.name === page.name) : null

  return (
    <div className="min-h-screen bg-nydus-bg">
      {/* ─── Header ─── */}
      <header className="border-b border-nydus-border bg-nydus-card">
        <div className="max-w-6xl mx-auto px-6 py-3 flex items-center justify-between">
          <div className="flex items-center gap-3">
            <button onClick={goHome} className="flex items-center gap-3 hover:opacity-80 transition-opacity">
              <Server className="w-5 h-5 text-nydus-accent" />
              <h1 className="text-lg font-bold">
                <span className="text-nydus-accent">Star</span>Claw Nydus
              </h1>
            </button>
            {page.view === 'repo' && (
              <>
                <span className="text-nydus-dim">/</span>
                <span className="text-nydus-text font-semibold">{page.name}</span>
              </>
            )}
          </div>
          <div className="flex items-center gap-4 text-sm">
            {healthy !== null && (
              <span className={`flex items-center gap-1.5 ${healthy ? 'text-nydus-green' : 'text-red-400'}`}>
                {healthy ? <CheckCircle2 className="w-3.5 h-3.5" /> : <XCircle className="w-3.5 h-3.5" />}
                {healthy ? 'Online' : 'Offline'}
              </span>
            )}
            {authed ? (
              <button onClick={handleLogout}
                className="flex items-center gap-1.5 text-nydus-green hover:text-red-400 transition-colors" title="Logout">
                <Unlock className="w-3.5 h-3.5" />
                <span className="hidden sm:inline">Admin</span>
              </button>
            ) : (
              <button onClick={() => setShowLogin(true)}
                className="flex items-center gap-1.5 text-nydus-muted hover:text-nydus-text transition-colors" title="Login">
                <Lock className="w-3.5 h-3.5" />
              </button>
            )}
            <button onClick={page.view === 'home' ? fetchHome : () => fetchRepo((page as { name: string }).name, treePath)}
              disabled={loading}
              className="flex items-center gap-1.5 text-nydus-muted hover:text-nydus-text transition-colors disabled:opacity-50">
              <RefreshCw className={`w-3.5 h-3.5 ${loading ? 'animate-spin' : ''}`} />
            </button>
            <a href="https://starclaw.me" className="text-nydus-muted hover:text-nydus-text transition-colors">Home</a>
            <a href="https://github.com/yinhe/starclaw" className="text-nydus-muted hover:text-nydus-text transition-colors">GitHub</a>
          </div>
        </div>
      </header>

      <main className="max-w-6xl mx-auto px-6 py-6">
        {page.view === 'home' ? (
          <HomePage
            repos={repos} deploys={deploys} release={release} releases={releases}
            serverStats={serverStats} loading={loading}
            onRepoClick={goRepo}
          />
        ) : (
          <RepoDetailPage
            repo={currentRepo || null}
            repoName={page.name}
            commits={commits} treeItems={treeItems} treePath={treePath}
            readme={readme} release={release} releases={releases} loading={loading}
            authed={authed}
            onNavigateTree={(item) => {
              if (item.type === 'tree') setTreePath(treePath ? `${treePath}/${item.name}` : item.name)
            }}
            onNavigateUp={() => {
              const parts = treePath.split('/')
              parts.pop()
              setTreePath(parts.join('/'))
            }}
            onBack={goHome}
          />
        )}
      </main>

      {/* ─── Login Modal ─── */}
      {showLogin && <LoginModal onLogin={handleLogin} onClose={() => setShowLogin(false)} />}

      {/* ─── Footer ─── */}
      <footer className="border-t border-nydus-border mt-8 py-6 text-center text-nydus-dim text-sm">
        &copy; {new Date().getFullYear()} StarClaw &middot; Nydus Git Server &middot;{' '}
        <a href="https://starclaw.me" className="text-nydus-blue hover:underline">starclaw.me</a>
      </footer>
    </div>
  )
}

/* ════════════════════════════════════════════════════════
   HOME PAGE — overview + repo list + recent activity
   ════════════════════════════════════════════════════════ */
function HomePage({ repos, deploys, release, releases, serverStats, loading, onRepoClick }: {
  repos: Repo[]; deploys: Deploy[]; release: Release | null; releases: ReleaseItem[]
  serverStats: ServerStats | null; loading: boolean
  onRepoClick: (name: string) => void
}) {
  return (
    <>
      {/* Hero */}
      <div className="text-center pb-6 mb-6 border-b border-nydus-border">
        <h2 className="text-3xl font-bold mb-2">Nydus Git Server</h2>
        <p className="text-nydus-muted max-w-xl mx-auto">
          Self-hosted Git infrastructure for StarClaw. Mirror, deploy, and distribute — independent of GitHub.
        </p>
        <div className="flex justify-center gap-2 mt-4 flex-wrap">
          <Badge color="green">Online</Badge>
          <Badge>Git SSH</Badge>
          <Badge>Release Mirror</Badge>
          <Badge>Auto Deploy</Badge>
        </div>
      </div>

      {/* Stats */}
      {serverStats && (
        <div className="grid grid-cols-2 sm:grid-cols-4 gap-3 mb-6">
          <StatCard icon={<Database className="w-4 h-4" />} label="Repositories" value={serverStats.repos} />
          <StatCard icon={<GitCommit className="w-4 h-4" />} label="Total Commits" value={serverStats.total_commits} />
          <StatCard icon={<Tag className="w-4 h-4" />} label="Tags / Releases" value={serverStats.total_tags} />
          <StatCard icon={<Rocket className="w-4 h-4" />} label="Deploy Targets" value={serverStats.targets} />
        </div>
      )}

      <div className="flex gap-6 flex-col lg:flex-row">
        {/* Left: Repos + Activity */}
        <div className="flex-1 min-w-0 space-y-6">
          {/* Repositories */}
          <Section title="Repositories" icon={<GitBranch className="w-5 h-5 text-nydus-muted" />}>
            {repos.length === 0 && !loading && (
              <p className="text-nydus-muted text-sm">No repositories configured.</p>
            )}
            <div className="space-y-3">
              {repos.map((r) => (
                <button key={r.name} onClick={() => onRepoClick(r.name)}
                  className="w-full text-left bg-nydus-card border border-nydus-border rounded-lg p-4 hover:border-nydus-blue/40 transition-colors group">
                  <div className="flex items-center gap-2 mb-1.5">
                    <GitBranch className="w-4 h-4 text-nydus-blue" />
                    <span className="text-base font-semibold text-nydus-blue group-hover:underline">{r.name}</span>
                    {r.initialized && (
                      <span className="text-[10px] px-1.5 py-0.5 rounded-full bg-green-700/10 text-nydus-green border border-green-700/20">active</span>
                    )}
                  </div>
                  <p className="text-nydus-muted text-sm mb-2">{r.description}</p>
                  <div className="flex items-center gap-4 text-xs text-nydus-muted flex-wrap">
                    {r.branches !== undefined && (
                      <span className="flex items-center gap-1">
                        <GitBranch className="w-3 h-3" />
                        <b className="text-nydus-text">{r.branches}</b> branch{r.branches !== 1 ? 'es' : ''}
                      </span>
                    )}
                    {r.tags !== undefined && (
                      <span className="flex items-center gap-1">
                        <Tag className="w-3 h-3" />
                        <b className="text-nydus-text">{r.tags}</b> tags
                      </span>
                    )}
                    {r.commit_count !== undefined && (
                      <span className="flex items-center gap-1">
                        <GitCommit className="w-3 h-3" />
                        <b className="text-nydus-text">{r.commit_count}</b> commits
                      </span>
                    )}
                    <span className="flex items-center gap-1">
                      <Rocket className="w-3 h-3" />
                      {r.targets} target{r.targets !== 1 ? 's' : ''}
                    </span>
                  </div>
                  {r.last_commit && (
                    <div className="flex items-center gap-2 mt-2 text-xs text-nydus-dim">
                      <span className="text-nydus-muted">{r.last_commit.author}</span>
                      <span className="truncate flex-1">{r.last_commit.message}</span>
                      <code className="text-nydus-blue shrink-0">{r.last_commit.short_hash}</code>
                      <span className="shrink-0">{r.last_commit.time_ago}</span>
                    </div>
                  )}
                </button>
              ))}
            </div>
          </Section>

          {/* Recent Deploys */}
          <Section title="Recent Deploys" icon={<Activity className="w-5 h-5 text-nydus-muted" />}>
            <div className="bg-nydus-card border border-nydus-border rounded-lg divide-y divide-nydus-border">
              {deploys.length === 0 && (
                <div className="px-4 py-3 text-nydus-muted text-sm">No deployments yet.</div>
              )}
              {deploys.slice(0, 10).map((d, i) => (
                <div key={i} className="flex items-center gap-3 px-4 py-2 text-sm">
                  {d.status === 'success' ? (
                    <CheckCircle2 className="w-4 h-4 text-nydus-green shrink-0" />
                  ) : (
                    <XCircle className="w-4 h-4 text-red-400 shrink-0" />
                  )}
                  <span className="font-medium text-nydus-blue shrink-0">{d.repo}</span>
                  <ChevronRight className="w-3 h-3 text-nydus-dim shrink-0" />
                  <span className="shrink-0">{d.target}</span>
                  <code className="text-xs text-nydus-muted">{d.rev}</code>
                  <span className="flex-1" />
                  <span className="text-nydus-dim text-xs shrink-0">{new Date(d.timestamp).toLocaleString()}</span>
                </div>
              ))}
            </div>
          </Section>
        </div>

        {/* Sidebar */}
        <div className="lg:w-72 shrink-0 space-y-4">
          <SidebarCard title="About">
            <p className="text-nydus-muted text-sm mb-3">
              Self-hosted Git infrastructure for StarClaw. Mirror, deploy, and distribute.
            </p>
            <div className="space-y-2 text-sm">
              <a href="https://nydus.starclaw.net" className="flex items-center gap-2 text-nydus-blue hover:underline">
                <ExternalLink className="w-3.5 h-3.5" /> nydus.starclaw.net
              </a>
              <div className="flex items-center gap-2 text-nydus-muted">
                <Server className="w-3.5 h-3.5" /> Git Server + CI/CD
              </div>
            </div>
          </SidebarCard>

          {releases.length > 0 && (
            <SidebarCard title={`Releases (${releases.length})`}>
              <div className="space-y-3 max-h-80 overflow-y-auto">
                {releases.map((rel) => (
                  <div key={rel.tag_name} className="flex items-start gap-2">
                    <Tag className={`w-3.5 h-3.5 mt-0.5 shrink-0 ${rel.latest ? 'text-nydus-green' : 'text-nydus-muted'}`} />
                    <div className="min-w-0">
                      <div className="flex items-center gap-1.5">
                        <a href={rel.html_url} target="_blank" rel="noreferrer"
                          className={`text-sm font-medium hover:underline ${rel.latest ? 'text-nydus-green' : 'text-nydus-blue'}`}>
                          {rel.tag_name}
                        </a>
                        {rel.latest && (
                          <span className="text-[10px] px-1.5 py-0.5 rounded-full bg-green-700/20 text-nydus-green border border-green-700/30">Latest</span>
                        )}
                      </div>
                      {rel.body && <p className="text-xs text-nydus-dim mt-0.5 truncate">{rel.body}</p>}
                    </div>
                  </div>
                ))}
              </div>
              {release && (
                <div className="mt-3 pt-3 border-t border-nydus-border">
                  <a href="/releases/source.tar.gz"
                    className="inline-flex items-center gap-1 px-2.5 py-1 rounded text-xs font-medium bg-green-700 text-white hover:bg-green-600 transition-colors">
                    <Download className="w-3 h-3" /> Download Latest tar.gz
                  </a>
                </div>
              )}
            </SidebarCard>
          )}
        </div>
      </div>
    </>
  )
}

/* ════════════════════════════════════════════════════════
   REPO DETAIL PAGE — file tree + README + commits
   ════════════════════════════════════════════════════════ */
function RepoDetailPage({ repo, repoName, commits, treeItems, treePath, readme, release, releases, loading, authed, onNavigateTree, onNavigateUp, onBack }: {
  repo: Repo | null; repoName: string
  commits: Commit[]; treeItems: TreeItem[]; treePath: string
  readme: string; release: Release | null; releases: ReleaseItem[]; loading: boolean
  authed: boolean
  onNavigateTree: (item: TreeItem) => void
  onNavigateUp: () => void
  onBack: () => void
}) {
  return (
    <>
      {/* Back link */}
      <button onClick={onBack}
        className="flex items-center gap-1.5 text-sm text-nydus-muted hover:text-nydus-blue transition-colors mb-4">
        <ArrowLeft className="w-3.5 h-3.5" /> Back to repositories
      </button>

      <div className="flex gap-6 flex-col lg:flex-row">
        {/* Main */}
        <div className="flex-1 min-w-0 space-y-6">
          {/* Repo header card */}
          <div className="bg-nydus-card border border-nydus-border rounded-lg overflow-hidden">
            {/* Repo info */}
            <div className="px-4 py-3 border-b border-nydus-border">
              <div className="flex items-center gap-2 mb-1">
                <GitBranch className="w-4 h-4 text-nydus-blue" />
                <h2 className="text-lg font-bold text-nydus-text">{repoName}</h2>
                {repo?.initialized && (
                  <span className="text-[10px] px-1.5 py-0.5 rounded-full bg-green-700/10 text-nydus-green border border-green-700/20">active</span>
                )}
              </div>
              {repo?.description && <p className="text-nydus-muted text-sm mb-2">{repo.description}</p>}
              <div className="flex items-center gap-4 text-xs text-nydus-muted flex-wrap">
                {repo?.branches !== undefined && (
                  <span className="flex items-center gap-1">
                    <GitBranch className="w-3.5 h-3.5" />
                    <b className="text-nydus-text">{repo.branches}</b> branch{repo.branches !== 1 ? 'es' : ''}
                  </span>
                )}
                {repo?.tags !== undefined && (
                  <span className="flex items-center gap-1">
                    <Tag className="w-3.5 h-3.5" />
                    <b className="text-nydus-text">{repo.tags}</b> tags
                  </span>
                )}
                {repo?.commit_count !== undefined && (
                  <span className="flex items-center gap-1">
                    <Clock className="w-3.5 h-3.5" />
                    <b className="text-nydus-text">{repo.commit_count}</b> commits
                  </span>
                )}
              </div>
              {repo?.last_commit && (
                <div className="flex items-center gap-2 mt-2 text-xs">
                  <span className="text-nydus-text font-medium">{repo.last_commit.author}</span>
                  <span className="text-nydus-muted truncate flex-1">{repo.last_commit.message}</span>
                  <code className="text-nydus-blue shrink-0">{repo.last_commit.short_hash}</code>
                  <span className="text-nydus-dim shrink-0">{repo.last_commit.time_ago}</span>
                </div>
              )}
            </div>

            {/* Breadcrumb */}
            {treePath && (
              <div className="px-4 py-2 border-b border-nydus-border bg-nydus-bg/50 flex items-center gap-1 text-sm">
                <button onClick={() => onNavigateUp()} className="text-nydus-blue hover:underline">
                  {repoName}
                </button>
                {treePath.split('/').map((seg, i, arr) => (
                  <span key={i} className="flex items-center gap-1">
                    <span className="text-nydus-dim">/</span>
                    {i === arr.length - 1 ? (
                      <span className="text-nydus-text font-medium">{seg}</span>
                    ) : (
                      <span className="text-nydus-muted">{seg}</span>
                    )}
                  </span>
                ))}
              </div>
            )}

            {/* File Tree */}
            <div className="divide-y divide-nydus-border">
              {treePath && (
                <button onClick={onNavigateUp}
                  className="flex items-center gap-2 px-4 py-2 text-sm text-nydus-blue hover:bg-nydus-bg/50 w-full text-left">
                  <ArrowLeft className="w-3.5 h-3.5" /> ..
                </button>
              )}
              {treeItems.length === 0 && !loading && (
                <div className="px-4 py-8 text-center text-nydus-muted text-sm">
                  Repository is empty or not initialized.
                </div>
              )}
              {treeItems.map((item) => (
                <div key={item.name}
                  className={`flex items-center gap-3 px-4 py-2 text-sm hover:bg-nydus-bg/50 ${item.type === 'tree' ? 'cursor-pointer' : ''}`}
                  onClick={() => onNavigateTree(item)}>
                  {item.type === 'tree' ? (
                    <Folder className="w-4 h-4 text-nydus-blue shrink-0" />
                  ) : (
                    <FileText className="w-4 h-4 text-nydus-muted shrink-0" />
                  )}
                  <span className={`shrink-0 ${item.type === 'tree' ? 'text-nydus-blue font-medium' : 'text-nydus-text'}`}>
                    {item.name}
                  </span>
                  <span className="flex-1 text-nydus-muted text-xs truncate">{item.message}</span>
                  <span className="text-nydus-dim text-xs shrink-0">{item.time_ago}</span>
                </div>
              ))}
            </div>
          </div>

          {/* README */}
          {readme && (
            <div className="bg-nydus-card border border-nydus-border rounded-lg overflow-hidden">
              <div className="flex items-center gap-2 px-4 py-2.5 border-b border-nydus-border bg-nydus-bg/50">
                <FileText className="w-4 h-4 text-nydus-muted" />
                <span className="text-sm font-medium">README.md</span>
              </div>
              <div className="px-6 py-4 text-sm leading-relaxed text-nydus-text/90 whitespace-pre-wrap font-mono">
                {readme}
              </div>
            </div>
          )}

          {/* Recent Commits */}
          <Section title="Recent Commits" icon={<GitCommit className="w-5 h-5 text-nydus-muted" />}>
            <div className="bg-nydus-card border border-nydus-border rounded-lg divide-y divide-nydus-border">
              {commits.length === 0 && (
                <div className="px-4 py-3 text-nydus-muted text-sm">No commits found.</div>
              )}
              {commits.map((c, i) => (
                <div key={i} className="flex items-center gap-3 px-4 py-2 text-sm">
                  <code className="text-nydus-blue bg-nydus-bg px-1.5 py-0.5 rounded text-xs shrink-0">{c.hash}</code>
                  <span className="flex-1 truncate">{c.message}</span>
                  {c.author && <span className="text-nydus-dim text-xs shrink-0">{c.author}</span>}
                  <span className="text-nydus-dim text-xs shrink-0">{c.time}</span>
                </div>
              ))}
            </div>
          </Section>
        </div>

        {/* Sidebar */}
        <div className="lg:w-72 shrink-0 space-y-4">
          {/* Clone */}
          {repo?.ssh_url && (
            <SidebarCard title="Clone">
              <div className="space-y-2">
                <div>
                  <div className="text-[10px] text-nydus-dim uppercase tracking-wider mb-1">HTTPS</div>
                  <code className="block text-xs bg-nydus-bg px-3 py-2 rounded border border-nydus-border text-nydus-muted select-all break-all">
                    git clone {repo.https_url || `https://nydus.starclaw.net/${repoName}.git`}
                  </code>
                </div>
                <div>
                  <div className="text-[10px] text-nydus-dim uppercase tracking-wider mb-1">SSH</div>
                  <code className="block text-xs bg-nydus-bg px-3 py-2 rounded border border-nydus-border text-nydus-muted select-all break-all">
                    git clone {repo.ssh_url}
                  </code>
                </div>
              </div>
            </SidebarCard>
          )}

          {/* Releases */}
          {releases.length > 0 && (
            <SidebarCard title={`Releases (${releases.length})`}>
              <div className="space-y-2.5 max-h-60 overflow-y-auto">
                {releases.slice(0, 10).map((rel) => (
                  <div key={rel.tag_name} className="flex items-center gap-2">
                    <Tag className={`w-3.5 h-3.5 shrink-0 ${rel.latest ? 'text-nydus-green' : 'text-nydus-muted'}`} />
                    <a href={rel.html_url} target="_blank" rel="noreferrer"
                      className={`text-sm hover:underline ${rel.latest ? 'text-nydus-green font-medium' : 'text-nydus-blue'}`}>
                      {rel.tag_name}
                    </a>
                    {rel.latest && (
                      <span className="text-[10px] px-1.5 py-0.5 rounded-full bg-green-700/20 text-nydus-green border border-green-700/30">Latest</span>
                    )}
                  </div>
                ))}
              </div>
              {release && (
                <div className="mt-2 pt-2 border-t border-nydus-border">
                  <a href="/releases/source.tar.gz"
                    className="inline-flex items-center gap-1 px-2.5 py-1 rounded text-xs font-medium bg-green-700 text-white hover:bg-green-600 transition-colors">
                    <Download className="w-3 h-3" /> tar.gz
                  </a>
                </div>
              )}
            </SidebarCard>
          )}

          {/* Deploy Targets */}
          {repo && repo.targets > 0 && (
            <SidebarCard title="Deploy Targets">
              <div className="text-sm text-nydus-muted">
                <span className="text-nydus-text font-medium">{repo.targets}</span> target{repo.targets !== 1 ? 's' : ''} configured
              </div>
            </SidebarCard>
          )}

          {/* API for this repo — admin only */}
          {authed && (
            <SidebarCard title="API Endpoints">
              <div className="space-y-1.5">
                <APIItem method="GET" path={`/v1/repos/${repoName}`} />
                <APIItem method="GET" path={`/v1/repos/${repoName}/tree`} />
                <APIItem method="GET" path={`/v1/repos/${repoName}/readme`} />
                <APIItem method="GET" path={`/v1/repos/${repoName}/branches`} />
                <APIItem method="GET" path={`/v1/repos/${repoName}/tags`} />
                <APIItem method="GET" path={`/v1/commits?repo=${repoName}`} />
              </div>
            </SidebarCard>
          )}
        </div>
      </div>
    </>
  )
}

/* ════════════════════════════════════════════════════════
   Shared Components
   ════════════════════════════════════════════════════════ */

function Section({ title, icon, children }: { title: string; icon: React.ReactNode; children: React.ReactNode }) {
  return (
    <section>
      <h2 className="text-lg font-semibold mb-3 flex items-center gap-2">{icon} {title}</h2>
      {children}
    </section>
  )
}

function StatCard({ icon, label, value }: { icon: React.ReactNode; label: string; value: number }) {
  return (
    <div className="bg-nydus-card border border-nydus-border rounded-lg px-4 py-3">
      <div className="flex items-center gap-2 text-nydus-muted text-xs mb-1">{icon} {label}</div>
      <div className="text-2xl font-bold text-nydus-text">{value.toLocaleString()}</div>
    </div>
  )
}

function SidebarCard({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div className="bg-nydus-card border border-nydus-border rounded-lg p-4">
      <h3 className="text-sm font-semibold mb-3 pb-2 border-b border-nydus-border">{title}</h3>
      {children}
    </div>
  )
}

function Badge({ children, color }: { children: React.ReactNode; color?: string }) {
  const cls = color === 'green'
    ? 'border-green-700 text-nydus-green'
    : 'border-nydus-border text-nydus-muted'
  return (
    <span className={`inline-flex items-center gap-1.5 px-3 py-1 rounded-full text-xs font-medium border bg-nydus-card ${cls}`}>
      {color === 'green' && <span className="w-1.5 h-1.5 rounded-full bg-nydus-green" />}
      {children}
    </span>
  )
}

function APIItem({ method, path }: { method: string; path: string }) {
  return (
    <div className="flex items-center gap-1.5 text-xs">
      <span className="font-bold text-nydus-green min-w-[28px]">{method}</span>
      <code className="text-nydus-muted truncate">{path}</code>
    </div>
  )
}

function LoginModal({ onLogin, onClose }: { onLogin: (secret: string) => Promise<boolean>; onClose: () => void }) {
  const [secret, setSecretVal] = useState('')
  const [error, setError] = useState('')
  const [verifying, setVerifying] = useState(false)

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!secret.trim()) return
    setVerifying(true)
    setError('')
    const ok = await onLogin(secret.trim())
    if (!ok) setError('Invalid secret')
    setVerifying(false)
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60" onClick={onClose}>
      <div className="bg-nydus-card border border-nydus-border rounded-lg p-6 w-full max-w-sm mx-4" onClick={e => e.stopPropagation()}>
        <div className="flex items-center gap-2 mb-4">
          <KeyRound className="w-5 h-5 text-nydus-accent" />
          <h2 className="text-lg font-bold">Admin Access</h2>
        </div>
        <p className="text-nydus-muted text-sm mb-4">
          Enter the Nydus secret to view private repositories and manage deployments.
        </p>
        <form onSubmit={handleSubmit}>
          <input
            type="password"
            value={secret}
            onChange={e => setSecretVal(e.target.value)}
            placeholder="X-Nydus-Secret"
            autoFocus
            className="w-full px-3 py-2 rounded-md bg-nydus-bg border border-nydus-border text-nydus-text placeholder:text-nydus-dim text-sm focus:outline-none focus:border-nydus-blue"
          />
          {error && <p className="text-red-400 text-xs mt-2">{error}</p>}
          <div className="flex gap-2 mt-4">
            <button type="submit" disabled={verifying || !secret.trim()}
              className="flex-1 px-4 py-2 rounded-md text-sm font-medium bg-nydus-accent text-white hover:opacity-90 transition-opacity disabled:opacity-50">
              {verifying ? 'Verifying...' : 'Login'}
            </button>
            <button type="button" onClick={onClose}
              className="px-4 py-2 rounded-md text-sm font-medium bg-nydus-bg border border-nydus-border text-nydus-muted hover:text-nydus-text transition-colors">
              Cancel
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}
