import { useState, useEffect } from 'react'
import { Paintbrush, Key, ToggleLeft, Save, Shield, CheckCircle, XCircle, AlertTriangle } from 'lucide-react'
import { broodAPI, type BrandConfig, type LicenseKeyInfo, type TierLimits, type FeatureWithAccess } from '../api/brood'

const TIER_LABELS: Record<string, { label: string; color: string }> = {
  community:  { label: 'Community',   color: 'text-gray-400 bg-gray-500/10' },
  starter:    { label: 'Starter',     color: 'text-blue-400 bg-blue-500/10' },
  pro:        { label: 'Pro',         color: 'text-purple-400 bg-purple-500/10' },
  enterprise: { label: 'Enterprise',  color: 'text-amber-400 bg-amber-500/10' },
  whitelabel: { label: 'White-Label', color: 'text-green-400 bg-green-500/10' },
}

const CATEGORY_LABELS: Record<string, string> = {
  core: '核心功能', advanced: '高级功能', enterprise: '企业功能', whitelabel: '白牌功能',
}

export default function BrandPage() {
  const [tab, setTab] = useState<'brand' | 'license' | 'features'>('license')

  // License state
  const [license, setLicense] = useState<LicenseKeyInfo | null>(null)
  const [tier, setTier] = useState('community')
  const [limits, setLimits] = useState<TierLimits | null>(null)
  const [activateKey, setActivateKey] = useState('')
  const [activateMsg, setActivateMsg] = useState('')

  // Brand state
  const [brand, setBrand] = useState<Partial<BrandConfig>>({})
  const [brandSaved, setBrandSaved] = useState(false)

  // Features state
  const [features, setFeatures] = useState<FeatureWithAccess[]>([])

  useEffect(() => {
    loadLicense()
    loadBrand()
    loadFeatures()
  }, [])

  const loadLicense = async () => {
    try {
      const res = await broodAPI.getLicense()
      setLicense(res.license)
      setTier(res.tier)
      setLimits(res.limits)
    } catch {}
  }

  const loadBrand = async () => {
    try {
      const res = await broodAPI.getBrand()
      setBrand(res.brand)
    } catch {}
  }

  const loadFeatures = async () => {
    try {
      const res = await broodAPI.listFeatures()
      setFeatures(res.features)
      setTier(res.current_tier)
    } catch {}
  }

  const handleActivate = async () => {
    if (!activateKey.trim()) return
    try {
      const res = await broodAPI.activateLicense(activateKey.trim())
      setActivateMsg(res.message)
      setActivateKey('')
      loadLicense()
      loadFeatures()
    } catch (e) {
      setActivateMsg(e instanceof Error ? e.message : '激活失败')
    }
  }

  const handleSaveBrand = async () => {
    try {
      const res = await broodAPI.updateBrand(brand)
      setBrand(res.brand)
      setBrandSaved(true)
      setTimeout(() => setBrandSaved(false), 2000)
    } catch {}
  }

  const handleToggleFeature = async (id: string, enabled: boolean) => {
    try {
      await broodAPI.updateFeature(id, { enabled })
      loadFeatures()
    } catch {}
  }

  const tierInfo = TIER_LABELS[tier] || TIER_LABELS.community

  return (
    <div className="p-8">
      <div className="max-w-5xl mx-auto space-y-6">
        {/* Header */}
        <div>
          <h1 className="text-2xl font-bold text-white">白牌 & 许可证</h1>
          <p className="text-sm text-gray-400 mt-1">品牌配置、许可证管理、功能开关矩阵</p>
        </div>

        {/* Current tier card */}
        <div className="rounded-xl border border-gray-800 bg-gray-900/50 p-5 flex items-center justify-between">
          <div className="flex items-center gap-4">
            <div className="w-12 h-12 rounded-xl bg-overlord-600/20 flex items-center justify-center">
              <Shield className="w-6 h-6 text-overlord-400" />
            </div>
            <div>
              <div className="text-sm text-gray-400">当前许可证</div>
              <div className="flex items-center gap-2 mt-0.5">
                <span className={`text-sm font-bold px-2.5 py-0.5 rounded ${tierInfo.color}`}>{tierInfo.label}</span>
                {license && (
                  <span className="text-xs text-gray-500">
                    {license.holder} · {license.expires_at ? `到期: ${new Date(license.expires_at).toLocaleDateString()}` : '永久'}
                  </span>
                )}
              </div>
            </div>
          </div>
          {limits && (
            <div className="flex gap-6 text-xs">
              <div><span className="text-gray-500">节点上限:</span> <span className="text-white">{limits.MaxNodes || '不限'}</span></div>
              <div><span className="text-gray-500">团队上限:</span> <span className="text-white">{limits.MaxTeams || '不限'}</span></div>
              <div><span className="text-gray-500">SSO:</span> <span className={limits.SSOEnabled ? 'text-green-400' : 'text-gray-600'}>{limits.SSOEnabled ? '✓' : '✗'}</span></div>
              <div><span className="text-gray-500">合规:</span> <span className={limits.Compliance ? 'text-green-400' : 'text-gray-600'}>{limits.Compliance ? '✓' : '✗'}</span></div>
              <div><span className="text-gray-500">品牌定制:</span> <span className={limits.BrandCustom ? 'text-green-400' : 'text-gray-600'}>{limits.BrandCustom ? '✓' : '✗'}</span></div>
            </div>
          )}
        </div>

        {/* Tabs */}
        <div className="flex gap-1 border-b border-gray-800 pb-px">
          {([
            { key: 'license', label: '许可证', icon: Key },
            { key: 'features', label: '功能开关', icon: ToggleLeft },
            { key: 'brand', label: '品牌配置', icon: Paintbrush },
          ] as const).map(t => (
            <button key={t.key} onClick={() => setTab(t.key)}
              className={`flex items-center gap-2 px-4 py-2.5 text-sm rounded-t-lg transition-colors ${
                tab === t.key ? 'bg-gray-800 text-white font-medium' : 'text-gray-400 hover:text-gray-200 hover:bg-gray-800/50'
              }`}>
              <t.icon size={15} /> {t.label}
            </button>
          ))}
        </div>

        {/* License tab */}
        {tab === 'license' && (
          <div className="space-y-6">
            {/* Activate license */}
            <div className="rounded-xl border border-gray-800 bg-gray-900/50 p-5 space-y-4">
              <h3 className="text-sm font-medium text-white">激活许可证</h3>
              <div className="flex gap-3">
                <input placeholder="输入许可证密钥" value={activateKey} onChange={e => setActivateKey(e.target.value)}
                  className="flex-1 rounded-lg border border-gray-700 bg-gray-800 px-4 py-2.5 text-sm text-white font-mono focus:outline-none focus:border-overlord-500" />
                <button onClick={handleActivate}
                  className="rounded-lg bg-overlord-600 px-6 py-2.5 text-sm font-medium text-white hover:bg-overlord-500">
                  激活
                </button>
              </div>
              {activateMsg && <div className="text-sm text-overlord-300">{activateMsg}</div>}
            </div>

            {/* License matrix */}
            <div className="rounded-xl border border-gray-800 bg-gray-900/50 p-5">
              <h3 className="text-sm font-medium text-white mb-4">功能矩阵</h3>
              <div className="overflow-x-auto">
                <table className="w-full text-xs">
                  <thead>
                    <tr className="text-gray-400 border-b border-gray-800">
                      <th className="text-left py-2 pr-4 font-medium">功能</th>
                      {Object.entries(TIER_LABELS).map(([k, v]) => (
                        <th key={k} className={`py-2 px-3 font-medium text-center ${k === tier ? 'text-overlord-300' : ''}`}>
                          {v.label}{k === tier ? ' ←' : ''}
                        </th>
                      ))}
                    </tr>
                  </thead>
                  <tbody>
                    {[
                      { name: '节点管理', vals: ['≤10', '≤20', '≤100', '≤500', '不限'] },
                      { name: '团队数', vals: ['1', '3', '不限', '不限', '不限'] },
                      { name: 'SSO', vals: ['✗', '✗', '✓', '✓', '✓'] },
                      { name: '审计日志', vals: ['7天', '30天', '180天', '永久', '永久'] },
                      { name: '用量分析', vals: ['基础', '基础', '高级', '高级', '高级'] },
                      { name: '合规面板', vals: ['✗', '✗', '✗', '✓', '✓'] },
                      { name: '品牌定制', vals: ['✗', '✗', '✗', '✗', '✓'] },
                      { name: '功能开关', vals: ['✗', '✗', '✗', '✗', '✓'] },
                      { name: 'SLA', vals: ['—', '—', '99.5%', '99.9%', '99.9%'] },
                    ].map((row, i) => (
                      <tr key={i} className="border-b border-gray-800/50">
                        <td className="py-2 pr-4 text-gray-300">{row.name}</td>
                        {row.vals.map((v, j) => {
                          const tiers = Object.keys(TIER_LABELS)
                          const isCurrent = tiers[j] === tier
                          return (
                            <td key={j} className={`py-2 px-3 text-center ${
                              isCurrent ? 'text-overlord-300 bg-overlord-600/5' :
                              v === '✓' ? 'text-green-400' : v === '✗' ? 'text-gray-600' : 'text-gray-400'
                            }`}>{v}</td>
                          )
                        })}
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </div>

            {/* Current license details */}
            {license && (
              <div className="rounded-xl border border-gray-800 bg-gray-900/50 p-5">
                <h3 className="text-sm font-medium text-white mb-3">当前许可证详情</h3>
                <div className="grid grid-cols-2 gap-3 text-sm">
                  <div><span className="text-gray-500">密钥:</span> <span className="text-white font-mono text-xs">{license.key.slice(0, 12)}...{license.key.slice(-4)}</span></div>
                  <div><span className="text-gray-500">持有人:</span> <span className="text-white">{license.holder || '-'}</span></div>
                  <div><span className="text-gray-500">邮箱:</span> <span className="text-white">{license.email || '-'}</span></div>
                  <div><span className="text-gray-500">状态:</span> <span className={license.status === 'active' ? 'text-green-400' : 'text-red-400'}>{license.status}</span></div>
                  <div><span className="text-gray-500">颁发时间:</span> <span className="text-white">{new Date(license.issued_at).toLocaleDateString()}</span></div>
                  <div><span className="text-gray-500">到期:</span> <span className="text-white">{license.expires_at ? new Date(license.expires_at).toLocaleDateString() : '永久'}</span></div>
                </div>
              </div>
            )}
          </div>
        )}

        {/* Features tab */}
        {tab === 'features' && (
          <div className="space-y-4">
            {Object.entries(CATEGORY_LABELS).map(([cat, catLabel]) => {
              const catFeatures = features.filter(f => f.category === cat)
              if (catFeatures.length === 0) return null
              return (
                <div key={cat} className="rounded-xl border border-gray-800 bg-gray-900/50 p-5">
                  <h3 className="text-sm font-medium text-white mb-3">{catLabel}</h3>
                  <div className="space-y-2">
                    {catFeatures.map(f => {
                      const tierReq = TIER_LABELS[f.min_tier] || TIER_LABELS.community
                      return (
                        <div key={f.id} className="flex items-center justify-between py-2 border-b border-gray-800/50 last:border-0">
                          <div className="flex items-center gap-3">
                            {f.has_access ? (
                              <CheckCircle size={16} className="text-green-400" />
                            ) : (
                              <XCircle size={16} className="text-gray-600" />
                            )}
                            <div>
                              <div className="text-sm text-white">{f.name}</div>
                              {f.description && <div className="text-xs text-gray-500">{f.description}</div>}
                            </div>
                          </div>
                          <div className="flex items-center gap-3">
                            <span className={`text-[10px] px-1.5 py-0.5 rounded ${tierReq.color}`}>{tierReq.label}+</span>
                            <button
                              onClick={() => handleToggleFeature(f.id, !f.enabled)}
                              className={`relative w-10 h-5 rounded-full transition-colors ${f.enabled ? 'bg-overlord-600' : 'bg-gray-700'}`}
                            >
                              <div className={`absolute top-0.5 w-4 h-4 rounded-full bg-white transition-transform ${f.enabled ? 'left-5.5 translate-x-0.5' : 'left-0.5'}`} />
                            </button>
                          </div>
                        </div>
                      )
                    })}
                  </div>
                </div>
              )
            })}
          </div>
        )}

        {/* Brand tab */}
        {tab === 'brand' && (
          <div className="space-y-6">
            {tier !== 'whitelabel' && (
              <div className="rounded-lg bg-amber-500/10 border border-amber-500/20 p-4 flex items-center gap-3">
                <AlertTriangle size={18} className="text-amber-400 shrink-0" />
                <div className="text-sm text-amber-300">品牌定制功能需要 White-Label 许可证。当前许可证: {tierInfo.label}</div>
              </div>
            )}

            <div className="grid grid-cols-2 gap-6">
              {/* Brand info */}
              <div className="rounded-xl border border-gray-800 bg-gray-900/50 p-5 space-y-4">
                <h3 className="text-sm font-medium text-white">品牌信息</h3>
                <div className="space-y-3">
                  <div>
                    <label className="block text-xs text-gray-400 mb-1">品牌名称</label>
                    <input value={brand.brand_name || ''} onChange={e => setBrand({ ...brand, brand_name: e.target.value })}
                      className="w-full rounded-lg border border-gray-700 bg-gray-800 px-3 py-2 text-sm text-white focus:outline-none focus:border-overlord-500" />
                  </div>
                  <div>
                    <label className="block text-xs text-gray-400 mb-1">Logo URL</label>
                    <input value={brand.logo_url || ''} onChange={e => setBrand({ ...brand, logo_url: e.target.value })}
                      className="w-full rounded-lg border border-gray-700 bg-gray-800 px-3 py-2 text-sm text-white focus:outline-none focus:border-overlord-500" />
                  </div>
                  <div>
                    <label className="block text-xs text-gray-400 mb-1">Favicon URL</label>
                    <input value={brand.favicon_url || ''} onChange={e => setBrand({ ...brand, favicon_url: e.target.value })}
                      className="w-full rounded-lg border border-gray-700 bg-gray-800 px-3 py-2 text-sm text-white focus:outline-none focus:border-overlord-500" />
                  </div>
                  <div>
                    <label className="block text-xs text-gray-400 mb-1">自定义域名</label>
                    <input value={brand.domain || ''} onChange={e => setBrand({ ...brand, domain: e.target.value })}
                      placeholder="ai.your-brand.com"
                      className="w-full rounded-lg border border-gray-700 bg-gray-800 px-3 py-2 text-sm text-white focus:outline-none focus:border-overlord-500" />
                  </div>
                  <div>
                    <label className="block text-xs text-gray-400 mb-1">客服邮箱</label>
                    <input value={brand.support_email || ''} onChange={e => setBrand({ ...brand, support_email: e.target.value })}
                      className="w-full rounded-lg border border-gray-700 bg-gray-800 px-3 py-2 text-sm text-white focus:outline-none focus:border-overlord-500" />
                  </div>
                </div>
              </div>

              {/* Colors + login */}
              <div className="space-y-4">
                <div className="rounded-xl border border-gray-800 bg-gray-900/50 p-5 space-y-4">
                  <h3 className="text-sm font-medium text-white">色调配置</h3>
                  <div className="grid grid-cols-2 gap-3">
                    {([
                      { key: 'primary_color', label: '主色' },
                      { key: 'secondary_color', label: '辅色' },
                      { key: 'accent_color', label: '强调色' },
                      { key: 'bg_color', label: '背景色' },
                    ] as const).map(c => (
                      <div key={c.key}>
                        <label className="block text-xs text-gray-400 mb-1">{c.label}</label>
                        <div className="flex items-center gap-2">
                          <input type="color" value={(brand as Record<string, string>)[c.key] || '#6d28d9'}
                            onChange={e => setBrand({ ...brand, [c.key]: e.target.value })}
                            className="w-8 h-8 rounded border border-gray-700 cursor-pointer" />
                          <input value={(brand as Record<string, string>)[c.key] || ''}
                            onChange={e => setBrand({ ...brand, [c.key]: e.target.value })}
                            className="flex-1 rounded-lg border border-gray-700 bg-gray-800 px-2 py-1.5 text-xs text-white font-mono focus:outline-none focus:border-overlord-500" />
                        </div>
                      </div>
                    ))}
                  </div>
                </div>

                <div className="rounded-xl border border-gray-800 bg-gray-900/50 p-5 space-y-3">
                  <h3 className="text-sm font-medium text-white">登录页 & 页脚</h3>
                  <div>
                    <label className="block text-xs text-gray-400 mb-1">登录标题</label>
                    <input value={brand.login_title || ''} onChange={e => setBrand({ ...brand, login_title: e.target.value })}
                      placeholder="欢迎使用 [你的品牌名]"
                      className="w-full rounded-lg border border-gray-700 bg-gray-800 px-3 py-2 text-sm text-white focus:outline-none focus:border-overlord-500" />
                  </div>
                  <div>
                    <label className="block text-xs text-gray-400 mb-1">版权信息</label>
                    <input value={brand.copyright_text || ''} onChange={e => setBrand({ ...brand, copyright_text: e.target.value })}
                      placeholder="© 2026 Your Company"
                      className="w-full rounded-lg border border-gray-700 bg-gray-800 px-3 py-2 text-sm text-white focus:outline-none focus:border-overlord-500" />
                  </div>
                  <div>
                    <label className="block text-xs text-gray-400 mb-1">ICP 备案号</label>
                    <input value={brand.icp_number || ''} onChange={e => setBrand({ ...brand, icp_number: e.target.value })}
                      className="w-full rounded-lg border border-gray-700 bg-gray-800 px-3 py-2 text-sm text-white focus:outline-none focus:border-overlord-500" />
                  </div>
                  <div className="flex items-center gap-6 pt-1">
                    <label className="flex items-center gap-2 text-sm text-gray-300 cursor-pointer">
                      <input type="checkbox" checked={brand.enabled || false} onChange={e => setBrand({ ...brand, enabled: e.target.checked })}
                        className="rounded border-gray-600" />
                      启用白牌模式
                    </label>
                    <label className="flex items-center gap-2 text-sm text-gray-300 cursor-pointer">
                      <input type="checkbox" checked={brand.powered_by !== false} onChange={e => setBrand({ ...brand, powered_by: e.target.checked })}
                        className="rounded border-gray-600" />
                      显示 "Powered by StarClaw"
                    </label>
                  </div>
                </div>
              </div>
            </div>

            <div className="flex justify-end">
              <button onClick={handleSaveBrand}
                className="inline-flex items-center gap-2 rounded-lg bg-overlord-600 px-6 py-2.5 text-sm font-medium text-white hover:bg-overlord-500">
                <Save size={16} /> {brandSaved ? '已保存 ✓' : '保存品牌配置'}
              </button>
            </div>
          </div>
        )}
      </div>
    </div>
  )
}
