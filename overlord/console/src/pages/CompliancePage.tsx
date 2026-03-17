import { useState, useEffect } from 'react'
import { ShieldCheck, AlertTriangle, Download, Plus, Trash2, CheckCircle, ArrowRight } from 'lucide-react'
import { broodAPI, type ComplianceStats, type ComplianceLogEntry, type SensitiveWordRule, type DataFlowRecord } from '../api/brood'

const SEV_MAP: Record<string, { label: string; color: string }> = {
  info: { label: '信息', color: 'text-blue-400 bg-blue-500/10' },
  warning: { label: '警告', color: 'text-amber-400 bg-amber-500/10' },
  critical: { label: '严重', color: 'text-red-400 bg-red-500/10' },
}

const TYPE_MAP: Record<string, string> = {
  data_access: '数据访问', data_export: '数据导出', sensitive_word: '敏感词',
  policy_violation: '策略违规', audit_export: '审计导出',
}

const WORD_CATS: Record<string, string> = {
  pii: '个人信息', financial: '金融', medical: '医疗', political: '政治', custom: '自定义',
}

const ACTION_MAP: Record<string, { label: string; color: string }> = {
  log: { label: '记录', color: 'text-blue-400' },
  block: { label: '拦截', color: 'text-red-400' },
  mask: { label: '脱敏', color: 'text-amber-400' },
}

