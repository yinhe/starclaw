import { FileText, FileEdit, FileAudio, FileVideo, FileCode, File, Terminal, Globe, Search, Eye, Wrench, Settings, Bot, XIcon } from 'lucide-react'

export interface ToolTypeInfo {
  icon: typeof Wrench
  label: string
  color: string
}

export const buildToolDescription = (toolName: string, action: string, args: Record<string, any>, agents?: { id: string; name: string }[]): string => {
  if (toolName === 'code') {
    switch (action) {
      case 'read_file': return `读取文件 ${args.path || ''}`
      case 'write_file': return `写入文件 ${args.path || ''} (${(args.content || '').length} chars)`
      case 'list_files': return `列出目录 ${args.path || '.'}`
      case 'search_files': return `搜索文件 ${args.pattern || ''}`
      case 'grep': return `搜索内容 "${args.pattern || ''}"`
      case 'execute': return `执行 ${args.language || ''} 代码`
      case 'run_command': return `运行命令 \`${(args.command || '').slice(0, 60)}${(args.command || '').length > 60 ? '...' : ''}\``
      case 'start_app': return `启动应用 \`${(args.command || '').slice(0, 40)}\``
      case 'stop_app': return `停止应用`
      case 'list_apps': return `查看运行中的应用`
      default: return `code.${action}`
    }
  }
  if (toolName === 'browser') {
    switch (action) {
      case 'navigate': return `打开网页 ${args.url || ''}`
      case 'click': return `点击 "${args.selector || ''}"`
      case 'type': return `输入文本`
      case 'screenshot': return '截取屏幕'
      case 'extract_text': return '提取文本'
      case 'scroll': return `滚动页面 ${args.direction || ''}`
      default: return `browser.${action}`
    }
  }
  if (toolName === 'web_search') return `搜索网络 "${args.query || ''}"`
  if (toolName === 'http_request') return `HTTP ${args.method || 'GET'} ${args.url || ''}`
  if (toolName === 'system') {
    switch (action) {
      case 'create_agent': return `创建 Agent "${args.name || ''}"`
      case 'delegate_to_agent': {
        const targetAgent = agents?.find(a => a.id === args.agent_id)
        return `委派任务给「${targetAgent?.name || args.agent_id?.slice(0, 8) + '...'}」`
      }
      case 'create_task': return `创建后台任务 "${args.title || ''}"`
      case 'update_task': return `更新任务进度 ${args.progress || ''}%`
      case 'list_tasks': return '查看后台任务'
      case 'notify_user': return `通知用户: ${args.title || ''}`
      case 'list_agents': return '列出所有 Agent'
      case 'list_models': return '列出可用模型'
      case 'create_workflow': return `创建工作流 "${args.name || ''}"`
      case 'schedule_task': return `创建定时任务 ${args.cron_expr || ''}`
      case 'list_schedules': return '列出定时任务'
      default: return `system.${action}`
    }
  }
  return `${toolName}${action ? '.' + action : ''}`
}

