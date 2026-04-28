import { useState, useEffect, useRef } from 'react'
import {
  X, Film, Clapperboard, Play, Plus, Check, CircleDot, CircleAlert, CircleX,
  Music, Scissors, Sparkles, Image as ImageIcon, Trash2, ChevronRight, Layers,
  Archive, FileText, History, Wand2, AlertTriangle, Lightbulb, Wrench,
  Copy, ExternalLink, Terminal, PlayCircle, Maximize2, Loader2,
} from 'lucide-react'
import type { Node } from '@xyflow/react'
import {
  SEASONS, sceneTakesSummary,
  VIDEO_RESOLUTION_OPTIONS, VIDEO_RATIO_OPTIONS,
  DEFAULT_VIDEO_RESOLUTION, DEFAULT_VIDEO_RATIO,
  type EpisodeData, type SceneSpec, type Take, type Composition,
} from './episodeTypes'
import { dramaAPI, videoAPI, imageAPI, type WriterReviewResponse, type PromoResponse } from '../../lib/api'
import VideoPreviewModal from './VideoPreviewModal'

interface Props {
  node: Node
  onUpdate: (id: string, data: Record<string, unknown>) => void
  onClose: () => void
  onProduce?: (episode: EpisodeData) => void
  onCancel?: () => void
  onFlushSave?: () => void   // 删除 take 等关键操作后立即落盘
  initialSceneId?: string   // 外部传入打开时自动展开的场景
  initialTab?: TabKey       // 外部传入打开时默认选中的 tab（一次性，消费后清空）
  onConsumeInitialTab?: () => void
}

type TabKey = 'scenes' | 'composition' | 'script' | 'meta'

const TAKE_STATUS_COLOR: Record<Take['status'], string> = {
  pending:   'border-gray-600 bg-gray-800 text-gray-400',
  running:   'border-amber-500 bg-amber-900/30 text-amber-300 animate-pulse',
  succeeded: 'border-emerald-500 bg-emerald-900/20 text-emerald-300',
  failed:    'border-red-500 bg-red-900/20 text-red-300',
  cancelled: 'border-gray-600 bg-gray-800/50 text-gray-500',
}

const TAKE_STATUS_ICON: Record<Take['status'], typeof Check> = {
  pending: CircleDot, running: CircleDot, succeeded: Check, failed: CircleX, cancelled: CircleX,
}

// 归档目录约定：docs/<project>/production/<epKey>/clips_v2/<scene>_<take>.mp4
//   被静态路由 /v1/projects/:project/*filepath 暴露。
function deriveArchivedLocalUrl(
  project: string | undefined,
  epKey: string | undefined,
  sceneId: string,
  takeId: string | undefined,
): string {
  if (!project || !epKey || !takeId) return ''
  return `/v1/projects/${project}/production/${epKey}/clips_v2/${sceneId}_${takeId}.mp4`
}

// 单个 take 的播放 URL 候选列表（按优先级）。
//   1. 显式 local_url（archive 端点回写）
//   2. 派生 archive 路径（state 里 local_url 丢了但 mp4 还在磁盘上）
//   3. TOS video_url（24h 过期签名）
function takeUrlCandidates(
  take: Take, project: string | undefined, epKey: string | undefined, sceneId: string,
): string[] {
  const out: string[] = []
  const push = (u?: string | null) => { if (u && !out.includes(u)) out.push(u) }
  push(take.local_url)
  push(deriveArchivedLocalUrl(project, epKey, sceneId, take.take_id))
  push(take.video_url)
  return out
}

export default function EpisodeWorkflowPanel({ node, onUpdate, onClose, onProduce, onCancel, onFlushSave, initialSceneId, initialTab, onConsumeInitialTab }: Props) {
  const data = node.data as unknown as EpisodeData
  const [tab, setTab] = useState<TabKey>(initialTab || 'scenes')
  const [expandedScene, setExpandedScene] = useState<string | null>(initialSceneId || null)

  // 外部切换 initialSceneId 时（sceneStep 节点点击），自动跳到 scenes tab 并展开该场景
  useEffect(() => {
    if (initialSceneId) {
      setTab('scenes')
      setExpandedScene(initialSceneId)
    }
  }, [initialSceneId, node.id])

  // 外部切 initialTab（如点 Final Cut 节点希望默认到 composition tab）。一次性信号，消费后清空。
  useEffect(() => {
    if (initialTab) {
      setTab(initialTab)
      onConsumeInitialTab?.()
    }
  }, [initialTab, onConsumeInitialTab])

  const scenes = data.scenes || []
  const comp: Composition = data.composition || { picked_clips: [], status: 'pending' }

  const seasonMeta = data.is_spinoff ? null : SEASONS.find(s => s.number === data.season)

  // 归档目录：docs/<project>/production/<epKey>/clips_v2/...
  //   每个 take 缩略图遇到过期 TOS 时，会回退到这里的 mp4。
  const archiveProject = 'swarm-universe'
  const archiveEpKey = `${data.is_spinoff ? 'sp' : 'ep'}${String(data.episode_number || 0).padStart(2, '0')}`

  const update = (patch: Partial<EpisodeData>) => {
    onUpdate(node.id, { ...data, ...patch } as unknown as Record<string, unknown>)
  }

  // Scene operations
  const addScene = () => {
    const next = scenes.length + 1
    update({
      scenes: [...scenes, {
        id: `S${next}`, label: `场景 ${next}`, duration: 8, takes: [],
      }],
    })
  }
  const removeScene = (sid: string) => {
    update({ scenes: scenes.filter(s => s.id !== sid) })
  }
  const updateScene = (sid: string, patch: Partial<SceneSpec>) => {
    update({ scenes: scenes.map(s => s.id === sid ? { ...s, ...patch } : s) })
  }
  const addTake = (sid: string) => {
    const scene = scenes.find(s => s.id === sid)
    if (!scene) return
    const takeNum = scene.takes.reduce((max, t) => {
      const m = /^t(\d+)$/.exec(t.take_id || '')
      return m ? Math.max(max, Number(m[1])) : max
    }, 0) + 1
    const newTake: Take = {
      take_id: `t${takeNum}`,
      status: 'pending',
      created_at: new Date().toISOString(),
    }
    updateScene(sid, { takes: [...scene.takes, newTake] })
  }
  const pickTake = (sid: string, tid: string) => {
    const newScenes = scenes.map(s => s.id === sid ? { ...s, picked_take: tid } : s)
    const picked_clips = newScenes.map(s => s.picked_take ? `${s.id}.${s.picked_take}` : '').filter(Boolean)
    update({ scenes: newScenes, composition: { ...comp, picked_clips } })
  }
  // ── Storyboard frame (GPT Image 2) ───────────────────────────
  // 调 /v1/images/generate 同步生成一张 720×1280 故事板静帧，
  // 完成后把 storyboard_url 写回 scene，后续 runEpisodeProduction
  // 会优先用这张图做 Seedance i2v 首帧。
  const generateStoryboard = async (sid: string, prompt: string, model = 'gpt-image-2') => {
    const scene = scenes.find(s => s.id === sid)
    if (!scene) return
    updateScene(sid, { storyboard_status: 'running', storyboard_prompt: prompt, storyboard_model: model })
    try {
      const resp = await imageAPI.generate({
        prompt,
        model,
        size: '720x1280',
        scene: `${data.label.split(' ')[0]}-${sid}`,
        style: 'storyboard',
      })
      const body = resp.data as { image_url?: string; url?: string; local_url?: string; display_url?: string; image_id?: string }
      const url = body.display_url || body.local_url || body.image_url || body.url || ''
      if (!url) throw new Error('no image url in response')
      updateScene(sid, {
        storyboard_status: 'succeeded',
        storyboard_url: url,
        storyboard_task_id: body.image_id,
      })
      onFlushSave?.()
    } catch (e) {
      const err = e as { response?: { data?: { error?: string } }; message?: string }
      const note = err?.response?.data?.error || err?.message || String(e)
      updateScene(sid, { storyboard_status: 'failed', storyboard_prompt: `${prompt}\n\n[失败] ${note}` })
    }
  }

  const removeTake = (sid: string, tid: string) => {
    const scene = scenes.find(s => s.id === sid)
    if (!scene) return
    const removed = scene.takes.find(t => t.take_id === tid)
    const takes = scene.takes.filter(t => t.take_id !== tid)
    // Track deleted task_id so archive backfill won't re-add it
    const deleted_task_ids = [...(scene.deleted_task_ids || [])]
    if (removed?.task_id && !deleted_task_ids.includes(removed.task_id)) {
      deleted_task_ids.push(removed.task_id)
    }
    updateScene(sid, {
      takes,
      deleted_task_ids,
      picked_take: scene.picked_take === tid ? undefined : scene.picked_take,
    })
    // Flush save so deletion persists across refresh
    onFlushSave?.()
  }

  // Compute totals
  const totalDuration = scenes.reduce((sum, s) => sum + (s.duration || 0), 0)
  const pickedCount = scenes.filter(s => s.picked_take).length
  const totalTakes = scenes.reduce((sum, s) => sum + s.takes.length, 0)

  return (
    <div className="w-[480px] border-l border-gray-800 bg-gray-900 flex flex-col h-full">
      {/* Header */}
      <div className="px-4 py-3 border-b border-gray-800 bg-gradient-to-br from-gray-850 to-gray-900">
        <div className="flex items-start justify-between mb-2">
          <div className="flex items-center gap-2 min-w-0 flex-1">
            <Clapperboard className="w-4 h-4 text-cyan-400 flex-shrink-0" />
            <h3 className="text-sm font-semibold text-gray-100 truncate">{data.label}</h3>
          </div>
          <button onClick={onClose} className="text-gray-500 hover:text-gray-200 transition-colors flex-shrink-0">
            <X className="w-4 h-4" />
          </button>
        </div>
        <div className="flex items-center gap-1.5 flex-wrap">
          {seasonMeta && (
            <span className={`px-2 py-0.5 rounded text-[10px] bg-gradient-to-r ${seasonMeta.gradient} text-white font-medium`}>
              {seasonMeta.title} · {seasonMeta.subtitle}
            </span>
          )}
          {data.is_spinoff && (
            <span className="px-2 py-0.5 rounded text-[10px] bg-slate-500/30 text-slate-200 border border-slate-500/40">
              衍生·{data.spinoff_group || '未分组'}
            </span>
          )}
          <span className="px-2 py-0.5 rounded text-[10px] bg-gray-800 border border-gray-700 text-gray-400">
            {scenes.length}镜 · {totalDuration}s / {data.duration || 0}s
          </span>
          <span className="px-2 py-0.5 rounded text-[10px] bg-gray-800 border border-gray-700 text-gray-400">
            已选 {pickedCount}/{scenes.length}
          </span>
          <CompositionStatusPill status={comp.status} />
          {data.history_preview && (
            <button
              onClick={() => window.open(data.history_preview!.clip, '_blank')}
              title={data.history_preview.note || '查看整集历史废稿'}
              className="px-2 py-0.5 rounded text-[10px] bg-red-900/30 border border-red-700/40 text-red-300 hover:bg-red-800/40 hover:text-red-200 transition flex items-center gap-1">
              <History className="w-2.5 h-2.5" /> 历史合成废稿
            </button>
          )}
        </div>
      </div>

      {/* Tabs */}
      <div className="flex border-b border-gray-800">
        <TabBtn active={tab === 'scenes'} onClick={() => setTab('scenes')} icon={Layers} label="场景" count={scenes.length} />
        <TabBtn active={tab === 'composition'} onClick={() => setTab('composition')} icon={Scissors} label="合成链路" count={comp.picked_clips.length} />
        <TabBtn active={tab === 'script'} onClick={() => setTab('script')} icon={FileText} label="剧本" />
        <TabBtn active={tab === 'meta'} onClick={() => setTab('meta')} icon={Sparkles} label="元数据" />
      </div>

      {/* Body */}
      <div className="flex-1 overflow-y-auto">
        {tab === 'scenes' && (
          <div className="p-3 space-y-2">
            {scenes.length === 0 && (
              <div className="text-center py-12 text-gray-500">
                <Film className="w-8 h-8 mx-auto mb-2 opacity-50" />
                <p className="text-xs">还没有场景</p>
                <button onClick={addScene} className="mt-3 px-3 py-1.5 text-xs rounded-lg bg-cyan-600 text-white hover:bg-cyan-500 transition inline-flex items-center gap-1">
                  <Plus className="w-3 h-3" /> 添加第一个场景
                </button>
              </div>
            )}
            {scenes.map((scene, idx) => (
              <SceneCard
                key={scene.id}
                scene={scene}
                expanded={expandedScene === scene.id}
                onToggle={() => setExpandedScene(expandedScene === scene.id ? null : scene.id)}
                onUpdate={(patch) => updateScene(scene.id, patch)}
                onRemove={() => removeScene(scene.id)}
                onAddTake={() => addTake(scene.id)}
                onPickTake={(tid) => pickTake(scene.id, tid)}
                onRemoveTake={(tid) => removeTake(scene.id, tid)}
                onGenerateStoryboard={(prompt, model) => generateStoryboard(scene.id, prompt, model)}
                sequenceNumber={idx + 1}
                archiveProject={archiveProject}
                archiveEpKey={archiveEpKey}
              />
            ))}
            {scenes.length > 0 && (
              <button onClick={addScene}
                className="w-full py-2 rounded-lg border border-dashed border-gray-700 hover:border-cyan-500/60 text-xs text-gray-500 hover:text-cyan-400 transition flex items-center justify-center gap-1.5">
                <Plus className="w-3 h-3" /> 添加场景
              </button>
            )}
            {/* Totals bar */}
            {scenes.length > 0 && (
              <div className="mt-3 p-2.5 rounded-lg bg-gray-800/50 border border-gray-700/50 text-[11px] text-gray-400 flex items-center justify-between">
                <span>总生成 <span className="text-gray-200 font-semibold">{totalTakes}</span> 次 · 选中 <span className="text-emerald-400 font-semibold">{pickedCount}</span> / {scenes.length}</span>
                <span>废片率 <span className="text-amber-400 font-semibold">{totalTakes ? Math.round((totalTakes - pickedCount) / totalTakes * 100) : 0}%</span></span>
              </div>
            )}
          </div>
        )}

        {tab === 'composition' && (
          <CompositionTab
            data={data}
            comp={comp}
            scenes={scenes}
            onUpdate={(c) => update({ composition: c })}
          />
        )}

        {tab === 'script' && (
          <ScriptTab data={data} />
        )}

        {tab === 'meta' && (
          <MetaTab data={data} onUpdate={update} />
        )}
      </div>

      {/* Footer actions */}
      <div className="px-4 py-3 border-t border-gray-800 bg-gray-950/50 flex items-center gap-2">
        <button
          disabled={!comp.final_video_url}
          onClick={() => { if (comp.final_video_url) { setTab('composition') } }}
          className="flex-1 px-3 py-2 text-xs font-medium rounded-lg bg-gray-800 border border-gray-700 text-gray-300 hover:text-white hover:bg-gray-700 disabled:opacity-40 disabled:cursor-not-allowed transition flex items-center justify-center gap-1.5">
          <Play className="w-3.5 h-3.5" /> 预览成片
        </button>
        {comp.final_video_url && (
          <a href={`${comp.final_video_url}?download=1`} download
            className="px-3 py-2 text-xs font-medium rounded-lg bg-cyan-700 text-white hover:bg-cyan-600 transition flex items-center justify-center gap-1">
            <ExternalLink className="w-3.5 h-3.5" />
          </a>
        )}
        {comp.status === 'generating' ? (
          <button
            onClick={() => onCancel?.()}
            disabled={!onCancel}
            className="flex-1 px-3 py-2 text-xs font-medium rounded-lg bg-rose-600/80 text-white hover:bg-rose-500 disabled:opacity-40 disabled:cursor-not-allowed transition flex items-center justify-center gap-1.5 shadow-lg shadow-rose-900/30">
            <CircleX className="w-3.5 h-3.5" /> 停止生产
          </button>
        ) : (
          <button
            onClick={() => onProduce?.(data)}
            disabled={!onProduce || scenes.length === 0}
            className="flex-1 px-3 py-2 text-xs font-medium rounded-lg bg-emerald-600 text-white hover:bg-emerald-500 disabled:opacity-40 disabled:cursor-not-allowed transition flex items-center justify-center gap-1.5 shadow-lg shadow-emerald-900/30">
            <Sparkles className="w-3.5 h-3.5" /> 开始生产 {data.label.split(' ')[0]}
          </button>
        )}
      </div>
    </div>
  )
}

