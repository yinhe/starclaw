import { useState, useEffect } from 'react'
import { growthAPI } from '../lib/api'
import PetAvatar from '../components/PetAvatar'

interface GrowthStats {
  conversations: number
  messages: number
  memories: number
  knowledge_docs: number
  tasks_completed: number
  tasks_failed: number
  goals_completed: number
  tools_used: number
  unique_tools: number
  thumbs_up: number
  thumbs_down: number
  days_since_first: number
  day_streak: number
  exp: number
  level: number
  level_progress: number
  next_level_exp: number
  satisfaction_rate: number
  hp: number
  atk: number
  def: number
  spd: number
}

interface MilestoneItem {
  id: string
  code: string
  title: string
  achieved_at: string
  notified_at: string | null
}

interface GrowthProfile {
  stats: GrowthStats
  evolution_path: string
  form_code: string
  title: string
  title_en: string
  path_emoji: string
  path_name: string
  days_with: number
  first_chat: string | null
  agent_count: number
  milestones: MilestoneItem[]
  new_milestones: MilestoneItem[]
}

type TabKey = 'growth' | 'report' | 'curve' | 'assets'

interface DailyReportData {
  date: string
  summary: string
  stats: {
    date: string
    conversations: number
    messages: number
    tasks_completed: number
    tasks_failed: number
    new_memories: number
    thumbs_up: number
    thumbs_down: number
    tools_used: number
    satisfaction_rate: number
  }
  has_data: boolean
}

interface CurveDayStats {
  date: string
  conversations: number
  messages: number
  tasks_completed: number
  new_memories: number
  thumbs_up: number
  thumbs_down: number
}

interface AssetOverview {
  knowledge: { memories: number; documents: number; knowledge_bases: number }
  creations: { agents_published: number; total_downloads: number; total_revenue_cents: number; avg_rating: number }
  node: { claw_id: string; online_days: number }
  agent_count: number
}

const evolutionTree: Record<string, { levels: number[]; names: string[]; namesEN: string[] }> = {
  abyss: {
    levels: [1, 5, 10, 20, 30, 50],
    names: ['小龙虾', '章鱼', '蛟', '鲲', '利维坦', '渊皇'],
    namesEN: ['Claw', 'Octopus', 'Jiao', 'Kun', 'Leviathan', 'Abyssal'],
  },
  terrain: {
    levels: [1, 5, 10, 20, 30, 50],
    names: ['跳虫', '刺蛇', '潜伏者', '雷兽', '泰坦', '陆皇'],
    namesEN: ['Zergling', 'Hydralisk', 'Lurker', 'Ultralisk', 'Titan', 'Colossus'],
  },
  sky: {
    levels: [1, 5, 10, 20, 30, 50],
    names: ['翼龙', '阿根廷巨鹰', '飞龙', '鹏', '守护者', '穹皇'],
    namesEN: ['Pterosaur', 'Argentavis', 'Mutalisk', 'Peng', 'Guardian', 'Skyward'],
  },
}

function StatCard({ icon, label, value, sub }: { icon: string; label: string; value: number | string; sub?: string }) {
  return (
    <div className="bg-white dark:bg-gray-800/90 rounded-xl p-4 text-center border border-gray-200 dark:border-gray-700 shadow-sm hover:shadow-md transition-shadow">
      <div className="text-xl mb-1">{icon}</div>
      <div className="text-lg font-bold text-white">{typeof value === 'number' ? value.toLocaleString() : value}</div>
      <div className="text-xs text-gray-400">{label}</div>
      {sub && <div className="text-[10px] text-gray-500 mt-0.5">{sub}</div>}
    </div>
  )
}

