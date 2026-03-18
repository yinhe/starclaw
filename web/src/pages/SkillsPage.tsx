import { useState, useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { Zap, Box, Plug, Search, ChevronDown, ChevronUp, Settings2 } from 'lucide-react'
import { toolAPI } from '../lib/api'

interface Skill {
  name: string
  description: string
  type: 'builtin' | 'plugin' | 'mcp'
  status: string
}

interface Summary {
  total: number
  builtin: number
  plugin: number
  mcp: number
}

const TYPE_META: Record<string, { label: string; color: string; bg: string; icon: typeof Zap }> = {
  builtin: { label: '内置技能', color: 'text-blue-400', bg: 'bg-blue-500/10 border-blue-500/20', icon: Zap },
  plugin:  { label: '插件', color: 'text-amber-400', bg: 'bg-amber-500/10 border-amber-500/20', icon: Box },
  mcp:     { label: 'MCP 外接', color: 'text-purple-400', bg: 'bg-purple-500/10 border-purple-500/20', icon: Plug },
}

const SKILL_ICONS: Record<string, string> = {
  system: '⚙️',
  code: '💻',
  web_search: '🔍',
  http_request: '🌐',
  browser: '🖥️',
  video_generation: '🎬',
  document: '📄',
}

export default function SkillsPage() {
  const navigate = useNavigate()
  const [skills, setSkills] = useState<Skill[]>([])
  const [summary, setSummary] = useState<Summary | null>(null)
  const [loading, setLoading] = useState(true)
  const [search, setSearch] = useState('')
  const [typeFilter, setTypeFilter] = useState<string>('all')
  const [expanded, setExpanded] = useState<Set<string>>(new Set())

  useEffect(() => {
    loadSkills()
  }, [])

  const loadSkills = async () => {
    setLoading(true)
    try {
      const res = await toolAPI.skills()
      setSkills(res.data.skills || [])
      setSummary(res.data.summary || null)
    } catch (e) {
      console.error('Failed to load skills', e)
    } finally {
      setLoading(false)
    }
  }

  const filtered = skills.filter(s => {
    if (typeFilter !== 'all' && s.type !== typeFilter) return false
    if (search && !s.name.toLowerCase().includes(search.toLowerCase()) && !s.description.toLowerCase().includes(search.toLowerCase())) return false
    return true
  })

  const toggleExpand = (name: string) => {
    setExpanded(prev => {
      const next = new Set(prev)
      if (next.has(name)) next.delete(name)
      else next.add(name)
      return next
    })
  }

  return (
    <div className="h-full overflow-y-auto bg-gray-50 dark:bg-gray-900">
      <div className="max-w-5xl mx-auto px-6 py-8">
        {/* Header */}
        <div className="mb-8 flex items-start justify-between">
          <div>
            <h1 className="text-2xl font-bold text-gray-900 dark:text-white flex items-center gap-2">
              <Zap className="w-6 h-6 text-blue-500" />
              技能 & MCP
            </h1>
            <p className="text-sm text-gray-500 dark:text-gray-400 mt-1">
              管理 Agent 可以使用的所有技能，包括内置工具、插件和 MCP 外接服务
            </p>
          </div>
          <button
            onClick={() => navigate('/mcp')}
            className="flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium text-purple-600 dark:text-purple-400 bg-purple-50 dark:bg-purple-900/20 border border-purple-200 dark:border-purple-800/40 rounded-lg hover:bg-purple-100 dark:hover:bg-purple-900/40 transition-colors"
          >
            <Settings2 className="w-3.5 h-3.5" />
            管理 MCP 服务器
          </button>
        </div>

        {/* Summary cards */}
        {summary && (
          <div className="grid grid-cols-2 md:grid-cols-4 gap-4 mb-6">
            <SummaryCard label="全部技能" value={summary.total} color="text-white" bg="bg-gradient-to-br from-gray-700 to-gray-900" />
            <SummaryCard label="内置技能" value={summary.builtin} color="text-white" bg="bg-gradient-to-br from-blue-600 to-blue-800" />
            <SummaryCard label="插件" value={summary.plugin} color="text-white" bg="bg-gradient-to-br from-amber-500 to-amber-700" />
            <SummaryCard label="MCP 外接" value={summary.mcp} color="text-white" bg="bg-gradient-to-br from-purple-600 to-purple-800" />
          </div>
        )}

        {/* Filters */}
        <div className="flex flex-wrap items-center gap-3 mb-6">
          <div className="relative flex-1 min-w-[200px] max-w-md">
            <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-gray-400" />
            <input
              value={search}
              onChange={e => setSearch(e.target.value)}
              placeholder="搜索技能名称或描述..."
              className="w-full pl-9 pr-4 py-2 rounded-lg border border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800 text-sm text-gray-900 dark:text-white focus:outline-none focus:ring-2 focus:ring-blue-500"
            />
          </div>
          <div className="flex gap-1.5">
            {['all', 'builtin', 'plugin', 'mcp'].map(t => (
              <button
                key={t}
                onClick={() => setTypeFilter(t)}
                className={`px-3 py-1.5 rounded-full text-xs font-medium transition-colors ${
                  typeFilter === t
                    ? 'bg-blue-600 text-white'
                    : 'bg-gray-200 dark:bg-gray-700 text-gray-600 dark:text-gray-300 hover:bg-gray-300 dark:hover:bg-gray-600'
                }`}
              >
                {t === 'all' ? '全部' : TYPE_META[t]?.label || t}
              </button>
            ))}
          </div>
        </div>

        {/* Skills list */}
        {loading ? (
          <div className="text-center py-16 text-gray-400">加载中...</div>
        ) : filtered.length === 0 ? (
          <div className="text-center py-16 text-gray-400">
            {search ? '没有匹配的技能' : '暂无技能'}
          </div>
        ) : (
          <div className="space-y-3">
            {filtered.map(skill => {
              const meta = TYPE_META[skill.type] || TYPE_META.builtin
              const Icon = meta.icon
              const emoji = SKILL_ICONS[skill.name] || '🔧'
              const isExpanded = expanded.has(skill.name)

              return (
                <div
                  key={skill.name}
                  className={`rounded-xl border ${meta.bg} backdrop-blur-sm overflow-hidden transition-all hover:shadow-md`}
                >
                  <div
                    className="flex items-center gap-4 px-5 py-4 cursor-pointer"
                    onClick={() => toggleExpand(skill.name)}
                  >
                    {/* Icon */}
                    <div className="text-2xl flex-none w-10 h-10 flex items-center justify-center rounded-lg bg-white/10">
                      {emoji}
                    </div>

                    {/* Info */}
                    <div className="flex-1 min-w-0">
                      <div className="flex items-center gap-2">
                        <span className="font-semibold text-gray-900 dark:text-white text-sm">
                          {skill.name}
                        </span>
                        <span className={`text-[10px] font-medium px-2 py-0.5 rounded-full ${meta.color} bg-white/10`}>
                          <Icon className="w-3 h-3 inline mr-0.5 -mt-0.5" />
                          {meta.label}
                        </span>
                        {skill.status === 'active' || skill.status === 'connected' ? (
                          <span className="w-2 h-2 rounded-full bg-green-500 flex-none" title="活跃" />
                        ) : (
                          <span className="w-2 h-2 rounded-full bg-gray-500 flex-none" title={skill.status} />
                        )}
                      </div>
                      <p className="text-xs text-gray-500 dark:text-gray-400 mt-0.5 truncate">
                        {skill.description.split('\n')[0].slice(0, 100)}
                      </p>
                    </div>

                    {/* Expand */}
                    <div className="flex-none text-gray-400">
                      {isExpanded ? <ChevronUp className="w-4 h-4" /> : <ChevronDown className="w-4 h-4" />}
                    </div>
                  </div>

                  {/* Expanded detail */}
                  {isExpanded && (
                    <div className="px-5 pb-4 pt-0 border-t border-white/10">
                      <pre className="text-xs text-gray-400 dark:text-gray-500 whitespace-pre-wrap leading-relaxed mt-3 bg-black/20 rounded-lg p-3 max-h-48 overflow-y-auto">
                        {skill.description}
                      </pre>
                      <div className="flex items-center gap-4 mt-3 text-xs text-gray-500">
                        <span>类型: <strong className={meta.color}>{meta.label}</strong></span>
                        <span>状态: <strong className={skill.status === 'active' ? 'text-green-400' : 'text-gray-400'}>{skill.status}</strong></span>
                      </div>
                    </div>
                  )}
                </div>
              )
            })}
          </div>
        )}
      </div>
    </div>
  )
}

function SummaryCard({ label, value, color, bg }: { label: string; value: number; color: string; bg: string }) {
  return (
    <div className={`rounded-xl p-4 ${bg} shadow-md`}>
      <div className={`text-2xl font-bold ${color}`}>{value}</div>
      <div className="text-xs text-white/70 mt-1">{label}</div>
    </div>
  )
}
