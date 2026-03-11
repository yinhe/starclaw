import { useState, useEffect, useCallback } from 'react'
import { Plus, Trash2, TestTube, ToggleLeft, ToggleRight, ChevronDown, ChevronUp, ExternalLink, MessageSquare } from 'lucide-react'
import { integrationAPI } from '../lib/api'

interface Integration {
  id: string
  type: string
  name: string
  config: Record<string, string>
  enabled: boolean
  created_at: string
}

const PLATFORM_META: Record<string, { label: string; icon: string; color: string; fields: { key: string; label: string; placeholder: string; secret?: boolean; required?: boolean }[] }> = {
  feishu: {
    label: '飞书',
    icon: '🐦',
    color: 'bg-blue-500',
    fields: [
      { key: 'app_id', label: 'App ID', placeholder: 'cli_xxxxxxxxxx', required: true },
      { key: 'app_secret', label: 'App Secret', placeholder: '飞书开放平台 App Secret', secret: true, required: true },
      { key: 'webhook_url', label: 'Webhook URL（可选）', placeholder: 'https://open.feishu.cn/open-apis/bot/v2/hook/...' },
    ],
  },
  dingtalk: {
    label: '钉钉',
    icon: '💬',
    color: 'bg-sky-500',
    fields: [
      { key: 'app_key', label: 'App Key', placeholder: 'dingxxxxxxxxxx', required: true },
      { key: 'app_secret', label: 'App Secret', placeholder: '钉钉开放平台 App Secret', secret: true, required: true },
      { key: 'webhook_url', label: 'Webhook URL（可选）', placeholder: 'https://oapi.dingtalk.com/robot/send?access_token=...' },
      { key: 'sign_secret', label: '签名密钥（可选）', placeholder: 'SECxxxxxxxx', secret: true },
    ],
  },
  wecom: {
    label: '企业微信',
    icon: '💼',
    color: 'bg-green-500',
    fields: [
      { key: 'corp_id', label: 'Corp ID', placeholder: 'ww...' },
      { key: 'agent_id', label: 'Agent ID', placeholder: '1000002' },
      { key: 'secret', label: 'Secret', placeholder: '应用 Secret', secret: true },
      { key: 'webhook_url', label: 'Webhook URL（可选）', placeholder: 'https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=...' },
    ],
  },
  slack: {
    label: 'Slack',
    icon: '💜',
    color: 'bg-purple-600',
    fields: [
      { key: 'bot_token', label: 'Bot Token', placeholder: 'xoxb-...', secret: true },
      { key: 'webhook_url', label: 'Webhook URL（可选）', placeholder: 'https://hooks.slack.com/services/...' },
    ],
  },
  discord: {
    label: 'Discord',
    icon: '🎮',
    color: 'bg-indigo-500',
    fields: [
      { key: 'bot_token', label: 'Bot Token', placeholder: 'Bot Token from Developer Portal', secret: true },
      { key: 'webhook_url', label: 'Webhook URL（可选）', placeholder: 'https://discord.com/api/webhooks/...' },
    ],
  },
  telegram: {
    label: 'Telegram',
    icon: '✈️',
    color: 'bg-cyan-500',
    fields: [
      { key: 'bot_token', label: 'Bot Token', placeholder: '123456:ABC-DEF...（从 @BotFather 获取）', secret: true, required: true },
      { key: 'chat_id', label: 'Chat ID（可选）', placeholder: '-1001234567890' },
    ],
  },
}

