import { useState, useCallback, useRef, useEffect, useMemo } from 'react'
import {
  ReactFlow,
  Controls,
  Background,
  MiniMap,
  addEdge,
  useNodesState,
  useEdgesState,
  type Connection,
  type Edge,
  type Node,
  type NodeMouseHandler,
  type ReactFlowInstance,
  BackgroundVariant,
  Panel,
} from '@xyflow/react'
import '@xyflow/react/dist/style.css'
import { Save, Plus, Loader2, Maximize2, Minimize2, Cpu, Wrench, GitBranch, Image, Trash2, ArrowLeft, PanelLeftClose, PanelLeftOpen, Users, Film, Package, FileText, ChevronDown, ChevronRight, Clapperboard, Sparkles, Layers, Camera, Coins, CheckCircle2, XCircle, AlertTriangle, Info, X, ClipboardCheck } from 'lucide-react'
import { useSearchParams, useNavigate } from 'react-router-dom'
import LLMNode from '../components/workflow/LLMNode'
import ToolNode from '../components/workflow/ToolNode'
import ConditionNode from '../components/workflow/ConditionNode'
import StartNode from '../components/workflow/StartNode'
import EndNode from '../components/workflow/EndNode'
import MediaNode from '../components/workflow/MediaNode'
import SceneStepNode from '../components/workflow/SceneStepNode'
import NodePropertyPanel from '../components/workflow/NodePropertyPanel'
import EpisodeWorkflowPanel, { EpisodeLogsPane } from '../components/workflow/EpisodeWorkflowPanel'
import PreflightModal from '../components/workflow/PreflightModal'
import CostAnalysisModal from '../components/workflow/CostAnalysisModal'
import PropEditorModal, { type PropData } from '../components/workflow/PropEditorModal'
import SnapshotsModal from '../components/workflow/SnapshotsModal'
import type { WorkflowSnapshot } from '../components/workflow/snapshots'
import UniverseOverviewModal from '../components/workflow/UniverseOverviewModal'
import AssetCoverageModal from '../components/workflow/AssetCoverageModal'
import CharacterCreatorModal from '../components/workflow/CharacterCreatorModal'
import EpisodeCreatorModal from '../components/workflow/EpisodeCreatorModal'
import ScriptImporterModal from '../components/workflow/ScriptImporterModal'
import { SEASONS, SPINOFF_GROUPS, type EpisodeData, type CharacterData, type Take } from '../components/workflow/episodeTypes'
import { buildSeedNodes, loadSwarmManifest } from '../components/workflow/swarmUniverseSeed'
import { modelAPI, toolAPI, workflowAPI, videoAPI } from '../lib/api'
import { parseTOSFreshness, refreshTOS } from '../components/workflow/tosUrlUtils'

const nodeTypes = {
  llm: LLMNode,
  tool: ToolNode,
  condition: ConditionNode,
  start: StartNode,
  end: EndNode,
  media: MediaNode,
  sceneStep: SceneStepNode,
}

const initialNodes: Node[] = [
  { id: 'start-1', type: 'start', position: { x: 250, y: 50 }, data: { label: '开始' } },
  { id: 'end-1', type: 'end', position: { x: 250, y: 400 }, data: { label: '结束' } },
]

const initialEdges: Edge[] = []

const NODE_PALETTE = [
  { type: 'llm', label: 'LLM 节点', icon: Cpu, color: 'text-blue-400', desc: '大语言模型' },
  { type: 'tool', label: '工具节点', icon: Wrench, color: 'text-amber-400', desc: '调用工具' },
  { type: 'condition', label: '条件分支', icon: GitBranch, color: 'text-green-400', desc: '条件路由' },
  { type: 'media', label: '素材节点', icon: Image, color: 'text-violet-400', desc: '图片/视频/角色', defaultData: { category: 'character' } },
]

