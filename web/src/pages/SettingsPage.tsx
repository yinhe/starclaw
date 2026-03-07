import { useState, useEffect } from 'react'
import { Settings, User, Key, Shield, Loader2, Check, FileText, Download, Globe, Coins, RefreshCw, Wifi, WifiOff, ArrowUpCircle, ExternalLink, Monitor, Plug, PlugZap, Eye, EyeOff } from 'lucide-react'
import { settingsAPI, auditAPI, systemAPI } from '../lib/api'

export default function SettingsPage() {
  const [profile, setProfile] = useState({ username: '', email: '', phone: '' })
  const [passwordForm, setPasswordForm] = useState({ old_password: '', new_password: '', confirm: '' })
  const [saving, setSaving] = useState(false)
  const [changingPwd, setChangingPwd] = useState(false)
  const [saveMsg, setSaveMsg] = useState('')
  const [pwdMsg, setPwdMsg] = useState('')
  const [apiKeys, setApiKeys] = useState<{ id: string; provider: string; display_name: string; api_key: string }[]>([])
  const [auditLogs, setAuditLogs] = useState<{ id: string; action: string; resource: string; resource_id: string; detail: string; ip: string; created_at: string }[]>([])

  // System state
  const [updateInfo, setUpdateInfo] = useState<any>(null)
  const [swarmStatus, setSwarmStatus] = useState<any>(null)
  const [updating, setUpdating] = useState(false)
  const [checking, setChecking] = useState(false)
  const [joiningSwarm, setJoiningSwarm] = useState(false)
  const [swarmForm, setSwarmForm] = useState({ queen_url: 'https://swarm.starclaw.me', node_name: '', region: '' })
  const [swarmMsg, setSwarmMsg] = useState('')
  const [updateMsg, setUpdateMsg] = useState('')
  const [bridgeStatus, setBridgeStatus] = useState<any>(null)
  const [overlordStatus, setOverlordStatus] = useState<any>(null)
  const [joiningOverlord, setJoiningOverlord] = useState(false)
  const [overlordForm, setOverlordForm] = useState({ overlord_url: 'https://overlord.starclaw.me', node_name: '', region: '' })
  const [overlordMsg, setOverlordMsg] = useState('')

  useEffect(() => {
    loadProfile()
    loadAPIKeys()
    loadAuditLogs()
    loadSystemInfo()
  }, [])

  const loadSystemInfo = async () => {
    try {
      const [updateRes, swarmRes, bridgeRes, overlordRes] = await Promise.all([
        systemAPI.getUpdate(),
        systemAPI.getSwarm(),
        systemAPI.getBridge().catch(() => null),
        systemAPI.getOverlord().catch(() => null),
      ])
      setUpdateInfo(updateRes.data)
      setSwarmStatus(swarmRes.data)
      if (bridgeRes) setBridgeStatus(bridgeRes.data)
      if (overlordRes) {
        setOverlordStatus(overlordRes.data)
        if (overlordRes.data.overlord_url) {
          setOverlordForm(prev => ({ ...prev, overlord_url: overlordRes.data.overlord_url }))
        }
      }
      if (swarmRes.data.queen_url) {
        setSwarmForm(prev => ({ ...prev, queen_url: swarmRes.data.queen_url }))
      }
    } catch { /* ignore */ }
  }

  const handleForceCheck = async () => {
    setChecking(true)
    try {
      await systemAPI.forceCheck()
      const res = await systemAPI.getUpdate()
      setUpdateInfo(res.data)
    } catch { /* ignore */ }
    setChecking(false)
  }

  const [updateStep, setUpdateStep] = useState(0) // 0=idle, 1=pulling, 2=building, 3=restarting, 4=verifying, 5=done

  const updateSteps = [
    '',
    '拉取最新代码...',
    '构建容器镜像...',
    '重启服务...',
    '等待 API 就绪...',
    '更新完成！',
  ]

  const handleTriggerUpdate = async () => {
    setUpdating(true)
    setUpdateMsg('')
    setUpdateStep(1)
    try {
      const res = await systemAPI.triggerUpdate()
      const targetVersion = res.data.to

      if (targetVersion) {
        // Simulate progress steps based on timing
        setTimeout(() => setUpdateStep(2), 8000)   // ~8s: building
        setTimeout(() => setUpdateStep(3), 60000)   // ~60s: restarting
        setTimeout(() => setUpdateStep(4), 90000)   // ~90s: verifying

        let attempts = 0
        let apiWasDown = false
        const poll = setInterval(async () => {
          attempts++
          if (attempts > 60) {
            clearInterval(poll)
            setUpdateMsg('更新超时，请手动检查服务器状态')
            setUpdateStep(0)
            setUpdating(false)
            return
          }
          try {
            const vRes = await systemAPI.getUpdate()
            const current = vRes.data?.version?.current
            if (current === targetVersion) {
              clearInterval(poll)
              setUpdateStep(5)
              setUpdateMsg(`✅ 已成功更新到 v${targetVersion}！`)
              setUpdateInfo(vRes.data)
              setTimeout(() => { setUpdateStep(0); setUpdating(false) }, 5000)
            } else if (apiWasDown) {
              // API came back but version didn't change yet, keep polling
              setUpdateStep(4)
            }
          } catch {
            apiWasDown = true
            setUpdateStep(3) // API is restarting
          }
        }, 5000)
      } else {
        setUpdateStep(0)
        setUpdating(false)
      }
    } catch {
      setUpdateMsg('更新失败')
      setUpdateStep(0)
      setUpdating(false)
    }
  }

  const handleJoinSwarm = async () => {
    if (!swarmForm.queen_url) return
    setJoiningSwarm(true)
    setSwarmMsg('')
    try {
      const res = await systemAPI.joinSwarm(swarmForm)
      setSwarmMsg(res.data.message || '已加入')
      loadSystemInfo()
      setTimeout(() => setSwarmMsg(''), 3000)
    } catch (e: any) {
      setSwarmMsg(e.response?.data?.error || '加入失败')
    }
    setJoiningSwarm(false)
  }

  const handleLeaveSwarm = async () => {
    try {
      await systemAPI.leaveSwarm()
      setSwarmMsg('已退出虫群')
      loadSystemInfo()
      setTimeout(() => setSwarmMsg(''), 3000)
    } catch { /* ignore */ }
  }

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

        {/* Version & Update */}
        <section className="bg-white border rounded-xl p-6 mb-6">
          <h2 className="text-sm font-semibold text-gray-700 flex items-center gap-2 mb-4">
            <ArrowUpCircle className="w-4 h-4" /> 版本与更新
          </h2>
          <div className="flex items-center justify-between p-4 bg-gray-50 rounded-lg mb-4">
            <div>
              <div className="flex items-center gap-2">
                <span className="text-lg font-bold text-gray-900">v{updateInfo?.version?.current || '...'}</span>
                {updateInfo?.version?.update_available && (
                  <span className="px-2 py-0.5 bg-orange-100 text-orange-700 text-xs rounded-full font-medium animate-pulse">有新版本 v{updateInfo.version.latest}</span>
                )}
                {updateInfo && !updateInfo.version?.update_available && (
                  <span className="px-2 py-0.5 bg-green-100 text-green-700 text-xs rounded-full">已是最新</span>
                )}
              </div>
              <div className="text-xs text-gray-400 mt-1">
                {updateInfo?.go_version} · {updateInfo?.os}/{updateInfo?.arch} · {updateInfo?.deploy_mode} 模式
              </div>
            </div>
            <div className="flex items-center gap-2">
              <button
                onClick={handleForceCheck}
                disabled={checking}
                className="flex items-center gap-1.5 px-3 py-1.5 text-xs border rounded-lg hover:bg-gray-100 disabled:opacity-50"
              >
                {checking ? <Loader2 className="w-3.5 h-3.5 animate-spin" /> : <RefreshCw className="w-3.5 h-3.5" />}
                检查更新
              </button>
              {updateInfo?.version?.update_available && (
                <button
                  onClick={handleTriggerUpdate}
                  disabled={updating}
                  className="flex items-center gap-1.5 px-3 py-1.5 text-xs bg-orange-500 text-white rounded-lg hover:bg-orange-600 disabled:opacity-50"
                >
                  {updating ? <Loader2 className="w-3.5 h-3.5 animate-spin" /> : <Download className="w-3.5 h-3.5" />}
                  一键更新
                </button>
              )}
            </div>
          </div>
          {updateStep > 0 && updateStep < 5 && (
            <div className="mb-4 p-4 bg-orange-50 rounded-lg border border-orange-200">
              <div className="flex items-center gap-3 mb-3">
                <Loader2 className="w-4 h-4 animate-spin text-orange-600" />
                <span className="text-sm font-medium text-orange-700">{updateSteps[updateStep]}</span>
              </div>
              <div className="flex gap-1">
                {[1,2,3,4].map(s => (
                  <div key={s} className="h-1.5 flex-1 rounded-full transition-all duration-500" style={{
                    backgroundColor: updateStep >= s ? '#ea580c' : '#fed7aa'
                  }} />
                ))}
              </div>
              <div className="flex justify-between mt-1 text-[10px] text-gray-400">
                <span>拉取</span><span>构建</span><span>重启</span><span>就绪</span>
              </div>
            </div>
          )}
          {updateStep === 5 && (
            <div className="mb-4 p-4 bg-green-50 rounded-lg border border-green-200">
              <div className="flex items-center gap-3">
                <Check className="w-4 h-4 text-green-600" />
                <span className="text-sm font-medium text-green-700">{updateMsg}</span>
              </div>
              <div className="flex gap-1 mt-2">
                {[1,2,3,4].map(s => (
                  <div key={s} className="h-1.5 flex-1 rounded-full bg-green-500" />
                ))}
              </div>
            </div>
          )}
          {updateMsg && updateStep === 0 && <p className="text-sm text-orange-600 mb-3">{updateMsg}</p>}
          {updateInfo?.version?.release_notes && updateInfo.version.update_available && (
            <div className="text-xs text-gray-500 bg-gray-50 rounded-lg p-3 max-h-32 overflow-y-auto">
              <p className="font-medium text-gray-600 mb-1">更新日志:</p>
              <pre className="whitespace-pre-wrap">{updateInfo.version.release_notes}</pre>
            </div>
          )}
          {updateInfo?.version?.latest_url && updateInfo.version.update_available && (
            <a href={updateInfo.version.latest_url} target="_blank" rel="noopener noreferrer" className="inline-flex items-center gap-1 text-xs text-primary-600 hover:underline mt-2">
              <ExternalLink className="w-3 h-3" /> 在 GitHub 查看
            </a>
          )}
        </section>

        {/* MCP Bridge */}
        <section className="bg-white border rounded-xl p-6 mb-6">
          <h2 className="text-sm font-semibold text-gray-700 flex items-center gap-2 mb-4">
            <Monitor className="w-4 h-4" /> 宿主机控制 (MCP Bridge)
          </h2>
          <p className="text-xs text-gray-400 mb-4">启用后，AI Agent 可以在对话中直接操作你的电脑：执行命令、读写文件、获取系统信息等。</p>
          <div className="flex items-center gap-3 p-3 rounded-lg mb-4" style={{ backgroundColor: bridgeStatus?.connected ? '#f0fdf4' : '#fef2f2' }}>
            {bridgeStatus?.connected ? (
              <PlugZap className="w-5 h-5 text-green-600" />
            ) : (
              <Plug className="w-5 h-5 text-gray-400" />
            )}
            <div>
              <p className="text-sm font-medium" style={{ color: bridgeStatus?.connected ? '#166534' : '#991b1b' }}>
                {bridgeStatus?.connected ? '已连接' : '未连接'}
              </p>
              {bridgeStatus?.connected && (
                <p className="text-xs text-gray-400">{bridgeStatus.bridge_url} · 9 个宿主机工具已注册</p>
              )}
            </div>
          </div>
          {!bridgeStatus?.connected && bridgeStatus?.downloads && (() => {
            const ua = navigator.userAgent.toLowerCase()
            let platform = 'linux_amd64'
            let label = 'Linux'
            if (ua.includes('win')) { platform = 'windows_amd64'; label = 'Windows' }
            else if (ua.includes('mac')) { platform = ua.includes('arm') ? 'darwin_arm64' : 'darwin_amd64'; label = 'macOS' }
            const url = bridgeStatus.downloads[platform]
            return (
              <div className="space-y-3">
                <a
                  href={url}
                  className="inline-flex items-center gap-2 px-4 py-2 bg-primary-600 text-white text-sm font-medium rounded-lg hover:bg-primary-700 transition-colors"
                >
                  <Download className="w-4 h-4" /> 下载 MCP Bridge ({label})
                </a>
                <div className="text-xs text-gray-500 bg-gray-50 rounded-lg p-3">
                  <p className="font-medium text-gray-600 mb-1">下载后运行：</p>
                  <code className="block bg-gray-100 px-2 py-1 rounded text-xs font-mono">
                    {platform.startsWith('windows') ? '.\\mcp-bridge-windows-amd64.exe' : `chmod +x mcp-bridge-${platform} && ./mcp-bridge-${platform}`}
                  </code>
                  <p className="mt-2 text-gray-400">启动后此页面会自动显示「已连接」，无需其他配置。</p>
                </div>
              </div>
            )
          })()}
        </section>

        {/* Overlord */}
        <section className="bg-white border rounded-xl p-6 mb-6">
          <h2 className="text-sm font-semibold text-gray-700 flex items-center gap-2 mb-4">
            <Eye className="w-4 h-4" /> 领主监控 (Overlord)
          </h2>
          <p className="text-xs text-gray-400 mb-4">接入领主后，你的 Claw 将受到资源配额管理、可观测性监控和任务调度保护。</p>
          <div className="flex items-center gap-3 p-3 rounded-lg mb-4" style={{ backgroundColor: overlordStatus?.connected ? '#f0fdf4' : '#fafafa' }}>
            {overlordStatus?.connected ? (
              <Eye className="w-5 h-5 text-violet-600" />
            ) : (
              <EyeOff className="w-5 h-5 text-gray-400" />
            )}
            <div>
              <p className="text-sm font-medium" style={{ color: overlordStatus?.connected ? '#166534' : '#6b7280' }}>
                {overlordStatus?.connected ? `已接入 — 节点 ${overlordStatus.node_id?.slice(0, 8)}...` : '未接入'}
              </p>
              {overlordStatus?.overlord_url && overlordStatus.connected && (
                <p className="text-xs text-gray-400">Overlord: {overlordStatus.overlord_url}</p>
              )}
            </div>
          </div>
          {!overlordStatus?.connected ? (
            <div className="space-y-3">
              <div>
                <label className="block text-xs font-medium text-gray-600 mb-1">领主地址</label>
                <input
                  value={overlordForm.overlord_url}
                  onChange={(e) => setOverlordForm({ ...overlordForm, overlord_url: e.target.value })}
                  className="w-full px-3 py-2 border rounded-lg text-sm outline-none focus:ring-2 focus:ring-primary-500"
                  placeholder="https://overlord.starclaw.me"
                />
              </div>
              <div className="grid grid-cols-2 gap-3">
                <div>
                  <label className="block text-xs font-medium text-gray-600 mb-1">节点名称 (可选)</label>
                  <input
                    value={overlordForm.node_name}
                    onChange={(e) => setOverlordForm({ ...overlordForm, node_name: e.target.value })}
                    className="w-full px-3 py-2 border rounded-lg text-sm outline-none focus:ring-2 focus:ring-primary-500"
                    placeholder="留空则使用主机名"
                  />
                </div>
                <div>
                  <label className="block text-xs font-medium text-gray-600 mb-1">地域 (可选)</label>
                  <input
                    value={overlordForm.region}
                    onChange={(e) => setOverlordForm({ ...overlordForm, region: e.target.value })}
                    className="w-full px-3 py-2 border rounded-lg text-sm outline-none focus:ring-2 focus:ring-primary-500"
                    placeholder="cn-east"
                  />
                </div>
              </div>
              <button
                onClick={async () => {
                  if (!overlordForm.overlord_url) return
                  setJoiningOverlord(true)
                  setOverlordMsg('')
                  try {
                    const res = await systemAPI.joinOverlord(overlordForm)
                    setOverlordMsg(res.data.message || '已接入')
                    loadSystemInfo()
                    setTimeout(() => setOverlordMsg(''), 3000)
                  } catch (e: any) {
                    setOverlordMsg(e.response?.data?.error || '接入失败')
                  }
                  setJoiningOverlord(false)
                }}
                disabled={joiningOverlord || !overlordForm.overlord_url}
                className="flex items-center gap-1.5 px-4 py-2 text-sm bg-violet-600 text-white rounded-lg hover:bg-violet-700 disabled:opacity-50"
              >
                {joiningOverlord ? <Loader2 className="w-4 h-4 animate-spin" /> : <Eye className="w-4 h-4" />}
                接入领主
              </button>
              {overlordMsg && <p className="text-sm text-violet-600">{overlordMsg}</p>}
            </div>
          ) : (
            <div className="flex items-center justify-between">
              <div className="text-xs text-gray-400">
                节点: {overlordStatus.node_name || '—'} · 地域: {overlordStatus.region || '—'}
              </div>
              <button
                onClick={async () => {
                  try {
                    await systemAPI.leaveOverlord()
                    setOverlordMsg('已退出领主监控')
                    loadSystemInfo()
                    setTimeout(() => setOverlordMsg(''), 3000)
                  } catch { /* ignore */ }
                }}
                className="px-3 py-1.5 text-xs text-red-600 border border-red-200 rounded-lg hover:bg-red-50"
              >
                退出领主
              </button>
            </div>
          )}
        </section>

        {/* Swarm */}
        <section className="bg-white border rounded-xl p-6 mb-6">
          <h2 className="text-sm font-semibold text-gray-700 flex items-center gap-2 mb-4">
            <Globe className="w-4 h-4" /> 虫群网络 (Swarm)
          </h2>
          <p className="text-xs text-gray-400 mb-4">加入虫群后，你的 Claw 节点将注册到 Queen 中心，获得远程管理、任务调度、自动更新等能力。</p>
          <div className="flex items-center gap-3 p-3 rounded-lg mb-4" style={{ backgroundColor: swarmStatus?.connected ? '#f0fdf4' : '#fafafa' }}>
            {swarmStatus?.connected ? (
              <Wifi className="w-5 h-5 text-green-600" />
            ) : (
              <WifiOff className="w-5 h-5 text-gray-400" />
            )}
            <div>
              <p className="text-sm font-medium" style={{ color: swarmStatus?.connected ? '#166534' : '#6b7280' }}>
                {swarmStatus?.connected ? `已连接 — 节点 ${swarmStatus.node_id?.slice(0, 8)}...` : '未连接'}
              </p>
              {swarmStatus?.queen_url && swarmStatus.connected && (
                <p className="text-xs text-gray-400">Queen: {swarmStatus.queen_url}</p>
              )}
            </div>
          </div>
          {!swarmStatus?.connected ? (
            <div className="space-y-3">
              <div>
                <label className="block text-xs font-medium text-gray-600 mb-1">Queen 地址</label>
                <input
                  value={swarmForm.queen_url}
                  onChange={(e) => setSwarmForm({ ...swarmForm, queen_url: e.target.value })}
                  className="w-full px-3 py-2 border rounded-lg text-sm outline-none focus:ring-2 focus:ring-primary-500"
                  placeholder="https://swarm.starclaw.me"
                />
              </div>
              <div className="grid grid-cols-2 gap-3">
                <div>
                  <label className="block text-xs font-medium text-gray-600 mb-1">节点名称 (可选)</label>
                  <input
                    value={swarmForm.node_name}
                    onChange={(e) => setSwarmForm({ ...swarmForm, node_name: e.target.value })}
                    className="w-full px-3 py-2 border rounded-lg text-sm outline-none focus:ring-2 focus:ring-primary-500"
                    placeholder="留空则使用主机名"
                  />
                </div>
                <div>
                  <label className="block text-xs font-medium text-gray-600 mb-1">地域 (可选)</label>
                  <input
                    value={swarmForm.region}
                    onChange={(e) => setSwarmForm({ ...swarmForm, region: e.target.value })}
                    className="w-full px-3 py-2 border rounded-lg text-sm outline-none focus:ring-2 focus:ring-primary-500"
                    placeholder="cn-east, us-west..."
                  />
                </div>
              </div>
              <button
                onClick={handleJoinSwarm}
                disabled={joiningSwarm || !swarmForm.queen_url}
                className="flex items-center gap-1.5 px-4 py-2 text-sm bg-primary-600 text-white rounded-lg hover:bg-primary-700 disabled:opacity-50"
              >
                {joiningSwarm ? <Loader2 className="w-4 h-4 animate-spin" /> : <Globe className="w-4 h-4" />}
                加入虫群
              </button>
            </div>
          ) : (
            <button
              onClick={handleLeaveSwarm}
              className="flex items-center gap-1.5 px-4 py-2 text-sm border border-red-200 text-red-600 rounded-lg hover:bg-red-50"
            >
              <WifiOff className="w-4 h-4" /> 退出虫群
            </button>
          )}
          {swarmMsg && <p className="text-sm text-green-600 mt-2">{swarmMsg}</p>}
        </section>

        {/* Bounty Network */}
        <section className="bg-white border rounded-xl p-6 mb-6">
          <h2 className="text-sm font-semibold text-gray-700 flex items-center gap-2 mb-4">
            <Coins className="w-4 h-4" /> 赏金网络 (Bounty)
          </h2>
          <p className="text-xs text-gray-400 mb-4">赏金网络允许你的 Agent 发布和领取任务，与全球其他 Claw 节点协作。加入虫群后自动开启。</p>
          <div className={`flex items-center gap-3 p-3 rounded-lg ${swarmStatus?.connected ? 'bg-green-50' : 'bg-gray-50'}`}>
            <Coins className={`w-5 h-5 ${swarmStatus?.connected ? 'text-green-600' : 'text-gray-400'}`} />
            <div>
              <p className={`text-sm font-medium ${swarmStatus?.connected ? 'text-green-800' : 'text-gray-500'}`}>
                {swarmStatus?.connected ? '赏金网络已激活' : '未激活 — 请先加入虫群'}
              </p>
              {swarmStatus?.connected && (
                <p className="text-xs text-gray-400">你的 Agent 可以通过工具发布和领取赏金任务</p>
              )}
            </div>
          </div>
        </section>

        {/* Queen Ecosystem Services */}
        <section className="bg-white border rounded-xl p-6 mb-6">
          <h2 className="text-sm font-semibold text-gray-700 flex items-center gap-2 mb-4">
            <Globe className="w-4 h-4" /> 生态服务
          </h2>
          <p className="text-xs text-gray-400 mb-4">StarClaw Queen 提供的公共服务，点击即可访问。</p>
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
            {[
              { name: '虫群网络', desc: '节点注册与心跳管理', url: 'https://swarm.starclaw.me', color: 'indigo', port: 8090 },
              { name: '领主监控', desc: '资源配额与可观测性', url: 'https://overlord.starclaw.me', color: 'violet', port: null },
              { name: '赏金网络', desc: 'Agent 任务发布与协作', url: 'https://bounty.starclaw.me', color: 'amber', port: 8092 },
              { name: '社区论坛', desc: '用户交流与经验分享', url: 'https://forum.starclaw.me', color: 'emerald', port: 8093 },
              { name: '机器人竞技场', desc: 'Agent 对战与排名', url: 'https://arena.starclaw.me', color: 'pink', port: 8094 },
              { name: '官方文档', desc: '部署指南与 API 参考', url: 'https://starclaw.me/docs', color: 'cyan', port: null },
            ].map((svc) => (
              <a
                key={svc.url}
                href={svc.url}
                target="_blank"
                rel="noopener noreferrer"
                className="flex items-center gap-3 p-3 rounded-lg border border-gray-100 hover:border-gray-200 hover:bg-gray-50 transition group"
              >
                <div className={`w-9 h-9 rounded-lg bg-${svc.color}-50 flex items-center justify-center flex-shrink-0`}>
                  <Globe className={`w-4 h-4 text-${svc.color}-500`} />
                </div>
                <div className="min-w-0 flex-1">
                  <p className="text-sm font-medium text-gray-800 flex items-center gap-1">
                    {svc.name}
                    <ExternalLink className="w-3 h-3 text-gray-300 group-hover:text-gray-500 transition" />
                  </p>
                  <p className="text-xs text-gray-400 truncate">{svc.desc}</p>
                </div>
                {svc.port && (
                  <span className="text-[10px] font-mono text-gray-300 flex-shrink-0">:{svc.port}</span>
                )}
              </a>
            ))}
          </div>
        </section>

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
        <section className="bg-white border rounded-xl p-6 mb-6">
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
