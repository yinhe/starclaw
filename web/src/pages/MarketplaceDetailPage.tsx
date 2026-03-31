import { useState, useEffect } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import {
  ArrowLeft, Download, Check, Trash2, Loader2, Bot,
  Wrench, Zap, GitBranch, Crown, Dna, Plug, Brain,
  ChevronDown, ChevronUp, Clock, Sparkles, Search,
  Film, Code, FileText, Settings,
} from 'lucide-react'
import { agentAPI, queenMarketplaceAPI } from '../lib/api'

// ── Types ──

interface PricingInfo {
  type: string; price: number; period?: string; currency?: string; display?: string
}
interface SkillSpec {
  trigger: string; description?: string; schedule?: string
  tools?: string[]; example_triggers?: string[]; auto_execute?: boolean; notify?: boolean
}
interface BundleSkill { name: string; spec: string }
interface BundleMCP { name: string; base_url: string; description: string }
interface BundleWorkflow { name: string; description: string }
interface MarketplaceItem {
  id: string; name: string; description: string; icon: string; tags: string
  config: string; downloads: number; rating: number; version?: string
  author?: { nickname?: string }
}

// ── Constants ──

type HexadTab = 'gene' | 'skill' | 'instinct' | 'mcp' | 'workflow' | 'memory'
const hexadTabs: { key: HexadTab; label: string; icon: typeof Dna; color: string }[] = [
  { key: 'gene', label: '基因', icon: Dna, color: 'text-rose-500' },
  { key: 'skill', label: '技能', icon: Wrench, color: 'text-blue-500' },
  { key: 'instinct', label: '本能', icon: Zap, color: 'text-amber-500' },
  { key: 'mcp', label: '外接服务', icon: Plug, color: 'text-purple-500' },
  { key: 'workflow', label: '工作流', icon: GitBranch, color: 'text-emerald-500' },
  { key: 'memory', label: '记忆', icon: Brain, color: 'text-pink-500' },
]

const toolMeta: Record<string, { label: string; icon: string; desc: string }> = {
  web_search: { label: '网页搜索', icon: '🔍', desc: '搜索互联网信息' },
  browser: { label: '浏览器操控', icon: '🌐', desc: '打开网页、点击、截图、提取内容' },
  http_request: { label: 'HTTP 请求', icon: '📡', desc: '发送请求，调用第三方 API' },
  video_generation: { label: '视频生成', icon: '🎬', desc: 'wan/veo3/sora2/kling/luma 多模型' },
  dubbing: { label: '配音字幕', icon: '🎙️', desc: 'TTS配音 + 字幕烧录' },
  mv_production: { label: 'MV 合成', icon: '🎵', desc: '节拍同步剪辑 + 专业转场' },
  comic_production: { label: '漫剧制作', icon: '📖', desc: '漫画图片 + 多角色配音组装' },
  music_generation: { label: '音乐创作', icon: '🎶', desc: 'ACE-Step/MiniMax/DiffRhythm' },
  image_generation: { label: '图片生成', icon: '🎨', desc: 'Flux/DALL-E 等 AI 绘画' },
  audio_analysis: { label: '音频分析', icon: '🔊', desc: 'BPM/能量分析、节拍检测' },
  code: { label: '代码执行', icon: '💻', desc: '14种语言、Web应用部署' },
  document: { label: '文档总结', icon: '📄', desc: '对话摘要 + Word 导出' },
  desktop: { label: '桌面操控', icon: '🖥️', desc: '截图/点击/输入桌面应用' },
  system: { label: '系统管理', icon: '⚙️', desc: 'Agent编排、任务调度、委派' },
  trading_scan: { label: '全市场扫描', icon: '🔍', desc: '5000+只A股四维打分筛选' },
  trading_kline: { label: 'K线数据', icon: '📊', desc: '获取OHLCV蜡烛图数据' },
  trading_quote: { label: '实时行情', icon: '📈', desc: '获取股票实时价格' },
  trading_positions_list: { label: '持仓查询', icon: '📋', desc: '查看所有持仓盈亏' },
  trading_check_exits: { label: '风险检测', icon: '🛡️', desc: '检查止损/止盈/时间止损' },
  trading_buy: { label: '买入下单', icon: '🟢', desc: 'QMT限价买入' },
  trading_sell: { label: '卖出下单', icon: '🔴', desc: 'QMT限价卖出' },
  trading_health: { label: '系统健康', icon: '💚', desc: 'QMT连接状态检查' },
  trading_premarket: { label: '盘前分析', icon: '🌅', desc: 'AI市场方向+仓位建议' },
  trading_daily_report: { label: '日终报告', icon: '📝', desc: '每日交易总结' },
  mcp_trading_bridge: { label: 'Trading Bridge', icon: '🔗', desc: 'miniQMT行情+交易接口' },
}
const skillIcons: Record<string, string> = {
  '个股诊断': '📊', '持仓复盘': '📋', '风险排查': '🛡️', '全市场扫描': '🔍', '市场解读': '📈',
  '盘前分析': '🌅', '自动扫描选股': '🔄', '持仓实时监控': '👁️', '日终复盘': '📝',
}
interface SkillGroup { key: string; label: string; icon: typeof Search; color: string; tools: string[]; desc: string }
const skillGroups: SkillGroup[] = [
  { key: 'info', label: '信息获取', icon: Search, color: 'text-amber-500', tools: ['web_search', 'browser', 'http_request'], desc: '搜索、浏览、抓取互联网信息' },
  { key: 'create', label: '内容创作', icon: Film, color: 'text-blue-500', tools: ['video_generation', 'dubbing', 'mv_production', 'comic_production', 'music_generation', 'image_generation', 'audio_analysis'], desc: '视频/音乐/图片/漫剧全链路创作' },
  { key: 'dev', label: '编程开发', icon: Code, color: 'text-emerald-500', tools: ['code'], desc: '编写代码、运行调试、部署应用' },
  { key: 'doc', label: '文档处理', icon: FileText, color: 'text-cyan-500', tools: ['document'], desc: '对话总结、Word文档导出' },
  { key: 'sys', label: '系统管理', icon: Settings, color: 'text-rose-500', tools: ['system'], desc: 'Agent编排、任务调度、委派' },
  { key: 'trading', label: '量化交易', icon: Search, color: 'text-red-500', tools: ['trading_scan', 'trading_kline', 'trading_quote', 'trading_positions_list', 'trading_check_exits', 'trading_buy', 'trading_sell', 'trading_health', 'trading_premarket', 'trading_daily_report', 'mcp_trading_bridge'], desc: 'A股扫描/行情/下单/风控' },
]

