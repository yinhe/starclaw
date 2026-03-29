import { useState, useEffect } from 'react'
import { arenaAPI, growthAPI, systemAPI } from '../lib/api'

type ArenaTab = 'leaderboard' | 'fighter' | 'shop' | 'history' | 'season' | 'craft' | 'stardust' | 'mutations'

interface Fighter {
  id: string
  claw_id: string
  name: string
  level: number
  evolution_path: string
  form_code: string
  base_hp: number
  base_atk: number
  base_def: number
  base_spd: number
  elo: number
  win_count: number
  lose_count: number
  win_streak: number
  weapon_id: string
  armor_id: string
  trinket_id: string
  last_battle_at: string | null
}

interface EquipItem {
  id: string
  claw_id: string
  def_id: string
  equipped: boolean
  bonus_hp: number
  bonus_atk: number
  bonus_def: number
  bonus_spd: number
  enhance_level: number
  def_name?: string
  def_slot?: string
  def_quality?: string
  special_desc?: string
}

interface ShopItem {
  id: string
  name: string
  slot: string
  quality: string
  path_only: string
  bonus_hp: number
  bonus_atk: number
  bonus_def: number
  bonus_spd: number
  crit_rate_bonus: number
  special_desc: string
  price_star: number
  price_dust: number
}

interface BattleRecord {
  id: string
  fighter_a_name: string
  fighter_a_path: string
  fighter_a_lv: number
  fighter_b_name: string
  fighter_b_path: string
  fighter_b_lv: number
  winner_id: string
  fighter_a_id: string
  fighter_b_id: string
  rounds: number
  elo_change_a: number
  elo_change_b: number
  created_at: string
}

const pathEmoji: Record<string, string> = { abyss: '🌊', terrain: '🏔️', sky: '🌪️', larva: '🥚' }
const qualityColor: Record<string, string> = {
  white: 'text-gray-300', green: 'text-green-400', blue: 'text-blue-400',
  purple: 'text-purple-400', orange: 'text-orange-400',
}
const qualityBg: Record<string, string> = {
  white: 'border-gray-600', green: 'border-green-600', blue: 'border-blue-600',
  purple: 'border-purple-600', orange: 'border-orange-600',
}

export default function ArenaPage() {
  const [tab, setTab] = useState<ArenaTab>('leaderboard')
  const [clawId, setClawId] = useState('')
  const [myFighter, setMyFighter] = useState<Fighter | null>(null)
  const [registered, setRegistered] = useState(false)

  // Load claw ID on mount
  useEffect(() => {
    systemAPI.getSwarm().then(res => {
      const id = res.data?.node_id || res.data?.claw_id || ''
      setClawId(id)
      if (id) {
        arenaAPI.getFighter(id)
          .then(r => { setMyFighter(r.data?.fighter); setRegistered(true) })
          .catch(() => setRegistered(false))
      }
    }).catch(() => {})
  }, [])

  const handleRegister = async () => {
    if (!clawId) return
    try {
      const growth = await growthAPI.getGrowth()
      const p = growth.data
      await arenaAPI.registerFighter({
        claw_id: clawId,
        name: p.title || '小龙虾',
        level: p.stats.level,
        evolution_path: p.evolution_path,
        form_code: p.form_code,
        base_hp: p.stats.hp,
        base_atk: p.stats.atk,
        base_def: p.stats.def,
        base_spd: p.stats.spd,
      })
      const f = await arenaAPI.getFighter(clawId)
      setMyFighter(f.data?.fighter)
      setRegistered(true)
      setTab('fighter')
    } catch {
      alert('注册失败，请确认已加入虫群网络')
    }
  }

  return (
    <div className="max-w-4xl mx-auto p-4 space-y-4">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold text-white">🦞 龙虾竞技场</h1>
        {myFighter && (
          <span className="text-sm text-gray-400">
            ELO {myFighter.elo} · {myFighter.win_count}胜 {myFighter.lose_count}负
          </span>
        )}
      </div>

      {/* Tab Bar */}
      <div className="space-y-1">
        <div className="flex gap-1 bg-gray-900/60 rounded-xl p-1 border border-gray-700/50">
          {([['leaderboard', '🏆', '排行榜'], ['fighter', '⚔️', '宠物'], ['shop', '🏪', '商店'], ['history', '📜', '记录']] as const).map(([key, icon, label]) => (
            <button key={key} onClick={() => setTab(key)}
              className={`flex-1 py-2 px-2 rounded-lg text-sm font-medium transition-all ${tab === key ? 'bg-gray-700 text-white shadow-sm' : 'text-gray-400 hover:text-gray-200 hover:bg-gray-800/50'}`}
            >{icon} {label}</button>
          ))}
        </div>
        <div className="flex gap-1 bg-gray-900/60 rounded-xl p-1 border border-gray-700/50">
          {([['season', '🌊', '赛季'], ['craft', '🔨', '打造'], ['stardust', '✨', '星尘'], ['mutations', '🧬', '变异']] as const).map(([key, icon, label]) => (
            <button key={key} onClick={() => setTab(key)}
              className={`flex-1 py-2 px-2 rounded-lg text-sm font-medium transition-all ${tab === key ? 'bg-gray-700 text-white shadow-sm' : 'text-gray-400 hover:text-gray-200 hover:bg-gray-800/50'}`}
            >{icon} {label}</button>
          ))}
        </div>
      </div>

      {!registered && tab !== 'leaderboard' && tab !== 'shop' && (
        <div className="bg-gray-900/80 rounded-xl p-6 border border-gray-700/50 text-center space-y-3">
          <p className="text-gray-300">你的宠物还未注册竞技场</p>
          <button onClick={handleRegister} className="bg-emerald-600 hover:bg-emerald-500 text-white px-6 py-2 rounded-lg font-medium">
            🦞 注册参战
          </button>
          <p className="text-xs text-gray-500">将同步你的成长属性到竞技场</p>
        </div>
      )}

      {tab === 'leaderboard' && <LeaderboardTab clawId={clawId} myFighter={myFighter} />}
      {tab === 'fighter' && registered && myFighter && <FighterTab fighter={myFighter} clawId={clawId} onUpdate={f => setMyFighter(f)} />}
      {tab === 'shop' && <ShopTab clawId={clawId} registered={registered} />}
      {tab === 'history' && registered && myFighter && <HistoryTab clawId={clawId} fighterId={myFighter.id} />}
      {tab === 'season' && <SeasonTab clawId={clawId} />}
      {tab === 'craft' && registered && <CraftTab clawId={clawId} />}
      {tab === 'stardust' && registered && <StardustTab clawId={clawId} />}
      {tab === 'mutations' && registered && <MutationsTab clawId={clawId} />}
    </div>
  )
}

