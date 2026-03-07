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
import { Save, Play, Plus, Loader2 } from 'lucide-react'
import { useSearchParams, useNavigate } from 'react-router-dom'
import LLMNode from '../components/workflow/LLMNode'
import ToolNode from '../components/workflow/ToolNode'
import ConditionNode from '../components/workflow/ConditionNode'
import StartNode from '../components/workflow/StartNode'
import EndNode from '../components/workflow/EndNode'
import NodePropertyPanel from '../components/workflow/NodePropertyPanel'
import { modelAPI, toolAPI, workflowAPI } from '../lib/api'

const nodeTypes = {
  llm: LLMNode,
  tool: ToolNode,
  condition: ConditionNode,
  start: StartNode,
  end: EndNode,
}

const initialNodes: Node[] = [
  {
    id: 'start-1',
    type: 'start',
    position: { x: 250, y: 50 },
    data: { label: '开始' },
  },
  {
    id: 'end-1',
    type: 'end',
    position: { x: 250, y: 400 },
    data: { label: '结束' },
  },
]

const initialEdges: Edge[] = []

const NODE_TEMPLATES = [
  { type: 'llm', label: 'LLM 节点', desc: '调用大语言模型' },
  { type: 'tool', label: '工具节点', desc: '调用外部工具' },
  { type: 'condition', label: '条件分支', desc: '根据条件走不同路径' },
]

