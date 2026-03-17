import { useEffect, useState } from 'react'
import { CreditCard, Crown, Zap, AlertTriangle, Plus, Trash2, Bell, CheckCircle, XCircle, Clock } from 'lucide-react'
import { broodAPI, type Plan, type BillingOverview, type Subscription, type BudgetAlert } from '../api/brood'

const planColors: Record<string, string> = {
  community: 'border-gray-600',
  starter: 'border-blue-500',
  pro: 'border-overlord-500',
  enterprise: 'border-amber-500',
  whitelabel: 'border-purple-500',
}

const planBadgeColors: Record<string, string> = {
  community: 'bg-gray-600/10 text-gray-400',
  starter: 'bg-blue-600/10 text-blue-400',
  pro: 'bg-overlord-600/10 text-overlord-400',
  enterprise: 'bg-amber-600/10 text-amber-400',
  whitelabel: 'bg-purple-600/10 text-purple-400',
}

function formatCents(cents: number): string {
  return `¥${(cents / 100).toLocaleString()}`
}

export default function BillingPage() {
  const [overview, setOverview] = useState<BillingOverview | null>(null)
  const [plans, setPlans] = useState<Plan[]>([])
  const [subscriptions, setSubscriptions] = useState<Subscription[]>([])
  const [alerts, setAlerts] = useState<BudgetAlert[]>([])
  const [loading, setLoading] = useState(true)
  const [showAlertForm, setShowAlertForm] = useState(false)
  const [alertForm, setAlertForm] = useState({ name: '', metric_type: 'tokens', threshold_value: 0, period: 'daily', notify_email: '' })

  const load = async () => {
    try {
      const [ov, pl, subs, al] = await Promise.all([
        broodAPI.billingOverview().catch(() => null),
        broodAPI.listPlans().catch(() => []),
        broodAPI.listSubscriptions({ status: 'active' }).catch(() => []),
        broodAPI.listAlerts().catch(() => []),
      ])
      setOverview(ov)
      setPlans(pl)
      setSubscriptions(Array.isArray(subs) ? subs : [])
      setAlerts(Array.isArray(al) ? al : [])
    } catch { /* */ }
    finally { setLoading(false) }
  }

  useEffect(() => { load() }, [])

  const handleSubscribe = async (planId: string) => {
    try {
      await broodAPI.createSubscription({ plan_id: planId, billing_cycle: 'monthly' })
      load()
    } catch { /* */ }
  }

  const handleCancelSub = async (id: string) => {
    if (!confirm('确定取消此订阅吗？')) return
    try { await broodAPI.cancelSubscription(id); load() } catch { /* */ }
  }

  const handleCreateAlert = async () => {
    if (!alertForm.name.trim() || alertForm.threshold_value <= 0) return
    try {
      await broodAPI.createAlert(alertForm as Partial<BudgetAlert>)
      setShowAlertForm(false)
      setAlertForm({ name: '', metric_type: 'tokens', threshold_value: 0, period: 'daily', notify_email: '' })
      load()
    } catch { /* */ }
  }

  const handleDeleteAlert = async (id: string) => {
    if (!confirm('确定删除此告警规则吗？')) return
    try { await broodAPI.deleteAlert(id); load() } catch { /* */ }
  }

  if (loading) {
    return (
      <div className="flex items-center justify-center h-full">
        <div className="animate-spin w-8 h-8 border-2 border-overlord-500 border-t-transparent rounded-full" />
      </div>
    )
  }

  const currentPlan = overview?.plan
  const currentSub = overview?.subscription

  return (
    <div className="p-8">
      <div className="mb-8">
        <h1 className="text-2xl font-bold text-white">计费管理</h1>
        <p className="text-sm text-gray-500 mt-1">订阅套餐、用量概览与预算告警</p>
      </div>

      {/* Overview Cards */}
      {overview && (
        <div className="grid grid-cols-4 gap-4 mb-8">
          <div className="bg-gray-900 border border-gray-800 rounded-xl p-5">
            <div className="flex items-center gap-2 mb-2">
              <Crown className="w-4 h-4 text-overlord-400" />
              <span className="text-sm text-gray-400">当前套餐</span>
            </div>
            <div className="text-xl font-bold text-white">{currentPlan?.display_name || '未订阅'}</div>
            {currentSub?.id && (
              <div className="text-xs text-gray-500 mt-1">
                {currentSub.billing_cycle === 'yearly' ? '年付' : '月付'} · 到期 {currentSub.current_period_end?.slice(0, 10)}
              </div>
            )}
          </div>
          <div className="bg-gray-900 border border-gray-800 rounded-xl p-5">
            <div className="flex items-center gap-2 mb-2">
              <Zap className="w-4 h-4 text-yellow-400" />
              <span className="text-sm text-gray-400">本月 Tokens</span>
            </div>
            <div className="text-xl font-bold text-white">{overview.month_usage.total_tokens.toLocaleString()}</div>
            <div className="text-xs text-gray-500 mt-1">{overview.month_usage.total_requests.toLocaleString()} 次请求</div>
          </div>
          <div className="bg-gray-900 border border-gray-800 rounded-xl p-5">
            <div className="flex items-center gap-2 mb-2">
              <CreditCard className="w-4 h-4 text-emerald-400" />
              <span className="text-sm text-gray-400">本月费用</span>
            </div>
            <div className="text-xl font-bold text-white">{formatCents(overview.month_usage.total_cost_cents)}</div>
            <div className="text-xs text-gray-500 mt-1">今日 {formatCents(overview.today_usage.total_cost_cents)}</div>
          </div>
          <div className="bg-gray-900 border border-gray-800 rounded-xl p-5">
            <div className="flex items-center gap-2 mb-2">
              <Bell className="w-4 h-4 text-red-400" />
              <span className="text-sm text-gray-400">活跃告警</span>
            </div>
            <div className="text-xl font-bold text-white">{overview.active_alerts}</div>
            <div className="text-xs text-gray-500 mt-1">预算告警规则</div>
          </div>
        </div>
      )}

      {/* Plans */}
      <div className="mb-8">
        <h2 className="text-lg font-semibold text-white mb-4">可用套餐</h2>
        <div className="grid grid-cols-5 gap-3">
          {plans.map(plan => {
            const isCurrent = currentPlan?.id === plan.id
            return (
              <div key={plan.id} className={`bg-gray-900 border-2 rounded-xl p-4 transition ${isCurrent ? planColors[plan.name] || 'border-overlord-500' : 'border-gray-800 hover:border-gray-700'}`}>
                <div className="flex items-center justify-between mb-3">
                  <span className={`text-xs px-2 py-0.5 rounded-full font-medium ${planBadgeColors[plan.name] || 'bg-gray-600/10 text-gray-400'}`}>
                    {plan.display_name}
                  </span>
                  {isCurrent && <CheckCircle className="w-4 h-4 text-emerald-400" />}
                </div>
                <div className="mb-3">
                  {plan.price_monthly === 0 ? (
                    <div className="text-2xl font-bold text-white">免费</div>
                  ) : (
                    <>
                      <div className="text-2xl font-bold text-white">{formatCents(plan.price_monthly)}<span className="text-sm text-gray-500 font-normal">/月</span></div>
                      {plan.price_yearly > 0 && (
                        <div className="text-xs text-gray-500">年付 {formatCents(plan.price_yearly)}/月</div>
                      )}
                    </>
                  )}
                </div>
                <div className="space-y-1.5 text-xs text-gray-400 mb-4">
                  <div>≤ {plan.max_nodes || '∞'} 节点</div>
                  <div>{plan.max_teams === 0 ? '不限' : `≤ ${plan.max_teams}`} 团队</div>
                </div>
                {!isCurrent && (
                  <button
                    onClick={() => handleSubscribe(plan.id)}
                    className="w-full py-1.5 text-xs rounded-lg bg-overlord-600/20 text-overlord-300 hover:bg-overlord-600/30 transition"
                  >
                    {plan.price_monthly === 0 ? '切换' : '订阅'}
                  </button>
                )}
              </div>
            )
          })}
        </div>
      </div>

      {/* Active Subscriptions */}
      {subscriptions.length > 0 && (
        <div className="mb-8">
          <h2 className="text-lg font-semibold text-white mb-4">活跃订阅</h2>
          <div className="space-y-2">
            {subscriptions.map(sub => (
              <div key={sub.id} className="bg-gray-900 border border-gray-800 rounded-xl p-4 flex items-center gap-4">
                <div className="w-10 h-10 rounded-lg bg-overlord-600/10 flex items-center justify-center shrink-0">
                  <CreditCard className="w-5 h-5 text-overlord-400" />
                </div>
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-2">
                    <span className="text-sm font-semibold text-white capitalize">{sub.plan_name}</span>
                    <span className={`text-[10px] px-1.5 py-0.5 rounded ${sub.status === 'active' ? 'bg-emerald-600/10 text-emerald-400' : 'bg-red-600/10 text-red-400'}`}>
                      {sub.status}
                    </span>
                    <span className="text-[10px] px-1.5 py-0.5 rounded bg-gray-800 text-gray-400">
                      {sub.billing_cycle === 'yearly' ? '年付' : '月付'}
                    </span>
                  </div>
                  <div className="flex items-center gap-3 text-xs text-gray-500 mt-1">
                    <span>开始: {sub.current_period_start?.slice(0, 10)}</span>
                    <span>到期: {sub.current_period_end?.slice(0, 10)}</span>
                    {sub.team_id && <span>团队: {sub.team_id.slice(0, 8)}...</span>}
                  </div>
                </div>
                {sub.status === 'active' && (
                  <button onClick={() => handleCancelSub(sub.id)} className="p-2 text-gray-500 hover:text-red-400 transition" title="取消订阅">
                    <XCircle className="w-4 h-4" />
                  </button>
                )}
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Budget Alerts */}
      <div>
        <div className="flex items-center justify-between mb-4">
          <h2 className="text-lg font-semibold text-white">预算告警</h2>
          <button
            onClick={() => setShowAlertForm(!showAlertForm)}
            className="flex items-center gap-2 px-3 py-1.5 bg-overlord-600 text-white rounded-lg text-xs hover:bg-overlord-500 transition"
          >
            <Plus className="w-3.5 h-3.5" /> 新建告警
          </button>
        </div>

        {showAlertForm && (
          <div className="bg-gray-900 border border-gray-800 rounded-xl p-5 mb-4">
            <h3 className="text-sm font-medium text-white mb-4">创建预算告警</h3>
            <div className="grid grid-cols-2 gap-4 mb-4">
              <div>
                <label className="block text-xs text-gray-400 mb-1">告警名称 *</label>
                <input value={alertForm.name} onChange={e => setAlertForm({ ...alertForm, name: e.target.value })}
                  placeholder="日用量超限告警" className="w-full bg-gray-800 border border-gray-700 rounded-lg px-3 py-2 text-white text-sm focus:outline-none focus:border-overlord-500" />
              </div>
              <div>
                <label className="block text-xs text-gray-400 mb-1">指标类型</label>
                <select value={alertForm.metric_type} onChange={e => setAlertForm({ ...alertForm, metric_type: e.target.value })}
                  className="w-full bg-gray-800 border border-gray-700 rounded-lg px-3 py-2 text-white text-sm focus:outline-none focus:border-overlord-500">
                  <option value="tokens">Tokens</option>
                  <option value="cost">费用 (分)</option>
                  <option value="star_energy">星能</option>
                  <option value="requests">请求数</option>
                </select>
              </div>
              <div>
                <label className="block text-xs text-gray-400 mb-1">阈值 *</label>
                <input type="number" value={alertForm.threshold_value} onChange={e => setAlertForm({ ...alertForm, threshold_value: +e.target.value })}
                  className="w-full bg-gray-800 border border-gray-700 rounded-lg px-3 py-2 text-white text-sm focus:outline-none focus:border-overlord-500" />
              </div>
              <div>
                <label className="block text-xs text-gray-400 mb-1">周期</label>
                <select value={alertForm.period} onChange={e => setAlertForm({ ...alertForm, period: e.target.value })}
                  className="w-full bg-gray-800 border border-gray-700 rounded-lg px-3 py-2 text-white text-sm focus:outline-none focus:border-overlord-500">
                  <option value="daily">每日</option>
                  <option value="monthly">每月</option>
                </select>
              </div>
              <div className="col-span-2">
                <label className="block text-xs text-gray-400 mb-1">通知邮箱</label>
                <input value={alertForm.notify_email} onChange={e => setAlertForm({ ...alertForm, notify_email: e.target.value })}
                  placeholder="admin@example.com" className="w-full bg-gray-800 border border-gray-700 rounded-lg px-3 py-2 text-white text-sm focus:outline-none focus:border-overlord-500" />
              </div>
            </div>
            <div className="flex gap-2">
              <button onClick={handleCreateAlert} className="px-4 py-2 bg-overlord-600 text-white text-sm rounded-lg hover:bg-overlord-500 transition">创建</button>
              <button onClick={() => setShowAlertForm(false)} className="px-4 py-2 bg-gray-800 text-gray-300 text-sm rounded-lg hover:bg-gray-700 transition">取消</button>
            </div>
          </div>
        )}

        {alerts.length === 0 ? (
          <div className="bg-gray-900 border border-gray-800 border-dashed rounded-xl p-8 text-center">
            <Bell className="w-8 h-8 text-gray-600 mx-auto mb-2" />
            <p className="text-sm text-gray-500">暂无预算告警规则</p>
            <p className="text-xs text-gray-600 mt-1">设置告警以便在用量超出预算时收到通知</p>
          </div>
        ) : (
          <div className="space-y-2">
            {alerts.map(alert => (
              <div key={alert.id} className="bg-gray-900 border border-gray-800 rounded-xl p-4 flex items-center gap-4 hover:border-gray-700 transition group">
                <div className={`w-10 h-10 rounded-lg flex items-center justify-center shrink-0 ${alert.enabled ? 'bg-emerald-600/10' : 'bg-gray-700/20'}`}>
                  <AlertTriangle className={`w-5 h-5 ${alert.enabled ? 'text-emerald-400' : 'text-gray-500'}`} />
                </div>
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-2">
                    <span className="text-sm font-semibold text-white">{alert.name}</span>
                    <span className={`text-[10px] px-1.5 py-0.5 rounded ${alert.enabled ? 'bg-emerald-600/10 text-emerald-400' : 'bg-gray-600/10 text-gray-500'}`}>
                      {alert.enabled ? '启用' : '禁用'}
                    </span>
                  </div>
                  <div className="flex items-center gap-3 text-xs text-gray-500 mt-1">
                    <span>{alert.metric_type} ≥ {alert.threshold_value.toLocaleString()}</span>
                    <span>{alert.period === 'daily' ? '每日' : '每月'}</span>
                    {alert.notify_email && <span>{alert.notify_email}</span>}
                    {alert.last_triggered && (
                      <span className="flex items-center gap-1 text-amber-500">
                        <Clock className="w-3 h-3" />
                        上次触发: {alert.last_triggered.slice(0, 16).replace('T', ' ')}
                      </span>
                    )}
                  </div>
                </div>
                <button onClick={() => handleDeleteAlert(alert.id)} className="p-2 text-gray-500 hover:text-red-400 transition opacity-0 group-hover:opacity-100" title="删除">
                  <Trash2 className="w-4 h-4" />
                </button>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  )
}