function LeaderboardTab({ clawId, myFighter }: { clawId: string; myFighter: Fighter | null }) {
  const [fighters, setFighters] = useState<Fighter[]>([])
  const [loading, setLoading] = useState(true)
  const [battleResult, setBattleResult] = useState<any>(null)

  useEffect(() => {
    arenaAPI.getLeaderboard()
      .then(r => { setFighters(r.data?.leaderboard || []); setLoading(false) })
      .catch(() => setLoading(false))
  }, [])

  const handleChallenge = async (opponentClawId: string) => {
    if (!clawId || !myFighter) return
    try {
      const res = await arenaAPI.challenge(clawId, opponentClawId)
      setBattleResult(res.data)
    } catch {
      alert('挑战失败')
    }
  }

  if (loading) return <div className="text-center py-12 text-gray-400">加载排行榜...</div>

  return (
    <div className="space-y-3">
      {battleResult && (
        <BattleResultCard result={battleResult} onClose={() => setBattleResult(null)} />
      )}
      {fighters.length === 0 && (
        <div className="text-center py-12 text-gray-500">竞技场暂无选手</div>
      )}
      {fighters.map((f, i) => (
        <div key={f.id} className="bg-gray-900/80 rounded-xl p-4 border border-gray-700/50 flex items-center gap-4">
          <span className="text-xl font-bold text-gray-500 w-8 text-center">
            {i === 0 ? '🥇' : i === 1 ? '🥈' : i === 2 ? '🥉' : `#${i + 1}`}
          </span>
          <div className="flex-1">
            <div className="flex items-center gap-2">
              <span className="text-white font-medium">{f.name}</span>
              <span className="text-xs">{pathEmoji[f.evolution_path] || '🥚'}</span>
              <span className="text-xs text-gray-400">Lv.{f.level}</span>
            </div>
            <div className="text-xs text-gray-500 mt-0.5">
              ELO {f.elo} · {f.win_count}W {f.lose_count}L
              {f.win_streak >= 3 && <span className="text-amber-400 ml-1">🔥{f.win_streak}连胜</span>}
            </div>
          </div>
          <div className="flex gap-2 text-[10px] text-gray-500">
            <span>HP {f.base_hp}</span>
            <span>ATK {f.base_atk}</span>
            <span>DEF {f.base_def}</span>
            <span>SPD {f.base_spd}</span>
          </div>
          {myFighter && f.claw_id !== clawId && (
            <button
              onClick={() => handleChallenge(f.claw_id)}
              className="bg-red-600/80 hover:bg-red-500 text-white text-xs px-3 py-1.5 rounded-lg"
            >
              ⚔️ 挑战
            </button>
          )}
        </div>
      ))}
    </div>
  )
}

