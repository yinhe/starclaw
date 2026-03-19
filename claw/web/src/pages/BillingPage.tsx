import { useState, useEffect } from 'react'
import { CreditCard, Users, BarChart3, Receipt, Plus, Trash2, Shield, ArrowUpRight, ArrowDownRight, Zap, Snowflake, Copy, Check, TrendingUp, Award, RefreshCw, WifiOff } from 'lucide-react'
import { billingAPI, tenantAPI, systemAPI, nodeAPI } from '../lib/api'

interface Tenant {
  id: string
  name: string
  owner_id: string
  balance: number
}

interface Member {
  id: string
  user_id: string
  username: string
  email: string
  role: string
  joined_at: string
}

interface UsageItem {
  month: string
  resource_type: string
  total: number
  total_cost: number
}

interface CreditTransaction {
  id: string
  from_claw: string
  to_claw: string
  type: string
  amount: number
  fee?: number
  remark: string
  status?: string
  created_at: string
}

interface CreditData {
  balance: number
  balance_energy?: number
  frozen?: number
  total_in: number
  total_out: number
  frozen_energy: number
  connected: boolean
  hp_status: string
  status?: string
  trust_level?: string
  updated_at: string
}

type TabType = 'overview' | 'usage' | 'transactions' | 'team'
type DirectionFilter = 'all' | 'in' | 'out'
type TxnTypeFilter = 'all' | 'transfer'

const resourceLabels: Record<string, string> = {
  tokens: 'Tokens', video: '视频', image: '图片', music: '音乐',
}

const hpLabels: Record<string, string> = {
  full: '充沛',
  healthy: '健康',
  low: '偏低',
  critical: '危急',
  hibernated: '休眠',
}

const hpConfig: Record<string, { color: string; textDark: string; gradient: string; glow: string; label: string; desc: string; border: string; bg: string }> = {
  full: { color: 'text-emerald-400', textDark: 'text-emerald-600', gradient: 'from-emerald-400 to-green-500', glow: 'shadow-emerald-500/50', label: '充沛', desc: '星能充足，所有功能满血运行', border: 'border-emerald-200', bg: 'bg-emerald-50' },
  healthy: { color: 'text-green-400', textDark: 'text-green-600', gradient: 'from-green-400 to-emerald-500', glow: 'shadow-green-400/40', label: '健康', desc: '星能健康，请保持', border: 'border-green-200', bg: 'bg-green-50' },
  low: { color: 'text-yellow-400', textDark: 'text-yellow-600', gradient: 'from-yellow-400 to-amber-500', glow: 'shadow-yellow-400/40', label: '偏低', desc: '星能偏低，建议充值', border: 'border-yellow-200', bg: 'bg-yellow-50' },
  critical: { color: 'text-orange-400', textDark: 'text-orange-600', gradient: 'from-orange-400 to-red-500', glow: 'shadow-orange-500/50', label: '危急', desc: '星能即将耗尽，请尽快充值', border: 'border-orange-200', bg: 'bg-orange-50' },
  hibernated: { color: 'text-red-400', textDark: 'text-red-600', gradient: 'from-red-400 to-red-600', glow: 'shadow-red-500/50', label: '休眠', desc: '星能耗尽，部分功能已暂停', border: 'border-red-200', bg: 'bg-red-50' },
}

const trustConfig: Record<string, { label: string; color: string; border: string; icon: typeof Award }> = {
  newcomer: { label: '新手', color: 'text-gray-500', border: 'border-gray-200', icon: Award },
  verified: { label: '已验证', color: 'text-blue-600', border: 'border-blue-200', icon: Award },
  trusted: { label: '可信', color: 'text-green-600', border: 'border-green-200', icon: Award },
  veteran: { label: '老将', color: 'text-purple-600', border: 'border-purple-200', icon: Award },
  legendary: { label: '传奇', color: 'text-amber-600', border: 'border-amber-200', icon: Award },
}

const ENERGY_UNIT = 10000

function formatEnergy(units: number): string {
  const stars = units / ENERGY_UNIT
  if (stars >= 10000) return `${(stars / 10000).toFixed(2)}万`
  if (stars >= 1000) return `${(stars / 1000).toFixed(2)}K`
  return stars.toFixed(2)
}

