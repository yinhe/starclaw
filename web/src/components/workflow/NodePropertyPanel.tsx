import { useState, useEffect, useMemo, useCallback } from 'react'
import { X, Wand2, Sparkles, Loader2, ImageIcon, RefreshCw, Pin, Banana, Upload, Link2, Trash2, Copy, ExternalLink } from 'lucide-react'
import type { Node } from '@xyflow/react'
import { parseTOSFreshness, freshnessLabel, refreshTOS } from './tosUrlUtils'
import { projectAPI, nanoAPI, cdnAPI } from '../../lib/api'

interface Props {
  node: Node | null
  models: { id: string; provider: string; model_name: string; display_name: string }[]
  tools: string[]
  onUpdate: (id: string, data: Record<string, unknown>) => void
  onClose: () => void
  /** 点击角色节点的「打开向导」按钮时触发，父组件负责打开 CharacterCreatorModal 的 edit 模式 */
  onEditCharacter?: (nodeId: string) => void
  /** 点击道具节点的「打开道具工坊」按钮时触发，父组件负责打开 PropEditorModal */
  onEditProp?: (nodeId: string) => void
}

const MEDIA_CATEGORIES = [
  { value: 'character', label: '角色' },
  { value: 'scene', label: '场景' },
  { value: 'prop', label: '道具' },
  { value: 'costume', label: '服装' },
  { value: 'reference', label: '参考' },
]

const APPEARANCE_FORM_LABELS: Record<string, string> = {
  wandering: '流浪/坠落虚弱态',
  normal: '正常形态',
  queen: '女王激活/虫后共振态',
}

const ZERG_APPEARANCE_CARDS: Record<string, string> = {
  wandering: '流浪/坠落虚弱态 ZERG（Stage 0）：刚坠落到现代、在街头流浪的外星生物机械犬，中型偏小但更瘦弱，体态蜷缩防御、动作迟缓；深灰与灰黑色甲壳暗淡失光，表面有裂纹、刮痕和尘土磨损，机械关节局部外露；cyan 青色光纹几乎不亮，只在眼睛、脊背和四肢缝隙里微弱闪烁；小而警觉的青色眼睛带疲惫感，耳朵略下垂，尾巴低垂或收紧；整体是无家可归、能量不足但仍有灵性的流浪机械犬，不要华丽金色装饰，不要健康高亮状态',
  normal: '正常/基础 Claw 态 ZERG（Stage 1）：恢复正常后的外星生物机械交易犬，中型犬体型，整体像机敏的机械柴犬/猎犬混合体；紫黑色与深灰色甲壳装甲完整，带细微六边形纹样，机械龙虾/节肢基体完整但仍保持犬类轮廓；青色 cyan 光纹稳定地沿脊背、四肢和关节流动，小而警觉的青色发光眼睛更主动；身体线条灵活敏捷，三角尖耳立起，深色爪，尾巴自然上扬；这是 EP07-20 的基础 Claw 正常态，不要虚弱破损，不要女王共振的半透明华丽态',
  queen: '女王激活/虫后共振态 ZERG（Stage 3）：收到 Queen 上层信号后被激活的虫后共振形态，仍是外星生物机械犬/交易犬轮廓，但姿态沉稳威严；全身 cyan 光纹连成复杂虫群网络，偶尔闪过金色信号，壳上出现更高阶的新纹样；甲壳呈半透明紫黑与深灰质感，内部能量网流动，脊背、胸口和四肢关节出现暗金色能量节点；青色眼睛更亮更深，像接入上层意志；整体强能量场、神圣但克制，表现 Queen 共振与虫群秩序，不要变成人形，不要丢失 ZERG 机械犬身份',
}

function normalizeAppearanceCards(value: unknown): Record<string, string> {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return {}
  const out: Record<string, string> = {}
  for (const [key, card] of Object.entries(value as Record<string, unknown>)) {
    if (typeof card === 'string' && card.trim()) out[key] = card
  }
  return out
}

function isZergData(data: Record<string, unknown>): boolean {
  const haystack = [data.key, data.label, data.description, data.appearance_card]
    .map((v) => String(v || '').toLowerCase())
    .join(' ')
  return haystack.includes('zerg') || haystack.includes('虫族种子')
}

function appearanceCardsForData(data: Record<string, unknown>): Record<string, string> {
  const cards = normalizeAppearanceCards(data.appearance_cards)
  if (Object.keys(cards).length > 0) return cards
  return isZergData(data) ? ZERG_APPEARANCE_CARDS : {}
}

function appearanceFormLabel(key: string): string {
  return APPEARANCE_FORM_LABELS[key] || key
}

function inferAppearanceFormFromUrl(url: string, keys: string[]): string {
  const lower = url.toLowerCase()
  if ((lower.includes('stage0') || lower.includes('weak') || lower.includes('wandering')) && keys.includes('wandering')) return 'wandering'
  if ((lower.includes('stage3') || lower.includes('queen')) && keys.includes('queen')) return 'queen'
  if ((lower.includes('stage1') || lower.includes('normal') || lower.includes('healthy')) && keys.includes('normal')) return 'normal'
  if ((lower.includes('turnaround') || lower.includes('ref.png') || lower.includes('unified_sheet_v1')) && keys.includes('normal')) return 'normal'
  return ''
}

function zergFormImageUrl(form: string): string {
  if (form === 'wandering') return '/v1/projects/swarm-universe/production/characters/zerg/unified_sheet_stage0.png'
  if (form === 'queen') return '/v1/projects/swarm-universe/production/characters/zerg/unified_sheet_stage3.png'
  if (form === 'normal') return '/v1/projects/swarm-universe/production/characters/zerg/unified_sheet_stage1.png'
  return ''
}

