// ── 道具工坊 Modal · 3 阶段流水线 ──
// 按用户要求：
//   S1 起始参考图 （上传 / AI 生成） → 上传 cdn.starclaw.net
//   S2 三视图 （喂给 nano-banana-2/edit 参考 S1 CDN） → 上传 cdn.starclaw.net
//   S3 最终定妆图 （喂给 doubao-seedream-3 或 nano-banana-2/edit，最小变化 0.01）→ 上传 cdn.starclaw.net
// 每个阶段都把历史尝试累积在 Gallery 中，可切换选中；所有图片显示出来不丢失。
// 保存写回节点：imageUrl=S3 选中本地，cdn_url=S3 选中 CDN，workshop=全部历史（可 resume）。
import { useRef, useState } from 'react'
import {
  X, Package, Upload, Sparkles, Loader2, RefreshCw, Check, AlertCircle,
  Image as ImageIcon, Wand2, ChevronRight, Layers,
} from 'lucide-react'
import { fileAPI, imageAPI, cdnAPI } from '../../lib/api'

// ── Types ──────────────────────────────────────────────────────────────
export interface WorkshopImage {
  id: string
  localUrl: string              // /v1/images/xxx 或 /v1/uploads/xxx
  cdnUrl?: string               // https://cdn.starclaw.net/...（SCP 上传后填）
  source: 'init' | 'upload' | 'ai'
  createdAt: number
  prompt?: string
  model?: string
}

export interface WorkshopStage {
  key: 's1' | 's2' | 's3'
  images: WorkshopImage[]
  selectedId: string | null
}

export interface WorkshopState {
  stages: { s1: WorkshopStage; s2: WorkshopStage; s3: WorkshopStage }
  finalizedAt?: number
}

export interface PropData {
  category: 'prop'
  label: string
  description?: string
  imageUrl?: string             // S3 选中本地
  cdn_url?: string              // S3 选中 CDN
  workshop?: WorkshopState      // 全流程历史，用于 resume
}

interface Props {
  open: boolean
  initial?: Partial<PropData>
  onClose: () => void
  onSave: (data: PropData) => void
}

// ── Helpers ───────────────────────────────────────────────────────────
const newId = () => Math.random().toString(36).slice(2, 10)

const emptyStage = (k: 's1' | 's2' | 's3'): WorkshopStage => ({ key: k, images: [], selectedId: null })
const emptyWorkshop = (): WorkshopState => ({ stages: { s1: emptyStage('s1'), s2: emptyStage('s2'), s3: emptyStage('s3') } })

// 从 initial.imageUrl/cdn_url 种一个 init 图，保证回编辑时能看到之前的终图
function hydrate(initial?: Partial<PropData>): WorkshopState {
  if (initial?.workshop) return initial.workshop
  const w = emptyWorkshop()
  if (initial?.imageUrl || initial?.cdn_url) {
    const img: WorkshopImage = {
      id: newId(),
      localUrl: initial.imageUrl || initial.cdn_url || '',
      cdnUrl: initial.cdn_url,
      source: 'init',
      createdAt: Date.now(),
    }
    w.stages.s3.images.push(img)
    w.stages.s3.selectedId = img.id
  }
  return w
}

const errMsg = (e: unknown) => (e as { message?: string })?.message || String(e)

// Prompt builders ─────────────────────────────────────────────────────
function buildRefPrompt(name: string, desc: string): string {
  const d = (desc || '').trim().slice(0, 200)
  return [
    `Single prop reference photo of ${name || 'an object'}.`,
    d,
    'Centered, full object visible, product photography lighting.',
    'Plain white background, realistic style, 4K, no text, no watermark.',
  ].filter(Boolean).join(' ')
}

function buildThreeViewPrompt(name: string): string {
  return [
    `Three-view orthographic reference sheet of ${name || 'the prop'}, same object as reference image.`,
    'Left: front view. Center: side view. Right: back view.',
    'Equal scale, white background, clean product photography lighting.',
    'Realistic style, 4K, no text, no watermark.',
  ].join(' ')
}

