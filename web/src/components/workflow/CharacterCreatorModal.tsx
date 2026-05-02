import { useRef, useState } from 'react'
import { X, Users, Upload, Sparkles, Loader2, RefreshCw, Check, ChevronLeft, ChevronRight, AlertCircle, Image as ImageIcon, Wand2, Plus } from 'lucide-react'
import type { CharacterData } from './episodeTypes'
import { fileAPI, imageAPI, characterAPI, cdnAPI } from '../../lib/api'

interface Props {
  open: boolean
  existingTags: string[]       // e.g. ["[图1]","[图2]"]
  /** 传入时进入编辑模式：预填字段，保留原 tag；onCreate 回调应做 update 而非 create */
  initial?: Partial<CharacterData>
  onClose: () => void
  onCreate: (data: CharacterData) => void
}

const ROLE_PRESETS = [
  { value: '女一', color: 'bg-rose-500/20 text-rose-300 border-rose-500/40' },
  { value: '女二', color: 'bg-pink-500/20 text-pink-300 border-pink-500/40' },
  { value: '男一', color: 'bg-cyan-500/20 text-cyan-300 border-cyan-500/40' },
  { value: '男二', color: 'bg-blue-500/20 text-blue-300 border-blue-500/40' },
  { value: '配角', color: 'bg-violet-500/20 text-violet-300 border-violet-500/40' },
  { value: '生物', color: 'bg-amber-500/20 text-amber-300 border-amber-500/40' },
]
// 自定义 role 持久到 localStorage，下次打开 modal 也能看到
const CUSTOM_ROLES_KEY = 'starclaw:character-custom-roles'
const CUSTOM_ROLE_COLOR = 'bg-teal-500/20 text-teal-300 border-teal-500/40'
function loadCustomRoles(): string[] {
  try { return JSON.parse(localStorage.getItem(CUSTOM_ROLES_KEY) || '[]') || [] } catch { return [] }
}
function saveCustomRoles(rs: string[]) {
  try { localStorage.setItem(CUSTOM_ROLES_KEY, JSON.stringify(rs.slice(0, 32))) } catch { /* ignore quota */ }
}

const STYLE_PRESETS = [
  { value: 'realistic', label: '写实', hint: 'EP01-04 验证，适配真人短剧' },
  { value: 'anime',     label: '动漫', hint: '二次元角色' },
  { value: '3d',        label: '3D',   hint: 'CG 风格' },
]

// 服装阶段预设（B 路 · 对齐 docs/swarm-universe/production/characters/lin_jianyue/reference.md
// 的 4 段服装表）。用户可以选一个，prompt 会在 appearance 后附上对应的完整服饰描述，
// 生成出来的三视图自动换装 —— 不用每次都重写 appearance_card。
// value='' 表示"跟随 appearance 原文"，对非林见月角色也兼容。
const COSTUME_PRESETS: { value: string; label: string; desc: string }[] = [
  { value: '', label: '跟随外观卡', desc: '用 appearance 原文生图' },
  {
    value: 'ancient_xianxia',
    label: '古装仙侠（EP01-02）',
    desc: 'mint-green ancient Chinese hanfu under sheer lace overcoat, lotus embroidery, silver star-shaped waist buckle, emerald tassel, barefoot or cloth shoes',
  },
  {
    value: 'luxury_heaven',
    label: '豪华天宫正装（觉醒）',
    desc: 'deep crimson silk robe with gold phoenix embroidery, openwork gold waist belt with ruby, gold phoenix crown, long flowing cape',
  },
  {
    value: 'modern_poor',
    label: '现代旧衣（EP03-10）',
    desc: 'oversized grey hoodie, worn-out blue jeans, scuffed canvas sneakers, slightly dishevelled',
  },
  {
    value: 'modern_normal',
    label: '现代正常装（EP11+）',
    desc: 'clean contemporary women\'s wear (simple knit top, high-waisted trousers or midi skirt) with a subtle retro touch',
  },
]

type Stage = 1 | 2 | 3

// 对齐 @docs/swarm-universe/production/characters/style_guide.md 的 "Premium realism,
// no cartoon" 铁律 —— 用户多次反馈生成出来的三视图跑偏成漫画风。只在 style=realistic
// 时注入，动漫/3D 不强加（那两种本来就不写实）。
const REALISM_CLAMP = 'live-action short-drama realism, cinematic photograph, natural lighting, real human skin pores and micro-texture, DSLR photographic quality, absolutely NOT cartoon / anime / illustration / 3D render / painting / CGI'

// ⚠️ 1:1 还原参考图 —— 用户多次反馈 nano-banana-2/edit 经常"二次创作"脸：
// 换脸、改瘦/改宽、改眼距、改肤色。明确强调 preserve identity / keep facial
// structure / do NOT stylize，给 edit 模型一个硬锚点。
const IDENTITY_PRESERVE_CLAMP = 'EXACT 1:1 identity match to the reference photo: preserve the same face, same facial structure, same eye shape and spacing, same nose bridge, same lip shape, same jawline, same skin tone, same hairline. Do NOT stylize, do NOT beautify, do NOT change proportions. Treat the reference as a photo of a real actress and reproduce her faithfully.'