export default function NodePropertyPanel({ node, models, tools, onUpdate, onClose, onEditCharacter, onEditProp }: Props) {
  const [localData, setLocalData] = useState<Record<string, unknown>>({})
  const [launderingTOS, setLaunderingTOS] = useState(false)
  const [launderErr, setLaunderErr] = useState<string>('')
  const [lightboxURL, setLightboxURL] = useState<string>('')
  const [lightboxLabel, setLightboxLabel] = useState<string>('')

  useEffect(() => {
    if (node) {
      setLocalData({ ...(node.data as Record<string, unknown>) })
      setLaunderErr('')
    }
  }, [node])

  // ESC 关闭大图
  useEffect(() => {
    if (!lightboxURL) return
    const onKey = (e: KeyboardEvent) => { if (e.key === 'Escape') setLightboxURL('') }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [lightboxURL])

  const openLightbox = (url: string, label: string) => {
    if (!url) return
    setLightboxURL(url)
    setLightboxLabel(label)
  }

  if (!node) return null

  const update = (key: string, value: unknown) => {
    const next = { ...localData, [key]: value }
    setLocalData(next)
    onUpdate(node.id, next)
  }

  const switchAppearanceForm = (form: string) => {
    const cards = appearanceCardsForData(localData)
    const next: Record<string, unknown> = { ...localData, appearance_form: form }
    if (cards[form]) next.appearance_card = cards[form]
    if (localData.key === 'zerg') {
      const formImageUrl = zergFormImageUrl(form)
      if (formImageUrl) next.imageUrl = formImageUrl
      next.cdn_url = ''
      next.tos_url = ''
    }
    setLocalData(next)
    onUpdate(node.id, next)
  }

  const pickCharacterImage = (url: string) => {
    const cards = appearanceCardsForData(localData)
    const keys = Object.keys(cards)
    const form = inferAppearanceFormFromUrl(url, keys)
    const next: Record<string, unknown> = { ...localData, imageUrl: url }
    if (form) {
      next.appearance_form = form
      next.appearance_card = cards[form]
      if (localData.key === 'zerg') {
        next.cdn_url = ''
        next.tos_url = ''
      }
    }
    setLocalData(next)
    onUpdate(node.id, next)
  }

  // 把字段写回 manifest.json —— 用于 tos_url / cdn_url 这些「生成出来的值」，
  // 否则只活在 React state 里，刷新页面从 manifest re-seed 就丢了（用户反馈：
  // 「我生成过的 tos URL 没有自动保存」）。只对有 category+key 的 media 节点生效。
  // 失败不 throw：调用点在 finally 外 await，失败只打日志 + 显示错误提示，
  // React state 里的值不回退（本会话仍可用）。
  const persistFieldToManifest = async (field: string, value: string) => {
    const category = (localData.category as string) || ''
    const key = (localData.key as string) || ''
    if (!key) return { ok: false, reason: 'no-key' as const }
    let kind: 'characters' | 'props' | null = null
    if (category === 'character') kind = 'characters'
    else if (category === 'prop') kind = 'props'
    if (!kind) return { ok: false, reason: 'unsupported-category' as const }
    try {
      // Only project currently alive in this workspace; matches the literal
      // used by syncImageToCDN above and the hard-coded 'swarm-universe' in
      // WorkflowPage.tsx. When we go multi-project, thread project as a prop.
      await projectAPI.patchManifestEntity('swarm-universe', kind, key, { [field]: value })
      return { ok: true as const }
    } catch (e) {
      const ax = e as { response?: { data?: { error?: string; detail?: string } }; message?: string }
      const msg = ax?.response?.data?.error || ax?.response?.data?.detail || ax?.message || '写回 manifest 失败'
      console.warn(`[NodePropertyPanel] persistFieldToManifest(${field}) failed:`, msg)
      return { ok: false as const, reason: 'api-error' as const, detail: msg }
    }
  }

  // launder 过程中若被 Seedream 敏感检测挡下（HTTP 422 + reason=seedream_sensitive_content），
  // 面板会显示「📎 用 CDN URL 代替」按钮让用户一键兜底。
  const [tosSensitiveBlock, setTosSensitiveBlock] = useState(false)
  // 成功后的简短提示（4s 自动消失）。用户反馈「点完什么都没变」—— 实际有变化只是没 affordance。
  const [launderOk, setLaunderOk] = useState<string | null>(null)

  // 生成 / 刷新 TOS URL。首选 resign（HMAC 7d，零成本）→ promote → Seedream launder（24h）
  const launderTOS = async () => {
    const hasAppearanceForms = Object.keys(appearanceCardsForData(localData)).length > 0
    const oldTOSUrl = hasAppearanceForms ? '' : ((localData.tos_url as string) || '')
    const fallbackSrc = ((localData.cdn_url as string) || (localData.imageUrl as string) || '').trim()
    if (!oldTOSUrl && !fallbackSrc) {
      setLaunderErr('先填「本地图片 URL」或「CDN URL」作为 laundering 源')
      return
    }
    setLaunderErr('')
    setLaunderOk(null)
    setTosSensitiveBlock(false)
    setLaunderingTOS(true)
    try {
      // Forward character context to the backend so Seedream retry prompts are
      // anchored ("Subject is [图1]林见月, 薄荷绿古装汉服…, live-action realism,
      // not cartoon") instead of the generic photorealism-only nudges.
      // Reads directly from localData — populated from manifest.json on open.
      const char = {
        appearance_card: (localData.appearance_card as string) || '',
        label: (localData.label as string) || '',
        tag: (localData.tag as string) || '',
      }
      const r = await refreshTOS(oldTOSUrl, fallbackSrc, char)
      update('tos_url', r.tosUrl)
      console.log(`[NodePropertyPanel] tos_url refreshed via ${r.source}${r.expiresSec ? ` (${r.expiresSec}s)` : ''}`)
      // 持久化到 manifest.json —— 这样刷新页面重新 seed 也能读到新 TOS URL。
      const persist = await persistFieldToManifest('tos_url', r.tosUrl)
      if (!persist.ok && persist.reason === 'api-error') {
        setLaunderErr(`TOS URL 已生成但写回 manifest 失败：${persist.detail}。当前会话内可用，刷新后会丢失。`)
      } else {
        // 成功 → 显示 toast 4s 然后自动消失
        const tag = r.source === 'launder' ? 'Seedream 重新生成（3K PNG · 24h）'
          : r.source === 'resign' ? 'HMAC 重签（源文件未变 · 7d）'
          : r.source === 'promote' ? 'promote 到 sheet（24h）' : r.source
        setLaunderOk(`✓ TOS 已更新 · ${tag}`)
        setTimeout(() => setLaunderOk(null), 4000)
      }
    } catch (e) {
      const err = e as { response?: { status?: number; data?: { error?: string; detail?: string; reason?: string; hint?: string } }; message?: string }
      // Special-case Seedream 敏感内容：422 + reason="seedream_sensitive_content"
      if (err.response?.status === 422 && err.response?.data?.reason === 'seedream_sensitive_content') {
        setTosSensitiveBlock(true)
        setLaunderErr(err.response?.data?.error || 'Seedream 内容过滤拒绝了这张图')
      } else {
        setLaunderErr(err.response?.data?.error || err.response?.data?.detail || err.message || '生成 TOS URL 失败')
      }
    } finally {
      setLaunderingTOS(false)
    }
  }

  // CDN 同步：把当前本地 imageUrl 推到 cdn.starclaw.net，成功后回写 cdn_url。
  const [cdnSyncing, setCdnSyncing] = useState(false)
  const [cdnSyncMsg, setCdnSyncMsg] = useState<{ tone: 'ok'|'err'; text: string } | null>(null)
  const syncImageToCDN = async () => {
    setCdnSyncMsg(null)
    const src = ((localData.imageUrl as string) || '').trim()
    if (!src) { setCdnSyncMsg({ tone: 'err', text: '先填「本地图片 URL」' }); return }
    if (src.startsWith('https://cdn.starclaw.net/') || src.startsWith('http://cdn.starclaw.net/')) {
      setCdnSyncMsg({ tone: 'err', text: '这就是 CDN URL 了，直接填到 CDN URL 框即可' }); return
    }
    // Derive a reasonable drama/asset_type from key + category for nice CDN paths.
    const drama = 'swarm-universe'
    const cat = (localData.category as string) || 'misc'
    const assetType = cat === 'character' ? 'characters' : cat === 'prop' ? 'props' : cat === 'scene' ? 'scenes' : 'misc'
    setCdnSyncing(true)
    try {
      const res = await cdnAPI.uploadFromLocal(src, { drama, asset_type: assetType })
      const d = res.data
      if (d.url) {
        update('cdn_url', d.url)
        setCdnSyncMsg({ tone: 'ok', text: `已推到 CDN: ${d.url.replace(/^https?:\/\/cdn\.starclaw\.net/, '')}` })
      } else {
        setCdnSyncMsg({ tone: 'err', text: '后端没返回 url' })
      }
    } catch (e) {
      const err = e as { response?: { status?: number; data?: { error?: string; hint?: string } }; message?: string }
      const msg = err.response?.data?.error || err.message || 'CDN 同步失败'
      const hint = err.response?.data?.hint
      setCdnSyncMsg({ tone: 'err', text: hint ? `${msg} · ${hint}` : msg })
    } finally {
      setCdnSyncing(false)
    }
  }

  // 「用 CDN URL 代替 TOS」：当 Seedream 连续 3 次被敏感过滤挡下时，直接把 cdn_url 赋给 tos_url。
  // Seedance 对 cdn.starclaw.net 域是白名单（EP04 已验证），完全可用，只是没有 24h 过期那层保护。
  const useCdnAsTos = async () => {
    const cdn = ((localData.cdn_url as string) || '').trim()
    if (!cdn) {
      setLaunderErr('还没有 CDN URL —— 先点「🔼 同步到 CDN」把本地图推上去')
      return
    }
    update('tos_url', cdn)
    setTosSensitiveBlock(false)
    setLaunderErr('')
    // 同 launderTOS：写回 manifest 才能跨会话保留
    const persist = await persistFieldToManifest('tos_url', cdn)
    if (!persist.ok && persist.reason === 'api-error') {
      setLaunderErr(`已用 CDN URL 代替 TOS 但写回 manifest 失败：${persist.detail}。当前会话可用，刷新后会丢。`)
    }
  }

  // TOS URL 新鲜度（每 30s 重算一次以驱动 label 更新，不做高频 setInterval 避免浪费）
  const tosFreshness = useMemo(() => parseTOSFreshness((localData.tos_url as string) || ''), [localData.tos_url])

  const nodeType = node.type || ''
  const appearanceCards = appearanceCardsForData(localData)
  const appearanceFormKeys = Object.keys(appearanceCards)

  return (
    <div className="w-72 border-l border-gray-800 bg-gray-900 flex flex-col h-full">
      <div className="px-4 py-3 border-b border-gray-800 flex items-center justify-between">
        <div>
          <h3 className="text-sm font-semibold text-gray-200">节点属性</h3>
          <p className="text-xs text-gray-500 mt-0.5">{(localData.label as string) || nodeType}</p>
        </div>
        <button onClick={onClose} className="text-gray-500 hover:text-gray-300 transition-colors">
          <X className="w-4 h-4" />
        </button>
      </div>

      <div className="flex-1 overflow-y-auto p-4 space-y-4">
        <Field label="名称">
          <input
            value={(localData.label as string) || ''}
            onChange={(e) => update('label', e.target.value)}
            className="input-dark"
          />
        </Field>

        {nodeType === 'llm' && (
          <>
            <Field label="模型">
              <select value={(localData.model as string) || ''} onChange={(e) => update('model', e.target.value)} className="input-dark">
                <option value="">选择模型</option>
                {models.map((m) => (
                  <option key={m.id} value={m.model_name}>{m.display_name || `${m.provider} / ${m.model_name}`}</option>
                ))}
              </select>
            </Field>
            <Field label="描述">
              <textarea value={(localData.description as string) || ''} onChange={(e) => update('description', e.target.value)}
                rows={3} className="input-dark resize-none" placeholder="节点功能说明" />
            </Field>
            <Field label="Prompt 模板">
              <textarea value={(localData.prompt as string) || ''} onChange={(e) => update('prompt', e.target.value)}
                rows={5} className="input-dark resize-none font-mono text-xs" placeholder="使用 {{input}} 引用上游输入" />
            </Field>
            <Field label="Temperature">
              <div className="flex items-center gap-2">
                <input type="range" min="0" max="2" step="0.1"
                  value={(localData.temperature as number) ?? 0.7}
                  onChange={(e) => update('temperature', parseFloat(e.target.value))}
                  className="flex-1 accent-blue-500" />
                <span className="text-xs text-gray-400 w-8 text-right">{(localData.temperature as number)?.toFixed(1) ?? '0.7'}</span>
              </div>
            </Field>
          </>
        )}

        {nodeType === 'tool' && (
          <>
            <Field label="工具">
              <select value={(localData.toolName as string) || ''} onChange={(e) => update('toolName', e.target.value)} className="input-dark">
                <option value="">选择工具</option>
                {tools.map((t) => (<option key={t} value={t}>{t}</option>))}
              </select>
            </Field>
            <Field label="描述">
              <textarea value={(localData.description as string) || ''} onChange={(e) => update('description', e.target.value)}
                rows={2} className="input-dark resize-none" placeholder="工具调用说明" />
            </Field>
            <Field label="参数模板 (JSON)">
              <textarea value={(localData.argsTemplate as string) || ''} onChange={(e) => update('argsTemplate', e.target.value)}
                rows={4} className="input-dark resize-none font-mono text-xs" placeholder='{"query": "{{input}}"}' />
            </Field>
          </>
        )}

        {nodeType === 'condition' && (
          <>
            <Field label="条件表达式">
              <textarea value={(localData.expression as string) || ''} onChange={(e) => update('expression', e.target.value)}
                rows={3} className="input-dark resize-none font-mono text-xs" placeholder='input.contains("error")' />
            </Field>
            <Field label="描述">
              <textarea value={(localData.description as string) || ''} onChange={(e) => update('description', e.target.value)}
                rows={2} className="input-dark resize-none" placeholder="分支逻辑说明" />
            </Field>
          </>
        )}

        {nodeType === 'media' && (
          <>
            <Field label="类型">
              <select value={(localData.category as string) || 'character'} onChange={(e) => update('category', e.target.value)} className="input-dark">
                {MEDIA_CATEGORIES.map((c) => (<option key={c.value} value={c.value}>{c.label}</option>))}
              </select>
            </Field>
            <Field label="本地图片 URL（相对路径）">
              <input value={(localData.imageUrl as string) || ''} onChange={(e) => update('imageUrl', e.target.value)}
                className="input-dark font-mono text-xs" placeholder="/v1/projects/xxx 或 /v1/uploads/xxx" />
              <UrlThumb url={(localData.imageUrl as string) || ''} badge="本地" tint="slate" onOpen={openLightbox} />
              {/* 角色节点：扫本地 docs/ 给候选三视图；点哪张就填哪张的 URL */}
              {localData.category === 'character' && (
                <LocalCandidateBar
                  project="swarm-universe"
                  entityKind="characters"
                  characterKey={(localData.key as string) || inferKeyFromImageUrl((localData.imageUrl as string) || '')}
                  characterLabel={(localData.label as string) || ''}
                  currentImageUrl={(localData.imageUrl as string) || ''}
                  onPick={pickCharacterImage}
                  onOpen={openLightbox}
                />
              )}
              <p className="text-[10px] text-gray-600 mt-1 leading-relaxed">容器内的真实路径，用于 laundering 输入 & 本地 fallback。</p>
            </Field>
            <Field label="CDN URL（cdn.starclaw.net）">
              <div className="flex gap-1.5">
                <input value={(localData.cdn_url as string) || ''} onChange={(e) => update('cdn_url', e.target.value)}
                  className="input-dark font-mono text-xs flex-1" placeholder="https://cdn.starclaw.net/..." />
                <button
                  onClick={syncImageToCDN}
                  disabled={cdnSyncing || !((localData.imageUrl as string) || '').trim()}
                  className="px-2.5 py-1.5 rounded-md bg-gradient-to-r from-indigo-600 to-sky-600 hover:from-indigo-500 hover:to-sky-500 disabled:opacity-50 disabled:cursor-not-allowed text-white text-xs font-medium flex items-center gap-1 shadow-md shadow-indigo-900/30 transition whitespace-nowrap"
                  title="把「本地图片 URL」读出来，SCP 推到 cdn.starclaw.net，回写到本框"
                >
                  {cdnSyncing ? <Loader2 className="w-3.5 h-3.5 animate-spin" /> : <Upload className="w-3.5 h-3.5" />}
                  {cdnSyncing ? '推送中' : ((localData.cdn_url as string) || '').trim() ? '重新同步' : '同步到 CDN'}
                </button>
              </div>
              <UrlThumb url={(localData.cdn_url as string) || ''} badge="CDN" tint="indigo" onOpen={openLightbox} />
              {cdnSyncMsg && (
                <p className={`text-[10px] mt-1 leading-relaxed ${cdnSyncMsg.tone === 'ok' ? 'text-emerald-400' : 'text-rose-400'}`}>
                  {cdnSyncMsg.text}
                </p>
              )}
              <p className="text-[10px] text-gray-600 mt-1 leading-relaxed">公网可访问，30d 稳定；给 Seedance 走 TOS 之前的退路。点「同步到 CDN」把本地图一键推上去。</p>
            </Field>
            <Field label="Volcengine TOS URL（Ark 信任域，bypass 隐私过滤）">
              <div className="flex gap-1.5">
                <input value={(localData.tos_url as string) || ''}
                  onChange={(e) => update('tos_url', e.target.value)}
                  className="input-dark font-mono text-xs flex-1"
                  placeholder="https://ark-acg-cn-beijing.tos-cn-beijing.volces.com/..." />
                <button
                  onClick={launderTOS}
                  disabled={launderingTOS}
                  className="px-2.5 py-1.5 rounded-md bg-gradient-to-r from-emerald-600 to-teal-600 hover:from-emerald-500 hover:to-teal-500 disabled:opacity-50 disabled:cursor-not-allowed text-white text-xs font-medium flex items-center gap-1 shadow-md shadow-emerald-900/30 transition whitespace-nowrap"
                  title="把本地图 / CDN 图过一遍 Seedream 5.0 lite，生成 bypass 隐私过滤的 Ark TOS URL（24h 有效）"
                >
                  {launderingTOS ? <Loader2 className="w-3.5 h-3.5 animate-spin" /> : <Sparkles className="w-3.5 h-3.5" />}
                  {launderingTOS ? '生成中' : '生成'}
                </button>
              </div>
              <UrlThumb url={(localData.tos_url as string) || ''} badge="TOS" tint="emerald" onOpen={openLightbox} />
              {tosFreshness.parsed && (() => {
                const lab = freshnessLabel(tosFreshness)
                const toneCls = lab.tone === 'ok'
                  ? 'text-emerald-400 bg-emerald-500/10 border-emerald-500/30'
                  : lab.tone === 'warn'
                    ? 'text-amber-400 bg-amber-500/10 border-amber-500/30'
                    : 'text-rose-400 bg-rose-500/10 border-rose-500/30'
                return (
                  <div className={`mt-2 px-2 py-1 rounded border text-[10px] font-medium flex items-center gap-1.5 ${toneCls}`}>
                    <span className={`inline-block w-1.5 h-1.5 rounded-full ${lab.tone === 'ok' ? 'bg-emerald-400' : lab.tone === 'warn' ? 'bg-amber-400 animate-pulse' : 'bg-rose-400 animate-pulse'}`} />
                    <span>{lab.text}</span>
                    {lab.tone !== 'ok' && (
                      <button type="button" onClick={launderTOS} disabled={launderingTOS} className="ml-auto underline underline-offset-2 hover:text-white disabled:opacity-50">
                        {launderingTOS ? '刷新中…' : '立即刷新'}
                      </button>
                    )}
                  </div>
                )
              })()}
              {launderErr && <p className="text-[10px] text-rose-400 mt-1 leading-relaxed">{launderErr}</p>}
              {launderOk && !launderErr && (
                <p className="text-[10px] text-emerald-400 mt-1 leading-relaxed font-medium">{launderOk}</p>
              )}
              {tosSensitiveBlock && (
                <div className="mt-2 px-2.5 py-2 rounded-md border border-amber-500/40 bg-amber-500/10 text-[10px] leading-relaxed">
                  <p className="text-amber-300 font-medium mb-1.5">Seedream 过滤了这张图（已重试 3 次，含风格化 prompt 变体）</p>
                  <p className="text-amber-200/80 mb-2">
                    通常是三视图里皮肤占比过高触发隐私过滤。可以直接把 <strong>CDN URL</strong> 填到 TOS 字段兜底 —— Seedance 对 cdn.starclaw.net 是白名单，EP04 已验证可用；只是没有 24h 过期那层保护。
                  </p>
                  <button
                    type="button"
                    onClick={useCdnAsTos}
                    disabled={!((localData.cdn_url as string) || '').trim()}
                    className="px-2.5 py-1 rounded bg-amber-600 hover:bg-amber-500 disabled:opacity-50 disabled:cursor-not-allowed text-white text-[11px] font-medium inline-flex items-center gap-1 transition"
                    title={((localData.cdn_url as string) || '').trim() ? '把 CDN URL 直接赋给 TOS 字段' : '先同步本地图到 CDN 才能用此兜底'}
                  >
                    <Link2 className="w-3 h-3" /> 用 CDN URL 代替 TOS
                  </button>
                  {!((localData.cdn_url as string) || '').trim() && (
                    <p className="text-amber-200/60 mt-1.5">↑ 按钮置灰是因为「CDN URL」还空着，先点上面的「同步到 CDN」把本地图推上去。</p>
                  )}
                </div>
              )}
              <p className="text-[10px] text-gray-600 mt-1 leading-relaxed">Seedance 生成视频时**优先**用 TOS URL（唯一能绕过隐私过滤的地址）。TOS 硬性 24h 过期（Volcengine 规则），EP 开跑前会自动预刷所有过期 URL。</p>
            </Field>
            <Field label="描述">
              <textarea value={(localData.description as string) || ''} onChange={(e) => update('description', e.target.value)}
                rows={3} className="input-dark resize-none" placeholder="角色/场景/道具描述" />
            </Field>
            {localData.category === 'prop' && onEditProp && (
              <button
                onClick={() => onEditProp(node.id)}
                className="w-full px-3 py-2 rounded-lg bg-gradient-to-r from-amber-600/80 to-orange-600/80 hover:from-amber-500 hover:to-orange-500 text-white text-xs font-medium flex items-center justify-center gap-1.5 shadow-md shadow-amber-900/30 transition"
                title="打开道具工坊：上传/AI 生成参考图 + 编辑信息"
              >
                <Wand2 className="w-3.5 h-3.5" /> 打开道具工坊（重新生成参考图）
              </button>
            )}
            {localData.category === 'character' && (
              <>
                {onEditCharacter && (
                  <button
                    onClick={() => onEditCharacter(node.id)}
                    className="w-full px-3 py-2 rounded-lg bg-gradient-to-r from-violet-600/80 to-cyan-600/80 hover:from-violet-500 hover:to-cyan-500 text-white text-xs font-medium flex items-center justify-center gap-1.5 shadow-md shadow-violet-900/30 transition"
                    title="打开 3 阶段向导：编辑信息 · 重新生成三视图 · 保存"
                  >
                    <Wand2 className="w-3.5 h-3.5" /> 打开角色工坊向导（重新生成三视图）
                  </button>
                )}
                <Field label="角色标签 (tag)">
                  <input value={(localData.tag as string) || ''} onChange={(e) => update('tag', e.target.value)}
                    className="input-dark font-mono text-xs" placeholder="[图1]" />
                </Field>
                <Field label="定位 (role)">
                  <input value={(localData.role as string) || ''} onChange={(e) => update('role', e.target.value)}
                    className="input-dark text-xs" placeholder="女一 / 男一 / 配角" />
                </Field>
                <Field label="外观卡 (appearance card)">
                  {appearanceFormKeys.length > 0 && (
                    <div className="mb-2 grid grid-cols-3 gap-1 rounded-lg border border-gray-800 bg-gray-950/50 p-1">
                      {appearanceFormKeys.map((key) => {
                        const active = ((localData.appearance_form as string) || appearanceFormKeys[0] || '') === key
                        return (
                          <button
                            key={key}
                            type="button"
                            onClick={() => switchAppearanceForm(key)}
                            className={`px-1.5 py-1 rounded-md text-[10px] font-medium transition truncate ${
                              active
                                ? 'bg-violet-600 text-white shadow shadow-violet-900/40'
                                : 'text-gray-400 hover:text-cyan-200 hover:bg-gray-800/80'
                            }`}
                            title={appearanceFormLabel(key)}
                          >
                            {appearanceFormLabel(key)}
                          </button>
                        )
                      })}
                    </div>
                  )}
                  <textarea value={(localData.appearance_card as string) || ''} onChange={(e) => update('appearance_card', e.target.value)}
                    rows={5} className="input-dark resize-none text-xs leading-relaxed" placeholder="给 AI 看图生视频用的一句话外观描述，注入到场景 prompt 的 [图N] 占位符" />
                </Field>
              </>
            )}
          </>
        )}

        {nodeType === 'start' && (
          <Field label="输入变量">
            <textarea value={(localData.inputVars as string) || ''} onChange={(e) => update('inputVars', e.target.value)}
              rows={3} className="input-dark resize-none font-mono text-xs" placeholder="user_message" />
          </Field>
        )}

        {nodeType === 'end' && (
          <Field label="输出映射">
            <textarea value={(localData.outputMapping as string) || ''} onChange={(e) => update('outputMapping', e.target.value)}
              rows={3} className="input-dark resize-none font-mono text-xs" placeholder="{{last_node_output}}" />
          </Field>
        )}

        <div className="pt-2 border-t border-gray-800">
          <p className="text-xs text-gray-600">
            ID: <code className="bg-gray-800 px-1.5 py-0.5 rounded text-gray-400">{node.id}</code>
          </p>
        </div>
      </div>

      {lightboxURL && (
        <div
          className="fixed inset-0 z-50 bg-black/90 backdrop-blur-sm flex items-center justify-center p-6 cursor-zoom-out"
          onClick={() => setLightboxURL('')}
          role="dialog" aria-modal="true"
        >
          <div className="relative max-w-[95vw] max-h-[95vh]">
            <img
              src={lightboxURL}
              alt={lightboxLabel}
              className="max-w-[95vw] max-h-[88vh] object-contain rounded-lg shadow-2xl shadow-black"
              onClick={(e) => e.stopPropagation()}
              onError={(e) => { (e.target as HTMLImageElement).style.opacity = '0.3' }}
            />
            <div className="mt-2 px-2 text-center">
              <p className="text-xs font-medium text-gray-300">{lightboxLabel}</p>
              <p className="text-[10px] font-mono text-gray-500 break-all mt-0.5 max-w-[80ch] mx-auto">{lightboxURL}</p>
              <p className="text-[10px] text-gray-600 mt-1">点黑色区域或按 ESC 关闭</p>
            </div>
            <button
              onClick={(e) => { e.stopPropagation(); setLightboxURL('') }}
              className="absolute -top-3 -right-3 w-8 h-8 rounded-full bg-gray-900 border border-gray-700 text-gray-300 hover:text-white hover:bg-gray-800 transition flex items-center justify-center shadow-lg"
              title="关闭 (Esc)"
            >
              <X className="w-4 h-4" />
            </button>
          </div>
        </div>
      )}
    </div>
  )
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div>
      <label className="block text-[11px] font-medium text-gray-400 mb-1.5 uppercase tracking-wider">{label}</label>
      {children}
    </div>
  )
}