// ── Helpers ──

function getPricing(cfg: any): PricingInfo | null {
  if (cfg?.pricing?.type && cfg.pricing.type !== 'free' && cfg.pricing.price > 0) return cfg.pricing
  return null
}
function parseJSON<T>(s: string, fallback: T): T { try { return JSON.parse(s) } catch { return fallback } }

// ── Main Component ──

export default function MarketplaceDetailPage() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const [item, setItem] = useState<MarketplaceItem | null>(null)
  const [loading, setLoading] = useState(true)
  const [isInstalled, setIsInstalled] = useState(false)
  const [installing, setInstalling] = useState(false)
  const [toast, setToast] = useState('')
  const [toastType, setToastType] = useState<'success' | 'error' | 'warning'>('success')
  const [activeTab, setActiveTab] = useState<HexadTab>('gene')
  const [payModal, setPayModal] = useState<{ open: boolean; pricing: PricingInfo | null; polling: boolean; orderNo: string }>({ open: false, pricing: null, polling: false, orderNo: '' })

  useEffect(() => {
    if (!id) return
    setLoading(true)
    Promise.all([
      queenMarketplaceAPI.get(id),
      agentAPI.installedSourceIDs(),
    ]).then(([itemRes, idsRes]) => {
      setItem(itemRes.data.item)
      setIsInstalled(new Set(idsRes.data.source_ids || []).has(id))
    }).catch(() => setItem(null))
      .finally(() => setLoading(false))
  }, [id])

  const showToast = (msg: string, type: 'success' | 'error' | 'warning' = 'success') => {
    setToast(msg); setToastType(type); setTimeout(() => setToast(''), 4000)
  }

  const handleInstall = async () => {
    if (!item) return
    const cfg = parseJSON(item.config, {} as any)
    const pricing = getPricing(cfg)
    if (pricing) { handlePurchase(pricing); return }
    setInstalling(true)
    try {
      await agentAPI.installFromMarketplace({
        source_id: item.id, name: item.name, description: item.description,
        system_prompt: cfg.system_prompt || '', tools: cfg.tools || '[]',
        config: cfg.config || '{}', icon: item.icon,
      })
      setIsInstalled(true); showToast(`已安装「${item.name}」`)
    } catch { showToast('安装失败', 'error') }
    setInstalling(false)
  }

  const handlePurchase = async (pricing: PricingInfo) => {
    setPayModal({ open: true, pricing, polling: false, orderNo: '' })
  }

  const startPayment = async (method: 'alipay' | 'wechatpay') => {
    if (!item || !payModal.pricing) return
    setInstalling(true)
    try {
      const res = await queenMarketplaceAPI.purchase(item.id, method)
      const { pay_url, code_url, order_no } = res.data
      const payUrl = pay_url || code_url
      if (!payUrl) { showToast('未获取到支付链接', 'error'); setInstalling(false); setPayModal(p => ({ ...p, open: false })); return }

      window.open(payUrl, '_blank')
      setPayModal(p => ({ ...p, polling: true, orderNo: order_no }))

      const pollInterval = setInterval(async () => {
        try {
          const statusRes = await queenMarketplaceAPI.pollPurchaseStatus(order_no)
          if (statusRes.data.status === 'paid') {
            clearInterval(pollInterval)
            setIsInstalled(true)
            setInstalling(false)
            setPayModal({ open: false, pricing: null, polling: false, orderNo: '' })
            showToast(statusRes.data.message || `支付成功，已安装「${item.name}」`)
          }
        } catch { /* continue polling */ }
      }, 3000)
      setTimeout(() => { clearInterval(pollInterval); setInstalling(false); setPayModal(p => ({ ...p, polling: false })) }, 600000)
    } catch (err: any) {
      const data = err?.response?.data
      if (err?.response?.status === 409) {
        showToast('已购买过该智能体', 'warning'); setIsInstalled(true)
      } else { showToast(data?.message || '创建支付订单失败', 'error') }
      setInstalling(false)
      setPayModal(p => ({ ...p, open: false }))
    }
  }

  const handleUninstall = async () => {
    if (!item || !confirm('确定要卸载这个智能体吗？')) return
    try { await agentAPI.uninstallBySourceID(item.id); setIsInstalled(false); showToast('已卸载') } catch {}
  }

  if (loading) return <div className="h-full flex items-center justify-center"><Loader2 className="w-6 h-6 animate-spin text-gray-400" /></div>
  if (!item) return <div className="h-full flex flex-col items-center justify-center text-gray-400"><p className="mb-4">未找到该智能体</p><button onClick={() => navigate('/marketplace')} className="text-primary-600 hover:underline text-sm">← 返回市场</button></div>

  const cfg = parseJSON(item.config, {} as any)
  const pricing = getPricing(cfg)
  const tools: string[] = parseJSON(cfg.tools || '[]', [])
  const tags: string[] = parseJSON(item.tags || '[]', [])
  const bundle = cfg.bundle || {}
  const bundleSkills: BundleSkill[] = bundle.skills || []
  const passiveSkills = bundleSkills.filter(s => { try { return parseJSON(s.spec, {} as any).trigger === 'passive' } catch { return true } })
  const proactiveSkills = bundleSkills.filter(s => { try { return parseJSON(s.spec, {} as any).trigger === 'proactive' } catch { return false } })
  const mcpServers: BundleMCP[] = bundle.mcp_servers || []
  const workflows: BundleWorkflow[] = bundle.workflows || []
  const builtinTools = tools.filter(t => !t.startsWith('mcp_'))
  const mcpTools = tools.filter(t => t.startsWith('mcp_'))

  const tabCounts: Partial<Record<HexadTab, number>> = {
    skill: builtinTools.length + passiveSkills.length,
    instinct: proactiveSkills.length,
    mcp: mcpTools.length + mcpServers.length,
  }

  return (
    <div className="h-full overflow-y-auto">
      <div className="max-w-5xl mx-auto p-8">
        {toast && <div className={`mb-4 px-4 py-2 text-sm rounded-lg ${toastType === 'error' ? 'bg-red-50 dark:bg-red-900/20 text-red-700 dark:text-red-400' : toastType === 'warning' ? 'bg-amber-50 dark:bg-amber-900/20 text-amber-700 dark:text-amber-400' : 'bg-green-50 dark:bg-green-900/20 text-green-700 dark:text-green-400'}`}>{toast}</div>}

        <button onClick={() => navigate('/marketplace')} className="flex items-center gap-1.5 text-sm text-gray-500 hover:text-gray-700 dark:hover:text-gray-300 mb-6 transition-colors">
          <ArrowLeft className="w-4 h-4" />返回市场
        </button>

        {/* Header Card */}
        <div className="bg-white dark:bg-gray-800 border dark:border-gray-700 rounded-2xl p-6 mb-6">
          <div className="flex items-start gap-4">
            <div className="w-14 h-14 rounded-xl flex items-center justify-center flex-shrink-0 bg-gradient-to-br from-primary-500 to-orange-500">
              <Bot className="w-7 h-7 text-white" />
            </div>
            <div className="flex-1 min-w-0">
              <div className="flex items-center gap-3">
                <h1 className="text-2xl font-bold text-gray-900 dark:text-white">{item.name}</h1>
                {pricing && <span className="px-2 py-0.5 bg-amber-50 dark:bg-amber-900/30 text-amber-600 dark:text-amber-400 text-xs rounded-full border border-amber-200 dark:border-amber-700 font-medium">{pricing.type === 'subscription' ? '订阅' : '付费'}</span>}
              </div>
              <p className="text-gray-500 dark:text-gray-400 mt-1 text-sm">by {item.author?.nickname || 'StarClaw 官方'} · v{item.version || '1.0.0'}</p>
              <p className="text-gray-600 dark:text-gray-300 mt-2 text-sm leading-relaxed whitespace-pre-line">{item.description}</p>
              {tags.length > 0 && <div className="flex gap-1.5 mt-3 flex-wrap">{tags.map(t => <span key={t} className="px-2 py-0.5 bg-gray-100 dark:bg-gray-700 text-gray-500 dark:text-gray-400 text-[11px] rounded-full">{t}</span>)}</div>}
              <div className="flex gap-3 mt-4">
                {isInstalled ? (
                  <button onClick={handleUninstall} className="flex items-center gap-2 px-4 py-2 bg-green-50 dark:bg-green-900/20 text-green-700 dark:text-green-400 rounded-lg text-sm font-medium hover:bg-red-50 hover:text-red-600 transition-colors group">
                    <Check className="w-4 h-4 group-hover:hidden" /><Trash2 className="w-4 h-4 hidden group-hover:block" />
                    <span className="group-hover:hidden">已安装</span><span className="hidden group-hover:inline">卸载</span>
                  </button>
                ) : pricing ? (
                  <button onClick={handleInstall} disabled={installing} className="flex items-center gap-2 px-5 py-2.5 bg-gradient-to-r from-amber-500 to-orange-500 text-white rounded-lg text-sm font-medium hover:from-amber-600 hover:to-orange-600 disabled:opacity-50 transition shadow-sm">
                    {installing ? <Loader2 className="w-4 h-4 animate-spin" /> : <Crown className="w-4 h-4" />}
                    购买 {pricing.display || `¥${(pricing.price / 100).toFixed(0)}`}
                  </button>
                ) : (
                  <button onClick={handleInstall} disabled={installing} className="flex items-center gap-2 px-4 py-2 bg-primary-600 text-white rounded-lg text-sm hover:bg-primary-700 disabled:opacity-50 transition-colors">
                    {installing ? <Loader2 className="w-4 h-4 animate-spin" /> : <Download className="w-4 h-4" />}
                    免费安装
                  </button>
                )}
              </div>
            </div>
          </div>
        </div>

        {/* Hexad Tabs */}
        <div className="flex gap-1 mb-6 bg-gray-100 dark:bg-gray-800 rounded-xl p-1 overflow-x-auto">
          {hexadTabs.map(tab => {
            const Icon = tab.icon; const isActive = activeTab === tab.key; const count = tabCounts[tab.key]
            return (
              <button key={tab.key} onClick={() => setActiveTab(tab.key)}
                className={`flex items-center gap-1.5 px-4 py-2.5 rounded-lg text-sm font-medium transition-all whitespace-nowrap ${isActive ? 'bg-white dark:bg-gray-700 shadow-sm text-gray-900 dark:text-white' : 'text-gray-500 hover:text-gray-700 hover:bg-gray-50 dark:hover:bg-gray-700/50'}`}>
                <Icon className={`w-4 h-4 ${isActive ? tab.color : ''}`} />
                {tab.label}
                {count !== undefined && count > 0 && <span className={`text-[10px] px-1.5 py-0.5 rounded-full font-medium ${isActive ? 'bg-gray-100 dark:bg-gray-600 text-gray-600 dark:text-gray-300' : 'bg-gray-200 dark:bg-gray-700 text-gray-500'}`}>{count}</span>}
              </button>
            )
          })}
        </div>

        {/* Payment Modal */}
        {payModal.open && payModal.pricing && (
          <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm" onClick={() => !payModal.polling && setPayModal({ open: false, pricing: null, polling: false, orderNo: '' })}>
            <div className="bg-white dark:bg-gray-800 rounded-2xl shadow-2xl w-full max-w-md mx-4 overflow-hidden" onClick={e => e.stopPropagation()}>
              <div className="bg-gradient-to-r from-amber-500 to-orange-500 px-6 py-5 text-white">
                <h3 className="text-lg font-bold">购买智能体</h3>
                <p className="text-amber-100 text-sm mt-1">{item?.name}</p>
              </div>
              <div className="p-6">
                <div className="text-center mb-6">
                  <div className="text-3xl font-bold text-gray-900 dark:text-white">{payModal.pricing.display || `¥${(payModal.pricing.price / 100).toFixed(0)}`}</div>
                  <div className="text-sm text-gray-400 mt-1">{payModal.pricing.type === 'subscription' ? '订阅服务' : '一次性购买'}</div>
                </div>
                {payModal.polling ? (
                  <div className="text-center py-4">
                    <Loader2 className="w-8 h-8 animate-spin text-amber-500 mx-auto mb-3" />
                    <p className="text-sm text-gray-600 dark:text-gray-300 font-medium">等待支付完成...</p>
                    <p className="text-xs text-gray-400 mt-1">请在新窗口中完成支付，支付后自动安装</p>
                  </div>
                ) : (
                  <div className="space-y-3">
                    <button onClick={() => startPayment('alipay')} disabled={installing}
                      className="w-full flex items-center justify-center gap-3 px-4 py-3.5 bg-[#1677FF] hover:bg-[#0958d9] text-white rounded-xl font-medium transition-colors disabled:opacity-50">
                      <svg viewBox="0 0 24 24" className="w-5 h-5 fill-current"><path d="M21.422 15.358c-3.087-1.26-5.364-2.184-5.9-2.442.6-1.5.984-3.132.984-4.836 0-.396-.024-.786-.078-1.17h3.384V5.484h-4.05c-.414-1.296-1.002-2.448-1.002-2.448h-2.55s.828 1.614 1.17 2.448H7.476V6.91h5.82c-.168.774-.462 1.512-.87 2.184H7.476v1.426h3.936c-1.134 1.404-2.82 2.604-5.04 3.258l.936 1.578c2.046-.72 3.69-1.86 4.86-3.162.87.42 5.742 2.766 5.742 2.766C19.734 19.302 15.372 21.6 12 21.6 6.708 21.6 2.4 17.292 2.4 12 2.4 6.708 6.708 2.4 12 2.4S21.6 6.708 21.6 12c0 1.14-.168 2.244-.48 3.282l1.548.648c.21-.618.378-1.254.498-1.908A12.015 12.015 0 0024 12C24 5.373 18.627 0 12 0S0 5.373 0 12s5.373 12 12 12c4.32 0 8.1-2.28 10.224-5.706l-.802-.936z"/></svg>
                      支付宝支付
                    </button>
                    <button onClick={() => startPayment('wechatpay')} disabled={installing}
                      className="w-full flex items-center justify-center gap-3 px-4 py-3.5 bg-[#07C160] hover:bg-[#06ae56] text-white rounded-xl font-medium transition-colors disabled:opacity-50">
                      <svg viewBox="0 0 24 24" className="w-5 h-5 fill-current"><path d="M8.691 2.188C3.891 2.188 0 5.476 0 9.53c0 2.212 1.17 4.203 3.002 5.55a.59.59 0 01.213.665l-.39 1.48c-.019.07-.048.141-.048.213 0 .163.13.295.29.295a.326.326 0 00.167-.054l1.903-1.114a.864.864 0 01.717-.098 10.16 10.16 0 002.837.403c.276 0 .543-.027.811-.05a6.093 6.093 0 01-.248-1.747c0-3.738 3.464-6.756 7.724-6.756.258 0 .507.025.761.046C16.852 4.612 13.121 2.188 8.691 2.188zm-2.65 4.168a.97.97 0 11-.001 1.94.97.97 0 01.001-1.94zm5.311 0a.97.97 0 110 1.94.97.97 0 010-1.94zM24 15.078c0-3.39-3.384-6.14-7.563-6.14-4.178 0-7.563 2.75-7.563 6.14 0 3.393 3.385 6.14 7.563 6.14.86 0 1.683-.128 2.454-.35a.72.72 0 01.587.082l1.574.923a.26.26 0 00.136.044c.132 0 .241-.108.241-.243 0-.06-.023-.118-.039-.174l-.325-1.222a.488.488 0 01.175-.546C22.932 18.922 24 17.126 24 15.078zm-10.065-.946a.808.808 0 110-1.616.808.808 0 010 1.616zm5.004 0a.808.808 0 11-.001-1.616.808.808 0 01.001 1.616z"/></svg>
                      微信支付
                    </button>
                  </div>
                )}
                {!payModal.polling && (
                  <button onClick={() => setPayModal({ open: false, pricing: null, polling: false, orderNo: '' })}
                    className="w-full mt-3 px-4 py-2.5 text-sm text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-200 transition-colors">
                    取消
                  </button>
                )}
              </div>
            </div>
          </div>
        )}

        {/* Tab Content */}
        <div className="min-h-[400px]">
          {activeTab === 'gene' && <GeneTab name={item.name} description={item.description} cfg={cfg} />}
          {activeTab === 'skill' && <SkillTab builtinTools={builtinTools} passiveSkills={passiveSkills} />}
          {activeTab === 'instinct' && <InstinctTab proactiveSkills={proactiveSkills} />}
          {activeTab === 'mcp' && <MCPTab mcpTools={mcpTools} mcpServers={mcpServers} />}
          {activeTab === 'workflow' && <WorkflowTab workflows={workflows} />}
          {activeTab === 'memory' && <EmptyState icon={Brain} text="暂无记忆" desc="安装后对话中自动积累记忆" />}
        </div>
      </div>
    </div>
  )
}