// EP04 成熟经验：prompt 简短精确，但身份锚点必须显式写明。
// 结构：realism clamp → identity clamp → 角色描述 → costume → 布局 → 风格 → 质量
function buildSheetPrompt(appearance: string, style: string, costumeDesc?: string): string {
  const styleWord = style === 'realistic' ? 'realistic photograph' : style === 'anime' ? 'anime' : '3D CG'
  const realismHead = style === 'realistic' ? `${REALISM_CLAMP}. ` : ''
  // 身份还原钳子只对写实/3D 有意义；动漫路线保留旧行为。
  const identityHead = style === 'anime' ? '' : `${IDENTITY_PRESERVE_CLAMP} `
  return [
    `${realismHead}${identityHead}Character reference sheet of the SAME person as the reference image.`,
    appearance.trim(),
    costumeDesc ? `Costume: ${costumeDesc}.` : '',
    `Top: close-up portrait front and side view.`,
    `Middle: full body front, side, back view.`,
    `Bottom: 3 expressions side by side (neutral, smile, determined).`,
    `White background, ${styleWord} style, 8K, no text.`,
  ].filter(Boolean).join(' ')
}

export default function CharacterCreatorModal({ open, existingTags, initial, onClose, onCreate }: Props) {
  const isEdit = !!initial
  const [stage, setStage] = useState<Stage>(1)
  // Stage 1 变量（最小集）—— edit 模式预填
  const [name, setName] = useState(initial?.label || '')
  const [role, setRole] = useState(initial?.role || '女一')
  const [appearance, setAppearance] = useState(initial?.appearance_card || '')
  const [style, setStyle] = useState('realistic')
  // 生图模型选择——默认 nano-banana-2/edit（EP01-06 验证），可切 gpt-image-2/edit（OpenAI alpha，2026-04 上线）。
  // 两者后端都走 imageAPI.generate 同一路径，body schema 后端自动分支。
  const [sheetModel, setSheetModel] = useState<'nano-banana-2/edit' | 'gpt-image-2/edit'>('nano-banana-2/edit')
  // 服装阶段（B 路）—— 用户反馈 EP03+ 需要现代装版三视图但每次都要手改 appearance。
  // 默认空字符串 = 用 appearance 原文（保持旧行为，兼容已存角色）。
  const [costume, setCostume] = useState('')
  const [refUrl, setRefUrl] = useState(initial?.imageUrl || '')
  const [refPreview, setRefPreview] = useState(initial?.imageUrl || '')
  const [uploading, setUploading] = useState(false)
  // 上传错误（替换掉旧的 alert() 弹窗）—— inline 展示更易复制/截图定位
  const [uploadError, setUploadError] = useState<string | null>(null)
  // Stage 2 生成结果
  const [generating, setGenerating] = useState(false)
  const [genError, setGenError] = useState<string | null>(null)
  const [sheetUrl, setSheetUrl] = useState('')
  const [attempts, setAttempts] = useState(0)

  const fileRef = useRef<HTMLInputElement>(null)

  // ── 自定义定位（持久化） ──
  const [customRoles, setCustomRoles] = useState<string[]>(() => loadCustomRoles())
  const [roleInputOpen, setRoleInputOpen] = useState(false)
  const [roleDraft, setRoleDraft] = useState('')
  const addCustomRole = () => {
    const v = roleDraft.trim()
    if (!v) return
    const known = new Set([...ROLE_PRESETS.map(r => r.value), ...customRoles])
    if (!known.has(v)) {
      const next = [...customRoles, v]
      setCustomRoles(next); saveCustomRoles(next)
    }
    setRole(v); setRoleDraft(''); setRoleInputOpen(false)
  }

  // ── AI 一键生成外观卡 ──
  const [genAppearLoading, setGenAppearLoading] = useState(false)
  const [genAppearError, setGenAppearError] = useState<string | null>(null)
  const generateAppearance = async () => {
    if (!name.trim()) { setGenAppearError('请先填角色名'); return }
    setGenAppearLoading(true); setGenAppearError(null)
    try {
      const res = await characterAPI.generateAppearance({
        name: name.trim(),
        role,
        notes: appearance.trim() || undefined,
        reference_url: refUrl || undefined,
      })
      const text = ((res.data as Record<string, unknown>).appearance_card as string) || ''
      if (text) setAppearance(text)
      else setGenAppearError('模型返回为空，请重试')
    } catch (e) {
      setGenAppearError(e instanceof Error ? e.message : String(e))
    } finally {
      setGenAppearLoading(false)
    }
  }

  // 首次打开时根据 initial 同步（同一 modal 实例被复用切换角色时）
  const [initKey, setInitKey] = useState<string>('')
  // ── 参考图上传（优先 cdn.starclaw.net，失败 fallback /v1/uploads） ──
  // ⚠️ 必须与其它 useState 一样在 early-return 之前声明。之前放在 `if (!open) return null`
  //    之后，导致 open 从 false 翻到 true 时多出一个 hook → React 报
  //    "Rendered more hooks than during the previous render"，表现为「角色 +」按钮点不动。
  const [uploadTarget, setUploadTarget] = useState<'cdn' | 'local' | ''>('')

  if (open) {
    const key = (initial?.tag || '') + '|' + (initial?.label || '')
    if (key !== initKey) {
      setInitKey(key)
      setStage(1)
      setName(initial?.label || '')
      setRole(initial?.role || '女一')
      setAppearance(initial?.appearance_card || '')
      setCostume('') // 每次切角色重置 costume → 默认跟随 appearance
      setRefUrl(initial?.imageUrl || '')
      setRefPreview(initial?.imageUrl || '')
      setSheetUrl('')
      setGenError(null)
      setAttempts(0)
    }
  }

  if (!open) return null

  // 编辑模式保留原 tag；新建模式自动分配下一个 [图N]
  const nextTag = isEdit ? (initial?.tag || '[图?]') : (() => {
    const used = new Set<number>()
    existingTags.forEach(t => {
      const m = t.match(/\[图(\d+)\]/)
      if (m) used.add(parseInt(m[1], 10))
    })
    for (let i = 1; i <= 99; i++) if (!used.has(i)) return `[图${i}]`
    return `[图${existingTags.length + 1}]`
  })()

  const canNextFromStage1 = name.trim() && appearance.trim() && refUrl
  const handleFilePick = async (file: File) => {
    setUploading(true)
    setUploadTarget('')
    setUploadError(null)
    const preview = URL.createObjectURL(file)
    setRefPreview(preview)
    // 基于角色名生成远端 filename（空则用原文件名）。中文角色名会被
    // sanitize 成 _ _ _；保留下划线以便在 CDN 目录里区分同名资源。
    const baseName = (name.trim() || 'ref')
      .replace(/[^\w\u4e00-\u9fa5-]+/g, '_')
      .toLowerCase()
    const ext = (file.name.split('.').pop() || 'png').toLowerCase()
    try {
      // Try CDN first (cdn.starclaw.net 走 scp；后端 CharacterStudioHandler.CDNUpload)
      const res = await cdnAPI.upload(file, {
        drama: 'swarm-universe',
        asset_type: 'characters',
        filename: `${baseName}/ref_photo.${ext}`,
      })
      const data = res.data as Record<string, unknown>
      const url = (data.url as string) || ''
      if (!url) throw new Error('CDN 未返回 URL')
      setRefUrl(url)
      setUploadTarget((data.target as 'cdn' | 'local') || 'cdn')
      // target=local 是 CDN 未配置（或 scp 失败）时的 fallback —— 提示一下，
      // 让用户知道这张图只是本地稳定 URL，不是 cdn.starclaw.net 的。
      if (data.target === 'local' && typeof data.note === 'string') {
        setUploadError(`CDN 未启用，已落本地稳定 URL：${data.note}`)
      }
    } catch (cdnErr) {
      // Fallback: local /v1/uploads —— 只有 CDN 端点彻底 500 时才走这里
      console.warn('[Character] /cdn/upload failed, falling back to /v1/uploads:', cdnErr)
      try {
        const res = await fileAPI.upload(file)
        const url: string = (res.data as Record<string, unknown>).url as string
        if (!url) throw new Error('本地上传未返回 URL')
        const absolute = url.startsWith('http') ? url : `${window.location.origin}${url}`
        setRefUrl(absolute)
        setUploadTarget('local')
        const cdnMsg = cdnErr instanceof Error ? cdnErr.message : String(cdnErr)
        setUploadError(`CDN 上传失败已退到本地：${cdnMsg}`)
      } catch (e) {
        const msg = e instanceof Error ? e.message : String(e)
        setUploadError(`上传失败：${msg}`)
      }
    } finally {
      setUploading(false)
    }
  }

  // ── Stage 1 → Stage 2：触发生成 ──
  const generateSheet = async () => {
    setGenerating(true)
    setGenError(null)
    setStage(2)
    try {
      // 解析 costume value → 完整描述（对齐 reference.md 的 4 段服装表）。
      const costumeDesc = COSTUME_PRESETS.find(c => c.value === costume)?.desc || ''
      const prompt = buildSheetPrompt(appearance, style, costumeDesc)
      const negativeBase = 'blurry, low quality, text, watermark, deformed, extra fingers, bad anatomy'
      const negativeRealism = style === 'realistic'
        ? `${negativeBase}, cartoon, anime, illustration, 3d render, cgi, painting, drawing`
        : negativeBase
      const res = await imageAPI.generate({
        prompt,
        model: sheetModel,
        image_url: refUrl,
        size: 'landscape_16_9',
        scene: `character_${name.trim().replace(/\s+/g, '_')}`,
        style,
        negative_prompt: negativeRealism,
      })
      const data = res.data as Record<string, unknown>
      const url = (data.local_url || data.display_url || data.url || data.image_url) as string
      if (!url) throw new Error('生成未返回有效 URL')
      const absolute = url.startsWith('http') ? url : `${window.location.origin}${url}`
      setSheetUrl(absolute)
      setAttempts(a => a + 1)
    } catch (e) {
      setGenError(e instanceof Error ? e.message : String(e))
    } finally {
      setGenerating(false)
    }
  }

  // ── Stage 3 确认 → 入库 ──
  const submit = () => {
    onCreate({
      category: 'character',
      label: name.trim(),
      tag: nextTag,
      role,
      appearance_card: appearance.trim(),
      imageUrl: sheetUrl || refUrl, // 首选生成的 sheet
      description: `${role}·${(appearance.split(/[，,]/)[0] || '').slice(0, 14)}`,
    })
    reset()
    onClose()
  }

  const reset = () => {
    setStage(1); setName(''); setRole('女一'); setAppearance(''); setStyle('realistic')
    setCostume('')
    setSheetModel('nano-banana-2/edit')
    setRefUrl(''); setRefPreview(''); setSheetUrl(''); setGenError(null); setAttempts(0)
  }

  const close = () => { reset(); onClose() }

  return (
    <div className="fixed inset-0 z-[100] flex items-center justify-center bg-black/70 backdrop-blur-sm p-4">
      <div className="w-full max-w-2xl bg-gray-900 border border-gray-700 rounded-2xl shadow-2xl overflow-hidden flex flex-col max-h-[90vh]">

        {/* Header */}
        <div className="px-5 py-3.5 border-b border-gray-800 flex items-center justify-between bg-gradient-to-r from-violet-900/40 via-gray-900 to-cyan-900/30">
          <div className="flex items-center gap-2">
            <Users className="w-4 h-4 text-violet-400" />
            <h3 className="text-sm font-semibold text-gray-100">角色工坊 · {isEdit ? '编辑角色' : '新建角色'}</h3>
            <span className="ml-1.5 px-2 py-0.5 rounded-md bg-violet-500/20 border border-violet-500/40 text-[11px] font-mono text-violet-300">
              {nextTag}
            </span>
          </div>
          <button onClick={close} className="text-gray-500 hover:text-gray-200 transition-colors">
            <X className="w-4 h-4" />
          </button>
        </div>

        {/* Stepper */}
        <div className="px-5 py-2.5 border-b border-gray-800 bg-gray-950/60 flex items-center gap-3 text-[11px]">
          <Step idx={1} active={stage === 1} done={stage > 1} label="① 基础设定" />
          <div className={`flex-1 h-px ${stage > 1 ? 'bg-violet-500' : 'bg-gray-700'}`} />
          <Step idx={2} active={stage === 2} done={stage > 2} label={`② ${sheetModel === 'gpt-image-2/edit' ? 'gpt-image-2' : 'nano-banana'} 生成三视图`} />
          <div className={`flex-1 h-px ${stage > 2 ? 'bg-emerald-500' : 'bg-gray-700'}`} />
          <Step idx={3} active={stage === 3} done={false} label="③ 确认入库" />
        </div>

        {/* Body */}
        <div className="flex-1 overflow-y-auto px-5 py-4">
          {stage === 1 && (
            <Stage1
              name={name} setName={setName}
              role={role} setRole={setRole}
              appearance={appearance} setAppearance={setAppearance}
              style={style} setStyle={setStyle}
              costume={costume} setCostume={setCostume}
              sheetModel={sheetModel} setSheetModel={setSheetModel}
              refPreview={refPreview} refUrl={refUrl}
              uploading={uploading}
              onPickFile={() => fileRef.current?.click()}
              onPasteUrl={url => { setRefUrl(url); setRefPreview(url) }}
              customRoles={customRoles}
              onRemoveCustomRole={(v) => {
                const next = customRoles.filter(x => x !== v)
                setCustomRoles(next); saveCustomRoles(next)
                if (role === v) setRole('女一')
              }}
              roleInputOpen={roleInputOpen}
              roleDraft={roleDraft}
              setRoleDraft={setRoleDraft}
              onOpenRoleInput={() => setRoleInputOpen(true)}
              onCloseRoleInput={() => { setRoleInputOpen(false); setRoleDraft('') }}
              onAddCustomRole={addCustomRole}
              uploadTarget={uploadTarget}
              uploadError={uploadError}
              onClearUploadError={() => setUploadError(null)}
              genAppearLoading={genAppearLoading}
              genAppearError={genAppearError}
              onGenerateAppearance={generateAppearance}
            />
          )}
          {stage === 2 && (
            <Stage2
              prompt={buildSheetPrompt(
                appearance,
                style,
                COSTUME_PRESETS.find(c => c.value === costume)?.desc,
              )}
              refUrl={refUrl}
              sheetUrl={sheetUrl}
              generating={generating}
              error={genError}
              attempts={attempts}
            />
          )}
          {stage === 3 && (
            <Stage3
              name={name} nextTag={nextTag} role={role} appearance={appearance}
              sheetUrl={sheetUrl} refUrl={refUrl}
            />
          )}
        </div>

        {/* Hidden file input */}
        <input
          ref={fileRef}
          type="file"
          accept="image/*"
          className="hidden"
          onChange={e => {
            const f = e.target.files?.[0]
            if (f) handleFilePick(f)
            if (fileRef.current) fileRef.current.value = ''
          }}
        />

        {/* Footer */}
        <div className="px-5 py-3 border-t border-gray-800 bg-gray-950/60 flex items-center gap-2">
          {stage > 1 && (
            <button onClick={() => setStage((stage - 1) as Stage)}
              className="px-3 py-1.5 text-xs text-gray-400 hover:text-gray-200 transition flex items-center gap-1">
              <ChevronLeft className="w-3.5 h-3.5" /> 上一步
            </button>
          )}
          <div className="flex-1" />
          <button onClick={close} className="px-3 py-1.5 text-xs text-gray-400 hover:text-gray-200 transition">取消</button>

          {stage === 1 && (
            <button onClick={generateSheet} disabled={!canNextFromStage1}
              className="px-4 py-1.5 text-xs font-medium rounded-lg bg-gradient-to-r from-violet-600 to-cyan-600 text-white hover:from-violet-500 hover:to-cyan-500 disabled:opacity-40 disabled:cursor-not-allowed transition shadow-md shadow-violet-900/30 flex items-center gap-1.5">
              <Wand2 className="w-3.5 h-3.5" /> 生成三视图
            </button>
          )}
          {stage === 2 && (
            <>
              <button onClick={generateSheet} disabled={generating}
                className="px-3 py-1.5 text-xs font-medium rounded-lg bg-gray-800 border border-gray-700 text-gray-300 hover:text-white hover:bg-gray-700 disabled:opacity-40 transition flex items-center gap-1.5">
                <RefreshCw className={`w-3.5 h-3.5 ${generating ? 'animate-spin' : ''}`} /> 重新生成
              </button>
              <button onClick={() => setStage(3)} disabled={!sheetUrl || generating}
                className="px-4 py-1.5 text-xs font-medium rounded-lg bg-emerald-600 text-white hover:bg-emerald-500 disabled:opacity-40 disabled:cursor-not-allowed transition flex items-center gap-1.5">
                确认满意 <ChevronRight className="w-3.5 h-3.5" />
              </button>
            </>
          )}
          {stage === 3 && (
            <button onClick={submit}
              className="px-4 py-1.5 text-xs font-semibold rounded-lg bg-emerald-600 text-white hover:bg-emerald-500 transition flex items-center gap-1.5 shadow-lg shadow-emerald-900/30">
              <Check className="w-3.5 h-3.5" /> {isEdit ? `保存 ${nextTag}` : `入库 ${nextTag}`}
            </button>
          )}
          {/* 编辑模式：Stage 1 也允许直接保存元数据（不重新生成图） */}
          {isEdit && stage === 1 && (
            <button onClick={() => { submit() }} disabled={!name.trim() || !appearance.trim()}
              className="px-3 py-1.5 text-xs font-medium rounded-lg bg-gray-700 border border-gray-600 text-gray-200 hover:bg-gray-600 disabled:opacity-40 transition flex items-center gap-1.5">
              <Check className="w-3.5 h-3.5" /> 仅保存文字
            </button>
          )}
        </div>
      </div>
    </div>
  )
}

