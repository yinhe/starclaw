import { useState, useEffect } from 'react'
import { Navbar } from '../components/Navbar'
import { Footer } from '../components/Footer'
import { growthAPI, type Fighter, type BattleRecord, type Season, type ShopItem, type Mutation } from '../lib/api'
import { isLoggedIn } from '../lib/auth'
import { useNavigate } from 'react-router-dom'
import { Swords, ShoppingBag, Dna, Clock, Trophy, Sparkles } from 'lucide-react'

const PATH_EMOJI: Record<string, string> = { larva: '🥚', abyss: '🦑', terrain: '🦂', sky: '🦅' }
const ENV_EMOJI: Record<string, string> = { abyss: '🌊', terrain: '🏔️', sky: '☁️' }

const RARITY_COLORS: Record<string, { text: string; bg: string; border: string }> = {
  common:    { text: 'text-gray-400',   bg: 'bg-gray-500/10',    border: 'border-gray-700' },
  uncommon:  { text: 'text-emerald-400', bg: 'bg-emerald-500/10', border: 'border-emerald-800' },
  rare:      { text: 'text-indigo-400',  bg: 'bg-indigo-500/10',  border: 'border-indigo-800' },
  epic:      { text: 'text-purple-400',  bg: 'bg-purple-500/10',  border: 'border-purple-800' },
  legendary: { text: 'text-amber-400',   bg: 'bg-amber-500/10',   border: 'border-amber-800' },
}

const RARITY_LABEL: Record<string, string> = {
  common: '普通', uncommon: '优秀', rare: '稀有', epic: '史诗', legendary: '传说',
}

function timeAgo(dateStr: string) {
  const diff = Math.floor((Date.now() - new Date(dateStr).getTime()) / 1000)
  if (diff < 60) return '刚刚'
  if (diff < 3600) return `${Math.floor(diff / 60)}分钟前`
  if (diff < 86400) return `${Math.floor(diff / 3600)}小时前`
  return `${Math.floor(diff / 86400)}天前`
}

export function ChrysalisPage() {
  const navigate = useNavigate()
  const [tab, setTab] = useState<'battles' | 'shop' | 'mutations'>('battles')
  const [fighter, setFighter] = useState<Fighter | null>(null)
  const [battles, setBattles] = useState<BattleRecord[]>([])
  const [season, setSeason] = useState<Season | null>(null)
  const [shop, setShop] = useState<ShopItem[]>([])
  const [mutations, setMutations] = useState<Mutation[]>([])
  const [leaderboard, setLeaderboard] = useState<Fighter[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    if (!isLoggedIn()) { navigate('/auth'); return }
    load()
  }, [navigate])

  async function load() {
    setLoading(true)
    try {
      const [fRes, lbRes, sRes, shopRes] = await Promise.all([
        growthAPI.fighter().catch(() => null),
        growthAPI.leaderboard().catch(() => null),
        growthAPI.season().catch(() => null),
        growthAPI.shop().catch(() => null),
      ])
      const f = (fRes as any)?.fighter ?? null
      setFighter(f)
      setLeaderboard((lbRes as any)?.fighters ?? [])
      setSeason((sRes as any)?.season ?? null)
      setShop((shopRes as any)?.items ?? [])

      if (f?.claw_id) {
        const [hRes, mRes] = await Promise.all([
          growthAPI.history(f.claw_id).catch(() => null),
          growthAPI.mutations(f.claw_id).catch(() => null),
        ])
        setBattles((hRes as any)?.battles ?? [])
        setMutations((mRes as any)?.mutations ?? [])
      }
    } catch { /* ignore */ }
    setLoading(false)
  }

  const tabs = [
    { key: 'battles' as const, label: '对战记录', icon: Swords },
    { key: 'shop' as const, label: '装备商店', icon: ShoppingBag },
    { key: 'mutations' as const, label: '变异列表', icon: Dna },
  ]

  if (loading) return <><Navbar /><div className="min-h-screen bg-gray-950 flex items-center justify-center text-gray-400">加载中...</div><Footer /></>

  return (
    <>
      <Navbar />
      <div className="min-h-screen bg-gray-950 pt-20 pb-16">
        <div className="max-w-5xl mx-auto px-4">

          {/* Header */}
          <div className="flex items-center gap-3 mb-2">
            <Sparkles className="w-6 h-6 text-purple-400" />
            <h1 className="text-2xl font-bold text-white">化蛹 · PK 对战</h1>
          </div>
          <p className="text-sm text-gray-500 mb-6">装备武器，在竞技场与其他节点的宠物对战</p>

          {/* Season + Fighter Summary Row */}
          <div className="grid md:grid-cols-2 gap-4 mb-6">
            {/* Season */}
            {season && (
              <div className="bg-gradient-to-r from-purple-900/30 to-indigo-900/30 border border-purple-800/40 rounded-xl p-4 flex items-center gap-3">
                <span className="text-3xl">{ENV_EMOJI[season.environment] ?? '🌍'}</span>
                <div>
                  <div className="text-white font-semibold">{season.name}</div>
                  <div className="text-xs text-gray-400">{season.active ? '🔥 进行中' : '⏸ 已结束'} · 环境: {season.environment}</div>
                </div>
              </div>
            )}
            {/* My Fighter Mini */}
            {fighter && (
              <div className="bg-gray-900 border border-gray-800 rounded-xl p-4 flex items-center gap-3">
                <span className="text-3xl">{PATH_EMOJI[fighter.evolution_path] ?? '🦞'}</span>
                <div className="flex-1 min-w-0">
                  <div className="text-white font-semibold truncate">{fighter.name}</div>
                  <div className="text-xs text-gray-400">Lv.{fighter.level} · ELO {fighter.elo} · {fighter.wins}胜 {fighter.losses}负</div>
                </div>
                <button onClick={() => navigate('/growth')} className="text-xs text-indigo-400 hover:text-indigo-300 whitespace-nowrap">
                  查看详情 →
                </button>
              </div>
            )}
          </div>

          {/* Tabs */}
          <div className="flex gap-1 bg-gray-900 rounded-xl p-1 mb-6">
            {tabs.map(t => (
              <button
                key={t.key}
                onClick={() => setTab(t.key)}
                className={`flex items-center gap-1.5 px-4 py-2.5 rounded-lg text-sm font-medium transition flex-1 justify-center ${
                  tab === t.key ? 'bg-indigo-600 text-white' : 'text-gray-400 hover:text-white hover:bg-gray-800'
                }`}
              >
                <t.icon className="w-4 h-4" /> {t.label}
              </button>
            ))}
          </div>

          {/* Tab Content */}
          {tab === 'battles' && <BattlesTab battles={battles} leaderboard={leaderboard} />}
          {tab === 'shop' && <ShopTab items={shop} />}
          {tab === 'mutations' && <MutationsTab mutations={mutations} />}
        </div>
      </div>
      <Footer />
    </>
  )
}