// ── Tab 1: 基因 ──
function GeneTab({ name, description, cfg }: { name: string; description: string; cfg: any }) {
  const [collapsed, setCollapsed] = useState(false)
  return (
    <div className="space-y-6">
      <div className="bg-white dark:bg-gray-800 border dark:border-gray-700 rounded-2xl p-6">
        <h3 className="text-sm font-semibold text-gray-500 uppercase tracking-wider mb-4 flex items-center gap-2"><Dna className="w-4 h-4 text-rose-500" />身份基因</h3>
        <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
          <div className="bg-gray-50 dark:bg-gray-900 rounded-xl p-4"><div className="text-xs text-gray-400 mb-1">名称</div><div className="font-semibold text-gray-900 dark:text-white">{name}</div></div>
          <div className="bg-gray-50 dark:bg-gray-900 rounded-xl p-4"><div className="text-xs text-gray-400 mb-1">模型</div><div className="font-semibold text-gray-900 dark:text-white">{cfg.model_name || '使用默认模型'}</div></div>
          <div className="bg-gray-50 dark:bg-gray-900 rounded-xl p-4"><div className="text-xs text-gray-400 mb-1">温度</div><div className="font-semibold text-gray-900 dark:text-white">{cfg.temperature ?? 0.3}</div></div>
          <div className="bg-gray-50 dark:bg-gray-900 rounded-xl p-4"><div className="text-xs text-gray-400 mb-1">最大 Token</div><div className="font-semibold text-gray-900 dark:text-white">{cfg.max_tokens ?? 4096}</div></div>
        </div>
        {description && <div className="mt-4 bg-gray-50 dark:bg-gray-900 rounded-xl p-4"><div className="text-xs text-gray-400 mb-1">角色定位</div><div className="text-sm text-gray-700 dark:text-gray-300">{description}</div></div>}
      </div>
      <div className="bg-white dark:bg-gray-800 border dark:border-gray-700 rounded-2xl p-6">
        <div className="flex items-center justify-between mb-3">
          <h3 className="text-sm font-semibold text-gray-500 uppercase tracking-wider">规则</h3>
          {cfg.system_prompt && <button onClick={() => setCollapsed(!collapsed)} className="flex items-center gap-1 text-xs text-primary-600 hover:text-primary-700">{collapsed ? '展开' : '收起'}{collapsed ? <ChevronDown className="w-3 h-3" /> : <ChevronUp className="w-3 h-3" />}</button>}
        </div>
        {!collapsed && <div className="bg-gray-50 dark:bg-gray-900 rounded-xl p-4 max-h-[600px] overflow-y-auto"><pre className="text-sm text-gray-700 dark:text-gray-300 whitespace-pre-wrap font-sans leading-relaxed">{cfg.system_prompt || '(未设置)'}</pre></div>}
      </div>
    </div>
  )
}

