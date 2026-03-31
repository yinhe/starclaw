import { useState, useEffect } from 'react'
import { swarmAPI } from '../lib/api'

interface SwarmUnit {
  id: string
  agent_name: string
  unit_type: string
  level: number
  hp: number
  atk: number
  def: number
  spd: number
  skill_1: string
}

const typeIcons: Record<string, string> = {
  financial: '🏦', creative: '🎬', social: '💬', engineer: '💻',
  scout: '🔍', scholar: '🧠', generic: '⚔️',
}

export default function LineupSelector({ onConfirm, maxSlots = 3 }: { onConfirm: (ids: string[]) => void; maxSlots?: number }) {
  const [units, setUnits] = useState<SwarmUnit[]>([])
  const [selected, setSelected] = useState<Set<string>>(new Set())
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    swarmAPI.list()
      .then(r => { setUnits(r.data?.units || []); setLoading(false) })
      .catch(() => setLoading(false))
  }, [])

  const toggle = (id: string) => {
    setSelected(prev => {
      const next = new Set(prev)
      if (next.has(id)) {
        next.delete(id)
      } else if (next.size < maxSlots) {
        next.add(id)
      }
      return next
    })
  }

  const totalPower = units.filter(u => selected.has(u.id)).reduce((sum, u) => sum + u.hp + u.atk + u.def + u.spd, 0)

  if (loading) return <div className="text-center py-8 text-gray-400 text-sm">加载虫群...</div>

  if (units.length === 0) {
    return (
      <div className="text-center py-12">
        <div className="text-4xl mb-3">🐛</div>
        <p className="text-gray-500 dark:text-gray-400">虫群为空，无法出战</p>
        <p className="text-xs text-gray-400 dark:text-gray-500 mt-1">创建智能体后自动获得虫群成员</p>
      </div>
    )
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h3 className="text-sm font-semibold text-gray-700 dark:text-gray-200">选择出战虫群 ({selected.size}/{maxSlots})</h3>
        <span className="text-xs text-primary-500">预估战力: {totalPower}</span>
      </div>

      <div className="grid grid-cols-1 sm:grid-cols-2 gap-2">
        {units.map(u => {
          const isSelected = selected.has(u.id)
          const icon = typeIcons[u.unit_type] || '⚔️'
          return (
            <button
              key={u.id}
              onClick={() => toggle(u.id)}
              className={`text-left rounded-xl p-3 border-2 transition-all ${
                isSelected
                  ? 'border-primary-500 bg-primary-50 dark:bg-primary-900/20 shadow-md'
                  : 'border-gray-200 dark:border-gray-700 hover:border-gray-300 dark:hover:border-gray-600'
              }`}
            >
              <div className="flex items-center gap-2">
                <span className="text-xl">{icon}</span>
                <div className="flex-1 min-w-0">
                  <div className="font-semibold text-sm text-gray-900 dark:text-white truncate">{u.agent_name}</div>
                  <div className="text-[10px] text-gray-500">Lv.{u.level}</div>
                </div>
                {isSelected && <span className="text-primary-500 text-lg">✓</span>}
              </div>
              <div className="flex gap-2 mt-1 text-[10px]">
                <span className="text-red-500">❤️{u.hp}</span>
                <span className="text-orange-500">⚔️{u.atk}</span>
                <span className="text-blue-500">🛡️{u.def}</span>
                <span className="text-green-500">💨{u.spd}</span>
              </div>
              {u.skill_1 && <div className="text-[10px] text-primary-500 mt-0.5">🎯 {u.skill_1}</div>}
            </button>
          )
        })}
      </div>

      <button
        onClick={() => onConfirm(Array.from(selected))}
        disabled={selected.size === 0}
        className="w-full py-3 rounded-xl bg-red-600 hover:bg-red-500 text-white font-semibold text-sm disabled:opacity-50 disabled:cursor-not-allowed transition shadow-lg shadow-red-600/20"
      >
        {selected.size === 0 ? '请选择至少 1 只虫出战' : `⚔️ 出战！(${selected.size} 只虫)`}
      </button>
    </div>
  )
}