/* ── Battles Tab ── */
function BattlesTab({ battles, leaderboard }: { battles: BattleRecord[]; leaderboard: Fighter[] }) {
  return (
    <div className="grid md:grid-cols-3 gap-6">
      {/* Battle History */}
      <div className="md:col-span-2">
        <h3 className="text-white font-semibold mb-3 flex items-center gap-2"><Swords className="w-4 h-4 text-red-400" /> 最近对战</h3>
        {battles.length === 0 ? (
          <div className="bg-gray-900 border border-gray-800 rounded-xl p-8 text-center text-gray-500">暂无对战记录</div>
        ) : (
          <div className="space-y-2">
            {battles.slice(0, 20).map(b => {
              const won = b.winner_id === b.challenger_id
              return (
                <div key={b.id} className="bg-gray-900 border border-gray-800 rounded-xl p-4 flex items-center gap-3">
                  <div className={`w-10 h-10 rounded-lg flex items-center justify-center text-lg ${won ? 'bg-emerald-500/15' : 'bg-red-500/15'}`}>
                    {won ? '🏆' : '💀'}
                  </div>
                  <div className="flex-1 min-w-0">
                    <div className="text-sm text-white">
                      <span className="font-medium">{b.challenger_name}</span>
                      <span className="text-gray-500 mx-2">vs</span>
                      <span className="font-medium">{b.opponent_name}</span>
                    </div>
                    <div className="text-xs text-gray-500 mt-0.5">
                      {b.rounds} 回合 · +{b.xp_gained} XP · HP {b.challenger_hp_left} : {b.opponent_hp_left}
                    </div>
                  </div>
                  <div className="text-right">
                    <div className={`text-xs font-medium ${won ? 'text-emerald-400' : 'text-red-400'}`}>{won ? '胜利' : '失败'}</div>
                    <div className="text-xs text-gray-600 flex items-center gap-1"><Clock className="w-3 h-3" />{timeAgo(b.created_at)}</div>
                  </div>
                </div>
              )
            })}
          </div>
        )}
      </div>

      {/* Mini Leaderboard */}
      <div>
        <h3 className="text-white font-semibold mb-3 flex items-center gap-2"><Trophy className="w-4 h-4 text-amber-400" /> Top 10</h3>
        <div className="bg-gray-900 border border-gray-800 rounded-xl overflow-hidden">
          {leaderboard.slice(0, 10).map((f, i) => {
            const medals = ['🥇', '🥈', '🥉']
            return (
              <div key={f.id} className="flex items-center px-4 py-2.5 border-b border-gray-800 last:border-0">
                <span className="w-6 text-center text-sm">{i < 3 ? medals[i] : <span className="text-gray-600">{i + 1}</span>}</span>
                <span className="ml-2 mr-1.5">{PATH_EMOJI[f.evolution_path] ?? '🦞'}</span>
                <span className="text-sm text-white flex-1 truncate">{f.name}</span>
                <span className="text-xs text-amber-400 font-medium">{f.elo}</span>
              </div>
            )
          })}
          {leaderboard.length === 0 && <div className="p-6 text-center text-gray-600 text-sm">暂无数据</div>}
        </div>
      </div>
    </div>
  )
}