// ── Tab 2: 技能 ──
function SkillTab({ builtinTools, passiveSkills }: { builtinTools: string[]; passiveSkills: BundleSkill[] }) {
  const activeGroups = skillGroups.filter(g => g.tools.some(t => builtinTools.includes(t)))
  return (
    <div className="space-y-6">
      {activeGroups.map(group => {
        const GroupIcon = group.icon
        const groupTools = group.tools.filter(t => builtinTools.includes(t))
        return (
          <div key={group.key} className="bg-white dark:bg-gray-800 border dark:border-gray-700 rounded-2xl p-6">
            <div className="flex items-center gap-2 mb-1"><GroupIcon className={`w-4 h-4 ${group.color}`} /><h3 className="text-sm font-semibold text-gray-700 dark:text-gray-200">{group.label}</h3><span className="text-[10px] text-gray-400 ml-1">{group.desc}</span></div>
            <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-2 mt-3">
              {groupTools.map(t => {
                const meta = toolMeta[t] || { label: t, icon: '⚡', desc: '' }
                return <div key={t} className="flex items-center gap-3 border dark:border-gray-700 rounded-lg p-3 bg-gray-50/50 dark:bg-gray-900/50 hover:bg-gray-50 dark:hover:bg-gray-900 transition-colors"><span className="text-lg flex-shrink-0">{meta.icon}</span><div className="min-w-0"><div className="font-medium text-gray-800 dark:text-gray-200 text-sm">{meta.label}</div><div className="text-[11px] text-gray-400 truncate">{meta.desc}</div></div></div>
              })}
            </div>
          </div>
        )
      })}
      {passiveSkills.length > 0 && (
        <div className="bg-white dark:bg-gray-800 border dark:border-gray-700 rounded-2xl p-6">
          <div className="flex items-center gap-2 mb-1"><Sparkles className="w-4 h-4 text-blue-500" /><h3 className="text-sm font-semibold text-gray-700 dark:text-gray-200">被动技能</h3><span className="text-[10px] text-gray-400 ml-1">用户提问时触发</span></div>
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3 mt-3">
            {passiveSkills.map(s => {
              const spec: SkillSpec = parseJSON(s.spec, { trigger: 'passive' })
              return (
                <div key={s.name} className="border dark:border-gray-700 rounded-xl p-4 hover:border-blue-300 dark:hover:border-blue-700 transition-colors bg-blue-50/30 dark:bg-blue-900/10">
                  <div className="flex items-center gap-2 mb-1.5"><span className="text-lg">{skillIcons[s.name] || '⚡'}</span><span className="font-semibold text-gray-800 dark:text-gray-200 text-sm">{s.name}</span></div>
                  <p className="text-xs text-gray-500 dark:text-gray-400 leading-relaxed">{spec.description || ''}</p>
                  {spec.example_triggers && spec.example_triggers.length > 0 && <div className="mt-2 flex flex-wrap gap-1">{spec.example_triggers.map((t, i) => <span key={i} className="text-[10px] bg-blue-100 dark:bg-blue-900/30 text-blue-600 dark:text-blue-400 px-1.5 py-0.5 rounded">"{t}"</span>)}</div>}
                </div>
              )
            })}
          </div>
        </div>
      )}
      {activeGroups.length === 0 && passiveSkills.length === 0 && <EmptyState icon={Wrench} text="暂无技能" desc="此智能体没有配置任何工具或技能" />}
    </div>
  )
}

