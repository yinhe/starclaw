import { useState, useEffect, useRef, useCallback } from 'react'
import { Play, Pause, Square, RotateCcw, Volume2, VolumeX } from 'lucide-react'
import { taskAPI } from '../lib/api'

// ── Types ──────────────────────────────────────────────
interface AgentData { id: string; name: string; description: string }
interface TaskData {
  id: string; title: string; status: string; priority: string
  progress: number; progress_note: string; agent_id: string
  parent_task_id: string; started_at: string | null; completed_at: string | null
}
interface ConvSummary { id: string; title: string }
interface VisData {
  agents: AgentData[]; tasks: TaskData[]
  edges: { from: string; to: string; task_id: string; status: string; label: string }[]
  stats: { total: number; running: number; pending: number; completed: number; failed: number }
  notifications: { id: string; title: string; content: string; type: string; created_at: string }[]
  worker_paused: boolean
  conversations: ConvSummary[]
  conversation_id: string
}

// ── Crayfish Character ─────────────────────────────────
interface CrawChar {
  id: string
  name: string
  role: 'pm' | 'worker'
  x: number; y: number
  targetX: number; targetY: number
  color: string
  status: 'idle' | 'working' | 'walking' | 'talking'
  walkPhase: number
  tasks: TaskData[]
  progress: number
  progressNote: string
  bubble: string
  bubbleTimer: number
  talkTarget: string
  deskX: number; deskY: number
}

// ── Message Particle ───────────────────────────────────
interface MsgParticle {
  fromId: string; toId: string; t: number; text: string; color: string
}

const STATUS_COLORS: Record<string, string> = {
  running: '#3b82f6', pending: '#eab308', waiting: '#a855f7',
  completed: '#22c55e', failed: '#ef4444', cancelled: '#6b7280',
}

const ROLE_COLORS = ['#ef4444', '#3b82f6', '#22c55e', '#f59e0b', '#8b5cf6', '#ec4899', '#06b6d4', '#f97316']