// ── Sub-components ──

function TabBtn({ active, onClick, icon: Icon, label, count }: { active: boolean; onClick: () => void; icon: typeof Layers; label: string; count?: number }) {
  return (
    <button onClick={onClick}
      className={`flex-1 flex items-center justify-center gap-1.5 px-3 py-2 text-xs font-medium border-b-2 transition ${active ? 'border-cyan-500 text-cyan-300 bg-cyan-900/10' : 'border-transparent text-gray-500 hover:text-gray-300'}`}>
      <Icon className="w-3.5 h-3.5" /> {label}
      {count !== undefined && <span className="text-[10px] opacity-60">({count})</span>}
    </button>
  )
}

function CompositionStatusPill({ status }: { status: Composition['status'] }) {
  const map: Record<Composition['status'], { label: string; cls: string }> = {
    pending:    { label: '待生产', cls: 'bg-gray-800 border-gray-700 text-gray-400' },
    generating: { label: '生产中', cls: 'bg-amber-900/30 border-amber-500/40 text-amber-300 animate-pulse' },
    ready:      { label: '已合成', cls: 'bg-emerald-900/30 border-emerald-500/40 text-emerald-300' },
    published:  { label: '已发布', cls: 'bg-violet-900/30 border-violet-500/40 text-violet-300' },
  }
  const m = map[status]
  return <span className={`px-2 py-0.5 rounded text-[10px] border ${m.cls}`}>{m.label}</span>
}

// 故事板静帧 UI：展示 GPT Image 2 生成的 720×1280 静帧，作为 Seedance i2v 首帧。
//   - storyboard_url 已存在 → 显示缩略图 + 「重新生成」 / 「清除」
//   - 无 → 显示一个 prompt 输入框 + 「生成静帧」 按钮
//   - 状态 running → 显示 spinner，禁用按钮
function StoryboardFrameSection({
  scene, onUpdate, onGenerate,
}: {
  scene: SceneSpec
  onUpdate: (p: Partial<SceneSpec>) => void
  onGenerate?: (prompt: string, model?: string) => void
}) {
  const [editing, setEditing] = useState(false)
  const [draft, setDraft] = useState(scene.storyboard_prompt || scene.prompt || '')
  const [model, setModel] = useState<string>(scene.storyboard_model || 'gpt-image-2')
  const status = scene.storyboard_status
  const isRunning = status === 'running'

  // 当父组件更新 scene 时（外部 prompt 改了），同步本地 draft 默认值
  useEffect(() => {
    if (!editing) setDraft(scene.storyboard_prompt || scene.prompt || '')
  }, [scene.storyboard_prompt, scene.prompt, editing])

  return (
    <div className="rounded border border-gray-700/40 bg-gray-900/40 p-2 space-y-1.5">
      <div className="flex items-center justify-between">
        <label className="text-[10px] font-medium text-gray-500 uppercase tracking-wider flex items-center gap-1">
          <ImageIcon className="w-3 h-3" />
          故事板静帧
          {scene.storyboard_url && <span className="text-emerald-400 normal-case tracking-normal">· i2v 锚定</span>}
        </label>
        {scene.storyboard_url && (
          <button
            onClick={() => onUpdate({ storyboard_url: undefined, storyboard_status: undefined, storyboard_task_id: undefined })}
            className="text-[10px] text-gray-500 hover:text-red-400 transition">
            清除
          </button>
        )}
      </div>

      {scene.storyboard_url ? (
        <div className="flex items-start gap-2">
          <a href={scene.storyboard_url} target="_blank" rel="noreferrer"
            className="block w-16 h-28 flex-shrink-0 rounded overflow-hidden bg-gray-950 border border-gray-700 hover:border-cyan-500 transition">
            <img src={scene.storyboard_url} alt={`${scene.id} storyboard`}
              className="w-full h-full object-cover" />
          </a>
          <div className="flex-1 min-w-0 text-[10px] text-gray-500 space-y-0.5">
            <div>模型 <span className="text-gray-300">{scene.storyboard_model || 'gpt-image-2'}</span></div>
            <div>720×1280 · 9:16</div>
            <button
              onClick={() => onGenerate?.(draft || scene.prompt || scene.label, model)}
              disabled={isRunning || !onGenerate}
              className="mt-1 px-2 py-0.5 rounded text-[10px] bg-violet-700/40 hover:bg-violet-600/50 border border-violet-600/40 text-violet-200 disabled:opacity-40 transition inline-flex items-center gap-1">
              {isRunning ? <Loader2 className="w-2.5 h-2.5 animate-spin" /> : <Sparkles className="w-2.5 h-2.5" />}
              重新生成
            </button>
          </div>
        </div>
      ) : (
        <>
          <textarea
            value={draft}
            onFocus={() => setEditing(true)}
            onBlur={() => setEditing(false)}
            onChange={e => setDraft(e.target.value)}
            rows={2}
            placeholder="复用左侧 Prompt，或单独写一段视觉描述（构图、机位、光影、人物动作）"
            className="w-full px-2 py-1 bg-gray-900 border border-gray-700 rounded text-[11px] text-gray-200 placeholder-gray-600 focus:border-violet-500 focus:outline-none resize-none font-mono" />
          <div className="flex items-center gap-1.5">
            <select
              value={model}
              onChange={e => setModel(e.target.value)}
              className="px-1.5 py-0.5 bg-gray-900 border border-gray-700 rounded text-[10px] text-gray-300 focus:border-violet-500 focus:outline-none">
              <option value="gpt-image-2">gpt-image-2（构图王）</option>
              <option value="nano-banana-2">nano-banana-2</option>
              <option value="flux-pro">flux-pro</option>
              <option value="flux-2">flux-2</option>
            </select>
            <button
              onClick={() => onGenerate?.(draft || scene.prompt || scene.label, model)}
              disabled={isRunning || !onGenerate || !(draft || scene.prompt)}
              className="flex-1 px-2 py-1 rounded text-[10px] font-medium bg-violet-600 hover:bg-violet-500 text-white disabled:opacity-40 disabled:cursor-not-allowed transition inline-flex items-center justify-center gap-1">
              {isRunning ? <Loader2 className="w-3 h-3 animate-spin" /> : <Sparkles className="w-3 h-3" />}
              {isRunning ? '生成中…' : '生成故事板静帧'}
            </button>
          </div>
          {status === 'failed' && (
            <div className="text-[10px] text-red-400">⚠ 上次生成失败，重试一下</div>
          )}
        </>
      )}
    </div>
  )
}

