import { useState, useEffect } from 'react'
import { GitBranch, Download, Search, Tag } from 'lucide-react'
import { workflowTemplateAPI } from '../lib/api'

interface Template {
  id: string
  name: string
  description: string
  category: string
  clone_count: number
  created_at: string
}

const categories = [
  { value: '', label: '全部' },
  { value: 'automation', label: '自动化' },
  { value: 'data', label: '数据处理' },
  { value: 'content', label: '内容生成' },
  { value: 'analysis', label: '分析' },
]

export default function WorkflowTemplatePage() {
  const [templates, setTemplates] = useState<Template[]>([])
  const [category, setCategory] = useState('')
  const [search, setSearch] = useState('')
  const [cloning, setCloning] = useState<string | null>(null)

  useEffect(() => { loadTemplates() }, [category])

  const loadTemplates = async () => {
    try {
      const res = await workflowTemplateAPI.list(category || undefined)
      setTemplates(res.data.templates || [])
    } catch { /* ignore */ }
  }

  const handleClone = async (id: string) => {
    setCloning(id)
    try {
      await workflowTemplateAPI.clone(id)
      loadTemplates()
    } catch { /* ignore */ }
    setCloning(null)
  }

  const filtered = search
    ? templates.filter((t) => t.name.toLowerCase().includes(search.toLowerCase()) || t.description?.toLowerCase().includes(search.toLowerCase()))
    : templates

  return (
    <div className="h-full overflow-y-auto">
      <div className="max-w-5xl mx-auto p-8">
        <div className="mb-8">
          <h1 className="text-2xl font-bold text-gray-900 dark:text-white">工作流模板市场</h1>
          <p className="text-gray-500 text-sm mt-1">浏览和克隆社区工作流模板</p>
        </div>

        <div className="flex flex-col md:flex-row gap-4 mb-6">
          <div className="relative flex-1">
            <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-gray-400" />
            <input
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              placeholder="搜索模板..."
              className="w-full pl-10 pr-4 py-2 border rounded-lg text-sm outline-none focus:ring-2 focus:ring-primary-500 bg-white dark:bg-gray-800 dark:text-gray-200 dark:border-gray-700"
            />
          </div>
          <div className="flex gap-2">
            {categories.map((cat) => (
              <button
                key={cat.value}
                onClick={() => setCategory(cat.value)}
                className={`px-3 py-1.5 rounded-lg text-sm transition-colors ${
                  category === cat.value
                    ? 'bg-primary-600 text-white'
                    : 'bg-gray-100 dark:bg-gray-800 text-gray-600 dark:text-gray-300 hover:bg-gray-200 dark:hover:bg-gray-700'
                }`}
              >
                {cat.label}
              </button>
            ))}
          </div>
        </div>

        {filtered.length === 0 ? (
          <div className="text-center py-20 text-gray-400">
            <GitBranch className="w-12 h-12 mx-auto mb-3 opacity-50" />
            <p>暂无模板</p>
            <p className="text-sm mt-1">可在「工作流」页面发布你的工作流到市场</p>
          </div>
        ) : (
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
            {filtered.map((tmpl) => (
              <div key={tmpl.id} className="bg-white dark:bg-gray-800 border dark:border-gray-700 rounded-xl p-5 hover:shadow-md transition-shadow">
                <div className="flex items-start justify-between mb-3">
                  <div className="w-10 h-10 bg-purple-100 dark:bg-purple-900/30 rounded-lg flex items-center justify-center">
                    <GitBranch className="w-5 h-5 text-purple-600 dark:text-purple-400" />
                  </div>
                  <button
                    onClick={() => handleClone(tmpl.id)}
                    disabled={cloning === tmpl.id}
                    className="flex items-center gap-1.5 px-3 py-1.5 bg-primary-600 text-white rounded-lg text-xs hover:bg-primary-700 disabled:opacity-50"
                  >
                    <Download className="w-3 h-3" />
                    {cloning === tmpl.id ? '克隆中...' : '克隆'}
                  </button>
                </div>
                <h3 className="font-semibold text-gray-900 dark:text-white">{tmpl.name}</h3>
                <p className="text-sm text-gray-500 mt-1 line-clamp-2">{tmpl.description || '暂无描述'}</p>
                <div className="flex items-center gap-3 mt-3 text-xs text-gray-400">
                  {tmpl.category && (
                    <span className="flex items-center gap-1">
                      <Tag className="w-3 h-3" />
                      {categories.find((c) => c.value === tmpl.category)?.label || tmpl.category}
                    </span>
                  )}
                  <span>{tmpl.clone_count} 次克隆</span>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  )
}