// ── Tab 3: 本能 ──
function InstinctTab({ proactiveSkills }: { proactiveSkills: BundleSkill[] }) {
  if (!proactiveSkills.length) return <EmptyState icon={Zap} text="暂无本能" desc="本能是智能体自动执行的能力，不需要用户触发" />
  return (
    <div className="bg-white dark:bg-gray-800 border dark:border-gray-700 rounded-2xl p-6">
      <div className="flex items-center gap-2 mb-1"><Zap className="w-4 h-4 text-amber-500" /><h3 className="text-sm font-semibold text-gray-700 dark:text-gray-200">主动技能</h3><span className="text-[10px] text-gray-400 ml-1">定时自动执行</span></div>
      <div className="space-y-2 mt-3">
        {proactiveSkills.map(s => {
          const spec: SkillSpec = parseJSON(s.spec, { trigger: 'proactive' })
          return (
            <div key={s.name} className="flex items-center gap-3 border dark:border-gray-700 rounded-xl p-3.5 bg-amber-50/30 dark:bg-amber-900/10 hover:border-amber-300 dark:hover:border-amber-700 transition-colors">
              <span className="text-lg flex-shrink-0">{skillIcons[s.name] || '⚡'}</span>
              <div className="flex-1 min-w-0">
                <div className="flex items-center gap-2"><span className="font-medium text-gray-800 dark:text-gray-200 text-sm">{s.name}</span></div>
                <div className="text-[11px] text-gray-500 dark:text-gray-400 mt-0.5">{spec.description || ''}</div>
              </div>
              {spec.schedule && <div className="flex items-center gap-1 text-[11px] text-amber-600 dark:text-amber-400 bg-amber-100 dark:bg-amber-900/30 px-2 py-0.5 rounded flex-shrink-0"><Clock className="w-3 h-3" />{spec.schedule}</div>}
            </div>
          )
        })}
      </div>
    </div>
  )
}