// E3: SVG Pentagon Radar Chart — 5-dimension personality visualization
function PersonalityRadar({ stats }: { stats: GrowthStats }) {
  // Compute 5 personality dimensions (0-100 scale)
  const dims = [
    { label: '💪 执行力', value: Math.min((stats.tasks_completed / 50) * 100, 100) },
    { label: '😊 亲和力', value: stats.satisfaction_rate * 100 },
    { label: '⚡ 活跃度', value: Math.min((stats.day_streak / 30) * 100, 100) },
    { label: '🧠 智慧',   value: Math.min(((stats.memories + stats.knowledge_docs) / 200) * 100, 100) },
    { label: '🛡️ 可靠性', value: (stats.tasks_completed + stats.tasks_failed) > 0
      ? (stats.tasks_completed / (stats.tasks_completed + stats.tasks_failed)) * 100
      : 50 },
  ]

  const cx = 120, cy = 120, r = 90, n = dims.length
  const angle = (i: number) => (Math.PI * 2 * i) / n - Math.PI / 2

  // Generate points for a given radius ratio (0-1)
  const polyPoints = (values: number[]) =>
    values.map((v, i) => {
      const ratio = v / 100
      const x = cx + r * ratio * Math.cos(angle(i))
      const y = cy + r * ratio * Math.sin(angle(i))
      return `${x.toFixed(1)},${y.toFixed(1)}`
    }).join(' ')

  // Grid rings at 25%, 50%, 75%, 100%
  const rings = [0.25, 0.5, 0.75, 1.0]

  return (
    <div className="flex flex-col items-center">
      <svg width="240" height="240" viewBox="0 0 240 240">
        {/* Grid rings */}
        {rings.map(ratio => (
          <polygon
            key={ratio}
            points={dims.map((_, i) => {
              const x = cx + r * ratio * Math.cos(angle(i))
              const y = cy + r * ratio * Math.sin(angle(i))
              return `${x.toFixed(1)},${y.toFixed(1)}`
            }).join(' ')}
            fill="none"
            stroke="rgba(107,114,128,0.3)"
            strokeWidth="1"
          />
        ))}
        {/* Axis lines */}
        {dims.map((_, i) => (
          <line
            key={i}
            x1={cx} y1={cy}
            x2={cx + r * Math.cos(angle(i))}
            y2={cy + r * Math.sin(angle(i))}
            stroke="rgba(107,114,128,0.2)"
            strokeWidth="1"
          />
        ))}
        {/* Data polygon */}
        <polygon
          points={polyPoints(dims.map(d => d.value))}
          fill="rgba(16,185,129,0.2)"
          stroke="rgb(16,185,129)"
          strokeWidth="2"
        />
        {/* Data dots */}
        {dims.map((d, i) => {
          const ratio = d.value / 100
          const x = cx + r * ratio * Math.cos(angle(i))
          const y = cy + r * ratio * Math.sin(angle(i))
          return <circle key={i} cx={x} cy={y} r="3" fill="rgb(16,185,129)" />
        })}
        {/* Labels */}
        {dims.map((d, i) => {
          const labelR = r + 22
          const x = cx + labelR * Math.cos(angle(i))
          const y = cy + labelR * Math.sin(angle(i))
          return (
            <text
              key={i}
              x={x} y={y}
              textAnchor="middle"
              dominantBaseline="middle"
              fill="rgb(156,163,175)"
              fontSize="11"
            >
              {d.label}
            </text>
          )
        })}
      </svg>
      {/* Legend */}
      <div className="flex flex-wrap gap-x-4 gap-y-1 text-[10px] text-gray-500 mt-1 justify-center">
        {dims.map((d, i) => (
          <span key={i}>{d.label.slice(2)}: {d.value.toFixed(0)}</span>
        ))}
      </div>
    </div>
  )
}

function BattleStatBar({ label, value, max, color }: { label: string; value: number; max: number; color: string }) {
  const pct = Math.min((value / max) * 100, 100)
  return (
    <div className="flex items-center gap-2 text-sm">
      <span className="w-10 text-gray-400 text-right">{label}</span>
      <div className="flex-1 h-2 bg-gray-700 rounded-full overflow-hidden">
        <div className={`h-full rounded-full ${color}`} style={{ width: `${pct}%` }} />
      </div>
      <span className="w-8 text-gray-300 text-right font-mono text-xs">{value}</span>
    </div>
  )
}