function buildFinalPrompt(name: string, desc: string): string {
  const d = (desc || '').trim().slice(0, 120)
  return [
    `High-quality hero shot of ${name || 'the prop'}, preserve exact design from reference image.`,
    d,
    'Minimal variation (~0.01), keep composition, materials, colors unchanged.',
    'Cinematic product photography, soft key light, shallow depth of field, 4K.',
  ].filter(Boolean).join(' ')
}

// 把任意 URL (本地或 CDN) fetch 成 File 便于再次上传
async function urlToFile(url: string, filename: string): Promise<File> {
  const resp = await fetch(url)
  if (!resp.ok) throw new Error(`下载 ${url} 失败 HTTP ${resp.status}`)
  const blob = await resp.blob()
  return new File([blob], filename, { type: blob.type || 'image/png' })
}

// ═══════════════════════════════════════════════════════════════════
// ───────────────────────── Main Component ──────────────────────────
// ═══════════════════════════════════════════════════════════════════
export default function PropEditorModal({ open, initial, onClose, onSave }: Props) {
  const isEdit = !!initial
  const [name, setName] = useState(initial?.label || '')
  const [description, setDescription] = useState(initial?.description || '')
  const [workshop, setWorkshop] = useState<WorkshopState>(() => hydrate(initial))
  const [activeStage, setActiveStage] = useState<'s1' | 's2' | 's3'>(() => {
    const w = hydrate(initial)
    if (w.stages.s3.selectedId) return 's3'
    if (w.stages.s2.selectedId) return 's3'
    if (w.stages.s1.selectedId) return 's2'
    return 's1'
  })
  const [busy, setBusy] = useState<null | { stage: 's1' | 's2' | 's3'; op: string }>(null)
  const [error, setError] = useState<string | null>(null)
  const fileInput = useRef<HTMLInputElement>(null)

  if (!open) return null

  const close = () => { setError(null); onClose() }

  // ── stage helpers ──
  const stage = (k: 's1' | 's2' | 's3') => workshop.stages[k]
  const selectedImage = (k: 's1' | 's2' | 's3'): WorkshopImage | null => {
    const s = stage(k)
    return s.images.find(i => i.id === s.selectedId) || null
  }

  const addImage = (k: 's1' | 's2' | 's3', img: WorkshopImage) => {
    setWorkshop(w => ({
      ...w,
      stages: { ...w.stages, [k]: { ...w.stages[k], images: [...w.stages[k].images, img], selectedId: img.id } },
    }))
  }

  const selectImage = (k: 's1' | 's2' | 's3', id: string) => {
    setWorkshop(w => ({ ...w, stages: { ...w.stages, [k]: { ...w.stages[k], selectedId: id } } }))
  }

  const patchImage = (k: 's1' | 's2' | 's3', id: string, patch: Partial<WorkshopImage>) => {
    setWorkshop(w => ({
      ...w,
      stages: {
        ...w.stages,
        [k]: { ...w.stages[k], images: w.stages[k].images.map(i => i.id === id ? { ...i, ...patch } : i) },
      },
    }))
  }

  // ── Stage 1: upload local ──
  const handleS1Upload = async (file: File) => {
    setBusy({ stage: 's1', op: 'upload' }); setError(null)
    try {
      const res = await fileAPI.upload(file)
      const url = res.data.url
      if (!url) throw new Error('上传接口未返回 url')
      addImage('s1', { id: newId(), localUrl: url, source: 'upload', createdAt: Date.now() })
    } catch (e) { setError('S1 上传失败：' + errMsg(e)) }
    finally { setBusy(null) }
  }

  // ── Stage 1: AI generate reference ──
  const handleS1AI = async () => {
    if (!name.trim() && !description.trim()) { setError('请至少填写名称或描述'); return }
    setBusy({ stage: 's1', op: 'ai' }); setError(null)
    try {
      const prompt = buildRefPrompt(name, description)
      const res = await imageAPI.generate({ prompt, model: 'nano-banana-2', size: 'square_hd' })
      const url = res.data.url || res.data.image_url || res.data.display_url
      if (!url) throw new Error('图片生成接口未返回 url')
      addImage('s1', { id: newId(), localUrl: url, source: 'ai', createdAt: Date.now(), prompt, model: 'nano-banana-2' })
    } catch (e) { setError('S1 AI 生成失败：' + errMsg(e)) }
    finally { setBusy(null) }
  }

  // ── Push selected image to CDN (scp) for any stage ──
  const pushToCDN = async (stageKey: 's1' | 's2' | 's3') => {
    const sel = selectedImage(stageKey)
    if (!sel) { setError(`请先在 ${stageKey.toUpperCase()} 选中一张图片`); return }
    if (sel.cdnUrl) { return }
    setBusy({ stage: stageKey, op: 'cdn' }); setError(null)
    try {
      const safeName = (name || 'prop').replace(/[^\w\u4e00-\u9fa5-]/g, '_')
      const file = await urlToFile(sel.localUrl, `${safeName}_${stageKey}_${sel.id}.png`)
      const res = await cdnAPI.upload(file, { drama: 'swarm-universe', asset_type: 'props', filename: file.name })
      const cdn = res.data.cdn_url || res.data.url
      if (!cdn) throw new Error('CDN 接口未返回 url')
      patchImage(stageKey, sel.id, { cdnUrl: cdn })
    } catch (e) { setError(`${stageKey.toUpperCase()} CDN 上传失败：` + errMsg(e)) }
    finally { setBusy(null) }
  }

  // ── Stage 2: generate three-view sheet from S1 CDN ──
  const handleS2Generate = async () => {
    const s1sel = selectedImage('s1')
    if (!s1sel?.cdnUrl) { setError('请先在 S1 选中图片并上传到 CDN（三视图需要稳定公网 URL）'); return }
    setBusy({ stage: 's2', op: 'ai' }); setError(null)
    try {
      const prompt = buildThreeViewPrompt(name)
      const res = await imageAPI.generate({
        prompt,
        model: 'nano-banana-2/edit',
        image_url: s1sel.cdnUrl,
        size: 'landscape_16_9',
      })
      const url = res.data.url || res.data.image_url || res.data.display_url
      if (!url) throw new Error('图片生成接口未返回 url')
      addImage('s2', { id: newId(), localUrl: url, source: 'ai', createdAt: Date.now(), prompt, model: 'nano-banana-2/edit' })
    } catch (e) { setError('S2 三视图生成失败：' + errMsg(e)) }
    finally { setBusy(null) }
  }

  // ── Stage 3: final hero image with minimal variation from S2 CDN ──
  const handleS3Generate = async (modelChoice: 'doubao-seedream-3-0-t2i-250401' | 'nano-banana-2/edit') => {
    const s2sel = selectedImage('s2')
    if (!s2sel?.cdnUrl) { setError('请先在 S2 选中三视图并上传到 CDN'); return }
    setBusy({ stage: 's3', op: 'ai' }); setError(null)
    try {
      const prompt = buildFinalPrompt(name, description)
      const res = await imageAPI.generate({
        prompt,
        model: modelChoice,
        image_url: s2sel.cdnUrl,
        size: 'square_hd',
      })
      const url = res.data.url || res.data.image_url || res.data.display_url
      if (!url) throw new Error('图片生成接口未返回 url')
      addImage('s3', { id: newId(), localUrl: url, source: 'ai', createdAt: Date.now(), prompt, model: modelChoice })
    } catch (e) { setError('S3 最终图生成失败：' + errMsg(e)) }
    finally { setBusy(null) }
  }

  // ── Save ──
  const handleSave = () => {
    if (!name.trim()) { setError('名称不能为空'); return }
    const s3sel = selectedImage('s3')
    const s2sel = selectedImage('s2')
    const s1sel = selectedImage('s1')
    const picked = s3sel || s2sel || s1sel
    const data: PropData = {
      category: 'prop',
      label: name.trim(),
      description: description.trim() || undefined,
      imageUrl: picked?.localUrl || undefined,
      cdn_url: picked?.cdnUrl || undefined,
      workshop: { ...workshop, finalizedAt: Date.now() },
    }
    onSave(data)
    close()
  }

  // ── Stage unlock gates ──
  const s1Cdn = !!selectedImage('s1')?.cdnUrl
  const s2Cdn = !!selectedImage('s2')?.cdnUrl

  return (
    <div className="fixed inset-0 z-[95] bg-black/70 backdrop-blur-sm flex items-center justify-center p-2"
      onClick={close}>
      <div className="bg-gradient-to-br from-gray-900 via-gray-900 to-gray-950 rounded-2xl shadow-2xl border border-amber-700/40 w-[min(1200px,98vw)] max-h-[95vh] overflow-hidden flex flex-col"
        onClick={(e) => e.stopPropagation()}>

        {/* Header */}
        <div className="px-6 py-3 border-b border-gray-800 flex items-center justify-between flex-shrink-0">
          <div className="flex items-center gap-3">
            <div className="p-2 rounded-lg bg-amber-500/20 border border-amber-500/40">
              <Package className="w-5 h-5 text-amber-300" />
            </div>
            <div>
              <h2 className="text-lg font-bold text-gray-100">{isEdit ? '编辑道具' : '新建道具'}</h2>
              <p className="text-[11px] text-gray-500">S1 起始参考图 → CDN → S2 三视图 → CDN → S3 最终图（最小变化）→ CDN</p>
            </div>
          </div>
          <button onClick={close}
            className="p-1.5 rounded-lg hover:bg-gray-800 text-gray-400 hover:text-white transition">
            <X className="w-4 h-4" />
          </button>
        </div>

        {/* Stage tabs */}
        <div className="px-6 py-2 border-b border-gray-800 flex items-center gap-2 flex-shrink-0 bg-gray-950/50">
          <StageTab k="s1" label="① 起始参考图" active={activeStage === 's1'} ok={s1Cdn} onClick={() => setActiveStage('s1')} />
          <ChevronRight className="w-3 h-3 text-gray-700" />
          <StageTab k="s2" label="② 三视图 (nano-banana-2)" active={activeStage === 's2'} ok={s2Cdn} locked={!s1Cdn} onClick={() => s1Cdn && setActiveStage('s2')} />
          <ChevronRight className="w-3 h-3 text-gray-700" />
          <StageTab k="s3" label="③ 最终定妆图 (最小变化 0.01)" active={activeStage === 's3'} ok={!!selectedImage('s3')?.cdnUrl} locked={!s2Cdn} onClick={() => s2Cdn && setActiveStage('s3')} />
        </div>

        {/* Body */}
        <div className="flex-1 overflow-y-auto p-4 grid grid-cols-1 lg:grid-cols-[320px_minmax(0,1fr)] gap-4">

          {/* LEFT: info */}
          <div className="space-y-3">
            <LabeledInput label="名称" value={name} onChange={setName} placeholder="古铜钱 / 手机 / 股票APP …" required />
            <LabeledTextarea label="描述 / 用途" value={description} onChange={setDescription}
              rows={5}
              placeholder={'源文明信物古铜钱，表面有灼烧过的符文，剧情用作时间锚点…'} />

            {/* Mini summary of selected per stage */}
            <div className="rounded-lg border border-gray-800 bg-gray-950/60 p-2 space-y-1">
              <div className="text-[10px] uppercase tracking-wider text-gray-500 font-semibold mb-1 flex items-center gap-1">
                <Layers className="w-3 h-3" /> 流水线状态
              </div>
              {(['s1', 's2', 's3'] as const).map(k => {
                const sel = selectedImage(k)
                return (
                  <div key={k} className="flex items-center gap-2 text-[10px]">
                    <span className="text-gray-500 w-6">{k.toUpperCase()}</span>
                    {sel ? (
                      <>
                        <img src={sel.localUrl} alt="" className="w-5 h-5 object-cover rounded border border-gray-700" />
                        <span className={sel.cdnUrl ? 'text-emerald-400' : 'text-amber-300'}>
                          {sel.cdnUrl ? '✓ CDN' : '⏳ 未上传 CDN'}
                        </span>
                      </>
                    ) : (
                      <span className="text-gray-600">—</span>
                    )}
                  </div>
                )
              })}
            </div>

            {error && (
              <div className="rounded-lg border border-red-700/50 bg-red-950/40 p-2 flex items-start gap-2 text-[11px] text-red-200">
                <AlertCircle className="w-3.5 h-3.5 text-red-400 flex-shrink-0 mt-0.5" />
                <span>{error}</span>
              </div>
            )}
          </div>

          {/* RIGHT: active stage panel */}
          <div className="rounded-lg border border-gray-800 bg-gray-950/40 p-3 flex flex-col min-h-[360px]">
            {activeStage === 's1' && (
              <StageS1
                stage={stage('s1')}
                busy={busy}
                onSelect={(id) => selectImage('s1', id)}
                onUploadClick={() => fileInput.current?.click()}
                onAIGen={handleS1AI}
                onPushCDN={() => pushToCDN('s1')}
              />
            )}
            {activeStage === 's2' && (
              <StageS2
                stage={stage('s2')}
                s1Selected={selectedImage('s1')}
                busy={busy}
                onSelect={(id) => selectImage('s2', id)}
                onGenerate={handleS2Generate}
                onPushCDN={() => pushToCDN('s2')}
              />
            )}
            {activeStage === 's3' && (
              <StageS3
                stage={stage('s3')}
                s2Selected={selectedImage('s2')}
                busy={busy}
                onSelect={(id) => selectImage('s3', id)}
                onGenerate={handleS3Generate}
                onPushCDN={() => pushToCDN('s3')}
              />
            )}
          </div>
        </div>

        <input ref={fileInput} type="file" accept="image/*" className="hidden"
          onChange={(e) => { const f = e.target.files?.[0]; if (f) handleS1Upload(f); e.target.value = '' }} />

        {/* Footer */}
        <div className="px-6 py-2.5 border-t border-gray-800 flex items-center justify-between bg-gray-950/50 flex-shrink-0">
          <div className="text-[10px] text-gray-600">
            保存时取 S3（若无则回退到 S2/S1）的选中图作为 <code className="text-gray-500">imageUrl / cdn_url</code>；全流程保存在 <code className="text-gray-500">workshop</code> 可下次 resume。
          </div>
          <div className="flex gap-2">
            <button onClick={close}
              className="px-4 py-1.5 rounded-lg text-xs text-gray-400 hover:text-white hover:bg-gray-800 transition">
              取消
            </button>
            <button onClick={handleSave} disabled={!name.trim()}
              className="px-4 py-1.5 rounded-lg bg-amber-600 hover:bg-amber-500 disabled:opacity-40 text-xs font-semibold text-white flex items-center gap-1.5 transition">
              <Check className="w-3.5 h-3.5" />
              {isEdit ? '保存修改' : '创建道具'}
            </button>
          </div>
        </div>
      </div>
    </div>
  )
}

