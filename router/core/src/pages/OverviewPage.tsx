import { useEffect, useState } from 'react';
import { Users, Key, CreditCard, Activity, TrendingUp, UserPlus } from 'lucide-react';
import { admin } from '../lib/api';

interface Stats {
  users: number;
  api_keys: number;
  paid_orders: number;
  total_revenue: number;
  total_requests: number;
  today_requests: number;
  today_users: number;
}

export default function OverviewPage() {
  const [stats, setStats] = useState<Stats | null>(null);

  useEffect(() => {
    admin.overview().then(setStats);
  }, []);

  if (!stats) return <div className="text-gray-500">加载中...</div>;

  const cards = [
    { label: '总用户', value: stats.users, icon: Users, color: 'from-blue-500 to-cyan-500' },
    { label: '今日新增', value: stats.today_users, icon: UserPlus, color: 'from-emerald-500 to-green-500' },
    { label: 'API Keys', value: stats.api_keys, icon: Key, color: 'from-amber-500 to-orange-500' },
    { label: '总请求', value: stats.total_requests.toLocaleString(), icon: Activity, color: 'from-violet-500 to-purple-500' },
    { label: '今日请求', value: stats.today_requests.toLocaleString(), icon: TrendingUp, color: 'from-rose-500 to-pink-500' },
    { label: '总收入', value: `¥${(stats.total_revenue / 100).toFixed(2)}`, icon: CreditCard, color: 'from-teal-500 to-emerald-500' },
  ];

  return (
    <div>
      <h1 className="text-2xl font-bold mb-6">总览</h1>
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
        {cards.map(({ label, value, icon: Icon, color }) => (
          <div key={label} className="bg-gray-900 border border-gray-800 rounded-xl p-5">
            <div className="flex items-center justify-between mb-3">
              <span className="text-sm text-gray-400">{label}</span>
              <div className={`w-8 h-8 bg-gradient-to-br ${color} rounded-lg flex items-center justify-center`}>
                <Icon className="w-4 h-4 text-white" />
              </div>
            </div>
            <div className="text-2xl font-bold text-white">{value}</div>
          </div>
        ))}
      </div>
    </div>
  );
}
