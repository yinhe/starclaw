import { useState, useRef, useEffect } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { Send, Loader2, Plus, Search, Globe, Wrench, MoreHorizontal, Pencil, Trash2, Download, ThumbsUp, ThumbsDown, Pin, ImagePlus, Mic, MicOff, Volume2, X as XIcon, Terminal, FileText, FileEdit, ChevronDown, ChevronRight, Eye, CheckCircle2, Settings, ExternalLink, Bot, StopCircle, Copy, Check, PanelRightOpen, PanelRightClose, ListTodo, Workflow, Video, PlayCircle, Clock, AlertCircle, ChevronUp, User, Paperclip, File, FileAudio, FileVideo, FileCode } from 'lucide-react'

const CrawfishIcon = ({ className }: { className?: string }) => (
  <svg className={className} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
    <path d="M12 19c-2 0-4-1-5-3s-1-4 0-6c1-1.5 3-3 5-3s4 1.5 5 3c1 2 1 4 0 6s-3 3-5 3z" />
    <path d="M9 10c-2-2-4-3-6-2" />
    <path d="M15 10c2-2 4-3 6-2" />
    <path d="M9 13h.01M15 13h.01" />
    <path d="M10 16c.5.5 1.5.5 2 0" />
    <path d="M8 7c-.5-1.5 0-3 1-4" />
    <path d="M16 7c.5-1.5 0-3-1-4" />
  </svg>
)
import { useChatStore } from '../stores/chatStore'
import { chatAPI, agentAPI, conversationAPI, multimodalAPI, superAgentAPI, codingAPI, fileAPI } from '../lib/api'
import ReactMarkdown from 'react-markdown'
import CodeBlock from '../components/CodeBlock'
import Skeleton from '../components/Skeleton'

interface Agent {
  id: string
  name: string
  description: string
}

interface ToolInteraction {
  id: string
  toolName: string
  action: string
  description: string
  status: 'calling' | 'completed' | 'error'
  args?: Record<string, any>
  result?: string
  reasoning?: string
  expanded: boolean
}

