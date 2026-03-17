import { useState, useRef, useEffect } from 'react'
import { Send, Loader2, Bot, User, Sparkles, RotateCcw } from 'lucide-react'

interface Message {
  id: string
  role: 'user' | 'assistant' | 'system'
  content: string
  timestamp: number
}

export default function ChatPage() {
  const [messages, setMessages] = useState<Message[]>([
    {
      id: 'welcome',
      role: 'system',
      content: '欢迎使用 StarClaw AI 智能体。请选择一个 Agent 或直接开始对话。',
      timestamp: Date.now(),
    },
  ])
  const [input, setInput] = useState('')
  const [loading, setLoading] = useState(false)
  const [selectedAgent, setSelectedAgent] = useState('default')
  const bottomRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [messages])

  const agents = [
    { id: 'default', name: '通用助手', icon: '🤖', desc: '通用 AI 对话' },
    { id: 'code', name: '代码助手', icon: '💻', desc: '编程与代码审查' },
    { id: 'doc', name: '文档助手', icon: '📄', desc: '文档撰写与总结' },
    { id: 'data', name: '数据分析', icon: '📊', desc: '数据分析与可视化' },
  ]

  const handleSend = async () => {
    if (!input.trim() || loading) return

    const userMsg: Message = {
      id: `user-${Date.now()}`,
      role: 'user',
      content: input.trim(),
      timestamp: Date.now(),
    }

    setMessages(prev => [...prev, userMsg])
    setInput('')
    setLoading(true)

    // Simulate AI response (will be replaced with real Claw proxy)
    setTimeout(() => {
      const aiMsg: Message = {
        id: `ai-${Date.now()}`,
        role: 'assistant',
        content: `[${agents.find(a => a.id === selectedAgent)?.name}] 收到您的消息：「${userMsg.content}」\n\n这是一条模拟回复。实际部署后，消息将通过 Overlord 路由到管辖的 Claw 节点进行 AI 推理。\n\n当前 Agent: ${selectedAgent}\n消息时间: ${new Date().toLocaleTimeString()}`,
        timestamp: Date.now(),
      }
      setMessages(prev => [...prev, aiMsg])
      setLoading(false)
    }, 800 + Math.random() * 1200)
  }

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault()
      handleSend()
    }
  }

  const handleClear = () => {
    setMessages([{
      id: 'welcome',
      role: 'system',
      content: '对话已清空。请开始新的对话。',
      timestamp: Date.now(),
    }])
  }

  return (
    <div className="flex h-full">
      {/* Agent selector sidebar */}
      <aside className="w-56 bg-gray-900/50 border-r border-gray-800 flex flex-col">
        <div className="px-4 py-4 border-b border-gray-800">
          <h2 className="text-xs font-semibold text-gray-400 uppercase tracking-wider">智能体</h2>
        </div>
        <div className="flex-1 px-2 py-2 space-y-1 overflow-auto">
          {agents.map(agent => (
            <button
              key={agent.id}
              onClick={() => setSelectedAgent(agent.id)}
              className={`w-full text-left px-3 py-2.5 rounded-lg transition ${
                selectedAgent === agent.id
                  ? 'bg-brand-600/15 text-brand-300'
                  : 'text-gray-400 hover:bg-gray-800 hover:text-gray-200'
              }`}
            >
              <div className="flex items-center gap-2">
                <span className="text-lg">{agent.icon}</span>
                <div>
                  <div className="text-sm font-medium">{agent.name}</div>
                  <div className="text-[10px] text-gray-500">{agent.desc}</div>
                </div>
              </div>
            </button>
          ))}
        </div>
      </aside>

      {/* Chat area */}
      <div className="flex-1 flex flex-col">
        {/* Header */}
        <header className="h-14 px-6 flex items-center justify-between border-b border-gray-800 bg-gray-900/30">
          <div className="flex items-center gap-2">
            <Sparkles className="w-4 h-4 text-brand-400" />
            <span className="text-sm font-medium text-white">
              {agents.find(a => a.id === selectedAgent)?.name}
            </span>
            <span className="text-[10px] text-gray-500 bg-gray-800 px-2 py-0.5 rounded-full">
              {agents.find(a => a.id === selectedAgent)?.desc}
            </span>
          </div>
          <button
            onClick={handleClear}
            className="flex items-center gap-1.5 text-xs text-gray-500 hover:text-gray-300 transition"
          >
            <RotateCcw className="w-3.5 h-3.5" />
            清空对话
          </button>
        </header>

        {/* Messages */}
        <div className="flex-1 overflow-auto px-6 py-4 space-y-4">
          {messages.map(msg => (
            <div key={msg.id} className={`flex gap-3 ${msg.role === 'user' ? 'justify-end' : ''}`}>
              {msg.role !== 'user' && (
                <div className="w-8 h-8 rounded-lg bg-brand-600/20 flex items-center justify-center shrink-0 mt-0.5">
                  <Bot className="w-4 h-4 text-brand-400" />
                </div>
              )}
              <div
                className={`max-w-[70%] rounded-2xl px-4 py-3 text-sm leading-relaxed whitespace-pre-wrap ${
                  msg.role === 'user'
                    ? 'bg-brand-600 text-white'
                    : msg.role === 'system'
                    ? 'bg-gray-800/50 text-gray-400 italic'
                    : 'bg-gray-800 text-gray-200'
                }`}
              >
                {msg.content}
              </div>
              {msg.role === 'user' && (
                <div className="w-8 h-8 rounded-lg bg-gray-700 flex items-center justify-center shrink-0 mt-0.5">
                  <User className="w-4 h-4 text-gray-300" />
                </div>
              )}
            </div>
          ))}
          {loading && (
            <div className="flex gap-3">
              <div className="w-8 h-8 rounded-lg bg-brand-600/20 flex items-center justify-center shrink-0">
                <Bot className="w-4 h-4 text-brand-400" />
              </div>
              <div className="bg-gray-800 rounded-2xl px-4 py-3 flex items-center gap-2">
                <Loader2 className="w-4 h-4 animate-spin text-brand-400" />
                <span className="text-sm text-gray-400">思考中...</span>
              </div>
            </div>
          )}
          <div ref={bottomRef} />
        </div>

        {/* Input */}
        <div className="px-6 py-4 border-t border-gray-800 bg-gray-900/30">
          <div className="flex gap-3 items-end">
            <textarea
              value={input}
              onChange={e => setInput(e.target.value)}
              onKeyDown={handleKeyDown}
              placeholder="输入消息... (Enter 发送, Shift+Enter 换行)"
              rows={1}
              className="flex-1 px-4 py-3 bg-gray-800 border border-gray-700 rounded-xl text-sm text-white placeholder-gray-500 focus:outline-none focus:border-brand-500 resize-none transition"
              style={{ minHeight: 44, maxHeight: 160 }}
              onInput={e => {
                const t = e.target as HTMLTextAreaElement
                t.style.height = 'auto'
                t.style.height = Math.min(t.scrollHeight, 160) + 'px'
              }}
            />
            <button
              onClick={handleSend}
              disabled={!input.trim() || loading}
              className="px-4 py-3 bg-brand-600 hover:bg-brand-500 disabled:bg-gray-700 disabled:text-gray-500 text-white rounded-xl transition"
            >
              <Send className="w-4 h-4" />
            </button>
          </div>
          <div className="text-[10px] text-gray-600 mt-2 text-center">
            AI 回复仅供参考，请核实重要信息
          </div>
        </div>
      </div>
    </div>
  )
}
