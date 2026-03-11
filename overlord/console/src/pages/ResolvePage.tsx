import { useState } from 'react'
import { Search, CheckCircle, XCircle, Wifi, Server } from 'lucide-react'
import { broodAPI } from '../api/brood'

interface ResolveResult {
  found: boolean
  address?: string
  name?: string
  claw_id?: string
  version?: string
  status?: string
  team?: string
}

export default function ResolvePage() {
  const [query, setQuery] = useState('')
  const [result, setResult] = useState<ResolveResult | null>(null)
  const [loading, setLoading] = useState(false)
  const [searched, setSearched] = useState(false)

  const handleResolve = async () => {
    const q = query.trim()
    if (!q) return
    setLoading(true)
    setSearched(true)
    try {
      const data = await broodAPI.resolve(q)
      setResult(data)
    } catch {
      setResult({ found: false })
    }
    setLoading(false)
  }

  return (
    <div className="p-8 max-w-2xl">
      <div className="mb-8">
        <h1 className="text-2xl font-bold text-white">地址解析</h1>
        <p className="text-sm text-gray-500 mt-1">通过 Claw ID 解析节点网络地址</p>
      </div>

      <div className="flex gap-2 mb-8">
        <input
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          onKeyDown={(e) => e.key === 'Enter' && handleResolve()}
          placeholder="输入 Claw ID（如 claw:b49edd9ceb...）"
          className="flex-1 bg-gray-900 border border-gray-800 rounded-xl px-4 py-3 text-white text-sm font-mono focus:outline-none focus:border-overlord-500 placeholder:text-gray-600"
        />
        <button
          onClick={handleResolve}
          disabled={loading || !query.trim()}
          className="flex items-center gap-2 px-5 py-3 bg-overlord-600 text-white rounded-xl text-sm hover:bg-overlord-500 disabled:opacity-50 transition"
        >
          <Search className="w-4 h-4" />
          解析
        </button>
      </div>

      {loading && (
        <div className="flex justify-center py-8">
          <div className="animate-spin w-6 h-6 border-2 border-overlord-500 border-t-transparent rounded-full" />
        </div>
      )}

      {!loading && searched && result && (
        result.found ? (
          <div className="bg-gray-900 border border-gray-800 rounded-xl p-6">
            <div className="flex items-center gap-3 mb-6">
              <CheckCircle className="w-6 h-6 text-emerald-400" />
              <span className="text-emerald-400 font-medium">解析成功</span>
            </div>
            <div className="space-y-4">
              {[
                { label: '名称', value: result.name, icon: Server },
                { label: '地址', value: result.address, icon: Wifi },
                { label: 'Claw ID', value: result.claw_id, mono: true },
                { label: '版本', value: result.version },
                { label: '状态', value: result.status },
                { label: '团队', value: result.team },
              ].filter(r => r.value).map(({ label, value, icon: Icon, mono }) => (
                <div key={label} className="flex items-start gap-3">
                  {Icon && <Icon className="w-4 h-4 text-gray-500 mt-0.5 shrink-0" />}
                  {!Icon && <div className="w-4 shrink-0" />}
                  <div>
                    <div className="text-xs text-gray-500">{label}</div>
                    <div className={`text-sm text-white ${mono ? 'font-mono' : ''}`}>{value}</div>
                  </div>
                </div>
              ))}
            </div>
          </div>
        ) : (
          <div className="bg-gray-900 border border-gray-800 rounded-xl p-6 text-center">
            <XCircle className="w-8 h-8 text-gray-500 mx-auto mb-3" />
            <p className="text-gray-400">未找到匹配的节点</p>
            <p className="text-xs text-gray-600 mt-1">该 Claw ID 在当前虫群中不存在或已离线</p>
          </div>
        )
      )}

      {!searched && (
        <div className="bg-gray-900/50 border border-gray-800 border-dashed rounded-xl p-8 text-center">
          <Search className="w-8 h-8 text-gray-600 mx-auto mb-3" />
          <p className="text-gray-500 text-sm">输入 Claw ID 开始解析</p>
        </div>
      )}
    </div>
  )
}