// ═══════════════════════════════════════════════════════════════════
// ───────────────────────── Sub Components ──────────────────────────
// ═══════════════════════════════════════════════════════════════════

function StageTab({ k, label, active, ok, locked, onClick }: {
  k: string; label: string; active: boolean; ok: boolean; locked?: boolean; onClick: () => void
}) {
  return (
    <button onClick={onClick} disabled={locked}
      className={`px-3 py-1 rounded-md text-xs font-medium transition flex items-center gap-1.5 ${
        active ? 'bg-amber-500/20 text-amber-200 border border-amber-500/50'
          : locked ? 'bg-gray-900 text-gray-700 border border-gray-800 cursor-not-allowed'
          : 'bg-gray-900 text-gray-400 hover:text-gray-200 hover:bg-gray-800 border border-gray-800'
      }`}>
      <span>{label}</span>
      {ok && <Check className="w-3 h-3 text-emerald-400" />}
      {locked && <span className="text-[9px] opacity-60">🔒</span>}
      <span className="sr-only">{k}</span>
    </button>
  )
}

function Gallery({ images, selectedId, onSelect }: {
  images: WorkshopImage[]; selectedId: string | null; onSelect: (id: string) => void
}) {
  if (images.length === 0) {
    return (
      <div className="flex-1 flex items-center justify-center border border-dashed border-gray-800 rounded-lg">
        <div className="text-center">
          <ImageIcon className="w-8 h-8 text-gray-700 mx-auto mb-1" />
          <p className="text-[11px] text-gray-600">尚无图片</p>
        </div>
      </div>
    )
  }
  return (
    <div className="grid grid-cols-3 md:grid-cols-4 gap-2">
      {images.map(img => {
        const isSel = img.id === selectedId
        return (
          <button key={img.id} onClick={() => onSelect(img.id)}
            className={`relative rounded-md overflow-hidden border-2 transition group ${
              isSel ? 'border-amber-400 shadow-lg shadow-amber-900/40' : 'border-gray-800 hover:border-gray-600'
            }`}>
            <img src={img.cdnUrl || img.localUrl} alt="" className="w-full aspect-square object-cover"
              onError={(e) => { (e.target as HTMLImageElement).style.opacity = '0.3' }} />
            <div className="absolute top-0.5 left-0.5 flex gap-0.5">
              <span className={`text-[8px] px-1 py-0.5 rounded ${
                img.source === 'ai' ? 'bg-violet-500/80' : img.source === 'upload' ? 'bg-cyan-500/80' : 'bg-gray-600/80'
              } text-white font-bold`}>
                {img.source === 'ai' ? 'AI' : img.source === 'upload' ? 'UP' : 'IN'}
              </span>
              {img.cdnUrl && <span className="text-[8px] px-1 py-0.5 rounded bg-emerald-500/80 text-white font-bold">CDN</span>}
            </div>
            {isSel && (
              <div className="absolute top-0.5 right-0.5 w-4 h-4 rounded-full bg-amber-400 flex items-center justify-center">
                <Check className="w-2.5 h-2.5 text-gray-900" />
              </div>
            )}
          </button>
        )
      })}
    </div>
  )
}