export default function GrowthPage() {
  const [profile, setProfile] = useState<GrowthProfile | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [tab, setTab] = useState<TabKey>('growth')

  // Load node growth profile
  useEffect(() => {
    setLoading(true)
    setError('')
    growthAPI.getGrowth()
      .then(res => {
        setProfile(res.data)
        setLoading(false)
      })
      .catch(err => {
        setError(err.response?.data?.error || 'Failed to load growth data')
        setLoading(false)
      })
  }, [])

  // Show new milestone popup
  useEffect(() => {
    if (profile?.new_milestones?.length) {
      // Could use a toast system here; for now just log
      profile.new_milestones.forEach(m => {
        console.log(`🏆 New milestone: ${m.title}`)
      })
    }
  }, [profile?.new_milestones])

  if (loading) {
    return (
      <div className="flex items-center justify-center min-h-[60vh]">
        <div className="text-center">
          <div className="text-4xl mb-2 animate-bounce">🦞</div>
          <p className="text-gray-400 text-sm">Loading growth data...</p>
        </div>
      </div>
    )
  }

  if (error) {
    return (
      <div className="flex items-center justify-center min-h-[60vh]">
        <div className="text-center">
          <div className="text-4xl mb-2">❌</div>
          <p className="text-red-400 text-sm">{error}</p>
        </div>
      </div>
    )
  }

  if (!profile) return null

  const { stats, evolution_path: path, form_code: formCode } = profile
  const tree = evolutionTree[path] || evolutionTree.abyss

  // Determine current evolution stage index
  const currentIdx = tree.levels.reduce((acc, lv, i) => (stats.level >= lv ? i : acc), 0)

  return (
    <div className="max-w-4xl mx-auto p-4 space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold text-white flex items-center gap-2">
          🐾 我的小龙虾
        </h1>
        {profile.agent_count > 0 && (
          <span className="text-sm text-gray-400">
            🤖 {profile.agent_count} 个 Agent 共同培养
          </span>
        )}
      </div>

      {/* Tab Bar */}
      <div className="flex gap-1 bg-white/5 dark:bg-gray-800/80 rounded-xl p-1.5 border border-gray-200 dark:border-gray-700">
        {([['growth', '🌱', '成长'], ['report', '📝', '日报'], ['curve', '📈', '曲线'], ['assets', '💼', '资产']] as const).map(([key, icon, label]) => (
          <button
            key={key}
            onClick={() => setTab(key)}
            className={`flex-1 py-2.5 px-3 rounded-lg text-sm font-semibold transition-all ${
              tab === key
                ? 'bg-primary-600 text-white shadow-lg shadow-primary-600/20'
                : 'text-gray-500 dark:text-gray-400 hover:text-gray-900 dark:hover:text-white hover:bg-gray-100 dark:hover:bg-gray-700/50'
            }`}
          >
            {icon} {label}
          </button>
        ))}
      </div>

      {tab === 'growth' && <>
      {/* Hero Section */}
      <div className="bg-white dark:bg-gray-800/90 rounded-2xl p-6 border border-gray-200 dark:border-gray-700 shadow-sm">
        <div className="flex items-start gap-6">
          {/* Pet Avatar */}
          <PetAvatar
            path={path as 'abyss' | 'terrain' | 'sky' | 'larva'}
            formCode={formCode}
            level={stats.level}
            size="lg"
          />

          {/* Info */}
          <div className="flex-1 min-w-0">
            <div className="flex items-center gap-2 mb-1">
              <span className="text-sm px-2 py-0.5 rounded-full bg-primary-100 dark:bg-primary-900/30 text-primary-700 dark:text-primary-300">
                {profile.path_emoji} {profile.path_name}
              </span>
            </div>
            <div className="text-lg text-gray-900 dark:text-gray-100 font-semibold mb-2">
              {profile.title} <span className="text-gray-500 text-sm">({profile.title_en})</span>
            </div>
            <div className="text-sm text-gray-500 dark:text-gray-400 mb-3">
              {profile.days_with > 0
                ? `陪伴你 ${profile.days_with} 天，记住了 ${stats.memories} 件事`
                : '等待首次对话...'}
            </div>

            {/* EXP bar */}
            <div className="mb-1 flex items-center justify-between text-xs text-gray-500 dark:text-gray-400 font-medium">
              <span>Lv.{stats.level}</span>
              <span>{stats.exp.toLocaleString()} / {stats.next_level_exp.toLocaleString()} EXP</span>
              <span>Lv.{stats.level + 1}</span>
            </div>
            <div className="h-3 bg-gray-200 dark:bg-gray-700 rounded-full overflow-hidden">
              <div
                className="h-full bg-gradient-to-r from-green-500 to-emerald-400 rounded-full transition-all duration-500"
                style={{ width: `${stats.level_progress * 100}%` }}
              />
            </div>

            {/* Day streak */}
            {stats.day_streak > 0 && (
              <div className="mt-2 text-xs text-amber-400 flex items-center gap-1">
                🔥 连续使用 {stats.day_streak} 天
              </div>
            )}
          </div>
        </div>
      </div>

      {/* Stats Grid */}
      <div className="grid grid-cols-4 gap-3">
        <StatCard icon="💬" label="对话" value={stats.conversations} />
        <StatCard icon="🧠" label="记忆" value={stats.memories} />
        <StatCard icon="✅" label="任务" value={stats.tasks_completed} />
        <StatCard icon="👍" label="好评" value={stats.thumbs_up}
          sub={stats.satisfaction_rate > 0 ? `${(stats.satisfaction_rate * 100).toFixed(0)}% 满意度` : undefined}
        />
      </div>

      {/* Personality Radar + Battle Stats + Evolution Tree */}
      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
        {/* Personality Radar */}
        <div className="bg-white dark:bg-gray-800/90 rounded-xl p-4 border border-gray-200 dark:border-gray-700 shadow-sm">
          <h3 className="text-sm font-semibold text-gray-700 dark:text-gray-200 mb-2">🎭 性格雷达</h3>
          <PersonalityRadar stats={stats} />
        </div>

        {/* Battle Stats */}
        <div className="bg-white dark:bg-gray-800/90 rounded-xl p-4 border border-gray-200 dark:border-gray-700 shadow-sm">
          <h3 className="text-sm font-semibold text-gray-700 dark:text-gray-200 mb-3">⚔️ 战斗属性</h3>
          <div className="space-y-2">
            <BattleStatBar label="HP" value={stats.hp} max={100} color="bg-red-500" />
            <BattleStatBar label="ATK" value={stats.atk} max={80} color="bg-orange-500" />
            <BattleStatBar label="DEF" value={stats.def} max={60} color="bg-blue-500" />
            <BattleStatBar label="SPD" value={stats.spd} max={80} color="bg-green-500" />
          </div>
          <p className="text-[10px] text-gray-500 mt-2">属性由日常使用决定：记忆→HP，任务→ATK，好评→DEF，对话→SPD</p>
        </div>

        {/* Evolution Tree */}
        <div className="bg-white dark:bg-gray-800/90 rounded-xl p-4 border border-gray-200 dark:border-gray-700 shadow-sm">
          <h3 className="text-sm font-semibold text-gray-700 dark:text-gray-200 mb-3">🧬 进化路线 — {profile.path_emoji} {profile.path_name}</h3>
          <div className="space-y-1">
            {tree.levels.map((lv, i) => {
              const reached = stats.level >= lv
              const current = i === currentIdx
              return (
                <div key={lv} className={`flex items-center gap-2 px-2 py-1 rounded-lg ${current ? 'bg-gray-700/50 ring-1 ring-gray-500' : ''}`}>
                  <span className={`w-5 h-5 rounded-full flex items-center justify-center text-[10px] ${reached ? 'bg-green-500 text-white' : 'bg-gray-700 text-gray-500'}`}>
                    {reached ? '✓' : lv}
                  </span>
                  <span className={`text-sm ${reached ? 'text-white' : 'text-gray-500'}`}>
                    Lv.{lv}
                  </span>
                  <span className={`text-sm font-medium ${current ? 'text-white' : reached ? 'text-gray-300' : 'text-gray-600'}`}>
                    {tree.names[i]}
                  </span>
                  <span className={`text-xs ${current ? 'text-gray-400' : 'text-gray-600'}`}>
                    {tree.namesEN[i]}
                  </span>
                  {current && <span className="text-xs text-yellow-400 ml-auto">← 当前</span>}
                </div>
              )
            })}
          </div>
        </div>
      </div>

      {/* Milestones */}
      <div className="bg-white dark:bg-gray-800/90 rounded-xl p-4 border border-gray-200 dark:border-gray-700 shadow-sm">
        <h3 className="text-sm font-semibold text-gray-700 dark:text-gray-200 mb-3">🏆 里程碑</h3>
        {profile.milestones.length === 0 ? (
          <p className="text-gray-500 text-sm">还没有里程碑，开始和 Agent 对话吧！</p>
        ) : (
          <div className="space-y-2">
            {profile.milestones.map(m => (
              <div key={m.id} className="flex items-center gap-3 text-sm">
                <span className="text-green-400">✅</span>
                <span className="text-white flex-1">{m.title}</span>
                <span className="text-xs text-gray-500">
                  {new Date(m.achieved_at).toLocaleDateString('zh-CN')}
                </span>
              </div>
            ))}
          </div>
        )}

        {/* Upcoming milestones (preview) */}
        <div className="mt-3 pt-3 border-t border-gray-700/50">
          <p className="text-xs text-gray-500 mb-2">即将达成：</p>
          <div className="space-y-1">
            {stats.conversations < 50 && (
              <UpcomingMilestone title="对话 50 次" current={stats.conversations} target={50} />
            )}
            {stats.memories < 100 && stats.memories >= 10 && (
              <UpcomingMilestone title="过目不忘 (100 记忆)" current={stats.memories} target={100} />
            )}
            {stats.tasks_completed < 50 && stats.tasks_completed >= 1 && (
              <UpcomingMilestone title="自主管家 (50 任务)" current={stats.tasks_completed} target={50} />
            )}
            {stats.level < 5 && (
              <UpcomingMilestone title="首次进化 (Lv.5)" current={stats.level} target={5} />
            )}
          </div>
        </div>
      </div>
      </>}

      {tab === 'report' && <DailyReportTab />}
      {tab === 'curve' && <GrowthCurveTab />}
      {tab === 'assets' && <AssetsTab />}
    </div>
  )
}

