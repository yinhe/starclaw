import { useState, useEffect, useCallback } from 'react'
import { Link } from 'react-router-dom'
import {
  Activity, TrendingUp, TrendingDown, Shield, Brain, Wifi, WifiOff,
  LogOut, RefreshCw, AlertTriangle, CheckCircle2, Clock, BarChart3,
} from 'lucide-react'

interface Props {
  token: string
  onLogout: () => void
}

interface NodeInfo {
  id: string
  name: string
  mode: 'follower' | 'collaborator' | 'autonomous'
  online: boolean
  account: string
  totalAssets: number
  pnlToday: number
  pnlPct: number
  positions: number
  clawActive: boolean
  lastHeartbeat: string
}

interface Position {
  code: string
  name: string
  volume: number
  costPrice: number
  currentPrice: number
  pnlPct: number
  aiAdvice: string
}

interface AILogEntry {
  time: string
  level: 'info' | 'buy' | 'sell' | 'warning'
  message: string
}

const MOCK_NODES: NodeInfo[] = [
  { id: 'server-d', name: 'Server D (Sim)', mode: 'follower', online: true, account: 'test1006', totalAssets: 1000000, pnlToday: 12340, pnlPct: 1.23, positions: 8, clawActive: false, lastHeartbeat: '2s ago' },
  { id: 'local', name: 'Local (Real)', mode: 'collaborator', online: false, account: 'real-001', totalAssets: 500000, pnlToday: -2100, pnlPct: -0.42, positions: 5, clawActive: true, lastHeartbeat: '5min ago' },
]

const MOCK_POSITIONS: Position[] = [
  { code: '600519.SH', name: 'Kweichow Moutai', volume: 100, costPrice: 1780, currentPrice: 1820, pnlPct: 2.25, aiAdvice: 'Hold' },
  { code: '000858.SZ', name: 'Wuliangye', volume: 200, costPrice: 156, currentPrice: 152, pnlPct: -2.56, aiAdvice: 'Reduce' },
  { code: '601318.SH', name: 'Ping An', volume: 300, costPrice: 45, currentPrice: 47.2, pnlPct: 4.89, aiAdvice: 'Add' },
  { code: '000001.SZ', name: 'Ping An Bank', volume: 500, costPrice: 10.5, currentPrice: 11.02, pnlPct: 4.95, aiAdvice: 'Hold' },
  { code: '600036.SH', name: 'CMB', volume: 200, costPrice: 35.2, currentPrice: 36.1, pnlPct: 2.56, aiAdvice: 'Hold' },
]

const MOCK_AI_LOG: AILogEntry[] = [
  { time: '09:31', level: 'buy', message: 'BUY 600519.SH @1800 x100 — Score 0.85, Claw confirmed (confidence 82%)' },
  { time: '09:45', level: 'info', message: 'Market env: sideways-bullish. Maintaining 65% position.' },
  { time: '10:02', level: 'sell', message: 'REDUCE 000858.SZ @156 x100 — High-volume stall detected, Claw advises reduce' },
  { time: '10:15', level: 'warning', message: 'EVENT: PBoC targeted RRR cut announced — bullish for banks' },
  { time: '10:16', level: 'buy', message: 'ADD 601318.SH @46 x100 — Banking sector momentum + policy catalyst' },
  { time: '11:20', level: 'info', message: 'Scan complete: 3155 stocks, 12 candidates, 8 confirmed by Claw' },
  { time: '13:05', level: 'warning', message: 'Node "Local" heartbeat lost — monitoring' },
]

const modeColors = {
  follower: 'bg-gray-600',
  collaborator: 'bg-yellow-600',
  autonomous: 'bg-red-600',
}

const logColors = {
  info: 'text-gray-400',
  buy: 'text-red-400',
  sell: 'text-green-400',
  warning: 'text-yellow-400',
}

