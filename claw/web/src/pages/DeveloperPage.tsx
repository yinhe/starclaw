import { useState, useEffect } from 'react'
import { Download, ExternalLink, Package, Play, Plus, RefreshCw, Search, Star } from 'lucide-react'
import { developerAPI } from '../lib/api'

type Tab = 'plugins' | 'installed' | 'publish' | 'playground'

export default function DeveloperPage() {
  const [tab, setTab] = useState<Tab>('plugins')
  const [plugins, setPlugins] = useState<any[]>([])
  const [installed, setInstalled] = useState<any[]>([])
  const [myPlugins, setMyPlugins] = useState<any[]>([])
  const [categories, setCategories] = useState<any[]>([])
  const [search, setSearch] = useState('')
  const [category, setCategory] = useState('')
  const [loading, setLoading] = useState(false)

  // Playground state
  const [pgMethod, setPgMethod] = useState('GET')
  const [pgPath, setPgPath] = useState('/agents')
  const [pgBody, setPgBody] = useState('')
  const [pgResult, setPgResult] = useState<any>(null)
  const [pgHistory, setPgHistory] = useState<any[]>([])
  const [pgLoading, setPgLoading] = useState(false)

  useEffect(() => { developerAPI.categories().then(r => setCategories(r.data || [])).catch(() => {}) }, [])
  useEffect(() => { loadData() }, [tab])

  const loadData = async () => {
    setLoading(true)
    try {
      if (tab === 'plugins') {
        const r = await developerAPI.listPlugins({ q: search || undefined, category: category || undefined })
        setPlugins(r.data?.items || [])
      } else if (tab === 'installed') {
        const r = await developerAPI.myInstalled()
        setInstalled(r.data || [])
      } else if (tab === 'publish') {
        const r = await developerAPI.myPlugins()
        setMyPlugins(r.data || [])
      } else if (tab === 'playground') {
        const r = await developerAPI.playgroundHistory(20)
        setPgHistory(r.data || [])
      }
    } catch {}
    setLoading(false)
  }

  const installPlugin = async (id: string) => { await developerAPI.installPlugin(id); loadData() }
  const uninstallPlugin = async (id: string) => { await developerAPI.uninstallPlugin(id); loadData() }

  const executePlayground = async () => {
    setPgLoading(true)
    try {
      const r = await developerAPI.playgroundExecute({ method: pgMethod, path: pgPath, body: pgBody || undefined })
      setPgResult(r.data)
    } catch (e: any) {
      setPgResult({ error: e.message })
    }
    setPgLoading(false)
  }

  return (
    <div className="p-6 max-w-7xl mx-auto">
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-2xl font-bold text-gray-900 dark:text-white">开发者门户</h1>
          <p className="text-sm text-gray-500 mt-1">插件市场、API 调试、OpenAPI 文档</p>
        </div>
        <div className="flex gap-2">
          <a href="/v1/developer/docs" target="_blank" className="flex items-center gap-1.5 px-3 py-2 text-sm bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg hover:bg-gray-50">
            <ExternalLink className="w-4 h-4" /> Swagger UI
          </a>
          <button onClick={loadData} className="flex items-center gap-2 px-3 py-2 text-sm bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg hover:bg-gray-50">
            <RefreshCw className={`w-4 h-4 ${loading ? 'animate-spin' : ''}`} />
          </button>
        </div>
      </div>

      <div className="flex gap-1 mb-6 bg-gray-100 dark:bg-gray-800 p-1 rounded-lg w-fit">
        {([['plugins', '插件市场', <Package className="w-4 h-4" key="p" />], ['installed', '已安装', <Download className="w-4 h-4" key="i" />], ['publish', '我的插件', <Plus className="w-4 h-4" key="m" />], ['playground', 'API 调试', <Play className="w-4 h-4" key="a" />]] as [Tab, string, React.ReactNode][]).map(([k, l, i]) => (
          <button key={k} onClick={() => setTab(k)}
            className={`flex items-center gap-1.5 px-3 py-1.5 text-sm rounded-md transition ${tab === k ? 'bg-white dark:bg-gray-700 shadow text-primary-600 font-medium' : 'text-gray-500 hover:text-gray-700'}`}>
            {i} {l}
          </button>
        ))}
      </div>

      {/* Plugin Marketplace */}
      {tab === 'plugins' && (
        <div>
          <div className="flex gap-2 mb-4">
            <div className="relative flex-1 max-w-sm">
              <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-gray-400" />
              <input value={search} onChange={e => setSearch(e.target.value)} onKeyDown={e => e.key === 'Enter' && loadData()}
                placeholder="搜索插件..." className="w-full pl-9 pr-3 py-2 text-sm bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg" />
            </div>
            <select value={category} onChange={e => { setCategory(e.target.value); setTimeout(loadData, 0) }}
              className="px-3 py-2 text-sm bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg">
              <option value="">全部分类</option>
              {categories.map((c: any) => <option key={c.id} value={c.id}>{c.name}</option>)}
            </select>
          </div>
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
            {plugins.map((p: any) => (
              <div key={p.id} className="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-4 hover:shadow-md transition">
                <div className="flex items-start justify-between mb-2">
                  <div className="flex items-center gap-2">
                    <div className="w-10 h-10 bg-primary-100 dark:bg-primary-900 rounded-lg flex items-center justify-center text-lg">{p.icon || '🔌'}</div>
                    <div>
                      <p className="font-medium text-gray-900 dark:text-white">{p.display_name}</p>
                      <p className="text-xs text-gray-400">v{p.version} · by {p.creator?.username || 'unknown'}</p>
                    </div>
                  </div>
                  <span className="px-2 py-0.5 text-xs bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-400 rounded">{p.category}</span>
                </div>
                <p className="text-sm text-gray-500 mb-3 line-clamp-2">{p.description}</p>
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-3 text-xs text-gray-400">
                    <span className="flex items-center gap-0.5"><Star className="w-3 h-3 text-yellow-400" />{p.rating?.toFixed(1) || '0.0'}</span>
                    <span>{p.install_count} 安装</span>
                    <span>{p.pricing === 'free' ? '免费' : `¥${(p.price_cents / 100).toFixed(0)}`}</span>
                  </div>
                  <button onClick={() => installPlugin(p.id)} className="px-3 py-1 text-xs bg-primary-600 text-white rounded hover:bg-primary-700">安装</button>
                </div>
              </div>
            ))}
            {plugins.length === 0 && <p className="col-span-3 text-center text-gray-400 py-8">暂无插件</p>}
          </div>
        </div>
      )}

      {/* Installed */}
      {tab === 'installed' && (
        <div className="space-y-2">
          {installed.length === 0 ? <p className="text-center text-gray-400 py-8">尚未安装任何插件</p> : installed.map((i: any) => (
            <div key={i.id} className="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-4 flex items-center justify-between">
              <div>
                <p className="font-medium text-gray-900 dark:text-white">{i.plugin?.display_name || i.plugin_id?.slice(0, 8)}</p>
                <p className="text-xs text-gray-500">v{i.version} · 安装于 {new Date(i.created_at).toLocaleDateString()}</p>
              </div>
              <button onClick={() => uninstallPlugin(i.plugin_id)} className="px-3 py-1 text-xs text-red-500 border border-red-200 rounded hover:bg-red-50">卸载</button>
            </div>
          ))}
        </div>
      )}

      {/* My Plugins */}
      {tab === 'publish' && (
        <div className="space-y-2">
          {myPlugins.length === 0 ? <p className="text-center text-gray-400 py-8">你还没有发布插件</p> : myPlugins.map((p: any) => (
            <div key={p.id} className="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-4 flex items-center justify-between">
              <div>
                <div className="flex items-center gap-2">
                  <p className="font-medium text-gray-900 dark:text-white">{p.display_name}</p>
                  <span className={`px-2 py-0.5 text-xs rounded ${p.status === 'published' ? 'bg-green-100 text-green-700' : p.status === 'pending_review' ? 'bg-yellow-100 text-yellow-700' : 'bg-gray-100 text-gray-600'}`}>{p.status}</span>
                </div>
                <p className="text-xs text-gray-500 mt-0.5">{p.install_count} 安装 · ⭐ {p.rating?.toFixed(1) || '0.0'}</p>
              </div>
            </div>
          ))}
        </div>
      )}

      {/* API Playground */}
      {tab === 'playground' && (
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
          <div>
            <div className="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-4 space-y-3">
              <p className="font-medium text-gray-900 dark:text-white">API 请求</p>
              <div className="flex gap-2">
                <select value={pgMethod} onChange={e => setPgMethod(e.target.value)} className="px-3 py-2 text-sm bg-gray-50 dark:bg-gray-900 border border-gray-200 dark:border-gray-700 rounded-lg w-24">
                  <option>GET</option><option>POST</option><option>PUT</option><option>DELETE</option>
                </select>
                <input value={pgPath} onChange={e => setPgPath(e.target.value)} placeholder="/agents"
                  className="flex-1 px-3 py-2 text-sm bg-gray-50 dark:bg-gray-900 border border-gray-200 dark:border-gray-700 rounded-lg font-mono" />
              </div>
              {(pgMethod === 'POST' || pgMethod === 'PUT') && (
                <textarea value={pgBody} onChange={e => setPgBody(e.target.value)} placeholder="Request body (JSON)" rows={4}
                  className="w-full px-3 py-2 text-sm bg-gray-50 dark:bg-gray-900 border border-gray-200 dark:border-gray-700 rounded-lg font-mono" />
              )}
              <button onClick={executePlayground} disabled={pgLoading} className="w-full py-2 text-sm bg-primary-600 text-white rounded-lg hover:bg-primary-700 disabled:opacity-50 flex items-center justify-center gap-1.5">
                <Play className="w-4 h-4" /> {pgLoading ? '执行中...' : '执行'}
              </button>
            </div>
            {pgResult && (
              <div className="mt-4 bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-4">
                <div className="flex items-center justify-between mb-2">
                  <p className="font-medium text-gray-900 dark:text-white">响应</p>
                  <div className="flex items-center gap-2 text-xs">
                    <span className={`px-2 py-0.5 rounded ${pgResult.status_code >= 200 && pgResult.status_code < 300 ? 'bg-green-100 text-green-700' : 'bg-red-100 text-red-700'}`}>{pgResult.status_code}</span>
                    <span className="text-gray-400">{pgResult.duration_ms}ms</span>
                  </div>
                </div>
                <pre className="text-xs bg-gray-50 dark:bg-gray-900 p-3 rounded-lg overflow-auto max-h-64 font-mono text-gray-700 dark:text-gray-300">{typeof pgResult.response_body === 'string' ? pgResult.response_body : JSON.stringify(pgResult, null, 2)}</pre>
              </div>
            )}
          </div>
          <div className="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-4">
            <p className="font-medium text-gray-900 dark:text-white mb-3">最近请求</p>
            <div className="space-y-2 max-h-[500px] overflow-y-auto">
              {pgHistory.map((h: any) => (
                <button key={h.id} onClick={() => { setPgMethod(h.method); setPgPath(h.path); setPgBody(h.request_body || '') }}
                  className="w-full text-left p-2 rounded-lg hover:bg-gray-50 dark:hover:bg-gray-700/50 text-sm">
                  <div className="flex items-center gap-2">
                    <span className="px-1.5 py-0.5 text-xs font-mono bg-gray-100 dark:bg-gray-700 rounded">{h.method}</span>
                    <span className="font-mono text-xs text-gray-600 dark:text-gray-400 truncate">{h.path}</span>
                    <span className={`ml-auto px-1.5 py-0.5 text-xs rounded ${h.response_code >= 200 && h.response_code < 300 ? 'bg-green-100 text-green-700' : 'bg-red-100 text-red-700'}`}>{h.response_code}</span>
                  </div>
                </button>
              ))}
              {pgHistory.length === 0 && <p className="text-center text-gray-400 text-sm py-4">暂无历史</p>}
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
