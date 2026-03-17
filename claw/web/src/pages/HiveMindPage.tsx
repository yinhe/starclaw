import { useState, useEffect, useRef, useMemo, useCallback, Component, type ReactNode } from 'react'
import { Canvas, useFrame } from '@react-three/fiber'
import { OrbitControls, Text, Html, Line } from '@react-three/drei'
import * as THREE from 'three'
import { squadAPI } from '../lib/api'
import { starclawWS } from '../lib/websocket'
import { Eye, LayoutGrid, Clock, ArrowLeft, RefreshCw, GitBranch, Wifi, MessageSquare, Send } from 'lucide-react'

// Lazy-load postprocessing to handle version incompatibility gracefully
let EffectComposer: any = null
let Bloom: any = null
try {
  const pp = require('@react-three/postprocessing')
  EffectComposer = pp.EffectComposer
  Bloom = pp.Bloom
} catch { /* postprocessing unavailable */ }

// Error boundary for 3D canvas crashes
class CanvasErrorBoundary extends Component<{ children: ReactNode }, { hasError: boolean }> {
  state = { hasError: false }
  static getDerivedStateFromError() { return { hasError: true } }
  componentDidCatch(err: any) { console.error('[HiveMind] 3D render error:', err) }
  render() {
    if (this.state.hasError) {
      return (
        <div className="flex items-center justify-center h-full bg-black text-gray-400">
          <div className="text-center">
            <p className="text-lg mb-2">3D 渲染出错</p>
            <p className="text-sm text-gray-600">WebGL 或 postprocessing 不可用</p>
            <button onClick={() => this.setState({ hasError: false })} className="mt-4 px-4 py-2 bg-purple-600 text-white rounded-lg text-sm">重试</button>
          </div>
        </div>
      )
    }
    return this.props.children
  }
}

// ═══════════════════════════════════════════════
//  Types
// ═══════════════════════════════════════════════

interface NodeData {
  id: string
  name: string
  role: string
  port: number
  status: 'running' | 'idle' | 'waiting' | 'failed' | 'reviewing' | 'done'
  currentTask: string
  progress: number
  branch: string
  lastAction: string
}

interface SprintData {
  number: number
  goal: string
  status: string
  doneSteps: number
  totalSteps: number
}

interface MissionData {
  id: string
  title: string
  status: string
  currentSprint: number
  maxSprints: number
  totalSteps: number
  doneSteps: number
}


// ═══════════════════════════════════════════════
//  Color & Config
// ═══════════════════════════════════════════════

const STATUS_COLORS: Record<string, string> = {
  running: '#00ff88',
  idle: '#4488ff',
  waiting: '#ffaa00',
  failed: '#ff3344',
  reviewing: '#aa44ff',
  done: '#ffffff',
}

const FLOW_COLORS: Record<string, string> = {
  code: '#00ff88',
  test: '#ffcc00',
  deploy: '#4488ff',
  git_push: '#ffd700',
  error: '#ff3344',
}

const NODE_CONFIGS = [
  { name: 'claw-lead', role: 'Captain', port: 8087, building: 'Hatchery', pos: [0, 0, 0] as [number, number, number], scale: 1.8 },
  { name: 'claw-backend', role: 'Backend', port: 8081, building: 'Spawning Pool', pos: [-4, 0, -2] as [number, number, number], scale: 1.2 },
  { name: 'claw-frontend', role: 'Frontend', port: 8082, building: 'Evolution Chamber', pos: [4, 0, -2] as [number, number, number], scale: 1.2 },
  { name: 'claw-qa', role: 'QA', port: 8083, building: 'Spire', pos: [-3, 0, 3] as [number, number, number], scale: 1.0 },
  { name: 'claw-pm', role: 'PM', port: 8084, building: 'Hydralisk Den', pos: [3, 0, 3] as [number, number, number], scale: 1.0 },
  { name: 'claw-design', role: 'Design', port: 8085, building: 'Roach Warren', pos: [-5, 0, 2] as [number, number, number], scale: 1.0 },
  { name: 'claw-devops', role: 'DevOps', port: 8086, building: 'Nydus Network', pos: [5, 0, 2] as [number, number, number], scale: 1.0 },
]

// ═══════════════════════════════════════════════
//  3D Components
// ═══════════════════════════════════════════════

function ZergNode({ config, data, selected, onClick }: {
  config: typeof NODE_CONFIGS[0]
  data: NodeData | undefined
  selected: boolean
  onClick: () => void
}) {
  const meshRef = useRef<THREE.Mesh>(null!)
  const glowRef = useRef<THREE.PointLight>(null!)
  const status = data?.status || 'idle'
  const color = STATUS_COLORS[status] || STATUS_COLORS.idle
  const isCaptain = config.role === 'Captain'

  useFrame((state) => {
    if (!meshRef.current) return
    const t = state.clock.elapsedTime

    // Pulsing animation based on status
    const pulseSpeed = status === 'running' ? 3 : status === 'waiting' ? 1.5 : 0.5
    const pulseAmount = status === 'running' ? 0.15 : status === 'failed' ? 0.2 : 0.05
    const scale = config.scale * (1 + Math.sin(t * pulseSpeed) * pulseAmount)
    meshRef.current.scale.setScalar(scale)

    // Rotation for captain
    if (isCaptain) {
      meshRef.current.rotation.y = t * 0.2
    }

    // Glow intensity
    if (glowRef.current) {
      glowRef.current.intensity = status === 'running' ? 2 + Math.sin(t * 4) * 1 :
        status === 'failed' ? 3 + Math.sin(t * 8) * 2 : 0.5
    }
  })

  const geometry = useMemo(() => {
    if (isCaptain) return new THREE.DodecahedronGeometry(1, 1)
    switch (config.role) {
      case 'Backend': return new THREE.TorusKnotGeometry(0.6, 0.2, 64, 8)
      case 'Frontend': return new THREE.OctahedronGeometry(0.8, 0)
      case 'QA': return new THREE.ConeGeometry(0.6, 1.5, 6)
      case 'PM': return new THREE.TetrahedronGeometry(0.8, 0)
      case 'Design': return new THREE.IcosahedronGeometry(0.7, 0)
      case 'DevOps': return new THREE.TorusGeometry(0.6, 0.25, 8, 16)
      default: return new THREE.SphereGeometry(0.7, 16, 16)
    }
  }, [config.role, isCaptain])

  return (
    <group position={config.pos}>
      {/* Main mesh */}
      <mesh
        ref={meshRef}
        geometry={geometry}
        onClick={(e) => { e.stopPropagation(); onClick() }}
      >
        <meshStandardMaterial
          color={color}
          emissive={color}
          emissiveIntensity={status === 'running' ? 0.6 : 0.2}
          roughness={0.3}
          metalness={0.7}
          wireframe={status === 'failed'}
          transparent
          opacity={status === 'idle' ? 0.6 : 0.9}
        />
      </mesh>

      {/* Glow light */}
      <pointLight ref={glowRef} color={color} intensity={1} distance={5} />

      {/* Selection ring */}
      {selected && (
        <mesh rotation={[Math.PI / 2, 0, 0]} position={[0, -0.5, 0]}>
          <ringGeometry args={[1.5, 1.7, 32]} />
          <meshBasicMaterial color="#ffffff" transparent opacity={0.6} side={THREE.DoubleSide} />
        </mesh>
      )}

      {/* Label */}
      <Text
        position={[0, isCaptain ? 2.2 : 1.6, 0]}
        fontSize={0.3}
        color="#ffffff"
        anchorX="center"
        anchorY="bottom"
        outlineWidth={0.02}
        outlineColor="#000000"
      >
        {config.building}
      </Text>
      <Text
        position={[0, isCaptain ? 1.85 : 1.25, 0]}
        fontSize={0.2}
        color={color}
        anchorX="center"
        anchorY="bottom"
      >
        {config.name} :{config.port}
      </Text>

      {/* Status badge */}
      {data && (
        <Html position={[0, isCaptain ? 2.8 : 2.2, 0]} center distanceFactor={10}>
          <div className="text-xs px-2 py-0.5 rounded-full whitespace-nowrap font-mono"
            style={{ backgroundColor: color + '33', color, border: `1px solid ${color}` }}>
            {status === 'running' ? `▶ ${data.progress}%` : status.toUpperCase()}
          </div>
        </Html>
      )}
    </group>
  )
}