function SceneCard({
  scene, expanded, sequenceNumber, onToggle, onUpdate, onRemove, onAddTake, onPickTake, onRemoveTake,
  onGenerateStoryboard,
  archiveProject, archiveEpKey,
}: {
  scene: SceneSpec
  expanded: boolean
  sequenceNumber: number
  onToggle: () => void
  onUpdate: (p: Partial<SceneSpec>) => void
  onRemove: () => void
  onAddTake: () => void
  onPickTake: (tid: string) => void
  onRemoveTake: (tid: string) => void
  onGenerateStoryboard?: (prompt: string, model?: string) => void
  archiveProject: string
  archiveEpKey: string
}) {
  const summary = sceneTakesSummary(scene)
  const pickedTake = scene.takes.find(t => t.take_id === scene.picked_take)

  return (
    <div className={`rounded-lg border transition ${summary.picked ? 'bg-emerald-900/10 border-emerald-500/30' : 'bg-gray-800/40 border-gray-700/50'}`}>
      {/* Header */}
      <div className="px-3 py-2 flex items-center gap-2">
        <button onClick={onToggle} className="text-gray-500 hover:text-gray-300 transition">
          <ChevronRight className={`w-3.5 h-3.5 transition ${expanded ? 'rotate-90' : ''}`} />
        </button>
        <div className={`flex items-center justify-center w-6 h-6 rounded text-[10px] font-bold ${summary.picked ? 'bg-emerald-500/30 text-emerald-300' : 'bg-gray-700 text-gray-400'}`}>
          {sequenceNumber}
        </div>
        <div className="flex-1 min-w-0">
          <input value={scene.label}
            onChange={e => onUpdate({ label: e.target.value })}
            className="w-full bg-transparent text-sm text-gray-200 font-medium outline-none focus:text-white" />
          <div className="flex items-center gap-1.5 mt-0.5 text-[10px] text-gray-500">
            <span className="font-mono">{scene.id}</span>
            <span>·</span>
            <span>{scene.duration}s</span>
            <span>·</span>
            <span>{summary.total} takes</span>
            {summary.picked && <span className="text-emerald-400">· ✓ 已选</span>}
            {summary.running > 0 && <span className="text-amber-400">· {summary.running} 进行中</span>}
          </div>
        </div>
        <button onClick={onRemove} className="p-1 text-gray-600 hover:text-red-400 transition opacity-0 group-hover:opacity-100">
          <Trash2 className="w-3 h-3" />
        </button>
      </div>

      {/* Picked take thumbnail (when collapsed) */}
      {!expanded && pickedTake && (
        <div className="px-3 pb-2">
          <TakeThumb take={pickedTake} isPicked small
            urls={takeUrlCandidates(pickedTake, archiveProject, archiveEpKey, scene.id)} />
        </div>
      )}

      {/* Expanded content */}
      {expanded && (
        <div className="px-3 pb-3 space-y-2 border-t border-gray-700/30 pt-2">
          {/* Prompt */}
          <div>
            <label className="block text-[10px] font-medium text-gray-500 mb-1 uppercase tracking-wider">Prompt</label>
            <textarea value={scene.prompt || ''}
              onChange={e => onUpdate({ prompt: e.target.value })}
              rows={2}
              placeholder="[图1]的林见月蜷在沙发上醒来，困惑茫然..."
              className="w-full px-2 py-1.5 bg-gray-900 border border-gray-700 rounded text-[11px] text-gray-200 placeholder-gray-600 focus:border-cyan-500 focus:outline-none resize-none font-mono" />
          </div>

          {/* 故事板静帧 (GPT Image 2) */}
          <StoryboardFrameSection
            scene={scene}
            onUpdate={onUpdate}
            onGenerate={onGenerateStoryboard}
          />

          {/* Duration */}
          <div className="flex items-center gap-2">
            <label className="text-[10px] text-gray-500 uppercase tracking-wider">时长</label>
            <input type="number" min={1} max={30} value={scene.duration}
              onChange={e => onUpdate({ duration: parseInt(e.target.value) || 8 })}
              className="w-14 px-2 py-0.5 bg-gray-900 border border-gray-700 rounded text-xs text-gray-200 focus:border-cyan-500 focus:outline-none" />
            <span className="text-[10px] text-gray-500">秒</span>
          </div>

          {/* Takes grid */}
          <div>
            <div className="flex items-center justify-between mb-1.5">
              <label className="text-[10px] font-medium text-gray-500 uppercase tracking-wider">Takes ({scene.takes.length})</label>
              <button onClick={onAddTake}
                className="text-[10px] text-cyan-400 hover:text-cyan-300 transition flex items-center gap-0.5">
                <Plus className="w-3 h-3" /> 新建 Take
              </button>
            </div>
            {scene.takes.length === 0 ? (
              <div className="text-[10px] text-gray-600 text-center py-3 border border-dashed border-gray-700 rounded">
                还没有生成 · 点「新建 Take」启动生产
              </div>
            ) : (
              <div className="grid grid-cols-2 gap-1.5">
                {scene.takes.map((take, idx) => (
                  <TakeCard
                    key={take.take_id}
                    take={take}
                    isPicked={scene.picked_take === take.take_id}
                    seqNum={idx + 1}
                    isNewest={idx === scene.takes.length - 1}
                    onPick={() => onPickTake(take.take_id)}
                    onRemove={() => onRemoveTake(take.take_id)}
                    urls={takeUrlCandidates(take, archiveProject, archiveEpKey, scene.id)}
                  />
                ))}
              </div>
            )}
          </div>

          {/* 历史版本（废片） */}
          {scene.rejected_takes && scene.rejected_takes.length > 0 && (
            <RejectedTakesSection rejected={scene.rejected_takes}
              archiveProject={archiveProject} archiveEpKey={archiveEpKey} sceneId={scene.id} />
          )}
        </div>
      )}
    </div>
  )
}

function RejectedTakesSection({ rejected, archiveProject, archiveEpKey, sceneId }: { rejected: Take[]; archiveProject: string; archiveEpKey: string; sceneId: string }) {
  const [open, setOpen] = useState(false)
  return (
    <div className="pt-2 border-t border-gray-700/40">
      <button onClick={() => setOpen(!open)}
        className="w-full flex items-center gap-1.5 text-[10px] text-gray-500 hover:text-gray-300 transition">
        <ChevronRight className={`w-3 h-3 transition ${open ? 'rotate-90' : ''}`} />
        <Archive className="w-3 h-3" />
        <span className="uppercase tracking-wider font-medium">历史版本 / 废片 ({rejected.length})</span>
      </button>
      {open && (
        <div className="mt-1.5 grid grid-cols-2 gap-1.5">
          {rejected.map(t => (
            <RejectedTakeItem key={t.take_id} take={t}
              urls={takeUrlCandidates(t, archiveProject, archiveEpKey, sceneId)} />
          ))}
        </div>
      )}
    </div>
  )
}

function RejectedTakeItem({ take, urls }: { take: Take; urls: string[] }) {
  const [idx, setIdx] = useState(0)
  const url = urls[idx] || ''
  return (
    <div className="group relative rounded border border-red-900/40 bg-red-950/20 overflow-hidden">
      <div className="h-16 bg-gray-900 relative">
        {url ? (
          <video key={url} src={url} muted loop className="w-full h-full object-cover opacity-60 hover:opacity-100 transition"
            onMouseEnter={e => (e.currentTarget as HTMLVideoElement).play()}
            onMouseLeave={e => { const v = e.currentTarget as HTMLVideoElement; v.pause(); v.currentTime = 0 }}
            onError={() => { if (idx < urls.length - 1) setIdx(i => i + 1) }} />
        ) : (
          <div className="w-full h-full flex items-center justify-center text-gray-600"><CircleX className="w-4 h-4" /></div>
        )}
        <span className="absolute top-0.5 left-0.5 px-1 py-0.5 rounded bg-red-900/80 text-red-200 text-[8px] font-bold">废</span>
      </div>
      <div className="px-1.5 py-1 bg-gray-850/80 text-[9px] text-gray-400 truncate" title={take.note}>
        <span className="font-mono text-red-300">{take.take_id}</span>
        {take.note && <span className="ml-1 text-gray-500">· {take.note}</span>}
      </div>
    </div>
  )
}

function ScriptTab({ data }: { data: EpisodeData }) {
  const [scriptMd, setScriptMd] = useState<string | null>(null)
  const [promptsMd, setPromptsMd] = useState<string | null>(null)
  const [which, setWhich] = useState<'script' | 'prompts'>('script')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  // 编剧 Agent 审稿状态
  const [reviewing, setReviewing] = useState(false)
  const [review, setReview] = useState<WriterReviewResponse | null>(null)
  const [reviewErr, setReviewErr] = useState<string | null>(null)
  const [reviewOpen, setReviewOpen] = useState(true)

  useEffect(() => {
    const s = data.script
    if (!s?.md && !s?.prompts_md) return
    setLoading(true); setError(null)
    Promise.all([
      s.md ? fetch(s.md).then(r => r.ok ? r.text() : Promise.reject(`HTTP ${r.status}`)).catch(e => { throw new Error(`剧本: ${e}`) }) : Promise.resolve(null),
      s.prompts_md ? fetch(s.prompts_md).then(r => r.ok ? r.text() : Promise.reject(`HTTP ${r.status}`)).catch(e => { throw new Error(`提示词: ${e}`) }) : Promise.resolve(null),
    ]).then(([a, b]) => { setScriptMd(a); setPromptsMd(b) })
      .catch(e => setError(e instanceof Error ? e.message : String(e)))
      .finally(() => setLoading(false))
  }, [data.script?.md, data.script?.prompts_md])

  const runWriterReview = async () => {
    setReviewing(true); setReviewErr(null); setReview(null); setReviewOpen(true)
    try {
      const meta = JSON.stringify({
        season: data.season, episode_number: data.episode_number,
        duration: data.duration, description: data.description,
        scenes: (data.scenes || []).map(s => ({ id: s.id, label: s.label, duration: s.duration })),
      })
      const res = await dramaAPI.writerReview({
        episode_label: data.label,
        episode_meta: meta,
        bible_url: '/v1/projects/swarm-universe/bible.md',
        script_url: data.script?.md,
        prompts_url: data.script?.prompts_md,
      })
      setReview(res.data)
    } catch (e) {
      const detail = (e as { response?: { data?: { error?: string } }; message?: string })
      setReviewErr(detail.response?.data?.error || detail.message || String(e))
    } finally { setReviewing(false) }
  }

  if (!data.script?.md && !data.script?.prompts_md) {
    return (
      <div className="p-6 text-center text-gray-500">
        <FileText className="w-8 h-8 mx-auto mb-2 opacity-40" />
        <p className="text-xs">本集暂无关联剧本 / 提示词文档</p>
        <p className="text-[10px] text-gray-600 mt-1">在 manifest.json 的 episodes[].script 里挂 md / prompts_md 路径即可</p>
      </div>
    )
  }

  const text = which === 'script' ? scriptMd : promptsMd

  return (
    <div className="p-3 space-y-2">
      <div className="flex gap-1 border-b border-gray-800 -mx-3 px-3 pb-2 items-center">
        <button onClick={() => setWhich('script')} disabled={!data.script?.md}
          className={`px-2.5 py-1 rounded text-[11px] font-medium transition ${which === 'script' ? 'bg-cyan-900/40 text-cyan-300 border border-cyan-700/50' : 'text-gray-500 hover:text-gray-300'} disabled:opacity-40`}>
          📜 剧本
        </button>
        <button onClick={() => setWhich('prompts')} disabled={!data.script?.prompts_md}
          className={`px-2.5 py-1 rounded text-[11px] font-medium transition ${which === 'prompts' ? 'bg-violet-900/40 text-violet-300 border border-violet-700/50' : 'text-gray-500 hover:text-gray-300'} disabled:opacity-40`}>
          ✨ 提示词总稿
        </button>
        <div className="flex-1" />
        <button onClick={runWriterReview} disabled={reviewing || loading}
          title="调用编剧 AI Agent 对本集剧本+提示词做 9 维度审稿并给出具体修改建议"
          className={`px-2.5 py-1 rounded text-[11px] font-semibold transition inline-flex items-center gap-1 ${reviewing ? 'bg-amber-900/40 text-amber-300 border border-amber-700/50 cursor-wait' : 'bg-emerald-900/40 text-emerald-300 border border-emerald-700/50 hover:bg-emerald-700/30'} disabled:opacity-50 disabled:cursor-not-allowed`}>
          {reviewing ? <CircleDot className="w-3 h-3 animate-spin" /> : <Wand2 className="w-3 h-3" />}
          {reviewing ? '审稿中…' : '编剧审'}
        </button>
      </div>

      {/* 审稿错误 */}
      {reviewErr && (
        <div className="p-2 rounded bg-red-900/20 border border-red-500/30 text-[11px] text-red-300">
          编剧审失败：{reviewErr}
        </div>
      )}

      {/* 审稿结果卡 */}
      {review && (
        <WriterReviewCard review={review} open={reviewOpen} onToggle={() => setReviewOpen(!reviewOpen)} />
      )}

      {loading && <div className="text-center py-8 text-xs text-gray-500"><CircleDot className="w-4 h-4 mx-auto animate-spin mb-1" />加载中</div>}
      {error && <div className="p-2 rounded bg-red-900/20 border border-red-500/30 text-[11px] text-red-300">加载失败：{error}</div>}
      {text && (
        <pre className="whitespace-pre-wrap break-words text-[11px] leading-relaxed text-gray-200 font-mono bg-gray-950/50 border border-gray-800 rounded p-3 max-h-[calc(100vh-280px)] overflow-y-auto">
          {text}
        </pre>
      )}
    </div>
  )
}

