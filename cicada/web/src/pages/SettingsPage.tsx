import { useEffect, useState } from 'react'
import { Shield, HardDrive, Activity } from 'lucide-react'
import { getHealth, getComplianceStats, getRecordingsUsage } from '../api'

export default function SettingsPage() {
  const [health, setHealth] = useState<any>(null)
  const [compliance, setCompliance] = useState<any>(null)
  const [disk, setDisk] = useState<any>(null)

  useEffect(() => {
    getHealth().then(setHealth).catch(() => {})
    getComplianceStats().then(setCompliance).catch(() => {})
    getRecordingsUsage().then(setDisk).catch(() => {})
  }, [])

  return (
    <div className="p-6 space-y-6">
      <h1 className="text-2xl font-bold">系统设置</h1>

      {/* Service Health */}
      <div className="bg-stone-900 border border-stone-800 rounded-xl p-5">
        <div className="flex items-center gap-2 mb-4">
          <Activity className="w-4 h-4 text-cicada-400" />
          <h2 className="text-sm font-medium">服务状态</h2>
        </div>
        {health ? (
          <div className="grid grid-cols-2 md:grid-cols-4 gap-4 text-sm">
            <div>
              <div className="text-xs text-stone-500">状态</div>
              <div className={health.status === 'ok' ? 'text-cicada-400 font-medium' : 'text-red-400'}>{health.status === 'ok' ? '正常运行' : '异常'}</div>
            </div>
            <div>
              <div className="text-xs text-stone-500">服务</div>
              <div className="text-stone-300">{health.service}</div>
            </div>
            <div>
              <div className="text-xs text-stone-500">版本</div>
              <div className="text-stone-300">{health.version}</div>
            </div>
            <div>
              <div className="text-xs text-stone-500">时间</div>
              <div className="text-stone-300">{health.timestamp ? new Date(health.timestamp).toLocaleString('zh-CN') : '-'}</div>
            </div>
          </div>
        ) : (
          <div className="text-stone-600 text-sm">加载中...</div>
        )}
      </div>

      {/* Compliance */}
      <div className="bg-stone-900 border border-stone-800 rounded-xl p-5">
        <div className="flex items-center gap-2 mb-4">
          <Shield className="w-4 h-4 text-yellow-400" />
          <h2 className="text-sm font-medium">合规统计</h2>
        </div>
        {compliance ? (
          <div className="grid grid-cols-3 gap-4 text-sm">
            <div>
              <div className="text-xs text-stone-500">黑名单数量</div>
              <div className="text-xl font-bold">{compliance.blacklist_count ?? 0}</div>
            </div>
            <div>
              <div className="text-xs text-stone-500">今日拦截</div>
              <div className="text-xl font-bold text-yellow-400">{compliance.today_blocks ?? 0}</div>
            </div>
            <div>
              <div className="text-xs text-stone-500">外显号码使用</div>
              <div className="text-sm text-stone-400 mt-1">
                {compliance.caller_usage && Object.keys(compliance.caller_usage).length > 0
                  ? Object.entries(compliance.caller_usage).map(([num, cnt]) => (
                      <div key={num}>{num}: {String(cnt)}次</div>
                    ))
                  : <span className="text-stone-600">暂无数据</span>
                }
              </div>
            </div>
          </div>
        ) : (
          <div className="text-stone-600 text-sm">加载中...</div>
        )}
      </div>

      {/* Storage */}
      <div className="bg-stone-900 border border-stone-800 rounded-xl p-5">
        <div className="flex items-center gap-2 mb-4">
          <HardDrive className="w-4 h-4 text-blue-400" />
          <h2 className="text-sm font-medium">存储用量</h2>
        </div>
        {disk ? (
          <div className="grid grid-cols-3 gap-4 text-sm">
            <div>
              <div className="text-xs text-stone-500">录音文件数</div>
              <div className="text-xl font-bold">{disk.total_files ?? 0}</div>
            </div>
            <div>
              <div className="text-xs text-stone-500">总大小</div>
              <div className="text-xl font-bold">{disk.total_size_mb ?? 0} MB</div>
            </div>
            <div>
              <div className="text-xs text-stone-500">原始字节</div>
              <div className="text-stone-400">{(disk.total_size_bytes ?? 0).toLocaleString()} bytes</div>
            </div>
          </div>
        ) : (
          <div className="text-stone-600 text-sm">加载中...</div>
        )}
      </div>

      {/* Config Info */}
      <div className="bg-stone-900 border border-stone-800 rounded-xl p-5">
        <h2 className="text-sm font-medium mb-3">配置说明</h2>
        <div className="text-sm text-stone-400 space-y-2">
          <p>Bridge 服务配置文件: <code className="bg-stone-800 px-1.5 py-0.5 rounded text-xs">cicada/bridge/config.yaml</code></p>
          <p>话术模板目录: <code className="bg-stone-800 px-1.5 py-0.5 rounded text-xs">cicada/scripts/</code></p>
          <p>环境变量:</p>
          <ul className="text-xs text-stone-500 space-y-1 ml-4">
            <li><code>CLOOPEN_ACCOUNT_SID</code> — 容联云账号 SID</li>
            <li><code>CLOOPEN_AUTH_TOKEN</code> — 容联云认证 Token</li>
            <li><code>DASHSCOPE_API_KEY</code> — DashScope API Key (ASR/TTS)</li>
            <li><code>LLM_API_KEY</code> — LLM API Key (qwen-turbo)</li>
            <li><code>LLM_BASE_URL</code> — LLM 接口地址 (默认 star-ai.net)</li>
            <li><code>CICADA_PORT</code> — Bridge 端口 (默认 8099)</li>
          </ul>
        </div>
      </div>
    </div>
  )
}