export default function ChatPage() {
  const { conversationId } = useParams()
  const navigate = useNavigate()
  const [input, setInput] = useState('')
  const [agents, setAgents] = useState<Agent[]>([])
  const [selectedAgentId, setSelectedAgentId] = useState<string>('')
  const messagesEndRef = useRef<HTMLDivElement>(null)
  const textareaRef = useRef<HTMLTextAreaElement>(null)
  const [toolInteractions, setToolInteractions] = useState<ToolInteraction[]>([])
  const [contextMenu, setContextMenu] = useState<string | null>(null)
  const [convSearch, setConvSearch] = useState('')
  const [sidebarOpen, setSidebarOpen] = useState(true)
  const [attachedImages, setAttachedImages] = useState<{ url: string; name: string }[]>([])
  const [browserScreenshots, setBrowserScreenshots] = useState<string[]>([])
  const [isRecording, setIsRecording] = useState(false)
  const [isTranscribing, setIsTranscribing] = useState(false)
  const [audioLevels, setAudioLevels] = useState<number[]>(new Array(32).fill(0))
  const [recordingDuration, setRecordingDuration] = useState(0)
  const mediaRecorderRef = useRef<MediaRecorder | null>(null)
  const audioContextRef = useRef<AudioContext | null>(null)
  const analyserRef = useRef<AnalyserNode | null>(null)
  const animFrameRef = useRef<number>(0)
  const recordTimerRef = useRef<ReturnType<typeof setInterval> | null>(null)
  const imageInputRef = useRef<HTMLInputElement>(null)
  const fileInputRef = useRef<HTMLInputElement>(null)
  const [attachedFiles, setAttachedFiles] = useState<{ id: string; filename: string; url: string; size: number; mime: string; category: string; stored: string }[]>([])
  const [fileUploading, setFileUploading] = useState(false)
  const abortControllerRef = useRef<AbortController | null>(null)
  const [copiedMsgId, setCopiedMsgId] = useState<string | null>(null)
  const [contextPanelOpen, setContextPanelOpen] = useState(false)
  const [convContext, setConvContext] = useState<any>(null)
  const [contextLoading, setContextLoading] = useState(false)
  const [contextExpandedSection, setContextExpandedSection] = useState<string>('tasks')
  const [showMentionPopup, setShowMentionPopup] = useState(false)
  const [mentionFilter, setMentionFilter] = useState('')
  const [mentionIndex, setMentionIndex] = useState(0)
  const [mentionedAgent, setMentionedAgent] = useState<Agent | null>(null)
  const [editingMsgId, setEditingMsgId] = useState<string | null>(null)
  const [editingText, setEditingText] = useState('')
  const [runningFileId, setRunningFileId] = useState<string | null>(null)
  const [fileRunResults, setFileRunResults] = useState<Record<string, { stdout: string; stderr: string; exit_code: number; duration: string } | null>>({})

  const {
    conversations,
    currentConversationId,
    messages,
    isLoading,
    streamingContent,
    setConversations,
    setCurrentConversation,
    setMessages,
    addMessage,
    setLoading,
    setStreamingContent,
    appendStreamingContent,
  } = useChatStore()

  useEffect(() => {
    loadConversations()
    loadAgents()
  }, [])

  useEffect(() => {
    if (conversationId) {
      setCurrentConversation(conversationId)
      loadMessages(conversationId)
      loadContext(conversationId)
    } else {
      setConvContext(null)
    }
  }, [conversationId])

  useEffect(() => {
    if (currentConversationId) {
      const interval = setInterval(() => loadContext(currentConversationId), 10000)
      return () => clearInterval(interval)
    }
  }, [currentConversationId])

  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [messages, streamingContent])

  const loadConversations = async () => {
    try {
      const res = await chatAPI.listConversations()
      setConversations(res.data.conversations || [])
    } catch { /* ignore */ }
  }

  const loadAgents = async () => {
    try {
      // Ensure the built-in agents exist (SuperAgent + specialists)
      let defaultAgentId = ''
      try {
        const superRes = await superAgentAPI.ensure()
        if (superRes.data?.agent?.id) {
          defaultAgentId = superRes.data.agent.id
        }
      } catch { /* ignore */ }

      const res = await agentAPI.list()
      const list = res.data.agents || []
      setAgents(list)
      // Always default to SuperAgent (router)
      if (!selectedAgentId) {
        if (defaultAgentId && list.find((a: Agent) => a.id === defaultAgentId)) {
          setSelectedAgentId(defaultAgentId)
        } else if (list.length > 0) {
          setSelectedAgentId(list[0].id)
        }
      }
    } catch { /* ignore */ }
  }

  const loadMessages = async (convId: string) => {
    try {
      const res = await chatAPI.getMessages(convId)
      setMessages(res.data.messages || [])
      setToolInteractions([])
    } catch { /* ignore */ }
  }

  // Parse saved tool_calls from a message into ToolInteraction[]
  const parseMessageToolCalls = (msg: any): ToolInteraction[] => {
    if (!msg.tool_calls || msg.tool_calls === '[]') return []
    try {
      const records = JSON.parse(msg.tool_calls)
      return records.map((rec: any, idx: number) => {
        try {
          const tc = JSON.parse(rec.tool_call)
          const fnName = tc.Function?.Name || tc.function?.name || 'tool'
          let args: Record<string, any> = {}
          try { args = JSON.parse(tc.Function?.Arguments || tc.function?.arguments || '{}') } catch {}
          const action = args.action || ''
          return {
            id: `ti-${msg.id}-${idx}`,
            toolName: fnName,
            action,
            description: buildToolDescription(fnName, action, args),
            status: 'completed' as const,
            args,
            result: rec.tool_result || undefined,
            reasoning: rec.reasoning || undefined,
            expanded: false,
          }
        } catch { return null }
      }).filter(Boolean) as ToolInteraction[]
    } catch { return [] }
  }

  // Track expanded state for persisted tool cards (keyed by ti id)
  const [expandedToolIds, setExpandedToolIds] = useState<Set<string>>(new Set())
  const togglePersistedToolExpanded = (id: string) => {
    setExpandedToolIds(prev => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id); else next.add(id)
      return next
    })
  }

  const loadContext = async (convId: string) => {
    setContextLoading(true)
    try {
      const res = await conversationAPI.context(convId)
      setConvContext(res.data)
    } catch { /* ignore */ }
    setContextLoading(false)
  }

  const handleImageUpload = async (files: FileList | null) => {
    if (!files) return
    for (const file of Array.from(files)) {
      if (!file.type.startsWith('image/')) continue
      try {
        const res = await multimodalAPI.uploadImage(file)
        setAttachedImages((prev) => [...prev, { url: res.data.url, name: res.data.filename }])
      } catch { /* ignore */ }
    }
  }

  const handleFileUpload = async (files: FileList | null) => {
    if (!files) return
    setFileUploading(true)
    for (const file of Array.from(files)) {
      // Skip images — those go through handleImageUpload
      if (file.type.startsWith('image/')) {
        try {
          const res = await multimodalAPI.uploadImage(file)
          setAttachedImages((prev) => [...prev, { url: res.data.url, name: res.data.filename }])
        } catch { /* ignore */ }
        continue
      }
      try {
        const res = await fileAPI.upload(file)
        setAttachedFiles((prev) => [...prev, {
          id: res.data.id,
          filename: res.data.filename,
          url: res.data.url,
          size: res.data.size,
          mime: res.data.mime,
          category: res.data.category,
          stored: res.data.stored,
        }])
      } catch (err: any) {
        alert(err.response?.data?.error || '文件上传失败')
      }
    }
    setFileUploading(false)
  }

  const getFileIcon = (category: string) => {
    switch (category) {
      case 'audio': return FileAudio
      case 'video': return FileVideo
      case 'document': return FileText
      case 'code': return FileCode
      case 'archive': return File
      default: return FileText
    }
  }

  const formatSize = (bytes: number) => {
    if (bytes < 1024) return bytes + ' B'
    if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB'
    return (bytes / (1024 * 1024)).toFixed(1) + ' MB'
  }

  const handlePaste = (e: React.ClipboardEvent) => {
    const items = e.clipboardData?.items
    if (!items) return
    const imageFiles: File[] = []
    for (const item of Array.from(items)) {
      if (item.type.startsWith('image/')) {
        const file = item.getAsFile()
        if (file) imageFiles.push(file)
      }
    }
    if (imageFiles.length > 0) {
      const dt = new DataTransfer()
      imageFiles.forEach((f) => dt.items.add(f))
      handleImageUpload(dt.files)
    }
  }

  const speechRecognitionRef = useRef<any>(null)
  const micStreamRef = useRef<MediaStream | null>(null)

  const stopAudioVisualization = () => {
    if (animFrameRef.current) {
      cancelAnimationFrame(animFrameRef.current)
      animFrameRef.current = 0
    }
    if (audioContextRef.current) {
      audioContextRef.current.close()
      audioContextRef.current = null
    }
    analyserRef.current = null
    if (recordTimerRef.current) {
      clearInterval(recordTimerRef.current)
      recordTimerRef.current = null
    }
    setAudioLevels(new Array(32).fill(0))
    setRecordingDuration(0)
  }

  const startAudioVisualization = (stream: MediaStream) => {
    try {
      const audioCtx = new AudioContext()
      const analyser = audioCtx.createAnalyser()
      analyser.fftSize = 64
      const source = audioCtx.createMediaStreamSource(stream)
      source.connect(analyser)
      audioContextRef.current = audioCtx
      analyserRef.current = analyser

      const dataArray = new Uint8Array(analyser.frequencyBinCount)
      const updateLevels = () => {
        if (!analyserRef.current) return
        analyserRef.current.getByteFrequencyData(dataArray)
        const levels = Array.from(dataArray).slice(0, 32).map(v => v / 255)
        setAudioLevels(levels)
        animFrameRef.current = requestAnimationFrame(updateLevels)
      }
      updateLevels()

      const start = Date.now()
      recordTimerRef.current = setInterval(() => {
        setRecordingDuration(Math.floor((Date.now() - start) / 1000))
      }, 1000)
    } catch { /* AudioContext not supported */ }
  }

  const stopRecording = () => {
    // Stop Speech Recognition
    if (speechRecognitionRef.current) {
      speechRecognitionRef.current.stop()
      speechRecognitionRef.current = null
    }
    // Stop MediaRecorder (fallback path)
    if (mediaRecorderRef.current && mediaRecorderRef.current.state !== 'inactive') {
      mediaRecorderRef.current.stop()
    }
    // Stop mic stream
    if (micStreamRef.current) {
      micStreamRef.current.getTracks().forEach(t => t.stop())
      micStreamRef.current = null
    }
    setIsRecording(false)
    stopAudioVisualization()
    setTimeout(() => textareaRef.current?.focus(), 100)
  }

  // Check if browser supports Web Speech API
  const getSpeechRecognition = (): any => {
    const w = window as any
    return w.SpeechRecognition || w.webkitSpeechRecognition || null
  }

  const startWithWebSpeech = async () => {
    const SpeechRecognition = getSpeechRecognition()
    const recognition = new SpeechRecognition()
    recognition.lang = 'zh-CN'
    recognition.continuous = true
    recognition.interimResults = true
    recognition.maxAlternatives = 1

    let finalTranscript = ''

    recognition.onresult = (event: any) => {
      let interim = ''
      for (let i = event.resultIndex; i < event.results.length; i++) {
        const transcript = event.results[i][0].transcript
        if (event.results[i].isFinal) {
          finalTranscript += transcript
        } else {
          interim = transcript
        }
      }
      // Show real-time transcription in input
      setInput(prev => {
        const base = prev.replace(/\[识别中: .*?\]$/, '').trimEnd()
        const combined = base ? base + ' ' : ''
        if (interim) {
          return combined + finalTranscript + `[识别中: ${interim}]`
        }
        return combined + finalTranscript
      })
    }

    recognition.onerror = (event: any) => {
      console.error('SpeechRecognition error:', event.error)
      if (event.error === 'no-speech') return // ignore silence
      stopRecording()
    }

    recognition.onend = () => {
      // Clean up interim markers
      setInput(prev => prev.replace(/\[识别中: .*?\]$/, '').trimEnd())
    }

    speechRecognitionRef.current = recognition
    recognition.start()
  }

  const startWithBackendSTT = async (stream: MediaStream) => {
    // Fallback: record audio and send to backend STT
    const recorder = new MediaRecorder(stream)
    const chunks: Blob[] = []
    recorder.ondataavailable = (e) => chunks.push(e.data)
    recorder.onstop = async () => {
      const blob = new Blob(chunks, { type: 'audio/webm' })
      if (blob.size < 1000) return
      setIsTranscribing(true)
      try {
        const res = await multimodalAPI.stt(blob)
        if (res.data.text) {
          setInput(prev => prev ? prev + ' ' + res.data.text : res.data.text)
        }
      } catch (err: any) {
        console.error('STT failed:', err)
        const msg = err?.response?.data?.error || '语音识别失败'
        setInput(prev => prev || `[语音识别错误: ${msg}]`)
      } finally {
        setIsTranscribing(false)
      }
    }
    mediaRecorderRef.current = recorder
    recorder.start()
  }

  const toggleRecording = async () => {
    if (isRecording) {
      stopRecording()
      return
    }
    try {
      const stream = await navigator.mediaDevices.getUserMedia({ audio: true })
      micStreamRef.current = stream
      setIsRecording(true)
      startAudioVisualization(stream)

      // Prefer Web Speech API (real-time, free, works in Chrome/Edge)
      if (getSpeechRecognition()) {
        await startWithWebSpeech()
      } else {
        // Fallback to backend STT
        await startWithBackendSTT(stream)
      }
    } catch (err) {
      console.error('Mic access denied:', err)
      alert('无法访问麦克风，请允许浏览器使用麦克风权限')
    }
  }

  // @ mention system
  const filteredMentionAgents = agents.filter(a =>
    a.name !== '全能助手' && a.name.toLowerCase().includes(mentionFilter.toLowerCase())
  )

  const handleInputChange = (e: React.ChangeEvent<HTMLTextAreaElement>) => {
    const val = e.target.value
    setInput(val)

    // Detect @ mention
    const cursorPos = e.target.selectionStart || 0
    const textBeforeCursor = val.slice(0, cursorPos)
    const atMatch = textBeforeCursor.match(/@([^\s]*)$/)

    if (atMatch) {
      setMentionFilter(atMatch[1])
      setShowMentionPopup(true)
      setMentionIndex(0)
    } else {
      setShowMentionPopup(false)
    }

    // Clear mentioned agent if @ prefix was removed
    if (mentionedAgent && !val.startsWith(`@${mentionedAgent.name} `)) {
      setMentionedAgent(null)
    }
  }

  const handleMentionSelect = (agent: Agent) => {
    const cursorPos = textareaRef.current?.selectionStart || 0
    const textBeforeCursor = input.slice(0, cursorPos)
    const atPos = textBeforeCursor.lastIndexOf('@')
    const textAfterCursor = input.slice(cursorPos)
    const newInput = input.slice(0, atPos) + `@${agent.name} ` + textAfterCursor
    setInput(newInput)
    setMentionedAgent(agent)
    setShowMentionPopup(false)
    setMentionFilter('')
    textareaRef.current?.focus()
  }

  const handleMentionKeyDown = (e: React.KeyboardEvent) => {
    if (!showMentionPopup || filteredMentionAgents.length === 0) return false
    if (e.key === 'ArrowDown') {
      e.preventDefault()
      setMentionIndex(i => (i + 1) % filteredMentionAgents.length)
      return true
    }
    if (e.key === 'ArrowUp') {
      e.preventDefault()
      setMentionIndex(i => (i - 1 + filteredMentionAgents.length) % filteredMentionAgents.length)
      return true
    }
    if (e.key === 'Enter' || e.key === 'Tab') {
      e.preventDefault()
      handleMentionSelect(filteredMentionAgents[mentionIndex])
      return true
    }
    if (e.key === 'Escape') {
      e.preventDefault()
      setShowMentionPopup(false)
      return true
    }
    return false
  }

  const handlePlayTTS = async (text: string) => {
    try {
      const res = await multimodalAPI.tts(text)
      const url = URL.createObjectURL(res.data)
      const audio = new Audio(url)
      audio.play()
      audio.onended = () => URL.revokeObjectURL(url)
    } catch { /* ignore */ }
  }

  const buildToolDescription = (toolName: string, action: string, args: Record<string, any>): string => {
    if (toolName === 'code') {
      switch (action) {
        case 'read_file': return `读取文件 ${args.path || ''}`
        case 'write_file': return `写入文件 ${args.path || ''} (${(args.content || '').length} chars)`
        case 'list_files': return `列出目录 ${args.path || '.'}`
        case 'search_files': return `搜索文件 ${args.pattern || ''}`
        case 'grep': return `搜索内容 "${args.pattern || ''}"`
        case 'execute': return `执行 ${args.language || ''} 代码`
        case 'run_command': return `运行命令 \`${(args.command || '').slice(0, 60)}${(args.command || '').length > 60 ? '...' : ''}\``
        case 'start_app': return `启动应用 \`${(args.command || '').slice(0, 40)}\``
        case 'stop_app': return `停止应用`
        case 'list_apps': return `查看运行中的应用`
        default: return `code.${action}`
      }
    }
    if (toolName === 'browser') {
      switch (action) {
        case 'navigate': return `打开网页 ${args.url || ''}`
        case 'click': return `点击 "${args.selector || ''}"`
        case 'type': return `输入文本`
        case 'screenshot': return '截取屏幕'
        case 'extract_text': return '提取文本'
        case 'scroll': return `滚动页面 ${args.direction || ''}`
        default: return `browser.${action}`
      }
    }
    if (toolName === 'web_search') return `搜索网络 "${args.query || ''}"`
    if (toolName === 'http_request') return `HTTP ${args.method || 'GET'} ${args.url || ''}`
    if (toolName === 'system') {
      switch (action) {
        case 'create_agent': return `创建 Agent "${args.name || ''}"`
        case 'delegate_to_agent': {
          const targetAgent = agents.find(a => a.id === args.agent_id)
          return `委派任务给「${targetAgent?.name || args.agent_id?.slice(0, 8) + '...'}」`
        }
        case 'create_task': return `创建后台任务 "${args.title || ''}"`
        case 'update_task': return `更新任务进度 ${args.progress || ''}%`
        case 'list_tasks': return '查看后台任务'
        case 'notify_user': return `通知用户: ${args.title || ''}`
        case 'list_agents': return '列出所有 Agent'
        case 'list_models': return '列出可用模型'
        case 'create_workflow': return `创建工作流 "${args.name || ''}"`
        case 'schedule_task': return `创建定时任务 ${args.cron_expr || ''}`
        case 'list_schedules': return '列出定时任务'
        default: return `system.${action}`
      }
    }
    return `${toolName}${action ? '.' + action : ''}`
  }

  const getToolTypeInfo = (toolName: string, action: string) => {
    if (toolName === 'code') {
      switch (action) {
        case 'read_file': return { icon: FileText, label: 'Read', color: 'text-blue-600 bg-blue-50 border-blue-200' }
        case 'write_file': return { icon: FileEdit, label: 'Write', color: 'text-green-600 bg-green-50 border-green-200' }
        case 'execute': return { icon: Terminal, label: 'Exec', color: 'text-purple-600 bg-purple-50 border-purple-200' }
        case 'run_command': return { icon: Terminal, label: 'Exec', color: 'text-purple-600 bg-purple-50 border-purple-200' }
        case 'start_app': return { icon: Globe, label: 'Deploy', color: 'text-emerald-600 bg-emerald-50 border-emerald-200' }
        case 'stop_app': return { icon: XIcon, label: 'Stop', color: 'text-red-600 bg-red-50 border-red-200' }
        case 'list_apps': return { icon: Eye, label: 'Apps', color: 'text-cyan-600 bg-cyan-50 border-cyan-200' }
        case 'grep': case 'search_files': return { icon: Search, label: 'Search', color: 'text-amber-600 bg-amber-50 border-amber-200' }
        case 'list_files': return { icon: Eye, label: 'List', color: 'text-cyan-600 bg-cyan-50 border-cyan-200' }
        default: return { icon: Wrench, label: 'Tool', color: 'text-gray-600 bg-gray-50 border-gray-200' }
      }
    }
    if (toolName === 'browser') return { icon: Globe, label: 'Browser', color: 'text-indigo-600 bg-indigo-50 border-indigo-200' }
    if (toolName === 'web_search') return { icon: Search, label: 'Search', color: 'text-amber-600 bg-amber-50 border-amber-200' }
    if (toolName === 'http_request') return { icon: Globe, label: 'HTTP', color: 'text-teal-600 bg-teal-50 border-teal-200' }
    if (toolName === 'music_generation') return { icon: Bot, label: 'Music', color: 'text-pink-600 bg-pink-50 border-pink-200' }
    if (toolName === 'image_generation') return { icon: Bot, label: 'Image', color: 'text-cyan-600 bg-cyan-50 border-cyan-200' }
    if (toolName === 'dubbing') return { icon: Bot, label: 'Dubbing', color: 'text-purple-600 bg-purple-50 border-purple-200' }
    if (toolName === 'mv_production') return { icon: Bot, label: 'MV', color: 'text-fuchsia-600 bg-fuchsia-50 border-fuchsia-200' }
    if (toolName === 'comic_production') return { icon: Bot, label: 'Comic', color: 'text-orange-600 bg-orange-50 border-orange-200' }
    if (toolName === 'system') {
      switch (action) {
        case 'create_agent': return { icon: Bot, label: 'Agent', color: 'text-emerald-600 bg-emerald-50 border-emerald-200' }
        case 'delegate_to_agent': return { icon: Bot, label: 'Delegate', color: 'text-violet-600 bg-violet-50 border-violet-200' }
        case 'create_task': return { icon: Bot, label: 'Task', color: 'text-violet-600 bg-violet-50 border-violet-200' }
        case 'update_task': return { icon: Bot, label: 'Progress', color: 'text-blue-600 bg-blue-50 border-blue-200' }
        case 'list_tasks': return { icon: Bot, label: 'Tasks', color: 'text-violet-600 bg-violet-50 border-violet-200' }
        case 'notify_user': return { icon: Bot, label: 'Notify', color: 'text-amber-600 bg-amber-50 border-amber-200' }
        case 'list_agents': return { icon: Bot, label: 'Agent', color: 'text-emerald-600 bg-emerald-50 border-emerald-200' }
        case 'create_workflow': case 'schedule_task': case 'list_schedules': return { icon: Settings, label: 'System', color: 'text-rose-600 bg-rose-50 border-rose-200' }
        case 'list_models': return { icon: Settings, label: 'Models', color: 'text-rose-600 bg-rose-50 border-rose-200' }
        default: return { icon: Settings, label: 'System', color: 'text-rose-600 bg-rose-50 border-rose-200' }
      }
    }
    return { icon: Wrench, label: 'Tool', color: 'text-gray-600 bg-gray-50 border-gray-200' }
  }

  const toggleToolExpanded = (id: string) => {
    setToolInteractions(prev => prev.map(t => t.id === id ? { ...t, expanded: !t.expanded } : t))
  }

  // Detect if a file is runnable based on extension
  const isRunnableFile = (path: string) => /\.(py|js|ts|go|sh|rb|php|java|rs|c|cpp|cxx|cc|pl|lua)$/i.test(path)

  const handleRunFile = async (tiId: string, workspaceId: string, filePath: string) => {
    setRunningFileId(tiId)
    setFileRunResults(prev => ({ ...prev, [tiId]: null }))
    try {
      const res = await codingAPI.runFile(workspaceId || '', filePath, currentConversationId || '')
      const r = res.data.result
      setFileRunResults(prev => ({ ...prev, [tiId]: { stdout: r.stdout || '', stderr: r.stderr || '', exit_code: r.exit_code, duration: r.duration || '' } }))
    } catch (e: any) {
      setFileRunResults(prev => ({ ...prev, [tiId]: { stdout: '', stderr: e.response?.data?.error || e.message || 'Execution failed', exit_code: -1, duration: '' } }))
    }
    setRunningFileId(null)
  }

  const handleStopFile = async () => {
    try { await codingAPI.stop() } catch { /* ignore */ }
    setRunningFileId(null)
  }

  const formatToolResult = (result: string): string => {
    try {
      const parsed = JSON.parse(result)
      if (parsed.result?.stdout || parsed.result?.stderr) {
        const parts = []
        if (parsed.result.stdout) parts.push(parsed.result.stdout)
        if (parsed.result.stderr) parts.push(`stderr: ${parsed.result.stderr}`)
        if (parsed.result.exit_code !== undefined) parts.push(`Exit code: ${parsed.result.exit_code} (${parsed.result.duration || ''})`)
        return parts.join('\n')
      }
      if (parsed.content) return parsed.content.length > 500 ? parsed.content.slice(0, 500) + '\n... [truncated]' : parsed.content
      if (parsed.files) return parsed.files.map((f: any) => `${f.is_dir ? '📁' : '📄'} ${f.path} ${f.is_dir ? '' : `(${f.size}B)`}`).join('\n')
      if (parsed.matches) return parsed.matches.map((m: any) => `${m.file}:${m.line} ${m.content}`).join('\n')
      if (parsed.status === 'success' && parsed.bytes !== undefined) return `✓ ${parsed.action} ${parsed.path || ''} (${parsed.bytes} bytes)`
      if (parsed.status === 'success' && parsed.agents) return `✓ ${parsed.action}: ${parsed.count} 个Agent`
      if (parsed.status === 'success' && parsed.count !== undefined) return `✓ ${parsed.action}: ${parsed.count} 项`
      if (parsed.status === 'success' && parsed.message) return `✓ ${parsed.message}`
      if (parsed.status === 'success') return `✓ ${parsed.action || 'done'}`
      if (parsed.error) return `✗ ${parsed.error}`
      if (parsed.message) return parsed.message
      return JSON.stringify(parsed, null, 2).slice(0, 500)
    } catch {
      return result.length > 500 ? result.slice(0, 500) + '\n... [truncated]' : result
    }
  }

  const handleEditResend = async (msgId: string) => {
    if (!editingText.trim() || isLoading || !selectedAgentId) return

    // Find the index of the message being edited
    const msgIndex = messages.findIndex(m => m.id === msgId)
    if (msgIndex === -1) return

    // Delete this message and all subsequent messages from the backend DB
    if (currentConversationId) {
      try {
        await conversationAPI.truncateMessages(currentConversationId, msgId)
      } catch { /* ignore - may be a new unsaved conversation */ }
    }

    // Truncate: keep messages up to (but not including) the edited one
    const truncated = messages.slice(0, msgIndex)
    setMessages(truncated)
    setEditingMsgId(null)
    setEditingText('')

    // Now send the edited text as a new message
    setInput(editingText.trim())
    // We need to trigger send after state update, so we do it inline
    const userMessage = editingText.trim()
    setToolInteractions([])
    setLoading(true)
    setStreamingContent('')

    addMessage({
      id: Date.now().toString(),
      role: 'user',
      content: userMessage,
      created_at: new Date().toISOString(),
    })

    try {
      const token = localStorage.getItem('starclaw_token')
      const abortController = new AbortController()
      abortControllerRef.current = abortController

      const response = await fetch('/v1/chat/completions', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${token}`,
        },
        body: JSON.stringify({
          agent_id: selectedAgentId,
          conversation_id: currentConversationId || '',
          message: userMessage,
          stream: true,
        }),
        signal: abortController.signal,
      })

      if (!response.ok) throw new Error('Chat request failed')

      const reader = response.body?.getReader()
      const decoder = new TextDecoder()

      if (reader) {
        let fullContent = ''
        let sseBuffer = ''
        while (true) {
          const { done, value } = await reader.read()
          if (done) {
            setToolInteractions(prev => prev.map(t => t.status === 'calling' ? { ...t, status: 'error' as const } : t))
            break
          }

          sseBuffer += decoder.decode(value, { stream: true })
          const parts = sseBuffer.split('\n\n')
          sseBuffer = parts.pop() || ''

          for (const part of parts) {
            const line = part.trim()
            if (!line || line.startsWith(':')) continue
            if (!line.startsWith('data: ')) continue
            try {
              const data = JSON.parse(line.slice(6))

              if (data.conversation_id && !currentConversationId) {
                setCurrentConversation(data.conversation_id)
                navigate(`/chat/${data.conversation_id}`, { replace: true })
              }

              if (data.done) {
                addMessage({
                  id: Date.now().toString(),
                  role: 'assistant',
                  content: fullContent,
                  created_at: new Date().toISOString(),
                })
                setStreamingContent('')
                loadConversations()
              } else if (data.tool_call) {
                try {
                  const tc = JSON.parse(data.tool_call)
                  const fnName = tc.Function?.Name || tc.function?.name || 'tool'
                  let args: Record<string, any> = {}
                  try { args = JSON.parse(tc.Function?.Arguments || tc.function?.arguments || '{}') } catch {}
                  const action = args.action || ''
                  const desc = buildToolDescription(fnName, action, args)
                  setToolInteractions(prev => [...prev, {
                    id: `ti-${Date.now()}-${Math.random().toString(36).slice(2, 6)}`,
                    toolName: fnName,
                    action,
                    description: desc,
                    status: 'calling',
                    args,
                    reasoning: data.reasoning || undefined,
                    expanded: false,
                  }])
                } catch {
                  setToolInteractions(prev => [...prev, {
                    id: `ti-${Date.now()}`,
                    toolName: 'tool',
                    action: '',
                    description: '调用工具...',
                    status: 'calling',
                    expanded: false,
                  }])
                }
              } else if (data.tool_result) {
                setToolInteractions(prev => {
                  const updated = [...prev]
                  const last = updated.findIndex(t => t.status === 'calling')
                  if (last >= 0) {
                    updated[last] = { ...updated[last], status: 'completed', result: data.tool_result }
                  }
                  return updated
                })
              } else if (data.content) {
                fullContent += data.content
                appendStreamingContent(data.content)
              }
            } catch { /* skip malformed lines */ }
          }
        }

        // If stream ended without a done event, save whatever was streamed
        if (fullContent && !messages.find(m => m.content === fullContent)) {
          addMessage({
            id: Date.now().toString(),
            role: 'assistant',
            content: fullContent,
            created_at: new Date().toISOString(),
          })
          setStreamingContent('')
        }
      }
    } catch (err: any) {
      if (err?.name === 'AbortError') {
        const partial = useChatStore.getState().streamingContent
        if (partial) {
          addMessage({
            id: Date.now().toString(),
            role: 'assistant',
            content: partial + '\n\n*[已停止生成]*',
            created_at: new Date().toISOString(),
          })
        }
        setStreamingContent('')
      } else {
        addMessage({
          id: (Date.now() + 1).toString(),
          role: 'assistant',
          content: `抱歉，发生了错误: ${err.message}`,
          created_at: new Date().toISOString(),
        })
      }
    } finally {
      setLoading(false)
      setStreamingContent('')
      abortControllerRef.current = null
      setInput('')
    }
  }

  const handleSend = async () => {
    if (!input.trim() || isLoading || !selectedAgentId) return

    let userMessage = input.trim()
    const images = attachedImages.map((img) => img.url)
    const files = attachedFiles.map(f => ({ id: f.id, filename: f.filename, url: f.url, size: f.size, mime: f.mime, category: f.category, stored: f.stored }))

    // Resolve @AgentName mention → use that agent's ID
    let agentIdToUse = selectedAgentId // default: SuperAgent (router)
    const mentionMatch = userMessage.match(/^@(.+?)\s/)
    if (mentionMatch) {
      const mentionName = mentionMatch[1]
      const found = agents.find(a => a.name === mentionName)
      if (found) {
        agentIdToUse = found.id
        userMessage = userMessage.slice(mentionMatch[0].length).trim()
      }
    } else if (mentionedAgent) {
      agentIdToUse = mentionedAgent.id
    }
    setInput('')
    setAttachedImages([])
    setAttachedFiles([])
    setBrowserScreenshots([])
    setToolInteractions([])
    setMentionedAgent(null)
    setShowMentionPopup(false)
    setLoading(true)
    setStreamingContent('')

    // Build display content for user message
    let displayContent = userMessage
    if (images.length > 0) displayContent += `\n\n[${images.length} 张图片]`
    if (files.length > 0) displayContent += `\n\n[${files.length} 个文件: ${files.map(f => f.filename).join(', ')}]`

    addMessage({
      id: Date.now().toString(),
      role: 'user',
      content: displayContent,
      created_at: new Date().toISOString(),
      attachments: files.length > 0 ? JSON.stringify(files) : undefined,
    })

    try {
      // Use SSE for streaming
      const token = localStorage.getItem('starclaw_token')
      const abortController = new AbortController()
      abortControllerRef.current = abortController

      const response = await fetch('/v1/chat/completions', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${token}`,
        },
        body: JSON.stringify({
          agent_id: agentIdToUse,
          conversation_id: currentConversationId || '',
          message: userMessage,
          images: images.length > 0 ? images : undefined,
          files: files.length > 0 ? files : undefined,
          stream: true,
        }),
        signal: abortController.signal,
      })

      if (!response.ok) throw new Error('Chat request failed')

      const reader = response.body?.getReader()
      const decoder = new TextDecoder()

      if (reader) {
        let fullContent = ''
        let activeConvId = currentConversationId || ''
        let sseBuffer = ''
        while (true) {
          const { done, value } = await reader.read()
          if (done) {
            // Mark any tools still in 'calling' as error (stream ended unexpectedly)
            setToolInteractions(prev => prev.map(t => t.status === 'calling' ? { ...t, status: 'error' as const } : t))
            break
          }

          sseBuffer += decoder.decode(value, { stream: true })
          const parts = sseBuffer.split('\n\n')
          sseBuffer = parts.pop() || '' // keep incomplete part in buffer

          for (const part of parts) {
            const line = part.trim()
            if (!line || line.startsWith(':')) continue // skip empty lines and heartbeat comments
            if (!line.startsWith('data: ')) continue
            try {
              const data = JSON.parse(line.slice(6))

              // Capture conversation_id as early as possible
              if (data.conversation_id && !activeConvId) {
                activeConvId = data.conversation_id
                setCurrentConversation(activeConvId)
                navigate(`/chat/${activeConvId}`, { replace: true })
              }

              if (data.done) {
                addMessage({
                  id: Date.now().toString(),
                  role: 'assistant',
                  content: fullContent,
                  created_at: new Date().toISOString(),
                })
                setStreamingContent('')
                loadConversations()
                // Refresh context panel after tools may have created workflows/tasks/videos
                if (activeConvId) loadContext(activeConvId)
              } else if (data.tool_call) {
                try {
                  const tc = JSON.parse(data.tool_call)
                  const fnName = tc.Function?.Name || tc.function?.name || 'tool'
                  let args: Record<string, any> = {}
                  try { args = JSON.parse(tc.Function?.Arguments || tc.function?.arguments || '{}') } catch {}
                  const action = args.action || ''
                  const desc = buildToolDescription(fnName, action, args)
                  setToolInteractions(prev => [...prev, {
                    id: `ti-${Date.now()}-${Math.random().toString(36).slice(2, 6)}`,
                    toolName: fnName,
                    action,
                    description: desc,
                    status: 'calling',
                    args,
                    reasoning: data.reasoning || undefined,
                    expanded: false,
                  }])
                } catch {
                  setToolInteractions(prev => [...prev, {
                    id: `ti-${Date.now()}`,
                    toolName: 'tool',
                    action: '',
                    description: '调用工具...',
                    status: 'calling',
                    expanded: false,
                  }])
                }
              } else if (data.tool_result) {
                setToolInteractions(prev => {
                  const updated = [...prev]
                  const last = updated.findIndex(t => t.status === 'calling')
                  if (last >= 0) {
                    updated[last] = { ...updated[last], status: 'completed', result: data.tool_result }
                  }
                  return updated
                })
                if (data.screenshot_url) {
                  setBrowserScreenshots((prev) => [...prev, data.screenshot_url])
                }
              } else if (data.content) {
                fullContent += data.content
                appendStreamingContent(data.content)
              }
            } catch { /* skip malformed lines */ }
          }
        }
      }
    } catch (err: any) {
      if (err?.name === 'AbortError') {
        // User stopped generation — save whatever was streamed
        const partial = useChatStore.getState().streamingContent
        if (partial) {
          addMessage({
            id: Date.now().toString(),
            role: 'assistant',
            content: partial + '\n\n*[已停止生成]*',
            created_at: new Date().toISOString(),
          })
        }
        setStreamingContent('')
      } else {
        addMessage({
          id: Date.now().toString(),
          role: 'assistant',
          content: '抱歉，发生了错误，请重试。',
          created_at: new Date().toISOString(),
        })
      }
    } finally {
      setLoading(false)
      abortControllerRef.current = null
    }
  }

  const handleRenameConversation = async (convId: string) => {
    const newTitle = prompt('输入新名称')
    if (!newTitle) return
    try {
      await conversationAPI.rename(convId, newTitle)
      loadConversations()
    } catch { /* ignore */ }
    setContextMenu(null)
  }

  const handleDeleteConversation = async (convId: string) => {
    if (!confirm('确定删除这个对话吗？')) return
    try {
      await conversationAPI.delete(convId)
      if (currentConversationId === convId) {
        setCurrentConversation(null)
        setMessages([])
        navigate('/chat', { replace: true })
      }
      loadConversations()
    } catch { /* ignore */ }
    setContextMenu(null)
  }

  const handleExportConversation = async (convId: string) => {
    try {
      const res = await conversationAPI.export(convId)
      const blob = new Blob([res.data.content], { type: 'text/markdown' })
      const url = URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = `${res.data.title || 'conversation'}.md`
      a.click()
      URL.revokeObjectURL(url)
    } catch { /* ignore */ }
    setContextMenu(null)
  }

  const handleStop = () => {
    abortControllerRef.current?.abort()
    abortControllerRef.current = null
  }

  const handleCopy = async (msgId: string, content: string) => {
    try {
      await navigator.clipboard.writeText(content)
      setCopiedMsgId(msgId)
      setTimeout(() => setCopiedMsgId(null), 2000)
    } catch { /* ignore */ }
  }

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault()
      handleSend()
    }
  }

  const handlePin = async (convId: string) => {
    try {
      await conversationAPI.pin(convId)
      loadConversations()
    } catch { /* ignore */ }
    setContextMenu(null)
  }

  const handleFeedback = async (msgId: string, value: number) => {
    if (!conversationId) return
    try {
      await conversationAPI.feedback(conversationId, msgId, value)
      // Update local state
      const updated = messages.map((m) =>
        m.id === msgId ? { ...m, feedback: m.feedback === value ? 0 : value } : m,
      )
      setMessages(updated)
    } catch { /* ignore */ }
  }

  const sortedConversations = [...conversations].sort((a, b) => {
    if (a.is_pinned && !b.is_pinned) return -1
    if (!a.is_pinned && b.is_pinned) return 1
    return 0
  })
  const filteredConversations = convSearch
    ? sortedConversations.filter((c) => (c.title || '').toLowerCase().includes(convSearch.toLowerCase()))
    : sortedConversations

  return (
    <div className="flex h-full">
      {/* Mobile sidebar toggle */}
      {!sidebarOpen && (
        <button
          onClick={() => setSidebarOpen(true)}
          className="md:hidden absolute top-3 left-3 z-20 p-2 bg-white dark:bg-gray-800 border rounded-lg shadow-sm"
        >
          <Search className="w-4 h-4 text-gray-500" />
        </button>
      )}

      {/* Conversation list */}
      <div className={`${sidebarOpen ? 'w-64' : 'w-0 overflow-hidden'} md:w-64 border-r bg-gray-50 dark:bg-gray-900 flex flex-col transition-all`}>
        <div className="p-3 border-b space-y-2">
          <button
            onClick={() => {
              setCurrentConversation(null)
              setMessages([])
              navigate('/chat', { replace: true })
            }}
            className="w-full flex items-center justify-center gap-2 px-3 py-2 bg-primary-600 text-white rounded-lg text-sm hover:bg-primary-700 transition-colors"
          >
            <Plus className="w-4 h-4" />
            新对话
          </button>
          <div className="relative">
            <Search className="absolute left-2.5 top-1/2 -translate-y-1/2 w-3.5 h-3.5 text-gray-400" />
            <input
              value={convSearch}
              onChange={(e) => setConvSearch(e.target.value)}
              placeholder="搜索对话..."
              className="w-full pl-8 pr-3 py-1.5 border rounded-lg text-xs outline-none focus:ring-1 focus:ring-primary-500 bg-white dark:bg-gray-800 dark:text-gray-200"
            />
          </div>
        </div>
        <div className="flex-1 overflow-y-auto scrollbar-thin p-2 space-y-1">
          {conversations.length === 0 && !convSearch && (
            <div className="space-y-2 p-1">
              <Skeleton className="h-9 w-full rounded-lg" count={5} />
            </div>
          )}
          {filteredConversations.map((conv) => (
            <div key={conv.id} className="relative group">
              <button
                onClick={() => {
                  setCurrentConversation(conv.id)
                  loadMessages(conv.id)
                  navigate(`/chat/${conv.id}`, { replace: true })
                  setContextMenu(null)
                  setSidebarOpen(false)
                }}
                className={`w-full text-left px-3 py-2 rounded-lg text-sm truncate transition-colors pr-8 ${
                  currentConversationId === conv.id
                    ? 'bg-primary-100 text-primary-700 dark:bg-primary-900/30 dark:text-primary-300'
                    : 'text-gray-600 dark:text-gray-400 hover:bg-gray-100 dark:hover:bg-gray-800'
                }`}
              >
                <span className="flex items-center gap-1">
                  {conv.is_pinned && <Pin className="w-3 h-3 text-amber-500 flex-shrink-0" />}
                  <span className="truncate">{conv.title || '新对话'}</span>
                </span>
              </button>
              <button
                onClick={(e) => { e.stopPropagation(); setContextMenu(contextMenu === conv.id ? null : conv.id) }}
                className="absolute right-1 top-1/2 -translate-y-1/2 p-1 rounded text-gray-400 hover:text-gray-600 opacity-0 group-hover:opacity-100 transition-opacity"
              >
                <MoreHorizontal className="w-3.5 h-3.5" />
              </button>
              {contextMenu === conv.id && (
                <div className="absolute right-0 top-full mt-1 z-10 bg-white dark:bg-gray-800 border rounded-lg shadow-lg py-1 w-32">
                  <button onClick={() => handleRenameConversation(conv.id)} className="w-full text-left px-3 py-1.5 text-xs text-gray-600 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-700 flex items-center gap-2">
                    <Pencil className="w-3 h-3" /> 重命名
                  </button>
                  <button onClick={() => handlePin(conv.id)} className="w-full text-left px-3 py-1.5 text-xs text-gray-600 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-700 flex items-center gap-2">
                    <Pin className="w-3 h-3" /> {conv.is_pinned ? '取消置顶' : '置顶'}
                  </button>
                  <button onClick={() => handleExportConversation(conv.id)} className="w-full text-left px-3 py-1.5 text-xs text-gray-600 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-700 flex items-center gap-2">
                    <Download className="w-3 h-3" /> 导出
                  </button>
                  <button onClick={() => handleDeleteConversation(conv.id)} className="w-full text-left px-3 py-1.5 text-xs text-red-600 hover:bg-red-50 dark:hover:bg-red-900/20 flex items-center gap-2">
                    <Trash2 className="w-3 h-3" /> 删除
                  </button>
                </div>
              )}
            </div>
          ))}
          {convSearch && filteredConversations.length === 0 && (
            <p className="text-center text-xs text-gray-400 py-4">无匹配对话</p>
          )}
        </div>
      </div>

      {/* Chat area */}
      <div className="flex-1 flex flex-col">
        {/* Header */}
        <div className="px-6 py-3 border-b bg-white flex items-center gap-4">
          <h2 className="font-semibold text-gray-800">对话</h2>
          {mentionedAgent && (
            <span className="inline-flex items-center gap-1 px-2.5 py-1 bg-primary-50 text-primary-700 rounded-full text-xs font-medium border border-primary-200">
              <Bot className="w-3 h-3" />
              {mentionedAgent.name}
              <button onClick={() => setMentionedAgent(null)} className="ml-0.5 hover:text-primary-900">
                <XIcon className="w-3 h-3" />
              </button>
            </span>
          )}
          <div className="flex-1" />
          {currentConversationId && (
            <button
              onClick={() => setContextPanelOpen(!contextPanelOpen)}
              className={`p-1.5 rounded-lg transition-colors ${contextPanelOpen ? 'bg-primary-100 text-primary-600' : 'text-gray-400 hover:text-gray-600 hover:bg-gray-100'}`}
              title="关联面板"
            >
              {contextPanelOpen ? <PanelRightClose className="w-4 h-4" /> : <PanelRightOpen className="w-4 h-4" />}
            </button>
          )}
          {currentConversationId && convContext?.stats && (convContext.stats.tasks_total > 0 || convContext.stats.workflows_total > 0 || convContext.stats.videos_total > 0) && !contextPanelOpen && (
            <div className="flex items-center gap-1.5 text-xs text-gray-400">
              {convContext.stats.tasks_total > 0 && (
                <span className="flex items-center gap-0.5" title="关联任务">
                  <ListTodo className="w-3 h-3" />{convContext.stats.tasks_total}
                </span>
              )}
              {convContext.stats.workflows_total > 0 && (
                <span className="flex items-center gap-0.5" title="关联工作流">
                  <Workflow className="w-3 h-3" />{convContext.stats.workflows_total}
                </span>
              )}
              {convContext.stats.videos_total > 0 && (
                <span className="flex items-center gap-0.5" title="关联视频">
                  <Video className="w-3 h-3" />{convContext.stats.videos_total}
                </span>
              )}
            </div>
          )}
        </div>

        {/* Messages */}
        <div className="flex-1 overflow-y-auto scrollbar-thin px-6 py-4 space-y-4">
          {messages.length === 0 && !streamingContent && (
            <div className="flex items-center justify-center h-full text-gray-400">
              <div className="text-center">
                <p className="text-lg font-medium mb-1">开始对话</p>
                <p className="text-sm">选择一个 Agent，输入消息开始</p>
              </div>
            </div>
          )}

          {messages.map((msg) => {
            const savedToolCalls = msg.role === 'assistant' ? parseMessageToolCalls(msg) : []
            return (
              <div key={msg.id} className="group">
                {/* Inline persisted tool cards */}
                {savedToolCalls.length > 0 && (
                  <div className="space-y-2 mb-2">
                    {savedToolCalls.map((ti) => {
                      const info = getToolTypeInfo(ti.toolName, ti.action)
                      const Icon = info.icon
                      const isExpanded = expandedToolIds.has(ti.id)
                      return (
                        <div key={ti.id} className="space-y-1">
                          {ti.reasoning && (
                            <div className="flex justify-start">
                              <div className="max-w-[85%] px-3 py-1.5 text-xs text-gray-500 bg-gray-50 border border-gray-100 rounded-lg italic">
                                💭 {ti.reasoning.length > 200 ? ti.reasoning.slice(0, 200) + '...' : ti.reasoning}
                              </div>
                            </div>
                          )}
                          <div className="flex justify-start">
                            <div className={`max-w-[85%] border rounded-xl overflow-hidden ${info.color} transition-all`}>
                              <button
                                onClick={() => ti.result && togglePersistedToolExpanded(ti.id)}
                                className="w-full flex items-center gap-2.5 px-3 py-2 text-left"
                              >
                                <Icon className="w-4 h-4 flex-shrink-0" />
                                <span className="text-xs font-medium">{info.label}</span>
                                <span className="text-xs opacity-75 truncate flex-1">{ti.description}</span>
                                <CheckCircle2 className="w-3.5 h-3.5 flex-shrink-0 opacity-60" />
                                {ti.result && (
                                  isExpanded
                                    ? <ChevronDown className="w-3.5 h-3.5 flex-shrink-0 opacity-40" />
                                    : <ChevronRight className="w-3.5 h-3.5 flex-shrink-0 opacity-40" />
                                )}
                              </button>
                              {isExpanded && ti.result && (
                                <div className="border-t px-3 py-2 bg-white/80 dark:bg-gray-900/80">
                                  <pre className="text-[11px] font-mono text-gray-700 dark:text-gray-300 whitespace-pre-wrap break-all max-h-48 overflow-y-auto scrollbar-thin">
                                    {formatToolResult(ti.result)}
                                  </pre>
                                  {ti.toolName === 'code' && ti.action === 'write_file' && ti.args?.path && isRunnableFile(ti.args.path) && (
                                    <div className="mt-2 pt-2 border-t">
                                      <div className="flex items-center gap-2">
                                        {runningFileId === ti.id ? (
                                          <button
                                            onClick={() => handleStopFile()}
                                            className="inline-flex items-center gap-1 px-2.5 py-1 text-xs font-medium text-red-600 bg-red-50 border border-red-200 rounded-lg hover:bg-red-100 transition-colors"
                                          >
                                            <StopCircle className="w-3.5 h-3.5" /> 停止
                                          </button>
                                        ) : (
                                          <button
                                            onClick={() => handleRunFile(ti.id, ti.args?.workspace_id || '', ti.args?.path || '')}
                                            className="inline-flex items-center gap-1 px-2.5 py-1 text-xs font-medium text-emerald-600 bg-emerald-50 border border-emerald-200 rounded-lg hover:bg-emerald-100 transition-colors"
                                          >
                                            <PlayCircle className="w-3.5 h-3.5" /> 运行
                                          </button>
                                        )}
                                        {fileRunResults[ti.id] && (
                                          <span className={`text-[10px] ${fileRunResults[ti.id]!.exit_code === 0 ? 'text-emerald-500' : 'text-red-500'}`}>
                                            {fileRunResults[ti.id]!.exit_code === 0 ? '✓ 成功' : '✗ 失败'} ({fileRunResults[ti.id]!.duration})
                                          </span>
                                        )}
                                      </div>
                                      {runningFileId === ti.id && !fileRunResults[ti.id] && (
                                        <div className="mt-2 flex items-center gap-2 text-xs text-gray-500">
                                          <Loader2 className="w-3 h-3 animate-spin" /> 运行中...
                                        </div>
                                      )}
                                      {fileRunResults[ti.id] && (
                                        <pre className="mt-2 text-[11px] font-mono whitespace-pre-wrap break-all max-h-48 overflow-y-auto scrollbar-thin p-2 rounded bg-gray-900 text-gray-100">
                                          {fileRunResults[ti.id]!.stdout || ''}
                                          {fileRunResults[ti.id]!.stderr ? `\n${fileRunResults[ti.id]!.stderr}` : ''}
                                        </pre>
                                      )}
                                    </div>
                                  )}
                                </div>
                              )}
                            </div>
                          </div>
                        </div>
                      )
                    })}
                  </div>
                )}
                {/* Message bubble */}
                <div className={`flex items-start gap-2.5 ${msg.role === 'user' ? 'justify-end' : 'justify-start'}`}>
                  {msg.role === 'assistant' && (
                    <div className="flex-shrink-0 w-8 h-8 rounded-full bg-gradient-to-br from-red-400 to-orange-500 flex items-center justify-center shadow-sm mt-0.5">
                      <CrawfishIcon className="w-5 h-5 text-white" />
                    </div>
                  )}
                  {msg.role === 'user' && editingMsgId === msg.id ? (
                    /* Edit mode for user message */
                    <div className="max-w-[70%] w-full">
                      <textarea
                        value={editingText}
                        onChange={(e) => setEditingText(e.target.value)}
                        className="w-full px-4 py-3 rounded-2xl rounded-br-md text-sm leading-relaxed bg-primary-50 dark:bg-primary-900/30 border-2 border-primary-400 dark:border-primary-600 text-gray-900 dark:text-white resize-none focus:outline-none focus:border-primary-500"
                        rows={Math.min(8, editingText.split('\n').length + 1)}
                        autoFocus
                        onKeyDown={(e) => {
                          if (e.key === 'Enter' && !e.shiftKey) {
                            e.preventDefault()
                            handleEditResend(msg.id)
                          }
                          if (e.key === 'Escape') {
                            setEditingMsgId(null)
                            setEditingText('')
                          }
                        }}
                      />
                      <div className="flex items-center gap-2 mt-2 justify-end">
                        <button
                          onClick={() => { setEditingMsgId(null); setEditingText('') }}
                          className="px-3 py-1.5 text-xs rounded-lg bg-gray-200 dark:bg-gray-700 text-gray-600 dark:text-gray-300 hover:bg-gray-300 dark:hover:bg-gray-600 transition-colors"
                        >
                          取消
                        </button>
                        <button
                          onClick={() => handleEditResend(msg.id)}
                          disabled={!editingText.trim() || isLoading}
                          className="px-3 py-1.5 text-xs rounded-lg bg-primary-600 text-white hover:bg-primary-700 disabled:opacity-50 transition-colors flex items-center gap-1"
                        >
                          <Send className="w-3 h-3" />
                          重新发送
                        </button>
                      </div>
                    </div>
                  ) : (
                    <div
                      className={`max-w-[70%] px-4 py-3 rounded-2xl text-sm leading-relaxed ${
                        msg.role === 'user'
                          ? 'bg-primary-600 text-white rounded-br-md'
                          : 'bg-gray-100 text-gray-800 rounded-bl-md'
                      }`}
                    >
                      {msg.role === 'assistant' ? (
                        <>
                          <div className="prose prose-sm max-w-none">
                            <ReactMarkdown components={{
                              code({ className, children, ...props }) {
                                const match = /language-(\w+)/.exec(className || '')
                                const text = String(children).replace(/\n$/, '')
                                if (match) {
                                  const lang = match[1]
                                  const isRunnable = /^(bash|sh|shell)$/i.test(lang) && /^(python3?|node|bun|go run|ruby|php|perl|lua|java|rustc|gcc|g\+\+|sh |bash |\.\/)\s/i.test(text.trim())
                                  return (
                                    <CodeBlock
                                      language={lang}
                                      onRun={isRunnable ? async () => {
                                        const res = await codingAPI.runCommand(text.trim(), '', currentConversationId || '')
                                        const r = res.data.result
                                        return { stdout: r.stdout || '', stderr: r.stderr || '', exit_code: r.exit_code, duration: r.duration || '' }
                                      } : undefined}
                                    >{text}</CodeBlock>
                                  )
                                }
                                return <code className="bg-gray-200 dark:bg-gray-700 px-1 py-0.5 rounded text-sm" {...props}>{children}</code>
                              }
                            }}>{msg.content}</ReactMarkdown>
                          </div>
                          <div className="flex items-center gap-1 mt-1.5 -mb-1">
                            <button
                              onClick={() => handleCopy(msg.id, msg.content)}
                              className={`p-1 rounded transition-colors ${copiedMsgId === msg.id ? 'text-green-500' : 'text-gray-300 hover:text-gray-500'}`}
                              title="复制"
                            >
                              {copiedMsgId === msg.id ? <Check className="w-3 h-3" /> : <Copy className="w-3 h-3" />}
                            </button>
                            <button
                              onClick={() => handleFeedback(msg.id, 1)}
                              className={`p-1 rounded transition-colors ${msg.feedback === 1 ? 'text-green-600' : 'text-gray-300 hover:text-green-500'}`}
                            >
                              <ThumbsUp className="w-3 h-3" />
                            </button>
                            <button
                              onClick={() => handleFeedback(msg.id, -1)}
                              className={`p-1 rounded transition-colors ${msg.feedback === -1 ? 'text-red-500' : 'text-gray-300 hover:text-red-400'}`}
                            >
                              <ThumbsDown className="w-3 h-3" />
                            </button>
                            <button
                              onClick={() => handlePlayTTS(msg.content)}
                              className="p-1 rounded text-gray-300 hover:text-primary-500 transition-colors"
                              title="朗读"
                            >
                              <Volume2 className="w-3 h-3" />
                            </button>
                          </div>
                        </>
                      ) : (
                        <>
                          <div className="whitespace-pre-wrap">{msg.content}</div>
                          {msg.attachments && msg.attachments !== '[]' && (() => {
                            try {
                              const files = JSON.parse(msg.attachments) as { filename: string; url: string; size: number; category: string }[]
                              if (!files.length) return null
                              return (
                                <div className="mt-2 flex flex-wrap gap-1.5">
                                  {files.map((f, i) => {
                                    const Icon = getFileIcon(f.category)
                                    return (
                                      <a
                                        key={i}
                                        href={f.url}
                                        target="_blank"
                                        rel="noopener noreferrer"
                                        className="flex items-center gap-1.5 px-2 py-1 bg-white/20 rounded-lg text-xs hover:bg-white/30 transition-colors"
                                      >
                                        <Icon className="w-3.5 h-3.5" />
                                        <span className="max-w-[100px] truncate">{f.filename}</span>
                                        <span className="opacity-70">{formatSize(f.size)}</span>
                                      </a>
                                    )
                                  })}
                                </div>
                              )
                            } catch { return null }
                          })()}
                        </>
                      )}
                    </div>
                  )}
                  {msg.role === 'user' && editingMsgId !== msg.id && (
                    <div className="flex flex-col items-center gap-1">
                      <div className="flex-shrink-0 w-8 h-8 rounded-full bg-gradient-to-br from-primary-500 to-primary-700 flex items-center justify-center shadow-sm mt-0.5">
                        <User className="w-4 h-4 text-white" />
                      </div>
                      <button
                        onClick={() => { setEditingMsgId(msg.id); setEditingText(msg.content) }}
                        className="p-1 rounded-full text-gray-300 hover:text-primary-500 hover:bg-gray-100 dark:hover:bg-gray-800 transition-colors opacity-0 group-hover:opacity-100"
                        title="编辑并重发"
                      >
                        <Pencil className="w-3 h-3" />
                      </button>
                    </div>
                  )}
                </div>
              </div>
            )
          })}

          {streamingContent && (
            <div className="flex items-start gap-2.5 justify-start">
              <div className="flex-shrink-0 w-8 h-8 rounded-full bg-gradient-to-br from-red-400 to-orange-500 flex items-center justify-center shadow-sm mt-0.5">
                <CrawfishIcon className="w-5 h-5 text-white" />
              </div>
              <div className="max-w-[70%] px-4 py-3 rounded-2xl rounded-bl-md bg-gray-100 text-gray-800 text-sm leading-relaxed">
                <div className="prose prose-sm max-w-none">
                  <ReactMarkdown>{streamingContent}</ReactMarkdown>
                </div>
              </div>
            </div>
          )}

          {toolInteractions.length > 0 && (
            <div className="space-y-2">
              {toolInteractions.map((ti) => {
                const info = getToolTypeInfo(ti.toolName, ti.action)
                const Icon = info.icon
                return (
                  <div key={ti.id} className="space-y-1">
                    {ti.reasoning && (
                      <div className="flex justify-start">
                        <div className="max-w-[85%] px-3 py-1.5 text-xs text-gray-500 bg-gray-50 border border-gray-100 rounded-lg italic">
                          {ti.reasoning.length > 200 ? ti.reasoning.slice(0, 200) + '...' : ti.reasoning}
                        </div>
                      </div>
                    )}
                    <div className="flex justify-start">
                    <div className={`max-w-[85%] border rounded-xl overflow-hidden ${info.color} transition-all`}>
                      <button
                        onClick={() => ti.result && toggleToolExpanded(ti.id)}
                        className="w-full flex items-center gap-2.5 px-3 py-2 text-left"
                      >
                        <Icon className={`w-4 h-4 flex-shrink-0 ${ti.status === 'calling' ? 'animate-pulse' : ''}`} />
                        <span className="text-xs font-medium">{info.label}</span>
                        <span className="text-xs opacity-75 truncate flex-1">{ti.description}</span>
                        {ti.status === 'calling' ? (
                          <Loader2 className="w-3.5 h-3.5 animate-spin flex-shrink-0 opacity-60" />
                        ) : (
                          <>
                            <CheckCircle2 className="w-3.5 h-3.5 flex-shrink-0 opacity-60" />
                            {ti.result && (
                              ti.expanded
                                ? <ChevronDown className="w-3.5 h-3.5 flex-shrink-0 opacity-40" />
                                : <ChevronRight className="w-3.5 h-3.5 flex-shrink-0 opacity-40" />
                            )}
                          </>
                        )}
                      </button>
                      {ti.expanded && ti.result && (
                        <div className="border-t px-3 py-2 bg-white/80 dark:bg-gray-900/80">
                          <pre className="text-[11px] font-mono text-gray-700 dark:text-gray-300 whitespace-pre-wrap break-all max-h-48 overflow-y-auto scrollbar-thin">
                            {formatToolResult(ti.result)}
                          </pre>
                          {ti.toolName === 'code' && ti.action === 'write_file' && ti.args?.path && /\.(html?|htm)$/i.test(ti.args.path) && (
                            <div className="mt-2 pt-2 border-t flex items-center gap-2">
                              <a
                                href={`/v1/preview/${ti.args.workspace_id || 'default'}/${ti.args.path}`}
                                target="_blank"
                                rel="noopener noreferrer"
                                className="inline-flex items-center gap-1 text-xs text-primary-600 hover:text-primary-800 font-medium"
                              >
                                <ExternalLink className="w-3 h-3" /> 预览网页
                              </a>
                            </div>
                          )}
                          {ti.toolName === 'code' && ti.action === 'write_file' && ti.args?.path && isRunnableFile(ti.args.path) && (
                            <div className="mt-2 pt-2 border-t">
                              <div className="flex items-center gap-2">
                                {runningFileId === ti.id ? (
                                  <button
                                    onClick={() => handleStopFile()}
                                    className="inline-flex items-center gap-1 px-2.5 py-1 text-xs font-medium text-red-600 bg-red-50 border border-red-200 rounded-lg hover:bg-red-100 transition-colors"
                                  >
                                    <StopCircle className="w-3.5 h-3.5" /> 停止
                                  </button>
                                ) : (
                                  <button
                                    onClick={() => handleRunFile(ti.id, ti.args?.workspace_id || '', ti.args?.path || '')}
                                    className="inline-flex items-center gap-1 px-2.5 py-1 text-xs font-medium text-emerald-600 bg-emerald-50 border border-emerald-200 rounded-lg hover:bg-emerald-100 transition-colors"
                                  >
                                    <PlayCircle className="w-3.5 h-3.5" /> 运行
                                  </button>
                                )}
                                {fileRunResults[ti.id] && (
                                  <span className={`text-[10px] ${fileRunResults[ti.id]!.exit_code === 0 ? 'text-emerald-500' : 'text-red-500'}`}>
                                    {fileRunResults[ti.id]!.exit_code === 0 ? '✓ 成功' : '✗ 失败'} ({fileRunResults[ti.id]!.duration})
                                  </span>
                                )}
                              </div>
                              {runningFileId === ti.id && !fileRunResults[ti.id] && (
                                <div className="mt-2 flex items-center gap-2 text-xs text-gray-500">
                                  <Loader2 className="w-3 h-3 animate-spin" /> 运行中...
                                </div>
                              )}
                              {fileRunResults[ti.id] && (
                                <pre className="mt-2 text-[11px] font-mono whitespace-pre-wrap break-all max-h-48 overflow-y-auto scrollbar-thin p-2 rounded bg-gray-900 text-gray-100">
                                  {fileRunResults[ti.id]!.stdout || ''}
                                  {fileRunResults[ti.id]!.stderr ? `\n${fileRunResults[ti.id]!.stderr}` : ''}
                                </pre>
                              )}
                            </div>
                          )}
                          {ti.toolName === 'code' && ti.action === 'start_app' && ti.result && (() => {
                            try {
                              const r = JSON.parse(ti.result)
                              if (r.url) return (
                                <div className="mt-2 pt-2 border-t flex items-center gap-2">
                                  <a
                                    href={r.url}
                                    target="_blank"
                                    rel="noopener noreferrer"
                                    className="inline-flex items-center gap-1 text-xs text-emerald-600 hover:text-emerald-800 font-medium"
                                  >
                                    <ExternalLink className="w-3 h-3" /> 打开应用
                                  </a>
                                  <span className="text-[10px] text-gray-400">端口 {r.port}</span>
                                </div>
                              )
                            } catch { /* ignore */ }
                            return null
                          })()}
                        </div>
                      )}
                    </div>
                  </div>
                  </div>
                )
              })}
            </div>
          )}

          {browserScreenshots.length > 0 && (
            <div className="flex justify-start">
              <div className="max-w-[80%] space-y-2">
                {browserScreenshots.map((src, idx) => (
                  <div key={idx} className="rounded-xl overflow-hidden border shadow-sm">
                    <div className="bg-gray-100 px-3 py-1 text-xs text-gray-500 flex items-center gap-1">
                      <Globe className="w-3 h-3" /> 浏览器截图 #{idx + 1}
                    </div>
                    <img src={src} alt={`Browser screenshot ${idx + 1}`} className="w-full" />
                  </div>
                ))}
              </div>
            </div>
          )}

          {isLoading && !streamingContent && toolInteractions.every(t => t.status !== 'calling') && (
            <div className="flex justify-start">
              <div className="px-4 py-3 rounded-2xl rounded-bl-md bg-gray-100">
                <Loader2 className="w-5 h-5 text-gray-400 animate-spin" />
              </div>
            </div>
          )}

          <div ref={messagesEndRef} />
        </div>

        {/* Input */}
        <div className="px-6 py-4 border-t bg-white">
          {/* Image preview strip */}
          {attachedImages.length > 0 && (
            <div className="flex gap-2 mb-2 flex-wrap">
              {attachedImages.map((img, idx) => (
                <div key={idx} className="relative group w-16 h-16 rounded-lg overflow-hidden border">
                  <img src={img.url} alt={img.name} className="w-full h-full object-cover" />
                  <button
                    onClick={() => setAttachedImages((prev) => prev.filter((_, i) => i !== idx))}
                    className="absolute top-0 right-0 bg-black/60 text-white p-0.5 rounded-bl opacity-0 group-hover:opacity-100 transition-opacity"
                  >
                    <XIcon className="w-3 h-3" />
                  </button>
                </div>
              ))}
            </div>
          )}
          {/* File preview strip */}
          {attachedFiles.length > 0 && (
            <div className="flex gap-2 mb-2 flex-wrap">
              {attachedFiles.map((f, idx) => {
                const Icon = getFileIcon(f.category)
                return (
                  <div key={idx} className="relative group flex items-center gap-2 px-3 py-1.5 bg-gray-50 border rounded-lg text-sm">
                    <Icon className="w-4 h-4 text-gray-500 flex-shrink-0" />
                    <span className="max-w-[120px] truncate text-gray-700">{f.filename}</span>
                    <span className="text-xs text-gray-400">{formatSize(f.size)}</span>
                    <button
                      onClick={() => setAttachedFiles((prev) => prev.filter((_, i) => i !== idx))}
                      className="ml-1 text-gray-400 hover:text-red-500 transition-colors"
                    >
                      <XIcon className="w-3.5 h-3.5" />
                    </button>
                  </div>
                )
              })}
            </div>
          )}
          {fileUploading && (
            <div className="flex items-center gap-2 mb-2 text-sm text-gray-500">
              <Loader2 className="w-4 h-4 animate-spin" />
              <span>上传中...</span>
            </div>
          )}
          <div className="flex items-center gap-2">
            {/* File upload (general) */}
            <input
              ref={fileInputRef}
              type="file"
              multiple
              className="hidden"
              onChange={(e) => { handleFileUpload(e.target.files); e.target.value = '' }}
            />
            <button
              onClick={() => fileInputRef.current?.click()}
              className="p-2.5 text-gray-400 hover:text-primary-600 transition-colors"
              title="上传文件（文档/音频/视频/代码等）"
            >
              <Paperclip className="w-5 h-5" />
            </button>
            {/* Image upload */}
            <input
              ref={imageInputRef}
              type="file"
              accept="image/*"
              multiple
              className="hidden"
              onChange={(e) => handleImageUpload(e.target.files)}
            />
            <button
              onClick={() => imageInputRef.current?.click()}
              className="p-2.5 text-gray-400 hover:text-primary-600 transition-colors"
              title="上传图片"
            >
              <ImagePlus className="w-5 h-5" />
            </button>
            {/* Voice recording */}
            <button
              onClick={toggleRecording}
              className={`p-2.5 transition-colors ${isRecording ? 'text-red-500' : isTranscribing ? 'text-amber-500 animate-pulse' : 'text-gray-400 hover:text-primary-600'}`}
              title={isRecording ? '停止录音' : isTranscribing ? '转写中...' : '语音输入'}
              disabled={isTranscribing}
            >
              {isRecording ? <MicOff className="w-5 h-5" /> : isTranscribing ? <Loader2 className="w-5 h-5 animate-spin" /> : <Mic className="w-5 h-5" />}
            </button>
            <div className="flex-1 relative">
              {/* @ mention popup */}
              {showMentionPopup && filteredMentionAgents.length > 0 && (
                <div className="absolute bottom-full left-0 mb-1 w-72 bg-white border border-gray-200 rounded-xl shadow-lg overflow-hidden z-50">
                  <div className="px-3 py-1.5 text-xs text-gray-400 border-b">选择 Agent</div>
                  {filteredMentionAgents.map((agent, idx) => (
                    <button
                      key={agent.id}
                      onClick={() => handleMentionSelect(agent)}
                      className={`w-full text-left px-3 py-2 flex items-center gap-2 text-sm transition-colors ${idx === mentionIndex ? 'bg-primary-50 text-primary-700' : 'hover:bg-gray-50 text-gray-700'}`}
                    >
                      <Bot className="w-4 h-4 text-primary-500 flex-shrink-0" />
                      <div className="min-w-0">
                        <div className="font-medium truncate">{agent.name}</div>
                        {agent.description && <div className="text-xs text-gray-400 truncate">{agent.description}</div>}
                      </div>
                    </button>
                  ))}
                </div>
              )}
              {/* Recording wave overlay */}
              {isRecording && (
                <div
                  className="absolute inset-0 flex items-center gap-1 px-4 bg-gradient-to-r from-red-50 to-orange-50 border border-red-200 rounded-xl z-10 cursor-pointer"
                  onClick={toggleRecording}
                >
                  <div className="flex items-center gap-1.5 mr-2">
                    <span className="w-2.5 h-2.5 bg-red-500 rounded-full animate-pulse" />
                    <span className="text-xs font-medium text-red-600">
                      {Math.floor(recordingDuration / 60).toString().padStart(2, '0')}:{(recordingDuration % 60).toString().padStart(2, '0')}
                    </span>
                  </div>
                  <div className="flex items-center gap-[2px] flex-1 h-8 overflow-hidden">
                    {audioLevels.map((level, i) => (
                      <div
                        key={i}
                        className="flex-1 min-w-[2px] max-w-[6px] rounded-full bg-gradient-to-t from-red-400 to-orange-400 transition-all duration-75"
                        style={{ height: `${Math.max(3, level * 28)}px` }}
                      />
                    ))}
                  </div>
                  <span className="text-[10px] text-red-400 ml-2 whitespace-nowrap">点击停止</span>
                </div>
              )}
              {/* Transcribing overlay */}
              {isTranscribing && (
                <div className="absolute inset-0 flex items-center justify-center gap-2 bg-amber-50 border border-amber-200 rounded-xl z-10">
                  <Loader2 className="w-4 h-4 text-amber-500 animate-spin" />
                  <span className="text-sm text-amber-600 font-medium">语音转写中...</span>
                </div>
              )}
              <textarea
                ref={textareaRef}
                value={input}
                onChange={handleInputChange}
                onKeyDown={(e) => { if (!handleMentionKeyDown(e)) handleKeyDown(e) }}
                onPaste={handlePaste}
                onDrop={(e) => { e.preventDefault(); handleFileUpload(e.dataTransfer.files) }}
                onDragOver={(e) => e.preventDefault()}
                rows={1}
                placeholder={isRecording ? '正在录音...' : isTranscribing ? '转写中...' : '输入消息，@ 可指定Agent...'}
                className="w-full resize-none px-4 py-2.5 border rounded-xl outline-none focus:ring-2 focus:ring-primary-500 text-sm"
              />
            </div>
            {isLoading ? (
              <button
                onClick={handleStop}
                className="p-2.5 bg-red-500 text-white rounded-xl hover:bg-red-600 transition-colors animate-pulse"
                title="停止生成"
              >
                <StopCircle className="w-4 h-4" />
              </button>
            ) : (
              <button
                onClick={handleSend}
                disabled={!input.trim()}
                className="p-2.5 bg-primary-600 text-white rounded-xl hover:bg-primary-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
              >
                <Send className="w-4 h-4" />
              </button>
            )}
          </div>
        </div>
      </div>

      {/* Context Panel - right sidebar */}
      {contextPanelOpen && currentConversationId && (
        <div className="w-80 border-l bg-gray-50 dark:bg-gray-900 flex flex-col overflow-hidden">
          <div className="px-4 py-3 border-b bg-white dark:bg-gray-800 flex items-center justify-between">
            <h3 className="text-sm font-semibold text-gray-700 dark:text-gray-200">关联面板</h3>
            <button onClick={() => setContextPanelOpen(false)} className="p-1 text-gray-400 hover:text-gray-600 rounded">
              <XIcon className="w-4 h-4" />
            </button>
          </div>

          {contextLoading && !convContext ? (
            <div className="flex-1 flex items-center justify-center">
              <Loader2 className="w-5 h-5 text-gray-400 animate-spin" />
            </div>
          ) : !convContext ? (
            <div className="flex-1 flex items-center justify-center text-gray-400 text-sm">无数据</div>
          ) : (
            <div className="flex-1 overflow-y-auto scrollbar-thin">
              {/* Stats summary */}
              <div className="px-4 py-3 border-b grid grid-cols-3 gap-2">
                <div className="text-center">
                  <div className="text-lg font-bold text-primary-600">{convContext.stats?.tasks_total || 0}</div>
                  <div className="text-[10px] text-gray-400">任务</div>
                </div>
                <div className="text-center">
                  <div className="text-lg font-bold text-violet-600">{convContext.stats?.workflows_total || 0}</div>
                  <div className="text-[10px] text-gray-400">工作流</div>
                </div>
                <div className="text-center">
                  <div className="text-lg font-bold text-emerald-600">{convContext.stats?.videos_total || 0}</div>
                  <div className="text-[10px] text-gray-400">视频</div>
                </div>
              </div>

              {/* Tasks section */}
              <div className="border-b">
                <button
                  onClick={() => setContextExpandedSection(contextExpandedSection === 'tasks' ? '' : 'tasks')}
                  className="w-full px-4 py-2.5 flex items-center gap-2 text-sm font-medium text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-800"
                >
                  <ListTodo className="w-4 h-4 text-primary-500" />
                  <span>任务</span>
                  <span className="text-xs text-gray-400 ml-1">
                    {convContext.stats?.tasks_running > 0 && <span className="text-amber-500">{convContext.stats.tasks_running} 运行中</span>}
                    {convContext.stats?.tasks_running > 0 && convContext.stats?.tasks_completed > 0 && ' · '}
                    {convContext.stats?.tasks_completed > 0 && <span className="text-green-500">{convContext.stats.tasks_completed} 完成</span>}
                  </span>
                  <div className="flex-1" />
                  {contextExpandedSection === 'tasks' ? <ChevronUp className="w-3.5 h-3.5 text-gray-400" /> : <ChevronDown className="w-3.5 h-3.5 text-gray-400" />}
                </button>
                {contextExpandedSection === 'tasks' && (
                  <div className="px-3 pb-3 space-y-2">
                    {(convContext.tasks || []).length === 0 ? (
                      <p className="text-xs text-gray-400 text-center py-2">暂无任务</p>
                    ) : (
                      (convContext.tasks || []).map((task: any) => (
                        <div
                          key={task.id}
                          onClick={() => navigate('/tasks')}
                          className="p-2.5 bg-white dark:bg-gray-800 rounded-lg border cursor-pointer hover:border-primary-300 transition-colors"
                        >
                          <div className="flex items-center gap-1.5 mb-1">
                            {task.status === 'running' && <Loader2 className="w-3 h-3 text-amber-500 animate-spin" />}
                            {task.status === 'completed' && <CheckCircle2 className="w-3 h-3 text-green-500" />}
                            {task.status === 'failed' && <AlertCircle className="w-3 h-3 text-red-500" />}
                            {task.status === 'pending' && <Clock className="w-3 h-3 text-gray-400" />}
                            <span className="text-xs font-medium text-gray-700 dark:text-gray-200 truncate flex-1">{task.title}</span>
                            <ExternalLink className="w-3 h-3 text-gray-300" />
                          </div>
                          {task.progress > 0 && (
                            <div className="w-full bg-gray-200 dark:bg-gray-700 rounded-full h-1 mt-1">
                              <div className="bg-primary-500 h-1 rounded-full transition-all" style={{ width: `${task.progress}%` }} />
                            </div>
                          )}
                        </div>
                      ))
                    )}
                  </div>
                )}
              </div>

              {/* Workflows section */}
              <div className="border-b">
                <button
                  onClick={() => setContextExpandedSection(contextExpandedSection === 'workflows' ? '' : 'workflows')}
                  className="w-full px-4 py-2.5 flex items-center gap-2 text-sm font-medium text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-800"
                >
                  <Workflow className="w-4 h-4 text-violet-500" />
                  <span>工作流</span>
                  <span className="text-xs text-gray-400 ml-1">{convContext.stats?.workflows_total || 0}</span>
                  <div className="flex-1" />
                  {contextExpandedSection === 'workflows' ? <ChevronUp className="w-3.5 h-3.5 text-gray-400" /> : <ChevronDown className="w-3.5 h-3.5 text-gray-400" />}
                </button>
                {contextExpandedSection === 'workflows' && (
                  <div className="px-3 pb-3 space-y-2">
                    {(convContext.workflows || []).length === 0 ? (
                      <p className="text-xs text-gray-400 text-center py-2">暂无工作流</p>
                    ) : (
                      (convContext.workflows || []).map((wf: any) => (
                        <div
                          key={wf.id}
                          onClick={() => navigate(`/workflows/${wf.id}`)}
                          className="p-2.5 bg-white dark:bg-gray-800 rounded-lg border cursor-pointer hover:border-violet-300 transition-colors"
                        >
                          <div className="flex items-center gap-1.5">
                            <Workflow className="w-3 h-3 text-violet-500" />
                            <span className="text-xs font-medium text-gray-700 dark:text-gray-200 truncate flex-1">{wf.name}</span>
                            <ExternalLink className="w-3 h-3 text-gray-300" />
                          </div>
                          {wf.description && (
                            <p className="text-[10px] text-gray-400 mt-1 truncate">{wf.description}</p>
                          )}
                        </div>
                      ))
                    )}
                  </div>
                )}
              </div>

              {/* Videos section */}
              <div className="border-b">
                <button
                  onClick={() => setContextExpandedSection(contextExpandedSection === 'videos' ? '' : 'videos')}
                  className="w-full px-4 py-2.5 flex items-center gap-2 text-sm font-medium text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-800"
                >
                  <Video className="w-4 h-4 text-emerald-500" />
                  <span>视频</span>
                  <span className="text-xs text-gray-400 ml-1">
                    {convContext.stats?.videos_merged > 0 && <span className="text-emerald-500">{convContext.stats.videos_merged} 合成</span>}
                    {convContext.stats?.videos_merged > 0 && convContext.stats?.videos_narrated > 0 && ' · '}
                    {convContext.stats?.videos_narrated > 0 && <span className="text-blue-500">{convContext.stats.videos_narrated} 配音</span>}
                  </span>
                  <div className="flex-1" />
                  {contextExpandedSection === 'videos' ? <ChevronUp className="w-3.5 h-3.5 text-gray-400" /> : <ChevronDown className="w-3.5 h-3.5 text-gray-400" />}
                </button>
                {contextExpandedSection === 'videos' && (
                  <div className="px-3 pb-3 space-y-2">
                    {(convContext.videos || []).length === 0 ? (
                      <p className="text-xs text-gray-400 text-center py-2">暂无视频</p>
                    ) : (
                      <>
                        {/* Merged videos first */}
                        {(convContext.videos || []).filter((v: any) => v.type === 'merged').map((v: any) => (
                          <div key={v.id} className="bg-white dark:bg-gray-800 rounded-lg border overflow-hidden">
                            <video
                              src={v.video_url}
                              controls
                              className="w-full aspect-video bg-black"
                              preload="metadata"
                            />
                            <div className="px-2.5 py-1.5 flex items-center gap-1.5">
                              <PlayCircle className="w-3 h-3 text-emerald-500" />
                              <span className="text-[10px] text-gray-500 truncate flex-1">合成视频 · {v.duration}秒</span>
                              <button onClick={() => navigate('/videos')} className="text-gray-300 hover:text-gray-500">
                                <ExternalLink className="w-3 h-3" />
                              </button>
                            </div>
                          </div>
                        ))}
                        {/* Clips */}
                        <div className="grid grid-cols-2 gap-1.5">
                          {(convContext.videos || []).filter((v: any) => v.type !== 'merged' && v.status === 'succeeded').map((v: any) => (
                            <div key={v.id} className="bg-white dark:bg-gray-800 rounded-lg border overflow-hidden">
                              <video
                                src={v.narrated_url || v.video_url}
                                className="w-full aspect-video bg-black"
                                preload="metadata"
                              />
                              <div className="px-1.5 py-1 flex items-center gap-1">
                                <span className="text-[9px] text-gray-400 truncate">{v.scene || '片段'}</span>
                                {v.narrated_url && <Volume2 className="w-2.5 h-2.5 text-blue-400 flex-shrink-0" />}
                              </div>
                            </div>
                          ))}
                        </div>
                      </>
                    )}
                  </div>
                )}
              </div>

              {/* Quick nav */}
              <div className="px-4 py-3 space-y-1.5">
                <p className="text-[10px] text-gray-400 uppercase font-medium mb-2">快捷跳转</p>
                <button onClick={() => navigate('/tasks')} className="w-full text-left px-3 py-2 text-xs text-gray-600 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-800 rounded-lg flex items-center gap-2">
                  <ListTodo className="w-3.5 h-3.5" /> 任务中心
                </button>
                <button onClick={() => navigate('/workflows')} className="w-full text-left px-3 py-2 text-xs text-gray-600 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-800 rounded-lg flex items-center gap-2">
                  <Workflow className="w-3.5 h-3.5" /> 工作流
                </button>
                <button onClick={() => navigate('/videos')} className="w-full text-left px-3 py-2 text-xs text-gray-600 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-800 rounded-lg flex items-center gap-2">
                  <Video className="w-3.5 h-3.5" /> 视频画廊
                </button>
                <button onClick={() => navigate('/visualization')} className="w-full text-left px-3 py-2 text-xs text-gray-600 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-800 rounded-lg flex items-center gap-2">
                  <Eye className="w-3.5 h-3.5" /> 可视化
                </button>
              </div>
            </div>
          )}
        </div>
      )}
    </div>
  )
}
