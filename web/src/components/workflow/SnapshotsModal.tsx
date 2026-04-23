import { useEffect, useRef, useState } from 'react'
import { X, Download, Trash2, RotateCcw, Upload, Camera, Edit2 } from 'lucide-react'
import {
  listSnapshots,
  saveSnapshot,
  deleteSnapshot,
  renameSnapshot,
  exportSnapshotAsFile,
  importSnapshotFile,
  type WorkflowSnapshot,
} from './snapshots'
import type { Node, Edge } from '@xyflow/react'

interface Props {
  open: boolean
  onClose: () => void
  // 当前画布状态（用于"存为快照"）
  current: {
    nodes: Node[]
    edges: Edge[]
    workflowName: string
    counter: number
    workflowId: string | null
  }
  // 恢复时由父组件写入画布
  onRestore: (snap: WorkflowSnapshot) => void
}

export default function SnapshotsModal({ open, onClose, current, onRestore }: Props) {
  const [list, setList] = useState<WorkflowSnapshot[]>([])
  const [snapName, setSnapName] = useState('')
  const [renamingId, setRenamingId] = useState<string | null>(null)
  const [renameValue, setRenameValue] = useState('')
  const fileRef = useRef<HTMLInputElement>(null)

  const refresh = () => setList(listSnapshots())

  useEffect(() => {
    if (open) {
      refresh()
      setSnapName(`${current.workflowName} · ${new Date().toLocaleString('zh-CN', { hour12: false })}`)
    }
  }, [open, current.workflowName])

  if (!open) return null

  const handleSaveSnapshot = () => {
    try {
      saveSnapshot({
        name: snapName.trim() || `快照 ${new Date().toLocaleString('zh-CN')}`,
        workflowId: current.workflowId,
        nodes: current.nodes,
        edges: current.edges,
        workflowName: current.workflowName,
        counter: current.counter,
      })
      refresh()
      setSnapName(`${current.workflowName} · ${new Date().toLocaleString('zh-CN', { hour12: false })}`)
    } catch (e) {
      alert(`存档失败：${e instanceof Error ? e.message : String(e)}`)
    }
  }

  const handleImport = async (file: File) => {
    try {
      const snap = await importSnapshotFile(file)
      // 保存一份到本地列表，同时询问是否立刻恢复
      saveSnapshot({
        name: `[导入] ${snap.name}`,
        workflowId: snap.workflowId,
        nodes: snap.data.nodes,
        edges: snap.data.edges,
        workflowName: snap.data.workflowName,
        counter: snap.data.counter,
      })
      refresh()
      if (confirm(`已导入快照「${snap.name}」，是否立即恢复到当前画布？\n（当前未存档的修改会丢失）`)) {
        onRestore(snap)
        onClose()
      }
    } catch (e) {
      alert(`导入失败：${e instanceof Error ? e.message : String(e)}`)
    }
  }

  const handleRename = (id: string) => {
    if (!renameValue.trim()) { setRenamingId(null); return }
    renameSnapshot(id, renameValue.trim())
    setRenamingId(null)
    refresh()
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/70 backdrop-blur-sm">
      <div className="bg-gray-900 border border-gray-700 rounded-xl w-[720px] max-h-[85vh] flex flex-col shadow-2xl">
        <div className="flex items-center justify-between px-5 py-3 border-b border-gray-700">
          <div className="flex items-center gap-2">
            <Camera size={18} className="text-purple-400" />
            <h2 className="text-lg font-semibold text-white">工作流存档</h2>
            <span className="text-xs text-gray-400 ml-2">{list.length} / 50 份本地快照</span>
          </div>
          <button onClick={onClose} className="p-1 hover:bg-gray-800 rounded">
            <X size={18} className="text-gray-400" />
          </button>
        </div>

        {/* 存为快照 */}
        <div className="px-5 py-4 border-b border-gray-700 bg-gray-850">
          <label className="text-xs text-gray-400 mb-1.5 block">📸 将当前画布存为新快照</label>
          <div className="flex gap-2">
            <input
              value={snapName}
              onChange={e => setSnapName(e.target.value)}
              placeholder="快照名称"
              className="flex-1 bg-gray-800 border border-gray-700 rounded px-3 py-1.5 text-sm text-white focus:outline-none focus:border-purple-500"
            />
            <button
              onClick={handleSaveSnapshot}
              className="px-4 py-1.5 bg-purple-600 hover:bg-purple-500 text-white text-sm rounded flex items-center gap-1.5"
            >
              <Camera size={14} /> 存档
            </button>
            <button
              onClick={() => fileRef.current?.click()}
              className="px-3 py-1.5 bg-gray-700 hover:bg-gray-600 text-white text-sm rounded flex items-center gap-1.5"
              title="从 JSON 文件导入"
            >
              <Upload size={14} /> 导入
            </button>
            <input
              ref={fileRef}
              type="file"
              accept="application/json"
              className="hidden"
              onChange={e => {
                const f = e.target.files?.[0]
                if (f) handleImport(f)
                if (fileRef.current) fileRef.current.value = ''
              }}
            />
          </div>
          <p className="text-[11px] text-gray-500 mt-1.5">
            当前画布：{current.nodes.length} 节点 / {current.edges.length} 连线 ·
            {current.workflowId ? <span className="text-emerald-400 ml-1">已绑定后端 id={current.workflowId}</span> : <span className="text-amber-400 ml-1">未保存到后端（仅本地）</span>}
          </p>
        </div>

        {/* 列表 */}
        <div className="flex-1 overflow-y-auto px-3 py-2">
          {list.length === 0 ? (
            <div className="py-12 text-center text-gray-500 text-sm">
              暂无快照。点上方「存档」按钮创建第一份。
            </div>
          ) : (
            <ul className="space-y-1.5">
              {list.map(s => (
                <li key={s.id} className="group flex items-center gap-3 px-3 py-2.5 bg-gray-800 hover:bg-gray-750 border border-gray-700 rounded-lg">
                  <div className="flex-1 min-w-0">
                    {renamingId === s.id ? (
                      <input
                        autoFocus
                        value={renameValue}
                        onChange={e => setRenameValue(e.target.value)}
                        onBlur={() => handleRename(s.id)}
                        onKeyDown={e => { if (e.key === 'Enter') handleRename(s.id); if (e.key === 'Escape') setRenamingId(null) }}
                        className="w-full bg-gray-900 border border-purple-500 rounded px-2 py-1 text-sm text-white focus:outline-none"
                      />
                    ) : (
                      <div className="flex items-center gap-2">
                        <span className="text-sm text-white font-medium truncate">{s.name}</span>
                        <button
                          onClick={() => { setRenamingId(s.id); setRenameValue(s.name) }}
                          className="opacity-0 group-hover:opacity-100 p-0.5 hover:bg-gray-700 rounded"
                          title="重命名"
                        >
                          <Edit2 size={11} className="text-gray-400" />
                        </button>
                      </div>
                    )}
                    <div className="text-[11px] text-gray-500 mt-0.5 flex gap-3">
                      <span>{new Date(s.createdAt).toLocaleString('zh-CN', { hour12: false })}</span>
                      <span>{s.nodeCount} 节点 · {s.edgeCount} 连线</span>
                      {s.workflowId && <span className="text-emerald-500">id={s.workflowId}</span>}
                    </div>
                  </div>
                  <button
                    onClick={() => {
                      if (confirm(`恢复「${s.name}」？\n当前画布的未存档修改会丢失（建议先点「存档」）。`)) {
                        onRestore(s)
                        onClose()
                      }
                    }}
                    className="px-2.5 py-1 bg-emerald-600/20 hover:bg-emerald-600/40 border border-emerald-600/40 text-emerald-400 text-xs rounded flex items-center gap-1"
                    title="恢复到画布"
                  >
                    <RotateCcw size={12} /> 恢复
                  </button>
                  <button
                    onClick={() => exportSnapshotAsFile(s)}
                    className="p-1.5 hover:bg-gray-700 rounded text-gray-400 hover:text-white"
                    title="下载 JSON"
                  >
                    <Download size={14} />
                  </button>
                  <button
                    onClick={() => {
                      if (confirm(`删除快照「${s.name}」？此操作不可撤销。`)) {
                        deleteSnapshot(s.id)
                        refresh()
                      }
                    }}
                    className="p-1.5 hover:bg-red-900/40 rounded text-gray-400 hover:text-red-400"
                    title="删除"
                  >
                    <Trash2 size={14} />
                  </button>
                </li>
              ))}
            </ul>
          )}
        </div>

        <div className="px-5 py-2.5 border-t border-gray-700 text-[11px] text-gray-500">
          💡 快照存在浏览器本地（localStorage）。换设备请使用「导出 JSON」离线备份，或点工具栏「💾 保存」写入后端。
        </div>
      </div>
    </div>
  )
}