// ── LocalCandidateBar ─────────────────────────────────
// 扫 docs/<project>/ 找「这个角色的所有参考图候选」→ 排成一条可横滑缩略图条，带 tab。
// 点哪张 → 填 imageUrl 为那张的完整 /v1/projects/... URL（不写回 manifest）。
// 当前 imageUrl 命中的候选会高亮一圈，便于一眼确认 manifest 当前指向。
interface LocalCandidate { path: string; url: string; size: number; score: number; reason: string; kind?: string }

type CandidateKind = 'all' | 'sheets' | 'variants' | 'nano' | 'raw' | 'legacy'

const KIND_LABELS: Record<CandidateKind, string> = {
  all: '全部',
  sheets: '定稿',
  variants: '变体',
  nano: 'nano',
  raw: '原始',
  legacy: '旧',
}

function LocalCandidateBar({ project, entityKind, characterKey, characterLabel, currentImageUrl, onPick, onOpen }: {
  project: string
  entityKind: 'characters' | 'props' | 'scenes'
  characterKey: string
  characterLabel: string
  currentImageUrl: string
  onPick: (url: string) => void
  onOpen: (url: string, label: string) => void
}) {
  const [candidates, setCandidates] = useState<LocalCandidate[] | null>(null)
  const [loading, setLoading] = useState(false)
  const [err, setErr] = useState<string>('')
  const [tab, setTab] = useState<CandidateKind>('all')

  // nano 润色 + 设为 sheet 的状态
  const [nanoOpen, setNanoOpen] = useState(false)
  const [nanoPrompt, setNanoPrompt] = useState('')
  const [nanoUseEdit, setNanoUseEdit] = useState(true) // 默认基于当前图编辑；空时自动退到纯文本
  const [nanoLoading, setNanoLoading] = useState(false)
  const [nanoMsg, setNanoMsg] = useState<{ tone: 'ok'|'err'; text: string } | null>(null)
  const [promoteLoading, setPromoteLoading] = useState(false)
  const [promoteMsg, setPromoteMsg] = useState<{ tone: 'ok'|'err'; text: string } | null>(null)

  // 右键菜单：右键任一候选缩略图弹浮窗，支持「复制路径 / 新窗口打开 / 删除」。
  // 删除只对 /entities/* 下生效（后端也有同样的保护）；manifest 当前指向的那张
  // 会被后端 409 拒绝，UI 把原因直接显示给用户。
  const [ctxMenu, setCtxMenu] = useState<{ x: number; y: number; cand: LocalCandidate } | null>(null)
  const [deletingPath, setDeletingPath] = useState<string>('')
  const [deleteMsg, setDeleteMsg] = useState<{ tone: 'ok'|'err'; text: string } | null>(null)

  // 至少有 key 或 label 才能扫
  const canScan = !!(characterKey || characterLabel)

  const fetchCandidates = useCallback(async () => {
    if (!canScan) return
    setLoading(true)
    setErr('')
    try {
      const res = await projectAPI.suggestRef(project, {
        character_key: characterKey || undefined,
        character_label: characterLabel || undefined,
        limit: 24,
      })
      setCandidates((res.data?.candidates || []) as LocalCandidate[])
    } catch (e) {
      const ax = e as { response?: { data?: { error?: string } }; message?: string }
      setErr(ax?.response?.data?.error || ax?.message || '扫描失败')
      setCandidates([])
    } finally {
      setLoading(false)
    }
  }, [project, characterKey, characterLabel, canScan])

  // 切换角色时自动扫一次
  useEffect(() => {
    if (!canScan) { setCandidates(null); return }
    void fetchCandidates()
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [characterKey, characterLabel])

  // Tab 过滤 & 每 tab 的计数，供 tabbar 实时反馈
  const filtered = useMemo(() => {
    if (!candidates) return []
    if (tab === 'all') return candidates
    return candidates.filter(c => (c.kind || 'other') === tab)
  }, [candidates, tab])

  const counts = useMemo(() => {
    const out: Record<CandidateKind, number> = { all: 0, sheets: 0, variants: 0, nano: 0, raw: 0, legacy: 0 }
    if (!candidates) return out
    out.all = candidates.length
    for (const c of candidates) {
      const k = (c.kind || 'other') as CandidateKind
      if (k in out) out[k]++
    }
    return out
  }, [candidates])

  if (!canScan) {
    return (
      <div className="mt-2 px-2 py-1.5 rounded border border-gray-800 bg-gray-900/40 text-[10px] text-gray-500">
        <ImageIcon className="w-3 h-3 inline mr-1" />
        先填写角色 <code className="font-mono text-gray-400">key</code> 或 <code className="font-mono text-gray-400">label</code> 才能扫本地候选三视图。
      </div>
    )
  }

  return (
    <div className="mt-2">
      <div className="flex items-center gap-2 mb-1">
        <div className="text-[10px] text-gray-500 flex-1">
          本地候选 {candidates ? `(${filtered.length}/${candidates.length})` : ''} <span className="text-gray-700">· 点击填充 URL · 双击看大图</span>
        </div>
        <button type="button" onClick={fetchCandidates} disabled={loading}
          className="text-[10px] px-1.5 py-0.5 rounded border border-gray-800 hover:border-cyan-700 text-gray-400 hover:text-cyan-300 disabled:opacity-50 flex items-center gap-1">
          {loading ? <Loader2 className="w-3 h-3 animate-spin" /> : <RefreshCw className="w-3 h-3" />}
          {loading ? '扫描中' : '重扫'}
        </button>
      </div>

      {/* 分类 tab — 直观看懂 sheets / variants / nano / raw / legacy 的分布 */}
      {candidates && candidates.length > 0 && (
        <div className="flex gap-1 mb-1.5 text-[10px]">
          {(['all','sheets','variants','nano','raw','legacy'] as CandidateKind[]).map(k => {
            const n = counts[k]
            if (k !== 'all' && n === 0) return null
            const active = tab === k
            return (
              <button key={k} type="button" onClick={() => setTab(k)}
                className={`px-1.5 py-0.5 rounded transition ${
                  active
                    ? 'bg-cyan-600/80 text-white'
                    : 'border border-gray-800 text-gray-500 hover:text-cyan-300 hover:border-cyan-700'
                }`}>
                {KIND_LABELS[k]} {n > 0 && <span className="opacity-60">({n})</span>}
              </button>
            )
          })}
        </div>
      )}

      {err && (
        <div className="text-[10px] px-2 py-1 rounded border border-rose-900/40 bg-rose-950/20 text-rose-300">{err}</div>
      )}

      {candidates && candidates.length === 0 && !loading && !err && (
        <div className="text-[10px] px-2 py-1.5 rounded border border-dashed border-gray-800 text-gray-600">
          没扫到 <code className="font-mono text-gray-500">{characterKey || characterLabel}</code> 的候选图 —— 检查 docs/{project}/entities/characters/{characterKey || '<key>'}/ 下是否放了图片。
        </div>
      )}

      {filtered.length > 0 && (
        <div className="flex gap-1.5 overflow-x-auto pb-1.5 pr-1 snap-x snap-mandatory" style={{ scrollbarWidth: 'thin' }}>
          {filtered.map(c => {
            const active = currentImageUrl === c.url || currentImageUrl.endsWith(c.path)
            const kindDot = kindDotColor(c.kind)
            const deleting = deletingPath === c.path
            return (
              <div key={c.path} className="flex-none snap-start">
                <button
                  type="button"
                  onClick={() => onPick(c.url)}
                  onDoubleClick={() => onOpen(c.url, `${characterLabel || characterKey}（${c.path}）`)}
                  onContextMenu={(e) => {
                    // 先自己处理右键，浏览器菜单就不弹了。
                    e.preventDefault()
                    e.stopPropagation()
                    setDeleteMsg(null)
                    setCtxMenu({ x: e.clientX, y: e.clientY, cand: c })
                  }}
                  disabled={deleting}
                  title={`${c.path}\n${c.reason} · ${Math.round(c.size / 1024)} KB · score ${c.score} · ${c.kind || 'other'}\n点: 作为本地 ref  双击: 看大图  右键: 更多`}
                  className={`relative w-16 h-20 rounded overflow-hidden border transition ${
                    active
                      ? 'border-emerald-400 ring-1 ring-emerald-500/60'
                      : 'border-gray-800 hover:border-cyan-600'
                  } ${deleting ? 'opacity-40' : ''}`}>
                  <img src={c.url} alt="" loading="lazy" className="w-full h-full object-cover" />
                  {active && (
                    <span className="absolute top-0.5 left-0.5 px-1 py-0.5 rounded bg-emerald-500/90 text-white text-[8px] font-semibold leading-none">本地</span>
                  )}
                  {c.kind && (
                    <span className={`absolute top-0.5 right-0.5 w-1.5 h-1.5 rounded-full ${kindDot}`} title={c.kind} />
                  )}
                  <span className="absolute bottom-0 left-0 right-0 px-1 py-0.5 bg-black/70 text-[8px] font-mono text-gray-300 text-center truncate">
                    {shortPathLabel(c.path)}
                  </span>
                  {deleting && (
                    <span className="absolute inset-0 flex items-center justify-center bg-black/60">
                      <Loader2 className="w-4 h-4 animate-spin text-rose-300" />
                    </span>
                  )}
                </button>
              </div>
            )
          })}
        </div>
      )}

      {deleteMsg && (
        <div className={`mt-1 text-[10px] px-2 py-1 rounded border ${
          deleteMsg.tone === 'ok'
            ? 'border-emerald-900/50 bg-emerald-950/30 text-emerald-300'
            : 'border-rose-900/50 bg-rose-950/30 text-rose-300'
        }`}>{deleteMsg.text}</div>
      )}

      {ctxMenu && (
        <CandidateContextMenu
          x={ctxMenu.x}
          y={ctxMenu.y}
          cand={ctxMenu.cand}
          currentImageUrl={currentImageUrl}
          onClose={() => setCtxMenu(null)}
          onCopyPath={() => {
            navigator.clipboard?.writeText(ctxMenu.cand.path).catch(() => {})
            setDeleteMsg({ tone: 'ok', text: `已复制路径：${ctxMenu.cand.path}` })
            setCtxMenu(null)
          }}
          onOpenNewTab={() => {
            window.open(ctxMenu.cand.url, '_blank', 'noopener,noreferrer')
            setCtxMenu(null)
          }}
          onDelete={async () => {
            const cand = ctxMenu.cand
            setCtxMenu(null)
            // 二次确认 —— 删除是破坏性操作，避免误触。
            const ok = window.confirm(
              `确定删除这张候选图？\n\n` +
              `路径：${cand.path}\n大小：${Math.round(cand.size / 1024)} KB\n分类：${cand.kind || 'other'}\n\n` +
              `注意：\n• 只能删 /entities/* 下的文件（旧 /production 和 /assets 受保护）\n` +
              `• 如果这张正在被 manifest.json 作为 ref 引用，后端会拒绝`,
            )
            if (!ok) return
            setDeletingPath(cand.path)
            setDeleteMsg(null)
            try {
              const res = await projectAPI.deleteRef(project, cand.path)
              const d = res.data as { deleted?: string; size_bytes?: number; note?: string }
              setDeleteMsg({ tone: 'ok', text: d.note || `已删 ${d.deleted}` })
              // 删完重扫 —— 本地 state 自动去掉那一张
              await fetchCandidates()
              // 如果刚删的正好是当前 imageUrl，清掉避免指向 404
              if (currentImageUrl === cand.url || currentImageUrl.endsWith(cand.path)) {
                onPick('')
              }
            } catch (e) {
              const ax = e as { response?: { status?: number; data?: { error?: string; hint?: string } }; message?: string }
              const msg = ax?.response?.data?.error || ax?.message || '删除失败'
              const hint = ax?.response?.data?.hint
              setDeleteMsg({ tone: 'err', text: hint ? `${msg} · ${hint}` : msg })
            } finally {
              setDeletingPath('')
            }
          }}
        />
      )}

      {/* 动作按钮：🍌 nano 润色 / 📌 设为 sheet */}
      {characterKey && (
        <div className="mt-1.5 flex items-center gap-1.5">
          <button type="button"
            onClick={() => { setNanoOpen(v => !v); setNanoMsg(null) }}
            disabled={nanoLoading}
            className="flex-1 text-[10px] px-2 py-1 rounded border border-fuchsia-900/50 bg-fuchsia-950/30 text-fuchsia-300 hover:bg-fuchsia-900/40 hover:text-fuchsia-200 disabled:opacity-50 flex items-center justify-center gap-1 transition"
            title="StarAI→fal.ai/nano-banana-2 生成或编辑；产物自动回写 entities/.../nano/">
            <Banana className="w-3 h-3" />
            {nanoOpen ? '收起 nano' : 'nano 润色'}
          </button>
          <button type="button"
            onClick={async () => {
              setPromoteMsg(null)
              if (!currentImageUrl) {
                setPromoteMsg({ tone: 'err', text: '先填或点一张候选图' }); return
              }
              const srcPath = stripProjectPrefix(currentImageUrl, project)
              if (!srcPath) {
                setPromoteMsg({ tone: 'err', text: 'imageUrl 不在 /v1/projects/<project>/ 下，无法晋级' }); return
              }
              if (srcPath.includes('/sheets/')) {
                setPromoteMsg({ tone: 'err', text: '当前已经是 sheets/，无需晋级' }); return
              }
              setPromoteLoading(true)
              try {
                const res = await projectAPI.promoteToSheet(project, entityKind, characterKey, { source_path: srcPath })
                const d = res.data as { new_url?: string; version?: number; note?: string }
                if (d.new_url) onPick(d.new_url)
                setPromoteMsg({ tone: 'ok', text: d.note || `已晋级为 sheet v${d.version}` })
                await fetchCandidates()
              } catch (e) {
                const ax = e as { response?: { data?: { error?: string } }; message?: string }
                setPromoteMsg({ tone: 'err', text: ax?.response?.data?.error || ax?.message || '晋级失败' })
              } finally {
                setPromoteLoading(false)
              }
            }}
            disabled={promoteLoading || !currentImageUrl}
            className="flex-1 text-[10px] px-2 py-1 rounded border border-emerald-900/50 bg-emerald-950/30 text-emerald-300 hover:bg-emerald-900/40 hover:text-emerald-200 disabled:opacity-50 flex items-center justify-center gap-1 transition"
            title="把当前选中的图复制为 entities/.../sheets/unified_sheet_v<N+1>.png 并 patch manifest.ref">
            {promoteLoading ? <Loader2 className="w-3 h-3 animate-spin" /> : <Pin className="w-3 h-3" />}
            设为 sheet
          </button>
        </div>
      )}

      {/* 晋级反馈 */}
      {promoteMsg && (
        <div className={`mt-1 text-[10px] px-2 py-1 rounded border ${
          promoteMsg.tone === 'ok'
            ? 'border-emerald-900/50 bg-emerald-950/30 text-emerald-300'
            : 'border-rose-900/50 bg-rose-950/30 text-rose-300'
        }`}>{promoteMsg.text}</div>
      )}

      {/* nano 润色内嵌面板 */}
      {nanoOpen && characterKey && (
        <div className="mt-1.5 p-2 rounded border border-fuchsia-900/40 bg-fuchsia-950/20 space-y-1.5">
          <div className="text-[10px] text-fuchsia-300 font-medium flex items-center gap-1">
            <Banana className="w-3 h-3" /> nano-banana 润色 / 生成
          </div>
          <textarea value={nanoPrompt} onChange={e => setNanoPrompt(e.target.value)}
            placeholder="描述你想要的变化，例如：让她微笑，颜色更暖一点，加一缕薄雾"
            rows={2}
            className="w-full input-dark text-xs resize-none" />
          <label className="flex items-center gap-1.5 text-[10px] text-fuchsia-200 cursor-pointer select-none">
            <input type="checkbox" checked={nanoUseEdit && !!currentImageUrl} disabled={!currentImageUrl}
              onChange={e => setNanoUseEdit(e.target.checked)}
              className="accent-fuchsia-500" />
            基于当前图编辑（nano-banana-2/edit） {currentImageUrl ? '' : '— 先选一张图'}
          </label>
          <div className="flex items-center gap-1.5">
            <button type="button"
              onClick={async () => {
                setNanoMsg(null)
                if (!nanoPrompt.trim()) { setNanoMsg({ tone: 'err', text: '先写 prompt' }); return }
                setNanoLoading(true)
                try {
                  const body = {
                    project,
                    entity_kind: entityKind,
                    entity_key: characterKey,
                    prompt: nanoPrompt.trim(),
                    source_url: (nanoUseEdit && currentImageUrl) ? currentImageUrl : undefined,
                    size: 'portrait_4_3',
                  }
                  const res = await nanoAPI.generate(body)
                  const d = res.data as { local_url?: string; request_id?: string }
                  setNanoMsg({ tone: 'ok', text: `生成成功 (req ${d.request_id?.slice(0, 8) || ''}) — 已回写 nano/` })
                  if (d.local_url) onPick(d.local_url)
                  await fetchCandidates()
                  setTab('nano')
                } catch (e) {
                  const ax = e as { response?: { data?: { error?: string; detail?: string } }; message?: string }
                  setNanoMsg({ tone: 'err', text: ax?.response?.data?.error || ax?.response?.data?.detail || ax?.message || '生成失败' })
                } finally {
                  setNanoLoading(false)
                }
              }}
              disabled={nanoLoading || !nanoPrompt.trim()}
              className="flex-1 text-[10px] px-2 py-1 rounded bg-fuchsia-600/80 hover:bg-fuchsia-500 text-white disabled:opacity-50 flex items-center justify-center gap-1">
              {nanoLoading ? <Loader2 className="w-3 h-3 animate-spin" /> : <Sparkles className="w-3 h-3" />}
              {nanoLoading ? '生成中 ~30s' : '生成'}
            </button>
            <button type="button" onClick={() => setNanoOpen(false)} disabled={nanoLoading}
              className="text-[10px] px-2 py-1 rounded border border-gray-800 text-gray-400 hover:text-white disabled:opacity-50">
              取消
            </button>
          </div>
          {nanoMsg && (
            <div className={`text-[10px] px-1.5 py-1 rounded ${
              nanoMsg.tone === 'ok'
                ? 'text-emerald-300 bg-emerald-950/30 border border-emerald-900/50'
                : 'text-rose-300 bg-rose-950/30 border border-rose-900/50'
            }`}>{nanoMsg.text}</div>
          )}
        </div>
      )}
    </div>
  )
}

// CandidateContextMenu ── 候选图缩略图右键菜单
// 绝对定位浮窗：复制路径 / 新窗口打开 / 删除（危险操作，红色）
// 点外部或按 ESC 自动关。如果弹出位置靠近右下角会自动反向翻转。
function CandidateContextMenu({
  x, y, cand, currentImageUrl, onClose, onCopyPath, onOpenNewTab, onDelete,
}: {
  x: number
  y: number
  cand: LocalCandidate
  currentImageUrl: string
  onClose: () => void
  onCopyPath: () => void
  onOpenNewTab: () => void
  onDelete: () => void
}) {
  // 下拉位置：如果靠近右/下边缘则向上或向左偏移，避免切掉。
  // 菜单体积约 180x150px。
  const W = 200
  const H = 130
  const vw = typeof window !== 'undefined' ? window.innerWidth : 1920
  const vh = typeof window !== 'undefined' ? window.innerHeight : 1080
  const left = Math.min(x, vw - W - 8)
  const top = Math.min(y, vh - H - 8)

  // 删除是否会被后端拒绝（已在 manifest 引用 或 不在 /entities/*）。
  // UI 预先灰掉按钮，省得用户点一下等后端返 403/409 再猜。
  const notUnderEntities = !cand.path.startsWith('/entities/')
  const isCurrentRef = currentImageUrl === cand.url || currentImageUrl.endsWith(cand.path)
  const deleteBlocked = notUnderEntities || isCurrentRef
  const deleteBlockReason = notUnderEntities
    ? '此文件不在 /entities/* 下（受保护）'
    : isCurrentRef
      ? '这张是 manifest.ref 当前指向，先 promote 另一张再删'
      : ''

  // 点外部或 ESC 关菜单
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => { if (e.key === 'Escape') onClose() }
    const onDown = (e: MouseEvent) => {
      // onClose 已在每个 action 里调了，这里兜底外部点击
      const t = e.target as HTMLElement | null
      if (t && t.closest('[data-candidate-ctx-menu]')) return
      onClose()
    }
    window.addEventListener('keydown', onKey)
    // 延迟一帧绑 mousedown，避免吃掉当前的右键事件
    const id = setTimeout(() => window.addEventListener('mousedown', onDown), 0)
    return () => {
      window.removeEventListener('keydown', onKey)
      window.removeEventListener('mousedown', onDown)
      clearTimeout(id)
    }
  }, [onClose])

  return (
    <div
      data-candidate-ctx-menu
      className="fixed z-50 w-[200px] rounded-md border border-gray-700 bg-gray-900 shadow-xl shadow-black/60 py-1 text-[11px]"
      style={{ left, top }}
      onContextMenu={(e) => e.preventDefault()}
    >
      <div className="px-2 py-1 border-b border-gray-800 text-[10px] text-gray-500 font-mono truncate" title={cand.path}>
        {cand.path.split('/').slice(-2).join('/')}
      </div>
      <button type="button" onClick={onCopyPath}
        className="w-full px-2 py-1.5 flex items-center gap-2 text-gray-300 hover:bg-gray-800 hover:text-white transition">
        <Copy className="w-3 h-3" /> 复制路径
      </button>
      <button type="button" onClick={onOpenNewTab}
        className="w-full px-2 py-1.5 flex items-center gap-2 text-gray-300 hover:bg-gray-800 hover:text-white transition">
        <ExternalLink className="w-3 h-3" /> 在新窗口打开
      </button>
      <div className="h-px bg-gray-800 my-0.5" />
      <button type="button"
        onClick={deleteBlocked ? undefined : onDelete}
        disabled={deleteBlocked}
        title={deleteBlocked ? deleteBlockReason : `从磁盘删除这张候选图（${Math.round(cand.size / 1024)} KB）`}
        className={`w-full px-2 py-1.5 flex items-center gap-2 transition ${
          deleteBlocked
            ? 'text-gray-600 cursor-not-allowed'
            : 'text-rose-400 hover:bg-rose-950/40 hover:text-rose-200'
        }`}>
        <Trash2 className="w-3 h-3" /> 删除
        {deleteBlocked && <span className="ml-auto text-[9px] text-gray-600">受保护</span>}
      </button>
      {deleteBlocked && (
        <div className="px-2 pb-1.5 text-[9.5px] text-gray-600 leading-tight">{deleteBlockReason}</div>
      )}
    </div>
  )
}

