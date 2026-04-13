import { useNavigate } from 'react-router-dom'
import { Loader2, X as XIcon, ListTodo, Workflow, Video, PlayCircle, Clock, AlertCircle, CheckCircle2, ChevronDown, ChevronUp, Eye, ExternalLink, Volume2, FileText, Pin, Brain, Lightbulb, Wrench, Star } from 'lucide-react'

interface ContextPanelProps {
  convContext: any
  contextLoading: boolean
  contextExpandedSection: string
  setContextExpandedSection: (s: string) => void
  contextMemories: any[]
  onClose: () => void
}

export default function ContextPanel({
  convContext,
  contextLoading,
  contextExpandedSection,
  setContextExpandedSection,
  contextMemories,
  onClose,
}: ContextPanelProps) {
  const navigate = useNavigate()

  return (
    <div className="fixed xl:relative right-0 top-0 xl:top-auto h-full z-30 xl:z-auto w-80 border-l bg-gray-50 dark:bg-gray-900 flex flex-col overflow-hidden shadow-xl xl:shadow-none">
      <div className="px-4 py-3 border-b bg-white dark:bg-gray-800 flex items-center justify-between">
        <h3 className="text-sm font-semibold text-gray-700 dark:text-gray-200">关联面板</h3>
        <button onClick={onClose} className="p-1 text-gray-400 hover:text-gray-600 rounded">
          <XIcon className="w-4 h-4" />
        </button>
      </div>

      {contextLoading && !convContext ? (
        <div className="flex-1 flex items-center justify-center">
          <Loader2 className="w-5 h-5 text-gray-400 animate-spin" />
        </div>
      ) : !convContext ? (
        <div className="flex-1 flex items-center justify-center text-gray-400 text-sm">无数据</div>
      ) : (
        <div className="flex-1 overflow-y-auto scrollbar-thin">
          {/* Stats summary */}
          <div className="px-4 py-3 border-b grid grid-cols-3 gap-2">
            <div className="text-center">
              <div className="text-lg font-bold text-primary-600">{convContext.stats?.tasks_total || 0}</div>
              <div className="text-[10px] text-gray-400">任务</div>
            </div>
            <div className="text-center">
              <div className="text-lg font-bold text-violet-600">{convContext.stats?.workflows_total || 0}</div>
              <div className="text-[10px] text-gray-400">工作流</div>
            </div>
            <div className="text-center">
              <div className="text-lg font-bold text-emerald-600">{convContext.stats?.videos_total || 0}</div>
              <div className="text-[10px] text-gray-400">视频</div>
            </div>
          </div>

          {/* Tasks section */}
          <div className="border-b">
            <button
              onClick={() => setContextExpandedSection(contextExpandedSection === 'tasks' ? '' : 'tasks')}
              className="w-full px-4 py-2.5 flex items-center gap-2 text-sm font-medium text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-800"
            >
              <ListTodo className="w-4 h-4 text-primary-500" />
              <span>任务</span>
              <span className="text-xs text-gray-400 ml-1">
                {convContext.stats?.tasks_running > 0 && <span className="text-amber-500">{convContext.stats.tasks_running} 运行中</span>}
                {convContext.stats?.tasks_running > 0 && convContext.stats?.tasks_completed > 0 && ' · '}
                {convContext.stats?.tasks_completed > 0 && <span className="text-green-500">{convContext.stats.tasks_completed} 完成</span>}
              </span>
              <div className="flex-1" />
              {contextExpandedSection === 'tasks' ? <ChevronUp className="w-3.5 h-3.5 text-gray-400" /> : <ChevronDown className="w-3.5 h-3.5 text-gray-400" />}
            </button>
            {contextExpandedSection === 'tasks' && (
              <div className="px-3 pb-3 space-y-2">
                {(convContext.tasks || []).length === 0 ? (
                  <p className="text-xs text-gray-400 text-center py-2">暂无任务</p>
                ) : (
                  (convContext.tasks || []).map((task: any) => (
                    <div
                      key={task.id}
                      onClick={() => navigate('/tasks')}
                      className="p-2.5 bg-white dark:bg-gray-800 rounded-lg border cursor-pointer hover:border-primary-300 transition-colors"
                    >
                      <div className="flex items-center gap-1.5 mb-1">
                        {task.status === 'running' && <Loader2 className="w-3 h-3 text-amber-500 animate-spin" />}
                        {task.status === 'completed' && <CheckCircle2 className="w-3 h-3 text-green-500" />}
                        {task.status === 'failed' && <AlertCircle className="w-3 h-3 text-red-500" />}
                        {task.status === 'pending' && <Clock className="w-3 h-3 text-gray-400" />}
                        <span className="text-xs font-medium text-gray-700 dark:text-gray-200 truncate flex-1">{task.title}</span>
                        <ExternalLink className="w-3 h-3 text-gray-300" />
                      </div>
                      {task.progress > 0 && (
                        <div className="w-full bg-gray-200 dark:bg-gray-700 rounded-full h-1 mt-1">
                          <div className="bg-primary-500 h-1 rounded-full transition-all" style={{ width: `${task.progress}%` }} />
                        </div>
                      )}
                    </div>
                  ))
                )}
              </div>
            )}
          </div>

          {/* Workflows section */}
          <div className="border-b">
            <button
              onClick={() => setContextExpandedSection(contextExpandedSection === 'workflows' ? '' : 'workflows')}
              className="w-full px-4 py-2.5 flex items-center gap-2 text-sm font-medium text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-800"
            >
              <Workflow className="w-4 h-4 text-violet-500" />
              <span>工作流</span>
              <span className="text-xs text-gray-400 ml-1">{convContext.stats?.workflows_total || 0}</span>
              <div className="flex-1" />
              {contextExpandedSection === 'workflows' ? <ChevronUp className="w-3.5 h-3.5 text-gray-400" /> : <ChevronDown className="w-3.5 h-3.5 text-gray-400" />}
            </button>
            {contextExpandedSection === 'workflows' && (
              <div className="px-3 pb-3 space-y-2">
                {(convContext.workflows || []).length === 0 ? (
                  <p className="text-xs text-gray-400 text-center py-2">暂无工作流</p>
                ) : (
                  (convContext.workflows || []).map((wf: any) => (
                    <div
                      key={wf.id}
                      onClick={() => navigate(`/workflows/${wf.id}`)}
                      className="p-2.5 bg-white dark:bg-gray-800 rounded-lg border cursor-pointer hover:border-violet-300 transition-colors"
                    >
                      <div className="flex items-center gap-1.5">
                        <Workflow className="w-3 h-3 text-violet-500" />
                        <span className="text-xs font-medium text-gray-700 dark:text-gray-200 truncate flex-1">{wf.name}</span>
                        <ExternalLink className="w-3 h-3 text-gray-300" />
                      </div>
                      {wf.description && (
                        <p className="text-[10px] text-gray-400 mt-1 truncate">{wf.description}</p>
                      )}
                    </div>
                  ))
                )}
              </div>
            )}
          </div>

          {/* Videos section */}
          <div className="border-b">
            <button
              onClick={() => setContextExpandedSection(contextExpandedSection === 'videos' ? '' : 'videos')}
              className="w-full px-4 py-2.5 flex items-center gap-2 text-sm font-medium text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-800"
            >
              <Video className="w-4 h-4 text-emerald-500" />
              <span>视频</span>
              <span className="text-xs text-gray-400 ml-1">
                {convContext.stats?.videos_merged > 0 && <span className="text-emerald-500">{convContext.stats.videos_merged} 合成</span>}
                {convContext.stats?.videos_merged > 0 && convContext.stats?.videos_narrated > 0 && ' · '}
                {convContext.stats?.videos_narrated > 0 && <span className="text-blue-500">{convContext.stats.videos_narrated} 配音</span>}
              </span>
              <div className="flex-1" />
              {contextExpandedSection === 'videos' ? <ChevronUp className="w-3.5 h-3.5 text-gray-400" /> : <ChevronDown className="w-3.5 h-3.5 text-gray-400" />}
            </button>
            {contextExpandedSection === 'videos' && (
              <div className="px-3 pb-3 space-y-2">
                {(convContext.videos || []).length === 0 ? (
                  <p className="text-xs text-gray-400 text-center py-2">暂无视频</p>
                ) : (
                  <>
                    {/* Merged videos first */}
                    {(convContext.videos || []).filter((v: any) => v.type === 'merged').map((v: any) => (
                      <div key={v.id} className="bg-white dark:bg-gray-800 rounded-lg border overflow-hidden">
                        <video
                          src={v.video_url}
                          controls
                          className="w-full aspect-video bg-black"
                          preload="metadata"
                        />
                        <div className="px-2.5 py-1.5 flex items-center gap-1.5">
                          <PlayCircle className="w-3 h-3 text-emerald-500" />
                          <span className="text-[10px] text-gray-500 truncate flex-1">合成视频 · {v.duration}秒</span>
                          <button onClick={() => navigate('/videos')} className="text-gray-300 hover:text-gray-500">
                            <ExternalLink className="w-3 h-3" />
                          </button>
                        </div>
                      </div>
                    ))}
                    {/* Clips */}
                    <div className="grid grid-cols-2 gap-1.5">
                      {(convContext.videos || []).filter((v: any) => v.type !== 'merged' && v.status === 'succeeded').map((v: any) => (
                        <div key={v.id} className="bg-white dark:bg-gray-800 rounded-lg border overflow-hidden">
                          <video
                            src={v.narrated_url || v.video_url}
                            className="w-full aspect-video bg-black"
                            preload="metadata"
                          />
                          <div className="px-1.5 py-1 flex items-center gap-1">
                            <span className="text-[9px] text-gray-400 truncate">{v.scene || '片段'}</span>
                            {v.narrated_url && <Volume2 className="w-2.5 h-2.5 text-blue-400 flex-shrink-0" />}
                          </div>
                        </div>
                      ))}
                    </div>
                  </>
                )}
              </div>
            )}
          </div>

          {/* Memory section */}
          <div className="border-b">
            <button
              onClick={() => setContextExpandedSection(contextExpandedSection === 'memories' ? '' : 'memories')}
              className="w-full px-4 py-2.5 flex items-center gap-2 text-sm font-medium text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-800"
            >
              <Brain className="w-4 h-4 text-violet-500" />
              <span>记忆</span>
              <span className="text-xs text-gray-400 ml-1">{contextMemories.length}</span>
              <div className="flex-1" />
              {contextExpandedSection === 'memories' ? <ChevronUp className="w-3.5 h-3.5 text-gray-400" /> : <ChevronDown className="w-3.5 h-3.5 text-gray-400" />}
            </button>
            {contextExpandedSection === 'memories' && (
              <div className="px-3 pb-3 space-y-1.5">
                {contextMemories.length === 0 ? (
                  <p className="text-xs text-gray-400 text-center py-2">暂无记忆</p>
                ) : (
                  contextMemories.slice(0, 8).map((mem: any) => (
                    <div key={mem.id} className="p-2 bg-white dark:bg-gray-800 rounded-lg border text-xs">
                      <div className="flex items-center gap-1.5 mb-0.5">
                        {mem.category === 'instruct' && <Pin className="w-3 h-3 text-red-500" />}
                        {mem.category === 'fact' && <FileText className="w-3 h-3 text-blue-500" />}
                        {mem.category === 'preference' && <Lightbulb className="w-3 h-3 text-amber-500" />}
                        {mem.category === 'skill' && <Wrench className="w-3 h-3 text-emerald-500" />}
                        {mem.category === 'context' && <Brain className="w-3 h-3 text-violet-500" />}
                        <span className="font-medium text-gray-700 dark:text-gray-200 truncate flex-1">{mem.key}</span>
                        <span className="flex items-center">
                          {[1,2,3].map(i => <Star key={i} className={`w-2 h-2 ${i <= Math.round(mem.importance * 3) ? 'text-amber-400 fill-amber-400' : 'text-gray-300'}`} />)}
                        </span>
                      </div>
                      <p className="text-gray-500 dark:text-gray-400 truncate">{mem.content}</p>
                    </div>
                  ))
                )}
                <button
                  onClick={() => navigate('/memories')}
                  className="w-full text-center text-[10px] text-violet-500 hover:text-violet-700 py-1"
                >
                  查看全部记忆 →
                </button>
              </div>
            )}
          </div>

          {/* Quick nav */}
          <div className="px-4 py-3 space-y-1.5">
            <p className="text-[10px] text-gray-400 uppercase font-medium mb-2">快捷跳转</p>
            <button onClick={() => navigate('/tasks')} className="w-full text-left px-3 py-2 text-xs text-gray-600 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-800 rounded-lg flex items-center gap-2">
              <ListTodo className="w-3.5 h-3.5" /> 任务中心
            </button>
            <button onClick={() => navigate('/workflows')} className="w-full text-left px-3 py-2 text-xs text-gray-600 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-800 rounded-lg flex items-center gap-2">
              <Workflow className="w-3.5 h-3.5" /> 工作流
            </button>
            <button onClick={() => navigate('/videos')} className="w-full text-left px-3 py-2 text-xs text-gray-600 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-800 rounded-lg flex items-center gap-2">
              <Video className="w-3.5 h-3.5" /> 视频画廊
            </button>
            <button onClick={() => navigate('/visualization')} className="w-full text-left px-3 py-2 text-xs text-gray-600 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-800 rounded-lg flex items-center gap-2">
              <Eye className="w-3.5 h-3.5" /> 可视化
            </button>
          </div>
        </div>
      )}
    </div>
  )
}
