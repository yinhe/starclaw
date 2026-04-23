// ── 成本分析 Modal ──
// 扫描画布上所有剧集节点，按 takes 累积计算 Seedance 实际费用，
// 加上可配置的 LLM / 图片 / BGM 估算，给出 EP 级成本表 + 构成 + 洞察卡。
import { useMemo, useState } from 'react'
import { X, Coins, AlertTriangle, Lightbulb, TrendingDown, Target } from 'lucide-react'
import type { Node } from '@xyflow/react'
import type { EpisodeData } from './episodeTypes'

interface Props {
  open: boolean
  onClose: () => void
  nodes: Node[]
}

// 默认费率（可在 UI 上编辑）
// 校准基准：docs/swarm-universe/production/SHORT_DRAMA_AGENT_BLUEPRINT.html
// EP01-EP04 实测 64 takes · 640-1280¥ · 59% 废片率 · 平均 ~15¥/take
// 反推：5s 720p 竖屏一次 V2V 生成 ≈ 500k tokens（Volcengine token = frame×resolution factor）
const DEFAULT_RATE_WITH_REF = 0.028    // ¥ / 千 tokens（V2V）
const DEFAULT_RATE_NO_REF = 0.046      // ¥ / 千 tokens（I2V）
const DEFAULT_TOKENS_PER_5S = 500000   // 5s 720p 9:16 基准 token 数（对齐 blueprint 实测）
const DEFAULT_IMG_UNIT = 1             // ¥ / 张（角色参考图生成，blueprint 约 1-2¥/张）
const DEFAULT_LLM_UNIT = 2.5           // ¥ / 次（qwen-max 长剧本生成，blueprint 约 2-5¥/次）
const DEFAULT_BGM_FLAT = 3             // ¥ / 集（BGM + TTS，blueprint < 1% 总成本）

interface EpisodeCostRow {
  id: string
  label: string
  totalTakes: number
  succeeded: number
  failed: number
  wastageRate: number
  seedanceCost: number
  note: string
}

