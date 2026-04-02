import { useState, useRef, useEffect } from 'react'
import { Send, Loader2, FolderOpen, FileText, Terminal, Code2, RefreshCw, ChevronRight, ChevronDown, Play } from 'lucide-react'
import { codingAPI } from '../lib/api'
import { useAuthStore } from '../stores/authStore'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import CodeBlock from '../components/CodeBlock'

interface FileNode {
  name: string
  path: string
  is_dir: boolean
  size: number
  children?: FileNode[]
  expanded?: boolean
}

interface ChatMessage {
  role: 'user' | 'assistant' | 'tool'
  content: string
  toolName?: string
}

interface ModelOption {
  id: string
  name: string
}

export default function CodingAgentPage() {
  const [input, setInput] = useState('')
  const [messages, setMessages] = useState<ChatMessage[]>([])
  const [loading, setLoading] = useState(false)
  const [streamingContent, setStreamingContent] = useState('')
  const [workspaceId, setWorkspaceId] = useState('default')
  const [files, setFiles] = useState<FileNode[]>([])
  const [selectedFile, setSelectedFile] = useState<{ path: string; content: string } | null>(null)
  const [toolActions, setToolActions] = useState<{ action: string; detail: string }[]>([])
  const [models, setModels] = useState<ModelOption[]>([])
  const [selectedModelId, setSelectedModelId] = useState('')
  const [activeTab, setActiveTab] = useState<'files' | 'terminal'>('files')
  const [terminalOutput, setTerminalOutput] = useState<string[]>([])
  const messagesEndRef = useRef<HTMLDivElement>(null)
  const token = useAuthStore((s) => s.token)

  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [messages, streamingContent])

  useEffect(() => {
    // Load model list
    const loadModels = async () => {
      try {
        const res = await fetch('/v1/models', {
          headers: { Authorization: `Bearer ${token}` },
        })
        const data = await res.json()
        const list = data.models || []
        setModels(list.map((m: any) => ({ id: m.id, name: `${m.provider}/${m.model_name}` })))
        if (list.length > 0 && !selectedModelId) {
          setSelectedModelId(list[0].id)
        }
      } catch { /* ignore */ }
    }
    loadModels()
  }, [])

  const loadFiles = async () => {
    try {
      const res = await codingAPI.listFiles(workspaceId)
      setFiles(res.data.files || [])
    } catch { /* ignore */ }
  }

  const handleFileClick = async (file: FileNode) => {
    if (file.is_dir) {
      // Toggle expand
      setFiles(prev => toggleDir(prev, file.path))
      // Load children
      try {
        const res = await codingAPI.listFiles(workspaceId, file.path)
        const children = res.data.files || []
        setFiles(prev => setChildren(prev, file.path, children))
      } catch { /* ignore */ }
    } else {
      try {
        const res = await codingAPI.readFile(workspaceId, file.path)
        setSelectedFile({ path: file.path, content: res.data.content || '' })
      } catch { /* ignore */ }
    }
  }

  const toggleDir = (nodes: FileNode[], path: string): FileNode[] => {
    return nodes.map(n => {
      if (n.path === path) return { ...n, expanded: !n.expanded }
      if (n.children) return { ...n, children: toggleDir(n.children, path) }
      return n
    })
  }

  const setChildren = (nodes: FileNode[], path: string, children: FileNode[]): FileNode[] => {
    return nodes.map(n => {
      if (n.path === path) return { ...n, children, expanded: true }
      if (n.children) return { ...n, children: setChildren(n.children, path, children) }
      return n
    })
  }

  const handleSend = async () => {
    if (!input.trim() || loading) return
    const userMessage = input.trim()
    setInput('')
    setMessages(prev => [...prev, { role: 'user', content: userMessage }])
    setLoading(true)
    setStreamingContent('')
    setToolActions([])

    try {
      const response = await fetch('/v1/coding/run', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${token}`,
        },
        body: JSON.stringify({
          workspace_id: workspaceId,
          message: userMessage,
          model_id: selectedModelId,
        }),
      })

      const reader = response.body?.getReader()
      const decoder = new TextDecoder()

      if (reader) {
        let fullContent = ''
        let sseBuffer = ''

        while (true) {
          const { done, value } = await reader.read()
          if (done) break

          sseBuffer += decoder.decode(value, { stream: true })
          const parts = sseBuffer.split('\n\n')
          sseBuffer = parts.pop() || ''

          for (const part of parts) {
            const line = part.trim()
            if (!line.startsWith('data: ')) continue
            try {
              const data = JSON.parse(line.slice(6))

              if (data.done) {
                if (fullContent) {
                  setMessages(prev => [...prev, { role: 'assistant', content: fullContent }])
                }
                setStreamingContent('')
                loadFiles()
              } else if (data.tool_call) {
                try {
                  const tc = JSON.parse(data.tool_call)
                  const fnName = tc.Function?.Name || tc.function?.name || 'code'
                  let args: any = {}
                  try {
                    args = JSON.parse(tc.Function?.Arguments || tc.function?.arguments || '{}')
                  } catch { /* ignore */ }
                  const action = args.action || 'unknown'
                  let detail = ''
                  if (action === 'read_file' || action === 'write_file') detail = args.path || ''
                  else if (action === 'execute') detail = `${args.language || ''}`
                  else if (action === 'run_command') detail = args.command || ''
                  else if (action === 'grep' || action === 'search_files') detail = args.pattern || ''
                  else if (action === 'list_files') detail = args.path || '.'

                  setToolActions(prev => [...prev, { action: `${fnName}.${action}`, detail }])
                } catch { /* ignore */ }
              } else if (data.tool_result) {
                // Parse tool result for terminal output
                try {
                  const tr = typeof data.tool_result === 'string' ? JSON.parse(data.tool_result) : data.tool_result
                  if (tr.result && (tr.result.stdout || tr.result.stderr)) {
                    const output = [
                      tr.result.stdout ? `$ stdout:\n${tr.result.stdout}` : '',
                      tr.result.stderr ? `$ stderr:\n${tr.result.stderr}` : '',
                      `Exit code: ${tr.result.exit_code} (${tr.result.duration})`,
                    ].filter(Boolean).join('\n')
                    setTerminalOutput(prev => [...prev, output])
                  }
                  if (tr.action === 'write_file' && tr.status === 'success') {
                    setTerminalOutput(prev => [...prev, `✓ Wrote ${tr.path} (${tr.bytes} bytes)`])
                  }
                  if (tr.action === 'read_file' && tr.path) {
                    setSelectedFile({ path: tr.path, content: tr.content || '' })
                  }
                } catch { /* ignore */ }
              } else if (data.content) {
                fullContent += data.content
                setStreamingContent(fullContent)
              } else if (data.error) {
                setMessages(prev => [...prev, { role: 'assistant', content: `Error: ${data.error}` }])
              }
            } catch { /* skip malformed */ }
          }
        }
      }
    } catch (err: any) {
      setMessages(prev => [...prev, { role: 'assistant', content: `Error: ${err.message}` }])
    } finally {
      setLoading(false)
    }
  }

  const renderFileTree = (nodes: FileNode[], depth = 0) => {
    return nodes.map((node) => (
      <div key={node.path}>
        <button
          onClick={() => handleFileClick(node)}
          className={`w-full text-left flex items-center gap-1.5 px-2 py-1 text-xs hover:bg-gray-100 dark:hover:bg-gray-700 rounded transition-colors ${
            selectedFile?.path === node.path ? 'bg-primary-50 text-primary-700 dark:bg-primary-900/30' : 'text-gray-600 dark:text-gray-300'
          }`}
          style={{ paddingLeft: `${depth * 16 + 8}px` }}
        >
          {node.is_dir ? (
            <>
              {node.expanded ? <ChevronDown className="w-3 h-3 flex-shrink-0" /> : <ChevronRight className="w-3 h-3 flex-shrink-0" />}
              <FolderOpen className="w-3.5 h-3.5 text-amber-500 flex-shrink-0" />
            </>
          ) : (
            <>
              <span className="w-3" />
              <FileText className="w-3.5 h-3.5 text-gray-400 flex-shrink-0" />
            </>
          )}
          <span className="truncate">{node.name}</span>
          {!node.is_dir && <span className="ml-auto text-[10px] text-gray-400">{formatSize(node.size)}</span>}
        </button>
        {node.is_dir && node.expanded && node.children && renderFileTree(node.children, depth + 1)}
      </div>
    ))
  }

  const formatSize = (bytes: number) => {
    if (bytes < 1024) return `${bytes}B`
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)}KB`
    return `${(bytes / (1024 * 1024)).toFixed(1)}MB`
  }

  return (
    <div className="flex h-full">
      {/* Left: Chat Panel */}
      <div className="flex-1 flex flex-col min-w-0 border-r">
        {/* Header */}
        <div className="border-b px-4 py-2 flex items-center gap-3 bg-white dark:bg-gray-900">
          <Code2 className="w-5 h-5 text-primary-600" />
          <h1 className="font-semibold text-sm">编程 Agent</h1>
          <div className="flex items-center gap-2 ml-auto">
            <label className="text-xs text-gray-500">工作区:</label>
            <input
              value={workspaceId}
              onChange={(e) => setWorkspaceId(e.target.value)}
              className="w-28 px-2 py-1 text-xs border rounded bg-white dark:bg-gray-800 dark:text-gray-200"
              placeholder="workspace ID"
            />
            <select
              value={selectedModelId}
              onChange={(e) => setSelectedModelId(e.target.value)}
              className="px-2 py-1 text-xs border rounded bg-white dark:bg-gray-800 dark:text-gray-200"
            >
              <option value="">选择模型</option>
              {models.map((m) => (
                <option key={m.id} value={m.id}>{m.name}</option>
              ))}
            </select>
          </div>
        </div>

        {/* Messages */}
        <div className="flex-1 overflow-y-auto px-4 py-4 space-y-3 scrollbar-thin">
          {messages.length === 0 && !streamingContent && (
            <div className="flex items-center justify-center h-full text-gray-400">
              <div className="text-center space-y-2">
                <Code2 className="w-12 h-12 mx-auto text-gray-300" />
                <p className="text-lg font-medium">自主编程 Agent</p>
                <p className="text-sm">描述你想要构建的程序，Agent 会自动编写、测试和修复代码</p>
                <div className="flex flex-wrap gap-2 justify-center mt-4">
                  {['写一个 Python 贪吃蛇游戏', '用 Node.js 写一个 TODO API', '写一个排序算法可视化的 HTML 页面'].map((s) => (
                    <button
                      key={s}
                      onClick={() => setInput(s)}
                      className="px-3 py-1.5 text-xs border rounded-full hover:bg-gray-50 dark:hover:bg-gray-800 transition-colors"
                    >
                      {s}
                    </button>
                  ))}
                </div>
              </div>
            </div>
          )}

          {messages.map((msg, i) => (
            <div key={i} className={`flex ${msg.role === 'user' ? 'justify-end' : 'justify-start'}`}>
              <div className={`max-w-[85%] rounded-2xl px-4 py-2.5 text-sm ${
                msg.role === 'user'
                  ? 'bg-primary-600 text-white rounded-br-md'
                  : 'bg-gray-100 dark:bg-gray-800 text-gray-800 dark:text-gray-200 rounded-bl-md'
              }`}>
                {msg.role === 'assistant' ? (
                  <ReactMarkdown remarkPlugins={[remarkGfm]} components={{
                    a({ href, children, ...props }: any) {
                      const isFile = href && href.startsWith('/v1/')
                      return <a href={href} {...(isFile ? { target: '_blank', rel: 'noopener noreferrer' } : {})} {...props}>{children}</a>
                    },
                    img({ src, alt, ...props }: any) {
                      return <img src={src} alt={alt || ''} {...props} className="max-w-full rounded-lg" loading="lazy" onError={(e: any) => { e.target.style.display = 'none' }} />
                    },
                    code: CodeBlock as any
                  }}>
                    {msg.content}
                  </ReactMarkdown>
                ) : (
                  msg.content
                )}
              </div>
            </div>
          ))}

          {/* Tool actions */}
          {toolActions.length > 0 && (
            <div className="flex justify-start">
              <div className="bg-gray-50 dark:bg-gray-800/50 border rounded-xl px-3 py-2 space-y-1 max-w-[85%]">
                {toolActions.map((ta, i) => (
                  <div key={i} className="flex items-center gap-2 text-xs text-gray-500">
                    <Play className="w-3 h-3 text-green-500 flex-shrink-0" />
                    <span className="font-mono">{ta.action}</span>
                    {ta.detail && <span className="text-gray-400 truncate max-w-[200px]">{ta.detail}</span>}
                  </div>
                ))}
                {loading && <Loader2 className="w-3 h-3 animate-spin text-primary-500 mt-1" />}
              </div>
            </div>
          )}

          {/* Streaming content */}
          {streamingContent && (
            <div className="flex justify-start">
              <div className="max-w-[85%] rounded-2xl rounded-bl-md bg-gray-100 dark:bg-gray-800 px-4 py-2.5 text-sm text-gray-800 dark:text-gray-200">
                <ReactMarkdown remarkPlugins={[remarkGfm]} components={{
                  a({ href, children, ...props }: any) {
                    const isFile = href && href.startsWith('/v1/')
                    return <a href={href} {...(isFile ? { target: '_blank', rel: 'noopener noreferrer' } : {})} {...props}>{children}</a>
                  },
                  img({ src, alt, ...props }: any) {
                    return <img src={src} alt={alt || ''} {...props} className="max-w-full rounded-lg" loading="lazy" onError={(e: any) => { e.target.style.display = 'none' }} />
                  },
                  code: CodeBlock as any
                }}>
                  {streamingContent}
                </ReactMarkdown>
              </div>
            </div>
          )}

          {loading && !streamingContent && toolActions.length === 0 && (
            <div className="flex justify-start">
              <div className="px-4 py-3 rounded-2xl rounded-bl-md bg-gray-100 dark:bg-gray-800">
                <Loader2 className="w-5 h-5 text-gray-400 animate-spin" />
              </div>
            </div>
          )}

          <div ref={messagesEndRef} />
        </div>

        {/* Input */}
        <div className="border-t p-3">
          <div className="flex items-center gap-2">
            <input
              value={input}
              onChange={(e) => setInput(e.target.value)}
              onKeyDown={(e) => e.key === 'Enter' && !e.shiftKey && handleSend()}
              placeholder="描述你想要编写的程序..."
              className="flex-1 px-4 py-2.5 border rounded-xl outline-none focus:ring-2 focus:ring-primary-500 text-sm bg-white dark:bg-gray-800 dark:text-gray-200"
              disabled={loading}
            />
            <button
              onClick={handleSend}
              disabled={loading || !input.trim()}
              className="p-2.5 bg-primary-600 text-white rounded-xl hover:bg-primary-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
            >
              {loading ? <Loader2 className="w-5 h-5 animate-spin" /> : <Send className="w-5 h-5" />}
            </button>
          </div>
        </div>
      </div>

      {/* Right: Workspace Panel */}
      <div className="w-[420px] flex flex-col bg-gray-50 dark:bg-gray-900 hidden lg:flex">
        {/* Tabs */}
        <div className="border-b flex items-center">
          <button
            onClick={() => setActiveTab('files')}
            className={`flex-1 px-4 py-2 text-xs font-medium flex items-center justify-center gap-1.5 border-b-2 transition-colors ${
              activeTab === 'files' ? 'border-primary-500 text-primary-600' : 'border-transparent text-gray-500 hover:text-gray-700'
            }`}
          >
            <FolderOpen className="w-3.5 h-3.5" /> 文件
          </button>
          <button
            onClick={() => setActiveTab('terminal')}
            className={`flex-1 px-4 py-2 text-xs font-medium flex items-center justify-center gap-1.5 border-b-2 transition-colors ${
              activeTab === 'terminal' ? 'border-primary-500 text-primary-600' : 'border-transparent text-gray-500 hover:text-gray-700'
            }`}
          >
            <Terminal className="w-3.5 h-3.5" /> 终端
          </button>
          <button
            onClick={loadFiles}
            className="px-3 py-2 text-gray-400 hover:text-gray-600 transition-colors"
            title="刷新文件"
          >
            <RefreshCw className="w-3.5 h-3.5" />
          </button>
        </div>

        {activeTab === 'files' ? (
          <>
            {/* File Tree */}
            <div className="h-48 overflow-y-auto border-b scrollbar-thin p-1">
              {files.length === 0 ? (
                <div className="flex items-center justify-center h-full text-xs text-gray-400">
                  <div className="text-center">
                    <FolderOpen className="w-8 h-8 mx-auto mb-1 text-gray-300" />
                    <p>工作区为空</p>
                    <p className="mt-1">Agent 编写代码后文件将显示在这里</p>
                  </div>
                </div>
              ) : (
                renderFileTree(files)
              )}
            </div>

            {/* File Viewer */}
            <div className="flex-1 overflow-hidden flex flex-col">
              {selectedFile ? (
                <>
                  <div className="px-3 py-1.5 border-b bg-white dark:bg-gray-800 flex items-center gap-2">
                    <FileText className="w-3.5 h-3.5 text-gray-400" />
                    <span className="text-xs font-mono text-gray-600 dark:text-gray-300 truncate">{selectedFile.path}</span>
                  </div>
                  <div className="flex-1 overflow-auto p-3 scrollbar-thin">
                    <pre className="text-xs font-mono text-gray-700 dark:text-gray-300 whitespace-pre-wrap break-all">
                      <code>{selectedFile.content}</code>
                    </pre>
                  </div>
                </>
              ) : (
                <div className="flex items-center justify-center h-full text-xs text-gray-400">
                  点击文件查看内容
                </div>
              )}
            </div>
          </>
        ) : (
          /* Terminal Output */
          <div className="flex-1 overflow-auto p-3 bg-gray-900 scrollbar-thin">
            {terminalOutput.length === 0 ? (
              <div className="flex items-center justify-center h-full text-xs text-gray-500">
                <div className="text-center">
                  <Terminal className="w-8 h-8 mx-auto mb-1 text-gray-600" />
                  <p>执行输出将显示在这里</p>
                </div>
              </div>
            ) : (
              <div className="space-y-3">
                {terminalOutput.map((output, i) => (
                  <pre key={i} className="text-xs font-mono text-green-400 whitespace-pre-wrap break-all border-b border-gray-700 pb-2">
                    {output}
                  </pre>
                ))}
              </div>
            )}
          </div>
        )}
      </div>
    </div>
  )
}
