import { useEffect, useState } from 'react'
import { KanbanSquare, Plus, ChevronDown, Circle, Clock, Eye, CheckCircle2, Archive } from 'lucide-react'
import { api } from '../api'

interface Issue {
  id: string
  key: string
  title: string
  type: string
  priority: string
  status: string
  assignee: string
  service: string
  story_points: number
}

const columns = [
  { key: 'backlog', label: 'Backlog', icon: Archive, color: 'text-stone-500' },
  { key: 'todo', label: 'Todo', icon: Circle, color: 'text-blue-400' },
  { key: 'in_progress', label: '进行中', icon: Clock, color: 'text-yellow-400' },
  { key: 'review', label: '审查', icon: Eye, color: 'text-purple-400' },
  { key: 'done', label: '完成', icon: CheckCircle2, color: 'text-green-400' },
]

const priorityColors: Record<string, string> = {
  critical: 'border-l-red-500',
  high: 'border-l-orange-400',
  medium: 'border-l-blue-400',
  low: 'border-l-stone-600',
}

const typeLabels: Record<string, { label: string; color: string }> = {
  epic: { label: 'Epic', color: 'bg-purple-500/20 text-purple-300' },
  story: { label: 'Story', color: 'bg-blue-500/20 text-blue-300' },
  task: { label: 'Task', color: 'bg-stone-500/20 text-stone-300' },
  bug: { label: 'Bug', color: 'bg-red-500/20 text-red-300' },
  improvement: { label: 'Improve', color: 'bg-green-500/20 text-green-300' },
}

export default function BoardPage() {
  const [board, setBoard] = useState<Record<string, Issue[]>>({})
  const [projects, setProjects] = useState<any[]>([])
  const [selectedProject, setSelectedProject] = useState<string>('')
  const [loading, setLoading] = useState(true)
  const [showCreate, setShowCreate] = useState(false)
  const [newIssue, setNewIssue] = useState({ title: '', type: 'task', priority: 'medium', service: '', task_type: 'code' })

  useEffect(() => {
    api.listProjects().then((r) => {
      const p = r.projects || []
      setProjects(p)
      if (p.length > 0) setSelectedProject(p[0].id)
    })
  }, [])

  useEffect(() => {
    if (!selectedProject) return
    setLoading(true)
    api.board(selectedProject).then((r) => {
      setBoard(r.board || {})
      setLoading(false)
    }).catch(() => setLoading(false))
  }, [selectedProject])

  const handleTransition = async (issueId: string, newStatus: string) => {
    await api.transitionIssue(issueId, newStatus)
    // Reload
    const r = await api.board(selectedProject)
    setBoard(r.board || {})
  }

  const handleCreate = async () => {
    if (!newIssue.title.trim() || !selectedProject) return
    await api.createIssue(selectedProject, newIssue)
    setNewIssue({ title: '', type: 'task', priority: 'medium', service: '', task_type: 'code' })
    setShowCreate(false)
    const r = await api.board(selectedProject)
    setBoard(r.board || {})
  }

  return (
    <div className="p-6 h-full flex flex-col">
      {/* Header */}
      <div className="flex items-center justify-between mb-6">
        <div className="flex items-center gap-3">
          <KanbanSquare className="w-6 h-6 text-forge-400" />
          <h1 className="text-xl font-bold">看板</h1>
          {projects.length > 0 && (
            <select
              value={selectedProject}
              onChange={(e) => setSelectedProject(e.target.value)}
              className="ml-4 bg-stone-800 border border-stone-700 rounded-lg px-3 py-1.5 text-sm text-stone-300 focus:outline-none focus:border-forge-500"
            >
              {projects.map((p: any) => (
                <option key={p.id} value={p.id}>{p.name} ({p.key})</option>
              ))}
            </select>
          )}
        </div>
        <button
          onClick={() => setShowCreate(!showCreate)}
          className="flex items-center gap-2 px-4 py-2 bg-forge-600 hover:bg-forge-500 text-white rounded-lg text-sm transition-colors"
        >
          <Plus className="w-4 h-4" />
          新建 Issue
        </button>
      </div>

      {/* Quick Create */}
      {showCreate && (
        <div className="glass rounded-xl p-4 mb-4 space-y-3">
          <div className="flex gap-3">
            <input
              value={newIssue.title}
              onChange={(e) => setNewIssue({ ...newIssue, title: e.target.value })}
              placeholder="Issue 标题..."
              className="flex-1 bg-stone-800 border border-stone-700 rounded-lg px-3 py-2 text-sm focus:outline-none focus:border-forge-500"
              onKeyDown={(e) => e.key === 'Enter' && handleCreate()}
            />
            <select value={newIssue.type} onChange={(e) => setNewIssue({ ...newIssue, type: e.target.value })} className="bg-stone-800 border border-stone-700 rounded-lg px-3 py-2 text-sm">
              <option value="task">Task</option>
              <option value="bug">Bug</option>
              <option value="story">Story</option>
              <option value="epic">Epic</option>
            </select>
            <select value={newIssue.priority} onChange={(e) => setNewIssue({ ...newIssue, priority: e.target.value })} className="bg-stone-800 border border-stone-700 rounded-lg px-3 py-2 text-sm">
              <option value="critical">Critical</option>
              <option value="high">High</option>
              <option value="medium">Medium</option>
              <option value="low">Low</option>
            </select>
            <input
              value={newIssue.service}
              onChange={(e) => setNewIssue({ ...newIssue, service: e.target.value })}
              placeholder="服务 (claw/api)"
              className="w-36 bg-stone-800 border border-stone-700 rounded-lg px-3 py-2 text-sm focus:outline-none focus:border-forge-500"
            />
            <button onClick={handleCreate} className="px-4 py-2 bg-forge-600 hover:bg-forge-500 text-white rounded-lg text-sm">
              创建
            </button>
          </div>
        </div>
      )}

      {/* Board */}
      {loading ? (
        <div className="flex-1 flex items-center justify-center text-stone-600">加载中...</div>
      ) : projects.length === 0 ? (
        <div className="flex-1 flex flex-col items-center justify-center text-stone-600 gap-2">
          <KanbanSquare className="w-12 h-12 text-stone-700" />
          <p>暂无项目，先创建一个项目</p>
        </div>
      ) : (
        <div className="flex-1 grid grid-cols-5 gap-4 min-h-0 overflow-hidden">
          {columns.map((col) => {
            const issues = board[col.key] || []
            return (
              <div key={col.key} className="flex flex-col min-h-0">
                <div className="flex items-center gap-2 mb-3 px-1">
                  <col.icon className={`w-4 h-4 ${col.color}`} />
                  <span className="text-sm font-medium text-stone-400">{col.label}</span>
                  <span className="text-xs text-stone-600 ml-auto">{issues.length}</span>
                </div>
                <div className="flex-1 overflow-auto space-y-2 pb-4">
                  {issues.map((issue) => (
                    <IssueCard
                      key={issue.id}
                      issue={issue}
                      colIdx={columns.findIndex((c) => c.key === col.key)}
                      onMove={(dir) => {
                        const newIdx = columns.findIndex((c) => c.key === col.key) + dir
                        if (newIdx >= 0 && newIdx < columns.length) {
                          handleTransition(issue.id, columns[newIdx].key)
                        }
                      }}
                    />
                  ))}
                </div>
              </div>
            )
          })}
        </div>
      )}
    </div>
  )
}

