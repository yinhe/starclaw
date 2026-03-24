import { useEffect, useState } from 'react'
import { api } from '../api'
import { GitCommit, Rocket, Calendar, Users, Flame } from 'lucide-react'

interface HeatmapEntry {
  date: string
  count: number
}

interface CommitEntry {
  hash: string
  message: string
  time: string
  author: string
}

interface DeployEntry {
  id: string
  repo: string
  branch: string
  target: string
  status: string
  duration_ms: number
  created_at: string
}

export default function TimelinePage() {
  const [heatmap, setHeatmap] = useState<HeatmapEntry[]>([])
  const [commits, setCommits] = useState<CommitEntry[]>([])
  const [deploys, setDeploys] = useState<DeployEntry[]>([])
  const [authors, setAuthors] = useState<Record<string, number>>({})
  const [totalCommits, setTotalCommits] = useState(0)
  const [activeDays, setActiveDays] = useState(0)
  const [days, setDays] = useState(30)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    loadData()
  }, [days])

  async function loadData() {
    setLoading(true)
    try {
      const [hm, cm, dp] = await Promise.all([
        api.heatmap('starclaw', days),
        api.commits('starclaw', 30),
        api.deploys(),
      ])
      setHeatmap(hm.heatmap || [])
      setTotalCommits(hm.total_commits || 0)
      setActiveDays(hm.active_days || 0)
      setAuthors(hm.authors || {})
      setCommits(cm.commits || [])
      setDeploys(dp.deploys || [])
    } catch (e) {
      console.error('Failed to load timeline data:', e)
    }
    setLoading(false)
  }

  const maxCount = Math.max(...heatmap.map((h) => h.count), 1)

  function heatColor(count: number): string {
    if (count === 0) return 'bg-stone-800'
    const ratio = count / maxCount
    if (ratio > 0.75) return 'bg-forge-500'
    if (ratio > 0.5) return 'bg-forge-600'
    if (ratio > 0.25) return 'bg-forge-700'
    return 'bg-forge-800'
  }

  const sortedAuthors = Object.entries(authors).sort((a, b) => b[1] - a[1])

  return (
    <div className="p-6 space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <Calendar className="w-6 h-6 text-forge-400" />
          <h1 className="text-xl font-bold text-white">Timeline & Activity</h1>
        </div>
        <div className="flex gap-2">
          {[7, 14, 30, 90].map((d) => (
            <button
              key={d}
              onClick={() => setDays(d)}
              className={`px-3 py-1 text-xs rounded-md transition ${
                days === d
                  ? 'bg-forge-500 text-white'
                  : 'bg-stone-800 text-stone-400 hover:bg-stone-700'
              }`}
            >
              {d}d
            </button>
          ))}
        </div>
      </div>

      {loading ? (
        <div className="text-stone-500 text-center py-20">Loading...</div>
      ) : (
        <>
          {/* Stats row */}
          <div className="grid grid-cols-3 gap-4">
            <div className="bg-stone-900 rounded-lg p-4 border border-stone-800">
              <div className="text-2xl font-bold text-white">{totalCommits}</div>
              <div className="text-xs text-stone-500">commits in {days} days</div>
            </div>
            <div className="bg-stone-900 rounded-lg p-4 border border-stone-800">
              <div className="text-2xl font-bold text-white">{activeDays}</div>
              <div className="text-xs text-stone-500">active days</div>
            </div>
            <div className="bg-stone-900 rounded-lg p-4 border border-stone-800">
              <div className="text-2xl font-bold text-white">{sortedAuthors.length}</div>
              <div className="text-xs text-stone-500">contributors</div>
            </div>
          </div>

          {/* Heatmap */}
          <div className="bg-stone-900 rounded-lg p-4 border border-stone-800">
            <h2 className="text-sm font-semibold text-stone-300 mb-3 flex items-center gap-2">
              <Flame className="w-4 h-4 text-forge-400" />
              Commit Heatmap
            </h2>
            <div className="flex flex-wrap gap-1">
              {heatmap.map((entry) => (
                <div
                  key={entry.date}
                  className={`w-6 h-6 rounded-sm ${heatColor(entry.count)} cursor-default`}
                  title={`${entry.date}: ${entry.count} commits`}
                />
              ))}
              {heatmap.length === 0 && (
                <div className="text-stone-600 text-xs">No commit data available</div>
              )}
            </div>
            <div className="flex items-center gap-2 mt-2 text-[10px] text-stone-600">
              <span>Less</span>
              <div className="w-3 h-3 rounded-sm bg-stone-800" />
              <div className="w-3 h-3 rounded-sm bg-forge-800" />
              <div className="w-3 h-3 rounded-sm bg-forge-700" />
              <div className="w-3 h-3 rounded-sm bg-forge-600" />
              <div className="w-3 h-3 rounded-sm bg-forge-500" />
              <span>More</span>
            </div>
          </div>

          {/* Contributors */}
          {sortedAuthors.length > 0 && (
            <div className="bg-stone-900 rounded-lg p-4 border border-stone-800">
              <h2 className="text-sm font-semibold text-stone-300 mb-3 flex items-center gap-2">
                <Users className="w-4 h-4 text-forge-400" />
                Contributors
              </h2>
              <div className="space-y-2">
                {sortedAuthors.map(([name, count]) => (
                  <div key={name} className="flex items-center gap-3">
                    <span className="text-sm text-stone-300 w-32 truncate">{name}</span>
                    <div className="flex-1 h-2 bg-stone-800 rounded-full overflow-hidden">
                      <div
                        className="h-full bg-forge-500 rounded-full"
                        style={{ width: `${(count / totalCommits) * 100}%` }}
                      />
                    </div>
                    <span className="text-xs text-stone-500 w-12 text-right">{count}</span>
                  </div>
                ))}
              </div>
            </div>
          )}

          <div className="grid grid-cols-2 gap-4">
            {/* Recent commits */}
            <div className="bg-stone-900 rounded-lg p-4 border border-stone-800">
              <h2 className="text-sm font-semibold text-stone-300 mb-3 flex items-center gap-2">
                <GitCommit className="w-4 h-4 text-forge-400" />
                Recent Commits
              </h2>
              <div className="space-y-1.5 max-h-80 overflow-auto">
                {commits.map((c, i) => (
                  <div key={i} className="flex items-start gap-2 text-xs">
                    <code className="text-forge-400 font-mono shrink-0">{c.hash}</code>
                    <span className="text-stone-300 truncate flex-1">{c.message}</span>
                    <span className="text-stone-600 shrink-0">{c.time}</span>
                  </div>
                ))}
                {commits.length === 0 && (
                  <div className="text-stone-600 text-xs">No commits</div>
                )}
              </div>
            </div>

            {/* Recent deploys */}
            <div className="bg-stone-900 rounded-lg p-4 border border-stone-800">
              <h2 className="text-sm font-semibold text-stone-300 mb-3 flex items-center gap-2">
                <Rocket className="w-4 h-4 text-forge-400" />
                Recent Deploys
              </h2>
              <div className="space-y-2 max-h-80 overflow-auto">
                {deploys.map((d, i) => (
                  <div key={i} className="flex items-center gap-2 text-xs">
                    <span
                      className={`w-2 h-2 rounded-full shrink-0 ${
                        d.status === 'success' ? 'bg-green-500' : d.status === 'failed' ? 'bg-red-500' : 'bg-yellow-500'
                      }`}
                    />
                    <span className="text-stone-300 truncate flex-1">
                      {d.repo}/{d.branch} → {d.target}
                    </span>
                    {d.duration_ms > 0 && (
                      <span className="text-stone-600">{(d.duration_ms / 1000).toFixed(1)}s</span>
                    )}
                  </div>
                ))}
                {deploys.length === 0 && (
                  <div className="text-stone-600 text-xs">No deploys</div>
                )}
              </div>
            </div>
          </div>
        </>
      )}
    </div>
  )
}
