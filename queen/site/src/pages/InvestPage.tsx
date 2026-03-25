import { Layout } from '../components/Layout'
import {
  Diamond,
  TrendingUp,
  Shield,
  Users,
  Layers,
  ArrowRight,
  CheckCircle2,
  Clock,
  Gem,
} from 'lucide-react'

const ROUNDS = [
  { label: '孢子期', price: '¥0.50', raised: '¥500万', mult: '1×', status: 'open' },
  { label: '幼虫期', price: '¥2.50', raised: '¥2,500万', mult: '5×', status: 'upcoming' },
  { label: '虫兵期', price: '¥12.50', raised: '¥1.25亿', mult: '25×', status: 'upcoming' },
  { label: '领主期', price: '¥62.50', raised: '¥6.25亿', mult: '125×', status: 'upcoming' },
  { label: '虫后期', price: '¥312.50', raised: '¥31.25亿', mult: '625×', status: 'upcoming' },
]

const HIGHLIGHTS = [
  { icon: Shield, title: '收益权凭证', desc: '非股权、非代币、非基金份额。投资人签署《收益权转让协议》，合规合法。' },
  { icon: TrendingUp, title: '双驱动定价', desc: '价格 = max(NAV净值, 轮次地板价)。双驱动定价机制为价格提供支撑，价格随业务发展动态调整。' },
  { icon: Users, title: '利润分配', desc: '平台交易利润的 10% 按份额分配给星钻持有人，按月结算。' },
  { icon: Layers, title: '总量恒定', desc: '星钻总量 1 亿份，固定永不增发。稀缺性保证价值。' },
]

const PRODUCT_STATUS = [
  { name: 'AI Agent 引擎（多模型/RAG/MCP/工作流）', done: true },
  { name: '企业管控台（RBAC/隧道/OTA/Webhook）', done: true },
  { name: 'AI 算力网关（40+ 模型/用量计费）', done: true },
  { name: 'Web UI（流式对话/深色模式/国际化）', done: true },
  { name: 'P2P 加密通信（Ed25519/Gossip）', done: true },
  { name: 'Agent 市场（创作者经济/分成）', done: true },
  { name: '移动端 App（Flutter）', done: false },
]

