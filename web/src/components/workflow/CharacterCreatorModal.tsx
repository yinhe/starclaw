import { useState } from 'react'
import { X, Users, Upload, Sparkles } from 'lucide-react'
import type { CharacterData } from './episodeTypes'

interface Props {
  open: boolean
  existingTags: string[]       // e.g. ["[图1]","[图2]"]
  onClose: () => void
  onCreate: (data: CharacterData) => void
}

const ROLE_PRESETS = [
  { value: '女一', color: 'bg-rose-500/20 text-rose-300 border-rose-500/40' },
  { value: '女二', color: 'bg-pink-500/20 text-pink-300 border-pink-500/40' },
  { value: '男一', color: 'bg-cyan-500/20 text-cyan-300 border-cyan-500/40' },
  { value: '男二', color: 'bg-blue-500/20 text-blue-300 border-blue-500/40' },
  { value: '配角', color: 'bg-violet-500/20 text-violet-300 border-violet-500/40' },
  { value: '生物', color: 'bg-amber-500/20 text-amber-300 border-amber-500/40' },
]

export default function CharacterCreatorModal({ open, existingTags, onClose, onCreate }: Props) {
  const [name, setName] = useState('')
  const [role, setRole] = useState('女一')
  const [appearance, setAppearance] = useState('')
  const [imageUrl, setImageUrl] = useState('')

  if (!open) return null

  // Auto-assign next [图N] tag
  const nextTagNumber = (() => {
    const used = new Set<number>()
    existingTags.forEach(t => {
      const m = t.match(/\[图(\d+)\]/)
      if (m) used.add(parseInt(m[1], 10))
    })
    for (let i = 1; i <= 99; i++) if (!used.has(i)) return i
    return existingTags.length + 1
  })()
  const nextTag = `[图${nextTagNumber}]`

  const submit = () => {
    if (!name.trim()) return
    onCreate({
      category: 'character',
      label: name.trim(),
      tag: nextTag,
      role,
      appearance_card: appearance.trim() || undefined,
      imageUrl: imageUrl.trim() || undefined,
      description: `${role}·${(appearance.split('，')[0] || '').slice(0, 14)}`,
    })
    // reset + close
    setName(''); setAppearance(''); setImageUrl(''); setRole('女一')
    onClose()
  }

  return (
    <div className="fixed inset-0 z-[100] flex items-center justify-center bg-black/60 backdrop-blur-sm p-4">
      <div className="w-full max-w-lg bg-gray-900 border border-gray-700 rounded-2xl shadow-2xl overflow-hidden">
        <div className="px-5 py-4 border-b border-gray-800 flex items-center justify-between bg-gradient-to-r from-violet-900/40 to-gray-900">
          <div className="flex items-center gap-2">
            <Users className="w-4 h-4 text-violet-400" />
            <h3 className="text-sm font-semibold text-gray-100">新建角色</h3>
            <span className="ml-2 px-2 py-0.5 rounded-md bg-violet-500/20 border border-violet-500/40 text-[11px] font-mono text-violet-300">
              {nextTag} 自动分配
            </span>
          </div>
          <button onClick={onClose} className="text-gray-500 hover:text-gray-200 transition-colors">
            <X className="w-4 h-4" />
          </button>
        </div>

        <div className="p-5 space-y-4">
          <div>
            <label className="block text-[11px] font-medium text-gray-400 mb-1.5 uppercase tracking-wider">角色名</label>
            <input
              value={name} onChange={e => setName(e.target.value)}
              placeholder="例：林见月 / ZERG / 苏蜜"
              className="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-sm text-gray-200 placeholder-gray-600 focus:border-violet-500 focus:outline-none"
              autoFocus />
          </div>

          <div>
            <label className="block text-[11px] font-medium text-gray-400 mb-1.5 uppercase tracking-wider">角色类型</label>
            <div className="flex flex-wrap gap-1.5">
              {ROLE_PRESETS.map(r => (
                <button key={r.value} type="button" onClick={() => setRole(r.value)}
                  className={`px-2.5 py-1 text-xs rounded-md border transition ${role === r.value ? r.color : 'bg-gray-800 text-gray-400 border-gray-700 hover:border-gray-600'}`}>
                  {r.value}
                </button>
              ))}
            </div>
          </div>

          <div>
            <label className="block text-[11px] font-medium text-gray-400 mb-1.5 uppercase tracking-wider flex items-center gap-1.5">
              <Sparkles className="w-3 h-3" /> 外观卡（Appearance Card · 全集复用）
            </label>
            <textarea
              value={appearance} onChange={e => setAppearance(e.target.value)}
              rows={3}
              placeholder="服装+发型+体型+配色，具体可拍。例：薄荷绿古装汉服+透纱外袍的瘦弱年轻中国女子，黑色长直发柔顺，古风流苏耳环+翡翠银腰扣"
              className="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-sm text-gray-200 placeholder-gray-600 focus:border-violet-500 focus:outline-none resize-none" />
            <p className="mt-1 text-[10px] text-gray-600">一字不差复用，不写抽象词（"美丽""优雅"），要具体可拍</p>
          </div>

          <div>
            <label className="block text-[11px] font-medium text-gray-400 mb-1.5 uppercase tracking-wider flex items-center gap-1.5">
              <Upload className="w-3 h-3" /> 参考图 URL（三视图/综合sheet）
            </label>
            <input
              value={imageUrl} onChange={e => setImageUrl(e.target.value)}
              placeholder="/v1/images/xxx.png 或 https://cdn.../sheet.png"
              className="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-xs font-mono text-gray-200 placeholder-gray-600 focus:border-violet-500 focus:outline-none" />
            {imageUrl && (
              <div className="mt-2 rounded-lg overflow-hidden border border-gray-700">
                <img src={imageUrl} alt="" className="w-full h-32 object-cover"
                  onError={e => { (e.target as HTMLImageElement).style.display = 'none' }} />
              </div>
            )}
          </div>
        </div>

        <div className="px-5 py-3 border-t border-gray-800 bg-gray-950/50 flex items-center justify-end gap-2">
          <button onClick={onClose}
            className="px-3 py-1.5 text-xs text-gray-400 hover:text-gray-200 transition">取消</button>
          <button onClick={submit} disabled={!name.trim()}
            className="px-4 py-1.5 text-xs font-medium rounded-lg bg-violet-600 text-white hover:bg-violet-500 disabled:opacity-40 disabled:cursor-not-allowed transition">
            创建 {nextTag}
          </button>
        </div>
      </div>
    </div>
  )
}
