import { useEffect, useState } from 'react'
import { FileBox, Download, FileText, Presentation, Video, FileSpreadsheet, Copy, Check } from 'lucide-react'
import { city, type Material } from '../lib/api'

const CATEGORIES = [
  { key: '', label: '全部' },
  { key: 'brochure', label: '产品手册' },
  { key: 'deck', label: '演示文稿' },
  { key: 'case_study', label: '案例研究' },
  { key: 'video', label: '视频素材' },
  { key: 'quote_template', label: '报价模板' },
]

const CATEGORY_ICONS: Record<string, React.ReactNode> = {
  brochure: <FileText size={20} />,
  deck: <Presentation size={20} />,
  case_study: <FileSpreadsheet size={20} />,
  video: <Video size={20} />,
  quote_template: <FileBox size={20} />,
}

export default function MaterialsPage() {
  const [materials, setMaterials] = useState<Material[]>([])
  const [category, setCategory] = useState('')
  const [refUrl, setRefUrl] = useState('')
  const [copied, setCopied] = useState(false)

  useEffect(() => {
    city.listMaterials(category || undefined).then(r => setMaterials(r.materials || [])).catch(console.error)
  }, [category])

  useEffect(() => {
    city.refLink().then(r => setRefUrl(r.utm_url)).catch(console.error)
  }, [])

  const copyUtm = () => {
    navigator.clipboard.writeText(refUrl)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  const fmtSize = (bytes: number) => {
    if (bytes < 1024) return `${bytes} B`
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(0)} KB`
    return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
  }

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold text-white">营销工具</h1>
        <p className="text-sm text-gray-400 mt-1">获取推广链接和销售物料</p>
      </div>

      {/* Referral link */}
      <div className="rounded-xl border border-claw-500/20 bg-claw-500/5 p-5">
        <h3 className="text-sm font-medium text-white mb-2">推广链接（含 UTM 归因）</h3>
        <div className="flex items-center gap-3">
          <code className="flex-1 bg-gray-900 rounded-lg px-4 py-2.5 text-sm text-claw-400 overflow-x-auto">
            {refUrl || '加载中...'}
          </code>
          <button
            onClick={copyUtm}
            disabled={!refUrl}
            className="shrink-0 rounded-lg border border-white/10 px-3 py-2.5 text-sm text-gray-400 hover:text-white hover:bg-white/5 transition-colors disabled:opacity-30"
          >
            {copied ? <Check size={16} className="text-green-400" /> : <Copy size={16} />}
          </button>
        </div>
        <p className="text-xs text-gray-500 mt-2">
          通过此链接注册的客户将自动归因到你的账户，佣金自动计算。
        </p>
      </div>

      {/* Category filter */}
      <div className="flex gap-2 flex-wrap">
        {CATEGORIES.map(c => (
          <button
            key={c.key}
            onClick={() => setCategory(c.key)}
            className={`px-3 py-1.5 rounded-lg text-xs transition-colors ${
              category === c.key ? 'bg-claw-500/10 text-claw-400' : 'text-gray-400 hover:text-white hover:bg-white/5'
            }`}
          >
            {c.label}
          </button>
        ))}
      </div>

      {/* Materials grid */}
      {materials.length === 0 ? (
        <div className="rounded-xl border border-white/10 border-dashed p-12 text-center">
          <FileBox className="w-10 h-10 text-gray-600 mx-auto mb-3" />
          <p className="text-gray-500">暂无物料</p>
          <p className="text-xs text-gray-600 mt-1">管理员正在准备中，请稍候</p>
        </div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {materials.map(m => (
            <div key={m.id} className="rounded-xl border border-white/10 bg-white/[0.02] p-5 hover:border-claw-500/30 transition-colors">
              <div className="flex items-start gap-3">
                <div className="w-10 h-10 rounded-lg bg-claw-500/10 text-claw-400 flex items-center justify-center shrink-0">
                  {CATEGORY_ICONS[m.category] || <FileBox size={20} />}
                </div>
                <div className="min-w-0 flex-1">
                  <h3 className="text-sm font-medium text-white truncate">{m.title}</h3>
                  <p className="text-xs text-gray-500 mt-0.5">
                    {CATEGORIES.find(c => c.key === m.category)?.label || m.category}
                    {m.file_size > 0 && ` · ${fmtSize(m.file_size)}`}
                  </p>
                </div>
              </div>
              {m.description && (
                <p className="text-xs text-gray-400 mt-3 line-clamp-2">{m.description}</p>
              )}
              <a
                href={m.file_url}
                target="_blank"
                rel="noopener noreferrer"
                className="inline-flex items-center gap-1.5 mt-4 text-xs text-claw-400 hover:text-claw-300 transition-colors"
              >
                <Download size={12} />
                下载
              </a>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