export function InvestPage() {
  return (
    <Layout>
      {/* Hero */}
      <section className="relative overflow-hidden">
        <div className="absolute inset-0 bg-gradient-to-b from-amber-950/20 via-transparent to-transparent" />
        <div className="mx-auto max-w-5xl px-4 sm:px-6 pt-16 sm:pt-24 pb-12 text-center relative">
          <div className="inline-flex items-center gap-2 rounded-full border border-amber-500/30 bg-amber-500/10 px-4 py-1.5 text-sm text-amber-400 mb-8">
            <Diamond size={14} />
            孢子期开放中
          </div>

          <h1 className="text-3xl sm:text-5xl md:text-6xl font-bold tracking-tight leading-tight">
            星钻
            <span className="text-transparent bg-clip-text bg-gradient-to-r from-amber-400 to-orange-400">
              {' '}Star Diamond
            </span>
          </h1>

          <p className="mt-6 text-lg md:text-xl text-gray-400 max-w-2xl mx-auto leading-relaxed">
            StarClaw 平台利润池收益权凭证。持有星钻，按份额享受平台交易利润的 10% 分配。
          </p>

          <div className="mt-10">
            <a
              href="mailto:invest@starclaw.net"
              className="inline-flex items-center gap-2 rounded-lg bg-amber-600 px-8 py-3.5 text-sm font-semibold text-white hover:bg-amber-500 transition-colors"
            >
              <Gem size={18} />
              联系我们
              <ArrowRight size={16} />
            </a>
          </div>
        </div>
      </section>

      {/* Current Round Banner */}
      <section className="py-8 border-t border-white/5">
        <div className="mx-auto max-w-5xl px-4 sm:px-6">
          <div className="rounded-2xl border border-amber-500/20 bg-amber-500/[0.04] p-8 md:p-12">
            <div className="grid grid-cols-1 md:grid-cols-4 gap-8 text-center">
              <div>
                <div className="text-sm text-gray-400 mb-1">当前轮次</div>
                <div className="text-2xl font-bold text-amber-400">孢子期</div>
              </div>
              <div>
                <div className="text-sm text-gray-400 mb-1">当前价格</div>
                <div className="text-2xl font-bold text-white">¥0.50<span className="text-sm text-gray-400">/份</span></div>
              </div>
              <div>
                <div className="text-sm text-gray-400 mb-1">本轮额度</div>
                <div className="text-2xl font-bold text-white">1,000万<span className="text-sm text-gray-400">份</span></div>
              </div>
              <div>
                <div className="text-sm text-gray-400 mb-1">本轮募资</div>
                <div className="text-2xl font-bold text-white">¥500万</div>
              </div>
            </div>
          </div>
        </div>
      </section>

      {/* Key Features */}
      <section className="py-16 border-t border-white/5">
        <div className="mx-auto max-w-5xl px-4 sm:px-6">
          <h2 className="text-2xl sm:text-3xl font-bold tracking-tight text-center mb-12">
            星钻核心机制
          </h2>
          <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
            {HIGHLIGHTS.map((h) => (
              <div
                key={h.title}
                className="rounded-xl border border-white/10 bg-white/[0.02] p-6 hover:border-amber-500/20 transition-colors"
              >
                <div className="inline-flex items-center justify-center w-10 h-10 rounded-lg bg-amber-500/10 text-amber-400 mb-4">
                  <h.icon size={20} />
                </div>
                <h3 className="text-lg font-semibold text-white mb-2">{h.title}</h3>
                <p className="text-sm text-gray-400 leading-relaxed">{h.desc}</p>
              </div>
            ))}
          </div>
        </div>
      </section>

      {/* Funding Rounds Table */}
      <section className="py-16 border-t border-white/5">
        <div className="mx-auto max-w-5xl px-4 sm:px-6">
          <h2 className="text-2xl sm:text-3xl font-bold tracking-tight text-center mb-4">
            5 轮融资路线
          </h2>
          <p className="text-center text-gray-400 mb-12">
            每轮 10% 份额（1,000万份），价格 5× 递增，总量 1 亿份永不增发
          </p>

          <div className="overflow-x-auto">
            <table className="w-full text-left">
              <thead>
                <tr className="border-b border-white/10">
                  <th className="py-3 px-4 text-sm font-medium text-gray-400">轮次</th>
                  <th className="py-3 px-4 text-sm font-medium text-gray-400 text-right">地板价</th>
                  <th className="py-3 px-4 text-sm font-medium text-gray-400 text-right">倍数</th>
                  <th className="py-3 px-4 text-sm font-medium text-gray-400 text-right">募资</th>
                  <th className="py-3 px-4 text-sm font-medium text-gray-400 text-center">状态</th>
                </tr>
              </thead>
              <tbody>
                {ROUNDS.map((r) => (
                  <tr key={r.label} className="border-b border-white/5 hover:bg-white/[0.02]">
                    <td className="py-4 px-4 font-semibold text-white">{r.label}</td>
                    <td className="py-4 px-4 text-right text-white">{r.price}</td>
                    <td className="py-4 px-4 text-right text-gray-400">{r.mult}</td>
                    <td className="py-4 px-4 text-right text-white">{r.raised}</td>
                    <td className="py-4 px-4 text-center">
                      {r.status === 'open' ? (
                        <span className="inline-flex items-center gap-1 rounded-full bg-green-500/10 border border-green-500/30 px-3 py-1 text-xs font-medium text-green-400">
                          <span className="w-1.5 h-1.5 rounded-full bg-green-400 animate-pulse" />
                          开放中
                        </span>
                      ) : (
                        <span className="inline-flex items-center gap-1 rounded-full bg-white/5 border border-white/10 px-3 py-1 text-xs font-medium text-gray-500">
                          <Clock size={10} />
                          即将开启
                        </span>
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
              <tfoot>
                <tr className="border-t border-white/10">
                  <td className="py-4 px-4 font-semibold text-gray-400">5轮合计</td>
                  <td className="py-4 px-4" />
                  <td className="py-4 px-4" />
                  <td className="py-4 px-4 text-right font-bold text-amber-400">¥39.75 亿</td>
                  <td className="py-4 px-4" />
                </tr>
              </tfoot>
            </table>
          </div>
        </div>
      </section>

      {/* Pricing Mechanism */}
      <section className="py-16 border-t border-white/5">
        <div className="mx-auto max-w-5xl px-4 sm:px-6">
          <h2 className="text-2xl sm:text-3xl font-bold tracking-tight text-center mb-12">
            定价机制
          </h2>
          <div className="rounded-xl border border-white/10 bg-white/[0.02] p-8">
            <div className="max-w-2xl mx-auto space-y-4 text-sm text-gray-400 leading-relaxed">
              <p>星钻采用双驱动定价机制：价格 = max(NAV 净值, 轮次地板价)。</p>
              <p><strong className="text-white">业务驱动：</strong>平台利润累积推高 NAV 净值，反映实际业务增长。</p>
              <p><strong className="text-white">里程碑驱动：</strong>融资轮次推进抬高地板价，为早期投资人提供价格支撑。</p>
              <p><strong className="text-white">退出机制：</strong>协议到期后按当时 NAV 结算退出，公司有优先回购权。</p>
              <p className="text-xs text-gray-600 border-t border-white/5 pt-4">
                * 价格随业务发展动态调整，不构成收益承诺。投资有风险，决策需谨慎。
              </p>
            </div>
          </div>
        </div>
      </section>

      {/* Product Status */}
      <section className="py-16 border-t border-white/5">
        <div className="mx-auto max-w-5xl px-4 sm:px-6">
          <h2 className="text-2xl sm:text-3xl font-bold tracking-tight text-center">
            加入孢子期
          </h2>
          <p className="text-center text-gray-400 mb-12">
            一人全栈完成，代码可运行、可演示
          </p>
          <div className="max-w-lg mx-auto space-y-3">
            {PRODUCT_STATUS.map((p) => (
              <div key={p.name} className="flex items-center gap-3 py-2">
                <CheckCircle2
                  size={18}
                  className={p.done ? 'text-green-400 shrink-0' : 'text-gray-600 shrink-0'}
                />
                <span className={p.done ? 'text-white' : 'text-gray-500'}>{p.name}</span>
                {!p.done && (
                  <span className="ml-auto text-xs text-gray-600">开发中</span>
                )}
              </div>
            ))}
          </div>
        </div>
      </section>

      {/* CTA */}
      <section className="py-16 border-t border-white/5">
        <div className="mx-auto max-w-3xl px-4 sm:px-6 text-center">
          <h2 className="text-2xl sm:text-3xl font-bold tracking-tight">
            加入孢子期
          </h2>
          <p className="mt-4 text-gray-400 text-lg">
            孢子期开放中，名额有限，欢迎一对一沟通。
          </p>
          <div className="mt-8">
            <a
              href="mailto:invest@starclaw.net"
              className="inline-flex items-center gap-2 rounded-lg bg-amber-600 px-8 py-3.5 text-sm font-semibold text-white hover:bg-amber-500 transition-colors"
            >
              <Gem size={18} />
              invest@starclaw.net
            </a>
          </div>
          <p className="mt-6 text-xs text-gray-600">
            本页面仅供信息展示，不构成公开募集或收益承诺。星钻为收益权凭证，投资人需签署《收益权转让协议》。
            协议到期后按当时 NAV 结算退出，公司享有优先回购权。投资有风险，决策需谨慎。
          </p>
        </div>
      </section>
    </Layout>
  )
}
