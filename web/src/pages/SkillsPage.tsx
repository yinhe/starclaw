import { useState, useEffect } from 'react'
import { Zap, Search, ChevronDown, ChevronUp } from 'lucide-react'
import { toolAPI } from '../lib/api'

interface Skill {
  name: string
  description: string
  type: 'builtin' | 'plugin' | 'mcp'
  status: string
  category: string
  category_label: string
  subcategory?: string
  subcategory_label?: string
  industry: string
  tags?: string[]
}

const SOURCE_BADGE: Record<string, { label: string; color: string }> = {
  builtin: { label: '内置', color: 'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400' },
  plugin:  { label: '插件', color: 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400' },
  mcp:     { label: 'MCP', color: 'bg-purple-100 text-purple-700 dark:bg-purple-900/30 dark:text-purple-400' },
}

const CATEGORY_BADGE: Record<string, string> = {
  medical_clinical: 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400',
  bioinformatics: 'bg-cyan-100 text-cyan-700 dark:bg-cyan-900/30 dark:text-cyan-400',
  research: 'bg-sky-100 text-sky-700 dark:bg-sky-900/30 dark:text-sky-400',
  engineering: 'bg-indigo-100 text-indigo-700 dark:bg-indigo-900/30 dark:text-indigo-400',
  creative_media: 'bg-pink-100 text-pink-700 dark:bg-pink-900/30 dark:text-pink-400',
  platform_system: 'bg-slate-100 text-slate-700 dark:bg-slate-900/30 dark:text-slate-400',
  integration: 'bg-violet-100 text-violet-700 dark:bg-violet-900/30 dark:text-violet-400',
  finance_trading: 'bg-orange-100 text-orange-700 dark:bg-orange-900/30 dark:text-orange-400',
  general_tools: 'bg-gray-100 text-gray-700 dark:bg-gray-700 dark:text-gray-200',
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
  const [categoryFilter, setCategoryFilter] = useState('all')
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

  const categoryOptions = Array.from(new Map(skills.map(skill => [skill.category, skill.category_label || skill.category])).entries())
    .map(([value, label]) => ({ value, label }))
    .sort((a, b) => a.label.localeCompare(b.label, 'zh-CN'))

  const filtered = skills.filter(s => {
    if (typeFilter !== 'all' && s.type !== typeFilter) return false
    if (categoryFilter !== 'all' && s.category !== categoryFilter) return false
    if (search) {
      const haystack = [
        s.name,
        s.description,
        s.category_label,
        s.subcategory_label || '',
        s.industry,
        ...(s.tags || []),
      ].join(' ').toLowerCase()
      if (!haystack.includes(search.toLowerCase())) return false
    }
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

        <div className="flex flex-wrap gap-2 mb-6">
          <button
            onClick={() => setCategoryFilter('all')}
            className={`px-3 py-1.5 rounded-full text-xs transition ${categoryFilter === 'all' ? 'bg-gray-900 text-white dark:bg-white dark:text-gray-900' : 'bg-white dark:bg-gray-800 text-gray-600 dark:text-gray-300 border border-gray-200 dark:border-gray-700'}`}
          >
            全部分类
          </button>
          {categoryOptions.map(category => (
            <button
              key={category.value}
              onClick={() => setCategoryFilter(category.value)}
              className={`px-3 py-1.5 rounded-full text-xs transition ${categoryFilter === category.value ? 'bg-gray-900 text-white dark:bg-white dark:text-gray-900' : 'bg-white dark:bg-gray-800 text-gray-600 dark:text-gray-300 border border-gray-200 dark:border-gray-700'}`}
            >
              {category.label}
            </button>
          ))}
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
              const categoryBadge = CATEGORY_BADGE[skill.category] || CATEGORY_BADGE.general_tools
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
                        <span className={`text-[10px] font-medium px-1.5 py-0.5 rounded-full ${categoryBadge}`}>{skill.category_label || skill.category}</span>
                        <span className={`w-1.5 h-1.5 rounded-full flex-none ${skill.status === 'active' ? 'bg-green-500' : 'bg-gray-400'}`} />
                      </div>
                      <p className="text-xs text-gray-500 dark:text-gray-400 mt-0.5 truncate">{skill.description.split('\n')[0].slice(0, 120)}</p>
                      <div className="flex items-center gap-2 mt-1 text-[11px] text-gray-400">
                        {skill.subcategory_label && <span>{skill.subcategory_label}</span>}
                        <span>{skill.industry}</span>
                      </div>
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
                        <span>分类: <strong>{skill.category_label || skill.category}</strong></span>
                        {skill.subcategory_label && <span>子类: <strong>{skill.subcategory_label}</strong></span>}
                        <span>状态: <strong className={skill.status === 'active' ? 'text-green-500' : ''}>{skill.status}</strong></span>
                      </div>
                      {!!skill.tags?.length && (
                        <div className="flex flex-wrap gap-2 mt-3">
                          {skill.tags.map(tag => (
                            <span key={`${skill.name}-${tag}`} className="text-[10px] px-2 py-1 rounded-full bg-gray-100 text-gray-600 dark:bg-gray-700 dark:text-gray-300">
                              {tag}
                            </span>
                          ))}
                        </div>
                      )}
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
