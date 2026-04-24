import { useState, useEffect, useMemo, useCallback } from 'react'
import { X, Wand2, Sparkles, Loader2, ImageIcon, RefreshCw, Pin, Banana, Upload, Link2 } from 'lucide-react'
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

  // launder 过程中若被 Seedream 敏感检测挡下（HTTP 422 + reason=seedream_sensitive_content），
  // 面板会显示「📎 用 CDN URL 代替」按钮让用户一键兜底。
  const [tosSensitiveBlock, setTosSensitiveBlock] = useState(false)

  // 生成 / 刷新 TOS URL。首选 resign（HMAC 7d，零成本）→ promote → Seedream launder（24h）
  const launderTOS = async () => {
    const oldTOSUrl = (localData.tos_url as string) || ''
    const fallbackSrc = ((localData.cdn_url as string) || (localData.imageUrl as string) || '').trim()
    if (!oldTOSUrl && !fallbackSrc) {
      setLaunderErr('先填「本地图片 URL」或「CDN URL」作为 laundering 源')
      return
    }
    setLaunderErr('')
    setTosSensitiveBlock(false)
    setLaunderingTOS(true)
    try {
      const r = await refreshTOS(oldTOSUrl, fallbackSrc)
      update('tos_url', r.tosUrl)
      console.log(`[NodePropertyPanel] tos_url refreshed via ${r.source}${r.expiresSec ? ` (${r.expiresSec}s)` : ''}`)
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
  const useCdnAsTos = () => {
    const cdn = ((localData.cdn_url as string) || '').trim()
    if (!cdn) {
      setLaunderErr('还没有 CDN URL —— 先点「🔼 同步到 CDN」把本地图推上去')
      return
    }
    update('tos_url', cdn)
    setTosSensitiveBlock(false)
    setLaunderErr('')
  }

  // TOS URL 新鲜度（每 30s 重算一次以驱动 label 更新，不做高频 setInterval 避免浪费）
  const tosFreshness = useMemo(() => parseTOSFreshness((localData.tos_url as string) || ''), [localData.tos_url])

  const nodeType = node.type || ''

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
                  onPick={(url) => update('imageUrl', url)}
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
                  {cdnSyncing ? '推送中' : '同步到 CDN'}
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
            return (
              <div key={c.path} className="flex-none snap-start">
                <button
                  type="button"
                  onClick={() => onPick(c.url)}
                  onDoubleClick={() => onOpen(c.url, `${characterLabel || characterKey}（${c.path}）`)}
                  title={`${c.path}\n${c.reason} · ${Math.round(c.size / 1024)} KB · score ${c.score} · ${c.kind || 'other'}\n点: 作为本地 ref  双击: 看大图`}
                  className={`relative w-16 h-20 rounded overflow-hidden border transition ${
                    active
                      ? 'border-emerald-400 ring-1 ring-emerald-500/60'
                      : 'border-gray-800 hover:border-cyan-600'
                  }`}>
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
                </button>
              </div>
            )
          })}
        </div>
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
      <img
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