function DailyReportTab() {
  const [report, setReport] = useState<DailyReportData | null>(null)
  const [loading, setLoading] = useState(true)
  const [date, setDate] = useState(() => {
    const d = new Date()
    d.setDate(d.getDate() - 1)
    return d.toISOString().slice(0, 10)
  })

  useEffect(() => {
    setLoading(true)
    growthAPI.getDailyReport(date)
      .then(res => { setReport(res.data); setLoading(false) })
      .catch(() => setLoading(false))
  }, [date])

  if (loading) return <div className="text-center py-12 text-gray-400">加载日报中...</div>
  if (!report) return <div className="text-center py-12 text-gray-500">暂无数据</div>

  const s = report.stats

  return (
    <div className="space-y-4">
      {/* Date picker */}
      <div className="flex items-center gap-3">
        <button onClick={() => {
          const d = new Date(date); d.setDate(d.getDate() - 1)
          setDate(d.toISOString().slice(0, 10))
        }} className="text-gray-400 hover:text-white px-2 py-1 rounded bg-gray-800">◀</button>
        <input
          type="date"
          value={date}
          onChange={e => setDate(e.target.value)}
          className="bg-gray-800 text-white border border-gray-600 rounded-lg px-3 py-1.5 text-sm"
        />
        <button onClick={() => {
          const d = new Date(date); d.setDate(d.getDate() + 1)
          setDate(d.toISOString().slice(0, 10))
        }} className="text-gray-400 hover:text-white px-2 py-1 rounded bg-gray-800">▶</button>
      </div>

      {/* Summary */}
      <div className="bg-gray-900/80 rounded-xl p-5 border border-gray-700/50">
        <h3 className="text-sm font-semibold text-gray-300 mb-3">📝 每日陪伴报告</h3>
        <p className="text-white text-base leading-relaxed">
          {report.has_data ? report.summary : '这一天很安静，没有互动记录。'}
        </p>
      </div>

      {/* Stats detail */}
      {report.has_data && (
        <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
          <StatCard icon="💬" label="对话" value={s.conversations} />
          <StatCard icon="📨" label="消息" value={s.messages} />
          <StatCard icon="✅" label="完成任务" value={s.tasks_completed} />
          <StatCard icon="❌" label="失败任务" value={s.tasks_failed} />
          <StatCard icon="🧠" label="新记忆" value={s.new_memories} />
          <StatCard icon="👍" label="好评" value={s.thumbs_up} />
          <StatCard icon="👎" label="差评" value={s.thumbs_down} />
          <StatCard icon="🔧" label="工具调用" value={s.tools_used} />
        </div>
      )}
    </div>
  )
}