function WriterReviewCard({ review, open, onToggle }: { review: WriterReviewResponse; open: boolean; onToggle: () => void }) {
  const scoreColor = review.overall_score >= 80 ? 'text-emerald-300' : review.overall_score >= 60 ? 'text-amber-300' : 'text-red-300'
  const scoreRing = review.overall_score >= 80 ? 'border-emerald-500/40' : review.overall_score >= 60 ? 'border-amber-500/40' : 'border-red-500/40'
  const [copied, setCopied] = useState(false)

  const handleCopy = async (e: React.MouseEvent) => {
    // 按钮套在展开 header 里，按了别顺带 toggle 折叠
    e.stopPropagation()
    try {
      await navigator.clipboard.writeText(formatReviewAsMarkdown(review))
      setCopied(true)
      setTimeout(() => setCopied(false), 1800)
    } catch {
      // 剪贴板权限可能被拒（非 https / iframe）—— fallback 用选中复制走不通浏览器 API，
      // 此时安静失败，按钮不变绿即为信号；真要 debug 看控制台 clipboard 事件。
    }
  }

  return (
    <div className={`rounded-lg border ${scoreRing} bg-gray-900/60 overflow-hidden`}>
      <div className="w-full px-3 py-2 flex items-center gap-2 hover:bg-gray-800/50 transition">
        <button onClick={onToggle} className="flex items-center gap-2 text-left flex-1 min-w-0">
          <ChevronRight className={`w-3.5 h-3.5 text-gray-400 transition ${open ? 'rotate-90' : ''}`} />
          <Wand2 className="w-3.5 h-3.5 text-emerald-400 flex-none" />
          <div className="flex-1 min-w-0">
            <div className="flex items-center gap-2">
              <span className="text-[11px] font-semibold text-gray-200 uppercase tracking-wider">编剧审稿</span>
              <span className={`text-lg font-bold ${scoreColor}`}>{Math.round(review.overall_score)}</span>
              <span className="text-[10px] text-gray-500">/ 100</span>
            </div>
            <div className="text-[11px] text-gray-300 truncate">{review.verdict}</div>
          </div>
        </button>
        <span className="text-[10px] text-gray-600 font-mono">{review.model}</span>
        <button
          type="button"
          onClick={handleCopy}
          title="复制整份审稿为 Markdown（评分+关键问题+建议+镶场改写）"
          className={`flex-none p-1 rounded border transition ${
            copied
              ? 'border-emerald-500/50 bg-emerald-900/30 text-emerald-300'
              : 'border-gray-700 text-gray-400 hover:border-cyan-600 hover:text-cyan-300 hover:bg-gray-800/50'
          }`}
        >
          {copied ? <Check className="w-3.5 h-3.5" /> : <Copy className="w-3.5 h-3.5" />}
        </button>
      </div>

      {open && (
        <div className="border-t border-gray-800 p-3 space-y-3 text-[11px]">
          {/* 9 维度条形图 */}
          {review.dimensions?.length > 0 && (
            <div className="space-y-1">
              <div className="text-[10px] uppercase tracking-wider text-gray-500 font-medium mb-1">9 维度评分</div>
              {review.dimensions.map(d => (
                <div key={d.key} className="grid grid-cols-[100px_1fr_30px] gap-2 items-center">
                  <div className="text-gray-300 truncate" title={d.comment}>{d.label}</div>
                  <div className="h-1.5 bg-gray-800 rounded-full overflow-hidden">
                    <div
                      className={`h-full rounded-full transition-all ${d.score >= 80 ? 'bg-emerald-500' : d.score >= 60 ? 'bg-amber-500' : 'bg-red-500'}`}
                      style={{ width: `${Math.min(100, Math.max(0, d.score))}%` }}
                    />
                  </div>
                  <div className={`text-right font-mono tabular-nums ${d.score >= 80 ? 'text-emerald-300' : d.score >= 60 ? 'text-amber-300' : 'text-red-300'}`}>{Math.round(d.score)}</div>
                </div>
              ))}
            </div>
          )}

          {/* Top issues */}
          {review.top_issues?.length > 0 && (
            <div>
              <div className="text-[10px] uppercase tracking-wider text-gray-500 font-medium mb-1.5 flex items-center gap-1">
                <AlertTriangle className="w-3 h-3 text-red-400" /> 关键问题 ({review.top_issues.length})
              </div>
              <div className="space-y-1.5">
                {review.top_issues.map((it, i) => (
                  <div key={i} className={`rounded border px-2 py-1.5 ${it.severity === 'high' ? 'border-red-500/40 bg-red-900/10' : it.severity === 'medium' ? 'border-amber-500/40 bg-amber-900/10' : 'border-gray-700 bg-gray-800/40'}`}>
                    <div className="flex items-center gap-1.5">
                      <span className={`text-[9px] px-1 py-0.5 rounded font-semibold ${it.severity === 'high' ? 'bg-red-500/30 text-red-200' : it.severity === 'medium' ? 'bg-amber-500/30 text-amber-200' : 'bg-gray-700 text-gray-300'}`}>
                        {it.severity.toUpperCase()}
                      </span>
                      <span className="text-[10px] text-gray-400 font-mono">{it.where}</span>
                    </div>
                    <div className="text-gray-200 mt-1">{it.problem}</div>
                    <div className="text-gray-500 mt-0.5 text-[10px]">→ {it.why}</div>
                  </div>
                ))}
              </div>
            </div>
          )}

          {/* Suggestions */}
          {review.suggestions?.length > 0 && (
            <div>
              <div className="text-[10px] uppercase tracking-wider text-gray-500 font-medium mb-1.5 flex items-center gap-1">
                <Lightbulb className="w-3 h-3 text-amber-400" /> 修改建议 ({review.suggestions.length})
              </div>
              <div className="space-y-1.5">
                {review.suggestions.map((s, i) => (
                  <div key={i} className="rounded border border-gray-700 bg-gray-800/40 px-2 py-1.5">
                    <div className="flex items-center gap-1.5 text-[10px] text-gray-400">
                      <span className="font-mono">{s.where}</span>
                      <span>·</span>
                      <span className="text-violet-300">{s.action}</span>
                    </div>
                    {s.original && (
                      <div className="text-[10px] text-gray-500 mt-1 line-through">原: {s.original}</div>
                    )}
                    <div className="text-gray-200 mt-0.5">新: {s.revised}</div>
                    <div className="text-gray-500 mt-0.5 text-[10px]">因: {s.reason}</div>
                  </div>
                ))}
              </div>
            </div>
          )}

          {/* Rewrite hints — 镶场 */}
          {review.rewrite_hints?.length > 0 && (
            <div>
              <div className="text-[10px] uppercase tracking-wider text-gray-500 font-medium mb-1.5 flex items-center gap-1">
                <Wrench className="w-3 h-3 text-cyan-400" /> 镶场改写 ({review.rewrite_hints.length})
              </div>
              <div className="space-y-1.5">
                {review.rewrite_hints.map((h, i) => (
                  <div key={i} className="rounded border border-cyan-800/40 bg-cyan-950/20 px-2 py-1.5">
                    <div className="flex items-center gap-1.5 text-[10px]">
                      <span className="font-mono text-cyan-300">{h.scene_id}</span>
                      <span className="text-gray-500">·</span>
                      <span className="text-gray-400">{h.field}</span>
                    </div>
                    <div className="text-[10px] text-gray-500 mt-1">Before: {h.before}</div>
                    <div className="text-gray-200 mt-0.5">After: {h.after}</div>
                    <div className="text-gray-500 mt-0.5 text-[10px]">→ {h.rationale}</div>
                  </div>
                ))}
              </div>
            </div>
          )}
        </div>
      )}
    </div>
  )
}

// formatReviewAsMarkdown 把审稿结果拼成一段可粘贴到微信/飞书/文档的 markdown。
// 重点是「该复制什么」：结论一行 + 9 维度得分表 + 关键问题 + 建议 + 镶场改写 before/after。
// 不复制 model 元数据（那是调试信息），也不复制 dimensions.comment 的长文字（避免太长）。
function formatReviewAsMarkdown(review: WriterReviewResponse): string {
  const lines: string[] = []
  lines.push(`# ${review.episode_label} · 编剧审稿`)
  lines.push('')
  lines.push(`**${review.overall_score} / 100** — ${review.verdict}`)
  lines.push('')

  if (review.dimensions?.length) {
    lines.push('## 9 维度评分')
    lines.push('')
    lines.push('| 维度 | 分 | 评语 |')
    lines.push('|------|---:|------|')
    for (const d of review.dimensions) {
      const safeComment = (d.comment || '').replace(/\|/g, '\\|').replace(/\r?\n/g, ' ')
      lines.push(`| ${d.label} | ${Math.round(d.score)} | ${safeComment} |`)
    }
    lines.push('')
  }

  if (review.top_issues?.length) {
    lines.push(`## 关键问题（${review.top_issues.length}）`)
    lines.push('')
    for (const it of review.top_issues) {
      lines.push(`- **[${it.severity.toUpperCase()}] ${it.where}** — ${it.problem}`)
      if (it.why) lines.push(`  - 原因：${it.why}`)
    }
    lines.push('')
  }

  if (review.suggestions?.length) {
    lines.push(`## 修改建议（${review.suggestions.length}）`)
    lines.push('')
    for (const s of review.suggestions) {
      lines.push(`### ${s.where} · ${s.action}`)
      if (s.original) lines.push(`- 原：${s.original}`)
      lines.push(`- 新：${s.revised}`)
      if (s.reason) lines.push(`- 因：${s.reason}`)
      lines.push('')
    }
  }

  if (review.rewrite_hints?.length) {
    lines.push(`## 镶场改写（${review.rewrite_hints.length}）`)
    lines.push('')
    for (const h of review.rewrite_hints) {
      lines.push(`### ${h.scene_id} · ${h.field}`)
      lines.push(`- Before：${h.before}`)
      lines.push(`- After：${h.after}`)
      if (h.rationale) lines.push(`- 为什么：${h.rationale}`)
      lines.push('')
    }
  }

  lines.push(`---`)
  lines.push(`_${review.model} · ${review.provider} · ${review.generated_at}_`)
  return lines.join('\n')
}

