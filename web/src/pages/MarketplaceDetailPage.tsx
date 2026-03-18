import { useState, useEffect } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import {
  ArrowLeft, Download, Check, Trash2, Loader2, Bot,
  Music, Video, Code2, BookOpen, Briefcase, Clapperboard,
  BarChart3, PenTool, Server, Palette, Search,
  Wrench, Zap, GitBranch, type LucideIcon,
} from 'lucide-react'
import { agentAPI, queenMarketplaceAPI } from '../lib/api'

const ICON_MAP: Record<string, LucideIcon> = {
  Music, Video, Code2, Search, BookOpen, Briefcase, Clapperboard,
  BarChart3, PenTool, Server, Palette, Bot,
}

function AgentIcon({ name, className }: { name?: string; className?: string }) {
  const Icon = name ? ICON_MAP[name] : null
  if (Icon) return <Icon className={className} />
  return <Bot className={className} />
}

const TOOL_LABELS: Record<string, { label: string; desc: string; color: string }> = {
  video_generation: { label: '视频生成', desc: '生成AI视频片段', color: 'bg-blue-50 text-blue-700 border-blue-200 dark:bg-blue-900/20 dark:text-blue-400 dark:border-blue-800' },
  dubbing: { label: '配音字幕', desc: '为视频添加配音和字幕', color: 'bg-purple-50 text-purple-700 border-purple-200 dark:bg-purple-900/20 dark:text-purple-400 dark:border-purple-800' },
  mv_production: { label: 'MV合成', desc: '合成音乐视频', color: 'bg-fuchsia-50 text-fuchsia-700 border-fuchsia-200 dark:bg-fuchsia-900/20 dark:text-fuchsia-400 dark:border-fuchsia-800' },
  comic_production: { label: '漫剧制作', desc: '生成漫画风格视频', color: 'bg-orange-50 text-orange-700 border-orange-200 dark:bg-orange-900/20 dark:text-orange-400 dark:border-orange-800' },
  music_generation: { label: '音乐生成', desc: '作曲、生成歌曲', color: 'bg-pink-50 text-pink-700 border-pink-200 dark:bg-pink-900/20 dark:text-pink-400 dark:border-pink-800' },
  image_generation: { label: '图片生成', desc: '生成AI图片', color: 'bg-cyan-50 text-cyan-700 border-cyan-200 dark:bg-cyan-900/20 dark:text-cyan-400 dark:border-cyan-800' },
  code: { label: '代码执行', desc: '在沙盒中执行代码', color: 'bg-emerald-50 text-emerald-700 border-emerald-200 dark:bg-emerald-900/20 dark:text-emerald-400 dark:border-emerald-800' },
  code_sandbox: { label: '代码沙盒', desc: '安全的代码执行环境', color: 'bg-emerald-50 text-emerald-700 border-emerald-200 dark:bg-emerald-900/20 dark:text-emerald-400 dark:border-emerald-800' },
  web_search: { label: '网页搜索', desc: '搜索互联网信息', color: 'bg-amber-50 text-amber-700 border-amber-200 dark:bg-amber-900/20 dark:text-amber-400 dark:border-amber-800' },
  browser: { label: '浏览器', desc: '浏览和抓取网页', color: 'bg-indigo-50 text-indigo-700 border-indigo-200 dark:bg-indigo-900/20 dark:text-indigo-400 dark:border-indigo-800' },
  http_request: { label: 'HTTP请求', desc: '调用外部API', color: 'bg-teal-50 text-teal-700 border-teal-200 dark:bg-teal-900/20 dark:text-teal-400 dark:border-teal-800' },
  system: { label: '系统管理', desc: '系统级操作', color: 'bg-rose-50 text-rose-700 border-rose-200 dark:bg-rose-900/20 dark:text-rose-400 dark:border-rose-800' },
}

interface MarketplaceItem {
  id: string
  name: string
  description: string
  icon: string
  tags: string
  config: string
  downloads: number
  rating: number
  version?: string
  author?: { nickname?: string }
}