/* ── Shop Tab ── */
function ShopTab({ items }: { items: ShopItem[] }) {
  if (items.length === 0) return <div className="bg-gray-900 border border-gray-800 rounded-xl p-12 text-center text-gray-500">商店暂无商品</div>

  return (
    <div className="grid sm:grid-cols-2 lg:grid-cols-3 gap-4">
      {items.map(item => {
        const rc = RARITY_COLORS[item.rarity] ?? RARITY_COLORS.common
        return (
          <div key={item.id} className={`bg-gray-900 border ${rc.border} rounded-xl p-5`}>
            <div className="flex items-center justify-between mb-3">
              <span className={`text-xs font-medium px-2 py-0.5 rounded-full ${rc.bg} ${rc.text}`}>
                {RARITY_LABEL[item.rarity] ?? item.rarity}
              </span>
              <span className="text-xs text-gray-500">{item.slot}</span>
            </div>
            <h4 className={`font-semibold ${rc.text} mb-2`}>{item.name}</h4>
            <div className="grid grid-cols-2 gap-1 text-xs text-gray-400 mb-3">
              {item.attack_bonus > 0 && <span>⚔️ +{item.attack_bonus} 攻击</span>}
              {item.defense_bonus > 0 && <span>🛡️ +{item.defense_bonus} 防御</span>}
              {item.speed_bonus > 0 && <span>⚡ +{item.speed_bonus} 速度</span>}
              {item.hp_bonus > 0 && <span>❤️ +{item.hp_bonus} 生命</span>}
            </div>
            <div className="flex items-center justify-between">
              <span className="text-amber-400 font-bold">⚡{item.price}</span>
              <span className="text-xs text-gray-600">在 Claw 客户端购买</span>
            </div>
          </div>
        )
      })}
    </div>
  )
}

/* ── Mutations Tab ── */
function MutationsTab({ mutations }: { mutations: Mutation[] }) {
  if (mutations.length === 0) return (
    <div className="bg-gray-900 border border-gray-800 rounded-xl p-12 text-center">
      <div className="text-4xl mb-3">🧬</div>
      <div className="text-gray-500">还没有变异</div>
      <div className="text-xs text-gray-600 mt-1">在 Claw 客户端中触发变异</div>
    </div>
  )

  return (
    <div className="grid sm:grid-cols-2 gap-4">
      {mutations.map(m => {
        const rc = RARITY_COLORS[m.rarity] ?? RARITY_COLORS.common
        return (
          <div key={m.id} className={`bg-gray-900 border ${rc.border} rounded-xl p-5`}>
            <div className="flex items-center gap-2 mb-2">
              <Dna className={`w-4 h-4 ${rc.text}`} />
              <span className={`font-semibold ${rc.text}`}>{m.name}</span>
              <span className={`text-xs px-2 py-0.5 rounded-full ${rc.bg} ${rc.text}`}>
                {RARITY_LABEL[m.rarity] ?? m.rarity}
              </span>
            </div>
            <p className="text-sm text-gray-400 mb-3">{m.effect}</p>
            <div className="flex gap-3 text-xs text-gray-500">
              {m.attack_mod !== 0 && <span className={m.attack_mod > 0 ? 'text-emerald-400' : 'text-red-400'}>⚔️ {m.attack_mod > 0 ? '+' : ''}{m.attack_mod}</span>}
              {m.defense_mod !== 0 && <span className={m.defense_mod > 0 ? 'text-emerald-400' : 'text-red-400'}>🛡️ {m.defense_mod > 0 ? '+' : ''}{m.defense_mod}</span>}
              {m.speed_mod !== 0 && <span className={m.speed_mod > 0 ? 'text-emerald-400' : 'text-red-400'}>⚡ {m.speed_mod > 0 ? '+' : ''}{m.speed_mod}</span>}
              {m.hp_mod !== 0 && <span className={m.hp_mod > 0 ? 'text-emerald-400' : 'text-red-400'}>❤️ {m.hp_mod > 0 ? '+' : ''}{m.hp_mod}</span>}
            </div>
            <div className="text-xs text-gray-600 mt-2">{timeAgo(m.created_at)}</div>
          </div>
        )
      })}
    </div>
  )
}
