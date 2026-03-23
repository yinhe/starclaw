import { useEffect, useState } from 'react'
import { Ticket, Plus, X, Copy, Trash2, Check } from 'lucide-react'
import { partner, type PartnerInvite } from '../lib/api'

const STATUS_MAP: Record<string, { text: string; color: string }> = {
  active: { text: '活跃', color: 'text-green-400 bg-green-500/10' },
  revoked: { text: '已撤销', color: 'text-red-400 bg-red-500/10' },
  expired: { text: '已过期', color: 'text-gray-400 bg-gray-500/10' },
}

export default function InvitesPage() {
  const [invites, setInvites] = useState<PartnerInvite[]>([])
  const [stats, setStats] = useState<{ total_invites: number; total_uses: number; active_invites: number; conversion_rate: string } | null>(null)
  const [showCreate, setShowCreate] = useState(false)
  const [form, setForm] = useState({ alias: '', label: '', max_uses: 1, region: '', comm_rate: 0.2, preset_name: '', preset_phone: '', preset_email: '' })
  const [creating, setCreating] = useState(false)
  const [error, setError] = useState('')
  const [copied, setCopied] = useState('')

  const load = () => {
    partner.listInvites().then(r => setInvites(r.invites || [])).catch(console.error)
    partner.inviteStats().then(r => setStats(r)).catch(console.error)
  }
  useEffect(() => { load() }, [])

  const handleCreate = async () => {
    setError('')
    setCreating(true)
    try {
      await partner.createInvite({
        alias: form.alias || undefined,
        label: form.label || undefined,
        max_uses: form.max_uses,
        region: form.region || undefined,
        comm_rate: form.comm_rate || undefined,
        preset_name: form.preset_name || undefined,
        preset_phone: form.preset_phone || undefined,
        preset_email: form.preset_email || undefined,
      })
      setShowCreate(false)
      setForm({ alias: '', label: '', max_uses: 1, region: '', comm_rate: 0.2, preset_name: '', preset_phone: '', preset_email: '' })
      load()
    } catch (err) {
      setError(err instanceof Error ? err.message : '创建失败')
    } finally {
      setCreating(false)
    }
  }

  const handleRevoke = async (id: string) => {
    if (!confirm('确定撤销此邀请码？已注册的用户不受影响。')) return
    await partner.revokeInvite(id)
    load()
  }

  const copyToClipboard = (text: string, id: string) => {
    navigator.clipboard.writeText(text)
    setCopied(id)
    setTimeout(() => setCopied(''), 2000)
  }

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-2xl font-bold text-claw-500">邀请码管理</h1>
          <p className="text-gray-500 text-sm mt-1">创建邀请码发展城市合伙人</p>
        </div>
        <button
          onClick={() => setShowCreate(true)}
          className="flex items-center gap-2 bg-claw-500 text-white px-4 py-2 rounded-lg hover:bg-claw-600 transition-colors"
        >
          <Plus size={16} /> 创建邀请码
        </button>
      </div>

      {/* Stats */}
      {stats && (
        <div className="grid grid-cols-4 gap-4 mb-6">
          {[
            { label: '总邀请码', value: stats.total_invites },
            { label: '活跃', value: stats.active_invites },
            { label: '已使用', value: stats.total_uses },
            { label: '转化率', value: stats.conversion_rate },
          ].map(s => (
            <div key={s.label} className="bg-gray-800/50 rounded-xl p-4 border border-white/5">
              <div className="text-xs text-gray-500">{s.label}</div>
              <div className="text-xl font-bold mt-1">{s.value}</div>
            </div>
          ))}
        </div>
      )}

      {/* Create Modal */}
      {showCreate && (
        <div className="fixed inset-0 bg-black/60 flex items-center justify-center z-50">
          <div className="bg-gray-900 rounded-2xl border border-white/10 p-6 w-full max-w-lg">
            <div className="flex items-center justify-between mb-4">
              <h2 className="text-lg font-semibold">创建城市合伙人邀请码</h2>
              <button onClick={() => setShowCreate(false)} className="text-gray-500 hover:text-white"><X size={20} /></button>
            </div>

            <div className="space-y-3">
              <div className="grid grid-cols-2 gap-3">
                <div>
                  <label className="text-xs text-gray-500">别名（可选，大写字母+数字+连字符）</label>
                  <input value={form.alias} onChange={e => setForm({ ...form, alias: e.target.value.toUpperCase() })}
                    placeholder="SC-HANGZHOU-001" className="w-full bg-gray-800 border border-white/10 rounded-lg px-3 py-2 text-sm mt-1" />
                </div>
                <div>
                  <label className="text-xs text-gray-500">区域</label>
                  <input value={form.region} onChange={e => setForm({ ...form, region: e.target.value })}
                    placeholder="杭州" className="w-full bg-gray-800 border border-white/10 rounded-lg px-3 py-2 text-sm mt-1" />
                </div>
              </div>

              <div>
                <label className="text-xs text-gray-500">备注（内部可见）</label>
                <input value={form.label} onChange={e => setForm({ ...form, label: e.target.value })}
                  placeholder="杭州代理-李四" className="w-full bg-gray-800 border border-white/10 rounded-lg px-3 py-2 text-sm mt-1" />
              </div>

              <div className="grid grid-cols-2 gap-3">
                <div>
                  <label className="text-xs text-gray-500">使用次数（1 = 一码一人）</label>
                  <input type="number" value={form.max_uses} onChange={e => setForm({ ...form, max_uses: parseInt(e.target.value) || 1 })}
                    min={1} className="w-full bg-gray-800 border border-white/10 rounded-lg px-3 py-2 text-sm mt-1" />
                </div>
                <div>
                  <label className="text-xs text-gray-500">佣金率</label>
                  <input type="number" value={form.comm_rate} onChange={e => setForm({ ...form, comm_rate: parseFloat(e.target.value) || 0 })}
                    step={0.05} min={0} max={1} className="w-full bg-gray-800 border border-white/10 rounded-lg px-3 py-2 text-sm mt-1" />
                </div>
              </div>

              <div className="border-t border-white/5 pt-3">
                <div className="text-xs text-gray-500 mb-2">预设候选人信息（可选，自动填入档案）</div>
                <div className="grid grid-cols-3 gap-3">
                  <input value={form.preset_name} onChange={e => setForm({ ...form, preset_name: e.target.value })}
                    placeholder="姓名" className="bg-gray-800 border border-white/10 rounded-lg px-3 py-2 text-sm" />
                  <input value={form.preset_phone} onChange={e => setForm({ ...form, preset_phone: e.target.value })}
                    placeholder="手机" className="bg-gray-800 border border-white/10 rounded-lg px-3 py-2 text-sm" />
                  <input value={form.preset_email} onChange={e => setForm({ ...form, preset_email: e.target.value })}
                    placeholder="邮箱" className="bg-gray-800 border border-white/10 rounded-lg px-3 py-2 text-sm" />
                </div>
              </div>

              {error && <div className="text-red-400 text-sm">{error}</div>}

              <button onClick={handleCreate} disabled={creating}
                className="w-full bg-claw-500 text-white py-2.5 rounded-lg hover:bg-claw-600 disabled:opacity-50 transition-colors font-medium">
                {creating ? '创建中...' : '创建邀请码'}
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
            <p>还没有邀请码</p>
            <p className="text-sm mt-1">点击「创建邀请码」开始发展城市合伙人</p>
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
                    <span className="text-xs text-gray-500">无限次</span>
                  ) : (
                    <span className="text-xs text-gray-500">{inv.used_count}/{inv.max_uses} 已使用</span>
                  )}
                </div>

                {inv.label && <div className="text-sm text-gray-500 mt-1">{inv.label}</div>}

                <div className="flex items-center gap-4 mt-2 text-xs text-gray-500">
                  {inv.region && <span>区域: {inv.region}</span>}
                  {inv.comm_rate > 0 && <span>佣金率: {(inv.comm_rate * 100).toFixed(0)}%</span>}
                  {inv.preset_name && <span>预设: {inv.preset_name}</span>}
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
