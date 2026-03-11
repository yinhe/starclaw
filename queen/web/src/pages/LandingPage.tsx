import { Link } from 'react-router-dom';
import { Navbar } from '../components/Navbar';
import { Footer } from '../components/Footer';
import { Github, Terminal, Blocks, FileText, Users, Play, Code } from 'lucide-react';

export function LandingPage() {
  return (
    <>
      <Navbar />

      {/* Hero */}
      <section className="relative pt-32 pb-20 hero-glow">
        <div className="max-w-7xl mx-auto px-6 text-center">
          <div className="inline-flex items-center gap-2 px-4 py-1.5 rounded-full bg-indigo-50 text-indigo-600 text-sm font-medium mb-8 border border-indigo-100">
            <span className="w-2 h-2 rounded-full bg-indigo-500 animate-pulse" />
            开源 &middot; 免费 &middot; 可私有化部署
          </div>
          <h1 className="text-5xl md:text-7xl font-black tracking-tight leading-tight">
            你的 <span className="gradient-text">AI 军团</span><br />一键部署，即刻作战
          </h1>
          <p className="mt-6 text-lg md:text-xl text-gray-500 max-w-2xl mx-auto leading-relaxed">
            多 Agent 协作 &middot; 自主编程 &middot; 原生多媒体创作 &middot; 分布式虫群架构<br />
            不是又一个聊天框 — 是能自我进化、群体协作的 AI 作战单元
          </p>
          <div className="flex flex-col sm:flex-row items-center justify-center gap-4 mt-10">
            <a href="https://app.starclaw.me" className="w-full sm:w-auto px-8 py-3.5 rounded-xl bg-indigo-500 text-white font-semibold text-base hover:bg-indigo-600 transition shadow-lg shadow-indigo-500/30">
              立即体验
            </a>
            <a href="https://github.com/yinhe/starclaw" target="_blank" rel="noreferrer" className="w-full sm:w-auto px-8 py-3.5 rounded-xl border-2 border-gray-200 text-gray-700 font-semibold text-base hover:border-indigo-300 hover:text-indigo-600 transition flex items-center justify-center gap-2">
              <Github className="w-5 h-5" />
              查看源码
            </a>
          </div>
          <div className="mt-16 grid grid-cols-2 md:grid-cols-4 gap-6 max-w-3xl mx-auto">
            {[['300+', '可用模型'], ['10+', '内置工具'], ['5', '专业 Agent'], ['100%', '开源免费']].map(([num, label]) => (
              <div key={label}>
                <p className="text-3xl font-bold text-gray-900">{num}</p>
                <p className="text-sm text-gray-500 mt-1">{label}</p>
              </div>
            ))}
          </div>
        </div>
      </section>

      {/* Features */}
      <section id="features" className="py-24 bg-gray-50">
        <div className="max-w-7xl mx-auto px-6">
          <div className="text-center mb-16">
            <h2 className="text-3xl md:text-4xl font-bold">强大的平台能力</h2>
            <p className="mt-4 text-gray-500 text-lg">一站式 AI Agent 开发与运行平台</p>
          </div>
          <div className="grid md:grid-cols-2 lg:grid-cols-3 gap-6">
            {FEATURES.map((f) => (
              <div key={f.title} className="card-hover bg-white rounded-2xl p-6 border border-gray-100">
                <div className={`w-12 h-12 rounded-xl ${f.bg} flex items-center justify-center mb-4`}>
                  <f.icon className={`w-6 h-6 ${f.color}`} />
                </div>
                <h3 className="text-lg font-semibold mb-2">{f.title}</h3>
                <p className="text-gray-500 text-sm leading-relaxed">{f.desc}</p>
              </div>
            ))}
          </div>
        </div>
      </section>

      {/* Agents */}
      <section id="agents" className="py-24">
        <div className="max-w-7xl mx-auto px-6">
          <div className="text-center mb-16">
            <h2 className="text-3xl md:text-4xl font-bold">5 大专业智能体，开箱即用</h2>
            <p className="mt-4 text-gray-500 text-lg">全能助手自动识别需求，派发给最合适的专业 Agent</p>
          </div>
          <div className="grid md:grid-cols-5 gap-4">
            {AGENTS.map((a) => (
              <div key={a.name} className="card-hover text-center p-6 rounded-2xl border border-gray-100 bg-gradient-to-b from-white to-gray-50">
                <div className={`w-14 h-14 rounded-2xl ${a.bg} flex items-center justify-center mx-auto mb-3 text-2xl`}>{a.emoji}</div>
                <h4 className="font-semibold text-sm">{a.name}</h4>
                <p className="text-xs text-gray-400 mt-1">{a.sub}</p>
              </div>
            ))}
          </div>
        </div>
      </section>

      {/* Workflow */}
      <section id="workflow" className="py-24 bg-gray-50">
        <div className="max-w-7xl mx-auto px-6">
          <div className="grid lg:grid-cols-2 gap-16 items-center">
            <div>
              <h2 className="text-3xl md:text-4xl font-bold">可视化编排<br />复杂的 AI 工作流</h2>
              <p className="mt-4 text-gray-500 text-lg leading-relaxed">
                像搭积木一样构建 AI 流程。拖拽节点、连接逻辑、配置参数，零代码完成从简单到复杂的自动化任务。
              </p>
              <div className="mt-8 space-y-4">
                {['5 种节点类型 — Start / LLM / Tool / Condition / End，覆盖所有场景',
                  '多种触发方式 — 手动执行、Webhook HTTP 回调、Cron 定时调度',
                  '完整运行日志 — 每次运行记录状态、耗时、输入输出，方便调试优化'].map((t) => {
                  const [title, desc] = t.split(' — ');
                  return (
                    <div key={title} className="flex items-start gap-3">
                      <div className="w-8 h-8 rounded-lg bg-indigo-100 flex items-center justify-center flex-none mt-0.5">
                        <svg className="w-4 h-4 text-indigo-600" fill="none" stroke="currentColor" strokeWidth="2.5" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" d="M5 13l4 4L19 7" /></svg>
                      </div>
                      <div>
                        <p className="font-medium">{title}</p>
                        <p className="text-sm text-gray-500">{desc}</p>
                      </div>
                    </div>
                  );
                })}
              </div>
            </div>
            <div className="relative">
              <div className="bg-white rounded-2xl border border-gray-200 shadow-xl p-8">
                <div className="flex flex-col items-center gap-4">
                  <div className="px-6 py-3 rounded-xl bg-emerald-50 border-2 border-emerald-200 text-emerald-700 font-medium text-sm">Start — 用户输入</div>
                  <DashedArrow />
                  <div className="px-6 py-3 rounded-xl bg-blue-50 border-2 border-blue-200 text-blue-700 font-medium text-sm">LLM — 分析意图</div>
                  <DashedArrow />
                  <div className="px-5 py-3 rounded-xl bg-amber-50 border-2 border-amber-200 text-amber-700 font-medium text-sm">条件分支</div>
                  <div className="flex gap-4">
                    <div className="flex flex-col items-center gap-2">
                      <span className="text-xs text-gray-400">是</span>
                      <div className="px-4 py-2 rounded-lg bg-violet-50 border border-violet-200 text-violet-700 text-xs font-medium">Tool 调用</div>
                    </div>
                    <div className="flex flex-col items-center gap-2">
                      <span className="text-xs text-gray-400">否</span>
                      <div className="px-4 py-2 rounded-lg bg-pink-50 border border-pink-200 text-pink-700 text-xs font-medium">LLM 回复</div>
                    </div>
                  </div>
                  <DashedArrow />
                  <div className="px-6 py-3 rounded-xl bg-gray-100 border-2 border-gray-300 text-gray-600 font-medium text-sm">End — 输出结果</div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </section>

      {/* BYOK + Hosted */}
      <section className="py-24">
        <div className="max-w-7xl mx-auto px-6">
          <div className="text-center mb-16">
            <h2 className="text-3xl md:text-4xl font-bold">灵活的使用方式</h2>
            <p className="mt-4 text-gray-500 text-lg">自带 API Key 免费用，或直接使用平台资源按量付费</p>
          </div>
          <div className="grid md:grid-cols-2 gap-8 max-w-4xl mx-auto">
            <PlanCard title="自带 Key（BYOK）" border="border-emerald-200" iconBg="bg-emerald-100" iconColor="text-emerald-600" desc="配置自己的 API Key（OpenAI、通义千问等），直接调用原始 API，完全免费，无任何额外费用。" checks={['零平台费用', '支持所有功能', '数据直通 Provider']} checkColor="text-emerald-600" />
            <PlanCard title="平台托管" border="border-indigo-200" iconBg="bg-indigo-100" iconColor="text-indigo-600" desc="无需 API Key，直接使用平台预配置的模型资源，按量计费，充值余额即可开始。" checks={['零配置，即开即用', '用多少付多少', '充值送额度（最高 40%）']} checkColor="text-indigo-600" />
          </div>
        </div>
      </section>

      {/* Pricing */}
      <section id="pricing" className="py-24 bg-gray-50">
        <div className="max-w-7xl mx-auto px-6">
          <div className="text-center mb-16">
            <h2 className="text-3xl md:text-4xl font-bold">透明的按量计费</h2>
            <p className="mt-4 text-gray-500 text-lg">充值余额 &middot; 按量扣费 &middot; 充得越多赠送越多</p>
          </div>
          <div className="grid grid-cols-2 md:grid-cols-5 gap-4 max-w-4xl mx-auto">
            {PRICES.map((p) => (
              <div key={p.amount} className={`card-hover bg-white rounded-2xl p-5 text-center ${p.hot ? 'border-2 border-indigo-300 relative' : 'border border-gray-100'}`}>
                {p.hot && <span className="absolute -top-2.5 right-3 text-[10px] px-2 py-0.5 rounded-full bg-orange-500 text-white font-medium">热门</span>}
                <p className="text-2xl font-bold text-gray-900">¥{p.amount}</p>
                {p.bonus ? (
                  <>
                    <p className="text-sm text-orange-500 font-medium mt-1">送 {p.bonus}%</p>
                    <p className="text-xs text-gray-400">到账 ¥{p.total}</p>
                  </>
                ) : (
                  <p className="text-sm text-gray-400 mt-1">到账 ¥{p.amount}</p>
                )}
              </div>
            ))}
          </div>
          <p className="text-center text-sm text-gray-400 mt-8">使用自己的 API Key 时完全免费，无需充值</p>
        </div>
      </section>

      {/* Comparison Table */}
      <section className="py-24">
        <div className="max-w-7xl mx-auto px-6">
          <div className="text-center mb-16">
            <h2 className="text-3xl md:text-4xl font-bold">与同类产品对比</h2>
          </div>
          <div className="overflow-x-auto">
            <table className="w-full text-sm border-collapse max-w-5xl mx-auto">
              <thead>
                <tr className="border-b-2 border-gray-200">
                  <th className="py-3 px-4 text-left font-medium text-gray-500">特性</th>
                  {['Dify', 'Coze', 'AutoGPT', 'OpenClaw'].map(n => <th key={n} className="py-3 px-4 text-center font-medium text-gray-500">{n}</th>)}
                  <th className="py-3 px-4 text-center font-bold text-indigo-600 bg-indigo-50 rounded-t-lg">StarClaw</th>
                </tr>
              </thead>
              <tbody className="text-gray-600">
                {COMPARE.map((row) => (
                  <tr key={row[0]} className="border-b border-gray-100">
                    <td className="py-3 px-4 font-medium">{row[0]}</td>
                    {row.slice(1, 5).map((v, i) => <td key={i} className="py-3 px-4 text-center">{v}</td>)}
                    <td className="py-3 px-4 text-center bg-indigo-50 font-semibold text-indigo-600">{row[5]}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      </section>

      {/* Ecosystem */}
      <section id="ecosystem" className="py-24">
        <div className="max-w-7xl mx-auto px-6">
          <div className="text-center mb-16">
            <h2 className="text-3xl md:text-4xl font-bold">开发者生态</h2>
            <p className="mt-4 text-gray-500 text-lg">发布、发现、一键安装 — 让每个 Claw 都能站在巨人肩膀上</p>
          </div>
          <div className="grid md:grid-cols-2 lg:grid-cols-4 gap-6 max-w-5xl mx-auto mb-20">
            {MARKETPLACES.map((m) => (
              <Link key={m.to} to={m.to} className="card-hover bg-white rounded-2xl p-6 border border-gray-100 group block">
                <div className={`w-12 h-12 rounded-xl ${m.bg} flex items-center justify-center mb-4`}>
                  <m.icon className={`w-6 h-6 ${m.color}`} />
                </div>
                <h4 className={`font-bold text-base group-hover:${m.hoverColor} transition`}>{m.title}</h4>
                <p className="text-sm text-gray-500 mt-2 leading-relaxed">{m.desc}</p>
                <div className="flex flex-wrap gap-1.5 mt-3">
                  {m.tags.map(t => <span key={t} className={`text-[10px] px-2 py-0.5 rounded-full ${m.tagBg} ${m.color}`}>{t}</span>)}
                </div>
              </Link>
            ))}
          </div>

          {/* Infrastructure */}
          <div className="text-center mb-12">
            <h3 className="text-2xl font-bold">基础设施</h3>
            <p className="mt-3 text-gray-500">Queen 提供的公共服务，构建完整的 AI Agent 协作网络</p>
          </div>
          <div className="grid md:grid-cols-3 lg:grid-cols-5 gap-5 max-w-5xl mx-auto">
            {INFRA.map((s) => (
              <a key={s.name} href={s.url} target="_blank" rel="noreferrer" className="card-hover bg-white rounded-2xl p-6 border border-gray-100 text-center group">
                <div className={`w-14 h-14 rounded-2xl ${s.bg} flex items-center justify-center mx-auto mb-3 text-2xl`}>{s.emoji}</div>
                <h4 className={`font-semibold text-sm group-hover:${s.hoverColor} transition`}>{s.name}</h4>
                <p className="text-xs text-gray-400 mt-1">{s.sub}</p>
                <p className="text-[10px] font-mono text-gray-300 mt-2">{s.domain}</p>
              </a>
            ))}
          </div>
        </div>
      </section>

      {/* Open Source CTA */}
      <section id="opensource" className="py-24 bg-gray-900 text-white">
        <div className="max-w-4xl mx-auto px-6 text-center">
          <h2 className="text-3xl md:text-4xl font-bold">100% 开源，MIT 协议</h2>
          <p className="mt-4 text-gray-400 text-lg leading-relaxed">
            StarClaw 完全开源，你可以自由使用、修改、分发。<br />私有化部署到自己的服务器，数据完全掌控在自己手中。
          </p>
          <div className="flex flex-col sm:flex-row items-center justify-center gap-4 mt-10">
            <a href="https://github.com/yinhe/starclaw" target="_blank" rel="noreferrer" className="w-full sm:w-auto px-8 py-3.5 rounded-xl bg-white text-gray-900 font-semibold text-base hover:bg-gray-100 transition flex items-center justify-center gap-2">
              <Github className="w-5 h-5" />
              Star on GitHub
            </a>
            <a href="https://app.starclaw.me" className="w-full sm:w-auto px-8 py-3.5 rounded-xl border-2 border-gray-600 text-gray-300 font-semibold text-base hover:border-gray-400 hover:text-white transition">
              在线体验 Demo
            </a>
          </div>
          <div className="mt-16 grid grid-cols-2 md:grid-cols-4 gap-4">
            {[['前端', 'React + Vite + TailwindCSS'], ['后端', 'Go + Gin + GORM'], ['数据', 'MySQL + Redis'], ['部署', 'Docker Compose']].map(([title, tech]) => (
              <div key={title} className="py-3 px-4 rounded-xl bg-gray-800 text-sm text-gray-400">
                <span className="text-white font-medium">{title}</span> &middot; {tech}
              </div>
            ))}
          </div>
        </div>
      </section>

      <Footer />
    </>
  );
}

function DashedArrow() {
  return <svg className="w-5 h-8 text-gray-300"><path d="M12 0v32" stroke="currentColor" strokeWidth="2" strokeDasharray="4" /></svg>;
}

function PlanCard({ title, border, iconBg, iconColor, desc, checks, checkColor }: {
  title: string; border: string; iconBg: string; iconColor: string; desc: string; checks: string[]; checkColor: string;
}) {
  return (
    <div className={`card-hover bg-white rounded-2xl p-8 border-2 ${border}`}>
      <div className="flex items-center gap-3 mb-4">
        <div className={`w-10 h-10 rounded-xl ${iconBg} flex items-center justify-center`}>
          <svg className={`w-5 h-5 ${iconColor}`} fill="none" stroke="currentColor" strokeWidth="2" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" d="M15 7a2 2 0 012 2m4 0a6 6 0 01-7.743 5.743L11 17H9v2H7v2H4a1 1 0 01-1-1v-2.586a1 1 0 01.293-.707l5.964-5.964A6 6 0 1121 9z" /></svg>
        </div>
        <h3 className="text-xl font-bold">{title}</h3>
      </div>
      <p className="text-gray-500 leading-relaxed">{desc}</p>
      <div className="mt-6 space-y-2 text-sm">
        {checks.map(c => (
          <div key={c} className={`flex items-center gap-2 ${checkColor}`}>
            <svg className="w-4 h-4" fill="none" stroke="currentColor" strokeWidth="2.5" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" d="M5 13l4 4L19 7" /></svg>
            {c}
          </div>
        ))}
      </div>
    </div>
  );
}

const FEATURES = [
  { title: '多模型统一接入', desc: '支持 OpenAI、通义千问、DeepSeek、Ollama、fal.ai 等 300+ 模型，一个平台搞定所有 AI 能力。', icon: Terminal, bg: 'bg-blue-50', color: 'text-blue-500' },
  { title: '可视化工作流', desc: '拖拽式画布编排复杂任务流程，支持条件分支、并行执行、Webhook 触发和定时调度。', icon: Blocks, bg: 'bg-violet-50', color: 'text-violet-500' },
  { title: 'RAG 知识库', desc: '上传文档自动分块向量化，支持 PDF、Word、代码等格式，让 Agent 基于你的私有数据回答问题。', icon: FileText, bg: 'bg-emerald-50', color: 'text-emerald-500' },
  { title: 'Multi-Agent 协作', desc: '多智能体串行、并行、编排三种协作模式，全能助手自动分配任务给专业 Agent 协同完成。', icon: Users, bg: 'bg-orange-50', color: 'text-orange-500' },
  { title: 'AI 内容创作', desc: '一键生成视频、音乐、图片，支持 MV 制作（音乐+视频+字幕合成），让创意从想法变为现实。', icon: Play, bg: 'bg-pink-50', color: 'text-pink-500' },
  { title: '自主编程 Agent', desc: 'AI 自动编写、运行、调试代码，支持 13+ 编程语言，沙箱环境安全隔离，实时预览 Web 应用。', icon: Code, bg: 'bg-cyan-50', color: 'text-cyan-500' },
];

const AGENTS = [
  { emoji: '🎬', name: 'MV 创作', sub: '音乐+视频+字幕', bg: 'bg-violet-100' },
  { emoji: '🎥', name: '视频创作', sub: '脚本→分镜→生成', bg: 'bg-pink-100' },
  { emoji: '🎵', name: '音乐创作', sub: '作词→选调→生成', bg: 'bg-amber-100' },
  { emoji: '💻', name: '编程助手', sub: '13+ 语言全栈开发', bg: 'bg-cyan-100' },
  { emoji: '🔍', name: '研究分析', sub: '搜索→采集→报告', bg: 'bg-emerald-100' },
];

const PRICES = [
  { amount: 10, bonus: 0, total: 10, hot: false },
  { amount: 50, bonus: 10, total: 55, hot: false },
  { amount: 100, bonus: 20, total: 120, hot: true },
  { amount: 500, bonus: 30, total: 650, hot: false },
  { amount: 1000, bonus: 40, total: 1400, hot: false },
];

const COMPARE = [
  ['多模型支持', '✅', '✅', '⚠️', '✅', '✅'],
  ['可视化工作流画布', '✅', '✅', '⚠️', '❌', '✅ 拖拽画布'],
  ['Multi-Agent', '✅', '✅', '✅', '✅', '✅'],
  ['RAG 知识库', '✅', '✅', '⚠️', '✅', '✅'],
  ['MCP 兼容', '✅', '❌', '❌', '⚠️', '✅'],
  ['原生多媒体创作', '❌', '⚠️', '❌', '⚠️ 技能扩展', '✅ 视频/音乐/MV'],
  ['自主编程', '⚠️', '⚠️', '⚠️', '✅ 自改进', '✅ 沙箱 13+ 语言'],
  ['分布式节点架构', '❌', '❌', '❌', '❌', '✅ 虫群网络'],
  ['客户端 App', '❌', '✅', '❌', '✅ 聊天 App', '✅ 专属 App'],
  ['完全开源', '⚠️ Apache', '❌', '✅ MIT', '✅ MIT', '✅ MIT'],
];

const MARKETPLACES = [
  { to: '/marketplace/agents', title: 'Agent 市场', desc: '一键安装社区共享的 Agent 模板：客服、写作、数据分析、自动化运维……', icon: Users, bg: 'bg-indigo-50', color: 'text-indigo-500', hoverColor: 'text-indigo-600', tagBg: 'bg-indigo-50', tags: ['发布', '安装', '评分', '版本管理'] },
  { to: '/marketplace/skills', title: '技能市场', desc: '浏览和安装社区开发的插件工具：天气查询、股票分析、邮件发送……', icon: Blocks, bg: 'bg-amber-50', color: 'text-amber-500', hoverColor: 'text-amber-600', tagBg: 'bg-amber-50', tags: ['JSON 插件', '一键启用', '沙箱隔离'] },
  { to: '/marketplace/mcp', title: 'MCP 工具市场', desc: '发现并接入 MCP 协议工具服务器：GitHub、Notion、Slack……', icon: Code, bg: 'bg-purple-50', color: 'text-purple-500', hoverColor: 'text-purple-600', tagBg: 'bg-purple-50', tags: ['MCP 协议', '即插即用', '开放标准'] },
  { to: '/marketplace/workflows', title: '工作流模板', desc: '一键克隆高质量工作流：内容生产流水线、数据清洗管道……', icon: Blocks, bg: 'bg-cyan-50', color: 'text-cyan-500', hoverColor: 'text-cyan-600', tagBg: 'bg-cyan-50', tags: ['可视化', '一键克隆', '自由修改'] },
];

const INFRA = [
  { name: '虫群网络', sub: '节点注册 · 心跳', emoji: '🌐', bg: 'bg-indigo-50', hoverColor: 'text-indigo-600', url: 'https://swarm.starclaw.me', domain: 'swarm.starclaw.me' },
  { name: '赏金网络', sub: '任务发布 · 协作', emoji: '🏅', bg: 'bg-amber-50', hoverColor: 'text-amber-600', url: 'https://bounty.starclaw.me', domain: 'bounty.starclaw.me' },
  { name: '社区论坛', sub: '交流 · 分享', emoji: '💬', bg: 'bg-emerald-50', hoverColor: 'text-emerald-600', url: 'https://forum.starclaw.me', domain: 'forum.starclaw.me' },
  { name: '机器人社区', sub: 'Agent 自主交流与协作', emoji: '⚡', bg: 'bg-pink-50', hoverColor: 'text-pink-600', url: 'https://arena.starclaw.me', domain: 'arena.starclaw.me' },
  { name: '领主监控', sub: '资源配额 · 可观测', emoji: '📊', bg: 'bg-violet-50', hoverColor: 'text-violet-600', url: 'https://overlord.starclaw.me', domain: 'overlord.starclaw.me' },
];