function StageS1({ stage, busy, onSelect, onUploadClick, onAIGen, onPushCDN }: {
  stage: WorkshopStage; busy: null | { stage: string; op: string };
  onSelect: (id: string) => void; onUploadClick: () => void; onAIGen: () => void; onPushCDN: () => void
}) {
  const b = busy?.stage === 's1' ? busy.op : null
  const sel = stage.images.find(i => i.id === stage.selectedId)
  return (
    <>
      <StageHeader title="① 起始参考图" hint="上传本地图片或用 nano-banana-2 生成一张干净的道具参考图。选中后点「上传到 CDN」获取公网 URL，进入 S2。" />
      <div className="flex gap-2 mb-2">
        <ActionBtn onClick={onUploadClick} disabled={!!busy} icon={<Upload className="w-3.5 h-3.5" />} loading={b === 'upload'}>上传本地</ActionBtn>
        <ActionBtn onClick={onAIGen} disabled={!!busy} icon={<Sparkles className="w-3.5 h-3.5" />} loading={b === 'ai'} variant="violet">AI 生成</ActionBtn>
        <ActionBtn onClick={onPushCDN} disabled={!!busy || !sel || !!sel?.cdnUrl} icon={<RefreshCw className="w-3.5 h-3.5" />} loading={b === 'cdn'} variant="emerald">
          {sel?.cdnUrl ? 'CDN ✓' : '上传到 CDN'}
        </ActionBtn>
      </div>
      <div className="flex-1 overflow-y-auto">
        <Gallery images={stage.images} selectedId={stage.selectedId} onSelect={onSelect} />
      </div>
      <SelectedFooter image={sel} />
    </>
  )
}

