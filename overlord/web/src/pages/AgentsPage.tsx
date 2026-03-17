import { useState } from 'react'
import { Search, Star, Play, Clock, Zap } from 'lucide-react'

interface Agent {
  id: string
  name: string
  icon: string
  category: string
  description: string
  author: string
  uses: number
  rating: number
  tags: string[]
}

const mockAgents: Agent[] = [
  {
    id: 'general',
    name: '通用助手',
    icon: '🤖',
    category: '通用',
    description: '通用 AI 对话助手，支持多轮对话、知识问答、翻译、总结等多种能力。',
    author: 'StarClaw',
    uses: 12580,
    rating: 4.8,
    tags: ['对话', '问答', '翻译'],
  },
  {
    id: 'code-review',
    name: '代码审查',
    icon: '🔍',
    category: '开发',
    description: '自动审查代码质量，发现潜在 Bug、性能问题和安全漏洞，提供改进建议。',
    author: 'StarClaw',
    uses: 8340,
    rating: 4.9,
    tags: ['代码', '审查', '质量'],
  },
  {
    id: 'doc-writer',
    name: '文档撰写',
    icon: '📝',
    category: '写作',
    description: '根据需求自动生成技术文档、产品文档、API 文档等，支持 Markdown 输出。',
    author: 'StarClaw',
    uses: 6720,
    rating: 4.7,
    tags: ['文档', '写作', 'Markdown'],
  },
  {
    id: 'data-analyst',
    name: '数据分析师',
    icon: '📊',
    category: '数据',
    description: '分析 CSV/Excel 数据，生成统计报告、趋势分析和可视化建议。',
    author: 'StarClaw',
    uses: 5100,
    rating: 4.6,
    tags: ['数据', '分析', '报表'],
  },
  {
    id: 'sql-expert',
    name: 'SQL 专家',
    icon: '🗄️',
    category: '开发',
    description: '根据自然语言生成 SQL 查询，优化慢查询，解释执行计划。',
    author: 'StarClaw',
    uses: 4200,
    rating: 4.8,
    tags: ['SQL', '数据库', '查询'],
  },
  {
    id: 'email-writer',
    name: '邮件助手',
    icon: '✉️',
    category: '写作',
    description: '自动撰写商务邮件，支持多种语气和场景，中英双语。',
    author: 'StarClaw',
    uses: 3800,
    rating: 4.5,
    tags: ['邮件', '商务', '沟通'],
  },
  {
    id: 'meeting-notes',
    name: '会议纪要',
    icon: '🎙️',
    category: '效率',
    description: '整理会议录音或文字记录，生成结构化会议纪要和行动项。',
    author: 'StarClaw',
    uses: 3200,
    rating: 4.7,
    tags: ['会议', '纪要', '效率'],
  },
  {
    id: 'translator',
    name: '专业翻译',
    icon: '🌐',
    category: '通用',
    description: '支持中英日韩多语言翻译，保持专业术语的准确性。',
    author: 'StarClaw',
    uses: 7600,
    rating: 4.8,
    tags: ['翻译', '多语言', '专业'],
  },
]

const categories = ['全部', '通用', '开发', '写作', '数据', '效率']

export default function AgentsPage() {
  const [search, setSearch] = useState('')
  const [category, setCategory] = useState('全部')

  const filtered = mockAgents.filter(a => {
    if (category !== '全部' && a.category !== category) return false
    if (search && !a.name.includes(search) && !a.description.includes(search)) return false
    return true
  })

  return (
    <div className="p-6">
      <div className="mb-6">
        <h1 className="text-xl font-bold text-white">智能体市场</h1>
        <p className="text-sm text-gray-500 mt-1">浏览和使用预置的 AI 智能体模板</p>
      </div>

      {/* Search + Filter */}
      <div className="flex items-center gap-4 mb-6">
        <div className="relative flex-1 max-w-md">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-gray-500" />
          <input
            type="text"
            value={search}
            onChange={e => setSearch(e.target.value)}
            placeholder="搜索智能体..."
            className="w-full pl-10 pr-4 py-2.5 bg-gray-800 border border-gray-700 rounded-lg text-sm text-white placeholder-gray-500 focus:outline-none focus:border-brand-500 transition"
          />
        </div>
        <div className="flex gap-1">
          {categories.map(c => (
            <button
              key={c}
              onClick={() => setCategory(c)}
              className={`px-3 py-1.5 text-xs rounded-lg transition ${
                category === c
                  ? 'bg-brand-600 text-white'
                  : 'bg-gray-800 text-gray-400 hover:bg-gray-700'
              }`}
            >
              {c}
            </button>
          ))}
        </div>
      </div>

      {/* Grid */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4">
        {filtered.map(agent => (
          <div
            key={agent.id}
            className="bg-gray-900 border border-gray-800 rounded-xl p-5 hover:border-gray-700 transition group"
          >
            <div className="flex items-start justify-between mb-3">
              <span className="text-3xl">{agent.icon}</span>
              <span className="text-[10px] text-gray-500 bg-gray-800 px-2 py-0.5 rounded-full">
                {agent.category}
              </span>
            </div>
            <h3 className="text-sm font-semibold text-white mb-1">{agent.name}</h3>
            <p className="text-xs text-gray-500 mb-3 line-clamp-2">{agent.description}</p>
            <div className="flex flex-wrap gap-1 mb-3">
              {agent.tags.map(tag => (
                <span key={tag} className="text-[10px] text-gray-400 bg-gray-800/80 px-1.5 py-0.5 rounded">
                  {tag}
                </span>
              ))}
            </div>
            <div className="flex items-center justify-between pt-3 border-t border-gray-800">
              <div className="flex items-center gap-3 text-[10px] text-gray-500">
                <span className="flex items-center gap-1">
                  <Star className="w-3 h-3 text-yellow-500" />
                  {agent.rating}
                </span>
                <span className="flex items-center gap-1">
                  <Zap className="w-3 h-3" />
                  {agent.uses.toLocaleString()}
                </span>
              </div>
              <button className="flex items-center gap-1 text-xs text-brand-400 hover:text-brand-300 transition opacity-0 group-hover:opacity-100">
                <Play className="w-3 h-3" />
                使用
              </button>
            </div>
          </div>
        ))}
      </div>

      {filtered.length === 0 && (
        <div className="text-center py-16 text-gray-500 text-sm">
          没有找到匹配的智能体
        </div>
      )}
    </div>
  )
}
