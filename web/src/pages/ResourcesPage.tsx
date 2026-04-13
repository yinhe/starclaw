import { useState, useEffect, useCallback } from 'react'
import { Film, Image, Music, FileText, Trash2, Download, Play, Pause, RefreshCw, Loader2, CheckCircle2, Clock, XCircle, Filter, ExternalLink, FileDown, FolderOpen, FolderLock, ChevronLeft, Lock, Unlock, ArrowRight } from 'lucide-react'
import { videoAPI, imageAPI, musicAPI, documentAPI, docFolderAPI } from '../lib/api'

type TabType = 'all' | 'videos' | 'images' | 'music' | 'documents'

interface VideoRecord {
  id: string
  task_id: string
  model: string
  prompt: string
  video_url: string
  narrated_url: string
  size: string
  duration: number
  scene: string
  status: string
  type: string
  conversation_id: string
  created_at: string
}

interface ImageRecord {
  id: string
  model: string
  prompt: string
  image_url: string
  local_url: string
  size: string
  status: string
  scene: string
  created_at: string
}

interface MusicRecord {
  id: string
  model: string
  prompt: string
  lyrics: string
  audio_url: string
  local_url: string
  duration: number
  status: string
  created_at: string
}

interface DocRecord {
  id?: string
  name: string
  path: string
  workspace: string
  size: number
  mod_time: string
  url: string
  category: string
  conversation_id?: string
  conv_title?: string
}

interface FolderInfo {
  conversation_id: string
  title: string
  file_count: number
  doc_count: number
  code_count: number
  total_size: number
  last_modified: string
  locked: boolean
}

const statusIcon = (status: string) => {
  switch (status) {
    case 'succeeded': return <CheckCircle2 className="w-3.5 h-3.5 text-green-500" />
    case 'running': return <Loader2 className="w-3.5 h-3.5 text-blue-500 animate-spin" />
    case 'pending': return <Clock className="w-3.5 h-3.5 text-yellow-500" />
    case 'failed': return <XCircle className="w-3.5 h-3.5 text-red-500" />
    default: return <Clock className="w-3.5 h-3.5 text-gray-400" />
  }
}