// stripProjectPrefix converts "/v1/projects/<project>/entities/.../x.png" to
// "/entities/.../x.png" so promote/suggest endpoints can consume it. Returns
// "" if the URL doesn't live under the expected project prefix.
function stripProjectPrefix(url: string, project: string): string {
  if (!url) return ''
  const prefix = `/v1/projects/${project}`
  if (!url.startsWith(prefix)) return ''
  const tail = url.slice(prefix.length)
  return tail.startsWith('/') ? tail : '/' + tail
}

// 从 /entities/characters/lin_jianyue/sheets/unified_sheet_v6.png 里提取显示友好的短标签
function shortPathLabel(path: string): string {
  const base = path.split('/').pop() || path
  const noExt = base.replace(/\.(png|jpg|jpeg|webp)$/i, '')
  return noExt.length > 13 ? noExt.slice(0, 12) + '…' : noExt
}

// 候选图右上角的小色点，对应 kind：一眼区分 sheets/variants/nano/raw/legacy
function kindDotColor(kind?: string): string {
  switch (kind) {
    case 'sheets': return 'bg-emerald-400'
    case 'variants': return 'bg-cyan-400'
    case 'nano': return 'bg-fuchsia-400'
    case 'raw': return 'bg-amber-400'
    case 'legacy': return 'bg-gray-500'
    default: return 'bg-gray-600'
  }
}