// ═══════════════════════════════════════════════
//  Event-driven animations (20.8.9)
// ═══════════════════════════════════════════════

interface BurstEvent {
  id: number
  position: [number, number, number]
  color: string
  count: number
  startTime: number
  duration: number
}

let burstIdCounter = 0

function BurstParticles({ burst }: { burst: BurstEvent }) {
  const meshRef = useRef<THREE.InstancedMesh>(null!)
  const dummy = useMemo(() => new THREE.Object3D(), [])
  const velocities = useMemo(() =>
    Array.from({ length: burst.count }, () => ({
      vx: (Math.random() - 0.5) * 4,
      vy: Math.random() * 3 + 1,
      vz: (Math.random() - 0.5) * 4,
    })),
  [burst.count])

  useFrame((state) => {
    if (!meshRef.current) return
    const elapsed = state.clock.elapsedTime - burst.startTime
    const progress = elapsed / burst.duration
    if (progress > 1) {
      meshRef.current.visible = false
      return
    }
    const fade = 1 - progress
    velocities.forEach((v, i) => {
      dummy.position.set(
        burst.position[0] + v.vx * elapsed,
        burst.position[1] + v.vy * elapsed - 2 * elapsed * elapsed,
        burst.position[2] + v.vz * elapsed,
      )
      dummy.scale.setScalar(0.06 * fade)
      dummy.updateMatrix()
      meshRef.current.setMatrixAt(i, dummy.matrix)
    })
    meshRef.current.instanceMatrix.needsUpdate = true
  })

  return (
    <instancedMesh ref={meshRef} args={[undefined, undefined, burst.count]}>
      <sphereGeometry args={[1, 4, 4]} />
      <meshBasicMaterial color={burst.color} transparent opacity={0.9} />
    </instancedMesh>
  )
}

function EventEffects({ bursts }: { bursts: BurstEvent[] }) {
  return (
    <>
      {bursts.map(b => (
        <BurstParticles key={b.id} burst={b} />
      ))}
    </>
  )
}

function FlowParticle({ from, to, color, speed, offset }: {
  from: [number, number, number]
  to: [number, number, number]
  color: string
  speed: number
  offset: number
}) {
  const meshRef = useRef<THREE.Mesh>(null!)

  useFrame((state) => {
    if (!meshRef.current) return
    const t = ((state.clock.elapsedTime * speed + offset) % 1)
    meshRef.current.position.set(
      from[0] + (to[0] - from[0]) * t,
      0.2 + Math.sin(t * Math.PI) * 0.5,
      from[2] + (to[2] - from[2]) * t,
    )
    meshRef.current.scale.setScalar(0.06 + Math.sin(t * Math.PI) * 0.04)
  })

  return (
    <mesh ref={meshRef}>
      <sphereGeometry args={[1, 6, 6]} />
      <meshBasicMaterial color={color} transparent opacity={0.9} />
    </mesh>
  )
}

function NydusCanal({ from, to, active, flowType }: {
  from: [number, number, number]
  to: [number, number, number]
  active: boolean
  flowType?: string
}) {
  const color = flowType ? (FLOW_COLORS[flowType] || '#4488ff') : '#334466'

  const midY = 0.3
  const midPoint: [number, number, number] = [
    (from[0] + to[0]) / 2,
    midY,
    (from[2] + to[2]) / 2,
  ]

  return (
    <group>
      <Line
        points={[from, midPoint, to]}
        color={active ? color : '#1a2233'}
        lineWidth={active ? 2.5 : 0.5}
        opacity={active ? 0.7 : 0.15}
        transparent
      />
      {/* Animated flow particles when active */}
      {active && (
        <>
          <FlowParticle from={from} to={to} color={color} speed={0.3} offset={0} />
          <FlowParticle from={from} to={to} color={color} speed={0.3} offset={0.33} />
          <FlowParticle from={from} to={to} color={color} speed={0.3} offset={0.66} />
        </>
      )}
    </group>
  )
}

