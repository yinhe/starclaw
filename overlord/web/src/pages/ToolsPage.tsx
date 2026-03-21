import { useState } from 'react'
import { Search, FileText, Code, Image, Globe, Database, Mail, Calculator, FileSpreadsheet, ExternalLink } from 'lucide-react'

interface Tool {
  id: string
  name: string
  icon: React.ReactNode
  category: string
  description: string
  status: 'available' | 'coming_soon'
}

const tools: Tool[] = [
  { id: 'doc-gen', name: '文档生成', icon: <FileText className="w-5 h-5" />, category: '文档', description: '从模板或描述自动生成各类文档', status: 'available' },
  { id: 'code-gen', name: '代码生成', icon: <Code className="w-5 h-5" />, category: '开发', description: '根据自然语言描述生成代码片段', status: 'available' },
  { id: 'img-analyze', name: '图片分析', icon: <Image className="w-5 h-5" />, category: '多模态', description: '上传图片进行 OCR、物体识别和描述', status: 'available' },
  { id: 'web-search', name: '联网搜索', icon: <Globe className="w-5 h-5" />, category: '信息', description: '实时搜索互联网获取最新信息', status: 'available' },
  { id: 'db-query', name: '数据库查询', icon: <Database className="w-5 h-5" />, category: '开发', description: '连接数据库执行自然语言查询', status: 'coming_soon' },
  { id: 'email-send', name: '邮件发送', icon: <Mail className="w-5 h-5" />, category: '效率', description: '撰写并直接发送邮件', status: 'coming_soon' },
  { id: 'calculator', name: '高级计算', icon: <Calculator className="w-5 h-5" />, category: '工具', description: '数学公式计算与符号运算', status: 'available' },
  { id: 'spreadsheet', name: '表格处理', icon: <FileSpreadsheet className="w-5 h-5" />, category: '数据', description: '读取和处理 CSV/Excel 文件', status: 'available' },
]

export default function ToolsPage() {
  const [search, setSearch] = useState('')

  const filtered = tools.filter(t => {
    if (search && !t.name.includes(search) && !t.description.includes(search)) return false
    return true
  })

  const available = filtered.filter(t => t.status === 'available')
  const comingSoon = filtered.filter(t => t.status === 'coming_soon')

  return (
    <div className="p-4 md:p-6">
      <div className="mb-4 md:mb-6">
        <h1 className="text-lg md:text-xl font-bold text-white">工具集合</h1>
        <p className="text-xs md:text-sm text-gray-500 mt-1">AI 智能体可调用的工具和能力扩展</p>
      </div>

      <div className="relative md:max-w-md mb-4 md:mb-6">
        <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-gray-500" />
        <input
          type="text"
          value={search}
          onChange={e => setSearch(e.target.value)}
          placeholder="搜索工具..."
          className="w-full pl-10 pr-4 py-2.5 bg-gray-800 border border-gray-700 rounded-lg text-sm text-white placeholder-gray-500 focus:outline-none focus:border-brand-500 transition"
        />
      </div>

      {/* Available */}
      {available.length > 0 && (
        <>
          <h2 className="text-sm font-semibold text-gray-400 mb-3">可用工具 ({available.length})</h2>
          <div className="grid grid-cols-2 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-3 mb-8">
            {available.map(tool => (
              <div
                key={tool.id}
                className="bg-gray-900 border border-gray-800 rounded-xl p-4 hover:border-brand-600/50 transition cursor-pointer group"
              >
                <div className="flex items-center gap-3 mb-2">
                  <div className="w-10 h-10 rounded-lg bg-brand-600/15 flex items-center justify-center text-brand-400">
                    {tool.icon}
                  </div>
                  <div>
                    <h3 className="text-sm font-medium text-white">{tool.name}</h3>
                    <span className="text-[10px] text-gray-500">{tool.category}</span>
                  </div>
                </div>
                <p className="text-xs text-gray-500">{tool.description}</p>
                <div className="mt-3 flex justify-end opacity-0 group-hover:opacity-100 transition">
                  <span className="text-[10px] text-brand-400 flex items-center gap-1">
                    <ExternalLink className="w-3 h-3" /> 在对话中使用
                  </span>
                </div>
              </div>
            ))}
          </div>
        </>
      )}

      {/* Coming soon */}
      {comingSoon.length > 0 && (
        <>
          <h2 className="text-sm font-semibold text-gray-400 mb-3">即将推出 ({comingSoon.length})</h2>
          <div className="grid grid-cols-2 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-3">
            {comingSoon.map(tool => (
              <div
                key={tool.id}
                className="bg-gray-900/50 border border-gray-800/50 rounded-xl p-4 opacity-60"
              >
                <div className="flex items-center gap-3 mb-2">
                  <div className="w-10 h-10 rounded-lg bg-gray-800 flex items-center justify-center text-gray-500">
                    {tool.icon}
                  </div>
                  <div>
                    <h3 className="text-sm font-medium text-gray-400">{tool.name}</h3>
                    <span className="text-[10px] text-gray-600">{tool.category}</span>
                  </div>
                </div>
                <p className="text-xs text-gray-600">{tool.description}</p>
                <div className="mt-3 text-right">
                  <span className="text-[10px] text-gray-600 bg-gray-800/50 px-2 py-0.5 rounded-full">敬请期待</span>
                </div>
              </div>
            ))}
          </div>
        </>
      )}
    </div>
  )
}