export default function BillingPage() {
  const [tab, setTab] = useState<TabType>('overview')
  const [tenant, setTenant] = useState<Tenant | null>(null)
  const [usage, setUsage] = useState<Record<string, number>>({})
  const [cost, setCost] = useState<Record<string, number>>({})
  const [period, setPeriod] = useState('')
  const [credits, setCredits] = useState<CreditData | null>(null)
  const [nodeInfo, setNodeInfo] = useState<any>(null)
  const [members, setMembers] = useState<Member[]>([])
  const [usageHistory, setUsageHistory] = useState<UsageItem[]>([])
  const [transactions, setTransactions] = useState<CreditTransaction[]>([])
  const [directionFilter, setDirectionFilter] = useState<DirectionFilter>('all')
  const [txnTypeFilter, setTxnTypeFilter] = useState<TxnTypeFilter>('all')
  const [refreshing, setRefreshing] = useState(false)
  const [copied, setCopied] = useState('')
  const [inviteEmail, setInviteEmail] = useState('')
  const [inviteRole, setInviteRole] = useState('member')
  const [teamName, setTeamName] = useState('')
  const [editingName, setEditingName] = useState(false)

  useEffect(() => { loadData() }, [])

  const loadData = async () => {
    try {
      const [planRes, creditsRes, nodeRes] = await Promise.all([
        billingAPI.getCurrentPlan().catch(() => null),
        systemAPI.getCredits({ refresh: true } as any).catch(() => null),
        nodeAPI.getInfo().catch(() => null),
      ])
      if (planRes?.data) {
        setTenant(planRes.data.tenant)
        setUsage(planRes.data.usage || {})
        setCost(planRes.data.cost || {})
        setPeriod(planRes.data.period || '')
        setTeamName(planRes.data.tenant?.name || '')
      }
      if (creditsRes?.data) setCredits(creditsRes.data)
      if (nodeRes?.data) setNodeInfo(nodeRes.data)
    } finally {
    }
  }

  const loadTeam = async () => {
    const res = await tenantAPI.get().catch(() => null)
    if (res?.data) {
      setMembers(res.data.members || [])
      setTenant(res.data.tenant)
    }
  }

  const loadUsageHistory = async () => {
    const res = await billingAPI.getUsageHistory().catch(() => null)
    if (res?.data) setUsageHistory(res.data.usage || [])
  }

  const loadTransactions = async () => {
    const res = await systemAPI.getCreditTransactions({ page: 1, page_size: 50, type: txnTypeFilter === 'all' ? undefined : txnTypeFilter }).catch(() => null)
    if (res?.data) setTransactions(res.data.transactions || [])
  }

  useEffect(() => {
    if (tab === 'team') loadTeam()
    if (tab === 'overview') loadData()
    if (tab === 'usage') loadUsageHistory()
    if (tab === 'transactions') loadTransactions()
  }, [tab, txnTypeFilter])

  const handleRefresh = async () => {
    setRefreshing(true)
    await Promise.all([loadData(), loadTransactions()])
    setRefreshing(false)
  }

  const copyToClipboard = (text: string, label: string) => {
    navigator.clipboard.writeText(text)
    setCopied(label)
    setTimeout(() => setCopied(''), 2000)
  }

  const handleInvite = async () => {
    if (!inviteEmail) return
    try {
      await tenantAPI.addMember(inviteEmail, inviteRole)
      setInviteEmail('')
      loadTeam()
    } catch (e: any) {
      alert(e.response?.data?.error || '邀请失败')
    }
  }

  const handleRemoveMember = async (userId: string) => {
    if (!confirm('确定移除此成员？')) return
    await tenantAPI.removeMember(userId)
    loadTeam()
  }

  const handleUpdateName = async () => {
    if (!teamName.trim()) return
    await tenantAPI.update({ name: teamName })
    setEditingName(false)
    loadData()
  }

  const formatNum = (n: number) => {
    if (n >= 1_000_000) return (n / 1_000_000).toFixed(1) + 'M'
    if (n >= 1_000) return (n / 1_000).toFixed(1) + 'K'
    return n.toString()
  }

  const totalCost = Object.values(cost).reduce((s, v) => s + v, 0)

  const connected = credits?.connected ?? false
  const totalInStars = (credits?.total_in ?? 0) / ENERGY_UNIT
  const totalOutStars = (credits?.total_out ?? 0) / ENERGY_UNIT
  const frozenStars = credits?.frozen_energy ?? 0
  const hp = credits?.hp_status ?? 'hibernated'
  const hpMeta = hpConfig[hp] || hpConfig.hibernated
  const trust = trustConfig[credits?.trust_level || 'newcomer'] || trustConfig.newcomer
  const pct = Math.min(100, ((credits?.balance_energy ?? 0) / 2000) * 100)
  const filteredTransactions = transactions.filter((tx) => {
    if (!nodeInfo?.node_id || directionFilter === 'all') return true
    if (directionFilter === 'in') return tx.to_claw === nodeInfo.node_id
    if (directionFilter === 'out') return tx.from_claw === nodeInfo.node_id
    return true
  })

  const tabs: { key: TabType; label: string; icon: typeof CreditCard }[] = [
    { key: 'overview', label: '概览', icon: Zap },
    { key: 'usage', label: '用量', icon: BarChart3 },
    { key: 'transactions', label: '流水', icon: Receipt },
    { key: 'team', label: '团队', icon: Users },
  ]

  return (
    <div className="h-full overflow-y-auto">
      <div className="max-w-5xl mx-auto px-6 py-6">
        <div className="mb-6">
          <h1 className="text-2xl font-bold text-gray-900 dark:text-white">星能中心</h1>
          <p className="text-sm text-gray-500 mt-1">查看真实星能余额、消耗用量和团队协作信息</p>
        </div>

        <div className="mb-6 rounded-xl border border-amber-200 bg-amber-50 px-4 py-3 dark:border-amber-800/40 dark:bg-amber-950/20">
          <div className="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
            <div>
              <p className="text-sm font-semibold text-amber-800 dark:text-amber-300">充值入口已统一收敛到 StarAI</p>
              <p className="mt-1 text-sm text-amber-700 dark:text-amber-400">Claw 这里展示真实星能与真实用量。需要购买星能时，请前往 StarAI 完成统一充值。</p>
            </div>
            <a
              href={nodeInfo?.node_id ? `https://star-ai.net/login?claw_url=${encodeURIComponent(window.location.origin)}&claw_id=${encodeURIComponent(nodeInfo.node_id)}` : 'https://star-ai.net/billing'}
              target="_blank"
              rel="noopener noreferrer"
              className="inline-flex items-center justify-center gap-2 rounded-lg bg-amber-500 px-4 py-2 text-sm font-medium text-white hover:bg-amber-600"
            >
              去 StarAI 充值
              <ArrowUpRight className="h-4 w-4" />
            </a>
          </div>
        </div>

        {/* Tabs */}
        <div className="flex gap-1 mb-6 bg-gray-100 dark:bg-gray-800 rounded-lg p-1 w-fit">
          {tabs.map(t => (
            <button
              key={t.key}
              onClick={() => setTab(t.key)}
              className={`flex items-center gap-1.5 px-4 py-2 rounded-md text-sm font-medium transition-colors ${
                tab === t.key
                  ? 'bg-white dark:bg-gray-700 text-gray-900 dark:text-white shadow-sm'
                  : 'text-gray-500 hover:text-gray-700 dark:hover:text-gray-300'
              }`}
            >
              <t.icon className="w-4 h-4" />
              {t.label}
            </button>
          ))}
        </div>

        {/* Overview Tab */}
        {tab === 'overview' && (
          <div className="space-y-6">
            {!connected && (
              <div className="rounded-xl border border-red-200 bg-red-50 px-4 py-3 dark:border-red-800/40 dark:bg-red-950/20 flex items-center justify-between gap-3">
                <div className="flex items-center gap-3">
                  <WifiOff className="h-5 w-5 text-red-500" />
                  <div>
                    <p className="text-sm font-medium text-red-800 dark:text-red-300">未连接虫群</p>
                    <p className="text-xs text-red-600 dark:text-red-400">星能余额通过虫群心跳同步，请先加入虫群。</p>
                  </div>
                </div>
                <a href="/settings" className="shrink-0 rounded-lg bg-red-500 px-4 py-2 text-sm font-medium text-white hover:bg-red-600">加入虫群</a>
              </div>
            )}

            <div className={`relative overflow-hidden rounded-2xl p-6 text-white ${connected ? 'bg-gradient-to-br from-gray-900 via-gray-800 to-gray-900' : 'bg-gradient-to-br from-gray-500 to-gray-600'}`}>
              <div className="absolute -top-8 -right-8 h-48 w-48 opacity-[0.04]"><Zap className="h-full w-full" fill="white" /></div>
              <div className="relative">
                <div className="mb-5 flex items-center justify-between gap-4">
                  <div>
                    <p className="text-xs font-medium uppercase tracking-wider text-gray-300">真实星能余额</p>
                    <div className="mt-2 flex items-end gap-3">
                      <span className="text-5xl font-extrabold tracking-tight">{connected ? formatEnergy(credits?.balance ?? 0) : '—'}</span>
                      <Zap className="mb-1 h-5 w-5 text-amber-400" fill="currentColor" />
                    </div>
                    {connected && credits?.balance != null && <p className="mt-1.5 text-xs font-mono text-gray-400">{credits.balance.toLocaleString()} 内部单位</p>}
                  </div>
                  <button onClick={handleRefresh} disabled={refreshing} className="inline-flex items-center gap-1.5 rounded-lg border border-gray-600 px-3 py-1.5 text-xs text-white hover:bg-white/5 disabled:opacity-50">
                    <RefreshCw className={`h-3.5 w-3.5 ${refreshing ? 'animate-spin' : ''}`} /> 刷新
                  </button>
                </div>

                <div className="mb-5">
                  <div className="mb-2 flex items-center justify-between">
                    <div className="flex items-center gap-2">
                      <Zap className={`h-4 w-4 ${hpMeta.color}`} fill="currentColor" />
                      <span className={`text-sm font-bold ${hpMeta.color}`}>HP · {hpMeta.label}</span>
                    </div>
                    <span className="text-sm font-mono text-gray-300">{Math.round(pct)}%</span>
                  </div>
                  <div className="h-3 w-full overflow-hidden rounded-full bg-gray-700/60 shadow-inner">
                    <div className={`h-full rounded-full bg-gradient-to-r ${hpMeta.gradient} shadow-lg ${hpMeta.glow}`} style={{ width: `${Math.max(connected ? pct : 0, 2)}%` }} />
                  </div>
                  <p className="mt-2 text-xs text-gray-400">{hpMeta.desc}</p>
                </div>

                <div className="flex flex-wrap items-center justify-between gap-3">
                  <div className="flex items-center gap-2">
                    <span className={`inline-flex items-center gap-1 rounded-full border px-2.5 py-1 text-xs font-medium ${trust.border} bg-white/10 ${trust.color}`}>
                      <trust.icon className="h-3 w-3" /> {trust.label}
                    </span>
                    {credits?.status && <span className="inline-flex items-center gap-1 rounded-full border border-gray-600 bg-gray-700/50 px-2.5 py-1 text-xs font-medium text-gray-300"><Shield className="h-3 w-3" /> {credits.status}</span>}
                  </div>
                  {nodeInfo?.node_id && (
                    <a href={`https://star-ai.net/login?claw_url=${encodeURIComponent(window.location.origin)}&claw_id=${encodeURIComponent(nodeInfo.node_id)}`} target="_blank" rel="noopener noreferrer" className="inline-flex items-center gap-1.5 rounded-xl bg-gradient-to-r from-amber-400 to-orange-500 px-4 py-2 text-xs font-bold text-white shadow-lg shadow-orange-500/30 hover:from-amber-500 hover:to-orange-600">
                      <Zap className="h-3.5 w-3.5" fill="white" /> 充值星能
                    </a>
                  )}
                </div>
              </div>
            </div>

            <div className="grid grid-cols-1 gap-4 md:grid-cols-3">
              <div className="rounded-xl border border-gray-200 bg-white p-4 dark:border-gray-700 dark:bg-gray-800"><div className="mb-2 flex items-center gap-2"><div className="flex h-8 w-8 items-center justify-center rounded-lg bg-emerald-50 dark:bg-emerald-900/20"><ArrowDownRight className="h-4 w-4 text-emerald-500" /></div><span className="text-xs text-gray-500">总收入</span></div><p className="text-lg font-bold text-gray-900 dark:text-white">{connected ? `${totalInStars.toFixed(1)}` : '—'}<span className="ml-1 text-xs text-gray-400">⚡</span></p></div>
              <div className="rounded-xl border border-gray-200 bg-white p-4 dark:border-gray-700 dark:bg-gray-800"><div className="mb-2 flex items-center gap-2"><div className="flex h-8 w-8 items-center justify-center rounded-lg bg-red-50 dark:bg-red-900/20"><ArrowUpRight className="h-4 w-4 text-red-500" /></div><span className="text-xs text-gray-500">总支出</span></div><p className="text-lg font-bold text-gray-900 dark:text-white">{connected ? `${totalOutStars.toFixed(1)}` : '—'}<span className="ml-1 text-xs text-gray-400">⚡</span></p></div>
              <div className="rounded-xl border border-gray-200 bg-white p-4 dark:border-gray-700 dark:bg-gray-800"><div className="mb-2 flex items-center gap-2"><div className="flex h-8 w-8 items-center justify-center rounded-lg bg-blue-50 dark:bg-blue-900/20"><Snowflake className="h-4 w-4 text-blue-500" /></div><span className="text-xs text-gray-500">冻结中</span></div><p className="text-lg font-bold text-gray-900 dark:text-white">{connected ? `${frozenStars.toFixed(1)}` : '—'}<span className="ml-1 text-xs text-gray-400">⚡</span></p></div>
            </div>

            {nodeInfo && (
              <div className="rounded-xl border border-gray-200 bg-white p-6 dark:border-gray-700 dark:bg-gray-800">
                <h3 className="mb-4 flex items-center gap-2 text-sm font-semibold text-gray-700 dark:text-gray-200"><Shield className="h-4 w-4 text-indigo-500" /> 节点身份</h3>
                <div className="space-y-3">
                  <div className="flex items-center justify-between rounded-lg bg-gray-50 p-3 dark:bg-gray-700/50"><div><p className="mb-0.5 text-xs text-gray-500">Claw 地址（钱包地址）</p><p className="font-mono text-sm text-gray-800 dark:text-gray-200">{nodeInfo.node_id}</p></div><button onClick={() => copyToClipboard(nodeInfo.node_id, 'claw_id')} className="p-2 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300">{copied === 'claw_id' ? <Check className="h-4 w-4 text-green-500" /> : <Copy className="h-4 w-4" />}</button></div>
                  {nodeInfo.fingerprint && <div className="flex items-center justify-between rounded-lg bg-gray-50 p-3 dark:bg-gray-700/50"><div><p className="mb-0.5 text-xs text-gray-500">基因指纹</p><p className="max-w-md truncate font-mono text-xs text-gray-600 dark:text-gray-400">{nodeInfo.fingerprint}</p></div><button onClick={() => copyToClipboard(nodeInfo.fingerprint, 'fingerprint')} className="shrink-0 p-2 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300">{copied === 'fingerprint' ? <Check className="h-4 w-4 text-green-500" /> : <Copy className="h-4 w-4" />}</button></div>}
                  {nodeInfo.address && <div className="flex items-center justify-between rounded-lg bg-gray-50 p-3 dark:bg-gray-700/50"><div><p className="mb-0.5 text-xs text-gray-500">Nydus 地址</p><p className="font-mono text-sm text-gray-700 dark:text-gray-300">{nodeInfo.address}</p></div><button onClick={() => copyToClipboard(nodeInfo.address, 'address')} className="p-2 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300">{copied === 'address' ? <Check className="h-4 w-4 text-green-500" /> : <Copy className="h-4 w-4" />}</button></div>}
                </div>
              </div>
            )}

            <div className="rounded-xl border border-gray-200 bg-white p-6 dark:border-gray-700 dark:bg-gray-800">
              <h3 className="mb-3 flex items-center gap-2 text-sm font-semibold text-gray-700 dark:text-gray-200"><Zap className="h-4 w-4 text-amber-500" fill="currentColor" /> 关于星能</h3>
              <div className="space-y-2 text-xs leading-relaxed text-gray-500 dark:text-gray-400">
                <p><strong className="text-gray-700 dark:text-gray-200">星能 ⚡</strong> 是 StarClaw 虫群网络的算力能量单位。1 ⚡ = 10,000 内部单位。</p>
                <p>星能用于 API 调用、赏金任务、算力贡献结算。在 <a href="https://star-ai.net/billing" target="_blank" rel="noopener noreferrer" className="font-medium text-amber-600 hover:underline">star-ai.net</a> 可直接充值。</p>
                <div className="mt-3 rounded-lg border border-indigo-100 bg-indigo-50 p-3 dark:border-indigo-800/30 dark:bg-indigo-950/30">
                  <p className="flex items-center gap-1.5 font-medium text-indigo-700 dark:text-indigo-300"><TrendingUp className="h-3.5 w-3.5" /> 获取星能的方式</p>
                  <div className="mt-2 grid grid-cols-3 gap-2 text-[10px]">{['充值购买', '推理挖矿', '赏金完成', '模板出售', 'P2P 转账', '邀请奖励'].map(way => <div key={way} className="flex items-center gap-1 text-indigo-600 dark:text-indigo-400"><Zap className="h-2.5 w-2.5" /> {way}</div>)}</div>
                </div>
              </div>
            </div>
          </div>
        )}

        {/* Usage Tab */}
        {tab === 'usage' && (
          <div className="space-y-6">
            <div className={`rounded-2xl p-6 text-white ${connected ? 'bg-gradient-to-r from-gray-900 via-gray-800 to-gray-900' : 'bg-gradient-to-r from-gray-500 to-gray-600'}`}>
              <div className="flex flex-col gap-5 lg:flex-row lg:items-start lg:justify-between">
                <div>
                  <p className="text-xs font-medium uppercase tracking-wider text-gray-300">真实星能余额</p>
                  <div className="mt-2 flex items-end gap-3">
                    <span className="text-4xl font-extrabold tracking-tight">{connected ? formatEnergy(credits?.balance ?? 0) : '—'}</span>
                    <Zap className="mb-1 h-5 w-5 text-amber-400" fill="currentColor" />
                  </div>
                  <p className="mt-2 text-sm text-gray-300">
                    {connected
                      ? `状态：${hpLabels[credits?.hp_status || 'hibernated'] || '未知'}${credits?.updated_at ? ` · 更新于 ${new Date(credits.updated_at).toLocaleString()}` : ''}`
                      : '当前未同步到 Queen 星能余额'}
                  </p>
                </div>
                <div className="grid grid-cols-3 gap-3 lg:min-w-[360px]">
                  <div className="rounded-xl bg-white/10 px-4 py-3">
                    <p className="text-xs text-gray-300">累计收入</p>
                    <p className="mt-1 text-lg font-bold">{connected ? totalInStars.toFixed(1) : '—'}<span className="ml-1 text-xs text-gray-300">⚡</span></p>
                  </div>
                  <div className="rounded-xl bg-white/10 px-4 py-3">
                    <p className="text-xs text-gray-300">累计支出</p>
                    <p className="mt-1 text-lg font-bold">{connected ? totalOutStars.toFixed(1) : '—'}<span className="ml-1 text-xs text-gray-300">⚡</span></p>
                  </div>
                  <div className="rounded-xl bg-white/10 px-4 py-3">
                    <p className="text-xs text-gray-300">冻结中</p>
                    <p className="mt-1 text-lg font-bold">{connected ? frozenStars.toFixed(1) : '—'}<span className="ml-1 text-xs text-gray-300">⚡</span></p>
                  </div>
                </div>
              </div>
            </div>

            <div className="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-5">
              <div className="flex items-start justify-between gap-4">
                <div>
                  <h3 className="font-semibold text-gray-900 dark:text-white mb-1 flex items-center gap-2">
                    <BarChart3 className="w-4 h-4 text-blue-500" /> 本月总消耗
                  </h3>
                  <p className="text-3xl font-bold text-gray-900 dark:text-white">¥{totalCost.toFixed(2)}</p>
                  <p className="text-sm text-gray-500 mt-1">统计周期：{period || '--'}</p>
                </div>
                {tenant && (
                  <div className="text-right text-sm text-gray-500">
                    <p>团队：<span className="font-medium text-gray-900 dark:text-white">{tenant.name}</span></p>
                  </div>
                )}
              </div>
            </div>

            <div className="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-5">
              <h3 className="font-semibold text-gray-900 dark:text-white mb-4 flex items-center gap-2">
                <BarChart3 className="w-4 h-4 text-blue-500" /> 本月用量 ({period})
              </h3>
              <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
                {['tokens', 'video', 'image', 'music'].map(key => (
                  <div key={key} className="text-center p-4 rounded-lg bg-gray-50 dark:bg-gray-700/50">
                    <p className="text-2xl font-bold text-gray-900 dark:text-white">{formatNum(usage[key] || 0)}</p>
                    <p className="text-xs text-gray-500 mt-1">{resourceLabels[key]}</p>
                    <p className="text-xs text-emerald-500 mt-1">¥{(cost[key] || 0).toFixed(2)}</p>
                  </div>
                ))}
              </div>
            </div>

            {usageHistory.length > 0 && (
              <div className="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-5">
                <h3 className="font-semibold text-gray-900 dark:text-white mb-4">历史用量</h3>
                <div className="overflow-x-auto">
                  <table className="w-full text-sm">
                    <thead>
                      <tr className="text-left text-gray-500 border-b dark:border-gray-700">
                        <th className="pb-2 font-medium">月份</th>
                        <th className="pb-2 font-medium">类型</th>
                        <th className="pb-2 font-medium text-right">用量</th>
                        <th className="pb-2 font-medium text-right">费用</th>
                      </tr>
                    </thead>
                    <tbody>
                      {usageHistory.map((item, idx) => (
                        <tr key={idx} className="border-b dark:border-gray-700/50">
                          <td className="py-2 text-gray-900 dark:text-white">{item.month}</td>
                          <td className="py-2 text-gray-600 dark:text-gray-400">{resourceLabels[item.resource_type] || item.resource_type}</td>
                          <td className="py-2 text-right font-medium text-gray-900 dark:text-white">{formatNum(item.total)}</td>
                          <td className="py-2 text-right text-emerald-600">¥{(item.total_cost || 0).toFixed(2)}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              </div>
            )}
          </div>
        )}

        {/* Transactions Tab */}
        {tab === 'transactions' && (
          <div className="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-5">
            <div className="mb-4 flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
              <h3 className="font-semibold text-gray-900 dark:text-white flex items-center gap-2">
                <Receipt className="w-4 h-4 text-blue-500" /> 交易流水
              </h3>
              <div className="flex flex-wrap items-center gap-2">
                <div className="flex gap-1 rounded-lg bg-gray-100 p-1 dark:bg-gray-700/50">
                  {[
                    { key: 'all', label: '全部' },
                    { key: 'in', label: '转入' },
                    { key: 'out', label: '转出' },
                  ].map((item) => (
                    <button
                      key={item.key}
                      onClick={() => setDirectionFilter(item.key as DirectionFilter)}
                      className={`rounded-md px-3 py-1 text-xs font-medium ${directionFilter === item.key ? 'bg-white text-gray-900 shadow-sm dark:bg-gray-800 dark:text-white' : 'text-gray-500 hover:text-gray-700 dark:hover:text-gray-300'}`}
                    >
                      {item.label}
                    </button>
                  ))}
                </div>
                <select
                  value={txnTypeFilter}
                  onChange={(e) => setTxnTypeFilter(e.target.value as TxnTypeFilter)}
                  className="rounded-lg border border-gray-200 px-3 py-2 text-xs dark:border-gray-600 dark:bg-gray-700 dark:text-white"
                >
                  <option value="all">全部类型</option>
                  <option value="transfer">Transfer</option>
                </select>
              </div>
            </div>
            {filteredTransactions.length === 0 ? (
              <p className="text-sm text-gray-400 py-8 text-center">暂无星能流水</p>
            ) : (
              <div className="space-y-2">
                {filteredTransactions.map(tx => (
                  <div key={tx.id} className="flex items-center justify-between py-2.5 px-3 rounded-lg hover:bg-gray-50 dark:hover:bg-gray-700/50">
                    <div className="flex items-center gap-3">
                      <div className={`w-8 h-8 rounded-full flex items-center justify-center ${
                        tx.to_claw === nodeInfo?.node_id ? 'bg-emerald-100 dark:bg-emerald-900/30' : 'bg-orange-100 dark:bg-orange-900/30'
                      }`}>
                        {tx.to_claw === nodeInfo?.node_id
                          ? <ArrowDownRight className="w-4 h-4 text-emerald-600" />
                          : <ArrowUpRight className="w-4 h-4 text-orange-600" />
                        }
                      </div>
                      <div>
                        <p className="text-sm text-gray-900 dark:text-white">{tx.remark || tx.type || '星能交易'}</p>
                        <p className="text-xs text-gray-500">{tx.from_claw} → {tx.to_claw}</p>
                        <p className="text-xs text-gray-400">{new Date(tx.created_at).toLocaleString('zh-CN')}</p>
                      </div>
                    </div>
                    <div className="text-right">
                      <p className={`text-sm font-medium ${tx.to_claw === nodeInfo?.node_id ? 'text-emerald-600' : 'text-orange-600'}`}>
                        {tx.to_claw === nodeInfo?.node_id ? '+' : '-'}{(Math.abs(tx.amount) / ENERGY_UNIT).toFixed(4)} ⚡
                      </p>
                      <p className="text-xs text-gray-400">{tx.status || 'confirmed'}{tx.fee ? ` · 手续费 ${(tx.fee / ENERGY_UNIT).toFixed(4)} ⚡` : ''}</p>
                    </div>
                  </div>
                ))}
              </div>
            )}
          </div>
        )}

        {/* Team Tab */}
        {tab === 'team' && (
          <div className="space-y-6">
            <div className="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-5">
              <h3 className="font-semibold text-gray-900 dark:text-white mb-3 flex items-center gap-2">
                <Shield className="w-4 h-4 text-violet-500" /> 团队信息
              </h3>
              <div className="flex items-center gap-3">
                {editingName ? (
                  <>
                    <input
                      value={teamName}
                      onChange={e => setTeamName(e.target.value)}
                      className="flex-1 text-sm border rounded-lg px-3 py-1.5 dark:bg-gray-700 dark:border-gray-600 dark:text-white"
                      autoFocus
                    />
                    <button onClick={handleUpdateName} className="text-sm px-3 py-1.5 bg-violet-500 text-white rounded-lg hover:bg-violet-600">保存</button>
                    <button onClick={() => setEditingName(false)} className="text-sm text-gray-500 hover:text-gray-700">取消</button>
                  </>
                ) : (
                  <>
                    <span className="text-gray-900 dark:text-white font-medium">{tenant?.name}</span>
                    <button onClick={() => setEditingName(true)} className="text-xs text-violet-500 hover:text-violet-600">编辑</button>
                  </>
                )}
              </div>
            </div>

            <div className="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-5">
              <h3 className="font-semibold text-gray-900 dark:text-white mb-3">邀请成员</h3>
              <div className="flex gap-2">
                <input
                  type="email"
                  placeholder="输入邮箱地址"
                  value={inviteEmail}
                  onChange={e => setInviteEmail(e.target.value)}
                  className="flex-1 text-sm border rounded-lg px-3 py-2 dark:bg-gray-700 dark:border-gray-600 dark:text-white"
                />
                <select
                  value={inviteRole}
                  onChange={e => setInviteRole(e.target.value)}
                  className="text-sm border rounded-lg px-3 py-2 dark:bg-gray-700 dark:border-gray-600 dark:text-white"
                >
                  <option value="member">成员</option>
                  <option value="admin">管理员</option>
                </select>
                <button
                  onClick={handleInvite}
                  className="flex items-center gap-1 px-4 py-2 bg-violet-500 text-white rounded-lg text-sm font-medium hover:bg-violet-600"
                >
                  <Plus className="w-4 h-4" /> 邀请
                </button>
              </div>
            </div>

            <div className="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-5">
              <h3 className="font-semibold text-gray-900 dark:text-white mb-3">成员列表 ({members.length})</h3>
              <div className="space-y-2">
                {members.map(m => (
                  <div key={m.id} className="flex items-center justify-between py-2 px-3 rounded-lg hover:bg-gray-50 dark:hover:bg-gray-700/50">
                    <div className="flex items-center gap-3">
                      <div className="w-8 h-8 rounded-full bg-violet-100 dark:bg-violet-900/30 flex items-center justify-center text-sm font-medium text-violet-600">
                        {m.username?.charAt(0)?.toUpperCase() || '?'}
                      </div>
                      <div>
                        <p className="text-sm font-medium text-gray-900 dark:text-white">{m.username}</p>
                        <p className="text-xs text-gray-400">{m.email}</p>
                      </div>
                    </div>
                    <div className="flex items-center gap-2">
                      <span className={`text-xs px-2 py-0.5 rounded-full ${
                        m.role === 'owner' ? 'bg-yellow-100 text-yellow-700 dark:bg-yellow-900/30 dark:text-yellow-400' :
                        m.role === 'admin' ? 'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400' :
                        'bg-gray-100 text-gray-600 dark:bg-gray-700 dark:text-gray-400'
                      }`}>
                        {m.role === 'owner' ? '所有者' : m.role === 'admin' ? '管理员' : '成员'}
                      </span>
                      {m.role !== 'owner' && (
                        <button onClick={() => handleRemoveMember(m.user_id)} className="p-1 rounded hover:bg-red-50 dark:hover:bg-red-900/20">
                          <Trash2 className="w-3.5 h-3.5 text-gray-400 hover:text-red-500" />
                        </button>
                      )}
                    </div>
                  </div>
                ))}
              </div>
            </div>
          </div>
        )}
      </div>
    </div>
  )
}
