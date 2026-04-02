import React, { useState, useEffect, useCallback } from 'react'
import {
  Activity, TrendingUp, TrendingDown, Brain,
  LogOut, RefreshCw, BarChart3, Flame, Eye,
  ArrowUp, ArrowDown, Minus, AlertTriangle, Loader2,
  Zap, Shield, Crosshair, LineChart as LineChartIcon,
  History, FileText, Lightbulb, BarChart2, Target, Layers,
  Sun, Moon, Clock,
} from 'lucide-react'
import {
  ResponsiveContainer, AreaChart, Area, XAxis, YAxis,
  CartesianGrid, Tooltip, ReferenceLine,
} from 'recharts'

interface Props { token: string; onLogout: () => void }

const API = '/bridge'

async function fetchJSON(url: string, timeout = 30000) {
  const c = new AbortController()
  const id = setTimeout(() => c.abort(), timeout)
  try { const r = await fetch(url, { signal: c.signal }); return await r.json() }
  catch { return null } finally { clearTimeout(id) }
}

/* ─── helpers ─── */
function envLabel(d: string) {
  const m: Record<string, string> = { bullish: '看多', bearish: '看空', sideways: '横盘', neutral: '中性' }
  return m[d] || d || '—'
}
function envColor(d: string) {
  if (d === 'bullish') return 'text-red-400'
  if (d === 'bearish') return 'text-green-400'
  return 'text-yellow-400'
}
function sentColor(s: number) {
  if (s >= 70) return 'text-red-400'
  if (s <= 30) return 'text-green-400'
  return 'text-yellow-400'
}
function fmtMoney(v: number) {
  if (Math.abs(v) >= 1e8) return `${(v / 1e8).toFixed(2)}亿`
  if (Math.abs(v) >= 1e4) return `${(v / 1e4).toFixed(2)}万`
  return v.toLocaleString('zh-CN', { minimumFractionDigits: 2, maximumFractionDigits: 2 })
}
function sentLabel(s: number) {
  if (s >= 80) return '极度贪婪'
  if (s >= 60) return '贪婪'
  if (s >= 40) return '中性'
  if (s >= 20) return '恐惧'
  return '极度恐惧'
}
function trendIcon(status: string) {
  if (status?.includes('bullish')) return <ArrowUp className="w-3.5 h-3.5 text-red-400" />
  if (status?.includes('bearish')) return <ArrowDown className="w-3.5 h-3.5 text-green-400" />
  return <Minus className="w-3.5 h-3.5 text-gray-400" />
}
function pnlClass(v: number) { return v >= 0 ? 'text-red-400' : 'text-green-400' }
function pnlStr(v: number) { return `${v >= 0 ? '+' : ''}${v.toFixed(2)}%` }

const TREND_CN: Record<string, string> = {
  strong_bullish: '强多(5>8>34>82)', bullish: '多头(5>8>34)', neutral_up: '偏多(>MA34)',
  neutral: '中性(>MA82)', neutral_down: '偏空', bearish: '空头(5<8,<MA34)', strong_bearish: '强空(5<8<34<82)',
}
const LIFECYCLE_CN: Record<string, string> = {
  accumulation: '吸筹', markup: '拉升', shakeout: '洗盘',
  distribution: '出货', markdown: '下跌', unknown: '未知',
}
function pct(v: number | undefined) {
  if (v == null) return 0
  return v <= 1 ? v * 100 : v
}

/* ═══════════════ MAIN ═══════════════ */
export default function Dashboard({ token, onLogout }: Props) {
  const [time, setTime] = useState(new Date())
  const [refreshing, setRefreshing] = useState(false)
  const [health, setHealth] = useState<any>(null)
  const [macro, setMacro] = useState<any>(null)
  const [sentiment, setSentiment] = useState<any>(null)
  const [sectors, setSectors] = useState<any>(null)
  const [positions, setPositions] = useState<any[]>([])
  const [accountInfo, setAccountInfo] = useState<any>(null)
  const [research, setResearch] = useState<Record<string, any>>({})
  const [master, setMaster] = useState<any>(null)
  const [masterLoading, setMasterLoading] = useState(false)
  const [equityCurve, setEquityCurve] = useState<any>(null)
  const [equityPeriod, setEquityPeriod] = useState('1m')
  const [strategyStatus, setStrategyStatus] = useState<any>(null)
  const [closedTrades, setClosedTrades] = useState<any[]>([])
  const [reviewData, setReviewData] = useState<any>(null)
  const [briefing, setBriefing] = useState<any>(null)
  const [briefingLoading, setBriefingLoading] = useState<'premarket' | 'eod' | null>(null)
  const [teamData, setTeamData] = useState<any>(null)

  // Fast refresh: positions + account (every 5s for realtime)
  const refreshFast = useCallback(async () => {
    const [h, pos, acct] = await Promise.all([
      fetchJSON(`${API}/health`),
      fetchJSON(`${API}/positions`),
      fetchJSON(`${API}/account/info`),
    ])
    setHealth(h)
    const posList = Array.isArray(pos) ? pos : []
    setPositions(posList.map((p: any) => ({ ...p, cost_price: p.cost_price || 0, market_price: p.market_price || 0, name: p.name || p.code })))
    setAccountInfo(acct)
  }, [])

  // Slow refresh: macro + sentiment + sectors + strategy + team (every 15s)
  const refreshSlow = useCallback(async () => {
    const [m, se, st, ss, ct, rv, br, tm] = await Promise.all([
      fetchJSON(`${API}/trading/macro`),
      fetchJSON(`${API}/trading/sentiment`),
      fetchJSON(`${API}/trading/sectors`),
      fetchJSON(`${API}/strategy/status?limit=80`),
      fetchJSON(`${API}/trading/history?limit=50`),
      fetchJSON(`${API}/trading/review`),
      fetchJSON(`${API}/trading/briefing/latest`),
      fetchJSON(`${API}/team/status`),
    ])
    setMacro(m); setSentiment(se); setSectors(st); setStrategyStatus(ss)
    if (Array.isArray(ct)) setClosedTrades(ct)
    if (rv && rv.stats) setReviewData(rv)
    if (br) setBriefing(br)
    if (tm && tm.agents) setTeamData(tm)
  }, [])

  const refresh = useCallback(async () => {
    setRefreshing(true)
    await Promise.all([refreshFast(), refreshSlow()])
    setRefreshing(false)
  }, [refreshFast, refreshSlow])

  useEffect(() => {
    if (positions.length === 0) return
    const run = async () => {
      const res: Record<string, any> = {}
      await Promise.allSettled(positions.map(async (p: any) => {
        const r = await fetchJSON(`${API}/trading/research?code=${p.code}&cost_price=${p.cost_price || 0}`)
        if (r) res[p.code] = r
      }))
      setResearch(res)
    }
    run()
  }, [positions])

  const fetchMaster = useCallback(async () => {
    setMasterLoading(true)
    setMaster(await fetchJSON(`${API}/trading/master`, 120000))
    setMasterLoading(false)
  }, [])

  const fetchEquity = useCallback(async (p?: string) => {
    const period = p || equityPeriod
    const data = await fetchJSON(`${API}/account/equity?period=${period}`)
    if (data) setEquityCurve(data)
  }, [equityPeriod])

  useEffect(() => { fetchEquity() }, [equityPeriod])

  useEffect(() => {
    refresh()
    const t1 = setInterval(() => setTime(new Date()), 1000)
    const t2 = setInterval(refreshFast, 5000)    // positions every 5s
    const t3 = setInterval(refreshSlow, 15000)   // market data every 15s
    return () => { clearInterval(t1); clearInterval(t2); clearInterval(t3) }
  }, [refresh, refreshFast, refreshSlow])

  const mktValue = positions.reduce((s: number, p: any) => s + (p.market_price || 0) * (p.volume || 0), 0)
  const totalPnl = positions.reduce((s: number, p: any) => s + (p.pnl_float || 0), 0)
  const todayPnlFromPos = positions.reduce((s: number, p: any) => {
    const lc = p.last_close || p.cost_price || 0
    return s + ((p.market_price || 0) - lc) * (p.volume || 0)
  }, 0)
  const isFallback = accountInfo?._fallback === true
  const eqLatest = equityCurve?.points?.length ? equityCurve.points[equityCurve.points.length - 1] : null
  const totalAssets = accountInfo?.total_assets || (isFallback && eqLatest ? eqLatest.total : 0) || mktValue
  const floatPnl = accountInfo?.float_pnl || totalPnl
  const todayPnl = accountInfo?.today_profit || todayPnlFromPos
  const mktValueActual = accountInfo?.market_value || mktValue
  const availableCash = accountInfo?.available || (isFallback && eqLatest ? eqLatest.cash : 0)
  const withdrawable = accountInfo?.withdrawable || 0
  const posRatio = accountInfo?.position_ratio || (totalAssets > 0 ? mktValueActual / totalAssets * 100 : 0)
  const qmtOk = health?.qmt_connected || false
  const sentScore = sentiment?.score ?? 50
  const mktDir = macro?.direction || '—'
  const confidence = pct(macro?.confidence)
  const posAdvice = macro?.position_advice || '—'

  return (
    <div className="min-h-screen bg-gray-950 text-white">
      {/* ── Header ── */}
      <header className="bg-gray-900/80 backdrop-blur border-b border-gray-800 px-6 py-3 flex items-center justify-between sticky top-0 z-50">
        <div className="flex items-center gap-2">
          <div className="w-8 h-8 rounded-lg bg-gradient-to-br from-red-500 to-amber-500 flex items-center justify-center font-bold text-xs">Q8</div>
          <span className="text-lg font-bold tracking-tight">Q8bot</span>
          <span className="text-sm text-gray-400 hidden sm:inline">AI量化大屏</span>
          <span className="text-xs text-gray-600 font-mono ml-3">{time.toLocaleTimeString()}</span>
          <span className={`ml-2 text-xs px-2 py-0.5 rounded-full ${qmtOk ? 'bg-emerald-500/20 text-emerald-400' : 'bg-red-500/20 text-red-400'}`}>
            QMT {qmtOk ? '在线' : '离线'}
          </span>
        </div>
        <div className="flex items-center gap-2">
          <button onClick={refresh} disabled={refreshing}
            className="flex items-center gap-1.5 text-sm bg-gray-800 hover:bg-gray-700 px-3 py-1.5 rounded-lg transition">
            <RefreshCw className={`w-3.5 h-3.5 ${refreshing ? 'animate-spin' : ''}`} />刷新
          </button>
          <button onClick={fetchMaster} disabled={masterLoading}
            className="flex items-center gap-1.5 text-sm bg-gradient-to-r from-red-600 to-amber-600 hover:from-red-500 hover:to-amber-500 disabled:from-gray-700 disabled:to-gray-700 px-3 py-1.5 rounded-lg transition font-medium">
            <Brain className={`w-3.5 h-3.5 ${masterLoading ? 'animate-pulse' : ''}`} />
            {masterLoading ? 'AI分析中…' : 'Master分析'}
          </button>
          <button onClick={onLogout} className="text-gray-500 hover:text-white transition"><LogOut className="w-4 h-4" /></button>
        </div>
      </header>

      <div className="p-4 md:p-6 space-y-5 max-w-[1600px] mx-auto">
        {/* ── Row 1: Account Summary ── */}
        <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
          <Card label="总资产" icon={BarChart3} value={`¥${fmtMoney(totalAssets)}`} />
          <Card label="浮动盈亏" icon={TrendingUp}
            value={`${floatPnl >= 0 ? '+' : ''}${fmtMoney(floatPnl)}`}
            vc={floatPnl >= 0 ? 'text-red-400' : 'text-green-400'} />
          <Card label="当日盈亏" icon={Activity}
            value={`${todayPnl >= 0 ? '+' : ''}${fmtMoney(todayPnl)}`}
            vc={todayPnl >= 0 ? 'text-red-400' : 'text-green-400'}
            sub={totalAssets > 0 ? `${(todayPnl / totalAssets * 100).toFixed(2)}%` : ''} />
          <Card label="总市值" icon={Flame} value={`¥${fmtMoney(mktValueActual)}`} />
        </div>
        <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
          <Card label="可用资金" icon={Shield} value={isFallback ? '收盘' : `¥${fmtMoney(availableCash)}`} vc={isFallback ? 'text-gray-500' : 'text-white'} />
          <Card label="可取资金" icon={Eye} value={isFallback ? '收盘' : `¥${fmtMoney(withdrawable)}`} vc={isFallback ? 'text-gray-500' : 'text-white'} />
          <Card label="仓位" icon={Crosshair} value={`${posRatio.toFixed(1)}%`}
            vc={posRatio >= 80 ? 'text-red-400' : posRatio >= 50 ? 'text-yellow-400' : 'text-emerald-400'}
            sub={`${positions.length}只 · 建议${posAdvice}`} />
          <Card label="市场情绪" icon={Zap} value={`${sentScore}`} vc={sentColor(sentScore)}
            sub={`${sentLabel(sentScore)} · ${envLabel(mktDir)}`} />
        </div>

        {/* ── Row 1.5: Equity Curve ── */}
        <EquityCurve data={equityCurve} period={equityPeriod} onPeriod={(p: string) => { setEquityPeriod(p); fetchEquity(p) }} />

        {/* ── Row 2: Market + Sectors ── */}
        <div className="grid lg:grid-cols-3 gap-5">
          <MarketPanel macro={macro} sentiment={sentiment} />
          <SectorPanel sectors={sectors} />
        </div>

        {/* ── Row 2.5: Strategy Lifecycle ── */}
        <StrategyPanel status={strategyStatus} />

        {/* ── Row 2.6: Team Agents ── */}
        <TeamPanel data={teamData} />

        {/* ── Row 2.7: Briefing (盘前分析 + 日终复盘) ── */}
        <BriefingPanel briefing={briefing} loading={briefingLoading} onGenerate={async (type: 'premarket' | 'eod') => {
          setBriefingLoading(type)
          try {
            const c = new AbortController()
            const tid = setTimeout(() => c.abort(), 90000)
            try {
              const r = await fetch(`${API}/trading/briefing/${type}`, { method: 'POST', signal: c.signal })
              const res = await r.json()
              if (res && !res.error) {
                setBriefing((prev: any) => ({ ...prev, [type]: { ...res, content: res.content } }))
              }
            } finally { clearTimeout(tid) }
          } catch (e) { console.error('briefing gen error', e) }
          finally { setBriefingLoading(null) }
        }} />

        {/* ── Row 3: Positions + Closed Trades ── */}
        <PositionTable positions={positions} research={research} closedTrades={closedTrades} reviewData={reviewData} />

        {/* ── Row 4: Master ── */}
        <MasterReport master={master} loading={masterLoading} />
      </div>
    </div>
  )
}

