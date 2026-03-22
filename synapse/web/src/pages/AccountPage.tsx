import { useEffect, useState } from 'react';
import { User, Lock, Save, Check } from 'lucide-react';
import { dash } from '../lib/api';

export default function AccountPage() {
  const [name, setName] = useState('');
  const [email, setEmail] = useState('');
  const [createdAt, setCreatedAt] = useState('');
  const [saving, setSaving] = useState(false);
  const [profileMsg, setProfileMsg] = useState('');

  const [oldPwd, setOldPwd] = useState('');
  const [newPwd, setNewPwd] = useState('');
  const [confirmPwd, setConfirmPwd] = useState('');
  const [pwdLoading, setPwdLoading] = useState(false);
  const [pwdMsg, setPwdMsg] = useState('');
  const [pwdError, setPwdError] = useState('');

  useEffect(() => {
    dash.profile().then(r => {
      setName(r.user.name);
      setEmail(r.user.email);
      setCreatedAt(r.user.created_at);
    }).catch(console.error);
  }, []);

  const saveProfile = async () => {
    setSaving(true);
    setProfileMsg('');
    try {
      await dash.updateProfile({ name, email });
      setProfileMsg('已保存');
      setTimeout(() => setProfileMsg(''), 3000);
    } catch (err: unknown) {
      setProfileMsg(err instanceof Error ? err.message : 'Failed');
    } finally {
      setSaving(false);
    }
  };

  const changePassword = async (e: React.FormEvent) => {
    e.preventDefault();
    setPwdError('');
    setPwdMsg('');
    if (newPwd !== confirmPwd) {
      setPwdError('两次输入的密码不一致');
      return;
    }
    if (newPwd.length < 6) {
      setPwdError('新密码至少 6 位');
      return;
    }
    setPwdLoading(true);
    try {
      await dash.changePassword(oldPwd, newPwd);
      setPwdMsg('密码已修改');
      setOldPwd('');
      setNewPwd('');
      setConfirmPwd('');
      setTimeout(() => setPwdMsg(''), 3000);
    } catch (err: unknown) {
      setPwdError(err instanceof Error ? err.message : 'Failed');
    } finally {
      setPwdLoading(false);
    }
  };

  return (
    <div className="space-y-8">
      <div>
        <h1 className="text-2xl font-bold text-white">账户设置</h1>
        <p className="text-gray-400 text-sm mt-1">管理你的个人信息和安全设置</p>
      </div>

      {/* Profile */}
      <div className="bg-gray-900 border border-gray-800 rounded-xl p-6">
        <div className="flex items-center gap-3 mb-6">
          <div className="w-10 h-10 bg-blue-500/10 rounded-lg flex items-center justify-center">
            <User className="w-5 h-5 text-blue-400" />
          </div>
          <div>
            <h2 className="text-white font-semibold">个人信息</h2>
            <p className="text-gray-500 text-xs">注册于 {createdAt ? new Date(createdAt).toLocaleDateString() : '...'}</p>
          </div>
        </div>

        <div className="grid grid-cols-1 sm:grid-cols-2 gap-4 mb-4">
          <div>
            <label className="block text-sm text-gray-400 mb-1">昵称</label>
            <input
              value={name}
              onChange={e => setName(e.target.value)}
              className="w-full bg-gray-800 border border-gray-700 rounded-lg px-3 py-2 text-white text-sm focus:outline-none focus:border-amber-500 transition-colors"
            />
          </div>
          <div>
            <label className="block text-sm text-gray-400 mb-1">邮箱</label>
            <input
              type="email"
              value={email}
              onChange={e => setEmail(e.target.value)}
              className="w-full bg-gray-800 border border-gray-700 rounded-lg px-3 py-2 text-white text-sm focus:outline-none focus:border-amber-500 transition-colors"
            />
          </div>
        </div>

        <div className="flex items-center gap-3">
          <button
            onClick={saveProfile}
            disabled={saving}
            className="flex items-center gap-2 bg-amber-500 hover:bg-amber-400 disabled:opacity-50 text-gray-900 font-medium px-4 py-2 rounded-lg text-sm transition-colors cursor-pointer"
          >
            {saving ? '保存中...' : <><Save className="w-4 h-4" /> 保存</>}
          </button>
          {profileMsg && (
            <span className="flex items-center gap-1 text-green-400 text-sm">
              <Check className="w-4 h-4" /> {profileMsg}
            </span>
          )}
        </div>
      </div>

      {/* Change Password */}
      <div className="bg-gray-900 border border-gray-800 rounded-xl p-6">
        <div className="flex items-center gap-3 mb-6">
          <div className="w-10 h-10 bg-red-500/10 rounded-lg flex items-center justify-center">
            <Lock className="w-5 h-5 text-red-400" />
          </div>
          <div>
            <h2 className="text-white font-semibold">修改密码</h2>
            <p className="text-gray-500 text-xs">定期修改密码有助于保护账户安全</p>
          </div>
        </div>

        <form onSubmit={changePassword} className="space-y-4 max-w-sm">
          {pwdError && (
            <div className="bg-red-500/10 border border-red-500/20 text-red-400 text-sm px-3 py-2 rounded-lg">
              {pwdError}
            </div>
          )}
          {pwdMsg && (
            <div className="bg-green-500/10 border border-green-500/20 text-green-400 text-sm px-3 py-2 rounded-lg flex items-center gap-2">
              <Check className="w-4 h-4" /> {pwdMsg}
            </div>
          )}

          <div>
            <label className="block text-sm text-gray-400 mb-1">当前密码</label>
            <input
              type="password"
              value={oldPwd}
              onChange={e => setOldPwd(e.target.value)}
              required
              className="w-full bg-gray-800 border border-gray-700 rounded-lg px-3 py-2 text-white text-sm focus:outline-none focus:border-amber-500 transition-colors"
            />
          </div>
          <div>
            <label className="block text-sm text-gray-400 mb-1">新密码</label>
            <input
              type="password"
              value={newPwd}
              onChange={e => setNewPwd(e.target.value)}
              required
              minLength={6}
              className="w-full bg-gray-800 border border-gray-700 rounded-lg px-3 py-2 text-white text-sm focus:outline-none focus:border-amber-500 transition-colors"
              placeholder="至少 6 位"
            />
          </div>
          <div>
            <label className="block text-sm text-gray-400 mb-1">确认新密码</label>
            <input
              type="password"
              value={confirmPwd}
              onChange={e => setConfirmPwd(e.target.value)}
              required
              className="w-full bg-gray-800 border border-gray-700 rounded-lg px-3 py-2 text-white text-sm focus:outline-none focus:border-amber-500 transition-colors"
            />
          </div>

          <button
            type="submit"
            disabled={pwdLoading}
            className="flex items-center gap-2 bg-red-500 hover:bg-red-400 disabled:opacity-50 text-white font-medium px-4 py-2 rounded-lg text-sm transition-colors cursor-pointer"
          >
            {pwdLoading ? '修改中...' : '修改密码'}
          </button>
        </form>
      </div>
    </div>
  );
}
