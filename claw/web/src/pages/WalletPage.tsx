import { useState, useEffect } from 'react'
import { Zap, Snowflake, Shield, Copy, Check, ArrowDownRight, ArrowUpRight, RefreshCw, WifiOff, Loader2, TrendingUp, Award } from 'lucide-react'
import { systemAPI, nodeAPI } from '../lib/api'

interface CreditData {
  connected: boolean
  balance?: number
  balance_energy?: number
  frozen?: number
  frozen_energy?: number
  total_in?: number
  total_out?: number
  nonce?: number
  status?: string
  hp_status?: string
  trust_level?: string
  updated_at?: string
  message?: string
}

const hpConfig: Record<string, { color: string; textDark: string; bg: string; gradient: string; glow: string; label: string; desc: string; barBg: string }> = {
  full:       { color: 'text-emerald-400', textDark: 'text-emerald-600', bg: 'bg-emerald-500', gradient: 'from-emerald-400 to-green-500',  glow: 'shadow-emerald-500/50', label: '充沛', desc: '星能充足，所有功能满血运行', barBg: 'bg-emerald-500/20' },
  healthy:    { color: 'text-green-400',   textDark: 'text-green-600',   bg: 'bg-green-400',   gradient: 'from-green-400 to-emerald-500',  glow: 'shadow-green-400/40',   label: '健康', desc: '星能健康，请保持',         barBg: 'bg-green-500/20' },
  low:        { color: 'text-yellow-400',  textDark: 'text-yellow-600',  bg: 'bg-yellow-400',  gradient: 'from-yellow-400 to-amber-500',   glow: 'shadow-yellow-400/40',  label: '偏低', desc: '星能偏低，建议充值',       barBg: 'bg-yellow-500/20' },
  critical:   { color: 'text-orange-400',  textDark: 'text-orange-600',  bg: 'bg-orange-500',  gradient: 'from-orange-400 to-red-500',     glow: 'shadow-orange-500/50',  label: '危急', desc: '星能即将耗尽，请尽快充值！', barBg: 'bg-orange-500/20' },
  hibernated: { color: 'text-red-400',     textDark: 'text-red-600',     bg: 'bg-red-500',     gradient: 'from-red-400 to-red-600',        glow: 'shadow-red-500/50',     label: '休眠', desc: '星能耗尽，部分功能已暂停',   barBg: 'bg-red-500/20' },
}

const trustConfig: Record<string, { label: string; color: string; border: string; icon: typeof Award }> = {
  newcomer:   { label: '新手',   color: 'text-gray-500',   border: 'border-gray-200',   icon: Award },
  verified:   { label: '已验证', color: 'text-blue-600',   border: 'border-blue-200',   icon: Award },
  trusted:    { label: '可信',   color: 'text-green-600',  border: 'border-green-200',  icon: Award },
  veteran:    { label: '老将',   color: 'text-purple-600', border: 'border-purple-200', icon: Award },
  legendary:  { label: '传奇',   color: 'text-amber-600',  border: 'border-amber-200',  icon: Award },
}

const ENERGY_UNIT = 10000

function formatEnergy(units: number): string {
  const stars = units / ENERGY_UNIT
  if (stars >= 10000) return `${(stars / 1000).toFixed(1)}K`
  if (stars >= 1000) return `${(stars / 1000).toFixed(2)}K`
  return stars.toFixed(2)
}