export default function Dashboard({ token, onLogout }: Props) {
  const [nodes, setNodes] = useState<NodeInfo[]>([])
  const [positions, setPositions] = useState<Position[]>([])
  const [aiLog, setAiLog] = useState<AILogEntry[]>([])
  const [time, setTime] = useState(new Date())
  const [scanning, setScanning] = useState(false)
  const [scanResult, setScanResult] = useState<any>(null)
  const [accountInfo, setAccountInfo] = useState<any>(null)

  // Fetch real data from Bridge API
  const fetchData = useCallback(async () => {
    try {
      // Account info
      const acctResp = await fetch('/bridge/account/info?account=27800348').then(r => r.json()).catch(() => null)
      if (acctResp) setAccountInfo(acctResp)

      // Positions from position manager
      const posResp = await fetch('/bridge/positions').then(r => r.json()).catch(() => [])
      const posList: Position[] = (Array.isArray(posResp) ? posResp : []).map((p: any) => ({
        code: p.code || '',
        name: p.code || '',
        volume: p.volume || 0,
        costPrice: p.entry_price || 0,
        currentPrice: p.highest_price || p.entry_price || 0,
        pnlPct: p.entry_price > 0 ? ((p.highest_price || p.entry_price) - p.entry_price) / p.entry_price * 100 : 0,
        aiAdvice: 'Hold',
      }))
      setPositions(posList)

      // Build node info from account
      const nodeLocal: NodeInfo = {
        id: 'local', name: 'Local QMT', mode: 'follower', online: !!acctResp,
        account: '27800348',
        totalAssets: acctResp?.total_assets || 0,
        pnlToday: 0, pnlPct: 0,
        positions: posList.length,
        clawActive: false, lastHeartbeat: 'now',
      }
      setNodes([nodeLocal])
    } catch (e) {
      console.error('fetchData error', e)
    }
  }, [])

  useEffect(() => {
    fetchData()
    const t1 = setInterval(() => setTime(new Date()), 1000)
    const t2 = setInterval(fetchData, 30000) // refresh every 30s
    return () => { clearInterval(t1); clearInterval(t2) }
  }, [fetchData])

  const triggerScan = useCallback(async () => {
    setScanning(true)
    setScanResult(null)
    setAiLog(prev => [...prev, { time: new Date().toLocaleTimeString().slice(0, 5), level: 'info' as const, message: 'Scan triggered...' }])
    try {
      const resp = await fetch('/bridge/scan', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: '{}',
      })
      const data = await resp.json()
      setScanResult(data)
      if (data.candidates) {
        setAiLog(prev => [...prev, { time: new Date().toLocaleTimeString().slice(0, 5), level: 'info' as const, message: `Scan complete: ${data.scanned} stocks, ${data.candidates} candidates, ${data.orders} orders` }])
      }
      if (data.order_details) {
        for (const o of (Array.isArray(data.order_details) ? data.order_details : [])) {
          setAiLog(prev => [...prev, { time: new Date().toLocaleTimeString().slice(0, 5), level: 'buy' as const, message: `BUY ${o.code} @${o.price} x${o.volume} score=${o.score?.toFixed(2)}` }])
        }
      }
      fetchData() // refresh positions after scan
    } catch (err: any) {
      setScanResult({ error: err.message })
      setAiLog(prev => [...prev, { time: new Date().toLocaleTimeString().slice(0, 5), level: 'warning' as const, message: `Scan error: ${err.message}` }])
    } finally {
      setScanning(false)
    }
  }, [fetchData])

  const totalAssets = accountInfo?.total_assets || 0
  const totalPnl = 0 // TODO: calculate from positions
  const totalPnlPct = 0
  const onlineNodes = nodes.filter((n) => n.online).length

  return (
    <div className="min-h-screen bg-gray-950">
      {/* Top bar */}
      <div className="bg-gray-900 border-b border-gray-800 px-6 py-3 flex items-center justify-between">
        <div className="flex items-center gap-1.5">
          <div className="w-8 h-8 rounded-lg bg-gradient-to-br from-red-500 to-amber-500 flex items-center justify-center text-white font-bold text-xs">Q8</div>
          <div className="flex items-baseline gap-1">
            <span className="text-lg font-bold text-white tracking-tight">Q8bot</span>
            <span className="text-[13px] text-gray-400 font-medium">AI量化智能体</span>
          </div>
          <span className="text-xs text-gray-600 font-mono ml-2">{time.toLocaleTimeString()}</span>
        </div>
        <div className="flex items-center gap-4">
          <button onClick={triggerScan} disabled={scanning} className="flex items-center gap-1.5 text-sm bg-red-600 hover:bg-red-500 disabled:bg-gray-700 text-white px-3 py-1.5 rounded-lg transition">
            <RefreshCw className={`w-3.5 h-3.5 ${scanning ? 'animate-spin' : ''}`} /> {scanning ? 'Scanning...' : 'Trigger Scan'}
          </button>
          <button onClick={onLogout} className="flex items-center gap-1.5 text-sm text-gray-400 hover:text-white transition">
            <LogOut className="w-4 h-4" /> Logout
          </button>
        </div>
      </div>

      <div className="p-6 space-y-6">
        {/* Summary cards */}
        <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
          <SummaryCard icon={BarChart3} label="Total Assets" value={`¥${(totalAssets / 10000).toFixed(1)}万`} />
          <SummaryCard icon={totalPnl >= 0 ? TrendingUp : TrendingDown} label="Today P&L" value={`${totalPnl >= 0 ? '+' : ''}¥${totalPnl.toLocaleString()}`} valueColor={totalPnl >= 0 ? 'text-red-400' : 'text-green-400'} sub={`${totalPnlPct >= 0 ? '+' : ''}${totalPnlPct.toFixed(2)}%`} />
          <SummaryCard icon={Activity} label="Nodes Online" value={`${onlineNodes}/${nodes.length}`} valueColor={onlineNodes === nodes.length ? 'text-red-400' : 'text-yellow-400'} />
          <SummaryCard icon={Shield} label="Risk Status" value="Normal" valueColor="text-red-400" />
        </div>

        <div className="grid lg:grid-cols-3 gap-6">
          {/* Node cards */}
          <div className="lg:col-span-1 space-y-4">
            <h2 className="text-sm font-medium text-gray-400 uppercase tracking-wider">Claw Nodes</h2>
            {nodes.map((n) => (
              <div key={n.id} className="bg-gray-900 border border-gray-800 rounded-xl p-4 space-y-3">
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-2">
                    {n.online ? <Wifi className="w-4 h-4 text-red-400" /> : <WifiOff className="w-4 h-4 text-gray-600" />}
                    <span className="font-medium text-white text-sm">{n.name}</span>
                  </div>
                  <span className={`text-xs px-2 py-0.5 rounded-full text-white ${modeColors[n.mode]}`}>{n.mode}</span>
                </div>
                <div className="grid grid-cols-2 gap-2 text-xs">
                  <div><span className="text-gray-500">Account:</span> <span className="text-white">{n.account}</span></div>
                  <div><span className="text-gray-500">Assets:</span> <span className="text-white">¥{(n.totalAssets / 10000).toFixed(0)}万</span></div>
                  <div><span className="text-gray-500">P&L:</span> <span className={n.pnlToday >= 0 ? 'text-red-400' : 'text-green-400'}>{n.pnlToday >= 0 ? '+' : ''}{n.pnlPct.toFixed(2)}%</span></div>
                  <div><span className="text-gray-500">Positions:</span> <span className="text-white">{n.positions}</span></div>
                </div>
                <div className="flex items-center justify-between text-xs">
                  <span className="text-gray-600">Claw: {n.clawActive ? <span className="text-amber-400">Active</span> : <span className="text-gray-500">Sleep</span>}</span>
                  <span className="text-gray-600"><Clock className="w-3 h-3 inline" /> {n.lastHeartbeat}</span>
                </div>
              </div>
            ))}
          </div>

          {/* AI Decision Log */}
          <div className="lg:col-span-2 space-y-4">
            <h2 className="text-sm font-medium text-gray-400 uppercase tracking-wider flex items-center gap-2">
              <Brain className="w-4 h-4" /> AI Decision Log
            </h2>
            <div className="bg-gray-900 border border-gray-800 rounded-xl p-4 max-h-[320px] overflow-y-auto space-y-2">
              {aiLog.map((entry, i) => (
                <div key={i} className="flex items-start gap-3 text-sm">
                  <span className="text-gray-600 font-mono text-xs min-w-[40px]">{entry.time}</span>
                  <span className={`${logColors[entry.level]} leading-relaxed`}>{entry.message}</span>
                </div>
              ))}
            </div>

            {/* Scan result */}
            {scanResult && (
              <div className="bg-gray-900 border border-gray-800 rounded-xl p-4">
                <h3 className="text-sm font-medium text-gray-400 mb-2">Latest Scan Result</h3>
                {scanResult.error ? (
                  <div className="text-red-400 text-sm">{scanResult.error}</div>
                ) : (
                  <div className="grid grid-cols-3 md:grid-cols-5 gap-3 text-sm">
                    <div><span className="text-gray-500">Scanned:</span> <span className="text-white">{scanResult.scanned}</span></div>
                    <div><span className="text-gray-500">Candidates:</span> <span className="text-green-400">{scanResult.candidates}</span></div>
                    <div><span className="text-gray-500">Confirmed:</span> <span className="text-green-400">{scanResult.confirmed}</span></div>
                    <div><span className="text-gray-500">Orders:</span> <span className="text-white">{scanResult.orders}</span></div>
                    <div><span className="text-gray-500">Time:</span> <span className="text-white">{scanResult.elapsed?.toFixed(1)}s</span></div>
                  </div>
                )}
              </div>
            )}
          </div>
        </div>

        {/* Positions table */}
        <div>
          <h2 className="text-sm font-medium text-gray-400 uppercase tracking-wider mb-4">All Positions</h2>
          <div className="bg-gray-900 border border-gray-800 rounded-xl overflow-hidden">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-gray-800 text-gray-500">
                  <th className="text-left px-4 py-3 font-medium">Code</th>
                  <th className="text-left px-4 py-3 font-medium">Name</th>
                  <th className="text-right px-4 py-3 font-medium">Volume</th>
                  <th className="text-right px-4 py-3 font-medium">Cost</th>
                  <th className="text-right px-4 py-3 font-medium">Current</th>
                  <th className="text-right px-4 py-3 font-medium">P&L %</th>
                  <th className="text-center px-4 py-3 font-medium">AI Advice</th>
                </tr>
              </thead>
              <tbody>
                {positions.map((p) => (
                  <tr key={p.code} className="border-b border-gray-800/50 hover:bg-gray-800/30 transition">
                    <td className="px-4 py-3 font-mono text-white">{p.code}</td>
                    <td className="px-4 py-3 text-gray-300">{p.name}</td>
                    <td className="px-4 py-3 text-right text-white">{p.volume}</td>
                    <td className="px-4 py-3 text-right text-gray-400">{p.costPrice.toFixed(2)}</td>
                    <td className="px-4 py-3 text-right text-white">{p.currentPrice.toFixed(2)}</td>
                    <td className={`px-4 py-3 text-right font-medium ${p.pnlPct >= 0 ? 'text-red-400' : 'text-green-400'}`}>
                      {p.pnlPct >= 0 ? '+' : ''}{p.pnlPct.toFixed(2)}%
                    </td>
                    <td className="px-4 py-3 text-center">
                      <AdviceBadge advice={p.aiAdvice} />
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      </div>
    </div>
  )
}

function SummaryCard({ icon: Icon, label, value, valueColor = 'text-white', sub }: {
  icon: any; label: string; value: string; valueColor?: string; sub?: string
}) {
  return (
    <div className="bg-gray-900 border border-gray-800 rounded-xl p-4">
      <div className="flex items-center gap-2 mb-2">
        <Icon className="w-4 h-4 text-gray-500" />
        <span className="text-xs text-gray-500 uppercase">{label}</span>
      </div>
      <div className={`text-xl font-bold ${valueColor}`}>{value}</div>
      {sub && <div className="text-xs text-gray-500 mt-0.5">{sub}</div>}
    </div>
  )
}

function AdviceBadge({ advice }: { advice: string }) {
  const colors: Record<string, string> = {
    Hold: 'bg-gray-700 text-gray-300',
    Add: 'bg-red-500/20 text-red-400',
    Reduce: 'bg-green-500/20 text-green-400',
    Sell: 'bg-green-600/20 text-green-400',
    Buy: 'bg-red-600/20 text-red-400',
  }
  return (
    <span className={`text-xs px-2 py-0.5 rounded-full ${colors[advice] || 'bg-gray-700 text-gray-400'}`}>
      {advice}
    </span>
  )
}
