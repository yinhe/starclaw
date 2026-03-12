import { useState, useEffect } from 'react'
import { Star, Snowflake, Shield, Copy, Check, ArrowDownRight, ArrowUpRight, Wallet, RefreshCw, WifiOff, Loader2, ExternalLink } from 'lucide-react'
import { systemAPI, nodeAPI } from '../lib/api'

interface CreditData {
  connected: boolean
  balance?: number
  balance_stars?: number
  frozen?: number
  frozen_stars?: number
  total_in?: number
  total_out?: number
  nonce?: number
  status?: string
  hp_status?: string
  trust_level?: string
  updated_at?: string
  message?: string
}

const hpConfig: Record<string, { color: string; bg: string; gradient: string; label: string; emoji: string; desc: string }> = {
  full:       { color: 'text-emerald-400', bg: 'bg-emerald-500', gradient: 'from-emerald-400 to-green-500',  label: '充沛', emoji: '🟢', desc: '星力充足，所有功能正常运行' },
  healthy:    { color: 'text-green-400',   bg: 'bg-green-400',   gradient: 'from-green-400 to-emerald-500',  label: '健康', emoji: '🟢', desc: '星力健康，请保持' },
  low:        { color: 'text-yellow-400',  bg: 'bg-yellow-400',  gradient: 'from-yellow-400 to-amber-500',   label: '偏低', emoji: '🟡', desc: '星力偏低，建议充值以避免服务中断' },
  critical:   { color: 'text-orange-400',  bg: 'bg-orange-500',  gradient: 'from-orange-400 to-red-500',     label: '危急', emoji: '🟠', desc: '星力即将耗尽，请尽快充值！' },
  hibernated: { color: 'text-red-400',     bg: 'bg-red-500',     gradient: 'from-red-400 to-red-600',        label: '休眠', emoji: '🔴', desc: '星力耗尽，部分功能已暂停' },
}

const trustConfig: Record<string, { label: string; color: string; icon: string }> = {
  newcomer:   { label: '新手',   color: 'bg-gray-100 text-gray-600',     icon: '🌱' },
  verified:   { label: '已验证', color: 'bg-blue-100 text-blue-700',     icon: '✓' },
  trusted:    { label: '可信',   color: 'bg-green-100 text-green-700',   icon: '⭐' },
  veteran:    { label: '老将',   color: 'bg-purple-100 text-purple-700', icon: '🏅' },
  legendary:  { label: '传奇',   color: 'bg-amber-100 text-amber-700',  icon: '👑' },
}

const STAR_UNIT = 10000

