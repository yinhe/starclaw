import { useState, useEffect } from 'react'
import { Link } from 'react-router-dom'
import { Star, Snowflake, WifiOff } from 'lucide-react'
import { systemAPI } from '../lib/api'

interface CreditData {
  connected: boolean
  balance_stars?: number
  hp_status?: string
  frozen_stars?: number
  status?: string
  message?: string
}

const hpConfig: Record<string, { color: string; bg: string; label: string; emoji: string }> = {
  full:       { color: 'text-green-400',  bg: 'bg-green-500',  label: '充沛', emoji: '🟢' },
  healthy:    { color: 'text-green-400',  bg: 'bg-green-400',  label: '健康', emoji: '🟢' },
  low:        { color: 'text-yellow-400', bg: 'bg-yellow-400', label: '偏低', emoji: '🟡' },
  critical:   { color: 'text-orange-400', bg: 'bg-orange-500', label: '危急', emoji: '🟠' },
  hibernated: { color: 'text-red-400',    bg: 'bg-red-500',    label: '休眠', emoji: '🔴' },
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
      <Link to="/wallet" className="block px-3 py-2 hover:bg-gray-800 transition-colors rounded-lg cursor-pointer" title="星力钱包">
        <div className="flex items-center gap-1.5 text-xs text-gray-500">
          <WifiOff className="w-3 h-3" />
          <span>离线</span>
        </div>
      </Link>
    )
  }

  const stars = data.balance_stars ?? 0
  const hp = data.hp_status ?? 'hibernated'
  const cfg = hpConfig[hp] || hpConfig.hibernated

  // HP bar percentage (cap at 2000 stars = 100%)
  const pct = Math.min(100, (stars / 2000) * 100)

  return (
    <Link to="/wallet" className="block px-3 py-2 space-y-1.5 hover:bg-gray-800 transition-colors rounded-lg cursor-pointer" title="星力钱包 — 点击查看详情">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-1.5">
          <Star className={`w-3.5 h-3.5 ${cfg.color}`} fill="currentColor" />
          <span className="text-xs font-semibold text-gray-200">
            {stars >= 1000 ? `${(stars / 1000).toFixed(1)}K` : stars.toFixed(1)} ⭐
          </span>
        </div>
        <span className={`text-[10px] font-medium ${cfg.color}`}>
          {cfg.emoji} {cfg.label}
        </span>
      </div>
      <div className="w-full h-1.5 bg-gray-700 rounded-full overflow-hidden">
        <div
          className={`h-full ${cfg.bg} rounded-full transition-all duration-700`}
          style={{ width: `${Math.max(pct, 2)}%` }}
        />
      </div>
      {data.frozen_stars != null && data.frozen_stars > 0 && (
        <div className="flex items-center gap-1 text-[10px] text-gray-500">
          <Snowflake className="w-2.5 h-2.5" />
          <span>冻结 {data.frozen_stars.toFixed(1)} ⭐</span>
        </div>
      )}
    </Link>
  )
}
