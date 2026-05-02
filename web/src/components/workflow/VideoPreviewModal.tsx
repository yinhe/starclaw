import { useEffect, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import { X, Volume2, VolumeX, Download, ExternalLink } from 'lucide-react'

interface Props {
  open: boolean
  src: string
  title?: string
  /** 如果给了就是用户"现在选中"的 take/scene 标签（S1 · t2 / EP05 · S1 之类） */
  subtitle?: string
  onClose: () => void
}

/**
 * 全屏视频预览 modal —— 用户双击 canvas 场景节点 / take 缩略图时弹出。
 * - 默认 autoPlay + 带声音（第一次可能被浏览器拦成静音，用户按右下角 🔊 解除）
 * - ESC 关闭
 * - 点击背景遮罩关闭，但不冒泡到 React Flow（nodrag/stopPropagation）
 */
export default function VideoPreviewModal({ open, src, title, subtitle, onClose }: Props) {
  const videoRef = useRef<HTMLVideoElement | null>(null)
  const [muted, setMuted] = useState(false)

  useEffect(() => {
    if (!open) return
    const onKey = (e: KeyboardEvent) => { if (e.key === 'Escape') onClose() }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [open, onClose])

  // open 翻转为 true 时尝试带声自动播放；若被浏览器策略拦截，退化为 muted autoplay
  useEffect(() => {
    if (!open) return
    const v = videoRef.current
    if (!v) return
    v.muted = false
    setMuted(false)
    v.play().catch(() => {
      // 被浏览器 autoplay policy 拦了 → 降级成 muted 播放，给用户一个 🔊 按钮手动解除
      v.muted = true
      setMuted(true)
      v.play().catch(() => { /* 彻底失败就让用户自己点播放 */ })
    })
  }, [open, src])

  if (!open) return null
  return createPortal(
    <div className="fixed inset-0 z-[200] flex items-center justify-center bg-black/85 backdrop-blur-sm nodrag"
         onClick={onClose}
         onMouseDown={(e) => e.stopPropagation()}>
      <div className="relative max-w-[min(90vw,1280px)] max-h-[90vh] w-full flex flex-col gap-2"
           onClick={(e) => e.stopPropagation()}>

        {/* 顶栏：标题 + 关闭 */}
        <div className="flex items-center justify-between px-1">
          <div className="min-w-0">
            {title && <div className="text-sm font-semibold text-gray-100 truncate">{title}</div>}
            {subtitle && <div className="text-[11px] text-gray-400 font-mono truncate">{subtitle}</div>}
          </div>
          <button onClick={onClose}
                  className="p-1.5 rounded-full bg-gray-800/80 hover:bg-gray-700 text-gray-300 hover:text-white transition">
            <X className="w-4 h-4" />
          </button>
        </div>

        {/* 视频本体 */}
        <div className="relative flex-1 rounded-xl overflow-hidden bg-black border border-gray-700 shadow-2xl">
          <video
            ref={videoRef}
            src={src}
            className="w-full h-full max-h-[80vh] object-contain bg-black"
            controls
            playsInline
            preload="auto"
          />
          {/* 右下角显示一个独立的静音状态指示；点它能切换。浏览器策略降级时特别有用。 */}
          <button onClick={() => {
                    const v = videoRef.current
                    if (!v) return
                    v.muted = !v.muted
                    setMuted(v.muted)
                    if (!v.muted) void v.play().catch(() => { /* noop */ })
                  }}
                  title={muted ? '当前静音，点击开声' : '静音'}
                  className="absolute top-2 right-2 p-2 rounded-full bg-black/70 hover:bg-black/90 text-white transition">
            {muted ? <VolumeX className="w-4 h-4" /> : <Volume2 className="w-4 h-4" />}
          </button>
        </div>

        {/* 底栏：外链 + 下载 */}
        <div className="flex items-center justify-end gap-2 px-1">
          <a href={src} target="_blank" rel="noreferrer"
             className="inline-flex items-center gap-1 px-2 py-1 text-[11px] rounded bg-gray-800/80 hover:bg-gray-700 text-gray-300 hover:text-white transition">
            <ExternalLink className="w-3 h-3" /> 新标签打开
          </a>
          <a href={src} download
             className="inline-flex items-center gap-1 px-2 py-1 text-[11px] rounded bg-gray-800/80 hover:bg-gray-700 text-gray-300 hover:text-white transition">
            <Download className="w-3 h-3" /> 下载
          </a>
        </div>
      </div>
    </div>,
    document.body
  )
}
