import { useState, useEffect } from 'react'
import { Link } from 'react-router-dom'
import { Zap, Snowflake, WifiOff } from 'lucide-react'
import { systemAPI } from '../lib/api'

interface CreditData {
  connected: boolean
  balance_energy?: number
  hp_status?: string
  frozen_energy?: number
  status?: string
  message?: string
}

const hpConfig: Record<string, { color: string; bg: string; glow: string; label: string }> = {
  full:       { color: 'text-emerald-400', bg: 'bg-emerald-500', glow: 'shadow-emerald-500/40', label: '充沛' },
  healthy:    { color: 'text-green-400',   bg: 'bg-green-400',   glow: 'shadow-green-400/30',   label: '健康' },
  low:        { color: 'text-yellow-400',  bg: 'bg-yellow-400',  glow: 'shadow-yellow-400/30',  label: '偏低' },
  critical:   { color: 'text-orange-400',  bg: 'bg-orange-500',  glow: 'shadow-orange-500/30',  label: '危急' },
  hibernated: { color: 'text-red-400',     bg: 'bg-red-500',     glow: 'shadow-red-500/30',     label: '休眠' },
}

export default function HPBar() {
  const [data, setData] = useState<CreditData | null>(null)

  useEffect(() => {
    const fetch = () => {
      systemAPI.getCredits().then(res => setData(res.data)).catch(() => {})
    }
    fetch()
    const interval = setInterval(fetch, 30000)
    return () => clearInterval(interval)
  }, [])

  if (!data || !data.connected) {
    return (
      <Link to="/wallet" className="block px-3 py-2 hover:bg-gray-800 transition-colors rounded-lg cursor-pointer" title="星能钱包">
        <div className="flex items-center gap-1.5 text-xs text-gray-500">
          <WifiOff className="w-3 h-3" />
          <span>离线</span>
        </div>
      </Link>
    )
  }

  const stars = data.balance_energy ?? 0
  const hp = data.hp_status ?? 'hibernated'
  const cfg = hpConfig[hp] || hpConfig.hibernated
  const isLow = hp === 'low' || hp === 'critical' || hp === 'hibernated'
  const pct = Math.min(100, (stars / 2000) * 100)

  return (
    <Link to="/wallet" className="block px-3 py-2 space-y-1.5 hover:bg-gray-800 transition-colors rounded-lg cursor-pointer" title="星能钱包 — 点击查看详情">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-1.5">
          <Zap className={`w-3.5 h-3.5 ${cfg.color} ${isLow ? 'animate-pulse' : ''}`} fill="currentColor" />
          <span className="text-xs font-semibold text-gray-200">
            {navigator.language?.startsWith('zh')
              ? (stars >= 10000 ? `${(stars / 10000).toFixed(2)}万` : stars.toFixed(1))
              : (stars >= 1000 ? `${(stars / 1000).toFixed(1)}K` : stars.toFixed(1))}
          </span>
        </div>
        <span className={`text-[10px] font-medium ${cfg.color}`}>
          {cfg.label}
        </span>
      </div>
      <div className={`w-full h-1.5 bg-gray-700 rounded-full overflow-hidden shadow-sm ${cfg.glow}`}>
        <div
          className={`h-full ${cfg.bg} rounded-full transition-all duration-700`}
          style={{ width: `${Math.max(pct, 2)}%` }}
        />
      </div>
      {data.frozen_energy != null && data.frozen_energy > 0 && (
        <div className="flex items-center gap-1 text-[10px] text-gray-500">
          <Snowflake className="w-2.5 h-2.5" />
          <span>冻结 {data.frozen_energy.toFixed(1)}</span>
        </div>
      )}
    </Link>
  )
}
