import { useEffect, useState } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { ArrowLeft } from 'lucide-react';
import { admin, type User, type Role } from '../lib/api';

export default function UserDetailPage() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const [user, setUser] = useState<User | null>(null);
  const [keyCount, setKeyCount] = useState(0);
  const [requestCount, setRequestCount] = useState(0);
  const [orderCount, setOrderCount] = useState(0);
  const [saving, setSaving] = useState(false);
  const [msg, setMsg] = useState('');

  // Editable fields
  const [status, setStatus] = useState('');
  const [balance, setBalance] = useState('');
  const [freeQuota, setFreeQuota] = useState('');
  const [isAdmin, setIsAdmin] = useState(false);

  // RBAC
  const [allRoles, setAllRoles] = useState<Role[]>([]);
  const [userRoleIds, setUserRoleIds] = useState<string[]>([]);

  const loadUserRoles = async (userId: string) => {
    // Load all roles, then check which ones have this user
    const rolesRes = await admin.roles();
    const roles = rolesRes.roles || [];
    setAllRoles(roles);
    const assigned: string[] = [];
    for (const role of roles) {
      const r = await admin.getRole(role.id);
      if ((r.users || []).some((u: User) => u.id === userId)) {
        assigned.push(role.id);
      }
    }
    setUserRoleIds(assigned);
  };

  useEffect(() => {
    if (!id) return;
    admin.getUser(id).then((res) => {
      setUser(res.user);
      setKeyCount(res.api_key_count);
      setRequestCount(res.request_count);
      setOrderCount(res.order_count);
      setStatus(res.user.status);
      setBalance(String(res.user.balance));
      setFreeQuota(String(res.user.free_quota));
      setIsAdmin(res.user.is_admin);
    });
    loadUserRoles(id);
  }, [id]);

  const handleSave = async () => {
    if (!id) return;
    setSaving(true);
    setMsg('');
    try {
      await admin.updateUser(id, {
        status,
        balance: Number(balance),
        free_quota: Number(freeQuota),
        is_admin: isAdmin,
      });
      setMsg('已保存');
    } catch (err: unknown) {
      setMsg(err instanceof Error ? err.message : '保存失败');
    } finally {
      setSaving(false);
    }
  };

  if (!user) return <div className="text-gray-500">加载中...</div>;

  return (
    <div>
      <button onClick={() => navigate('/users')} className="flex items-center gap-1.5 text-sm text-gray-400 hover:text-white mb-4 cursor-pointer">
        <ArrowLeft className="w-4 h-4" /> 返回用户列表
      </button>

      <h1 className="text-2xl font-bold mb-6">{user.email}</h1>

      <div className="grid grid-cols-4 gap-4 mb-8">
        {[
          { label: '姓名', value: user.name },
          { label: 'API Keys', value: keyCount },
          { label: '请求数', value: requestCount.toLocaleString() },
          { label: '已付订单', value: orderCount },
        ].map(({ label, value }) => (
          <div key={label} className="bg-gray-900 border border-gray-800 rounded-xl p-4">
            <div className="text-xs text-gray-500 mb-1">{label}</div>
            <div className="text-lg font-semibold text-white">{value}</div>
          </div>
        ))}
      </div>

      <div className="bg-gray-900 border border-gray-800 rounded-xl p-6 space-y-4 max-w-lg">
        <h2 className="text-lg font-semibold mb-2">编辑用户</h2>

        <div>
          <label className="block text-sm text-gray-400 mb-1">状态</label>
          <select value={status} onChange={(e) => setStatus(e.target.value)} className="w-full bg-gray-800 border border-gray-700 rounded-lg px-3 py-2 text-white text-sm">
            <option value="active">active</option>
            <option value="disabled">disabled</option>
            <option value="suspended">suspended</option>
          </select>
        </div>

        <div>
          <label className="block text-sm text-gray-400 mb-1">余额 (分)</label>
          <input type="number" value={balance} onChange={(e) => setBalance(e.target.value)} className="w-full bg-gray-800 border border-gray-700 rounded-lg px-3 py-2 text-white text-sm" />
        </div>

        <div>
          <label className="block text-sm text-gray-400 mb-1">免费额度</label>
          <input type="number" value={freeQuota} onChange={(e) => setFreeQuota(e.target.value)} className="w-full bg-gray-800 border border-gray-700 rounded-lg px-3 py-2 text-white text-sm" />
        </div>

        <div className="flex items-center gap-2">
          <input type="checkbox" id="isAdmin" checked={isAdmin} onChange={(e) => setIsAdmin(e.target.checked)} className="rounded" />
          <label htmlFor="isAdmin" className="text-sm text-gray-300">管理员权限</label>
        </div>

        <div className="flex items-center gap-3">
          <button onClick={handleSave} disabled={saving} className="bg-rose-600 hover:bg-rose-700 text-white text-sm px-5 py-2 rounded-lg disabled:opacity-50 cursor-pointer">
            {saving ? '保存中...' : '保存'}
          </button>
          {msg && <span className="text-sm text-emerald-400">{msg}</span>}
        </div>
      </div>

      {/* RBAC Role Assignment */}
      <div className="bg-gray-900 border border-gray-800 rounded-xl p-6 mt-6 max-w-lg">
        <h2 className="text-lg font-semibold mb-3">角色分配</h2>
        <div className="space-y-2">
          {allRoles.map((role) => {
            const assigned = userRoleIds.includes(role.id);
            return (
              <div key={role.id} className="flex items-center justify-between px-3 py-2 rounded-lg bg-gray-800/50">
                <div>
                  <span className="text-sm text-white">{role.name}</span>
                  <span className="text-xs text-gray-500 ml-2">{role.description}</span>
                </div>
                <button
                  onClick={async () => {
                    if (!id) return;
                    if (assigned) {
                      await admin.revokeRole(id, role.id);
                    } else {
                      await admin.assignRole(id, role.id);
                    }
                    loadUserRoles(id);
                  }}
                  className={`text-xs px-3 py-1 rounded cursor-pointer ${
                    assigned
                      ? 'bg-red-500/10 text-red-400 hover:bg-red-500/20'
                      : 'bg-emerald-500/10 text-emerald-400 hover:bg-emerald-500/20'
                  }`}
                >
                  {assigned ? '移除' : '分配'}
                </button>
              </div>
            );
          })}
          {allRoles.length === 0 && (
            <div className="text-gray-500 text-sm">加载中...</div>
          )}
        </div>
      </div>
    </div>
  );
}
