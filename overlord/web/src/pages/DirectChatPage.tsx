import { useState, useEffect, useRef, useCallback } from 'react'
import { Send, Loader2, Bot, User } from 'lucide-react'
import ReactMarkdown from 'react-markdown'
import { api, streamChat } from '../api/client'

interface ChatMsg {
  id: string; role: string; content: string; model?: string
  tokens_in?: number; tokens_out?: number; duration_ms?: number; created_at: string
}

export default function DirectChatPage() {
  const [messages, setMessages] = useState<ChatMsg[]>([])
  const [input, setInput] = useState('')
  const [sending, setSending] = useState(false)
  const [loading, setLoading] = useState(true)
  const endRef = useRef<HTMLDivElement>(null)
  const inputRef = useRef<HTMLTextAreaElement>(null)

  const loadHistory = useCallback(async () => {
    try {
      const res = await api.directChatHistory()
      setMessages(res.messages || [])
    } catch {}
    setLoading(false)
  }, [])

  useEffect(() => { loadHistory() }, [loadHistory])

  useEffect(() => {
    endRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [messages])

  async function handleSend() {
    const msg = input.trim()
    if (!msg || sending) return
    setInput('')
    setSending(true)

    const userMsg: ChatMsg = {
      id: 'user-' + Date.now(), role: 'user', content: msg, created_at: new Date().toISOString()
    }
    const streamId = 'stream-' + Date.now()
    setMessages(prev => [...prev, userMsg, { id: streamId, role: 'assistant', content: '', created_at: new Date().toISOString() }])

    await streamChat(
      '/chat',
      msg,
      (chunk) => {
        setMessages(prev => prev.map(m => m.id === streamId ? { ...m, content: m.content + chunk } : m))
      },
      () => setSending(false),
      (err) => {
        setMessages(prev => prev.map(m => m.id === streamId ? { ...m, content: '\u26A0\uFE0F ' + err } : m))
        setSending(false)
      },
    )
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
        <div className="w-9 h-9 rounded-xl bg-brand-600/20 flex items-center justify-center text-lg shrink-0">
          <Bot className="w-5 h-5 text-brand-400" />
        </div>
        <div className="flex-1 min-w-0">
          <div className="text-sm font-bold text-white">AI 助手</div>
          <div className="text-[11px] text-gray-500">{'StarClaw AI · 随时开始对话'}</div>
        </div>
      </div>

      {/* Messages */}
      <div className="flex-1 overflow-auto px-4 py-4 space-y-4">
        {loading ? (
          <div className="flex items-center justify-center h-full text-gray-500 text-sm">
            <Loader2 className="w-4 h-4 animate-spin mr-2" /> {'加载中...'}
          </div>
        ) : messages.length === 0 ? (
          <div className="flex flex-col items-center justify-center h-full text-center">
            <div className="w-16 h-16 rounded-2xl bg-brand-600/10 flex items-center justify-center mb-4">
              <Bot className="w-8 h-8 text-brand-400" />
            </div>
            <div className="text-sm text-gray-300 font-medium">{'通用 AI 助手'}</div>
            <div className="text-xs text-gray-500 mt-1 max-w-xs">
              {'发送消息开始对话，AI 将为你提供专业的回答'}
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
                {m.role === 'assistant' ? (
                  <div className="prose prose-invert prose-sm max-w-none break-words [&>*:first-child]:mt-0 [&>*:last-child]:mb-0">
                    <ReactMarkdown>{m.content}</ReactMarkdown>
                  </div>
                ) : (
                  <div className="whitespace-pre-wrap break-words">{m.content}</div>
                )}
                {m.role === 'assistant' && (m.tokens_in || 0) + (m.tokens_out || 0) > 0 && (
                  <div className="text-[10px] text-gray-500 mt-1.5 tabular-nums">
                    {m.model} {' · '} {((m.tokens_in || 0) + (m.tokens_out || 0)).toLocaleString()} tokens {' · '} {m.duration_ms}ms
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
