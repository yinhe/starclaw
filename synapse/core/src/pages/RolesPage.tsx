import { useEffect, useState } from 'react';
import { admin, type Role, type User } from '../lib/api';

export default function RolesPage() {
  const [roles, setRoles] = useState<Role[]>([]);
  const [selectedRole, setSelectedRole] = useState<Role | null>(null);
  const [roleUsers, setRoleUsers] = useState<User[]>([]);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    admin.roles().then((res) => setRoles(res.roles || []));
  }, []);

  const selectRole = async (role: Role) => {
    setSelectedRole(role);
    setLoading(true);
    try {
      const res = await admin.getRole(role.id);
      setRoleUsers(res.users || []);
    } finally {
      setLoading(false);
    }
  };

  const handleRevoke = async (userId: string) => {
    if (!selectedRole) return;
    if (!confirm('确认移除该用户的此角色？')) return;
    await admin.revokeRole(userId, selectedRole.id);
    // Refresh
    const res = await admin.getRole(selectedRole.id);
    setRoleUsers(res.users || []);
  };

  return (
    <div>
      <h1 className="text-2xl font-bold mb-6">角色权限管理</h1>

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* Roles list */}
        <div className="space-y-3">
          <h2 className="text-sm font-medium text-gray-400 mb-2">角色列表</h2>
          {roles.map((role) => (
            <button
              key={role.id}
              onClick={() => selectRole(role)}
              className={`w-full text-left px-4 py-3 rounded-xl border transition-colors cursor-pointer ${
                selectedRole?.id === role.id
                  ? 'bg-rose-500/10 border-rose-500/30 text-rose-400'
                  : 'bg-gray-900 border-gray-800 text-gray-300 hover:border-gray-700'
              }`}
            >
              <div className="font-medium">{role.name}</div>
              <div className="text-xs text-gray-500 mt-0.5">{role.description}</div>
              <div className="text-xs text-gray-600 mt-1">
                {role.permissions?.length || 0} 项权限
              </div>
            </button>
          ))}
        </div>

        {/* Role detail */}
        <div className="lg:col-span-2">
          {selectedRole ? (
            <div className="space-y-6">
              {/* Permissions */}
              <div className="bg-gray-900 border border-gray-800 rounded-xl p-5">
                <h3 className="text-sm font-medium text-gray-400 mb-3">
                  {selectedRole.name} — 权限
                </h3>
                <div className="flex flex-wrap gap-2">
                  {selectedRole.permissions?.map((p) => (
                    <span
                      key={p.id}
                      className="inline-flex items-center gap-1 px-2.5 py-1 rounded-lg bg-gray-800 text-xs text-gray-300"
                    >
                      <span className="w-1.5 h-1.5 rounded-full bg-emerald-400" />
                      {p.name}
                      <span className="text-gray-600 ml-1">({p.description})</span>
                    </span>
                  ))}
                  {(!selectedRole.permissions || selectedRole.permissions.length === 0) && (
                    <span className="text-gray-500 text-sm">无权限</span>
                  )}
                </div>
              </div>

              {/* Assigned users */}
              <div className="bg-gray-900 border border-gray-800 rounded-xl p-5">
                <h3 className="text-sm font-medium text-gray-400 mb-3">
                  已分配用户 ({roleUsers.length})
                </h3>
                {loading ? (
                  <div className="text-gray-500 text-sm">加载中...</div>
                ) : roleUsers.length === 0 ? (
                  <div className="text-gray-500 text-sm">暂无用户分配此角色</div>
                ) : (
                  <div className="space-y-2">
                    {roleUsers.map((u) => (
                      <div
                        key={u.id}
                        className="flex items-center justify-between px-3 py-2 rounded-lg bg-gray-800/50"
                      >
                        <div>
                          <span className="text-white text-sm">{u.email}</span>
                          <span className="text-gray-500 text-xs ml-2">{u.name}</span>
                        </div>
                        <button
                          onClick={() => handleRevoke(u.id)}
                          className="text-xs text-red-400 hover:text-red-300 cursor-pointer"
                        >
                          移除
                        </button>
                      </div>
                    ))}
                  </div>
                )}
              </div>
            </div>
          ) : (
            <div className="bg-gray-900 border border-gray-800 rounded-xl p-8 text-center text-gray-500">
              选择一个角色查看详情
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
