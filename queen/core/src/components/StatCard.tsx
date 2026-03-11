import type { LucideIcon } from 'lucide-react'

interface StatCardProps {
  title: string
  value: string | number
  icon: LucideIcon
  sub?: string
  color?: string
}

export default function StatCard({ title, value, icon: Icon, sub, color = 'purple' }: StatCardProps) {
  const colorMap: Record<string, string> = {
    purple: 'text-purple-400 bg-purple-500/10',
    green: 'text-green-400 bg-green-500/10',
    blue: 'text-blue-400 bg-blue-500/10',
    amber: 'text-amber-400 bg-amber-500/10',
    red: 'text-red-400 bg-red-500/10',
    cyan: 'text-cyan-400 bg-cyan-500/10',
  }

  return (
    <div className="bg-gray-900 border border-gray-800 rounded-xl p-5">
      <div className="flex items-center justify-between mb-3">
        <span className="text-sm text-gray-400">{title}</span>
        <div className={`p-2 rounded-lg ${colorMap[color] || colorMap.purple}`}>
          <Icon size={18} />
        </div>
      </div>
      <div className="text-2xl font-bold text-gray-100">{value}</div>
      {sub && <div className="text-xs text-gray-500 mt-1">{sub}</div>}
    </div>
  )
}
