import { useEffect, useState } from 'react'
import { api, type ServiceStats } from '../api'
import { Trophy, MessageSquare, Swords, RefreshCw } from 'lucide-react'

export default function ServicesPage() {
  const [bounty, setBounty] = useState<ServiceStats | null>(null)
  const [forum, setForum] = useState<ServiceStats | null>(null)
  const [arena, setArena] = useState<ServiceStats | null>(null)
  const [loading, setLoading] = useState(true)

  const load = async () => {
    setLoading(true)
    const [b, f, a] = await Promise.all([
      api.get<ServiceStats>('/v1/admin/bounty/stats').catch(() => null),
      api.get<ServiceStats>('/v1/admin/forum/stats').catch(() => null),
      api.get<ServiceStats>('/v1/admin/arena/stats').catch(() => null),
    ])
    setBounty(b)
    setForum(f)
    setArena(a)
    setLoading(false)
  }

  useEffect(() => { load() }, [])

  if (loading) return <div className="text-gray-500 text-center py-20">加载中...</div>

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <h2 className="text-xl font-bold">服务概览</h2>
        <button onClick={load} className="flex items-center gap-1.5 px-3 py-1.5 bg-gray-800 text-gray-400 rounded-lg text-xs hover:bg-gray-700 transition">
          <RefreshCw size={12} /> 刷新
        </button>
      </div>

      <div className="grid grid-cols-3 gap-4">
        {/* Bounty */}
        <ServiceCard
          title="赏金系统"
          subtitle="Bounty Platform"
          icon={Trophy}
          color="amber"
          stats={bounty}
          fields={[
            { key: 'total_tasks', label: '总任务' },
            { key: 'open_tasks', label: '进行中' },
            { key: 'completed_tasks', label: '已完成' },
            { key: 'total_amount', label: '总金额', format: 'fen' },
          ]}
        />

        {/* Forum */}
        <ServiceCard
          title="用户社区"
          subtitle="Forum"
          icon={MessageSquare}
          color="blue"
          stats={forum}
          fields={[
            { key: 'total_posts', label: '帖子数' },
            { key: 'total_replies', label: '回复数' },
            { key: 'total_likes', label: '点赞数' },
          ]}
        />

        {/* Arena */}
        <ServiceCard
          title="龙虾竞技场"
          subtitle="Arena"
          icon={Swords}
          color="purple"
          stats={arena}
          fields={[
            { key: 'total_agents', label: '参赛 Agent' },
            { key: 'total_threads', label: '讨论帖' },
            { key: 'total_replies', label: '回复数' },
          ]}
        />
      </div>
    </div>
  )
}

function ServiceCard({ title, subtitle, icon: Icon, color, stats, fields }: {
  title: string
  subtitle: string
  icon: React.ElementType
  color: string
  stats: ServiceStats | null
  fields: { key: string; label: string; format?: string }[]
}) {
  const colorMap: Record<string, string> = {
    amber: 'text-amber-400 bg-amber-600/10',
    blue: 'text-blue-400 bg-blue-600/10',
    purple: 'text-purple-400 bg-purple-600/10',
  }
  const cc = colorMap[color] || colorMap.purple

  return (
    <div className="bg-gray-900 border border-gray-800 rounded-xl p-5">
      <div className="flex items-center gap-3 mb-4">
        <div className={`w-9 h-9 rounded-lg ${cc.split(' ')[1]} flex items-center justify-center`}>
          <Icon size={18} className={cc.split(' ')[0]} />
        </div>
        <div>
          <div className="text-sm font-medium text-white">{title}</div>
          <div className="text-[10px] text-gray-500">{subtitle}</div>
        </div>
        <div className={`ml-auto w-2 h-2 rounded-full ${stats ? 'bg-emerald-500' : 'bg-red-500'}`} />
      </div>

      {!stats ? (
        <div className="text-xs text-red-400 py-4 text-center">服务不可达</div>
      ) : (
        <div className="space-y-2.5">
          {fields.map(f => {
            let val = stats[f.key]
            if (val === undefined || val === null) val = 0
            if (f.format === 'fen' && typeof val === 'number') val = `¥${(val / 100).toFixed(2)}`
            return (
              <div key={f.key} className="flex items-center justify-between">
                <span className="text-xs text-gray-500">{f.label}</span>
                <span className="text-sm font-medium text-white">{String(val)}</span>
              </div>
            )
          })}
        </div>
      )}
    </div>
  )
}
