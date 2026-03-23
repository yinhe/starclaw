import { useEffect, useState } from 'react'
import { Ticket, Plus, X, Copy, Trash2, Check } from 'lucide-react'
import { city, type CityInvite } from '../lib/api'

const STATUS_MAP: Record<string, { text: string; color: string }> = {
  active: { text: '活跃', color: 'text-green-400 bg-green-500/10' },
  revoked: { text: '已撤销', color: 'text-red-400 bg-red-500/10' },
  expired: { text: '已过期', color: 'text-gray-400 bg-gray-500/10' },
}

export default function InvitesPage() {
  const [invites, setInvites] = useState<CityInvite[]>([])
  const [showCreate, setShowCreate] = useState(false)
  const [form, setForm] = useState({ alias: '', label: '', max_uses: 0 })
  const [creating, setCreating] = useState(false)
  const [error, setError] = useState('')
  const [copied, setCopied] = useState('')

  const load = () => {
    city.listInvites().then(r => setInvites(r.invites || [])).catch(console.error)
  }
  useEffect(() => { load() }, [])

  const handleCreate = async () => {
    setError('')
    setCreating(true)
    try {
      await city.createInvite({
        alias: form.alias || undefined,
        label: form.label || undefined,
        max_uses: form.max_uses,
      })
      setShowCreate(false)
      setForm({ alias: '', label: '', max_uses: 0 })
      load()
    } catch (err) {
      setError(err instanceof Error ? err.message : '创建失败')
    } finally {
      setCreating(false)
    }
  }

  const handleRevoke = async (id: string) => {
    if (!confirm('确定撤销此推荐码？已注册的用户不受影响。')) return
    await city.revokeInvite(id)
    load()
  }

  const copyToClipboard = (text: string, id: string) => {
    navigator.clipboard.writeText(text)
    setCopied(id)
    setTimeout(() => setCopied(''), 2000)
  }

  const activeCount = invites.filter(i => i.status === 'active').length
  const totalUsed = invites.reduce((sum, i) => sum + i.used_count, 0)

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-2xl font-bold text-claw-500">推荐码管理</h1>
          <p className="text-gray-500 text-sm mt-1">创建推荐码拉新用户，每个注册用户永久归属于你</p>
        </div>
        <button
          onClick={() => setShowCreate(true)}
          className="flex items-center gap-2 bg-claw-500 text-white px-4 py-2 rounded-lg hover:bg-claw-600 transition-colors"
        >
          <Plus size={16} /> 创建推荐码
        </button>
      </div>

      {/* Stats */}
      <div className="grid grid-cols-3 gap-4 mb-6">
        {[
          { label: '推荐码总数', value: invites.length },
          { label: '活跃', value: activeCount },
          { label: '总注册用户', value: totalUsed },
        ].map(s => (
          <div key={s.label} className="bg-gray-800/50 rounded-xl p-4 border border-white/5">
            <div className="text-xs text-gray-500">{s.label}</div>
            <div className="text-xl font-bold mt-1">{s.value}</div>
          </div>
        ))}
      </div>

      {/* Create Modal */}
      {showCreate && (
        <div className="fixed inset-0 bg-black/60 flex items-center justify-center z-50">
          <div className="bg-gray-900 rounded-2xl border border-white/10 p-6 w-full max-w-md">
            <div className="flex items-center justify-between mb-4">
              <h2 className="text-lg font-semibold">创建推荐码</h2>
              <button onClick={() => setShowCreate(false)} className="text-gray-500 hover:text-white"><X size={20} /></button>
            </div>

            <div className="space-y-3">
              <div>
                <label className="text-xs text-gray-500">别名（可选，大写字母+数字+连字符）</label>
                <input value={form.alias} onChange={e => setForm({ ...form, alias: e.target.value.toUpperCase() })}
                  placeholder="SC-HZ-VIP" className="w-full bg-gray-800 border border-white/10 rounded-lg px-3 py-2 text-sm mt-1" />
                <p className="text-xs text-gray-600 mt-1">不填则系统自动生成随机码</p>
              </div>

              <div>
                <label className="text-xs text-gray-500">备注（内部可见）</label>
                <input value={form.label} onChange={e => setForm({ ...form, label: e.target.value })}
                  placeholder="朋友圈推广专用" className="w-full bg-gray-800 border border-white/10 rounded-lg px-3 py-2 text-sm mt-1" />
              </div>

              <div>
                <label className="text-xs text-gray-500">使用次数（0 = 无限次）</label>
                <input type="number" value={form.max_uses} onChange={e => setForm({ ...form, max_uses: parseInt(e.target.value) || 0 })}
                  min={0} className="w-full bg-gray-800 border border-white/10 rounded-lg px-3 py-2 text-sm mt-1" />
                <p className="text-xs text-gray-600 mt-1">推荐设为 0（无限次），方便反复分享给不同客户</p>
              </div>

              {error && <div className="text-red-400 text-sm">{error}</div>}

              <button onClick={handleCreate} disabled={creating}
                className="w-full bg-claw-500 text-white py-2.5 rounded-lg hover:bg-claw-600 disabled:opacity-50 transition-colors font-medium">
                {creating ? '创建中...' : '创建推荐码'}
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Invites List */}
      <div className="space-y-3">
        {invites.length === 0 ? (
          <div className="text-center py-16 text-gray-500">
            <Ticket size={40} className="mx-auto mb-3 opacity-50" />
            <p>还没有推荐码</p>
            <p className="text-sm mt-1">点击「创建推荐码」开始拉新用户</p>
          </div>
        ) : invites.map(inv => (
          <div key={inv.id} className="bg-gray-800/50 rounded-xl border border-white/5 p-4">
            <div className="flex items-start justify-between">
              <div className="flex-1">
                <div className="flex items-center gap-3">
                  <span className="font-mono text-lg font-bold text-claw-400">{inv.display_code}</span>
                  <span className={`text-xs px-2 py-0.5 rounded-full ${STATUS_MAP[inv.status]?.color || 'text-gray-400'}`}>
                    {STATUS_MAP[inv.status]?.text || inv.status}
                  </span>
                  {inv.max_uses === 0 ? (
                    <span className="text-xs text-gray-500">无限次 · {inv.used_count} 人已注册</span>
                  ) : (
                    <span className="text-xs text-gray-500">{inv.used_count}/{inv.max_uses} 已使用</span>
                  )}
                </div>

                {inv.label && <div className="text-sm text-gray-500 mt-1">{inv.label}</div>}

                <div className="flex items-center gap-4 mt-2 text-xs text-gray-500">
                  <span>创建: {new Date(inv.created_at).toLocaleDateString()}</span>
                  {inv.expires_at && <span>过期: {new Date(inv.expires_at).toLocaleDateString()}</span>}
                </div>

                <div className="flex items-center gap-2 mt-2">
                  <code className="text-xs bg-gray-900 px-2 py-1 rounded text-gray-400 flex-1 truncate">{inv.join_url}</code>
                  <button
                    onClick={() => copyToClipboard(inv.join_url, inv.id)}
                    className="text-gray-500 hover:text-claw-400 transition-colors"
                    title="复制链接"
                  >
                    {copied === inv.id ? <Check size={14} className="text-green-400" /> : <Copy size={14} />}
                  </button>
                  <button
                    onClick={() => copyToClipboard(inv.display_code, inv.id + '-code')}
                    className="text-xs text-gray-500 hover:text-white bg-gray-800 px-2 py-1 rounded transition-colors"
                  >
                    {copied === inv.id + '-code' ? '已复制' : '复制码'}
                  </button>
                </div>
              </div>

              {inv.status === 'active' && (
                <button onClick={() => handleRevoke(inv.id)}
                  className="text-gray-500 hover:text-red-400 transition-colors ml-3" title="撤销">
                  <Trash2 size={16} />
                </button>
              )}
            </div>
          </div>
        ))}
      </div>
    </div>
  )
}