export default function WorkflowPage() {
  const [searchParams] = useSearchParams()
  const navigate = useNavigate()
  const workflowId = searchParams.get('id')

  const [nodes, setNodes, onNodesChange] = useNodesState(initialNodes)
  const [edges, setEdges, onEdgesChange] = useEdgesState(initialEdges)
  const [selectedNode, setSelectedNode] = useState<Node | null>(null)
  // selectedNode 是在点击时拍的一张快照，不会随 setNodes 自动更新。
  // 派生一个「实时版本」供右侧属性面板/日志面板读取，这样 runEpisodeProduction
  // 新写入的 take 才能即时出现在 UI 上（否则面板一直停留在点击时的 0 takes 状态）。
  const selectedNodeLive = useMemo(
    () => (selectedNode ? nodes.find(n => n.id === selectedNode.id) || selectedNode : null),
    [selectedNode, nodes],
  )
  const [workflowName, setWorkflowName] = useState('未命名工作流')
  const [workflowCategory, setWorkflowCategory] = useState<string>('')
  const [saving, setSaving] = useState(false)
  // 保存反馈（toast），3s 后自动消失
  const [saveToast, setSaveToast] = useState<{ kind: 'ok' | 'err' | 'info'; msg: string } | null>(null)
  // 富通知 toast（浮动于画布右下角，支持多行 / 图标 / 关闭按钮，替代原生 alert）
  const [appToast, setAppToast] = useState<{ kind: 'ok' | 'err' | 'warn' | 'info'; title: string; body?: string; ctaLabel?: string; onCta?: () => void } | null>(null)
  const appToastTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  // 自动保存：每次 nodes/edges 变化后 debounce 3s 触发（仅当 workflowId 存在）
  const autoSaveTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const lastSavedDefRef = useRef<string>('')
  // Fix #1: flush save ref —— runEpisodeProduction 在 take 状态变化后立刻调一下，
  // 把 3s debounce 收到 200ms 内落盘。否则 Seedance 秒失败 → 用户秒刷新 → takes 消失。
  // 用 ref 打破 doSave 定义顺序（doSave 在组件后半段 useCallback，此处用 ref 绕开）
  const doSaveRef = useRef<(() => Promise<void>) | null>(null)
  const [isFullscreen, setIsFullscreen] = useState(false)
  const [showPalette, setShowPalette] = useState(false)
  const [contextMenu, setContextMenu] = useState<{ x: number; y: number } | null>(null)
  // 节点右键管理菜单（删除/复制/重命名）
  const [nodeContextMenu, setNodeContextMenu] = useState<{ x: number; y: number; node: Node } | null>(null)
  // 连线右键菜单（删除）
  const [edgeContextMenu, setEdgeContextMenu] = useState<{ x: number; y: number; edge: Edge } | null>(null)
  // 撤销栈：最近 20 次破坏性操作（删节点/删连线/删场景），Ctrl+Z 恢复
  type UndoSnap = {
    kind: 'delete-node' | 'delete-edge' | 'delete-scene'
    nodes: Node[]
    edges: Edge[]
    label: string
  }
  const [undoStack, setUndoStack] = useState<UndoSnap[]>([])
  const [models, setModels] = useState<{ id: string; provider: string; model_name: string; display_name: string }[]>([])
  const [availableTools, setAvailableTools] = useState<string[]>([])
  const [showLeftPanel, setShowLeftPanel] = useState(true)
  // 画布底部日志 dock：选中剧集节点时可展开，支持拖拽调高
  const [logsDockOpen, setLogsDockOpen] = useState(false)
  const [logsDockHeight, setLogsDockHeight] = useState(288) // default h-72 = 288px
  const logsDragRef = useRef<{ startY: number; startH: number } | null>(null)
  // 成本分析 Modal
  const [showCostModal, setShowCostModal] = useState(false)
  const [expandedSections, setExpandedSections] = useState<Record<string, boolean>>({ characters: true, episodes: true, props: true, pipeline: false, spinoff: true, 's1': true, 's2': false, 's3': false, 's4': false, 's5': false })
  const [showCharModal, setShowCharModal] = useState(false)
  const [editCharNodeId, setEditCharNodeId] = useState<string | null>(null)
  // 道具工坊·编辑模式
  const [editPropNodeId, setEditPropNodeId] = useState<string | null>(null)
  const [showEpModal, setShowEpModal] = useState(false)
  const [epModalSeason, setEpModalSeason] = useState<number>(1)
  const [epModalSpinoffGroup, setEpModalSpinoffGroup] = useState<string | undefined>(undefined)
  // 广告剧本导入 modal
  const [showImportModal, setShowImportModal] = useState(false)
  const [importTargetType, setImportTargetType] = useState<{ id: string; label: string } | null>(null)
  const containerRef = useRef<HTMLDivElement>(null)
  const nodeIdCounter = useRef(100)
  const rfRef = useRef<ReactFlowInstance | null>(null)

  // 广告宣传片工作流判定：画布含 category=type 的 media 节点（优先于短剧判定）
  const isAdWorkflow = useMemo(() => {
    return nodes.some(n => n.type === 'media' && (n.data as Record<string, unknown>).category === 'type')
  }, [nodes])

  // 短剧工作流判定：category === 'content' 或画布有 scene 节点但不是广告工作流
  const isDramaWorkflow = useMemo(() => {
    if (isAdWorkflow) return false
    if (workflowCategory === 'content') return true
    return nodes.some(n => n.type === 'media' && (n.data as Record<string, unknown>).category === 'scene')
  }, [workflowCategory, nodes, isAdWorkflow])

  // 聚焦模式：选中某一集时隐藏其他剧集节点，突出当前集的工作流
  const [focusedEpisodeId, setFocusedEpisodeId] = useState<string | null>(null)
  const [focusedSceneId, setFocusedSceneId] = useState<string | null>(null)
  const [focusMode, setFocusMode] = useState(true)
  // 点击 Final Cut 节点时让右侧面板默认到 composition tab（一次性信号，面板消费后清空）
  const [panelInitialTab, setPanelInitialTab] = useState<'scenes' | 'composition' | 'script' | 'meta' | null>(null)

  // 派单前自检（点「开始生产 EPxx」先开 modal，自检通过才真正 runEpisodeProduction）
  const [preflightTarget, setPreflightTarget] = useState<{ episode: EpisodeData; nodeId: string } | null>(null)

  // 场景子图节点位置的本地存储覆盖（key: __scene__<epId>__<sceneId> → {x,y}）
  const SCENE_POS_KEY = 'wf-scene-positions'
  const [scenePositions, setScenePositions] = useState<Record<string, { x: number; y: number }>>(() => {
    try { return JSON.parse(localStorage.getItem(SCENE_POS_KEY) || '{}') } catch { return {} }
  })
  useEffect(() => {
    try { localStorage.setItem(SCENE_POS_KEY, JSON.stringify(scenePositions)) } catch { /* ignore */ }
  }, [scenePositions])

  // 聚焦到某一集：中央画布缩放到该节点 + 打开右侧工作流面板
  const focusEpisode = useCallback((n: Node) => {
    setSelectedNode(n)
    setFocusedEpisodeId(n.id)
    setFocusedSceneId(null)
    // 下一帧再 fitView，让 hidden 生效后再缩放
    setTimeout(() => {
      try {
        rfRef.current?.fitView({ nodes: [{ id: n.id }], duration: 500, padding: 0.4, maxZoom: 1.4 })
      } catch { /* ignore */ }
    }, 80)
  }, [])

  // 广告工作流：把导入的剧本（EpisodeData）落地为画布节点 + 类型连线 + 自动聚焦
  const addImportedAdScript = useCallback((typeNodeId: string, data: EpisodeData) => {
    const newId = `script-${typeNodeId}-${Date.now()}`
    setNodes(nds => {
      const typeNode = nds.find(n => n.id === typeNodeId)
      if (!typeNode) return nds
      const existingCount = nds.filter(n => n.type === 'media'
        && (n.data as Record<string, unknown>).category === 'scene'
        && (n.data as Record<string, unknown>).ad_type === typeNodeId).length
      const baseY = 1500
      const newNode: Node = {
        id: newId,
        type: 'media',
        position: { x: typeNode.position.x, y: baseY + existingCount * 220 },
        data: { ...data, ad_type: typeNodeId } as unknown as Record<string, unknown>,
      }
      return [...nds, newNode]
    })
    setEdges(eds => eds.concat([{
      id: `edge-${typeNodeId}-${newId}`,
      source: typeNodeId,
      target: newId,
      animated: true,
      style: { stroke: '#6366f1', strokeWidth: 2 },
    } as unknown as Edge]))
    // 立即聚焦
    setTimeout(() => {
      setNodes(curr => {
        const node = curr.find(n => n.id === newId)
        if (node) {
          setSelectedNode(node)
          setFocusedEpisodeId(newId)
          setFocusedSceneId(null)
          setTimeout(() => {
            try { rfRef.current?.fitView({ nodes: [{ id: newId }], duration: 500, padding: 0.4, maxZoom: 1.4 }) } catch { /* ignore */ }
          }, 80)
        }
        return curr
      })
    }, 0)
  }, [setNodes, setEdges])

  // runEpisodeProduction 在下面定义，用 ref 打破循环依赖
  const runEpisodeProductionRef = useRef<(ep: EpisodeData, id: string, opts?: { initialRefVideoUrl?: string }) => void>(() => {})
  // Cancel production: set to true to abort the scene loop + polling
  const cancelProductionRef = useRef(false)

  // Fix #2: archive backfill —— 从后端 /v1/videos 捞回这一集每个场景已经归档的 takes，
  // 合并进 scene.takes[]。解决「派单过的 take 只在 workflow.definition 里，workflow 没保存就丢」
  // 的问题：workflow 没保存也没关系 —— DB 里的 VideoRecord 永远是 source of truth。
  // 只在「聚焦一集」时触发一次，避免把所有集都拉一遍打爆 API。
  const backfilledEpisodesRef = useRef<Set<string>>(new Set())
  useEffect(() => {
    if (!focusedEpisodeId) return
    // 一集只回灌一次 —— 避免 takes 变化 → 触发 useEffect → 再次 fetch → 无限循环。
    if (backfilledEpisodesRef.current.has(focusedEpisodeId)) return
    const epNode = nodes.find(n => n.id === focusedEpisodeId)
    if (!epNode) return
    const ep = epNode.data as unknown as EpisodeData
    if (!ep?.scenes?.length) return
    const epLabel = ep.label
    if (!epLabel) return
    backfilledEpisodesRef.current.add(focusedEpisodeId)
    void (async () => {
      try {
        // 没有 episode 维度的 filter，只能 scene 过滤后客户端按 label 前缀筛
        // 后端 videoAPI.list 可选 `scene` query param（精确匹配），为了减少请求数，
        // 一次拉 100 条不带 scene 再按「<label>.<Sx>」前缀过滤更简洁。
        const res = await videoAPI.list()
        const records: Array<{
          id: string
          task_id: string
          model: string
          prompt: string
          video_url: string
          img_url: string
          scene: string
          status: string
          duration: number
          created_at: string
        }> = res.data?.videos || []
        const prefix = `${epLabel}.`
        const byScene = new Map<string, typeof records>()
        for (const r of records) {
          if (!r.scene?.startsWith(prefix)) continue
          const sceneId = r.scene.slice(prefix.length)
          if (!sceneId) continue
          if (!byScene.has(sceneId)) byScene.set(sceneId, [])
          byScene.get(sceneId)!.push(r)
        }
        let touched = 0
        setNodes(nds => nds.map(n => {
          if (n.id !== focusedEpisodeId) return n
          const d = n.data as unknown as EpisodeData
          // Stale take cleanup: mark running/pending takes older than 10 min as failed
          const STALE_MS = 10 * 60 * 1000
          const now = Date.now()
          const cleanStaleTakes = (takes: Take[]) => takes.map(t => {
            if ((t.status === 'running' || t.status === 'pending') && t.created_at) {
              const age = now - new Date(t.created_at).getTime()
              if (age > STALE_MS) {
                return { ...t, status: 'failed' as const, note: t.note || '超时未完成（自动清理）' }
              }
            }
            return t
          })
          // Deduplicate take_ids: if two takes share the same id, rename the later one
          const deduplicateTakes = (takes: Take[]) => {
            const seen = new Set<string>()
            let maxNum = takes.reduce((max, t) => {
              const m = /^t(\d+)$/.exec(t.take_id || '')
              return m ? Math.max(max, Number(m[1])) : max
            }, 0)
            return takes.map(t => {
              if (seen.has(t.take_id)) {
                maxNum += 1
                return { ...t, take_id: `t${maxNum}` }
              }
              seen.add(t.take_id)
              return t
            })
          }
          const newScenes = (d.scenes || []).map(s => {
            const recs = byScene.get(s.id)
            if (!recs?.length) {
              // No backfill records, but still clean stale takes + deduplicate
              const cleaned = deduplicateTakes(cleanStaleTakes(s.takes || []))
              if (cleaned.every((t, i) => t === (s.takes || [])[i])) return s
              return { ...s, takes: cleaned }
            }
            const existingTaskIds = new Set((s.takes || []).map(t => t.task_id).filter(Boolean))
            const deletedTaskIds = new Set(s.deleted_task_ids || [])
            const maxTakeNum = (s.takes || []).reduce((max, t) => {
              const m = /^t(\d+)$/.exec(t.take_id || '')
              return m ? Math.max(max, Number(m[1])) : max
            }, 0)
            // Fix: 先同步已有 take 的状态（task_id 匹配的，用后端真实状态覆盖）
            const recByTaskId = new Map(recs.map(r => [r.task_id, r]))
            let updatedExisting = (s.takes || []).map(t => {
              if (!t.task_id) return t
              const r = recByTaskId.get(t.task_id)
              if (!r) return t
              const backendStatus = r.status === 'succeeded' ? 'succeeded' as const
                : r.status === 'failed' ? 'failed' as const
                : t.status
              if (backendStatus === t.status && (t.video_url || !r.video_url)) return t
              touched++
              return {
                ...t,
                status: backendStatus,
                video_url: r.video_url || t.video_url,
                finished_at: t.finished_at || (backendStatus === 'succeeded' || backendStatus === 'failed' ? new Date().toISOString() : undefined),
                note: t.note || '从视频库回灌',
              }
            })
            const toAdd: Take[] = []
            // 按 created_at 升序补 take_id，保证 t1/t2/... 不打架
            const sortedRecs = [...recs].sort((a, b) => (a.created_at || '').localeCompare(b.created_at || ''))
            let nextTakeNum = maxTakeNum
            for (const r of sortedRecs) {
              if (existingTaskIds.has(r.task_id) || deletedTaskIds.has(r.task_id)) continue
              nextTakeNum += 1
              toAdd.push({
                take_id: `t${nextTakeNum}`,
                status: r.status === 'succeeded' ? 'succeeded'
                  : r.status === 'failed' ? 'failed'
                  : r.status === 'running' ? 'running'
                  : 'pending',
                video_url: r.video_url,
                task_id: r.task_id,
                created_at: r.created_at,
                finished_at: r.status === 'succeeded' || r.status === 'failed' ? r.created_at : undefined,
                prompt: r.prompt,
                ref_image_url: r.img_url,
                model: r.model,
                duration: r.duration,
                note: '从视频库回灌',
              } as Take)
            }
            if (toAdd.length === 0 && updatedExisting.every((t, i) => t === (s.takes || [])[i])) {
              // 只做 stale/dedup 清理
              const cleaned = deduplicateTakes(cleanStaleTakes(s.takes || []))
              if (cleaned.every((t, i) => t === (s.takes || [])[i])) return s
              return { ...s, takes: cleaned }
            }
            touched += toAdd.length
            const mergedTakes = [...updatedExisting, ...toAdd]
            // 如果本场还没 picked_take，且有 succeeded 的，自动 pick 最新一个
            let picked = s.picked_take
            if (!picked) {
              const latestSucc = [...mergedTakes].reverse().find(t => t.status === 'succeeded')
              if (latestSucc) picked = latestSucc.take_id
            }
            return { ...s, takes: deduplicateTakes(cleanStaleTakes(mergedTakes)), picked_take: picked }
          })
          const picked_clips = newScenes.map(s => s.picked_take ? `${s.id}.${s.picked_take}` : '').filter(Boolean)
          const composition = { ...(d.composition || { picked_clips: [], status: 'pending' as const }), picked_clips }
          return { ...n, data: { ...d, scenes: newScenes, composition } as unknown as Record<string, unknown> }
        }))
        if (touched > 0) {
          console.log(`[archive-backfill] ${epLabel}: 回灌 ${touched} 个 takes（来自 /v1/videos）`)
        }
      } catch (e) {
        const ax = e as { response?: { data?: { error?: string } }; message?: string }
        console.warn(`[archive-backfill] ${epLabel}: 拉 /v1/videos 失败`, ax?.response?.data?.error || ax?.message || e)
      }
    })()
  }, [focusedEpisodeId, nodes, setNodes])

  // 重拍单场景：构造只含该场景的 EpisodeData 投递给同一工作流
  // 关键：从前一场景的 picked_take 抽取 video_url 作为 initialRefVideoUrl 保证尾帧链式不断
  const rerunScene = useCallback((epId: string, sceneId: string) => {
    setNodes(nds => {
      const epNode = nds.find(n => n.id === epId)
      if (!epNode) return nds
      const ep = epNode.data as unknown as EpisodeData
      const allScenes = ep.scenes || []
      const sceneIdx = allScenes.findIndex(s => s.id === sceneId)
      if (sceneIdx < 0) return nds
      const scene = allScenes[sceneIdx]
      const prev = sceneIdx > 0 ? allScenes[sceneIdx - 1] : undefined
      const prevPicked = prev ? prev.takes.find(t => t.take_id === prev.picked_take) : undefined
      const initialRefVideoUrl = prevPicked?.video_url || ''

      // 立即在 UI 上添加一个 running take 作为视觉反馈
      // Fix: 先把该场景里所有残留的 running/pending take 标为 cancelled（容器重启后孤儿 take）
      const cleanedTakes = (scene.takes || []).map(t =>
        (t.status === 'running' || t.status === 'pending')
          ? { ...t, status: 'cancelled' as const, note: t.note || '被新一次重拍取代' }
          : t
      )
      const takeNum = cleanedTakes.reduce((max, t) => {
        const m = /^t(\d+)$/.exec(t.take_id || '')
        return m ? Math.max(max, Number(m[1])) : max
      }, 0) + 1
      const newTakeId = `t${takeNum}`
      const updatedScenes = allScenes.map(s => s.id !== sceneId ? s : {
        ...s,
        takes: [...cleanedTakes, {
          take_id: newTakeId,
          status: 'running' as const,
          created_at: new Date().toISOString(),
          model: 'doubao-seedance-2-0-260128',
          duration: scene.duration || 5,
        }],
      })

      // 异步触发生产（scoped 只含该场景 + 传上一场尾帧）
      // NOTE: runEpisodeProduction 内部会检测已存在的 running take 并更新而非重复创建
      setTimeout(() => {
        const scopedEp: EpisodeData = { ...ep, scenes: [scene] }
        void runEpisodeProductionRef.current(scopedEp, epId, { initialRefVideoUrl })
      }, 0)
      return nds.map(n => n.id === epId
        ? { ...n, data: { ...ep, scenes: updatedScenes } as unknown as Record<string, unknown> }
        : n)
    })
  }, [setNodes])

  // 切换 picked take（由 sceneStep takes 缩略图墙触发）
  const pickSceneTake = useCallback((epId: string, sceneId: string, takeId: string) => {
    setNodes(nds => nds.map(n => {
      if (n.id !== epId) return n
      const d = n.data as unknown as EpisodeData
      const newScenes = (d.scenes || []).map(s => s.id === sceneId ? { ...s, picked_take: takeId } : s)
      const picked_clips = newScenes.map(s => s.picked_take ? `${s.id}.${s.picked_take}` : '').filter(Boolean)
      const composition = { ...(d.composition || { picked_clips: [] }), picked_clips }
      return { ...n, data: { ...d, scenes: newScenes, composition } as unknown as Record<string, unknown> }
    }))
  }, [setNodes])

  // 派生画布显示节点：focusMode 开启且选中某一集时
  //   · 隐藏其他剧集节点
  //   · 在该集下方注入场景子图（S1→S2→…→Final）
  const { displayNodes, displayEdges } = useMemo(() => {
    if (!focusMode || !focusedEpisodeId) {
      // 广告工作流：隐藏 type/style media 节点（已移至左侧边栏）
      if (isAdWorkflow) {
        const filtered = nodes.map(n => {
          if (n.type !== 'media') return n
          const cat = (n.data as Record<string, unknown>).category
          if (cat === 'type' || cat === 'style') return { ...n, hidden: true }
          return n
        })
        return { displayNodes: filtered, displayEdges: edges }
      }
      return { displayNodes: nodes, displayEdges: edges }
    }
    const focused = nodes.find(n => n.id === focusedEpisodeId)
    const visibleNodes = nodes.map(n => {
      if (n.type !== 'media') return n
      const cat = (n.data as Record<string, unknown>).category
      if (cat !== 'scene') return n
      return { ...n, hidden: n.id !== focusedEpisodeId }
    })
    // 如果聚焦节点不存在或没有 scenes，不注入子图
    const ep = focused?.data as unknown as EpisodeData | undefined
    if (!focused || !ep?.scenes || ep.scenes.length === 0) {
      return { displayNodes: visibleNodes, displayEdges: edges }
    }

    // 场景节点子图布局：在聚焦集下方一行水平排开
    const SCENE_W = 190
    const SCENE_H_OFFSET = 280    // 聚焦集下方距离
    const n = ep.scenes.length
    const rowWidth = (n + 1) * SCENE_W  // +1 给 final 节点
    const startX = (focused.position?.x ?? 0) - rowWidth / 2 + 100 // 大致居中
    const y = (focused.position?.y ?? 0) + SCENE_H_OFFSET

    // 归档目录约定：docs/<project>/production/<epKey>/clips_v2/<scene>_<take>.mp4
    //   被静态路由 /v1/projects/:project/*filepath 映射为可访问 URL。
    //   当 takes 里的 TOS video_url 过期（24h）但本地 mp4 还在时，SceneStepNode
    //   会用 archiveProject/archiveEpKey 派生出 /v1/projects/... 的播放退路。
    const archiveProject = 'swarm-universe'
    const archiveEpKey = `${ep.is_spinoff ? 'sp' : 'ep'}${String(ep.episode_number || 0).padStart(2, '0')}`

    const sceneNodes: Node[] = ep.scenes.map((s, i) => {
      const pickedTake = s.picked_take ? s.takes?.find(t => t.take_id === s.picked_take) : undefined
      const anyTake = s.takes?.find(t => t.status === 'succeeded')
      const nodeId = `__scene__${focusedEpisodeId}__${s.id}`
      const savedPos = scenePositions[nodeId]
      return {
        id: nodeId,
        type: 'sceneStep',
        position: savedPos || { x: startX + i * SCENE_W, y },
        data: {
          sceneId: s.id,
          label: s.label,
          duration: s.duration,
          hasClip: !!anyTake,
          isPicked: !!pickedTake,
          // 注意：只用 local_url 填 videoUrl；TOS video_url 放到 takes[] 里按需 fallback。
          // 避免顶层 videoUrl 锁定一个过期 URL，导致 SceneStepNode fallback 派生逻辑失效。
          videoUrl: pickedTake?.local_url || anyTake?.local_url,
          archiveProject,
          archiveEpKey,
          thumbnail: (s as unknown as Record<string, string>).thumbnail,
          takes: s.takes || [],
          pickedTakeId: s.picked_take,
          prompt: s.prompt || '',
          onRerun: (sid: string) => rerunScene(focusedEpisodeId, sid),
          onPickTake: (sid: string, tid: string) => pickSceneTake(focusedEpisodeId, sid, tid),
          onUpdatePrompt: (sid: string, prompt: string) => {
            setNodes(nds => nds.map(nd => {
              if (nd.id !== focusedEpisodeId) return nd
              const dd = nd.data as unknown as EpisodeData
              const newScenes = (dd.scenes || []).map(sc => sc.id === sid ? { ...sc, prompt } : sc)
              return { ...nd, data: { ...dd, scenes: newScenes } as unknown as Record<string, unknown> }
            }))
          },
          onSelectScene: (sid: string) => setFocusedSceneId(sid),
        },
        draggable: true,
        selectable: true,
      } as Node
    })

    // 终点合成节点
    const finalNodeId = `__scene__${focusedEpisodeId}__final`
    const finalSaved = scenePositions[finalNodeId]
    const finalNode: Node = {
      id: finalNodeId,
      type: 'sceneStep',
      position: finalSaved || { x: startX + n * SCENE_W, y },
      data: {
        isFinal: true, sceneId: 'FIN', label: '合成成片', duration: 0,
        hasClip: !!ep.composition?.final_video_url,
        isPicked: !!ep.composition?.final_video_url,
        videoUrl: ep.composition?.final_video_url,
        // 点 Final Cut 节点 → 选中本集 + 切到 composition tab。
        // SceneStepNode 在 isFinal 分支里调用 onSelectScene('__FINAL__')。
        onSelectScene: (sid: string) => {
          if (sid === '__FINAL__') {
            const epNode = nodes.find(nn => nn.id === focusedEpisodeId)
            if (epNode) {
              setSelectedNode(epNode)
              setFocusedSceneId(null)
              setPanelInitialTab('composition')
            }
          }
        },
      },
      draggable: true,
      selectable: true,
    }

    // 边：episode → S1 → S2 → ... → Sn → Final
    const baseEdgeStyle = { stroke: '#06b6d4', strokeWidth: 2 }
    const subEdges: Edge[] = []
    subEdges.push({
      id: `__edge__${focusedEpisodeId}__ep-s1`,
      source: focusedEpisodeId,
      target: sceneNodes[0].id,
      animated: true,
      style: baseEdgeStyle,
    })
    for (let i = 0; i < sceneNodes.length - 1; i++) {
      subEdges.push({
        id: `__edge__${focusedEpisodeId}__${ep.scenes[i].id}-${ep.scenes[i + 1].id}`,
        source: sceneNodes[i].id,
        target: sceneNodes[i + 1].id,
        animated: true,
        style: baseEdgeStyle,
      })
    }
    subEdges.push({
      id: `__edge__${focusedEpisodeId}__last-final`,
      source: sceneNodes[sceneNodes.length - 1].id,
      target: finalNode.id,
      animated: true,
      style: { stroke: '#10b981', strokeWidth: 2.5 },
    })

    return {
      displayNodes: [...visibleNodes, ...sceneNodes, finalNode],
      displayEdges: [...edges, ...subEdges],
    }
  }, [nodes, edges, focusMode, focusedEpisodeId, scenePositions, rerunScene, pickSceneTake])

  // localStorage key for 无 workflowId 情况下的草稿画布（按 tab 隔离）
  const DRAFT_KEY = workflowId ? `wf-draft:${workflowId}` : 'wf-draft:__new__'
  // hydrated=true 之后才允许写 localStorage，避免首渲染 nodes=[] 覆盖已有草稿
  const [hydrated, setHydrated] = useState(false)

  useEffect(() => {
    setHydrated(false)
    loadModels()
    loadTools()
    if (workflowId) {
      // 先尝试本地草稿立刻显示（避免后端慢 → 空态覆盖），再后台 load backend
      try {
        const raw = localStorage.getItem(DRAFT_KEY)
        if (raw) {
          const d = JSON.parse(raw) as { nodes?: Node[]; edges?: Edge[]; name?: string; counter?: number }
          if (d.nodes && d.nodes.length) setNodes(d.nodes as Node[])
          if (d.edges) setEdges(d.edges as Edge[])
          if (d.name) setWorkflowName(d.name)
          if (typeof d.counter === 'number') nodeIdCounter.current = d.counter
        }
      } catch { /* ignore */ }
      loadWorkflow(workflowId).finally(() => setHydrated(true))
    } else {
      // 无 workflowId 时从 localStorage 恢复草稿
      try {
        const raw = localStorage.getItem(DRAFT_KEY)
        if (raw) {
          const d = JSON.parse(raw) as { nodes?: Node[]; edges?: Edge[]; name?: string; counter?: number }
          if (d.nodes && d.nodes.length) setNodes(d.nodes as Node[])
          if (d.edges) setEdges(d.edges as Edge[])
          if (d.name) setWorkflowName(d.name)
          if (typeof d.counter === 'number') nodeIdCounter.current = d.counter
        }
      } catch { /* ignore corrupted draft */ }
      setHydrated(true)
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [workflowId])

  // 自动保存草稿到 localStorage（debounced；hydrated 后且非完全空态才写）
  useEffect(() => {
    if (!hydrated) return
    // 护栏：仅 Start+End 两个默认节点 + 0 edges 视为"空"，不覆盖之前的草稿
    const meaningful = nodes.length > 2 || edges.length > 0 ||
      nodes.some(n => n.type !== 'start' && n.type !== 'end')
    if (!meaningful) return
    const t = setTimeout(() => {
      try {
        localStorage.setItem(DRAFT_KEY, JSON.stringify({
          nodes, edges, name: workflowName, counter: nodeIdCounter.current,
          savedAt: Date.now(),
        }))
      } catch { /* quota exceeded, ignore */ }
    }, 500)
    return () => clearTimeout(t)
  }, [nodes, edges, workflowName, DRAFT_KEY, hydrated])

  // ── 一次性 manifest 同步（hydrated 后跑一次） ──
  //
  // 既覆盖 loadWorkflow 路径也覆盖 localStorage 草稿路径。即使 backend def 为空、
  // localStorage 里有 stale 节点，这里也会把 manifest 里有但画布上没的角色/道具
  // 节点补建，并把 ref_video / tos_url / appearance_card 同步到老节点。
  //
  // 触发条件：画布上至少有一个 character 节点（认为这是虫群宇宙工作流），避免
  // 在空画布或非短剧工作流里乱注入角色。
  const manifestSyncedRef = useRef(false)
  useEffect(() => {
    if (!hydrated) return
    if (manifestSyncedRef.current) return
    const hasCharNode = nodes.some(n => {
      const d = n.data as Record<string, unknown> | undefined
      return d?.category === 'character'
    })
    if (!hasCharNode) return
    manifestSyncedRef.current = true
    let cancelled = false
    ;(async () => {
      try {
        const manifest = await loadSwarmManifest(true)
        const charMap = new Map(manifest.characters.map(c => [c.key, c]))
        const propMap = new Map(manifest.props.map(p => [p.key, p]))
        const absUrl = (rel: string | null | undefined): string => {
          if (!rel) return ''
          if (rel.startsWith('http') || rel.startsWith('/v1/')) return rel
          return manifest.url_prefix + rel
        }
        if (cancelled) return
        setNodes(prev => {
          const existingCharKeys = new Set<string>()
          const existingPropKeys = new Set<string>()
          let maxCharX = 0
          let maxPropX = 0
          // Pass 1: patch 已有节点（用 immutable map，避免 React 漏 re-render）
          const patched: Node[] = prev.map(n => {
            if (n.type !== 'media') return n
            const d = n.data as Record<string, unknown>
            if (d.category === 'character' && typeof d.key === 'string') {
              existingCharKeys.add(d.key)
              if (n.position?.x && n.position.x > maxCharX) maxCharX = n.position.x
              const src = charMap.get(d.key)
              if (!src) return n
              const newD: Record<string, unknown> = { ...d }
              if (src.appearance_card) newD.appearance_card = src.appearance_card
              if (src.tos_url) newD.tos_url = src.tos_url
              if (src.ref_video) newD.ref_video = src.ref_video
              return { ...n, data: newD }
            }
            if (d.category === 'prop' && typeof d.key === 'string') {
              existingPropKeys.add(d.key)
              if (n.position?.x && n.position.x > maxPropX) maxPropX = n.position.x
              const src = propMap.get(d.key)
              if (!src) return n
              const newD: Record<string, unknown> = { ...d }
              if (src.tos_url) newD.tos_url = src.tos_url
              if (src.tag) newD.tag = src.tag
              return { ...n, data: newD }
            }
            return n
          })
          // Pass 2: 补建 manifest 里有、画布上没有的节点
          const COL_W = 200
          const CHAR_Y = 40
          const PROPS_Y = 1380   // 与 buildSeedNodes 布局对齐
          const additions: Node[] = []
          let stamp = Date.now()
          for (const c of manifest.characters) {
            if (existingCharKeys.has(c.key)) continue
            maxCharX += COL_W
            additions.push({
              id: `media-sync-${stamp++}`,
              type: 'media',
              position: { x: maxCharX || 80, y: CHAR_Y },
              data: {
                category: 'character',
                label: c.label, tag: c.tag, role: c.role, key: c.key,
                appearance_card: c.appearance_card,
                description: c.description,
                imageUrl: absUrl(c.ref),
                tos_url: c.tos_url,
                ref_video: c.ref_video,
              },
            } as unknown as Node)
            console.log(`[manifestSync] add missing character: ${c.tag} ${c.label} (key=${c.key})`)
          }
          for (const p of manifest.props) {
            if (existingPropKeys.has(p.key)) continue
            maxPropX += COL_W
            additions.push({
              id: `media-sync-${stamp++}`,
              type: 'media',
              position: { x: maxPropX || 80, y: PROPS_Y },
              data: {
                category: 'prop',
                key: p.key,
                label: p.label,
                description: p.description,
                imageUrl: p.ref ? absUrl(p.ref) : undefined,
                tos_url: p.tos_url,
                tag: p.tag,
              },
            } as unknown as Node)
            console.log(`[manifestSync] add missing prop: 「${p.label}」(key=${p.key})`)
          }
          if (additions.length === 0 && patched.every((n, i) => n === prev[i])) {
            return prev   // 完全没变，避免触发重渲
          }
          return [...patched, ...additions]
        })
      } catch (e) {
        console.warn('[manifestSync] failed:', e)
        manifestSyncedRef.current = false   // 让下次 nodes 变化重试
      }
    })()
    return () => { cancelled = true }
  }, [hydrated, nodes, setNodes])

  // Fullscreen API
  useEffect(() => {
    const onFSChange = () => setIsFullscreen(!!document.fullscreenElement)
    document.addEventListener('fullscreenchange', onFSChange)
    return () => document.removeEventListener('fullscreenchange', onFSChange)
  }, [])

  // Keyboard shortcuts
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'F11') { e.preventDefault(); toggleFullscreen() }
      if (e.key === 'Delete' && selectedNode) deleteSelectedNode()
      // Ctrl/Cmd+Z → 撤销最近一次删除
      if ((e.ctrlKey || e.metaKey) && !e.shiftKey && e.key.toLowerCase() === 'z') {
        // 如果焦点在输入框/textarea/contenteditable 中，不拦截原生 undo
        const t = e.target as HTMLElement | null
        const tag = t?.tagName
        if (tag === 'INPUT' || tag === 'TEXTAREA' || t?.isContentEditable) return
        e.preventDefault()
        undoLastDelete()
      }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [selectedNode, undoStack])

  const toggleFullscreen = () => {
    if (!document.fullscreenElement) {
      containerRef.current?.requestFullscreen()
    } else {
      document.exitFullscreen()
    }
  }

  // 快照当前 nodes+edges 到撤销栈（压入前裁成最多 20 条）
  const pushUndo = useCallback((kind: UndoSnap['kind'], label: string) => {
    setUndoStack(stk => {
      const snap: UndoSnap = { kind, label, nodes: [...nodes], edges: [...edges] }
      const next = [...stk, snap]
      if (next.length > 20) next.shift()
      return next
    })
  }, [nodes, edges])

  const undoLastDelete = useCallback(() => {
    setUndoStack(stk => {
      if (stk.length === 0) return stk
      const next = [...stk]
      const snap = next.pop()!
      setNodes(snap.nodes)
      setEdges(snap.edges)
      return next
    })
  }, [setNodes, setEdges])

  const deleteSelectedNode = () => {
    if (!selectedNode) return
    if (selectedNode.type === 'start' || selectedNode.type === 'end') return

    // ── 场景子图节点特殊处理：不是真实 node，而是从 episode.scenes[] 派生 ──
    if (selectedNode.type === 'sceneStep') {
      const d = selectedNode.data as Record<string, unknown>
      if (d.isFinal) return // Final Cut 不允许删
      const sceneId = d.sceneId as string
      if (!focusedEpisodeId || !sceneId) return
      pushUndo('delete-scene', `删除场景 ${sceneId}`)
      setNodes(nds => nds.map(n => {
        if (n.id !== focusedEpisodeId) return n
        const ep = n.data as unknown as EpisodeData
        const newScenes = (ep.scenes || []).filter(s => s.id !== sceneId)
        const picked_clips = newScenes.map(s => s.picked_take ? `${s.id}.${s.picked_take}` : '').filter(Boolean)
        const composition = { ...(ep.composition || { picked_clips: [], status: 'pending' as const }), picked_clips }
        return { ...n, data: { ...ep, scenes: newScenes, composition } as unknown as Record<string, unknown> }
      }))
      setSelectedNode(null)
      setFocusedSceneId(null)
      return
    }

    // 普通节点：push 快照，再删除
    pushUndo('delete-node', `删除 ${selectedNode.type} · ${(selectedNode.data as Record<string, unknown>)?.label || selectedNode.id}`)
    setNodes(nds => nds.filter(n => n.id !== selectedNode.id))
    setEdges(eds => eds.filter(e => e.source !== selectedNode.id && e.target !== selectedNode.id))
    setSelectedNode(null)
  }

  // 删除单条 edge（含 undo）
  const deleteEdge = useCallback((edgeId: string) => {
    pushUndo('delete-edge', `删除连线 ${edgeId}`)
    setEdges(eds => eds.filter(e => e.id !== edgeId))
    setEdgeContextMenu(null)
  }, [pushUndo, setEdges])

  const loadModels = async () => {
    try { const res = await modelAPI.list(); setModels(res.data.models || []) } catch { /* */ }
  }
  const loadTools = async () => {
    try { const res = await toolAPI.list(); setAvailableTools(res.data.tools || []) } catch { /* */ }
  }

  const loadWorkflow = async (id: string) => {
    try {
      const res = await workflowAPI.get(id)
      const wf = res.data.workflow
      if (wf) {
        setWorkflowName(wf.name || '未命名工作流')
        setWorkflowCategory(wf.category || '')
        const def = typeof wf.definition === 'string' ? JSON.parse(wf.definition) : wf.definition
        // 只有当后端 definition 真的有内容时才覆盖本地草稿，避免空态擦掉用户数据
        if (def?.nodes && Array.isArray(def.nodes) && def.nodes.length > 0) {
          // Auto-sync character/prop nodes from manifest (单一真相源)
          try {
            const manifest = await loadSwarmManifest(true)
            const charMap = new Map(manifest.characters.map(c => [c.key, c]))
            const propMap = new Map(manifest.props.map(p => [p.key, p]))
            // 1) 已有 char/prop 节点：原地 patch（appearance_card / tos_url / ref_video）
            const existingCharKeys = new Set<string>()
            const existingPropKeys = new Set<string>()
            let maxCharX = 0
            let maxPropX = 0
            for (const n of def.nodes) {
              if (n.type !== 'media') continue
              const d = n.data as Record<string, unknown>
              if (d.category === 'character' && typeof d.key === 'string') {
                existingCharKeys.add(d.key)
                if (n.position?.x && n.position.x > maxCharX) maxCharX = n.position.x
                const src = charMap.get(d.key)
                if (src) {
                  if (src.appearance_card) d.appearance_card = src.appearance_card
                  if (src.tos_url) d.tos_url = src.tos_url
                  // ref_video 是后加字段，老快照里没有；同步补上才能让 v2v-only
                  // 角色（EP07 三个混混）的 preflight 短路分支正确触发。
                  if (src.ref_video) d.ref_video = src.ref_video
                }
              } else if (d.category === 'prop' && typeof d.key === 'string') {
                existingPropKeys.add(d.key)
                if (n.position?.x && n.position.x > maxPropX) maxPropX = n.position.x
                const src = propMap.get(d.key)
                if (src?.tos_url) d.tos_url = src.tos_url
                if (src?.tag) d.tag = src.tag
              }
            }
            // 2) manifest 里有、画布上没有的角色/道具：直接补建节点（跟 buildSeedNodes 同布局）
            // 修复场景：保存工作流后 manifest 又加了新角色（如 EP07 三个混混），
            // 老 def 不会自动包含，preflight 会报「[图N] 找不到对应角色」。
            const COL_W = 200
            const CHAR_Y = 40
            // PROPS_Y = EP_Y_START(300) + 6 * ROW_H(170) + 60 = 1380（与 buildSeedNodes 对齐）
            const PROPS_Y = 1380
            const absUrl = (rel: string | null | undefined): string => {
              if (!rel) return ''
              if (rel.startsWith('http') || rel.startsWith('/v1/')) return rel
              return manifest.url_prefix + rel
            }
            let nextSyncId = Date.now()  // 防 ID 冲突，用毫秒戳起头
            for (const c of manifest.characters) {
              if (existingCharKeys.has(c.key)) continue
              maxCharX += COL_W
              def.nodes.push({
                id: `media-sync-${nextSyncId++}`,
                type: 'media',
                position: { x: maxCharX || 80, y: CHAR_Y },
                data: {
                  category: 'character',
                  label: c.label,
                  tag: c.tag,
                  role: c.role,
                  key: c.key,
                  appearance_card: c.appearance_card,
                  description: c.description,
                  imageUrl: absUrl(c.ref),
                  tos_url: c.tos_url,
                  ref_video: c.ref_video,
                },
              })
              console.log(`[loadWorkflow] auto-add missing character: ${c.tag} ${c.label} (key=${c.key})`)
            }
            for (const p of manifest.props) {
              if (existingPropKeys.has(p.key)) continue
              maxPropX += COL_W
              def.nodes.push({
                id: `media-sync-${nextSyncId++}`,
                type: 'media',
                position: { x: maxPropX || 80, y: PROPS_Y },
                data: {
                  category: 'prop',
                  key: p.key,
                  label: p.label,
                  description: p.description,
                  imageUrl: p.ref ? absUrl(p.ref) : undefined,
                  tos_url: p.tos_url,
                  tag: p.tag,
                },
              })
              console.log(`[loadWorkflow] auto-add missing prop: 「${p.label}」(key=${p.key})`)
            }
            // Auto-sync scene prompts from manifest (manifest 是 prompt 的单一真相源)
            // Workflow node label 格式 "EP05 夜袭"；manifest 用 title "夜袭" + number 5
            const epByTitle = new Map(manifest.episodes.map(e => {
              const prefix = `EP${String(e.number).padStart(2, '0')} ${e.title}`
              return [prefix, e]
            }))
            for (const n of def.nodes) {
              if (n.type !== 'media') continue
              const d = n.data as Record<string, unknown>
              if (d.category !== 'scene') continue
              const epLabel = d.label as string
              if (!epLabel) continue
              const mEp = epByTitle.get(epLabel)
              if (!mEp?.scenes?.length || !Array.isArray(d.scenes)) continue
              const mSceneMap = new Map(mEp.scenes.map(s => [s.id, s]))
              for (const s of d.scenes as Array<{ id: string; prompt?: string; label?: string; duration?: number }>) {
                const ms = mSceneMap.get(s.id)
                if (!ms) continue
                if (ms.prompt && ms.prompt !== s.prompt) {
                  console.log(`[loadWorkflow] sync prompt ${epLabel}.${s.id}: "${(s.prompt || '').slice(0, 30)}…" → "${ms.prompt.slice(0, 30)}…"`)
                  s.prompt = ms.prompt
                }
                if (ms.label && ms.label !== s.label) s.label = ms.label
                if (ms.duration && ms.duration !== s.duration) s.duration = ms.duration
              }
            }
          } catch { /* manifest unavailable — keep node data as-is */ }
          // Fix: 页面刚加载时没有活跃的生产进程。
          // 只取消没有 task_id 的 running/pending take（从未提交到后端的孤儿）。
          // 有 task_id 的保持 running —— 后续 backfill 会从 /v1/videos 查询真实状态并同步。
          for (const n of def.nodes) {
            if (n.type !== 'media') continue
            const d = n.data as Record<string, unknown>
            if (d.category !== 'scene' || !Array.isArray(d.scenes)) continue
            let dirty = false
            for (const s of d.scenes as Array<{ takes?: Array<{ status: string; note?: string; task_id?: string }> }>) {
              for (const t of (s.takes || [])) {
                if ((t.status === 'running' || t.status === 'pending') && !t.task_id) {
                  t.status = 'cancelled'
                  t.note = t.note || '页面重载时自动取消（未提交的残留）'
                  dirty = true
                }
              }
            }
            if (dirty) console.log(`[loadWorkflow] 清理无 task_id 的孤儿 takes: ${d.label || n.id}`)
          }
          setNodes(def.nodes)
        }
        if (def?.edges && Array.isArray(def.edges) && def.edges.length > 0) setEdges(def.edges)
      }
    } catch { /* */ }
  }

  const onConnect = useCallback(
    (params: Connection) => setEdges(eds => addEdge({ ...params, animated: true, style: { stroke: '#475569', strokeWidth: 1.5 } }, eds)),
    [setEdges],
  )

  const onNodeClick: NodeMouseHandler = useCallback((_e, node) => {
    setContextMenu(null)
    // 点到剧集节点 → 聚焦
    if (node.type === 'media' && (node.data as Record<string, unknown>).category === 'scene') {
      setSelectedNode(node)
      setFocusedEpisodeId(node.id)
      setFocusedSceneId(null)
      return
    }
    // 点到场景子图节点 → 选中其父剧集并打开对应 scene tab
    if (node.type === 'sceneStep') {
      const d = node.data as Record<string, unknown>
      if (d.isFinal) return
      const sceneId = d.sceneId as string
      // 解析父剧集 ID：__scene__<epId>__<sceneId>
      const prefix = `__scene__`
      if (node.id.startsWith(prefix)) {
        const rest = node.id.slice(prefix.length)
        const epId = rest.substring(0, rest.lastIndexOf('__'))
        const epNode = nodes.find(n => n.id === epId)
        if (epNode) {
          setSelectedNode(epNode)
          setFocusedSceneId(sceneId)
        }
      }
      return
    }
    setSelectedNode(node)
  }, [nodes])

  // 场景子图节点拖动后持久化位置
  const onNodeDragStop = useCallback((_e: React.MouseEvent, node: Node) => {
    if (node.type !== 'sceneStep') return
    setScenePositions(prev => ({ ...prev, [node.id]: { x: node.position.x, y: node.position.y } }))
  }, [])

  const handleNodeDataUpdate = useCallback(
    (id: string, data: Record<string, unknown>) => {
      setNodes(nds => nds.map(n => n.id === id ? { ...n, data: { ...n.data, ...data } } : n))
    },
    [setNodes],
  )

  const addNode = useCallback(
    (type: string, label: string, extra?: Record<string, unknown>) => {
      const id = `${type}-${nodeIdCounter.current++}`
      const pos = contextMenu
        ? { x: contextMenu.x, y: contextMenu.y }
        : { x: 300 + Math.random() * 200, y: 200 + Math.random() * 200 }
      const newNode: Node = {
        id, type, position: pos,
        data: {
          label,
          ...(type === 'llm' ? { model: '', prompt: '', temperature: 0.7, maxTokens: 4096 } : {}),
          ...(type === 'tool' ? { toolName: '', argsTemplate: '' } : {}),
          ...(type === 'condition' ? { expression: '' } : {}),
          ...(type === 'media' ? { category: 'character', imageUrl: '', description: '' } : {}),
          ...extra,
        },
      }
      setNodes(nds => [...nds, newNode])
      setShowPalette(false)
      setContextMenu(null)
    },
    [setNodes, contextMenu],
  )

  const [showSwarmConfirm, setShowSwarmConfirm] = useState(false)
  const [showSnapshots, setShowSnapshots] = useState(false)
  const [showOverview, setShowOverview] = useState(false)
  const [showAssetCoverage, setShowAssetCoverage] = useState(false)

  const focusEpisodeFromOverview = useCallback((nodeId: string) => {
    const n = nodes.find(x => x.id === nodeId)
    if (!n) return
    setSelectedNode(n)
    setFocusedEpisodeId(nodeId)
    // 让 ReactFlow 缩到该集
    setTimeout(() => rfRef.current?.setCenter(n.position.x + 100, n.position.y + 70, { zoom: 1.4, duration: 600 }), 50)
  }, [nodes])

  const restoreSnapshot = useCallback((snap: WorkflowSnapshot) => {
    setNodes(snap.data.nodes)
    setEdges(snap.data.edges)
    setWorkflowName(snap.data.workflowName || '未命名工作流')
    if (typeof snap.data.counter === 'number') nodeIdCounter.current = snap.data.counter
  }, [setNodes, setEdges])

  const loadSwarmUniverse = useCallback(() => {
    setShowSwarmConfirm(true)
  }, [])

  const [loadingSwarm, setLoadingSwarm] = useState(false)
  const doLoadSwarmUniverse = useCallback(async () => {
    setLoadingSwarm(true)
    let seed: Node[] = []
    let nextId = nodeIdCounter.current
    try {
      const r = await buildSeedNodes({ startIdCounter: nodeIdCounter.current })
      seed = r.nodes as unknown as Node[]
      nextId = r.nextId
    } catch (e) {
      alert(`加载 manifest 失败：${e instanceof Error ? e.message : String(e)}\n\n请确认 /v1/projects/swarm-universe/assets/manifest.json 可访问。`)
      setLoadingSwarm(false)
      return
    }
    nodeIdCounter.current = nextId
    // 先清理旧的 media 节点（角色/剧集/道具）避免重复，再追加新 seed
    setNodes(nds => [
      ...nds.filter(n => {
        if (n.type !== 'media') return true
        const cat = (n.data as Record<string, unknown>).category
        return cat !== 'character' && cat !== 'scene' && cat !== 'prop'
      }),
      ...(seed as unknown as Node[]),
    ])
    // 一并清除指向被删节点的连线
    setEdges(eds => eds.filter(e => !e.id.startsWith('e-seed-')))
    // Auto-save workflow name if still default
    if (workflowName === '未命名工作流') setWorkflowName('虫群宇宙 · 短剧完整红本')
    setShowSwarmConfirm(false)
  }, [setNodes, setEdges, workflowName])

  // ── 剧本/bible/提示词文本缓存 + 获取 ──
  const textCacheRef = useRef<Record<string, string>>({})
  const fetchTextCached = useCallback(async (url: string): Promise<string> => {
    if (!url) return ''
    if (textCacheRef.current[url]) return textCacheRef.current[url]
    try {
      const r = await fetch(url, { cache: 'no-cache' })
      if (!r.ok) return ''
      const t = await r.text()
      textCacheRef.current[url] = t
      return t
    } catch { return '' }
  }, [])

  // 从 nodes 里收集角色参考（tag → imageUrl 与 label → imageUrl）
  const collectCharacterRefs = useCallback((): { byTag: Record<string, string>; list: Array<{ label: string; tag?: string; url: string }> } => {
    const byTag: Record<string, string> = {}
    const list: Array<{ label: string; tag?: string; url: string }> = []
    for (const n of nodes) {
      if (n.type !== 'media') continue
      const d = n.data as Record<string, unknown>
      if (d.category !== 'character') continue
      // Priority: TOS/CDN URL (bypasses Seedance privacy filter) > local imageUrl (fallback)
      const url = (d.tos_url as string) || (d.cdn_url as string) || (d.imageUrl as string) || ''
      if (!url) continue
      const tag = (d.tag as string) || ''
      const label = (d.label as string) || ''
      list.push({ label, tag, url })
      if (tag) byTag[tag] = url
    }
    return { byTag, list }
  }, [nodes])

  // 收集道具参考（label → publicUrl）。和角色不同：道具一般不带 [图N] tag，只能按
  // 中文 label 在 prompt 文本里 substring 匹配命中后注入 Seedance ref。
  // Priority: tos_url > cdn_url（必须公网可达，本地 /v1/uploads 跳过）。
  const collectPropRefs = useCallback((): Array<{ label: string; tag?: string; url: string }> => {
    const out: Array<{ label: string; tag?: string; url: string }> = []
    for (const n of nodes) {
      if (n.type !== 'media') continue
      const d = n.data as Record<string, unknown>
      if (d.category !== 'prop') continue
      const url = (d.tos_url as string) || (d.cdn_url as string) || ''
      if (!url || !/^https?:\/\//.test(url)) continue // 必须公网
      const label = (d.label as string) || ''
      const tag = (d.tag as string) || ''
      if (!label) continue
      out.push({ label, tag, url })
    }
    return out
  }, [nodes])

  // 轮询 Seedance 视频任务状态
  const pollVideoStatus = useCallback(async (taskId: string, sceneLabel: string, maxSec = 20 * 60): Promise<{ video_url?: string; lastframe_url?: string; status: string; record_id?: string }> => {
    const start = Date.now()
    let pollInterval = 5000
    while ((Date.now() - start) / 1000 < maxSec) {
      if (cancelProductionRef.current) return { status: 'cancelled' }
      // Sleep in 500ms chunks so cancel is detected quickly
      for (let waited = 0; waited < pollInterval; waited += 500) {
        if (cancelProductionRef.current) return { status: 'cancelled' }
        await new Promise(r => setTimeout(r, Math.min(500, pollInterval - waited)))
      }
      if (cancelProductionRef.current) return { status: 'cancelled' }
      try {
        const res = await videoAPI.statusByTaskId(taskId)
        const videos: Array<{ id: string; task_id: string; video_url: string; status: string; lastframe_url?: string }> = res.data.videos || []
        const rec = videos.find(v => v.task_id === taskId)
        if (!rec) { pollInterval = Math.min(pollInterval + 2000, 15000); continue }
        if (rec.status === 'succeeded') {
          return { video_url: rec.video_url, lastframe_url: rec.lastframe_url, status: 'succeeded', record_id: rec.id }
        }
        if (rec.status === 'failed' || rec.status === 'cancelled') {
          return { status: rec.status, record_id: rec.id }
        }
        // running / pending → keep polling
      } catch (e) {
        console.warn(`[runEpisodeProduction] poll ${sceneLabel} error:`, e)
      }
      pollInterval = Math.min(pollInterval + 1000, 20000)
    }
    return { status: 'timeout' }
  }, [])

  const runEpisodeProduction = useCallback(async (episodeData: EpisodeData, nodeId: string, opts?: { initialRefVideoUrl?: string }) => {
    const scenes = episodeData.scenes || []
    if (scenes.length === 0) { showAppToast('warn', '该集没有场景', '请先在面板里添加至少 1 个场景再开始生产。'); return }

    // Reset cancel flag on new production run
    cancelProductionRef.current = false

    // Optimistic UI: composition.status = generating
    setNodes(nds => nds.map(n => {
      if (n.id !== nodeId) return n
      const d = n.data as unknown as EpisodeData
      return { ...n, data: { ...d, composition: { ...(d.composition || { picked_clips: [] }), status: 'generating', picked_clips: d.composition?.picked_clips || [] } } as unknown as Record<string, unknown> }
    }))

    // 1) 加载剧本 / bible / 提示词作为 style_prefix 依据
    const scriptUrl = episodeData.script?.md || ''
    const promptsUrl = episodeData.script?.prompts_md || ''
    const BIBLE_URL = '/v1/projects/swarm-universe/bible.md'
    const [bibleText, , promptsText] = await Promise.all([
      fetchTextCached(BIBLE_URL),
      fetchTextCached(scriptUrl),   // 目前不直接拼入 prompt，仅预热缓存供 ScriptTab 复用
      fetchTextCached(promptsUrl),
    ])
    // 从 bible 首段/前 600 字抽取风格线索，作为 Seedance style_prefix
    const stylePrefix = (() => {
      const lines = (bibleText || '').split('\n').filter(l => l.trim() && !l.trim().startsWith('#')).slice(0, 4).join(' ')
      return lines ? lines.slice(0, 600) : '竖屏短剧 720x1280，现代都市+奇幻元素，冷暖色对比，电影感构图，浅景深，自然光源'
    })()

    // 2) 收集角色参考图 + 道具参考图
    const { byTag, list: charList } = collectCharacterRefs()
    const propList = collectPropRefs()

    // 2.5) 自动刷新即将过期的 TOS URL
    //   refreshTOS 混合策略：先调 /v1/cdn/resign-tos（HMAC 7d，零成本），失败 fallback 到
    //   /v1/cdn/launder-tos（Seedream 再洗 24h）。详见 tosUrlUtils.ts。
    const refreshPlan: Array<{ idx: number; tag: string; oldUrl: string; source: string; nodeId: string }> = []
    for (let i = 0; i < charList.length; i++) {
      const f = parseTOSFreshness(charList[i].url)
      if (!f.parsed || !f.staleOrExpired) continue
      const charTag = charList[i].tag || ''
      const charLabel = charList[i].label
      const n = nodes.find(nn => {
        if (nn.type !== 'media') return false
        const d = nn.data as Record<string, unknown>
        if (d.category !== 'character') return false
        return (charTag && d.tag === charTag) || d.label === charLabel
      })
      if (!n) continue
      const d = n.data as Record<string, unknown>
      const source = (d.cdn_url as string) || (d.imageUrl as string) || ''
      refreshPlan.push({ idx: i, tag: charTag, oldUrl: charList[i].url, source, nodeId: n.id })
    }
    if (refreshPlan.length > 0) {
      console.log(`[runEpisodeProduction] auto-refreshing ${refreshPlan.length} stale TOS URL(s) — resign first, launder fallback`)
      const results = await Promise.allSettled(
        refreshPlan.map(async (p) => ({ p, r: await refreshTOS(p.oldUrl, p.source) })),
      )
      const nodeUpdates: Record<string, string> = {}
      const refreshFailures: string[] = []
      let resignCount = 0
      let promoteCount = 0
      let launderCount = 0
      for (const r of results) {
        if (r.status === 'fulfilled') {
          const { p, r: rr } = r.value
          const refreshSource = rr.source as 'resign' | 'promote' | 'launder'
          charList[p.idx].url = rr.tosUrl
          if (p.tag) byTag[p.tag] = rr.tosUrl
          nodeUpdates[p.nodeId] = rr.tosUrl
          if (refreshSource === 'resign') resignCount++
          else if (refreshSource === 'promote') promoteCount++
          else launderCount++
        } else {
          const rr = r.reason as { response?: { data?: { error?: string } }; message?: string }
          refreshFailures.push(rr?.response?.data?.error || rr?.message || 'unknown')
        }
      }
      console.log(`[runEpisodeProduction] TOS refresh done: resign=${resignCount} promote=${promoteCount} launder=${launderCount} fail=${refreshFailures.length}`)
      if (Object.keys(nodeUpdates).length > 0) {
        setNodes(nds => nds.map(n => nodeUpdates[n.id]
          ? { ...n, data: { ...(n.data as Record<string, unknown>), tos_url: nodeUpdates[n.id] } }
          : n,
        ))
      }
      if (refreshFailures.length > 0) {
        console.warn('[runEpisodeProduction] TOS refresh partial failures:', refreshFailures)
      }
    }

    // 3) 对场景 prompt 做占位符替换（把 [图N] 替换为 label 文字描述，真实的 URL 通过 img_url 传递）
    // 规则：[图1]-[图6] 是角色、[图7]+ 是道具（在 manifest.props[].tag 里声明）。同名 label
    // 多次出现或 [图N] 重复出现，在最终 img_url 传递时会去重。
    const resolveScenePrompt = (rawPrompt: string): { resolved: string; usedUrls: string[] } => {
      const usedTags = new Set<string>()
      let out = rawPrompt || ''
      // [图N] 识别——优先匹配角色表，未命中再查道具表
      out = out.replace(/\[图(\d+)\]/g, (_m, n) => {
        const tag = `[图${n}]`
        usedTags.add(tag)
        const char = charList.find(c => c.tag === tag)
        if (char) return char.label
        const prop = propList.find(p => p.tag === tag)
        if (prop) return prop.label
        return _m
      })
      // Fix: manifest prompt 写 "[图3]苏蜜…" 替换后变 "苏蜜苏蜜…"，折叠重复
      for (const c of charList) {
        if (c.label && out.includes(c.label + c.label)) {
          out = out.split(c.label + c.label).join(c.label)
        }
      }
      for (const p of propList) {
        if (p.label && out.includes(p.label + p.label)) {
          out = out.split(p.label + p.label).join(p.label)
        }
      }
      // 采集 ref URL：
      // 1. 从 [图N] tag 查角色 byTag
      // 2. 从 [图N] tag 查道具 propList
      // 3. 后退：prompt 文本出现道具 label（未走 [图N]）也注入 ref
      const usedUrls: string[] = []
      const seen = new Set<string>()
      const pushUrl = (u: string) => {
        if (!u || seen.has(u)) return
        usedUrls.push(u)
        seen.add(u)
      }
      for (const t of usedTags) {
        if (byTag[t]) pushUrl(byTag[t])
        else {
          const prop = propList.find(p => p.tag === t)
          if (prop) pushUrl(prop.url)
        }
      }
      for (const p of propList) {
        if (!p.url) continue
        if (out.includes(p.label)) pushUrl(p.url)
      }
      return { resolved: out, usedUrls }
    }

    // 4) 从 prompts_md 里提取每个场景的详细 Seedance 英文 prompt（如果存在则优先使用）
    //    简单策略：按 "### S{n}" 或 "## S{n}" 标题切分
    const sceneBlockFromPrompts = (sid: string): string => {
      if (!promptsText) return ''
      const re = new RegExp(`#{2,3}\\s*${sid}[\\s\\S]*?(?=\\n#{2,3}\\s*S\\d|$)`, 'i')
      const m = promptsText.match(re)
      return m ? m[0] : ''
    }

    // 5) 逐场景串行生产（单集顺序，拿上一场 picked_take 尾帧作为下一场 ref_video_url）
    let prevRefVideoUrl: string = opts?.initialRefVideoUrl || ''
    let prevRecordId: string = ''
    const failures: string[] = []
    const skipped: string[] = []
    // Fix #5: 批量模式（scenes.length > 1）下，已经 picked_take 的场景视为"已完成"不再重跑。
    // rerunScene 传进来的是 scopedEp（scenes.length === 1），不触发此逻辑，仍然能强制重拍。
    // 解决用户「做过 3 次派单，每次都从 S1 开始」的问题 —— 现在会自动续接从第一个未完成场景开始。
    const isBatch = scenes.length > 1

    for (const scene of scenes) {
      // Check cancel flag before each scene
      if (cancelProductionRef.current) {
        console.log('[runEpisodeProduction] 用户取消生产')
        break
      }

      // Fix #5 核心：跳过已 picked 的场景，但仍然把它的 picked_take.video_url
      // 挂到 prevRefVideoUrl，这样下一场的「尾帧链」不断。
      if (isBatch && scene.picked_take) {
        const picked = scene.takes?.find(t => t.take_id === scene.picked_take)
        if (picked?.status === 'succeeded' && picked.video_url) {
          prevRefVideoUrl = picked.video_url
          prevRecordId = ''  // 没有缓存的 record_id，后端会 fallback 到 video_url 解析
          skipped.push(scene.id)
          console.log(`[runEpisodeProduction] 跳过 ${scene.id}（已 picked ${scene.picked_take}）`)
          continue
        }
      }
      const takeId = `t${(scene.takes || []).reduce((max, t) => {
        const m = /^t(\d+)$/.exec(t.take_id || '')
        return m ? Math.max(max, Number(m[1])) : max
      }, 0) + 1}`
      const sceneLabelFull = `${episodeData.label} · ${scene.id}`

      // 5b) 组装 prompt + 参考图（先组装，再把 take 标 running 以便日志立即可见）
      const { resolved, usedUrls } = resolveScenePrompt(scene.prompt || scene.label || '')
      const promptsSnippet = sceneBlockFromPrompts(scene.id)

      // Fix #10：古铜钱道具归属约束。
      //   Seedance 在同画面 3 角色 + 多道具场景里会把小物件乱挂——实际已观察到
      //   EP05 里古铜钱被画到了 [图3] 苏蜜胸口（铜钱是 [图1] 林见月的源文明信物）。
      //   本场 prompt 若没提到 [图1] 林见月，或没提到古铜钱关键词，就在 prompt 末尾
      //   塞一段负面约束，告诉模型"这一镜没有古铜钱，任何角色身上都不应该出现"。
      const rawPromptText = (scene.prompt || '') + (promptsSnippet || '')
      const hasLin = /\[图1\]|林见月|Lin Jianyue/i.test(rawPromptText)
      const mentionsCoin = /古铜钱|铜钱|bronze coin|coin/i.test(rawPromptText)
      const coinGuard = !mentionsCoin
        ? '\n\n[道具归属约束] 本镜不要出现古铜钱 / 铜钱 / 任何挂坠铜器；苏蜜、ZERG、颜术、温婉身上绝对不要出现古铜钱。这部短剧是写实风格，真人短剧质感，绝不要卡通 / 动漫 / 插画风格。'
        : !hasLin
          ? '\n\n[道具归属约束] 古铜钱只属于 [图1] 林见月；本镜没有林见月出场时，苏蜜/ZERG/颜术/温婉身上绝对不要出现古铜钱挂饰。写实真人短剧风格，绝不卡通。'
          : '\n\n[道具归属约束] 古铜钱/铜钱发光/金色光晕只能出现在 [图1] 林见月（女主）身上——绝对不能出现在苏蜜、ZERG、颜术、温婉或任何其他角色身上。古铜钱挂在林见月胸口汉服下，发光时金色光从她胸前透出。写实真人短剧质感，电影级画面，绝不要卡通 / 动漫 / 插画风格。'

      const fullPrompt = [
        resolved,
        promptsSnippet ? `\n\n[镜别细则（摘自提示词总稿）]\n${promptsSnippet}` : '',
        coinGuard,
        '\n\n[画面约束] 画面中绝对不要出现任何文字、字幕、水印、标题、对话气泡、歌词条。角色说话只生成语音和口型动作，不要把台词渲染成屏幕上的文字。纯视觉画面，无任何文字叠加。ABSOLUTELY NO on-screen text, subtitles, captions, watermarks, or dialogue bubbles. Dialogue is audio-only with lip sync, never rendered as visible text.',
      ].join('').slice(0, 4000)

      // img_url：[图N] 角色参考 + 命中道具 ref。
      //   后端 parseImageURLList 支持逗号分隔 → 多个 URL 会作为多张 reference_image
      //   （早期只传 usedUrls[0]，导致 S1 里 [图1][图2][图3] 只上 [图3] 苏蜜）
      //
      //   故事板静帧 (scene.storyboard_url) 需同时满足：
      //   - 用户勾选了“用作 i2v 首帧”（storyboard_use_as_ref === true）
      //   - URL 是公网 https 开头（本地 /v1/images/... Seedance 后端无法访问）
      const sbUrl = scene.storyboard_url || ''
      const sbUsable = scene.storyboard_use_as_ref === true && /^https?:\/\//.test(sbUrl)
      const refUrlList = [
        ...(sbUsable ? [sbUrl] : []),        // 故事板首帧（勾选且公网时）
        ...usedUrls,                         // [图N] 角色参考 + 道具 ref
      ].filter(Boolean)
      const imgUrl = refUrlList.join(',')
      // ref_video_url 始终使用上一场 picked_take 的 video_url（Seedance 多模态参考）
      const refVideoUrl = prevRefVideoUrl
      const modelName = 'doubao-seedance-2-0-260128'
      // Seedance 2.0 的 duration 参数合法范围是 4-15（或 -1 auto）。
      // EP06 有些镜头 manifest 里标的 3s（S4a / S5a / S5b / S6a 等），直接派单会被
      // 后端拒绝：InvalidParameter: "the parameter duration specified in the request is not valid"
      // 这里统一 clamp 到 [4,15]，多出来的 1s 用户可以在剪辑时掐掉。
      const seedanceDuration = Math.min(15, Math.max(4, scene.duration || 5))

      // 5a) 先把本场 take 标记为 running 入 UI（并塞入日志字段）
      //     如果 rerunScene 已经预创建了同 takeId 的 running take，更新而非追加
      const takePayload = {
        take_id: takeId,
        status: 'running' as const,
        created_at: new Date().toISOString(),
        prompt: fullPrompt,
        ref_image_url: imgUrl,
        ref_video_url: refVideoUrl,
        ref_video_id: prevRecordId,
        model: modelName,
        duration: seedanceDuration,
      }
      setNodes(nds => nds.map(n => {
        if (n.id !== nodeId) return n
        const d = n.data as unknown as EpisodeData
        const newScenes = (d.scenes || []).map(s => {
          if (s.id !== scene.id) return s
          const existing = (s.takes || []).find(t => t.take_id === takeId)
          return {
            ...s,
            takes: existing
              ? (s.takes || []).map(t => t.take_id === takeId ? { ...t, ...takePayload } : t)
              : [...(s.takes || []), takePayload],
          }
        })
        return { ...n, data: { ...d, scenes: newScenes } as unknown as Record<string, unknown> }
      }))
      // Fix #1: flush save — 跳过 3s debounce。
      // running 状态也落盘的原因： Seedance 单镜可能要跑 2–5 分钟，
      // 中间刷页 / 系统崩 不会丢掉派单状态，task_id 还在，下次 open 能重接轮询。
      if (workflowId) setTimeout(() => { void doSaveRef.current?.() }, 200)

      console.log(`[runEpisodeProduction] ${sceneLabelFull}`, { imgUrl, refVideoUrl, promptLen: fullPrompt.length })

      // 5c) 调 /v1/videos/generate
      let taskId = ''
      let videoUrl = ''
      let lastframeUrl = ''
      let recordId = ''
      let statusNote = ''
      try {
        // Seedance 2.0 原生参数优先：resolution + ratio（对齐豆包文档表格）。
        // size 仅作为 legacy fallback 保留；后端 video_tool_providers.go 会优先
        // 读 resolution/ratio，没给才会从 size 推导。
        // 集面板可以覆盖；没填时默认 1080p + 9:16（抖音短剧）。
        const epRes = (episodeData.video_resolution || '').trim() || '1080p'
        const epRatio = (episodeData.video_ratio || '').trim() || '9:16'
        const requestBody = {
          prompt: fullPrompt,
          model: 'doubao-seedance-2-0-260128',
          img_url: imgUrl || undefined,
          ref_video_url: refVideoUrl || undefined,
          ref_video_id: prevRecordId || undefined,
          duration: seedanceDuration,
          resolution: epRes,
          ratio: epRatio,
          scene: `${episodeData.label}.${scene.id}`,
          style_prefix: stylePrefix,
          generate_audio: true,
          return_last_frame: true,
          watermark: false,
          category: 'short_drama',
        }
        // 把请求体存到 take 上，方便日志复盘
        setNodes(nds => nds.map(n => {
          if (n.id !== nodeId) return n
          const d = n.data as unknown as EpisodeData
          return { ...n, data: { ...d, scenes: (d.scenes || []).map(s => s.id !== scene.id ? s : {
            ...s, takes: (s.takes || []).map(t => t.take_id !== takeId ? t : { ...t, request_body: requestBody })
          })} as unknown as Record<string, unknown> }
        }))
        const res = await videoAPI.generate(requestBody)
        const data = res.data || {}
        taskId = data.task_id || data.result?.task_id || ''
        statusNote = data.message || data.status || ''
        if (!taskId) {
          throw new Error(`后端未返回 task_id: ${JSON.stringify(data).slice(0, 200)}`)
        }
        // 5d) 轮询状态
        const poll = await pollVideoStatus(taskId, sceneLabelFull)
        if (poll.status === 'succeeded' && poll.video_url) {
          videoUrl = poll.video_url
          lastframeUrl = poll.lastframe_url || ''
          recordId = poll.record_id || ''
        } else {
          statusNote = `轮询状态=${poll.status}`
        }
      } catch (e) {
        // 优先解包 axios 后端错误 (err.response.data.error) —— 让用户看到真正原因
        // 例如：no Volcengine API key found. Please configure volcengine or use StarAI
        const ax = e as { response?: { status?: number; data?: { error?: string; message?: string } }; message?: string }
        const backendErr = ax?.response?.data?.error || ax?.response?.data?.message
        const httpStatus = ax?.response?.status
        const rawNote = backendErr
          ? `${backendErr}${httpStatus ? ` (HTTP ${httpStatus})` : ''}`
          : (ax?.message || String(e))
        // Fix #3: Seedance 隐私过滤 —— `InputImageSensitiveContentDetected.PrivacyInformation`
        // 表示服务端判定参考图含真人元素。这是 Seedance 端的硬拦截，我们绕不开。
        // 但可以给用户一个能看懂的解释 + 下一步可执行动作。
        if (/InputImageSensitiveContentDetected|PrivacyInformation|real person/i.test(rawNote)) {
          statusNote = `[Seedance 隐私过滤] 服务端判定本镜的参考图像含真人元素，直接拦截未生成。\n` +
            `可尝试：① 回角色面板点「洗 TOS URL」重新生成偏风格化的 ref（降低写实度）；② 改用 JPEG/低分辨率再试；③ 右键本镜「重拍」用另一套 ref 组合。\n` +
            `原始错误：${rawNote}`
        } else {
          statusNote = rawNote
        }
        console.error(`[runEpisodeProduction] ${sceneLabelFull} generate failed:`, e)
      }

      // 5e) 把 take 更新为 succeeded/failed，并自动 pick
      const ok = !!videoUrl
      setNodes(nds => nds.map(n => {
        if (n.id !== nodeId) return n
        const d = n.data as unknown as EpisodeData
        const newScenes = (d.scenes || []).map(s => {
          if (s.id !== scene.id) return s
          const takes = (s.takes || []).map(t => t.take_id !== takeId ? t : {
            ...t,
            status: ok ? 'succeeded' as const : 'failed' as const,
            video_url: videoUrl || t.video_url,
            lastframe_url: lastframeUrl || t.lastframe_url,
            task_id: taskId || t.task_id,
            note: statusNote || t.note,
            finished_at: new Date().toISOString(),
          })
          return {
            ...s,
            takes,
            picked_take: ok ? takeId : s.picked_take,
          }
        })
        const picked_clips = newScenes.map(s => s.picked_take ? `${s.id}.${s.picked_take}` : '').filter(Boolean)
        const composition = { ...(d.composition || { picked_clips: [] }), picked_clips }
        return { ...n, data: { ...d, scenes: newScenes, composition } as unknown as Record<string, unknown> }
      }))
      // Fix #1: flush save — take 已落到 succeeded/failed。
      // 它是「刷新后状态没保存」的关键点：privacy reject 失败后 → 用户立刻刷新 →
      // 没有此 flush 的话 3s debounce 并没触发到 → takes 消失。
      if (workflowId) setTimeout(() => { void doSaveRef.current?.() }, 200)

      if (ok) {
        prevRefVideoUrl = videoUrl   // 下一场用本场片段整段作为多模态参考
        prevRecordId = recordId       // 也传 record_id，后端会自动 extract_last_frame

        // 5f) 归档到本地剧本目录（fire-and-forget）——不阻塞下一场
        //   docs/<project>/production/<ep>/clips_v2/<scene>_<take>.mp4
        //   + _generated_urls.json 追写。成功后把 take.local_path 挂上，UI 优先用本地源。
        const episodeKey = `${episodeData.is_spinoff ? 'sp' : 'ep'}${String(episodeData.episode_number).padStart(2, '0')}`
        const archiveReq = {
          video_url: videoUrl,
          project: 'swarm-universe',
          episode: episodeKey,
          scene: scene.id,
          take_id: takeId,
          prompt: fullPrompt,
          ref_images: usedUrls.filter(Boolean),
          task_id: taskId,
          model: modelName,
          overwrite: true,
        }
        void videoAPI.archive(archiveReq)
          .then(res => {
            const { local_path, local_url, size, ledger_entries } = res.data || {}
            console.log(`[runEpisodeProduction] ${sceneLabelFull} archived: ${local_path} (${(size / 1024 / 1024).toFixed(2)} MB, ledger=${ledger_entries})`)
            setNodes(nds => nds.map(n => {
              if (n.id !== nodeId) return n
              const d = n.data as unknown as EpisodeData
              const newScenes = (d.scenes || []).map(s => {
                if (s.id !== scene.id) return s
                const takes = (s.takes || []).map(t => t.take_id !== takeId ? t : ({
                  ...t,
                  local_path: local_path || (t as unknown as { local_path?: string }).local_path,
                  local_url: local_url || (t as unknown as { local_url?: string }).local_url,
                } as Take))
                return { ...s, takes }
              })
              return { ...n, data: { ...d, scenes: newScenes } as unknown as Record<string, unknown> }
            }))
            // Flush save immediately — local_url 是关键字段（TOS 24h 过期后的唯一退路），
            // 不能让 autosave 3s debounce 决定它能否留下。
            if (workflowId) setTimeout(() => { void doSaveRef.current?.() }, 200)
          })
          .catch(err => {
            const ax = err as { response?: { data?: { error?: string } }; message?: string }
            console.warn(`[runEpisodeProduction] ${sceneLabelFull} archive failed:`, ax?.response?.data?.error || ax?.message || err)
          })
      } else {
        failures.push(`${scene.id}(${statusNote.slice(0, 200)})`)
        // 失败则中断整集，避免错误扩散
        break
      }
    }

    // 检测典型"配置缺失"错误，给出一条可执行的引导
    const needsKey = failures.some(f => /no\s+\w+\s+API key|not initialized|unauthorized node|API key not configured|no api key/i.test(f))

    // 6) 更新最终状态
    setNodes(nds => nds.map(n => {
      if (n.id !== nodeId) return n
      const d = n.data as unknown as EpisodeData
      const allPicked = (d.scenes || []).every(s => s.picked_take)
      return { ...n, data: { ...d, composition: { ...(d.composition || { picked_clips: [] }), status: allPicked ? 'ready' : 'pending' } } as unknown as Record<string, unknown> }
    }))
    // Fix #1: 一集跑完再 flush 一次 — 防止 composition.status 没保下。
    if (workflowId) setTimeout(() => { void doSaveRef.current?.() }, 200)

    if (cancelProductionRef.current) {
      showAppToast('warn', `${episodeData.label} 已停止生产`, '已完成的镜头已保留，未完成的场景可稍后继续。', { durationMs: 6000 })
    } else if (failures.length) {
      const hint = needsKey
        ? '\n\n🔑 看起来缺少模型供应商密钥。请到「模型配置」页新增一条 volcengine（豆包/Seedance）或 StarAI 密钥，保存后重试。'
        : '\n\n已完成的镜头已保留，可在右侧面板点击对应场景「重拍」单独重试。'
      if (needsKey) {
        showAppToast(
          'err',
          `${episodeData.label} 生产失败`,
          `${failures.join('\n')}${hint}`,
          { durationMs: 15000, ctaLabel: '前往模型配置', onCta: () => navigate('/models') },
        )
      } else {
        showAppToast('err', `${episodeData.label} 生产部分失败`, `${failures.join('\n')}${hint}`, { durationMs: 12000 })
      }
    } else {
      // Fix #5: 在成功 toast 里显示"跳过"信息，让用户明白批量续跑时哪些没重拍。
      const skippedNote = skipped.length > 0
        ? `\n已跳过 ${skipped.length} 镜（已完成）：${skipped.join(', ')}\n本次新生成 ${scenes.length - skipped.length} 镜。`
        : ''
      showAppToast(
        'ok',
        `${episodeData.label} 全部 ${scenes.length} 镜已就绪`,
        `picked_take 已自动选中。请在"合成链路" tab 检查并推进 BGM / 最终合成。${skippedNote}`,
        { durationMs: 8000 },
      )
    }
  }, [setNodes, fetchTextCached, collectCharacterRefs, pollVideoStatus, navigate])

  // 同步 ref，让 rerunScene 可以前向引用 runEpisodeProduction
  useEffect(() => { runEpisodeProductionRef.current = runEpisodeProduction }, [runEpisodeProduction])

  const addMediaNodeWithData = useCallback(
    (data: Record<string, unknown>) => {
      const id = `media-${nodeIdCounter.current++}`
      // Smart positioning: characters left column, episodes by season row
      const isEpisode = data.category === 'scene'
      const isChar = data.category === 'character'
      const existing = nodes.filter(n => n.type === 'media' && (n.data as Record<string, unknown>).category === data.category)
      let pos = { x: 250 + Math.random() * 150, y: 250 + Math.random() * 150 }
      if (isChar) {
        pos = { x: 80, y: 100 + existing.length * 110 }
      } else if (isEpisode) {
        const season = (data as unknown as EpisodeData).season || 1
        const sameSeasonCount = existing.filter(n => ((n.data as unknown) as EpisodeData).season === season).length
        pos = { x: 280 + sameSeasonCount * 220, y: 100 + season * 160 }
      }
      setNodes(nds => [...nds, { id, type: 'media', position: pos, data }])
      return id
    },
    [nodes, setNodes],
  )

  // ── 显示临时 toast · 3s 自动消失 ──
  const showToast = useCallback((kind: 'ok' | 'err' | 'info', msg: string) => {
    setSaveToast({ kind, msg })
    setTimeout(() => setSaveToast(null), 3000)
  }, [])

  // ── 富通知 toast：带图标 + 多行 body + 可选操作按钮，替代原生 alert ──
  const showAppToast = useCallback(
    (
      kind: 'ok' | 'err' | 'warn' | 'info',
      title: string,
      body?: string,
      opts?: { durationMs?: number; ctaLabel?: string; onCta?: () => void },
    ) => {
      if (appToastTimerRef.current) clearTimeout(appToastTimerRef.current)
      setAppToast({ kind, title, body, ctaLabel: opts?.ctaLabel, onCta: opts?.onCta })
      const dur = opts?.durationMs ?? (kind === 'err' ? 10000 : body ? 7000 : 4000)
      appToastTimerRef.current = setTimeout(() => setAppToast(null), dur)
    },
    [],
  )

  // ── 核心保存：auto=true 时走静默模式（不弹 alert，toast 用 info） ──
  const doSave = useCallback(async (opts?: { auto?: boolean }) => {
    const auto = !!opts?.auto
    setSaving(true)
    try {
      const definition = JSON.stringify({ nodes, edges })
      if (definition === lastSavedDefRef.current) {
        if (!auto) showToast('info', '无改动，无需保存')
        return
      }
      if (workflowId) {
        await workflowAPI.update(workflowId, { name: workflowName, definition })
        lastSavedDefRef.current = definition
        showToast(auto ? 'info' : 'ok', auto ? `已自动保存 · ${new Date().toLocaleTimeString('zh-CN')}` : `已保存 · ${nodes.length} 节点 ${edges.length} 连线`)
      } else {
        const res = await workflowAPI.create({ name: workflowName, definition })
        lastSavedDefRef.current = definition
        const newId = res.data.workflow?.id
        if (newId) {
          navigate(`/workflows?id=${newId}`, { replace: true })
          showToast('ok', `已创建并保存 · ${nodes.length} 节点`)
        } else {
          showToast('err', '保存成功但未返回 workflow id')
        }
      }
    } catch (e) {
      const msg = (e as { response?: { data?: { error?: string } }; message?: string })?.response?.data?.error
        || (e as { message?: string })?.message || String(e)
      console.error('[WorkflowPage] save failed:', e)
      showToast('err', `保存失败：${msg}`)
    } finally {
      setSaving(false)
    }
  }, [nodes, edges, workflowId, workflowName, navigate, showToast])

  const handleSave = () => { void doSave() }

  // ── 自动保存：nodes/edges 任意改动 3s debounce 后后台保存（仅当 workflowId 存在） ──
  useEffect(() => {
    if (!workflowId) return
    if (autoSaveTimerRef.current) clearTimeout(autoSaveTimerRef.current)
    autoSaveTimerRef.current = setTimeout(() => { void doSave({ auto: true }) }, 3000)
    return () => { if (autoSaveTimerRef.current) clearTimeout(autoSaveTimerRef.current) }
  }, [nodes, edges, workflowId, doSave])

  // Fix #1: sync doSave into ref so runEpisodeProduction (declared earlier) can trigger
  // immediate save after take running/succeeded/failed transitions without waiting 3s。
  useEffect(() => { doSaveRef.current = () => doSave({ auto: true }) }, [doSave])

  const onPaneContextMenu = useCallback((e: MouseEvent | React.MouseEvent) => {
    e.preventDefault()
    setNodeContextMenu(null)
    setEdgeContextMenu(null)
    setContextMenu({ x: (e as MouseEvent).clientX - 250, y: (e as MouseEvent).clientY - 100 })
  }, [])

  // 连线右键 → 弹连线管理菜单（目前只有删除，保留扩展空间）
  const onEdgeContextMenu = useCallback((e: React.MouseEvent, edge: Edge) => {
    e.preventDefault()
    e.stopPropagation()
    setContextMenu(null)
    setNodeContextMenu(null)
    setEdgeContextMenu({ x: (e as unknown as MouseEvent).clientX, y: (e as unknown as MouseEvent).clientY, edge })
  }, [])

  // 节点右键 → 弹节点管理菜单（删除/复制）
  const onNodeContextMenu = useCallback((e: React.MouseEvent, node: Node) => {
    e.preventDefault()
    e.stopPropagation()
    setContextMenu(null)
    setSelectedNode(node)
    setNodeContextMenu({ x: (e as unknown as MouseEvent).clientX, y: (e as unknown as MouseEvent).clientY, node })
  }, [])

  const duplicateNode = useCallback((node: Node) => {
    const id = `${node.type}-${nodeIdCounter.current++}`
    const copy: Node = {
      ...node, id,
      position: { x: node.position.x + 40, y: node.position.y + 40 },
      selected: false,
      data: JSON.parse(JSON.stringify(node.data || {})),
    }
    setNodes(nds => [...nds, copy])
    setNodeContextMenu(null)
  }, [setNodes])

  return (
    <div ref={containerRef} className="h-full flex flex-col bg-gray-950">
      {/* ── Floating Top Bar ── */}
      <div className="absolute top-3 left-3 right-3 z-50 flex items-center justify-between pointer-events-none">
        <div className="flex items-center gap-2 pointer-events-auto">
          <button onClick={() => navigate(-1)}
            className="p-2 rounded-lg bg-gray-800/80 backdrop-blur border border-gray-700/50 text-gray-400 hover:text-white hover:bg-gray-700/80 transition-all">
            <ArrowLeft className="w-4 h-4" />
          </button>
          {/* 侧边栏折叠时的展开按钮（与 ArrowLeft 平行，避免与 back 按钮重叠） */}
          {!showLeftPanel && (isDramaWorkflow || isAdWorkflow) && (
            <button onClick={() => setShowLeftPanel(true)}
              title="展开侧边栏（项目资产）"
              className="p-2 rounded-lg bg-gray-800/80 backdrop-blur border border-gray-700/50 text-gray-400 hover:text-white hover:bg-gray-700/80 transition-all">
              <PanelLeftOpen className="w-4 h-4" />
            </button>
          )}
          <div className="px-3 py-1.5 rounded-lg bg-gray-800/80 backdrop-blur border border-gray-700/50">
            <input
              value={workflowName}
              onChange={e => setWorkflowName(e.target.value)}
              className="text-sm font-semibold text-gray-200 bg-transparent outline-none w-48"
            />
          </div>
          <div className="px-2.5 py-1 rounded-md bg-violet-500/20 border border-violet-500/30 text-violet-300 text-xs font-mono">
            {nodes.length} nodes · {edges.length} edges
          </div>
        </div>

        <div className="flex items-center gap-1.5 pointer-events-auto">
          {/* 聚焦模式开关 */}
          {isDramaWorkflow && (
          <button onClick={() => {
              const next = !focusMode
              setFocusMode(next)
              if (!next) setFocusedEpisodeId(null)
              else if (selectedNode && (selectedNode.data as Record<string,unknown>).category === 'scene') {
                setFocusedEpisodeId(selectedNode.id)
              }
            }}
            title={focusMode ? '聚焦模式：只显示当前集（点击切换为总览）' : '总览模式：显示全部剧集（点击切换为聚焦）'}
            className={`flex items-center gap-1.5 px-2.5 py-1.5 text-xs font-medium rounded-lg backdrop-blur border transition-all ${
              focusMode
                ? 'bg-cyan-600/20 border-cyan-500/50 text-cyan-200 hover:bg-cyan-600/30'
                : 'bg-gray-800/80 border-gray-700 text-gray-400 hover:text-gray-200'
            }`}>
            {focusMode ? <Film className="w-3.5 h-3.5" /> : <Layers className="w-3.5 h-3.5" />}
            {focusMode ? '聚焦' : '总览'}
          </button>
          )}
          {/* 退出聚焦快捷键（focused 时） */}
          {isDramaWorkflow && focusMode && focusedEpisodeId && (
            <button onClick={() => {
                setFocusedEpisodeId(null)
                setTimeout(() => { try { rfRef.current?.fitView({ duration: 400, padding: 0.2 }) } catch {} }, 50)
              }}
              title="退出聚焦，查看全部"
              className="flex items-center gap-1 px-2.5 py-1.5 text-xs font-medium rounded-lg bg-gray-800/80 backdrop-blur border border-gray-700 text-gray-300 hover:bg-gray-700 hover:text-white transition">
              <ArrowLeft className="w-3.5 h-3.5" /> 全部
            </button>
          )}
          {isDramaWorkflow && (
          <button onClick={loadSwarmUniverse}
            title="一键加载虫群宇宙完整资产 (5角色 + 7道具 + 50集 + 衍生剧)"
            className="flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium rounded-lg bg-gradient-to-r from-violet-600/90 to-cyan-600/90 backdrop-blur border border-violet-500/50 text-white hover:from-violet-500 hover:to-cyan-500 transition-all shadow-lg shadow-violet-900/30">
            <Sparkles className="w-3.5 h-3.5" /> 虫群宇宙
          </button>
          )}
          {isDramaWorkflow && (
          <button onClick={() => setShowAssetCoverage(true)}
            title="全集物料对齐扫描：基础健康（角色/道具 ref+TOS）+ 跨集 preflight 汇总"
            className="flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium rounded-lg bg-gray-800/80 backdrop-blur border border-emerald-700/40 text-emerald-200 hover:text-white hover:bg-emerald-900/40 transition-all">
            <ClipboardCheck className="w-3.5 h-3.5" /> 物料对齐
          </button>
          )}
          <button onClick={() => setShowPalette(!showPalette)}
            className="flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium rounded-lg bg-gray-800/80 backdrop-blur border border-gray-700/50 text-gray-300 hover:text-white hover:bg-gray-700/80 transition-all">
            <Plus className="w-3.5 h-3.5" /> 添加
          </button>
          {selectedNode && selectedNode.type !== 'start' && selectedNode.type !== 'end' && (
            <button onClick={deleteSelectedNode}
              className="flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium rounded-lg bg-red-900/40 backdrop-blur border border-red-700/50 text-red-300 hover:text-white hover:bg-red-800/60 transition-all">
              <Trash2 className="w-3.5 h-3.5" /> 删除
            </button>
          )}
          <div className="w-px h-6 bg-gray-700/50 mx-1" />
          <button onClick={() => setShowCostModal(true)}
            className="flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium rounded-lg bg-amber-900/40 backdrop-blur border border-amber-700/50 text-amber-200 hover:text-white hover:bg-amber-800/60 transition-all"
            title="成本分析：扫描所有剧集 takes 计算 Seedance/LLM/图片/BGM 费用">
            <Coins className="w-3.5 h-3.5" /> 成本
          </button>
          <button onClick={() => setShowSnapshots(true)}
            className="flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium rounded-lg bg-purple-900/40 backdrop-blur border border-purple-700/50 text-purple-200 hover:text-white hover:bg-purple-800/60 transition-all"
            title="存档 / 快照管理（本地）">
            <Camera className="w-3.5 h-3.5" /> 存档
          </button>
          <button onClick={handleSave} disabled={saving}
            className="flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium rounded-lg bg-gray-800/80 backdrop-blur border border-gray-700/50 text-gray-300 hover:text-white hover:bg-gray-700/80 disabled:opacity-50 transition-all"
            title="保存到后端（跨设备） · 改动后 3s 会自动保存">
            {saving ? <Loader2 className="w-3.5 h-3.5 animate-spin" /> : <Save className="w-3.5 h-3.5" />} 保存
          </button>
          {/* 保存状态 toast，3s 自动消失 */}
          {saveToast && (
            <div className={`flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium rounded-lg backdrop-blur border transition-all ${
              saveToast.kind === 'ok' ? 'bg-emerald-900/60 border-emerald-600/50 text-emerald-100'
              : saveToast.kind === 'err' ? 'bg-red-900/60 border-red-600/50 text-red-100'
              : 'bg-gray-800/80 border-gray-600/50 text-gray-300'
            }`}>
              {saveToast.msg}
            </div>
          )}
          <button onClick={toggleFullscreen}
            className="p-2 rounded-lg bg-gray-800/80 backdrop-blur border border-gray-700/50 text-gray-400 hover:text-white hover:bg-gray-700/80 transition-all">
            {isFullscreen ? <Minimize2 className="w-4 h-4" /> : <Maximize2 className="w-4 h-4" />}
          </button>
        </div>
      </div>

      {/* ── Canvas + Left Panel ── */}
      <div className="flex-1 flex overflow-hidden">
        {/* Left Asset Panel */}
        {showLeftPanel && isDramaWorkflow && (
          <div className="w-56 bg-gray-900 border-r border-gray-800 flex flex-col overflow-hidden z-30">
            <div className="px-3 py-2.5 pt-14 border-b border-gray-800 flex items-center justify-between">
              <span className="text-[11px] font-semibold text-gray-400 uppercase tracking-wider">项目资产</span>
              <button onClick={() => setShowLeftPanel(false)} className="p-1 text-gray-600 hover:text-gray-400 transition-colors">
                <PanelLeftClose className="w-3.5 h-3.5" />
              </button>
            </div>
            {/* 🌌 宇宙总览入口 */}
            <button
              onClick={() => setShowOverview(true)}
              className="mx-2 mt-2 mb-1 px-2.5 py-2 rounded-lg bg-gradient-to-r from-violet-600/20 via-cyan-600/20 to-emerald-600/20 hover:from-violet-600/40 hover:via-cyan-600/40 hover:to-emerald-600/40 border border-violet-500/30 hover:border-violet-400/60 text-xs font-medium text-violet-200 hover:text-white transition flex items-center gap-2 shadow-md shadow-violet-900/20"
              title="查看 5 季 50 集项目全景 + 角色卡 + 世界观骨架"
            >
              <Sparkles className="w-3.5 h-3.5" />
              <span className="flex-1 text-left">🌌 宇宙总览</span>
              <ChevronRight className="w-3 h-3 opacity-60" />
            </button>
            <div className="flex-1 overflow-y-auto scrollbar-thin">
              {/* Characters */}
              {(() => {
                const chars = nodes.filter(n => n.type === 'media' && (n.data as Record<string,unknown>).category === 'character')
                return (
                  <div className="border-b border-gray-800/50">
                    <div className="flex items-center justify-between pr-2">
                      <button onClick={() => setExpandedSections(s => ({ ...s, characters: !s.characters }))}
                        className="flex-1 flex items-center gap-2 px-3 py-2 text-xs font-medium text-gray-400 hover:text-gray-200 transition-colors">
                        {expandedSections.characters ? <ChevronDown className="w-3 h-3" /> : <ChevronRight className="w-3 h-3" />}
                        <Users className="w-3.5 h-3.5 text-violet-400" />
                        <span>角色 ({chars.length})</span>
                      </button>
                      <button onClick={() => setShowCharModal(true)} title="新建角色"
                        className="p-1 rounded text-violet-400 hover:text-violet-300 hover:bg-violet-500/10 transition">
                        <Plus className="w-3.5 h-3.5" />
                      </button>
                    </div>
                    {expandedSections.characters && (
                      <div className="pb-2 space-y-0.5">
                        {chars.length === 0 && (
                          <button onClick={() => setShowCharModal(true)}
                            className="w-[calc(100%-16px)] mx-2 py-2 rounded-lg border border-dashed border-gray-700 hover:border-violet-500/60 text-[11px] text-gray-500 hover:text-violet-400 transition flex items-center justify-center gap-1">
                            <Plus className="w-3 h-3" /> 新建第一个角色
                          </button>
                        )}
                        {chars.map(n => {
                          const d = n.data as unknown as CharacterData
                          return (
                            <button key={n.id} onClick={() => { setSelectedNode(n) }}
                              className={`w-full flex items-center gap-2 px-3 py-1.5 text-left hover:bg-gray-800/60 transition-colors ${selectedNode?.id === n.id ? 'bg-violet-900/30 border-l-2 border-violet-500' : ''}`}>
                              {d.imageUrl ? (
                                <img src={d.imageUrl} alt="" className="w-7 h-7 rounded object-cover border border-gray-700 flex-shrink-0" />
                              ) : (
                                <div className="w-7 h-7 rounded bg-violet-900/40 border border-violet-700/50 flex items-center justify-center flex-shrink-0">
                                  <Users className="w-3 h-3 text-violet-400" />
                                </div>
                              )}
                              <div className="min-w-0 flex-1">
                                <div className="text-xs text-gray-300 truncate flex items-center gap-1">
                                  {d.tag && <span className="text-violet-400 font-mono text-[10px]">{d.tag}</span>}
                                  <span className="truncate">{d.label}</span>
                                </div>
                                {d.description && <div className="text-[10px] text-gray-600 truncate">{d.description}</div>}
                              </div>
                            </button>
                          )
                        })}
                      </div>
                    )}
                  </div>
                )
              })()}

              {/* Episodes by Season */}
              {(() => {
                const allEps = nodes.filter(n => n.type === 'media' && (n.data as Record<string,unknown>).category === 'scene')
                const mainSeries = allEps.filter(n => !(n.data as unknown as EpisodeData).is_spinoff)
                const spinoffs = allEps.filter(n => (n.data as unknown as EpisodeData).is_spinoff)

                const renderEpItem = (n: Node) => {
                  const d = n.data as unknown as EpisodeData
                  const scenes = d.scenes || []
                  const picked = scenes.filter(s => s.picked_take).length
                  return (
                    <button key={n.id} onClick={() => focusEpisode(n)}
                      className={`w-full flex items-center gap-2 px-3 py-1.5 text-left hover:bg-gray-800/60 transition-colors ${selectedNode?.id === n.id ? 'bg-cyan-900/30 border-l-2 border-cyan-500' : ''}`}>
                      <Film className="w-3.5 h-3.5 text-cyan-500 flex-shrink-0" />
                      <div className="min-w-0 flex-1">
                        <div className="text-xs text-gray-300 truncate">{d.label}</div>
                        <div className="text-[10px] text-gray-600 truncate flex items-center gap-1">
                          <span>{scenes.length}镜 · {d.duration || 0}s</span>
                          {scenes.length > 0 && (
                            <span className={picked === scenes.length ? 'text-emerald-400' : picked > 0 ? 'text-amber-400' : ''}>
                              · {picked}/{scenes.length} 已选
                            </span>
                          )}
                        </div>
                      </div>
                      {d.composition?.status === 'ready' && <span className="text-[9px] text-emerald-400">●</span>}
                      {d.composition?.status === 'generating' && <span className="text-[9px] text-amber-400 animate-pulse">●</span>}
                    </button>
                  )
                }

                return (
                  <>
                    {/* Season groups */}
                    {SEASONS.map(season => {
                      const eps = mainSeries.filter(n => (n.data as unknown as EpisodeData).season === season.number)
                      const sectionKey = `s${season.number}`
                      return (
                        <div key={season.number} className="border-b border-gray-800/50">
                          <div className="flex items-center justify-between pr-2">
                            <button onClick={() => setExpandedSections(s => ({ ...s, [sectionKey]: !s[sectionKey] }))}
                              className="flex-1 flex items-center gap-2 px-3 py-2 text-xs font-medium text-gray-400 hover:text-gray-200 transition-colors">
                              {expandedSections[sectionKey] ? <ChevronDown className="w-3 h-3" /> : <ChevronRight className="w-3 h-3" />}
                              <span className={`inline-block w-1.5 h-1.5 rounded-full bg-gradient-to-r ${season.gradient}`} />
                              <span className="flex-1 truncate">{season.title} · {season.subtitle}</span>
                              <span className="text-[10px] text-gray-600">{eps.length}/10</span>
                            </button>
                            <button onClick={() => { setEpModalSeason(season.number); setEpModalSpinoffGroup(undefined); setShowEpModal(true) }}
                              title={`新建${season.title}剧集`}
                              className="p-1 rounded text-cyan-400 hover:text-cyan-300 hover:bg-cyan-500/10 transition">
                              <Plus className="w-3.5 h-3.5" />
                            </button>
                          </div>
                          {expandedSections[sectionKey] && (
                            <div className="pb-2 space-y-0.5">
                              {eps.length === 0 ? (
                                <div className="px-3 py-1.5 text-[10px] text-gray-600 italic">{season.arc} · {season.episode_range} · {season.duration_hint}</div>
                              ) : (
                                eps.sort((a, b) => ((a.data as unknown as EpisodeData).episode_number || 0) - ((b.data as unknown as EpisodeData).episode_number || 0)).map(renderEpItem)
                              )}
                            </div>
                          )}
                        </div>
                      )
                    })}

                    {/* Spin-offs */}
                    <div className="border-b border-gray-800/50">
                      <div className="flex items-center justify-between pr-2">
                        <button onClick={() => setExpandedSections(s => ({ ...s, spinoff: !s.spinoff }))}
                          className="flex-1 flex items-center gap-2 px-3 py-2 text-xs font-medium text-gray-400 hover:text-gray-200 transition-colors">
                          {expandedSections.spinoff ? <ChevronDown className="w-3 h-3" /> : <ChevronRight className="w-3 h-3" />}
                          <Clapperboard className="w-3.5 h-3.5 text-slate-400" />
                          <span className="flex-1 truncate">衍生剧</span>
                          <span className="text-[10px] text-gray-600">{spinoffs.length}</span>
                        </button>
                        <button onClick={() => { setEpModalSeason(0); setEpModalSpinoffGroup(SPINOFF_GROUPS[0].key); setShowEpModal(true) }}
                          title="新建衍生剧"
                          className="p-1 rounded text-slate-400 hover:text-slate-300 hover:bg-slate-500/10 transition">
                          <Plus className="w-3.5 h-3.5" />
                        </button>
                      </div>
                      {expandedSections.spinoff && (
                        <div className="pb-2 space-y-0.5">
                          {spinoffs.length === 0 ? (
                            <div className="px-3 py-1.5 text-[10px] text-gray-600 italic">《道裂》前传 / MCU外传 / 产品联动</div>
                          ) : (
                            // group by spinoff_group
                            Array.from(new Set(spinoffs.map(n => (n.data as unknown as EpisodeData).spinoff_group || '未分组'))).map(group => (
                              <div key={group} className="space-y-0.5">
                                <div className="px-3 pt-1 pb-0.5 text-[10px] text-slate-500 uppercase tracking-wider">{group}</div>
                                {spinoffs.filter(n => ((n.data as unknown as EpisodeData).spinoff_group || '未分组') === group)
                                  .sort((a, b) => ((a.data as unknown as EpisodeData).episode_number || 0) - ((b.data as unknown as EpisodeData).episode_number || 0))
                                  .map(renderEpItem)}
                              </div>
                            ))
                          )}
                        </div>
                      )}
                    </div>
                  </>
                )
              })()}

              {/* Props */}
              {(() => {
                const props = nodes.filter(n => n.type === 'media' && (n.data as Record<string,unknown>).category === 'prop')
                if (props.length === 0) return null
                return (
                  <div className="border-b border-gray-800/50">
                    <button onClick={() => setExpandedSections(s => ({ ...s, props: !s.props }))}
                      className="w-full flex items-center gap-2 px-3 py-2 text-xs font-medium text-gray-400 hover:text-gray-200 transition-colors">
                      {expandedSections.props ? <ChevronDown className="w-3 h-3" /> : <ChevronRight className="w-3 h-3" />}
                      <Package className="w-3.5 h-3.5 text-amber-400" />
                      <span>道具 ({props.length})</span>
                    </button>
                    {expandedSections.props && (
                      <div className="pb-2 space-y-0.5">
                        {props.map(n => {
                          const d = n.data as Record<string,unknown>
                          return (
                            <button key={n.id} onClick={() => { setSelectedNode(n) }}
                              className={`w-full flex items-center gap-2 px-3 py-1.5 text-left hover:bg-gray-800/60 transition-colors ${selectedNode?.id === n.id ? 'bg-amber-900/30 border-l-2 border-amber-500' : ''}`}>
                              {d.imageUrl ? (
                                <img src={d.imageUrl as string} alt="" className="w-7 h-7 rounded object-cover border border-gray-700 flex-shrink-0" />
                              ) : (
                                <Package className="w-3.5 h-3.5 text-amber-400 flex-shrink-0" />
                              )}
                              <div className="text-xs text-gray-300 truncate">{d.label as string}</div>
                            </button>
                          )
                        })}
                      </div>
                    )}
                  </div>
                )
              })()}

              {/* Pipeline / LLM / Tool nodes */}
              {(() => {
                const pipeline = nodes.filter(n => n.type === 'llm' || n.type === 'tool')
                if (pipeline.length === 0) return null
                return (
                  <div className="border-b border-gray-800/50">
                    <button onClick={() => setExpandedSections(s => ({ ...s, pipeline: !s.pipeline }))}
                      className="w-full flex items-center gap-2 px-3 py-2 text-xs font-medium text-gray-400 hover:text-gray-200 transition-colors">
                      {expandedSections.pipeline ? <ChevronDown className="w-3 h-3" /> : <ChevronRight className="w-3 h-3" />}
                      <FileText className="w-3.5 h-3.5 text-blue-400" />
                      <span>流水线 ({pipeline.length})</span>
                    </button>
                    {expandedSections.pipeline && (
                      <div className="pb-2 space-y-0.5">
                        {pipeline.map(n => {
                          const d = n.data as Record<string,unknown>
                          return (
                            <button key={n.id} onClick={() => { setSelectedNode(n) }}
                              className={`w-full flex items-center gap-2 px-3 py-1.5 text-left hover:bg-gray-800/60 transition-colors ${selectedNode?.id === n.id ? 'bg-blue-900/30 border-l-2 border-blue-500' : ''}`}>
                              {n.type === 'llm' ? <Cpu className="w-3.5 h-3.5 text-blue-400 flex-shrink-0" /> : <Wrench className="w-3.5 h-3.5 text-amber-400 flex-shrink-0" />}
                              <div className="text-xs text-gray-300 truncate">{d.label as string}</div>
                            </button>
                          )
                        })}
                      </div>
                    )}
                  </div>
                )
              })()}
            </div>
          </div>
        )}

        {/* Left Ad Panel — 广告宣传片类型/风格菜单 */}
        {showLeftPanel && isAdWorkflow && !isDramaWorkflow && (
          <div className="w-56 bg-gray-900 border-r border-gray-800 flex flex-col overflow-hidden z-30">
            <div className="px-3 py-2.5 pt-14 border-b border-gray-800 flex items-center justify-between">
              <span className="text-[11px] font-semibold text-gray-400 uppercase tracking-wider">宣传片菜单</span>
              <button onClick={() => setShowLeftPanel(false)} className="p-1 text-gray-600 hover:text-gray-400 transition-colors">
                <PanelLeftClose className="w-3.5 h-3.5" />
              </button>
            </div>
            <div className="flex-1 overflow-y-auto scrollbar-thin">
              {/* 广告类型 + 类型下的剧本（可折叠，类似短剧 SEASONS） */}
              {(() => {
                const typeNodes = nodes.filter(n => n.type === 'media' && (n.data as Record<string,unknown>).category === 'type')
                const allScripts = nodes.filter(n => n.type === 'media' && (n.data as Record<string,unknown>).category === 'scene')
                const renderScriptItem = (n: Node) => {
                  const d = n.data as unknown as EpisodeData
                  const sceneArr = d.scenes || []
                  const picked = sceneArr.filter(s => s.picked_take).length
                  const cover = (d as unknown as { cover_url?: string }).cover_url
                  return (
                    <button key={n.id} onClick={() => focusEpisode(n)}
                      className={`w-full flex items-center gap-2 pl-9 pr-3 py-1.5 text-left hover:bg-gray-800/60 transition-colors ${selectedNode?.id === n.id ? 'bg-indigo-900/30 border-l-2 border-indigo-500' : ''}`}>
                      {cover ? (
                        <img src={cover} alt="" className="w-7 h-7 rounded object-cover border border-gray-700 flex-shrink-0" />
                      ) : (
                        <Film className="w-3.5 h-3.5 text-cyan-500 flex-shrink-0" />
                      )}
                      <div className="min-w-0 flex-1">
                        <div className="text-xs text-gray-300 truncate">{d.label}</div>
                        <div className="text-[10px] text-gray-600 truncate flex items-center gap-1">
                          <span>{sceneArr.length}镜 · {d.duration || 0}s</span>
                          {sceneArr.length > 0 && (
                            <span className={picked === sceneArr.length ? 'text-emerald-400' : picked > 0 ? 'text-amber-400' : ''}>
                              · {picked}/{sceneArr.length} 已选
                            </span>
                          )}
                        </div>
                      </div>
                      {d.composition?.status === 'ready' && <span className="text-[9px] text-emerald-400">●</span>}
                      {d.composition?.status === 'generating' && <span className="text-[9px] text-amber-400 animate-pulse">●</span>}
                    </button>
                  )
                }
                return (
                  <div className="border-b border-gray-800/50">
                    <div className="flex items-center gap-2 px-3 py-2 text-xs font-medium text-gray-400">
                      <Layers className="w-3.5 h-3.5 text-indigo-400" />
                      <span>广告类型 ({typeNodes.length})</span>
                    </div>
                    <div className="pb-2 space-y-0.5">
                      {typeNodes.map(n => {
                        const d = n.data as Record<string,unknown>
                        const typeScripts = allScripts.filter(s => ((s.data as Record<string,unknown>).ad_type as string) === n.id)
                        const sectionKey = `adtype-${n.id}`
                        const isExpanded = expandedSections[sectionKey] ?? typeScripts.length > 0
                        return (
                          <div key={n.id}>
                            <div className="flex items-center pr-1">
                              <button onClick={() => setExpandedSections(s => ({ ...s, [sectionKey]: !isExpanded }))}
                                className="p-1 text-gray-600 hover:text-gray-400 transition-colors flex-shrink-0">
                                {isExpanded ? <ChevronDown className="w-3 h-3" /> : <ChevronRight className="w-3 h-3" />}
                              </button>
                              <button onClick={() => { setSelectedNode(n); rfRef.current?.setCenter(n.position.x + 100, n.position.y + 50, { zoom: 1, duration: 500 }) }}
                                className={`flex-1 flex items-center gap-2.5 px-1 py-1.5 text-left hover:bg-gray-800/60 transition-colors ${selectedNode?.id === n.id ? 'bg-indigo-900/30 border-l-2 border-indigo-500' : ''}`}>
                                <div className="w-8 h-8 rounded-lg bg-indigo-900/40 border border-indigo-700/50 flex items-center justify-center flex-shrink-0 text-base">
                                  {((d.label as string) || '').slice(0, 2)}
                                </div>
                                <div className="min-w-0 flex-1">
                                  <div className="text-xs text-gray-200 truncate">{((d.label as string) || '').slice(2).trim()}</div>
                                  <div className="text-[10px] text-gray-500 truncate">{d.description as string}</div>
                                </div>
                                <span className="text-[10px] text-gray-600 flex-shrink-0">{typeScripts.length}</span>
                              </button>
                              <button onClick={() => {
                                  setImportTargetType({ id: n.id, label: (d.label as string).replace(/^[\p{Emoji}\s]+/u, '').trim() })
                                  setShowImportModal(true)
                                }}
                                title={`导入 ${(d.label as string).slice(2).trim()} 剧本 (.md)`}
                                className="p-1 rounded text-indigo-400 hover:text-indigo-300 hover:bg-indigo-500/10 transition flex-shrink-0">
                                <Plus className="w-3.5 h-3.5" />
                              </button>
                            </div>
                            {isExpanded && (
                              <div className="pb-1 space-y-0.5">
                                {typeScripts.length === 0 ? (
                                  <div className="pl-9 pr-3 py-1.5 text-[10px] text-gray-600 italic">暂无剧本 · 点 + 新建</div>
                                ) : (
                                  typeScripts.map(renderScriptItem)
                                )}
                              </div>
                            )}
                          </div>
                        )
                      })}
                    </div>
                  </div>
                )
              })()}

              {/* 视觉风格 */}
              {(() => {
                const styleNodes = nodes.filter(n => n.type === 'media' && (n.data as Record<string,unknown>).category === 'style')
                if (styleNodes.length === 0) return null
                return (
                  <div className="border-b border-gray-800/50">
                    <div className="flex items-center gap-2 px-3 py-2 text-xs font-medium text-gray-400">
                      <Camera className="w-3.5 h-3.5 text-amber-400" />
                      <span>视觉风格 ({styleNodes.length})</span>
                    </div>
                    <div className="pb-2 space-y-0.5">
                      {styleNodes.map(n => {
                        const d = n.data as Record<string,unknown>
                        return (
                          <button key={n.id} onClick={() => { setSelectedNode(n); rfRef.current?.setCenter(n.position.x + 100, n.position.y + 50, { zoom: 1, duration: 500 }) }}
                            className={`w-full flex items-center gap-2.5 px-3 py-2 text-left hover:bg-gray-800/60 transition-colors ${selectedNode?.id === n.id ? 'bg-amber-900/30 border-l-2 border-amber-500' : ''}`}>
                            <div className="w-8 h-8 rounded-lg bg-amber-900/40 border border-amber-700/50 flex items-center justify-center flex-shrink-0">
                              <Camera className="w-3.5 h-3.5 text-amber-400" />
                            </div>
                            <div className="min-w-0 flex-1">
                              <div className="text-xs text-gray-200 truncate">{d.label as string}</div>
                              <div className="text-[10px] text-gray-500 truncate">{d.description as string}</div>
                            </div>
                          </button>
                        )
                      })}
                    </div>
                  </div>
                )
              })()}

              {/* 制作流水线 */}
              {(() => {
                const pipeline = nodes.filter(n => n.type === 'llm' || n.type === 'tool')
                if (pipeline.length === 0) return null
                return (
                  <div className="border-b border-gray-800/50">
                    <div className="flex items-center gap-2 px-3 py-2 text-xs font-medium text-gray-400">
                      <GitBranch className="w-3.5 h-3.5 text-cyan-400" />
                      <span>制作流水线 ({pipeline.length})</span>
                    </div>
                    <div className="pb-2 space-y-0.5">
                      {pipeline.map(n => {
                        const d = n.data as Record<string,unknown>
                        return (
                          <button key={n.id} onClick={() => { setSelectedNode(n); rfRef.current?.setCenter(n.position.x + 100, n.position.y + 50, { zoom: 1, duration: 500 }) }}
                            className={`w-full flex items-center gap-2 px-3 py-1.5 text-left hover:bg-gray-800/60 transition-colors ${selectedNode?.id === n.id ? 'bg-cyan-900/30 border-l-2 border-cyan-500' : ''}`}>
                            {n.type === 'llm' ? <Cpu className="w-3.5 h-3.5 text-blue-400 flex-shrink-0" /> : <Wrench className="w-3.5 h-3.5 text-amber-400 flex-shrink-0" />}
                            <div className="text-xs text-gray-300 truncate">{d.label as string}</div>
                          </button>
                        )
                      })}
                    </div>
                  </div>
                )
              })()}
            </div>
          </div>
        )}

        {/* Left panel toggle when hidden → 已移到顶栏与 ArrowLeft 并排 */}

        <div className="flex-1 relative" style={{ touchAction: 'none' }}>
          <ReactFlow
            nodes={displayNodes}
            edges={displayEdges}
            onInit={(inst) => { rfRef.current = inst }}
            onNodesChange={onNodesChange}
            onEdgesChange={onEdgesChange}
            onConnect={onConnect}
            onNodeClick={onNodeClick}
            onNodeDragStop={onNodeDragStop}
            onPaneClick={() => { setSelectedNode(null); setContextMenu(null); setNodeContextMenu(null); setEdgeContextMenu(null); setFocusedSceneId(null) }}
            onPaneContextMenu={onPaneContextMenu}
            onNodeContextMenu={onNodeContextMenu}
            onEdgeContextMenu={onEdgeContextMenu}
            nodeTypes={nodeTypes}
            fitView
            className="!bg-gray-950"
            defaultEdgeOptions={{ style: { stroke: '#475569', strokeWidth: 1.5 }, animated: true }}
            deleteKeyCode="Delete"
            minZoom={0.1}
            maxZoom={3}
            panOnDrag={[0, 1, 2]}
            zoomOnPinch
            zoomOnDoubleClick={false}
            selectNodesOnDrag={false}
            nodeDragThreshold={1}
          >
            <Controls position="bottom-left"
              className="!bg-gray-800/90 !border-gray-700 !rounded-lg !shadow-xl [&>button]:!bg-gray-800 [&>button]:!border-gray-700 [&>button]:!text-gray-400 [&>button:hover]:!bg-gray-700 [&>button:hover]:!text-white" />
            <MiniMap position="bottom-right"
              nodeStrokeColor="#475569" nodeColor="#1e293b"
              maskColor="rgba(0,0,0,0.5)"
              className="!bg-gray-900/90 !border !border-gray-700 !rounded-lg !shadow-xl" />
            <Background variant={BackgroundVariant.Dots} gap={24} size={1} color="#1e293b" />

            {/* Node Palette Panel */}
            {showPalette && (
              <Panel position="top-left" className="!top-16">
                <div className="bg-gray-800/95 backdrop-blur-xl rounded-xl shadow-2xl border border-gray-700/50 p-3 w-52">
                  <p className="text-[10px] font-semibold text-gray-500 uppercase tracking-wider mb-2 px-1">添加节点</p>
                  <div className="space-y-1">
                    {NODE_PALETTE.map(tpl => (
                      <button key={tpl.type}
                        onClick={() => addNode(tpl.type, tpl.label, tpl.defaultData)}
                        className="w-full flex items-center gap-2.5 p-2 rounded-lg hover:bg-gray-700/60 transition-colors group">
                        <tpl.icon className={`w-4 h-4 ${tpl.color} group-hover:scale-110 transition-transform`} />
                        <div className="text-left">
                          <span className="text-xs font-medium text-gray-300">{tpl.label}</span>
                          <p className="text-[10px] text-gray-500">{tpl.desc}</p>
                        </div>
                      </button>
                    ))}
                  </div>
                  <div className="mt-2 pt-2 border-t border-gray-700/50">
                    <p className="text-[10px] text-gray-600 px-1">提示：右键画布也可添加节点</p>
                  </div>
                </div>
              </Panel>
            )}

            {/* Right-click Context Menu · 画布空白：添加节点 */}
            {contextMenu && (
              <div className="absolute bg-gray-800/95 backdrop-blur-xl rounded-lg shadow-2xl border border-gray-700/50 py-1 w-44 z-50"
                style={{ left: contextMenu.x + 250, top: contextMenu.y + 100 }}>
                {NODE_PALETTE.map(tpl => (
                  <button key={tpl.type}
                    onClick={() => addNode(tpl.type, tpl.label, tpl.defaultData)}
                    className="w-full flex items-center gap-2 px-3 py-1.5 hover:bg-gray-700/60 transition-colors">
                    <tpl.icon className={`w-3.5 h-3.5 ${tpl.color}`} />
                    <span className="text-xs text-gray-300">{tpl.label}</span>
                  </button>
                ))}
              </div>
            )}
          </ReactFlow>

          {/* 画布底部·生产日志 Dock（选中剧集节点时出现，可折叠，可拖拽调高） */}
          {selectedNodeLive && selectedNodeLive.type === 'media' && (selectedNodeLive.data as Record<string, unknown>).category === 'scene' && (
            <div className="absolute left-0 right-0 bottom-0 z-40 bg-gray-950/95 backdrop-blur-xl border-t border-gray-800 shadow-2xl"
                 style={{ height: logsDockOpen ? logsDockHeight : 32 }}>
              {/* 拖拽手柄 */}
              {logsDockOpen && (
                <div className="absolute -top-1 left-0 right-0 h-2 cursor-ns-resize z-50 hover:bg-cyan-500/30 active:bg-cyan-500/50 transition"
                     onMouseDown={(e) => {
                       e.preventDefault()
                       logsDragRef.current = { startY: e.clientY, startH: logsDockHeight }
                       const onMove = (ev: MouseEvent) => {
                         if (!logsDragRef.current) return
                         const delta = logsDragRef.current.startY - ev.clientY
                         setLogsDockHeight(Math.max(120, Math.min(window.innerHeight * 0.7, logsDragRef.current.startH + delta)))
                       }
                       const onUp = () => {
                         logsDragRef.current = null
                         document.removeEventListener('mousemove', onMove)
                         document.removeEventListener('mouseup', onUp)
                       }
                       document.addEventListener('mousemove', onMove)
                       document.addEventListener('mouseup', onUp)
                     }} />
              )}
              <button onClick={() => setLogsDockOpen(v => !v)}
                className="w-full h-8 px-3 flex items-center gap-2 bg-gray-900/80 hover:bg-gray-800 border-b border-gray-800 transition">
                <ChevronRight className={`w-3 h-3 text-gray-500 transition-transform ${logsDockOpen ? 'rotate-90' : ''}`} />
                <span className="text-[11px] font-semibold text-gray-300 uppercase tracking-wider">生产日志</span>
                <span className="text-[10px] text-gray-500">
                  · {((selectedNodeLive.data as unknown as EpisodeData).label) || selectedNodeLive.id}
                </span>
                <span className="ml-auto text-[10px] text-gray-600">
                  {(() => {
                    const ep = selectedNodeLive.data as unknown as EpisodeData
                    const takes = (ep.scenes || []).reduce((n, s) => n + (s.takes?.length || 0), 0)
                    return `${takes} takes`
                  })()}
                </span>
              </button>
              {logsDockOpen && (
                <div className="absolute top-8 left-0 right-0 bottom-0 overflow-y-auto">
                  <EpisodeLogsPane data={selectedNodeLive.data as unknown as EpisodeData} />
                </div>
              )}
            </div>
          )}

          {/* Edge Right-click Context Menu · 连线管理 */}
          {edgeContextMenu && (() => {
            const edge = edgeContextMenu.edge
            return (
              <div className="fixed bg-gray-800/95 backdrop-blur-xl rounded-lg shadow-2xl border border-gray-700/50 py-1 w-48 z-[100]"
                style={{ left: edgeContextMenu.x, top: edgeContextMenu.y }}
                onClick={(e) => e.stopPropagation()}>
                <div className="px-3 py-1 border-b border-gray-700/50 text-[10px] text-gray-500 uppercase tracking-wider truncate">
                  连线 · {edge.source.slice(0, 8)} → {edge.target.slice(0, 8)}
                </div>
                <button onClick={() => deleteEdge(edge.id)}
                  className="w-full flex items-center gap-2 px-3 py-1.5 hover:bg-red-900/40 transition-colors">
                  <Trash2 className="w-3.5 h-3.5 text-red-400" />
                  <span className="text-xs text-red-300">删除连线</span>
                </button>
                <div className="px-3 py-1 border-t border-gray-700/50 text-[10px] text-gray-600">
                  Ctrl+Z 可撤销
                </div>
              </div>
            )
          })()}

          {/* Node Right-click Context Menu · 节点管理（与连线 X 删除呼应） */}
          {nodeContextMenu && (() => {
            const node = nodeContextMenu.node
            const isProtected = node.type === 'start' || node.type === 'end'
            return (
              <div className="fixed bg-gray-800/95 backdrop-blur-xl rounded-lg shadow-2xl border border-gray-700/50 py-1 w-48 z-[100]"
                style={{ left: nodeContextMenu.x, top: nodeContextMenu.y }}
                onClick={(e) => e.stopPropagation()}>
                <div className="px-3 py-1 border-b border-gray-700/50 text-[10px] text-gray-500 uppercase tracking-wider truncate">
                  {node.type} · {node.id.slice(0, 16)}
                </div>
                <button onClick={() => { setSelectedNode(node); setNodeContextMenu(null) }}
                  className="w-full flex items-center gap-2 px-3 py-1.5 hover:bg-gray-700/60 transition-colors">
                  <Wrench className="w-3.5 h-3.5 text-cyan-400" />
                  <span className="text-xs text-gray-300">打开属性面板</span>
                </button>
                <button onClick={() => duplicateNode(node)}
                  disabled={isProtected}
                  className="w-full flex items-center gap-2 px-3 py-1.5 hover:bg-gray-700/60 disabled:opacity-40 disabled:cursor-not-allowed transition-colors">
                  <Plus className="w-3.5 h-3.5 text-emerald-400" />
                  <span className="text-xs text-gray-300">复制节点</span>
                </button>
                <button onClick={() => {
                    setSelectedNode(node)
                    setNodeContextMenu(null)
                    setTimeout(() => deleteSelectedNode(), 0)
                  }}
                  disabled={isProtected}
                  className="w-full flex items-center gap-2 px-3 py-1.5 hover:bg-red-900/40 disabled:opacity-40 disabled:cursor-not-allowed transition-colors">
                  <Trash2 className="w-3.5 h-3.5 text-red-400" />
                  <span className="text-xs text-red-300">{isProtected ? '删除（start/end 锁定）' : '删除节点'}</span>
                </button>
                <div className="px-3 py-1 border-t border-gray-700/50 text-[10px] text-gray-600">
                  提示：Delete 键直接删除已选节点
                </div>
              </div>
            )
          })()}
        </div>

        {/* Right Property Panel — episode gets the rich workflow panel (pt-14 avoids toolbar overlap) */}
        {selectedNodeLive && selectedNodeLive.type === 'media' && (selectedNodeLive.data as Record<string, unknown>).category === 'scene' ? (
          <div className="pt-14 h-full">
            <EpisodeWorkflowPanel
              node={selectedNodeLive}
              nodes={nodes}
              onUpdate={handleNodeDataUpdate}
              onClose={() => { setSelectedNode(null); setFocusedSceneId(null) }}
              onProduce={(ep) => setPreflightTarget({ episode: ep, nodeId: selectedNodeLive.id })}
              onFlushSave={() => { if (workflowId) setTimeout(() => { void doSaveRef.current?.() }, 300) }}
              onCancel={() => {
                cancelProductionRef.current = true
                showAppToast('warn', '正在停止生产…', '当前场景轮询结束后将停止，已完成镜头保留。', { durationMs: 5000 })
                // Reset composition status + mark all running takes as cancelled
                setNodes(nds => nds.map(n => {
                  if (n.id !== selectedNodeLive!.id) return n
                  const d = n.data as unknown as EpisodeData
                  const scenes = (d.scenes || []).map(s => ({
                    ...s,
                    takes: (s.takes || []).map(t =>
                      t.status === 'running' ? { ...t, status: 'failed' as const, note: '用户取消生产' } : t
                    ),
                  }))
                  return { ...n, data: { ...d, scenes, composition: { ...(d.composition || { picked_clips: [] }), status: 'pending' } } as unknown as Record<string, unknown> }
                }))
                // Flush save immediately so status persists across refresh
                if (workflowId) setTimeout(() => { void doSaveRef.current?.() }, 300)
              }}
              initialSceneId={focusedSceneId || undefined}
              initialTab={panelInitialTab || undefined}
              onConsumeInitialTab={() => setPanelInitialTab(null)}
            />
          </div>
        ) : selectedNodeLive ? (
          <div className="pt-14 h-full">
            <NodePropertyPanel
              node={selectedNodeLive}
              models={models}
              tools={availableTools}
              onUpdate={handleNodeDataUpdate}
              onClose={() => setSelectedNode(null)}
              onEditCharacter={(id) => setEditCharNodeId(id)}
              onEditProp={(id) => setEditPropNodeId(id)}
            />
          </div>
        ) : null}
      </div>

      {/* Floating App Toast（替代原生 alert · 右下角，自动消失） */}
      {appToast && (
        <div className="fixed bottom-4 right-4 z-[120] w-[min(460px,calc(100vw-32px))] animate-in slide-in-from-bottom-4 fade-in duration-200">
          <div className={`group flex items-start gap-3 p-3.5 rounded-xl backdrop-blur-xl border shadow-2xl ${
            appToast.kind === 'ok' ? 'bg-emerald-950/85 border-emerald-500/40 shadow-emerald-900/50'
            : appToast.kind === 'err' ? 'bg-red-950/85 border-red-500/40 shadow-red-900/50'
            : appToast.kind === 'warn' ? 'bg-amber-950/85 border-amber-500/40 shadow-amber-900/50'
            : 'bg-gray-900/90 border-gray-700/50 shadow-black/50'
          }`}>
            <div className={`flex-shrink-0 mt-0.5 ${
              appToast.kind === 'ok' ? 'text-emerald-400'
              : appToast.kind === 'err' ? 'text-red-400'
              : appToast.kind === 'warn' ? 'text-amber-400'
              : 'text-cyan-400'
            }`}>
              {appToast.kind === 'ok' ? <CheckCircle2 className="w-5 h-5" />
                : appToast.kind === 'err' ? <XCircle className="w-5 h-5" />
                : appToast.kind === 'warn' ? <AlertTriangle className="w-5 h-5" />
                : <Info className="w-5 h-5" />}
            </div>
            <div className="flex-1 min-w-0">
              <div className={`text-sm font-semibold leading-snug ${
                appToast.kind === 'ok' ? 'text-emerald-100'
                : appToast.kind === 'err' ? 'text-red-100'
                : appToast.kind === 'warn' ? 'text-amber-100'
                : 'text-gray-100'
              }`}>{appToast.title}</div>
              {appToast.body && (
                <div className={`mt-1 text-xs leading-relaxed whitespace-pre-line break-words ${
                  appToast.kind === 'ok' ? 'text-emerald-200/85'
                  : appToast.kind === 'err' ? 'text-red-200/85'
                  : appToast.kind === 'warn' ? 'text-amber-200/85'
                  : 'text-gray-300'
                }`}>{appToast.body}</div>
              )}
              {appToast.ctaLabel && appToast.onCta && (
                <button
                  onClick={() => { appToast.onCta?.(); setAppToast(null); if (appToastTimerRef.current) clearTimeout(appToastTimerRef.current) }}
                  className={`mt-2.5 inline-flex items-center gap-1.5 px-2.5 py-1 text-[11px] font-medium rounded-md border transition-colors ${
                    appToast.kind === 'err' ? 'bg-red-500/20 border-red-400/40 text-red-100 hover:bg-red-500/30'
                    : appToast.kind === 'warn' ? 'bg-amber-500/20 border-amber-400/40 text-amber-100 hover:bg-amber-500/30'
                    : 'bg-emerald-500/20 border-emerald-400/40 text-emerald-100 hover:bg-emerald-500/30'
                  }`}
                >{appToast.ctaLabel}</button>
              )}
            </div>
            <button
              onClick={() => { setAppToast(null); if (appToastTimerRef.current) clearTimeout(appToastTimerRef.current) }}
              className="flex-shrink-0 -mr-1 -mt-1 p-1 rounded-md text-gray-500 hover:text-white hover:bg-white/10 transition-colors"
              aria-label="关闭通知"
            >
              <X className="w-4 h-4" />
            </button>
          </div>
        </div>
      )}

      {/* Modals */}
      <CharacterCreatorModal
        open={showCharModal}
        existingTags={nodes
          .filter(n => n.type === 'media' && (n.data as Record<string, unknown>).category === 'character')
          .map(n => ((n.data as unknown as CharacterData).tag) || '')
          .filter(Boolean)}
        onClose={() => setShowCharModal(false)}
        onCreate={(data) => { addMediaNodeWithData(data as unknown as Record<string, unknown>) }}
      />

      {/* 道具编辑器 */}
      {editPropNodeId && (() => {
        const n = nodes.find(x => x.id === editPropNodeId)
        if (!n) return null
        const d = n.data as unknown as PropData
        return (
          <PropEditorModal
            open={true}
            initial={d}
            onClose={() => setEditPropNodeId(null)}
            onSave={(data: PropData) => {
              setNodes(nds => nds.map(x => x.id === editPropNodeId
                ? { ...x, data: { ...(x.data as Record<string, unknown>), ...(data as unknown as Record<string, unknown>) } }
                : x))
              setEditPropNodeId(null)
            }}
          />
        )
      })()}

      {/* 角色编辑向导（右侧属性面板点"打开角色工坊向导"触发） */}
      {editCharNodeId && (() => {
        const n = nodes.find(x => x.id === editCharNodeId)
        if (!n) return null
        const d = n.data as unknown as CharacterData
        return (
          <CharacterCreatorModal
            open={true}
            existingTags={[]}
            initial={d}
            onClose={() => setEditCharNodeId(null)}
            onCreate={(data) => {
              // 合并回原节点（保留 tag / 其他自定义字段）
              setNodes(nds => nds.map(x => x.id === editCharNodeId
                ? { ...x, data: { ...(x.data as Record<string, unknown>), ...(data as unknown as Record<string, unknown>), tag: d.tag || data.tag } }
                : x))
              setEditCharNodeId(null)
            }}
          />
        )
      })()}
      <EpisodeCreatorModal
        open={showEpModal}
        defaultSeason={epModalSeason}
        defaultSpinoffGroup={epModalSpinoffGroup}
        existingEpisodes={nodes
          .filter(n => n.type === 'media' && (n.data as Record<string, unknown>).category === 'scene')
          .map(n => n.data as unknown as EpisodeData)}
        onClose={() => setShowEpModal(false)}
        onCreate={(data) => { addMediaNodeWithData(data as unknown as Record<string, unknown>) }}
      />

      {/* 💰 成本分析 */}
      <CostAnalysisModal
        open={showCostModal}
        onClose={() => setShowCostModal(false)}
        nodes={nodes}
      />

      {/* 🛫 派单前自检（preflight）——点「开始生产 EPxx」时打开，通过后才真正派 Seedance */}
      {preflightTarget && (
        <PreflightModal
          episode={preflightTarget.episode}
          nodes={nodes}
          project="swarm-universe"
          onClose={() => setPreflightTarget(null)}
          onPatchNode={handleNodeDataUpdate}
          onProceed={() => {
            const t = preflightTarget
            setPreflightTarget(null)
            if (t) runEpisodeProduction(t.episode, t.nodeId)
          }}
          onRegenAll={() => {
            const t = preflightTarget
            setPreflightTarget(null)
            if (!t) return
            // Clear all picked_take → batch mode won't skip any scene
            const clearedScenes = (t.episode.scenes || []).map(s => ({ ...s, picked_take: undefined }))
            const clearedEp = { ...t.episode, scenes: clearedScenes, composition: { picked_clips: [] as string[], status: 'pending' as const } }
            // Write cleared state back to the node
            setNodes(nds => nds.map(n => n.id === t.nodeId
              ? { ...n, data: { ...(n.data as Record<string, unknown>), scenes: clearedScenes, composition: clearedEp.composition } }
              : n))
            runEpisodeProduction(clearedEp, t.nodeId)
          }}
        />
      )}

      {/* 📥 广告剧本导入 (.md → scenes) */}
      <ScriptImporterModal
        open={showImportModal}
        adType={importTargetType ?? undefined}
        onClose={() => { setShowImportModal(false); setImportTargetType(null) }}
        onImport={(data) => {
          if (importTargetType) addImportedAdScript(importTargetType.id, data)
          setShowImportModal(false)
          setImportTargetType(null)
        }}
      />

      {/* 🌌 宇宙总览 */}
      <UniverseOverviewModal
        open={showOverview}
        onClose={() => setShowOverview(false)}
        nodes={nodes}
        onFocusEpisode={focusEpisodeFromOverview}
      />

      {/* 📋 物料对齐 · 全集扫描 */}
      <AssetCoverageModal
        open={showAssetCoverage}
        onClose={() => setShowAssetCoverage(false)}
        nodes={nodes}
        onPatchNode={handleNodeDataUpdate}
      />

      {/* 存档 / 快照管理 */}
      <SnapshotsModal
        open={showSnapshots}
        onClose={() => setShowSnapshots(false)}
        current={{
          nodes, edges,
          workflowName,
          counter: nodeIdCounter.current,
          workflowId: workflowId || null,
        }}
        onRestore={restoreSnapshot}
      />

      {/* Swarm Universe 确认对话框 */}
      {showSwarmConfirm && (
        <div className="fixed inset-0 z-[200] flex items-center justify-center bg-black/70 backdrop-blur-sm animate-in fade-in duration-150"
          onClick={() => setShowSwarmConfirm(false)}>
          <div className="relative w-[480px] max-w-[92vw] rounded-2xl border border-violet-500/30 bg-gradient-to-br from-gray-900 via-gray-900 to-violet-950/40 shadow-2xl shadow-violet-900/40 overflow-hidden"
            onClick={e => e.stopPropagation()}>
            {/* 顶部装饰渐变条 */}
            <div className="h-1 bg-gradient-to-r from-violet-500 via-cyan-400 to-violet-500" />
            {/* Header */}
            <div className="px-6 pt-5 pb-3 flex items-start gap-3">
              <div className="flex-shrink-0 w-10 h-10 rounded-xl bg-gradient-to-br from-violet-500 to-cyan-500 flex items-center justify-center shadow-lg shadow-violet-900/40">
                <Sparkles className="w-5 h-5 text-white" />
              </div>
              <div className="flex-1 min-w-0">
                <h3 className="text-base font-semibold text-white">加载 虫群宇宙 完整资产</h3>
                <p className="text-xs text-gray-400 mt-0.5">一键铺设角色库 · 剧集骨架 · 道具 · 成片 takes</p>
              </div>
            </div>
            {/* Body: 统计卡片 */}
            <div className="px-6 pb-4 grid grid-cols-2 gap-2">
              <StatCard icon={Users} label="角色" value="5" hint="林见月·ZERG·苏蜜·颜术·温婉" color="from-cyan-500/20 to-cyan-500/5 border-cyan-500/30 text-cyan-300" />
              <StatCard icon={Package} label="道具" value="7" hint="古铜钱·半块饼·手机·吹风机…" color="from-amber-500/20 to-amber-500/5 border-amber-500/30 text-amber-300" />
              <StatCard icon={Film} label="正片" value="50" hint="五季×10集 · EP01-04 已成片" color="from-emerald-500/20 to-emerald-500/5 border-emerald-500/30 text-emerald-300" />
              <StatCard icon={Clapperboard} label="衍生剧" value="8" hint="《道裂》前传 + MCU 外传" color="from-violet-500/20 to-violet-500/5 border-violet-500/30 text-violet-300" />
            </div>
            {/* 警告条 */}
            <div className="mx-6 mb-4 px-3 py-2 rounded-lg bg-amber-500/10 border border-amber-500/30 flex items-start gap-2">
              <div className="w-1.5 h-1.5 rounded-full bg-amber-400 mt-1.5 flex-shrink-0 animate-pulse" />
              <p className="text-[11px] text-amber-200/90 leading-relaxed">
                将<span className="font-semibold text-amber-100">清除画布上现有</span>的角色 / 剧集 / 道具节点后重新加载，
                <span className="text-gray-400">保留 start / end / LLM / tool 节点</span>。
              </p>
            </div>
            {/* Footer 按钮 */}
            <div className="px-6 py-4 bg-gray-950/60 border-t border-gray-800 flex items-center justify-end gap-2">
              <button onClick={() => setShowSwarmConfirm(false)}
                className="px-4 py-2 text-xs font-medium rounded-lg bg-gray-800 border border-gray-700 text-gray-300 hover:bg-gray-700 hover:text-white transition">
                取消
              </button>
              <button onClick={doLoadSwarmUniverse} disabled={loadingSwarm}
                className="px-4 py-2 text-xs font-semibold rounded-lg bg-gradient-to-r from-violet-600 to-cyan-600 text-white hover:from-violet-500 hover:to-cyan-500 disabled:opacity-60 disabled:cursor-not-allowed transition shadow-lg shadow-violet-900/40 flex items-center gap-1.5">
                {loadingSwarm ? (<><Loader2 className="w-3.5 h-3.5 animate-spin" /> 加载中…</>) : (<><Sparkles className="w-3.5 h-3.5" /> 加载虫群宇宙</>)}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}

function StatCard({ icon: Icon, label, value, hint, color }: {
  icon: typeof Users
  label: string
  value: string
  hint: string
  color: string
}) {
  return (
    <div className={`rounded-lg border bg-gradient-to-br ${color} px-3 py-2.5`}>
      <div className="flex items-center gap-2 mb-1">
        <Icon className="w-3.5 h-3.5" />
        <span className="text-[10px] font-medium uppercase tracking-wider opacity-80">{label}</span>
        <span className="ml-auto text-lg font-bold leading-none">{value}</span>
      </div>
      <p className="text-[10px] text-gray-400 leading-snug truncate" title={hint}>{hint}</p>
    </div>
  )
}
