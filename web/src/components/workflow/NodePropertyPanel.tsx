import { useState, useEffect, useMemo } from 'react'
import { X, Wand2, Sparkles, Loader2 } from 'lucide-react'
import type { Node } from '@xyflow/react'
import { parseTOSFreshness, freshnessLabel, refreshTOS } from './tosUrlUtils'

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

  // 生成 / 刷新 TOS URL。首选 resign（HMAC 7d，零成本）；失败 fallback 到 Seedream launder（24h）
  const launderTOS = async () => {
    const oldTOSUrl = (localData.tos_url as string) || ''
    const fallbackSrc = ((localData.cdn_url as string) || (localData.imageUrl as string) || '').trim()
    if (!oldTOSUrl && !fallbackSrc) {
      setLaunderErr('先填「本地图片 URL」或「CDN URL」作为 laundering 源')
      return
    }
    setLaunderErr('')
    setLaunderingTOS(true)
    try {
      const r = await refreshTOS(oldTOSUrl, fallbackSrc)
      update('tos_url', r.tosUrl)
      // 可以在 console 看到走了哪条路径
      console.log(`[NodePropertyPanel] tos_url refreshed via ${r.source}${r.expiresSec ? ` (${r.expiresSec}s)` : ''}`)
    } catch (e) {
      const err = e as { response?: { data?: { error?: string; detail?: string } }; message?: string }
      setLaunderErr(err.response?.data?.error || err.response?.data?.detail || err.message || '生成 TOS URL 失败')
    } finally {
      setLaunderingTOS(false)
    }
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
              <p className="text-[10px] text-gray-600 mt-1 leading-relaxed">容器内的真实路径，用于 laundering 输入 & 本地 fallback。</p>
            </Field>
            <Field label="CDN URL（cdn.starclaw.net）">
              <input value={(localData.cdn_url as string) || ''} onChange={(e) => update('cdn_url', e.target.value)}
                className="input-dark font-mono text-xs" placeholder="https://cdn.starclaw.net/..." />
              <UrlThumb url={(localData.cdn_url as string) || ''} badge="CDN" tint="indigo" onOpen={openLightbox} />
              <p className="text-[10px] text-gray-600 mt-1 leading-relaxed">公网可访问，30d 稳定；给 Seedance 走 TOS 之前的退路。</p>
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