/* ═══════════════ CARD ═══════════════ */
function Card({ label, icon: Icon, value, vc = 'text-white', sub }: {
  label: string; icon: any; value: string; vc?: string; sub?: string
}) {
  return (
    <div className="bg-gray-900 border border-gray-800 rounded-xl p-4">
      <div className="flex items-center gap-1.5 mb-1.5">
        <Icon className="w-3.5 h-3.5 text-gray-500" />
        <span className="text-[11px] text-gray-500 uppercase tracking-wider">{label}</span>
      </div>
      <div className={`text-xl font-bold ${vc}`}>{value}</div>
      {sub && <div className="text-xs text-gray-500 mt-0.5">{sub}</div>}
    </div>
  )
}

/* ═══════════════ L1+L3 MARKET ═══════════════ */
function MarketPanel({ macro, sentiment }: { macro: any; sentiment: any }) {
  const sentScore = sentiment?.score ?? 50
  const ind = sentiment?.indicators || {}
  const idx = macro?.indices?.['000001.SH'] || {}

  return (
    <div className="lg:col-span-2 bg-gray-900 border border-gray-800 rounded-xl p-5 space-y-4">
      <div className="flex items-center gap-2 text-sm font-medium text-gray-300">
        <Activity className="w-4 h-4 text-amber-400" />
        <span>市场概览</span>
        <span className="text-[10px] text-gray-600 ml-auto">L1 宏观 + L3 情绪</span>
      </div>

      {/* Sentiment gauge bar */}
      <div className="space-y-1.5">
        <div className="flex items-center justify-between text-xs">
          <span className="text-green-400">恐惧</span>
          <span className={`font-bold text-base ${sentColor(sentScore)}`}>{sentScore}</span>
          <span className="text-red-400">贪婪</span>
        </div>
        <div className="h-3 rounded-full bg-gradient-to-r from-green-600 via-yellow-500 to-red-600 relative overflow-hidden">
          <div className="absolute top-0 h-full w-1 bg-white rounded shadow-lg shadow-white/50 transition-all duration-700"
            style={{ left: `${Math.min(Math.max(sentScore, 2), 98)}%` }} />
        </div>
      </div>

      {/* Grid of metrics */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
        <Metric label="涨跌比" value={ind.ad_ratio?.toFixed(2) ?? '—'} />
        <Metric label="涨停" value={ind.limit_up ?? '—'} color="text-red-400" />
        <Metric label="跌停" value={ind.limit_down ?? '—'} color="text-green-400" />
        <Metric label="量比" value={ind.index_vol_ratio?.toFixed(2) ?? '—'} />
        <Metric label="上证" value={idx.price ? `${idx.price.toFixed(0)}` : '—'}
          color={idx.chg_1d > 0 ? 'text-red-400' : idx.chg_1d < 0 ? 'text-green-400' : 'text-white'} />
        <Metric label="日涨跌" value={idx.chg_1d != null ? `${idx.chg_1d > 0 ? '+' : ''}${idx.chg_1d.toFixed(2)}%` : '—'}
          color={idx.chg_1d > 0 ? 'text-red-400' : 'text-green-400'} />
        <Metric label="宽度" value={macro?.breadth_score != null ? `${(macro.breadth_score * 100).toFixed(0)}%` : '—'} />
        <Metric label="信心度" value={`${pct(macro?.confidence).toFixed(0)}%`}
          color={pct(macro?.confidence) >= 60 ? 'text-red-400' : 'text-yellow-400'} />
      </div>

      {/* AI Signals & Reasons */}
      <div className="space-y-2 border-t border-gray-800 pt-3">
        {sentiment?.signal && (
          <div className="flex items-start gap-2 text-xs">
            <Zap className="w-3.5 h-3.5 text-amber-400 mt-0.5 shrink-0" />
            <span className="text-gray-300">{sentiment.signal}</span>
          </div>
        )}
        {(macro?.reasons || []).map((r: string, i: number) => (
          <div key={i} className="flex items-start gap-2 text-xs">
            <Eye className="w-3.5 h-3.5 text-blue-400 mt-0.5 shrink-0" />
            <span className="text-gray-400">{r}</span>
          </div>
        ))}
        {macro?.position_advice && (
          <div className="flex items-start gap-2 text-xs">
            <Shield className="w-3.5 h-3.5 text-emerald-400 mt-0.5 shrink-0" />
            <span className="text-gray-300">仓位建议: <strong className="text-white">{macro.position_advice}</strong></span>
          </div>
        )}
      </div>
    </div>
  )
}

function Metric({ label, value, color = 'text-white' }: { label: string; value: string | number; color?: string }) {
  return (
    <div className="bg-gray-800/50 rounded-lg px-3 py-2">
      <div className="text-[10px] text-gray-500 uppercase">{label}</div>
      <div className={`text-sm font-semibold ${color}`}>{value}</div>
    </div>
  )
}

/* ═══════════════ L2 SECTORS ═══════════════ */
function SectorPanel({ sectors }: { sectors: any }) {
  const hot = sectors?.hot_sectors || []
  const cold = sectors?.cold_sectors || []
  const all = sectors?.all_sectors || []
  const showHot = hot.length > 0 ? hot.slice(0, 5) : all.slice(0, 6)
  const showCold = cold.length > 0 ? cold.slice(0, 5) : all.slice(-3).reverse()

  return (
    <div className="bg-gray-900 border border-gray-800 rounded-xl p-5 space-y-4">
      <div className="flex items-center gap-2 text-sm font-medium text-gray-300">
        <Flame className="w-4 h-4 text-orange-400" />
        <span>板块轮动</span>
        <span className="text-[10px] text-gray-600 ml-auto">L2</span>
      </div>

      <div className="space-y-3">
        <div>
          <div className="text-[10px] text-gray-500 uppercase mb-1.5">热门板块</div>
          {showHot.length === 0 ? (
            <div className="text-xs text-gray-600">暂无热点板块</div>
          ) : (
            <div className="space-y-1">
              {showHot.map((s: any, i: number) => (
                <div key={i} className="flex items-center justify-between text-sm bg-red-500/5 rounded px-2 py-1">
                  <span className="text-gray-200 truncate">{s.name}</span>
                  <span className={`font-mono text-xs ${s.chg_1d >= 0 ? 'text-red-400' : 'text-green-400'}`}>
                    {s.chg_1d >= 0 ? '+' : ''}{(s.chg_1d || 0).toFixed(2)}%
                  </span>
                </div>
              ))}
            </div>
          )}
        </div>

        <div>
          <div className="text-[10px] text-gray-500 uppercase mb-1.5">回避板块</div>
          {showCold.length === 0 ? (
            <div className="text-xs text-gray-600">暂无弱势板块</div>
          ) : (
            <div className="space-y-1">
              {showCold.map((s: any, i: number) => (
                <div key={i} className="flex items-center justify-between text-sm bg-green-500/5 rounded px-2 py-1">
                  <span className="text-gray-200 truncate">{s.name}</span>
                  <span className={`font-mono text-xs ${s.chg_1d >= 0 ? 'text-red-400' : 'text-green-400'}`}>
                    {s.chg_1d >= 0 ? '+' : ''}{(s.chg_1d || 0).toFixed(2)}%
                  </span>
                </div>
              ))}
            </div>
          )}
        </div>
      </div>
    </div>
  )
}

/* ═══════════════ STRATEGY LIFECYCLE ═══════════════ */
function StrategyPanel({ status }: { status: any }) {
  const [showAll, setShowAll] = useState(false)
  const [logFilter, setLogFilter] = useState<'all'|'warn'|'error'>('all')
  if (!status) return null
  const phase = status.phase || 'idle'
  const risk = status.risk || {}
  const lastScan = status.last_scan || {}
  const logs: any[] = status.logs || []
  const warnCount = logs.filter((l: any) => l.level === 'WARNING').length
  const errorCount = logs.filter((l: any) => l.level === 'ERROR').length
  const filteredLogs = logFilter === 'all' ? logs :
    logFilter === 'warn' ? logs.filter((l: any) => l.level === 'WARNING' || l.level === 'ERROR') :
    logs.filter((l: any) => l.level === 'ERROR')

  const phaseMap: Record<string, { label: string; color: string; icon: string }> = {
    idle: { label: '空闲', color: 'bg-gray-600', icon: '⏸' },
    scanning: { label: '扫描选股', color: 'bg-blue-500 animate-pulse', icon: '🔍' },
    scoring: { label: '多因子打分', color: 'bg-cyan-500 animate-pulse', icon: '📊' },
    confirming: { label: 'AI确认', color: 'bg-purple-500 animate-pulse', icon: '🤖' },
    ordering: { label: '下单执行', color: 'bg-amber-500 animate-pulse', icon: '📝' },
    monitoring: { label: '持仓监控', color: 'bg-emerald-500', icon: '👁' },
  }
  const p = phaseMap[phase] || phaseMap.idle

  const levelColor = (lvl: string) => {
    if (lvl === 'WARNING') return 'text-yellow-400'
    if (lvl === 'ERROR') return 'text-red-400'
    return 'text-gray-400'
  }

  const highlightMsg = (msg: string) => {
    if (msg.includes('SCAN START') || msg.includes('SCAN COMPLETE') || msg.includes('AUTO SCAN')) return 'text-blue-300 font-medium'
    if (msg.includes('ORDER:')) return 'text-amber-300 font-medium'
    if (msg.includes('RISK BLOCKED')) return 'text-red-400'
    if (msg.includes('SKIP')) return 'text-yellow-400'
    if (msg.includes('confirm')) return 'text-purple-300'
    if (msg.includes('candidate')) return 'text-cyan-300'
    if (msg.includes('exit') || msg.includes('sell') || msg.includes('liquidat')) return 'text-green-300'
    return 'text-gray-300'
  }

  const visibleLogs = showAll ? filteredLogs : filteredLogs.slice(-20)

  return (
    <div className="bg-gray-900 border border-gray-800 rounded-xl p-5 space-y-4">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2 text-sm font-medium text-gray-300">
          <Activity className="w-4 h-4 text-blue-400" />
          <span>策略生命周期</span>
          <span className="text-[10px] text-gray-600">实时状态 · 日志</span>
        </div>
        <div className="flex items-center gap-3">
          {/* Phase badge */}
          <div className="flex items-center gap-1.5">
            <span className="text-sm">{p.icon}</span>
            <span className={`text-xs px-2 py-0.5 rounded-full text-white ${p.color}`}>{p.label}</span>
          </div>
        </div>
      </div>

      {/* Status cards */}
      <div className="grid grid-cols-2 md:grid-cols-5 gap-3 text-xs">
        <div className="bg-gray-800/50 rounded-lg p-2.5">
          <div className="text-gray-500 mb-0.5">市场环境</div>
          <div className="font-medium">{risk.market_env || '—'}</div>
        </div>
        <div className="bg-gray-800/50 rounded-lg p-2.5">
          <div className="text-gray-500 mb-0.5">持仓数</div>
          <div className="font-medium">{risk.holdings ?? '—'} / {risk.max_holdings ?? '—'}</div>
        </div>
        <div className="bg-gray-800/50 rounded-lg p-2.5">
          <div className="text-gray-500 mb-0.5">可买预算</div>
          <div className={`font-medium ${(risk.buy_budget ?? 0) > 0 ? 'text-emerald-400' : 'text-red-400'}`}>
            ¥{((risk.buy_budget ?? 0) / 10000).toFixed(1)}万
          </div>
        </div>
        <div className="bg-gray-800/50 rounded-lg p-2.5">
          <div className="text-gray-500 mb-0.5">上次扫描</div>
          <div className="font-medium">{lastScan.time || '—'}</div>
        </div>
        <div className="bg-gray-800/50 rounded-lg p-2.5">
          <div className="text-gray-500 mb-0.5">扫描结果</div>
          <div className="font-medium">{lastScan.duration ? `${lastScan.duration.toFixed(0)}s · ${lastScan.orders}单` : '—'}</div>
        </div>
      </div>

      {/* Pipeline visualization — animated */}
      <style>{`
        @keyframes flowDot { 0% { left: 0; opacity: 0 } 20% { opacity: 1 } 80% { opacity: 1 } 100% { left: 100%; opacity: 0 } }
        @keyframes glowPulse { 0%,100% { box-shadow: 0 0 4px rgba(59,130,246,0.3) } 50% { box-shadow: 0 0 12px rgba(59,130,246,0.7) } }
        @keyframes breathe { 0%,100% { opacity: 0.5 } 50% { opacity: 1 } }
        .pipe-active { animation: glowPulse 1.5s ease-in-out infinite }
        .pipe-idle { animation: breathe 3s ease-in-out infinite }
        .pipe-flow { position: relative; overflow: hidden }
        .pipe-flow::after { content: ''; position: absolute; top: 50%; transform: translateY(-50%); width: 6px; height: 6px; border-radius: 50%; background: #3b82f6; animation: flowDot 1.2s ease-in-out infinite }
      `}</style>
      <div className="flex items-center text-[10px] overflow-x-auto pb-1">
        {['scanning', 'scoring', 'confirming', 'ordering', 'monitoring'].map((s, i) => {
          const sp = phaseMap[s]
          const stages = ['scanning', 'scoring', 'confirming', 'ordering', 'monitoring']
          const phaseIdx = stages.indexOf(phase)
          const isActive = s === phase
          const isPast = phaseIdx > i
          const isNext = phaseIdx === i - 1
          return (
            <React.Fragment key={s}>
              {i > 0 && (
                <div className={`w-8 h-0.5 mx-0.5 rounded-full ${
                  isNext && phase !== 'idle' ? 'pipe-flow bg-blue-900' :
                  isPast ? 'bg-blue-500' : 'bg-gray-700/50'
                }`} />
              )}
              <div className={`flex items-center gap-1.5 px-3 py-1.5 rounded-lg whitespace-nowrap transition-all duration-500 ${
                isActive ? 'pipe-active bg-blue-500/20 text-blue-200 ring-1 ring-blue-400/60 scale-105' :
                isPast ? 'bg-emerald-500/10 text-emerald-400 ring-1 ring-emerald-500/30' :
                phase === 'idle' ? 'pipe-idle bg-gray-800/40 text-gray-500' :
                'bg-gray-800/30 text-gray-600'
              }`}>
                <span className="text-sm">{isPast ? '✓' : sp.icon}</span>
                <span className={isActive ? 'font-medium' : ''}>{sp.label}</span>
              </div>
            </React.Fragment>
          )
        })}
      </div>

      {/* Log tail */}
      <div className="space-y-1">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2">
            <span className="text-[10px] text-gray-500 uppercase">策略日志</span>
            {/* Filter tabs */}
            {(['all', 'warn', 'error'] as const).map(f => {
              const label = f === 'all' ? `全部 ${logs.length}` : f === 'warn' ? `⚠ 警告 ${warnCount}` : `✕ 错误 ${errorCount}`
              const active = logFilter === f
              const hasItems = f === 'all' || (f === 'warn' && warnCount > 0) || (f === 'error' && errorCount > 0)
              return (
                <button key={f} onClick={() => setLogFilter(f)}
                  className={`text-[10px] px-1.5 py-0.5 rounded transition ${
                    active ? (f === 'error' ? 'bg-red-500/20 text-red-400' : f === 'warn' ? 'bg-yellow-500/20 text-yellow-400' : 'bg-gray-700 text-gray-300') :
                    hasItems ? 'text-gray-500 hover:text-gray-300' : 'text-gray-700 cursor-default'
                  }`}>{label}</button>
              )
            })}
          </div>
          <button onClick={() => setShowAll(!showAll)}
            className="text-[10px] text-gray-500 hover:text-gray-300 transition">
            {showAll ? `最近20条` : `全部${filteredLogs.length}条`}
          </button>
        </div>
        {/* Error summary banner */}
        {errorCount > 0 && logFilter === 'all' && (
          <div className="bg-red-500/10 border border-red-500/20 rounded-lg px-3 py-1.5 flex items-center gap-2 text-xs">
            <span className="text-red-400 font-medium">✕ {errorCount} 个错误</span>
            <span className="text-gray-500">·</span>
            <span className="text-gray-400 truncate">{logs.filter((l: any) => l.level === 'ERROR').slice(-1)[0]?.msg}</span>
            <button onClick={() => setLogFilter('error')} className="ml-auto text-red-400 hover:text-red-300 shrink-0">查看</button>
          </div>
        )}
        <div className="bg-gray-950 border border-gray-800 rounded-lg p-2 max-h-64 overflow-y-auto font-mono text-[11px] space-y-px">
          {visibleLogs.length === 0 ? (
            <div className="text-gray-600 text-center py-4">暂无策略日志</div>
          ) : visibleLogs.map((log: any, i: number) => (
            <div key={i} className="flex gap-2 hover:bg-gray-900/50 px-1 rounded">
              <span className="text-gray-600 shrink-0">{log.time}</span>
              <span className={`shrink-0 w-4 text-center ${levelColor(log.level)}`}>
                {log.level === 'WARNING' ? '⚠' : log.level === 'ERROR' ? '✕' : '·'}
              </span>
              <span className={highlightMsg(log.msg)}>{log.msg}</span>
            </div>
          ))}
        </div>
      </div>
    </div>
  )
}

/* ═══════════════ POSITIONS + CLOSED TRADES ═══════════════ */
function PositionTable({ positions, research, closedTrades, reviewData }: { positions: any[]; research: Record<string, any>; closedTrades: any[]; reviewData: any }) {
  const [expanded, setExpanded] = useState<Record<string, boolean>>({})
  const [tab, setTab] = useState<'holding' | 'closed'>('holding')
  const [closedSub, setClosedSub] = useState<'list' | 'review' | 'suggest'>('list')
  const toggle = (code: string) => setExpanded(prev => ({ ...prev, [code]: !prev[code] }))

  return (
    <div>
      <div className="flex items-center gap-2 mb-3">
        <Crosshair className="w-4 h-4 text-blue-400" />
        <span className="text-sm font-medium text-gray-300">持仓分析</span>
        <div className="flex gap-1 ml-3">
          <button onClick={() => setTab('holding')}
            className={`px-3 py-1 text-xs rounded-md transition ${tab === 'holding' ? 'bg-blue-600 text-white' : 'bg-gray-800 text-gray-400 hover:text-white'}`}>
            持仓中 ({positions.length})
          </button>
          <button onClick={() => setTab('closed')}
            className={`px-3 py-1 text-xs rounded-md transition ${tab === 'closed' ? 'bg-blue-600 text-white' : 'bg-gray-800 text-gray-400 hover:text-white'}`}>
            <History className="w-3 h-3 inline mr-1" />已卖出 ({closedTrades.length})
          </button>
        </div>
        {tab === 'closed' && (
          <div className="flex gap-1 ml-3">
            <button onClick={() => setClosedSub('list')}
              className={`px-2.5 py-1 text-[11px] rounded transition ${closedSub === 'list' ? 'bg-purple-600 text-white' : 'bg-gray-800 text-gray-500 hover:text-white'}`}>
              <FileText className="w-3 h-3 inline mr-0.5" />交割单
            </button>
            <button onClick={() => setClosedSub('review')}
              className={`px-2.5 py-1 text-[11px] rounded transition ${closedSub === 'review' ? 'bg-purple-600 text-white' : 'bg-gray-800 text-gray-500 hover:text-white'}`}>
              <BarChart2 className="w-3 h-3 inline mr-0.5" />复盘分析
            </button>
            <button onClick={() => setClosedSub('suggest')}
              className={`px-2.5 py-1 text-[11px] rounded transition ${closedSub === 'suggest' ? 'bg-purple-600 text-white' : 'bg-gray-800 text-gray-500 hover:text-white'}`}>
              <Lightbulb className="w-3 h-3 inline mr-0.5" />策略反思
            </button>
          </div>
        )}
        <span className="text-[10px] text-gray-600 ml-auto">
          {tab === 'holding' ? 'L4 个股研报 · 点击行展开详情' : closedSub === 'list' ? '交割单明细' : closedSub === 'review' ? '复盘数据分析' : '策略改进建议'}
        </span>
      </div>

      {tab === 'holding' ? (
      <div className="bg-gray-900 border border-gray-800 rounded-xl overflow-x-auto">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-gray-800 text-gray-500 text-xs">
              <th className="text-left px-4 py-3 font-medium">代码</th>
              <th className="text-left px-3 py-3 font-medium">名称</th>
              <th className="text-right px-3 py-3 font-medium">持仓</th>
              <th className="text-right px-3 py-3 font-medium">可用</th>
              <th className="text-right px-3 py-3 font-medium">成本</th>
              <th className="text-right px-3 py-3 font-medium">现价</th>
              <th className="text-right px-3 py-3 font-medium">市值</th>
              <th className="text-right px-3 py-3 font-medium">盈亏%</th>
              <th className="text-center px-3 py-3 font-medium">评分</th>
              <th className="text-center px-3 py-3 font-medium">趋势</th>
              <th className="text-center px-3 py-3 font-medium">生命周期</th>
              <th className="text-center px-3 py-3 font-medium">建议</th>
              <th className="text-left px-3 py-3 font-medium">买入理由</th>
            </tr>
          </thead>
          <tbody>
            {positions.map((p: any) => {
              const r = research[p.code]
              const cost = p.cost_price || 0
              const cur = p.market_price || cost
              const pnl = cost > 0 ? (cur - cost) / cost * 100 : 0
              const score = r?.composite_score
              const trend = r?.trend?.status
              const rec = r?.recommendation
              const lifecycle = r?.lifecycle
              const isOpen = expanded[p.code]
              return (
                <React.Fragment key={p.code}>
                <tr onClick={() => toggle(p.code)}
                    className={`border-b border-gray-800/40 hover:bg-gray-800/30 transition cursor-pointer ${isOpen ? 'bg-gray-800/20' : ''}`}>
                  <td className="px-4 py-3 font-mono text-white text-xs">
                    <span className={`inline-block w-3 text-gray-600 mr-1 transition-transform ${isOpen ? 'rotate-90' : ''}`}>▸</span>
                    {p.code}
                  </td>
                  <td className="px-3 py-3 text-gray-300 truncate max-w-[80px]">{p.name || '—'}</td>
                  <td className="px-3 py-3 text-right text-gray-300">{p.volume}</td>
                  <td className={`px-3 py-3 text-right ${(p.avail_volume ?? p.volume) >= p.volume ? 'text-gray-400' : 'text-yellow-500'}`}>
                    {p.avail_volume ?? p.volume}
                  </td>
                  <td className="px-3 py-3 text-right text-gray-500">{cost.toFixed(2)}</td>
                  <td className="px-3 py-3 text-right text-white">{cur.toFixed(2)}</td>
                  <td className="px-3 py-3 text-right text-gray-300 text-xs">{fmtMoney(cur * p.volume)}</td>
                  <td className={`px-3 py-3 text-right font-medium ${pnlClass(pnl)}`}>{pnlStr(pnl)}</td>
                  <td className="px-3 py-3 text-center">
                    {score != null ? (
                      <span className={`font-bold ${score >= 70 ? 'text-red-400' : score >= 50 ? 'text-yellow-400' : 'text-green-400'}`}>
                        {score.toFixed(1)}
                      </span>
                    ) : <span className="text-gray-600">—</span>}
                  </td>
                  <td className="px-3 py-3 text-center">
                    <span className="inline-flex items-center gap-1">
                      {trendIcon(trend)}
                      <span className="text-xs text-gray-400">{TREND_CN[trend || ''] || trend || '—'}</span>
                    </span>
                  </td>
                  <td className="px-3 py-3 text-center text-xs text-gray-400">{LIFECYCLE_CN[lifecycle || ''] || lifecycle || '—'}</td>
                  <td className="px-3 py-3 text-center"><RecBadge rec={rec} /></td>
                  <td className="px-3 py-3 text-left text-[11px] text-gray-400 max-w-[160px] truncate" title={p.entry_reason || ''}>
                    {p.entry_reason ? (
                      <span className="flex items-center gap-1">
                        <FileText className="w-3 h-3 text-blue-400 shrink-0" />
                        {p.entry_reason}
                      </span>
                    ) : <span className="text-gray-700">—</span>}
                  </td>
                </tr>
                {isOpen && r && (
                  <tr className="bg-gray-800/40">
                    <td colSpan={13} className="px-5 py-4">
                      <StockDetail r={r} code={p.code} entryReason={p.entry_reason} entryTime={p.entry_time} />
                    </td>
                  </tr>
                )}
                </React.Fragment>
              )
            })}
            {positions.length === 0 && (
              <tr><td colSpan={13} className="px-4 py-8 text-center text-gray-600">暂无持仓数据</td></tr>
            )}
          </tbody>
        </table>
      </div>
      ) : closedSub === 'list' ? (
      <ClosedTradesTable trades={closedTrades} />
      ) : closedSub === 'review' ? (
      <ReviewPanel data={reviewData} />
      ) : (
      <SuggestionsPanel data={reviewData} />
      )}
    </div>
  )
}

/* ═══════════════ CLOSED TRADES TABLE (交割单) ═══════════════ */
function ClosedTradesTable({ trades }: { trades: any[] }) {
  if (trades.length === 0) {
    return (
      <div className="bg-gray-900 border border-gray-800 rounded-xl p-8 text-center text-gray-600">
        暂无已完成的交易记录
      </div>
    )
  }
  const totalPnl = trades.reduce((s, t) => s + (t.pnl_amount || 0), 0)
  const winCount = trades.filter(t => (t.pnl_pct || 0) > 0).length
  const winRate = trades.length > 0 ? (winCount / trades.length * 100).toFixed(0) : '0'

  return (
    <div className="bg-gray-900 border border-gray-800 rounded-xl overflow-x-auto">
      <div className="px-4 py-2 border-b border-gray-800 flex items-center gap-4 text-xs">
        <span className="text-gray-400">共 <strong className="text-white">{trades.length}</strong> 笔</span>
        <span className="text-gray-400">胜率 <strong className={Number(winRate) >= 50 ? 'text-red-400' : 'text-green-400'}>{winRate}%</strong></span>
        <span className="text-gray-400">总盈亏 <strong className={totalPnl >= 0 ? 'text-red-400' : 'text-green-400'}>¥{fmtMoney(totalPnl)}</strong></span>
      </div>
      <table className="w-full text-sm">
        <thead>
          <tr className="border-b border-gray-800 text-gray-500 text-xs">
            <th className="text-left px-4 py-2 font-medium">代码</th>
            <th className="text-left px-3 py-2 font-medium">名称</th>
            <th className="text-center px-2 py-2 font-medium">策略</th>
            <th className="text-right px-3 py-2 font-medium">买入价</th>
            <th className="text-right px-3 py-2 font-medium">卖出价</th>
            <th className="text-right px-3 py-2 font-medium">数量</th>
            <th className="text-right px-3 py-2 font-medium">盈亏%</th>
            <th className="text-right px-3 py-2 font-medium">盈亏额</th>
            <th className="text-center px-3 py-2 font-medium">持仓天数</th>
            <th className="text-left px-3 py-2 font-medium">买入理由</th>
            <th className="text-left px-3 py-2 font-medium">卖出理由</th>
            <th className="text-left px-3 py-2 font-medium">卖出时间</th>
          </tr>
        </thead>
        <tbody>
          {trades.map((t: any, i: number) => (
            <tr key={i} className="border-b border-gray-800/40 hover:bg-gray-800/30 transition">
              <td className="px-4 py-2.5 font-mono text-white text-xs">{t.code}</td>
              <td className="px-3 py-2.5 text-gray-300 text-xs truncate max-w-[80px]">{t.name || '—'}</td>
              <td className="px-2 py-2.5 text-center">
                {t.strategy_type ? (
                  <span className={`px-1.5 py-0.5 text-[9px] rounded font-medium ${
                    t.strategy_type === 'trend' ? 'bg-cyan-500/20 text-cyan-400' :
                    t.strategy_type === 'breakout' ? 'bg-amber-500/20 text-amber-400' :
                    t.strategy_type === 'volume' ? 'bg-purple-500/20 text-purple-400' :
                    'bg-gray-700/50 text-gray-400'
                  }`}>{t.strategy_type}</span>
                ) : <span className="text-gray-700 text-[9px]">—</span>}
              </td>
              <td className="px-3 py-2.5 text-right text-xs">
                {t.entry_price > 0 ? <span className="text-gray-500">{t.entry_price.toFixed(2)}</span> : <span className="text-gray-700">未知</span>}
              </td>
              <td className="px-3 py-2.5 text-right text-white text-xs">{(t.exit_price || 0).toFixed(2)}</td>
              <td className="px-3 py-2.5 text-right text-gray-400 text-xs">{t.volume}</td>
              <td className={`px-3 py-2.5 text-right font-medium text-xs ${t.entry_price > 0 ? pnlClass(t.pnl_pct || 0) : 'text-gray-700'}`}>
                {t.entry_price > 0 ? pnlStr(t.pnl_pct || 0) : '—'}
              </td>
              <td className={`px-3 py-2.5 text-right text-xs ${t.entry_price > 0 ? ((t.pnl_amount || 0) >= 0 ? 'text-red-400' : 'text-green-400') : 'text-gray-700'}`}>
                {t.entry_price > 0 ? `¥${(t.pnl_amount || 0).toFixed(0)}` : '—'}
              </td>
              <td className="px-3 py-2.5 text-center text-xs text-gray-400">{t.holding_days || 0}天</td>
              <td className="px-3 py-2.5 text-left text-[11px] text-blue-400 max-w-[140px] truncate" title={t.entry_reason || ''}>
                {t.entry_reason || '—'}
              </td>
              <td className="px-3 py-2.5 text-left text-[11px] max-w-[160px] truncate" title={t.exit_reason || ''}
                  style={{ color: (t.exit_reason || '').includes('stop_loss') ? '#f87171' : (t.exit_reason || '').includes('take_profit') || (t.exit_reason || '').includes('staged') ? '#34d399' : '#9ca3af' }}>
                {t.exit_reason || '—'}
              </td>
              <td className="px-3 py-2.5 text-left text-[11px] text-gray-500">
                {t.exit_time ? t.exit_time.slice(5, 16) : '—'}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}

/* ═══════════════ REVIEW PANEL (复盘分析) ═══════════════ */
function ReviewPanel({ data }: { data: any }) {
  if (!data || !data.stats) {
    return <div className="bg-gray-900 border border-gray-800 rounded-xl p-8 text-center text-gray-600">暂无复盘数据</div>
  }
  const s = data.stats
  const StatBox = ({ label, value, color = 'text-white' }: { label: string; value: string | number; color?: string }) => (
    <div className="bg-gray-800/60 rounded-lg px-4 py-3 text-center">
      <div className="text-[11px] text-gray-500 mb-1">{label}</div>
      <div className={`text-lg font-bold ${color}`}>{value}</div>
    </div>
  )

  return (
    <div className="space-y-4">
      {/* KPI Cards */}
      <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-5 gap-3">
        <StatBox label="总交易" value={s.total_trades} />
        <StatBox label="胜率" value={`${(s.win_rate * 100).toFixed(0)}%`}
          color={s.win_rate >= 0.5 ? 'text-red-400' : 'text-green-400'} />
        <StatBox label="盈亏比" value={s.profit_factor?.toFixed(2)}
          color={s.profit_factor >= 1.5 ? 'text-red-400' : s.profit_factor >= 1 ? 'text-yellow-400' : 'text-green-400'} />
        <StatBox label="期望值" value={`${s.expectancy_pct >= 0 ? '+' : ''}${s.expectancy_pct?.toFixed(2)}%`}
          color={s.expectancy_pct >= 0 ? 'text-red-400' : 'text-green-400'} />
        <StatBox label="总盈亏" value={`¥${fmtMoney(s.total_pnl || 0)}`}
          color={(s.total_pnl || 0) >= 0 ? 'text-red-400' : 'text-green-400'} />
      </div>
      <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
        <StatBox label="平均盈利" value={`${s.avg_win_pct >= 0 ? '+' : ''}${s.avg_win_pct?.toFixed(2)}%`} color="text-red-400" />
        <StatBox label="平均亏损" value={`${s.avg_loss_pct?.toFixed(2)}%`} color="text-green-400" />
        <StatBox label="平均持仓" value={`${s.avg_holding_days?.toFixed(1)}天`} />
        <StatBox label="最大连亏" value={`${s.max_consecutive_losses}笔`}
          color={s.max_consecutive_losses >= 4 ? 'text-red-400' : 'text-gray-300'} />
      </div>

      <div className="grid md:grid-cols-2 gap-4">
        {/* Exit Reason Attribution */}
        <div className="bg-gray-900 border border-gray-800 rounded-xl p-4">
          <div className="text-xs font-medium text-purple-400 mb-3 flex items-center gap-1">
            <Target className="w-3 h-3" /> 卖出理由归因
          </div>
          <table className="w-full text-xs">
            <thead>
              <tr className="text-gray-500 border-b border-gray-800">
                <th className="text-left py-1.5 font-medium">理由</th>
                <th className="text-right py-1.5 font-medium">笔数</th>
                <th className="text-right py-1.5 font-medium">胜率</th>
                <th className="text-right py-1.5 font-medium">均盈亏</th>
              </tr>
            </thead>
            <tbody>
              {(data.attribution || []).map((a: any, i: number) => (
                <tr key={i} className="border-b border-gray-800/30">
                  <td className="py-1.5 text-gray-300">{a.exit_reason}</td>
                  <td className="py-1.5 text-right text-gray-400">{a.count}</td>
                  <td className={`py-1.5 text-right ${a.win_rate >= 0.5 ? 'text-red-400' : 'text-green-400'}`}>
                    {(a.win_rate * 100).toFixed(0)}%
                  </td>
                  <td className={`py-1.5 text-right font-medium ${a.avg_pnl_pct >= 0 ? 'text-red-400' : 'text-green-400'}`}>
                    {a.avg_pnl_pct >= 0 ? '+' : ''}{a.avg_pnl_pct?.toFixed(2)}%
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>

        {/* Strategy Type Attribution (Phase 11) */}
        <div className="bg-gray-900 border border-gray-800 rounded-xl p-4">
          <div className="text-xs font-medium text-cyan-400 mb-3 flex items-center gap-1">
            <Layers className="w-3 h-3" /> 策略类型归因
          </div>
          {(data.strategy_type_attr || []).length > 0 ? (
            <div className="space-y-2">
              {(data.strategy_type_attr || []).map((st: any, i: number) => {
                const colors: Record<string, string> = { trend: 'bg-cyan-500', breakout: 'bg-amber-500', volume: 'bg-purple-500' }
                const labels: Record<string, string> = { trend: '趋势', breakout: '突破', volume: '量能', other: '其他', unknown: '未知' }
                const barColor = colors[st.strategy_type] || 'bg-gray-500'
                return (
                  <div key={i} className="flex items-center gap-2 text-xs">
                    <span className="w-10 text-right text-gray-400 shrink-0">{labels[st.strategy_type] || st.strategy_type}</span>
                    <div className="flex-1 h-4 bg-gray-800 rounded overflow-hidden relative">
                      <div className={`h-full rounded ${barColor}/60`}
                        style={{ width: `${Math.min(100, st.win_rate * 100)}%` }} />
                      <span className="absolute inset-0 flex items-center justify-center text-[10px] text-white/80">
                        {(st.win_rate * 100).toFixed(0)}%
                      </span>
                    </div>
                    <span className="w-8 text-right text-gray-500">{st.count}笔</span>
                    <span className={`w-14 text-right font-medium ${st.avg_pnl_pct >= 0 ? 'text-red-400' : 'text-green-400'}`}>
                      {st.avg_pnl_pct >= 0 ? '+' : ''}{st.avg_pnl_pct?.toFixed(1)}%
                    </span>
                  </div>
                )
              })}
            </div>
          ) : (
            <div className="text-gray-600 text-xs text-center py-4">暂无策略分类数据</div>
          )}
        </div>
      </div>

      <div className="grid md:grid-cols-2 gap-4">
        {/* Holding Period Distribution */}
        <div className="bg-gray-900 border border-gray-800 rounded-xl p-4">
          <div className="text-xs font-medium text-blue-400 mb-3 flex items-center gap-1">
            <History className="w-3 h-3" /> 持仓周期分布
          </div>
          <table className="w-full text-xs">
            <thead>
              <tr className="text-gray-500 border-b border-gray-800">
                <th className="text-left py-1.5 font-medium">周期</th>
                <th className="text-right py-1.5 font-medium">笔数</th>
                <th className="text-right py-1.5 font-medium">胜率</th>
                <th className="text-right py-1.5 font-medium">均盈亏</th>
              </tr>
            </thead>
            <tbody>
              {(data.holding_dist || []).map((h: any, i: number) => (
                <tr key={i} className="border-b border-gray-800/30">
                  <td className="py-1.5 text-gray-300">{h.period}</td>
                  <td className="py-1.5 text-right text-gray-400">{h.count}</td>
                  <td className={`py-1.5 text-right ${h.win_rate >= 0.5 ? 'text-red-400' : 'text-green-400'}`}>
                    {(h.win_rate * 100).toFixed(0)}%
                  </td>
                  <td className={`py-1.5 text-right font-medium ${h.avg_pnl >= 0 ? 'text-red-400' : 'text-green-400'}`}>
                    {h.avg_pnl >= 0 ? '+' : ''}{h.avg_pnl?.toFixed(2)}%
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>

      <div className="grid md:grid-cols-2 gap-4">
        {/* P&L Distribution */}
        <div className="bg-gray-900 border border-gray-800 rounded-xl p-4">
          <div className="text-xs font-medium text-amber-400 mb-3 flex items-center gap-1">
            <BarChart2 className="w-3 h-3" /> 盈亏分布
          </div>
          <div className="space-y-1.5">
            {(data.pnl_dist || []).map((p: any, i: number) => {
              const maxPct = Math.max(...(data.pnl_dist || []).map((x: any) => x.pct))
              const barW = maxPct > 0 ? (p.pct / maxPct * 100) : 0
              const isLoss = p.range.startsWith('亏')
              return (
                <div key={i} className="flex items-center gap-2 text-xs">
                  <span className="w-16 text-right text-gray-400 shrink-0">{p.range}</span>
                  <div className="flex-1 h-5 bg-gray-800 rounded overflow-hidden">
                    <div className={`h-full rounded ${isLoss ? 'bg-green-600/60' : 'bg-red-600/60'}`}
                      style={{ width: `${barW}%` }} />
                  </div>
                  <span className="w-10 text-right text-gray-400">{p.count}笔</span>
                  <span className="w-12 text-right text-gray-500">{p.pct}%</span>
                </div>
              )
            })}
          </div>
        </div>

        {/* Weekday Distribution */}
        <div className="bg-gray-900 border border-gray-800 rounded-xl p-4">
          <div className="text-xs font-medium text-cyan-400 mb-3 flex items-center gap-1">
            <Activity className="w-3 h-3" /> 星期分布
          </div>
          <table className="w-full text-xs">
            <thead>
              <tr className="text-gray-500 border-b border-gray-800">
                <th className="text-left py-1.5 font-medium">星期</th>
                <th className="text-right py-1.5 font-medium">笔数</th>
                <th className="text-right py-1.5 font-medium">胜率</th>
                <th className="text-right py-1.5 font-medium">均盈亏</th>
              </tr>
            </thead>
            <tbody>
              {(data.time_dist || []).map((td: any, i: number) => (
                <tr key={i} className="border-b border-gray-800/30">
                  <td className="py-1.5 text-gray-300">{td.period}</td>
                  <td className="py-1.5 text-right text-gray-400">{td.count}</td>
                  <td className={`py-1.5 text-right ${td.win_rate >= 0.5 ? 'text-red-400' : 'text-green-400'}`}>
                    {(td.win_rate * 100).toFixed(0)}%
                  </td>
                  <td className={`py-1.5 text-right font-medium ${td.avg_pnl >= 0 ? 'text-red-400' : 'text-green-400'}`}>
                    {td.avg_pnl >= 0 ? '+' : ''}{td.avg_pnl?.toFixed(2)}%
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>

      {/* Top Winners & Losers */}
      <div className="grid md:grid-cols-2 gap-4">
        <div className="bg-gray-900 border border-gray-800 rounded-xl p-4">
          <div className="text-xs font-medium text-red-400 mb-2">Top 盈利</div>
          {(data.top_winners || []).map((t: any, i: number) => (
            <div key={i} className="flex items-center justify-between text-xs py-1 border-b border-gray-800/30">
              <span className="text-gray-300 font-mono">{t.code}</span>
              <span className="text-gray-500">{t.name}</span>
              <span className="text-red-400 font-medium">+{t.pnl_pct?.toFixed(2)}%</span>
              <span className="text-red-400/70">¥{(t.pnl_amount || 0).toFixed(0)}</span>
            </div>
          ))}
        </div>
        <div className="bg-gray-900 border border-gray-800 rounded-xl p-4">
          <div className="text-xs font-medium text-green-400 mb-2">Top 亏损</div>
          {(data.top_losers || []).map((t: any, i: number) => (
            <div key={i} className="flex items-center justify-between text-xs py-1 border-b border-gray-800/30">
              <span className="text-gray-300 font-mono">{t.code}</span>
              <span className="text-gray-500">{t.name}</span>
              <span className="text-green-400 font-medium">{t.pnl_pct?.toFixed(2)}%</span>
              <span className="text-green-400/70">¥{(t.pnl_amount || 0).toFixed(0)}</span>
            </div>
          ))}
        </div>
      </div>
    </div>
  )
}

/* ═══════════════ SUGGESTIONS PANEL (策略反思) ═══════════════ */
function SuggestionsPanel({ data }: { data: any }) {
  if (!data || !data.suggestions) {
    return <div className="bg-gray-900 border border-gray-800 rounded-xl p-8 text-center text-gray-600">暂无策略建议数据</div>
  }
  const s = data.stats || {}

  return (
    <div className="space-y-4">
      {/* Quick Summary */}
      <div className="bg-gray-900 border border-gray-800 rounded-xl p-5">
        <div className="text-sm font-medium text-amber-400 mb-3 flex items-center gap-2">
          <Lightbulb className="w-4 h-4" /> 策略健康度
        </div>
        <div className="grid grid-cols-3 gap-4 mb-4">
          <div className="text-center">
            <div className="text-xs text-gray-500 mb-1">胜率</div>
            <div className={`text-2xl font-bold ${(s.win_rate || 0) >= 0.5 ? 'text-red-400' : (s.win_rate || 0) >= 0.4 ? 'text-yellow-400' : 'text-green-400'}`}>
              {((s.win_rate || 0) * 100).toFixed(0)}%
            </div>
            <div className="text-[10px] text-gray-600 mt-0.5">{(s.win_rate || 0) >= 0.5 ? '良好' : (s.win_rate || 0) >= 0.4 ? '一般' : '需改进'}</div>
          </div>
          <div className="text-center">
            <div className="text-xs text-gray-500 mb-1">盈亏比</div>
            <div className={`text-2xl font-bold ${(s.profit_factor || 0) >= 1.5 ? 'text-red-400' : (s.profit_factor || 0) >= 1 ? 'text-yellow-400' : 'text-green-400'}`}>
              {(s.profit_factor || 0).toFixed(2)}
            </div>
            <div className="text-[10px] text-gray-600 mt-0.5">{(s.profit_factor || 0) >= 1.5 ? '优秀' : (s.profit_factor || 0) >= 1 ? '及格' : '亏损'}</div>
          </div>
          <div className="text-center">
            <div className="text-xs text-gray-500 mb-1">期望值</div>
            <div className={`text-2xl font-bold ${(s.expectancy_pct || 0) > 0 ? 'text-red-400' : 'text-green-400'}`}>
              {(s.expectancy_pct || 0) >= 0 ? '+' : ''}{(s.expectancy_pct || 0).toFixed(2)}%
            </div>
            <div className="text-[10px] text-gray-600 mt-0.5">{(s.expectancy_pct || 0) > 0.5 ? '正期望' : (s.expectancy_pct || 0) > 0 ? '微正' : '负期望'}</div>
          </div>
        </div>
      </div>

      {/* Suggestions List */}
      <div className="bg-gray-900 border border-gray-800 rounded-xl p-5">
        <div className="text-sm font-medium text-purple-400 mb-4 flex items-center gap-2">
          <Brain className="w-4 h-4" /> 策略改进建议 ({data.suggestions.length}条)
        </div>
        <div className="space-y-3">
          {data.suggestions.map((sug: string, i: number) => (
            <div key={i} className="bg-gray-800/50 border border-gray-700/50 rounded-lg px-4 py-3 text-sm text-gray-200 leading-relaxed">
              {sug}
            </div>
          ))}
        </div>
      </div>

      {/* Action Items */}
      <div className="bg-gray-900 border border-amber-900/30 rounded-xl p-5">
        <div className="text-sm font-medium text-amber-400 mb-3">下一步行动</div>
        <div className="text-xs text-gray-400 space-y-2">
          <div className="flex items-start gap-2">
            <span className="text-amber-500 mt-0.5">1.</span>
            <span>根据上述建议调整策略参数（止损/止盈/持仓时间）</span>
          </div>
          <div className="flex items-start gap-2">
            <span className="text-amber-500 mt-0.5">2.</span>
            <span>用调整后的参数运行回测验证改进效果</span>
          </div>
          <div className="flex items-start gap-2">
            <span className="text-amber-500 mt-0.5">3.</span>
            <span>小仓位实盘验证2周后再对比复盘数据</span>
          </div>
        </div>
      </div>
    </div>
  )
}

/* ═══════════════ STOCK DETAIL (展开面板) ═══════════════ */
function StockDetail({ r, code, entryReason, entryTime }: { r: any; code: string; entryReason?: string; entryTime?: string }) {
  const t = r.trend || {}
  const rs = r.relative_strength || {}
  const vol = r.volume || {}
  const chip = r.chip || {}
  const risk = r.risk || {}
  const pos = r.position || {}
  const layers = r.layers || {}

  const DetailItem = ({ label, value, color }: { label: string; value: any; color?: string }) => (
    <div className="flex justify-between items-center py-0.5">
      <span className="text-gray-500 text-[11px]">{label}</span>
      <span className={`text-[11px] font-medium ${color || 'text-gray-300'}`}>{value ?? '—'}</span>
    </div>
  )

  const tgt = r.targets || {}
  const isHeld = pos.cost_price > 0
  const hTrendColor = tgt.hourly_trend === 'danger' ? 'text-red-500' : tgt.hourly_trend === 'bearish' ? 'text-green-400' : 'text-red-400'
  const hTrendLabel = tgt.hourly_trend === 'danger' ? '⚠ M5<M34 清仓' : tgt.hourly_trend === 'bearish' ? '↓ M5<M8 减仓' : tgt.hourly_trend === 'bullish' ? '↑ M5>M8 持有' : '—'

  return (
    <div className="grid grid-cols-2 md:grid-cols-5 gap-4 text-xs">
      {/* 买卖点 */}
      <div className="space-y-1">
        <div className="text-rose-400 font-medium mb-1.5 flex items-center gap-1">
          <Crosshair className="w-3 h-3" /> 买卖点
        </div>
        {entryReason && (
          <div className="bg-blue-500/10 border border-blue-500/20 rounded px-2 py-1.5 mb-2">
            <div className="text-[10px] text-blue-400 mb-0.5">买入理由</div>
            <div className="text-[11px] text-gray-200 leading-snug">{entryReason}</div>
            {entryTime && <div className="text-[10px] text-gray-500 mt-0.5">{entryTime}</div>}
          </div>
        )}
        {isHeld ? (<>
          <DetailItem label="止损价" value={tgt.stop_loss?.toFixed(2)} color="text-green-400" />
          <DetailItem label="止损幅度" value={tgt.stop_loss_pct != null ? `-${tgt.stop_loss_pct}%` : null} color="text-green-400" />
          <DetailItem label="止盈1 (+8%)" value={tgt.tp1?.toFixed(2)} color="text-red-400" />
          <DetailItem label="止盈2 (+15%)" value={tgt.tp2?.toFixed(2)} color="text-red-400" />
          <DetailItem label="保本价" value={tgt.breakeven?.toFixed(2)} color="text-yellow-400" />
        </>) : (<>
          <DetailItem label="入场区间" value={tgt.entry_zone_low && tgt.entry_zone_high ? `${tgt.entry_zone_low} ~ ${tgt.entry_zone_high}` : null} color="text-blue-400" />
          <DetailItem label="建议止损" value={tgt.stop_loss_if_buy?.toFixed(2)} color="text-green-400" />
          <DetailItem label="止损幅度" value={tgt.stop_loss_pct != null ? `-${tgt.stop_loss_pct}%` : null} color="text-green-400" />
        </>)}
        <DetailItem label="MA20支撑" value={tgt.ma_support?.toFixed(2)} color="text-blue-300" />
        {tgt.ma_resistance && <DetailItem label="MA60压力" value={tgt.ma_resistance.toFixed(2)} color="text-yellow-300" />}
        <div className="mt-1 pt-1 border-t border-gray-700/50">
          <DetailItem label="60分趋势" value={hTrendLabel} color={hTrendColor} />
          {tgt.hourly_ma5 && <DetailItem label="H-MA5/8" value={`${tgt.hourly_ma5} / ${tgt.hourly_ma8}`} />}
          {tgt.hourly_ma34 && <DetailItem label="H-MA34" value={tgt.hourly_ma34?.toFixed(2)} />}
        </div>
      </div>

      {/* 趋势分析 */}
      <div className="space-y-1">
        <div className="text-blue-400 font-medium mb-1.5 flex items-center gap-1">
          <TrendingUp className="w-3 h-3" /> 趋势分析
        </div>
        <DetailItem label="MA5" value={t.ma5?.toFixed(2)} />
        <DetailItem label="MA8" value={t.ma8?.toFixed(2)} />
        <DetailItem label="MA34" value={t.ma34?.toFixed(2)} />
        <DetailItem label="MA82" value={t.ma82?.toFixed(2)} />
        <DetailItem label="站上MA34" value={t.above_ma34 != null ? (t.above_ma34 ? '✓ 是' : '✗ 否') : null}
          color={t.above_ma34 ? 'text-red-400' : 'text-green-400'} />
        <DetailItem label="站上MA82" value={t.above_ma82 != null ? (t.above_ma82 ? '✓ 是' : '✗ 否') : null}
          color={t.above_ma82 ? 'text-red-400' : 'text-green-400'} />
        <DetailItem label="趋势分" value={t.score} />
      </div>

      {/* 相对强度 */}
      <div className="space-y-1">
        <div className="text-amber-400 font-medium mb-1.5 flex items-center gap-1">
          <Zap className="w-3 h-3" /> 相对强度
        </div>
        <DetailItem label="5日RS" value={rs.rs_5d != null ? `${rs.rs_5d > 0 ? '+' : ''}${rs.rs_5d.toFixed(2)}%` : null}
          color={(rs.rs_5d || 0) > 0 ? 'text-red-400' : 'text-green-400'} />
        <DetailItem label="20日RS" value={rs.rs_20d != null ? `${rs.rs_20d > 0 ? '+' : ''}${rs.rs_20d.toFixed(2)}%` : null}
          color={(rs.rs_20d || 0) > 0 ? 'text-red-400' : 'text-green-400'} />
        <DetailItem label="强于大盘" value={rs.stronger_than_index != null ? (rs.stronger_than_index ? '✓ 是' : '✗ 否') : null}
          color={rs.stronger_than_index ? 'text-red-400' : 'text-green-400'} />
        <DetailItem label="RS得分" value={rs.score?.toFixed(0)} />
      </div>

      {/* 量价特征 */}
      <div className="space-y-1">
        <div className="text-cyan-400 font-medium mb-1.5 flex items-center gap-1">
          <BarChart3 className="w-3 h-3" /> 量价特征
        </div>
        <DetailItem label="5日量比" value={vol.ratio_5d?.toFixed(2)} />
        <DetailItem label="20日量比" value={vol.ratio_20d?.toFixed(2)} />
        <DetailItem label="今日成交量" value={vol.today ? `${(vol.today / 10000).toFixed(0)}万` : null} />
        <DetailItem label="5日均量" value={vol.avg5 ? `${(vol.avg5 / 10000).toFixed(0)}万` : null} />
        <DetailItem label="放量" value={vol.expanding != null ? (vol.expanding ? '✓ 放量' : vol.shrinking ? '↓ 缩量' : '— 正常') : null}
          color={vol.expanding ? 'text-red-400' : vol.shrinking ? 'text-green-400' : 'text-yellow-400'} />
      </div>

      {/* 综合信息 */}
      <div className="space-y-1">
        <div className="text-emerald-400 font-medium mb-1.5 flex items-center gap-1">
          <Shield className="w-3 h-3" /> 综合信息
        </div>
        <DetailItem label="综合评分" value={r.composite_score?.toFixed(1)}
          color={r.composite_score >= 70 ? 'text-red-400' : r.composite_score >= 50 ? 'text-yellow-400' : 'text-green-400'} />
        <DetailItem label="生命周期" value={LIFECYCLE_CN[r.lifecycle || ''] || r.lifecycle} />
        <DetailItem label="筹码形态" value={chip.shape || chip.label || chip.type || (typeof chip === 'string' ? chip : null)} />
        {/* Phase 11: W/D/H Layer states */}
        {(layers.W || layers.D || layers.H) && (
          <div className="flex items-center gap-1.5 py-0.5">
            <span className="text-gray-500 text-[11px]">层级</span>
            <div className="flex gap-1">
              {['W', 'D', 'H'].map(k => {
                const v = layers[k]
                if (!v) return null
                const isRed = v === 'RED'
                return (
                  <span key={k} className={`px-1.5 py-0.5 text-[9px] rounded font-bold ${isRed ? 'bg-red-500/20 text-red-400' : 'bg-gray-700/50 text-gray-500'}`}>
                    {k}:{isRed ? '多' : '空'}
                  </span>
                )
              })}
            </div>
          </div>
        )}
        <DetailItem label="日波动率" value={risk.daily_volatility != null ? `${risk.daily_volatility}%` : null}
          color={risk.high_volatility ? 'text-yellow-400' : 'text-gray-300'} />
        <DetailItem label="20日回撤" value={risk.max_drawdown_20d != null ? `${risk.max_drawdown_20d}%` : null}
          color={(risk.max_drawdown_20d || 0) > 15 ? 'text-red-400' : 'text-gray-300'} />
        {pos.pnl_pct != null && (
          <DetailItem label="持仓盈亏" value={`${pos.pnl_pct > 0 ? '+' : ''}${pos.pnl_pct.toFixed(2)}%`}
            color={pos.pnl_pct >= 0 ? 'text-red-400' : 'text-green-400'} />
        )}
        {(r.risk_flags || []).length > 0 && (
          <div className="mt-1 flex flex-wrap gap-1">
            {r.risk_flags.map((f: string, i: number) => (
              <span key={i} className="px-1.5 py-0.5 bg-yellow-500/10 text-yellow-400 rounded text-[10px]">{f}</span>
            ))}
          </div>
        )}
      </div>
    </div>
  )
}

function RecBadge({ rec }: { rec?: string }) {
  const m: Record<string, string> = {
    buy: 'bg-red-500/20 text-red-400',
    sell: 'bg-green-500/20 text-green-400',
    hold: 'bg-gray-700 text-gray-300',
    reduce: 'bg-green-500/15 text-green-400',
    strong_buy: 'bg-red-600/30 text-red-300',
  }
  const label: Record<string, string> = {
    buy: '买入', sell: '卖出', hold: '持有', reduce: '减仓', strong_buy: '强烈买入',
  }
  if (!rec) return <span className="text-gray-600 text-xs">—</span>
  return (
    <span className={`text-xs px-2 py-0.5 rounded-full font-medium ${m[rec] || 'bg-gray-700 text-gray-400'}`}>
      {label[rec] || rec}
    </span>
  )
}

/* ═══════════════ EQUITY CURVE ═══════════════ */
const PERIODS = [
  { key: '1w', label: '周' },
  { key: '1m', label: '月' },
  { key: '3m', label: '季' },
  { key: '1y', label: '年' },
  { key: 'all', label: '全部' },
]

function EquityCurve({ data, period, onPeriod }: { data: any; period: string; onPeriod: (p: string) => void }) {
  const points = data?.points || []
  const summary = data?.summary || {}
  const hasData = points.length > 0

  const CustomTooltip = ({ active, payload, label }: any) => {
    if (!active || !payload?.length) return null
    const d = payload[0]?.payload
    if (!d) return null
    return (
      <div className="bg-gray-900 border border-gray-700 rounded-lg px-3 py-2 text-xs shadow-xl">
        <div className="text-gray-400 mb-1">{d.date}</div>
        <div className="text-white font-medium">总资产: ¥{fmtMoney(d.total)}</div>
        <div className={d.ret >= 0 ? 'text-red-400' : 'text-green-400'}>
          收益率: {d.ret >= 0 ? '+' : ''}{d.ret}%
        </div>
        {d.mv > 0 && <div className="text-gray-400">市值: ¥{fmtMoney(d.mv)}</div>}
      </div>
    )
  }

  const retColor = (summary.return_pct || 0) >= 0
  const minRet = Math.min(0, ...points.map((p: any) => p.ret))
  const maxRet = Math.max(0, ...points.map((p: any) => p.ret))

  return (
    <div className="bg-gray-900 border border-gray-800 rounded-xl p-5">
      <div className="flex items-center justify-between mb-4">
        <div className="flex items-center gap-2">
          <LineChartIcon className="w-4 h-4 text-blue-400" />
          <span className="text-sm font-medium text-gray-300">资产收益曲线</span>
          {hasData && (
            <span className={`text-sm font-bold ml-2 ${retColor ? 'text-red-400' : 'text-green-400'}`}>
              {summary.return_pct >= 0 ? '+' : ''}{summary.return_pct}%
            </span>
          )}
          {hasData && summary.max_drawdown > 0 && (
            <span className="text-[10px] text-gray-500 ml-2">
              最大回撤 {summary.max_drawdown}%
            </span>
          )}
        </div>
        <div className="flex gap-1">
          {PERIODS.map(p => (
            <button key={p.key} onClick={() => onPeriod(p.key)}
              className={`px-2.5 py-1 text-xs rounded-md transition ${
                period === p.key
                  ? 'bg-blue-600 text-white'
                  : 'bg-gray-800 text-gray-400 hover:text-white hover:bg-gray-700'
              }`}>
              {p.label}
            </button>
          ))}
        </div>
      </div>

      {!hasData ? (
        <div className="text-center text-gray-600 text-sm py-12">
          暂无历史数据 · 每交易日收盘后自动记录
        </div>
      ) : (
        <div className="h-[220px]">
          <ResponsiveContainer width="100%" height="100%">
            <AreaChart data={points} margin={{ top: 5, right: 10, left: 0, bottom: 0 }}>
              <defs>
                <linearGradient id="retGradUp" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="5%" stopColor="#ef4444" stopOpacity={0.3} />
                  <stop offset="95%" stopColor="#ef4444" stopOpacity={0} />
                </linearGradient>
                <linearGradient id="retGradDown" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="5%" stopColor="#22c55e" stopOpacity={0.3} />
                  <stop offset="95%" stopColor="#22c55e" stopOpacity={0} />
                </linearGradient>
              </defs>
              <CartesianGrid strokeDasharray="3 3" stroke="#1f2937" />
              <XAxis dataKey="date" tick={{ fontSize: 10, fill: '#6b7280' }} tickLine={false} axisLine={false}
                tickFormatter={(v: string) => v.slice(5)} interval="preserveStartEnd" />
              <YAxis tick={{ fontSize: 10, fill: '#6b7280' }} tickLine={false} axisLine={false}
                tickFormatter={(v: number) => `${v}%`}
                domain={[Math.floor(minRet - 1), Math.ceil(maxRet + 1)]} />
              <Tooltip content={<CustomTooltip />} />
              <ReferenceLine y={0} stroke="#374151" strokeDasharray="3 3" />
              <Area type="monotone" dataKey="ret" name="收益率"
                stroke={retColor ? '#ef4444' : '#22c55e'}
                fill={retColor ? 'url(#retGradUp)' : 'url(#retGradDown)'}
                strokeWidth={2} dot={false} activeDot={{ r: 4, strokeWidth: 0 }} />
            </AreaChart>
          </ResponsiveContainer>
        </div>
      )}

      {hasData && (
        <div className="flex items-center justify-between mt-3 text-[10px] text-gray-500">
          <span>{summary.start_date} ~ {summary.end_date}</span>
          <span>{summary.days}个交易日</span>
          <span>起始 ¥{fmtMoney(summary.start_val)} → 当前 ¥{fmtMoney(summary.end_val)}</span>
        </div>
      )}
    </div>
  )
}

/* ═══════════════ BRIEFING PANEL (盘前分析 + 日终复盘) ═══════════════ */
function BriefingPanel({ briefing, loading, onGenerate }: {
  briefing: any; loading: 'premarket' | 'eod' | null;
  onGenerate: (type: 'premarket' | 'eod') => void
}) {
  const [tab, setTab] = useState<'premarket' | 'eod'>('premarket')
  const current = briefing?.[tab]
  const content = current?.content || ''
  const isToday = current?.date === new Date().toISOString().slice(0, 10)

  const renderMarkdown = (text: string) => {
    return text.split('\n\n').map((block: string, i: number) => {
      const isHeading = block.startsWith('###') || block.startsWith('## ')
      if (isHeading) {
        const title = block.replace(/^#{1,4}\s*/, '').replace(/\*\*/g, '')
        return (
          <div key={i} className="text-sm font-semibold text-amber-400 mt-3 mb-1 flex items-center gap-1.5">
            <span className="w-1 h-4 bg-amber-500 rounded-full" />
            {title}
          </div>
        )
      }
      return (
        <div key={i} className="bg-gray-800/40 rounded-lg p-3 mb-2">
          {block.split('\n').map((line: string, j: number) => (
            <p key={j} className="mb-1 last:mb-0 text-sm text-gray-300 leading-relaxed" dangerouslySetInnerHTML={{
              __html: line
                .replace(/\*\*(.*?)\*\*/g, '<strong class="text-white">$1</strong>')
                .replace(/^- /, '<span class="text-amber-400 mr-1">•</span>')
                .replace(/^(\d+)\.\s/, '<span class="text-amber-400 mr-1">$1.</span>')
            }} />
          ))}
        </div>
      )
    })
  }

  return (
    <div className="bg-gray-900 border border-gray-800 rounded-xl p-5 space-y-3">
      <div className="flex items-center gap-2">
        <Sun className="w-4 h-4 text-amber-400" />
        <span className="text-sm font-medium text-gray-300">AI 盘前分析 & 日终复盘</span>
        <div className="flex gap-1 ml-3">
          <button onClick={() => setTab('premarket')}
            className={`flex items-center gap-1 px-3 py-1 text-xs rounded-md transition ${tab === 'premarket' ? 'bg-amber-600 text-white' : 'bg-gray-800 text-gray-400 hover:text-white'}`}>
            <Sun className="w-3 h-3" />盘前分析
          </button>
          <button onClick={() => setTab('eod')}
            className={`flex items-center gap-1 px-3 py-1 text-xs rounded-md transition ${tab === 'eod' ? 'bg-indigo-600 text-white' : 'bg-gray-800 text-gray-400 hover:text-white'}`}>
            <Moon className="w-3 h-3" />日终复盘
          </button>
        </div>
        <div className="ml-auto flex items-center gap-2">
          {current && (
            <span className="text-[10px] text-gray-600 flex items-center gap-1">
              <Clock className="w-3 h-3" />
              {current.date} {current.created_at?.slice(11, 16) || ''}
              {current.model && ` · ${current.model}`}
              {current.tokens_used > 0 && ` · ${current.tokens_used}t`}
            </span>
          )}
          <button onClick={() => onGenerate(tab)} disabled={loading !== null}
            className="flex items-center gap-1 px-2.5 py-1 text-[11px] rounded-md bg-gray-800 hover:bg-gray-700 text-gray-400 hover:text-white transition disabled:opacity-50">
            {loading === tab ? <Loader2 className="w-3 h-3 animate-spin" /> : <RefreshCw className="w-3 h-3" />}
            {loading === tab ? '生成中…' : isToday ? '重新生成' : '生成今日'}
          </button>
        </div>
      </div>

      {loading === tab && (
        <div className="flex items-center gap-2 text-sm text-gray-400 py-10 justify-center">
          <Loader2 className="w-5 h-5 animate-spin" />
          <span>{tab === 'premarket' ? 'Qwen 正在生成盘前分析…' : 'Qwen 正在生成日终复盘…'}（约20-40秒）</span>
        </div>
      )}

      {loading !== tab && content && (
        <div className="max-h-[500px] overflow-y-auto pr-1">
          {renderMarkdown(content)}
        </div>
      )}

      {loading !== tab && !content && (
        <div className="text-gray-600 text-sm py-8 text-center">
          {tab === 'premarket'
            ? '暂无盘前分析报告 · 点击右上角「生成今日」按钮生成'
            : '暂无日终复盘报告 · 收盘后点击「生成今日」按钮生成'}
        </div>
      )}
    </div>
  )
}

/* ═══════════════ TEAM AGENTS PANEL ═══════════════ */

const GLAND_COLORS: Record<string, string> = {
  calm: 'text-blue-400', fearful: 'text-yellow-400', greedy: 'text-red-400',
  alert: 'text-orange-400', frozen: 'text-gray-500',
}
const GLAND_CN: Record<string, string> = {
  calm: '冷静', fearful: '恐惧', greedy: '贪婪', alert: '警觉', frozen: '冻结',
}
const STATUS_ANIM: Record<string, string> = {
  idle: '', scanning: 'animate-pulse', analyzing: 'animate-pulse',
  approving: 'ring-2 ring-emerald-500/50', vetoing: 'ring-2 ring-red-500/50',
  executing: 'animate-bounce', arbitrating: 'animate-spin-slow',
  reviewing: 'animate-pulse', alerting: 'ring-2 ring-yellow-500/60 animate-pulse',
}
const STATUS_CN: Record<string, string> = {
  idle: '待命', scanning: '扫描中', analyzing: '分析中', approving: '审批中',
  vetoing: '否决中', executing: '执行中', arbitrating: '仲裁中',
  reviewing: '复盘中', alerting: '警报',
}
const GENE_CN: Record<string, string> = {
  balanced: '平衡', defensive: '防御', aggressive: '攻击', paranoid: '偏执', analytical: '分析',
}

function DecisionTimeline({ decisions }: { decisions: any[] }) {
  const [expandedIdx, setExpandedIdx] = useState<number | null>(null)
  return (
    <div>
      <div className="text-[11px] text-gray-500 mb-1.5">决策流 · Agent通信</div>
      <div className="max-h-[180px] overflow-y-auto space-y-1 pr-1">
        {decisions.slice(0, 15).map((d: any, i: number) => {
          const isVeto = d.action === 'veto' || d.action?.includes('stop_loss')
          const isApprove = d.action === 'approve' || d.action?.includes('approve')
          const isArbitrate = d.action?.includes('arbitrate')
          const isMutation = d.action === 'gene_mutation'
          const isReview = d.action === 'daily_review'
          const flow = d.action === 'propose_buy' ? '🎯→🛡️' :
                       d.action === 'approve' ? '🛡️→✅' :
                       d.action === 'veto' ? '🛡️→❌' :
                       d.action === 'reduce_size' ? '🛡️→⚖️' :
                       isArbitrate ? '🧠⚖️' :
                       d.action?.includes('protect') ? '🛡️→🧠' :
                       d.action?.includes('trailing') ? '🛡️→📉' :
                       isMutation ? '📊→🧬' :
                       isReview ? '📊📋' :
                       d.action?.includes('market') ? '🔭📊' : ''
          const actionColor = isVeto ? 'text-red-400' :
                              isApprove ? 'text-emerald-400' :
                              isArbitrate ? 'text-purple-400' :
                              isMutation ? 'text-amber-400' :
                              isReview ? 'text-cyan-400' : 'text-gray-300'
          const isExp = expandedIdx === i
          return (
            <div key={i}>
              <div className={`flex items-center gap-1.5 text-[10px] rounded px-2 py-1 cursor-pointer transition-colors ${
                isExp ? 'bg-gray-700/40' : 'bg-gray-800/30 hover:bg-gray-800/50'
              }`} onClick={() => setExpandedIdx(isExp ? null : i)}>
                <span className="text-gray-600 w-12 shrink-0">{d.ts?.slice(11, 19)}</span>
                {flow && <span className="w-8 shrink-0 text-center">{flow}</span>}
                <span className="w-10 shrink-0 text-gray-400">{d.agent}</span>
                <span className={`w-24 shrink-0 font-medium truncate ${actionColor}`}>{d.action}</span>
                <span className="text-gray-500 truncate flex-1">{d.reasoning || d.target}</span>
                {d.outcome === 'profit' && <span className="text-red-400 shrink-0" title="盈利">💰</span>}
                {d.outcome === 'loss' && <span className="text-green-400 shrink-0" title="亏损">📉</span>}
                <span className="text-gray-600 ml-auto shrink-0">{d.confidence?.toFixed(0)}%</span>
              </div>
              {isExp && (
                <div className="ml-4 mt-0.5 mb-1 p-2 bg-gray-800/60 rounded text-[9px] text-gray-400 space-y-0.5 border-l-2 border-cyan-500/30">
                  {d.target && <div><span className="text-gray-600">标的:</span> {d.target}</div>}
                  {d.reasoning && <div><span className="text-gray-600">理由:</span> {d.reasoning}</div>}
                  {d.confidence != null && <div><span className="text-gray-600">信心:</span> {d.confidence}%</div>}
                  {d.data && typeof d.data === 'object' && Object.keys(d.data).length > 0 && (
                    <div className="mt-1 pt-1 border-t border-gray-700/50">
                      {Object.entries(d.data).slice(0, 6).map(([k, v]: [string, any]) => (
                        <div key={k} className="truncate">
                          <span className="text-cyan-400/60">{k}:</span>{' '}
                          {typeof v === 'object' ? JSON.stringify(v).slice(0, 60) : String(v).slice(0, 60)}
                        </div>
                      ))}
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

function TeamPanel({ data }: { data: any }) {
  const [expanded, setExpanded] = useState<string | null>(null)
  const [perf, setPerf] = useState<any>(null)
  const [agentMem, setAgentMem] = useState<any>(null)

  // Fetch performance once on mount
  useEffect(() => {
    fetchJSON(`${API}/team/performance`).then(p => { if (p) setPerf(p) })
  }, [data])

  // Fetch agent memory when expanded
  useEffect(() => {
    if (expanded) {
      fetchJSON(`${API}/team/memory/${expanded}`).then(m => { if (m) setAgentMem(m) })
    } else {
      setAgentMem(null)
    }
  }, [expanded])

  if (!data || !data.agents) {
    return (
      <div className="bg-gray-900 border border-gray-800 rounded-xl p-5">
        <div className="flex items-center gap-2 mb-3">
          <Shield className="w-4 h-4 text-cyan-400" />
          <span className="text-sm font-medium text-gray-300">团队智能体</span>
          <span className="text-[10px] text-gray-600">5 Agents · 加载中…</span>
        </div>
        <div className="text-gray-600 text-sm text-center py-6">正在连接团队…</div>
      </div>
    )
  }

  const agents = data.agents
  const order = ['leader', 'macro', 'hunter', 'risk', 'coach']
  const decisions = data.recent_decisions || []

  return (
    <div className="bg-gray-900 border border-gray-800 rounded-xl p-5 space-y-4">
      {/* Header */}
      <div className="flex items-center gap-2 flex-wrap">
        <Shield className="w-4 h-4 text-cyan-400" />
        <span className="text-sm font-medium text-gray-300">团队智能体</span>
        <span className="text-[10px] text-gray-600">5 Agents · 七元组架构</span>
        {agents.macro?.risk_multiplier != null && (
          <span className={`text-[10px] px-1.5 py-0.5 rounded ${
            agents.macro.risk_multiplier < 0.8 ? 'bg-red-500/20 text-red-400' :
            agents.macro.risk_multiplier > 1.1 ? 'bg-emerald-500/20 text-emerald-400' :
            'bg-gray-700/50 text-gray-400'
          }`}>风险×{agents.macro.risk_multiplier}</span>
        )}
        {data.protect_mode && (
          <span className="px-2 py-0.5 text-[10px] bg-yellow-500/20 text-yellow-400 rounded-full animate-pulse">
            <AlertTriangle className="w-3 h-3 inline mr-0.5" />保本模式
          </span>
        )}
        {(() => {
          const alertCount = decisions.filter((d: any) =>
            d.action?.includes('stop_loss') || d.action?.includes('trailing') ||
            d.action?.includes('protect') || d.action === 'veto'
          ).length
          return alertCount > 0 ? (
            <span className="px-1.5 py-0.5 text-[9px] bg-red-500 text-white rounded-full font-bold">
              {alertCount}
            </span>
          ) : null
        })()}
      </div>

      {/* Alert Banner — show latest high-confidence alerts */}
      {decisions.filter((d: any) => d.confidence >= 90 || d.action?.includes('protect') || d.action === 'veto'
        || d.action?.includes('stop_loss') || d.action?.includes('trailing')
      ).slice(0, 3).map((d: any, i: number) => (
        <div key={i} className={`flex items-center gap-2 text-[10px] px-3 py-1.5 rounded-lg ${
          d.action?.includes('stop_loss') || d.action?.includes('trailing') ? 'bg-orange-500/10 text-orange-400 border border-orange-500/20' :
          d.action?.includes('protect') ? 'bg-yellow-500/10 text-yellow-400 border border-yellow-500/20' :
          d.action === 'veto' ? 'bg-red-500/10 text-red-400 border border-red-500/20' :
          'bg-cyan-500/10 text-cyan-400 border border-cyan-500/20'
        }`}>
          <AlertTriangle className="w-3 h-3 shrink-0" />
          <span className="font-medium">{d.agent}</span>
          <span>{d.reasoning || d.action}</span>
          <span className="ml-auto text-gray-600">{d.ts?.slice(11, 19)}</span>
        </div>
      ))}

      {/* Battle Plan + Coach Review Compact Panels */}
      {(() => {
        const briefing = decisions.find((d: any) => d.agent === 'leader' && d.action === 'morning_briefing')
        const review = decisions.find((d: any) => d.agent === 'coach' && d.action === 'daily_review')
        let plan = ''
        let report = ''
        try {
          if (briefing?.data) {
            const bd = typeof briefing.data === 'string' ? JSON.parse(briefing.data) : briefing.data
            plan = bd.battle_plan || ''
          }
          if (review?.data) {
            const rd = typeof review.data === 'string' ? JSON.parse(review.data) : review.data
            report = rd.llm_report || ''
          }
        } catch {}
        if (!plan && !report) return null
        return (
          <div className="grid grid-cols-1 lg:grid-cols-2 gap-2">
            {plan && (
              <div className="bg-cyan-500/5 border border-cyan-500/20 rounded-lg px-3 py-2">
                <div className="text-[10px] text-cyan-500 font-medium mb-0.5">🧠 组长作战计划</div>
                <div className="text-[10px] text-gray-300 leading-relaxed line-clamp-3">{plan}</div>
              </div>
            )}
            {report && (
              <div className="bg-purple-500/5 border border-purple-500/20 rounded-lg px-3 py-2">
                <div className="text-[10px] text-purple-500 font-medium mb-0.5">📊 教练复盘</div>
                <div className="text-[10px] text-gray-300 leading-relaxed line-clamp-3">{report}</div>
              </div>
            )}
          </div>
        )
      })()}

      {/* Performance Mini-Dashboard */}
      {perf && perf.total_days > 0 && (
        <div className="flex items-center gap-4 text-[10px] bg-gray-800/30 rounded-lg px-3 py-2 flex-wrap">
          <div className="flex items-center gap-1.5">
            <span className="text-gray-500">胜率</span>
            <span className={`font-medium ${perf.win_rate > 0.5 ? 'text-red-400' : 'text-green-400'}`}>
              {(perf.win_rate * 100).toFixed(0)}%
            </span>
            <span className="text-gray-600">({perf.profitable_days}盈/{perf.loss_days}亏/{perf.total_days}天)</span>
          </div>
          {perf.agent_accuracies && Object.entries(perf.agent_accuracies).map(([role, acc]: [string, any]) => {
            const ma = perf.multi_accuracy?.[role]
            const trend7v30 = ma ? (ma['7d'] - ma['30d']) : 0
            return (
              <div key={role} className="flex items-center gap-1" title={ma ? `7d:${(ma['7d']*100).toFixed(0)}% 14d:${(ma['14d']*100).toFixed(0)}% 30d:${(ma['30d']*100).toFixed(0)}%` : ''}>
                <span className="text-gray-600">{role}</span>
                <div className="w-12 h-1.5 bg-gray-700 rounded-full overflow-hidden">
                  <div className={`h-full rounded-full ${acc > 0.55 ? 'bg-emerald-500' : acc < 0.45 ? 'bg-red-500' : 'bg-gray-500'}`}
                       style={{ width: `${Math.min(100, acc * 100)}%` }} />
                </div>
                <span className="text-gray-500">{(acc * 100).toFixed(0)}%</span>
                {Math.abs(trend7v30) > 0.05 && (
                  <span className={`text-[8px] ${trend7v30 > 0 ? 'text-emerald-400' : 'text-red-400'}`}>
                    {trend7v30 > 0 ? '↑' : '↓'}
                  </span>
                )}
              </div>
            )
          })}
          {perf.pnl_history?.length > 1 && (
            <div className="flex items-center gap-0.5 ml-auto">
              <span className="text-gray-600">P&L</span>
              {perf.pnl_history.slice(-7).map((d: any, i: number) => (
                <div key={i} className={`w-1.5 rounded-sm ${d.pnl > 0 ? 'bg-red-500' : d.pnl < 0 ? 'bg-green-500' : 'bg-gray-600'}`}
                     style={{ height: `${Math.max(4, Math.min(14, Math.abs(d.pnl) * 5 + 4))}px` }}
                     title={`${d.date}: ${d.pnl > 0 ? '+' : ''}${d.pnl}%`} />
              ))}
            </div>
          )}
        </div>
      )}

      {/* Agent Cards Row */}
      <div className="grid grid-cols-5 gap-3">
        {order.map(role => {
          const a = agents[role]
          if (!a) return null
          const isExp = expanded === role
          const statusAnim = STATUS_ANIM[a.status] || ''
          const glandColor = GLAND_COLORS[a.gland?.state] || 'text-gray-400'

          return (
            <div key={role}
              onClick={() => setExpanded(isExp ? null : role)}
              className={`relative bg-gray-800/60 border rounded-xl p-3 cursor-pointer transition-all hover:bg-gray-800 ${
                isExp ? 'border-cyan-500/50 bg-gray-800' : 'border-gray-700/50'
              } ${statusAnim}`}>

              {/* Avatar + Name */}
              <div className="flex items-center gap-2 mb-2">
                <span className="text-2xl" title={a.name}>{a.avatar}</span>
                <div className="min-w-0">
                  <div className="text-sm font-medium text-white truncate">{a.name}</div>
                  <div className="text-[10px] text-gray-500">{a.role}</div>
                </div>
              </div>

              {/* Gene Badge */}
              <div className="flex items-center gap-1 mb-1.5">
                <span className="text-[10px] px-1.5 py-0.5 rounded bg-purple-500/20 text-purple-300">
                  基因: {GENE_CN[a.gene?.gene_type] || a.gene?.gene_type}
                </span>
                <span className={`text-[10px] px-1.5 py-0.5 rounded bg-gray-700 ${glandColor}`}>
                  {GLAND_CN[a.gland?.state] || a.gland?.state}
                </span>
              </div>

              {/* Status */}
              <div className="text-[11px] text-gray-400">
                {a.status !== 'idle' ? (
                  <span className="text-cyan-400 flex items-center gap-1">
                    <Loader2 className="w-3 h-3 animate-spin" />
                    {STATUS_CN[a.status] || a.status}
                  </span>
                ) : (
                  <span className="text-gray-600">{STATUS_CN[a.status]}</span>
                )}
              </div>

              {/* Voting weight badge */}
              {perf?.agent_weights?.[role] != null && role !== 'coach' && (
                <div className="mt-1 text-[9px] text-gray-600">
                  权重: <span className={`font-medium ${
                    perf.agent_weights[role] > 1.2 ? 'text-emerald-400' :
                    perf.agent_weights[role] < 0.8 ? 'text-red-400' : 'text-gray-400'
                  }`}>{perf.agent_weights[role].toFixed(2)}×</span>
                </div>
              )}

              {/* Quick stats */}
              <div className="mt-2 text-[10px] text-gray-500 space-y-0.5">
                {a.last_action && (
                  <div className="truncate">最近: {a.last_action}</div>
                )}
                {a.stats?.accuracy != null && (
                  <div>准确率: <span className={a.stats.accuracy >= 0.6 ? 'text-emerald-400' : 'text-yellow-400'}>
                    {(a.stats.accuracy * 100).toFixed(0)}%
                  </span></div>
                )}
                {role === 'risk' && a.veto_rate != null && (
                  <div>否决率: <span className={a.veto_rate > 0.7 ? 'text-red-400' : 'text-gray-300'}>
                    {(a.veto_rate * 100).toFixed(0)}%
                  </span> / 上限{((a.gene?.veto_rate_cap || 0.7) * 100).toFixed(0)}%</div>
                )}
                {role === 'risk' && a.daily_pnl_pct != null && (
                  <div>日P&L: <span className={a.daily_pnl_pct >= 0 ? 'text-red-400' : 'text-emerald-400'}>
                    {a.daily_pnl_pct > 0 ? '+' : ''}{a.daily_pnl_pct.toFixed(2)}%
                  </span>{a.protect_mode && <span className="text-yellow-400 ml-1">保本</span>}</div>
                )}
                {role === 'macro' && perf?.macro_rating_history?.length > 1 && (
                  <div className="flex items-center gap-0.5 mt-0.5">
                    <span className="text-gray-600 mr-1">评级:</span>
                    {perf.macro_rating_history.slice(-12).map((r: any, i: number) => {
                      const h = Math.max(3, Math.min(14, r.rating / 7))
                      return <div key={i} className={`w-1.5 rounded-sm ${r.rating > 60 ? 'bg-red-500' : r.rating < 40 ? 'bg-emerald-500' : 'bg-gray-500'}`}
                        style={{ height: `${h}px` }} title={`${r.date}: ${r.rating}`} />
                    })}
                    <span className="text-gray-600 ml-1">{a.current_rating ?? '—'}</span>
                  </div>
                )}
                {role === 'hunter' && a.proposals_today != null && (
                  <div>已提交: {a.proposals_today}/{a.max_proposals}</div>
                )}
                {role === 'hunter' && perf?.blacklist?.length > 0 && (
                  <div className="text-red-400/70">黑名单: {perf.blacklist.length}只</div>
                )}
                {role === 'leader' && a.daily_budget != null && (
                  <div>预算: ¥{(a.daily_budget / 10000).toFixed(1)}万</div>
                )}
                {role === 'leader' && a.llm_calls_today != null && (
                  <div>LLM: {a.llm_calls_today}/{a.max_llm_calls} · 仲裁{a.arbitrations_today || 0}次</div>
                )}
                {role === 'leader' && a.daily_pnl_pct != null && a.daily_pnl_pct !== 0 && (
                  <div>日P&L: <span className={a.daily_pnl_pct >= 0 ? 'text-red-400' : 'text-emerald-400'}>
                    {a.daily_pnl_pct > 0 ? '+' : ''}{a.daily_pnl_pct.toFixed(2)}%
                  </span></div>
                )}
              </div>

              {/* Gland bar */}
              <div className="mt-2 space-y-1">
                <div className="flex items-center gap-1 text-[9px] text-gray-600">
                  <span>皮质醇</span>
                  <div className="flex-1 h-1 bg-gray-700 rounded-full overflow-hidden">
                    <div className="h-full bg-yellow-500 rounded-full transition-all"
                      style={{ width: `${a.gland?.cortisol || 0}%` }} />
                  </div>
                  <span className="w-5 text-right">{(a.gland?.cortisol || 0).toFixed(0)}</span>
                </div>
                <div className="flex items-center gap-1 text-[9px] text-gray-600">
                  <span>多巴胺</span>
                  <div className="flex-1 h-1 bg-gray-700 rounded-full overflow-hidden">
                    <div className="h-full bg-cyan-500 rounded-full transition-all"
                      style={{ width: `${a.gland?.dopamine || 0}%` }} />
                  </div>
                  <span className="w-5 text-right">{(a.gland?.dopamine || 0).toFixed(0)}</span>
                </div>
              </div>
            </div>
          )
        })}
      </div>

      {/* Expanded Detail */}
      {expanded && agents[expanded] && (() => {
        const a = agents[expanded]
        // Find latest daily_review or battle_plan from decisions
        const latestReview = decisions.find((d: any) => d.agent === 'coach' && d.action === 'daily_review')
        const latestBriefing = decisions.find((d: any) => d.agent === 'leader' && d.action === 'morning_briefing')
        let reviewReport = ''
        let battlePlan = ''
        try {
          if (latestReview?.data) {
            const rd = typeof latestReview.data === 'string' ? JSON.parse(latestReview.data) : latestReview.data
            reviewReport = rd.llm_report || ''
          }
          if (latestBriefing?.data) {
            const bd = typeof latestBriefing.data === 'string' ? JSON.parse(latestBriefing.data) : latestBriefing.data
            battlePlan = bd.battle_plan || ''
          }
        } catch {}
        return (
          <div className="bg-gray-800/40 border border-gray-700/50 rounded-xl p-4 space-y-3 animate-in fade-in">
            <div className="flex items-center gap-2 mb-2">
              <span className="text-xl">{a.avatar}</span>
              <span className="text-sm font-medium text-white">{a.name}</span>
              <span className="text-xs text-gray-500">七元组详情</span>
            </div>

            {/* Multi-window accuracy bars (Phase 10) */}
            {perf?.multi_accuracy?.[expanded] && (() => {
              const ma = perf.multi_accuracy[expanded]
              return (
                <div className="flex items-center gap-3 text-[10px]">
                  <span className="text-gray-600">准确率趋势:</span>
                  {['7d', '14d', '30d'].map(w => {
                    const v = ma[w] ?? 0.5
                    return (
                      <div key={w} className="flex items-center gap-1">
                        <span className="text-gray-500 w-6">{w}</span>
                        <div className="w-16 h-1.5 bg-gray-700 rounded-full overflow-hidden">
                          <div className={`h-full rounded-full transition-all ${v > 0.55 ? 'bg-emerald-500' : v < 0.45 ? 'bg-red-500' : 'bg-gray-500'}`}
                               style={{ width: `${Math.min(100, v * 100)}%` }} />
                        </div>
                        <span className={`w-7 text-right ${v > 0.55 ? 'text-emerald-400' : v < 0.45 ? 'text-red-400' : 'text-gray-500'}`}>
                          {(v * 100).toFixed(0)}%
                        </span>
                      </div>
                    )
                  })}
                </div>
              )
            })()}

            {/* Role-specific panels */}
            {expanded === 'leader' && battlePlan && (
              <div className="bg-cyan-500/5 border border-cyan-500/20 rounded-lg p-2.5">
                <div className="text-cyan-400 text-[11px] font-medium mb-1">今日作战计划 (LLM)</div>
                <div className="text-[10px] text-gray-300 leading-relaxed">{battlePlan}</div>
              </div>
            )}
            {expanded === 'hunter' && (
              <div className="bg-amber-500/5 border border-amber-500/20 rounded-lg p-2.5 flex flex-wrap gap-3 text-[10px]">
                <div><span className="text-gray-500">评分阈值:</span> <span className="text-amber-400">{a.score_threshold || 0.60}</span></div>
                {(a.hot_strategies || []).length > 0 && (
                  <div><span className="text-gray-500">热策略:</span> {(a.hot_strategies || []).map((s: string, i: number) => (
                    <span key={i} className="ml-1 px-1.5 py-0.5 bg-emerald-500/10 text-emerald-400 rounded">{s}</span>
                  ))}</div>
                )}
                {(a.cold_sectors || []).length > 0 && (
                  <div><span className="text-gray-500">冷板块:</span> {(a.cold_sectors || []).map((s: string, i: number) => (
                    <span key={i} className="ml-1 px-1.5 py-0.5 bg-red-500/10 text-red-400 rounded">{s}</span>
                  ))}</div>
                )}
                {perf?.strategy_type_stats && Object.keys(perf.strategy_type_stats).length > 0 && (
                  <div className="mt-1.5 pt-1.5 border-t border-gray-700/50">
                    <span className="text-gray-500">策略胜率:</span>
                    <div className="flex flex-wrap gap-2 mt-1">
                      {Object.entries(perf.strategy_type_stats).map(([t, s]: [string, any]) => (
                        <div key={t} className="flex items-center gap-1">
                          <span className="text-[9px] text-gray-500 w-10">{
                            {trend:'趋势',breakout:'突破',volume:'量能',other:'其他'}[t] || t
                          }</span>
                          <div className="w-14 h-1.5 bg-gray-700 rounded-full overflow-hidden">
                            <div className={`h-full rounded-full ${s.win_rate > 0.55 ? 'bg-emerald-500' : s.win_rate < 0.4 ? 'bg-red-500' : 'bg-yellow-500'}`}
                                 style={{ width: `${Math.min(100, s.win_rate * 100)}%` }} />
                          </div>
                          <span className="text-[9px] text-gray-400">{(s.win_rate * 100).toFixed(0)}% ({s.total})</span>
                        </div>
                      ))}
                    </div>
                  </div>
                )}
              </div>
            )}
            {expanded === 'coach' && reviewReport && (
              <div className="bg-purple-500/5 border border-purple-500/20 rounded-lg p-2.5">
                <div className="text-purple-400 text-[11px] font-medium mb-1">教练复盘 (LLM)</div>
                <div className="text-[10px] text-gray-300 leading-relaxed">{reviewReport}</div>
              </div>
            )}
            {expanded === 'coach' && perf?.gene_mutations?.length > 0 && (
              <div className="bg-amber-500/5 border border-amber-500/20 rounded-lg p-2.5">
                <div className="text-amber-400 text-[11px] font-medium mb-1">基因微调历史</div>
                <div className="space-y-1 max-h-[80px] overflow-y-auto">
                  {perf.gene_mutations.map((m: any, i: number) => (
                    <div key={i} className="flex items-center gap-2 text-[9px]">
                      <span className="text-gray-600 w-16 shrink-0">{m.date || '—'}</span>
                      <span className="text-amber-400">{m.agent || '—'}</span>
                      <span className="text-gray-400 truncate">{m.field}: {m.old_val} → {m.new_val}</span>
                      <span className="text-gray-600 ml-auto">{m.reason || ''}</span>
                    </div>
                  ))}
                </div>
              </div>
            )}

            <div className="grid grid-cols-2 lg:grid-cols-4 gap-3 text-xs">
              {/* Gene */}
              <div className="bg-gray-900/60 rounded-lg p-2.5">
                <div className="text-purple-400 font-medium mb-1">基因</div>
                <div className="text-gray-300">{GENE_CN[a.gene?.gene_type] || '—'}</div>
                <div className="text-[10px] text-gray-500 mt-1 space-y-0.5">
                  <div>风险容忍: {((a.gene?.risk_tolerance || 0) * 100).toFixed(0)}%</div>
                  <div>信心偏差: {a.gene?.confidence_bias > 0 ? '+' : ''}{((a.gene?.confidence_bias || 0) * 100).toFixed(0)}%</div>
                  {a.gene?.veto_rate_cap < 1 && <div>否决上限: {((a.gene?.veto_rate_cap || 1) * 100).toFixed(0)}%</div>}
                </div>
              </div>
              {/* Skills */}
              <div className="bg-gray-900/60 rounded-lg p-2.5">
                <div className="text-blue-400 font-medium mb-1">技能</div>
                <div className="flex flex-wrap gap-1">
                  {(a.skills || []).map((s: string, i: number) => (
                    <span key={i} className="text-[10px] px-1.5 py-0.5 bg-blue-500/10 text-blue-300 rounded">{s}</span>
                  ))}
                </div>
              </div>
              {/* Instincts */}
              <div className="bg-gray-900/60 rounded-lg p-2.5">
                <div className="text-amber-400 font-medium mb-1">本能</div>
                <div className="space-y-0.5">
                  {(a.instincts || []).map((inst: string, i: number) => (
                    <div key={i} className="text-[10px] text-gray-400">{inst}</div>
                  ))}
                </div>
              </div>
              {/* MCP + Workflows + Memory */}
              <div className="bg-gray-900/60 rounded-lg p-2.5 space-y-2">
                <div>
                  <div className="text-emerald-400 font-medium mb-0.5">MCP 外接</div>
                  <div className="text-[10px] text-gray-400">{(a.mcp || []).join(' · ')}</div>
                </div>
                <div>
                  <div className="text-pink-400 font-medium mb-0.5">工作流</div>
                  <div className="text-[10px] text-gray-400">{(a.workflows || []).join(' ')}</div>
                </div>
                <div>
                  <div className="text-orange-400 font-medium mb-0.5">记忆 (持久化)</div>
                  {agentMem && Object.keys(agentMem).length > 0 ? (
                    <div className="space-y-0.5 max-h-[60px] overflow-y-auto">
                      {Object.entries(agentMem).slice(0, 8).map(([k, v]: [string, any]) => (
                        <div key={k} className="text-[9px] flex gap-1">
                          <span className="text-orange-400/60 shrink-0">{k}:</span>
                          <span className="text-gray-500 truncate">
                            {typeof v?.value === 'object' ? JSON.stringify(v.value).slice(0, 40) : String(v?.value ?? '').slice(0, 40)}
                          </span>
                        </div>
                      ))}
                    </div>
                  ) : (
                    <div className="text-[10px] text-gray-400">{(a.memory_keys || []).join(' · ') || '暂无记忆'}</div>
                  )}
                </div>
              </div>
            </div>
          </div>
        )
      })()}

      {/* Recent Decisions Timeline with Agent Communication Flow */}
      {decisions.length > 0 && (
        <DecisionTimeline decisions={decisions} />
      )}
    </div>
  )
}

/* ═══════════════ L5 MASTER ═══════════════ */
function MasterReport({ master, loading }: { master: any; loading: boolean }) {
  return (
    <div className="bg-gray-900 border border-gray-800 rounded-xl p-5 space-y-3">
      <div className="flex items-center gap-2">
        <Brain className="w-4 h-4 text-purple-400" />
        <span className="text-sm font-medium text-gray-300">Master 综合研判</span>
        <span className="text-[10px] text-gray-600">L5 Qwen LLM</span>
        {master?.usage && (
          <span className="text-[10px] text-gray-600 ml-auto">
            {master.model} · {master.usage.prompt_tokens}+{master.usage.completion_tokens} tokens
          </span>
        )}
      </div>

      {loading && (
        <div className="flex items-center gap-2 text-sm text-gray-400 py-8 justify-center">
          <Loader2 className="w-5 h-5 animate-spin" />
          <span>Qwen 正在分析持仓…（约15-30秒）</span>
        </div>
      )}

      {!loading && master?.error && (
        <div className="text-yellow-400 text-sm bg-yellow-500/10 rounded-lg p-3">
          <AlertTriangle className="w-4 h-4 inline mr-1" />{master.error}
          {master.hint && <span className="text-gray-400 ml-2">{master.hint}</span>}
        </div>
      )}

      {!loading && master?.analysis && (
        <div className="prose prose-invert prose-sm max-w-none text-gray-300 leading-relaxed space-y-2">
          {master.analysis.split('\n\n').map((block: string, i: number) => (
            <div key={i} className="bg-gray-800/40 rounded-lg p-3">
              {block.split('\n').map((line: string, j: number) => (
                <p key={j} className="mb-1 last:mb-0" dangerouslySetInnerHTML={{
                  __html: line
                    .replace(/\*\*(.*?)\*\*/g, '<strong class="text-white">$1</strong>')
                    .replace(/^- /, '<span class="text-amber-400 mr-1">•</span>')
                }} />
              ))}
            </div>
          ))}
        </div>
      )}

      {!loading && !master && (
        <div className="text-gray-600 text-sm py-6 text-center">
          点击顶部「Master分析」按钮生成 Qwen AI 综合研判报告
        </div>
      )}
    </div>
  )
}
