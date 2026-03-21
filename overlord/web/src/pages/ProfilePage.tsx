import { useState, useEffect } from 'react'
import { User, BarChart3, Clock, Zap, Shield } from 'lucide-react'
import { getUser } from '../api/client'

export default function ProfilePage() {
  const user = getUser()

  // Mock usage data
  const [usage] = useState({
    tokensToday: 12450,
    tokensMonth: 384200,
    tokenLimit: 500000,
    conversations: 28,
    avgResponseMs: 1240,
  })

  const usagePercent = Math.round((usage.tokensMonth / usage.tokenLimit) * 100)

  return (
    <div className="p-4 md:p-6 max-w-3xl">
      <div className="mb-4 md:mb-6">
        <h1 className="text-lg md:text-xl font-bold text-white">个人中心</h1>
        <p className="text-xs md:text-sm text-gray-500 mt-1">账号信息与用量统计</p>
      </div>

      {/* Profile card */}
      <div className="bg-gray-900 border border-gray-800 rounded-xl p-4 md:p-6 mb-4 md:mb-6">
        <div className="flex items-center gap-4">
          <div className="w-14 h-14 rounded-full bg-brand-600/20 flex items-center justify-center">
            <User className="w-7 h-7 text-brand-400" />
          </div>
          <div>
            <h2 className="text-lg font-semibold text-white">{user?.username || '用户'}</h2>
            <div className="flex items-center gap-3 mt-1">
              <span className="flex items-center gap-1 text-xs text-gray-400">
                <Shield className="w-3 h-3" />
                {user?.role || 'viewer'}
              </span>
              {user?.team_id && (
                <span className="text-xs text-gray-500">团队: {user.team_id}</span>
              )}
            </div>
          </div>
        </div>
      </div>

      {/* Usage stats */}
      <h2 className="text-sm font-semibold text-gray-400 mb-3">本月用量</h2>
      <div className="grid grid-cols-3 gap-2 md:gap-4 mb-4 md:mb-6">
        <div className="bg-gray-900 border border-gray-800 rounded-xl p-4">
          <div className="flex items-center gap-2 mb-2">
            <Zap className="w-4 h-4 text-yellow-500" />
            <span className="text-xs text-gray-400">Token 消耗</span>
          </div>
          <div className="text-lg font-bold text-white">{usage.tokensMonth.toLocaleString()}</div>
          <div className="text-[10px] text-gray-500">限额 {usage.tokenLimit.toLocaleString()}</div>
          <div className="mt-2 h-1.5 bg-gray-800 rounded-full overflow-hidden">
            <div
              className={`h-full rounded-full transition-all ${
                usagePercent > 80 ? 'bg-red-500' : usagePercent > 50 ? 'bg-yellow-500' : 'bg-brand-500'
              }`}
              style={{ width: `${usagePercent}%` }}
            />
          </div>
          <div className="text-[10px] text-gray-500 mt-1">{usagePercent}% 已使用</div>
        </div>

        <div className="bg-gray-900 border border-gray-800 rounded-xl p-4">
          <div className="flex items-center gap-2 mb-2">
            <BarChart3 className="w-4 h-4 text-brand-400" />
            <span className="text-xs text-gray-400">今日对话</span>
          </div>
          <div className="text-lg font-bold text-white">{usage.conversations}</div>
          <div className="text-[10px] text-gray-500">今日 Token: {usage.tokensToday.toLocaleString()}</div>
        </div>

        <div className="bg-gray-900 border border-gray-800 rounded-xl p-4">
          <div className="flex items-center gap-2 mb-2">
            <Clock className="w-4 h-4 text-green-400" />
            <span className="text-xs text-gray-400">平均响应</span>
          </div>
          <div className="text-lg font-bold text-white">{(usage.avgResponseMs / 1000).toFixed(1)}s</div>
          <div className="text-[10px] text-gray-500">{usage.avgResponseMs}ms</div>
        </div>
      </div>

      {/* Settings placeholder */}
      <h2 className="text-sm font-semibold text-gray-400 mb-3">设置</h2>
      <div className="bg-gray-900 border border-gray-800 rounded-xl divide-y divide-gray-800">
        <div className="px-5 py-4 flex items-center justify-between">
          <div>
            <div className="text-sm text-white">默认智能体</div>
            <div className="text-xs text-gray-500">选择对话时默认使用的智能体</div>
          </div>
          <select className="bg-gray-800 border border-gray-700 text-sm text-white rounded-lg px-3 py-1.5 focus:outline-none">
            <option>通用助手</option>
            <option>代码助手</option>
            <option>文档助手</option>
          </select>
        </div>
        <div className="px-5 py-4 flex items-center justify-between">
          <div>
            <div className="text-sm text-white">语言偏好</div>
            <div className="text-xs text-gray-500">AI 回复使用的语言</div>
          </div>
          <select className="bg-gray-800 border border-gray-700 text-sm text-white rounded-lg px-3 py-1.5 focus:outline-none">
            <option>中文</option>
            <option>English</option>
            <option>自动检测</option>
          </select>
        </div>
        <div className="px-5 py-4 flex items-center justify-between">
          <div>
            <div className="text-sm text-white">对话历史</div>
            <div className="text-xs text-gray-500">保留最近对话的天数</div>
          </div>
          <select className="bg-gray-800 border border-gray-700 text-sm text-white rounded-lg px-3 py-1.5 focus:outline-none">
            <option>7 天</option>
            <option>30 天</option>
            <option>90 天</option>
            <option>永久</option>
          </select>
        </div>
      </div>
    </div>
  )
}