function CreepGround({ coverage }: { coverage: number }) {
  const meshRef = useRef<THREE.Mesh>(null!)
  const ringRef = useRef<THREE.Mesh>(null!)

  useFrame((state) => {
    if (!meshRef.current) return
    const t = state.clock.elapsedTime
    const material = meshRef.current.material as THREE.MeshStandardMaterial
    material.emissiveIntensity = 0.12 + Math.sin(t * 0.5) * 0.06

    // Animate creep radius expansion
    const targetRadius = 3 + coverage * 7
    const currentScale = meshRef.current.scale.x
    meshRef.current.scale.setScalar(currentScale + (targetRadius / 10 - currentScale) * 0.02)

    // Pulse the outer ring
    if (ringRef.current) {
      const ringMat = ringRef.current.material as THREE.MeshBasicMaterial
      ringMat.opacity = 0.15 + Math.sin(t * 2) * 0.1
      ringRef.current.scale.setScalar(currentScale * 1.05 + Math.sin(t) * 0.1)
    }
  })

  return (
    <group>
      {/* Main creep */}
      <mesh ref={meshRef} rotation={[-Math.PI / 2, 0, 0]} position={[0, -0.5, 0]} scale={1}>
        <circleGeometry args={[10, 64]} />
        <meshStandardMaterial
          color="#1a0a2e"
          emissive="#6a1a8e"
          emissiveIntensity={0.12}
          roughness={0.9}
          metalness={0.1}
          transparent
          opacity={0.7}
        />
      </mesh>
      {/* Outer pulse ring */}
      <mesh ref={ringRef} rotation={[-Math.PI / 2, 0, 0]} position={[0, -0.49, 0]} scale={1}>
        <ringGeometry args={[9.5, 10, 64]} />
        <meshBasicMaterial color="#aa44ff" transparent opacity={0.2} side={THREE.DoubleSide} />
      </mesh>
    </group>
  )
}

function FloatingParticles({ count = 200 }: { count?: number }) {
  const meshRef = useRef<THREE.InstancedMesh>(null!)
  const dummy = useMemo(() => new THREE.Object3D(), [])
  const speeds = useMemo(() => Array.from({ length: count }, () => ({
    x: (Math.random() - 0.5) * 12,
    y: Math.random() * 3 + 0.5,
    z: (Math.random() - 0.5) * 12,
    speed: Math.random() * 0.5 + 0.2,
    offset: Math.random() * Math.PI * 2,
  })), [count])

  useFrame((state) => {
    if (!meshRef.current) return
    const t = state.clock.elapsedTime
    speeds.forEach((p, i) => {
      dummy.position.set(
        p.x + Math.sin(t * p.speed + p.offset) * 0.5,
        p.y + Math.sin(t * p.speed * 0.7 + p.offset) * 0.3,
        p.z + Math.cos(t * p.speed + p.offset) * 0.5
      )
      dummy.scale.setScalar(0.02 + Math.sin(t * 2 + p.offset) * 0.01)
      dummy.updateMatrix()
      meshRef.current.setMatrixAt(i, dummy.matrix)
    })
    meshRef.current.instanceMatrix.needsUpdate = true
  })

  return (
    <instancedMesh ref={meshRef} args={[undefined, undefined, count]}>
      <sphereGeometry args={[1, 4, 4]} />
      <meshBasicMaterial color="#8844ff" transparent opacity={0.6} />
    </instancedMesh>
  )
}

function HiveScene({ nodes, sprint, selectedNode, onSelectNode, bursts }: {
  nodes: NodeData[]
  sprint: SprintData | null
  selectedNode: string | null
  onSelectNode: (name: string | null) => void
  bursts: BurstEvent[]
}) {
  const coverage = sprint ? (sprint.totalSteps > 0 ? sprint.doneSteps / sprint.totalSteps : 0) : 0

  // Generate canals between captain and all other nodes
  const captainPos = NODE_CONFIGS[0].pos
  const nodeMap = new Map(nodes.map(n => [n.name, n]))

  return (
    <>
      {/* Lighting */}
      <ambientLight intensity={0.15} />
      <directionalLight position={[10, 15, 10]} intensity={0.4} color="#8888cc" />
      <pointLight position={[0, 8, 0]} intensity={0.3} color="#6644aa" />

      {/* Creep ground */}
      <CreepGround coverage={coverage} />

      {/* Grid floor */}
      <gridHelper args={[20, 20, '#1a1a3a', '#111128']} position={[0, -0.49, 0]} />

      {/* Nydus canals (lines from captain to each node) */}
      {NODE_CONFIGS.slice(1).map((cfg) => {
        const nodeData = nodeMap.get(cfg.name)
        const active = nodeData?.status === 'running' || nodeData?.status === 'reviewing'
        return (
          <NydusCanal
            key={cfg.name}
            from={captainPos}
            to={cfg.pos}
            active={active}
            flowType={active ? 'code' : undefined}
          />
        )
      })}

      {/* Zerg nodes */}
      {NODE_CONFIGS.map((cfg) => (
        <ZergNode
          key={cfg.name}
          config={cfg}
          data={nodeMap.get(cfg.name)}
          selected={selectedNode === cfg.name}
          onClick={() => onSelectNode(selectedNode === cfg.name ? null : cfg.name)}
        />
      ))}

      {/* Floating particles */}
      <FloatingParticles count={150} />

      {/* Event-driven burst effects */}
      <EventEffects bursts={bursts} />

      {/* Bloom post-processing (safe: skipped if library incompatible) */}
      {EffectComposer && Bloom && (
        <EffectComposer>
          <Bloom
            luminanceThreshold={0.2}
            luminanceSmoothing={0.9}
            intensity={1.5}
            mipmapBlur
          />
        </EffectComposer>
      )}

      {/* Camera controls */}
      <OrbitControls
        makeDefault
        enableDamping
        dampingFactor={0.05}
        minDistance={5}
        maxDistance={30}
        maxPolarAngle={Math.PI / 2.2}
        target={[0, 0, 0]}
      />
    </>
  )
}

// ═══════════════════════════════════════════════
//  2D Overlay Components
// ═══════════════════════════════════════════════

function StatusBar({ mission, sprint }: { mission: MissionData | null, sprint: SprintData | null }) {
  if (!mission) return null
  const progress = mission.totalSteps > 0 ? Math.round((mission.doneSteps / mission.totalSteps) * 100) : 0

  return (
    <div className="absolute top-3 left-3 right-3 z-10">
      <div className="bg-black/70 backdrop-blur-md rounded-xl border border-purple-500/30 px-4 py-2 flex items-center gap-6 text-sm">
        <div className="flex items-center gap-2">
          <span className="text-purple-400 font-bold">🏠</span>
          <span className="text-white font-medium truncate max-w-[200px]">{mission.title}</span>
        </div>
        <div className="text-purple-300">
          Sprint {sprint?.number ?? mission.currentSprint}/{mission.maxSprints}
        </div>
        <div className="flex items-center gap-2 flex-1">
          <div className="flex-1 h-2 bg-gray-800 rounded-full overflow-hidden">
            <div
              className="h-full rounded-full transition-all duration-500"
              style={{
                width: `${progress}%`,
                background: `linear-gradient(90deg, #7c3aed, #06b6d4)`,
              }}
            />
          </div>
          <span className="text-cyan-400 font-mono text-xs">{progress}%</span>
        </div>
        <div className="text-gray-400 text-xs">
          Steps: {mission.doneSteps}/{mission.totalSteps}
        </div>
        <div className="text-gray-500 text-xs font-mono">
          {mission.status}
        </div>
      </div>
    </div>
  )
}