// ── 日志面板 · 按时间顺序列出本集所有 Take 的 Seedance 调用上下文 ──
// 已从右侧 tab 移至画布底部，导出给 WorkflowPage 的 BottomLogsDock 使用。
export function EpisodeLogsPane({ data }: { data: EpisodeData }) {
  const [expanded, setExpanded] = useState<Set<string>>(new Set())
  const scenes = data.scenes || []
  // 展开所有 take 成扁平时间线
  type Entry = { sceneId: string; sceneLabel: string; take: Take }
  const entries: Entry[] = []
  for (const s of scenes) {
    for (const t of (s.takes || [])) {
      entries.push({ sceneId: s.id, sceneLabel: s.label || '', take: t })
    }
  }
  // 排序：running 置顶，其余按 created_at 倒序（最新在上）
  const statusPriority: Record<string, number> = { running: 0, pending: 1, succeeded: 2, failed: 3 }
  entries.sort((a, b) => {
    const sa = statusPriority[a.take.status] ?? 9
    const sb = statusPriority[b.take.status] ?? 9
    if (sa !== sb) return sa - sb
    return (b.take.created_at || '').localeCompare(a.take.created_at || '')
  })

  // running 条目默认展开
  const runningKeys = new Set(entries.filter(e => e.take.status === 'running').map(e => `${e.sceneId}.${e.take.take_id}`))

  const total = entries.length
  const succeeded = entries.filter(e => e.take.status === 'succeeded').length
  const failed = entries.filter(e => e.take.status === 'failed').length
  const running = entries.filter(e => e.take.status === 'running').length

  const toggle = (key: string) => {
    setExpanded(prev => {
      const next = new Set(prev)
      if (next.has(key)) next.delete(key); else next.add(key)
      return next
    })
  }

  const copyText = async (text: string) => {
    try { await navigator.clipboard.writeText(text) } catch { /* ignore */ }
  }

  const fmtTime = (iso?: string) => {
    if (!iso) return '-'
    try {
      const d = new Date(iso)
      return `${d.getHours().toString().padStart(2, '0')}:${d.getMinutes().toString().padStart(2, '0')}:${d.getSeconds().toString().padStart(2, '0')}`
    } catch { return iso.slice(11, 19) }
  }
  const fmtDuration = (t: Take) => {
    if (!t.created_at || !t.finished_at) return ''
    try {
      const ms = new Date(t.finished_at).getTime() - new Date(t.created_at).getTime()
      if (ms < 1000) return `${ms}ms`
      return `${(ms / 1000).toFixed(1)}s`
    } catch { return '' }
  }

  if (total === 0) {
    return (
      <div className="p-6 text-center text-gray-500">
        <Terminal className="w-8 h-8 mx-auto mb-2 opacity-40" />
        <p className="text-xs">暂无生产日志</p>
        <p className="text-[10px] text-gray-600 mt-1">点击「开始生产 {data.label}」后，每个 Take 的 Seedance 调用记录会汇总到这里</p>
      </div>
    )
  }

  return (
    <div className="p-3 space-y-2">
      {/* 统计条 */}
      <div className="grid grid-cols-4 gap-1 text-center">
        <div className="rounded bg-gray-900/60 border border-gray-800 py-1">
          <div className="text-[9px] text-gray-500 uppercase">总计</div>
          <div className="text-sm font-bold text-gray-200">{total}</div>
        </div>
        <div className="rounded bg-emerald-950/40 border border-emerald-900/50 py-1">
          <div className="text-[9px] text-emerald-400/80 uppercase">成功</div>
          <div className="text-sm font-bold text-emerald-300">{succeeded}</div>
        </div>
        <div className="rounded bg-amber-950/40 border border-amber-900/50 py-1">
          <div className="text-[9px] text-amber-400/80 uppercase">进行中</div>
          <div className="text-sm font-bold text-amber-300">{running}</div>
        </div>
        <div className="rounded bg-red-950/40 border border-red-900/50 py-1">
          <div className="text-[9px] text-red-400/80 uppercase">失败</div>
          <div className="text-sm font-bold text-red-300">{failed}</div>
        </div>
      </div>

      {/* 时间线 */}
      <div className="space-y-1.5">
        {entries.map(({ sceneId, sceneLabel, take }) => {
          const key = `${sceneId}.${take.take_id}`
          const isOpen = expanded.has(key) || runningKeys.has(key)
          const Icon = TAKE_STATUS_ICON[take.status]
          const statusTint = take.status === 'succeeded' ? 'border-emerald-700/40 bg-emerald-950/20'
            : take.status === 'running' ? 'border-amber-700/40 bg-amber-950/20 animate-pulse'
            : take.status === 'failed' ? 'border-red-700/40 bg-red-950/20'
            : 'border-gray-700 bg-gray-900/40'
          return (
            <div key={key} className={`rounded border ${statusTint} overflow-hidden`}>
              <button onClick={() => toggle(key)}
                className="w-full px-2 py-1.5 flex items-center gap-2 text-left hover:bg-gray-800/30 transition">
                <ChevronRight className={`w-3 h-3 text-gray-500 transition ${isOpen ? 'rotate-90' : ''}`} />
                <Icon className={`w-3 h-3 ${take.status === 'succeeded' ? 'text-emerald-400' : take.status === 'failed' ? 'text-red-400' : take.status === 'running' ? 'text-amber-400' : 'text-gray-400'}`} />
                <span className="text-[11px] font-mono text-cyan-300">{sceneId}.{take.take_id.replace(/^t/, '')}</span>
                <span className="text-[10px] text-gray-400 truncate flex-1">{sceneLabel}</span>
                <span className="text-[10px] text-gray-500 font-mono">{fmtTime(take.created_at)}</span>
                {take.finished_at && <span className="text-[10px] text-gray-600 font-mono">Δ{fmtDuration(take)}</span>}
              </button>

              {isOpen && (
                <div className="border-t border-gray-800 px-2 py-2 space-y-1.5 text-[10px]">
                  {/* Basic */}
                  <LogRow label="模型" value={take.model || 'doubao-seedance-2-0-260128'} mono onCopy={copyText} />
                  {take.task_id && <LogRow label="Task ID" value={take.task_id} mono onCopy={copyText} />}
                  {typeof take.duration === 'number' && <LogRow label="时长" value={`${take.duration}s`} onCopy={copyText} />}
                  {take.ref_image_url && (() => {
                    const urls = take.ref_image_url!.split(',').map(u => u.trim()).filter(Boolean)
                    return urls.length <= 1
                      ? <LogRow label="角色参考图" value={urls[0] || ''} link onCopy={copyText} />
                      : (
                        <div className="flex items-start gap-2">
                          <span className="text-gray-500 w-20 flex-shrink-0 uppercase text-[9px] tracking-wider pt-0.5">角色参考图</span>
                          <div className="flex-1 min-w-0 space-y-0.5">
                            {urls.map((u, i) => (
                              <div key={i} className="flex items-center gap-1">
                                <span className="text-gray-600 text-[9px] font-mono w-4 flex-shrink-0">[{i + 1}]</span>
                                <a href={u} target="_blank" rel="noreferrer" className="text-cyan-400 hover:text-cyan-300 underline text-[10px] truncate">{u.length > 70 ? u.slice(0, 70) + '…' : u}</a>
                                <ExternalLink className="w-2.5 h-2.5 text-cyan-400 flex-shrink-0" />
                              </div>
                            ))}
                          </div>
                          <button onClick={() => copyText(take.ref_image_url || '')} className="text-gray-500 hover:text-gray-300 transition flex-shrink-0 mt-0.5">
                            <Copy className="w-2.5 h-2.5" />
                          </button>
                        </div>
                      )
                  })()}
                  {take.ref_video_url && <LogRow label="上场尾帧视频" value={take.ref_video_url} link onCopy={copyText} />}
                  {take.ref_video_id && <LogRow label="上场 VideoRecord" value={take.ref_video_id} mono onCopy={copyText} />}
                  {take.video_url && <LogRow label="本场产出 (TOS)" value={take.video_url} link onCopy={copyText} />}
                  {take.local_url && <LogRow label="本地归档" value={take.local_url} link onCopy={copyText} />}
                  {take.local_path && <LogRow label="剧本目录路径" value={take.local_path} mono onCopy={copyText} />}
                  {take.lastframe_url && <LogRow label="本场尾帧" value={take.lastframe_url} link onCopy={copyText} />}

                  {/* Prompt */}
                  {take.prompt && (
                    <div className="mt-1.5">
                      <div className="flex items-center gap-2 mb-0.5">
                        <span className="text-gray-500 uppercase text-[9px] tracking-wider">Prompt ({take.prompt.length}字)</span>
                        <button onClick={() => copyText(take.prompt || '')} className="text-gray-500 hover:text-gray-300 transition">
                          <Copy className="w-2.5 h-2.5" />
                        </button>
                      </div>
                      <pre className="whitespace-pre-wrap break-words text-gray-300 bg-gray-950/70 border border-gray-800 rounded p-1.5 max-h-40 overflow-y-auto font-mono text-[10px] leading-relaxed">
                        {take.prompt}
                      </pre>
                    </div>
                  )}

                  {/* POST 请求体 */}
                  {take.request_body && (
                    <div className="mt-1.5">
                      <div className="flex items-center gap-2 mb-0.5">
                        <span className="text-gray-500 uppercase text-[9px] tracking-wider">POST 参数</span>
                        <button onClick={() => copyText(JSON.stringify(take.request_body, null, 2))} className="text-gray-500 hover:text-gray-300 transition">
                          <Copy className="w-2.5 h-2.5" />
                        </button>
                      </div>
                      <pre className="whitespace-pre-wrap break-words text-gray-400 bg-gray-950/70 border border-gray-800 rounded p-1.5 max-h-32 overflow-y-auto font-mono text-[10px] leading-relaxed">
                        {JSON.stringify(Object.fromEntries(Object.entries(take.request_body).filter(([k]) => k !== 'prompt')), null, 2)}
                      </pre>
                    </div>
                  )}

                  {/* Note / error */}
                  {take.note && (
                    <div className={`mt-1 rounded p-1.5 text-[10px] ${take.status === 'failed' ? 'bg-red-900/30 border border-red-800/50 text-red-200' : 'bg-gray-800/50 border border-gray-700 text-gray-300'}`}>
                      <span className="text-gray-500 uppercase text-[9px] tracking-wider">{take.status === 'failed' ? '错误' : '备注'}：</span>
                      {take.note}
                    </div>
                  )}
                </div>
              )}
            </div>
          )
        })}
      </div>
    </div>
  )
}

function LogRow({ label, value, mono, link, onCopy }: { label: string; value: string; mono?: boolean; link?: boolean; onCopy: (t: string) => void }) {
  return (
    <div className="flex items-start gap-2">
      <span className="text-gray-500 w-20 flex-shrink-0 uppercase text-[9px] tracking-wider pt-0.5">{label}</span>
      <div className={`flex-1 min-w-0 ${mono ? 'font-mono' : ''} text-gray-300 break-all`}>
        {link ? <a href={value} target="_blank" rel="noreferrer" className="text-cyan-400 hover:text-cyan-300 underline inline-flex items-center gap-1">{value.length > 80 ? value.slice(0, 80) + '…' : value}<ExternalLink className="w-2.5 h-2.5" /></a> : value}
      </div>
      <button onClick={() => onCopy(value)} className="text-gray-500 hover:text-gray-300 transition flex-shrink-0 mt-0.5">
        <Copy className="w-2.5 h-2.5" />
      </button>
    </div>
  )
}

function TakeThumb({ take, isPicked, small, seqNum, isNewest, urls = [] }: { take: Take; isPicked?: boolean; small?: boolean; seqNum?: number; isNewest?: boolean; urls?: string[] }) {
  const Icon = TAKE_STATUS_ICON[take.status]
  const [idx, setIdx] = useState(0)
  const url = urls[idx] || ''
  return (
    <div className={`relative rounded overflow-hidden ${small ? 'h-12' : 'h-20'} ${TAKE_STATUS_COLOR[take.status]} border`}>
      {url ? (
        <video key={url} src={url} muted className="w-full h-full object-cover"
          onError={() => { if (idx < urls.length - 1) setIdx(i => i + 1) }} />
      ) : (
        <div className="w-full h-full flex items-center justify-center">
          <ImageIcon className="w-4 h-4 opacity-40" />
        </div>
      )}
      <div className="absolute top-0.5 left-0.5 flex items-center gap-0.5 px-1 py-0.5 rounded bg-black/60 text-[9px]">
        <Icon className="w-2.5 h-2.5" />
        {seqNum ?? take.take_id}
        {isNewest && <span className="px-1 rounded bg-cyan-500 text-white text-[7px] font-bold leading-tight">NEW</span>}
      </div>
      {isPicked && (
        <div className="absolute top-0.5 right-0.5 px-1 py-0.5 rounded bg-emerald-500 text-white text-[9px] font-bold flex items-center gap-0.5">
          <Check className="w-2.5 h-2.5" /> 选
        </div>
      )}
    </div>
  )
}

/**
 * 场景面板的 take 卡片：
 * - 单击视频区域 → 就地带声播放/暂停（默认第一次尝试带声，被拦再降级静音）
 * - 双击 → 打开全屏 VideoPreviewModal
 * - 右上角「⛶」按钮 → 手动打开全屏（覆盖 hover 出现）
 * - 底栏「选」按钮 → pick 这个 take（与播放解耦，不会误触）
 * - 底栏「🗑」按钮 → 删除
 */
function TakeCard({ take, isPicked, seqNum, isNewest, onPick, onRemove, urls = [] }: { take: Take; isPicked: boolean; seqNum?: number; isNewest?: boolean; onPick: () => void; onRemove: () => void; urls?: string[] }) {
  const Icon = TAKE_STATUS_ICON[take.status]
  const [urlIdx, setUrlIdx] = useState(0)
  const url = urls[urlIdx] || take.local_url || take.video_url || ''
  const videoRef = useRef<HTMLVideoElement | null>(null)
  const [playing, setPlaying] = useState(false)
  const [preview, setPreview] = useState(false)

  const toggle = (e: React.MouseEvent) => {
    e.stopPropagation()
    const v = videoRef.current
    if (!v || !url) return
    if (v.paused) {
      v.muted = false
      v.play().catch(() => {
        v.muted = true
        void v.play().catch(() => { /* noop */ })
      })
      setPlaying(true)
    } else {
      v.pause()
      setPlaying(false)
    }
  }

  return (
    <div className={`group relative rounded border overflow-hidden transition ${
      isPicked ? 'border-emerald-500 ring-1 ring-emerald-500/40' : `${TAKE_STATUS_COLOR[take.status]}`
    }`}>
      <div className="h-20 bg-gray-900 relative cursor-pointer"
           onClick={toggle}
           onDoubleClick={(e) => { e.stopPropagation(); if (url) setPreview(true) }}
           title={url ? '单击播放/暂停（带声）· 双击放大预览' : ''}>
        {url ? (
          <video key={url} ref={videoRef} src={url} className="w-full h-full object-cover"
                 preload="metadata" playsInline
                 onPlay={() => setPlaying(true)}
                 onPause={() => setPlaying(false)}
                 onEnded={() => setPlaying(false)}
                 onError={() => { if (urlIdx < urls.length - 1) setUrlIdx(i => i + 1) }} />
        ) : (
          <div className="w-full h-full flex items-center justify-center text-gray-600">
            {take.status === 'running' ? (
              <div className="animate-pulse text-[10px]">生产中...</div>
            ) : take.status === 'failed' ? (
              <CircleAlert className="w-5 h-5" />
            ) : (
              <ImageIcon className="w-5 h-5 opacity-40" />
            )}
          </div>
        )}
        {/* 中央播放提示：未播放且 hover 时浮现 */}
        {!playing && url && (
          <div className="absolute inset-0 flex items-center justify-center pointer-events-none opacity-0 group-hover:opacity-100 transition">
            <PlayCircle className="w-8 h-8 text-white/80 drop-shadow-lg" />
          </div>
        )}
        {/* 右上角放大按钮（在 hover 时出现，不挡删除按钮） */}
        {url && (
          <button onClick={(e) => { e.stopPropagation(); setPreview(true) }}
                  title="放大预览"
                  className="absolute top-0.5 left-0.5 p-0.5 rounded bg-black/60 hover:bg-black/90 text-white opacity-0 group-hover:opacity-100 transition">
            <Maximize2 className="w-2.5 h-2.5" />
          </button>
        )}
      </div>
      <div className="px-1.5 py-1 bg-gray-850/80 flex items-center gap-1 text-[10px]">
        <Icon className="w-2.5 h-2.5 flex-shrink-0" />
        <span className="font-mono truncate flex-1 flex items-center gap-1">{seqNum ?? take.take_id}{isNewest && <span className="px-1 rounded bg-cyan-500 text-white text-[7px] font-bold leading-tight">NEW</span>}</span>
        {!isPicked && take.status === 'succeeded' && (
          <button onClick={(e) => { e.stopPropagation(); onPick() }}
            className="px-1.5 py-0.5 rounded bg-emerald-600/30 hover:bg-emerald-600 text-emerald-200 hover:text-white text-[9px] font-semibold transition">
            选
          </button>
        )}
        {isPicked && (
          <span className="px-1.5 py-0.5 rounded bg-emerald-500 text-white text-[9px] font-bold flex items-center gap-0.5">
            <Check className="w-2 h-2" /> 选中
          </span>
        )}
      </div>
      <button onClick={(e) => { e.stopPropagation(); onRemove() }}
        className="absolute top-0.5 right-0.5 p-0.5 rounded bg-black/60 text-gray-400 hover:text-red-400 opacity-0 group-hover:opacity-100 transition">
        <Trash2 className="w-2.5 h-2.5" />
      </button>
      {take.note && (
        <div className="absolute bottom-6 left-0 right-0 px-1.5 py-0.5 bg-black/70 text-[9px] text-gray-300 truncate pointer-events-none">
          {take.note}
        </div>
      )}
      {preview && url && (
        <VideoPreviewModal
          open
          src={url}
          title={seqNum ? `#${seqNum}` : take.take_id}
          subtitle={take.note}
          onClose={() => setPreview(false)}
        />
      )}
    </div>
  )
}

