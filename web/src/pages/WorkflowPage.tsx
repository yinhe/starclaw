import { useState, useCallback, useRef, useEffect } from 'react'
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
  BackgroundVariant,
  Panel,
} from '@xyflow/react'
import '@xyflow/react/dist/style.css'
import { Save, Play, Plus, Loader2, Maximize2, Minimize2, Cpu, Wrench, GitBranch, Image, Trash2, ArrowLeft, PanelLeftClose, PanelLeftOpen, Users, Film, Package, FileText, ChevronDown, ChevronRight, Clapperboard, Sparkles } from 'lucide-react'
import { useSearchParams, useNavigate } from 'react-router-dom'
import LLMNode from '../components/workflow/LLMNode'
import ToolNode from '../components/workflow/ToolNode'
import ConditionNode from '../components/workflow/ConditionNode'
import StartNode from '../components/workflow/StartNode'
import EndNode from '../components/workflow/EndNode'
import MediaNode from '../components/workflow/MediaNode'
import NodePropertyPanel from '../components/workflow/NodePropertyPanel'
import EpisodeWorkflowPanel from '../components/workflow/EpisodeWorkflowPanel'
import CharacterCreatorModal from '../components/workflow/CharacterCreatorModal'
import EpisodeCreatorModal from '../components/workflow/EpisodeCreatorModal'
import { SEASONS, SPINOFF_GROUPS, type EpisodeData, type CharacterData } from '../components/workflow/episodeTypes'
import { buildSeedNodes } from '../components/workflow/swarmUniverseSeed'
import { modelAPI, toolAPI, workflowAPI } from '../lib/api'