function StageS2({ stage, s1Selected, busy, onSelect, onGenerate, onPushCDN }: {
  stage: WorkshopStage; s1Selected: WorkshopImage | null; busy: null | { stage: string; op: string };
  onSelect: (id: string) => void; onGenerate: () => void; onPushCDN: () => void
}) {
  const b = busy?.stage === 's2' ? busy.op : null
  const sel = stage.images.find(i => i.id === stage.selectedId)
  const refReady = !!s1Selected?.cdnUrl
  return (
    <>
      <StageHeader title="② 三视图（nano-banana-2/edit）" hint="以 S1 的 CDN 图为参考，nano-banana-2 编辑模式生成正/侧/背三视图。确认满意再上传到 CDN，进入 S3。" />
      {!refReady && <div className="text-[11px] text-amber-300 mb-2">需要 S1 已上传 CDN</div>}
      <div className="flex gap-2 mb-2 items-center">
        {s1Selected && (
          <div className="flex items-center gap-1.5 p-1 rounded-md bg-gray-900 border border-gray-800">
            <img src={s1Selected.cdnUrl || s1Selected.localUrl} alt="" className="w-8 h-8 object-cover rounded" />
            <span className="text-[10px] text-gray-500 pr-1.5">参考 S1</span>
          </div>
        )}
        <ActionBtn onClick={onGenerate} disabled={!refReady || !!busy} icon={<Wand2 className="w-3.5 h-3.5" />} loading={b === 'ai'} variant="violet">生成三视图</ActionBtn>
        <ActionBtn onClick={onPushCDN} disabled={!!busy || !sel || !!sel?.cdnUrl} icon={<RefreshCw className="w-3.5 h-3.5" />} loading={b === 'cdn'} variant="emerald">
          {sel?.cdnUrl ? 'CDN ✓' : '上传到 CDN'}
        </ActionBtn>
      </div>
      <div className="flex-1 overflow-y-auto">
        <Gallery images={stage.images} selectedId={stage.selectedId} onSelect={onSelect} />
      </div>
      <SelectedFooter image={sel} />
    </>
  )
}

