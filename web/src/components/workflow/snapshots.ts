import type { Node, Edge } from '@xyflow/react'

export interface WorkflowSnapshot {
  id: string
  name: string
  workflowId: string | null // 所属后端 workflow.id，可为 null（草稿）
  createdAt: number
  nodeCount: number
  edgeCount: number
  data: { nodes: Node[]; edges: Edge[]; workflowName: string; counter: number }
}

const KEY = 'wf-snapshots'
const MAX_SNAPSHOTS = 50 // 上限防止 localStorage 爆

export function listSnapshots(): WorkflowSnapshot[] {
  try {
    const raw = localStorage.getItem(KEY)
    if (!raw) return []
    const arr = JSON.parse(raw) as WorkflowSnapshot[]
    return Array.isArray(arr) ? arr.sort((a, b) => b.createdAt - a.createdAt) : []
  } catch {
    return []
  }
}

export function saveSnapshot(params: {
  name: string
  workflowId: string | null
  nodes: Node[]
  edges: Edge[]
  workflowName: string
  counter: number
}): WorkflowSnapshot {
  const snap: WorkflowSnapshot = {
    id: `snap-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`,
    name: params.name || `快照 ${new Date().toLocaleString('zh-CN')}`,
    workflowId: params.workflowId,
    createdAt: Date.now(),
    nodeCount: params.nodes.length,
    edgeCount: params.edges.length,
    data: {
      nodes: params.nodes,
      edges: params.edges,
      workflowName: params.workflowName,
      counter: params.counter,
    },
  }
  const all = listSnapshots()
  all.unshift(snap)
  // 超上限时按 createdAt 尾部淘汰
  const trimmed = all.slice(0, MAX_SNAPSHOTS)
  try {
    localStorage.setItem(KEY, JSON.stringify(trimmed))
  } catch (e) {
    // 配额爆了：再砍掉一半重试
    try {
      localStorage.setItem(KEY, JSON.stringify(trimmed.slice(0, Math.floor(MAX_SNAPSHOTS / 2))))
    } catch {
      throw new Error('localStorage 配额不足，请先导出/删除旧快照')
    }
    throw e
  }
  return snap
}

export function deleteSnapshot(id: string): void {
  const all = listSnapshots().filter(s => s.id !== id)
  localStorage.setItem(KEY, JSON.stringify(all))
}

export function renameSnapshot(id: string, name: string): void {
  const all = listSnapshots().map(s => s.id === id ? { ...s, name } : s)
  localStorage.setItem(KEY, JSON.stringify(all))
}

/** 导出：生成可下载的 JSON Blob URL。调用方负责 revoke */
export function exportSnapshotAsFile(snap: WorkflowSnapshot): void {
  const payload = {
    version: 1,
    kind: 'starclaw-workflow-snapshot',
    ...snap,
  }
  const blob = new Blob([JSON.stringify(payload, null, 2)], { type: 'application/json' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = `${snap.name.replace(/[^\w\u4e00-\u9fa5-]+/g, '_')}_${snap.id}.json`
  document.body.appendChild(a)
  a.click()
  a.remove()
  URL.revokeObjectURL(url)
}

/** 导入：从 File 读入并解析。返回可直接 saveSnapshot 或直接恢复的 payload */
export async function importSnapshotFile(file: File): Promise<WorkflowSnapshot> {
  const text = await file.text()
  const parsed = JSON.parse(text)
  if (parsed?.kind !== 'starclaw-workflow-snapshot' || !parsed?.data?.nodes) {
    throw new Error('不是合法的工作流快照 JSON')
  }
  return parsed as WorkflowSnapshot
}