function formatStars(units: number): string {
  const stars = units / STAR_UNIT
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
        <Loader2 className="w-6 h-6 animate-spin text-gray-400" />
      </div>
    )
  }

  const connected = credits?.connected ?? false
  const stars = credits?.balance_stars ?? 0
  const hp = credits?.hp_status ?? 'hibernated'
  const cfg = hpConfig[hp] || hpConfig.hibernated
  const trust = trustConfig[credits?.trust_level || 'newcomer'] || trustConfig.newcomer
  const pct = Math.min(100, (stars / 2000) * 100)

  const totalInStars = (credits?.total_in ?? 0) / STAR_UNIT
  const totalOutStars = (credits?.total_out ?? 0) / STAR_UNIT
  const frozenStars = credits?.frozen_stars ?? 0

  return (
    <div className="h-full overflow-y-auto">
      <div className="max-w-3xl mx-auto p-8">
        {/* Header */}
        <div className="flex items-center justify-between mb-8">
          <div>
            <h1 className="text-2xl font-bold text-gray-900 flex items-center gap-2">
              <Wallet className="w-6 h-6" /> 星力钱包
            </h1>
            <p className="text-gray-500 text-sm mt-1">算力信用余额与节点身份</p>
          </div>
          <button
            onClick={handleRefresh}
            disabled={refreshing}
            className="flex items-center gap-1.5 px-3 py-1.5 text-xs border rounded-lg hover:bg-gray-100 disabled:opacity-50 transition-colors"
          >
            <RefreshCw className={`w-3.5 h-3.5 ${refreshing ? 'animate-spin' : ''}`} />
            刷新
          </button>
        </div>

        {/* Offline Warning */}
        {!connected && (
          <div className="mb-6 p-4 bg-red-50 border border-red-200 rounded-xl flex items-center gap-3">
            <WifiOff className="w-5 h-5 text-red-500 shrink-0" />
            <div>
              <p className="text-sm font-medium text-red-800">未连接虫群</p>
              <p className="text-xs text-red-600 mt-0.5">星力余额通过虫群心跳同步，请先在设置中加入虫群。</p>
            </div>
          </div>
        )}

        {/* Main Balance Card */}
        <section className={`relative overflow-hidden rounded-2xl p-6 mb-6 bg-gradient-to-br ${connected ? 'from-gray-900 to-gray-800' : 'from-gray-400 to-gray-500'}`}>
          {/* Background decoration */}
          <div className="absolute top-0 right-0 w-64 h-64 opacity-5">
            <Star className="w-full h-full" />
          </div>

          <div className="relative">
            {/* Balance */}
            <div className="mb-4">
              <p className="text-gray-400 text-xs font-medium mb-1">可用余额</p>
              <div className="flex items-baseline gap-2">
                <span className="text-4xl font-bold text-white tracking-tight">
                  {connected ? formatStars(credits?.balance ?? 0) : '—'}
                </span>
                <span className="text-lg text-gray-400">⭐</span>
              </div>
              {connected && credits?.balance != null && (
                <p className="text-xs text-gray-500 mt-1 font-mono">{credits.balance.toLocaleString()} 单位</p>
              )}
            </div>

            {/* HP Bar */}
            <div className="mb-4">
              <div className="flex items-center justify-between mb-1.5">
                <span className={`text-xs font-medium ${cfg.color}`}>
                  {cfg.emoji} HP: {cfg.label}
                </span>
                <span className="text-xs text-gray-500">{Math.round(pct)}%</span>
              </div>
              <div className="w-full h-2.5 bg-gray-700/50 rounded-full overflow-hidden">
                <div
                  className={`h-full bg-gradient-to-r ${cfg.gradient} rounded-full transition-all duration-1000 ease-out`}
                  style={{ width: `${Math.max(connected ? pct : 0, 2)}%` }}
                />
              </div>
              <p className="text-xs text-gray-500 mt-1.5">{cfg.desc}</p>
            </div>

            {/* Trust + Status badges + Top Up */}
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-2">
                <span className={`inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs font-medium ${trust.color}`}>
                  {trust.icon} {trust.label}
                </span>
                {credits?.status && (
                  <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs font-medium bg-gray-700 text-gray-300">
                    <Shield className="w-3 h-3" /> {credits.status}
                  </span>
                )}
              </div>
              {nodeInfo?.node_id && (
                <a
                  href={`https://star-ai.net/billing?claw_id=${encodeURIComponent(nodeInfo.node_id)}`}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="inline-flex items-center gap-1.5 px-4 py-1.5 rounded-lg bg-gradient-to-r from-amber-400 to-orange-500 text-white text-xs font-semibold hover:from-amber-500 hover:to-orange-600 transition-all shadow-lg shadow-orange-500/25"
                >
                  <ExternalLink className="w-3.5 h-3.5" /> 充值星力
                </a>
              )}
            </div>
          </div>
        </section>

        {/* Stats Grid */}
        <div className="grid grid-cols-3 gap-4 mb-6">
          <div className="bg-white border rounded-xl p-4">
            <div className="flex items-center gap-2 mb-2">
              <div className="w-7 h-7 rounded-lg bg-green-50 flex items-center justify-center">
                <ArrowDownRight className="w-4 h-4 text-green-600" />
              </div>
              <span className="text-xs text-gray-500">总收入</span>
            </div>
            <p className="text-lg font-bold text-gray-900">
              {connected ? `${totalInStars.toFixed(1)}` : '—'} <span className="text-xs text-gray-400">⭐</span>
            </p>
          </div>
          <div className="bg-white border rounded-xl p-4">
            <div className="flex items-center gap-2 mb-2">
              <div className="w-7 h-7 rounded-lg bg-red-50 flex items-center justify-center">
                <ArrowUpRight className="w-4 h-4 text-red-500" />
              </div>
              <span className="text-xs text-gray-500">总支出</span>
            </div>
            <p className="text-lg font-bold text-gray-900">
              {connected ? `${totalOutStars.toFixed(1)}` : '—'} <span className="text-xs text-gray-400">⭐</span>
            </p>
          </div>
          <div className="bg-white border rounded-xl p-4">
            <div className="flex items-center gap-2 mb-2">
              <div className="w-7 h-7 rounded-lg bg-blue-50 flex items-center justify-center">
                <Snowflake className="w-4 h-4 text-blue-500" />
              </div>
              <span className="text-xs text-gray-500">冻结</span>
            </div>
            <p className="text-lg font-bold text-gray-900">
              {connected ? `${frozenStars.toFixed(1)}` : '—'} <span className="text-xs text-gray-400">⭐</span>
            </p>
          </div>
        </div>

        {/* Node Identity */}
        {nodeInfo && (
          <section className="bg-white border rounded-xl p-6 mb-6">
            <h2 className="text-sm font-semibold text-gray-700 flex items-center gap-2 mb-4">
              <Shield className="w-4 h-4" /> 节点身份
            </h2>
            <div className="space-y-3">
              {/* Claw Address */}
              <div className="flex items-center justify-between p-3 bg-gray-50 rounded-lg">
                <div>
                  <p className="text-xs text-gray-500 mb-0.5">Claw 地址（钱包地址）</p>
                  <p className="font-mono text-sm text-gray-800">{nodeInfo.node_id}</p>
                </div>
                <button
                  onClick={() => copyToClipboard(nodeInfo.node_id, 'claw_id')}
                  className="p-2 text-gray-400 hover:text-gray-600 transition-colors"
                  title="复制地址"
                >
                  {copied === 'claw_id' ? <Check className="w-4 h-4 text-green-500" /> : <Copy className="w-4 h-4" />}
                </button>
              </div>
              {/* Fingerprint */}
              {nodeInfo.fingerprint && (
                <div className="flex items-center justify-between p-3 bg-gray-50 rounded-lg">
                  <div>
                    <p className="text-xs text-gray-500 mb-0.5">基因指纹</p>
                    <p className="font-mono text-xs text-gray-600 truncate max-w-md">{nodeInfo.fingerprint}</p>
                  </div>
                  <button
                    onClick={() => copyToClipboard(nodeInfo.fingerprint, 'fingerprint')}
                    className="p-2 text-gray-400 hover:text-gray-600 transition-colors shrink-0"
                  >
                    {copied === 'fingerprint' ? <Check className="w-4 h-4 text-green-500" /> : <Copy className="w-4 h-4" />}
                  </button>
                </div>
              )}
              {/* Network address */}
              {nodeInfo.address && (
                <div className="flex items-center justify-between p-3 bg-gray-50 rounded-lg">
                  <div>
                    <p className="text-xs text-gray-500 mb-0.5">Nydus 地址</p>
                    <p className="font-mono text-sm text-gray-700">{nodeInfo.address}</p>
                  </div>
                  <button
                    onClick={() => copyToClipboard(nodeInfo.address, 'address')}
                    className="p-2 text-gray-400 hover:text-gray-600 transition-colors"
                  >
                    {copied === 'address' ? <Check className="w-4 h-4 text-green-500" /> : <Copy className="w-4 h-4" />}
                  </button>
                </div>
              )}
            </div>
          </section>
        )}

        {/* Star Credits Explainer */}
        <section className="bg-white border rounded-xl p-6">
          <h2 className="text-sm font-semibold text-gray-700 flex items-center gap-2 mb-3">
            <Star className="w-4 h-4" /> 关于星力 (Star Credits)
          </h2>
          <div className="text-xs text-gray-500 space-y-2 leading-relaxed">
            <p>
              <strong className="text-gray-700">星力 ⭐</strong> 是 StarClaw 虫群网络的算力信用单位。
              1 ⭐ = 10,000 内部单位，精度为 4 位小数。
            </p>
            <p>
              每个新注册的 Claw 节点加入虫群时自动获得 <strong className="text-gray-700">100 ⭐ 欢迎奖励</strong>。
              星力可用于支付 API 调用费用、发布赏金任务、以及算力贡献结算。
              你也可以在 <a href="https://star-ai.net/billing" target="_blank" rel="noopener noreferrer" className="text-indigo-600 hover:underline font-medium">star-ai.net</a> 直接充值星力。
            </p>
            <div className="mt-3 p-3 bg-amber-50 rounded-lg border border-amber-100">
              <p className="text-amber-800 font-medium">HP 状态等级</p>
              <div className="mt-2 grid grid-cols-5 gap-1.5">
                {Object.entries(hpConfig).map(([key, val]) => (
                  <div key={key} className="text-center">
                    <span className="text-base">{val.emoji}</span>
                    <p className="text-[10px] text-gray-600 mt-0.5">{val.label}</p>
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