// ── Tab 4: 外接服务 ──
function MCPTab({ mcpTools, mcpServers }: { mcpTools: string[]; mcpServers: BundleMCP[] }) {
  if (!mcpTools.length && !mcpServers.length) return <EmptyState icon={Plug} text="暂无外接服务" desc="通过 MCP 协议连接第三方工具和数据源" />
  return (
    <div className="space-y-6">
      {mcpServers.length > 0 && (
        <div className="bg-white dark:bg-gray-800 border dark:border-gray-700 rounded-2xl p-6">
          <div className="flex items-center gap-2 mb-1"><Plug className="w-4 h-4 text-purple-500" /><h3 className="text-sm font-semibold text-gray-700 dark:text-gray-200">MCP 服务器</h3></div>
          <div className="space-y-2 mt-3">
            {mcpServers.map(s => (
              <div key={s.name} className="flex items-center gap-3 border dark:border-gray-700 rounded-xl p-3 bg-purple-50/30 dark:bg-purple-900/10">
                <div className="w-2 h-2 rounded-full bg-purple-500 flex-shrink-0" />
                <div className="flex-1 min-w-0">
                  <div className="font-medium text-gray-800 dark:text-gray-200 text-sm">{s.name}</div>
                  <div className="text-[11px] text-gray-500 dark:text-gray-400">{s.description}</div>
                  <div className="text-[10px] text-gray-400 mt-0.5">{s.base_url}</div>
                </div>
                <span className="text-[10px] text-purple-500 bg-purple-100 dark:bg-purple-900/30 px-2 py-0.5 rounded font-medium">MCP</span>
              </div>
            ))}
          </div>
        </div>
      )}
      {mcpTools.length > 0 && (
        <div className="bg-white dark:bg-gray-800 border dark:border-gray-700 rounded-2xl p-6">
          <div className="flex items-center gap-2 mb-1"><Wrench className="w-4 h-4 text-purple-500" /><h3 className="text-sm font-semibold text-gray-700 dark:text-gray-200">已绑定外接工具</h3></div>
          <div className="space-y-2 mt-3">
            {mcpTools.map(t => {
              const meta = toolMeta[t] || { label: t.replace(/^mcp_/, '').replace(/_/g, ' '), icon: '🔗', desc: '' }
              return <div key={t} className="flex items-center gap-3 border dark:border-gray-700 rounded-xl p-3 bg-purple-50/30 dark:bg-purple-900/10"><span className="text-lg">{meta.icon}</span><div className="flex-1"><div className="font-medium text-gray-800 dark:text-gray-200 text-sm">{meta.label}</div>{meta.desc && <div className="text-[11px] text-gray-400">{meta.desc}</div>}</div><span className="text-[10px] text-purple-500 bg-purple-100 dark:bg-purple-900/30 px-2 py-0.5 rounded font-medium">外接</span></div>
            })}
          </div>
        </div>
      )}
    </div>
  )
}

