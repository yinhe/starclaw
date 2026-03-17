import { useEffect, useState } from 'react'
import { Rocket, Plus, StopCircle, ExternalLink } from 'lucide-react'
import { partner, type Deployment } from '../lib/api'

const STATUS_MAP: Record<string, { text: string; color: string }> = {
  pending: { text: '等待中', color: 'text-yellow-400 bg-yellow-500/10' },
  provisioning: { text: '部署中', color: 'text-blue-400 bg-blue-500/10' },
  running: { text: '运行中', color: 'text-green-400 bg-green-500/10' },
  stopped: { text: '已停止', color: 'text-gray-400 bg-gray-500/10' },
  failed: { text: '失败', color: 'text-red-400 bg-red-500/10' },
}

export default function DeployPage() {
  const [deps, setDeps] = useState<Deployment[]>([])
  const [showAdd, setShowAdd] = useState(false)
  const [form, setForm] = useState({ client_name: '', type: 'docker', domain: '', admin_email: '', region: '' })

  const load = () => {
    partner.listDeployments().then(r => setDeps(r.deployments || [])).catch(console.error)
  }
  useEffect(() => { load() }, [])

  const handleCreate = async () => {
    if (!form.client_name || !form.admin_email) return
    await partner.createDeployment(form)
    setForm({ client_name: '', type: 'docker', domain: '', admin_email: '', region: '' })
    setShowAdd(false)
    load()
    // Poll for status update
    setTimeout(load, 3000)
  }

  const handleStop = async (id: string) => {
    await partner.stopDeployment(id)
    load()
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-white">一键部署</h1>
          <p className="text-sm text-gray-400 mt-1">为客户快速部署 Overlord 实例</p>
        </div>
        <button onClick={() => setShowAdd(!showAdd)}
          className="inline-flex items-center gap-2 rounded-lg bg-claw-600 px-4 py-2 text-sm font-medium text-white hover:bg-claw-500">
          <Plus size={16} /> 新建部署
        </button>
      </div>

      {showAdd && (
        <div className="rounded-xl border border-claw-500/20 bg-claw-500/5 p-5 space-y-4">
          <h3 className="text-sm font-medium text-white">部署配置</h3>
          <div className="grid grid-cols-2 gap-3">
            <input placeholder="客户名称 *" value={form.client_name}
              onChange={e => setForm({ ...form, client_name: e.target.value })}
              className="rounded-lg border border-white/10 bg-gray-900 px-3 py-2 text-sm text-white focus:outline-none focus:border-claw-500" />
            <input placeholder="管理员邮箱 *" value={form.admin_email}
              onChange={e => setForm({ ...form, admin_email: e.target.value })}
              className="rounded-lg border border-white/10 bg-gray-900 px-3 py-2 text-sm text-white focus:outline-none focus:border-claw-500" />
            <input placeholder="域名 (如 ai.client.com)" value={form.domain}
              onChange={e => setForm({ ...form, domain: e.target.value })}
              className="rounded-lg border border-white/10 bg-gray-900 px-3 py-2 text-sm text-white focus:outline-none focus:border-claw-500" />
            <select value={form.type} onChange={e => setForm({ ...form, type: e.target.value })}
              className="rounded-lg border border-white/10 bg-gray-900 px-3 py-2 text-sm text-white focus:outline-none focus:border-claw-500">
              <option value="docker">Docker Compose</option>
              <option value="k8s">Kubernetes</option>
              <option value="cloud">云市场</option>
            </select>
          </div>
          <div className="flex gap-2">
            <button onClick={handleCreate}
              className="rounded-lg bg-claw-600 px-4 py-2 text-sm text-white hover:bg-claw-500">
              开始部署
            </button>
            <button onClick={() => setShowAdd(false)}
              className="rounded-lg border border-white/10 px-4 py-2 text-sm text-gray-400 hover:text-white">
              取消
            </button>
          </div>
        </div>
      )}

      {deps.length === 0 ? (
        <div className="rounded-xl border border-white/10 border-dashed p-12 text-center">
          <Rocket className="w-10 h-10 text-gray-600 mx-auto mb-3" />
          <p className="text-gray-500">暂无部署实例</p>
          <p className="text-xs text-gray-600 mt-1">点击"新建部署"为客户部署 Overlord</p>
        </div>
      ) : (
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
          {deps.map(d => {
            const st = STATUS_MAP[d.status] || { text: d.status, color: 'text-gray-400 bg-gray-500/10' }
            return (
              <div key={d.id} className="rounded-xl border border-white/10 bg-white/[0.02] p-5">
                <div className="flex items-start justify-between mb-3">
                  <div>
                    <h3 className="text-sm font-medium text-white">{d.client_name}</h3>
                    <p className="text-xs text-gray-500 mt-0.5">{d.domain || '未配置域名'}</p>
                  </div>
                  <span className={`text-xs px-2 py-0.5 rounded ${st.color}`}>{st.text}</span>
                </div>

                <div className="grid grid-cols-2 gap-2 text-xs mb-3">
                  <div><span className="text-gray-500">类型:</span> <span className="text-gray-300">{d.type}</span></div>
                  <div><span className="text-gray-500">版本:</span> <span className="text-gray-300">{d.version}</span></div>
                  <div><span className="text-gray-500">管理员:</span> <span className="text-gray-300">{d.admin_email}</span></div>
                  <div><span className="text-gray-500">创建:</span> <span className="text-gray-300">{new Date(d.created_at).toLocaleDateString()}</span></div>
                </div>

                <div className="flex gap-2">
                  {d.health_url && d.status === 'running' && (
                    <a href={d.health_url} target="_blank" rel="noopener noreferrer"
                      className="inline-flex items-center gap-1 text-xs text-claw-400 hover:text-claw-300">
                      <ExternalLink size={12} /> 健康检查
                    </a>
                  )}
                  {d.status === 'running' && (
                    <button onClick={() => handleStop(d.id)}
                      className="inline-flex items-center gap-1 text-xs text-red-400 hover:text-red-300">
                      <StopCircle size={12} /> 停止
                    </button>
                  )}
                </div>
              </div>
            )
          })}
        </div>
      )}
    </div>
  )
}
