import { useEffect, useState } from 'react'
import { api, type BillingStats } from '../api'
import StatCard from '../components/StatCard'
import {
  DollarSign,
  TrendingUp,
  ShoppingCart,
  Wallet,
  ArrowDownCircle,
  Users,
  CreditCard,
  PiggyBank,
} from 'lucide-react'

function fen2yuan(fen: number): string {
  return `¥${(fen / 100).toFixed(2)}`
}

export default function BillingStatsPage() {
  const [stats, setStats] = useState<BillingStats | null>(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    api.get<BillingStats>('/v1/admin/billing/stats')
      .then(setStats)
      .catch(() => {})
      .finally(() => setLoading(false))
  }, [])

  if (loading) return <div className="text-gray-500 text-center py-20">加载中...</div>
  if (!stats) return <div className="text-gray-500 text-center py-20">无法加载数据</div>

  return (
    <div>
      <h2 className="text-xl font-bold mb-6">收入统计</h2>

      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
        <StatCard title="今日收入" value={fen2yuan(stats.today_revenue)} icon={DollarSign} sub={`${stats.today_orders} 笔`} color="green" />
        <StatCard title="本月收入" value={fen2yuan(stats.month_revenue)} icon={TrendingUp} color="blue" />
        <StatCard title="累计收入" value={fen2yuan(stats.total_revenue)} icon={ShoppingCart} sub={`${stats.total_orders} 笔`} color="purple" />
        <StatCard title="总充值用户" value={stats.total_users} icon={Users} color="cyan" />
        <StatCard title="用户总余额" value={fen2yuan(stats.total_balance)} icon={Wallet} color="amber" />
        <StatCard title="总消费额" value={fen2yuan(stats.total_consumed)} icon={ArrowDownCircle} color="red" />
        <StatCard title="今日订单" value={stats.today_orders} icon={CreditCard} color="green" />
        <StatCard title="总订单" value={stats.total_orders} icon={PiggyBank} color="purple" />
      </div>
    </div>
  )
}
