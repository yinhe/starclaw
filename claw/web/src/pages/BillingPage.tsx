import { useState, useEffect } from 'react'
import { CreditCard, Users, BarChart3, Receipt, Plus, Trash2, Shield, ArrowUpRight, ArrowDownRight, Zap } from 'lucide-react'
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

interface Transaction {
  id: string
  type: string
  amount: number
  balance: number
  remark: string
  created_at: string
}

interface CreditData {
  balance: number
  total_in: number
  total_out: number
  frozen_energy: number
  connected: boolean
  hp_status: string
  updated_at: string
}

type TabType = 'usage' | 'team' | 'transactions'

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

const ENERGY_UNIT = 10000

function formatEnergy(units: number): string {
  const stars = units / ENERGY_UNIT
  if (stars >= 10000) return `${(stars / 10000).toFixed(2)}万`
  if (stars >= 1000) return `${(stars / 1000).toFixed(2)}K`
  return stars.toFixed(2)
}

export default function BillingPage() {
  const [tab, setTab] = useState<TabType>('usage')
  const [tenant, setTenant] = useState<Tenant | null>(null)
  const [usage, setUsage] = useState<Record<string, number>>({})
  const [cost, setCost] = useState<Record<string, number>>({})
  const [period, setPeriod] = useState('')
  const [credits, setCredits] = useState<CreditData | null>(null)
  const [nodeInfo, setNodeInfo] = useState<any>(null)
  const [members, setMembers] = useState<Member[]>([])
  const [usageHistory, setUsageHistory] = useState<UsageItem[]>([])
  const [transactions, setTransactions] = useState<Transaction[]>([])
  const [inviteEmail, setInviteEmail] = useState('')
  const [inviteRole, setInviteRole] = useState('member')
  const [teamName, setTeamName] = useState('')
  const [editingName, setEditingName] = useState(false)

  useEffect(() => { loadData() }, [])

  const loadData = async () => {
    try {
      const [planRes, creditsRes, nodeRes] = await Promise.all([
        billingAPI.getCurrentPlan().catch(() => null),
        systemAPI.getCredits().catch(() => null),
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
    const res = await billingAPI.listTransactions().catch(() => null)
    if (res?.data) setTransactions(res.data.transactions || [])
  }

  useEffect(() => {
    if (tab === 'team') loadTeam()
    if (tab === 'usage') loadUsageHistory()
    if (tab === 'transactions') loadTransactions()
  }, [tab])

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

  const tabs: { key: TabType; label: string; icon: typeof CreditCard }[] = [
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
            <h3 className="font-semibold text-gray-900 dark:text-white mb-4 flex items-center gap-2">
              <Receipt className="w-4 h-4 text-blue-500" /> 交易流水
            </h3>
            {transactions.length === 0 ? (
              <p className="text-sm text-gray-400 py-8 text-center">暂无交易记录</p>
            ) : (
              <div className="space-y-2">
                {transactions.map(tx => (
                  <div key={tx.id} className="flex items-center justify-between py-2.5 px-3 rounded-lg hover:bg-gray-50 dark:hover:bg-gray-700/50">
                    <div className="flex items-center gap-3">
                      <div className={`w-8 h-8 rounded-full flex items-center justify-center ${
                        tx.type === 'recharge' ? 'bg-emerald-100 dark:bg-emerald-900/30' : 'bg-orange-100 dark:bg-orange-900/30'
                      }`}>
                        {tx.type === 'recharge'
                          ? <ArrowDownRight className="w-4 h-4 text-emerald-600" />
                          : <ArrowUpRight className="w-4 h-4 text-orange-600" />
                        }
                      </div>
                      <div>
                        <p className="text-sm text-gray-900 dark:text-white">{tx.remark}</p>
                        <p className="text-xs text-gray-400">{new Date(tx.created_at).toLocaleString('zh-CN')}</p>
                      </div>
                    </div>
                    <div className="text-right">
                      <p className={`text-sm font-medium ${tx.type === 'recharge' ? 'text-emerald-600' : 'text-orange-600'}`}>
                        {tx.type === 'recharge' ? '+' : '-'}¥{Math.abs(tx.amount).toFixed(2)}
                      </p>
                      <p className="text-xs text-gray-400">余额 ¥{tx.balance.toFixed(2)}</p>
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
