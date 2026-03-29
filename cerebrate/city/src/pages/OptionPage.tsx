import { useEffect, useState } from 'react'
import { option } from '../lib/api'
import type { OptionInfo } from '../lib/api'
import { Gem, TrendingUp, AlertCircle, CheckCircle } from 'lucide-react'

const ROUND_LABELS: Record<string, string> = {
  spore: '孢子期', larva: '幼虫期', zergling: '虫兵期', overlord: '领主期', queen: '虫后期',
}

function fen(v: number): string {
  return '¥' + (v / 100).toLocaleString('zh-CN', { minimumFractionDigits: 2 })
}

function pct(v: number): string {
  return (v * 100).toFixed(1) + '%'
}

export default function OptionPage() {
  const [info, setInfo] = useState<OptionInfo | null>(null)
  const [loading, setLoading] = useState(true)
  const [amount, setAmount] = useState('')
  const [purchasing, setPurchasing] = useState(false)
  const [msg, setMsg] = useState<{ type: 'ok' | 'err'; text: string } | null>(null)

  const load = () => {
    setLoading(true)
    option.myOptions().then(setInfo).catch(() => {}).finally(() => setLoading(false))
  }

  useEffect(() => { load() }, [])

  const handlePurchase = async () => {
    const yuanStr = amount.trim()
    if (!yuanStr) return
    const yuan = parseFloat(yuanStr)
    if (isNaN(yuan) || yuan <= 0) { setMsg({ type: 'err', text: '请输入有效金额' }); return }
    const fenVal = Math.round(yuan * 100)
    setPurchasing(true)
    setMsg(null)
    try {
      const res = await option.purchase(fenVal)
      setMsg({ type: 'ok', text: `购买成功！获得 ${res.shares.toLocaleString()} 份星钻，佣金率更新为 ${pct(res.new_comm_rate)}` })
      setAmount('')
      load()
    } catch (e: unknown) {
      setMsg({ type: 'err', text: (e as Error).message || '购买失败' })
    } finally {
      setPurchasing(false)
    }
  }

  if (loading) return <div className="text-gray-400 animate-pulse">加载中...</div>
  if (!info) return <div className="text-gray-500">无法加载期权信息</div>

  const cr = info.current_round

  return (
    <div className="space-y-6 max-w-4xl">
      <div>
        <h1 className="text-2xl font-bold">合伙人期权</h1>
        <p className="text-gray-400 text-sm mt-1">投资星钻提升佣金率，投多少决定佣金率多少</p>
      </div>

      {/* Rate Cards */}
      <div className="grid grid-cols-3 gap-4">
        <div className="bg-gray-800/50 border border-white/10 rounded-xl p-5">
          <div className="text-xs text-gray-500 mb-1">当前佣金率</div>
          <div className="text-3xl font-bold text-claw-400">{pct(info.effective_rate)}</div>
          {info.in_transition && (
            <div className="text-xs text-yellow-500 mt-1 flex items-center gap-1">
              <AlertCircle size={12} /> 过渡期（旧佣金率 {pct(info.legacy_rate)}）
            </div>
          )}
        </div>
        <div className="bg-gray-800/50 border border-white/10 rounded-xl p-5">
          <div className="text-xs text-gray-500 mb-1">累计投资</div>
          <div className="text-2xl font-bold">{fen(info.total_invested)}</div>
        </div>
        <div className="bg-gray-800/50 border border-white/10 rounded-xl p-5">
          <div className="text-xs text-gray-500 mb-1">持有星钻</div>
          <div className="text-2xl font-bold">{info.total_shares.toLocaleString()} <span className="text-sm text-gray-500">份</span></div>
        </div>
      </div>

      {/* Purchase Section */}
      <div className="bg-gray-800/50 border border-white/10 rounded-xl p-6">
        <div className="flex items-center gap-2 mb-4">
          <Gem size={20} className="text-claw-400" />
          <h2 className="text-lg font-semibold">购买期权星钻</h2>
        </div>

        <div className="grid grid-cols-2 gap-6">
          <div>
            <div className="text-sm text-gray-400 mb-3">
              当前轮次：<span className="text-white font-medium">{ROUND_LABELS[cr.round] || cr.round}</span>
            </div>
            <div className="text-sm text-gray-400 mb-1">
              本轮已投：{fen(cr.invested)} / {fen(cr.max)}
            </div>
            <div className="w-full bg-gray-700 rounded-full h-2 mb-3">
              <div
                className="bg-claw-500 h-2 rounded-full transition-all"
                style={{ width: `${Math.min(100, (cr.invested / cr.max) * 100)}%` }}
              />
            </div>
            <div className="text-sm text-gray-500">
              还可投：<span className="text-claw-400 font-medium">{fen(cr.remaining)}</span>
            </div>
          </div>

          <div>
            <label className="block text-sm text-gray-400 mb-2">投资金额（元）</label>
            <div className="flex gap-3">
              <input
                type="number"
                value={amount}
                onChange={e => setAmount(e.target.value)}
                placeholder="输入金额"
                className="flex-1 bg-gray-900 border border-white/10 rounded-lg px-4 py-2.5 text-white placeholder:text-gray-600 focus:border-claw-500 focus:outline-none"
              />
              <button
                onClick={handlePurchase}
                disabled={purchasing || !amount}
                className="px-6 py-2.5 bg-claw-500 hover:bg-claw-600 disabled:opacity-40 rounded-lg text-white font-medium transition-colors"
              >
                {purchasing ? '购买中...' : '购买'}
              </button>
            </div>
            {msg && (
              <div className={`mt-3 flex items-center gap-2 text-sm ${msg.type === 'ok' ? 'text-green-400' : 'text-red-400'}`}>
                {msg.type === 'ok' ? <CheckCircle size={14} /> : <AlertCircle size={14} />}
                {msg.text}
              </div>
            )}
          </div>
        </div>
      </div>

      {/* Round Breakdown */}
      {info.rounds.length > 0 && (
        <div className="bg-gray-800/50 border border-white/10 rounded-xl p-6">
          <div className="flex items-center gap-2 mb-4">
            <TrendingUp size={20} className="text-claw-400" />
            <h2 className="text-lg font-semibold">各轮投资明细</h2>
          </div>
          <table className="w-full text-sm">
            <thead>
              <tr className="text-gray-500 border-b border-white/5">
                <th className="text-left py-2 font-medium">轮次</th>
                <th className="text-right py-2 font-medium">投资金额</th>
                <th className="text-right py-2 font-medium">星钻份额</th>
                <th className="text-right py-2 font-medium">对应佣金率</th>
              </tr>
            </thead>
            <tbody>
              {info.rounds.map(r => (
                <tr key={r.round} className="border-b border-white/5">
                  <td className="py-2.5">{ROUND_LABELS[r.round] || r.round}</td>
                  <td className="py-2.5 text-right">{fen(r.total_amount)}</td>
                  <td className="py-2.5 text-right">{r.total_shares.toLocaleString()}</td>
                  <td className="py-2.5 text-right text-claw-400 font-medium">{pct(r.comm_rate)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {/* Rate Explanation */}
      <div className="bg-gray-800/30 border border-white/5 rounded-xl p-5 text-sm text-gray-400">
        <p className="font-medium text-gray-300 mb-2">佣金率计算规则</p>
        <ul className="space-y-1 list-disc pl-4">
          <li>不投资 = 10% 基础佣金率</li>
          <li>{info.partner_type === 'city' ? '城市合伙人：投满本轮最高额 → 30%' : '团队合伙人：投满本轮最高额 → 20%'}</li>
          <li>佣金率 = 10% + (投资额 / 本轮最高额) × {info.partner_type === 'city' ? '20%' : '10%'}</li>
          <li>跨轮投资取最高佣金率，同一轮可多次投资</li>
          <li>早期轮次价格更低，同样金额可获得更多星钻 → 更多分红</li>
        </ul>
      </div>
    </div>
  )
}
