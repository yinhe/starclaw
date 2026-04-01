import { useEffect, useState } from 'react'
import { FileText, Eye, X } from 'lucide-react'
import { getScripts, getBuiltinScript } from '../api'

export default function ScriptPage() {
  const [scripts, setScripts] = useState<any>({ scripts: [], builtin: [] })
  const [preview, setPreview] = useState<any>(null)

  useEffect(() => {
    getScripts().then(setScripts).catch(() => {})
  }, [])

  const showPreview = async (industry: string) => {
    const data = await getBuiltinScript(industry)
    setPreview(data)
  }

  return (
    <div className="p-6 space-y-6">
      <h1 className="text-2xl font-bold">话术管理</h1>

      {/* Builtin Scripts */}
      <div>
        <h2 className="text-sm font-medium text-stone-400 mb-3">内置话术模板</h2>
        <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
          {(scripts.builtin || []).map((s: any) => (
            <div key={s.key} className="bg-stone-900 border border-stone-800 rounded-xl p-4 hover:border-stone-700 transition">
              <div className="flex items-start justify-between mb-2">
                <FileText className="w-5 h-5 text-cicada-400" />
                {s.is_builtin && <span className="text-[10px] px-1.5 py-0.5 rounded bg-cicada-500/20 text-cicada-400">内置</span>}
              </div>
              <div className="font-medium text-sm mb-1">{s.name}</div>
              <div className="text-xs text-stone-500 mb-3">{s.industry}</div>
              <button onClick={() => showPreview(s.key)}
                className="flex items-center gap-1 text-xs text-cicada-400 hover:text-cicada-300">
                <Eye className="w-3 h-3" /> 预览
              </button>
            </div>
          ))}
        </div>
      </div>

      {/* Custom Scripts from DB */}
      {(scripts.scripts || []).length > 0 && (
        <div>
          <h2 className="text-sm font-medium text-stone-400 mb-3">自定义话术</h2>
          <div className="bg-stone-900 border border-stone-800 rounded-xl overflow-hidden">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-stone-800 text-stone-500 text-xs">
                  <th className="text-left px-4 py-3 font-medium">名称</th>
                  <th className="text-left px-4 py-3 font-medium">行业</th>
                  <th className="text-left px-4 py-3 font-medium">语音</th>
                  <th className="text-left px-4 py-3 font-medium">创建时间</th>
                </tr>
              </thead>
              <tbody>
                {(scripts.scripts as any[]).map((s: any) => (
                  <tr key={s.id} className="border-b border-stone-800/50 hover:bg-stone-800/30">
                    <td className="px-4 py-3 font-medium">{s.name}</td>
                    <td className="px-4 py-3 text-stone-400">{s.industry}</td>
                    <td className="px-4 py-3 text-stone-400">{s.voice}</td>
                    <td className="px-4 py-3 text-stone-500 text-xs">{s.created_at ? new Date(s.created_at).toLocaleString('zh-CN') : '-'}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {/* Preview Modal */}
      {preview && (
        <div className="fixed inset-0 bg-black/60 flex items-center justify-center z-50" onClick={() => setPreview(null)}>
          <div className="bg-stone-900 border border-stone-700 rounded-xl w-full max-w-2xl max-h-[80vh] overflow-auto m-4" onClick={e => e.stopPropagation()}>
            <div className="flex items-center justify-between px-5 py-4 border-b border-stone-800">
              <h3 className="font-medium">{preview.name}</h3>
              <button onClick={() => setPreview(null)} className="p-1 rounded hover:bg-stone-800"><X className="w-4 h-4" /></button>
            </div>
            <div className="p-5 space-y-4 text-sm">
              <div>
                <div className="text-xs text-stone-500 mb-1">开场白</div>
                <div className="bg-stone-950 rounded-lg p-3 text-stone-300">{preview.greeting}</div>
              </div>
              {preview.key_points && (
                <div>
                  <div className="text-xs text-stone-500 mb-1">关键卖点</div>
                  <ul className="space-y-1">
                    {(preview.key_points as string[]).map((p: string, i: number) => (
                      <li key={i} className="text-stone-400 flex gap-2"><span className="text-cicada-400">•</span>{p}</li>
                    ))}
                  </ul>
                </div>
              )}
              {preview.qa_library && (
                <div>
                  <div className="text-xs text-stone-500 mb-1">常见问答</div>
                  <div className="space-y-2">
                    {(preview.qa_library as any[]).map((qa: any, i: number) => (
                      <div key={i} className="bg-stone-950 rounded-lg p-3">
                        <div className="text-stone-300 font-medium">问: {qa.q}</div>
                        <div className="text-stone-400 mt-1">答: {qa.a}</div>
                      </div>
                    ))}
                  </div>
                </div>
              )}
              {preview.objections && (
                <div>
                  <div className="text-xs text-stone-500 mb-1">异议处理</div>
                  <div className="space-y-1">
                    {(preview.objections as any[]).map((o: any, i: number) => (
                      <div key={i} className="text-stone-400">
                        <span className="text-red-400">"{o.trigger}"</span> → {o.response}
                      </div>
                    ))}
                  </div>
                </div>
              )}
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