function NodeDetailPanel({ node, steps, onClose }: { node: NodeData, steps: any[], onClose: () => void }) {
  const termRef = useRef<HTMLDivElement>(null)
  const config = NODE_CONFIGS.find(c => c.name === node.name)

  // Find steps assigned to this node
  const nodeSteps = steps.filter((s: any) =>
    s.target_node && (s.target_node.includes(node.name) || s.target_agent?.includes(node.role.toLowerCase()))
  )
  const activeStep = nodeSteps.find((s: any) => s.status === 'running' || s.status === 'dispatched')
  const doneSteps = nodeSteps.filter((s: any) => s.status === 'done')

  // Auto-scroll terminal
  useEffect(() => {
    if (termRef.current) termRef.current.scrollTop = termRef.current.scrollHeight
  }, [activeStep?.output])

  return (
    <div className="absolute right-3 top-16 bottom-16 w-96 z-10 bg-black/85 backdrop-blur-md rounded-xl border border-purple-500/30 flex flex-col overflow-hidden">
      {/* Header */}
      <div className="p-3 border-b border-purple-500/20 flex items-center justify-between">
        <div>
          <div className="flex items-center gap-2">
            <div className="w-2.5 h-2.5 rounded-full animate-pulse" style={{ backgroundColor: STATUS_COLORS[node.status] }} />
            <h3 className="text-white font-bold">{config?.building || node.name}</h3>
          </div>
          <p className="text-purple-400 text-xs mt-0.5">{node.name} :{config?.port} | {node.role}</p>
        </div>
        <button onClick={onClose} className="text-gray-400 hover:text-white p-1 rounded hover:bg-gray-800">
          <ArrowLeft className="w-4 h-4" />
        </button>
      </div>

      <div className="flex-1 overflow-y-auto p-3 space-y-3">
        {/* Status + Progress */}
        <div className="grid grid-cols-2 gap-2">
          <div className="bg-gray-900/60 rounded-lg p-2">
            <div className="text-[10px] text-gray-500 uppercase tracking-wider">Status</div>
            <div className="flex items-center gap-1.5 mt-1">
              <div className="w-2 h-2 rounded-full" style={{ backgroundColor: STATUS_COLORS[node.status] }} />
              <span className="text-white text-sm font-mono">{node.status}</span>
            </div>
          </div>
          <div className="bg-gray-900/60 rounded-lg p-2">
            <div className="text-[10px] text-gray-500 uppercase tracking-wider">Progress</div>
            <div className="flex items-center gap-2 mt-1">
              <div className="flex-1 h-1.5 bg-gray-800 rounded-full overflow-hidden">
                <div className="h-full bg-green-500 rounded-full transition-all" style={{ width: `${node.progress}%` }} />
              </div>
              <span className="text-green-400 text-xs font-mono">{node.progress}%</span>
            </div>
          </div>
        </div>

        {/* Current Task */}
        {activeStep && (
          <div className="bg-gray-900/60 rounded-lg p-2.5">
            <div className="text-[10px] text-gray-500 uppercase tracking-wider mb-1">Current Task</div>
            <p className="text-white text-xs leading-relaxed">{activeStep.task?.slice(0, 200)}</p>
            {activeStep.branch && (
              <div className="mt-2 flex items-center gap-1.5">
                <span className="text-[10px] text-gray-600">branch:</span>
                <span className="text-cyan-400 text-[11px] font-mono">{activeStep.branch}</span>
              </div>
            )}
          </div>
        )}

        {/* Terminal Output */}
        <div className="bg-gray-950 rounded-lg border border-gray-800 overflow-hidden">
          <div className="px-2.5 py-1.5 border-b border-gray-800 flex items-center gap-2">
            <div className="flex gap-1">
              <div className="w-2 h-2 rounded-full bg-red-500/80" />
              <div className="w-2 h-2 rounded-full bg-yellow-500/80" />
              <div className="w-2 h-2 rounded-full bg-green-500/80" />
            </div>
            <span className="text-gray-500 text-[10px] font-mono">terminal — {node.name}</span>
          </div>
          <div ref={termRef} className="p-2.5 h-36 overflow-y-auto font-mono text-[11px] leading-relaxed">
            {activeStep?.output ? (
              <pre className="text-green-400 whitespace-pre-wrap break-all">{activeStep.output.slice(-500)}</pre>
            ) : nodeSteps.length > 0 ? (
              <div className="text-gray-600">
                <div>$ agent --node {node.name}</div>
                <div className="text-gray-500 mt-1">Waiting for output...</div>
              </div>
            ) : (
              <div className="text-gray-600">
                <div>$ status</div>
                <div className="text-gray-500 mt-1">No active tasks assigned to this node.</div>
              </div>
            )}
          </div>
        </div>

        {/* Completed steps */}
        {doneSteps.length > 0 && (
          <div className="bg-gray-900/60 rounded-lg p-2.5">
            <div className="text-[10px] text-gray-500 uppercase tracking-wider mb-2">Completed ({doneSteps.length})</div>
            <div className="space-y-1.5">
              {doneSteps.slice(-5).map((s: any, i: number) => (
                <div key={i} className="flex items-start gap-2 text-[11px]">
                  <span className="text-green-500 mt-0.5">✓</span>
                  <div className="flex-1 min-w-0">
                    <p className="text-gray-300 truncate">{s.task?.slice(0, 80)}</p>
                    {s.branch && <p className="text-cyan-600 font-mono text-[10px]">{s.branch}</p>}
                  </div>
                </div>
              ))}
            </div>
          </div>
        )}

        {/* Node info */}
        <div className="bg-gray-900/60 rounded-lg p-2.5">
          <div className="text-[10px] text-gray-500 uppercase tracking-wider mb-1.5">Node Info</div>
          <div className="grid grid-cols-2 gap-y-1 text-[11px]">
            <span className="text-gray-500">Building</span>
            <span className="text-white">{config?.building}</span>
            <span className="text-gray-500">Role</span>
            <span className="text-white">{node.role}</span>
            <span className="text-gray-500">Port</span>
            <span className="text-cyan-400 font-mono">:{config?.port}</span>
            <span className="text-gray-500">Tasks</span>
            <span className="text-white">{nodeSteps.length} total</span>
          </div>
        </div>
      </div>
    </div>
  )
}