// ── Stepper ──
function Step({ idx, active, done, label }: { idx: number; active: boolean; done: boolean; label: string }) {
  return (
    <div className="flex items-center gap-1.5">
      <div className={`w-5 h-5 rounded-full flex items-center justify-center text-[10px] font-bold ${
        done ? 'bg-emerald-500 text-white' :
        active ? 'bg-violet-500 text-white ring-2 ring-violet-400/50' :
        'bg-gray-800 text-gray-500 border border-gray-700'
      }`}>
        {done ? <Check className="w-3 h-3" /> : idx}
      </div>
      <span className={`text-[11px] ${active ? 'text-white font-medium' : done ? 'text-emerald-400' : 'text-gray-500'}`}>
        {label}
      </span>
    </div>
  )
}

// ── Stage 1 ──
function Stage1(props: {
  name: string; setName: (v: string) => void
  role: string; setRole: (v: string) => void
  appearance: string; setAppearance: (v: string) => void
  style: string; setStyle: (v: string) => void
  // 服装阶段（B 路）——从 reference.md 表选一段，生图时追加到 prompt
  costume: string; setCostume: (v: string) => void
  // 生图模型——nano-banana-2/edit 默认，gpt-image-2/edit 是 OpenAI alpha 备选
  sheetModel: 'nano-banana-2/edit' | 'gpt-image-2/edit'
  setSheetModel: (v: 'nano-banana-2/edit' | 'gpt-image-2/edit') => void
  refPreview: string; refUrl: string; uploading: boolean
  onPickFile: () => void
  onPasteUrl: (url: string) => void
  // 自定义 role
  customRoles: string[]
  onRemoveCustomRole: (v: string) => void
  roleInputOpen: boolean
  roleDraft: string
  setRoleDraft: (v: string) => void
  onOpenRoleInput: () => void
  onCloseRoleInput: () => void
  onAddCustomRole: () => void
  // CDN 上传目标 + AI 外观卡
  uploadTarget: 'cdn' | 'local' | ''
  uploadError: string | null
  onClearUploadError: () => void
  genAppearLoading: boolean
  genAppearError: string | null
  onGenerateAppearance: () => void
}) {
  const { name, setName, role, setRole, appearance, setAppearance, style, setStyle,
    costume, setCostume,
    sheetModel, setSheetModel,
    refPreview, refUrl, uploading, onPickFile, onPasteUrl,
    customRoles, onRemoveCustomRole, roleInputOpen, roleDraft, setRoleDraft,
    onOpenRoleInput, onCloseRoleInput, onAddCustomRole,
    uploadTarget, uploadError, onClearUploadError,
    genAppearLoading, genAppearError, onGenerateAppearance } = props
  return (
    <div className="space-y-4">
      {/* 角色名 */}
      <div>
        <label className="block text-[11px] font-medium text-gray-400 mb-1.5 uppercase tracking-wider">角色名 *</label>
        <input
          value={name} onChange={e => setName(e.target.value)}
          placeholder="林见月 / ZERG / 苏蜜"
          className="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-sm text-gray-200 placeholder-gray-600 focus:border-violet-500 focus:outline-none"
          autoFocus />
      </div>

      {/* 定位（支持自定义 + 按钮） */}
      <div>
        <label className="block text-[11px] font-medium text-gray-400 mb-1.5 uppercase tracking-wider">定位</label>
        <div className="flex flex-wrap gap-1.5 items-center">
          {ROLE_PRESETS.map(r => (
            <button key={r.value} type="button" onClick={() => setRole(r.value)}
              className={`px-2.5 py-1 text-xs rounded-md border transition ${role === r.value ? r.color : 'bg-gray-800 text-gray-400 border-gray-700 hover:border-gray-600'}`}>
              {r.value}
            </button>
          ))}
          {customRoles.map(v => (
            <span key={v} className={`group inline-flex items-center gap-1 pl-2.5 pr-1 py-1 text-xs rounded-md border transition ${role === v ? CUSTOM_ROLE_COLOR : 'bg-gray-800 text-gray-400 border-gray-700 hover:border-gray-600'}`}>
              <button type="button" onClick={() => setRole(v)}>{v}</button>
              <button type="button" onClick={() => onRemoveCustomRole(v)} title="删除此自定义"
                className="text-gray-600 hover:text-red-400 opacity-0 group-hover:opacity-100 transition">
                <X className="w-3 h-3" />
              </button>
            </span>
          ))}
          {roleInputOpen ? (
            <span className="inline-flex items-center gap-1">
              <input
                value={roleDraft} onChange={e => setRoleDraft(e.target.value)}
                onKeyDown={e => {
                  if (e.key === 'Enter') { e.preventDefault(); onAddCustomRole() }
                  else if (e.key === 'Escape') { e.preventDefault(); onCloseRoleInput() }
                }}
                placeholder="e.g. 反派·大BOSS"
                autoFocus
                className="px-2 py-1 w-28 bg-gray-800 border border-teal-600/60 rounded-md text-xs text-teal-200 placeholder-gray-600 focus:outline-none focus:border-teal-400" />
              <button type="button" onClick={onAddCustomRole}
                className="px-1.5 py-1 text-xs rounded-md bg-teal-600 text-white hover:bg-teal-500 transition"><Check className="w-3 h-3" /></button>
              <button type="button" onClick={onCloseRoleInput}
                className="px-1.5 py-1 text-xs rounded-md bg-gray-800 border border-gray-700 text-gray-400 hover:text-gray-200 transition"><X className="w-3 h-3" /></button>
            </span>
          ) : (
            <button type="button" onClick={onOpenRoleInput}
              title="自定义定位（保存在本地）"
              className="px-2 py-1 text-xs rounded-md border border-dashed border-gray-600 text-gray-500 hover:text-teal-300 hover:border-teal-500/60 transition inline-flex items-center gap-1">
              <Plus className="w-3 h-3" /> 自定义
            </button>
          )}
        </div>
      </div>

      {/* 外观卡（含 AI 一键生成） */}
      <div>
        <div className="flex items-center justify-between mb-1.5">
          <label className="block text-[11px] font-medium text-gray-400 uppercase tracking-wider flex items-center gap-1.5">
            <Sparkles className="w-3 h-3" /> 外观卡 * <span className="text-gray-600 font-normal normal-case">（一字不差复用，要具体可拍）</span>
          </label>
          <button type="button" onClick={onGenerateAppearance} disabled={genAppearLoading || !name.trim()}
            title={!name.trim() ? '请先填角色名' : 'AI 基于角色名/定位/已有片段写一段具体可拍的外观卡'}
            className="px-2 py-1 text-[11px] rounded-md bg-gradient-to-r from-violet-600/80 to-fuchsia-600/80 hover:from-violet-500 hover:to-fuchsia-500 text-white inline-flex items-center gap-1 disabled:opacity-40 disabled:cursor-not-allowed transition shadow-sm shadow-violet-900/30">
            {genAppearLoading ? <Loader2 className="w-3 h-3 animate-spin" /> : <Wand2 className="w-3 h-3" />}
            AI 一键生成
          </button>
        </div>
        <textarea
          value={appearance} onChange={e => setAppearance(e.target.value)}
          rows={3}
          placeholder="薄荷绿古装汉服+透纱外袍的瘦弱年轻中国女子，黑色长直发柔顺，古风流苏耳环+翡翠银腰扣"
          className="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-sm text-gray-200 placeholder-gray-600 focus:border-violet-500 focus:outline-none resize-none" />
        {genAppearError && (
          <p className="mt-1 text-[10px] text-red-400 flex items-center gap-1"><AlertCircle className="w-3 h-3" />AI 生成失败：{genAppearError}</p>
        )}
        <p className="mt-1 text-[10px] text-gray-600">示例：服装+发型+体型+配色。不写抽象词（"美丽""优雅"）· AI 会参考你填过的片段和上传的参考图</p>
      </div>

      {/* 参考图上传 —— 上传按钮做成主操作（实心），URL 粘贴做成次操作（虚线框）。
          用户反馈：上一版两个同等灰按钮容易忽略「本地上传」入口。 */}
      <div>
        <label className="block text-[11px] font-medium text-gray-400 mb-1.5 uppercase tracking-wider flex items-center gap-1.5">
          <Upload className="w-3 h-3" /> 参考图 * <span className="text-gray-600 font-normal normal-case">（原照或已有三视图，fal 需公网可访问）</span>
          {uploadTarget === 'cdn' && <span className="px-1.5 py-0.5 rounded bg-emerald-600/20 border border-emerald-500/40 text-[9px] text-emerald-300 font-mono normal-case">CDN</span>}
          {uploadTarget === 'local' && <span className="px-1.5 py-0.5 rounded bg-amber-600/20 border border-amber-500/40 text-[9px] text-amber-300 font-mono normal-case">LOCAL</span>}
        </label>
        <div className="grid grid-cols-[1fr_140px] gap-2">
          <div className="space-y-1.5">
            {/* 主操作：本地文件 → cdn.starclaw.net */}
            <button onClick={onPickFile} disabled={uploading}
              title="从本地选择图片，上传到 cdn.starclaw.net（公网可访问，30 天稳定）"
              className="w-full px-3 py-2.5 rounded-lg text-xs font-medium transition flex items-center justify-center gap-1.5 disabled:opacity-50 disabled:cursor-not-allowed
                bg-gradient-to-r from-violet-600 to-cyan-600 hover:from-violet-500 hover:to-cyan-500 text-white shadow-md shadow-violet-900/30">
              {uploading
                ? <><Loader2 className="w-3.5 h-3.5 animate-spin" /> 上传中…</>
                : <><Upload className="w-3.5 h-3.5" /> 选择本地图片上传到 cdn.starclaw.net</>}
            </button>
            {/* 次操作：粘贴公网 URL */}
            <input
              value={refUrl} onChange={e => onPasteUrl(e.target.value.trim())}
              placeholder="或粘贴已有公网 URL（fal/Seedance 需可直接访问）"
              className="w-full px-3 py-2 bg-gray-800 border border-dashed border-gray-700 rounded-lg text-xs font-mono text-gray-200 placeholder-gray-600 focus:border-violet-500 focus:outline-none" />
          </div>
          <div className="h-[88px] rounded-lg border border-gray-700 bg-gray-950 overflow-hidden flex items-center justify-center">
            {refPreview || refUrl ? (
              <img
                key={refPreview || refUrl}
                src={refPreview || refUrl} alt="" className="w-full h-full object-cover"
                onError={e => { (e.target as HTMLImageElement).style.display = 'none' }} />
            ) : (
              <ImageIcon className="w-6 h-6 text-gray-700" />
            )}
          </div>
        </div>
        {/* 成功：CDN */}
        {uploadTarget === 'cdn' && refUrl && !uploadError && (
          <div className="mt-1.5 p-2 rounded-md bg-emerald-900/20 border border-emerald-700/30 text-[10px] text-emerald-300 flex items-start gap-1.5">
            <Check className="w-3 h-3 flex-shrink-0 mt-0.5" />
            <div className="min-w-0 flex-1">
              <span className="font-medium">已上传到 cdn.starclaw.net</span>
              <p className="font-mono text-emerald-400/80 mt-0.5 truncate" title={refUrl}>{refUrl}</p>
            </div>
          </div>
        )}
        {/* 失败 / fallback 到本地 —— inline 展示，可以截图/复制 */}
        {uploadError && (
          <div className="mt-1.5 p-2 rounded-md bg-amber-900/20 border border-amber-700/40 text-[10px] text-amber-300 flex items-start gap-1.5">
            <AlertCircle className="w-3 h-3 flex-shrink-0 mt-0.5" />
            <div className="min-w-0 flex-1">
              <p className="leading-relaxed break-words">{uploadError}</p>
            </div>
            <button type="button" onClick={onClearUploadError}
              className="text-amber-500 hover:text-amber-200 transition flex-shrink-0" title="关闭提示">
              <X className="w-3 h-3" />
            </button>
          </div>
        )}
      </div>

      {/* 风格 */}
      <div>
        <label className="block text-[11px] font-medium text-gray-400 mb-1.5 uppercase tracking-wider">风格</label>
        <div className="grid grid-cols-3 gap-2">
          {STYLE_PRESETS.map(s => (
            <button key={s.value} type="button" onClick={() => setStyle(s.value)}
              className={`px-2.5 py-2 text-xs rounded-lg border transition text-left ${
                style === s.value
                  ? 'bg-violet-500/20 border-violet-500/60 text-violet-200'
                  : 'bg-gray-800 border-gray-700 text-gray-400 hover:border-gray-600'
              }`}>
              <div className="font-medium">{s.label}</div>
              <div className="text-[9px] opacity-70 mt-0.5">{s.hint}</div>
            </button>
          ))}
        </div>
      </div>

      {/* 服装阶段（B 路 · 对齐 reference.md 4 段服装表） */}
      <div>
        <label className="block text-[11px] font-medium text-gray-400 mb-1.5 uppercase tracking-wider flex items-center gap-1.5">
          服装阶段 <span className="text-gray-600 font-normal normal-case">（按剧情阶段换装，会追加到生图 prompt）</span>
        </label>
        <select
          value={costume}
          onChange={e => setCostume(e.target.value)}
          className="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-xs text-gray-200 focus:border-violet-500 focus:outline-none">
          {COSTUME_PRESETS.map(c => (
            <option key={c.value || '_default'} value={c.value}>{c.label}</option>
          ))}
        </select>
        {costume && (() => {
          const desc = COSTUME_PRESETS.find(c => c.value === costume)?.desc
          return desc ? (
            <p className="mt-1 text-[10px] text-gray-500 leading-relaxed font-mono">
              {'→ Costume: ' + desc}
            </p>
          ) : null
        })()}
      </div>

      {/* 生图模型（nano-banana-2/edit 默认；gpt-image-2/edit 是 OpenAI alpha 备选） */}
      <div>
        <label className="block text-[11px] font-medium text-gray-400 mb-1.5 uppercase tracking-wider">
          生图模型 <span className="text-gray-600 font-normal normal-case">（参考图 → 三视图 sheet 的 i2i 模型）</span>
        </label>
        <div className="grid grid-cols-2 gap-2">
          <button type="button" onClick={() => setSheetModel('nano-banana-2/edit')}
            className={`px-2.5 py-2 text-xs rounded-lg border transition text-left ${
              sheetModel === 'nano-banana-2/edit'
                ? 'bg-violet-500/20 border-violet-500/60 text-violet-200'
                : 'bg-gray-800 border-gray-700 text-gray-400 hover:border-gray-600'
            }`}>
            <div className="font-medium font-mono">nano-banana-2/edit</div>
            <div className="text-[9px] opacity-70 mt-0.5">Gemini 2.5 Flash · EP01-06 验证 · 默认</div>
          </button>
          <button type="button" onClick={() => setSheetModel('gpt-image-2/edit')}
            className={`px-2.5 py-2 text-xs rounded-lg border transition text-left ${
              sheetModel === 'gpt-image-2/edit'
                ? 'bg-emerald-500/20 border-emerald-500/60 text-emerald-200'
                : 'bg-gray-800 border-gray-700 text-gray-400 hover:border-gray-600'
            }`}>
            <div className="font-medium font-mono">gpt-image-2/edit</div>
            <div className="text-[9px] opacity-70 mt-0.5">OpenAI alpha · 4K · 文字渲染更稳</div>
          </button>
        </div>
      </div>

      <div className="p-2.5 rounded-lg bg-cyan-900/20 border border-cyan-700/30 text-[11px] text-cyan-200 flex items-start gap-2">
        <Sparkles className="w-3.5 h-3.5 flex-shrink-0 mt-0.5" />
        <p>
          下一步会用 <span className="font-mono text-cyan-300">{sheetModel}</span> 基于参考图生成一张综合 sheet：
          <span className="text-cyan-100">近景正侧 + 全身三视图 + 三表情</span>，横版 16:9。
          生成完确认满意后自动入库为 <span className="font-mono">{name ? `media-${name}` : '角色节点'}</span>。
        </p>
      </div>
    </div>
  )
}

