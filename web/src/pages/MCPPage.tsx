import { useState, useEffect } from 'react'
import { Plug, Plus, Trash2, RefreshCw, X, CheckCircle, AlertCircle, Loader2 } from 'lucide-react'
import { mcpAPI } from '../lib/api'

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

  useEffect(() => { loadServers() }, [])

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
            <h1 className="text-2xl font-bold text-gray-900">MCP 工具市场</h1>
            <p className="text-gray-500 text-sm mt-1">连接外部 MCP 工具服务器，扩展 Agent 能力</p>
          </div>
          <button
            onClick={() => { setShowModal(true); setAddError('') }}
            className="flex items-center gap-2 px-4 py-2 bg-primary-600 text-white rounded-lg text-sm hover:bg-primary-700"
          >
            <Plus className="w-4 h-4" /> 添加服务器
          </button>
        </div>

        {servers.length === 0 ? (
          <div className="text-center py-20 text-gray-400">
            <Plug className="w-12 h-12 mx-auto mb-3 opacity-50" />
            <p>还没有连接 MCP 服务器</p>
            <p className="text-xs mt-1">MCP 服务器提供外部工具供 Agent 调用</p>
          </div>
        ) : (
          <div className="space-y-3">
            {servers.map((s) => (
              <div key={s.id} className="bg-white border rounded-xl p-5 flex items-center justify-between group">
                <div className="flex items-center gap-4">
                  <div className={`w-10 h-10 rounded-lg flex items-center justify-center ${
                    s.status === 'active' ? 'bg-green-100' : 'bg-red-100'
                  }`}>
                    <Plug className={`w-5 h-5 ${s.status === 'active' ? 'text-green-600' : 'text-red-600'}`} />
                  </div>
                  <div>
                    <div className="flex items-center gap-2">
                      <h3 className="font-semibold text-gray-900">{s.name}</h3>
                      {s.status === 'active' ? (
                        <CheckCircle className="w-4 h-4 text-green-500" />
                      ) : (
                        <AlertCircle className="w-4 h-4 text-red-500" />
                      )}
                    </div>
                    <div className="flex items-center gap-3 text-xs text-gray-400 mt-0.5">
                      <span className="truncate max-w-[300px]">{s.base_url}</span>
                      <span>{s.tool_count} 工具</span>
                    </div>
                  </div>
                </div>
                <div className="flex items-center gap-2">
                  <button
                    onClick={() => handleTest(s.id)}
                    disabled={testing === s.id}
                    className="p-2 text-gray-400 hover:text-primary-600 rounded-lg hover:bg-primary-50 transition-colors"
                    title="测试连接"
                  >
                    {testing === s.id ? (
                      <Loader2 className="w-4 h-4 animate-spin" />
                    ) : (
                      <RefreshCw className="w-4 h-4" />
                    )}
                  </button>
                  <button
                    onClick={() => handleDelete(s.id)}
                    className="p-2 text-gray-400 hover:text-red-500 rounded-lg hover:bg-red-50 opacity-0 group-hover:opacity-100 transition-opacity"
                  >
                    <Trash2 className="w-4 h-4" />
                  </button>
                </div>
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