function KanbanView({ steps }: { steps: any[] }) {
  const columns = [
    { key: 'pending', label: 'Pending', color: '#888' },
    { key: 'running', label: 'Running', color: '#00ff88' },
    { key: 'reviewing', label: 'Review', color: '#aa44ff' },
    { key: 'done', label: 'Done', color: '#ffffff' },
    { key: 'failed', label: 'Failed', color: '#ff3344' },
  ]

  const grouped = columns.map(col => ({
    ...col,
    items: steps.filter(s => {
      if (col.key === 'running') return s.status === 'running' || s.status === 'dispatched'
      return s.status === col.key
    }),
  }))

  return (
    <div className="absolute inset-0 z-20 bg-black/85 backdrop-blur-md flex flex-col">
      <div className="p-4 border-b border-purple-500/20">
        <h2 className="text-white font-bold text-lg">📋 Kanban Board</h2>
      </div>
      <div className="flex-1 overflow-x-auto p-4">
        <div className="flex gap-4 h-full min-w-max">
          {grouped.map(col => (
            <div key={col.key} className="w-64 flex flex-col bg-gray-900/50 rounded-xl border border-gray-800">
              <div className="p-3 border-b border-gray-800 flex items-center gap-2">
                <div className="w-2 h-2 rounded-full" style={{ backgroundColor: col.color }} />
                <span className="text-white font-medium text-sm">{col.label}</span>
                <span className="text-gray-500 text-xs ml-auto">{col.items.length}</span>
              </div>
              <div className="flex-1 overflow-y-auto p-2 space-y-2">
                {col.items.map((step: any) => (
                  <div key={step.id} className="bg-gray-800/80 rounded-lg p-2.5 border border-gray-700/50 hover:border-purple-500/50 transition-colors">
                    <p className="text-white text-xs font-medium line-clamp-2">{step.task}</p>
                    <div className="mt-1.5 flex items-center gap-2">
                      {step.target_node && (
                        <span className="text-purple-400 text-[10px] font-mono">
                          {step.target_node.slice(0, 12)}
                        </span>
                      )}
                      {step.branch && (
                        <span className="text-cyan-400 text-[10px] font-mono truncate">
                          {step.branch}
                        </span>
                      )}
                    </div>
                    {step.status === 'running' && (
                      <div className="mt-1.5 h-1 bg-gray-700 rounded-full overflow-hidden">
                        <div className="h-full bg-green-500 rounded-full animate-pulse" style={{ width: '60%' }} />
                      </div>
                    )}
                  </div>
                ))}
                {col.items.length === 0 && (
                  <div className="text-gray-600 text-xs text-center py-4">Empty</div>
                )}
              </div>
            </div>
          ))}
        </div>
      </div>
    </div>
  )
}

function TimelineView({ sprints }: { sprints: SprintData[] }) {
  return (
    <div className="absolute inset-0 z-20 bg-black/85 backdrop-blur-md flex flex-col">
      <div className="p-4 border-b border-purple-500/20">
        <h2 className="text-white font-bold text-lg">⏱ Sprint Timeline</h2>
      </div>
      <div className="flex-1 overflow-x-auto p-6">
        <div className="flex items-center gap-6 h-full">
          {sprints.length === 0 && (
            <div className="text-gray-500 text-center w-full">No sprints yet</div>
          )}
          {sprints.map((sp, i) => {
            const progress = sp.totalSteps > 0 ? Math.round((sp.doneSteps / sp.totalSteps) * 100) : 0
            const isActive = sp.status === 'executing'
            const isDone = sp.status === 'done'
            const isFailed = sp.status === 'failed'

            return (
              <div key={i} className="flex items-center gap-4">
                <div className={`w-48 rounded-xl border p-4 transition-all ${
                  isActive ? 'border-cyan-500 bg-cyan-500/10 shadow-lg shadow-cyan-500/20' :
                  isDone ? 'border-green-500/50 bg-green-500/5' :
                  isFailed ? 'border-red-500/50 bg-red-500/5' :
                  'border-gray-700 bg-gray-900/50'
                }`}>
                  <div className="flex items-center gap-2 mb-2">
                    <span className="text-lg">
                      {isDone ? '✅' : isFailed ? '❌' : isActive ? '🔄' : '⏳'}
                    </span>
                    <span className="text-white font-bold text-sm">Sprint {sp.number}</span>
                  </div>
                  <p className="text-gray-400 text-xs mb-2 line-clamp-2">{sp.goal}</p>
                  <div className="flex items-center gap-2">
                    <div className="flex-1 h-1.5 bg-gray-800 rounded-full overflow-hidden">
                      <div
                        className={`h-full rounded-full ${isDone ? 'bg-green-500' : isFailed ? 'bg-red-500' : 'bg-cyan-500'}`}
                        style={{ width: `${progress}%` }}
                      />
                    </div>
                    <span className="text-gray-400 text-[10px] font-mono">{sp.doneSteps}/{sp.totalSteps}</span>
                  </div>
                </div>
                {i < sprints.length - 1 && (
                  <div className="w-8 h-0.5 bg-gray-700" />
                )}
              </div>
            )
          })}
        </div>
      </div>
    </div>
  )
}