export default function WorkflowPage() {
  const [searchParams] = useSearchParams()
  const navigate = useNavigate()
  const workflowId = searchParams.get('id')

  const [nodes, setNodes, onNodesChange] = useNodesState(initialNodes)
  const [edges, setEdges, onEdgesChange] = useEdgesState(initialEdges)
  const [showNodePanel, setShowNodePanel] = useState(false)
  const [selectedNode, setSelectedNode] = useState<Node | null>(null)
  const [workflowName, setWorkflowName] = useState('未命名工作流')
  const [saving, setSaving] = useState(false)
  const [running, setRunning] = useState(false)
  const [models, setModels] = useState<{ id: string; provider: string; model_name: string; display_name: string }[]>([])
  const [availableTools, setAvailableTools] = useState<string[]>([])
  const nodeIdCounter = useRef(2)

  useEffect(() => {
    loadModels()
    loadTools()
    if (workflowId) loadWorkflow(workflowId)
  }, [workflowId])

  const loadModels = async () => {
    try {
      const res = await modelAPI.list()
      setModels(res.data.models || [])
    } catch { /* ignore */ }
  }

  const loadTools = async () => {
    try {
      const res = await toolAPI.list()
      setAvailableTools(res.data.tools || [])
    } catch { /* ignore */ }
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
    } catch { /* ignore */ }
  }

  const onConnect = useCallback(
    (params: Connection) => setEdges((eds) => addEdge({ ...params, animated: true, style: { stroke: '#94a3b8' } }, eds)),
    [setEdges],
  )

  const onNodeClick: NodeMouseHandler = useCallback((_event, node) => {
    setSelectedNode(node)
  }, [])

  const handleNodeDataUpdate = useCallback(
    (id: string, data: Record<string, unknown>) => {
      setNodes((nds) =>
        nds.map((n) => (n.id === id ? { ...n, data: { ...n.data, ...data } } : n)),
      )
    },
    [setNodes],
  )

  const addNode = useCallback(
    (type: string, label: string) => {
      const id = `${type}-${nodeIdCounter.current++}`
      const newNode: Node = {
        id,
        type,
        position: { x: 250 + Math.random() * 100, y: 150 + Math.random() * 150 },
        data: {
          label,
          ...(type === 'llm' ? { model: '', prompt: '', temperature: 0.7, maxTokens: 4096 } : {}),
          ...(type === 'tool' ? { toolName: '', argsTemplate: '' } : {}),
          ...(type === 'condition' ? { expression: '' } : {}),
        },
      }
      setNodes((nds) => [...nds, newNode])
      setShowNodePanel(false)
    },
    [setNodes],
  )

  const handleSave = async () => {
    setSaving(true)
    try {
      const definition = JSON.stringify({ nodes, edges })
      if (workflowId) {
        await workflowAPI.update(workflowId, { name: workflowName, definition })
      } else {
        const res = await workflowAPI.create({ name: workflowName, definition })
        if (res.data.workflow?.id) {
          navigate(`/workflows?id=${res.data.workflow.id}`, { replace: true })
        }
      }
    } catch { /* ignore */ }
    setSaving(false)
  }

  const handleRun = async () => {
    setRunning(true)
    try {
      const id = workflowId
      if (!id) {
        alert('请先保存工作流')
        setRunning(false)
        return
      }
      const res = await workflowAPI.run(id, { input: '' })
      console.log('Workflow result:', res.data)
      alert(`运行完成！\n输出: ${JSON.stringify(res.data.output || res.data.result, null, 2)}`)
    } catch (e) {
      console.error(e)
      alert('运行失败')
    }
    setRunning(false)
  }

  return (
    <div className="h-full flex flex-col">
      {/* Header */}
      <div className="px-4 py-2.5 border-b bg-white flex items-center justify-between">
        <div className="flex items-center gap-3">
          <input
            value={workflowName}
            onChange={(e) => setWorkflowName(e.target.value)}
            className="text-sm font-semibold text-gray-800 bg-transparent border-b border-transparent hover:border-gray-300 focus:border-primary-500 outline-none px-1 py-0.5 transition-colors"
          />
        </div>
        <div className="flex items-center gap-2">
          <button
            onClick={() => setShowNodePanel(!showNodePanel)}
            className="flex items-center gap-1.5 px-3 py-1.5 text-sm border rounded-lg hover:bg-gray-50 transition-colors"
          >
            <Plus className="w-4 h-4" />
            添加节点
          </button>
          <button
            onClick={handleSave}
            disabled={saving}
            className="flex items-center gap-1.5 px-3 py-1.5 text-sm border rounded-lg hover:bg-gray-50 disabled:opacity-50 transition-colors"
          >
            {saving ? <Loader2 className="w-4 h-4 animate-spin" /> : <Save className="w-4 h-4" />}
            保存
          </button>
          <button
            onClick={handleRun}
            disabled={running}
            className="flex items-center gap-1.5 px-3 py-1.5 text-sm bg-primary-600 text-white rounded-lg hover:bg-primary-700 disabled:opacity-50 transition-colors"
          >
            {running ? <Loader2 className="w-4 h-4 animate-spin" /> : <Play className="w-4 h-4" />}
            运行
          </button>
        </div>
      </div>

      {/* Canvas + Property Panel */}
      <div className="flex-1 flex overflow-hidden">
        <div className="flex-1 relative">
          <ReactFlow
            nodes={nodes}
            edges={edges}
            onNodesChange={onNodesChange}
            onEdgesChange={onEdgesChange}
            onConnect={onConnect}
            onNodeClick={onNodeClick}
            onPaneClick={() => setSelectedNode(null)}
            nodeTypes={nodeTypes}
            fitView
            className="bg-gray-50"
          >
            <Controls position="bottom-left" />
            <MiniMap
              position="bottom-right"
              nodeStrokeColor="#94a3b8"
              nodeColor="#fff"
              maskColor="rgba(0,0,0,0.08)"
            />
            <Background variant={BackgroundVariant.Dots} gap={20} size={1} color="#d1d5db" />

            {showNodePanel && (
              <Panel position="top-left">
                <div className="bg-white rounded-xl shadow-lg border p-4 w-56">
                  <p className="text-xs font-medium text-gray-500 mb-3">选择节点类型</p>
                  <div className="space-y-2">
                    {NODE_TEMPLATES.map((tpl) => (
                      <button
                        key={tpl.type}
                        onClick={() => addNode(tpl.type, tpl.label)}
                        className="w-full text-left p-2.5 rounded-lg border hover:border-primary-300 hover:bg-primary-50 transition-colors"
                      >
                        <span className="text-sm font-medium text-gray-800">{tpl.label}</span>
                        <p className="text-xs text-gray-400 mt-0.5">{tpl.desc}</p>
                      </button>
                    ))}
                  </div>
                </div>
              </Panel>
            )}
          </ReactFlow>
        </div>

        {/* Right panel */}
        {selectedNode && (
          <NodePropertyPanel
            node={selectedNode}
            models={models}
            tools={availableTools}
            onUpdate={handleNodeDataUpdate}
            onClose={() => setSelectedNode(null)}
          />
        )}
      </div>
    </div>
  )
}