// ── Stage 2 ──
function Stage2({ prompt, refUrl, sheetUrl, generating, error, attempts }: {
  prompt: string; refUrl: string; sheetUrl: string
  generating: boolean; error: string | null; attempts: number
}) {
  return (
    <div className="space-y-3">
      {/* Prompt 预览 */}
      <div className="p-2.5 rounded-lg bg-gray-850/60 border border-gray-700/50">
        <div className="flex items-center justify-between mb-1">
          <span className="text-[10px] font-medium text-gray-500 uppercase tracking-wider">nano-banana 提示词（EP04 成熟模板）</span>
          <span className="text-[10px] text-gray-500">第 {attempts || 1} 次尝试</span>
        </div>
        <p className="text-[11px] text-gray-300 font-mono leading-relaxed whitespace-pre-wrap">{prompt}</p>
      </div>

      {/* 双栏预览：参考图 + 生成 sheet */}
      <div className="grid grid-cols-2 gap-3">
        <div>
          <div className="text-[10px] font-medium text-gray-500 mb-1 uppercase tracking-wider">原参考图</div>
          <div className="aspect-[16/9] rounded-lg border border-gray-700 bg-gray-950 overflow-hidden flex items-center justify-center">
            {refUrl ? <img src={refUrl} alt="" className="w-full h-full object-contain" /> : <ImageIcon className="w-8 h-8 text-gray-700" />}
          </div>
        </div>
        <div>
          <div className="text-[10px] font-medium text-gray-500 mb-1 uppercase tracking-wider">
            生成的三视图 {sheetUrl && <span className="text-emerald-400 normal-case">· 已就绪</span>}
          </div>
          <div className="aspect-[16/9] rounded-lg border border-violet-700/50 bg-gray-950 overflow-hidden flex items-center justify-center relative">
            {generating ? (
              <div className="flex flex-col items-center gap-2 text-gray-400">
                <Loader2 className="w-8 h-8 animate-spin text-violet-400" />
                <span className="text-[11px]">nano-banana-2/edit 生成中…</span>
                <span className="text-[9px] text-gray-600">通常 10-30 秒</span>
              </div>
            ) : error ? (
              <div className="flex flex-col items-center gap-2 text-red-400 px-4 text-center">
                <AlertCircle className="w-6 h-6" />
                <span className="text-[11px] leading-relaxed">{error}</span>
              </div>
            ) : sheetUrl ? (
              <img src={sheetUrl} alt="" className="w-full h-full object-contain" />
            ) : (
              <ImageIcon className="w-8 h-8 text-gray-700" />
            )}
          </div>
        </div>
      </div>

      {sheetUrl && (
        <div className="p-2.5 rounded-lg bg-emerald-900/20 border border-emerald-700/30 text-[11px] text-emerald-200 flex items-start gap-2">
          <Check className="w-3.5 h-3.5 flex-shrink-0 mt-0.5" />
          <div>
            <p className="font-medium">已生成稳定本地 URL</p>
            <p className="font-mono text-[10px] text-emerald-300/80 mt-0.5 break-all">{sheetUrl}</p>
            <p className="text-[10px] text-emerald-300/60 mt-1">
              ※ 供视频生成（Seedance）使用时，系统会按需自动转换为 Ark TOS URL（24h 信任链）
            </p>
          </div>
        </div>
      )}
    </div>
  )
}

