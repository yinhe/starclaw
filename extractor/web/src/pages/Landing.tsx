import { useState } from 'react'
import { Link } from 'react-router-dom'
import { TrendingUp, Shield, Brain, BarChart3, Clock, Eye, Users, ChevronRight, Globe2 } from 'lucide-react'
import { type Lang, t, getBrowserLang } from '../i18n'

export default function Landing() {
  const [lang, setLang] = useState<Lang>(getBrowserLang())
  const L = (zh: string, en: string) => t(zh, en, lang)

  return (
    <div className="min-h-screen bg-gray-950">
      {/* Nav */}
      <nav className="fixed top-0 w-full z-50 bg-gray-950/80 backdrop-blur-md border-b border-gray-800">
        <div className="max-w-7xl mx-auto px-6 h-16 flex items-center justify-between">
          <div className="flex items-center gap-1.5">
            <div className="w-8 h-8 rounded-lg bg-gradient-to-br from-red-500 to-amber-500 flex items-center justify-center text-white font-bold text-xs">Q8</div>
            <div className="flex items-baseline gap-1">
              <span className="text-lg font-bold text-white tracking-tight">Q8bot</span>
              <span className="text-[13px] text-gray-400 font-medium">{L('AI量化智能体', 'Quantitative AI Agent')}</span>
            </div>
          </div>
          <div className="flex items-center gap-5">
            <a href="#why" className="text-sm text-gray-400 hover:text-white transition hidden md:inline">{L('为什么选我们', 'Why Us')}</a>
            <a href="#how" className="text-sm text-gray-400 hover:text-white transition hidden md:inline">{L('运作方式', 'How It Works')}</a>
            <a href="#safety" className="text-sm text-gray-400 hover:text-white transition hidden md:inline">{L('安全保障', 'Safety')}</a>
            <a href="#faq" className="text-sm text-gray-400 hover:text-white transition hidden md:inline">FAQ</a>
            <button onClick={() => setLang(lang === 'zh' ? 'en' : 'zh')} className="flex items-center gap-1 text-sm text-gray-400 hover:text-white border border-gray-700 rounded-lg px-2.5 py-1 transition">
              <Globe2 className="w-3.5 h-3.5" /> {lang === 'zh' ? 'EN' : '中文'}
            </button>
            <Link to="/login" className="text-sm bg-red-600 hover:bg-red-500 text-white px-4 py-2 rounded-lg transition">
              {L('投资人登录', 'Investor Login')}
            </Link>
          </div>
        </div>
      </nav>

      {/* Hero */}
      <section className="pt-32 pb-20 px-6">
        <div className="max-w-4xl mx-auto text-center">
          <div className="inline-flex items-center gap-2 bg-red-500/10 border border-red-500/20 rounded-full px-4 py-1.5 mb-8">
            <div className="w-2 h-2 rounded-full bg-red-500 animate-pulse" />
            <span className="text-sm text-red-400">{L('系统运行中 — 实时监控 5,000+ 只 A 股', 'System Live — Monitoring 5,000+ A-Shares in Real-time')}</span>
          </div>
          <h1 className="text-4xl sm:text-6xl font-bold text-white leading-tight mb-6">
            {L('让 AI 帮你管钱', 'Let AI Manage')}<br />
            <span className="gradient-text">{L('24小时不休息', 'Your Wealth 24/7')}</span>
          </h1>
          <p className="text-lg sm:text-xl text-gray-400 max-w-2xl mx-auto mb-10 leading-relaxed">
            {L(
              '我们用人工智能 + 量化算法，覆盖主板、创业板、科创板全市场 5000+ 只股票，实时筛选最佳投资机会。不靠猜，不靠运气，靠数据和算法。',
              'We combine AI with quantitative algorithms, covering Main Board, GEM (ChiNext), and STAR Market — 5,000+ stocks scanned in real-time. No guessing, no luck — just data and algorithms.'
            )}
          </p>
          <div className="flex flex-col sm:flex-row items-center justify-center gap-4">
            <a href="#how" className="inline-flex items-center gap-2 bg-red-600 hover:bg-red-500 text-white px-8 py-3.5 rounded-xl text-lg font-medium transition">
              {L('了解运作方式', 'See How It Works')} <ChevronRight className="w-5 h-5" />
            </a>
            <Link to="/login" className="inline-flex items-center gap-2 border border-gray-700 hover:border-gray-500 text-gray-300 px-8 py-3.5 rounded-xl text-lg transition">
              {L('查看我的账户', 'View My Portfolio')}
            </Link>
          </div>
        </div>
      </section>

      {/* Trust Numbers */}
      <section className="py-14 border-y border-gray-800">
        <div className="max-w-5xl mx-auto px-6 grid grid-cols-2 md:grid-cols-4 gap-8">
          {[
            { value: '5,000+', zh: '全市场股票覆盖', en: 'Full Market Coverage' },
            { value: '62%', zh: '历史胜率', en: 'Historical Win Rate' },
            { value: '2.1:1', zh: '盈亏比', en: 'Profit/Loss Ratio' },
            { value: '24/7', zh: 'AI全天候运行', en: 'AI Running Non-stop' },
          ].map((s) => (
            <div key={s.en} className="text-center">
              <div className="text-3xl font-bold text-red-400">{s.value}</div>
              <div className="text-sm text-gray-500 mt-1">{L(s.zh, s.en)}</div>
            </div>
          ))}
        </div>
      </section>

      {/* Why Us */}
      <section id="why" className="py-24 px-6">
        <div className="max-w-6xl mx-auto">
          <h2 className="text-3xl font-bold text-white text-center mb-4">{L('为什么选择我们？', 'Why Choose Us?')}</h2>
          <p className="text-gray-400 text-center mb-16 max-w-2xl mx-auto">
            {L(
              '和传统理财不一样，我们不是人在盯盘，而是让 AI 7×24 小时帮你做决策。',
              "Unlike traditional fund managers, we don't rely on humans watching screens. AI makes decisions for you around the clock."
            )}
          </p>
          <div className="grid md:grid-cols-2 lg:grid-cols-3 gap-6">
            {[
              { icon: Brain, zh: 'AI 比人更冷静', en: 'AI Never Panics', descZh: '人会在暴跌时恐慌割肉，在暴涨时贪婪追高。AI 不会。它严格按照数据和算法执行，没有情绪。', descEn: 'Humans panic-sell during crashes and greedily chase rallies. AI doesn\'t. It follows data and algorithms with zero emotion.' },
              { icon: TrendingUp, zh: '从5000+只股票中选最好的', en: 'Best Picks from 5,000+ Stocks', descZh: '每天扫描主板、创业板、科创板全市场，用四维评分模型（趋势、动量、量价、波动率）找出最有潜力的10只。人不可能看完5000只，AI可以。', descEn: 'Daily scan across Main Board, GEM, and STAR Market using a 4-factor scoring model (trend, momentum, volume, volatility) to find the top 10. No human can analyze 5,000 stocks — AI can.' },
              { icon: Shield, zh: '严格的风险控制', en: 'Strict Risk Management', descZh: '每笔交易自动设止损，单只股票不超过总资金25%，亏损超过3%自动暂停策略。永远把保住本金放在第一位。', descEn: 'Auto stop-loss on every trade, max 25% per stock, auto-pause if daily loss exceeds 3%. Capital preservation always comes first.' },
              { icon: Clock, zh: '比你起得早，比你睡得晚', en: 'Earlier Than You, Later Than You', descZh: '每天早上8点AI就开始分析全球市场和新闻，9:30准时开始执行。收盘后自动复盘。你只需要每天看一眼收益。', descEn: 'AI starts analyzing global markets at 8am, executes at 9:30 market open, auto-reviews after close. You just check your returns once a day.' },
              { icon: Eye, zh: '全透明，你随时能看到', en: 'Full Transparency', descZh: '每一笔买卖、每一个AI决策、每天的收益曲线，你都能在大屏上实时看到。没有黑箱。', descEn: 'Every trade, every AI decision, every P&L curve — visible on your dashboard in real-time. No black box.' },
              { icon: Users, zh: '多账户独立运作', en: 'Multi-Account Independence', descZh: '每个投资人的资金独立管理，互不干扰。你的钱只在你的证券账户里，我们只发出交易指令。', descEn: 'Each investor\'s capital is managed independently. Your money stays in your own brokerage account — we only send trading signals.' },
            ].map((f) => (
              <div key={f.en} className="bg-gray-900 border border-gray-800 rounded-2xl p-6 hover:border-red-500/30 transition group">
                <div className="w-12 h-12 rounded-xl bg-red-500/10 flex items-center justify-center mb-4 group-hover:bg-red-500/20 transition">
                  <f.icon className="w-6 h-6 text-red-400" />
                </div>
                <h3 className="text-lg font-semibold text-white mb-2">{L(f.zh, f.en)}</h3>
                <p className="text-sm text-gray-400 leading-relaxed">{L(f.descZh, f.descEn)}</p>
              </div>
            ))}
          </div>
        </div>
      </section>

      {/* How It Works */}
      <section id="how" className="py-24 px-6 bg-gray-900/50">
        <div className="max-w-4xl mx-auto">
          <h2 className="text-3xl font-bold text-white text-center mb-4">{L('运作方式', 'How It Works')}</h2>
          <p className="text-gray-400 text-center mb-16">{L('简单四步，让 AI 帮你打理投资', 'Four simple steps to let AI manage your investments')}</p>
          <div className="space-y-8">
            {[
              { step: '01', zh: '开设证券账户', en: 'Open a Brokerage Account', descZh: '您在正规券商开户（如中金财富），资金始终在您自己的账户里，安全有保障。', descEn: 'Open an account at a licensed broker (e.g. CICC Wealth). Your funds always stay in your own account — safe and regulated.' },
              { step: '02', zh: '授权AI交易', en: 'Authorize AI Trading', descZh: '通过券商的量化交易接口（QMT），授权我们的AI系统代为执行交易。您随时可以收回授权。', descEn: 'Through the broker\'s quantitative trading API (QMT), authorize our AI system to execute trades on your behalf. You can revoke access anytime.' },
              { step: '03', zh: 'AI自动运行', en: 'AI Runs Automatically', descZh: 'AI每天自动分析市场、筛选股票、判断买卖点、执行交易、控制风险。全程无需您操作。', descEn: 'AI automatically analyzes markets, screens stocks, identifies entry/exit points, executes trades, and manages risk. No action needed from you.' },
              { step: '04', zh: '查看收益', en: 'Check Your Returns', descZh: '登录可视化大屏，随时查看您的持仓、盈亏、AI决策记录。每笔交易都有AI解释为什么这样做。', descEn: 'Log into the visual dashboard to check your positions, P&L, and AI decision logs anytime. Every trade comes with an AI explanation.' },
            ].map((item) => (
              <div key={item.step} className="flex gap-6 items-start">
                <div className="w-14 h-14 rounded-2xl bg-red-500/10 border border-red-500/20 flex items-center justify-center flex-shrink-0">
                  <span className="text-red-400 font-bold text-lg">{item.step}</span>
                </div>
                <div>
                  <h3 className="text-lg font-semibold text-white mb-1">{L(item.zh, item.en)}</h3>
                  <p className="text-gray-400 leading-relaxed">{L(item.descZh, item.descEn)}</p>
                </div>
              </div>
            ))}
          </div>
        </div>
      </section>

      {/* Safety */}
      <section id="safety" className="py-24 px-6">
        <div className="max-w-4xl mx-auto">
          <h2 className="text-3xl font-bold text-white text-center mb-4">{L('安全保障', 'Safety & Security')}</h2>
          <p className="text-gray-400 text-center mb-16">{L('我们把安全放在赚钱前面', 'We put safety before profits')}</p>
          <div className="grid md:grid-cols-2 gap-6">
            {[
              { zh: '资金安全', en: 'Fund Safety', descZh: '您的资金始终在您自己的证券账户中。我们无法转出您的资金，只能发出买卖指令。即使我们的系统被黑客攻击，您的资金也是安全的。', descEn: 'Your funds always stay in your own brokerage account. We cannot withdraw your money — only send buy/sell orders. Even if our system is compromised, your capital is safe.' },
              { zh: '三级风控', en: '3-Level Risk Control', descZh: '策略级：单笔止损-2%。账户级：日亏损超-3%暂停。系统级：全局亏损超-5%一键熔断。三重保护，层层兜底。', descEn: 'Strategy level: -2% stop-loss per trade. Account level: pause if daily loss > -3%. System level: circuit breaker if global loss > -5%. Triple protection, layer by layer.' },
              { zh: '断网保护', en: 'Network Failure Protection', descZh: '如果我们的主控系统意外断线，每个交易节点上的AI会自动接管，切换到保守模式保护您的持仓，而不是放任不管。', descEn: 'If our master system goes offline unexpectedly, the local AI on each trading node automatically takes over in conservative mode to protect your positions — never left unattended.' },
              { zh: '全程透明', en: 'Full Audit Trail', descZh: '每一笔交易都有完整记录：谁下的单、为什么、什么时候、结果如何。您可以随时回查每一个AI决策的完整理由。', descEn: 'Every trade is fully logged: who ordered it, why, when, and the result. You can review the complete reasoning behind every AI decision at any time.' },
            ].map((item) => (
              <div key={item.en} className="bg-gray-900 border border-gray-800 rounded-2xl p-6">
                <h3 className="text-lg font-semibold text-white mb-2">{L(item.zh, item.en)}</h3>
                <p className="text-sm text-gray-400 leading-relaxed">{L(item.descZh, item.descEn)}</p>
              </div>
            ))}
          </div>
        </div>
      </section>

      {/* Performance Preview */}
      <section className="py-24 px-6 bg-gray-900/50">
        <div className="max-w-4xl mx-auto text-center">
          <h2 className="text-3xl font-bold text-white mb-4">{L('实时运行数据', 'Live Performance')}</h2>
          <p className="text-gray-400 mb-12">{L('以下数据来自我们的实际运行系统', 'Data from our live trading system')}</p>
          <div className="bg-gray-950 border border-gray-800 rounded-2xl p-8">
            <div className="grid grid-cols-2 md:grid-cols-4 gap-6 mb-8">
              <div><div className="text-2xl font-bold text-white">5,000+</div><div className="text-xs text-gray-500">{L('每日扫描股票数', 'Stocks Scanned Daily')}</div></div>
              <div><div className="text-2xl font-bold text-red-400">10</div><div className="text-xs text-gray-500">{L('精选候选股', 'Selected Candidates')}</div></div>
              <div><div className="text-2xl font-bold text-white">~150s</div><div className="text-xs text-gray-500">{L('完整扫描耗时', 'Full Scan Time')}</div></div>
              <div><div className="text-2xl font-bold text-red-400">4</div><div className="text-xs text-gray-500">{L('AI决策层级', 'AI Decision Layers')}</div></div>
            </div>
            <p className="text-xs text-gray-600">{L(
              '* 以上数据为2026年3月30日实测数据。过往表现不代表未来收益。投资有风险，入市需谨慎。',
              '* Data measured on March 30, 2026. Past performance does not guarantee future results. Investment involves risk.'
            )}</p>
          </div>
        </div>
      </section>

      {/* FAQ */}
      <section id="faq" className="py-24 px-6">
        <div className="max-w-3xl mx-auto">
          <h2 className="text-3xl font-bold text-white text-center mb-12">{L('常见问题', 'FAQ')}</h2>
          <div className="space-y-6">
            {[
              { qZh: '我的钱安全吗？', qEn: 'Is my money safe?', aZh: '绝对安全。您的资金始终在您自己名下的证券账户中（如中金财富），我们的系统只能发出买卖指令，无法转出任何资金。这和基金经理帮你操盘完全不一样——你的钱始终在你手上。', aEn: 'Absolutely. Your funds always stay in your own brokerage account (e.g. CICC Wealth). Our system can only send buy/sell orders — it cannot transfer any funds. Unlike a fund manager, your money never leaves your hands.' },
              { qZh: '我需要懂股票吗？', qEn: 'Do I need to understand stocks?', aZh: '完全不需要。AI 会自动帮你选股、买入、卖出、控制风险。你只需要每天看一下收益就好。当然，如果你感兴趣，你可以在大屏上看到 AI 每个决策的详细理由。', aEn: 'Not at all. AI automatically selects stocks, buys, sells, and manages risk for you. You just need to check your returns once a day. Of course, if you\'re curious, you can see the detailed reasoning behind every AI decision on the dashboard.' },
              { qZh: '最低需要投入多少钱？', qEn: 'What\'s the minimum investment?', aZh: '建议最低10万元人民币起投。资金量太小会影响分散投资效果，导致风控策略无法充分发挥作用。10万以上可以同时持有5-8只股票，风险更加分散。', aEn: 'We recommend a minimum of 100,000 CNY (~$14,000 USD). Too little capital limits diversification and risk management effectiveness. With 100K+, you can hold 5-8 stocks simultaneously for better risk distribution.' },
              { qZh: '收费模式是什么？', qEn: 'What are the fees?', aZh: '我们采用业绩分成模式：只有在帮你赚钱的时候才收取一定比例的管理费，亏损不收费。具体比例请联系我们的投资顾问。我们的利益和你完全一致。', aEn: 'We use a performance fee model: we only charge a percentage when we make money for you — zero fee on losses. Contact our investment advisor for specific rates. Our interests are fully aligned with yours.' },
              { qZh: 'AI 会不会亏钱？', qEn: 'Can AI lose money?', aZh: '任何投资都有风险，AI也不例外。但AI的优势是：严格执行止损纪律、没有情绪干扰、能同时分析数千只股票。历史回测显示胜率约62%、盈亏比2.1:1——意味着即使有错误，整体是盈利的。', aEn: 'All investments carry risk, and AI is no exception. But AI\'s advantage is: strict stop-loss discipline, zero emotional interference, and ability to analyze thousands of stocks simultaneously. Backtests show ~62% win rate with 2.1:1 profit/loss ratio — meaning even with mistakes, the overall result is profitable.' },
            ].map((item, i) => (
              <div key={i} className="bg-gray-900 border border-gray-800 rounded-2xl p-6">
                <h3 className="text-base font-semibold text-white mb-2">{L(item.qZh, item.qEn)}</h3>
                <p className="text-sm text-gray-400 leading-relaxed">{L(item.aZh, item.aEn)}</p>
              </div>
            ))}
          </div>
        </div>
      </section>

      {/* Team */}
      <section className="py-24 px-6">
        <div className="max-w-4xl mx-auto">
          <h2 className="text-3xl font-bold text-white text-center mb-4">{L('团队', 'Our Team')}</h2>
          <p className="text-gray-400 text-center mb-12">{L('技术驱动，交易验证', 'Tech-Driven, Market-Proven')}</p>
          <div className="grid md:grid-cols-3 gap-6">
            {[
              { role: { zh: '创始人 / 系统架构师', en: 'Founder / System Architect' }, desc: { zh: '10年+全栈工程经验，开源项目 StarClaw 作者。负责整体架构设计、AI Agent 引擎、分布式节点协调。', en: '10+ years full-stack engineering. Author of open-source StarClaw. Leads system architecture, AI Agent engine, and distributed node coordination.' } },
              { role: { zh: 'AI 量化研究员', en: 'AI Quant Researcher' }, desc: { zh: '深耕A股量化策略，主升浪四维评分模型设计者。负责策略研发、回测验证、参数优化。', en: 'Deep expertise in A-share quant strategies. Designer of the 4-factor main wave scoring model. Leads strategy R&D, backtesting, and parameter optimization.' } },
              { role: { zh: '风控 / 运营', en: 'Risk Control / Operations' }, desc: { zh: '负责三级风控体系、投资人账户管理、日常运营监控。确保每一分钱都在安全红线之内。', en: 'Manages 3-level risk control, investor account operations, and daily monitoring. Ensures every dollar stays within safety limits.' } },
            ].map((m, i) => (
              <div key={i} className="bg-gray-900 border border-gray-800 rounded-2xl p-6 text-center">
                <div className="w-16 h-16 rounded-full bg-gradient-to-br from-red-500/20 to-amber-500/20 flex items-center justify-center mx-auto mb-4">
                  <span className="text-2xl">{['👨‍💻', '📊', '🛡️'][i]}</span>
                </div>
                <h3 className="text-sm font-semibold text-red-400 mb-1">{L(m.role.zh, m.role.en)}</h3>
                <p className="text-xs text-gray-400 leading-relaxed">{L(m.desc.zh, m.desc.en)}</p>
              </div>
            ))}
          </div>
        </div>
      </section>

      {/* Contact */}
      <section className="py-24 px-6 bg-gray-900/50">
        <div className="max-w-3xl mx-auto text-center">
          <h2 className="text-3xl font-bold text-white mb-4">{L('准备好让 AI 帮你理财了吗？', 'Ready to Let AI Manage Your Wealth?')}</h2>
          <p className="text-gray-400 mb-10">{L(
            '联系我们的投资顾问，了解合作方式和开户流程。',
            'Contact our investment advisor to learn about partnership and account setup.'
          )}</p>
          <div className="grid sm:grid-cols-2 gap-6 mb-10">
            <div className="bg-gray-950 border border-gray-800 rounded-2xl p-6">
              <div className="text-2xl mb-3">💬</div>
              <h3 className="font-semibold text-white mb-1">{L('微信咨询', 'WeChat')}</h3>
              <p className="text-sm text-gray-400">{L('添加微信：yinheark', 'Add WeChat: yinheark')}</p>
            </div>
            <div className="bg-gray-950 border border-gray-800 rounded-2xl p-6">
              <div className="text-2xl mb-3">📧</div>
              <h3 className="font-semibold text-white mb-1">{L('邮箱', 'Email')}</h3>
              <p className="text-sm text-gray-400">7895056@qq.com</p>
            </div>
          </div>
          <Link to="/login" className="inline-flex items-center gap-2 bg-red-600 hover:bg-red-500 text-white px-8 py-3.5 rounded-xl text-lg font-medium transition">
            {L('投资人登录', 'Investor Login')} <ChevronRight className="w-5 h-5" />
          </Link>
        </div>
      </section>

      {/* Footer */}
      <footer className="border-t border-gray-800 py-12 px-6">
        <div className="max-w-6xl mx-auto">
          <div className="flex flex-col md:flex-row items-center justify-between gap-6">
            <div className="flex items-center gap-2">
              <div className="w-6 h-6 rounded bg-gradient-to-br from-red-500 to-amber-500 flex items-center justify-center text-white font-bold text-[9px]">Q8</div>
              <span className="text-sm text-gray-400">Q8bot × StarClaw</span>
            </div>
            <div className="flex items-center gap-6 text-xs text-gray-600">
              <span>{L('风险提示：投资有风险，入市需谨慎。过往业绩不代表未来表现。', 'Risk Disclosure: Investment involves risk. Past performance does not guarantee future results.')}</span>
            </div>
          </div>
          <div className="text-center text-xs text-gray-700 mt-6">&copy; 2026 StarClaw. All rights reserved.</div>
          <div className="text-center text-xs text-gray-700 mt-2">
            <a href="https://beian.miit.gov.cn/" target="_blank" rel="noopener noreferrer" className="hover:text-gray-500 transition">浙ICP备2024132268号-3</a>
          </div>
        </div>
      </footer>
    </div>
  )
}
