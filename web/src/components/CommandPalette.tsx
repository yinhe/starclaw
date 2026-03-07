import { useState, useEffect, useRef } from 'react'
import { useNavigate } from 'react-router-dom'
import { Search, MessageSquare, Bot, GitBranch, BookOpen, Plug, Users, Settings, LayoutDashboard, Store, X } from 'lucide-react'

const commands = [
  { label: '新建对话', to: '/chat', icon: MessageSquare, shortcut: 'N' },
  { label: '仪表盘', to: '/dashboard', icon: LayoutDashboard },
  { label: 'Agents', to: '/agents', icon: Bot },
  { label: 'Agent 市场', to: '/marketplace', icon: Store },
  { label: '工作流', to: '/workflows', icon: GitBranch },
  { label: '知识库', to: '/knowledge', icon: BookOpen },
  { label: 'MCP 工具', to: '/mcp', icon: Plug },
  { label: '多 Agent', to: '/multi-agent', icon: Users },
  { label: '设置', to: '/settings', icon: Settings },
]

export default function CommandPalette() {
  const [open, setOpen] = useState(false)
  const [query, setQuery] = useState('')
  const inputRef = useRef<HTMLInputElement>(null)
  const navigate = useNavigate()

  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if ((e.ctrlKey || e.metaKey) && e.key === 'k') {
        e.preventDefault()
        setOpen((v) => !v)
      }
      if ((e.ctrlKey || e.metaKey) && e.key === 'n') {
        e.preventDefault()
        navigate('/chat')
      }
      if (e.key === 'Escape') setOpen(false)
    }
    window.addEventListener('keydown', handler)
    return () => window.removeEventListener('keydown', handler)
  }, [navigate])

  useEffect(() => {
    if (open) {
      setQuery('')
      setTimeout(() => inputRef.current?.focus(), 50)
    }
  }, [open])

  const filtered = query
    ? commands.filter((c) => c.label.toLowerCase().includes(query.toLowerCase()))
    : commands

  const handleSelect = (to: string) => {
    navigate(to)
    setOpen(false)
  }

  if (!open) return null

  return (
    <div className="fixed inset-0 z-50 flex items-start justify-center pt-[20vh]">
      <div className="fixed inset-0 bg-black/40" onClick={() => setOpen(false)} />
      <div className="relative w-full max-w-md bg-white dark:bg-gray-800 rounded-xl shadow-2xl border dark:border-gray-700 overflow-hidden">
        <div className="flex items-center gap-3 px-4 py-3 border-b dark:border-gray-700">
          <Search className="w-4 h-4 text-gray-400" />
          <input
            ref={inputRef}
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder="搜索页面或功能..."
            className="flex-1 bg-transparent text-sm outline-none text-gray-800 dark:text-gray-200 placeholder-gray-400"
          />
          <button onClick={() => setOpen(false)} className="text-gray-400 hover:text-gray-600">
            <X className="w-4 h-4" />
          </button>
        </div>
        <div className="max-h-64 overflow-y-auto py-1">
          {filtered.length === 0 ? (
            <p className="text-center text-sm text-gray-400 py-6">无匹配结果</p>
          ) : (
            filtered.map((cmd) => (
              <button
                key={cmd.to}
                onClick={() => handleSelect(cmd.to)}
                className="w-full flex items-center gap-3 px-4 py-2.5 text-sm text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors"
              >
                <cmd.icon className="w-4 h-4 text-gray-400" />
                <span className="flex-1 text-left">{cmd.label}</span>
                {cmd.shortcut && (
                  <kbd className="text-xs text-gray-400 bg-gray-100 dark:bg-gray-700 px-1.5 py-0.5 rounded">
                    Ctrl+{cmd.shortcut}
                  </kbd>
                )}
              </button>
            ))
          )}
        </div>
        <div className="px-4 py-2 border-t dark:border-gray-700 flex items-center gap-4 text-xs text-gray-400">
          <span><kbd className="bg-gray-100 dark:bg-gray-700 px-1 rounded">↑↓</kbd> 导航</span>
          <span><kbd className="bg-gray-100 dark:bg-gray-700 px-1 rounded">Enter</kbd> 打开</span>
          <span><kbd className="bg-gray-100 dark:bg-gray-700 px-1 rounded">Esc</kbd> 关闭</span>
        </div>
      </div>
    </div>
  )
}