// ── Stage 3 ──
function Stage3({ name, nextTag, role, appearance, sheetUrl, refUrl }: {
  name: string; nextTag: string; role: string; appearance: string
  sheetUrl: string; refUrl: string
}) {
  return (
    <div className="space-y-3">
      <div className="rounded-xl border border-violet-700/50 bg-gradient-to-br from-violet-900/20 via-gray-900 to-cyan-900/10 overflow-hidden">
        {/* 成品 sheet */}
        <div className="aspect-[16/9] bg-gray-950 border-b border-violet-700/40">
          {sheetUrl ? (
            <img src={sheetUrl} alt="" className="w-full h-full object-contain" />
          ) : refUrl ? (
            <img src={refUrl} alt="" className="w-full h-full object-contain" />
          ) : null}
        </div>
        {/* 信息卡 */}
        <div className="p-4 space-y-2">
          <div className="flex items-center gap-2">
            <h4 className="text-lg font-bold text-white">{name || '（未命名）'}</h4>
            <span className="px-2 py-0.5 rounded bg-violet-500/30 border border-violet-500/50 text-[10px] font-mono text-violet-200">{nextTag}</span>
            <span className="px-2 py-0.5 rounded bg-gray-800 border border-gray-700 text-[10px] text-gray-300">{role}</span>
          </div>
          <p className="text-xs text-gray-300 leading-relaxed">{appearance || '（外观卡为空）'}</p>
          <div className="flex items-center gap-4 text-[10px] text-gray-500 pt-2 border-t border-gray-800">
            <div>
              <span className="text-gray-600">参考图：</span>
              <span className="font-mono text-gray-400 truncate max-w-[180px] inline-block align-bottom" title={refUrl}>{refUrl.slice(-40)}</span>
            </div>
            <div>
              <span className="text-gray-600">三视图：</span>
              <span className="font-mono text-emerald-400 truncate max-w-[180px] inline-block align-bottom" title={sheetUrl}>{sheetUrl.slice(-40) || '（未生成）'}</span>
            </div>
          </div>
        </div>
      </div>
      <div className="p-2.5 rounded-lg bg-cyan-900/20 border border-cyan-700/30 text-[11px] text-cyan-200">
        💡 入库后可在左侧「角色」列表查看 · 编辑 tag/role/外观卡 请进入节点属性面板 · 分镜 prompt 里写 <span className="font-mono bg-black/30 px-1 rounded">{nextTag}</span> 可自动绑定此角色
      </div>
    </div>
  )
}
