import { useState, useEffect, useRef } from 'react'
import { BookOpen, Plus, Trash2, Upload, FileText, Search, X, Loader2, ChevronLeft } from 'lucide-react'
import { knowledgeBaseAPI } from '../lib/api'

interface KnowledgeBase {
  id: string
  name: string
  description: string
  embedding_model: string
  document_count: number
  updated_at: string
}

interface Document {
  id: string
  name: string
  status: string
  chunk_count: number
  size: number
  created_at: string
}

interface SearchResult {
  chunk_id: string
  content: string
  score: number
}

export default function KnowledgeBasePage() {
  const [kbs, setKBs] = useState<KnowledgeBase[]>([])
  const [selectedKB, setSelectedKB] = useState<KnowledgeBase | null>(null)
  const [documents, setDocuments] = useState<Document[]>([])
  const [showCreateModal, setShowCreateModal] = useState(false)
  const [showTextModal, setShowTextModal] = useState(false)
  const [uploading, setUploading] = useState(false)
  const [searchQuery, setSearchQuery] = useState('')
  const [searchResults, setSearchResults] = useState<SearchResult[]>([])
  const [searching, setSearching] = useState(false)
  const fileInputRef = useRef<HTMLInputElement>(null)

  const [createForm, setCreateForm] = useState({ name: '', description: '', chunk_size: 500, chunk_overlap: 50 })
  const [textForm, setTextForm] = useState({ name: '', content: '' })

  useEffect(() => { loadKBs() }, [])

  const loadKBs = async () => {
    try {
      const res = await knowledgeBaseAPI.list()
      setKBs(res.data.knowledge_bases || [])
    } catch { /* ignore */ }
  }

  const loadKBDetail = async (kb: KnowledgeBase) => {
    setSelectedKB(kb)
    setSearchResults([])
    setSearchQuery('')
    try {
      const res = await knowledgeBaseAPI.get(kb.id)
      setDocuments(res.data.documents || [])
    } catch { /* ignore */ }
  }

  const handleCreateKB = async () => {
    try {
      await knowledgeBaseAPI.create(createForm)
      setShowCreateModal(false)
      setCreateForm({ name: '', description: '', chunk_size: 500, chunk_overlap: 50 })
      loadKBs()
    } catch { /* ignore */ }
  }

  const handleDeleteKB = async (id: string) => {
    if (!confirm('确定要删除这个知识库及其所有文档吗？')) return
    try {
      await knowledgeBaseAPI.delete(id)
      if (selectedKB?.id === id) {
        setSelectedKB(null)
        setDocuments([])
      }
      loadKBs()
    } catch { /* ignore */ }
  }

  const handleFileUpload = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (!file || !selectedKB) return
    setUploading(true)
    try {
      await knowledgeBaseAPI.uploadFile(selectedKB.id, file)
      setTimeout(() => loadKBDetail(selectedKB), 1000)
    } catch { /* ignore */ }
    setUploading(false)
    if (fileInputRef.current) fileInputRef.current.value = ''
  }

  const handleTextUpload = async () => {
    if (!selectedKB || !textForm.name || !textForm.content) return
    setUploading(true)
    try {
      await knowledgeBaseAPI.uploadText(selectedKB.id, textForm)
      setShowTextModal(false)
      setTextForm({ name: '', content: '' })
      setTimeout(() => loadKBDetail(selectedKB), 1000)
    } catch { /* ignore */ }
    setUploading(false)
  }

  const handleDeleteDoc = async (docId: string) => {
    if (!selectedKB) return
    try {
      await knowledgeBaseAPI.deleteDocument(selectedKB.id, docId)
      loadKBDetail(selectedKB)
    } catch { /* ignore */ }
  }

  const handleSearch = async () => {
    if (!selectedKB || !searchQuery.trim()) return
    setSearching(true)
    try {
      const res = await knowledgeBaseAPI.search(selectedKB.id, { query: searchQuery, top_k: 5 })
      setSearchResults(res.data.results || [])
    } catch { /* ignore */ }
    setSearching(false)
  }

  const formatSize = (bytes: number) => {
    if (bytes < 1024) return `${bytes} B`
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
    return `${(bytes / 1024 / 1024).toFixed(1)} MB`
  }

  // --- Detail View ---
  if (selectedKB) {
    return (
      <div className="h-full overflow-y-auto">
        <div className="max-w-5xl mx-auto p-8">
          <button
            onClick={() => { setSelectedKB(null); setDocuments([]) }}
            className="flex items-center gap-1 text-sm text-gray-500 hover:text-gray-700 mb-4"
          >
            <ChevronLeft className="w-4 h-4" /> 返回列表
          </button>

          <div className="flex items-center justify-between mb-6">
            <div>
              <h1 className="text-2xl font-bold text-gray-900">{selectedKB.name}</h1>
              <p className="text-gray-500 text-sm mt-1">{selectedKB.description || '暂无描述'}</p>
            </div>
            <div className="flex items-center gap-2">
              <button
                onClick={() => setShowTextModal(true)}
                className="flex items-center gap-1.5 px-3 py-1.5 text-sm border rounded-lg hover:bg-gray-50"
              >
                <FileText className="w-4 h-4" /> 添加文本
              </button>
              <label className="flex items-center gap-1.5 px-3 py-1.5 text-sm bg-primary-600 text-white rounded-lg hover:bg-primary-700 cursor-pointer">
                {uploading ? <Loader2 className="w-4 h-4 animate-spin" /> : <Upload className="w-4 h-4" />}
                上传文件
                <input
                  ref={fileInputRef}
                  type="file"
                  accept=".txt,.md,.csv,.json,.xml,.yaml,.yml,.py,.go,.js,.ts,.html"
                  onChange={handleFileUpload}
                  className="hidden"
                />
              </label>
            </div>
          </div>

          {/* Search */}
          <div className="bg-white border rounded-xl p-4 mb-6">
            <div className="flex items-center gap-2">
              <Search className="w-4 h-4 text-gray-400" />
              <input
                value={searchQuery}
                onChange={(e) => setSearchQuery(e.target.value)}
                onKeyDown={(e) => e.key === 'Enter' && handleSearch()}
                placeholder="搜索知识库内容..."
                className="flex-1 text-sm outline-none"
              />
              <button
                onClick={handleSearch}
                disabled={searching}
                className="px-3 py-1 text-sm bg-primary-600 text-white rounded-lg hover:bg-primary-700 disabled:opacity-50"
              >
                {searching ? '搜索中...' : '搜索'}
              </button>
            </div>

            {searchResults.length > 0 && (
              <div className="mt-4 space-y-3">
                <p className="text-xs font-medium text-gray-500">搜索结果 ({searchResults.length})</p>
                {searchResults.map((r, i) => (
                  <div key={r.chunk_id} className="p-3 bg-gray-50 rounded-lg text-sm">
                    <div className="flex items-center justify-between mb-1">
                      <span className="text-xs text-gray-400">#{i + 1}</span>
                      <span className="text-xs text-primary-600">相似度: {(r.score * 100).toFixed(1)}%</span>
                    </div>
                    <p className="text-gray-700 whitespace-pre-wrap line-clamp-4">{r.content}</p>
                  </div>
                ))}
              </div>
            )}
          </div>

          {/* Documents */}
          <h3 className="text-sm font-semibold text-gray-700 mb-3">文档 ({documents.length})</h3>
          {documents.length === 0 ? (
            <div className="text-center py-12 text-gray-400">
              <FileText className="w-10 h-10 mx-auto mb-2 opacity-50" />
              <p className="text-sm">暂无文档，上传文件或添加文本开始</p>
            </div>
          ) : (
            <div className="space-y-2">
              {documents.map((doc) => (
                <div key={doc.id} className="bg-white border rounded-lg px-4 py-3 flex items-center justify-between group">
                  <div className="flex items-center gap-3">
                    <FileText className="w-5 h-5 text-gray-400" />
                    <div>
                      <p className="text-sm font-medium text-gray-800">{doc.name}</p>
                      <div className="flex items-center gap-3 text-xs text-gray-400 mt-0.5">
                        <span>{formatSize(doc.size)}</span>
                        <span>{doc.chunk_count} 分块</span>
                        <span className={`px-1.5 py-0.5 rounded text-xs ${
                          doc.status === 'ready' ? 'bg-green-50 text-green-600' :
                          doc.status === 'processing' ? 'bg-yellow-50 text-yellow-600' :
                          doc.status === 'error' ? 'bg-red-50 text-red-600' :
                          'bg-gray-50 text-gray-500'
                        }`}>
                          {doc.status === 'ready' ? '就绪' : doc.status === 'processing' ? '处理中' : doc.status === 'error' ? '错误' : '等待中'}
                        </span>
                      </div>
                    </div>
                  </div>
                  <button
                    onClick={() => handleDeleteDoc(doc.id)}
                    className="p-1.5 text-gray-400 hover:text-red-500 rounded-md hover:bg-red-50 opacity-0 group-hover:opacity-100 transition-opacity"
                  >
                    <Trash2 className="w-3.5 h-3.5" />
                  </button>
                </div>
              ))}
            </div>
          )}
        </div>

        {/* Text Upload Modal */}
        {showTextModal && (
          <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
            <div className="bg-white rounded-2xl w-full max-w-lg mx-4 p-6">
              <div className="flex items-center justify-between mb-4">
                <h2 className="text-lg font-semibold">添加文本</h2>
                <button onClick={() => setShowTextModal(false)} className="text-gray-400 hover:text-gray-600">
                  <X className="w-5 h-5" />
                </button>
              </div>
              <div className="space-y-4">
                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-1">名称</label>
                  <input
                    value={textForm.name}
                    onChange={(e) => setTextForm({ ...textForm, name: e.target.value })}
                    className="w-full px-3 py-2 border rounded-lg text-sm outline-none focus:ring-2 focus:ring-primary-500"
                    placeholder="文档名称"
                  />
                </div>
                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-1">内容</label>
                  <textarea
                    value={textForm.content}
                    onChange={(e) => setTextForm({ ...textForm, content: e.target.value })}
                    rows={10}
                    className="w-full px-3 py-2 border rounded-lg text-sm outline-none focus:ring-2 focus:ring-primary-500 resize-none"
                    placeholder="粘贴或输入文本内容..."
                  />
                </div>
              </div>
              <div className="flex justify-end gap-3 mt-6">
                <button onClick={() => setShowTextModal(false)} className="px-4 py-2 text-sm text-gray-600 hover:bg-gray-100 rounded-lg">取消</button>
                <button
                  onClick={handleTextUpload}
                  disabled={!textForm.name || !textForm.content || uploading}
                  className="px-4 py-2 text-sm bg-primary-600 text-white rounded-lg hover:bg-primary-700 disabled:opacity-50"
                >
                  {uploading ? '处理中...' : '添加'}
                </button>
              </div>
            </div>
          </div>
        )}
      </div>
    )
  }

  // --- List View ---
  return (
    <div className="h-full overflow-y-auto">
      <div className="max-w-5xl mx-auto p-8">
        <div className="flex items-center justify-between mb-8">
          <div>
            <h1 className="text-2xl font-bold text-gray-900">知识库</h1>
            <p className="text-gray-500 text-sm mt-1">管理 RAG 知识库，为 Agent 提供参考资料</p>
          </div>
          <button
            onClick={() => setShowCreateModal(true)}
            className="flex items-center gap-2 px-4 py-2 bg-primary-600 text-white rounded-lg text-sm hover:bg-primary-700"
          >
            <Plus className="w-4 h-4" /> 创建知识库
          </button>
        </div>

        {kbs.length === 0 ? (
          <div className="text-center py-20 text-gray-400">
            <BookOpen className="w-12 h-12 mx-auto mb-3 opacity-50" />
            <p>还没有知识库，点击上方按钮创建</p>
          </div>
        ) : (
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
            {kbs.map((kb) => (
              <div
                key={kb.id}
                className="bg-white border rounded-xl p-5 hover:shadow-md transition-shadow cursor-pointer group"
                onClick={() => loadKBDetail(kb)}
              >
                <div className="flex items-start justify-between mb-3">
                  <div className="w-10 h-10 bg-blue-100 rounded-lg flex items-center justify-center">
                    <BookOpen className="w-5 h-5 text-blue-600" />
                  </div>
                  <button
                    onClick={(e) => { e.stopPropagation(); handleDeleteKB(kb.id) }}
                    className="p-1.5 text-gray-400 hover:text-red-500 rounded-md hover:bg-red-50 opacity-0 group-hover:opacity-100 transition-opacity"
                  >
                    <Trash2 className="w-3.5 h-3.5" />
                  </button>
                </div>
                <h3 className="font-semibold text-gray-900">{kb.name}</h3>
                <p className="text-sm text-gray-500 mt-1 line-clamp-2">{kb.description || '暂无描述'}</p>
                <div className="flex items-center gap-3 mt-3 text-xs text-gray-400">
                  <span>{kb.document_count} 文档</span>
                  <span>{kb.embedding_model}</span>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>

      {/* Create KB Modal */}
      {showCreateModal && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
          <div className="bg-white rounded-2xl w-full max-w-lg mx-4 p-6">
            <div className="flex items-center justify-between mb-4">
              <h2 className="text-lg font-semibold">创建知识库</h2>
              <button onClick={() => setShowCreateModal(false)} className="text-gray-400 hover:text-gray-600">
                <X className="w-5 h-5" />
              </button>
            </div>
            <div className="space-y-4">
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">名称</label>
                <input
                  value={createForm.name}
                  onChange={(e) => setCreateForm({ ...createForm, name: e.target.value })}
                  className="w-full px-3 py-2 border rounded-lg text-sm outline-none focus:ring-2 focus:ring-primary-500"
                  placeholder="例如：产品文档"
                />
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">描述</label>
                <input
                  value={createForm.description}
                  onChange={(e) => setCreateForm({ ...createForm, description: e.target.value })}
                  className="w-full px-3 py-2 border rounded-lg text-sm outline-none focus:ring-2 focus:ring-primary-500"
                  placeholder="简要描述知识库的内容"
                />
              </div>
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-1">分块大小</label>
                  <input
                    type="number"
                    value={createForm.chunk_size}
                    onChange={(e) => setCreateForm({ ...createForm, chunk_size: parseInt(e.target.value) || 500 })}
                    className="w-full px-3 py-2 border rounded-lg text-sm outline-none focus:ring-2 focus:ring-primary-500"
                  />
                </div>
                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-1">分块重叠</label>
                  <input
                    type="number"
                    value={createForm.chunk_overlap}
                    onChange={(e) => setCreateForm({ ...createForm, chunk_overlap: parseInt(e.target.value) || 50 })}
                    className="w-full px-3 py-2 border rounded-lg text-sm outline-none focus:ring-2 focus:ring-primary-500"
                  />
                </div>
              </div>
            </div>
            <div className="flex justify-end gap-3 mt-6">
              <button onClick={() => setShowCreateModal(false)} className="px-4 py-2 text-sm text-gray-600 hover:bg-gray-100 rounded-lg">取消</button>
              <button
                onClick={handleCreateKB}
                disabled={!createForm.name}
                className="px-4 py-2 text-sm bg-primary-600 text-white rounded-lg hover:bg-primary-700 disabled:opacity-50"
              >
                创建
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