function StageS3({ stage, s2Selected, busy, onSelect, onGenerate, onPushCDN }: {
  stage: WorkshopStage; s2Selected: WorkshopImage | null; busy: null | { stage: string; op: string };
  onSelect: (id: string) => void;
  onGenerate: (m: 'doubao-seedream-3-0-t2i-250401' | 'nano-banana-2/edit') => void;
  onPushCDN: () => void
}) {
  const b = busy?.stage === 's3' ? busy.op : null
  const sel = stage.images.find(i => i.id === stage.selectedId)
  const refReady = !!s2Selected?.cdnUrl
  return (
    <>
      <StageHeader title="③ 最终定妆图（最小变化 0.01）" hint="以 S2 三视图 CDN 为参考，生成最终的单张定妆图；最小变化保留组件和材质不变。可选 Doubao Seedream-3 或 Nano-Banana-2/edit。" />
      {!refReady && <div className="text-[11px] text-amber-300 mb-2">需要 S2 已上传 CDN</div>}
      <div className="flex gap-2 mb-2 items-center flex-wrap">
        {s2Selected && (
          <div className="flex items-center gap-1.5 p-1 rounded-md bg-gray-900 border border-gray-800">
            <img src={s2Selected.cdnUrl || s2Selected.localUrl} alt="" className="w-8 h-8 object-cover rounded" />
            <span className="text-[10px] text-gray-500 pr-1.5">参考 S2</span>
          </div>
        )}
        <ActionBtn onClick={() => onGenerate('doubao-seedream-3-0-t2i-250401')} disabled={!refReady || !!busy}
          icon={<Wand2 className="w-3.5 h-3.5" />} loading={b === 'ai'} variant="violet">Doubao Seedream</ActionBtn>
        <ActionBtn onClick={() => onGenerate('nano-banana-2/edit')} disabled={!refReady || !!busy}
          icon={<Wand2 className="w-3.5 h-3.5" />} loading={b === 'ai'}>Nano-Banana-2</ActionBtn>
        <ActionBtn onClick={onPushCDN} disabled={!!busy || !sel || !!sel?.cdnUrl} icon={<RefreshCw className="w-3.5 h-3.5" />} loading={b === 'cdn'} variant="emerald">
          {sel?.cdnUrl ? 'CDN ✓' : '上传到 CDN'}
        </ActionBtn>
      </div>
      <div className="flex-1 overflow-y-auto">
        <Gallery images={stage.images} selectedId={stage.selectedId} onSelect={onSelect} />
      </div>
      <SelectedFooter image={sel} />
    </>
  )
}

