import { useState, useEffect } from 'react'
import {
  X, Film, Clapperboard, Play, Plus, Check, CircleDot, CircleAlert, CircleX,
  Music, Scissors, Sparkles, Image as ImageIcon, Trash2, ChevronRight, Layers,
  Archive, FileText, History, Wand2, AlertTriangle, Lightbulb, Wrench,
  Copy, ExternalLink, Terminal,
} from 'lucide-react'
import type { Node } from '@xyflow/react'
import { SEASONS, sceneTakesSummary, type EpisodeData, type SceneSpec, type Take, type Composition } from './episodeTypes'
import { dramaAPI, type WriterReviewResponse, type PromoResponse } from '../../lib/api'

interface Props {
  node: Node
  onUpdate: (id: string, data: Record<string, unknown>) => void
  onClose: () => void
  onProduce?: (episode: EpisodeData) => void
  initialSceneId?: string   // 外部传入打开时自动展开的场景
}

type TabKey = 'scenes' | 'composition' | 'script' | 'meta'

const TAKE_STATUS_COLOR: Record<Take['status'], string> = {
  pending:   'border-gray-600 bg-gray-800 text-gray-400',
  running:   'border-amber-500 bg-amber-900/30 text-amber-300 animate-pulse',
  succeeded: 'border-emerald-500 bg-emerald-900/20 text-emerald-300',
  failed:    'border-red-500 bg-red-900/20 text-red-300',
}

const TAKE_STATUS_ICON: Record<Take['status'], typeof Check> = {
  pending: CircleDot, running: CircleDot, succeeded: Check, failed: CircleX,
}

export default function EpisodeWorkflowPanel({ node, onUpdate, onClose, onProduce, initialSceneId }: Props) {
  const data = node.data as unknown as EpisodeData
  const [tab, setTab] = useState<TabKey>('scenes')
  const [expandedScene, setExpandedScene] = useState<string | null>(initialSceneId || null)

  // 外部切换 initialSceneId 时（sceneStep 节点点击），自动跳到 scenes tab 并展开该场景
  useEffect(() => {
    if (initialSceneId) {
      setTab('scenes')
      setExpandedScene(initialSceneId)
    }
  }, [initialSceneId, node.id])

  const scenes = data.scenes || []
  const comp: Composition = data.composition || { picked_clips: [], status: 'pending' }

  const seasonMeta = data.is_spinoff ? null : SEASONS.find(s => s.number === data.season)

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
    const takeNum = scene.takes.length + 1
    const newTake: Take = {
      take_id: `t${takeNum}`,
      status: 'pending',
      created_at: new Date().toISOString(),
    }
    updateScene(sid, { takes: [...scene.takes, newTake] })
  }
  const pickTake = (sid: string, tid: string) => {
    updateScene(sid, { picked_take: tid })
    // auto-update composition
    const picked_clips = scenes.map(s => {
      const p = s.id === sid ? tid : s.picked_take
      return p ? `${s.id}.${p}` : ''
    }).filter(Boolean)
    update({ composition: { ...comp, picked_clips } })
  }
  const removeTake = (sid: string, tid: string) => {
    const scene = scenes.find(s => s.id === sid)
    if (!scene) return
    const takes = scene.takes.filter(t => t.take_id !== tid)
    updateScene(sid, {
      takes,
      picked_take: scene.picked_take === tid ? undefined : scene.picked_take,
    })
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
                sequenceNumber={idx + 1}
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
          onClick={() => comp.final_video_url && window.open(comp.final_video_url, '_blank')}
          className="flex-1 px-3 py-2 text-xs font-medium rounded-lg bg-gray-800 border border-gray-700 text-gray-300 hover:text-white hover:bg-gray-700 disabled:opacity-40 disabled:cursor-not-allowed transition flex items-center justify-center gap-1.5">
          <Play className="w-3.5 h-3.5" /> 预览成片
        </button>
        <button
          onClick={() => onProduce?.(data)}
          disabled={!onProduce || comp.status === 'generating' || scenes.length === 0}
          className="flex-1 px-3 py-2 text-xs font-medium rounded-lg bg-emerald-600 text-white hover:bg-emerald-500 disabled:opacity-40 disabled:cursor-not-allowed transition flex items-center justify-center gap-1.5 shadow-lg shadow-emerald-900/30">
          {comp.status === 'generating' ? (
            <><CircleDot className="w-3.5 h-3.5 animate-pulse" /> 生产中...</>
          ) : (
            <><Sparkles className="w-3.5 h-3.5" /> 开始生产 {data.label.split(' ')[0]}</>
          )}
        </button>
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

function SceneCard({
  scene, expanded, sequenceNumber, onToggle, onUpdate, onRemove, onAddTake, onPickTake, onRemoveTake,
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
          <TakeThumb take={pickedTake} isPicked small />
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
                {scene.takes.map(take => (
                  <TakeCard
                    key={take.take_id}
                    take={take}
                    isPicked={scene.picked_take === take.take_id}
                    onPick={() => onPickTake(take.take_id)}
                    onRemove={() => onRemoveTake(take.take_id)}
                  />
                ))}
              </div>
            )}
          </div>

          {/* 历史版本（废片） */}
          {scene.rejected_takes && scene.rejected_takes.length > 0 && (
            <RejectedTakesSection rejected={scene.rejected_takes} />
          )}
        </div>
      )}
    </div>
  )
}

