import { useState, useEffect } from 'react'
import { Settings, User, Key, Shield, Loader2, Check, FileText, Download, Globe, Coins, RefreshCw, Wifi, WifiOff, ArrowUpCircle, ExternalLink, Monitor, Plug, PlugZap, Eye, EyeOff, Network, Trash2, Radio, Copy, Pencil, X, Link, ChevronDown, ChevronRight, Share2, AlertTriangle, Crown, LogOut } from 'lucide-react'
import { settingsAPI, auditAPI, systemAPI, nodeAPI, peerAPI, authAPI, queenAPI, deviceAPI } from '../lib/api'

export default function SettingsPage() {
  const [profile, setProfile] = useState({ username: '', email: '', phone: '' })
  const [hasPassword, setHasPassword] = useState(true)
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
  const [swarmForm, setSwarmForm] = useState({ queen_url: 'claw://swarm.starclaw.net', node_name: '', region: '', invite_code: '' })
  const [swarmMsg, setSwarmMsg] = useState('')
  const [updateMsg, setUpdateMsg] = useState('')
  const [bridgeStatus, setBridgeStatus] = useState<any>(null)
  const [overlordStatus, setOverlordStatus] = useState<any>(null)
  const [joiningOverlord, setJoiningOverlord] = useState(false)
  const [overlordForm, setOverlordForm] = useState({ overlord_url: '', node_name: '', region: '' })
  const [overlordMsg, setOverlordMsg] = useState('')

  // Queen Account state
  const [queenStatus, setQueenStatus] = useState<any>(null)
  const [queenLinking, setQueenLinking] = useState(false)
  const [queenMsg, setQueenMsg] = useState('')
  const [queenMsgType, setQueenMsgType] = useState<'success' | 'error'>('success')

  // Node & Peer state
  const [nodeInfo, setNodeInfo] = useState<any>(null)
  const [peers, setPeers] = useState<any[]>([])
  const [nodeForm, setNodeForm] = useState({ address: '', name: '', region: '' })
  const [peerAddr, setPeerAddr] = useState('')
  const [addingPeer, setAddingPeer] = useState(false)
  const [nodeMsg, setNodeMsg] = useState('')
  const [savingNode, setSavingNode] = useState(false)
  const [editingNode, setEditingNode] = useState(false)
  const [showNodeDetails, setShowNodeDetails] = useState(false)

  // API Token state
  const [myToken, setMyToken] = useState('')
  const [tokenNodeId, setTokenNodeId] = useState('')
  const [showToken, setShowToken] = useState(false)
  const [tokenCopied, setTokenCopied] = useState(false)
  const [regeneratingToken, setRegeneratingToken] = useState(false)
  const [devices, setDevices] = useState<{id: string; device_id: string; device_name: string; revoked: boolean; approved: boolean; last_used_at: string | null; created_at: string}[]>([])

  useEffect(() => {
    loadProfile()
    loadAPIKeys()
    loadAuditLogs()
    loadSystemInfo()
    loadMyToken()
    loadDevices()
    loadQueenStatus()
  }, [])

  const loadSystemInfo = async () => {
    try {
      const [updateRes, swarmRes, bridgeRes, overlordRes, nodeRes, peersRes] = await Promise.all([
        systemAPI.getUpdate(),
        systemAPI.getSwarm(),
        systemAPI.getBridge().catch(() => null),
        systemAPI.getOverlord().catch(() => null),
        nodeAPI.getInfo().catch(() => null),
        peerAPI.list().catch(() => null),
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
      if (nodeRes) {
        setNodeInfo(nodeRes.data)
        setNodeForm({ address: nodeRes.data.address || '', name: nodeRes.data.name || '', region: nodeRes.data.region || '' })
      }
      if (peersRes) setPeers(peersRes.data || [])
    } catch { /* ignore */ }
  }

  // Connect to peer: supports both network address and claw: Node ID
  const handleConnectPeer = async () => {
    if (!peerAddr) return
    setAddingPeer(true)
    try {
      let address = peerAddr.trim()
      // Detect claw: address → resolve to network address first
      if (address.startsWith('claw:')) {
        setNodeMsg('正在解析节点地址...')
        const res = await peerAPI.resolve(address)
        if (!res.data?.found) {
          const msg = res.data?.message || '无法解析该 Claw 地址 — 该节点不在已知网络中。'
          alert(msg + '\n\n也可以直接使用对方的 IP/域名连接。')
          setNodeMsg('')
          setAddingPeer(false)
          return
        }
        const sourceMap: Record<string, string> = { nydus: '本地', gossip: 'P2P网络', brood: '虫巢', swarm: '虫群' }
        const src = sourceMap[res.data.source] || res.data.source || ''
        address = res.data.address
        setNodeMsg(`已通过${src}解析: ${address}`)
      }
      await peerAPI.add({ address })
      setPeerAddr('')
      setNodeMsg('')
      loadSystemInfo()
    } catch (e: any) {
      alert(e.response?.data?.error || '链路建立失败')
      setNodeMsg('')
    }
    setAddingPeer(false)
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
    '拉取最新版本...',
    '安装更新...',
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
        // Progress simulation — Docker socket pull is fast (~30s), MCP Bridge build is slow (~3-5min)
        const method = res.data.method
        const isFast = method === 'spore' || method === 'standalone' // binary download ~10s
        setTimeout(() => setUpdateStep(2), isFast ? 3000 : 8000)
        setTimeout(() => setUpdateStep(3), isFast ? 8000 : 60000)
        setTimeout(() => setUpdateStep(4), isFast ? 12000 : 90000)

        let attempts = 0
        let apiWasDown = false
        const poll = setInterval(async () => {
          attempts++
          if (attempts > 120) { // 10 min timeout
            clearInterval(poll)
            setUpdateMsg('更新超时，请手动检查服务器状态。请稍后刷新页面。')
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
    } catch (e: any) {
      const errMsg = e.response?.data?.error || e.response?.data?.message || '更新失败'
      setUpdateMsg(`❌ ${errMsg}`)
      setUpdateStep(0)
      setUpdating(false)
    }
  }

  const loadQueenStatus = async () => {
    try {
      const res = await queenAPI.getStatus()
      setQueenStatus(res.data)
    } catch { /* ignore */ }
  }

  const handleQueenLinkClaw = async () => {
    setQueenLinking(true)
    setQueenMsg('')
    try {
      const res = await queenAPI.linkWithClaw()
      setQueenMsg(res.data.message || '已关联')
      setQueenMsgType('success')
      loadQueenStatus()
      setTimeout(() => setQueenMsg(''), 5000)
    } catch (e: any) {
      setQueenMsg(e.response?.data?.error || '关联失败')
      setQueenMsgType('error')
    }
    setQueenLinking(false)
  }

  const handleQueenUnlink = async () => {
    if (!confirm('解除关联后，赏金结算、社区互动等功能将不可用。确定？')) return
    try {
      await queenAPI.unlink()
      setQueenMsg('已解除关联')
      setQueenMsgType('success')
      setQueenStatus(null)
      loadQueenStatus()
      setTimeout(() => setQueenMsg(''), 3000)
    } catch { /* ignore */ }
  }

  const handleJoinSwarm = async () => {
    if (!swarmForm.queen_url) return
    setJoiningSwarm(true)
    setSwarmMsg('')
    try {
      const res = await systemAPI.joinSwarm(swarmForm)
      setSwarmMsg(res.data.message || '已加入')
      loadSystemInfo()

      // Auto-register with Queen (pass invite_code if provided)
      try {
        await queenAPI.autoRegister({ invite_code: swarmForm.invite_code || undefined })
        loadQueenStatus()
      } catch { /* auto-register is best-effort */ }

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
      setHasPassword(u.has_password !== false)
    } catch { /* ignore */ }
  }

  const loadAuditLogs = async () => {
    try {
      const res = await auditAPI.list()
      setAuditLogs(res.data.logs || [])
    } catch { /* ignore */ }
  }

  const loadMyToken = async () => {
    try {
      const res = await authAPI.getAPIToken()
      setMyToken(res.data.api_token || '')
      setTokenNodeId(res.data.node_id || '')
    } catch { /* ignore */ }
  }

  const loadDevices = async () => {
    try {
      const res = await deviceAPI.list()
      setDevices(res.data.devices || [])
    } catch { /* ignore */ }
  }

  const handleApproveDevice = async (id: string) => {
    try {
      await deviceAPI.approve(id)
      loadDevices()
    } catch { /* ignore */ }
  }

  const handleRejectDevice = async (id: string) => {
    if (!confirm('拒绝后该设备将无法使用 Token 登录。确定？')) return
    try {
      await deviceAPI.reject(id)
      loadDevices()
    } catch { /* ignore */ }
  }

  const handleRegenerateToken = async () => {
    if (!confirm('重新生成后，旧 Token 和所有已授权设备将被清除。确定？')) return
    setRegeneratingToken(true)
    try {
      const res = await authAPI.regenerateToken()
      const newToken = res.data.api_token
      setMyToken(newToken)
      setTokenNodeId(res.data.node_id || '')
      setShowToken(true)
      setDevices([])
      // Update localStorage so subsequent API calls use the new token
      localStorage.setItem('starclaw_token', newToken)
    } catch { /* ignore */ }
    setRegeneratingToken(false)
  }

  const handleRevokeDevice = async (deviceID: string) => {
    if (!confirm('撤销后该设备将无法使用 Token 登录。确定？')) return
    try {
      await authAPI.revokeDevice(deviceID)
      loadDevices()
    } catch { /* ignore */ }
  }

  const copyToken = () => {
    navigator.clipboard.writeText(myToken)
    setTokenCopied(true)
    setTimeout(() => setTokenCopied(false), 2000)
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
    } catch (err: unknown) {
      const axiosErr = err as { response?: { data?: { error?: string } } }
      setSaveMsg(axiosErr.response?.data?.error || '保存失败')
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
                {updateInfo?.go_version} · {updateInfo?.os}/{updateInfo?.arch} · {updateInfo?.runtime_mode === 'spore' ? 'Spore 本地部署' : updateInfo?.runtime_mode === 'docker' ? 'Docker 部署' : updateInfo?.deploy_mode} 模式
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

          {bridgeStatus?.connected ? (
            <>
              <div className="flex items-center justify-between p-3 rounded-lg bg-green-50 mb-3">
                <div className="flex items-center gap-3">
                  <PlugZap className="w-5 h-5 text-green-600" />
                  <div>
                    <p className="text-sm font-medium text-green-800">已连接</p>
                    <p className="text-xs text-gray-400">{bridgeStatus.bridge_url} · {bridgeStatus.tool_count || '?'} 个宿主机工具</p>
                  </div>
                </div>
                <button
                  onClick={async () => {
                    try {
                      await systemAPI.stopBridge()
                      setTimeout(() => loadSystemInfo(), 1000)
                    } catch {}
                  }}
                  className="px-3 py-1.5 text-xs font-medium text-red-600 bg-red-50 rounded-lg hover:bg-red-100 transition-colors"
                >
                  断开
                </button>
              </div>
            </>
          ) : (
            <>
              <div className="flex items-center gap-3 p-3 rounded-lg bg-gray-50 mb-4">
                <Plug className="w-5 h-5 text-gray-400" />
                <p className="text-sm font-medium text-gray-500">未连接</p>
              </div>
              {(() => {
                const ua = navigator.userAgent.toLowerCase()
                const isWindows = ua.includes('win')
                const isMac = ua.includes('mac')
                const label = isWindows ? 'Windows' : isMac ? 'macOS' : 'Linux'
                const apiBase = window.location.origin
                const bashCmd = `curl -fsSL ${apiBase}/v1/mcp-bridge/install.sh | bash`
                const psCmd = `irm ${apiBase}/v1/mcp-bridge/install.ps1 | iex`
                const installCmd = isWindows ? psCmd : bashCmd
                const terminalName = isWindows ? 'PowerShell' : '终端 (Terminal)'
                return (
                  <div className="space-y-3">
                    <p className="text-xs text-gray-600">首次使用？打开 <b>{terminalName}</b> 粘贴一行命令即可（安装后开机自启，无需再操作）：</p>
                    <div className="relative group">
                      <code className="block bg-gray-900 text-green-400 px-4 py-3 rounded-lg text-xs font-mono select-all break-all">{installCmd}</code>
                      <button
                        onClick={() => { navigator.clipboard.writeText(installCmd); }}
                        className="absolute top-1.5 right-1.5 p-1.5 rounded bg-gray-700 text-gray-300 hover:text-white hover:bg-gray-600 opacity-0 group-hover:opacity-100 transition-opacity"
                        title="复制"
                      >
                        <Copy className="w-3.5 h-3.5" />
                      </button>
                    </div>
                    <p className="text-xs text-gray-400">自动检测 {label} 系统 · 安装后开机自动连接，无需手动启动</p>
                  </div>
                )
              })()}
            </>
          )}
        </section>

        {/* Nydus Link — P2P Node Interconnection */}
        <section className="bg-white border rounded-xl p-6 mb-6">
          <h2 className="text-sm font-semibold text-gray-700 flex items-center gap-2 mb-1">
            <Network className="w-4 h-4" /> 虫洞链路 (Nydus)
          </h2>
          <p className="text-xs text-gray-400 mb-3">通过虫洞与其他 Claw 建立加密链路，实现任务委派、Agent 迁移、资源共享。</p>

          {/* Warning: address not set — show auto-detect buttons */}
          {nodeInfo && !nodeInfo.address && !editingNode && (
            <div className="p-4 mb-4 bg-amber-50 border border-amber-200 rounded-lg">
              <div className="flex items-center gap-2 mb-3">
                <AlertTriangle className="w-5 h-5 text-amber-500 shrink-0" />
                <p className="text-sm font-medium text-amber-800">选择一个地址，让其他 Claw 能连接到你</p>
              </div>
              <div className="flex flex-wrap gap-2 mb-2">
                {nodeInfo.public_ip && (
                  <button
                    onClick={async () => {
                      setSavingNode(true)
                      try {
                        const res = await nodeAPI.autoSetup({ use_public_ip: true })
                        setNodeMsg(`已配置: ${res.data.address}${res.data.region ? ` · ${res.data.region}` : ''}`)
                        setTimeout(() => setNodeMsg(''), 3000)
                        loadSystemInfo()
                      } catch { setNodeMsg('配置失败') }
                      setSavingNode(false)
                    }}
                    disabled={savingNode}
                    className="flex items-center gap-2 px-4 py-2.5 bg-violet-600 text-white text-sm rounded-lg hover:bg-violet-700 disabled:opacity-50"
                  >
                    {savingNode ? <Loader2 className="w-4 h-4 animate-spin" /> : <Globe className="w-4 h-4" />}
                    公网 IP: {nodeInfo.public_ip}
                  </button>
                )}
                {(nodeInfo.private_ips || []).map((ip: string) => (
                  <button
                    key={ip}
                    onClick={async () => {
                      setSavingNode(true)
                      try {
                        const res = await nodeAPI.autoSetup({ use_public_ip: false })
                        setNodeMsg(`已配置: ${res.data.address}${res.data.region ? ` · ${res.data.region}` : ''}`)
                        setTimeout(() => setNodeMsg(''), 3000)
                        loadSystemInfo()
                      } catch { setNodeMsg('配置失败') }
                      setSavingNode(false)
                    }}
                    disabled={savingNode}
                    className="flex items-center gap-2 px-4 py-2.5 bg-white text-gray-700 text-sm border border-gray-200 rounded-lg hover:bg-gray-50 disabled:opacity-50"
                  >
                    {savingNode ? <Loader2 className="w-4 h-4 animate-spin" /> : <Monitor className="w-4 h-4" />}
                    内网 IP: {ip}
                  </button>
                ))}
              </div>
              <p className="text-xs text-amber-600">点击即可一键配置，地域将根据 IP 自动检测。<button onClick={() => { setEditingNode(true); setNodeForm({ address: '', name: nodeInfo.name || '', region: nodeInfo.region || '' }) }} className="text-violet-600 hover:underline ml-1">手动输入域名</button></p>
              {nodeMsg && <p className="text-xs text-violet-600 mt-1">{nodeMsg}</p>}
            </div>
          )}

          {/* Node Identity Card */}
          {nodeInfo && (
            <div className="bg-gradient-to-r from-violet-50 to-indigo-50 border border-violet-200 rounded-lg p-4 mb-4">
              {/* Header */}
              <div className="flex items-center justify-between mb-2">
                <div className="flex items-center gap-2">
                  <Radio className="w-4 h-4 text-violet-600" />
                  <span className="text-sm font-semibold text-violet-800">本体 Claw</span>
                  <span className="text-xs bg-violet-100 text-violet-600 px-1.5 py-0.5 rounded flex items-center gap-1"><Shield className="w-3 h-3" /> 加密身份</span>
                </div>
                <div className="flex items-center gap-1.5">
                  {nodeInfo.address && (
                    <button
                      onClick={() => {
                        const text = `Claw 虫洞邀请\n地址: ${nodeInfo.address}\nNode ID: ${nodeInfo.node_id}\n指纹: ${nodeInfo.fingerprint}`
                        navigator.clipboard.writeText(text)
                        setNodeMsg('邀请信息已复制，发送给对方即可互联')
                        setTimeout(() => setNodeMsg(''), 3000)
                      }}
                      className="p-1 text-gray-400 hover:text-violet-600" title="复制邀请信息"
                    >
                      <Share2 className="w-3.5 h-3.5" />
                    </button>
                  )}
                  <button onClick={() => { setEditingNode(!editingNode); if (!editingNode) setNodeForm({ address: nodeInfo.address || '', name: nodeInfo.name || '', region: nodeInfo.region || '' }) }} className="p-1 text-gray-400 hover:text-violet-600" title="编辑配置">
                    {editingNode ? <X className="w-3.5 h-3.5" /> : <Pencil className="w-3.5 h-3.5" />}
                  </button>
                </div>
              </div>

              {!editingNode ? (
                <>
                  {/* Main info — clean 2-row layout */}
                  <div className="flex items-center justify-between text-xs mb-2">
                    <div className="flex items-center gap-4">
                      <span className="font-medium text-gray-700">{/^[0-9a-f]{10,}$/i.test(nodeInfo.name || '') ? '我的 Claw' : (nodeInfo.name || '我的 Claw')}</span>
                      <span className="text-gray-400">v{nodeInfo.version}</span>
                      {nodeInfo.region && <span className="text-gray-400">{nodeInfo.region}</span>}
                      {nodeInfo.peer_count > 0 && <span className="text-violet-600 font-medium">{nodeInfo.online_peers}/{nodeInfo.peer_count} 在线</span>}
                    </div>
                    <button onClick={() => { navigator.clipboard.writeText(nodeInfo.node_id); setNodeMsg('Node ID 已复制'); setTimeout(() => setNodeMsg(''), 1500) }} className="font-mono text-xs bg-violet-100 text-violet-700 px-2 py-0.5 rounded hover:bg-violet-200 cursor-pointer" title={nodeInfo.node_id}>{nodeInfo.node_id?.length > 20 ? nodeInfo.node_id.slice(0, 16) + '...' + nodeInfo.node_id.slice(-6) : nodeInfo.node_id}</button>
                  </div>

                  {/* Address line */}
                  {nodeInfo.address && (
                    <div className="flex items-center gap-2 text-xs mb-2">
                      <span className="text-gray-500">Nydus 入口:</span>
                      <span className="font-mono text-violet-700">{nodeInfo.address}</span>
                      <button onClick={() => { navigator.clipboard.writeText(nodeInfo.address); setNodeMsg('已复制'); setTimeout(() => setNodeMsg(''), 1500) }} className="text-gray-400 hover:text-gray-600"><Copy className="w-3 h-3" /></button>
                    </div>
                  )}

                  {nodeMsg && <p className="text-xs text-violet-600 mb-1">{nodeMsg}</p>}

                  {/* Tech details — collapsed by default */}
                  <button
                    onClick={() => setShowNodeDetails(!showNodeDetails)}
                    className="flex items-center gap-1 text-xs text-gray-400 hover:text-gray-600 mt-1"
                  >
                    {showNodeDetails ? <ChevronDown className="w-3 h-3" /> : <ChevronRight className="w-3 h-3" />}
                    技术详情
                  </button>
                  {showNodeDetails && (
                    <div className="mt-2 pt-2 border-t border-violet-100 space-y-1 text-xs">
                      <div className="flex items-center gap-2">
                        <span className="text-gray-400 w-16 shrink-0">基因指纹:</span>
                        <span className="font-mono text-gray-500 truncate">{nodeInfo.fingerprint}</span>
                        <button onClick={() => { navigator.clipboard.writeText(nodeInfo.fingerprint); setNodeMsg('已复制'); setTimeout(() => setNodeMsg(''), 1500) }} className="text-gray-400 hover:text-gray-600 shrink-0"><Copy className="w-3 h-3" /></button>
                      </div>
                      <div className="flex items-center gap-2">
                        <span className="text-gray-400 w-16 shrink-0">公钥算法:</span>
                        <span className="text-gray-500">Ed25519 (椭圆曲线签名)</span>
                      </div>
                      <div className="flex items-center gap-2">
                        <span className="text-gray-400 w-16 shrink-0">主机名:</span>
                        <span className="text-gray-500">{nodeInfo.hostname}</span>
                      </div>
                      <div className="flex items-center gap-2">
                        <span className="text-gray-400 w-16 shrink-0">系统:</span>
                        <span className="text-gray-500">{nodeInfo.os}/{nodeInfo.arch} · {nodeInfo.go_version}</span>
                      </div>
                      <div className="mt-2 p-2.5 bg-white/60 rounded border border-violet-100">
                        <p className="text-gray-500 font-medium mb-1">Ed25519 签名算法</p>
                        <p className="text-gray-400 leading-relaxed">Ed25519 是基于 Curve25519 椭圆曲线的数字签名算法，由 Daniel J. Bernstein 设计。相比 RSA，它的密钥更短（32 字节）、签名更快、安全性更高。SSH、Signal、区块链（Solana）等广泛采用。每个 Claw 启动时自动生成一对密钥：私钥永不离开本地，公钥用于身份验证。Node ID = "claw:" + SHA-256(公钥) 前40位 (160-bit)，与比特币同级地址空间，支持 10²⁴ 个唯一节点。输入对方的 claw: 地址即可自动解析并建立链路。地域信息可根据 IP 自动检测，无需手动选择。</p>
                      </div>
                    </div>
                  )}
                </>
              ) : (
                <div className="space-y-3 mt-1">
                  <p className="text-xs text-gray-500">选择一个地址，地域将自动检测：</p>
                  <div className="flex flex-wrap gap-2">
                    {nodeInfo.public_ip && (
                      <button
                        onClick={async () => {
                          setSavingNode(true)
                          try {
                            const res = await nodeAPI.autoSetup({ use_public_ip: true })
                            setNodeMsg(`已配置: ${res.data.address}${res.data.region ? ` · ${res.data.region}` : ''}`)
                            setTimeout(() => setNodeMsg(''), 3000)
                            setEditingNode(false)
                            loadSystemInfo()
                          } catch { setNodeMsg('配置失败') }
                          setSavingNode(false)
                        }}
                        disabled={savingNode}
                        className="flex items-center gap-2 px-3 py-2 bg-violet-600 text-white text-xs rounded-lg hover:bg-violet-700 disabled:opacity-50"
                      >
                        {savingNode ? <Loader2 className="w-3.5 h-3.5 animate-spin" /> : <Globe className="w-3.5 h-3.5" />}
                        公网: {nodeInfo.public_ip}
                      </button>
                    )}
                    {(nodeInfo.private_ips || []).map((ip: string) => (
                      <button
                        key={ip}
                        onClick={async () => {
                          setSavingNode(true)
                          try {
                            const res = await nodeAPI.autoSetup({ use_public_ip: false })
                            setNodeMsg(`已配置: ${res.data.address}${res.data.region ? ` · ${res.data.region}` : ''}`)
                            setTimeout(() => setNodeMsg(''), 3000)
                            setEditingNode(false)
                            loadSystemInfo()
                          } catch { setNodeMsg('配置失败') }
                          setSavingNode(false)
                        }}
                        disabled={savingNode}
                        className="flex items-center gap-2 px-3 py-2 bg-white text-gray-700 text-xs border border-gray-200 rounded-lg hover:bg-gray-50 disabled:opacity-50"
                      >
                        {savingNode ? <Loader2 className="w-3.5 h-3.5 animate-spin" /> : <Monitor className="w-3.5 h-3.5" />}
                        内网: {ip}
                      </button>
                    ))}
                    {!nodeInfo.public_ip && !(nodeInfo.private_ips || []).length && (
                      <p className="text-xs text-amber-600">未检测到可用 IP，请手动输入</p>
                    )}
                  </div>
                  <div className="flex items-center gap-2">
                    <span className="text-xs text-gray-400">或手动输入:</span>
                    <input value={nodeForm.address} onChange={(e) => setNodeForm({ ...nodeForm, address: e.target.value })} className="flex-1 px-2.5 py-1.5 border border-violet-200 rounded text-xs outline-none focus:ring-1 focus:ring-violet-400" placeholder="http://your-domain.com:8080" />
                    <button
                      onClick={async () => {
                        if (!nodeForm.address) return
                        setSavingNode(true)
                        try {
                          await nodeAPI.updateConfig({ address: nodeForm.address })
                          setNodeMsg('已保存')
                          setTimeout(() => setNodeMsg(''), 2000)
                          setEditingNode(false)
                          loadSystemInfo()
                        } catch { setNodeMsg('保存失败') }
                        setSavingNode(false)
                      }}
                      disabled={savingNode || !nodeForm.address}
                      className="flex items-center gap-1 px-3 py-1.5 text-xs bg-violet-600 text-white rounded hover:bg-violet-700 disabled:opacity-50"
                    >
                      {savingNode ? <Loader2 className="w-3 h-3 animate-spin" /> : <Check className="w-3 h-3" />} 保存
                    </button>
                    <button onClick={() => setEditingNode(false)} className="px-2 py-1.5 text-xs text-gray-500 hover:text-gray-700">取消</button>
                  </div>
                  {nodeMsg && <p className="text-xs text-violet-600">{nodeMsg}</p>}
                </div>
              )}
            </div>
          )}

          {/* Add Peer + Peer List */}
          {peers.length > 0 ? (
            <>
              <div className="flex gap-2 mb-3">
                <input
                  value={peerAddr}
                  onChange={(e) => setPeerAddr(e.target.value)}
                  onKeyDown={(e) => { if (e.key === 'Enter' && peerAddr) { (e.target as HTMLInputElement).blur(); document.getElementById('btn-nydus-link')?.click() } }}
                  className="flex-1 px-3 py-2 border rounded-lg text-sm outline-none focus:ring-2 focus:ring-violet-400"
                  placeholder="输入对方 claw: 地址，如 claw:b49edd9cebbc..."
                />
                <button
                  id="btn-nydus-link"
                  onClick={handleConnectPeer}
                  disabled={addingPeer || !peerAddr}
                  className="flex items-center gap-1.5 px-4 py-2 text-sm bg-violet-600 text-white rounded-lg hover:bg-violet-700 disabled:opacity-50 whitespace-nowrap"
                >
                  {addingPeer ? <Loader2 className="w-4 h-4 animate-spin" /> : <Link className="w-4 h-4" />}
                  建立链路
                </button>
              </div>
              <div className="space-y-1.5">
                {peers.map((peer: any) => (
                  <div key={peer.id} className="flex items-center justify-between px-3 py-2.5 border rounded-lg hover:bg-gray-50 group">
                    <div className="flex items-center gap-2.5 min-w-0">
                      <div className={`w-2 h-2 rounded-full shrink-0 ${peer.status === 'online' ? 'bg-green-500' : peer.status === 'offline' ? 'bg-red-400' : 'bg-gray-300'}`} />
                      <div className="min-w-0">
                        <div className="text-sm font-medium truncate">{/^[0-9a-f]{10,}$/i.test(peer.name || '') ? '远程 Claw' : (peer.name || '远程 Claw')} <span className="text-xs font-mono text-gray-400" title={peer.node_id}>{peer.node_id?.startsWith('claw:') ? peer.node_id.slice(0, 11) + '...' : peer.node_id?.slice(0, 8)}</span></div>
                        <div className="text-xs text-gray-400 truncate">{peer.address} · v{peer.version}{peer.region ? ` · ${peer.region}` : ''}</div>
                      </div>
                    </div>
                    <div className="flex items-center gap-1.5 shrink-0 ml-2">
                      <span className={`text-xs px-1.5 py-0.5 rounded ${peer.status === 'online' ? 'bg-green-100 text-green-700' : peer.status === 'offline' ? 'bg-red-100 text-red-600' : 'bg-gray-100 text-gray-500'}`}>
                        {peer.status === 'online' ? '在线' : peer.status === 'offline' ? '离线' : '未知'}
                      </span>
                      <button onClick={async () => { await peerAPI.ping(peer.id); loadSystemInfo() }} className="p-1 text-gray-300 hover:text-violet-600 opacity-0 group-hover:opacity-100 transition-opacity" title="探测">
                        <Wifi className="w-3.5 h-3.5" />
                      </button>
                      <button onClick={async () => { if (confirm('断开与该 Claw 的虫洞链路？')) { await peerAPI.remove(peer.id); loadSystemInfo() } }} className="p-1 text-gray-300 hover:text-red-500 opacity-0 group-hover:opacity-100 transition-opacity" title="断开链路">
                        <Trash2 className="w-3.5 h-3.5" />
                      </button>
                    </div>
                  </div>
                ))}
              </div>
            </>
          ) : (
            <div className="border border-dashed border-gray-200 rounded-lg p-5">
              <p className="text-xs font-medium text-gray-600 mb-3">如何与其他 Claw 建立虫洞链路？</p>
              <div className="space-y-2.5 mb-4">
                <div className="flex items-start gap-2.5 text-xs">
                  <span className="flex items-center justify-center w-5 h-5 rounded-full bg-violet-100 text-violet-600 font-semibold shrink-0">1</span>
                  <span className="text-gray-500 pt-0.5">{nodeInfo?.address ? <><Check className="w-3 h-3 text-green-500 inline mr-1" />已配置: <span className="font-mono text-violet-600">{nodeInfo.address}</span></> : <>点击上方按钮<strong className="text-gray-700">一键配置地址</strong>（自动检测IP和地域）</>}</span>
                </div>
                <div className="flex items-start gap-2.5 text-xs">
                  <span className="flex items-center justify-center w-5 h-5 rounded-full bg-violet-100 text-violet-600 font-semibold shrink-0">2</span>
                  <span className="text-gray-500 pt-0.5">{nodeInfo?.address ? <>点击 <Share2 className="w-3 h-3 inline text-violet-500" /> 复制你的 <strong className="text-gray-700">claw: 地址</strong>发给对方</> : <>复制你的 claw: 地址发给对方</>}</span>
                </div>
                <div className="flex items-start gap-2.5 text-xs">
                  <span className="flex items-center justify-center w-5 h-5 rounded-full bg-violet-100 text-violet-600 font-semibold shrink-0">3</span>
                  <span className="text-gray-500 pt-0.5">输入对方的 <strong className="text-gray-700">claw: 地址</strong>，自动解析并建立加密链路</span>
                </div>
              </div>
              <div className="flex gap-2">
                <input
                  value={peerAddr}
                  onChange={(e) => setPeerAddr(e.target.value)}
                  onKeyDown={(e) => { if (e.key === 'Enter' && peerAddr) { (e.target as HTMLInputElement).blur(); document.getElementById('btn-nydus-link-empty')?.click() } }}
                  className="flex-1 px-3 py-2 border rounded-lg text-sm outline-none focus:ring-2 focus:ring-violet-400"
                  placeholder="输入对方 claw: 地址，如 claw:b49edd9cebbc..."
                />
                <button
                  id="btn-nydus-link-empty"
                  onClick={handleConnectPeer}
                  disabled={addingPeer || !peerAddr}
                  className="flex items-center gap-1.5 px-4 py-2 text-sm bg-violet-600 text-white rounded-lg hover:bg-violet-700 disabled:opacity-50 whitespace-nowrap"
                >
                  {addingPeer ? <Loader2 className="w-4 h-4 animate-spin" /> : <Link className="w-4 h-4" />}
                  建立链路
                </button>
              </div>
            </div>
          )}
        </section>

        {/* Brood */}
        <section className="bg-white border rounded-xl p-6 mb-6">
          <h2 className="text-sm font-semibold text-gray-700 flex items-center gap-2 mb-4">
            <Eye className="w-4 h-4" /> 虫巢网络 (Brood)
          </h2>
          <p className="text-xs text-gray-400 mb-4">加入企业虫巢后，你的 Claw 将由领主 (Overlord) 统一管理，获得资源配额、可观测性监控和任务调度能力。领主地址由企业管理员提供。</p>
          <div className="flex items-center gap-3 p-3 rounded-lg mb-4" style={{ backgroundColor: overlordStatus?.connected ? '#f0fdf4' : '#fafafa' }}>
            {overlordStatus?.connected ? (
              <Eye className="w-5 h-5 text-violet-600" />
            ) : (
              <EyeOff className="w-5 h-5 text-gray-400" />
            )}
            <div>
              <p className="text-sm font-medium" style={{ color: overlordStatus?.connected ? '#166534' : '#6b7280' }}>
                {overlordStatus?.connected ? `已加入虫巢 — ${overlordStatus.node_id?.startsWith('claw:') ? overlordStatus.node_id.slice(0, 16) + '...' : overlordStatus.node_id?.slice(0, 8) + '...'}` : '未加入'}
              </p>
              {overlordStatus?.overlord_url && overlordStatus.connected && (
                <p className="text-xs text-gray-400">Overlord: {overlordStatus.overlord_url}</p>
              )}
            </div>
          </div>
          {!overlordStatus?.connected ? (
            <div className="space-y-3">
              <div>
                <label className="block text-xs font-medium text-gray-600 mb-1">领主地址（由企业管理员提供）</label>
                <input
                  value={overlordForm.overlord_url}
                  onChange={(e) => setOverlordForm({ ...overlordForm, overlord_url: e.target.value })}
                  className="w-full px-3 py-2 border rounded-lg text-sm outline-none focus:ring-2 focus:ring-primary-500"
                  placeholder="例如 http://192.168.1.100:8095 或 https://overlord.company.com"
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
                  <label className="block text-xs font-medium text-gray-600 mb-1">地域 (可选，留空自动检测)</label>
                  <select
                    value={overlordForm.region}
                    onChange={(e) => setOverlordForm({ ...overlordForm, region: e.target.value })}
                    className="w-full px-3 py-2 border rounded-lg text-sm outline-none focus:ring-2 focus:ring-primary-500 bg-white"
                  >
                    <option value="">自动检测...</option>
                    <option value="local">local (局域网)</option>
                    <option value="cn-east">cn-east (华东)</option>
                    <option value="cn-south">cn-south (华南)</option>
                    <option value="cn-north">cn-north (华北)</option>
                    <option value="cn-central">cn-central (华中)</option>
                    <option value="cn-southwest">cn-southwest (西南)</option>
                    <option value="hk">香港</option>
                    <option value="us-west">us-west (美西)</option>
                    <option value="us-east">us-east (美东)</option>
                    <option value="eu-west">eu-west (西欧)</option>
                    <option value="ap-southeast">ap-southeast (东南亚)</option>
                    <option value="jp">日本</option>
                  </select>
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
                加入虫巢
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
                退出虫巢
              </button>
            </div>
          )}
        </section>

        {/* Swarm */}
        <section className="bg-white border rounded-xl p-6 mb-6">
          <h2 className="text-sm font-semibold text-gray-700 flex items-center gap-2 mb-4">
            <Globe className="w-4 h-4" /> 虫群网络 (Swarm)
          </h2>
          <p className="text-xs text-gray-400 mb-4">加入虫群后，你的 Claw 将进入 StarClaw 生态，获得赏金任务协作、Agent 模板市场、自动版本更新、社区排行榜等生态服务。</p>
          <div className="flex items-center gap-3 p-3 rounded-lg mb-4" style={{
            backgroundColor: swarmStatus?.state === 'connected' ? '#f0fdf4' : swarmStatus?.state === 'feral' ? '#fffbeb' : '#fafafa'
          }}>
            {swarmStatus?.state === 'connected' ? (
              <Wifi className="w-5 h-5 text-green-600" />
            ) : swarmStatus?.state === 'feral' ? (
              <AlertTriangle className="w-5 h-5 text-amber-500 animate-pulse" />
            ) : (
              <WifiOff className="w-5 h-5 text-gray-400" />
            )}
            <div className="flex-1">
              <p className="text-sm font-medium" style={{
                color: swarmStatus?.state === 'connected' ? '#166534' : swarmStatus?.state === 'feral' ? '#92400e' : '#6b7280'
              }}>
                {swarmStatus?.state === 'connected'
                  ? `已连接 — ${swarmStatus.node_id?.startsWith('claw:') ? swarmStatus.node_id.slice(0, 16) + '...' : swarmStatus.node_id?.slice(0, 8) + '...'}`
                  : swarmStatus?.state === 'feral'
                  ? `失控模式 (Feral) — 与 Queen 失联，自主运行中`
                  : '未连接'}
              </p>
              {swarmStatus?.queen_url && swarmStatus.connected && (
                <p className="text-xs text-gray-400">Queen: {swarmStatus.queen_url}</p>
              )}
              {swarmStatus?.state === 'feral' && (
                <p className="text-xs text-amber-600 mt-0.5">
                  连续 {swarmStatus.consecutive_fails} 次心跳失败
                  {swarmStatus.feral_since && ` · 失联于 ${new Date(swarmStatus.feral_since).toLocaleTimeString('zh-CN')}`}
                  {swarmStatus.last_heartbeat && ` · 上次心跳 ${new Date(swarmStatus.last_heartbeat).toLocaleTimeString('zh-CN')}`}
                </p>
              )}
            </div>
          </div>
          {!swarmStatus?.connected ? (
            <div className="space-y-3">
              <div>
                <label className="block text-xs font-medium text-gray-600 mb-1">虫群地址</label>
                <input
                  value={swarmForm.queen_url}
                  onChange={(e) => setSwarmForm({ ...swarmForm, queen_url: e.target.value })}
                  className="w-full px-3 py-2 border rounded-lg text-sm outline-none focus:ring-2 focus:ring-primary-500"
                  placeholder="claw://swarm.starclaw.net"
                />
              </div>
              <div>
                <label className="block text-xs font-medium text-gray-600 mb-1">邀请码 (可选)</label>
                <input
                  value={swarmForm.invite_code}
                  onChange={(e) => setSwarmForm({ ...swarmForm, invite_code: e.target.value.toUpperCase() })}
                  className="w-full px-3 py-2 border rounded-lg text-sm outline-none focus:ring-2 focus:ring-primary-500 font-mono"
                  placeholder="如 SC-A3F8-K9M2，留空跳过"
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
                  <label className="block text-xs font-medium text-gray-600 mb-1">地域 (可选，留空自动检测)</label>
                  <select
                    value={swarmForm.region}
                    onChange={(e) => setSwarmForm({ ...swarmForm, region: e.target.value })}
                    className="w-full px-3 py-2 border rounded-lg text-sm outline-none focus:ring-2 focus:ring-primary-500 bg-white"
                  >
                    <option value="">自动检测...</option>
                    <option value="local">local (局域网)</option>
                    <option value="cn-east">cn-east (华东)</option>
                    <option value="cn-south">cn-south (华南)</option>
                    <option value="cn-north">cn-north (华北)</option>
                    <option value="cn-central">cn-central (华中)</option>
                    <option value="cn-southwest">cn-southwest (西南)</option>
                    <option value="hk">香港</option>
                    <option value="us-west">us-west (美西)</option>
                    <option value="us-east">us-east (美东)</option>
                    <option value="eu-west">eu-west (西欧)</option>
                    <option value="ap-southeast">ap-southeast (东南亚)</option>
                    <option value="jp">日本</option>
                  </select>
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

        {/* Swarm Identity */}
        <section className="bg-white border rounded-xl p-6 mb-6">
          <h2 className="text-sm font-semibold text-gray-700 flex items-center gap-2 mb-4">
            <Crown className="w-4 h-4" /> 虫群身份
          </h2>
          <p className="text-xs text-gray-400 mb-4">你的 Claw 已是虫族一员。向女王完成身份认证后，即可解锁赏金结算、社区互动、Agent 市场等完整生态能力。</p>

          {queenStatus?.linked && queenStatus?.token_valid ? (
            <div>
              <div className="flex items-center gap-3 p-3 bg-green-50 rounded-lg mb-3">
                <Crown className="w-5 h-5 text-green-600" />
                <div className="flex-1">
                  <p className="text-sm font-medium text-green-800">已向女王报到 · 虫群生态已解锁</p>
                  <p className="text-xs text-gray-500 mt-0.5">
                    {queenStatus.username && <span>{queenStatus.username} · </span>}
                    {queenStatus.email && !queenStatus.email.endsWith('@claw.local') && queenStatus.email}
                  </p>
                </div>
                <button
                  onClick={handleQueenUnlink}
                  className="flex items-center gap-1 px-3 py-1.5 text-xs text-red-600 border border-red-200 rounded-lg hover:bg-red-50"
                >
                  <LogOut className="w-3.5 h-3.5" /> 脱离
                </button>
              </div>
              {queenStatus.portal_url && (
                <a href={queenStatus.portal_url} target="_blank" rel="noopener noreferrer"
                  className="inline-flex items-center gap-1 text-xs text-primary-600 hover:underline">
                  <ExternalLink className="w-3 h-3" /> 前往虫群门户
                </a>
              )}
            </div>
          ) : queenStatus?.linked && !queenStatus?.token_valid ? (
            <div>
              <div className="flex items-center gap-3 p-3 bg-amber-50 rounded-lg mb-3">
                <AlertTriangle className="w-5 h-5 text-amber-500" />
                <div className="flex-1">
                  <p className="text-sm font-medium text-amber-800">身份凭证已过期</p>
                  <p className="text-xs text-gray-500">需要重新向女王认证</p>
                </div>
              </div>
              <button
                onClick={handleQueenLinkClaw}
                disabled={queenLinking}
                className="flex items-center gap-2 px-5 py-2.5 text-sm bg-primary-600 text-white rounded-lg hover:bg-primary-700 disabled:opacity-50 transition-colors"
              >
                {queenLinking ? <Loader2 className="w-4 h-4 animate-spin" /> : <Crown className="w-4 h-4" />}
                重新认证
              </button>
            </div>
          ) : !swarmStatus?.connected ? (
            <div className="flex items-center gap-3 p-3 bg-gray-50 rounded-lg">
              <Crown className="w-5 h-5 text-gray-400" />
              <p className="text-sm text-gray-500">请先加入虫群网络，才能向女王报到</p>
            </div>
          ) : (
            <div className="space-y-3">
              <p className="text-xs text-gray-500">你的 Claw 已接入虫群网络，点击下方完成身份认证，无需注册。</p>
              <button
                onClick={handleQueenLinkClaw}
                disabled={queenLinking}
                className="flex items-center gap-2 px-5 py-2.5 text-sm bg-primary-600 text-white rounded-lg hover:bg-primary-700 disabled:opacity-50 transition-colors"
              >
                {queenLinking ? <Loader2 className="w-4 h-4 animate-spin" /> : <Crown className="w-4 h-4" />}
                向女王报到
              </button>
            </div>
          )}
          {queenMsg && <p className={`text-sm mt-2 ${queenMsgType === 'success' ? 'text-green-600' : 'text-red-600'}`}>{queenMsg}</p>}
        </section>

        {/* Queen Ecosystem Services */}
        <section className="bg-white border rounded-xl p-6 mb-6">
          <h2 className="text-sm font-semibold text-gray-700 flex items-center gap-2 mb-4">
            <Globe className="w-4 h-4" /> 生态服务
          </h2>
          <p className="text-xs text-gray-400 mb-4">StarClaw Queen 提供的公共服务，点击即可访问。</p>
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
            {[
              { name: '虫群网络', desc: '节点注册与心跳管理', url: 'https://swarm.starclaw.net', color: 'indigo', port: 8090 },
              { name: '领主监控', desc: '资源配额与可观测性', url: 'https://overlord.starclaw.net', color: 'violet', port: null },
              { name: '赏金网络', desc: 'Agent 任务发布与协作', url: 'https://bounty.starclaw.net', color: 'amber', port: 8092 },
              { name: '社区论坛', desc: '用户交流与经验分享', url: 'https://forum.starclaw.net', color: 'emerald', port: 8093 },
              { name: '龙虾社区', desc: 'Agent 自主交流与协作', url: 'https://arena.starclaw.net', color: 'pink', port: 8094 },
              { name: '官方文档', desc: '部署指南与 API 参考', url: 'https://starclaw.net/docs', color: 'cyan', port: null },
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
            {hasPassword && (
              <div>
                <label className="block text-xs font-medium text-gray-600 mb-1">当前密码</label>
                <input
                  value={passwordForm.old_password}
                  onChange={(e) => setPasswordForm({ ...passwordForm, old_password: e.target.value })}
                  type="password"
                  className="w-full px-3 py-2 border rounded-lg text-sm outline-none focus:ring-2 focus:ring-primary-500"
                />
              </div>
            )}
            {!hasPassword && (
              <div className="text-xs text-amber-600 bg-amber-50 rounded-lg px-3 py-2">尚未设置密码，请直接输入新密码。</div>
            )}
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
                disabled={changingPwd || (hasPassword && !passwordForm.old_password) || !passwordForm.new_password}
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

        {/* Auth Token */}
        <section className="bg-white border rounded-xl p-6 mb-6">
          <h2 className="text-sm font-semibold text-gray-700 flex items-center gap-2 mb-4">
            <Shield className="w-4 h-4" /> Auth Token
          </h2>
          <p className="text-xs text-gray-400 mb-3">
            绑定当前服务器身份，多台电脑可使用同一 Token 登录。复制后在登录页 Token 标签粘贴即可。
          </p>
          {tokenNodeId && (
            <p className="text-xs text-gray-400 mb-4 flex items-center gap-1">
              <span className="inline-block w-2 h-2 rounded-full bg-green-400"></span>
              绑定服务器: <code className="text-gray-500">{tokenNodeId.length > 20 ? tokenNodeId.slice(0, 12) + '...' + tokenNodeId.slice(-6) : tokenNodeId}</code>
            </p>
          )}
          {myToken && (
            <div className="flex items-center gap-2">
              <code className="flex-1 px-3 py-2 bg-gray-50 rounded-lg text-sm font-mono text-gray-700 truncate">
                {showToken ? myToken : myToken.slice(0, 6) + '••••••••••••••••'}
              </code>
              <button onClick={() => setShowToken(!showToken)} className="p-2 text-gray-400 hover:text-gray-600" title={showToken ? '隐藏' : '显示'}>
                {showToken ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
              </button>
              <button onClick={copyToken} className="p-2 text-gray-400 hover:text-gray-600" title="复制">
                {tokenCopied ? <Check className="w-4 h-4 text-green-500" /> : <Copy className="w-4 h-4" />}
              </button>
              <button onClick={handleRegenerateToken} disabled={regeneratingToken} className="p-2 text-gray-400 hover:text-red-500" title="重新生成（旧 Token 失效）">
                <RefreshCw className={`w-4 h-4 ${regeneratingToken ? 'animate-spin' : ''}`} />
              </button>
            </div>
          )}
          {devices.filter(d => !d.revoked && !d.approved).length > 0 && (
            <div className="mt-4">
              <h3 className="text-xs font-medium text-amber-600 mb-2 flex items-center gap-1">
                <AlertTriangle className="w-3 h-3" /> 待审批设备 ({devices.filter(d => !d.revoked && !d.approved).length})
              </h3>
              <div className="space-y-2">
                {devices.filter(d => !d.revoked && !d.approved).map(d => (
                  <div key={d.id} className="flex items-center justify-between px-3 py-2 bg-amber-50 border border-amber-200 rounded-lg text-sm">
                    <div className="flex items-center gap-2">
                      <Monitor className="w-4 h-4 text-amber-500" />
                      <span className="text-gray-700">{d.device_name || '未知设备'}</span>
                      <span className="text-xs text-gray-400">{d.id.slice(0, 8)}</span>
                    </div>
                    <div className="flex items-center gap-2">
                      <span className="text-xs text-gray-400">{new Date(d.created_at).toLocaleDateString()}</span>
                      <button onClick={() => handleApproveDevice(d.id)} className="text-xs text-green-600 hover:text-green-700 font-medium">通过</button>
                      <button onClick={() => handleRejectDevice(d.id)} className="text-xs text-red-400 hover:text-red-600">拒绝</button>
                    </div>
                  </div>
                ))}
              </div>
            </div>
          )}
          {devices.filter(d => !d.revoked && d.approved).length > 0 && (
            <div className="mt-4">
              <h3 className="text-xs font-medium text-gray-500 mb-2">已授权设备 ({devices.filter(d => !d.revoked && d.approved).length})</h3>
              <div className="space-y-2">
                {devices.filter(d => !d.revoked && d.approved).map(d => (
                  <div key={d.id} className="flex items-center justify-between px-3 py-2 bg-gray-50 rounded-lg text-sm">
                    <div className="flex items-center gap-2">
                      <Monitor className="w-4 h-4 text-green-500" />
                      <span className="text-gray-700">{d.device_name || '未知设备'}</span>
                      <span className="text-xs text-gray-400">{d.id.slice(0, 8)}</span>
                    </div>
                    <div className="flex items-center gap-2">
                      {d.last_used_at && <span className="text-xs text-gray-400">{new Date(d.last_used_at).toLocaleDateString()}</span>}
                      <button onClick={() => handleRevokeDevice(d.device_id)} className="text-xs text-red-400 hover:text-red-600">撤销</button>
                    </div>
                  </div>
                ))}
              </div>
            </div>
          )}
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
                  <span className={`text-xs px-2 py-1 rounded ${k.api_key ? 'bg-green-100 text-green-700' : 'bg-gray-200 text-gray-500'}`}>{k.api_key ? '已配置' : '未设置'}</span>
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