function StageHeader({ title, hint }: { title: string; hint: string }) {
  return (
    <div className="mb-2">
      <div className="text-sm font-semibold text-gray-100">{title}</div>
      <div className="text-[11px] text-gray-500 leading-relaxed mt-0.5">{hint}</div>
    </div>
  )
}

function SelectedFooter({ image }: { image: WorkshopImage | undefined | null }) {
  if (!image) return <div className="mt-2 text-[10px] text-gray-600">未选择图片</div>
  return (
    <div className="mt-2 rounded-md bg-gray-950/60 border border-gray-800 p-2 space-y-0.5 text-[10px] font-mono">
      <div className="flex items-center gap-1.5">
        <span className="text-gray-500 w-10">本地</span>
        <span className="flex-1 truncate text-gray-300" title={image.localUrl}>{image.localUrl}</span>
      </div>
      <div className="flex items-center gap-1.5">
        <span className="text-gray-500 w-10">CDN</span>
        <span className={`flex-1 truncate ${image.cdnUrl ? 'text-emerald-300' : 'text-gray-600'}`} title={image.cdnUrl || '—'}>
          {image.cdnUrl || '未上传'}
        </span>
      </div>
      {image.prompt && (
        <div className="flex items-start gap-1.5">
          <span className="text-gray-500 w-10 flex-shrink-0">prompt</span>
          <span className="flex-1 text-gray-500 line-clamp-2" title={image.prompt}>{image.prompt}</span>
        </div>
      )}
    </div>
  )
}

