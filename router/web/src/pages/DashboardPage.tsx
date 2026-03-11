import { useEffect, useState } from 'react';
import { Zap, Key, BarChart3, Coins } from 'lucide-react';
import { dash } from '../lib/api';

export default function DashboardPage() {
  const [profile, setProfile] = useState<{ user: { name: string; email: string; balance: number; free_quota: number; created_at: string }; api_key_count: number } | null>(null);
  const [usage, setUsage] = useState<{ total_tokens: number; total_cost: number; total_requests: number } | null>(null);

  useEffect(() => {
    dash.profile().then(setProfile).catch(console.error);
    dash.usage(7).then(setUsage).catch(console.error);
  }, []);

  const fmt = (cents: number) => `¥${(cents / 100).toFixed(2)}`;

  return (
    <div className="space-y-8">
      <div>
        <h1 className="text-2xl font-bold text-white">Dashboard</h1>
        {profile && (
          <p className="text-gray-400 text-sm mt-1">
            欢迎，{profile.user.name}
          </p>
        )}
      </div>

      {/* Stats Cards */}
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
        <StatCard
          icon={<Coins className="w-5 h-5 text-green-400" />}
          label="余额"
          value={profile ? fmt(profile.user.balance) : '...'}
          sub={profile ? `免费额度: ${fmt(profile.user.free_quota)}` : ''}
          color="green"
        />
        <StatCard
          icon={<Key className="w-5 h-5 text-blue-400" />}
          label="API Keys"
          value={profile ? String(profile.api_key_count) : '...'}
          sub="活跃密钥"
          color="blue"
        />
        <StatCard
          icon={<BarChart3 className="w-5 h-5 text-purple-400" />}
          label="7 天请求"
          value={usage ? String(usage.total_requests) : '...'}
          sub={usage ? `${(usage.total_tokens / 1000).toFixed(1)}K tokens` : ''}
          color="purple"
        />
        <StatCard
          icon={<Zap className="w-5 h-5 text-amber-400" />}
          label="7 天消费"
          value={usage ? fmt(usage.total_cost) : '...'}
          sub="API 调用费用"
          color="amber"
        />
      </div>

      {/* Quick Start */}
      <div className="bg-gray-900 border border-gray-800 rounded-xl p-6">
        <h2 className="text-lg font-semibold text-white mb-4">快速开始</h2>
        <div className="bg-gray-800 rounded-lg p-4 font-mono text-sm">
          <div className="text-gray-400 mb-2"># 使用你的 API Key 调用</div>
          <div className="text-green-400">
            curl https://api.star-ai.net/v1/chat/completions \
          </div>
          <div className="text-green-400 pl-4">
            -H "Authorization: Bearer sk-star-xxx" \
          </div>
          <div className="text-green-400 pl-4">
            -H "Content-Type: application/json" \
          </div>
          <div className="text-green-400 pl-4">
            -d '{`{"model":"qwen/qwen-turbo","messages":[{"role":"user","content":"你好"}]}`}'
          </div>
        </div>
        <p className="text-gray-500 text-xs mt-3">
          兼容 OpenAI API 格式，支持 40+ 模型。在 API Keys 页面获取你的密钥。
        </p>
      </div>
    </div>
  );
}

function StatCard({ icon, label, value, sub, color }: { icon: React.ReactNode; label: string; value: string; sub: string; color: string }) {
  const bgMap: Record<string, string> = {
    green: 'bg-green-500/10',
    blue: 'bg-blue-500/10',
    purple: 'bg-purple-500/10',
    amber: 'bg-amber-500/10',
  };
  return (
    <div className="bg-gray-900 border border-gray-800 rounded-xl p-5">
      <div className="flex items-center gap-3 mb-3">
        <div className={`w-9 h-9 ${bgMap[color]} rounded-lg flex items-center justify-center`}>
          {icon}
        </div>
        <span className="text-sm text-gray-400">{label}</span>
      </div>
      <div className="text-2xl font-bold text-white">{value}</div>
      <div className="text-xs text-gray-500 mt-1">{sub}</div>
    </div>
  );
}
