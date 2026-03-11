import { useState, useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { Film, Trash2, Download, Clock, CheckCircle2, Loader2, XCircle, Play, Ban, RefreshCw, ChevronDown, ChevronRight, Layers, MessageSquare, Volume2, Music2 } from 'lucide-react'
import { videoAPI, musicAPI } from '../lib/api'
import { useToastStore } from '../stores/toastStore'

interface VideoRecord {
  id: string
  task_id: string
  model: string
  prompt: string
  video_url: string
  img_url: string
  narrated_url: string
  size: string
  duration: number
  scene: string
  status: string
  type: string
  clip_ids: string
  conversation_id: string
  created_at: string
}

const STATUS_MAP: Record<string, { label: string; color: string; icon: typeof Clock }> = {
  running:   { label: '生成中', color: 'text-blue-500', icon: Loader2 },
  pending:   { label: '等待中', color: 'text-yellow-500', icon: Clock },
  succeeded: { label: '已完成', color: 'text-green-500', icon: CheckCircle2 },
  failed:    { label: '失败', color: 'text-red-500', icon: XCircle },
  cancelled: { label: '已取消', color: 'text-gray-400', icon: Ban },
}

export default function VideosPage() {
  const navigate = useNavigate()
  const addToast = useToastStore(s => s.addToast)
  const [videos, setVideos] = useState<VideoRecord[]>([])
  const [loading, setLoading] = useState(true)
  const [playingId, setPlayingId] = useState<string | null>(null)
  const [expandedMerge, setExpandedMerge] = useState<string | null>(null)
  const [confirmModal, setConfirmModal] = useState<{ title: string; message: string; onConfirm: () => void } | null>(null)
  const [dubModal, setDubModal] = useState<{ videoId: string; prompt: string; duration: number } | null>(null)
  const [dubText, setDubText] = useState('')
  const [dubVoice, setDubVoice] = useState('longyuan')
  const [dubSubtitle, setDubSubtitle] = useState(true)
  const [dubLoading, setDubLoading] = useState(false)
  const [activeTab, setActiveTab] = useState<'merged' | 'clips'>('merged')
  const [musicModal, setMusicModal] = useState<{ videoId: string } | null>(null)
  const [musicList, setMusicList] = useState<{ id: string; prompt: string; lyrics: string; local_url: string; status: string; duration: number; model: string; created_at: string }[]>([])
  const [selectedMusicId, setSelectedMusicId] = useState<string | null>(null)
  const [musicLoading, setMusicLoading] = useState(false)

  useEffect(() => { loadVideos() }, [])

  useEffect(() => {
    const hasActive = videos.some(v => v.status === 'running' || v.status === 'pending')
    if (!hasActive) return
    const timer = setInterval(loadVideos, 10000)
    return () => clearInterval(timer)
  }, [videos])

  const loadVideos = async () => {
    setLoading(true)
    try {
      const res = await videoAPI.list()
      setVideos(res.data.videos || [])
    } catch (e) {
      console.error('Failed to load videos', e)
    } finally {
      setLoading(false)
    }
  }

  const showConfirm = (title: string, message: string, onConfirm: () => void) => {
    setConfirmModal({ title, message, onConfirm })
  }
  const handleDelete = (id: string) => {
    showConfirm('删除视频', '确定要删除这个视频吗？此操作不可撤销。', async () => {
      try { await videoAPI.delete(id); addToast('success', '视频已删除'); loadVideos() } catch (e) { console.error(e); addToast('error', '删除失败') }
    })
  }
  const handleCancel = async (id: string) => {
    try { await videoAPI.cancel(id); loadVideos() } catch (e) { console.error(e) }
  }
  const handleRetry = async (id: string) => {
    try { await videoAPI.retry(id); addToast('info', '正在重新生成...'); loadVideos(); setTimeout(loadVideos, 2000); setTimeout(loadVideos, 5000) } catch (e) { console.error(e); addToast('error', '重试失败') }
  }
  const handleRegenerate = (id: string) => {
    showConfirm('重新生成片段', '原片段将被删除并重新生成，确定继续吗？', async () => {
      try { await videoAPI.regenerate(id); addToast('info', '片段正在重新生成，请稍候...'); loadVideos(); setTimeout(loadVideos, 2000); setTimeout(loadVideos, 5000) } catch (e) { console.error(e); addToast('error', '重新生成失败') }
    })
  }
  const handleRemerge = (id: string) => {
    showConfirm('重新合成', '使用原始片段重新合成视频，确定继续吗？', async () => {
      try { await videoAPI.remerge(id); addToast('info', '视频正在重新合成，请稍候...'); loadVideos(); setTimeout(loadVideos, 2000); setTimeout(loadVideos, 5000) } catch (e) { console.error(e); addToast('error', '重新合成失败') }
    })
  }
  const openDubModal = (video: VideoRecord) => {
    setDubText(video.prompt || '')
    setDubVoice('longyuan')
    setDubSubtitle(true)
    setDubModal({ videoId: video.id, prompt: video.prompt, duration: video.duration })
  }
  const handleDub = async () => {
    if (!dubModal || !dubText.trim()) return
    setDubLoading(true)
    try {
      await videoAPI.dub(dubModal.videoId, dubText.trim(), dubVoice, dubSubtitle ? 'auto' : 'none')
      setDubModal(null)
      addToast('success', '配音任务已开始，完成后将自动显示在视频画廊')
      loadVideos(); setTimeout(loadVideos, 3000); setTimeout(loadVideos, 8000)
    } catch (e) { console.error(e); addToast('error', '配音失败') } finally { setDubLoading(false) }
  }
  const openMusicModal = async (videoId: string) => {
    setSelectedMusicId(null)
    setMusicModal({ videoId })
    try {
      const res = await musicAPI.list()
      const all = (res.data.music || []).filter((m: any) => m.status === 'succeeded' && m.local_url)
      setMusicList(all)
    } catch (e) { console.error(e); setMusicList([]) }
  }
  const handleAddMusic = async () => {
    if (!musicModal || !selectedMusicId) return
    setMusicLoading(true)
    try {
      await videoAPI.addMusic(musicModal.videoId, selectedMusicId)
      setMusicModal(null)
      addToast('success', '配乐任务已开始，完成后将自动显示在视频画廊')
      loadVideos(); setTimeout(loadVideos, 3000); setTimeout(loadVideos, 8000)
    } catch (e) { console.error(e); addToast('error', '配乐失败') } finally { setMusicLoading(false) }
  }

  // Conversations that have an MV: hide the intermediate merged version
  const mvConvIds = new Set(videos.filter(v => v.type === 'mv').map(v => v.conversation_id))
  const mergedVideos = videos.filter(v => {
    if (v.type === 'mv') return true
    if (v.type === 'merged') return !mvConvIds.has(v.conversation_id) // hide merged if MV exists
    return false
  }).sort((a, b) => {
    // MV first, then by date descending
    if (a.type === 'mv' && b.type !== 'mv') return -1
    if (a.type !== 'mv' && b.type === 'mv') return 1
    return new Date(b.created_at).getTime() - new Date(a.created_at).getTime()
  })

  const formatTime = (dateStr: string) => {
    const d = new Date(dateStr)
    const now = new Date()
    const diffMs = now.getTime() - d.getTime()
    const diffMin = Math.floor(diffMs / 60000)
    const diffHr = Math.floor(diffMs / 3600000)
    const diffDay = Math.floor(diffMs / 86400000)
    if (diffMin < 1) return '刚刚'
    if (diffMin < 60) return `${diffMin}分钟前`
    if (diffHr < 24) return `${diffHr}小时前`
    if (diffDay < 7) return `${diffDay}天前`
    return d.toLocaleDateString('zh-CN', { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' })
  }
  const narratedVideos = videos.filter(v => v.type === 'narrated')
  const clipVideos = videos.filter(v => v.type !== 'merged' && v.type !== 'narrated' && v.type !== 'mv').sort((a, b) => {
    // Running/pending first, then by date descending
    const statusOrder: Record<string, number> = { running: 0, pending: 1, succeeded: 2, failed: 3, cancelled: 4 }
    const sa = statusOrder[a.status] ?? 2
    const sb = statusOrder[b.status] ?? 2
    if (sa !== sb) return sa - sb
    return new Date(b.created_at).getTime() - new Date(a.created_at).getTime()
  })
  const runningCount = clipVideos.filter(v => v.status === 'running' || v.status === 'pending').length
  const failedCount = clipVideos.filter(v => v.status === 'failed' || v.status === 'cancelled').length

  // Find narrated version for a merged video
  const getNarratedForMerge = (mergeVideo: VideoRecord): VideoRecord | undefined => {
    return narratedVideos.find(nv => {
      try {
        const sourceIds: string[] = JSON.parse(nv.clip_ids || '[]')
        return sourceIds.includes(mergeVideo.id)
      } catch { return false }
    })
  }
  // Find orphaned narrated videos (no matching merged parent in mergedVideos list)
  const orphanedNarrated = narratedVideos.filter(nv => {
    try {
      const sourceIds: string[] = JSON.parse(nv.clip_ids || '[]')
      return !mergedVideos.some(mv => sourceIds.includes(mv.id))
    } catch { return true }
  })

  const getClipsForMerge = (mergeVideo: VideoRecord): VideoRecord[] => {
    try {
      const ids: string[] = JSON.parse(mergeVideo.clip_ids || '[]')
      return ids.map(id => videos.find(v => v.id === id)).filter(Boolean) as VideoRecord[]
    } catch { return [] }
  }

  const renderVideoPreview = (video: VideoRecord, small = false) => {
    const isPlaying = playingId === video.id
    const aspectClass = small ? 'aspect-video' : 'aspect-video'
    const thumbStyle = video.img_url ? { backgroundImage: `url(${video.img_url})`, backgroundSize: 'cover', backgroundPosition: 'center' } : {}
    return (
      <div className={`relative ${aspectClass} bg-gray-900 flex items-center justify-center`} style={!isPlaying ? thumbStyle : {}}>
        {video.status === 'succeeded' && video.video_url ? (
          isPlaying ? (
            <video src={video.video_url} controls autoPlay className="w-full h-full object-contain" onEnded={() => setPlayingId(null)} />
          ) : (
            <button onClick={() => setPlayingId(video.id)} className="flex flex-col items-center gap-2 text-white/70 hover:text-white transition-colors w-full h-full justify-center">
              <div className={`${small ? 'w-10 h-10' : 'w-14 h-14'} rounded-full bg-black/40 backdrop-blur-sm flex items-center justify-center hover:bg-black/50`}>
                <Play className={`${small ? 'w-5 h-5' : 'w-7 h-7'} ml-0.5`} />
              </div>
              <span className="text-xs bg-black/40 px-2 py-0.5 rounded">{video.duration}秒 · {video.size}</span>
            </button>
          )
        ) : video.status === 'running' || video.status === 'pending' ? (
          <div className="flex flex-col items-center gap-2 text-white/50">
            <Loader2 className="w-8 h-8 animate-spin" />
            <span className="text-xs">生成中...</span>
          </div>
        ) : video.status === 'cancelled' ? (
          <div className="flex flex-col items-center gap-2 text-white/30">
            <Ban className="w-8 h-8" />
            <span className="text-xs">已取消</span>
          </div>
        ) : (
          <div className="flex flex-col items-center gap-2 text-white/30">
            <XCircle className="w-8 h-8" />
            <span className="text-xs">生成失败</span>
          </div>
        )}
      </div>
    )
  }

  const renderActions = (video: VideoRecord, options?: { showRegenerate?: boolean; showRemerge?: boolean }) => (
    <div className="flex items-center gap-1">
      {(video.status === 'running' || video.status === 'pending') && (
        <button onClick={() => handleCancel(video.id)} className="p-1.5 rounded-lg text-gray-400 hover:text-orange-500 hover:bg-orange-50 dark:hover:bg-orange-900/20 transition-colors" title="取消生成">
          <Ban className="w-4 h-4" />
        </button>
      )}
      {(video.status === 'failed' || video.status === 'cancelled') && (
        <button onClick={() => handleRetry(video.id)} className="p-1.5 rounded-lg text-gray-400 hover:text-blue-500 hover:bg-blue-50 dark:hover:bg-blue-900/20 transition-colors" title="重新生成">
          <RefreshCw className="w-4 h-4" />
        </button>
      )}
      {options?.showRegenerate && video.status === 'succeeded' && (
        <button onClick={() => openDubModal(video)} className="flex items-center gap-1 px-2 py-1 rounded-lg text-xs font-medium text-blue-600 bg-blue-50 hover:bg-blue-100 dark:text-blue-400 dark:bg-blue-900/30 dark:hover:bg-blue-900/50 transition-colors" title="配音+字幕">
          <Volume2 className="w-3.5 h-3.5" />
          配音
        </button>
      )}
      {options?.showRegenerate && video.status === 'succeeded' && (
        <button onClick={() => handleRegenerate(video.id)} className="flex items-center gap-1 px-2 py-1 rounded-lg text-xs font-medium text-violet-600 bg-violet-50 hover:bg-violet-100 dark:text-violet-400 dark:bg-violet-900/30 dark:hover:bg-violet-900/50 transition-colors" title="重新生成此片段">
          <RefreshCw className="w-3.5 h-3.5" />
          重做
        </button>
      )}
      {options?.showRemerge && video.status === 'succeeded' && (
        <button onClick={() => openMusicModal(video.id)} className="flex items-center gap-1 px-2 py-1 rounded-lg text-xs font-medium text-amber-600 bg-amber-50 hover:bg-amber-100 dark:text-amber-400 dark:bg-amber-900/30 dark:hover:bg-amber-900/50 transition-colors" title="添加背景音乐">
          <Music2 className="w-3.5 h-3.5" />
          配乐
        </button>
      )}
      {options?.showRemerge && video.status === 'succeeded' && (
        <button onClick={() => handleRemerge(video.id)} className="flex items-center gap-1 px-2 py-1 rounded-lg text-xs font-medium text-emerald-600 bg-emerald-50 hover:bg-emerald-100 dark:text-emerald-400 dark:bg-emerald-900/30 dark:hover:bg-emerald-900/50 transition-colors" title="重新合成视频">
          <RefreshCw className="w-3.5 h-3.5" />
          重新合成
        </button>
      )}
      {video.video_url && (
        <a href={video.video_url} download className="p-1.5 rounded-lg text-gray-400 hover:text-blue-500 hover:bg-blue-50 dark:hover:bg-blue-900/20 transition-colors" title="下载">
          <Download className="w-4 h-4" />
        </a>
      )}
      <button onClick={() => handleDelete(video.id)} className="p-1.5 rounded-lg text-gray-400 hover:text-red-500 hover:bg-red-50 dark:hover:bg-red-900/20 transition-colors" title="删除">
        <Trash2 className="w-4 h-4" />
      </button>
    </div>
  )

  return (
    <div className="h-full overflow-y-auto bg-gray-50 dark:bg-gray-900">
      <div className="max-w-6xl mx-auto px-6 py-8">
        <div className="mb-6">
          <h1 className="text-2xl font-bold text-gray-900 dark:text-white flex items-center gap-2">
            <Film className="w-6 h-6 text-violet-500" />
            视频画廊
          </h1>
          <p className="text-sm text-gray-500 dark:text-gray-400 mt-1">
            通过对话生成的所有 AI 视频都在这里查看和下载
          </p>
        </div>

        {/* Tab bar */}
        <div className="flex items-center gap-1 mb-6 border-b border-gray-200 dark:border-gray-700">
          <button
            onClick={() => setActiveTab('merged')}
            className={`px-4 py-2.5 text-sm font-medium border-b-2 transition-colors ${activeTab === 'merged' ? 'border-violet-500 text-violet-600 dark:text-violet-400' : 'border-transparent text-gray-500 hover:text-gray-700 dark:text-gray-400'}`}
          >
            <Layers className="w-4 h-4 inline mr-1.5 -mt-0.5" />
            合成视频
            {mergedVideos.length > 0 && <span className="ml-1.5 px-1.5 py-0.5 text-xs rounded-full bg-violet-100 dark:bg-violet-900/30 text-violet-600 dark:text-violet-400">{mergedVideos.length}</span>}
          </button>
          <button
            onClick={() => setActiveTab('clips')}
            className={`px-4 py-2.5 text-sm font-medium border-b-2 transition-colors ${activeTab === 'clips' ? 'border-blue-500 text-blue-600 dark:text-blue-400' : 'border-transparent text-gray-500 hover:text-gray-700 dark:text-gray-400'}`}
          >
            <Film className="w-4 h-4 inline mr-1.5 -mt-0.5" />
            视频片段
            {clipVideos.length > 0 && <span className="ml-1.5 px-1.5 py-0.5 text-xs rounded-full bg-blue-100 dark:bg-blue-900/30 text-blue-600 dark:text-blue-400">{clipVideos.length}</span>}
          </button>
          <div className="ml-auto flex items-center gap-3 pb-1">
            {runningCount > 0 && <span className="text-xs text-blue-500"><Loader2 className="w-3 h-3 inline animate-spin mr-0.5" />{runningCount} 生成中</span>}
            {failedCount > 0 && <span className="text-xs text-red-500">{failedCount} 失败</span>}
            <button onClick={loadVideos} className="text-xs text-violet-600 hover:text-violet-700 dark:text-violet-400">刷新</button>
          </div>
        </div>

        {loading && videos.length === 0 ? (
          <div className="text-center py-20 text-gray-400">
            <Loader2 className="w-8 h-8 animate-spin mx-auto mb-2" />
            加载中...
          </div>
        ) : videos.length === 0 ? (
          <div className="text-center py-20">
            <Film className="w-12 h-12 text-gray-300 dark:text-gray-600 mx-auto mb-3" />
            <p className="text-gray-400 dark:text-gray-500 text-sm">还没有生成过视频</p>
            <p className="text-gray-400 dark:text-gray-500 text-xs mt-1">在对话中告诉 Agent &quot;帮我生成一个视频&quot;即可开始</p>
          </div>
        ) : (
          <>
            {/* Merged Videos Tab */}
            {activeTab === 'merged' && (
              <div>
                <div className="space-y-4">
                  {mergedVideos.map(mv => {
                    const isMV = mv.type === 'mv'
                    const clips = getClipsForMerge(mv)
                    const narrated = !isMV ? getNarratedForMerge(mv) : undefined
                    const isExpanded = expandedMerge === mv.id
                    const showingNarrated = narrated && playingId === `narrated-${mv.id}`
                    const activeVideo = showingNarrated ? narrated : mv
                    const borderColor = isMV ? 'border-rose-200 dark:border-rose-800' : 'border-violet-200 dark:border-violet-800'
                    return (
                      <div key={mv.id} className={`rounded-xl border-2 ${borderColor} bg-white dark:bg-gray-800 overflow-hidden shadow-sm`}>
                        <div className="flex gap-4 p-4">
                          <div className="w-80 flex-none">
                            {/* Version toggle tabs — only for non-MV merged videos */}
                            {narrated && (
                              <div className="flex mb-2 rounded-lg overflow-hidden border border-gray-200 dark:border-gray-600">
                                <button
                                  onClick={() => setPlayingId(showingNarrated ? null : null)}
                                  className={`flex-1 text-xs py-1.5 font-medium transition-colors ${!showingNarrated ? 'bg-violet-500 text-white' : 'bg-gray-50 dark:bg-gray-700 text-gray-500 dark:text-gray-400 hover:bg-gray-100 dark:hover:bg-gray-600'}`}
                                >
                                  原版
                                </button>
                                <button
                                  onClick={() => setPlayingId(`narrated-${mv.id}`)}
                                  className={`flex-1 text-xs py-1.5 font-medium transition-colors ${showingNarrated ? 'bg-emerald-500 text-white' : 'bg-gray-50 dark:bg-gray-700 text-gray-500 dark:text-gray-400 hover:bg-gray-100 dark:hover:bg-gray-600'}`}
                                >
                                  🎙 配音版
                                </button>
                              </div>
                            )}
                            <div className="rounded-lg overflow-hidden">
                              {renderVideoPreview(activeVideo)}
                            </div>
                          </div>
                          <div className="flex-1 min-w-0">
                            <div className="flex items-center gap-2 mb-2 flex-wrap">
                              {isMV ? (
                                <span className="text-xs font-medium px-2 py-0.5 rounded-full bg-rose-100 text-rose-700 dark:bg-rose-900/30 dark:text-rose-300">
                                  🎵 MV
                                </span>
                              ) : (
                                <span className="text-xs font-medium px-2 py-0.5 rounded-full bg-violet-100 text-violet-700 dark:bg-violet-900/30 dark:text-violet-300">
                                  合成视频
                                </span>
                              )}
                              {narrated && (
                                <span className="text-xs font-medium px-2 py-0.5 rounded-full bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300">
                                  🎙 已配音
                                </span>
                              )}
                              <span className="text-xs text-gray-400">{mv.duration}秒{clips.length > 0 ? ` · ${clips.length}个片段` : ''} · {mv.size} · {formatTime(mv.created_at)}</span>
                              {mv.conversation_id && (
                                <button
                                  onClick={() => navigate(`/chat/${mv.conversation_id}`)}
                                  className="flex items-center gap-0.5 px-1.5 py-0.5 bg-blue-50 dark:bg-blue-900/30 text-blue-500 rounded-full text-[10px] hover:bg-blue-100 dark:hover:bg-blue-900/50 transition-colors"
                                  title="跳转到来源对话"
                                >
                                  <MessageSquare className="w-2.5 h-2.5" />来源对话
                                </button>
                              )}
                            </div>
                            <p className="text-sm text-gray-700 dark:text-gray-300 mb-1">{mv.prompt}</p>
                            {narrated && showingNarrated && (
                              <p className="text-xs text-emerald-600 dark:text-emerald-400 mb-2">{narrated.prompt}</p>
                            )}
                            <div className="flex items-center gap-2">
                              {clips.length > 0 && (
                                <button
                                  onClick={() => setExpandedMerge(isExpanded ? null : mv.id)}
                                  className="flex items-center gap-1 text-xs text-violet-600 dark:text-violet-400 hover:text-violet-700"
                                >
                                  {isExpanded ? <ChevronDown className="w-3.5 h-3.5" /> : <ChevronRight className="w-3.5 h-3.5" />}
                                  查看 {clips.length} 个片段
                                </button>
                              )}
                              <div className="ml-auto flex items-center gap-1">
                                {renderActions(activeVideo, { showRemerge: true })}
                              </div>
                            </div>
                          </div>
                        </div>
                        {isExpanded && clips.length > 0 && (
                          <div className="border-t border-violet-100 dark:border-violet-900/30 bg-violet-50/50 dark:bg-violet-950/20 p-4">
                            <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-4 gap-3">
                              {clips.map((clip, idx) => (
                                <div key={clip.id} className="rounded-lg border border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800 overflow-hidden">
                                  {renderVideoPreview(clip, true)}
                                  <div className="p-2">
                                    <div className="flex items-center gap-1 mb-1">
                                      <span className="text-[10px] font-medium text-gray-500">#{idx + 1}</span>
                                      {clip.scene && <span className="text-[10px] bg-gray-100 dark:bg-gray-700 text-gray-400 px-1 rounded">{clip.scene}</span>}
                                      {clip.status === 'succeeded' && (
                                        <button onClick={() => handleRegenerate(clip.id)} className="ml-auto p-0.5 rounded text-gray-300 hover:text-violet-500 transition-colors" title="重新生成此片段">
                                          <RefreshCw className="w-3 h-3" />
                                        </button>
                                      )}
                                    </div>
                                    <p className="text-[11px] text-gray-600 dark:text-gray-400 line-clamp-2">{clip.prompt}</p>
                                  </div>
                                </div>
                              ))}
                            </div>
                          </div>
                        )}
                      </div>
                    )
                  })}
                  {/* Orphaned narrated videos (source merged video was deleted) */}
                  {orphanedNarrated.map(nv => (
                    <div key={nv.id} className="rounded-xl border-2 border-emerald-200 dark:border-emerald-800 bg-white dark:bg-gray-800 overflow-hidden shadow-sm">
                      <div className="flex gap-4 p-4">
                        <div className="w-80 flex-none rounded-lg overflow-hidden">
                          {renderVideoPreview(nv)}
                        </div>
                        <div className="flex-1 min-w-0">
                          <div className="flex items-center gap-2 mb-2">
                            <span className="text-xs font-medium px-2 py-0.5 rounded-full bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300">
                              🎙 配音视频
                            </span>
                            <span className="text-xs text-gray-400">{nv.duration}秒 · {nv.size}</span>
                          </div>
                          <p className="text-sm text-gray-700 dark:text-gray-300 mb-3">{nv.prompt}</p>
                          <div className="ml-auto">{renderActions(nv)}</div>
                        </div>
                      </div>
                    </div>
                  ))}
                </div>
              </div>
            )}

            {/* Clips Tab */}
            {activeTab === 'clips' && (
              <div>
                <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-5">
                  {clipVideos.map(video => {
                    const st = STATUS_MAP[video.status] || STATUS_MAP.pending
                    const StIcon = st.icon
                    return (
                      <div key={video.id} className="rounded-xl border border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800 overflow-hidden shadow-sm hover:shadow-md transition-shadow">
                        {renderVideoPreview(video)}
                        <div className="p-4">
                          <div className="flex items-center gap-2 mb-2">
                            <StIcon className={`w-4 h-4 ${st.color} ${video.status === 'running' ? 'animate-spin' : ''}`} />
                            <span className={`text-xs font-medium ${st.color}`}>{st.label}</span>
                            {video.scene && (
                              <span className="text-[10px] bg-gray-100 dark:bg-gray-700 text-gray-500 dark:text-gray-400 px-1.5 py-0.5 rounded">{video.scene}</span>
                            )}
                            <span className="text-[10px] text-gray-400 ml-auto">{video.model}</span>
                            {video.conversation_id && (
                              <button
                                onClick={() => navigate(`/chat/${video.conversation_id}`)}
                                className="flex items-center gap-0.5 px-1 py-0.5 bg-blue-50 dark:bg-blue-900/30 text-blue-500 rounded-full text-[9px] hover:bg-blue-100 dark:hover:bg-blue-900/50 transition-colors ml-1"
                                title="跳转到来源对话"
                              >
                                <MessageSquare className="w-2 h-2" />
                              </button>
                            )}
                          </div>
                          <p className="text-sm text-gray-700 dark:text-gray-300 line-clamp-2 leading-relaxed">{video.prompt || '(无描述)'}</p>
                          <div className="flex items-center justify-between mt-3 pt-3 border-t border-gray-100 dark:border-gray-700">
                            <span className="text-[10px] text-gray-400">{new Date(video.created_at).toLocaleString('zh-CN')}</span>
                            {renderActions(video, { showRegenerate: true })}
                          </div>
                        </div>
                      </div>
                    )
                  })}
                </div>
              </div>
            )}
          </>
        )}
      </div>

      {/* Custom Confirm Modal */}
      {confirmModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center">
          <div className="absolute inset-0 bg-black/40 backdrop-blur-sm" onClick={() => setConfirmModal(null)} />
          <div className="relative bg-white dark:bg-gray-800 rounded-2xl shadow-2xl w-[360px] mx-4 overflow-hidden animate-in fade-in zoom-in-95 duration-200">
            <div className="px-6 pt-6 pb-4">
              <h3 className="text-lg font-semibold text-gray-900 dark:text-white">{confirmModal.title}</h3>
              <p className="mt-2 text-sm text-gray-500 dark:text-gray-400 leading-relaxed">{confirmModal.message}</p>
            </div>
            <div className="flex gap-3 px-6 pb-6">
              <button
                onClick={() => setConfirmModal(null)}
                className="flex-1 px-4 py-2.5 rounded-xl text-sm font-medium text-gray-700 dark:text-gray-300 bg-gray-100 dark:bg-gray-700 hover:bg-gray-200 dark:hover:bg-gray-600 transition-colors"
              >
                取消
              </button>
              <button
                onClick={() => { confirmModal.onConfirm(); setConfirmModal(null) }}
                className="flex-1 px-4 py-2.5 rounded-xl text-sm font-medium text-white bg-violet-500 hover:bg-violet-600 transition-colors"
              >
                确定
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Dub Modal */}
      {dubModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center">
          <div className="absolute inset-0 bg-black/40 backdrop-blur-sm" onClick={() => !dubLoading && setDubModal(null)} />
          <div className="relative bg-white dark:bg-gray-800 rounded-2xl shadow-2xl w-[480px] mx-4 overflow-hidden">
            <div className="px-6 pt-6 pb-2">
              <div className="flex items-center gap-2 mb-1">
                <Volume2 className="w-5 h-5 text-blue-500" />
                <h3 className="text-lg font-semibold text-gray-900 dark:text-white">视频配音</h3>
              </div>
              <p className="text-xs text-gray-400">为视频添加语音旁白和字幕 · 时长 {dubModal.duration}秒</p>
            </div>

            <div className="px-6 py-4 space-y-4">
              {/* Narration text */}
              <div>
                <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1.5">配音文案</label>
                <textarea
                  value={dubText}
                  onChange={(e) => setDubText(e.target.value)}
                  rows={4}
                  className="w-full px-3 py-2 text-sm border border-gray-200 dark:border-gray-600 rounded-xl bg-gray-50 dark:bg-gray-700 text-gray-900 dark:text-white outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent resize-none"
                  placeholder="输入旁白文案，系统会自动按视频时长分配时间..."
                />
                <p className="mt-1 text-[11px] text-gray-400">{dubText.length} 字 · 约 {Math.max(1, Math.round(dubText.length / 4))} 秒语音</p>
              </div>

              {/* Voice selector */}
              <div>
                <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1.5">音色</label>
                <div className="grid grid-cols-2 gap-2">
                  {[
                    { id: 'longyuan', name: '龙媛', gender: '女', style: '温柔知性' },
                    { id: 'longxiaochun', name: '龙小淳', gender: '女', style: '活泼甜美' },
                    { id: 'longshu', name: '龙书', gender: '女', style: '故事旁白' },
                    { id: 'longwan', name: '龙婉', gender: '女', style: '端庄大气' },
                    { id: 'longhua', name: '龙华', gender: '男', style: '沉稳大方' },
                    { id: 'longjing', name: '龙靖', gender: '男', style: '播音腔' },
                    { id: 'longshuo', name: '龙硕', gender: '男', style: '年轻活力' },
                    { id: 'longfei', name: '龙飞', gender: '男', style: '浑厚低沉' },
                  ].map(v => (
                    <button
                      key={v.id}
                      onClick={() => setDubVoice(v.id)}
                      className={`flex items-center gap-2 px-3 py-2 rounded-lg text-left text-sm transition-colors border ${
                        dubVoice === v.id
                          ? 'border-blue-500 bg-blue-50 dark:bg-blue-900/30 text-blue-700 dark:text-blue-300'
                          : 'border-gray-200 dark:border-gray-600 hover:bg-gray-50 dark:hover:bg-gray-700 text-gray-700 dark:text-gray-300'
                      }`}
                    >
                      <span className={`text-[10px] px-1.5 py-0.5 rounded-full font-medium ${v.gender === '女' ? 'bg-pink-100 text-pink-600 dark:bg-pink-900/30 dark:text-pink-400' : 'bg-sky-100 text-sky-600 dark:bg-sky-900/30 dark:text-sky-400'}`}>{v.gender}</span>
                      <span className="font-medium">{v.name}</span>
                      <span className="text-[11px] text-gray-400 ml-auto">{v.style}</span>
                    </button>
                  ))}
                </div>
              </div>

              {/* Subtitle toggle */}
              <div className="flex items-center justify-between">
                <div>
                  <span className="text-sm font-medium text-gray-700 dark:text-gray-300">同时添加字幕</span>
                  <p className="text-[11px] text-gray-400">自动根据旁白生成同步字幕</p>
                </div>
                <button
                  onClick={() => setDubSubtitle(!dubSubtitle)}
                  className={`relative w-11 h-6 rounded-full transition-colors ${dubSubtitle ? 'bg-blue-500' : 'bg-gray-300 dark:bg-gray-600'}`}
                >
                  <span className={`absolute top-0.5 left-0.5 w-5 h-5 bg-white rounded-full shadow transition-transform ${dubSubtitle ? 'translate-x-5' : ''}`} />
                </button>
              </div>
            </div>

            <div className="flex gap-3 px-6 pb-6">
              <button
                onClick={() => setDubModal(null)}
                disabled={dubLoading}
                className="flex-1 px-4 py-2.5 rounded-xl text-sm font-medium text-gray-700 dark:text-gray-300 bg-gray-100 dark:bg-gray-700 hover:bg-gray-200 dark:hover:bg-gray-600 transition-colors disabled:opacity-50"
              >
                取消
              </button>
              <button
                onClick={handleDub}
                disabled={dubLoading || !dubText.trim()}
                className="flex-1 px-4 py-2.5 rounded-xl text-sm font-medium text-white bg-blue-500 hover:bg-blue-600 transition-colors disabled:opacity-50 flex items-center justify-center gap-2"
              >
                {dubLoading ? <><Loader2 className="w-4 h-4 animate-spin" />配音中...</> : <><Volume2 className="w-4 h-4" />开始配音</>}
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Music Selector Modal */}
      {musicModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center">
          <div className="absolute inset-0 bg-black/40 backdrop-blur-sm" onClick={() => !musicLoading && setMusicModal(null)} />
          <div className="relative bg-white dark:bg-gray-800 rounded-2xl shadow-2xl w-[480px] mx-4 overflow-hidden max-h-[80vh] flex flex-col">
            <div className="px-6 pt-6 pb-2">
              <div className="flex items-center gap-2 mb-1">
                <Music2 className="w-5 h-5 text-amber-500" />
                <h3 className="text-lg font-semibold text-gray-900 dark:text-white">视频配乐</h3>
              </div>
              <p className="text-xs text-gray-400">选择一首已生成的音乐，替换视频背景音</p>
            </div>

            <div className="px-6 py-4 overflow-y-auto flex-1 min-h-0">
              {musicList.length === 0 ? (
                <div className="text-center py-8 text-gray-400">
                  <Music2 className="w-10 h-10 mx-auto mb-2 opacity-30" />
                  <p className="text-sm">还没有已完成的音乐</p>
                  <p className="text-xs mt-1">在对话中让Agent生成音乐后，就能在这里选择配乐</p>
                </div>
              ) : (
                <div className="space-y-2">
                  {musicList.map(m => (
                    <button
                      key={m.id}
                      onClick={() => setSelectedMusicId(m.id)}
                      className={`w-full flex items-start gap-3 p-3 rounded-xl text-left transition-colors border ${
                        selectedMusicId === m.id
                          ? 'border-amber-500 bg-amber-50 dark:bg-amber-900/20'
                          : 'border-gray-200 dark:border-gray-600 hover:bg-gray-50 dark:hover:bg-gray-700'
                      }`}
                    >
                      <div className="flex-shrink-0 mt-0.5">
                        {selectedMusicId === m.id ? (
                          <div className="w-5 h-5 rounded-full bg-amber-500 flex items-center justify-center">
                            <CheckCircle2 className="w-3.5 h-3.5 text-white" />
                          </div>
                        ) : (
                          <div className="w-5 h-5 rounded-full border-2 border-gray-300 dark:border-gray-500" />
                        )}
                      </div>
                      <div className="flex-1 min-w-0">
                        <p className="text-sm font-medium text-gray-900 dark:text-white line-clamp-1">{m.prompt || '(无描述)'}</p>
                        <div className="flex items-center gap-2 mt-1">
                          <span className="text-[10px] px-1.5 py-0.5 rounded bg-gray-100 dark:bg-gray-700 text-gray-500">{m.model}</span>
                          <span className="text-[10px] text-gray-400">{m.duration}秒</span>
                          <span className="text-[10px] text-gray-400">{new Date(m.created_at).toLocaleDateString('zh-CN')}</span>
                        </div>
                        {m.lyrics && <p className="text-[11px] text-gray-400 mt-1 line-clamp-2">{m.lyrics.replace(/\[.*?\]/g, '').trim()}</p>}
                      </div>
                      {m.local_url && (
                        <audio src={m.local_url} controls className="flex-shrink-0 w-24 h-8" style={{ minWidth: '96px' }} />
                      )}
                    </button>
                  ))}
                </div>
              )}
            </div>

            <div className="flex gap-3 px-6 pb-6 pt-2">
              <button
                onClick={() => setMusicModal(null)}
                disabled={musicLoading}
                className="flex-1 px-4 py-2.5 rounded-xl text-sm font-medium text-gray-700 dark:text-gray-300 bg-gray-100 dark:bg-gray-700 hover:bg-gray-200 dark:hover:bg-gray-600 transition-colors disabled:opacity-50"
              >
                取消
              </button>
              <button
                onClick={handleAddMusic}
                disabled={musicLoading || !selectedMusicId}
                className="flex-1 px-4 py-2.5 rounded-xl text-sm font-medium text-white bg-amber-500 hover:bg-amber-600 transition-colors disabled:opacity-50 flex items-center justify-center gap-2"
              >
                {musicLoading ? <><Loader2 className="w-4 h-4 animate-spin" />合成中...</> : <><Music2 className="w-4 h-4" />开始配乐</>}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