function ActionBtn({ onClick, disabled, icon, loading, variant, children }: {
  onClick: () => void; disabled?: boolean; icon: React.ReactNode; loading?: boolean;
  variant?: 'violet' | 'emerald' | 'default'; children: React.ReactNode
}) {
  const cls =
    variant === 'violet' ? 'bg-gradient-to-r from-violet-600/80 to-cyan-600/80 hover:from-violet-500 hover:to-cyan-500 text-white'
    : variant === 'emerald' ? 'bg-emerald-900/40 hover:bg-emerald-800/60 border border-emerald-700/50 text-emerald-200'
    : 'bg-gray-800 hover:bg-gray-700 border border-gray-700 text-gray-200'
  return (
    <button onClick={onClick} disabled={disabled}
      className={`px-2.5 py-1.5 rounded-md text-[11px] font-medium flex items-center gap-1.5 transition disabled:opacity-40 ${cls}`}>
      {loading ? <Loader2 className="w-3.5 h-3.5 animate-spin" /> : icon}
      {children}
    </button>
  )
}

function LabeledInput({ label, value, onChange, placeholder, required }: {
  label: string; value: string; onChange: (v: string) => void; placeholder?: string; required?: boolean
}) {
  return (
    <label className="block">
      <span className="text-[10px] uppercase tracking-wider text-gray-500 font-semibold">
        {label}{required && <span className="text-red-400 ml-1">*</span>}
      </span>
      <input value={value} onChange={(e) => onChange(e.target.value)} placeholder={placeholder}
        className="mt-1 w-full px-2 py-1.5 bg-gray-800 border border-gray-700 rounded text-xs text-gray-200 placeholder-gray-600 focus:border-amber-500 outline-none transition" />
    </label>
  )
}

function LabeledTextarea({ label, value, onChange, rows, placeholder }: {
  label: string; value: string; onChange: (v: string) => void; rows?: number; placeholder?: string
}) {
  return (
    <label className="block">
      <span className="text-[10px] uppercase tracking-wider text-gray-500 font-semibold">{label}</span>
      <textarea value={value} onChange={(e) => onChange(e.target.value)} rows={rows || 4} placeholder={placeholder}
        className="mt-1 w-full px-2 py-1.5 bg-gray-800 border border-gray-700 rounded text-xs text-gray-200 placeholder-gray-600 focus:border-amber-500 outline-none resize-none leading-relaxed" />
    </label>
  )
}