function CompositionTab({ data, comp, scenes, onUpdate }: { data: EpisodeData; comp: Composition; scenes: SceneSpec[]; onUpdate: (c: Composition) => void }) {
  const pickedScenes = scenes.filter(s => s.picked_take)
  const missingCount = scenes.length - pickedScenes.length
  // 与外层 EpisodeWorkflowPanel 保持一致的归档目录派生
  const archiveProject = 'swarm-universe'
  const archiveEpKey = `${data.is_spinoff ? 'sp' : 'ep'}${String(data.episode_number || 0).padStart(2, '0')}`

  // 推广文案生成状态
  const [promoLoading, setPromoLoading] = useState(false)
  const [promo, setPromo] = useState<PromoResponse | null>(null)
  const [promoErr, setPromoErr] = useState<string | null>(null)

  // Fix #9：发布到剧本目录状态。
  //  从 label 推出 episode 目录名（"EP05 夜袭" → "ep05"）。如果命中不到就禁用按钮。
  const epId = (() => {
    const m = /\bEP(\d{1,3})\b/i.exec(data.label || '')
    return m ? `ep${m[1].padStart(2, '0').toLowerCase()}` : ''
  })()
  const epTitle = (() => {
    // "EP05 夜袭" → "夜袭"
    const s = (data.label || '').replace(/^EP\d{1,3}\s*/i, '').trim()
    return s || data.label
  })()
  const [publishLoading, setPublishLoading] = useState(false)
  const [publishResult, setPublishResult] = useState<{ published: number; missing: number; dir: string; readme: string } | null>(null)
  const [publishErr, setPublishErr] = useState<string | null>(null)
  const runPublish = async () => {
    setPublishLoading(true); setPublishErr(null); setPublishResult(null)
    try {
      const picked = pickedScenes.map(s => ({ scene: s.id, take_id: s.picked_take as string }))
      const res = await videoAPI.publishEpisode({
        project: 'swarm-universe',
        episode: epId,
        picked,
        title: epTitle,
        description: data.description,
      })
      const r = res.data as { published?: unknown[]; missing?: unknown[]; episode_dir?: string; readme?: string }
      setPublishResult({
        published: (r.published || []).length,
        missing: (r.missing || []).length,
        dir: r.episode_dir || `/episodes/${epId}`,
        readme: r.readme || `/episodes/${epId}/README.md`,
      })
    } catch (e) {
      const detail = (e as { response?: { data?: { error?: string } }; message?: string })
      setPublishErr(detail.response?.data?.error || detail.message || String(e))
    } finally {
      setPublishLoading(false)
    }
  }

  const runPromo = async () => {
    setPromoLoading(true); setPromoErr(null); setPromo(null)
    try {
      const meta = JSON.stringify({
        season: data.season, episode_number: data.episode_number,
        duration: data.duration, description: data.description,
        scenes_count: scenes.length,
      })
      const res = await dramaAPI.generatePromo({
        episode_label: data.label,
        episode_meta: meta,
        bible_url: '/v1/projects/swarm-universe/bible.md',
        script_url: data.script?.md,
        prompts_url: data.script?.prompts_md,
        cover_url: data.cover_url,
        final_video_url: comp.final_video_url,
        picked_clips: comp.picked_clips,
      })
      setPromo(res.data)
    } catch (e) {
      const detail = (e as { response?: { data?: { error?: string; raw?: string } }; message?: string })
      setPromoErr(detail.response?.data?.error || detail.message || String(e))
    } finally { setPromoLoading(false) }
  }

  // 合成成片
  const [mergeLoading, setMergeLoading] = useState(false)
  const [mergeErr, setMergeErr] = useState<string | null>(null)
  const [mergeMsg, setMergeMsg] = useState<string | null>(null)
  const runMerge = async () => {
    setMergeLoading(true); setMergeErr(null); setMergeMsg(null)
    try {
      // 收集每个 picked take 的 task_id（按场景顺序）
      const taskIds: string[] = []
      for (const s of scenes) {
        const take = s.takes.find(t => t.take_id === s.picked_take)
        if (!take?.task_id) { setMergeErr(`场景 ${s.id} 没有 picked take 或缺少 task_id`); return }
        taskIds.push(take.task_id)
      }
      const res = await videoAPI.merge(taskIds, data.label, {
        season: data.season || 1,
        episode_number: data.episode_number || 0,
        title: (data.label || '').replace(/^EP\d{1,3}\s*/i, '').trim(),
      })
      const d = res.data as { status?: string; message?: string; conv_id?: string }
      setMergeMsg(d.message || '合成已开始')
      onUpdate({ ...comp, status: 'generating' })
      // 轮询等待合成完成
      const convId = d.conv_id || `workflow-${data.label}`
      const poll = async () => {
        for (let i = 0; i < 60; i++) {
          await new Promise(r => setTimeout(r, 3000))
          try {
            const listRes = await videoAPI.list({ model: 'merged' })
            const vids = (listRes.data as { videos?: Array<{ conversation_id?: string; video_url?: string; status?: string }> }).videos || []
            const merged = vids.find(v => v.conversation_id === convId && v.status === 'succeeded' && v.video_url)
            if (merged) {
              onUpdate({ ...comp, status: 'ready', final_video_url: merged.video_url })
              setMergeMsg(`合成完成！${merged.video_url}`)
              // 自动同步到剧本目录：把 picked takes 拷到 /episodes/<ep>/scenes/ 并写 README。
              // 不阻塞 / 失败也不影响合成成功。
              if (epId && pickedScenes.length > 0) {
                void runPublish().catch(() => { /* 已在 runPublish 内显示错误 */ })
              }
              return
            }
          } catch { /* retry */ }
        }
        setMergeMsg('合成超时，请稍后在视频库检查')
      }
      poll()
    } catch (e) {
      const detail = (e as { response?: { data?: { error?: string } }; message?: string })
      setMergeErr(detail.response?.data?.error || detail.message || String(e))
    } finally {
      setMergeLoading(false)
    }
  }

  const copy = async (text: string) => {
    try { await navigator.clipboard.writeText(text) } catch { /* ignore */ }
  }

  return (
    <div className="p-3 space-y-3">
      {/* Pipeline chain */}
      <div>
        <label className="block text-[10px] font-medium text-gray-500 mb-2 uppercase tracking-wider">合成链路</label>
        {scenes.length === 0 ? (
          <div className="text-center py-6 text-xs text-gray-600">还没有场景</div>
        ) : (
          <div className="space-y-1.5">
            {scenes.map((scene, i) => {
              const take = scene.takes.find(t => t.take_id === scene.picked_take)
              return (
                <div key={scene.id} className="flex items-center gap-2 text-xs">
                  <div className={`flex items-center justify-center w-6 h-6 rounded text-[10px] font-bold flex-shrink-0 ${take ? 'bg-emerald-500/30 text-emerald-300' : 'bg-red-500/20 text-red-400'}`}>
                    {i + 1}
                  </div>
                  <div className="flex-1 min-w-0 flex items-center gap-1.5">
                    <span className="font-mono text-[11px] text-gray-400">{scene.id}</span>
                    <span className="text-gray-300 truncate">{scene.label}</span>
                    {take ? (
                      <span className="px-1.5 py-0.5 rounded bg-emerald-500/20 text-emerald-300 text-[9px] font-mono">
                        {take.take_id}
                      </span>
                    ) : (
                      <span className="px-1.5 py-0.5 rounded bg-red-500/20 text-red-400 text-[9px]">
                        未选
                      </span>
                    )}
                  </div>
                  <span className="text-[10px] text-gray-500">{scene.duration}s</span>
                </div>
              )
            })}
            {/* BGM + Final */}
            <div className="pt-2 mt-2 border-t border-gray-700/50 space-y-1.5">
              <div className="flex items-center gap-2 text-xs">
                <div className="flex items-center justify-center w-6 h-6 rounded bg-violet-500/30 text-violet-300 flex-shrink-0">
                  <Music className="w-3 h-3" />
                </div>
                <div className="flex-1 flex items-center gap-1.5">
                  <span className="text-gray-300">BGM</span>
                  {comp.bgm_id ? (
                    <span className="px-1.5 py-0.5 rounded bg-violet-500/20 text-violet-300 text-[9px] font-mono">
                      {comp.bgm_id.slice(0, 12)}
                    </span>
                  ) : (
                    <span className="text-[10px] text-gray-500">（可选）</span>
                  )}
                </div>
              </div>
              <div className="flex items-center gap-2 text-xs">
                <div className={`flex items-center justify-center w-6 h-6 rounded flex-shrink-0 ${comp.final_video_url ? 'bg-emerald-500/30 text-emerald-300' : 'bg-gray-700 text-gray-500'}`}>
                  <Scissors className="w-3 h-3" />
                </div>
                <div className="flex-1">
                  <span className="text-gray-300">最终合成</span>
                  {comp.final_video_url && (
                    <span className="ml-1 text-[10px] text-emerald-400">· 就绪</span>
                  )}
                </div>
              </div>
            </div>
          </div>
        )}
      </div>

      {/* Missing warning */}
      {missingCount > 0 && (
        <div className="p-2.5 rounded-lg bg-red-900/20 border border-red-500/30 flex items-start gap-2">
          <CircleAlert className="w-4 h-4 text-red-400 flex-shrink-0 mt-0.5" />
          <div className="text-[11px] text-red-300">
            还有 <span className="font-semibold">{missingCount}</span> 个场景未选 take，无法合成。请先在"场景" tab 为每个场景选一个 take。
          </div>
        </div>
      )}

      {/* 合成成片按钮 */}
      <div className="pt-2">
        <button onClick={runMerge}
          disabled={missingCount > 0 || mergeLoading}
          className="w-full px-3 py-2.5 text-sm font-semibold rounded-lg bg-gradient-to-r from-violet-600 to-cyan-600 text-white hover:from-violet-500 hover:to-cyan-500 disabled:opacity-40 disabled:cursor-not-allowed transition flex items-center justify-center gap-2 shadow-lg">
          {mergeLoading ? (
            <><Loader2 className="w-4 h-4 animate-spin" /> 合成中…</>
          ) : (
            <><Scissors className="w-4 h-4" /> 合成成片（{scenes.length} 个片段）</>
          )}
        </button>
        {mergeMsg && (
          <div className="mt-1.5 p-2 rounded bg-emerald-900/20 border border-emerald-500/30 text-[11px] text-emerald-300">
            {mergeMsg}
          </div>
        )}
        {mergeErr && (
          <div className="mt-1.5 p-2 rounded bg-red-900/20 border border-red-500/30 text-[11px] text-red-300">
            {mergeErr}
          </div>
        )}
      </div>

      {/* BGM picker */}
      <div>
        <label className="block text-[10px] font-medium text-gray-500 mb-1 uppercase tracking-wider">BGM ID（音乐总监生成的 music_id）</label>
        <input value={comp.bgm_id || ''}
          onChange={e => onUpdate({ ...comp, bgm_id: e.target.value })}
          placeholder="music_xxx"
          className="w-full px-2 py-1.5 bg-gray-800 border border-gray-700 rounded text-xs font-mono text-gray-200 placeholder-gray-600 focus:border-violet-500 focus:outline-none" />
      </div>

      {/* Final video preview + download */}
      {comp.final_video_url ? (
        <div className="space-y-2">
          <label className="block text-[10px] font-medium text-gray-500 uppercase tracking-wider">成片预览</label>
          <video
            src={comp.final_video_url}
            controls
            playsInline
            className="w-full rounded-lg border border-gray-700 bg-black"
            style={{ maxHeight: 400 }}
          />
          <div className="flex gap-2">
            <a href={`${comp.final_video_url}?download=1`}
              download
              className="flex-1 px-3 py-2 text-xs font-medium rounded-lg bg-cyan-600 text-white hover:bg-cyan-500 transition flex items-center justify-center gap-1.5">
              <ExternalLink className="w-3.5 h-3.5" /> 下载成片
            </a>
            <button onClick={() => onUpdate({ ...comp, final_video_url: '' })}
              className="px-3 py-2 text-xs rounded-lg bg-gray-800 border border-gray-700 text-gray-400 hover:text-red-300 hover:border-red-700 transition">
              <Trash2 className="w-3.5 h-3.5" />
            </button>
          </div>
          <input value={comp.final_video_url}
            onChange={e => onUpdate({ ...comp, final_video_url: e.target.value })}
            className="w-full px-2 py-1 bg-gray-800 border border-gray-700 rounded text-[10px] font-mono text-gray-500 focus:border-emerald-500 focus:outline-none" />
        </div>
      ) : (
        <PreMergePreview
          scenes={scenes}
          archiveProject={archiveProject}
          archiveEpKey={archiveEpKey}
          comp={comp}
          onUpdate={onUpdate}
        />
      )}

      {/* Status select */}
      <div>
        <label className="block text-[10px] font-medium text-gray-500 mb-1 uppercase tracking-wider">状态</label>
        <select value={comp.status}
          onChange={e => onUpdate({ ...comp, status: e.target.value as Composition['status'] })}
          className="w-full px-2 py-1.5 bg-gray-800 border border-gray-700 rounded text-xs text-gray-200 focus:border-emerald-500 focus:outline-none">
          <option value="pending">待生产</option>
          <option value="generating">生产中</option>
          <option value="ready">已合成</option>
          <option value="published">已发布</option>
        </select>
      </div>

      {/* Fix #9：发布到剧本目录（production/<ep>/clips_v2 → episodes/<ep>/scenes） */}
      <div className="pt-3 border-t border-gray-800">
        <div className="flex items-center justify-between mb-2">
          <label className="block text-[10px] font-medium text-gray-500 uppercase tracking-wider flex items-center gap-1">
            <Archive className="w-3 h-3 text-cyan-400" /> 同步到剧本目录
          </label>
          <button onClick={runPublish}
            disabled={publishLoading || pickedScenes.length === 0 || !epId}
            title={
              !epId ? '无法从 episode label 推出目录名（EP05 → ep05）'
                : pickedScenes.length === 0 ? '还没有 picked take，先在"场景" tab 选镜'
                : `把 ${pickedScenes.length} 个 picked clip 拷到 /episodes/${epId}/scenes/ 并重写 README`
            }
            className={`px-2.5 py-1 rounded text-[11px] font-semibold transition inline-flex items-center gap-1 ${
              publishLoading ? 'bg-amber-900/40 text-amber-300 border border-amber-700/50 cursor-wait'
                : 'bg-cyan-900/40 text-cyan-300 border border-cyan-700/50 hover:bg-cyan-700/30'
            } disabled:opacity-40 disabled:cursor-not-allowed`}>
            {publishLoading ? <CircleDot className="w-3 h-3 animate-spin" /> : <FileText className="w-3 h-3" />}
            {publishLoading ? '发布中…' : `发布到 /episodes/${epId || '?'}`}
          </button>
        </div>
        {publishErr && (
          <div className="p-2 rounded bg-red-900/20 border border-red-500/30 text-[11px] text-red-300">
            发布失败：{publishErr}
          </div>
        )}
        {publishResult && !publishErr && (
          <div className="p-2 rounded bg-cyan-900/20 border border-cyan-500/30 text-[11px] text-cyan-200 space-y-0.5">
            <div>✓ 已发布 <span className="font-semibold">{publishResult.published}</span> 个场景到 <span className="font-mono text-cyan-100">{publishResult.dir}/scenes/</span></div>
            {publishResult.missing > 0 && (
              <div className="text-amber-300">⚠ 有 <span className="font-semibold">{publishResult.missing}</span> 个场景的源文件缺失（检查 clips_v2 是否已归档）</div>
            )}
            <div className="text-[10px] text-cyan-400/80 font-mono truncate">README: {publishResult.readme}</div>
          </div>
        )}
      </div>

      {/* Final video preview */}
      {comp.final_video_url && (
        <div className="rounded-lg overflow-hidden border border-emerald-500/30">
          <video src={comp.final_video_url} controls className="w-full" />
        </div>
      )}

      {/* 最后一步：生成推广文案 */}
      <div className="pt-3 border-t border-gray-800">
        <div className="flex items-center justify-between mb-2">
          <label className="block text-[10px] font-medium text-gray-500 uppercase tracking-wider flex items-center gap-1">
            <Sparkles className="w-3 h-3 text-pink-400" /> 最后一步·发布文案
          </label>
          <button onClick={runPromo} disabled={promoLoading || missingCount > 0}
            title={missingCount > 0 ? `还有 ${missingCount} 个场景未选 take` : '调用编剧 AI 生成抖音/朋友圈/小红书全套文案'}
            className={`px-2.5 py-1 rounded text-[11px] font-semibold transition inline-flex items-center gap-1 ${promoLoading ? 'bg-amber-900/40 text-amber-300 border border-amber-700/50 cursor-wait' : 'bg-pink-900/40 text-pink-300 border border-pink-700/50 hover:bg-pink-700/30'} disabled:opacity-40 disabled:cursor-not-allowed`}>
            {promoLoading ? <CircleDot className="w-3 h-3 animate-spin" /> : <Sparkles className="w-3 h-3" />}
            {promoLoading ? '生成中…' : '生成文案'}
          </button>
        </div>
        {promoErr && (
          <div className="p-2 rounded bg-red-900/20 border border-red-500/30 text-[11px] text-red-300">
            生成失败：{promoErr}
          </div>
        )}
        {promo && <PromoResultCard promo={promo} onCopy={copy} />}
        {!promo && !promoLoading && !promoErr && (
          <div className="p-3 rounded bg-gray-900/40 border border-gray-800 text-[10px] text-gray-500 leading-relaxed">
            → 生成后将得到：<span className="text-pink-300">抖音标题（多候选）</span> / 正文 / 话题标签 / 置顶评论，
            <span className="text-cyan-300">朋友圈多版文案</span>（短/中/长/带@），
            <span className="text-rose-300">小红书长文</span>。<br/>
            每段文案均有一键复制按钮。
          </div>
        )}
      </div>
    </div>
  )
}

