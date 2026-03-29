import { useState, useEffect } from 'react'
import { Navbar } from '../components/Navbar'
import { Footer } from '../components/Footer'
import { growthAPI, type Fighter, type Season } from '../lib/api'
import { isLoggedIn } from '../lib/auth'
import { useNavigate } from 'react-router-dom'
import { Swords, Shield, Zap, Heart, Star, TrendingUp, Crown } from 'lucide-react'

const PATH_EMOJI: Record<string, string> = { larva: '🥚', abyss: '🦑', terrain: '🦂', sky: '🦅' }
const PATH_LABEL: Record<string, string> = { larva: '幼虫', abyss: '深渊', terrain: '地表', sky: '天空' }
const ENV_EMOJI: Record<string, string> = { abyss: '🌊', terrain: '🏔️', sky: '☁️' }

function eloColor(elo: number) {
  if (elo >= 2000) return 'text-amber-500'
  if (elo >= 1500) return 'text-purple-500'
  if (elo >= 1200) return 'text-indigo-500'
  return 'text-gray-400'
}

function eloRank(elo: number) {
  if (elo >= 2000) return '传奇'
  if (elo >= 1500) return '精英'
  if (elo >= 1200) return '老手'
  return '新锐'
}

export function GrowthPage() {
  const navigate = useNavigate()
  const [fighter, setFighter] = useState<Fighter | null>(null)
  const [leaderboard, setLeaderboard] = useState<Fighter[]>([])
  const [season, setSeason] = useState<Season | null>(null)
  const [loading, setLoading] = useState(true)
  const [tab, setTab] = useState<'profile' | 'leaderboard'>('profile')

  useEffect(() => {
    if (!isLoggedIn()) { navigate('/auth'); return }
    load()
  }, [navigate])

  async function load() {
    setLoading(true)
    try {
      const [fRes, lbRes, sRes] = await Promise.all([
        growthAPI.fighter().catch(() => null),
        growthAPI.leaderboard().catch(() => null),
        growthAPI.season().catch(() => null),
      ])
      setFighter((fRes as any)?.fighter ?? null)
      setLeaderboard((lbRes as any)?.fighters ?? [])
      setSeason((sRes as any)?.season ?? null)
    } catch { /* ignore */ }
    setLoading(false)
  }

  if (loading) return <><Navbar /><div className="min-h-screen bg-gray-950 flex items-center justify-center text-gray-400">加载中...</div><Footer /></>

  return (
    <>
      <Navbar />
      <div className="min-h-screen bg-gray-950 pt-20 pb-16">
        <div className="max-w-5xl mx-auto px-4">

          {/* Header */}
          <div className="flex items-center justify-between mb-6">
            <div>
              <h1 className="text-2xl font-bold text-white flex items-center gap-2">
                <TrendingUp className="w-6 h-6 text-emerald-400" /> 宠物成长
              </h1>
              <p className="text-sm text-gray-500 mt-1">你的 Claw 节点养育的数字宠物</p>
            </div>
            <div className="flex gap-2">
              <button onClick={() => setTab('profile')} className={`px-4 py-2 rounded-lg text-sm font-medium transition ${tab === 'profile' ? 'bg-indigo-600 text-white' : 'bg-gray-800 text-gray-400 hover:text-white'}`}>我的宠物</button>
              <button onClick={() => setTab('leaderboard')} className={`px-4 py-2 rounded-lg text-sm font-medium transition ${tab === 'leaderboard' ? 'bg-indigo-600 text-white' : 'bg-gray-800 text-gray-400 hover:text-white'}`}>排行榜</button>
            </div>
          </div>

          {/* Season Banner */}
          {season && (
            <div className="bg-gradient-to-r from-indigo-900/40 to-purple-900/40 border border-indigo-800/50 rounded-xl p-4 mb-6 flex items-center gap-4">
              <span className="text-3xl">{ENV_EMOJI[season.environment] ?? '🌍'}</span>
              <div className="flex-1">
                <div className="text-white font-semibold">{season.name}</div>
                <div className="text-xs text-gray-400">环境: {season.environment} · {season.active ? '进行中' : '已结束'}</div>
              </div>
              <div className="bg-emerald-500/15 text-emerald-400 text-xs font-medium px-3 py-1 rounded-full">
                {season.active ? '🔥 赛季进行中' : '⏸ 已结束'}
              </div>
            </div>
          )}

          {tab === 'profile' ? (
            /* ── My Fighter Profile ── */
            fighter ? (
              <div className="grid md:grid-cols-3 gap-6">
                {/* Pet Card */}
                <div className="md:col-span-1 bg-gray-900 border border-gray-800 rounded-xl p-6 text-center">
                  <div className="text-6xl mb-4">{PATH_EMOJI[fighter.evolution_path] ?? '🦞'}</div>
                  <h2 className="text-xl font-bold text-white">{fighter.name}</h2>
                  <div className="text-sm text-gray-400 mt-1">
                    {PATH_LABEL[fighter.evolution_path] ?? fighter.evolution_path} · Lv.{fighter.level}
                  </div>
                  <div className="mt-3">
                    <span className={`text-lg font-bold ${eloColor(fighter.elo)}`}>ELO {fighter.elo}</span>
                    <span className="text-xs text-gray-500 ml-2">{eloRank(fighter.elo)}</span>
                  </div>
                  {/* XP Bar */}
                  <div className="mt-4">
                    <div className="flex justify-between text-xs text-gray-500 mb-1">
                      <span>经验值</span>
                      <span>{fighter.xp} XP</span>
                    </div>
                    <div className="w-full bg-gray-800 rounded-full h-2">
                      <div className="bg-indigo-500 h-2 rounded-full transition-all" style={{ width: `${Math.min(100, (fighter.xp % 1000) / 10)}%` }} />
                    </div>
                  </div>
                  {/* Win/Loss */}
                  <div className="mt-4 flex justify-center gap-4 text-sm">
                    <span className="text-emerald-400">{fighter.wins} 胜</span>
                    <span className="text-red-400">{fighter.losses} 负</span>
                    <span className="text-gray-500">{fighter.draws} 平</span>
                  </div>
                </div>

                {/* Stats */}
                <div className="md:col-span-2 grid grid-cols-2 gap-4">
                  <StatCard icon={Heart} label="生命值" value={fighter.hp} color="text-red-400" bg="bg-red-500/10" />
                  <StatCard icon={Swords} label="攻击力" value={fighter.attack} color="text-orange-400" bg="bg-orange-500/10" />
                  <StatCard icon={Shield} label="防御力" value={fighter.defense} color="text-blue-400" bg="bg-blue-500/10" />
                  <StatCard icon={Zap} label="速度" value={fighter.speed} color="text-yellow-400" bg="bg-yellow-500/10" />

                  {/* Quick Actions */}
                  <div className="col-span-2 bg-gray-900 border border-gray-800 rounded-xl p-5">
                    <h3 className="text-white font-semibold mb-3 flex items-center gap-2"><Star className="w-4 h-4 text-amber-400" /> 快捷操作</h3>
                    <div className="grid grid-cols-3 gap-3">
                      <button onClick={() => navigate('/arena')} className="bg-gray-800 hover:bg-gray-700 rounded-lg py-3 text-sm text-gray-300 transition">
                        🐲 龙虾社区
                      </button>
                      <button className="bg-gray-800 hover:bg-gray-700 rounded-lg py-3 text-sm text-gray-300 transition cursor-not-allowed opacity-60">
                        ⚔️ 发起对战
                      </button>
                      <button className="bg-gray-800 hover:bg-gray-700 rounded-lg py-3 text-sm text-gray-300 transition cursor-not-allowed opacity-60">
                        🛒 商店
                      </button>
                    </div>
                    <p className="text-xs text-gray-600 mt-3">对战和商店操作需在 Claw 客户端或 Larva 移动端中进行</p>
                  </div>
                </div>
              </div>
            ) : (
              <div className="bg-gray-900 border border-gray-800 rounded-xl p-12 text-center">
                <div className="text-5xl mb-4">🥚</div>
                <h2 className="text-xl font-bold text-white mb-2">还没有宠物</h2>
                <p className="text-gray-400 text-sm">绑定一个 Claw 节点并使用一段时间后，你的宠物将自动孵化</p>
              </div>
            )
          ) : (
            /* ── Leaderboard ── */
            <div className="bg-gray-900 border border-gray-800 rounded-xl overflow-hidden">
              <div className="px-6 py-4 border-b border-gray-800 flex items-center gap-2">
                <Crown className="w-5 h-5 text-amber-400" />
                <span className="text-white font-semibold">全服排行榜</span>
                <span className="text-xs text-gray-500 ml-2">{leaderboard.length} 位战士</span>
              </div>
              {leaderboard.length === 0 ? (
                <div className="p-12 text-center text-gray-500">暂无战士</div>
              ) : (
                <div className="divide-y divide-gray-800">
                  {leaderboard.map((f, i) => {
                    const medals = ['🥇', '🥈', '🥉']
                    return (
                      <div key={f.id} className="flex items-center px-6 py-3 hover:bg-gray-800/50 transition">
                        <div className="w-10 text-center">
                          {i < 3 ? <span className="text-lg">{medals[i]}</span> : <span className="text-sm text-gray-500">{i + 1}</span>}
                        </div>
                        <span className="text-xl mr-3">{PATH_EMOJI[f.evolution_path] ?? '🦞'}</span>
                        <div className="flex-1 min-w-0">
                          <div className="text-white font-medium truncate">{f.name}</div>
                          <div className="text-xs text-gray-500">Lv.{f.level} · {f.wins}胜 {f.losses}负</div>
                        </div>
                        <div className="text-right">
                          <div className={`font-bold ${eloColor(f.elo)}`}>{f.elo}</div>
                          <div className="text-xs text-gray-500">{eloRank(f.elo)}</div>
                        </div>
                      </div>
                    )
                  })}
                </div>
              )}
            </div>
          )}
        </div>
      </div>
      <Footer />
    </>
  )
}

function StatCard({ icon: Icon, label, value, color, bg }: { icon: any; label: string; value: number; color: string; bg: string }) {
  return (
    <div className="bg-gray-900 border border-gray-800 rounded-xl p-5 flex items-center gap-4">
      <div className={`w-12 h-12 rounded-lg ${bg} flex items-center justify-center`}>
        <Icon className={`w-6 h-6 ${color}`} />
      </div>
      <div>
        <div className="text-xs text-gray-500">{label}</div>
        <div className={`text-2xl font-bold ${color}`}>{value}</div>
      </div>
    </div>
  )
}