export default function CompliancePage() {
  const [tab, setTab] = useState<'overview' | 'logs' | 'words' | 'flows'>('overview')
  const [stats, setStats] = useState<ComplianceStats | null>(null)
  const [logs, setLogs] = useState<ComplianceLogEntry[]>([])
  const [logsTotal, setLogsTotal] = useState(0)
  const [logFilter, setLogFilter] = useState<{ severity?: string; event_type?: string; resolved?: string }>({})
  const [words, setWords] = useState<SensitiveWordRule[]>([])
  const [flows, setFlows] = useState<DataFlowRecord[]>([])
  const [showAddWord, setShowAddWord] = useState(false)
  const [newWord, setNewWord] = useState({ word: '', category: 'custom', action: 'log' })
  const [showAddFlow, setShowAddFlow] = useState(false)
  const [newFlow, setNewFlow] = useState({ source: '', destination: '', data_type: '', encryption: 'tls_1.3', region: '', cross_border: false })

  useEffect(() => { loadStats(); loadLogs(); loadWords(); loadFlows() }, [])
  useEffect(() => { loadLogs() }, [logFilter])

  const loadStats = () => broodAPI.complianceStats().then(setStats).catch(() => {})
  const loadLogs = () => {
    broodAPI.complianceLogs({ ...logFilter, size: 50 }).then(r => {
      setLogs(r.logs || [])
      setLogsTotal(r.total)
    }).catch(() => {})
  }
  const loadWords = () => broodAPI.listSensitiveWords().then(r => setWords(r.rules || [])).catch(() => {})
  const loadFlows = () => broodAPI.listDataFlows().then(r => setFlows(r.flows || [])).catch(() => {})

  const handleResolve = async (id: string) => {
    await broodAPI.resolveComplianceLog(id)
    loadLogs()
    loadStats()
  }

  const handleExport = async () => {
    const data = await broodAPI.exportComplianceLogs()
    const blob = new Blob([JSON.stringify(data, null, 2)], { type: 'application/json' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `compliance-export-${new Date().toISOString().slice(0, 10)}.json`
    a.click()
    URL.revokeObjectURL(url)
    loadLogs()
  }

  const handleAddWord = async () => {
    if (!newWord.word.trim()) return
    await broodAPI.createSensitiveWord(newWord)
    setNewWord({ word: '', category: 'custom', action: 'log' })
    setShowAddWord(false)
    loadWords()
  }

  const handleDeleteWord = async (id: string) => {
    await broodAPI.deleteSensitiveWord(id)
    loadWords()
  }

  const handleAddFlow = async () => {
    if (!newFlow.source || !newFlow.destination) return
    await broodAPI.createDataFlow(newFlow)
    setNewFlow({ source: '', destination: '', data_type: '', encryption: 'tls_1.3', region: '', cross_border: false })
    setShowAddFlow(false)
    loadFlows()
  }

  const handleDeleteFlow = async (id: string) => {
    await broodAPI.deleteDataFlow(id)
    loadFlows()
  }

  return (
    <div className="p-8">
      <div className="max-w-6xl mx-auto space-y-6">
        <div className="flex items-center justify-between">
          <div>
            <h1 className="text-2xl font-bold text-white">合规中心</h1>
            <p className="text-sm text-gray-400 mt-1">数据安全合规 · 敏感词过滤 · 审计导出 · 数据流向</p>
          </div>
          <button onClick={handleExport}
            className="inline-flex items-center gap-2 rounded-lg border border-gray-700 px-4 py-2 text-sm text-gray-300 hover:bg-gray-800">
            <Download size={15} /> 导出审计
          </button>
        </div>

        {/* Stats cards */}
        {stats && (
          <div className="grid grid-cols-4 gap-4">
            <div className="rounded-xl border border-gray-800 bg-gray-900/50 p-4">
              <div className="text-xs text-gray-500">总事件</div>
              <div className="text-2xl font-bold text-white mt-1">{stats.total}</div>
            </div>
            <div className="rounded-xl border border-gray-800 bg-gray-900/50 p-4">
              <div className="text-xs text-gray-500">未处理</div>
              <div className="text-2xl font-bold text-amber-400 mt-1">{stats.unresolved}</div>
            </div>
            <div className="rounded-xl border border-gray-800 bg-gray-900/50 p-4">
              <div className="text-xs text-gray-500">严重告警</div>
              <div className="text-2xl font-bold text-red-400 mt-1">{stats.critical}</div>
            </div>
            <div className="rounded-xl border border-gray-800 bg-gray-900/50 p-4">
              <div className="text-xs text-gray-500">敏感词规则</div>
              <div className="text-2xl font-bold text-blue-400 mt-1">{words.length}</div>
            </div>
          </div>
        )}

        {/* 7-day chart */}
        {stats && stats.daily_7d && stats.daily_7d.length > 0 && (
          <div className="rounded-xl border border-gray-800 bg-gray-900/50 p-5">
            <h3 className="text-sm font-medium text-white mb-3">近 7 天事件趋势</h3>
            <div className="flex items-end gap-2 h-20">
              {stats.daily_7d.map(d => {
                const maxC = Math.max(...stats.daily_7d.map(x => x.count), 1)
                const h = Math.max((d.count / maxC) * 100, 4)
                return (
                  <div key={d.date} className="flex-1 flex flex-col items-center gap-1">
                    <span className="text-[10px] text-gray-500">{d.count}</span>
                    <div className="w-full rounded-t bg-overlord-500/40" style={{ height: `${h}%` }} />
                    <span className="text-[9px] text-gray-600">{d.date.slice(5)}</span>
                  </div>
                )
              })}
            </div>
          </div>
        )}

        {/* Tabs */}
        <div className="flex gap-1 border-b border-gray-800 pb-px">
          {([
            { key: 'overview', label: '事件日志' },
            { key: 'words', label: '敏感词规则' },
            { key: 'flows', label: '数据流向图' },
          ] as const).map(t => (
            <button key={t.key} onClick={() => setTab(t.key as typeof tab)}
              className={`px-4 py-2.5 text-sm rounded-t-lg transition-colors ${
                tab === t.key ? 'bg-gray-800 text-white font-medium' : 'text-gray-400 hover:text-gray-200 hover:bg-gray-800/50'
              }`}>
              {t.label}
            </button>
          ))}
        </div>

        {/* Logs tab */}
        {tab === 'overview' && (
          <div className="space-y-3">
            <div className="flex gap-2 flex-wrap">
              <select value={logFilter.severity || ''} onChange={e => setLogFilter({ ...logFilter, severity: e.target.value || undefined })}
                className="bg-gray-800 border border-gray-700 rounded-lg px-3 py-1.5 text-xs text-white">
                <option value="">全部级别</option>
                {Object.entries(SEV_MAP).map(([k, v]) => <option key={k} value={k}>{v.label}</option>)}
              </select>
              <select value={logFilter.event_type || ''} onChange={e => setLogFilter({ ...logFilter, event_type: e.target.value || undefined })}
                className="bg-gray-800 border border-gray-700 rounded-lg px-3 py-1.5 text-xs text-white">
                <option value="">全部类型</option>
                {Object.entries(TYPE_MAP).map(([k, v]) => <option key={k} value={k}>{v}</option>)}
              </select>
              <select value={logFilter.resolved || ''} onChange={e => setLogFilter({ ...logFilter, resolved: e.target.value || undefined })}
                className="bg-gray-800 border border-gray-700 rounded-lg px-3 py-1.5 text-xs text-white">
                <option value="">全部状态</option>
                <option value="false">未处理</option>
                <option value="true">已处理</option>
              </select>
              <span className="text-xs text-gray-500 self-center ml-2">共 {logsTotal} 条</span>
            </div>

            <div className="rounded-xl border border-gray-800 overflow-hidden">
              <table className="w-full text-xs">
                <thead>
                  <tr className="border-b border-gray-800 text-gray-400 text-left">
                    <th className="px-4 py-2.5 font-medium">时间</th>
                    <th className="px-4 py-2.5 font-medium">级别</th>
                    <th className="px-4 py-2.5 font-medium">类型</th>
                    <th className="px-4 py-2.5 font-medium">操作人</th>
                    <th className="px-4 py-2.5 font-medium">资源</th>
                    <th className="px-4 py-2.5 font-medium">状态</th>
                    <th className="px-4 py-2.5 font-medium">操作</th>
                  </tr>
                </thead>
                <tbody>
                  {logs.map(l => {
                    const sev = SEV_MAP[l.severity] || SEV_MAP.info
                    return (
                      <tr key={l.id} className="border-b border-gray-800/50 hover:bg-gray-800/30">
                        <td className="px-4 py-2.5 text-gray-400">{new Date(l.created_at).toLocaleString()}</td>
                        <td className="px-4 py-2.5"><span className={`px-1.5 py-0.5 rounded ${sev.color}`}>{sev.label}</span></td>
                        <td className="px-4 py-2.5 text-gray-300">{TYPE_MAP[l.event_type] || l.event_type}</td>
                        <td className="px-4 py-2.5 text-gray-400">{l.actor}</td>
                        <td className="px-4 py-2.5 text-gray-400 max-w-[200px] truncate">{l.resource || '-'}</td>
                        <td className="px-4 py-2.5">
                          {l.resolved
                            ? <span className="text-green-400">已处理</span>
                            : <span className="text-amber-400">待处理</span>}
                        </td>
                        <td className="px-4 py-2.5">
                          {!l.resolved && (
                            <button onClick={() => handleResolve(l.id)} className="text-overlord-400 hover:text-overlord-300">
                              <CheckCircle size={14} />
                            </button>
                          )}
                        </td>
                      </tr>
                    )
                  })}
                  {logs.length === 0 && (
                    <tr><td colSpan={7} className="px-4 py-8 text-center text-gray-600">暂无合规事件</td></tr>
                  )}
                </tbody>
              </table>
            </div>
          </div>
        )}

        {/* Sensitive words tab */}
        {tab === 'words' && (
          <div className="space-y-4">
            <div className="flex justify-end">
              <button onClick={() => setShowAddWord(!showAddWord)}
                className="inline-flex items-center gap-2 rounded-lg bg-overlord-600 px-4 py-2 text-xs text-white hover:bg-overlord-500">
                <Plus size={14} /> 添加敏感词
              </button>
            </div>

            {showAddWord && (
              <div className="rounded-xl border border-overlord-500/20 bg-overlord-500/5 p-4 flex gap-3 items-end">
                <div className="flex-1">
                  <label className="block text-xs text-gray-400 mb-1">敏感词</label>
                  <input value={newWord.word} onChange={e => setNewWord({ ...newWord, word: e.target.value })}
                    className="w-full rounded-lg border border-gray-700 bg-gray-800 px-3 py-2 text-sm text-white focus:outline-none" />
                </div>
                <div>
                  <label className="block text-xs text-gray-400 mb-1">分类</label>
                  <select value={newWord.category} onChange={e => setNewWord({ ...newWord, category: e.target.value })}
                    className="rounded-lg border border-gray-700 bg-gray-800 px-3 py-2 text-sm text-white">
                    {Object.entries(WORD_CATS).map(([k, v]) => <option key={k} value={k}>{v}</option>)}
                  </select>
                </div>
                <div>
                  <label className="block text-xs text-gray-400 mb-1">动作</label>
                  <select value={newWord.action} onChange={e => setNewWord({ ...newWord, action: e.target.value })}
                    className="rounded-lg border border-gray-700 bg-gray-800 px-3 py-2 text-sm text-white">
                    <option value="log">记录</option>
                    <option value="block">拦截</option>
                    <option value="mask">脱敏</option>
                  </select>
                </div>
                <button onClick={handleAddWord} className="rounded-lg bg-overlord-600 px-4 py-2 text-sm text-white hover:bg-overlord-500">添加</button>
              </div>
            )}

            <div className="rounded-xl border border-gray-800 overflow-hidden">
              <table className="w-full text-xs">
                <thead>
                  <tr className="border-b border-gray-800 text-gray-400 text-left">
                    <th className="px-4 py-2.5 font-medium">敏感词</th>
                    <th className="px-4 py-2.5 font-medium">分类</th>
                    <th className="px-4 py-2.5 font-medium">动作</th>
                    <th className="px-4 py-2.5 font-medium">状态</th>
                    <th className="px-4 py-2.5 font-medium">操作</th>
                  </tr>
                </thead>
                <tbody>
                  {words.map(w => {
                    const act = ACTION_MAP[w.action] || ACTION_MAP.log
                    return (
                      <tr key={w.id} className="border-b border-gray-800/50 hover:bg-gray-800/30">
                        <td className="px-4 py-2.5 text-white font-mono">{w.word}</td>
                        <td className="px-4 py-2.5 text-gray-400">{WORD_CATS[w.category] || w.category}</td>
                        <td className="px-4 py-2.5"><span className={act.color}>{act.label}</span></td>
                        <td className="px-4 py-2.5">{w.enabled ? <span className="text-green-400">启用</span> : <span className="text-gray-600">禁用</span>}</td>
                        <td className="px-4 py-2.5">
                          <button onClick={() => handleDeleteWord(w.id)} className="text-red-400 hover:text-red-300"><Trash2 size={13} /></button>
                        </td>
                      </tr>
                    )
                  })}
                  {words.length === 0 && (
                    <tr><td colSpan={5} className="px-4 py-8 text-center text-gray-600">暂无敏感词规则</td></tr>
                  )}
                </tbody>
              </table>
            </div>
          </div>
        )}

        {/* Data flows tab */}
        {tab === 'flows' && (
          <div className="space-y-4">
            <div className="flex justify-end">
              <button onClick={() => setShowAddFlow(!showAddFlow)}
                className="inline-flex items-center gap-2 rounded-lg bg-overlord-600 px-4 py-2 text-xs text-white hover:bg-overlord-500">
                <Plus size={14} /> 添加数据流
              </button>
            </div>

            {showAddFlow && (
              <div className="rounded-xl border border-overlord-500/20 bg-overlord-500/5 p-4 space-y-3">
                <div className="grid grid-cols-3 gap-3">
                  <div>
                    <label className="block text-xs text-gray-400 mb-1">数据来源</label>
                    <input value={newFlow.source} onChange={e => setNewFlow({ ...newFlow, source: e.target.value })}
                      placeholder="如: user_input"
                      className="w-full rounded-lg border border-gray-700 bg-gray-800 px-3 py-2 text-sm text-white focus:outline-none" />
                  </div>
                  <div>
                    <label className="block text-xs text-gray-400 mb-1">目的地</label>
                    <input value={newFlow.destination} onChange={e => setNewFlow({ ...newFlow, destination: e.target.value })}
                      placeholder="如: openai_api"
                      className="w-full rounded-lg border border-gray-700 bg-gray-800 px-3 py-2 text-sm text-white focus:outline-none" />
                  </div>
                  <div>
                    <label className="block text-xs text-gray-400 mb-1">数据类型</label>
                    <input value={newFlow.data_type} onChange={e => setNewFlow({ ...newFlow, data_type: e.target.value })}
                      placeholder="如: prompt"
                      className="w-full rounded-lg border border-gray-700 bg-gray-800 px-3 py-2 text-sm text-white focus:outline-none" />
                  </div>
                  <div>
                    <label className="block text-xs text-gray-400 mb-1">加密方式</label>
                    <select value={newFlow.encryption} onChange={e => setNewFlow({ ...newFlow, encryption: e.target.value })}
                      className="w-full rounded-lg border border-gray-700 bg-gray-800 px-3 py-2 text-sm text-white">
                      <option value="tls_1.3">TLS 1.3</option>
                      <option value="aes_256">AES-256</option>
                      <option value="none">无加密</option>
                    </select>
                  </div>
                  <div>
                    <label className="block text-xs text-gray-400 mb-1">地区</label>
                    <input value={newFlow.region} onChange={e => setNewFlow({ ...newFlow, region: e.target.value })}
                      placeholder="如: cn-east"
                      className="w-full rounded-lg border border-gray-700 bg-gray-800 px-3 py-2 text-sm text-white focus:outline-none" />
                  </div>
                  <div className="flex items-end">
                    <label className="flex items-center gap-2 text-sm text-gray-300 cursor-pointer pb-2">
                      <input type="checkbox" checked={newFlow.cross_border} onChange={e => setNewFlow({ ...newFlow, cross_border: e.target.checked })} />
                      跨境传输
                    </label>
                  </div>
                </div>
                <button onClick={handleAddFlow} className="rounded-lg bg-overlord-600 px-4 py-2 text-sm text-white hover:bg-overlord-500">添加</button>
              </div>
            )}

            {/* Visual data flow diagram */}
            {flows.length > 0 ? (
              <div className="space-y-3">
                {flows.map(f => (
                  <div key={f.id} className="rounded-xl border border-gray-800 bg-gray-900/50 p-4 flex items-center gap-4">
                    <div className="flex-1 flex items-center gap-3 min-w-0">
                      <div className="rounded-lg bg-blue-500/10 px-3 py-2 text-sm text-blue-300 font-mono whitespace-nowrap">{f.source}</div>
                      <div className="flex flex-col items-center">
                        <ArrowRight size={16} className="text-gray-500" />
                        <span className="text-[9px] text-gray-600">{f.encryption || 'n/a'}</span>
                      </div>
                      <div className="rounded-lg bg-green-500/10 px-3 py-2 text-sm text-green-300 font-mono whitespace-nowrap">{f.destination}</div>
                    </div>
                    <div className="flex items-center gap-4 text-xs shrink-0">
                      {f.data_type && <span className="text-gray-400">{f.data_type}</span>}
                      {f.region && <span className="text-gray-500">{f.region}</span>}
                      {f.cross_border && (
                        <span className="text-red-400 flex items-center gap-1">
                          <AlertTriangle size={12} /> 跨境
                        </span>
                      )}
                      <button onClick={() => handleDeleteFlow(f.id)} className="text-gray-600 hover:text-red-400">
                        <Trash2 size={13} />
                      </button>
                    </div>
                  </div>
                ))}
              </div>
            ) : (
              <div className="rounded-xl border border-gray-800 border-dashed p-12 text-center">
                <ShieldCheck className="w-10 h-10 text-gray-700 mx-auto mb-3" />
                <p className="text-gray-500">暂无数据流记录</p>
                <p className="text-xs text-gray-600 mt-1">添加数据流记录以满足合规审查要求</p>
              </div>
            )}
          </div>
        )}
      </div>
    </div>
  )
}