export default function WalletPage() {
  const [credits, setCredits] = useState<CreditData | null>(null)
  const [nodeInfo, setNodeInfo] = useState<any>(null)
  const [loading, setLoading] = useState(true)
  const [refreshing, setRefreshing] = useState(false)
  const [copied, setCopied] = useState('')

  const load = async () => {
    try {
      const [creditsRes, nodeRes] = await Promise.all([
        systemAPI.getCredits().catch(() => null),
        nodeAPI.getInfo().catch(() => null),
      ])
      if (creditsRes) setCredits(creditsRes.data)
      if (nodeRes) setNodeInfo(nodeRes.data)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => { load() }, [])

  useEffect(() => {
    const interval = setInterval(() => {
      systemAPI.getCredits().then(res => setCredits(res.data)).catch(() => {})
    }, 30000)
    return () => clearInterval(interval)
  }, [])

  const handleRefresh = async () => {
    setRefreshing(true)
    await load()
    setRefreshing(false)
  }

  const copyToClipboard = (text: string, label: string) => {
    navigator.clipboard.writeText(text)
    setCopied(label)
    setTimeout(() => setCopied(''), 2000)
  }

  if (loading) {
    return (
      <div className="h-full flex items-center justify-center">
        <Loader2 className="w-6 h-6 animate-spin text-amber-400" />
      </div>
    )
  }

  const connected = credits?.connected ?? false
  const stars = credits?.balance_energy ?? 0
  const hp = credits?.hp_status ?? 'hibernated'
  const cfg = hpConfig[hp] || hpConfig.hibernated
  const trust = trustConfig[credits?.trust_level || 'newcomer'] || trustConfig.newcomer
  const pct = Math.min(100, (stars / 2000) * 100)
  const isLow = hp === 'low' || hp === 'critical' || hp === 'hibernated'

  const totalInStars = (credits?.total_in ?? 0) / ENERGY_UNIT
  const totalOutStars = (credits?.total_out ?? 0) / ENERGY_UNIT
  const frozenStars = credits?.frozen_energy ?? 0

  return (
    <div className="h-full overflow-y-auto">
      <div className="max-w-3xl mx-auto p-8">
        {/* Header */}
        <div className="flex items-center justify-between mb-8">
          <div>
            <h1 className="text-2xl font-bold text-gray-900 dark:text-white flex items-center gap-2">
              <div className="w-8 h-8 rounded-lg bg-gradient-to-br from-amber-400 to-orange-500 flex items-center justify-center shadow-lg shadow-amber-500/25">
                <Zap className="w-5 h-5 text-white" fill="white" />
              </div>
              星能
            </h1>
            <p className="text-gray-500 text-sm mt-1">算力能量 · 节点身份 · 信用余额</p>
          </div>
          <button
            onClick={handleRefresh}
            disabled={refreshing}
            className="flex items-center gap-1.5 px-3 py-1.5 text-xs border border-gray-200 dark:border-gray-700 rounded-lg hover:bg-gray-50 dark:hover:bg-gray-800 disabled:opacity-50 transition-colors"
          >
            <RefreshCw className={`w-3.5 h-3.5 ${refreshing ? 'animate-spin' : ''}`} />
            刷新
          </button>
        </div>

        {/* Offline Warning */}
        {!connected && (
          <div className="mb-6 p-4 bg-red-50 dark:bg-red-950/30 border border-red-200 dark:border-red-800 rounded-xl flex items-center gap-3">
            <WifiOff className="w-5 h-5 text-red-500 shrink-0" />
            <div>
              <p className="text-sm font-medium text-red-800 dark:text-red-300">未连接虫群</p>
              <p className="text-xs text-red-600 dark:text-red-400 mt-0.5">星能余额通过虫群心跳同步，请先在设置中加入虫群。</p>
            </div>
          </div>
        )}

        {/* Main Balance Card */}
        <section className={`relative overflow-hidden rounded-2xl p-6 mb-6 ${connected ? 'bg-gradient-to-br from-gray-900 via-gray-800 to-gray-900' : 'bg-gradient-to-br from-gray-500 to-gray-600'}`}>
          {/* Background lightning decoration */}
          <div className="absolute -top-8 -right-8 w-48 h-48 opacity-[0.04]">
            <Zap className="w-full h-full" fill="white" />
          </div>
          <div className="absolute bottom-2 left-4 w-24 h-24 opacity-[0.03] rotate-12">
            <Zap className="w-full h-full" fill="white" />
          </div>

          <div className="relative">
            {/* Balance */}
            <div className="mb-5">
              <p className="text-gray-400 text-xs font-medium uppercase tracking-wider mb-2">可用余额</p>
              <div className="flex items-baseline gap-3">
                <span className="text-5xl font-extrabold text-white tracking-tight">
                  {connected ? formatEnergy(credits?.balance ?? 0) : '—'}
                </span>
                <div className="flex items-center gap-1">
                  <Zap className="w-5 h-5 text-amber-400" fill="currentColor" />
                  <span className="text-sm font-bold text-amber-400">⚡</span>
                </div>
              </div>
              {connected && credits?.balance != null && (
                <p className="text-xs text-gray-500 mt-1.5 font-mono">{credits.balance.toLocaleString()} 内部单位</p>
              )}
            </div>

            {/* HP Bar */}
            <div className="mb-5">
              <div className="flex items-center justify-between mb-2">
                <div className="flex items-center gap-2">
                  <Zap className={`w-4 h-4 ${cfg.color} ${isLow ? 'animate-pulse' : ''}`} fill="currentColor" />
                  <span className={`text-sm font-bold ${cfg.color}`}>
                    HP · {cfg.label}
                  </span>
                </div>
                <span className="text-sm font-mono text-gray-300">{Math.round(pct)}%</span>
              </div>
              <div className={`w-full h-3 bg-gray-700/60 rounded-full overflow-hidden shadow-inner`}>
                <div
                  className={`h-full bg-gradient-to-r ${cfg.gradient} rounded-full transition-all duration-1000 ease-out shadow-lg ${cfg.glow}`}
                  style={{ width: `${Math.max(connected ? pct : 0, 2)}%` }}
                />
              </div>
              <p className="text-xs text-gray-400 mt-2">{cfg.desc}</p>
            </div>

            {/* Trust + Status + Top Up */}
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-2">
                <span className={`inline-flex items-center gap-1 px-2.5 py-1 rounded-full text-xs font-medium border ${trust.border} bg-white/10 ${trust.color}`}>
                  <trust.icon className="w-3 h-3" /> {trust.label}
                </span>
                {credits?.status && (
                  <span className="inline-flex items-center gap-1 px-2.5 py-1 rounded-full text-xs font-medium bg-gray-700/50 text-gray-300 border border-gray-600">
                    <Shield className="w-3 h-3" /> {credits.status}
                  </span>
                )}
              </div>
              {nodeInfo?.node_id && (
                <a
                  href={`https://starclaw.net/billing?claw_id=${encodeURIComponent(nodeInfo.node_id)}`}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="inline-flex items-center gap-1.5 px-4 py-2 rounded-xl bg-gradient-to-r from-amber-400 to-orange-500 text-white text-xs font-bold hover:from-amber-500 hover:to-orange-600 transition-all shadow-lg shadow-orange-500/30 hover:shadow-orange-500/50 hover:scale-105"
                >
                  <Zap className="w-3.5 h-3.5" fill="white" /> 充值星能
                </a>
              )}
            </div>
          </div>
        </section>

        {/* Stats Grid */}
        <div className="grid grid-cols-3 gap-4 mb-6">
          <div className="bg-white dark:bg-gray-800 border border-gray-100 dark:border-gray-700 rounded-xl p-4 hover:shadow-md transition-shadow">
            <div className="flex items-center gap-2 mb-2">
              <div className="w-8 h-8 rounded-lg bg-emerald-50 dark:bg-emerald-900/20 flex items-center justify-center">
                <ArrowDownRight className="w-4 h-4 text-emerald-500" />
              </div>
              <span className="text-xs text-gray-500">总收入</span>
            </div>
            <p className="text-lg font-bold text-gray-900 dark:text-white">
              {connected ? `${totalInStars.toFixed(1)}` : '—'}
              <span className="text-xs text-gray-400 ml-1">⚡</span>
            </p>
          </div>
          <div className="bg-white dark:bg-gray-800 border border-gray-100 dark:border-gray-700 rounded-xl p-4 hover:shadow-md transition-shadow">
            <div className="flex items-center gap-2 mb-2">
              <div className="w-8 h-8 rounded-lg bg-red-50 dark:bg-red-900/20 flex items-center justify-center">
                <ArrowUpRight className="w-4 h-4 text-red-500" />
              </div>
              <span className="text-xs text-gray-500">总支出</span>
            </div>
            <p className="text-lg font-bold text-gray-900 dark:text-white">
              {connected ? `${totalOutStars.toFixed(1)}` : '—'}
              <span className="text-xs text-gray-400 ml-1">⚡</span>
            </p>
          </div>
          <div className="bg-white dark:bg-gray-800 border border-gray-100 dark:border-gray-700 rounded-xl p-4 hover:shadow-md transition-shadow">
            <div className="flex items-center gap-2 mb-2">
              <div className="w-8 h-8 rounded-lg bg-blue-50 dark:bg-blue-900/20 flex items-center justify-center">
                <Snowflake className="w-4 h-4 text-blue-500" />
              </div>
              <span className="text-xs text-gray-500">冻结中</span>
            </div>
            <p className="text-lg font-bold text-gray-900 dark:text-white">
              {connected ? `${frozenStars.toFixed(1)}` : '—'}
              <span className="text-xs text-gray-400 ml-1">⚡</span>
            </p>
          </div>
        </div>

        {/* Node Identity */}
        {nodeInfo && (
          <section className="bg-white dark:bg-gray-800 border border-gray-100 dark:border-gray-700 rounded-xl p-6 mb-6">
            <h2 className="text-sm font-semibold text-gray-700 dark:text-gray-200 flex items-center gap-2 mb-4">
              <Shield className="w-4 h-4 text-indigo-500" /> 节点身份
            </h2>
            <div className="space-y-3">
              {/* Claw Address */}
              <div className="flex items-center justify-between p-3 bg-gray-50 dark:bg-gray-700/50 rounded-lg">
                <div>
                  <p className="text-xs text-gray-500 mb-0.5">Claw 地址（钱包地址）</p>
                  <p className="font-mono text-sm text-gray-800 dark:text-gray-200">{nodeInfo.node_id}</p>
                </div>
                <button
                  onClick={() => copyToClipboard(nodeInfo.node_id, 'claw_id')}
                  className="p-2 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 transition-colors"
                  title="复制地址"
                >
                  {copied === 'claw_id' ? <Check className="w-4 h-4 text-green-500" /> : <Copy className="w-4 h-4" />}
                </button>
              </div>
              {/* Fingerprint */}
              {nodeInfo.fingerprint && (
                <div className="flex items-center justify-between p-3 bg-gray-50 dark:bg-gray-700/50 rounded-lg">
                  <div>
                    <p className="text-xs text-gray-500 mb-0.5">基因指纹</p>
                    <p className="font-mono text-xs text-gray-600 dark:text-gray-400 truncate max-w-md">{nodeInfo.fingerprint}</p>
                  </div>
                  <button
                    onClick={() => copyToClipboard(nodeInfo.fingerprint, 'fingerprint')}
                    className="p-2 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 transition-colors shrink-0"
                  >
                    {copied === 'fingerprint' ? <Check className="w-4 h-4 text-green-500" /> : <Copy className="w-4 h-4" />}
                  </button>
                </div>
              )}
              {/* Network address */}
              {nodeInfo.address && (
                <div className="flex items-center justify-between p-3 bg-gray-50 dark:bg-gray-700/50 rounded-lg">
                  <div>
                    <p className="text-xs text-gray-500 mb-0.5">Nydus 地址</p>
                    <p className="font-mono text-sm text-gray-700 dark:text-gray-300">{nodeInfo.address}</p>
                  </div>
                  <button
                    onClick={() => copyToClipboard(nodeInfo.address, 'address')}
                    className="p-2 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 transition-colors"
                  >
                    {copied === 'address' ? <Check className="w-4 h-4 text-green-500" /> : <Copy className="w-4 h-4" />}
                  </button>
                </div>
              )}
            </div>
          </section>
        )}

        {/* Star Energy Explainer */}
        <section className="bg-white dark:bg-gray-800 border border-gray-100 dark:border-gray-700 rounded-xl p-6">
          <h2 className="text-sm font-semibold text-gray-700 dark:text-gray-200 flex items-center gap-2 mb-3">
            <Zap className="w-4 h-4 text-amber-500" fill="currentColor" /> 关于星能 (Star Energy)
          </h2>
          <div className="text-xs text-gray-500 dark:text-gray-400 space-y-2 leading-relaxed">
            <p>
              <strong className="text-gray-700 dark:text-gray-200">星能 ⚡</strong> 是 StarClaw 虫群网络的算力能量单位。
              1 ⚡ = 10,000 内部单位，精度为 4 位小数。
            </p>
            <p>
              每个新节点加入虫群时自动获得 <strong className="text-gray-700 dark:text-gray-200">100 ⚡ 欢迎奖励</strong>。
              星能用于 API 调用、赏金任务、算力贡献结算。在 <a href="https://star-ai.net/billing" target="_blank" rel="noopener noreferrer" className="text-amber-600 hover:underline font-medium">star-ai.net</a> 可直接充值。
            </p>
            <div className="mt-3 p-3 bg-gradient-to-r from-amber-50 to-orange-50 dark:from-amber-950/30 dark:to-orange-950/30 rounded-lg border border-amber-100 dark:border-amber-800/30">
              <p className="text-amber-800 dark:text-amber-300 font-medium flex items-center gap-1.5">
                <Zap className="w-3.5 h-3.5" fill="currentColor" /> HP 状态等级
              </p>
              <div className="mt-2 grid grid-cols-5 gap-2">
                {Object.entries(hpConfig).map(([key, val]) => (
                  <div key={key} className="text-center py-1.5 rounded-lg" style={{ backgroundColor: key === hp ? 'rgba(0,0,0,0.05)' : 'transparent' }}>
                    <Zap className={`w-4 h-4 mx-auto ${val.color}`} fill="currentColor" />
                    <p className={`text-[10px] mt-0.5 font-medium ${key === hp ? val.textDark : 'text-gray-500'}`}>{val.label}</p>
                  </div>
                ))}
              </div>
            </div>
            <div className="mt-3 p-3 bg-indigo-50 dark:bg-indigo-950/30 rounded-lg border border-indigo-100 dark:border-indigo-800/30">
              <p className="text-indigo-700 dark:text-indigo-300 font-medium flex items-center gap-1.5">
                <TrendingUp className="w-3.5 h-3.5" /> 获取星能的方式
              </p>
              <div className="mt-2 grid grid-cols-3 gap-2 text-[10px]">
                {['充值购买', '推理挖矿', '赏金完成', '模板出售', 'P2P 转账', '邀请奖励'].map(way => (
                  <div key={way} className="flex items-center gap-1 text-indigo-600 dark:text-indigo-400">
                    <Zap className="w-2.5 h-2.5" /> {way}
                  </div>
                ))}
              </div>
            </div>
          </div>
        </section>

        {/* Last updated */}
        {credits?.updated_at && (
          <p className="text-xs text-gray-400 text-center mt-4">
            上次同步: {new Date(credits.updated_at).toLocaleString()}
          </p>
        )}
      </div>
    </div>
  )
}