function BattleResultCard({ result, onClose }: { result: any; onClose: () => void }) {
  const battle = result.battle
  const r = result.result
  const drops = result.drops
  const winnerIsA = r.winner_id === battle.fighter_a_id
  const [showLog, setShowLog] = useState(false)

  const dropIcons: Record<string, string> = {
    'mat-shell': '🐚', 'mat-claw': '🦀', 'mat-crystal': '💎',
    'mat-feather': '🪶', 'mat-stone': '🪨', 'mat-pearl': '🔮',
    'mat-ink': '🖤', 'mat-scale': '🐉',
  }

  return (
    <div className="bg-gray-900 rounded-xl p-5 border-2 border-amber-500/50 space-y-3 animate-[fadeIn_0.3s_ease-out]">
      <style>{`
        @keyframes fadeIn { from { opacity: 0; transform: translateY(-10px); } to { opacity: 1; transform: translateY(0); } }
        @keyframes dropBounce { 0% { opacity: 0; transform: scale(0) translateY(-20px); } 50% { transform: scale(1.2) translateY(0); } 100% { opacity: 1; transform: scale(1) translateY(0); } }
        @keyframes victoryPulse { 0%,100% { text-shadow: 0 0 8px rgba(245,158,11,0.3); } 50% { text-shadow: 0 0 20px rgba(245,158,11,0.8); } }
        .drop-item { animation: dropBounce 0.4s ease-out forwards; opacity: 0; }
        .victory-text { animation: victoryPulse 1.5s ease-in-out infinite; }
      `}</style>

      <div className="flex items-center justify-between">
        <h3 className="text-lg font-bold text-white">⚔️ 战斗结果</h3>
        <button onClick={onClose} className="text-gray-400 hover:text-white text-sm">✕</button>
      </div>

      {/* Fighter matchup */}
      <div className="flex items-center justify-center gap-6 py-2">
        <div className={`text-center transition-all ${winnerIsA ? 'text-emerald-400 scale-110' : 'text-red-400 opacity-70'}`}>
          <div className="text-3xl">{pathEmoji[battle.fighter_a_path] || '🥚'}</div>
          <div className="text-sm font-medium">{battle.fighter_a_name}</div>
          <div className="text-xs">Lv.{battle.fighter_a_lv}</div>
          <div className="mt-1">
            <div className="w-20 h-1.5 bg-gray-700 rounded-full mx-auto overflow-hidden">
              <div className="h-full bg-red-500 rounded-full transition-all" style={{ width: `${Math.max(0, (r.final_hp_a / (battle.fighter_a_lv * 10 + 50)) * 100)}%` }} />
            </div>
            <div className="text-[10px] mt-0.5">HP {r.final_hp_a}</div>
          </div>
          {winnerIsA && <div className="text-amber-400 text-xs mt-1">👑 胜者</div>}
        </div>
        <div className="text-gray-600 text-2xl font-black">VS</div>
        <div className={`text-center transition-all ${!winnerIsA ? 'text-emerald-400 scale-110' : 'text-red-400 opacity-70'}`}>
          <div className="text-3xl">{pathEmoji[battle.fighter_b_path] || '🥚'}</div>
          <div className="text-sm font-medium">{battle.fighter_b_name}</div>
          <div className="text-xs">Lv.{battle.fighter_b_lv}</div>
          <div className="mt-1">
            <div className="w-20 h-1.5 bg-gray-700 rounded-full mx-auto overflow-hidden">
              <div className="h-full bg-red-500 rounded-full transition-all" style={{ width: `${Math.max(0, (r.final_hp_b / (battle.fighter_b_lv * 10 + 50)) * 100)}%` }} />
            </div>
            <div className="text-[10px] mt-0.5">HP {r.final_hp_b}</div>
          </div>
          {!winnerIsA && <div className="text-amber-400 text-xs mt-1">👑 胜者</div>}
        </div>
      </div>

      {/* Victory banner */}
      <div className="text-center py-1">
        <span className="text-amber-400 font-bold text-lg victory-text">
          🏆 {winnerIsA ? battle.fighter_a_name : battle.fighter_b_name} 胜利！
        </span>
        <div className="text-xs text-gray-500 mt-0.5">{r.rounds} 回合 · ELO {battle.elo_change_a > 0 ? '+' : ''}{battle.elo_change_a} / {battle.elo_change_b > 0 ? '+' : ''}{battle.elo_change_b}</div>
      </div>

      {/* Battle drops */}
      {drops && (drops.a?.length > 0 || drops.b?.length > 0) && (
        <div className="bg-gray-800/40 rounded-lg p-3">
          <div className="text-xs text-gray-400 mb-2">🎁 战利品</div>
          <div className="grid grid-cols-2 gap-3">
            {drops.a?.length > 0 && (
              <div>
                <div className="text-[10px] text-gray-500 mb-1">{battle.fighter_a_name}</div>
                <div className="flex flex-wrap gap-1">
                  {drops.a.map((d: any, i: number) => (
                    <span key={i} className="drop-item inline-flex items-center gap-0.5 bg-gray-700/50 rounded px-1.5 py-0.5 text-[10px] text-gray-300"
                      style={{ animationDelay: `${i * 0.15}s` }}>
                      {dropIcons[d.material_id] || '📦'} {d.name || d.material_id} ×{d.quantity}
                    </span>
                  ))}
                </div>
              </div>
            )}
            {drops.b?.length > 0 && (
              <div>
                <div className="text-[10px] text-gray-500 mb-1">{battle.fighter_b_name}</div>
                <div className="flex flex-wrap gap-1">
                  {drops.b.map((d: any, i: number) => (
                    <span key={i} className="drop-item inline-flex items-center gap-0.5 bg-gray-700/50 rounded px-1.5 py-0.5 text-[10px] text-gray-300"
                      style={{ animationDelay: `${(i + (drops.a?.length || 0)) * 0.15}s` }}>
                      {dropIcons[d.material_id] || '📦'} {d.name || d.material_id} ×{d.quantity}
                    </span>
                  ))}
                </div>
              </div>
            )}
          </div>
        </div>
      )}

      {/* Battle log toggle */}
      {r.log && r.log.length > 0 && (
        <div>
          <button onClick={() => setShowLog(!showLog)} className="text-xs text-gray-500 hover:text-gray-300 w-full text-center py-1">
            {showLog ? '▲ 收起战斗日志' : '▼ 展开战斗日志'}
          </button>
          {showLog && (
            <div className="bg-gray-800/50 rounded-lg p-3 max-h-40 overflow-y-auto text-xs text-gray-400 space-y-0.5 mt-1">
              {r.log.map((l: any, i: number) => (
                <div key={i} className={l.crit ? 'text-amber-400 font-medium' : ''}>
                  <span className="text-gray-600">R{l.round}</span> {l.detail}
                  {l.crit && ' 💥'}
                </div>
              ))}
            </div>
          )}
        </div>
      )}
    </div>
  )
}