function CodeFlowView({ steps, nodes }: { steps: any[], nodes: NodeData[] }) {
  const doneOrRunning = steps.filter((s: any) => s.status === 'done' || s.status === 'running' || s.status === 'dispatched')
  const nodeMap = new Map(nodes.map(n => [n.name, n]))

  return (
    <div className="absolute inset-0 z-20 bg-black/85 backdrop-blur-md flex flex-col">
      <div className="p-4 border-b border-purple-500/20 flex items-center gap-2">
        <GitBranch className="w-5 h-5 text-cyan-400" />
        <h2 className="text-white font-bold text-lg">Code Flow</h2>
        <span className="text-gray-500 text-xs ml-2">{doneOrRunning.length} steps</span>
      </div>
      <div className="flex-1 overflow-y-auto p-4">
        {/* Git commit-style timeline */}
        <div className="relative">
          {/* Vertical line */}
          <div className="absolute left-4 top-0 bottom-0 w-0.5 bg-gradient-to-b from-cyan-500 via-purple-500 to-gray-800" />

          {doneOrRunning.length === 0 && (
            <div className="text-gray-500 text-center py-12">No code flow activity yet</div>
          )}

          {doneOrRunning.map((step: any, i: number) => {
            const isRunning = step.status === 'running' || step.status === 'dispatched'
            const isDone = step.status === 'done'
            const matched = NODE_CONFIGS.find(c =>
              step.target_node && (step.target_node.includes(c.name) || step.target_agent?.includes(c.role.toLowerCase()))
            )
            const nodeData = matched ? nodeMap.get(matched.name) : undefined
            const statusColor = isRunning ? '#00ff88' : isDone ? '#4488ff' : '#888'

            return (
              <div key={step.id || i} className="relative pl-10 pb-6 group">
                {/* Commit dot */}
                <div
                  className={`absolute left-2.5 top-1 w-3.5 h-3.5 rounded-full border-2 ${isRunning ? 'animate-pulse' : ''}`}
                  style={{ backgroundColor: statusColor, borderColor: statusColor }}
                />

                {/* Flow arrow to node */}
                <div className="bg-gray-900/60 rounded-lg border border-gray-800 p-3 hover:border-purple-500/40 transition-colors">
                  <div className="flex items-center gap-3 mb-1.5">
                    {/* Step sequence */}
                    <span className="text-[10px] font-mono bg-gray-800 px-1.5 py-0.5 rounded text-gray-400">
                      S{step.sequence ?? i}
                    </span>

                    {/* Node tag */}
                    {matched && (
                      <span className="text-[10px] font-mono px-1.5 py-0.5 rounded"
                        style={{ backgroundColor: statusColor + '22', color: statusColor }}>
                        {matched.building}
                      </span>
                    )}

                    {/* Status */}
                    <span className={`text-[10px] ml-auto ${isRunning ? 'text-green-400' : isDone ? 'text-blue-400' : 'text-gray-500'}`}>
                      {isRunning ? 'RUNNING' : isDone ? 'DONE' : step.status?.toUpperCase()}
                    </span>
                  </div>

                  {/* Task description */}
                  <p className="text-white text-xs leading-relaxed line-clamp-2">
                    {step.task?.slice(0, 150)}
                  </p>

                  {/* Branch + output preview */}
                  <div className="mt-2 flex items-center gap-3 text-[10px]">
                    {step.branch && (
                      <span className="text-cyan-500 font-mono flex items-center gap-1">
                        <GitBranch className="w-2.5 h-2.5" />
                        {step.branch}
                      </span>
                    )}
                    {nodeData?.role && (
                      <span className="text-purple-400">{nodeData.role}</span>
                    )}
                  </div>

                  {/* Output snippet for done steps */}
                  {isDone && step.output && (
                    <div className="mt-2 bg-gray-950 rounded p-2 font-mono text-[10px] text-green-400/70 line-clamp-3 overflow-hidden">
                      {step.output.slice(-200)}
                    </div>
                  )}
                </div>
              </div>
            )
          })}
        </div>
      </div>
    </div>
  )
}

function FeedbackPanel({ missionId }: { missionId: string | null }) {
  const [open, setOpen] = useState(false)
  const [text, setText] = useState('')
  const [sending, setSending] = useState(false)
  const [sent, setSent] = useState(false)

  if (!missionId) return null

  const submit = async () => {
    if (!text.trim()) return
    setSending(true)
    try {
      await squadAPI.submitFeedback(missionId, text.trim())
      setText('')
      setSent(true)
      setTimeout(() => setSent(false), 3000)
    } catch { /* ignore */ }
    setSending(false)
  }

  return (
    <div className="absolute bottom-14 right-3 z-10">
      {open ? (
        <div className="bg-black/80 backdrop-blur-md rounded-xl border border-purple-500/30 p-3 w-72">
          <div className="flex items-center justify-between mb-2">
            <span className="text-white text-xs font-bold flex items-center gap-1.5">
              <MessageSquare className="w-3.5 h-3.5 text-purple-400" /> Feedback
            </span>
            <button onClick={() => setOpen(false)} className="text-gray-500 hover:text-white text-xs">✕</button>
          </div>
          <textarea
            value={text}
            onChange={e => setText(e.target.value)}
            placeholder="Tell the AI team what to improve..."
            className="w-full h-20 bg-gray-900 border border-gray-700 rounded-lg p-2 text-xs text-white placeholder-gray-600 resize-none focus:outline-none focus:border-purple-500"
          />
          <div className="flex items-center justify-between mt-2">
            {sent && <span className="text-green-400 text-[10px]">Feedback sent!</span>}
            {!sent && <span />}
            <button
              onClick={submit}
              disabled={sending || !text.trim()}
              className="flex items-center gap-1 bg-purple-600 hover:bg-purple-500 disabled:bg-gray-700 text-white text-xs px-3 py-1.5 rounded-lg transition-colors"
            >
              <Send className="w-3 h-3" />
              {sending ? 'Sending...' : 'Send'}
            </button>
          </div>
        </div>
      ) : (
        <button
          onClick={() => setOpen(true)}
          className="bg-black/70 backdrop-blur-md rounded-lg border border-purple-500/30 p-2 text-gray-400 hover:text-purple-400 transition-colors"
          title="Submit feedback"
        >
          <MessageSquare className="w-4 h-4" />
        </button>
      )}
    </div>
  )
}

function ViewToolbar({ view, onViewChange }: { view: string, onViewChange: (v: string) => void }) {
  const views = [
    { key: 'hive', icon: Eye, label: 'Hive' },
    { key: 'kanban', icon: LayoutGrid, label: 'Kanban' },
    { key: 'timeline', icon: Clock, label: 'Timeline' },
    { key: 'codeflow', icon: GitBranch, label: 'Code' },
  ]

  return (
    <div className="absolute bottom-3 left-1/2 -translate-x-1/2 z-10">
      <div className="bg-black/70 backdrop-blur-md rounded-xl border border-purple-500/30 px-2 py-1.5 flex items-center gap-1">
        {views.map(v => (
          <button
            key={v.key}
            onClick={() => onViewChange(v.key)}
            className={`flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-medium transition-all ${
              view === v.key
                ? 'bg-purple-600 text-white'
                : 'text-gray-400 hover:text-white hover:bg-gray-800'
            }`}
          >
            <v.icon className="w-3.5 h-3.5" />
            {v.label}
          </button>
        ))}
      </div>
    </div>
  )
}