// ── Main Component ─────────────────────────────────────
export default function VisualizationPage() {
  const canvasRef = useRef<HTMLCanvasElement>(null)
  const containerRef = useRef<HTMLDivElement>(null)
  const animRef = useRef<number>(0)
  const charsRef = useRef<CrawChar[]>([])
  const particlesRef = useRef<MsgParticle[]>([])
  const dataRef = useRef<VisData | null>(null)
  const hoveredRef = useRef<CrawChar | null>(null)
  const mouseRef = useRef({ x: 0, y: 0 })
  const timeRef = useRef(0)
  const lastSpokeRef = useRef<Map<string, number>>(new Map())

  const [data, setData] = useState<VisData | null>(null)
  const [paused, setPaused] = useState(false)
  const [selectedChar, setSelectedChar] = useState<CrawChar | null>(null)
  const [muted, setMuted] = useState(false)
  const mutedRef = useRef(false)
  const [conversations, setConversations] = useState<ConvSummary[]>([])
  const [selectedConvId, setSelectedConvId] = useState<string>('')

  // Keep ref in sync with state for use inside animation loop
  useEffect(() => { mutedRef.current = muted }, [muted])

  // ── TTS Speech utility ───────────────────────────────
  const speak = useCallback((text: string, charName: string, pitch?: number) => {
    if (mutedRef.current) return
    if (!window.speechSynthesis) return
    // Throttle: same character can't speak within 4 seconds
    const now = Date.now()
    const lastTime = lastSpokeRef.current.get(charName) || 0
    if (now - lastTime < 4000) return
    lastSpokeRef.current.set(charName, now)
    // Cancel if too many queued
    if (window.speechSynthesis.pending) {
      const queued = window.speechSynthesis.pending
      if (queued) return // skip if already queued
    }
    const utter = new SpeechSynthesisUtterance(text)
    utter.lang = 'zh-CN'
    utter.rate = 1.1
    utter.pitch = pitch ?? 1.0
    utter.volume = 0.6
    // Try to pick a Chinese voice
    const voices = window.speechSynthesis.getVoices()
    const zhVoice = voices.find(v => v.lang.startsWith('zh'))
    if (zhVoice) utter.voice = zhVoice
    window.speechSynthesis.speak(utter)
  }, [])

  // ── Fetch ──────────────────────────────────────────
  const selectedConvRef = useRef('')
  useEffect(() => { selectedConvRef.current = selectedConvId }, [selectedConvId])

  const fetchData = useCallback(async () => {
    try {
      const cid = selectedConvRef.current || undefined
      const res = await taskAPI.visualization(cid)
      const d = res.data as VisData
      dataRef.current = d
      setData(d)
      setPaused(d.worker_paused)
      if (d.conversations) setConversations(d.conversations)
      // Auto-select first conversation if none selected
      if (!selectedConvRef.current && d.conversations?.length > 0) {
        selectedConvRef.current = d.conversations[0].id
        setSelectedConvId(d.conversations[0].id)
      }
      buildChars(d)
    } catch (e) { console.error('vis fetch', e) }
  }, [])

  useEffect(() => {
    fetchData()
    const iv = setInterval(fetchData, 4000)
    return () => clearInterval(iv)
  }, [fetchData])

  // ── Worker controls ────────────────────────────────
  const handlePause = async () => { await taskAPI.workerPause(); setPaused(true) }
  const handleResume = async () => { await taskAPI.workerResume(); setPaused(false) }
  const handleStop = async () => { await taskAPI.workerStop(); setPaused(true); fetchData() }
  const handleRestart = async () => { await taskAPI.workerResume(); setPaused(false); fetchData() }

  // ── Build characters from data ─────────────────────
  const buildChars = useCallback((d: VisData) => {
    const canvas = canvasRef.current
    if (!canvas) return
    const W = canvas.width, H = canvas.height
    const existing = new Map<string, CrawChar>()
    charsRef.current.forEach(c => existing.set(c.id, c))

    const chars: CrawChar[] = []
    const floorY = H * 0.55
    const pmDeskX = W * 0.5, pmDeskY = floorY - 80

    // PM (project manager)
    const pmOld = existing.get('pm')
    chars.push({
      id: 'pm', name: '项目经理', role: 'pm',
      x: pmOld?.x ?? pmDeskX, y: pmOld?.y ?? pmDeskY,
      targetX: pmDeskX, targetY: pmDeskY,
      color: '#dc2626',
      status: (d.tasks || []).some(t => t.status === 'running') ? 'working' : 'idle',
      walkPhase: pmOld?.walkPhase ?? 0,
      tasks: d.tasks || [],
      progress: 0, progressNote: '',
      bubble: (d.tasks || []).some(t => t.status === 'running') ? '团队加油！' : '等待任务中...',
      bubbleTimer: 300,
      talkTarget: '', deskX: pmDeskX, deskY: pmDeskY,
    })

    // Worker agents
    const agents = d.agents || []
    const spacing = Math.min(180, (W - 200) / Math.max(agents.length, 1))
    const startX = (W - spacing * (agents.length - 1)) / 2

    agents.forEach((agent, i) => {
      const old = existing.get(agent.id)
      const agentTasks = (d.tasks || []).filter(t => t.agent_id === agent.id)
      const running = agentTasks.filter(t => t.status === 'running')
      const deskX = startX + i * spacing
      const deskY = floorY + 60

      let status: CrawChar['status'] = 'idle'
      let bubble = '等待分配任务...'
      const completedTasks = agentTasks.filter(t => t.status === 'completed')
      const failedTasks = agentTasks.filter(t => t.status === 'failed')
      if (running.length > 0) {
        status = 'working'
        const rt = running[0]
        bubble = rt.progress_note || `正在${rt.title.slice(0, 10)} ${rt.progress}%`
      } else if (completedTasks.length > 0) {
        const ct = completedTasks[completedTasks.length - 1]
        bubble = `已完成「${ct.title.slice(0, 8)}」✅`
      } else if (failedTasks.length > 0) {
        const ft = failedTasks[failedTasks.length - 1]
        bubble = `「${ft.title.slice(0, 8)}」遇到问题 ❌`
      } else if (agentTasks.some(t => t.status === 'pending')) {
        const pt = agentTasks.find(t => t.status === 'pending')!
        bubble = `准备开始「${pt.title.slice(0, 8)}」...`
      }

      // Randomly walk to PM or another agent to "talk"
      let tx = deskX, ty = deskY
      let talkTarget = ''
      if (old && old.status === 'walking' && Math.hypot(old.x - old.targetX, old.y - old.targetY) > 5) {
        tx = old.targetX; ty = old.targetY
        status = 'walking'
        talkTarget = old.talkTarget
      } else if (status === 'working' && Math.random() < 0.02 && !old?.talkTarget) {
        // Walk to PM to report real progress
        tx = pmDeskX + (Math.random() - 0.5) * 60
        ty = pmDeskY + 40
        status = 'walking'
        talkTarget = 'pm'
        const rt = running[0]
        bubble = `汇报「${rt.title.slice(0, 8)}」${rt.progress}%`
      }

      chars.push({
        id: agent.id, name: agent.name, role: 'worker',
        x: old?.x ?? deskX, y: old?.y ?? deskY,
        targetX: tx, targetY: ty,
        color: ROLE_COLORS[i % ROLE_COLORS.length],
        status, walkPhase: old?.walkPhase ?? Math.random() * Math.PI * 2,
        tasks: agentTasks,
        progress: running.length > 0 ? running[0].progress : 0,
        progressNote: running.length > 0 ? (running[0].progress_note || '') : '',
        bubble, bubbleTimer: old?.bubbleTimer ?? 200 + Math.random() * 100,
        talkTarget, deskX, deskY,
      })
    })

    charsRef.current = chars

    // Spawn message particles for running task assignments
    const runningEdges = (d.edges || []).filter(e => e.status === 'running')
    runningEdges.forEach(edge => {
      if (particlesRef.current.filter(p => p.fromId === 'pm' && p.toId === edge.to).length < 1) {
        particlesRef.current.push({
          fromId: 'pm', toId: edge.to, t: 0,
          text: '📋', color: STATUS_COLORS[edge.status] || '#3b82f6',
        })
      }
    })
  }, [])

  // ── Animation loop ─────────────────────────────────
  const animate = useCallback(() => {
    const canvas = canvasRef.current
    if (!canvas) return
    const ctx = canvas.getContext('2d')
    if (!ctx) return
    const W = canvas.width, H = canvas.height
    timeRef.current += 0.016

    // ── Office background + task kanban ──
    drawOffice(ctx, W, H, timeRef.current, dataRef.current?.tasks || [])

    const chars = charsRef.current
    const charMap = new Map<string, CrawChar>()
    chars.forEach(c => charMap.set(c.id, c))

    // ── Update positions ──
    chars.forEach(c => {
      const speed = c.role === 'pm' ? 0.03 : 0.04
      const dx = c.targetX - c.x, dy = c.targetY - c.y
      const dist = Math.hypot(dx, dy)
      if (dist > 2) {
        c.x += dx * speed
        c.y += dy * speed
        c.walkPhase += 0.15
        if (c.status !== 'walking') c.status = 'walking'
      } else if (c.status === 'walking') {
        // Arrived at destination — real task conversation
        if (c.talkTarget) {
          c.status = 'talking'
          c.bubbleTimer = 150
          const target = charMap.get(c.talkTarget)
          const myRunning = c.tasks.find(t => t.status === 'running')
          const myCompleted = c.tasks.filter(t => t.status === 'completed')
          const myFailed = c.tasks.find(t => t.status === 'failed')
          if (c.talkTarget === 'pm') {
            if (myRunning) {
              c.bubble = `「${myRunning.title.slice(0, 8)}」进度${myRunning.progress}%`
            } else if (myCompleted.length > 0) {
              c.bubble = `「${myCompleted[myCompleted.length-1].title.slice(0, 8)}」已完成!`
            } else if (myFailed) {
              c.bubble = `「${myFailed.title.slice(0, 8)}」出了问题...`
            } else {
              c.bubble = '目前没有进行中的任务'
            }
          } else if (c.role === 'pm') {
            if (target && target.tasks.length > 0) {
              const tt = target.tasks.find(t => t.status === 'running') || target.tasks[0]
              c.bubble = `「${tt.title.slice(0, 8)}」进展如何?`
            } else {
              c.bubble = `${target?.name || ''}有新任务给你`
            }
          } else {
            if (myRunning && target) {
              const peerTask = target.tasks.find(t => t.status === 'running')
              c.bubble = peerTask ? `你的「${peerTask.title.slice(0, 6)}」怎么样了?` : `我在做「${myRunning.title.slice(0, 8)}」`
            } else {
              c.bubble = '交流一下进展'
            }
          }
          // 🔊 Speak the initiator's bubble
          speak(c.bubble, c.name, c.role === 'pm' ? 0.8 : 1.0 + Math.random() * 0.3)
          if (target) {
            target.status = 'talking'
            target.bubbleTimer = 150
            const tgtRunning = target.tasks.find(t => t.status === 'running')
            const tgtCompleted = target.tasks.filter(t => t.status === 'completed')
            if (c.role === 'pm') {
              if (tgtRunning) {
                target.bubble = `正在做，进度${tgtRunning.progress}%`
              } else if (tgtCompleted.length > 0) {
                target.bubble = `「${tgtCompleted[tgtCompleted.length-1].title.slice(0, 8)}」搞定了!`
              } else {
                target.bubble = '等待新任务分配'
              }
            } else if (c.talkTarget === 'pm') {
              if (myRunning && myRunning.progress >= 80) {
                target.bubble = '快完成了，加油！'
              } else if (myRunning && myRunning.progress >= 50) {
                target.bubble = '进度不错，继续'
              } else if (myFailed) {
                target.bubble = '我看看怎么解决'
              } else if (myRunning) {
                target.bubble = `收到，${myRunning.title.slice(0, 6)}继续推进`
              } else {
                target.bubble = '好的，稍后安排'
              }
            } else {
              if (tgtRunning) {
                target.bubble = `我在忙「${tgtRunning.title.slice(0, 6)}」${tgtRunning.progress}%`
              } else {
                target.bubble = '我这边完成了'
              }
            }
            // 🔊 Speak the responder's bubble (with slight delay via setTimeout)
            const replyText = target.bubble
            const replyName = target.name
            const replyPitch = target.role === 'pm' ? 0.8 : 1.0 + Math.random() * 0.3
            setTimeout(() => speak(replyText, replyName, replyPitch), 2000)
          }
        } else {
          c.status = c.tasks.some(t => t.status === 'running') ? 'working' : 'idle'
        }
      }

      // Return to desk after talking
      if (c.status === 'talking') {
        c.bubbleTimer--
        if (c.bubbleTimer <= 0) {
          c.targetX = c.deskX
          c.targetY = c.deskY
          c.talkTarget = ''
          c.status = 'walking'
        }
      }

      // Idle bob
      if (c.status === 'idle') c.walkPhase += 0.02
      if (c.status === 'working') c.walkPhase += 0.08

      // Random walk triggers — driven by real task data
      if (c.role === 'worker' && c.status !== 'walking' && c.status !== 'talking') {
        const myRunning = c.tasks.find(t => t.status === 'running')
        const myFailed = c.tasks.find(t => t.status === 'failed')
        const r = Math.random()
        if (r < 0.003 && c.tasks.length > 0) {
          // Walk to PM to report real task
          const pm = charMap.get('pm')
          if (pm) {
            c.targetX = pm.deskX + (Math.random() - 0.5) * 80
            c.targetY = pm.deskY + 30 + Math.random() * 20
            c.status = 'walking'
            c.talkTarget = 'pm'
            if (myRunning) {
              c.bubble = `汇报「${myRunning.title.slice(0, 8)}」${myRunning.progress}%`
            } else if (myFailed) {
              c.bubble = `「${myFailed.title.slice(0, 8)}」需要帮助`
            } else {
              c.bubble = '请求新任务'
            }
            c.bubbleTimer = 200
          }
        } else if (r < 0.005) {
          // Walk to another agent to discuss real work
          const others = chars.filter(o => o.id !== c.id && o.role === 'worker' && o.status !== 'walking')
          if (others.length > 0) {
            const peer = others[Math.floor(Math.random() * others.length)]
            c.targetX = peer.deskX + (Math.random() - 0.5) * 40
            c.targetY = peer.deskY
            c.status = 'walking'
            c.talkTarget = peer.id
            const peerTask = peer.tasks.find(t => t.status === 'running')
            if (myRunning && peerTask) {
              c.bubble = `「${myRunning.title.slice(0, 6)}」和你的「${peerTask.title.slice(0, 6)}」有关联`
            } else if (myRunning) {
              c.bubble = `聊聊「${myRunning.title.slice(0, 8)}」`
            } else {
              c.bubble = '看看你在做什么'
            }
            c.bubbleTimer = 200
          }
        } else if (r < 0.006 && !myRunning) {
          // Stroll only when idle
          c.targetX = c.deskX + (Math.random() - 0.5) * 120
          c.targetY = c.deskY + (Math.random() - 0.5) * 40
          c.status = 'walking'
          c.talkTarget = ''
          c.bubble = '看看有什么能帮忙的'
          c.bubbleTimer = 100
        }
      }
      // PM walks to check on specific agent's real task
      if (c.role === 'pm' && c.status !== 'walking' && c.status !== 'talking' && Math.random() < 0.002) {
        const busyWorkers = chars.filter(o => o.role === 'worker' && o.tasks.length > 0)
        const target = busyWorkers.length > 0 ? busyWorkers[Math.floor(Math.random() * busyWorkers.length)] : null
        if (target) {
          c.targetX = target.deskX + (Math.random() - 0.5) * 40
          c.targetY = target.deskY - 20
          c.status = 'walking'
          c.talkTarget = target.id
          const tt = target.tasks.find(t => t.status === 'running') || target.tasks[0]
          c.bubble = `检查「${tt.title.slice(0, 8)}」进度`
          c.bubbleTimer = 200
        }
      }
    })

    // ── Draw desks ──
    chars.forEach(c => {
      if (c.role === 'pm') {
        drawDesk(ctx, c.deskX, c.deskY + 25, 80, true)
      } else {
        drawDesk(ctx, c.deskX, c.deskY + 25, 60, false)
      }
    })

    // ── Draw connection lines (when talking) ──
    chars.forEach(c => {
      if (c.talkTarget && (c.status === 'walking' || c.status === 'talking')) {
        const target = charMap.get(c.talkTarget)
        if (target) {
          ctx.save()
          ctx.setLineDash([4, 4])
          ctx.strokeStyle = 'rgba(255,255,255,0.15)'
          ctx.lineWidth = 1
          ctx.beginPath()
          ctx.moveTo(c.x, c.y)
          ctx.lineTo(target.x, target.y)
          ctx.stroke()
          ctx.restore()
        }
      }
    })

    // ── Draw message particles ──
    particlesRef.current = particlesRef.current.filter(p => {
      p.t += 0.008
      if (p.t > 1) return false
      const from = charMap.get(p.fromId)
      const to = charMap.get(p.toId)
      if (!from || !to) return false
      const x = from.x + (to.x - from.x) * p.t
      const y = from.y + (to.y - from.y) * p.t - Math.sin(p.t * Math.PI) * 30
      ctx.font = '16px sans-serif'
      ctx.textAlign = 'center'
      ctx.globalAlpha = 1 - p.t * 0.5
      ctx.fillText(p.text, x, y)
      ctx.globalAlpha = 1
      return true
    })

    // ── Draw characters (sorted by y for 3D depth) ──
    hoveredRef.current = null
    const mx = mouseRef.current.x, my = mouseRef.current.y
    const sorted = [...chars].sort((a, b) => a.y - b.y)

    sorted.forEach(c => {
      // 3D perspective: characters further back (lower y) are smaller
      const depthFactor = 0.7 + (c.y / H) * 0.5
      const dist = Math.hypot(mx - c.x, my - c.y)
      if (dist < 35 * depthFactor) hoveredRef.current = c
      const isHover = dist < 35 * depthFactor
      drawCrayfish(ctx, c, timeRef.current, isHover, depthFactor)
    })

    // ── Tooltip ──
    drawTooltip(ctx, hoveredRef.current, W, H)

    animRef.current = requestAnimationFrame(animate)
  }, [])

  // ── Canvas setup ───────────────────────────────────
  useEffect(() => {
    const canvas = canvasRef.current
    const container = containerRef.current
    if (!canvas || !container) return

    const resize = () => {
      canvas.width = container.clientWidth
      canvas.height = container.clientHeight
      if (dataRef.current) buildChars(dataRef.current)
    }
    resize()
    window.addEventListener('resize', resize)

    const handleMouse = (e: MouseEvent) => {
      const r = canvas.getBoundingClientRect()
      mouseRef.current = { x: e.clientX - r.left, y: e.clientY - r.top }
    }
    const handleClick = () => {
      setSelectedChar(hoveredRef.current)
    }
    canvas.addEventListener('mousemove', handleMouse)
    canvas.addEventListener('click', handleClick)
    animRef.current = requestAnimationFrame(animate)

    return () => {
      window.removeEventListener('resize', resize)
      canvas.removeEventListener('mousemove', handleMouse)
      canvas.removeEventListener('click', handleClick)
      cancelAnimationFrame(animRef.current)
    }
  }, [animate, buildChars])

  const stats = data?.stats

  return (
    <div className="h-full flex flex-col bg-[#1a1a2e] overflow-hidden">
      {/* Top bar */}
      <div className="flex-none flex items-center justify-between px-5 py-2.5 border-b border-white/10 bg-[#16162a]/90 backdrop-blur-sm z-10">
        <div className="flex items-center gap-4">
          <span className="text-base font-bold text-white">🦞 StarClaw 团队协作</span>
          {/* Conversation selector */}
          {conversations.length > 0 && (
            <select
              value={selectedConvId}
              onChange={e => { setSelectedConvId(e.target.value); selectedConvRef.current = e.target.value; fetchData() }}
              className="bg-[#2a2a4a] text-white text-xs border border-white/20 rounded-md px-2 py-1 max-w-[200px] truncate focus:outline-none focus:border-blue-500"
            >
              <option value="">全部会话</option>
              {conversations.map(c => (
                <option key={c.id} value={c.id}>{c.title || c.id.slice(0, 8)}</option>
              ))}
            </select>
          )}
          {/* Controls */}
          <div className="flex items-center gap-1.5 ml-2">
            {paused ? (
              <button onClick={handleResume} className="flex items-center gap-1 px-3 py-1 rounded-full bg-green-600 hover:bg-green-500 text-white text-xs font-medium transition">
                <Play size={12} /> 继续
              </button>
            ) : (
              <button onClick={handlePause} className="flex items-center gap-1 px-3 py-1 rounded-full bg-amber-600 hover:bg-amber-500 text-white text-xs font-medium transition">
                <Pause size={12} /> 暂停
              </button>
            )}
            <button onClick={handleStop} className="flex items-center gap-1 px-3 py-1 rounded-full bg-red-600 hover:bg-red-500 text-white text-xs font-medium transition">
              <Square size={12} /> 停止
            </button>
            <button onClick={handleRestart} className="flex items-center gap-1 px-3 py-1 rounded-full bg-violet-600 hover:bg-violet-500 text-white text-xs font-medium transition">
              <RotateCcw size={12} /> 重启
            </button>
            <button onClick={() => { setMuted(m => !m); if (!muted) window.speechSynthesis?.cancel() }} className={`flex items-center gap-1 px-3 py-1 rounded-full text-white text-xs font-medium transition ${muted ? 'bg-gray-600 hover:bg-gray-500' : 'bg-cyan-600 hover:bg-cyan-500'}`}>
              {muted ? <><VolumeX size={12} /> 静音</> : <><Volume2 size={12} /> 语音</>}
            </button>
            {paused && <span className="text-xs text-amber-400 ml-2 animate-pulse">⏸ 已暂停</span>}
          </div>
        </div>
        {stats && (
          <div className="flex items-center gap-4 text-xs">
            <SB color="#3b82f6" label="运行" value={stats.running} pulse />
            <SB color="#eab308" label="排队" value={stats.pending} />
            <SB color="#22c55e" label="完成" value={stats.completed} />
            <SB color="#ef4444" label="失败" value={stats.failed} />
            <span className="text-gray-500">共 {stats.total}</span>
          </div>
        )}
      </div>

      {/* Canvas */}
      <div ref={containerRef} className="flex-1 relative">
        <canvas ref={canvasRef} className="absolute inset-0 cursor-pointer" />

        {/* Detail panel */}
        {selectedChar && (
          <div className="absolute right-4 top-4 w-72 bg-[#16162a]/95 border border-white/10 rounded-xl p-4 backdrop-blur-md shadow-2xl z-10">
            <div className="flex items-center justify-between mb-3">
              <h3 className="text-white font-bold text-sm">🦞 {selectedChar.name}</h3>
              <button onClick={() => setSelectedChar(null)} className="text-gray-500 hover:text-white">×</button>
            </div>
            <div className="text-xs text-gray-400 mb-2">{selectedChar.role === 'pm' ? '👔 项目经理' : '🔧 AI Agent'}</div>
            <div className="flex items-center gap-2 mb-3">
              <span className="w-2 h-2 rounded-full" style={{ backgroundColor: selectedChar.color }} />
              <span className="text-xs text-gray-300 capitalize">{selectedChar.status}</span>
              {selectedChar.progress > 0 && <span className="text-xs text-green-400 ml-auto font-mono">{selectedChar.progress}%</span>}
            </div>
            {selectedChar.progressNote && (
              <div className="text-xs text-blue-400 mb-3 bg-blue-500/10 px-2 py-1 rounded">{selectedChar.progressNote}</div>
            )}
            <div className="space-y-2 max-h-48 overflow-y-auto">
              {selectedChar.tasks.map(task => (
                <div key={task.id} className="bg-white/5 rounded-lg px-3 py-2">
                  <div className="flex items-center gap-2">
                    <span className="w-1.5 h-1.5 rounded-full flex-none" style={{ backgroundColor: STATUS_COLORS[task.status] || '#6b7280' }} />
                    <span className="text-xs text-white truncate">{task.title}</span>
                  </div>
                  {task.progress > 0 && (
                    <div className="mt-1.5 h-1 bg-white/10 rounded-full overflow-hidden">
                      <div className="h-full rounded-full transition-all" style={{ width: `${task.progress}%`, backgroundColor: STATUS_COLORS[task.status] }} />
                    </div>
                  )}
                </div>
              ))}
              {selectedChar.tasks.length === 0 && <p className="text-xs text-gray-600 text-center py-2">暂无任务</p>}
            </div>
          </div>
        )}

        {/* Activity feed */}
        {data?.notifications && data.notifications.length > 0 && (
          <div className="absolute left-4 bottom-4 w-60 bg-[#16162a]/90 border border-white/10 rounded-xl p-3 backdrop-blur-md z-10 max-h-44 overflow-y-auto">
            <h4 className="text-xs font-semibold text-gray-400 mb-2">📡 团队动态</h4>
            {data.notifications.slice(0, 5).map(n => (
              <div key={n.id} className="flex items-start gap-2 mb-2">
                <span className="text-[10px] mt-0.5">🦞</span>
                <div>
                  <p className="text-xs text-gray-300">{n.title}</p>
                  <p className="text-[10px] text-gray-600">{new Date(n.created_at).toLocaleTimeString('zh-CN')}</p>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  )
}

// ── Draw office background ───────────────────────────
function drawOffice(ctx: CanvasRenderingContext2D, W: number, H: number, t: number, tasks: TaskData[]) {
  // Sky gradient
  const sky = ctx.createLinearGradient(0, 0, 0, H)
  sky.addColorStop(0, '#0f0f23')
  sky.addColorStop(0.3, '#1a1a3e')
  sky.addColorStop(0.5, '#252550')
  sky.addColorStop(1, '#1e1e3a')
  ctx.fillStyle = sky; ctx.fillRect(0, 0, W, H)

  // 3D perspective floor
  const floorY = H * 0.55
  const floorGrad = ctx.createLinearGradient(0, floorY + 40, 0, H)
  floorGrad.addColorStop(0, '#1e1e38')
  floorGrad.addColorStop(1, '#12122a')
  ctx.fillStyle = floorGrad
  ctx.fillRect(0, floorY + 40, W, H)

  // Perspective grid on floor
  ctx.strokeStyle = 'rgba(100,100,200,0.08)'; ctx.lineWidth = 1
  const vanishX = W / 2
  for (let i = 0; i <= 12; i++) {
    const x = (i / 12) * W
    ctx.beginPath(); ctx.moveTo(x, H); ctx.lineTo(vanishX + (x - vanishX) * 0.3, floorY + 40); ctx.stroke()
  }
  for (let j = 0; j < 8; j++) {
    const ratio = j / 8
    const y = floorY + 40 + (H - floorY - 40) * ratio
    const shrink = 1 - ratio * 0.3
    ctx.beginPath(); ctx.moveTo(W * (1 - shrink) / 2, y); ctx.lineTo(W * (1 + shrink) / 2, y); ctx.stroke()
  }

  // ── Task Kanban Board (left wall) ──
  const wbX = W * 0.04, wbY = H * 0.04, wbW = W * 0.28, wbH = H * 0.42
  ctx.fillStyle = '#141430'; ctx.fillRect(wbX, wbY, wbW, wbH)
  ctx.strokeStyle = '#3a3a6a'; ctx.lineWidth = 2; ctx.strokeRect(wbX, wbY, wbW, wbH)

  // Kanban header
  ctx.fillStyle = '#2a2a5a'; ctx.fillRect(wbX, wbY, wbW, 28)
  ctx.fillStyle = '#8888cc'; ctx.font = 'bold 12px sans-serif'; ctx.textAlign = 'center'
  ctx.fillText('📊 任务看板', wbX + wbW / 2, wbY + 18)

  // Overall progress
  const total = tasks.length
  const completed = tasks.filter(t => t.status === 'completed').length
  const running = tasks.filter(t => t.status === 'running').length
  const failed = tasks.filter(t => t.status === 'failed').length
  const pct = total > 0 ? Math.round((completed / total) * 100) : 0

  if (total > 0) {
    // Progress bar
    const pbX = wbX + 10, pbY = wbY + 36, pbW = wbW - 20, pbH = 10
    ctx.fillStyle = '#1a1a35'; roundRect(ctx, pbX, pbY, pbW, pbH, 4); ctx.fill()
    if (pct > 0) {
      ctx.fillStyle = '#22c55e'; roundRect(ctx, pbX, pbY, pbW * pct / 100, pbH, 4); ctx.fill()
    }
    ctx.fillStyle = '#8888bb'; ctx.font = '10px sans-serif'; ctx.textAlign = 'left'
    ctx.fillText(`总进度 ${pct}%  (${completed}/${total} 完成, ${running} 运行, ${failed} 失败)`, pbX, pbY + 22)

    // Task list
    const visibleTasks = tasks.slice(0, Math.min(8, Math.floor((wbH - 70) / 24)))
    visibleTasks.forEach((task, i) => {
      const ty = wbY + 66 + i * 24
      const sc = STATUS_COLORS[task.status] || '#6b7280'
      // Status dot
      ctx.fillStyle = sc
      ctx.beginPath(); ctx.arc(wbX + 18, ty + 4, 4, 0, Math.PI * 2); ctx.fill()
      // Title
      ctx.fillStyle = '#c8c8e0'; ctx.font = '11px sans-serif'; ctx.textAlign = 'left'
      const title = task.title.length > 14 ? task.title.slice(0, 14) + '…' : task.title
      ctx.fillText(title, wbX + 28, ty + 7)
      // Mini progress bar
      const mpX = wbX + wbW - 70, mpW = 50, mpH = 6
      ctx.fillStyle = '#1a1a30'; roundRect(ctx, mpX, ty, mpW, mpH, 3); ctx.fill()
      if (task.progress > 0) {
        ctx.fillStyle = sc; roundRect(ctx, mpX, ty, mpW * task.progress / 100, mpH, 3); ctx.fill()
      }
      ctx.fillStyle = '#8888aa'; ctx.font = '9px monospace'; ctx.textAlign = 'right'
      ctx.fillText(`${task.progress}%`, wbX + wbW - 8, ty + 7)
    })
    if (tasks.length > visibleTasks.length) {
      ctx.fillStyle = '#5555aa'; ctx.font = '10px sans-serif'; ctx.textAlign = 'center'
      ctx.fillText(`... 还有 ${tasks.length - visibleTasks.length} 个任务`, wbX + wbW / 2, wbY + wbH - 8)
    }
  } else {
    ctx.fillStyle = '#4a4a7a'; ctx.font = '12px sans-serif'; ctx.textAlign = 'center'
    ctx.fillText('暂无任务', wbX + wbW / 2, wbY + wbH / 2)
    ctx.fillText('在对话中创建任务即可看到', wbX + wbW / 2, wbY + wbH / 2 + 18)
  }

  // ── Company branding (right side) ──
  ctx.fillStyle = 'rgba(255,255,255,0.04)'; ctx.font = `bold ${W * 0.035}px sans-serif`
  ctx.textAlign = 'center'; ctx.fillText('🦞 StarClaw Inc.', W * 0.65, H * 0.1)

  // Stats summary (right wall)
  if (total > 0) {
    const sx = W * 0.7, sy = H * 0.16
    ctx.fillStyle = '#141430'; roundRect(ctx, sx, sy, W * 0.22, 70, 8); ctx.fill()
    ctx.strokeStyle = '#3a3a6a'; ctx.lineWidth = 1; roundRect(ctx, sx, sy, W * 0.22, 70, 8); ctx.stroke()
    ctx.font = 'bold 11px sans-serif'; ctx.textAlign = 'left'
    ctx.fillStyle = '#8888cc'; ctx.fillText('📈 实时统计', sx + 12, sy + 18)
    ctx.font = '11px sans-serif'
    ctx.fillStyle = '#3b82f6'; ctx.fillText(`⚡ 运行中: ${running}`, sx + 12, sy + 36)
    ctx.fillStyle = '#eab308'; ctx.fillText(`⏳ 排队: ${tasks.filter(t => t.status === 'pending' || t.status === 'waiting').length}`, sx + 12, sy + 52)
    ctx.fillStyle = '#22c55e'; ctx.fillText(`✅ 完成: ${completed}`, sx + W * 0.11, sy + 36)
    ctx.fillStyle = '#ef4444'; ctx.fillText(`❌ 失败: ${failed}`, sx + W * 0.11, sy + 52)
  }

  // Ambient particles
  ctx.globalAlpha = 0.3
  for (let i = 0; i < 20; i++) {
    const px = (Math.sin(t * 0.3 + i * 1.7) * 0.5 + 0.5) * W
    const py = (Math.cos(t * 0.2 + i * 2.3) * 0.5 + 0.5) * H * 0.5
    ctx.fillStyle = `hsl(${(i * 40 + t * 20) % 360}, 60%, 60%)`
    ctx.beginPath(); ctx.arc(px, py, 2, 0, Math.PI * 2); ctx.fill()
  }
  ctx.globalAlpha = 1
}

// ── Draw tooltip ─────────────────────────────────────
function drawTooltip(ctx: CanvasRenderingContext2D, hov: CrawChar | null, W: number, H: number) {
  if (!hov) return
  const tw = 240, th = hov.tasks.length > 0 ? 80 + Math.min(hov.tasks.length, 3) * 20 : 65
  let tx = hov.x + 40, ty = hov.y - th / 2
  if (tx + tw > W) tx = hov.x - tw - 40
  if (ty < 5) ty = 5
  if (ty + th > H - 5) ty = H - th - 5

  ctx.save()
  ctx.fillStyle = 'rgba(20, 20, 40, 0.94)'
  ctx.strokeStyle = hov.color + '60'
  ctx.lineWidth = 1
  roundRect(ctx, tx, ty, tw, th, 10); ctx.fill(); ctx.stroke()

  ctx.fillStyle = '#fff'; ctx.font = 'bold 13px sans-serif'
  ctx.textAlign = 'left'; ctx.textBaseline = 'top'
  ctx.fillText(`🦞 ${hov.name}`, tx + 12, ty + 10)

  ctx.fillStyle = '#94a3b8'; ctx.font = '11px sans-serif'
  ctx.fillText(hov.role === 'pm' ? '👔 项目经理 · 负责任务分配协调' : '🔧 AI Agent · 自主执行任务', tx + 12, ty + 28)

  if (hov.progressNote) {
    ctx.fillStyle = '#60a5fa'
    ctx.fillText(hov.progressNote, tx + 12, ty + 46)
  }

  hov.tasks.slice(0, 3).forEach((task: TaskData, i: number) => {
    const yy = ty + 60 + i * 20
    ctx.fillStyle = STATUS_COLORS[task.status] || '#6b7280'
    ctx.beginPath(); ctx.arc(tx + 18, yy + 5, 4, 0, Math.PI * 2); ctx.fill()
    ctx.fillStyle = '#cbd5e1'; ctx.font = '11px sans-serif'
    ctx.fillText(`${task.title.slice(0, 16)} ${task.progress}%`, tx + 28, yy)
  })
  ctx.restore()
}

// ── Draw desk ────────────────────────────────────────
function drawDesk(ctx: CanvasRenderingContext2D, x: number, y: number, w: number, isPM: boolean) {
  ctx.save()
  // Desk surface
  ctx.fillStyle = isPM ? '#3a2a20' : '#2a2a40'
  roundRect(ctx, x - w / 2, y, w, 16, 4); ctx.fill()
  // Monitor
  ctx.fillStyle = '#1a1a30'
  const mw = w * 0.4, mh = w * 0.3
  ctx.fillRect(x - mw / 2, y - mh - 8, mw, mh)
  ctx.strokeStyle = '#3a3a5a'; ctx.lineWidth = 1
  ctx.strokeRect(x - mw / 2, y - mh - 8, mw, mh)
  // Monitor glow
  ctx.fillStyle = isPM ? 'rgba(239,68,68,0.15)' : 'rgba(59,130,246,0.15)'
  ctx.fillRect(x - mw / 2 + 2, y - mh - 6, mw - 4, mh - 4)
  // Monitor stand
  ctx.fillStyle = '#2a2a40'
  ctx.fillRect(x - 3, y - 8, 6, 8)
  ctx.restore()
}

// ── Draw crayfish character ──────────────────────────
function drawCrayfish(ctx: CanvasRenderingContext2D, c: CrawChar, t: number, hover: boolean, depth: number = 1) {
  ctx.save()
  const bob = Math.sin(c.walkPhase) * (c.status === 'walking' ? 4 : 1.5)
  const lean = c.status === 'walking' ? Math.sin(c.walkPhase * 2) * 3 : 0
  const cx = c.x, cy = c.y + bob
  const s = (c.role === 'pm' ? 1.3 : 1.0) * depth

  // Shadow
  ctx.fillStyle = 'rgba(0,0,0,0.3)'
  ctx.beginPath()
  ctx.ellipse(cx, cy + 28 * s, 18 * s, 6 * s, 0, 0, Math.PI * 2)
  ctx.fill()

  // Body
  ctx.translate(cx, cy)
  ctx.rotate(lean * Math.PI / 180)

  // Tail
  ctx.fillStyle = darken(c.color, 30)
  ctx.beginPath()
  ctx.ellipse(0, 18 * s, 8 * s, 12 * s, 0, 0, Math.PI * 2)
  ctx.fill()

  // Main body
  const bodyGrad = ctx.createRadialGradient(-3 * s, -3 * s, 0, 0, 0, 20 * s)
  bodyGrad.addColorStop(0, lighten(c.color, 40))
  bodyGrad.addColorStop(1, c.color)
  ctx.fillStyle = bodyGrad
  ctx.beginPath()
  ctx.ellipse(0, 0, 14 * s, 18 * s, 0, 0, Math.PI * 2)
  ctx.fill()
  ctx.strokeStyle = darken(c.color, 40)
  ctx.lineWidth = 1.5
  ctx.stroke()

  // Head
  ctx.fillStyle = lighten(c.color, 20)
  ctx.beginPath()
  ctx.ellipse(0, -16 * s, 11 * s, 10 * s, 0, 0, Math.PI * 2)
  ctx.fill()
  ctx.stroke()

  // Eyes
  ctx.fillStyle = '#fff'
  ctx.beginPath(); ctx.ellipse(-4 * s, -18 * s, 3.5 * s, 4 * s, 0, 0, Math.PI * 2); ctx.fill()
  ctx.beginPath(); ctx.ellipse(4 * s, -18 * s, 3.5 * s, 4 * s, 0, 0, Math.PI * 2); ctx.fill()
  ctx.fillStyle = '#111'
  ctx.beginPath(); ctx.arc(-4 * s, -17.5 * s, 1.8 * s, 0, Math.PI * 2); ctx.fill()
  ctx.beginPath(); ctx.arc(4 * s, -17.5 * s, 1.8 * s, 0, Math.PI * 2); ctx.fill()
  // Eye highlight
  ctx.fillStyle = '#fff'
  ctx.beginPath(); ctx.arc(-3 * s, -19 * s, 0.8 * s, 0, Math.PI * 2); ctx.fill()
  ctx.beginPath(); ctx.arc(5 * s, -19 * s, 0.8 * s, 0, Math.PI * 2); ctx.fill()

  // Claws (antennae)
  const clawWave = Math.sin(t * 2 + c.walkPhase) * 8
  ctx.strokeStyle = c.color; ctx.lineWidth = 2.5 * s; ctx.lineCap = 'round'
  ctx.beginPath()
  ctx.moveTo(-8 * s, -22 * s)
  ctx.quadraticCurveTo(-22 * s, -35 * s + clawWave, -18 * s, -42 * s + clawWave)
  ctx.stroke()
  ctx.beginPath()
  ctx.moveTo(8 * s, -22 * s)
  ctx.quadraticCurveTo(22 * s, -35 * s - clawWave, 18 * s, -42 * s - clawWave)
  ctx.stroke()

  // Pincer tips
  ctx.fillStyle = lighten(c.color, 30)
  ctx.beginPath(); ctx.arc(-18 * s, -42 * s + clawWave, 4 * s, 0, Math.PI * 2); ctx.fill()
  ctx.beginPath(); ctx.arc(18 * s, -42 * s - clawWave, 4 * s, 0, Math.PI * 2); ctx.fill()

  // Legs (walking animation)
  ctx.strokeStyle = darken(c.color, 20); ctx.lineWidth = 2 * s
  for (let i = 0; i < 3; i++) {
    const legPhase = c.walkPhase + i * 0.8
    const legX = (i - 1) * 8 * s
    const legLen = c.status === 'walking' ? Math.sin(legPhase) * 5 : 0
    ctx.beginPath()
    ctx.moveTo(legX - 10 * s, 6 * s + i * 4 * s)
    ctx.lineTo(legX - 18 * s - legLen, 14 * s + i * 4 * s)
    ctx.stroke()
    ctx.beginPath()
    ctx.moveTo(legX + 10 * s, 6 * s + i * 4 * s)
    ctx.lineTo(legX + 18 * s + legLen, 14 * s + i * 4 * s)
    ctx.stroke()
  }

  // PM hat / crown
  if (c.role === 'pm') {
    ctx.fillStyle = '#fbbf24'
    ctx.beginPath()
    ctx.moveTo(-8 * s, -25 * s)
    ctx.lineTo(-10 * s, -35 * s)
    ctx.lineTo(-4 * s, -30 * s)
    ctx.lineTo(0, -38 * s)
    ctx.lineTo(4 * s, -30 * s)
    ctx.lineTo(10 * s, -35 * s)
    ctx.lineTo(8 * s, -25 * s)
    ctx.closePath()
    ctx.fill()
    ctx.strokeStyle = '#d97706'; ctx.lineWidth = 1; ctx.stroke()
  }

  // Working indicator
  if (c.status === 'working') {
    ctx.fillStyle = '#3b82f6'
    ctx.font = `${10 * s}px sans-serif`; ctx.textAlign = 'center'
    const dots = '.'.repeat(Math.floor(t * 3) % 4)
    ctx.fillText(`⚡${dots}`, 0, 32 * s)
  }

  ctx.setTransform(1, 0, 0, 1, 0, 0)

  // Hover glow
  if (hover) {
    ctx.save()
    ctx.shadowColor = c.color; ctx.shadowBlur = 20
    ctx.strokeStyle = c.color + '60'; ctx.lineWidth = 2
    ctx.beginPath(); ctx.arc(c.x, c.y, 32 * s, 0, Math.PI * 2); ctx.stroke()
    ctx.restore()
  }

  // Progress ring
  if (c.progress > 0 && c.progress < 100) {
    ctx.save()
    ctx.beginPath()
    ctx.arc(c.x, c.y, 30 * s, -Math.PI / 2, -Math.PI / 2 + (c.progress / 100) * Math.PI * 2)
    ctx.strokeStyle = '#22c55e'; ctx.lineWidth = 3; ctx.lineCap = 'round'
    ctx.shadowColor = '#22c55e'; ctx.shadowBlur = 8
    ctx.stroke()
    ctx.restore()
    // Progress text
    ctx.fillStyle = '#22c55e'; ctx.font = `bold ${10 * s}px monospace`; ctx.textAlign = 'center'
    ctx.fillText(`${c.progress}%`, c.x, c.y + 42 * s)
  }

  // Name tag
  ctx.fillStyle = '#fff'; ctx.font = `bold ${11 * s}px sans-serif`; ctx.textAlign = 'center'
  ctx.fillText(c.name.length > 6 ? c.name.slice(0, 6) + '…' : c.name, c.x, c.y + (c.progress > 0 ? 54 : 42) * s)

  // Speech bubble
  if (c.bubble && c.bubbleTimer > 0) {
    const alpha = Math.min(1, c.bubbleTimer / 30)
    ctx.save()
    ctx.globalAlpha = alpha
    const bx = c.x + 25 * s, by = c.y - 55 * s
    const bw = Math.min(ctx.measureText(c.bubble).width + 16, 160)
    ctx.fillStyle = 'rgba(255,255,255,0.95)'
    roundRect(ctx, bx, by - 12, bw, 24, 8); ctx.fill()
    // Triangle
    ctx.beginPath()
    ctx.moveTo(bx + 5, by + 12)
    ctx.lineTo(bx - 5, by + 20)
    ctx.lineTo(bx + 15, by + 12)
    ctx.fillStyle = 'rgba(255,255,255,0.95)'; ctx.fill()
    // Text
    ctx.fillStyle = '#1a1a2e'; ctx.font = '11px sans-serif'; ctx.textAlign = 'left'
    ctx.fillText(c.bubble, bx + 8, by + 3)
    ctx.restore()

    if (c.status !== 'talking') c.bubbleTimer -= 0.3
  }

  ctx.restore()
}

// ── Stat badge ───────────────────────────────────────
function SB({ color, label, value, pulse }: { color: string; label: string; value: number; pulse?: boolean }) {
  return (
    <div className="flex items-center gap-1.5">
      <span className={`w-2 h-2 rounded-full ${pulse && value > 0 ? 'animate-pulse' : ''}`} style={{ backgroundColor: color }} />
      <span className="text-gray-400">{label}</span>
      <span className="text-white font-bold">{value}</span>
    </div>
  )
}

// ── Canvas helpers ───────────────────────────────────
function roundRect(ctx: CanvasRenderingContext2D, x: number, y: number, w: number, h: number, r: number) {
  ctx.beginPath()
  ctx.moveTo(x + r, y); ctx.arcTo(x + w, y, x + w, y + h, r)
  ctx.arcTo(x + w, y + h, x, y + h, r); ctx.arcTo(x, y + h, x, y, r)
  ctx.arcTo(x, y, x + w, y, r); ctx.closePath()
}
function lighten(hex: string, amt: number): string {
  const n = parseInt(hex.replace('#', ''), 16)
  const r = Math.min(255, (n >> 16) + amt), g = Math.min(255, ((n >> 8) & 0xff) + amt), b = Math.min(255, (n & 0xff) + amt)
  return `#${(r << 16 | g << 8 | b).toString(16).padStart(6, '0')}`
}
function darken(hex: string, amt: number): string {
  const n = parseInt(hex.replace('#', ''), 16)
  const r = Math.max(0, (n >> 16) - amt), g = Math.max(0, ((n >> 8) & 0xff) - amt), b = Math.max(0, (n & 0xff) - amt)
  return `#${(r << 16 | g << 8 | b).toString(16).padStart(6, '0')}`
}
