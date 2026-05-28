import { useState, useRef } from 'react'
import { X, Upload, FileText, Sparkles, AlertTriangle } from 'lucide-react'
import { parseProjectScriptMarkdown, parseScriptMarkdown, SAMPLE_SCRIPT_MD, type ProjectScriptData } from './scriptParser'
import type { EpisodeData } from './episodeTypes'

interface Props {
  open: boolean
  adType?: { id: string; label: string }   // 选中的广告类型 (e.g. 企业宣传片)
  onClose: () => void
  onImport: (data: EpisodeData) => void
  onImportProject?: (project: ProjectScriptData) => void
}

export default function ScriptImporterModal({ open, adType, onClose, onImport, onImportProject }: Props) {
  const [text, setText] = useState('')
  const [warnings, setWarnings] = useState<string[]>([])
  const [preview, setPreview] = useState<EpisodeData | null>(null)
  const [projectPreview, setProjectPreview] = useState<ProjectScriptData | null>(null)
  const fileRef = useRef<HTMLInputElement>(null)

  if (!open) return null

  const reparse = (raw: string) => {
    setText(raw)
    if (!raw.trim()) {
      setPreview(null)
      setProjectPreview(null)
      setWarnings([])
      return
    }
    try {
      if (!adType) {
        const projectResult = parseProjectScriptMarkdown(raw)
        if (projectResult.project && projectResult.project.episodes.length > 0) {
          setProjectPreview(projectResult.project)
          setPreview(projectResult.project.episodes[0])
          setWarnings(projectResult.warnings)
          return
        }
      }
      const { data, warnings } = parseScriptMarkdown(raw)
      setProjectPreview(null)
      setPreview(data)
      setWarnings(warnings)
    } catch (e) {
      setPreview(null)
      setProjectPreview(null)
      setWarnings([(e as Error).message || '解析失败'])
    }
  }

  const handleFile = (f: File) => {
    const reader = new FileReader()
    reader.onload = () => reparse(String(reader.result || ''))
    reader.readAsText(f, 'utf-8')
  }

  const submit = () => {
    if (projectPreview && onImportProject) {
      onImportProject(projectPreview)
      setText('')
      setProjectPreview(null)
      setPreview(null)
      setWarnings([])
      onClose()
      return
    }
    if (!preview) return
    // 把广告类型注入到导入的剧本里
    const data: EpisodeData = adType
      ? ({ ...preview, ad_type: adType.id, ad_type_label: adType.label } as EpisodeData & { ad_type: string; ad_type_label: string })
      : preview
    onImport(data)
    setText('')
    setProjectPreview(null)
    setPreview(null)
    setWarnings([])
    onClose()
  }

  return (
    <div className="fixed inset-0 z-[100] flex items-center justify-center bg-black/60 backdrop-blur-sm p-4"
         onClick={onClose}>
      <div className="w-full max-w-3xl bg-gray-900 border border-gray-700 rounded-2xl shadow-2xl overflow-hidden flex flex-col max-h-[88vh]"
           onClick={e => e.stopPropagation()}>
        <div className="px-5 py-4 border-b border-gray-800 flex items-center justify-between bg-gradient-to-r from-indigo-900/40 to-gray-900">
          <div className="flex items-center gap-2">
            <FileText className="w-4 h-4 text-indigo-400" />
            <h3 className="text-sm font-semibold text-gray-100">导入剧本</h3>
            {adType && (
              <span className="ml-2 px-2 py-0.5 text-[10px] rounded bg-indigo-900/40 border border-indigo-700/50 text-indigo-200">
                {adType.label}
              </span>
            )}
          </div>
          <button onClick={onClose} className="text-gray-500 hover:text-gray-200 transition-colors">
            <X className="w-4 h-4" />
          </button>
        </div>

        <div className="p-5 grid grid-cols-2 gap-4 overflow-y-auto">
          {/* 左：输入 */}
          <div className="flex flex-col gap-2">
            <div className="flex items-center justify-between">
              <span className="text-[11px] font-medium text-gray-400 uppercase tracking-wider">Markdown 源</span>
              <div className="flex items-center gap-1.5">
                <button onClick={() => fileRef.current?.click()}
                  className="flex items-center gap-1 px-2 py-1 text-[10px] rounded bg-gray-800 hover:bg-gray-700 text-gray-300 border border-gray-700 transition">
                  <Upload className="w-3 h-3" /> 上传 .md
                </button>
                <button onClick={() => reparse(SAMPLE_SCRIPT_MD)}
                  className="flex items-center gap-1 px-2 py-1 text-[10px] rounded bg-gray-800 hover:bg-gray-700 text-gray-300 border border-gray-700 transition">
                  <Sparkles className="w-3 h-3" /> 示例骨架
                </button>
                <input type="file" ref={fileRef} accept=".md,text/markdown,text/plain" className="hidden"
                  onChange={e => { const f = e.target.files?.[0]; if (f) handleFile(f); e.currentTarget.value = '' }} />
              </div>
            </div>
            <textarea
              className="flex-1 min-h-[420px] bg-gray-950 border border-gray-800 rounded-lg p-3 text-xs font-mono text-gray-200 resize-none focus:outline-none focus:border-indigo-600"
              placeholder={`粘贴剧本 Markdown，或上传 .md 文件 / 点「示例骨架」生成模板。\n\n约定格式：\n---\nlabel: 剧本标题\nduration: 60\n---\n\n## S0 开场 (6s)\nprompt: ...`}
              value={text}
              onChange={e => reparse(e.target.value)}
            />
          </div>

          {/* 右：解析预览 */}
          <div className="flex flex-col gap-2">
            <span className="text-[11px] font-medium text-gray-400 uppercase tracking-wider">解析结果</span>
            {!preview ? (
              <div className="flex-1 min-h-[420px] flex items-center justify-center text-xs text-gray-600 border border-dashed border-gray-800 rounded-lg">
                输入 Markdown 自动预览
              </div>
            ) : (
              <div className="flex-1 min-h-[420px] bg-gray-950 border border-gray-800 rounded-lg p-3 overflow-y-auto">
                <div className="text-sm text-gray-100 font-medium">{projectPreview?.title || preview.label}</div>
                <div className="text-[11px] text-gray-500 mt-0.5">{projectPreview?.description || preview.description}</div>
                <div className="mt-2 text-[10px] text-gray-500 flex items-center gap-3">
                  {projectPreview ? <span>{projectPreview.episodes.length} 集</span> : <span>{preview.scenes?.length || 0} 镜</span>}
                  {projectPreview && <span>{projectPreview.characters.length} 角色</span>}
                  {projectPreview && <span>{projectPreview.props.length} 物料</span>}
                  {!projectPreview && <span>{preview.duration || 0}s</span>}
                  {!projectPreview && preview.video_resolution && <span>{preview.video_resolution}</span>}
                  {!projectPreview && preview.video_ratio && <span>{preview.video_ratio}</span>}
                </div>
                {projectPreview && (
                  <div className="mt-3 grid grid-cols-2 gap-2">
                    <div className="rounded bg-gray-900 border border-gray-800 p-2">
                      <div className="text-[10px] text-gray-500 mb-1">角色</div>
                      {projectPreview.characters.slice(0, 8).map(c => <div key={c.label} className="text-[10px] text-gray-300 truncate">{c.tag} {c.label}</div>)}
                    </div>
                    <div className="rounded bg-gray-900 border border-gray-800 p-2">
                      <div className="text-[10px] text-gray-500 mb-1">物料</div>
                      {projectPreview.props.slice(0, 8).map(p => <div key={p.label} className="text-[10px] text-gray-300 truncate">{p.tag} {p.label}</div>)}
                    </div>
                  </div>
                )}
                <div className="mt-3 space-y-1.5">
                  {(projectPreview
                    ? projectPreview.episodes.map(ep => ({ key: ep.label, title: ep.label, duration: ep.duration || 0, sceneCount: (ep.scenes || []).length }))
                    : (preview.scenes || []).map(s => ({ key: s.id, title: `${s.id} · ${s.label}`, duration: s.duration || 0, prompt: s.prompt, storyboardUrl: s.storyboard_url }))
                  ).map((item) => (
                    <div key={item.key} className="px-2 py-1.5 rounded bg-gray-900 border border-gray-800">
                      <div className="text-[11px] text-gray-200 flex items-center justify-between">
                        <span className="font-medium">{item.title}</span>
                        <span className="text-[10px] text-gray-500">{item.duration}s</span>
                      </div>
                      {'prompt' in item && item.prompt && (
                        <div className="text-[10px] text-gray-500 mt-0.5 line-clamp-2">{item.prompt}</div>
                      )}
                      {'sceneCount' in item && (
                        <div className="text-[10px] text-gray-500 mt-0.5">{item.sceneCount} 场</div>
                      )}
                      {'storyboardUrl' in item && item.storyboardUrl && (
                        <div className="text-[10px] text-cyan-500 mt-0.5 truncate">📷 {item.storyboardUrl}</div>
                      )}
                    </div>
                  ))}
                </div>
                {warnings.length > 0 && (
                  <div className="mt-3 p-2 rounded bg-amber-900/20 border border-amber-700/40 text-[10px] text-amber-200 flex items-start gap-1.5">
                    <AlertTriangle className="w-3 h-3 mt-0.5 flex-shrink-0" />
                    <div className="space-y-0.5">{warnings.map((w, i) => <div key={i}>{w}</div>)}</div>
                  </div>
                )}
              </div>
            )}
          </div>
        </div>

        <div className="px-5 py-3 border-t border-gray-800 flex items-center justify-between bg-gray-950">
          <span className="text-[10px] text-gray-600">
            导入后会在画布生成 1 个剧本节点 + N 个场景, 可继续编辑、生成视频、合成成片。
          </span>
          <div className="flex items-center gap-2">
            <button onClick={onClose}
              className="px-3 py-1.5 text-xs rounded bg-gray-800 hover:bg-gray-700 text-gray-300 border border-gray-700 transition">
              取消
            </button>
            <button onClick={submit}
              disabled={(!preview || (preview.scenes?.length || 0) === 0) && !projectPreview}
              className="flex items-center gap-1.5 px-4 py-1.5 text-xs font-medium rounded bg-gradient-to-r from-indigo-600 to-cyan-600 hover:from-indigo-500 hover:to-cyan-500 text-white shadow disabled:opacity-40 disabled:cursor-not-allowed transition">
              <FileText className="w-3.5 h-3.5" /> {projectPreview ? '导入整部新剧' : '导入剧本'}
            </button>
          </div>
        </div>
      </div>
    </div>
  )
}