// ═══════════════════════════════════════════════
//  Main Page
// ═══════════════════════════════════════════════

export default function HiveMindPage() {
  const [view, setView] = useState('hive')
  const [selectedNode, setSelectedNode] = useState<string | null>(null)
  const [mission, setMission] = useState<MissionData | null>(null)
  const [nodes, setNodes] = useState<NodeData[]>([])
  const [steps, setSteps] = useState<any[]>([])
  const [sprints, setSprints] = useState<SprintData[]>([])
  const [bursts, setBursts] = useState<BurstEvent[]>([])
  const clockRef = useRef(0)

  // Trigger a burst effect at a node position
  const triggerBurst = useCallback((nodeName: string, color: string, count: number, duration: number) => {
    const cfg = NODE_CONFIGS.find(c => c.name === nodeName)
    if (!cfg) return
    const burst: BurstEvent = {
      id: burstIdCounter++,
      position: cfg.pos,
      color,
      count,
      startTime: clockRef.current,
      duration,
    }
    setBursts(prev => [...prev.slice(-10), burst]) // keep max 10 active bursts
  }, [])

  // Fetch squad data
  const fetchData = useCallback(async () => {
    try {
      // Get active missions
      const missionsRes = await squadAPI.missions()
      const allMissions = missionsRes.data?.missions || missionsRes.data || []
      const activeMission = allMissions.find((m: any) => m.status === 'executing') || allMissions[0]

      if (activeMission) {
        setMission({
          id: activeMission.id,
          title: activeMission.title,
          status: activeMission.status,
          currentSprint: activeMission.current_sprint || 0,
          maxSprints: activeMission.max_sprints || 4,
          totalSteps: activeMission.total_steps || 0,
          doneSteps: activeMission.done_steps || 0,
        })

        // Get mission steps
        try {
          const stepsRes = await squadAPI.missionSteps(activeMission.id)
          const stepsData = stepsRes.data?.steps || stepsRes.data || []
          setSteps(stepsData)

          // Build node data from steps
          const nodeDataMap = new Map<string, NodeData>()
          NODE_CONFIGS.forEach(cfg => {
            nodeDataMap.set(cfg.name, {
              id: cfg.name,
              name: cfg.name,
              role: cfg.role,
              port: cfg.port,
              status: 'idle',
              currentTask: '',
              progress: 0,
              branch: '',
              lastAction: '',
            })
          })

          stepsData.forEach((step: any) => {
            // Match step target_node to node configs
            const matchedConfig = NODE_CONFIGS.find(c =>
              step.target_node && (step.target_node.includes(c.name) || step.target_agent?.includes(c.role.toLowerCase()))
            )
            if (matchedConfig) {
              const nd = nodeDataMap.get(matchedConfig.name)!
              if (step.status === 'running' || step.status === 'dispatched') {
                nd.status = 'running'
                nd.currentTask = step.task?.slice(0, 100) || ''
                nd.branch = step.branch || ''
                nd.progress = 50 // estimate
              } else if (step.status === 'done' && nd.status === 'idle') {
                nd.status = 'done'
                nd.currentTask = step.task?.slice(0, 100) || ''
                nd.progress = 100
              } else if (step.status === 'failed') {
                nd.status = 'failed'
                nd.currentTask = step.error_msg || step.task?.slice(0, 100) || ''
              }
            }
          })

          // Captain is always at least idle or running if mission is executing
          const captain = nodeDataMap.get('claw-lead')!
          if (activeMission.status === 'executing') {
            captain.status = 'running'
            captain.currentTask = 'Orchestrating mission'
            captain.progress = activeMission.total_steps > 0
              ? Math.round((activeMission.done_steps / activeMission.total_steps) * 100)
              : 0
          }

          setNodes(Array.from(nodeDataMap.values()))
        } catch {}

        // Get sprints
        try {
          const sprintsRes = await squadAPI.sprints(activeMission.id)
          const sprintsData = sprintsRes.data?.sprints || sprintsRes.data || []
          setSprints(sprintsData.map((s: any) => ({
            number: s.number,
            goal: s.goal,
            status: s.status,
            doneSteps: s.done_steps || 0,
            totalSteps: s.total_steps || 0,
          })))
        } catch {
          // Generate dummy sprint from mission data
          setSprints([{
            number: activeMission.current_sprint || 0,
            goal: activeMission.title,
            status: activeMission.status === 'completed' ? 'done' : 'executing',
            doneSteps: activeMission.done_steps || 0,
            totalSteps: activeMission.total_steps || 0,
          }])
        }
      } else {
        // No active mission — show demo nodes
        setNodes(NODE_CONFIGS.map(cfg => ({
          id: cfg.name,
          name: cfg.name,
          role: cfg.role,
          port: cfg.port,
          status: 'idle' as const,
          currentTask: '',
          progress: 0,
          branch: '',
          lastAction: '',
        })))
      }
    } catch (err) {
      console.error('Failed to fetch hive data:', err)
      // Show demo data on error
      setNodes(NODE_CONFIGS.map(cfg => ({
        id: cfg.name,
        name: cfg.name,
        role: cfg.role,
        port: cfg.port,
        status: (['running', 'idle', 'waiting', 'done', 'reviewing', 'idle', 'idle'] as const)[NODE_CONFIGS.indexOf(cfg)],
        currentTask: cfg.role === 'Captain' ? 'Orchestrating Sprint 2' : '',
        progress: cfg.role === 'Captain' ? 67 : cfg.role === 'Backend' ? 78 : 0,
        branch: cfg.role === 'Backend' ? 'sprint-2/step-0-coding' : '',
        lastAction: '',
      })))
    }
  }, [])

  useEffect(() => {
    fetchData()
    const interval = setInterval(fetchData, 10000)
    return () => clearInterval(interval)
  }, [fetchData])

  // WebSocket real-time updates
  useEffect(() => {
    const unsubStep = starclawWS.on('hive_step_update', (data: any) => {
      if (!data?.steps) return

      // Compare with previous steps to detect status changes and trigger bursts
      setSteps(prevSteps => {
        const prevMap = new Map(prevSteps.map((s: any) => [s.step_id || s.id, s.status]))
        data.steps.forEach((step: any) => {
          const prevStatus = prevMap.get(step.step_id || step.id)
          const matched = NODE_CONFIGS.find(c =>
            step.target_node && (step.target_node.includes(c.name) || step.target_node.includes(c.role?.toLowerCase()))
          )
          const nodeName = matched?.name || 'claw-lead'
          if (prevStatus && prevStatus !== step.status) {
            if (step.status === 'done') {
              triggerBurst(nodeName, '#ffd700', 30, 2) // gold burst
              triggerBurst('claw-lead', '#ffd700', 15, 1.5) // captain absorbs
            } else if (step.status === 'failed') {
              triggerBurst(nodeName, '#ff3344', 40, 2.5) // red alert burst
            }
          }
        })
        return data.steps
      })

      // Rebuild node data from updated steps
      const nodeDataMap = new Map<string, NodeData>()
      NODE_CONFIGS.forEach(cfg => {
        nodeDataMap.set(cfg.name, {
          id: cfg.name, name: cfg.name, role: cfg.role, port: cfg.port,
          status: 'idle', currentTask: '', progress: 0, branch: '', lastAction: '',
        })
      })
      data.steps.forEach((step: any) => {
        const matched = NODE_CONFIGS.find(c =>
          step.target_node && (step.target_node.includes(c.name) || step.target_node.includes(c.role.toLowerCase()))
        )
        if (matched) {
          const nd = nodeDataMap.get(matched.name)!
          if (step.status === 'running' || step.status === 'dispatched') {
            nd.status = 'running'
            nd.currentTask = step.task || ''
            nd.branch = step.branch || ''
            nd.progress = 50
          } else if (step.status === 'done' && nd.status === 'idle') {
            nd.status = 'done'
            nd.progress = 100
          } else if (step.status === 'failed') {
            nd.status = 'failed'
          }
        }
      })
      const captain = nodeDataMap.get('claw-lead')!
      if (captain.status === 'idle') {
        captain.status = 'running'
        captain.currentTask = 'Orchestrating mission'
      }
      setNodes(Array.from(nodeDataMap.values()))
    })

    const unsubSprint = starclawWS.on('hive_sprint', (data: any) => {
      if (!data) return
      setMission(prev => prev ? {
        ...prev,
        doneSteps: data.done_steps ?? prev.doneSteps,
        totalSteps: data.total_steps ?? prev.totalSteps,
      } : prev)
      if (data.sprint_number !== undefined) {
        setSprints(prev => {
          const idx = prev.findIndex(s => s.number === data.sprint_number)
          const prevSprint = idx >= 0 ? prev[idx] : null
          const updated: SprintData = {
            number: data.sprint_number,
            goal: data.sprint_goal || '',
            status: data.sprint_status || 'executing',
            doneSteps: data.done_steps || 0,
            totalSteps: data.total_steps || 0,
          }
          // Sprint completion celebration: white burst on all nodes
          if (prevSprint && prevSprint.status !== 'done' && updated.status === 'done') {
            NODE_CONFIGS.forEach(cfg => triggerBurst(cfg.name, '#ffffff', 20, 3))
          }
          if (idx >= 0) {
            const copy = [...prev]
            copy[idx] = updated
            return copy
          }
          return [...prev, updated]
        })
      }
    })

    return () => { unsubStep(); unsubSprint() }
  }, [triggerBurst])

  // Keyboard shortcuts
  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if (e.target instanceof HTMLInputElement || e.target instanceof HTMLTextAreaElement) return
      switch (e.key) {
        case '1': setView('hive'); break
        case '2': setView('kanban'); break
        case '3': setView('timeline'); break
        case '4': setView('codeflow'); break
        case 'Escape': setSelectedNode(null); setView('hive'); break
      }
    }
    window.addEventListener('keydown', handler)
    return () => window.removeEventListener('keydown', handler)
  }, [])

  const selectedNodeData = nodes.find(n => n.name === selectedNode)
  const currentSprint = sprints.find(s => s.status === 'executing') || sprints[sprints.length - 1]

  return (
    <div className="h-[calc(100vh-64px)] relative bg-black overflow-hidden">
      {/* Status bar */}
      <StatusBar mission={mission} sprint={currentSprint || null} />

      {/* 3D Canvas */}
      <CanvasErrorBoundary>
        <Canvas
          camera={{ position: [0, 12, 12], fov: 50 }}
          style={{ background: '#0a0a1a' }}
          gl={{ antialias: true, alpha: false }}
        >
          <fog attach="fog" args={['#0a0a1a', 15, 35]} />
          <HiveScene
            nodes={nodes}
            sprint={currentSprint || null}
            selectedNode={selectedNode}
            onSelectNode={setSelectedNode}
            bursts={bursts}
          />
        </Canvas>
      </CanvasErrorBoundary>

      {/* Node detail panel */}
      {selectedNodeData && view === 'hive' && (
        <NodeDetailPanel node={selectedNodeData} steps={steps} onClose={() => setSelectedNode(null)} />
      )}

      {/* Kanban overlay */}
      {view === 'kanban' && <KanbanView steps={steps} />}

      {/* Timeline overlay */}
      {view === 'timeline' && <TimelineView sprints={sprints} />}

      {/* Code Flow overlay */}
      {view === 'codeflow' && <CodeFlowView steps={steps} nodes={nodes} />}

      {/* View toolbar */}
      <ViewToolbar view={view} onViewChange={setView} />

      {/* Refresh + WS status */}
      <div className="absolute top-16 right-3 z-10 flex items-center gap-2">
        <div className="bg-black/70 backdrop-blur-md rounded-lg border border-purple-500/30 p-2 flex items-center gap-1.5" title="WebSocket status">
          <Wifi className="w-3.5 h-3.5 text-green-500" />
          <span className="text-[10px] text-green-500 font-mono">LIVE</span>
        </div>
        <button
          onClick={fetchData}
          className="bg-black/70 backdrop-blur-md rounded-lg border border-purple-500/30 p-2 text-gray-400 hover:text-white transition-colors"
          title="Refresh"
        >
          <RefreshCw className="w-4 h-4" />
        </button>
      </div>

      {/* Legend */}
      <div className="absolute bottom-14 left-3 z-10">
        <div className="bg-black/60 backdrop-blur-sm rounded-lg border border-gray-800 px-3 py-2 flex gap-3 text-[10px]">
          {Object.entries(STATUS_COLORS).map(([status, color]) => (
            <div key={status} className="flex items-center gap-1">
              <div className="w-1.5 h-1.5 rounded-full" style={{ backgroundColor: color }} />
              <span className="text-gray-500">{status}</span>
            </div>
          ))}
        </div>
      </div>

      {/* Feedback panel */}
      <FeedbackPanel missionId={mission?.id || null} />
    </div>
  )
}
