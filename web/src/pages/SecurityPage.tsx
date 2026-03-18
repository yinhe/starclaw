import { useState, useEffect } from 'react'
import { CheckCircle2, Download, Lock, RefreshCw, Shield, Trash2, XCircle } from 'lucide-react'
import { securityAPI } from '../lib/api'

type Tab = 'overview' | 'audit' | 'gdpr' | 'compliance'

export default function SecurityPage() {
  const [tab, setTab] = useState<Tab>('overview')
  const [overview, setOverview] = useState<any>(null)
  const [auditEntries, setAuditEntries] = useState<any[]>([])
  const [auditTotal, setAuditTotal] = useState(0)
  const [auditPage, setAuditPage] = useState(1)
  const [auditSeverity, setAuditSeverity] = useState('')
  const [chainValid, setChainValid] = useState<any>(null)
  const [compliance, setCompliance] = useState<any>(null)
  const [complianceFramework, setComplianceFramework] = useState('djcp')
  const [loading, setLoading] = useState(false)

  useEffect(() => { loadData() }, [tab, auditPage, auditSeverity, complianceFramework])

  const loadData = async () => {
    setLoading(true)
    try {
      if (tab === 'overview') {
        const r = await securityAPI.overview()
        setOverview(r.data)
      } else if (tab === 'audit') {
        const r = await securityAPI.auditQuery({ severity: auditSeverity || undefined, page: auditPage, page_size: 30 })
        setAuditEntries(r.data?.items || [])
        setAuditTotal(r.data?.total || 0)
      } else if (tab === 'gdpr') {
        // no pre-load needed
      } else if (tab === 'compliance') {
        const r = await securityAPI.compliance(complianceFramework)
        setCompliance(r.data)
      }
    } catch {}
    setLoading(false)
  }

  const verifyChain = async () => {
    const r = await securityAPI.auditVerify()
    setChainValid(r.data)
  }

  const exportAudit = async () => {
    try {
      const r = await securityAPI.auditExport()
      const url = URL.createObjectURL(new Blob([r.data as any]))
      const a = document.createElement('a'); a.href = url; a.download = 'audit-chain-export.json'; a.click()
    } catch {}
  }

  const exportGDPR = async () => {
    try {
      const r = await securityAPI.gdprExport()
      const url = URL.createObjectURL(new Blob([r.data as any]))
      const a = document.createElement('a'); a.href = url; a.download = 'gdpr-export.json'; a.click()
    } catch {}
  }

  const deleteGDPR = async () => {
    if (!confirm('⚠️ 这将永久删除你的所有数据！此操作不可逆。确认继续？')) return
    if (!confirm('再次确认：删除所有个人数据？')) return
    await securityAPI.gdprDelete(true)
    alert('数据已删除')
  }

  const severityColor: Record<string, string> = {
    info: 'bg-blue-100 text-blue-700', warning: 'bg-yellow-100 text-yellow-700', critical: 'bg-red-100 text-red-700',
  }

  return (
    <div className="p-6 max-w-7xl mx-auto">
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-2xl font-bold text-gray-900 dark:text-white flex items-center gap-2"><Shield className="w-6 h-6 text-primary-600" /> 安全中心</h1>
          <p className="text-sm text-gray-500 mt-1">加密状态、不可篡改审计链、GDPR 数据权利、合规清单</p>
        </div>
        <button onClick={loadData} className="flex items-center gap-2 px-3 py-2 text-sm bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg hover:bg-gray-50 dark:hover:bg-gray-700">
          <RefreshCw className={`w-4 h-4 ${loading ? 'animate-spin' : ''}`} />
        </button>
      </div>

      <div className="flex gap-1 mb-6 bg-gray-100 dark:bg-gray-800 p-1 rounded-lg w-fit">
        {([['overview', '总览'], ['audit', '审计链'], ['gdpr', '数据权利'], ['compliance', '合规清单']] as [Tab, string][]).map(([k, l]) => (
          <button key={k} onClick={() => setTab(k)}
            className={`px-3 py-1.5 text-sm rounded-md transition ${tab === k ? 'bg-white dark:bg-gray-700 shadow text-primary-600 font-medium' : 'text-gray-500 hover:text-gray-700'}`}>
            {l}
          </button>
        ))}
      </div>

      {/* Overview */}
      {tab === 'overview' && overview && (
        <div className="space-y-6">
          <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
            <div className="bg-white dark:bg-gray-800 rounded-xl p-5 border border-gray-200 dark:border-gray-700">
              <p className="text-sm text-gray-500">加密算法</p>
              <p className="text-lg font-bold mt-1 text-gray-900 dark:text-white flex items-center gap-1"><Lock className="w-4 h-4 text-green-500" /> {overview.encryption?.algorithm}</p>
            </div>
            <div className="bg-white dark:bg-gray-800 rounded-xl p-5 border border-gray-200 dark:border-gray-700">
              <p className="text-sm text-gray-500">密钥指纹</p>
              <p className="text-sm font-mono mt-1 text-gray-700 dark:text-gray-300">{overview.encryption?.key_fingerprint}</p>
            </div>
            <div className="bg-white dark:bg-gray-800 rounded-xl p-5 border border-gray-200 dark:border-gray-700">
              <p className="text-sm text-gray-500">审计链条目</p>
              <p className="text-3xl font-bold mt-1 text-gray-900 dark:text-white">{overview.audit_chain?.total_entries ?? 0}</p>
            </div>
            <div className="bg-white dark:bg-gray-800 rounded-xl p-5 border border-gray-200 dark:border-gray-700">
              <p className="text-sm text-gray-500">链完整性</p>
              <p className="text-lg font-bold mt-1 flex items-center gap-1">
                {overview.audit_chain?.chain_valid ? <><CheckCircle2 className="w-5 h-5 text-green-500" /> 完整</> : <><XCircle className="w-5 h-5 text-red-500" /> 异常</>}
              </p>
            </div>
          </div>
          <div className="grid grid-cols-3 gap-4">
            {['gdpr', 'djcp', 'soc2'].map(f => (
              <div key={f} className="bg-white dark:bg-gray-800 rounded-xl p-5 border border-gray-200 dark:border-gray-700">
                <p className="text-sm font-medium text-gray-900 dark:text-white">{f === 'djcp' ? '等保三级' : f.toUpperCase()}</p>
                <p className="text-sm text-gray-500 mt-1">{overview[`${f}_ready`] ? '✅ 就绪' : '⏳ 部分就绪'}</p>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Audit Chain */}
      {tab === 'audit' && (
        <div>
          <div className="flex gap-2 mb-4 items-center">
            <select value={auditSeverity} onChange={e => { setAuditSeverity(e.target.value); setAuditPage(1) }}
              className="px-3 py-2 text-sm bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg">
              <option value="">全部级别</option>
              <option value="info">Info</option><option value="warning">Warning</option><option value="critical">Critical</option>
            </select>
            <button onClick={verifyChain} className="px-3 py-2 text-sm bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg hover:bg-gray-50 flex items-center gap-1"><Shield className="w-4 h-4" /> 验证完整性</button>
            <button onClick={exportAudit} className="px-3 py-2 text-sm bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg hover:bg-gray-50 flex items-center gap-1"><Download className="w-4 h-4" /> 导出</button>
            {chainValid && (
              <span className={`ml-2 px-3 py-1.5 text-sm rounded-lg ${chainValid.valid ? 'bg-green-100 text-green-700' : 'bg-red-100 text-red-700'}`}>
                {chainValid.valid ? '✅ 链完整' : '❌ 链异常'} · {chainValid.entry_count} 条
              </span>
            )}
          </div>
          <div className="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 overflow-hidden">
            <table className="w-full text-sm">
              <thead className="bg-gray-50 dark:bg-gray-900 text-gray-500">
                <tr>
                  <th className="px-4 py-3 text-left font-medium">序号</th>
                  <th className="px-4 py-3 text-left font-medium">操作</th>
                  <th className="px-4 py-3 text-left font-medium">执行者</th>
                  <th className="px-4 py-3 text-left font-medium">目标</th>
                  <th className="px-4 py-3 text-left font-medium">级别</th>
                  <th className="px-4 py-3 text-left font-medium">时间</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-gray-100 dark:divide-gray-700">
                {auditEntries.length === 0 ? (
                  <tr><td colSpan={6} className="px-4 py-8 text-center text-gray-400">暂无审计记录</td></tr>
                ) : auditEntries.map((e: any) => (
                  <tr key={e.id} className="hover:bg-gray-50 dark:hover:bg-gray-700/50">
                    <td className="px-4 py-3 font-mono text-xs text-gray-400">#{e.sequence}</td>
                    <td className="px-4 py-3 font-medium">{e.action}</td>
                    <td className="px-4 py-3 text-gray-500">{e.actor?.slice(0, 12)}</td>
                    <td className="px-4 py-3 text-gray-500 text-xs">{e.target}</td>
                    <td className="px-4 py-3"><span className={`px-1.5 py-0.5 rounded text-xs ${severityColor[e.severity] || 'bg-gray-100 text-gray-600'}`}>{e.severity}</span></td>
                    <td className="px-4 py-3 text-gray-400 text-xs">{new Date(e.created_at).toLocaleString()}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          {auditTotal > 30 && (
            <div className="flex justify-center gap-2 mt-4">
              <button onClick={() => setAuditPage(p => Math.max(1, p - 1))} disabled={auditPage <= 1} className="px-3 py-1 text-sm border rounded disabled:opacity-30">上一页</button>
              <span className="px-3 py-1 text-sm text-gray-500">第 {auditPage} 页</span>
              <button onClick={() => setAuditPage(p => p + 1)} disabled={auditEntries.length < 30} className="px-3 py-1 text-sm border rounded disabled:opacity-30">下一页</button>
            </div>
          )}
        </div>
      )}

      {/* GDPR */}
      {tab === 'gdpr' && (
        <div className="max-w-2xl space-y-4">
          <div className="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-5">
            <h3 className="font-medium text-gray-900 dark:text-white mb-2">📦 数据导出 (Article 20)</h3>
            <p className="text-sm text-gray-500 mb-3">导出你的所有个人数据，包括 Agent、对话、工作流等。</p>
            <button onClick={exportGDPR} className="px-4 py-2 text-sm bg-primary-600 text-white rounded-lg hover:bg-primary-700 flex items-center gap-1.5"><Download className="w-4 h-4" /> 导出我的数据</button>
          </div>
          <div className="bg-white dark:bg-gray-800 rounded-xl border border-red-200 dark:border-red-900 p-5">
            <h3 className="font-medium text-red-600 mb-2">🗑️ 数据删除 (Article 17)</h3>
            <p className="text-sm text-gray-500 mb-3">永久删除你的所有个人数据。此操作不可逆！账户将被匿名化保留用于审计合规。</p>
            <button onClick={deleteGDPR} className="px-4 py-2 text-sm bg-red-600 text-white rounded-lg hover:bg-red-700 flex items-center gap-1.5"><Trash2 className="w-4 h-4" /> 删除我的数据</button>
          </div>
        </div>
      )}

      {/* Compliance */}
      {tab === 'compliance' && (
        <div>
          <div className="flex gap-2 mb-4">
            {[['djcp', '等保三级'], ['gdpr', 'GDPR'], ['soc2', 'SOC 2']].map(([k, l]) => (
              <button key={k} onClick={() => setComplianceFramework(k)}
                className={`px-4 py-2 text-sm rounded-lg border ${complianceFramework === k ? 'bg-primary-600 text-white border-primary-600' : 'bg-white dark:bg-gray-800 border-gray-200 dark:border-gray-700 text-gray-700 dark:text-gray-300 hover:bg-gray-50'}`}>
                {l}
              </button>
            ))}
          </div>
          {compliance && (
            <div className="space-y-4">
              <p className="text-lg font-medium text-gray-900 dark:text-white">{compliance.framework}</p>
              {(compliance.categories || compliance.articles || []).map((cat: any, ci: number) => (
                <div key={ci} className="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-4">
                  <h4 className="font-medium text-gray-900 dark:text-white mb-3">{cat.name || cat.article} {cat.title || ''}</h4>
                  {cat.items ? (
                    <div className="space-y-2">
                      {cat.items.map((item: any) => (
                        <div key={item.id} className="flex items-center justify-between py-1.5 border-b border-gray-100 dark:border-gray-700 last:border-0">
                          <div>
                            <span className="text-sm text-gray-700 dark:text-gray-300">{item.id}: {item.title}</span>
                            <p className="text-xs text-gray-400 mt-0.5">{item.note}</p>
                          </div>
                          <span className={`px-2 py-0.5 text-xs rounded shrink-0 ml-2 ${item.status === 'pass' ? 'bg-green-100 text-green-700' : item.status === 'partial' ? 'bg-yellow-100 text-yellow-700' : item.status === 'n/a' ? 'bg-gray-100 text-gray-500' : 'bg-orange-100 text-orange-700'}`}>{item.status}</span>
                        </div>
                      ))}
                    </div>
                  ) : (
                    <div className="flex items-center justify-between">
                      <p className="text-xs text-gray-400">{cat.note}</p>
                      <span className={`px-2 py-0.5 text-xs rounded ${cat.status === 'pass' ? 'bg-green-100 text-green-700' : cat.status === 'partial' ? 'bg-yellow-100 text-yellow-700' : 'bg-orange-100 text-orange-700'}`}>{cat.status}</span>
                    </div>
                  )}
                </div>
              ))}
            </div>
          )}
        </div>
      )}
    </div>
  )
}
