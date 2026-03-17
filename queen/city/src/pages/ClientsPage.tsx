import { useEffect, useState } from 'react'
import { Plus, UserCheck, UserX, Clock, Users } from 'lucide-react'
import { city, type Client } from '../lib/api'

const STATUS_LABELS: Record<string, { label: string; color: string }> = {
  lead: { label: '线索', color: 'text-gray-400 bg-gray-500/10' },
  trial: { label: '试用', color: 'text-blue-400 bg-blue-500/10' },
  active: { label: '活跃', color: 'text-green-400 bg-green-500/10' },
  churned: { label: '流失', color: 'text-red-400 bg-red-500/10' },
}

export default function ClientsPage() {
  const [clients, setClients] = useState<Client[]>([])
  const [filter, setFilter] = useState('')
  const [showAdd, setShowAdd] = useState(false)
  const [newName, setNewName] = useState('')
  const [newContact, setNewContact] = useState('')

  const load = () => {
    city.listClients(filter || undefined).then(r => setClients(r.clients || [])).catch(console.error)
  }

  useEffect(() => { load() }, [filter])

  const handleAdd = async () => {
    if (!newName.trim()) return
    await city.addClient({ client_name: newName, contact_info: newContact })
    setNewName('')
    setNewContact('')
    setShowAdd(false)
    load()
  }

  const updateStatus = async (id: string, status: string) => {
    await city.updateClient(id, { status })
    load()
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-white">我的客户</h1>
          <p className="text-sm text-gray-400 mt-1">{clients.length} 个客户</p>
        </div>
        <button
          onClick={() => setShowAdd(!showAdd)}
          className="inline-flex items-center gap-2 rounded-lg bg-claw-600 px-4 py-2 text-sm font-medium text-white hover:bg-claw-500 transition-colors"
        >
          <Plus size={16} />
          添加客户
        </button>
      </div>

      {/* Add form */}
      {showAdd && (
        <div className="rounded-xl border border-white/10 bg-white/[0.02] p-5 space-y-3">
          <div className="grid grid-cols-2 gap-3">
            <input
              placeholder="客户名称 *"
              value={newName}
              onChange={e => setNewName(e.target.value)}
              className="rounded-lg border border-white/10 bg-gray-900 px-3 py-2 text-sm text-white focus:outline-none focus:border-claw-500"
            />
            <input
              placeholder="联系方式"
              value={newContact}
              onChange={e => setNewContact(e.target.value)}
              className="rounded-lg border border-white/10 bg-gray-900 px-3 py-2 text-sm text-white focus:outline-none focus:border-claw-500"
            />
          </div>
          <button onClick={handleAdd} className="rounded-lg bg-claw-600 px-4 py-2 text-sm text-white hover:bg-claw-500">
            确认添加
          </button>
        </div>
      )}

      {/* Filter */}
      <div className="flex gap-2">
        {['', 'lead', 'trial', 'active', 'churned'].map(s => (
          <button
            key={s}
            onClick={() => setFilter(s)}
            className={`px-3 py-1.5 rounded-lg text-xs transition-colors ${
              filter === s ? 'bg-claw-500/10 text-claw-400' : 'text-gray-400 hover:text-white hover:bg-white/5'
            }`}
          >
            {s === '' ? '全部' : STATUS_LABELS[s]?.label || s}
          </button>
        ))}
      </div>

      {/* Table */}
      {clients.length === 0 ? (
        <div className="rounded-xl border border-white/10 border-dashed p-12 text-center">
          <Users className="w-10 h-10 text-gray-600 mx-auto mb-3" />
          <p className="text-gray-500">暂无客户</p>
        </div>
      ) : (
        <div className="rounded-xl border border-white/10 overflow-hidden">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-white/10 text-gray-400 text-left">
                <th className="px-4 py-3 font-medium">客户名称</th>
                <th className="px-4 py-3 font-medium">联系方式</th>
                <th className="px-4 py-3 font-medium">套餐</th>
                <th className="px-4 py-3 font-medium">状态</th>
                <th className="px-4 py-3 font-medium">签约时间</th>
                <th className="px-4 py-3 font-medium">操作</th>
              </tr>
            </thead>
            <tbody>
              {clients.map(c => {
                const st = STATUS_LABELS[c.status] || { label: c.status, color: 'text-gray-400 bg-gray-500/10' }
                return (
                  <tr key={c.id} className="border-b border-white/5 hover:bg-white/[0.02]">
                    <td className="px-4 py-3 text-white font-medium">{c.client_name}</td>
                    <td className="px-4 py-3 text-gray-400">{c.contact_info || '-'}</td>
                    <td className="px-4 py-3 text-gray-400">{c.plan || '-'}</td>
                    <td className="px-4 py-3">
                      <span className={`text-xs px-2 py-0.5 rounded ${st.color}`}>{st.label}</span>
                    </td>
                    <td className="px-4 py-3 text-gray-500">{c.signed_at ? new Date(c.signed_at).toLocaleDateString() : '-'}</td>
                    <td className="px-4 py-3">
                      <div className="flex gap-1">
                        {c.status === 'lead' && (
                          <button onClick={() => updateStatus(c.id, 'trial')} className="text-blue-400 hover:text-blue-300" title="转试用">
                            <Clock size={14} />
                          </button>
                        )}
                        {(c.status === 'lead' || c.status === 'trial') && (
                          <button onClick={() => updateStatus(c.id, 'active')} className="text-green-400 hover:text-green-300" title="转签约">
                            <UserCheck size={14} />
                          </button>
                        )}
                        {c.status === 'active' && (
                          <button onClick={() => updateStatus(c.id, 'churned')} className="text-red-400 hover:text-red-300" title="标记流失">
                            <UserX size={14} />
                          </button>
                        )}
                      </div>
                    </td>
                  </tr>
                )
              })}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}