function FighterTab({ fighter, clawId, onUpdate }: { fighter: Fighter; clawId: string; onUpdate: (f: Fighter) => void }) {
  const [inventory, setInventory] = useState<EquipItem[]>([])

  useEffect(() => {
    arenaAPI.getInventory(clawId)
      .then(r => setInventory(r.data?.items || []))
      .catch(() => {})
  }, [clawId])

  const handleSync = async () => {
    try {
      const growth = await growthAPI.getGrowth()
      const p = growth.data
      await arenaAPI.registerFighter({
        claw_id: clawId,
        name: p.title || fighter.name,
        level: p.stats.level,
        evolution_path: p.evolution_path,
        form_code: p.form_code,
        base_hp: p.stats.hp,
        base_atk: p.stats.atk,
        base_def: p.stats.def,
        base_spd: p.stats.spd,
      })
      const f = await arenaAPI.getFighter(clawId)
      onUpdate(f.data?.fighter)
    } catch {
      alert('同步失败')
    }
  }

  const handleEquip = async (itemId: string) => {
    await arenaAPI.equip(clawId, itemId)
    const f = await arenaAPI.getFighter(clawId)
    onUpdate(f.data?.fighter)
    const inv = await arenaAPI.getInventory(clawId)
    setInventory(inv.data?.items || [])
  }

  const handleUnequip = async (slot: string) => {
    await arenaAPI.unequip(clawId, slot)
    const f = await arenaAPI.getFighter(clawId)
    onUpdate(f.data?.fighter)
    const inv = await arenaAPI.getInventory(clawId)
    setInventory(inv.data?.items || [])
  }

  const equipped = inventory.filter(i => i.equipped)
  const backpack = inventory.filter(i => !i.equipped)

  return (
    <div className="space-y-4">
      {/* Fighter card */}
      <div className="bg-gray-900/80 rounded-xl p-5 border border-gray-700/50">
        <div className="flex items-start justify-between mb-4">
          <div>
            <div className="flex items-center gap-2">
              <span className="text-3xl">{pathEmoji[fighter.evolution_path] || '🥚'}</span>
              <div>
                <h3 className="text-lg font-bold text-white">{fighter.name}</h3>
                <span className="text-xs text-gray-400">Lv.{fighter.level} · ELO {fighter.elo}</span>
              </div>
            </div>
          </div>
          <button onClick={handleSync} className="text-xs bg-gray-700 hover:bg-gray-600 text-white px-3 py-1 rounded-lg">
            🔄 同步属性
          </button>
        </div>

        <div className="grid grid-cols-4 gap-3 mb-3">
          <StatBox label="❤️ HP" value={fighter.base_hp} color="text-red-400" />
          <StatBox label="⚔️ ATK" value={fighter.base_atk} color="text-orange-400" />
          <StatBox label="🛡️ DEF" value={fighter.base_def} color="text-blue-400" />
          <StatBox label="⚡ SPD" value={fighter.base_spd} color="text-green-400" />
        </div>

        <div className="flex gap-4 text-xs text-gray-400">
          <span>🏆 {fighter.win_count}W {fighter.lose_count}L</span>
          {fighter.win_streak >= 3 && <span className="text-amber-400">🔥 {fighter.win_streak}连胜</span>}
        </div>
      </div>

      {/* Equipped items */}
      <div className="bg-gray-900/80 rounded-xl p-4 border border-gray-700/50">
        <h3 className="text-sm font-semibold text-gray-300 mb-3">🎒 已装备</h3>
        <div className="grid grid-cols-3 gap-3">
          {(['weapon', 'armor', 'trinket'] as const).map(slot => {
            const item = equipped.find(i => i.def_slot === slot)
            const slotLabel = slot === 'weapon' ? '🗡️ 武器' : slot === 'armor' ? '🛡️ 护甲' : '💍 饰品'
            return (
              <div key={slot} className={`rounded-lg p-3 border ${item ? qualityBg[item.def_quality || 'white'] : 'border-gray-700 border-dashed'} bg-gray-800/50`}>
                <div className="text-xs text-gray-500 mb-1">{slotLabel}</div>
                {item ? (
                  <>
                    <div className={`text-sm font-medium ${qualityColor[item.def_quality || 'white']}`}>{item.def_name}</div>
                    <div className="text-[10px] text-gray-500 mt-1">
                      {item.bonus_atk > 0 && `ATK+${item.bonus_atk} `}
                      {item.bonus_def > 0 && `DEF+${item.bonus_def} `}
                      {item.bonus_hp > 0 && `HP+${item.bonus_hp} `}
                      {item.bonus_spd > 0 && `SPD+${item.bonus_spd}`}
                    </div>
                    {item.special_desc && <div className="text-[10px] text-amber-400/70 mt-0.5">{item.special_desc}</div>}
                    <button onClick={() => handleUnequip(slot)} className="text-[10px] text-red-400 hover:text-red-300 mt-1">卸下</button>
                  </>
                ) : (
                  <div className="text-xs text-gray-600">空</div>
                )}
              </div>
            )
          })}
        </div>
      </div>

      {/* Backpack */}
      {backpack.length > 0 && (
        <div className="bg-gray-900/80 rounded-xl p-4 border border-gray-700/50">
          <h3 className="text-sm font-semibold text-gray-300 mb-3">📦 背包 ({backpack.length})</h3>
          <div className="space-y-2">
            {backpack.map(item => (
              <div key={item.id} className={`flex items-center gap-3 p-2 rounded-lg border ${qualityBg[item.def_quality || 'white']} bg-gray-800/30`}>
                <div className="flex-1">
                  <span className={`text-sm font-medium ${qualityColor[item.def_quality || 'white']}`}>{item.def_name}</span>
                  <span className="text-[10px] text-gray-500 ml-2">
                    {item.def_slot === 'weapon' ? '🗡️' : item.def_slot === 'armor' ? '🛡️' : '💍'}
                    {item.bonus_atk > 0 && ` ATK+${item.bonus_atk}`}
                    {item.bonus_def > 0 && ` DEF+${item.bonus_def}`}
                    {item.bonus_hp > 0 && ` HP+${item.bonus_hp}`}
                    {item.bonus_spd > 0 && ` SPD+${item.bonus_spd}`}
                  </span>
                </div>
                <button onClick={() => handleEquip(item.id)} className="text-xs bg-emerald-600/80 hover:bg-emerald-500 text-white px-2 py-1 rounded">装备</button>
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  )
}

function ShopTab({ clawId, registered }: { clawId: string; registered: boolean }) {
  const [items, setItems] = useState<ShopItem[]>([])
  const [loading, setLoading] = useState(true)
  const [buying, setBuying] = useState('')

  useEffect(() => {
    arenaAPI.getShop()
      .then(r => { setItems(r.data?.items || []); setLoading(false) })
      .catch(() => setLoading(false))
  }, [])

  const handleBuy = async (defId: string) => {
    if (!clawId || !registered) { alert('请先注册竞技场'); return }
    setBuying(defId)
    try {
      await arenaAPI.buyEquipment(clawId, defId)
      alert('购买成功！查看背包')
    } catch {
      alert('购买失败')
    }
    setBuying('')
  }

  if (loading) return <div className="text-center py-12 text-gray-400">加载商店...</div>

  const grouped: Record<string, ShopItem[]> = {}
  items.forEach(it => {
    const key = it.slot === 'weapon' ? '🗡️ 武器' : it.slot === 'armor' ? '🛡️ 护甲' : '💍 饰品'
    if (!grouped[key]) grouped[key] = []
    grouped[key].push(it)
  })

  return (
    <div className="space-y-4">
      {Object.entries(grouped).map(([cat, catItems]) => (
        <div key={cat} className="bg-gray-900/80 rounded-xl p-4 border border-gray-700/50">
          <h3 className="text-sm font-semibold text-gray-300 mb-3">{cat}</h3>
          <div className="space-y-2">
            {catItems.map(item => (
              <div key={item.id} className={`flex items-center gap-3 p-3 rounded-lg border ${qualityBg[item.quality]} bg-gray-800/30`}>
                <div className="flex-1">
                  <div className="flex items-center gap-2">
                    <span className={`font-medium ${qualityColor[item.quality]}`}>{item.name}</span>
                    {item.path_only && <span className="text-[10px] text-gray-500">{pathEmoji[item.path_only]} 专属</span>}
                  </div>
                  <div className="text-[10px] text-gray-500 mt-0.5">
                    {item.bonus_atk > 0 && `ATK+${item.bonus_atk} `}
                    {item.bonus_def > 0 && `DEF+${item.bonus_def} `}
                    {item.bonus_hp > 0 && `HP+${item.bonus_hp} `}
                    {item.bonus_spd > 0 && `SPD+${item.bonus_spd} `}
                    {item.crit_rate_bonus > 0 && `暴击+${item.crit_rate_bonus}%`}
                  </div>
                  {item.special_desc && <div className="text-[10px] text-amber-400/70 mt-0.5">{item.special_desc}</div>}
                </div>
                <div className="text-right">
                  {item.price_star > 0 && <div className="text-xs text-yellow-400">⚡{item.price_star}</div>}
                  {item.price_dust > 0 && <div className="text-xs text-purple-400">✨{item.price_dust}</div>}
                  <button
                    onClick={() => handleBuy(item.id)}
                    disabled={buying === item.id}
                    className="text-xs bg-gray-700 hover:bg-gray-600 text-white px-2 py-1 rounded mt-1 disabled:opacity-50"
                  >
                    {buying === item.id ? '...' : '购买'}
                  </button>
                </div>
              </div>
            ))}
          </div>
        </div>
      ))}
    </div>
  )
}

function HistoryTab({ clawId, fighterId }: { clawId: string; fighterId: string }) {
  const [battles, setBattles] = useState<BattleRecord[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    arenaAPI.getHistory(clawId)
      .then(r => { setBattles(r.data?.battles || []); setLoading(false) })
      .catch(() => setLoading(false))
  }, [clawId])

  if (loading) return <div className="text-center py-12 text-gray-400">加载战斗记录...</div>
  if (battles.length === 0) return <div className="text-center py-12 text-gray-500">暂无战斗记录</div>

  return (
    <div className="space-y-2">
      {battles.map(b => {
        const isA = b.fighter_a_id === fighterId
        const won = b.winner_id === fighterId
        const eloChange = isA ? b.elo_change_a : b.elo_change_b
        const opponent = isA ? b.fighter_b_name : b.fighter_a_name
        const opPath = isA ? b.fighter_b_path : b.fighter_a_path
        const opLv = isA ? b.fighter_b_lv : b.fighter_a_lv

        return (
          <div key={b.id} className={`rounded-xl p-3 border ${won ? 'border-emerald-700/50 bg-emerald-900/10' : 'border-red-700/50 bg-red-900/10'}`}>
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-2">
                <span className={`text-sm font-bold ${won ? 'text-emerald-400' : 'text-red-400'}`}>
                  {won ? '✅ 胜' : '❌ 败'}
                </span>
                <span className="text-sm text-white">vs {pathEmoji[opPath] || '🥚'} {opponent}</span>
                <span className="text-xs text-gray-500">Lv.{opLv}</span>
              </div>
              <div className="text-right">
                <span className={`text-xs font-mono ${eloChange > 0 ? 'text-emerald-400' : 'text-red-400'}`}>
                  ELO {eloChange > 0 ? '+' : ''}{eloChange}
                </span>
                <div className="text-[10px] text-gray-500">{b.rounds}回合</div>
              </div>
            </div>
          </div>
        )
      })}
    </div>
  )
}

function StatBox({ label, value, color }: { label: string; value: number; color: string }) {
  return (
    <div className="bg-gray-800/50 rounded-lg p-2 text-center">
      <div className="text-xs text-gray-500">{label}</div>
      <div className={`text-lg font-bold ${color}`}>{value}</div>
    </div>
  )
}

// ─── Season Tab ───
function SeasonTab({ clawId }: { clawId: string }) {
  const [season, setSeason] = useState<any>(null)
  const [record, setRecord] = useState<any>(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    Promise.all([
      arenaAPI.getSeason().catch(() => ({ data: {} })),
      clawId ? arenaAPI.getSeasonRecord(clawId).catch(() => ({ data: {} })) : Promise.resolve({ data: {} }),
    ]).then(([s, r]) => {
      setSeason(s.data?.season)
      setRecord(r.data?.record)
      setLoading(false)
    })
  }, [clawId])

  if (loading) return <div className="text-center py-12 text-gray-400">加载赛季...</div>
  if (!season) return <div className="text-center py-12 text-gray-500">暂无活跃赛季</div>

  const envEmoji: Record<string, string> = { abyss: '🌊 深渊', terrain: '🏔️ 大地', sky: '🌪️ 穹天' }

  return (
    <div className="space-y-4">
      <div className="bg-gray-900/80 rounded-xl p-5 border border-gray-700/50">
        <div className="flex items-center justify-between mb-3">
          <h3 className="text-lg font-bold text-white">🏟️ {season.name}</h3>
          <span className="text-xs bg-emerald-600/30 text-emerald-400 px-2 py-0.5 rounded">赛季 #{season.number}</span>
        </div>
        <div className="space-y-2 text-sm text-gray-300">
          <div>🌍 赛季环境: <span className="text-white font-medium">{envEmoji[season.environment] || season.environment}</span></div>
          <div className="text-xs text-gray-500">
            该路线宠物获得 ATK+{season.path_atk_bonus}% / DEF+{season.path_def_bonus}% / SPD+{season.path_spd_bonus}% 加成
          </div>
          <div className="text-xs text-gray-500">
            ⚠️ 赛季内无战斗记录者扣除 {season.inactive_decay} ELO
          </div>
          <div className="text-xs text-gray-500">
            📅 {new Date(season.start_at).toLocaleDateString()} ~ {new Date(season.end_at).toLocaleDateString()}
          </div>
        </div>
      </div>

      {record && (
        <div className="bg-gray-900/80 rounded-xl p-4 border border-gray-700/50">
          <h3 className="text-sm font-semibold text-gray-300 mb-3">📊 我的赛季战绩</h3>
          <div className="grid grid-cols-4 gap-3">
            <StatBox label="胜" value={record.wins} color="text-emerald-400" />
            <StatBox label="负" value={record.losses} color="text-red-400" />
            <StatBox label="峰值ELO" value={record.peak_elo} color="text-amber-400" />
            <StatBox label="排名" value={record.season_rank || 0} color="text-blue-400" />
          </div>
        </div>
      )}
    </div>
  )
}

// ─── Stardust Tab ───
function StardustTab({ clawId }: { clawId: string }) {
  const [account, setAccount] = useState<any>(null)
  const [txns, setTxns] = useState<any[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    arenaAPI.getStardust(clawId)
      .then(r => { setAccount(r.data?.account); setTxns(r.data?.transactions || []); setLoading(false) })
      .catch(() => setLoading(false))
  }, [clawId])

  if (loading) return <div className="text-center py-12 text-gray-400">加载星尘...</div>

  const typeLabel: Record<string, string> = {
    cogen: '⚡ 伴生', battle_reward: '⚔️ 战斗奖励', craft_spend: '🔨 打造消耗', shop_spend: '🏪 商店消耗',
  }

  return (
    <div className="space-y-4">
      <div className="bg-gray-900/80 rounded-xl p-5 border border-purple-700/30">
        <div className="flex items-center justify-between">
          <div>
            <div className="text-xs text-gray-400">✨ 星尘余额</div>
            <div className="text-3xl font-bold text-purple-400">{account?.balance || 0}</div>
          </div>
          <div className="text-right text-xs text-gray-500">
            <div>总收入: {account?.total_in || 0}</div>
            <div>总支出: {account?.total_out || 0}</div>
          </div>
        </div>
        <p className="text-[10px] text-gray-500 mt-2">消耗星能时自动伴生星尘 (10:1)，用于购买紫色装备和打造</p>
      </div>

      {txns.length > 0 && (
        <div className="bg-gray-900/80 rounded-xl p-4 border border-gray-700/50">
          <h3 className="text-sm font-semibold text-gray-300 mb-3">📋 流水记录</h3>
          <div className="space-y-1.5 max-h-60 overflow-y-auto">
            {txns.map((t: any, i: number) => (
              <div key={i} className="flex items-center justify-between text-xs py-1 border-b border-gray-800/50">
                <div>
                  <span className="text-gray-400">{typeLabel[t.type] || t.type}</span>
                  <span className="text-gray-500 ml-2">{t.remark}</span>
                </div>
                <span className={t.amount > 0 ? 'text-emerald-400' : 'text-red-400'}>
                  {t.amount > 0 ? '+' : ''}{t.amount}
                </span>
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  )
}

// ─── Craft Tab ───
function CraftTab({ clawId }: { clawId: string }) {
  const [recipes, setRecipes] = useState<any[]>([])
  const [myMats, setMyMats] = useState<any[]>([])
  const [loading, setLoading] = useState(true)
  const [crafting, setCrafting] = useState('')
  const [collecting, setCollecting] = useState(false)

  const loadData = () => {
    Promise.all([
      arenaAPI.getRecipes().catch(() => ({ data: { recipes: [] } })),
      arenaAPI.getMyMaterials(clawId).catch(() => ({ data: { materials: [] } })),
    ]).then(([r, m]) => {
      setRecipes(r.data?.recipes || [])
      setMyMats(m.data?.materials || [])
      setLoading(false)
    })
  }

  useEffect(() => { loadData() }, [clawId])

  const handleCollect = async () => {
    setCollecting(true)
    try {
      const res = await arenaAPI.collectMaterials(clawId)
      const items = res.data?.collected || []
      alert(`采集成功！获得 ${items.map((i: any) => `${i.material_id} ×${i.quantity}`).join(', ')}`)
      loadData()
    } catch { alert('采集失败') }
    setCollecting(false)
  }

  const handleCraft = async (recipeId: string) => {
    setCrafting(recipeId)
    try {
      await arenaAPI.craft(clawId, recipeId)
      alert('打造成功！查看背包')
      loadData()
    } catch (e: any) {
      alert(e.response?.data?.error || '打造失败')
    }
    setCrafting('')
  }

  if (loading) return <div className="text-center py-12 text-gray-400">加载打造系统...</div>

  const rarityColor: Record<string, string> = { common: 'text-gray-300', uncommon: 'text-green-400', rare: 'text-blue-400', epic: 'text-purple-400' }

  return (
    <div className="space-y-4">
      {/* My Materials */}
      <div className="bg-gray-900/80 rounded-xl p-4 border border-gray-700/50">
        <div className="flex items-center justify-between mb-3">
          <h3 className="text-sm font-semibold text-gray-300">🎒 我的材料</h3>
          <button onClick={handleCollect} disabled={collecting}
            className="text-xs bg-emerald-600/80 hover:bg-emerald-500 text-white px-3 py-1 rounded-lg disabled:opacity-50">
            {collecting ? '采集中...' : '🌿 每日采集'}
          </button>
        </div>
        {myMats.length === 0 ? (
          <div className="text-xs text-gray-500 text-center py-3">暂无材料，点击每日采集获取</div>
        ) : (
          <div className="flex flex-wrap gap-2">
            {myMats.map((m: any) => (
              <div key={m.material_id} className="bg-gray-800/50 rounded-lg px-3 py-1.5 flex items-center gap-1.5">
                <span>{m.icon}</span>
                <span className={`text-xs ${rarityColor[m.rarity] || 'text-gray-300'}`}>{m.name}</span>
                <span className="text-xs text-white font-bold">×{m.quantity}</span>
              </div>
            ))}
          </div>
        )}
      </div>

      {/* Recipes */}
      <div className="bg-gray-900/80 rounded-xl p-4 border border-gray-700/50">
        <h3 className="text-sm font-semibold text-gray-300 mb-3">📜 打造配方</h3>
        <div className="space-y-2">
          {recipes.map((r: any) => {
            let mats: any[] = []
            try { mats = JSON.parse(r.materials) } catch {}
            return (
              <div key={r.id} className="bg-gray-800/30 rounded-lg p-3 border border-gray-700/50">
                <div className="flex items-center justify-between">
                  <div>
                    <span className="text-sm font-medium text-white">{r.result_name}</span>
                    {r.level_req > 0 && <span className="text-[10px] text-gray-500 ml-2">Lv.{r.level_req}+</span>}
                  </div>
                  <button onClick={() => handleCraft(r.id)} disabled={crafting === r.id}
                    className="text-xs bg-amber-600/80 hover:bg-amber-500 text-white px-3 py-1 rounded disabled:opacity-50">
                    {crafting === r.id ? '...' : '🔨 打造'}
                  </button>
                </div>
                <div className="text-[10px] text-gray-500 mt-1">
                  需要: {mats.map((m: any) => `${m.material_id} ×${m.quantity}`).join(' + ')}
                  {r.dust_cost > 0 && ` + ✨${r.dust_cost}`}
                </div>
              </div>
            )
          })}
        </div>
      </div>
    </div>
  )
}

// ─── Mutations Tab ───
function MutationsTab({ clawId }: { clawId: string }) {
  const [mutations, setMutations] = useState<any[]>([])
  const [loading, setLoading] = useState(true)
  const [triggering, setTriggering] = useState(false)

  const loadMutations = () => {
    arenaAPI.getMutations(clawId)
      .then(r => { setMutations(r.data?.mutations || []); setLoading(false) })
      .catch(() => setLoading(false))
  }

  useEffect(() => { loadMutations() }, [clawId])

  const handleTrigger = async () => {
    setTriggering(true)
    try {
      const res = await arenaAPI.triggerMutation(clawId)
      const m = res.data?.mutation
      if (m) {
        alert(`🧬 获得变异: ${m.name} (${m.rarity})\n${m.desc}`)
        loadMutations()
      } else {
        alert(res.data?.message || '没有可用的变异')
      }
    } catch (e: any) {
      alert(e.response?.data?.error || '触发失败')
    }
    setTriggering(false)
  }

  if (loading) return <div className="text-center py-12 text-gray-400">加载变异...</div>

  const rarityStyle: Record<string, string> = {
    common: 'border-gray-600 text-gray-300',
    rare: 'border-blue-600 text-blue-400',
    legendary: 'border-amber-500 text-amber-400',
  }

  return (
    <div className="space-y-4">
      <div className="bg-gray-900/80 rounded-xl p-4 border border-gray-700/50 text-center">
        <p className="text-sm text-gray-300 mb-3">宠物进化时有概率触发变异，获得永久属性加成</p>
        <button onClick={handleTrigger} disabled={triggering}
          className="bg-purple-600/80 hover:bg-purple-500 text-white px-6 py-2 rounded-lg font-medium disabled:opacity-50">
          {triggering ? '进化中...' : '🧬 尝试触发变异'}
        </button>
        <p className="text-[10px] text-gray-500 mt-2">60% 普通 · 30% 稀有 · 10% 传说</p>
      </div>

      {mutations.length === 0 ? (
        <div className="text-center py-8 text-gray-500 text-sm">暂无变异记录</div>
      ) : (
        <div className="space-y-2">
          {mutations.map((m: any) => (
            <div key={m.id} className={`bg-gray-900/80 rounded-xl p-4 border ${rarityStyle[m.rarity] || 'border-gray-600'}`}>
              <div className="flex items-center justify-between">
                <div>
                  <span className={`font-medium ${rarityStyle[m.rarity]?.split(' ')[1] || 'text-gray-300'}`}>{m.name}</span>
                  <span className="text-[10px] text-gray-500 ml-2">{m.rarity}</span>
                </div>
                <div className="text-[10px] text-gray-500">
                  {m.bonus_hp > 0 && `HP+${m.bonus_hp} `}
                  {m.bonus_atk > 0 && `ATK+${m.bonus_atk} `}
                  {m.bonus_def > 0 && `DEF+${m.bonus_def} `}
                  {m.bonus_spd > 0 && `SPD+${m.bonus_spd}`}
                </div>
              </div>
              <div className="text-xs text-gray-400 mt-1">{m.desc}</div>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