function RejectedTakesSection({ rejected }: { rejected: Take[] }) {
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
            <div key={t.take_id} className="group relative rounded border border-red-900/40 bg-red-950/20 overflow-hidden">
              <div className="h-16 bg-gray-900 relative">
                {(t.local_url || t.video_url) ? (
                  <video src={t.local_url || t.video_url} muted loop className="w-full h-full object-cover opacity-60 hover:opacity-100 transition"
                    onMouseEnter={e => (e.currentTarget as HTMLVideoElement).play()}
                    onMouseLeave={e => { const v = e.currentTarget as HTMLVideoElement; v.pause(); v.currentTime = 0 }} />
                ) : (
                  <div className="w-full h-full flex items-center justify-center text-gray-600"><CircleX className="w-4 h-4" /></div>
                )}
                <span className="absolute top-0.5 left-0.5 px-1 py-0.5 rounded bg-red-900/80 text-red-200 text-[8px] font-bold">废</span>
              </div>
              <div className="px-1.5 py-1 bg-gray-850/80 text-[9px] text-gray-400 truncate" title={t.note}>
                <span className="font-mono text-red-300">{t.take_id}</span>
                {t.note && <span className="ml-1 text-gray-500">· {t.note}</span>}
              </div>
            </div>
          ))}
        </div>
      )}
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
  return (
    <div className={`rounded-lg border ${scoreRing} bg-gray-900/60 overflow-hidden`}>
      <button onClick={onToggle}
        className="w-full px-3 py-2 flex items-center gap-2 text-left hover:bg-gray-800/50 transition">
        <ChevronRight className={`w-3.5 h-3.5 text-gray-400 transition ${open ? 'rotate-90' : ''}`} />
        <Wand2 className="w-3.5 h-3.5 text-emerald-400" />
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2">
            <span className="text-[11px] font-semibold text-gray-200 uppercase tracking-wider">编剧审稿</span>
            <span className={`text-lg font-bold ${scoreColor}`}>{Math.round(review.overall_score)}</span>
            <span className="text-[10px] text-gray-500">/ 100</span>
          </div>
          <div className="text-[11px] text-gray-300 truncate">{review.verdict}</div>
        </div>
        <span className="text-[10px] text-gray-600 font-mono ml-2">{review.model}</span>
      </button>

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
  // 按 created_at 倒序（最新在上）
  entries.sort((a, b) => (b.take.created_at || '').localeCompare(a.take.created_at || ''))

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
          const isOpen = expanded.has(key)
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
                <span className="text-[11px] font-mono text-cyan-300">{sceneId}.{take.take_id}</span>
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
                  {take.ref_image_url && <LogRow label="角色参考图" value={take.ref_image_url} link onCopy={copyText} />}
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

function TakeThumb({ take, isPicked, small }: { take: Take; isPicked?: boolean; small?: boolean }) {
  const Icon = TAKE_STATUS_ICON[take.status]
  return (
    <div className={`relative rounded overflow-hidden ${small ? 'h-12' : 'h-20'} ${TAKE_STATUS_COLOR[take.status]} border`}>
      {take.local_url || take.video_url || take.thumbnail_url ? (
        <video src={take.local_url || take.video_url} muted className="w-full h-full object-cover" />
      ) : (
        <div className="w-full h-full flex items-center justify-center">
          <ImageIcon className="w-4 h-4 opacity-40" />
        </div>
      )}
      <div className="absolute top-0.5 left-0.5 flex items-center gap-0.5 px-1 py-0.5 rounded bg-black/60 text-[9px]">
        <Icon className="w-2.5 h-2.5" />
        {take.take_id}
      </div>
      {isPicked && (
        <div className="absolute top-0.5 right-0.5 px-1 py-0.5 rounded bg-emerald-500 text-white text-[9px] font-bold flex items-center gap-0.5">
          <Check className="w-2.5 h-2.5" /> 选
        </div>
      )}
    </div>
  )
}

function TakeCard({ take, isPicked, onPick, onRemove }: { take: Take; isPicked: boolean; onPick: () => void; onRemove: () => void }) {
  const Icon = TAKE_STATUS_ICON[take.status]
  return (
    <div className={`group relative rounded border overflow-hidden transition ${
      isPicked ? 'border-emerald-500 ring-1 ring-emerald-500/40' : `${TAKE_STATUS_COLOR[take.status]}`
    }`}>
      <div className="h-20 bg-gray-900 relative">
        {(take.local_url || take.video_url) ? (
          <video src={take.local_url || take.video_url} muted loop className="w-full h-full object-cover"
            onMouseEnter={e => (e.currentTarget as HTMLVideoElement).play()}
            onMouseLeave={e => { const v = e.currentTarget as HTMLVideoElement; v.pause(); v.currentTime = 0 }} />
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
      </div>
      <div className="px-1.5 py-1 bg-gray-850/80 flex items-center gap-1 text-[10px]">
        <Icon className="w-2.5 h-2.5 flex-shrink-0" />
        <span className="font-mono truncate flex-1">{take.take_id}</span>
        {!isPicked && take.status === 'succeeded' && (
          <button onClick={onPick}
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
      <button onClick={onRemove}
        className="absolute top-0.5 right-0.5 p-0.5 rounded bg-black/60 text-gray-400 hover:text-red-400 opacity-0 group-hover:opacity-100 transition">
        <Trash2 className="w-2.5 h-2.5" />
      </button>
      {take.note && (
        <div className="absolute bottom-6 left-0 right-0 px-1.5 py-0.5 bg-black/70 text-[9px] text-gray-300 truncate">
          {take.note}
        </div>
      )}
    </div>
  )
}

function CompositionTab({ data, comp, scenes, onUpdate }: { data: EpisodeData; comp: Composition; scenes: SceneSpec[]; onUpdate: (c: Composition) => void }) {
  const pickedScenes = scenes.filter(s => s.picked_take)
  const missingCount = scenes.length - pickedScenes.length

  // 推广文案生成状态
  const [promoLoading, setPromoLoading] = useState(false)
  const [promo, setPromo] = useState<PromoResponse | null>(null)
  const [promoErr, setPromoErr] = useState<string | null>(null)

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

      {/* BGM picker */}
      <div>
        <label className="block text-[10px] font-medium text-gray-500 mb-1 uppercase tracking-wider">BGM ID（音乐总监生成的 music_id）</label>
        <input value={comp.bgm_id || ''}
          onChange={e => onUpdate({ ...comp, bgm_id: e.target.value })}
          placeholder="music_xxx"
          className="w-full px-2 py-1.5 bg-gray-800 border border-gray-700 rounded text-xs font-mono text-gray-200 placeholder-gray-600 focus:border-violet-500 focus:outline-none" />
      </div>

      {/* Final video url */}
      <div>
        <label className="block text-[10px] font-medium text-gray-500 mb-1 uppercase tracking-wider">成片 URL（剪辑师合成后）</label>
        <input value={comp.final_video_url || ''}
          onChange={e => onUpdate({ ...comp, final_video_url: e.target.value })}
          placeholder="/v1/videos/finals/xxx.mp4"
          className="w-full px-2 py-1.5 bg-gray-800 border border-gray-700 rounded text-xs font-mono text-gray-200 placeholder-gray-600 focus:border-emerald-500 focus:outline-none" />
      </div>

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