export default function MarketplaceDetailPage() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const [item, setItem] = useState<MarketplaceItem | null>(null)
  const [loading, setLoading] = useState(true)
  const [isInstalled, setIsInstalled] = useState(false)
  const [installing, setInstalling] = useState(false)
  const [toast, setToast] = useState('')

  useEffect(() => {
    if (!id) return
    setLoading(true)
    Promise.all([
      queenMarketplaceAPI.get(id),
      agentAPI.installedSourceIDs(),
    ]).then(([itemRes, idsRes]) => {
      setItem(itemRes.data.item)
      const ids = new Set(idsRes.data.source_ids || [])
      setIsInstalled(ids.has(id))
    }).catch(() => {
      setItem(null)
    }).finally(() => setLoading(false))
  }, [id])

  const parseTags = (tags: string): string[] => {
    if (!tags) return []
    try { return JSON.parse(tags) } catch { return tags.split(',').map(t => t.trim()).filter(Boolean) }
  }

  const parseConfig = (config: string) => {
    try { return JSON.parse(config) } catch { return {} }
  }

  const parseTools = (toolsStr: string): string[] => {
    try { return JSON.parse(toolsStr) } catch { return [] }
  }

  const handleInstall = async () => {
    if (!item) return
    setInstalling(true)
    try {
      const cfg = parseConfig(item.config)
      await agentAPI.installFromMarketplace({
        source_id: item.id,
        name: item.name,
        description: item.description,
        system_prompt: cfg.system_prompt || '',
        tools: cfg.tools || '[]',
        config: cfg.config || '{}',
        icon: item.icon,
      })
      setIsInstalled(true)
      setToast(`已安装「${item.name}」`)
      setTimeout(() => setToast(''), 2500)
    } catch {
      setToast('安装失败')
      setTimeout(() => setToast(''), 2500)
    }
    setInstalling(false)
  }

  const handleUninstall = async () => {
    if (!item || !confirm('确定要卸载这个智能体吗？')) return
    try {
      await agentAPI.uninstallBySourceID(item.id)
      setIsInstalled(false)
      setToast('已卸载')
      setTimeout(() => setToast(''), 2500)
    } catch { /* ignore */ }
  }

  if (loading) {
    return (
      <div className="h-full flex items-center justify-center">
        <Loader2 className="w-6 h-6 animate-spin text-gray-400" />
      </div>
    )
  }

  if (!item) {
    return (
      <div className="h-full flex flex-col items-center justify-center text-gray-400">
        <p className="mb-4">未找到该智能体</p>
        <button onClick={() => navigate('/marketplace')} className="text-primary-600 hover:underline text-sm">← 返回市场</button>
      </div>
    )
  }

  const cfg = parseConfig(item.config)
  const tools = parseTools(cfg.tools || '[]')
  const tags = parseTags(item.tags)

  return (
    <div className="h-full overflow-y-auto">
      <div className="max-w-4xl mx-auto p-8">
        {/* Toast */}
        {toast && (
          <div className="mb-4 px-4 py-2 bg-green-50 dark:bg-green-900/20 text-green-700 dark:text-green-400 text-sm rounded-lg">{toast}</div>
        )}

        {/* Back */}
        <button onClick={() => navigate('/marketplace')}
          className="inline-flex items-center gap-1.5 text-sm text-gray-500 hover:text-primary-600 mb-6 transition">
          <ArrowLeft className="w-4 h-4" />返回市场
        </button>

        {/* Hero */}
        <div className="bg-white dark:bg-gray-800 rounded-2xl border dark:border-gray-700 p-6">
          <div className="flex items-start gap-4">
            <div className="w-14 h-14 rounded-2xl bg-gradient-to-br from-indigo-100 to-purple-100 dark:from-indigo-900/30 dark:to-purple-900/30 flex items-center justify-center shrink-0">
              <AgentIcon name={item.icon} className="w-7 h-7 text-indigo-600 dark:text-indigo-400" />
            </div>
            <div className="flex-1 min-w-0">
              <h1 className="text-xl font-bold text-gray-900 dark:text-white mb-1">{item.name}</h1>
              <p className="text-gray-500 dark:text-gray-400 text-sm mb-2">by {item.author?.nickname || 'StarClaw 官方'} · v{item.version || '1.0.0'}</p>
              <p className="text-gray-600 dark:text-gray-300 text-sm leading-relaxed">{item.description}</p>
              {tags.length > 0 && (
                <div className="flex gap-1.5 mt-3 flex-wrap">
                  {tags.map(tag => (
                    <span key={tag} className="px-2 py-0.5 bg-gray-100 dark:bg-gray-700 text-gray-500 dark:text-gray-400 text-[11px] rounded-full">{tag}</span>
                  ))}
                </div>
              )}
            </div>
            <div className="shrink-0">
              {isInstalled ? (
                <button onClick={handleUninstall}
                  className="flex items-center gap-1.5 px-4 py-2 bg-green-50 dark:bg-green-900/20 text-green-700 dark:text-green-400 rounded-xl text-sm font-medium hover:bg-red-50 hover:text-red-600 dark:hover:bg-red-900/20 dark:hover:text-red-400 transition-colors group">
                  <Check className="w-4 h-4 group-hover:hidden" />
                  <Trash2 className="w-4 h-4 hidden group-hover:block" />
                  <span className="group-hover:hidden">已安装</span>
                  <span className="hidden group-hover:inline">卸载</span>
                </button>
              ) : (
                <button onClick={handleInstall} disabled={installing}
                  className="flex items-center gap-1.5 px-4 py-2 bg-primary-600 text-white rounded-xl text-sm font-medium hover:bg-primary-700 disabled:opacity-50 transition">
                  {installing ? <Loader2 className="w-4 h-4 animate-spin" /> : <Download className="w-4 h-4" />}
                  安装
                </button>
              )}
            </div>
          </div>

          {/* Stats */}
          <div className="flex gap-6 mt-5 pt-5 border-t dark:border-gray-700">
            <div className="text-center">
              <p className="text-lg font-bold text-gray-900 dark:text-white">{item.downloads}</p>
              <p className="text-xs text-gray-400">安装次数</p>
            </div>
            <div className="text-center">
              <p className="text-lg font-bold text-gray-900 dark:text-white">{item.rating > 0 ? item.rating.toFixed(1) : '-'}</p>
              <p className="text-xs text-gray-400">评分</p>
            </div>
          </div>
        </div>

        {/* Capabilities grid */}
        <div className="grid md:grid-cols-2 gap-5 mt-6">
          {/* Skills (tools) */}
          <div className="bg-white dark:bg-gray-800 rounded-2xl border dark:border-gray-700 p-5">
            <div className="flex items-center gap-2 mb-3">
              <Wrench className="w-4 h-4 text-indigo-600 dark:text-indigo-400" />
              <h2 className="font-bold text-sm text-gray-900 dark:text-white">技能（被动）</h2>
              <span className="text-xs text-gray-400">可调用的工具</span>
            </div>
            {tools.length === 0 ? (
              <p className="text-sm text-gray-400 py-4 text-center">暂无内置技能</p>
            ) : (
              <div className="space-y-2">
                {tools.map(tool => {
                  const info = TOOL_LABELS[tool] || { label: tool, desc: '', color: 'bg-gray-50 text-gray-600 border-gray-200 dark:bg-gray-700 dark:text-gray-400 dark:border-gray-600' }
                  return (
                    <div key={tool} className={`flex items-center gap-3 px-3 py-2 rounded-xl border ${info.color}`}>
                      <Wrench className="w-3.5 h-3.5 shrink-0" />
                      <div className="min-w-0">
                        <p className="text-xs font-medium">{info.label}</p>
                        {info.desc && <p className="text-[10px] opacity-70">{info.desc}</p>}
                      </div>
                    </div>
                  )
                })}
              </div>
            )}
          </div>

          {/* Workflow & Instinct */}
          <div className="space-y-5">
            <div className="bg-white dark:bg-gray-800 rounded-2xl border dark:border-gray-700 p-5">
              <div className="flex items-center gap-2 mb-3">
                <GitBranch className="w-4 h-4 text-green-600 dark:text-green-400" />
                <h2 className="font-bold text-sm text-gray-900 dark:text-white">工作流</h2>
                <span className="text-xs text-gray-400">预设执行流程</span>
              </div>
              <p className="text-sm text-gray-400 py-4 text-center">暂无预设工作流</p>
            </div>

            <div className="bg-white dark:bg-gray-800 rounded-2xl border dark:border-gray-700 p-5">
              <div className="flex items-center gap-2 mb-3">
                <Zap className="w-4 h-4 text-amber-500" />
                <h2 className="font-bold text-sm text-gray-900 dark:text-white">本能（主动）</h2>
                <span className="text-xs text-gray-400">Agent 自发行为</span>
              </div>
              <p className="text-sm text-gray-400 py-4 text-center">暂无主动行为配置</p>
            </div>
          </div>
        </div>

        {/* System Prompt */}
        {cfg.system_prompt && (
          <div className="bg-white dark:bg-gray-800 rounded-2xl border dark:border-gray-700 p-5 mt-6">
            <h2 className="font-bold text-sm text-gray-900 dark:text-white mb-3">系统提示词</h2>
            <div className="bg-gray-50 dark:bg-gray-900 rounded-xl p-4 text-gray-600 dark:text-gray-300 whitespace-pre-wrap leading-relaxed max-h-[400px] overflow-y-auto font-mono text-xs">
              {cfg.system_prompt}
            </div>
          </div>
        )}
      </div>
    </div>
  )
}
