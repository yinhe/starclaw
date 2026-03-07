import { useState, useEffect } from 'react'
import { Settings, User, Key, Shield, Loader2, Check, FileText } from 'lucide-react'
import { settingsAPI, auditAPI } from '../lib/api'

export default function SettingsPage() {
  const [profile, setProfile] = useState({ username: '', email: '', phone: '' })
  const [passwordForm, setPasswordForm] = useState({ old_password: '', new_password: '', confirm: '' })
  const [saving, setSaving] = useState(false)
  const [changingPwd, setChangingPwd] = useState(false)
  const [saveMsg, setSaveMsg] = useState('')
  const [pwdMsg, setPwdMsg] = useState('')
  const [apiKeys, setApiKeys] = useState<{ id: string; provider: string; display_name: string; api_key: string }[]>([])
  const [auditLogs, setAuditLogs] = useState<{ id: string; action: string; resource: string; resource_id: string; detail: string; ip: string; created_at: string }[]>([])

  useEffect(() => {
    loadProfile()
    loadAPIKeys()
    loadAuditLogs()
  }, [])

  const loadProfile = async () => {
    try {
      const res = await settingsAPI.getProfile()
      const u = res.data.user
      setProfile({ username: u.username || '', email: u.email || '', phone: u.phone || '' })
    } catch { /* ignore */ }
  }

  const loadAuditLogs = async () => {
    try {
      const res = await auditAPI.list()
      setAuditLogs(res.data.logs || [])
    } catch { /* ignore */ }
  }

  const loadAPIKeys = async () => {
    try {
      const res = await settingsAPI.getAPIKeys()
      setApiKeys(res.data.api_keys || [])
    } catch { /* ignore */ }
  }

  const handleSaveProfile = async () => {
    setSaving(true)
    setSaveMsg('')
    try {
      await settingsAPI.updateProfile(profile)
      setSaveMsg('保存成功')
      setTimeout(() => setSaveMsg(''), 2000)
    } catch {
      setSaveMsg('保存失败')
    }
    setSaving(false)
  }

  const handleChangePassword = async () => {
    if (passwordForm.new_password !== passwordForm.confirm) {
      setPwdMsg('两次密码不一致')
      return
    }
    if (passwordForm.new_password.length < 6) {
      setPwdMsg('密码至少 6 位')
      return
    }
    setChangingPwd(true)
    setPwdMsg('')
    try {
      await settingsAPI.changePassword({
        old_password: passwordForm.old_password,
        new_password: passwordForm.new_password,
      })
      setPwdMsg('密码已更新')
      setPasswordForm({ old_password: '', new_password: '', confirm: '' })
      setTimeout(() => setPwdMsg(''), 2000)
    } catch (e: unknown) {
      const err = e as { response?: { data?: { error?: string } } }
      setPwdMsg(err.response?.data?.error || '修改失败')
    }
    setChangingPwd(false)
  }

  return (
    <div className="h-full overflow-y-auto">
      <div className="max-w-3xl mx-auto p-8">
        <div className="mb-8">
          <h1 className="text-2xl font-bold text-gray-900 flex items-center gap-2">
            <Settings className="w-6 h-6" /> 设置
          </h1>
          <p className="text-gray-500 text-sm mt-1">管理账户和配置</p>
        </div>

        {/* Profile */}
        <section className="bg-white border rounded-xl p-6 mb-6">
          <h2 className="text-sm font-semibold text-gray-700 flex items-center gap-2 mb-4">
            <User className="w-4 h-4" /> 个人信息
          </h2>
          <div className="space-y-4">
            <div>
              <label className="block text-xs font-medium text-gray-600 mb-1">用户名</label>
              <input
                value={profile.username}
                onChange={(e) => setProfile({ ...profile, username: e.target.value })}
                className="w-full px-3 py-2 border rounded-lg text-sm outline-none focus:ring-2 focus:ring-primary-500"
              />
            </div>
            <div>
              <label className="block text-xs font-medium text-gray-600 mb-1">邮箱</label>
              <input
                value={profile.email}
                onChange={(e) => setProfile({ ...profile, email: e.target.value })}
                className="w-full px-3 py-2 border rounded-lg text-sm outline-none focus:ring-2 focus:ring-primary-500"
                type="email"
              />
            </div>
            <div>
              <label className="block text-xs font-medium text-gray-600 mb-1">手机号</label>
              <input
                value={profile.phone}
                onChange={(e) => setProfile({ ...profile, phone: e.target.value })}
                className="w-full px-3 py-2 border rounded-lg text-sm outline-none focus:ring-2 focus:ring-primary-500"
                type="tel"
                placeholder="绑定手机号后可用手机号登录"
              />
            </div>
            <div className="flex items-center gap-3">
              <button
                onClick={handleSaveProfile}
                disabled={saving}
                className="flex items-center gap-1.5 px-4 py-2 text-sm bg-primary-600 text-white rounded-lg hover:bg-primary-700 disabled:opacity-50"
              >
                {saving ? <Loader2 className="w-4 h-4 animate-spin" /> : <Check className="w-4 h-4" />}
                保存
              </button>
              {saveMsg && (
                <span className={`text-sm ${saveMsg === '保存成功' ? 'text-green-600' : 'text-red-600'}`}>
                  {saveMsg}
                </span>
              )}
            </div>
          </div>
        </section>

        {/* Password */}
        <section className="bg-white border rounded-xl p-6 mb-6">
          <h2 className="text-sm font-semibold text-gray-700 flex items-center gap-2 mb-4">
            <Shield className="w-4 h-4" /> 修改密码
          </h2>
          <div className="space-y-4">
            <div>
              <label className="block text-xs font-medium text-gray-600 mb-1">当前密码</label>
              <input
                value={passwordForm.old_password}
                onChange={(e) => setPasswordForm({ ...passwordForm, old_password: e.target.value })}
                type="password"
                className="w-full px-3 py-2 border rounded-lg text-sm outline-none focus:ring-2 focus:ring-primary-500"
              />
            </div>
            <div className="grid grid-cols-2 gap-4">
              <div>
                <label className="block text-xs font-medium text-gray-600 mb-1">新密码</label>
                <input
                  value={passwordForm.new_password}
                  onChange={(e) => setPasswordForm({ ...passwordForm, new_password: e.target.value })}
                  type="password"
                  className="w-full px-3 py-2 border rounded-lg text-sm outline-none focus:ring-2 focus:ring-primary-500"
                />
              </div>
              <div>
                <label className="block text-xs font-medium text-gray-600 mb-1">确认新密码</label>
                <input
                  value={passwordForm.confirm}
                  onChange={(e) => setPasswordForm({ ...passwordForm, confirm: e.target.value })}
                  type="password"
                  className="w-full px-3 py-2 border rounded-lg text-sm outline-none focus:ring-2 focus:ring-primary-500"
                />
              </div>
            </div>
            <div className="flex items-center gap-3">
              <button
                onClick={handleChangePassword}
                disabled={changingPwd || !passwordForm.old_password || !passwordForm.new_password}
                className="flex items-center gap-1.5 px-4 py-2 text-sm bg-primary-600 text-white rounded-lg hover:bg-primary-700 disabled:opacity-50"
              >
                {changingPwd ? <Loader2 className="w-4 h-4 animate-spin" /> : <Shield className="w-4 h-4" />}
                修改密码
              </button>
              {pwdMsg && (
                <span className={`text-sm ${pwdMsg === '密码已更新' ? 'text-green-600' : 'text-red-600'}`}>
                  {pwdMsg}
                </span>
              )}
            </div>
          </div>
        </section>

        {/* API Keys */}
        <section className="bg-white border rounded-xl p-6">
          <h2 className="text-sm font-semibold text-gray-700 flex items-center gap-2 mb-4">
            <Key className="w-4 h-4" /> API Keys
          </h2>
          <p className="text-xs text-gray-400 mb-4">
            在「模型」页面配置各 Provider 的 API Key。以下是已配置的模型列表。
          </p>
          {apiKeys.length === 0 ? (
            <p className="text-sm text-gray-400 text-center py-4">暂未配置模型，请前往「模型」页面添加</p>
          ) : (
            <div className="space-y-2">
              {apiKeys.map((k) => (
                <div key={k.id} className="flex items-center justify-between p-3 rounded-lg bg-gray-50">
                  <div>
                    <p className="text-sm font-medium text-gray-800">{k.display_name || k.provider}</p>
                    <p className="text-xs text-gray-400">{k.provider}</p>
                  </div>
                  <code className="text-xs bg-gray-200 px-2 py-1 rounded">{k.api_key || '未设置'}</code>
                </div>
              ))}
            </div>
          )}
        </section>

        {/* Audit Logs */}
        <section className="bg-white dark:bg-gray-800 border rounded-xl p-6">
          <h2 className="text-sm font-semibold text-gray-700 dark:text-gray-200 flex items-center gap-2 mb-4">
            <FileText className="w-4 h-4" /> 操作日志
          </h2>
          {auditLogs.length === 0 ? (
            <p className="text-sm text-gray-400 text-center py-4">暂无操作记录</p>
          ) : (
            <div className="space-y-1.5 max-h-64 overflow-y-auto">
              {auditLogs.map((log) => (
                <div key={log.id} className="flex items-center justify-between text-xs py-1.5 border-b border-gray-100 dark:border-gray-700 last:border-0">
                  <div className="flex items-center gap-2">
                    <span className="px-1.5 py-0.5 bg-gray-100 dark:bg-gray-700 rounded text-gray-600 dark:text-gray-300 font-mono">{log.action}</span>
                    {log.resource && <span className="text-gray-400">{log.resource}</span>}
                  </div>
                  <span className="text-gray-400">
                    {new Date(log.created_at).toLocaleString('zh-CN', { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' })}
                  </span>
                </div>
              ))}
            </div>
          )}
        </section>
      </div>
    </div>
  )
}
