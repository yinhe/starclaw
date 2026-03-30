import { useState } from 'react'
import { growthAPI } from '../lib/api'

interface RealmOption {
  key: string
  emoji: string
  name: string
  nameEN: string
  aura: string
  desc: string
  stats: string
  tiers: string[]
  skills: string[]
}

const realms: RealmOption[] = [
  {
    key: 'immortal', emoji: '✨', name: '仙道', nameEN: 'Immortal Path',
    aura: '金色光环', desc: '以德服人，守护虫群。治愈与防御的极致。',
    stats: 'DEF +15%, HP +10%',
    tiers: ['仙徒', '仙人', '神'],
    skills: ['治愈之光', '天罡护盾', '群体祝福', '不死金身', '万物复苏'],
  },
  {
    key: 'demon', emoji: '🔥', name: '魔道', nameEN: 'Demon Path',
    aura: '暗红光环', desc: '以力证道，吞噬一切。攻击与吸收的极致。',
    stats: 'ATK +15%, SPD +10%',
    tiers: ['魔徒', '魔将', '魔神'],
    skills: ['嗜血之爪', '恐惧光环', '灵魂吞噬', '毁灭之息', '深渊吞噬'],
  },
  {
    key: 'monster', emoji: '🌿', name: '妖道', nameEN: 'Monster Path',
    aura: '翠绿光环', desc: '万物化形，诡变莫测。速度与幻术的极致。',
    stats: 'SPD +15%, ATK +10%',
    tiers: ['妖修', '妖王', '妖皇'],
    skills: ['幻影步', '分身术', '化形', '万化归一', '天地同寿'],
  },
]

export default function RealmChoiceModal({ onClose, onChoose }: { onClose: () => void; onChoose: (realm: string) => void }) {
  const [selected, setSelected] = useState<string | null>(null)
  const [confirming, setConfirming] = useState(false)

  const handleConfirm = async () => {
    if (!selected) return
    setConfirming(true)
    try {
      await growthAPI.chooseRealm(selected)
      onChoose(selected)
      onClose()
    } catch (err: any) {
      alert(err.response?.data?.error || 'Failed')
    } finally {
      setConfirming(false)
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm">
      <div className="bg-white dark:bg-gray-900 rounded-2xl w-full max-w-2xl max-h-[90vh] overflow-y-auto p-6 mx-4 shadow-2xl">
        <div className="text-center mb-6">
          <div className="text-4xl mb-2">⚡</div>
          <h2 className="text-xl font-bold text-gray-900 dark:text-white">选择境界道路</h2>
          <p className="text-sm text-gray-500 dark:text-gray-400 mt-1">觉醒 2 星！选择你的灵魂修行方向。</p>
          <p className="text-xs text-amber-500 mt-1">⚠️ 选择后不可更改（除非转生）· 觉醒 5 星后三道合一为「圣」</p>
        </div>

        <div className="grid grid-cols-1 sm:grid-cols-3 gap-4 mb-6">
          {realms.map(r => (
            <button
              key={r.key}
              onClick={() => setSelected(r.key)}
              className={`text-left rounded-xl p-5 border-2 transition-all ${
                selected === r.key
                  ? 'border-primary-500 bg-primary-50 dark:bg-primary-900/20 ring-2 ring-primary-300 shadow-lg scale-[1.02]'
                  : 'border-gray-200 dark:border-gray-700 hover:border-primary-300 hover:shadow-md'
              }`}
            >
              <div className="text-3xl mb-2 text-center">{r.emoji}</div>
              <div className="font-bold text-base text-center text-gray-900 dark:text-white">{r.name}</div>
              <div className="text-[10px] text-gray-400 text-center mb-2">{r.nameEN} · {r.aura}</div>
              <div className="text-xs text-gray-600 dark:text-gray-300 mb-2">{r.desc}</div>
              <div className="text-[10px] text-primary-500 mb-2">属性: {r.stats}</div>
              <div className="text-[10px] text-gray-400 mb-1">进阶: {r.tiers.join(' → ')}</div>
              <div className="flex flex-wrap gap-1 mt-1">
                {r.skills.map((s, i) => (
                  <span key={i} className="text-[9px] bg-gray-100 dark:bg-gray-800 text-gray-600 dark:text-gray-400 px-1.5 py-0.5 rounded">{s}</span>
                ))}
              </div>
            </button>
          ))}
        </div>

        <div className="flex justify-center gap-3">
          <button onClick={onClose} className="px-6 py-2.5 rounded-xl border border-gray-300 dark:border-gray-600 text-sm text-gray-600 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-800 transition">
            稍后选择
          </button>
          <button
            onClick={handleConfirm}
            disabled={!selected || confirming}
            className="px-8 py-2.5 rounded-xl bg-primary-600 text-white text-sm font-semibold hover:bg-primary-500 disabled:opacity-50 disabled:cursor-not-allowed transition shadow-lg shadow-primary-600/20"
          >
            {confirming ? '确认中...' : selected ? `踏入${realms.find(r => r.key === selected)?.name}` : '请先选择道路'}
          </button>
        </div>
      </div>
    </div>
  )
}
