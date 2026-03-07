import { useState, useEffect } from 'react'
import { Bot, Download, Search, User } from 'lucide-react'
import { agentAPI } from '../lib/api'

interface PublicAgent {
  id: string
  name: string
  description: string
  system_prompt: string
  tools: string
  user?: { username: string }
  created_at: string
}

export default function MarketplacePage() {
  const [agents, setAgents] = useState<PublicAgent[]>([])
  const [search, setSearch] = useState('')
  const [cloning, setCloning] = useState<string | null>(null)
  const [cloneMsg, setCloneMsg] = useState('')

  useEffect(() => { loadAgents() }, [])

  const loadAgents = async () => {
    try {
      const res = await agentAPI.marketplace()
      setAgents(res.data.agents || [])
    } catch { /* ignore */ }
  }

  const handleClone = async (id: string) => {
    setCloning(id)
    setCloneMsg('')
    try {
      await agentAPI.clone(id)
      setCloneMsg('已添加到我的 Agent')
      setTimeout(() => setCloneMsg(''), 2000)
    } catch {
      setCloneMsg('克隆失败')
    }
    setCloning(null)
  }

  const filtered = search
    ? agents.filter(
        (a) =>
          a.name.toLowerCase().includes(search.toLowerCase()) ||
          a.description?.toLowerCase().includes(search.toLowerCase()),
      )
    : agents

  const parseTools = (tools: string): string[] => {
    try { return JSON.parse(tools) || [] } catch { return [] }
  }

  return (
    <div className="h-full overflow-y-auto">
      <div className="max-w-5xl mx-auto p-8">
        <div className="mb-8">
          <h1 className="text-2xl font-bold text-gray-900">Agent 市场</h1>
          <p className="text-gray-500 text-sm mt-1">发现和使用社区共享的 AI Agent</p>
        </div>

        {/* Search */}
        <div className="relative mb-6">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-gray-400" />
          <input
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="搜索 Agent..."
            className="w-full pl-10 pr-4 py-2.5 border rounded-xl text-sm outline-none focus:ring-2 focus:ring-primary-500"
          />
        </div>

        {cloneMsg && (
          <div className="mb-4 px-4 py-2 bg-green-50 text-green-700 text-sm rounded-lg">{cloneMsg}</div>
        )}

        {filtered.length === 0 ? (
          <div className="text-center py-20 text-gray-400">
            <Bot className="w-12 h-12 mx-auto mb-3 opacity-50" />
            <p>{search ? '没有找到匹配的 Agent' : '暂无公开的 Agent'}</p>
          </div>
        ) : (
          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            {filtered.map((agent) => (
              <div key={agent.id} className="bg-white border rounded-xl p-5 hover:shadow-md transition-shadow">
                <div className="flex items-start justify-between mb-3">
                  <div className="w-10 h-10 bg-blue-100 rounded-lg flex items-center justify-center">
                    <Bot className="w-5 h-5 text-blue-600" />
                  </div>
                  <button
                    onClick={() => handleClone(agent.id)}
                    disabled={cloning === agent.id}
                    className="flex items-center gap-1.5 px-3 py-1.5 text-xs bg-primary-600 text-white rounded-lg hover:bg-primary-700 disabled:opacity-50"
                  >
                    <Download className="w-3.5 h-3.5" />
                    {cloning === agent.id ? '添加中...' : '添加到我的'}
                  </button>
                </div>
                <h3 className="font-semibold text-gray-900">{agent.name}</h3>
                <p className="text-sm text-gray-500 mt-1 line-clamp-2">
                  {agent.description || '暂无描述'}
                </p>
                {agent.system_prompt && (
                  <p className="text-xs text-gray-400 mt-2 line-clamp-2 italic">
                    {agent.system_prompt}
                  </p>
                )}
                <div className="flex items-center justify-between mt-3">
                  <div className="flex items-center gap-1 text-xs text-gray-400">
                    <User className="w-3 h-3" />
                    {agent.user?.username || '匿名'}
                  </div>
                  {parseTools(agent.tools).length > 0 && (
                    <div className="flex gap-1">
                      {parseTools(agent.tools).map((t) => (
                        <span key={t} className="px-1.5 py-0.5 bg-gray-100 text-gray-500 text-xs rounded">
                          {t}
                        </span>
                      ))}
                    </div>
                  )}
                </div>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  )
}