// 推广文案结果卡
function PromoResultCard({ promo, onCopy }: { promo: PromoResponse; onCopy: (t: string) => void }) {
  const [openPlat, setOpenPlat] = useState<'douyin' | 'wechat' | 'xhs' | null>('douyin')
  return (
    <div className="rounded-lg border border-pink-700/40 bg-gradient-to-br from-pink-950/20 via-gray-900/60 to-purple-950/20 overflow-hidden">
      {/* 核心钩子 */}
      <div className="p-3 border-b border-pink-800/30">
        <div className="text-[10px] uppercase tracking-wider text-pink-400/70 mb-1">Core Hook</div>
        <div className="text-sm font-semibold text-pink-100 flex items-start gap-2">
          <span className="flex-1">{promo.core_hook}</span>
          <button onClick={() => onCopy(promo.core_hook)} className="text-gray-500 hover:text-gray-300 mt-1"><Copy className="w-3 h-3" /></button>
        </div>
        <div className="text-[10px] text-gray-500 mt-1">目标受众情绪：<span className="text-amber-300">{promo.audience_vibe}</span> · <span className="font-mono">{promo.model}</span></div>
      </div>

      {/* 平台 tab */}
      <div className="flex gap-0 border-b border-gray-800">
        <PlatBtn active={openPlat === 'douyin'} label="抖音" color="pink" onClick={() => setOpenPlat(openPlat === 'douyin' ? null : 'douyin')} />
        <PlatBtn active={openPlat === 'wechat'} label="朋友圈" color="cyan" onClick={() => setOpenPlat(openPlat === 'wechat' ? null : 'wechat')} />
        {promo.xiaohongshu && <PlatBtn active={openPlat === 'xhs'} label="小红书" color="rose" onClick={() => setOpenPlat(openPlat === 'xhs' ? null : 'xhs')} />}
      </div>

      {openPlat === 'douyin' && (
        <div className="p-3 space-y-2 text-[11px]">
          <PromoBlock label="前 3s 锁屏文字" value={promo.douyin.first_frame_caption} onCopy={onCopy} hint="销引画面用，6-12 字" />
          <div>
            <div className="flex items-center justify-between mb-1">
              <span className="text-[10px] uppercase tracking-wider text-gray-500">标题候选 ({promo.douyin.titles?.length || 0})</span>
            </div>
            <div className="space-y-1">
              {promo.douyin.titles?.map((t, i) => (
                <div key={i} className="flex items-center gap-2 rounded bg-gray-900/60 border border-gray-800 px-2 py-1.5">
                  <span className="text-pink-400 font-mono text-[9px]">#{i + 1}</span>
                  <span className="flex-1 text-gray-200">{t}</span>
                  <span className="text-[9px] text-gray-600">{t.length}字</span>
                  <button onClick={() => onCopy(t)} className="text-gray-500 hover:text-gray-300"><Copy className="w-3 h-3" /></button>
                </div>
              ))}
            </div>
          </div>
          <PromoBlock label="正文配文" value={promo.douyin.body} onCopy={onCopy} multiline />
          <div>
            <div className="text-[10px] uppercase tracking-wider text-gray-500 mb-1 flex items-center justify-between">
              <span>话题标签 ({promo.douyin.hashtags?.length || 0})</span>
              <button onClick={() => onCopy((promo.douyin.hashtags || []).join(' '))} className="text-gray-500 hover:text-gray-300 inline-flex items-center gap-1 text-[10px]"><Copy className="w-2.5 h-2.5" />全复制</button>
            </div>
            <div className="flex flex-wrap gap-1">
              {promo.douyin.hashtags?.map((h, i) => (
                <span key={i} className="px-1.5 py-0.5 rounded bg-pink-900/40 text-pink-300 text-[10px] border border-pink-800/40 cursor-pointer" onClick={() => onCopy(h)}>{h}</span>
              ))}
            </div>
          </div>
          {promo.douyin.series_tag && <PromoBlock label="系列标签" value={promo.douyin.series_tag} onCopy={onCopy} />}
          {promo.douyin.pinned_comment && <PromoBlock label="作者置顶评论" value={promo.douyin.pinned_comment} onCopy={onCopy} hint="发布后自己评论并置顶，驱动互动" />}
        </div>
      )}

      {openPlat === 'wechat' && (
        <div className="p-3 space-y-2 text-[11px]">
          <PromoBlock label="短版 ≤ 30字" value={promo.wechat_moments.copy_short} onCopy={onCopy} />
          <PromoBlock label="中版 80-120字" value={promo.wechat_moments.copy_medium} onCopy={onCopy} multiline />
          <PromoBlock label="长版 150-200字" value={promo.wechat_moments.copy_long} onCopy={onCopy} multiline />
          <PromoBlock label="带 @朋友版" value={promo.wechat_moments.with_friend_tag} onCopy={onCopy} multiline />
          {promo.wechat_moments.share_hint && <PromoBlock label="转发钩子" value={promo.wechat_moments.share_hint} onCopy={onCopy} />}
        </div>
      )}

      {openPlat === 'xhs' && promo.xiaohongshu && (
        <div className="p-3 space-y-2 text-[11px]">
          <PromoBlock label="小红书正文" value={promo.xiaohongshu} onCopy={onCopy} multiline />
        </div>
      )}
    </div>
  )
}

