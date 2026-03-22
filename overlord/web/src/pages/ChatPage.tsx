import { useState, useEffect, useRef, useCallback } from 'react'
import { ArrowLeft, Send, Loader2, Bot, User } from 'lucide-react'
import { api } from '../api/client'

interface ChatMsg {
  id: string; role: string; content: string; model?: string
  tokens_in?: number; tokens_out?: number; duration_ms?: number; created_at: string
}

interface TeamInstance {
  id: string; name: string; template_name: string; status: string
  energy_budget: number; energy_used: number
}

const templateIcons: Record<string, string> = {
  MedClaw: '🩺', DevClaw: '💻', MarketClaw: '📢', SupportClaw: '🎧', DataClaw: '📊',
  QuantClaw: '📈', EcomClaw: '🛒', DramaClaw: '🎬', SalesClaw: '🤝', OpsClaw: '⚙️',
}

function getIcon(name: string): string {
  for (const [k, v] of Object.entries(templateIcons)) {
    if (name.includes(k)) return v
  }
  return '🤖'
}

export default function ChatPage({ instance, onBack }: { instance: TeamInstance; onBack: () => void }) {
  const [messages, setMessages] = useState<ChatMsg[]>([])
  const [input, setInput] = useState('')
  const [sending, setSending] = useState(false)
  const [loading, setLoading] = useState(true)
  const endRef = useRef<HTMLDivElement>(null)
  const inputRef = useRef<HTMLTextAreaElement>(null)

  const loadHistory = useCallback(async () => {
    try {
      const res = await api.chatHistory(instance.id)
      setMessages(res.messages || [])
    } catch {}
    setLoading(false)
  }, [instance.id])

  useEffect(() => { loadHistory() }, [loadHistory])

  useEffect(() => {
    endRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [messages])

  async function handleSend() {
    const msg = input.trim()
    if (!msg || sending) return
    setInput('')
    setSending(true)

    // Optimistic: add user message
    const tempUser: ChatMsg = {
      id: 'temp-user', role: 'user', content: msg, created_at: new Date().toISOString()
    }
    setMessages(prev => [...prev, tempUser])

    try {
      const res = await api.sendChat(instance.id, msg)
      // Replace temp + add assistant response
      setMessages(prev => {
        const without = prev.filter(m => m.id !== 'temp-user')
        // Find the user message that was just created (it's in the response context)
        return [...without, { ...tempUser, id: 'sent-' + Date.now() }, res.message]
      })
    } catch (err: any) {
      setMessages(prev => {
        const without = prev.filter(m => m.id !== 'temp-user')
        return [...without, { ...tempUser, id: 'sent-' + Date.now() }, {
          id: 'err-' + Date.now(), role: 'assistant', content: '⚠️ ' + (err.message || '发送失败'),
          created_at: new Date().toISOString()
        }]
      })
    }
    setSending(false)
    inputRef.current?.focus()
  }

  function handleKeyDown(e: React.KeyboardEvent) {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault()
      handleSend()
    }
  }

  return (
    <div className="h-full flex flex-col bg-gray-950">
      {/* Header */}
      <div className="px-4 py-3 border-b border-gray-800 flex items-center gap-3 bg-gray-900/60 shrink-0">
        <button onClick={onBack} className="text-gray-400 hover:text-white transition">
          <ArrowLeft className="w-5 h-5" />
        </button>
        <div className="w-9 h-9 rounded-xl bg-gray-700/40 flex items-center justify-center text-lg shrink-0">
          {getIcon(instance.template_name)}
        </div>
        <div className="flex-1 min-w-0">
          <div className="text-sm font-bold text-white truncate">{instance.name}</div>
          <div className="text-[11px] text-gray-500">{instance.template_name}</div>
        </div>
        {instance.energy_used > 0 && (
          <div className="text-[11px] text-gray-500 tabular-nums">{instance.energy_used}⚡</div>
        )}
      </div>

      {/* Messages */}
      <div className="flex-1 overflow-auto px-4 py-4 space-y-4">
        {loading ? (
          <div className="flex items-center justify-center h-full text-gray-500 text-sm">
            <Loader2 className="w-4 h-4 animate-spin mr-2" /> 加载中...
          </div>
        ) : messages.length === 0 ? (
          <div className="flex flex-col items-center justify-center h-full text-center">
            <div className="text-4xl mb-3">{getIcon(instance.template_name)}</div>
            <div className="text-sm text-gray-300 font-medium">{instance.name}</div>
            <div className="text-xs text-gray-500 mt-1 max-w-xs">
              向 {instance.template_name} 团队发送消息，AI 助手将根据团队专业领域为你提供帮助
            </div>
          </div>
        ) : (
          messages.map(m => (
            <div key={m.id} className={`flex gap-3 ${m.role === 'user' ? 'justify-end' : ''}`}>
              {m.role !== 'user' && (
                <div className="w-7 h-7 rounded-lg bg-brand-600/20 flex items-center justify-center shrink-0 mt-0.5">
                  <Bot className="w-4 h-4 text-brand-400" />
                </div>
              )}
              <div className={`max-w-[80%] rounded-2xl px-4 py-2.5 text-sm leading-relaxed ${
                m.role === 'user'
                  ? 'bg-brand-600 text-white rounded-br-md'
                  : 'bg-gray-800 text-gray-200 rounded-bl-md'
              }`}>
                <div className="whitespace-pre-wrap break-words">{m.content}</div>
                {m.role === 'assistant' && (m.tokens_in || 0) + (m.tokens_out || 0) > 0 && (
                  <div className="text-[10px] text-gray-500 mt-1.5 tabular-nums">
                    {m.model} · {((m.tokens_in || 0) + (m.tokens_out || 0)).toLocaleString()} tokens · {m.duration_ms}ms
                  </div>
                )}
              </div>
              {m.role === 'user' && (
                <div className="w-7 h-7 rounded-lg bg-gray-700 flex items-center justify-center shrink-0 mt-0.5">
                  <User className="w-4 h-4 text-gray-400" />
                </div>
              )}
            </div>
          ))
        )}
        {sending && (
          <div className="flex gap-3">
            <div className="w-7 h-7 rounded-lg bg-brand-600/20 flex items-center justify-center shrink-0">
              <Bot className="w-4 h-4 text-brand-400" />
            </div>
            <div className="bg-gray-800 rounded-2xl rounded-bl-md px-4 py-3">
              <div className="flex items-center gap-1.5">
                <div className="w-1.5 h-1.5 rounded-full bg-brand-400 animate-bounce" style={{ animationDelay: '0ms' }} />
                <div className="w-1.5 h-1.5 rounded-full bg-brand-400 animate-bounce" style={{ animationDelay: '150ms' }} />
                <div className="w-1.5 h-1.5 rounded-full bg-brand-400 animate-bounce" style={{ animationDelay: '300ms' }} />
              </div>
            </div>
          </div>
        )}
        <div ref={endRef} />
      </div>

      {/* Input */}
      <div className="px-4 py-3 border-t border-gray-800 bg-gray-900/60 shrink-0">
        <div className="flex items-end gap-2">
          <textarea
            ref={inputRef}
            value={input}
            onChange={e => setInput(e.target.value)}
            onKeyDown={handleKeyDown}
            placeholder="输入消息..."
            rows={1}
            className="flex-1 bg-gray-800 border border-gray-700 rounded-xl px-3.5 py-2.5 text-sm text-white placeholder-gray-500 focus:border-brand-500 focus:outline-none transition resize-none max-h-32"
            style={{ minHeight: '40px' }}
          />
          <button
            onClick={handleSend}
            disabled={sending || !input.trim()}
            className="w-10 h-10 flex items-center justify-center bg-brand-600 hover:bg-brand-500 text-white rounded-xl transition disabled:opacity-40 shrink-0"
          >
            {sending ? <Loader2 className="w-4 h-4 animate-spin" /> : <Send className="w-4 h-4" />}
          </button>
        </div>
      </div>
    </div>
  )
}