export default function ResourcesPage() {
  const [tab, setTab] = useState<TabType>('all')
  const [videos, setVideos] = useState<VideoRecord[]>([])
  const [images, setImages] = useState<ImageRecord[]>([])
  const [music, setMusic] = useState<MusicRecord[]>([])
  const [folders, setFolders] = useState<FolderInfo[]>([])
  const [totalFiles, setTotalFiles] = useState(0)
  const [openFolder, setOpenFolder] = useState<string | null>(null)
  const [openFolderTitle, setOpenFolderTitle] = useState('')
  const [openFolderLocked, setOpenFolderLocked] = useState(false)
  const [folderFiles, setFolderFiles] = useState<DocRecord[]>([])
  const [folderFilesTotal, setFolderFilesTotal] = useState(0)
  const [folderPage, setFolderPage] = useState(1)
  const [folderLoading, setFolderLoading] = useState(false)
  const [loading, setLoading] = useState(true)
  const [playingAudio, setPlayingAudio] = useState<string | null>(null)
  const [audioEl] = useState(() => new Audio())
  const [lightbox, setLightbox] = useState<string | null>(null)

  useEffect(() => { loadAll() }, [])

  const loadFolders = useCallback(async () => {
    try {
      const res = await docFolderAPI.listFolders()
      setFolders(res.data.folders || [])
      setTotalFiles(res.data.total_files || 0)
    } catch {}
  }, [])

  useEffect(() => { if (tab === 'documents' || tab === 'all') loadFolders() }, [tab, loadFolders])

  const enterFolder = async (convId: string, title: string) => {
    setOpenFolder(convId)
    setOpenFolderTitle(title)
    setFolderPage(1)
    setFolderLoading(true)
    try {
      const res = await docFolderAPI.listFiles(convId, 1, 50)
      setFolderFiles(res.data.files || [])
      setFolderFilesTotal(res.data.total || 0)
      setOpenFolderLocked(res.data.locked || false)
    } finally {
      setFolderLoading(false)
    }
  }

  const loadMoreFiles = async () => {
    if (!openFolder) return
    const nextPage = folderPage + 1
    setFolderLoading(true)
    try {
      const res = await docFolderAPI.listFiles(openFolder, nextPage, 50)
      setFolderFiles(prev => [...prev, ...(res.data.files || [])])
      setFolderPage(nextPage)
    } finally {
      setFolderLoading(false)
    }
  }

  const handleDeleteFolder = async (convId: string) => {
    if (!confirm('确定删除此文件夹及其所有文件？此操作不可恢复。')) return
    try {
      await docFolderAPI.deleteFolder(convId)
      setFolders(prev => prev.filter(f => f.conversation_id !== convId))
      if (openFolder === convId) { setOpenFolder(null); setFolderFiles([]) }
    } catch (e: any) {
      alert(e.response?.data?.error || '删除失败')
    }
  }

  const handleToggleLock = async (convId: string, currentlyLocked: boolean) => {
    try {
      if (currentlyLocked) {
        await docFolderAPI.unlockFolder(convId)
      } else {
        await docFolderAPI.lockFolder(convId)
      }
      setFolders(prev => prev.map(f => f.conversation_id === convId ? { ...f, locked: !currentlyLocked } : f))
      if (openFolder === convId) setOpenFolderLocked(!currentlyLocked)
    } catch {}
  }

  const handleDeleteDoc = async (doc: DocRecord) => {
    if (!confirm(`确定删除文件 ${doc.name}？`)) return
    await documentAPI.delete(doc.workspace, doc.path)
    setFolderFiles(prev => prev.filter(d => !(d.workspace === doc.workspace && d.path === doc.path)))
    setFolderFilesTotal(prev => prev - 1)
  }

  useEffect(() => {
    return () => { audioEl.pause() }
  }, [audioEl])

  const loadAll = async () => {
    setLoading(true)
    try {
      const [vRes, iRes, mRes] = await Promise.all([
        videoAPI.list().catch(() => ({ data: { videos: [] } })),
        imageAPI.list().catch(() => ({ data: { images: [] } })),
        musicAPI.list().catch(() => ({ data: { music: [] } })),
      ])
      setVideos(vRes.data.videos || [])
      setImages(iRes.data.images || [])
      setMusic(mRes.data.music || [])
    } finally {
      setLoading(false)
    }
  }

  const handleDeleteVideo = async (id: string) => {
    if (!confirm('确定删除此视频？')) return
    await videoAPI.delete(id)
    setVideos(prev => prev.filter(v => v.id !== id))
  }

  const handleDeleteImage = async (id: string) => {
    if (!confirm('确定删除此图片？')) return
    await imageAPI.delete(id)
    setImages(prev => prev.filter(i => i.id !== id))
  }

  const handleDeleteMusic = async (id: string) => {
    if (!confirm('确定删除此音乐？')) return
    await musicAPI.delete(id)
    setMusic(prev => prev.filter(m => m.id !== id))
  }


  const formatFileSize = (bytes: number) => {
    if (bytes < 1024) return bytes + ' B'
    if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB'
    return (bytes / (1024 * 1024)).toFixed(1) + ' MB'
  }

  const getFileIcon = (name: string) => {
    const ext = name.split('.').pop()?.toLowerCase()
    if (['html', 'htm'].includes(ext || '')) return '🌐'
    if (['md', 'txt'].includes(ext || '')) return '📄'
    if (['pdf'].includes(ext || '')) return '📕'
    if (['json', 'csv', 'xml'].includes(ext || '')) return '📊'
    if (['py', 'js', 'ts', 'go', 'java'].includes(ext || '')) return '💻'
    return '📄'
  }

  const playAudio = (url: string, id: string) => {
    if (playingAudio === id) {
      audioEl.pause()
      setPlayingAudio(null)
    } else {
      audioEl.src = url
      audioEl.play()
      setPlayingAudio(id)
      audioEl.onended = () => setPlayingAudio(null)
    }
  }

  const getVideoUrl = (v: VideoRecord) => v.narrated_url || v.video_url || ''
  const getImageUrl = (img: ImageRecord) => img.local_url || img.image_url || ''
  const getMusicUrl = (m: MusicRecord) => m.local_url || m.audio_url || ''

  // Hide intermediate merged videos when an MV exists in the same conversation (matches VideosPage logic)
  const mvConvIds = new Set(videos.filter(v => v.type === 'mv' && v.status === 'succeeded').map(v => v.conversation_id))
  const succeededVideos = videos.filter(v => {
    if (v.status !== 'succeeded') return false
    if (v.type === 'mv' || v.type === 'comic') return true
    if (v.type === 'merged') return !mvConvIds.has(v.conversation_id)
    return false
  })
  const succeededImages = images.filter(i => i.status === 'succeeded')
  const succeededMusic = music.filter(m => m.status === 'succeeded')

  const tabs: { key: TabType; label: string; icon: typeof Film; count: number }[] = [
    { key: 'all', label: '全部', icon: Filter, count: succeededVideos.length + succeededImages.length + succeededMusic.length + totalFiles },
    { key: 'videos', label: '视频', icon: Film, count: succeededVideos.length },
    { key: 'images', label: '图片', icon: Image, count: succeededImages.length },
    { key: 'music', label: '音乐', icon: Music, count: succeededMusic.length },
    { key: 'documents', label: '文档', icon: FileText, count: totalFiles },
  ]

  return (
    <div className="h-full overflow-y-auto">
      <div className="max-w-7xl mx-auto px-6 py-6">
        {/* Header */}
        <div className="flex items-center justify-between mb-6">
          <div>
            <h1 className="text-2xl font-bold text-gray-900 dark:text-white">资源中心</h1>
            <p className="text-sm text-gray-500 dark:text-gray-400 mt-1">管理所有 AI 创作的视频、图片、音乐和文档</p>
          </div>
          <button
            onClick={loadAll}
            className="flex items-center gap-2 px-3 py-2 text-sm bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg hover:bg-gray-50 dark:hover:bg-gray-750 transition-colors"
          >
            <RefreshCw className={`w-4 h-4 ${loading ? 'animate-spin' : ''}`} />
            刷新
          </button>
        </div>

        {/* Tabs */}
        <div className="flex gap-1 mb-6 bg-gray-100 dark:bg-gray-800 rounded-lg p-1">
          {tabs.map(t => (
            <button
              key={t.key}
              onClick={() => setTab(t.key)}
              className={`flex items-center gap-2 px-4 py-2 rounded-md text-sm font-medium transition-colors ${
                tab === t.key
                  ? 'bg-white dark:bg-gray-700 text-gray-900 dark:text-white shadow-sm'
                  : 'text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-300'
              }`}
            >
              <t.icon className="w-4 h-4" />
              {t.label}
              <span className={`text-xs px-1.5 py-0.5 rounded-full ${
                tab === t.key ? 'bg-violet-100 dark:bg-violet-900/30 text-violet-600 dark:text-violet-400' : 'bg-gray-200 dark:bg-gray-700 text-gray-500'
              }`}>{t.count}</span>
            </button>
          ))}
        </div>

        {loading ? (
          <div className="flex items-center justify-center py-20">
            <Loader2 className="w-8 h-8 animate-spin text-violet-500" />
          </div>
        ) : (
          <div className="space-y-8">
            {/* Videos */}
            {(tab === 'all' || tab === 'videos') && succeededVideos.length > 0 && (
              <section>
                {tab === 'all' && <h2 className="text-lg font-semibold text-gray-900 dark:text-white mb-4 flex items-center gap-2"><Film className="w-5 h-5 text-violet-500" /> 视频 ({succeededVideos.length})</h2>}
                <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4">
                  {succeededVideos.map(v => (
                    <div key={v.id} className="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 overflow-hidden group hover:shadow-lg transition-shadow">
                      <div className="aspect-video bg-black relative">
                        <video
                          src={getVideoUrl(v)}
                          className="w-full h-full object-contain"
                          controls
                          preload="metadata"
                        />
                      </div>
                      <div className="p-3">
                        <p className="text-sm text-gray-700 dark:text-gray-300 line-clamp-2">{v.prompt || v.scene || '未命名视频'}</p>
                        <div className="flex items-center justify-between mt-2">
                          <div className="flex items-center gap-2 text-xs text-gray-400">
                            {statusIcon(v.status)}
                            <span>{v.type || 'clip'}</span>
                            <span>{v.duration}s</span>
                          </div>
                          <div className="flex items-center gap-1">
                            {getVideoUrl(v) && (
                              <a href={getVideoUrl(v)} download className="p-1.5 rounded hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors">
                                <Download className="w-3.5 h-3.5 text-gray-400" />
                              </a>
                            )}
                            <button onClick={() => handleDeleteVideo(v.id)} className="p-1.5 rounded hover:bg-red-50 dark:hover:bg-red-900/20 transition-colors">
                              <Trash2 className="w-3.5 h-3.5 text-gray-400 hover:text-red-500" />
                            </button>
                          </div>
                        </div>
                        <p className="text-[10px] text-gray-400 mt-1">{new Date(v.created_at).toLocaleString('zh-CN')}</p>
                      </div>
                    </div>
                  ))}
                </div>
              </section>
            )}

            {/* Images */}
            {(tab === 'all' || tab === 'images') && succeededImages.length > 0 && (
              <section>
                {tab === 'all' && <h2 className="text-lg font-semibold text-gray-900 dark:text-white mb-4 flex items-center gap-2"><Image className="w-5 h-5 text-blue-500" /> 图片 ({succeededImages.length})</h2>}
                <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-4 xl:grid-cols-6 gap-3">
                  {succeededImages.map(img => (
                    <div key={img.id} className="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 overflow-hidden group hover:shadow-lg transition-shadow">
                      <div
                        className="aspect-square bg-gray-100 dark:bg-gray-900 cursor-pointer relative"
                        onClick={() => setLightbox(getImageUrl(img))}
                      >
                        <img
                          src={getImageUrl(img)}
                          alt={img.prompt}
                          className="w-full h-full object-cover"
                          loading="lazy"
                        />
                      </div>
                      <div className="p-2">
                        <p className="text-xs text-gray-600 dark:text-gray-400 line-clamp-2">{img.prompt || '未命名图片'}</p>
                        <div className="flex items-center justify-between mt-1.5">
                          <span className="text-[10px] text-gray-400">{img.model} · {img.size}</span>
                          <div className="flex items-center gap-1">
                            {getImageUrl(img) && (
                              <a href={getImageUrl(img)} download className="p-1 rounded hover:bg-gray-100 dark:hover:bg-gray-700">
                                <Download className="w-3 h-3 text-gray-400" />
                              </a>
                            )}
                            <button onClick={() => handleDeleteImage(img.id)} className="p-1 rounded hover:bg-red-50 dark:hover:bg-red-900/20">
                              <Trash2 className="w-3 h-3 text-gray-400 hover:text-red-500" />
                            </button>
                          </div>
                        </div>
                      </div>
                    </div>
                  ))}
                </div>
              </section>
            )}

            {/* Music */}
            {(tab === 'all' || tab === 'music') && succeededMusic.length > 0 && (
              <section>
                {tab === 'all' && <h2 className="text-lg font-semibold text-gray-900 dark:text-white mb-4 flex items-center gap-2"><Music className="w-5 h-5 text-pink-500" /> 音乐 ({succeededMusic.length})</h2>}
                <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3">
                  {succeededMusic.map(m => (
                    <div key={m.id} className="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-4 flex items-start gap-3 hover:shadow-lg transition-shadow">
                      <button
                        onClick={() => playAudio(getMusicUrl(m), m.id)}
                        className="flex-none w-10 h-10 rounded-full bg-pink-100 dark:bg-pink-900/30 flex items-center justify-center hover:bg-pink-200 dark:hover:bg-pink-900/50 transition-colors"
                      >
                        {playingAudio === m.id
                          ? <Pause className="w-4 h-4 text-pink-600" />
                          : <Play className="w-4 h-4 text-pink-600 ml-0.5" />
                        }
                      </button>
                      <div className="flex-1 min-w-0">
                        <p className="text-sm font-medium text-gray-900 dark:text-white line-clamp-1">{m.prompt || '未命名音乐'}</p>
                        <p className="text-xs text-gray-400 mt-0.5">{m.model} · {m.duration}s</p>
                        {m.lyrics && <p className="text-xs text-gray-500 dark:text-gray-400 mt-1 line-clamp-2">{m.lyrics.slice(0, 80)}...</p>}
                        <p className="text-[10px] text-gray-400 mt-1">{new Date(m.created_at).toLocaleString('zh-CN')}</p>
                      </div>
                      <div className="flex items-center gap-1 flex-none">
                        {getMusicUrl(m) && (
                          <a href={getMusicUrl(m)} download className="p-1.5 rounded hover:bg-gray-100 dark:hover:bg-gray-700">
                            <Download className="w-3.5 h-3.5 text-gray-400" />
                          </a>
                        )}
                        <button onClick={() => handleDeleteMusic(m.id)} className="p-1.5 rounded hover:bg-red-50 dark:hover:bg-red-900/20">
                          <Trash2 className="w-3.5 h-3.5 text-gray-400 hover:text-red-500" />
                        </button>
                      </div>
                    </div>
                  ))}
                </div>
              </section>
            )}

            {/* Documents - folder browsing mode */}
            {(tab === 'all' || tab === 'documents') && folders.length > 0 && (
              <section className="space-y-4">
                {tab === 'all' && !openFolder && <h2 className="text-lg font-semibold text-gray-900 dark:text-white mb-4 flex items-center gap-2"><FileText className="w-5 h-5 text-emerald-500" /> 文档 ({totalFiles})</h2>}

                {/* Breadcrumb */}
                {openFolder && (
                  <div className="flex items-center gap-2 text-sm">
                    <button onClick={() => { setOpenFolder(null); setFolderFiles([]) }} className="flex items-center gap-1 text-violet-600 dark:text-violet-400 hover:underline">
                      <ChevronLeft className="w-4 h-4" /> 返回文件夹列表
                    </button>
                    <span className="text-gray-400">/</span>
                    <span className="text-gray-900 dark:text-white font-medium flex items-center gap-1.5">
                      {openFolderLocked && <Lock className="w-3.5 h-3.5 text-amber-500" />}
                      {openFolderTitle}
                    </span>
                    <span className="text-xs text-gray-400">({folderFilesTotal} 个文件)</span>
                    <div className="ml-auto flex items-center gap-1">
                      <button onClick={() => handleToggleLock(openFolder, openFolderLocked)} className={`p-1.5 rounded text-xs flex items-center gap-1 transition-colors ${openFolderLocked ? 'bg-amber-50 dark:bg-amber-900/20 text-amber-600' : 'bg-gray-100 dark:bg-gray-800 text-gray-500 hover:text-gray-700'}`} title={openFolderLocked ? '点击解锁' : '点击锁定'}>
                        {openFolderLocked ? <><Unlock className="w-3.5 h-3.5" /> 解锁</> : <><Lock className="w-3.5 h-3.5" /> 锁定</>}
                      </button>
                      {!openFolderLocked && (
                        <button onClick={() => handleDeleteFolder(openFolder)} className="p-1.5 rounded bg-red-50 dark:bg-red-900/20 text-red-500 text-xs flex items-center gap-1 hover:bg-red-100 dark:hover:bg-red-900/30 transition-colors">
                          <Trash2 className="w-3.5 h-3.5" /> 删除文件夹
                        </button>
                      )}
                    </div>
                  </div>
                )}

                {/* Folder list */}
                {!openFolder && (
                  <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3">
                    {folders.map(f => (
                      <div key={f.conversation_id} className="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-4 hover:shadow-lg transition-shadow group">
                        <div className="flex items-start gap-3">
                          <div className={`flex-none w-10 h-10 rounded-lg flex items-center justify-center ${f.locked ? 'bg-amber-50 dark:bg-amber-900/20' : 'bg-violet-50 dark:bg-violet-900/20'}`}>
                            {f.locked ? <FolderLock className="w-5 h-5 text-amber-500" /> : <FolderOpen className="w-5 h-5 text-violet-500" />}
                          </div>
                          <div className="flex-1 min-w-0 cursor-pointer" onClick={() => enterFolder(f.conversation_id, f.title)}>
                            <p className="text-sm font-medium text-gray-900 dark:text-white truncate group-hover:text-violet-600 dark:group-hover:text-violet-400 transition-colors" title={f.title}>{f.title}</p>
                            <div className="flex items-center gap-2 mt-1 text-xs text-gray-400">
                              <span>{f.file_count} 个文件</span>
                              <span>·</span>
                              <span>{formatFileSize(f.total_size)}</span>
                              {f.doc_count > 0 && <span className="px-1 py-0.5 rounded bg-emerald-50 dark:bg-emerald-900/20 text-emerald-600 dark:text-emerald-400">📄 {f.doc_count}</span>}
                              {f.code_count > 0 && <span className="px-1 py-0.5 rounded bg-blue-50 dark:bg-blue-900/20 text-blue-600 dark:text-blue-400">💻 {f.code_count}</span>}
                            </div>
                            <p className="text-[10px] text-gray-400 mt-1">{f.last_modified}</p>
                          </div>
                          <div className="flex items-center gap-0.5 flex-none opacity-0 group-hover:opacity-100 transition-opacity">
                            <button onClick={(e) => { e.stopPropagation(); handleToggleLock(f.conversation_id, f.locked) }} className="p-1.5 rounded hover:bg-gray-100 dark:hover:bg-gray-700" title={f.locked ? '解锁' : '锁定'}>
                              {f.locked ? <Unlock className="w-3.5 h-3.5 text-amber-500" /> : <Lock className="w-3.5 h-3.5 text-gray-400" />}
                            </button>
                            {!f.locked && (
                              <button onClick={(e) => { e.stopPropagation(); handleDeleteFolder(f.conversation_id) }} className="p-1.5 rounded hover:bg-red-50 dark:hover:bg-red-900/20">
                                <Trash2 className="w-3.5 h-3.5 text-gray-400 hover:text-red-500" />
                              </button>
                            )}
                            <button onClick={() => enterFolder(f.conversation_id, f.title)} className="p-1.5 rounded hover:bg-gray-100 dark:hover:bg-gray-700">
                              <ArrowRight className="w-3.5 h-3.5 text-gray-400" />
                            </button>
                          </div>
                        </div>
                      </div>
                    ))}
                  </div>
                )}

                {/* File list inside folder */}
                {openFolder && (
                  <div className="space-y-2">
                    {folderLoading && folderFiles.length === 0 ? (
                      <div className="flex items-center justify-center py-12"><Loader2 className="w-6 h-6 animate-spin text-violet-500" /></div>
                    ) : (
                      <>
                        {folderFiles.map((doc, idx) => (
                          <div key={`${doc.workspace}-${doc.path}-${idx}`} className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 px-4 py-3 flex items-center gap-3 hover:shadow-md transition-shadow">
                            <div className={`flex-none w-9 h-9 rounded-lg flex items-center justify-center text-base ${doc.category === 'code' ? 'bg-blue-50 dark:bg-blue-900/20' : 'bg-emerald-50 dark:bg-emerald-900/20'}`}>
                              {getFileIcon(doc.name)}
                            </div>
                            <div className="flex-1 min-w-0">
                              <p className="text-sm font-medium text-gray-900 dark:text-white truncate" title={doc.name}>{doc.name}</p>
                              <span className="text-xs text-gray-400">{formatFileSize(doc.size)} · {doc.mod_time}</span>
                            </div>
                            <div className="flex items-center gap-0.5 flex-none">
                              <a href={`/v1/preview/${doc.workspace}/${doc.path}`} target="_blank" rel="noopener noreferrer" className="p-1.5 rounded hover:bg-gray-100 dark:hover:bg-gray-700" title="预览"><ExternalLink className="w-3.5 h-3.5 text-gray-400" /></a>
                              {/\.(html?|htm|md|txt)$/i.test(doc.name) && (
                                <a href={`/v1/pdf/${doc.workspace}/${doc.path}`} className="p-1.5 rounded hover:bg-emerald-50 dark:hover:bg-emerald-900/20" title="下载 PDF"><FileDown className="w-3.5 h-3.5 text-emerald-500" /></a>
                              )}
                              <a href={`/v1/preview/${doc.workspace}/${doc.path}`} download className="p-1.5 rounded hover:bg-gray-100 dark:hover:bg-gray-700" title="下载"><Download className="w-3.5 h-3.5 text-gray-400" /></a>
                              {!openFolderLocked && (
                                <button onClick={() => handleDeleteDoc(doc)} className="p-1.5 rounded hover:bg-red-50 dark:hover:bg-red-900/20" title="删除"><Trash2 className="w-3.5 h-3.5 text-gray-400 hover:text-red-500" /></button>
                              )}
                            </div>
                          </div>
                        ))}
                        {folderFiles.length < folderFilesTotal && (
                          <button onClick={loadMoreFiles} disabled={folderLoading} className="w-full py-3 text-sm text-violet-600 dark:text-violet-400 bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 hover:bg-violet-50 dark:hover:bg-violet-900/10 transition-colors flex items-center justify-center gap-2">
                            {folderLoading ? <Loader2 className="w-4 h-4 animate-spin" /> : null}
                            加载更多 ({folderFiles.length}/{folderFilesTotal})
                          </button>
                        )}
                        {folderFiles.length === 0 && !folderLoading && (
                          <div className="text-center py-8 text-gray-400 text-sm">此文件夹暂无文件</div>
                        )}
                      </>
                    )}
                  </div>
                )}
              </section>
            )}

            {/* Empty state */}
            {succeededVideos.length === 0 && succeededImages.length === 0 && succeededMusic.length === 0 && folders.length === 0 && (
              <div className="text-center py-20">
                <div className="w-16 h-16 mx-auto mb-4 rounded-full bg-gray-100 dark:bg-gray-800 flex items-center justify-center">
                  <Film className="w-8 h-8 text-gray-300 dark:text-gray-600" />
                </div>
                <p className="text-gray-500 dark:text-gray-400">还没有创作任何资源</p>
                <p className="text-sm text-gray-400 dark:text-gray-500 mt-1">通过对话让 AI 帮你生成视频、图片、音乐和文档</p>
              </div>
            )}
          </div>
        )}
      </div>

      {/* Image lightbox */}
      {lightbox && (
        <div
          className="fixed inset-0 z-50 bg-black/80 flex items-center justify-center cursor-pointer"
          onClick={() => setLightbox(null)}
        >
          <img src={lightbox} alt="" className="max-w-[90vw] max-h-[90vh] object-contain rounded-lg" />
        </div>
      )}
    </div>
  )
}
