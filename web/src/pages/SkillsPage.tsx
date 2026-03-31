import { useState, useEffect } from 'react'
import { Zap, Search, ChevronDown, ChevronUp } from 'lucide-react'
import { toolAPI } from '../lib/api'

interface Skill {
  name: string
  description: string
  type: 'builtin' | 'plugin' | 'mcp'
  status: string
}

const SOURCE_BADGE: Record<string, { label: string; color: string }> = {
  builtin: { label: '内置', color: 'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400' },
  plugin:  { label: '插件', color: 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400' },
  mcp:     { label: 'MCP', color: 'bg-purple-100 text-purple-700 dark:bg-purple-900/30 dark:text-purple-400' },
}

const SKILL_ICON: Record<string, string> = {
  system: '⚙️', code: '💻', web_search: '🔍', http_request: '🌐', browser: '🖥️',
  video_generation: '🎬', image_generation: '🎨', music_generation: '🎵',
  audio_analysis: '🎧', mv_production: '🎞️', dubbing: '🗣️', comic_production: '📖',
  deploy_web: '🚀', bind_domain: '🌐', verify_online: '✅',
  trading_scan: '📊', trading_kline: '📈', trading_buy: '🟢', trading_sell: '🔴',
  trading_positions_list: '📋', trading_check_exits: '🛡️', trading_quote: '💰',
  trading_health: '💚', trading_premarket: '🌅', trading_daily_report: '📰',
  trading_positions: '📋',
}

export default function SkillsPage() {
  const [skills, setSkills] = useState<Skill[]>([])
  const [loading, setLoading] = useState(true)
  const [search, setSearch] = useState('')
  const [typeFilter, setTypeFilter] = useState('all')
  const [expanded, setExpanded] = useState<Set<string>>(new Set())

  useEffect(() => { load() }, [])

  const load = async () => {
    setLoading(true)
    try {
      const res = await toolAPI.skills()
      setSkills(res.data.skills || [])
    } catch {}
    setLoading(false)
  }

  const toggle = (name: string) => setExpanded(prev => { const n = new Set(prev); n.has(name) ? n.delete(name) : n.add(name); return n })

  const filtered = skills.filter(s => {
    if (typeFilter !== 'all' && s.type !== typeFilter) return false
    if (search && !s.name.toLowerCase().includes(search.toLowerCase()) && !s.description.toLowerCase().includes(search.toLowerCase())) return false
    return true
  })

  const builtinCount = skills.filter(s => s.type === 'builtin').length
  const pluginCount = skills.filter(s => s.type === 'plugin').length
  const mcpCount = skills.filter(s => s.type === 'mcp').length

  return (
    <div className="h-full overflow-y-auto bg-gray-50 dark:bg-gray-900">
      <div className="max-w-5xl mx-auto px-6 py-8">
        <div className="mb-6">
          <h1 className="text-2xl font-bold text-gray-900 dark:text-white flex items-center gap-2">
            <Zap className="w-6 h-6 text-blue-500" /> 技能
          </h1>
          <p className="text-sm text-gray-500 mt-1">你让我做，我就做 — 智能体可装备的所有能力</p>
        </div>

        {/* Stats */}
        <div className="grid grid-cols-4 gap-3 mb-6">
          <button onClick={() => setTypeFilter('all')} className={`rounded-xl p-4 shadow-md text-left transition ${typeFilter === 'all' ? 'ring-2 ring-white/50' : ''} bg-gradient-to-br from-gray-700 to-gray-900`}>
            <div className="text-2xl font-bold text-white">{skills.length}</div>
            <div className="text-xs text-white/70 mt-1">全部技能</div>
          </button>
          <button onClick={() => setTypeFilter('builtin')} className={`rounded-xl p-4 shadow-md text-left transition ${typeFilter === 'builtin' ? 'ring-2 ring-white/50' : ''} bg-gradient-to-br from-blue-600 to-blue-800`}>
            <div className="text-2xl font-bold text-white">{builtinCount}</div>
            <div className="text-xs text-white/70 mt-1">内置技能</div>
          </button>
          <button onClick={() => setTypeFilter('plugin')} className={`rounded-xl p-4 shadow-md text-left transition ${typeFilter === 'plugin' ? 'ring-2 ring-white/50' : ''} bg-gradient-to-br from-amber-500 to-amber-700`}>
            <div className="text-2xl font-bold text-white">{pluginCount}</div>
            <div className="text-xs text-white/70 mt-1">插件技能</div>
          </button>
          <button onClick={() => setTypeFilter('mcp')} className={`rounded-xl p-4 shadow-md text-left transition ${typeFilter === 'mcp' ? 'ring-2 ring-white/50' : ''} bg-gradient-to-br from-purple-600 to-purple-800`}>
            <div className="text-2xl font-bold text-white">{mcpCount}</div>
            <div className="text-xs text-white/70 mt-1">MCP 技能</div>
          </button>
        </div>

        {/* Search */}
        <div className="relative max-w-md mb-6">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-gray-400" />
          <input value={search} onChange={e => setSearch(e.target.value)} placeholder="搜索技能名称或描述..."
            className="w-full pl-9 pr-4 py-2 rounded-lg border border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800 text-sm text-gray-900 dark:text-white" />
        </div>

        {loading ? (
          <div className="text-center py-16 text-gray-400">加载中...</div>
        ) : filtered.length === 0 ? (
          <div className="text-center py-16 text-gray-400">
            <Zap className="w-10 h-10 mx-auto mb-2 opacity-40" />
            <p>{search ? '没有匹配的技能' : '暂无技能'}</p>
          </div>
        ) : (
          <div className="space-y-2">
            {filtered.map(skill => {
              const badge = SOURCE_BADGE[skill.type] || SOURCE_BADGE.builtin
              const emoji = SKILL_ICON[skill.name] || '🔧'
              const isOpen = expanded.has(skill.name)
              return (
                <div key={skill.name} className="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 overflow-hidden hover:shadow-md transition">
                  <div className="flex items-center gap-4 px-5 py-3.5 cursor-pointer" onClick={() => toggle(skill.name)}>
                    <span className="text-xl flex-none">{emoji}</span>
                    <div className="flex-1 min-w-0">
                      <div className="flex items-center gap-2">
                        <span className="font-medium text-sm text-gray-900 dark:text-white">{skill.name}</span>
                        <span className={`text-[10px] font-medium px-1.5 py-0.5 rounded-full ${badge.color}`}>{badge.label}</span>
                        <span className={`w-1.5 h-1.5 rounded-full flex-none ${skill.status === 'active' ? 'bg-green-500' : 'bg-gray-400'}`} />
                      </div>
                      <p className="text-xs text-gray-500 dark:text-gray-400 mt-0.5 truncate">{skill.description.split('\n')[0].slice(0, 120)}</p>
                    </div>
                    {isOpen ? <ChevronUp className="w-4 h-4 text-gray-400 flex-none" /> : <ChevronDown className="w-4 h-4 text-gray-400 flex-none" />}
                  </div>
                  {isOpen && (
                    <div className="px-5 pb-4 border-t border-gray-100 dark:border-gray-700">
                      <pre className="text-xs text-gray-500 whitespace-pre-wrap leading-relaxed mt-3 bg-gray-50 dark:bg-gray-750 rounded-lg p-3 max-h-48 overflow-y-auto">
                        {skill.description}
                      </pre>
                      <div className="flex items-center gap-3 mt-2 text-[11px] text-gray-400">
                        <span>来源: <strong>{badge.label}</strong></span>
                        <span>状态: <strong className={skill.status === 'active' ? 'text-green-500' : ''}>{skill.status}</strong></span>
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