export const getToolTypeInfo = (toolName: string, action: string): ToolTypeInfo => {
  if (toolName === 'code') {
    switch (action) {
      case 'read_file': return { icon: FileText, label: 'Read', color: 'text-blue-600 bg-blue-50 border-blue-200' }
      case 'write_file': return { icon: FileEdit, label: 'Write', color: 'text-green-600 bg-green-50 border-green-200' }
      case 'execute': return { icon: Terminal, label: 'Exec', color: 'text-purple-600 bg-purple-50 border-purple-200' }
      case 'run_command': return { icon: Terminal, label: 'Exec', color: 'text-purple-600 bg-purple-50 border-purple-200' }
      case 'start_app': return { icon: Globe, label: 'Deploy', color: 'text-emerald-600 bg-emerald-50 border-emerald-200' }
      case 'stop_app': return { icon: XIcon, label: 'Stop', color: 'text-red-600 bg-red-50 border-red-200' }
      case 'list_apps': return { icon: Eye, label: 'Apps', color: 'text-cyan-600 bg-cyan-50 border-cyan-200' }
      case 'grep': case 'search_files': return { icon: Search, label: 'Search', color: 'text-amber-600 bg-amber-50 border-amber-200' }
      case 'list_files': return { icon: Eye, label: 'List', color: 'text-cyan-600 bg-cyan-50 border-cyan-200' }
      default: return { icon: Wrench, label: 'Tool', color: 'text-gray-600 bg-gray-50 border-gray-200' }
    }
  }
  if (toolName === 'browser') return { icon: Globe, label: 'Browser', color: 'text-indigo-600 bg-indigo-50 border-indigo-200' }
  if (toolName === 'web_search') return { icon: Search, label: 'Search', color: 'text-amber-600 bg-amber-50 border-amber-200' }
  if (toolName === 'http_request') return { icon: Globe, label: 'HTTP', color: 'text-teal-600 bg-teal-50 border-teal-200' }
  if (toolName === 'music_generation') return { icon: Bot, label: 'Music', color: 'text-pink-600 bg-pink-50 border-pink-200' }
  if (toolName === 'image_generation') return { icon: Bot, label: 'Image', color: 'text-cyan-600 bg-cyan-50 border-cyan-200' }
  if (toolName === 'dubbing') return { icon: Bot, label: 'Dubbing', color: 'text-purple-600 bg-purple-50 border-purple-200' }
  if (toolName === 'mv_production') return { icon: Bot, label: 'MV', color: 'text-fuchsia-600 bg-fuchsia-50 border-fuchsia-200' }
  if (toolName === 'comic_production') return { icon: Bot, label: 'Comic', color: 'text-orange-600 bg-orange-50 border-orange-200' }
  if (toolName === 'system') {
    switch (action) {
      case 'create_agent': return { icon: Bot, label: 'Agent', color: 'text-emerald-600 bg-emerald-50 border-emerald-200' }
      case 'delegate_to_agent': return { icon: Bot, label: 'Delegate', color: 'text-violet-600 bg-violet-50 border-violet-200' }
      case 'create_task': return { icon: Bot, label: 'Task', color: 'text-violet-600 bg-violet-50 border-violet-200' }
      case 'update_task': return { icon: Bot, label: 'Progress', color: 'text-blue-600 bg-blue-50 border-blue-200' }
      case 'list_tasks': return { icon: Bot, label: 'Tasks', color: 'text-violet-600 bg-violet-50 border-violet-200' }
      case 'notify_user': return { icon: Bot, label: 'Notify', color: 'text-amber-600 bg-amber-50 border-amber-200' }
      case 'list_agents': return { icon: Bot, label: 'Agent', color: 'text-emerald-600 bg-emerald-50 border-emerald-200' }
      case 'create_workflow': case 'schedule_task': case 'list_schedules': return { icon: Settings, label: 'System', color: 'text-rose-600 bg-rose-50 border-rose-200' }
      case 'list_models': return { icon: Settings, label: 'Models', color: 'text-rose-600 bg-rose-50 border-rose-200' }
      default: return { icon: Settings, label: 'System', color: 'text-rose-600 bg-rose-50 border-rose-200' }
    }
  }
  return { icon: Wrench, label: 'Tool', color: 'text-gray-600 bg-gray-50 border-gray-200' }
}

export const formatToolResult = (result: string): string => {
  try {
    const parsed = JSON.parse(result)
    if (parsed.result?.stdout || parsed.result?.stderr) {
      const parts = []
      if (parsed.result.stdout) parts.push(parsed.result.stdout)
      if (parsed.result.stderr) parts.push(`stderr: ${parsed.result.stderr}`)
      if (parsed.result.exit_code !== undefined) parts.push(`Exit code: ${parsed.result.exit_code} (${parsed.result.duration || ''})`)
      return parts.join('\n')
    }
    if (parsed.content) return parsed.content.length > 500 ? parsed.content.slice(0, 500) + '\n... [truncated]' : parsed.content
    if (parsed.files) return parsed.files.map((f: any) => `${f.is_dir ? '📁' : '📄'} ${f.path} ${f.is_dir ? '' : `(${f.size}B)`}`).join('\n')
    if (parsed.matches) return parsed.matches.map((m: any) => `${m.file}:${m.line} ${m.content}`).join('\n')
    if (parsed.status === 'success' && parsed.bytes !== undefined) return `✓ ${parsed.action} ${parsed.path || ''} (${parsed.bytes} bytes)`
    if (parsed.status === 'success' && parsed.agents) return `✓ ${parsed.action}: ${parsed.count} 个Agent`
    if (parsed.status === 'success' && parsed.count !== undefined) return `✓ ${parsed.action}: ${parsed.count} 项`
    if (parsed.status === 'success' && parsed.message) return `✓ ${parsed.message}`
    if (parsed.status === 'success') return `✓ ${parsed.action || 'done'}`
    if (parsed.error) return `✗ ${parsed.error}`
    if (parsed.message) return parsed.message
    return JSON.stringify(parsed, null, 2).slice(0, 500)
  } catch {
    return result.length > 500 ? result.slice(0, 500) + '\n... [truncated]' : result
  }
}

export const extractMediaFromToolResult = (result: string | undefined): { type: 'image' | 'video' | 'audio'; url: string } | null => {
  if (!result) return null
  try {
    const parsed = JSON.parse(result)
    if (parsed.image_url) return { type: 'image', url: parsed.image_url }
    if (parsed.video_url) return { type: 'video', url: parsed.video_url }
    if (parsed.audio_url) return { type: 'audio', url: parsed.audio_url }
  } catch { /* ignore */ }
  return null
}

export const formatSize = (bytes: number): string => {
  if (bytes < 1024) return bytes + ' B'
  if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB'
  return (bytes / (1024 * 1024)).toFixed(1) + ' MB'
}

export const getFileIcon = (category: string) => {
  switch (category) {
    case 'audio': return FileAudio
    case 'video': return FileVideo
    case 'code': return FileCode
    case 'document': return FileText
    default: return File
  }
}

export const isRunnableFile = (path: string) => /\.(py|js|ts|go|sh|rb|php|java|rs|c|cpp|cxx|cc|pl|lua)$/i.test(path)