const nodeTypes = {
  llm: LLMNode,
  tool: ToolNode,
  condition: ConditionNode,
  start: StartNode,
  end: EndNode,
  media: MediaNode,
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
  const [workflowName, setWorkflowName] = useState('未命名工作流')
  const [saving, setSaving] = useState(false)
  const [running, setRunning] = useState(false)
  const [isFullscreen, setIsFullscreen] = useState(false)
  const [showPalette, setShowPalette] = useState(false)
  const [contextMenu, setContextMenu] = useState<{ x: number; y: number } | null>(null)
  const [models, setModels] = useState<{ id: string; provider: string; model_name: string; display_name: string }[]>([])
  const [availableTools, setAvailableTools] = useState<string[]>([])
  const [showLeftPanel, setShowLeftPanel] = useState(true)
  const [expandedSections, setExpandedSections] = useState<Record<string, boolean>>({ characters: true, episodes: true, props: true, pipeline: false, spinoff: true, 's1': true, 's2': false, 's3': false, 's4': false, 's5': false })
  const [showCharModal, setShowCharModal] = useState(false)
  const [showEpModal, setShowEpModal] = useState(false)
  const [epModalSeason, setEpModalSeason] = useState<number>(1)
  const [epModalSpinoffGroup, setEpModalSpinoffGroup] = useState<string | undefined>(undefined)
  const containerRef = useRef<HTMLDivElement>(null)
  const nodeIdCounter = useRef(100)

  useEffect(() => {
    loadModels()
    loadTools()
    if (workflowId) loadWorkflow(workflowId)
  }, [workflowId])

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
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [selectedNode])

  const toggleFullscreen = () => {
    if (!document.fullscreenElement) {
      containerRef.current?.requestFullscreen()
    } else {
      document.exitFullscreen()
    }
  }

  const deleteSelectedNode = () => {
    if (!selectedNode) return
    if (selectedNode.type === 'start' || selectedNode.type === 'end') return
    setNodes(nds => nds.filter(n => n.id !== selectedNode.id))
    setEdges(eds => eds.filter(e => e.source !== selectedNode.id && e.target !== selectedNode.id))
    setSelectedNode(null)
  }

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
        const def = typeof wf.definition === 'string' ? JSON.parse(wf.definition) : wf.definition
        if (def?.nodes) setNodes(def.nodes)
        if (def?.edges) setEdges(def.edges)
      }
    } catch { /* */ }
  }

  const onConnect = useCallback(
    (params: Connection) => setEdges(eds => addEdge({ ...params, animated: true, style: { stroke: '#475569', strokeWidth: 1.5 } }, eds)),
    [setEdges],
  )

  const onNodeClick: NodeMouseHandler = useCallback((_e, node) => {
    setSelectedNode(node)
    setContextMenu(null)
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

  const loadSwarmUniverse = useCallback(() => {
    if (!confirm('将加载虫群宇宙完整资产：\n\n• 5 个角色（林见月·ZERG·苏蜜·颜术·温婉）\n• 7 个道具（古铜钱·半块饼·手机·吹风机等）\n• 50 集正片（五季×10集·EP01-EP04 已含真实成片）\n• 8 集衍生剧（《道裂》前传 + MCU 外传）\n\n⚠ 将清除画布上现有的角色/剧集/道具节点后重新加载（保留 start/end/LLM/tool 节点）。')) return
    const { nodes: seed, nextId } = buildSeedNodes({ startIdCounter: nodeIdCounter.current })
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
  }, [setNodes, setEdges, workflowName])

  const runEpisodeProduction = useCallback(async (episodeData: EpisodeData, nodeId: string) => {
    // Mark episode as generating; downstream: call short-drama team workflow with episode-scoped input
    const payload = {
      episode_id: nodeId,
      episode_label: episodeData.label,
      season: episodeData.season,
      episode_number: episodeData.episode_number,
      scenes: episodeData.scenes || [],
      duration: episodeData.duration,
      description: episodeData.description,
    }

    // Optimistic UI: set composition.status = generating
    setNodes(nds => nds.map(n => {
      if (n.id !== nodeId) return n
      const d = n.data as unknown as EpisodeData
      return { ...n, data: { ...d, composition: { ...(d.composition || { picked_clips: [] }), status: 'generating', picked_clips: d.composition?.picked_clips || [] } } as unknown as Record<string, unknown> }
    }))

    try {
      if (workflowId) {
        const res = await workflowAPI.run(workflowId, { input: JSON.stringify(payload) })
        alert(`✅ 已提交 ${episodeData.label} 到短剧团队工作流\n\n返回：${JSON.stringify(res.data.output || res.data.result, null, 2).slice(0, 500)}`)
      } else {
        alert(`⚠️ 请先保存工作流，然后点“开始生产 ${episodeData.label}”将交给短剧团队。\n\n当前 payload 预览：\n${JSON.stringify(payload, null, 2).slice(0, 600)}`)
      }
    } catch (e) {
      alert(`生产提交失败：${(e as Error).message || e}`)
      // rollback status
      setNodes(nds => nds.map(n => {
        if (n.id !== nodeId) return n
        const d = n.data as unknown as EpisodeData
        return { ...n, data: { ...d, composition: { ...(d.composition || { picked_clips: [] }), status: 'pending', picked_clips: d.composition?.picked_clips || [] } } as unknown as Record<string, unknown> }
      }))
    }
  }, [setNodes, workflowId])

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

  const handleSave = async () => {
    setSaving(true)
    try {
      const definition = JSON.stringify({ nodes, edges })
      if (workflowId) {
        await workflowAPI.update(workflowId, { name: workflowName, definition })
      } else {
        const res = await workflowAPI.create({ name: workflowName, definition })
        if (res.data.workflow?.id) navigate(`/workflows?id=${res.data.workflow.id}`, { replace: true })
      }
    } catch { /* */ }
    setSaving(false)
  }

  const handleRun = async () => {
    setRunning(true)
    try {
      if (!workflowId) { alert('请先保存工作流'); setRunning(false); return }
      const res = await workflowAPI.run(workflowId, { input: '' })
      alert(`运行完成！\n输出: ${JSON.stringify(res.data.output || res.data.result, null, 2)}`)
    } catch { alert('运行失败') }
    setRunning(false)
  }

  const onPaneContextMenu = useCallback((e: MouseEvent | React.MouseEvent) => {
    e.preventDefault()
    setContextMenu({ x: (e as MouseEvent).clientX - 250, y: (e as MouseEvent).clientY - 100 })
  }, [])

  return (
    <div ref={containerRef} className="h-full flex flex-col bg-gray-950">
      {/* ── Floating Top Bar ── */}
      <div className="absolute top-3 left-3 right-3 z-50 flex items-center justify-between pointer-events-none">
        <div className="flex items-center gap-2 pointer-events-auto">
          <button onClick={() => navigate(-1)}
            className="p-2 rounded-lg bg-gray-800/80 backdrop-blur border border-gray-700/50 text-gray-400 hover:text-white hover:bg-gray-700/80 transition-all">
            <ArrowLeft className="w-4 h-4" />
          </button>
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
          <button onClick={loadSwarmUniverse}
            title="一键加载虫群宇宙完整资产 (5角色 + 7道具 + 50集 + 衍生剧)"
            className="flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium rounded-lg bg-gradient-to-r from-violet-600/90 to-cyan-600/90 backdrop-blur border border-violet-500/50 text-white hover:from-violet-500 hover:to-cyan-500 transition-all shadow-lg shadow-violet-900/30">
            <Sparkles className="w-3.5 h-3.5" /> 虫群宇宙
          </button>
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
          <button onClick={handleSave} disabled={saving}
            className="flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium rounded-lg bg-gray-800/80 backdrop-blur border border-gray-700/50 text-gray-300 hover:text-white hover:bg-gray-700/80 disabled:opacity-50 transition-all">
            {saving ? <Loader2 className="w-3.5 h-3.5 animate-spin" /> : <Save className="w-3.5 h-3.5" />} 保存
          </button>
          <button onClick={handleRun} disabled={running}
            className="flex items-center gap-1.5 px-4 py-2 text-xs font-bold rounded-lg bg-emerald-600 backdrop-blur border border-emerald-500/50 text-white hover:bg-emerald-500 disabled:opacity-50 transition-all shadow-lg shadow-emerald-900/30">
            {running ? <Loader2 className="w-4 h-4 animate-spin" /> : <Play className="w-4 h-4" />} 开始生产
          </button>
          <div className="w-px h-6 bg-gray-700/50 mx-1" />
          <button onClick={toggleFullscreen}
            className="p-2 rounded-lg bg-gray-800/80 backdrop-blur border border-gray-700/50 text-gray-400 hover:text-white hover:bg-gray-700/80 transition-all">
            {isFullscreen ? <Minimize2 className="w-4 h-4" /> : <Maximize2 className="w-4 h-4" />}
          </button>
        </div>
      </div>

      {/* ── Canvas + Left Panel ── */}
      <div className="flex-1 flex overflow-hidden">
        {/* Left Asset Panel */}
        {showLeftPanel && (
          <div className="w-56 bg-gray-900 border-r border-gray-800 flex flex-col overflow-hidden z-30">
            <div className="px-3 py-2.5 pt-14 border-b border-gray-800 flex items-center justify-between">
              <span className="text-[11px] font-semibold text-gray-400 uppercase tracking-wider">项目资产</span>
              <button onClick={() => setShowLeftPanel(false)} className="p-1 text-gray-600 hover:text-gray-400 transition-colors">
                <PanelLeftClose className="w-3.5 h-3.5" />
              </button>
            </div>
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
                    <button key={n.id} onClick={() => setSelectedNode(n)}
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

        {/* Left panel toggle when hidden */}
        {!showLeftPanel && (
          <button onClick={() => setShowLeftPanel(true)}
            className="absolute left-3 top-14 z-40 p-1.5 rounded-lg bg-gray-800/80 backdrop-blur border border-gray-700/50 text-gray-500 hover:text-white transition-colors">
            <PanelLeftOpen className="w-4 h-4" />
          </button>
        )}

        <div className="flex-1 relative" style={{ touchAction: 'none' }}>
          <ReactFlow
            nodes={nodes}
            edges={edges}
            onNodesChange={onNodesChange}
            onEdgesChange={onEdgesChange}
            onConnect={onConnect}
            onNodeClick={onNodeClick}
            onPaneClick={() => { setSelectedNode(null); setContextMenu(null) }}
            onPaneContextMenu={onPaneContextMenu}
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

            {/* Right-click Context Menu */}
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
        </div>

        {/* Right Property Panel — episode gets the rich workflow panel (pt-14 avoids toolbar overlap) */}
        {selectedNode && selectedNode.type === 'media' && (selectedNode.data as Record<string, unknown>).category === 'scene' ? (
          <div className="pt-14 h-full">
            <EpisodeWorkflowPanel
              node={selectedNode}
              onUpdate={handleNodeDataUpdate}
              onClose={() => setSelectedNode(null)}
              onProduce={(ep) => runEpisodeProduction(ep, selectedNode.id)}
            />
          </div>
        ) : selectedNode ? (
          <div className="pt-14 h-full">
            <NodePropertyPanel
              node={selectedNode}
              models={models}
              tools={availableTools}
              onUpdate={handleNodeDataUpdate}
              onClose={() => setSelectedNode(null)}
            />
          </div>
        ) : null}
      </div>

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
    </div>
  )
}
