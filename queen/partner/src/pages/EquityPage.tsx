import { useEffect, useState } from 'react'
import { TrendingUp } from 'lucide-react'
import { partner, type EquityGrant } from '../lib/api'

export default function EquityPage() {
  const [grants, setGrants] = useState<EquityGrant[]>([])

  useEffect(() => {
    partner.getEquity().then(r => setGrants(r.grants || [])).catch(console.error)
  }, [])

  const now = new Date()

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold text-white">期权归属</h1>
        <p className="text-sm text-gray-400 mt-1">查看期权授予和归属进度</p>
      </div>

      {grants.length === 0 ? (
        <div className="rounded-xl border border-white/10 border-dashed p-12 text-center">
          <TrendingUp className="w-10 h-10 text-gray-600 mx-auto mb-3" />
          <p className="text-gray-500">暂无期权授予</p>
        </div>
      ) : (
        <div className="space-y-4">
          {grants.map(g => {
            const cliffDate = new Date(g.cliff_date)
            const fullDate = new Date(g.full_vest_date)
            const grantDate = new Date(g.grant_date)
            const isPreCliff = now < cliffDate
            const isFullyVested = now >= fullDate

            const totalDays = (fullDate.getTime() - grantDate.getTime()) / (1000 * 60 * 60 * 24)
            const elapsedDays = Math.max(0, (now.getTime() - grantDate.getTime()) / (1000 * 60 * 60 * 24))
            const pct = Math.min((elapsedDays / totalDays) * 100, 100)
            const cliffPct = ((cliffDate.getTime() - grantDate.getTime()) / (fullDate.getTime() - grantDate.getTime())) * 100

            const vestValue = g.vested_shares * (g.current_value - g.strike_price)
            const totalValue = g.total_shares * (g.current_value - g.strike_price)

            return (
              <div key={g.id} className="rounded-xl border border-white/10 bg-white/[0.02] p-6">
                {/* Header */}
                <div className="flex items-center justify-between mb-4">
                  <div>
                    <span className={`text-xs px-2 py-0.5 rounded ${
                      g.status === 'active' ? 'text-green-400 bg-green-500/10' :
                      g.status === 'exercised' ? 'text-blue-400 bg-blue-500/10' :
                      'text-gray-400 bg-gray-500/10'
                    }`}>
                      {g.status === 'active' ? '进行中' : g.status === 'exercised' ? '已行权' : '已取消'}
                    </span>
                    <span className="text-xs text-gray-500 ml-2">
                      授予日: {grantDate.toLocaleDateString()}
                    </span>
                  </div>
                  <div className="text-right">
                    <div className="text-sm text-gray-400">行权价 ¥{g.strike_price}/股</div>
                    <div className="text-sm text-gray-400">当前估值 ¥{g.current_value}/股</div>
                  </div>
                </div>

                {/* Stats */}
                <div className="grid grid-cols-4 gap-4 mb-5">
                  <div>
                    <div className="text-xs text-gray-500">总授予</div>
                    <div className="text-lg font-bold text-white">{g.total_shares.toLocaleString()}</div>
                  </div>
                  <div>
                    <div className="text-xs text-gray-500">已归属</div>
                    <div className="text-lg font-bold text-green-400">{g.vested_shares.toLocaleString()}</div>
                  </div>
                  <div>
                    <div className="text-xs text-gray-500">未归属</div>
                    <div className="text-lg font-bold text-gray-400">{(g.total_shares - g.vested_shares).toLocaleString()}</div>
                  </div>
                  <div>
                    <div className="text-xs text-gray-500">已归属价值</div>
                    <div className="text-lg font-bold text-amber-400">
                      ¥{vestValue > 0 ? vestValue.toLocaleString() : '0'}
                    </div>
                  </div>
                </div>

                {/* Progress bar */}
                <div className="relative">
                  <div className="h-3 bg-white/5 rounded-full overflow-hidden">
                    <div
                      className={`h-full rounded-full transition-all ${
                        isPreCliff ? 'bg-gray-500/40' :
                        isFullyVested ? 'bg-gradient-to-r from-green-600 to-green-400' :
                        'bg-gradient-to-r from-claw-600 to-claw-400'
                      }`}
                      style={{ width: `${pct}%` }}
                    />
                  </div>
                  {/* Cliff marker */}
                  <div
                    className="absolute top-0 w-0.5 h-3 bg-yellow-400/60"
                    style={{ left: `${cliffPct}%` }}
                  />
                  <div className="flex justify-between text-[10px] mt-1.5">
                    <span className="text-gray-500">授予 {grantDate.toLocaleDateString()}</span>
                    <span className={`${isPreCliff ? 'text-yellow-400' : 'text-gray-500'}`}>
                      Cliff {cliffDate.toLocaleDateString()}
                      {isPreCliff && ` (${Math.ceil((cliffDate.getTime() - now.getTime()) / (1000*60*60*24))} 天后)`}
                    </span>
                    <span className="text-gray-500">完全归属 {fullDate.toLocaleDateString()}</span>
                  </div>
                </div>

                {/* Summary */}
                <div className="mt-4 p-3 rounded-lg bg-white/[0.02] border border-white/5 text-xs text-gray-400">
                  {isPreCliff ? (
                    <span>Cliff 期内，尚未开始归属。Cliff 后将一次性归属 {Math.round(g.total_shares * (cliffPct / 100)).toLocaleString()} 股。</span>
                  ) : isFullyVested ? (
                    <span>已完全归属！总价值 ¥{totalValue > 0 ? totalValue.toLocaleString() : '0'}（按当前估值）</span>
                  ) : (
                    <span>每月归属约 {Math.round(g.total_shares / g.vesting_months).toLocaleString()} 股，剩余 {(g.total_shares - g.vested_shares).toLocaleString()} 股待归属。</span>
                  )}
                </div>
              </div>
            )
          })}
        </div>
      )}
    </div>
  )
}
