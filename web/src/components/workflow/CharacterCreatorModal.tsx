import { useRef, useState } from 'react'
import { X, Users, Upload, Sparkles, Loader2, RefreshCw, Check, ChevronLeft, ChevronRight, AlertCircle, Image as ImageIcon, Wand2 } from 'lucide-react'
import type { CharacterData } from './episodeTypes'
import { fileAPI, imageAPI } from '../../lib/api'

interface Props {
  open: boolean
  existingTags: string[]       // e.g. ["[图1]","[图2]"]
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

const STYLE_PRESETS = [
  { value: 'realistic', label: '写实', hint: 'EP01-04 验证，适配真人短剧' },
  { value: 'anime',     label: '动漫', hint: '二次元角色' },
  { value: '3d',        label: '3D',   hint: 'CG 风格' },
]

type Stage = 1 | 2 | 3

// EP04 成熟经验：prompt 简短精确，总长 < 100 词
// 结构：角色描述 + 布局说明 + 风格 + 质量
function buildSheetPrompt(appearance: string, style: string): string {
  const styleWord = style === 'realistic' ? 'realistic' : style === 'anime' ? 'anime' : '3D CG'
  return [
    `Character reference sheet, same face as reference image.`,
    appearance.trim(),
    `Top: close-up portrait front and side view.`,
    `Middle: full body front, side, back view.`,
    `Bottom: 3 expressions side by side (neutral, smile, determined).`,
    `White background, ${styleWord} style, 8K, no text.`,
  ].filter(Boolean).join(' ')
}

export default function CharacterCreatorModal({ open, existingTags, onClose, onCreate }: Props) {
  const [stage, setStage] = useState<Stage>(1)
  // Stage 1 变量（最小集）
  const [name, setName] = useState('')
  const [role, setRole] = useState('女一')
  const [appearance, setAppearance] = useState('')
  const [style, setStyle] = useState('realistic')
  const [refUrl, setRefUrl] = useState('')            // 参考图稳定 URL（fal 可直接访问）
  const [refPreview, setRefPreview] = useState('')    // 本地预览（data: 或 /v1/uploads/）
  const [uploading, setUploading] = useState(false)
  // Stage 2 生成结果
  const [generating, setGenerating] = useState(false)
  const [genError, setGenError] = useState<string | null>(null)
  const [sheetUrl, setSheetUrl] = useState('')       // nano-banana 生成的 sheet 稳定 URL
  const [attempts, setAttempts] = useState(0)

  const fileRef = useRef<HTMLInputElement>(null)

  if (!open) return null

  // 自动分配 [图N] tag
  const nextTag = (() => {
    const used = new Set<number>()
    existingTags.forEach(t => {
      const m = t.match(/\[图(\d+)\]/)
      if (m) used.add(parseInt(m[1], 10))
    })
    for (let i = 1; i <= 99; i++) if (!used.has(i)) return `[图${i}]`
    return `[图${existingTags.length + 1}]`
  })()

  const canNextFromStage1 = name.trim() && appearance.trim() && refUrl

  // ── 参考图上传 ──
  const handleFilePick = async (file: File) => {
    setUploading(true)
    try {
      const preview = URL.createObjectURL(file)
      setRefPreview(preview)
      const res = await fileAPI.upload(file)
      const url: string = (res.data as Record<string, unknown>).url as string
      if (!url) throw new Error('上传未返回 URL')
      // 本地 /v1/uploads/xxx → 拼上 origin 成完整公网 URL（fal 需要公网可访问）
      const absolute = url.startsWith('http') ? url : `${window.location.origin}${url}`
      setRefUrl(absolute)
    } catch (e) {
      alert(`上传失败：${e instanceof Error ? e.message : String(e)}`)
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
      const prompt = buildSheetPrompt(appearance, style)
      const res = await imageAPI.generate({
        prompt,
        model: 'nano-banana-2/edit',
        image_url: refUrl,
        size: 'landscape_16_9',
        scene: `character_${name.trim().replace(/\s+/g, '_')}`,
        style,
        negative_prompt: 'blurry, low quality, text, watermark, deformed, extra fingers, bad anatomy',
      })
      const data = res.data as Record<string, unknown>
      const url = (data.local_url || data.display_url || data.url) as string
      if (!url) throw new Error('生成未返回有效 URL')
      // 若是相对 /v1/images/xxx，拼完整域名让外部模型后续也能拉
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
            <h3 className="text-sm font-semibold text-gray-100">角色工坊 · 新建角色</h3>
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
          <Step idx={2} active={stage === 2} done={stage > 2} label="② nano-banana 生成三视图" />
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
              refPreview={refPreview} refUrl={refUrl}
              uploading={uploading}
              onPickFile={() => fileRef.current?.click()}
              onPasteUrl={url => { setRefUrl(url); setRefPreview(url) }}
            />
          )}
          {stage === 2 && (
            <Stage2
              prompt={buildSheetPrompt(appearance, style)}
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
              <Check className="w-3.5 h-3.5" /> 入库 {nextTag}
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
  refPreview: string; refUrl: string; uploading: boolean
  onPickFile: () => void
  onPasteUrl: (url: string) => void
}) {
  const { name, setName, role, setRole, appearance, setAppearance, style, setStyle,
    refPreview, refUrl, uploading, onPickFile, onPasteUrl } = props
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

      {/* 定位 */}
      <div>
        <label className="block text-[11px] font-medium text-gray-400 mb-1.5 uppercase tracking-wider">定位</label>
        <div className="flex flex-wrap gap-1.5">
          {ROLE_PRESETS.map(r => (
            <button key={r.value} type="button" onClick={() => setRole(r.value)}
              className={`px-2.5 py-1 text-xs rounded-md border transition ${role === r.value ? r.color : 'bg-gray-800 text-gray-400 border-gray-700 hover:border-gray-600'}`}>
              {r.value}
            </button>
          ))}
        </div>
      </div>

      {/* 外观卡 */}
      <div>
        <label className="block text-[11px] font-medium text-gray-400 mb-1.5 uppercase tracking-wider flex items-center gap-1.5">
          <Sparkles className="w-3 h-3" /> 外观卡 * <span className="text-gray-600 font-normal normal-case">（一字不差复用，要具体可拍）</span>
        </label>
        <textarea
          value={appearance} onChange={e => setAppearance(e.target.value)}
          rows={3}
          placeholder="薄荷绿古装汉服+透纱外袍的瘦弱年轻中国女子，黑色长直发柔顺，古风流苏耳环+翡翠银腰扣"
          className="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-sm text-gray-200 placeholder-gray-600 focus:border-violet-500 focus:outline-none resize-none" />
        <p className="mt-1 text-[10px] text-gray-600">示例：服装+发型+体型+配色。不写抽象词（"美丽""优雅"）</p>
      </div>

      {/* 参考图上传 */}
      <div>
        <label className="block text-[11px] font-medium text-gray-400 mb-1.5 uppercase tracking-wider flex items-center gap-1.5">
          <Upload className="w-3 h-3" /> 参考图 * <span className="text-gray-600 font-normal normal-case">（原照或已有三视图，fal 需公网可访问）</span>
        </label>
        <div className="grid grid-cols-[1fr_140px] gap-2">
          <div>
            <input
              value={refUrl} onChange={e => onPasteUrl(e.target.value.trim())}
              placeholder="粘贴公网 URL · 或 →"
              className="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-xs font-mono text-gray-200 placeholder-gray-600 focus:border-violet-500 focus:outline-none" />
            <button onClick={onPickFile} disabled={uploading}
              className="mt-1.5 w-full px-3 py-2 bg-gray-800 border border-dashed border-gray-700 hover:border-violet-500/60 text-xs text-gray-300 rounded-lg transition flex items-center justify-center gap-1.5 disabled:opacity-50">
              {uploading ? <><Loader2 className="w-3.5 h-3.5 animate-spin" /> 上传中…</> : <><Upload className="w-3.5 h-3.5" /> 从本地上传</>}
            </button>
          </div>
          <div className="h-[88px] rounded-lg border border-gray-700 bg-gray-950 overflow-hidden flex items-center justify-center">
            {refPreview || refUrl ? (
              <img src={refPreview || refUrl} alt="" className="w-full h-full object-cover"
                onError={e => { (e.target as HTMLImageElement).style.display = 'none' }} />
            ) : (
              <ImageIcon className="w-6 h-6 text-gray-700" />
            )}
          </div>
        </div>
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

      <div className="p-2.5 rounded-lg bg-cyan-900/20 border border-cyan-700/30 text-[11px] text-cyan-200 flex items-start gap-2">
        <Sparkles className="w-3.5 h-3.5 flex-shrink-0 mt-0.5" />
        <p>
          下一步会用 <span className="font-mono text-cyan-300">nano-banana-2/edit</span> 基于参考图生成一张综合 sheet：
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