function PlatBtn({ active, label, color, onClick }: { active: boolean; label: string; color: 'pink' | 'cyan' | 'rose'; onClick: () => void }) {
  const base = active
    ? (color === 'pink' ? 'text-pink-300 border-pink-500 bg-pink-900/20'
     : color === 'cyan' ? 'text-cyan-300 border-cyan-500 bg-cyan-900/20'
     : 'text-rose-300 border-rose-500 bg-rose-900/20')
    : 'text-gray-500 border-transparent hover:text-gray-300'
  return (
    <button onClick={onClick}
      className={`px-3 py-1.5 text-[11px] font-medium border-b-2 transition ${base}`}>
      {label}
    </button>
  )
}

function PromoBlock({ label, value, onCopy, multiline, hint }: { label: string; value: string; onCopy: (t: string) => void; multiline?: boolean; hint?: string }) {
  if (!value) return null
  return (
    <div>
      <div className="flex items-center justify-between mb-0.5">
        <span className="text-[10px] uppercase tracking-wider text-gray-500">{label}{hint ? <span className="normal-case text-gray-600 ml-1 tracking-normal">· {hint}</span> : null}</span>
        <button onClick={() => onCopy(value)} className="text-gray-500 hover:text-gray-300 inline-flex items-center gap-1 text-[10px]">
          <Copy className="w-2.5 h-2.5" />复制
        </button>
      </div>
      {multiline ? (
        <pre className="whitespace-pre-wrap break-words text-gray-200 bg-gray-900/60 border border-gray-800 rounded p-2 font-mono text-[10px] leading-relaxed">{value}</pre>
      ) : (
        <div className="text-gray-200 bg-gray-900/60 border border-gray-800 rounded p-2 text-[11px]">{value}</div>
      )}
    </div>
  )
}

function MetaTab({ data, onUpdate }: { data: EpisodeData; onUpdate: (p: Partial<EpisodeData>) => void }) {
  return (
    <div className="p-3 space-y-3">
      <div>
        <label className="block text-[10px] font-medium text-gray-500 mb-1 uppercase tracking-wider">集名</label>
        <input value={data.label}
          onChange={e => onUpdate({ label: e.target.value })}
          className="w-full px-2 py-1.5 bg-gray-800 border border-gray-700 rounded text-sm text-gray-200 focus:border-cyan-500 focus:outline-none" />
      </div>
      <div className="grid grid-cols-2 gap-2">
        <div>
          <label className="block text-[10px] font-medium text-gray-500 mb-1 uppercase tracking-wider">季</label>
          <select value={data.season}
            onChange={e => {
              const v = parseInt(e.target.value)
              onUpdate({ season: v, is_spinoff: v === 0 })
            }}
            className="w-full px-2 py-1.5 bg-gray-800 border border-gray-700 rounded text-xs text-gray-200 focus:border-cyan-500 focus:outline-none">
            {SEASONS.map(s => <option key={s.number} value={s.number}>{s.title} {s.subtitle}</option>)}
            <option value={0}>衍生剧</option>
          </select>
        </div>
        <div>
          <label className="block text-[10px] font-medium text-gray-500 mb-1 uppercase tracking-wider">集号</label>
          <input type="number" value={data.episode_number || 1}
            onChange={e => onUpdate({ episode_number: parseInt(e.target.value) || 1 })}
            className="w-full px-2 py-1.5 bg-gray-800 border border-gray-700 rounded text-xs text-gray-200 focus:border-cyan-500 focus:outline-none" />
        </div>
      </div>
      {data.is_spinoff && (
        <div>
          <label className="block text-[10px] font-medium text-gray-500 mb-1 uppercase tracking-wider">衍生剧分组</label>
          <input value={data.spinoff_group || ''}
            onChange={e => onUpdate({ spinoff_group: e.target.value })}
            placeholder="道裂前传 / MCU外传 / 联动日历"
            className="w-full px-2 py-1.5 bg-gray-800 border border-gray-700 rounded text-xs text-gray-200 focus:border-cyan-500 focus:outline-none" />
        </div>
      )}
      <div>
        <label className="block text-[10px] font-medium text-gray-500 mb-1 uppercase tracking-wider">目标时长 (秒)</label>
        <input type="number" value={data.duration || 45}
          onChange={e => onUpdate({ duration: parseInt(e.target.value) || 45 })}
          className="w-full px-2 py-1.5 bg-gray-800 border border-gray-700 rounded text-xs text-gray-200 focus:border-cyan-500 focus:outline-none" />
      </div>
      {/* Seedance 2.0 视频规格 —— 本集所有镜头共用（对齐豆包官方 resolution + ratio 参数表） */}
      <div className="grid grid-cols-2 gap-2">
        <div>
          <label className="block text-[10px] font-medium text-gray-500 mb-1 uppercase tracking-wider" title="Seedance 2.0 输出分辨率（480p/720p/1080p）">
            视频分辨率
          </label>
          <select
            value={data.video_resolution || DEFAULT_VIDEO_RESOLUTION}
            onChange={e => onUpdate({ video_resolution: e.target.value })}
            className="w-full px-2 py-1.5 bg-gray-800 border border-gray-700 rounded text-xs text-gray-200 focus:border-cyan-500 focus:outline-none">
            {VIDEO_RESOLUTION_OPTIONS.map(o => (
              <option key={o.value} value={o.value} title={o.hint}>{o.label}</option>
            ))}
          </select>
          <p className="mt-1 text-[9px] text-gray-600 leading-snug">
            {VIDEO_RESOLUTION_OPTIONS.find(o => o.value === (data.video_resolution || DEFAULT_VIDEO_RESOLUTION))?.hint}
          </p>
        </div>
        <div>
          <label className="block text-[10px] font-medium text-gray-500 mb-1 uppercase tracking-wider" title="Seedance 2.0 宽高比（21:9/16:9/4:3/1:1/3:4/9:16）">
            宽高比
          </label>
          <select
            value={data.video_ratio || DEFAULT_VIDEO_RATIO}
            onChange={e => onUpdate({ video_ratio: e.target.value })}
            className="w-full px-2 py-1.5 bg-gray-800 border border-gray-700 rounded text-xs text-gray-200 focus:border-cyan-500 focus:outline-none">
            {VIDEO_RATIO_OPTIONS.map(o => (
              <option key={o.value} value={o.value} title={o.hint}>{o.label}</option>
            ))}
          </select>
          <p className="mt-1 text-[9px] text-gray-600 leading-snug">
            {VIDEO_RATIO_OPTIONS.find(o => o.value === (data.video_ratio || DEFAULT_VIDEO_RATIO))?.hint}
          </p>
        </div>
      </div>
      <div>
        <label className="block text-[10px] font-medium text-gray-500 mb-1 uppercase tracking-wider">描述</label>
        <textarea value={data.description || ''}
          onChange={e => onUpdate({ description: e.target.value })}
          rows={3}
          className="w-full px-2 py-1.5 bg-gray-800 border border-gray-700 rounded text-xs text-gray-200 focus:border-cyan-500 focus:outline-none resize-none" />
      </div>
      <div>
        <label className="block text-[10px] font-medium text-gray-500 mb-1 uppercase tracking-wider">封面 URL</label>
        <input value={data.cover_url || ''}
          onChange={e => onUpdate({ cover_url: e.target.value })}
          className="w-full px-2 py-1.5 bg-gray-800 border border-gray-700 rounded text-xs font-mono text-gray-200 focus:border-cyan-500 focus:outline-none" />
        {data.cover_url && (
          <div className="mt-2 rounded overflow-hidden border border-gray-700">
            <img src={data.cover_url} alt="" className="w-full h-24 object-cover"
              onError={e => { (e.target as HTMLImageElement).style.display = 'none' }} />
          </div>
        )}
      </div>
    </div>
  )
}

// ── 未合成时的预览块：把每个 picked take 串起来顺播。
// 行为：单击主播放器上的 Prev/Next 切镜；切到末尾后自动停。
// 候选 URL 顺序与场景节点保持一致：local_url → 派生归档路径 → TOS。
// 这样 EP05 这种"全选完未合成"的卡片也能在右侧面板看到效果。
function PreMergePreview({
  scenes, archiveProject, archiveEpKey, comp, onUpdate,
}: {
  scenes: SceneSpec[]
  archiveProject: string
  archiveEpKey: string
  comp: Composition
  onUpdate: (c: Composition) => void
}) {
  // 收集所有已 picked 的 take 的候选 URL 列表（按场景顺序）
  const clips = scenes
    .map(s => {
      const t = s.takes.find(tk => tk.take_id === s.picked_take)
      if (!t) return null
      const urls = takeUrlCandidates(t, archiveProject, archiveEpKey, s.id)
      if (urls.length === 0) return null
      return { sceneId: s.id, label: s.label, duration: s.duration, urls }
    })
    .filter((x): x is NonNullable<typeof x> => x !== null)

  const [idx, setIdx] = useState(0)
  const [urlIdx, setUrlIdx] = useState(0)
  const videoRef = useRef<HTMLVideoElement | null>(null)
  // 切镜时候选下标复位
  useEffect(() => { setUrlIdx(0) }, [idx])

  if (clips.length === 0) {
    return (
      <div>
        <label className="block text-[10px] font-medium text-gray-500 mb-1 uppercase tracking-wider">成片 URL（合成后自动填入）</label>
        <input value={comp.final_video_url || ''}
          onChange={e => onUpdate({ ...comp, final_video_url: e.target.value })}
          placeholder="/v1/videos/merged/xxx.mp4"
          className="w-full px-2 py-1.5 bg-gray-800 border border-gray-700 rounded text-xs font-mono text-gray-200 placeholder-gray-600 focus:border-emerald-500 focus:outline-none" />
        <p className="mt-2 text-[10px] text-gray-500">还没有 picked take，无法预览。请到「场景」tab 给每镜选 take。</p>
      </div>
    )
  }

  const cur = clips[idx]
  const playableUrl = cur.urls[urlIdx] || ''
  const totalDur = clips.reduce((s, c) => s + (c.duration || 0), 0)

  return (
    <div className="space-y-2">
      <div className="flex items-center justify-between">
        <label className="block text-[10px] font-medium text-gray-500 uppercase tracking-wider">合成前预览（{clips.length} 镜 · {totalDur}s）</label>
        <span className="text-[10px] text-amber-400">尚未合成 · 单镜顺播</span>
      </div>
      <div className="relative rounded-lg overflow-hidden border border-amber-500/30 bg-black">
        <video
          key={playableUrl}
          ref={videoRef}
          src={playableUrl}
          controls
          autoPlay
          playsInline
          className="w-full"
          style={{ maxHeight: 400 }}
          onEnded={() => {
            // 自动跳下一镜；最后一镜结束停留
            if (idx < clips.length - 1) setIdx(i => i + 1)
          }}
          onError={() => {
            // 当前候选失败 → 试下一个；都试完仍失败则跳下一镜
            if (urlIdx < cur.urls.length - 1) setUrlIdx(i => i + 1)
            else if (idx < clips.length - 1) setIdx(i => i + 1)
          }}
        />
        <div className="absolute top-1.5 left-1.5 px-1.5 py-0.5 rounded bg-black/70 text-[10px] font-mono text-cyan-300 pointer-events-none">
          {idx + 1}/{clips.length} · {cur.sceneId}
        </div>
      </div>
      {/* 镜列表导航 */}
      <div className="flex flex-wrap gap-1">
        {clips.map((c, i) => (
          <button key={c.sceneId} onClick={() => setIdx(i)}
            className={`px-2 py-0.5 rounded text-[10px] font-mono transition border ${
              i === idx
                ? 'bg-amber-500/30 border-amber-400 text-amber-200'
                : 'bg-gray-800 border-gray-700 text-gray-400 hover:text-gray-200 hover:border-gray-600'
            }`}>
            {c.sceneId}
          </button>
        ))}
      </div>
      <div className="text-[10px] text-gray-500 leading-relaxed">
        提示：这是把每镜 picked take 顺序串起来的预览，<span className="text-amber-300">片段间没有 BGM/转场</span>。点底部「合成成片」生成带 BGM 的最终成片。
      </div>
      <input value={comp.final_video_url || ''}
        onChange={e => onUpdate({ ...comp, final_video_url: e.target.value })}
        placeholder="/v1/videos/merged/xxx.mp4（合成后自动填入）"
        className="w-full px-2 py-1 bg-gray-800 border border-gray-700 rounded text-[10px] font-mono text-gray-500 focus:border-emerald-500 focus:outline-none" />
    </div>
  )
}
