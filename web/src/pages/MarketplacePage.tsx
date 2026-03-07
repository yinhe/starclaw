import { useState, useEffect } from 'react'
import { Bot, Download, Search, User, Star, Code2, PenTool, BarChart3, Palette, Server, BookOpen, Briefcase, Sparkles, Filter } from 'lucide-react'
import { templateAPI } from '../lib/api'

interface AgentTemplate {
  id: string
  name: string
  description: string
  category: string
  tags: string
  system_prompt: string
  tools: string
  icon: string
  featured: boolean
  install_count: number
  rating: number
  rating_count: number
  is_builtin: boolean
  author?: { id: string; username: string }
  created_at: string
}

interface Category {
  id: string
  name: string
  name_en: string
  icon: string
}

const iconMap: Record<string, React.ComponentType<{ className?: string }>> = {
  Bot, Code2, PenTool, BarChart3, Palette, Server, BookOpen, Briefcase,
}

export default function MarketplacePage() {
  const [templates, setTemplates] = useState<AgentTemplate[]>([])
  const [categories, setCategories] = useState<Category[]>([])
  const [search, setSearch] = useState('')
  const [activeCategory, setActiveCategory] = useState('')
  const [installing, setInstalling] = useState<string | null>(null)
  const [toast, setToast] = useState('')
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    templateAPI.categories().then(res => setCategories(res.data.categories || [])).catch(() => {})
    loadTemplates()
  }, [])

  const loadTemplates = async (category?: string, q?: string) => {
    setLoading(true)
    try {
      const params: Record<string, string> = {}
      if (category) params.category = category
      if (q) params.q = q
      const res = await templateAPI.list(params)
      setTemplates(res.data.templates || [])
    } catch { /* ignore */ }
    setLoading(false)
  }

  const handleCategoryClick = (catId: string) => {
    const next = activeCategory === catId ? '' : catId
    setActiveCategory(next)
    loadTemplates(next, search)
  }

  const handleSearch = (val: string) => {
    setSearch(val)
    loadTemplates(activeCategory, val)
  }

  const handleInstall = async (id: string) => {
    setInstalling(id)
    try {
      await templateAPI.install(id)
      setToast('已安装到我的 Agent')
      setTemplates(prev => prev.map(t => t.id === id ? { ...t, install_count: t.install_count + 1 } : t))
      setTimeout(() => setToast(''), 2500)
    } catch {
      setToast('安装失败')
      setTimeout(() => setToast(''), 2500)
    }
    setInstalling(null)
  }

  const parseTags = (tags: string): string[] => {
    try { return JSON.parse(tags) || [] } catch { return [] }
  }

  const getIcon = (iconName: string) => {
    const Icon = iconMap[iconName] || Bot
    return Icon
  }

  const featured = templates.filter(t => t.featured)
  const regular = templates.filter(t => !t.featured)

  return (
    <div className="h-full overflow-y-auto">
      <div className="max-w-6xl mx-auto p-8">
        {/* Header */}
        <div className="mb-8">
          <div className="flex items-center gap-3 mb-2">
            <div className="w-10 h-10 bg-gradient-to-br from-violet-500 to-purple-600 rounded-xl flex items-center justify-center">
              <Sparkles className="w-5 h-5 text-white" />
            </div>
            <div>
              <h1 className="text-2xl font-bold text-gray-900 dark:text-white">Agent 模板市场</h1>
              <p className="text-gray-500 dark:text-gray-400 text-sm">Creep 菌毯 — 发现、安装和分享 AI Agent 模板</p>
            </div>
          </div>
        </div>

        {/* Search + Filter */}
        <div className="flex gap-3 mb-6">
          <div className="relative flex-1">
            <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-gray-400" />
            <input
              value={search}
              onChange={(e) => handleSearch(e.target.value)}
              placeholder="搜索模板..."
              className="w-full pl-10 pr-4 py-2.5 border dark:border-gray-700 bg-white dark:bg-gray-800 text-gray-900 dark:text-white rounded-xl text-sm outline-none focus:ring-2 focus:ring-primary-500"
            />
          </div>
          <button className="flex items-center gap-2 px-4 py-2.5 border dark:border-gray-700 rounded-xl text-sm text-gray-600 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-800">
            <Filter className="w-4 h-4" />
            筛选
          </button>
        </div>

        {/* Categories */}
        <div className="flex gap-2 mb-6 overflow-x-auto pb-1">
          <button
            onClick={() => handleCategoryClick('')}
            className={`flex items-center gap-1.5 px-3 py-1.5 rounded-full text-xs font-medium whitespace-nowrap transition-colors ${
              activeCategory === ''
                ? 'bg-primary-600 text-white'
                : 'bg-gray-100 dark:bg-gray-800 text-gray-600 dark:text-gray-300 hover:bg-gray-200 dark:hover:bg-gray-700'
            }`}
          >
            全部
          </button>
          {categories.map(cat => {
            const CatIcon = getIcon(cat.icon)
            return (
              <button
                key={cat.id}
                onClick={() => handleCategoryClick(cat.id)}
                className={`flex items-center gap-1.5 px-3 py-1.5 rounded-full text-xs font-medium whitespace-nowrap transition-colors ${
                  activeCategory === cat.id
                    ? 'bg-primary-600 text-white'
                    : 'bg-gray-100 dark:bg-gray-800 text-gray-600 dark:text-gray-300 hover:bg-gray-200 dark:hover:bg-gray-700'
                }`}
              >
                <CatIcon className="w-3.5 h-3.5" />
                {cat.name}
              </button>
            )
          })}
        </div>

        {/* Toast */}
        {toast && (
          <div className="mb-4 px-4 py-2 bg-green-50 dark:bg-green-900/20 text-green-700 dark:text-green-400 text-sm rounded-lg">{toast}</div>
        )}

        {loading ? (
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
            {[1,2,3,4,5,6].map(i => (
              <div key={i} className="bg-white dark:bg-gray-800 border dark:border-gray-700 rounded-xl p-5 animate-pulse">
                <div className="flex items-start gap-3 mb-3">
                  <div className="w-10 h-10 bg-gray-200 dark:bg-gray-700 rounded-lg" />
                  <div className="flex-1">
                    <div className="h-4 bg-gray-200 dark:bg-gray-700 rounded w-2/3 mb-2" />
                    <div className="h-3 bg-gray-200 dark:bg-gray-700 rounded w-1/3" />
                  </div>
                </div>
                <div className="h-3 bg-gray-200 dark:bg-gray-700 rounded w-full mb-2" />
                <div className="h-3 bg-gray-200 dark:bg-gray-700 rounded w-4/5" />
              </div>
            ))}
          </div>
        ) : templates.length === 0 ? (
          <div className="text-center py-20 text-gray-400">
            <Bot className="w-12 h-12 mx-auto mb-3 opacity-50" />
            <p>{search ? '没有找到匹配的模板' : '暂无模板'}</p>
          </div>
        ) : (
          <>
            {/* Featured */}
            {featured.length > 0 && !activeCategory && !search && (
              <div className="mb-8">
                <h2 className="text-lg font-semibold text-gray-900 dark:text-white mb-4 flex items-center gap-2">
                  <Star className="w-5 h-5 text-yellow-500" />
                  精选推荐
                </h2>
                <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                  {featured.map(tpl => {
                    const TplIcon = getIcon(tpl.icon)
                    return (
                      <div key={tpl.id} className="bg-gradient-to-br from-violet-50 to-purple-50 dark:from-violet-900/20 dark:to-purple-900/20 border border-violet-200 dark:border-violet-800 rounded-xl p-5 hover:shadow-md transition-shadow">
                        <div className="flex items-start justify-between mb-3">
                          <div className="flex items-center gap-3">
                            <div className="w-10 h-10 bg-violet-100 dark:bg-violet-900/40 rounded-lg flex items-center justify-center">
                              <TplIcon className="w-5 h-5 text-violet-600 dark:text-violet-400" />
                            </div>
                            <div>
                              <h3 className="font-semibold text-gray-900 dark:text-white">{tpl.name}</h3>
                              <div className="flex items-center gap-2 mt-0.5">
                                <span className="text-xs text-violet-600 dark:text-violet-400 font-medium">精选</span>
                                <span className="text-xs text-gray-400">·</span>
                                <span className="text-xs text-gray-400">{tpl.install_count} 次安装</span>
                              </div>
                            </div>
                          </div>
                          <button
                            onClick={() => handleInstall(tpl.id)}
                            disabled={installing === tpl.id}
                            className="flex items-center gap-1.5 px-3 py-1.5 text-xs bg-violet-600 text-white rounded-lg hover:bg-violet-700 disabled:opacity-50"
                          >
                            <Download className="w-3.5 h-3.5" />
                            {installing === tpl.id ? '安装中...' : '安装'}
                          </button>
                        </div>
                        <p className="text-sm text-gray-600 dark:text-gray-300 line-clamp-2">{tpl.description}</p>
                        <div className="flex items-center gap-2 mt-3 flex-wrap">
                          {parseTags(tpl.tags).slice(0, 4).map(tag => (
                            <span key={tag} className="px-2 py-0.5 bg-violet-100 dark:bg-violet-900/30 text-violet-600 dark:text-violet-400 text-xs rounded-full">{tag}</span>
                          ))}
                        </div>
                      </div>
                    )
                  })}
                </div>
              </div>
            )}

            {/* All templates */}
            <div>
              {featured.length > 0 && !activeCategory && !search && (
                <h2 className="text-lg font-semibold text-gray-900 dark:text-white mb-4">全部模板</h2>
              )}
              <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
                {(activeCategory || search ? templates : regular).map(tpl => {
                  const TplIcon = getIcon(tpl.icon)
                  return (
                    <div key={tpl.id} className="bg-white dark:bg-gray-800 border dark:border-gray-700 rounded-xl p-5 hover:shadow-md transition-shadow">
                      <div className="flex items-start justify-between mb-3">
                        <div className="w-10 h-10 bg-blue-50 dark:bg-blue-900/30 rounded-lg flex items-center justify-center">
                          <TplIcon className="w-5 h-5 text-blue-600 dark:text-blue-400" />
                        </div>
                        <button
                          onClick={() => handleInstall(tpl.id)}
                          disabled={installing === tpl.id}
                          className="flex items-center gap-1.5 px-3 py-1.5 text-xs bg-primary-600 text-white rounded-lg hover:bg-primary-700 disabled:opacity-50"
                        >
                          <Download className="w-3.5 h-3.5" />
                          {installing === tpl.id ? '...' : '安装'}
                        </button>
                      </div>
                      <h3 className="font-semibold text-gray-900 dark:text-white">{tpl.name}</h3>
                      <p className="text-sm text-gray-500 dark:text-gray-400 mt-1 line-clamp-2">{tpl.description || '暂无描述'}</p>
                      <div className="flex items-center justify-between mt-3">
                        <div className="flex items-center gap-1 text-xs text-gray-400">
                          <User className="w-3 h-3" />
                          {tpl.author?.username || (tpl.is_builtin ? 'StarClaw' : '匿名')}
                        </div>
                        <div className="flex items-center gap-2 text-xs text-gray-400">
                          {tpl.rating > 0 && (
                            <span className="flex items-center gap-0.5">
                              <Star className="w-3 h-3 text-yellow-500 fill-yellow-500" />
                              {tpl.rating.toFixed(1)}
                            </span>
                          )}
                          <span>{tpl.install_count} 安装</span>
                        </div>
                      </div>
                      {parseTags(tpl.tags).length > 0 && (
                        <div className="flex gap-1 mt-2 flex-wrap">
                          {parseTags(tpl.tags).slice(0, 3).map(tag => (
                            <span key={tag} className="px-1.5 py-0.5 bg-gray-100 dark:bg-gray-700 text-gray-500 dark:text-gray-400 text-xs rounded">{tag}</span>
                          ))}
                        </div>
                      )}
                    </div>
                  )
                })}
              </div>
            </div>
          </>
        )}
      </div>
    </div>
  )
}