export default function IntegrationsPage() {
  const [integrations, setIntegrations] = useState<Integration[]>([])
  const [loading, setLoading] = useState(true)
  const [showCreate, setShowCreate] = useState(false)
  const [createType, setCreateType] = useState('feishu')
  const [createName, setCreateName] = useState('')
  const [createConfig, setCreateConfig] = useState<Record<string, string>>({})
  const [creating, setCreating] = useState(false)
  const [expandedId, setExpandedId] = useState<string | null>(null)
  const [testResults, setTestResults] = useState<Record<string, { success: boolean; message: string }>>({})
  const [testing, setTesting] = useState<string | null>(null)

  const fetchIntegrations = useCallback(async () => {
    try {
      const res = await integrationAPI.list()
      setIntegrations(res.data.integrations || [])
    } catch {
      // ignore
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => { fetchIntegrations() }, [fetchIntegrations])

  const handleCreate = async () => {
    if (!createName.trim()) return
    setCreating(true)
    try {
      await integrationAPI.create({ type: createType, name: createName, config: createConfig })
      setShowCreate(false)
      setCreateName('')
      setCreateConfig({})
      fetchIntegrations()
    } catch (err: unknown) {
      const axiosErr = err as { response?: { data?: { error?: string } } }
      alert(axiosErr.response?.data?.error || '创建失败')
    } finally {
      setCreating(false)
    }
  }

  const handleDelete = async (id: string) => {
    if (!confirm('确认删除此集成？')) return
    try {
      await integrationAPI.delete(id)
      fetchIntegrations()
    } catch {
      alert('删除失败')
    }
  }

  const handleToggle = async (integration: Integration) => {
    try {
      await integrationAPI.update(integration.id, { enabled: !integration.enabled })
      fetchIntegrations()
    } catch {
      alert('更新失败')
    }
  }

  const handleTest = async (id: string) => {
    setTesting(id)
    try {
      const res = await integrationAPI.test(id)
      setTestResults(prev => ({ ...prev, [id]: { success: res.data.success, message: res.data.message || res.data.error || '' } }))
    } catch (err: unknown) {
      const axiosErr = err as { response?: { data?: { error?: string } } }
      setTestResults(prev => ({ ...prev, [id]: { success: false, message: axiosErr.response?.data?.error || '测试失败' } }))
    } finally {
      setTesting(null)
    }
  }

  const meta = PLATFORM_META[createType] || PLATFORM_META.feishu

  return (
    <div className="max-w-4xl mx-auto p-6">
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-2xl font-bold flex items-center gap-2">
            <MessageSquare className="w-6 h-6 text-primary-600" />
            通讯集成
          </h1>
          <p className="text-gray-500 text-sm mt-1">连接飞书、钉钉、企业微信、Slack 等通讯平台，让 AI Agent 可以发送消息和通知。</p>
        </div>
        <button
          onClick={() => setShowCreate(true)}
          className="flex items-center gap-2 px-4 py-2 bg-primary-600 text-white rounded-lg hover:bg-primary-700 transition-colors"
        >
          <Plus className="w-4 h-4" />
          添加集成
        </button>
      </div>

      {/* Create Dialog */}
      {showCreate && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
          <div className="bg-white rounded-2xl shadow-xl w-full max-w-lg mx-4 max-h-[90vh] overflow-y-auto">
            <div className="p-6">
              <h2 className="text-lg font-semibold mb-4">添加通讯集成</h2>

              {/* Platform selector */}
              <div className="mb-4">
                <label className="block text-sm font-medium text-gray-700 mb-2">平台</label>
                <div className="grid grid-cols-3 gap-2">
                  {Object.entries(PLATFORM_META).map(([key, pm]) => (
                    <button
                      key={key}
                      onClick={() => { setCreateType(key); setCreateConfig({}) }}
                      className={`flex items-center gap-2 p-3 rounded-lg border-2 transition-all text-sm ${
                        createType === key ? 'border-primary-500 bg-primary-50' : 'border-gray-200 hover:border-gray-300'
                      }`}
                    >
                      <span className="text-lg">{pm.icon}</span>
                      <span className="font-medium">{pm.label}</span>
                    </button>
                  ))}
                </div>
              </div>

              {/* Name */}
              <div className="mb-4">
                <label className="block text-sm font-medium text-gray-700 mb-1">名称</label>
                <input
                  type="text"
                  value={createName}
                  onChange={e => setCreateName(e.target.value)}
                  placeholder={`我的${meta.label}机器人`}
                  className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-primary-500 focus:border-transparent outline-none"
                />
              </div>

              {/* Platform-specific fields */}
              {meta.fields.map(field => (
                <div key={field.key} className="mb-3">
                  <label className="block text-sm font-medium text-gray-700 mb-1">
                    {field.label}
                    {field.required && <span className="text-red-500 ml-1">*</span>}
                  </label>
                  <input
                    type={field.secret ? 'password' : 'text'}
                    value={createConfig[field.key] || ''}
                    onChange={e => setCreateConfig(prev => ({ ...prev, [field.key]: e.target.value }))}
                    placeholder={field.placeholder}
                    className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-primary-500 focus:border-transparent outline-none text-sm font-mono"
                  />
                </div>
              ))}

              {/* Help link */}
              <div className="mt-4 p-3 bg-blue-50 rounded-lg">
                <p className="text-xs text-blue-700 flex items-center gap-1">
                  <ExternalLink className="w-3 h-3" />
                  {createType === 'feishu' && <a href="https://open.feishu.cn/app" target="_blank" rel="noopener noreferrer" className="underline">前往飞书开放平台创建应用 →</a>}
                  {createType === 'dingtalk' && <a href="https://open-dev.dingtalk.com" target="_blank" rel="noopener noreferrer" className="underline">前往钉钉开放平台创建应用 →</a>}
                  {createType === 'wecom' && <a href="https://work.weixin.qq.com/wework_admin/frame#apps" target="_blank" rel="noopener noreferrer" className="underline">前往企业微信管理后台 →</a>}
                  {createType === 'slack' && <a href="https://api.slack.com/apps" target="_blank" rel="noopener noreferrer" className="underline">前往 Slack App Dashboard →</a>}
                  {createType === 'discord' && <a href="https://discord.com/developers/applications" target="_blank" rel="noopener noreferrer" className="underline">前往 Discord Developer Portal →</a>}
                  {createType === 'telegram' && <span>在 Telegram 搜索 @BotFather 创建 Bot</span>}
                </p>
              </div>

              {/* Actions */}
              <div className="flex justify-end gap-3 mt-6">
                <button
                  onClick={() => { setShowCreate(false); setCreateConfig({}) }}
                  className="px-4 py-2 text-gray-600 hover:bg-gray-100 rounded-lg transition-colors"
                >
                  取消
                </button>
                <button
                  onClick={handleCreate}
                  disabled={creating || !createName.trim()}
                  className="px-4 py-2 bg-primary-600 text-white rounded-lg hover:bg-primary-700 disabled:opacity-50 transition-colors"
                >
                  {creating ? '创建中...' : '创建'}
                </button>
              </div>
            </div>
          </div>
        </div>
      )}

      {/* Integration List */}
      {loading ? (
        <div className="flex justify-center py-12">
          <div className="w-6 h-6 border-2 border-primary-600 border-t-transparent rounded-full animate-spin" />
        </div>
      ) : integrations.length === 0 ? (
        <div className="text-center py-16 bg-gray-50 rounded-2xl">
          <MessageSquare className="w-12 h-12 text-gray-300 mx-auto mb-3" />
          <p className="text-gray-500 mb-2">还没有配置通讯集成</p>
          <p className="text-gray-400 text-sm">添加飞书、钉钉等平台，让 Agent 能够发送消息通知</p>
        </div>
      ) : (
        <div className="space-y-3">
          {integrations.map(integration => {
            const pm = PLATFORM_META[integration.type] || { label: integration.type, icon: '🔗', color: 'bg-gray-500', fields: [] }
            const expanded = expandedId === integration.id
            const testResult = testResults[integration.id]

            return (
              <div key={integration.id} className="bg-white border border-gray-200 rounded-xl overflow-hidden">
                {/* Header */}
                <div className="flex items-center justify-between p-4">
                  <div className="flex items-center gap-3">
                    <div className={`w-10 h-10 ${pm.color} rounded-lg flex items-center justify-center text-xl text-white`}>
                      {pm.icon}
                    </div>
                    <div>
                      <div className="font-medium">{integration.name}</div>
                      <div className="text-xs text-gray-400">{pm.label} · {new Date(integration.created_at).toLocaleDateString()}</div>
                    </div>
                  </div>

                  <div className="flex items-center gap-2">
                    {/* Test */}
                    <button
                      onClick={() => handleTest(integration.id)}
                      disabled={testing === integration.id}
                      className="p-2 text-gray-400 hover:text-blue-600 hover:bg-blue-50 rounded-lg transition-colors"
                      title="测试连接"
                    >
                      <TestTube className={`w-4 h-4 ${testing === integration.id ? 'animate-pulse' : ''}`} />
                    </button>

                    {/* Toggle */}
                    <button
                      onClick={() => handleToggle(integration)}
                      className="p-2 hover:bg-gray-50 rounded-lg transition-colors"
                      title={integration.enabled ? '禁用' : '启用'}
                    >
                      {integration.enabled
                        ? <ToggleRight className="w-5 h-5 text-green-500" />
                        : <ToggleLeft className="w-5 h-5 text-gray-400" />
                      }
                    </button>

                    {/* Delete */}
                    <button
                      onClick={() => handleDelete(integration.id)}
                      className="p-2 text-gray-400 hover:text-red-600 hover:bg-red-50 rounded-lg transition-colors"
                      title="删除"
                    >
                      <Trash2 className="w-4 h-4" />
                    </button>

                    {/* Expand */}
                    <button
                      onClick={() => setExpandedId(expanded ? null : integration.id)}
                      className="p-2 text-gray-400 hover:text-gray-600 hover:bg-gray-50 rounded-lg transition-colors"
                    >
                      {expanded ? <ChevronUp className="w-4 h-4" /> : <ChevronDown className="w-4 h-4" />}
                    </button>
                  </div>
                </div>

                {/* Test result */}
                {testResult && (
                  <div className={`mx-4 mb-3 p-2 rounded-lg text-sm ${testResult.success ? 'bg-green-50 text-green-700' : 'bg-red-50 text-red-700'}`}>
                    {testResult.success ? '✓ ' : '✗ '}{testResult.message}
                  </div>
                )}

                {/* Expanded config details */}
                {expanded && (
                  <div className="px-4 pb-4 border-t border-gray-100 pt-3">
                    <div className="text-xs text-gray-500 mb-2">配置信息</div>
                    <div className="space-y-1.5">
                      {Object.entries(integration.config).map(([k, v]) => (
                        v ? (
                          <div key={k} className="flex items-center gap-2 text-sm">
                            <span className="text-gray-500 font-mono min-w-[120px]">{k}:</span>
                            <span className="font-mono text-gray-700 truncate">{v}</span>
                          </div>
                        ) : null
                      ))}
                    </div>
                    <div className="mt-3 p-2 bg-gray-50 rounded-lg">
                      <p className="text-xs text-gray-500">
                        Integration ID: <code className="bg-gray-200 px-1 rounded">{integration.id}</code>
                        <br />
                        在 Agent 对话中使用 <code className="bg-gray-200 px-1 rounded">feishu</code> 工具即可发送消息
                      </p>
                    </div>
                  </div>
                )}
              </div>
            )
          })}
        </div>
      )}

      {/* Usage Guide */}
      <div className="mt-8 p-5 bg-gradient-to-r from-blue-50 to-purple-50 rounded-xl">
        <h3 className="font-semibold text-gray-800 mb-2">💡 使用说明</h3>
        <ul className="text-sm text-gray-600 space-y-1.5">
          <li>1. 在上方添加通讯平台集成，填入应用凭证</li>
          <li>2. 点击「测试连接」验证凭证是否有效</li>
          <li>3. 创建或编辑 Agent 时，在工具列表中启用 <code className="bg-white px-1.5 py-0.5 rounded text-primary-600 font-mono">feishu</code> 工具</li>
          <li>4. 在对话中让 Agent 发送消息，例如：「把这个总结发到飞书群里」</li>
        </ul>
      </div>
    </div>
  )
}
