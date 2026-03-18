import { useState, useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { Bot, Download, Search, Check, Trash2, Loader2, Store } from 'lucide-react'
import { agentAPI, queenMarketplaceAPI } from '../lib/api'

interface MarketplaceItem {
  id: string
  name: string
  description: string
  icon: string
  tags: string
  config: string
  downloads: number
  rating: number
  author?: { nickname?: string }
}

export default function MarketplacePage() {
  const navigate = useNavigate()
  const [items, setItems] = useState<MarketplaceItem[]>([])
  const [installedIDs, setInstalledIDs] = useState<Set<string>>(new Set())
  const [search, setSearch] = useState('')
  const [installing, setInstalling] = useState<string | null>(null)
  const [toast, setToast] = useState('')
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    loadMarketplace()
    loadInstalled()
  }, [])

  const loadMarketplace = async (q?: string) => {
    setLoading(true)
    try {
      const res = await queenMarketplaceAPI.list({ q: q || undefined })
      setItems(res.data.items || [])
    } catch { setItems([]) }
    setLoading(false)
  }

  const loadInstalled = async () => {
    try {
      const res = await agentAPI.installedSourceIDs()
      setInstalledIDs(new Set(res.data.source_ids || []))
    } catch { /* ignore */ }
  }

  const handleSearch = (val: string) => {
    setSearch(val)
    loadMarketplace(val)
  }

  const handleInstall = async (item: MarketplaceItem) => {
    setInstalling(item.id)
    try {
      let cfg: { system_prompt?: string; tools?: string; config?: string } = {}
      try { cfg = JSON.parse(item.config) } catch { /* ignore */ }
      await agentAPI.installFromMarketplace({
        source_id: item.id,
        name: item.name,
        description: item.description,
        system_prompt: cfg.system_prompt || '',
        tools: cfg.tools || '[]',
        config: cfg.config || '{}',
        icon: item.icon,
      })
      setInstalledIDs(prev => new Set([...prev, item.id]))
      setToast(`已安装「${item.name}」`)
      setTimeout(() => setToast(''), 2500)
    } catch {
      setToast('安装失败')
      setTimeout(() => setToast(''), 2500)
    }
    setInstalling(null)
  }

  const handleUninstall = async (sourceId: string) => {
    if (!confirm('确定要卸载这个智能体吗？')) return
    try {
      await agentAPI.uninstallBySourceID(sourceId)
      setInstalledIDs(prev => { const s = new Set(prev); s.delete(sourceId); return s })
      setToast('已卸载')
      setTimeout(() => setToast(''), 2500)
    } catch { /* ignore */ }
  }

  const parseTags = (tags: string): string[] => {
    if (!tags) return []
    try { return JSON.parse(tags) } catch { return tags.split(',').map(t => t.trim()).filter(Boolean) }
  }

  return (
    <div className="h-full overflow-y-auto">
      <div className="max-w-6xl mx-auto p-8">
        {/* Header */}
        <div className="mb-8">
          <div className="flex items-center gap-3 mb-2">
            <div className="w-10 h-10 bg-gradient-to-br from-violet-500 to-purple-600 rounded-xl flex items-center justify-center">
              <Store className="w-5 h-5 text-white" />
            </div>
            <div>
              <h1 className="text-2xl font-bold text-gray-900 dark:text-white">智能体市场</h1>
              <p className="text-gray-500 dark:text-gray-400 text-sm">发现、安装社区智能体 — 一键添加到我的智能体</p>
            </div>
          </div>
        </div>

        {/* Search */}
        <div className="mb-6">
          <div className="relative">
            <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-gray-400" />
            <input
              value={search}
              onChange={(e) => handleSearch(e.target.value)}
              placeholder="搜索智能体..."
              className="w-full pl-10 pr-4 py-2.5 border dark:border-gray-700 bg-white dark:bg-gray-800 text-gray-900 dark:text-white rounded-xl text-sm outline-none focus:ring-2 focus:ring-primary-500"
            />
          </div>
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
        ) : items.length === 0 ? (
          <div className="text-center py-20 text-gray-400">
            <Store className="w-12 h-12 mx-auto mb-3 opacity-50" />
            <p>{search ? '没有找到匹配的智能体' : '暂无社区智能体'}</p>
          </div>
        ) : (
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
            {items.map(item => {
              const isInstalled = installedIDs.has(item.id)
              const isInstalling = installing === item.id
              return (
                <div key={item.id} onClick={() => navigate(`/marketplace/${item.id}`)} className="bg-white dark:bg-gray-800 border dark:border-gray-700 rounded-xl p-5 hover:shadow-md transition-shadow cursor-pointer">
                  <div className="flex items-start justify-between mb-3">
                    <div className="w-10 h-10 bg-gradient-to-br from-indigo-100 to-purple-100 dark:from-indigo-900/30 dark:to-purple-900/30 rounded-lg flex items-center justify-center">
                      <Bot className="w-5 h-5 text-indigo-600 dark:text-indigo-400" />
                    </div>
                    {isInstalled ? (
                      <button
                        onClick={(e) => { e.stopPropagation(); handleUninstall(item.id); }}
                        className="flex items-center gap-1 px-3 py-1.5 bg-green-50 dark:bg-green-900/20 text-green-700 dark:text-green-400 rounded-lg text-xs font-medium hover:bg-red-50 hover:text-red-600 dark:hover:bg-red-900/20 dark:hover:text-red-400 transition-colors group"
                      >
                        <Check className="w-3.5 h-3.5 group-hover:hidden" />
                        <Trash2 className="w-3.5 h-3.5 hidden group-hover:block" />
                        <span className="group-hover:hidden">已安装</span>
                        <span className="hidden group-hover:inline">卸载</span>
                      </button>
                    ) : (
                      <button
                        onClick={(e) => { e.stopPropagation(); handleInstall(item); }}
                        disabled={isInstalling}
                        className="flex items-center gap-1.5 px-3 py-1.5 text-xs bg-primary-600 text-white rounded-lg hover:bg-primary-700 disabled:opacity-50"
                      >
                        {isInstalling ? <Loader2 className="w-3.5 h-3.5 animate-spin" /> : <Download className="w-3.5 h-3.5" />}
                        安装
                      </button>
                    )}
                  </div>
                  <h3 className="font-semibold text-gray-900 dark:text-white">{item.name}</h3>
                  <p className="text-sm text-gray-500 dark:text-gray-400 mt-1 line-clamp-2">{item.description || '暂无描述'}</p>
                  {parseTags(item.tags).length > 0 && (
                    <div className="flex gap-1 mt-2 flex-wrap">
                      {parseTags(item.tags).slice(0, 3).map(tag => (
                        <span key={tag} className="px-1.5 py-0.5 bg-gray-100 dark:bg-gray-700 text-gray-500 dark:text-gray-400 text-[10px] rounded">{tag}</span>
                      ))}
                    </div>
                  )}
                  <div className="flex items-center justify-between mt-3">
                    <span className="text-xs text-gray-400">{item.author?.nickname || 'StarClaw 官方'}</span>
                    {item.downloads > 0 && <span className="text-[10px] text-gray-400">{item.downloads} 次安装</span>}
                  </div>
                </div>
              )
            })}
          </div>
        )}
      </div>
    </div>
  )
}
