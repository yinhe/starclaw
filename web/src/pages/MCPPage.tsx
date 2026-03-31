import { useState, useEffect } from 'react'
import { Plug, PlugZap, Plus, Trash2, RefreshCw, X, CheckCircle, AlertCircle, Loader2, Monitor } from 'lucide-react'
import { mcpAPI, systemAPI } from '../lib/api'

interface MCPServer {
  id: string
  name: string
  base_url: string
  api_key: string
  status: string
  tool_count: number
  created_at: string
}

export default function MCPPage() {
  const [servers, setServers] = useState<MCPServer[]>([])
  const [showModal, setShowModal] = useState(false)
  const [testing, setTesting] = useState<string | null>(null)
  const [form, setForm] = useState({ name: '', base_url: '', api_key: '' })
  const [adding, setAdding] = useState(false)
  const [addError, setAddError] = useState('')
  const [bridge, setBridge] = useState<any>(null)
  const [expandedServer, setExpandedServer] = useState<string | null>(null)
  const [serverTools, setServerTools] = useState<Record<string, any[]>>({})

  useEffect(() => { loadServers(); loadBridge() }, [])

  // Load tools when a server is expanded
  useEffect(() => {
    if (!expandedServer || serverTools[expandedServer]) return
    const srv = servers.find(s => s.id === expandedServer)
    if (!srv) return
    // Fetch tool list from the MCP server via our API
    mcpAPI.testServer(expandedServer).then(res => {
      setServerTools(prev => ({ ...prev, [expandedServer]: res.data?.tools || [] }))
    }).catch(() => {
      // Fallback: generate placeholder tools from tool_count
      const placeholders = Array.from({ length: srv.tool_count }, (_, i) => ({ name: `tool_${i + 1}`, description: '' }))
      setServerTools(prev => ({ ...prev, [expandedServer]: placeholders }))
    })
  }, [expandedServer, servers])

  const loadBridge = async () => {
    try {
      const res = await systemAPI.getBridge()
      setBridge(res.data)
    } catch { /* ignore */ }
  }

  const loadServers = async () => {
    try {
      const res = await mcpAPI.listServers()
      setServers(res.data.servers || [])
    } catch { /* ignore */ }
  }

  const handleAdd = async () => {
    setAdding(true)
    setAddError('')
    try {
      await mcpAPI.addServer(form)
      setShowModal(false)
      setForm({ name: '', base_url: '', api_key: '' })
      loadServers()
    } catch (e: unknown) {
      const err = e as { response?: { data?: { error?: string } } }
      setAddError(err.response?.data?.error || '连接失败')
    }
    setAdding(false)
  }

  const handleDelete = async (id: string) => {
    if (!confirm('确定要移除这个 MCP 服务器吗？')) return
    try {
      await mcpAPI.deleteServer(id)
      loadServers()
    } catch { /* ignore */ }
  }

  const handleTest = async (id: string) => {
    setTesting(id)
    try {
      await mcpAPI.testServer(id)
      loadServers()
    } catch { /* ignore */ }
    setTesting(null)
  }

  return (
    <div className="h-full overflow-y-auto">
      <div className="max-w-5xl mx-auto p-8">
        <div className="flex items-center justify-between mb-8">
          <div>
            <h1 className="text-2xl font-bold text-gray-900 dark:text-white flex items-center gap-2"><Plug className="w-6 h-6 text-purple-500" /> 外接服务</h1>
            <p className="text-gray-500 text-sm mt-1">MCP 外部工具服务 — 点击展开查看暴露的工具列表</p>
          </div>
          <button
            onClick={() => { setShowModal(true); setAddError('') }}
            className="flex items-center gap-2 px-4 py-2 bg-primary-600 text-white rounded-lg text-sm hover:bg-primary-700"
          >
            <Plus className="w-4 h-4" /> 添加服务器
          </button>
        </div>

        {/* Built-in MCP Bridge */}
        {bridge && (
          <div className={`border rounded-xl p-5 mb-6 flex items-center justify-between ${bridge.connected ? 'bg-green-50 border-green-200' : 'bg-gray-50 border-gray-200'}`}>
            <div className="flex items-center gap-4">
              <div className={`w-10 h-10 rounded-lg flex items-center justify-center ${bridge.connected ? 'bg-green-100' : 'bg-gray-100'}`}>
                <Monitor className={`w-5 h-5 ${bridge.connected ? 'text-green-600' : 'text-gray-400'}`} />
              </div>
              <div>
                <div className="flex items-center gap-2">
                  <h3 className="font-semibold text-gray-900">宿主机控制 (内置)</h3>
                  {bridge.connected ? (
                    <span className="flex items-center gap-1 text-xs text-green-600 bg-green-100 px-2 py-0.5 rounded-full"><PlugZap className="w-3 h-3" /> 已连接</span>
                  ) : (
                    <span className="text-xs text-gray-400 bg-gray-100 px-2 py-0.5 rounded-full">未连接</span>
                  )}
                </div>
                <div className="text-xs text-gray-400 mt-0.5">
                  {bridge.connected
                    ? <>{bridge.bridge_url} · {bridge.tool_count || '?'} 个工具: {(bridge.tool_names || []).join(', ') || '加载中...'}</>
                    : '在宿主机运行 MCP Bridge 即可启用，详见 设置 → 宿主机控制'
                  }
                </div>
              </div>
            </div>
          </div>
        )}

        {servers.length === 0 && !bridge?.connected ? (
          <div className="text-center py-20 text-gray-400">
            <Plug className="w-12 h-12 mx-auto mb-3 opacity-50" />
            <p>还没有连接 MCP 外接服务</p>
            <p className="text-xs mt-1">安装智能体时自动注册，或手动添加</p>
          </div>
        ) : servers.length === 0 ? null : (
          <div className="space-y-3">
            {servers.map((s) => (
              <div key={s.id} className="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-xl overflow-hidden group">
                <div className="flex items-center justify-between p-5 cursor-pointer" onClick={() => setExpandedServer(expandedServer === s.id ? null : s.id)}>
                  <div className="flex items-center gap-4">
                    <div className={`w-10 h-10 rounded-lg flex items-center justify-center ${
                      s.status === 'active' ? 'bg-green-100 dark:bg-green-900/30' : 'bg-red-100 dark:bg-red-900/30'
                    }`}>
                      <Plug className={`w-5 h-5 ${s.status === 'active' ? 'text-green-600' : 'text-red-600'}`} />
                    </div>
                    <div>
                      <div className="flex items-center gap-2">
                        <h3 className="font-semibold text-gray-900 dark:text-white">{s.name}</h3>
                        {s.status === 'active' ? (
                          <CheckCircle className="w-4 h-4 text-green-500" />
                        ) : (
                          <AlertCircle className="w-4 h-4 text-red-500" />
                        )}
                      </div>
                      <div className="flex items-center gap-3 text-xs text-gray-400 mt-0.5">
                        <span className="truncate max-w-[300px]">{s.base_url}</span>
                        <span className="font-medium">{s.tool_count} 个工具</span>
                      </div>
                    </div>
                  </div>
                  <div className="flex items-center gap-2">
                    <button
                      onClick={(e) => { e.stopPropagation(); handleTest(s.id) }}
                      disabled={testing === s.id}
                      className="p-2 text-gray-400 hover:text-primary-600 rounded-lg hover:bg-primary-50 dark:hover:bg-primary-900/20 transition-colors"
                      title="测试连接"
                    >
                      {testing === s.id ? <Loader2 className="w-4 h-4 animate-spin" /> : <RefreshCw className="w-4 h-4" />}
                    </button>
                    <button
                      onClick={(e) => { e.stopPropagation(); handleDelete(s.id) }}
                      className="p-2 text-gray-400 hover:text-red-500 rounded-lg hover:bg-red-50 dark:hover:bg-red-900/20 opacity-0 group-hover:opacity-100 transition-opacity"
                    >
                      <Trash2 className="w-4 h-4" />
                    </button>
                  </div>
                </div>
                {/* Expanded: show tool list */}
                {expandedServer === s.id && serverTools[s.id] && (
                  <div className="border-t border-gray-100 dark:border-gray-700 px-5 py-3 bg-gray-50 dark:bg-gray-750">
                    <div className="text-xs font-medium text-gray-500 mb-2">暴露的工具 ({serverTools[s.id].length})</div>
                    <div className="grid grid-cols-1 sm:grid-cols-2 gap-2">
                      {serverTools[s.id].map((tool: any, i: number) => (
                        <div key={i} className="flex items-start gap-2 px-3 py-2 bg-white dark:bg-gray-800 rounded-lg border border-gray-100 dark:border-gray-700">
                          <span className="text-purple-500 mt-0.5">⚡</span>
                          <div className="min-w-0">
                            <div className="text-xs font-medium text-gray-800 dark:text-gray-200">{tool.name || tool}</div>
                            {tool.description && <div className="text-[10px] text-gray-400 truncate">{tool.description}</div>}
                          </div>
                        </div>
                      ))}
                    </div>
                  </div>
                )}
                {expandedServer === s.id && !serverTools[s.id] && (
                  <div className="border-t border-gray-100 dark:border-gray-700 px-5 py-4 text-center text-xs text-gray-400">
                    <Loader2 className="w-4 h-4 animate-spin inline mr-1" /> 加载工具列表...
                  </div>
                )}
              </div>
            ))}
          </div>
        )}
      </div>

      {/* Add Server Modal */}
      {showModal && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
          <div className="bg-white rounded-2xl w-full max-w-lg mx-4 p-6">
            <div className="flex items-center justify-between mb-4">
              <h2 className="text-lg font-semibold">添加 MCP 服务器</h2>
              <button onClick={() => setShowModal(false)} className="text-gray-400 hover:text-gray-600">
                <X className="w-5 h-5" />
              </button>
            </div>
            <div className="space-y-4">
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">名称</label>
                <input
                  value={form.name}
                  onChange={(e) => setForm({ ...form, name: e.target.value })}
                  className="w-full px-3 py-2 border rounded-lg text-sm outline-none focus:ring-2 focus:ring-primary-500"
                  placeholder="例如：weather-tools"
                />
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">服务器地址</label>
                <input
                  value={form.base_url}
                  onChange={(e) => setForm({ ...form, base_url: e.target.value })}
                  className="w-full px-3 py-2 border rounded-lg text-sm outline-none focus:ring-2 focus:ring-primary-500"
                  placeholder="http://localhost:3001/mcp"
                />
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">API Key（可选）</label>
                <input
                  value={form.api_key}
                  onChange={(e) => setForm({ ...form, api_key: e.target.value })}
                  className="w-full px-3 py-2 border rounded-lg text-sm outline-none focus:ring-2 focus:ring-primary-500"
                  placeholder="可选"
                  type="password"
                />
              </div>
              {addError && (
                <div className="text-sm text-red-600 bg-red-50 rounded-lg px-3 py-2">
                  {addError}
                </div>
              )}
            </div>
            <div className="flex justify-end gap-3 mt-6">
              <button onClick={() => setShowModal(false)} className="px-4 py-2 text-sm text-gray-600 hover:bg-gray-100 rounded-lg">
                取消
              </button>
              <button
                onClick={handleAdd}
                disabled={!form.name || !form.base_url || adding}
                className="px-4 py-2 text-sm bg-primary-600 text-white rounded-lg hover:bg-primary-700 disabled:opacity-50"
              >
                {adding ? '连接中...' : '添加并连接'}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