// inferKeyFromImageUrl 从 imageUrl 路径里抓出 manifest key。
//   /v1/projects/<p>/entities/characters/<KEY>/sheets/.. → KEY
//   /v1/projects/<p>/production/characters/<KEY>/..      → KEY
//   /v1/projects/<p>/assets/characters/<KEY>/..          → KEY
//   /v1/projects/<p>/entities/props/<KEY>/..             → KEY
// 用于老 workflow 快照里 data.key 缺失、前端又要调 suggest 的兜底。
function inferKeyFromImageUrl(url: string): string {
  if (!url) return ''
  // Match each family in priority order: entities > legacy production > legacy assets
  const patterns = [
    /\/entities\/(?:characters|props|scenes)\/([^/]+)\//,
    /\/production\/(?:characters|props|scenes)\/([^/]+)\//,
    /\/assets\/(?:characters|props|scenes)\/([^/]+)\//,
  ]
  for (const re of patterns) {
    const m = url.match(re)
    if (m && m[1]) return m[1]
  }
  return ''
}

// 小缩略图：满宽 64px 高，点击开大图。url 为空时渲染占位框。
function UrlThumb({ url, badge, tint, onOpen }: {
  url: string
  badge: string
  tint: 'slate' | 'indigo' | 'emerald'
  onOpen: (u: string, label: string) => void
}) {
  const tintRing: Record<'slate' | 'indigo' | 'emerald', string> = {
    slate: 'ring-slate-600/60 bg-slate-500/80',
    indigo: 'ring-indigo-600/60 bg-indigo-500/80',
    emerald: 'ring-emerald-600/60 bg-emerald-500/80',
  }
  if (!url) {
    return (
      <div className="mt-2 h-16 rounded-md border border-dashed border-gray-800 bg-gray-900/40 flex items-center justify-center">
        <span className="text-[10px] text-gray-600">无 {badge} 图</span>
      </div>
    )
  }
  return (
    <button
      type="button"
      onClick={() => onOpen(url, badge)}
      className={`mt-2 w-full h-16 rounded-md overflow-hidden relative ring-1 ${tintRing[tint].split(' ')[0]} hover:ring-2 transition cursor-zoom-in group`}
      title={`点击查看 ${badge} 大图`}
    >
      {/* key={url} 强制 img 在 URL 变化时 unmount+remount —— 解决用户反馈的"点生成后缩略图
          没刷新"：React 会重用同一个 <img> 元素，部分浏览器不会立刻重新 fetch 新 URL 的
          图像。给 key 绑死后每次 URL 换（新 TOS 签名 / 新 CDN 上传）都会走清空重载路径。 */}
      <img
        key={url}
        src={url}
        alt={badge}
        className="w-full h-full object-cover group-hover:scale-105 transition-transform duration-200"
        onError={(e) => {
          const t = e.target as HTMLImageElement
          t.style.display = 'none'
          const parent = t.parentElement
          if (parent && !parent.querySelector('[data-failfallback]')) {
            const span = document.createElement('span')
            span.setAttribute('data-failfallback', '1')
            span.className = 'absolute inset-0 flex items-center justify-center text-[10px] text-rose-400 bg-rose-950/20'
            span.textContent = `${badge} 图访问失败 (地址可能过期或错误)`
            parent.appendChild(span)
          }
        }}
      />
      <span className={`absolute top-1 left-1 px-1.5 py-0.5 rounded text-white text-[9px] font-semibold shadow ${tintRing[tint].split(' ')[1]}`}>
        {badge}
      </span>
    </button>
  )
}