function GrowthCurveTab() {
  const [curve, setCurve] = useState<CurveDayStats[]>([])
  const [days, setDays] = useState(7)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    setLoading(true)
    growthAPI.getGrowthCurve(days)
      .then(res => { setCurve(res.data?.curve || []); setLoading(false) })
      .catch(() => setLoading(false))
  }, [days])

  if (loading) return <div className="text-center py-12 text-gray-400">加载曲线中...</div>

  const maxConv = Math.max(...curve.map(d => d.conversations), 1)
  const maxMsg = Math.max(...curve.map(d => d.messages), 1)

  return (
    <div className="space-y-4">
      {/* Days selector */}
      <div className="flex gap-2">
        {[7, 14, 30].map(d => (
          <button
            key={d}
            onClick={() => setDays(d)}
            className={`px-3 py-1 rounded-lg text-sm ${
              days === d ? 'bg-gray-700 text-white' : 'text-gray-400 hover:text-white bg-gray-800/50'
            }`}
          >
            {d} 天
          </button>
        ))}
      </div>

      {/* Conversation chart */}
      <div className="bg-gray-900/80 rounded-xl p-4 border border-gray-700/50">
        <h3 className="text-sm font-semibold text-gray-300 mb-3">💬 对话数</h3>
        <div className="flex items-end gap-1 h-32">
          {curve.map((d, i) => {
            const h = (d.conversations / maxConv) * 100
            return (
              <div key={i} className="flex-1 flex flex-col items-center gap-1">
                <span className="text-[10px] text-gray-400">{d.conversations || ''}</span>
                <div
                  className="w-full bg-emerald-500/80 rounded-t transition-all"
                  style={{ height: `${Math.max(h, 2)}%` }}
                />
                <span className="text-[9px] text-gray-500 truncate w-full text-center">
                  {d.date.slice(5)}
                </span>
              </div>
            )
          })}
        </div>
      </div>

      {/* Messages chart */}
      <div className="bg-gray-900/80 rounded-xl p-4 border border-gray-700/50">
        <h3 className="text-sm font-semibold text-gray-300 mb-3">📨 消息数</h3>
        <div className="flex items-end gap-1 h-32">
          {curve.map((d, i) => {
            const h = (d.messages / maxMsg) * 100
            return (
              <div key={i} className="flex-1 flex flex-col items-center gap-1">
                <span className="text-[10px] text-gray-400">{d.messages || ''}</span>
                <div
                  className="w-full bg-blue-500/80 rounded-t transition-all"
                  style={{ height: `${Math.max(h, 2)}%` }}
                />
                <span className="text-[9px] text-gray-500 truncate w-full text-center">
                  {d.date.slice(5)}
                </span>
              </div>
            )
          })}
        </div>
      </div>

      {/* Summary table */}
      <div className="bg-gray-900/80 rounded-xl p-4 border border-gray-700/50">
        <h3 className="text-sm font-semibold text-gray-300 mb-3">📊 详细数据</h3>
        <div className="overflow-x-auto">
          <table className="w-full text-xs">
            <thead>
              <tr className="text-gray-400 border-b border-gray-700">
                <th className="py-1 text-left">日期</th>
                <th className="py-1 text-right">对话</th>
                <th className="py-1 text-right">消息</th>
                <th className="py-1 text-right">任务</th>
                <th className="py-1 text-right">记忆</th>
                <th className="py-1 text-right">👍</th>
              </tr>
            </thead>
            <tbody>
              {curve.map((d, i) => (
                <tr key={i} className="text-gray-300 border-b border-gray-800">
                  <td className="py-1">{d.date.slice(5)}</td>
                  <td className="py-1 text-right">{d.conversations}</td>
                  <td className="py-1 text-right">{d.messages}</td>
                  <td className="py-1 text-right">{d.tasks_completed}</td>
                  <td className="py-1 text-right">{d.new_memories}</td>
                  <td className="py-1 text-right">{d.thumbs_up}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  )
}

function AssetsTab() {
  const [assets, setAssets] = useState<AssetOverview | null>(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    growthAPI.getAssets()
      .then(res => { setAssets(res.data); setLoading(false) })
      .catch(() => setLoading(false))
  }, [])

  if (loading) return <div className="text-center py-12 text-gray-400">加载资产中...</div>
  if (!assets) return <div className="text-center py-12 text-gray-500">暂无数据</div>

  return (
    <div className="space-y-4">
      {/* Knowledge Assets */}
      <div className="bg-gray-900/80 rounded-xl p-4 border border-gray-700/50">
        <h3 className="text-sm font-semibold text-gray-300 mb-3">🧠 知识资产</h3>
        <div className="grid grid-cols-3 gap-3">
          <StatCard icon="💾" label="记忆" value={assets.knowledge.memories} />
          <StatCard icon="📄" label="文档" value={assets.knowledge.documents} />
          <StatCard icon="📚" label="知识库" value={assets.knowledge.knowledge_bases} />
        </div>
      </div>

      {/* Creation Assets */}
      <div className="bg-gray-900/80 rounded-xl p-4 border border-gray-700/50">
        <h3 className="text-sm font-semibold text-gray-300 mb-3">🎨 创作资产</h3>
        <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
          <StatCard icon="🤖" label="已发布 Agent" value={assets.creations.agents_published} />
          <StatCard icon="📥" label="总下载" value={assets.creations.total_downloads} />
          <StatCard icon="💰" label="总收入" value={`¥${(assets.creations.total_revenue_cents / 100).toFixed(2)}`} />
          <StatCard icon="⭐" label="平均评分" value={assets.creations.avg_rating > 0 ? assets.creations.avg_rating.toFixed(1) : '-'} />
        </div>
      </div>

      {/* Node Assets */}
      <div className="bg-gray-900/80 rounded-xl p-4 border border-gray-700/50">
        <h3 className="text-sm font-semibold text-gray-300 mb-3">🖥️ 节点信息</h3>
        <div className="grid grid-cols-2 gap-3">
          <div className="bg-gray-800/50 rounded-xl p-3 border border-gray-700/50">
            <div className="text-xs text-gray-400 mb-1">Claw ID</div>
            <div className="text-white text-sm font-mono truncate">{assets.node.claw_id || '-'}</div>
          </div>
          <StatCard icon="🤖" label="Agent 数量" value={assets.agent_count} />
        </div>
      </div>
    </div>
  )
}

function UpcomingMilestone({ title, current, target }: { title: string; current: number; target: number }) {
  const pct = Math.min((current / target) * 100, 100)
  return (
    <div className="flex items-center gap-2 text-xs">
      <span className="text-gray-500">🔲</span>
      <span className="text-gray-400 flex-1">{title}</span>
      <span className="text-gray-500">{current}/{target}</span>
      <div className="w-16 h-1.5 bg-gray-700 rounded-full overflow-hidden">
        <div className="h-full bg-gray-500 rounded-full" style={{ width: `${pct}%` }} />
      </div>
    </div>
  )
}
