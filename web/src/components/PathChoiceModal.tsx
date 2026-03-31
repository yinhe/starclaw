import { useState } from 'react'
import { growthAPI } from '../lib/api'

interface PathOption {
  key: string
  emoji: string
  name: string
  nameEN: string
  desc: string
  stats: string
  creatures: string
  ultimate: string
}

const paths: PathOption[] = [
  { key: 'ocean', emoji: '🌊', name: '水域之路', nameEN: 'Ocean Path', desc: '海洋霸主，深如渊海', stats: 'HP + DEF', creatures: '帝王蟹 → 章鱼 → 大白鲨 → 海豚 → 虎鲸 → 蓝鲸', ultimate: '🔱 利维坦' },
  { key: 'terrain', emoji: '🏔️', name: '大地之路', nameEN: 'Terrain Path', desc: '陆地之王，执行力强', stats: 'ATK + SPD', creatures: '蝎子 → 科莫多龙 → 灰狼 → 灰熊 → 狮子 → 非洲象', ultimate: '🦖 霸王龙' },
  { key: 'sky', emoji: '🌪️', name: '天空之路', nameEN: 'Sky Path', desc: '空中统治，灵感如飞', stats: 'SPD + ATK', creatures: '蜻蜓 → 猫头鹰 → 猎隼 → 金雕 → 翼龙', ultimate: '🔥 凤凰' },
  { key: 'wisdom', emoji: '🧬', name: '智慧之路', nameEN: 'Wisdom Path', desc: '进化巅峰，全面均衡', stats: '均衡 + 技能多', creatures: '乌鸦 → 章鱼 → 海豚 → 大猩猩 → 黑猩猩 → 智人', ultimate: '🤖 超智体' },
  { key: 'ancient', emoji: '🔥', name: '远古之路', nameEN: 'Ancient Path', desc: '史前巨兽，暴力输出', stats: 'HP + ATK', creatures: '三叶虫 → 邓氏鱼 → 迅猛龙 → 棘龙 → 霸王龙', ultimate: '☄️ 哥斯拉' },
  { key: 'symbiont', emoji: '🌿', name: '共生之路', nameEN: 'Symbiont Path', desc: '生态守护，治愈之力', stats: 'DEF + 治愈', creatures: '蜜蜂 → 珊瑚 → 红杉 → 灯塔水母 → 菌丝网络', ultimate: '🌏 盖亚' },
]

export default function PathChoiceModal({ onClose, onChoose }: { onClose: () => void; onChoose: (path: string) => void }) {
  const [selected, setSelected] = useState<string | null>(null)
  const [confirming, setConfirming] = useState(false)

  const handleConfirm = async () => {
    if (!selected) return
    setConfirming(true)
    try {
      await growthAPI.choosePath(selected)
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
      <div className="bg-white dark:bg-gray-900 rounded-2xl w-full max-w-3xl max-h-[90vh] overflow-y-auto p-6 mx-4 shadow-2xl">
        <div className="text-center mb-6">
          <div className="text-4xl mb-2">🦞</div>
          <h2 className="text-xl font-bold text-gray-900 dark:text-white">选择进化路线</h2>
          <p className="text-sm text-gray-500 dark:text-gray-400 mt-1">你的小龙虾已达到 Lv.5！选择一条进化路线，将决定未来的形态和能力方向。</p>
          <p className="text-xs text-amber-500 mt-1">⚠️ 选择后不可更改（除非转生）</p>
        </div>

        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3 mb-6">
          {paths.map(p => (
            <button
              key={p.key}
              onClick={() => setSelected(p.key)}
              className={`text-left rounded-xl p-4 border-2 transition-all ${
                selected === p.key
                  ? 'border-primary-500 bg-primary-50 dark:bg-primary-900/20 ring-2 ring-primary-300 shadow-lg'
                  : 'border-gray-200 dark:border-gray-700 hover:border-primary-300 hover:shadow-md'
              }`}
            >
              <div className="text-2xl mb-1">{p.emoji}</div>
              <div className="font-bold text-sm text-gray-900 dark:text-white">{p.name}</div>
              <div className="text-[10px] text-gray-400">{p.nameEN}</div>
              <div className="text-xs text-gray-600 dark:text-gray-300 mt-1">{p.desc}</div>
              <div className="text-[10px] text-primary-500 mt-1">属性: {p.stats}</div>
              <div className="text-[10px] text-gray-400 mt-1">{p.creatures}</div>
              <div className="text-xs font-semibold mt-2">终极形态: {p.ultimate}</div>
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
            {confirming ? '确认中...' : selected ? `选择 ${paths.find(p => p.key === selected)?.name}` : '请先选择路线'}
          </button>
        </div>
      </div>
    </div>
  )
}