// ── Tab 5: 工作流 ──
function WorkflowTab({ workflows }: { workflows: BundleWorkflow[] }) {
  if (!workflows.length) return <EmptyState icon={GitBranch} text="暂无工作流" desc="安装后可在工作流编辑器中查看和修改" />
  return (
    <div className="space-y-4">
      {workflows.map(wf => (
        <div key={wf.name} className="bg-white dark:bg-gray-800 border dark:border-gray-700 rounded-2xl p-6">
          <div className="flex items-center gap-2 mb-3"><GitBranch className="w-4 h-4 text-emerald-500" /><h3 className="text-sm font-semibold text-gray-700 dark:text-gray-200">{wf.name}</h3></div>
          <div className="bg-emerald-50/50 dark:bg-emerald-900/10 border border-emerald-200 dark:border-emerald-800 rounded-xl p-4">
            <p className="text-sm text-gray-700 dark:text-gray-300">{wf.description}</p>
          </div>
        </div>
      ))}
    </div>
  )
}

// ── Empty State ──
function EmptyState({ icon: Icon, text, desc }: { icon: typeof Wrench; text: string; desc: string }) {
  return (
    <div className="bg-white dark:bg-gray-800 border dark:border-gray-700 rounded-2xl p-10 text-center">
      <Icon className="w-10 h-10 text-gray-300 dark:text-gray-600 mx-auto mb-3" />
      <p className="text-gray-500 dark:text-gray-400 text-sm font-medium">{text}</p>
      <p className="text-xs text-gray-400 dark:text-gray-500 mt-1">{desc}</p>
    </div>
  )
}