function IssueCard({ issue, colIdx, onMove }: { issue: Issue; colIdx: number; onMove: (dir: number) => void }) {
  const pColor = priorityColors[issue.priority] || 'border-l-stone-600'
  const tInfo = typeLabels[issue.type] || typeLabels.task

  return (
    <div className={`glass rounded-lg p-3 border-l-2 ${pColor} glass-hover group cursor-pointer`}>
      <div className="flex items-start justify-between gap-2">
        <div className="flex-1 min-w-0">
          <p className="text-sm text-stone-200 leading-snug">{issue.title}</p>
          <div className="flex items-center gap-2 mt-2 flex-wrap">
            <span className="text-[10px] font-mono text-stone-500">{issue.key}</span>
            <span className={`text-[10px] px-1.5 py-0.5 rounded ${tInfo.color}`}>{tInfo.label}</span>
            {issue.service && <span className="text-[10px] px-1.5 py-0.5 rounded bg-stone-800 text-stone-400">{issue.service}</span>}
            {issue.story_points > 0 && <span className="text-[10px] px-1.5 py-0.5 rounded bg-forge-500/10 text-forge-400">{issue.story_points}pt</span>}
          </div>
        </div>
      </div>
      {/* Quick transition arrows */}
      <div className="flex justify-end gap-1 mt-2 opacity-0 group-hover:opacity-100 transition-opacity">
        {colIdx > 0 && (
          <button onClick={() => onMove(-1)} className="text-[10px] px-2 py-0.5 rounded bg-stone-800 hover:bg-stone-700 text-stone-400">← 退回</button>
        )}
        {colIdx < 4 && (
          <button onClick={() => onMove(1)} className="text-[10px] px-2 py-0.5 rounded bg-forge-600/30 hover:bg-forge-600/50 text-forge-300">推进 →</button>
        )}
      </div>
      {issue.assignee && (
        <p className="text-[10px] text-stone-600 mt-1.5">{issue.assignee}</p>
      )}
    </div>
  )
}