export default function CostAnalysisModal({ open, onClose, nodes }: Props) {
  // 可编辑费率
  const [rateWith, setRateWith] = useState(DEFAULT_RATE_WITH_REF)
  const [rateNo, setRateNo] = useState(DEFAULT_RATE_NO_REF)
  const [tokens5s, setTokens5s] = useState(DEFAULT_TOKENS_PER_5S)
  const [imgUnit, setImgUnit] = useState(DEFAULT_IMG_UNIT)
  const [llmUnit, setLlmUnit] = useState(DEFAULT_LLM_UNIT)
  const [bgmFlat, setBgmFlat] = useState(DEFAULT_BGM_FLAT)

  // 聚合计算
  const { rows, totals, breakdown } = useMemo(() => {
    const epNodes = nodes.filter(n =>
      n.type === 'media' && (n.data as Record<string, unknown>)?.category === 'scene'
    )
    const charNodes = nodes.filter(n =>
      n.type === 'media' && (n.data as Record<string, unknown>)?.category === 'character'
    )

    const rows: EpisodeCostRow[] = []
    let totalSeedance = 0
    let totalTakes = 0
    let totalSucceeded = 0

    for (const n of epNodes) {
      const ep = n.data as unknown as EpisodeData
      const scenes = ep.scenes || []
      const takes = scenes.flatMap(s => s.takes || [])
      const succeeded = takes.filter(t => t.status === 'succeeded').length
      const failed = takes.filter(t => t.status === 'failed').length

      let seedanceCost = 0
      for (const t of takes) {
        if (t.status !== 'succeeded' && t.status !== 'failed') continue
        const dur = t.duration || 5
        const tokens = tokens5s * (dur / 5)
        const rate = t.ref_video_url ? rateWith : rateNo
        seedanceCost += (tokens / 1000) * rate
      }

      // 最多失败那场的 note（简易提示）
      const worst = scenes.find(s => (s.takes || []).filter(t => t.status === 'failed').length >= 3)
      const note = worst ? `${worst.id} 反复重试（≥3次）` : succeeded > 0 ? '基本顺利' : takes.length === 0 ? '尚未生产' : '部分失败'

      const total = takes.length
      rows.push({
        id: n.id,
        label: ep.label,
        totalTakes: total,
        succeeded,
        failed,
        wastageRate: total ? (total - succeeded) / total : 0,
        seedanceCost,
        note,
      })
      totalTakes += total
      totalSucceeded += succeeded
      totalSeedance += seedanceCost
    }

    // 其他成本估算
    // - LLM: 每集 2 次（writer_review + promo）
    // - 图片: 每个角色 1 张参考图
    // - BGM: 每集 flat
    const llmTotal = rows.length * 2 * llmUnit
    const imgTotal = charNodes.length * imgUnit
    const bgmTotal = rows.length * bgmFlat

    const totals = {
      totalTakes,
      totalSucceeded,
      totalFailed: totalTakes - totalSucceeded,
      wastageRate: totalTakes ? (totalTakes - totalSucceeded) / totalTakes : 0,
      episodes: rows.length,
      characters: charNodes.length,
      seedanceCost: totalSeedance,
      llmCost: llmTotal,
      imgCost: imgTotal,
      bgmCost: bgmTotal,
      grandTotal: totalSeedance + llmTotal + imgTotal + bgmTotal,
    }
    const g = totals.grandTotal || 1
    const breakdown = [
      { key: 'seedance', label: 'Seedance 2.0 视频生成', color: 'from-violet-500 to-fuchsia-500',
        amount: totalSeedance, pct: totalSeedance / g * 100 },
      { key: 'llm', label: 'LLM 对话（编剧/推广）', color: 'from-cyan-500 to-blue-500',
        amount: llmTotal, pct: llmTotal / g * 100 },
      { key: 'img', label: '图片生成 + TOS 转换', color: 'from-amber-500 to-orange-500',
        amount: imgTotal, pct: imgTotal / g * 100 },
      { key: 'bgm', label: 'BGM + TTS 合成', color: 'from-emerald-500 to-teal-500',
        amount: bgmTotal, pct: bgmTotal / g * 100 },
    ]

    return { rows, totals, breakdown }
  }, [nodes, rateWith, rateNo, tokens5s, imgUnit, llmUnit, bgmFlat])

  if (!open) return null

  const fmt = (n: number) => `${n.toFixed(2)}¥`
  const fmtPct = (n: number) => `${(n * 100).toFixed(0)}%`

  // 洞察：废片率趋势
  const trendRows = rows.filter(r => r.totalTakes > 0)
  const trend = trendRows.length >= 2
    ? trendRows.slice(-4).map(r => `${r.label.split(' ')[0]} ${fmtPct(r.wastageRate)}`).join(' → ')
    : '数据不足'
  const avgWastage = trendRows.length ? trendRows.reduce((s, r) => s + r.wastageRate, 0) / trendRows.length : 0

  return (
    <div className="fixed inset-0 z-[90] bg-black/70 backdrop-blur-sm flex items-center justify-center p-4"
      onClick={onClose}>
      <div className="bg-gradient-to-br from-gray-900 via-gray-900 to-gray-950 rounded-2xl shadow-2xl border border-gray-800 max-w-6xl w-full max-h-[90vh] overflow-hidden flex flex-col"
        onClick={e => e.stopPropagation()}>

        {/* Header */}
        <div className="px-6 py-4 border-b border-gray-800 flex items-center justify-between">
          <div className="flex items-center gap-3">
            <div className="p-2 rounded-lg bg-amber-500/20 border border-amber-500/40">
              <Coins className="w-5 h-5 text-amber-300" />
            </div>
            <div>
              <h2 className="text-lg font-bold text-gray-100">实际成本分析（含废片）</h2>
              <p className="text-[11px] text-gray-500">
                基于画布上 {totals.episodes} 集 · {totals.totalTakes} takes · {totals.characters} 角色，
                按 Seedance 2.0 实际费率估算
              </p>
            </div>
          </div>
          <button onClick={onClose}
            className="p-1.5 rounded-lg hover:bg-gray-800 text-gray-400 hover:text-white transition">
            <X className="w-4 h-4" />
          </button>
        </div>

        {/* Body */}
        <div className="flex-1 overflow-y-auto p-6 space-y-5">

          {/* 费率卡：可编辑 */}
          <div className="grid grid-cols-2 md:grid-cols-3 gap-3">
            <RateCard label="Seedance 2.0 含视频输入" subtitle="V2V · 多场串联"
              value={rateWith} unit="¥ / 千 tokens" color="violet"
              onChange={setRateWith} step={0.001} />
            <RateCard label="Seedance 2.0 不含视频输入" subtitle="I2V · 首镜"
              value={rateNo} unit="¥ / 千 tokens" color="fuchsia"
              onChange={setRateNo} step={0.001} />
            <RateCard label="5s 720p token 基准" subtitle="每 take 消耗"
              value={tokens5s} unit="tokens / 5s" color="cyan"
              onChange={setTokens5s} step={100} />
          </div>

          {/* EP 成本表 */}
          <div className="rounded-lg border border-gray-800 overflow-hidden">
            <div className="grid grid-cols-[1fr_80px_80px_80px_80px_120px_1fr] gap-2 px-3 py-2 bg-gray-900 border-b border-gray-800 text-[10px] uppercase tracking-wider text-gray-500 font-semibold">
              <span>集</span>
              <span className="text-right">总次数</span>
              <span className="text-right">成功</span>
              <span className="text-right">废片</span>
              <span className="text-right">废片率</span>
              <span className="text-right">Seedance 费用</span>
              <span>说明</span>
            </div>
            {rows.length === 0 && (
              <div className="p-6 text-center text-gray-500 text-sm">画布上还没有剧集节点</div>
            )}
            {rows.map(r => (
              <div key={r.id} className="grid grid-cols-[1fr_80px_80px_80px_80px_120px_1fr] gap-2 px-3 py-2 border-b border-gray-800/50 hover:bg-gray-900/40 text-xs">
                <span className="font-semibold text-gray-200 truncate">{r.label}</span>
                <span className="text-right font-mono text-gray-300">{r.totalTakes}</span>
                <span className="text-right font-mono text-emerald-400">{r.succeeded}</span>
                <span className="text-right font-mono text-red-400">{r.failed}</span>
                <span className="text-right">
                  <span className={`inline-block px-1.5 py-0.5 rounded font-mono text-[10px] ${
                    r.wastageRate >= 0.6 ? 'bg-red-900/40 text-red-300'
                    : r.wastageRate >= 0.3 ? 'bg-amber-900/40 text-amber-300'
                    : 'bg-emerald-900/40 text-emerald-300'}`}>
                    {fmtPct(r.wastageRate)}
                  </span>
                </span>
                <span className="text-right font-mono text-amber-300">{fmt(r.seedanceCost)}</span>
                <span className="text-gray-500 text-[11px] truncate">{r.note}</span>
              </div>
            ))}
            {rows.length > 0 && (
              <div className="grid grid-cols-[1fr_80px_80px_80px_80px_120px_1fr] gap-2 px-3 py-2 bg-gradient-to-r from-amber-950/40 to-gray-900 text-xs font-bold">
                <span className="text-amber-300">合计 / 平均</span>
                <span className="text-right font-mono text-amber-300">{totals.totalTakes}</span>
                <span className="text-right font-mono text-emerald-300">{totals.totalSucceeded}</span>
                <span className="text-right font-mono text-red-300">{totals.totalFailed}</span>
                <span className="text-right font-mono text-amber-300">{fmtPct(totals.wastageRate)}</span>
                <span className="text-right font-mono text-amber-300">{fmt(totals.seedanceCost)}</span>
                <span className="text-gray-400 text-[11px]">Seedance 单项；全链路见下方</span>
              </div>
            )}
          </div>

          {/* 双栏：每集次数 vs 可用 / 成本构成 */}
          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            {/* Left: 每集生成次数 vs 可用镜头 */}
            <div className="rounded-lg border border-gray-800 p-3 bg-gray-900/40">
              <div className="text-xs font-semibold text-gray-300 mb-3 flex items-center gap-1.5">
                <Target className="w-3.5 h-3.5 text-cyan-400" /> 每集生成次数 vs 可用镜头
              </div>
              <div className="space-y-2.5">
                {rows.length === 0 && <div className="text-xs text-gray-500">—</div>}
                {rows.map(r => {
                  const max = Math.max(...rows.map(x => x.totalTakes), 1)
                  const gPct = (r.succeeded / max) * 100
                  const rPct = (r.failed / max) * 100
                  return (
                    <div key={r.id}>
                      <div className="flex items-center justify-between mb-0.5">
                        <span className="text-[11px] text-gray-400 truncate">{r.label}</span>
                        <span className="text-[10px] text-gray-500 font-mono">
                          {r.totalTakes}次 → {r.succeeded}可用
                        </span>
                      </div>
                      <div className="flex h-3 rounded overflow-hidden bg-gray-800">
                        <div className="bg-emerald-500" style={{ width: `${gPct}%` }} title={`${r.succeeded} 可用`} />
                        <div className="bg-red-500/70" style={{ width: `${rPct}%` }} title={`${r.failed} 废片`} />
                      </div>
                    </div>
                  )
                })}
              </div>
              <div className="mt-3 flex items-center gap-3 text-[10px] text-gray-500">
                <span className="flex items-center gap-1"><span className="w-2 h-2 bg-emerald-500 rounded-sm" /> 可用</span>
                <span className="flex items-center gap-1"><span className="w-2 h-2 bg-red-500/70 rounded-sm" /> 废片</span>
              </div>
            </div>

            {/* Right: 成本构成 */}
            <div className="rounded-lg border border-gray-800 p-3 bg-gray-900/40">
              <div className="text-xs font-semibold text-gray-300 mb-3 flex items-center gap-1.5">
                <Coins className="w-3.5 h-3.5 text-amber-400" /> 成本构成（{fmt(totals.grandTotal)} 总计）
              </div>
              <div className="space-y-2.5">
                {breakdown.map(b => (
                  <div key={b.key}>
                    <div className="flex items-center justify-between mb-0.5 text-[11px]">
                      <span className="text-gray-300">{b.label}</span>
                      <span className="font-mono text-gray-400">{fmt(b.amount)} · ~{b.pct.toFixed(0)}%</span>
                    </div>
                    <div className="h-2 rounded bg-gray-800 overflow-hidden">
                      <div className={`h-full bg-gradient-to-r ${b.color} transition-all`}
                        style={{ width: `${Math.max(b.pct, 2)}%` }} />
                    </div>
                  </div>
                ))}
              </div>
              <div className="mt-3 p-2 rounded bg-amber-950/40 border border-amber-800/50 text-[10px] text-amber-200/80 flex items-start gap-1.5">
                <AlertTriangle className="w-3 h-3 text-amber-400 flex-shrink-0 mt-0.5" />
                <span>
                  废片是最大成本黑洞 —
                  <span className="font-semibold text-amber-200 mx-1">{fmtPct(totals.wastageRate)}</span>
                  的生成都作废。降低 wastage 比降价更有效。
                </span>
              </div>

              {/* Tweakable cost assumptions */}
              <div className="mt-3 pt-3 border-t border-gray-800 grid grid-cols-3 gap-2">
                <TinyInput label="每张图 ¥" value={imgUnit} onChange={setImgUnit} step={0.01} />
                <TinyInput label="每次LLM ¥" value={llmUnit} onChange={setLlmUnit} step={0.01} />
                <TinyInput label="每集BGM ¥" value={bgmFlat} onChange={setBgmFlat} step={0.5} />
              </div>
            </div>
          </div>

          {/* 洞察卡 */}
          <div className="grid grid-cols-1 md:grid-cols-4 gap-3">
            <InsightCard icon={TrendingDown} color="red" title="废片率趋势"
              body={trend}
              note={`均值 ${fmtPct(avgWastage)}`}
            />
            <InsightCard icon={Target} color="violet" title="[图N] 标签效果"
              body={totals.characters > 0 ? `${totals.characters} 个角色卡均贴 [图N]` : '无角色卡'}
              note="降 wastage 最快手段"
            />
            <InsightCard icon={Lightbulb} color="emerald" title="优化方向"
              body="走 720P 预览 + 精选 1080P 正式生成"
              note="可省 30–50%"
            />
            <InsightCard icon={AlertTriangle} color="amber" title="关键教训"
              body="单镜 >3 次重试的就换镜头 / 改 prompt"
              note="避免单场 >30% 成本黑洞"
            />
          </div>

          {/* 底部总结 */}
          <div className="rounded-lg border border-amber-700/40 bg-gradient-to-br from-amber-950/20 to-gray-900 p-4 flex items-center justify-between">
            <div>
              <div className="text-[10px] uppercase tracking-wider text-amber-400/80">全链路估算总成本</div>
              <div className="text-2xl font-bold text-amber-200 mt-1 flex items-baseline gap-2">
                {fmt(totals.grandTotal)}
                {totals.episodes > 0 && (
                  <span className="text-xs text-gray-400 font-normal">
                    · ~{fmt(totals.grandTotal / totals.episodes)}/集
                    · ~{totals.totalSucceeded ? fmt(totals.grandTotal / totals.totalSucceeded) : '—'}/可用镜头
                  </span>
                )}
              </div>
            </div>
            <div className="text-right">
              <div className="text-[10px] uppercase tracking-wider text-gray-500">若 wastage 降至 30%</div>
              <div className="text-lg font-bold text-emerald-300">
                ~{fmt(totals.grandTotal * (1 - (totals.wastageRate - 0.3) * 0.9))}
              </div>
              <div className="text-[10px] text-gray-500">省 ~{fmt(totals.grandTotal * (totals.wastageRate - 0.3) * 0.9)}</div>
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}

function RateCard({ label, subtitle, value, unit, color, onChange, step }: {
  label: string; subtitle: string; value: number; unit: string;
  color: 'violet' | 'fuchsia' | 'cyan'; onChange: (v: number) => void; step: number;
}) {
  const tint = color === 'violet' ? 'from-violet-900/40 to-gray-900 border-violet-700/40 text-violet-200'
    : color === 'fuchsia' ? 'from-fuchsia-900/40 to-gray-900 border-fuchsia-700/40 text-fuchsia-200'
    : 'from-cyan-900/40 to-gray-900 border-cyan-700/40 text-cyan-200'
  return (
    <div className={`rounded-lg border bg-gradient-to-br p-3 ${tint}`}>
      <div className="text-[10px] uppercase tracking-wider opacity-80">{label}</div>
      <div className="text-[10px] opacity-60">{subtitle}</div>
      <div className="mt-1.5 flex items-baseline gap-1">
        <input type="number" step={step} value={value}
          onChange={e => onChange(parseFloat(e.target.value) || 0)}
          className="w-20 text-2xl font-bold bg-transparent outline-none border-b border-transparent hover:border-white/30 focus:border-white/60" />
        <span className="text-[10px] opacity-60">{unit}</span>
      </div>
    </div>
  )
}

function TinyInput({ label, value, onChange, step }: {
  label: string; value: number; onChange: (v: number) => void; step: number;
}) {
  return (
    <label className="flex flex-col gap-0.5">
      <span className="text-[9px] text-gray-500 uppercase tracking-wider">{label}</span>
      <input type="number" step={step} value={value}
        onChange={e => onChange(parseFloat(e.target.value) || 0)}
        className="px-1.5 py-1 rounded bg-gray-800 border border-gray-700 text-xs text-gray-200 font-mono focus:border-amber-500 outline-none w-full" />
    </label>
  )
}

function InsightCard({ icon: Icon, color, title, body, note }: {
  icon: React.ComponentType<{ className?: string }>; color: 'red' | 'violet' | 'emerald' | 'amber';
  title: string; body: string; note: string;
}) {
  const tint = color === 'red' ? 'border-red-700/40 bg-red-950/20 text-red-300'
    : color === 'violet' ? 'border-violet-700/40 bg-violet-950/20 text-violet-300'
    : color === 'emerald' ? 'border-emerald-700/40 bg-emerald-950/20 text-emerald-300'
    : 'border-amber-700/40 bg-amber-950/20 text-amber-300'
  return (
    <div className={`rounded-lg border p-3 ${tint}`}>
      <div className="flex items-center gap-1.5 mb-1.5">
        <Icon className="w-3.5 h-3.5" />
        <span className="text-[11px] font-semibold uppercase tracking-wider">{title}</span>
      </div>
      <div className="text-[11px] text-gray-200 leading-relaxed">{body}</div>
      <div className="mt-1.5 text-[10px] opacity-70">{note}</div>
    </div>
  )
}
