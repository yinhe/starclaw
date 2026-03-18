import { Link } from 'react-router-dom';
import { Zap, Shield, Globe, BarChart3, ArrowRight, Code, Cpu, Sparkles, Server, Handshake, TrendingUp, Settings } from 'lucide-react';

const models = [
  { name: 'GPT-4o', provider: 'OpenAI', input: '¥0.018/千', output: '¥0.054/千', tag: '旗舰' },
  { name: 'GPT-4o-mini', provider: 'OpenAI', input: '¥0.001/千', output: '¥0.004/千', tag: '性价比' },
  { name: 'Claude 3.5 Sonnet', provider: 'Anthropic', input: '¥0.022/千', output: '¥0.108/千', tag: '旗舰' },
  { name: 'DeepSeek-V3', provider: 'DeepSeek', input: '¥0.001/千', output: '¥0.002/千', tag: '国产' },
  { name: 'Qwen-Max', provider: '通义千问', input: '¥0.002/千', output: '¥0.006/千', tag: '国产' },
  { name: 'Qwen-Turbo', provider: '通义千问', input: '¥0.0003/千', output: '¥0.0006/千', tag: '极速' },
  { name: 'Gemini 2.0 Flash', provider: 'Google', input: '¥0.0005/千', output: '¥0.002/千', tag: '多模态' },
  { name: 'Grok-2', provider: 'xAI', input: '¥0.014/千', output: '¥0.072/千', tag: '推理' },
];

const features = [
  {
    icon: <Globe className="w-6 h-6" />,
    title: '100+ 模型，一个 API',
    desc: '统一的 OpenAI 兼容接口。国内外模型无缝切换，无需管理多个 SDK 和密钥。',
  },
  {
    icon: <Shield className="w-6 h-6" />,
    title: '国内直连，海外中转',
    desc: '国产模型（Qwen/DeepSeek）直连无延迟。海外模型自动通过中转节点加速访问。',
  },
  {
    icon: <BarChart3 className="w-6 h-6" />,
    title: '按量计费，透明定价',
    desc: '按实际 Token 用量扣费，加价仅 30%。5 档充值包最高 40% 赠送。',
  },
  {
    icon: <Code className="w-6 h-6" />,
    title: 'OpenAI 兼容',
    desc: '替换 base_url 即可接入。支持 Chat Completions、Embeddings、图片、音频全系列接口。',
  },
  {
    icon: <Cpu className="w-6 h-6" />,
    title: '智能路由',
    desc: '自动选择最优线路、负载均衡、故障转移。多 Key 轮转避免限速。',
  },
  {
    icon: <Sparkles className="w-6 h-6" />,
    title: 'Claw 生态',
    desc: '与 StarClaw 开源 AI Agent 平台深度集成。未来支持 Claw 地址一键登录。',
  },
];

export default function LandingPage() {
  return (
    <div className="min-h-screen bg-gray-950 text-gray-100">
      {/* Nav */}
      <nav className="border-b border-gray-800/50 backdrop-blur-sm sticky top-0 z-50 bg-gray-950/80">
        <div className="max-w-6xl mx-auto px-6 h-16 flex items-center justify-between">
          <div className="flex items-center gap-2.5">
            <div className="w-8 h-8 bg-gradient-to-br from-amber-400 to-orange-500 rounded-lg flex items-center justify-center">
              <Zap className="w-4.5 h-4.5 text-white" />
            </div>
            <span className="text-lg font-bold tracking-tight">Star<span className="bg-gradient-to-r from-amber-400 to-orange-400 bg-clip-text text-transparent">AI</span></span>
          </div>
          <div className="flex items-center gap-3">
            <a href="#models" className="text-sm text-gray-400 hover:text-white px-3 py-2 transition-colors">模型</a>
            <a href="#features" className="text-sm text-gray-400 hover:text-white px-3 py-2 transition-colors">特性</a>
            <a href="#pricing" className="text-sm text-gray-400 hover:text-white px-3 py-2 transition-colors">定价</a>
            <a href="#partner" className="text-sm text-gray-400 hover:text-white px-3 py-2 transition-colors">合作</a>
            <Link to="/download" className="text-sm text-gray-400 hover:text-white px-3 py-2 transition-colors">下载</Link>
            <Link to="/login" className="text-sm text-gray-300 hover:text-white px-3 py-2 transition-colors">登录</Link>
            <Link to="/register" className="text-sm bg-amber-500 hover:bg-amber-400 text-gray-900 font-medium px-4 py-2 rounded-lg transition-colors">
              开始使用
            </Link>
          </div>
        </div>
      </nav>

      {/* Hero */}
      <section className="relative overflow-hidden">
        <div className="absolute inset-0 bg-gradient-to-b from-amber-500/5 via-transparent to-transparent" />
        <div className="absolute top-20 left-1/2 -translate-x-1/2 w-[600px] h-[600px] bg-amber-500/5 rounded-full blur-[120px]" />
        <div className="relative max-w-4xl mx-auto px-6 pt-24 pb-20 text-center">
          <div className="inline-flex items-center gap-2 bg-amber-500/10 border border-amber-500/20 text-amber-400 text-xs font-medium px-3 py-1.5 rounded-full mb-6">
            <Zap className="w-3 h-3" />
            统一 AI 算力接口
          </div>
          <h1 className="text-5xl sm:text-6xl font-bold tracking-tight mb-6">
            一个 API，
            <span className="bg-gradient-to-r from-amber-400 to-orange-400 bg-clip-text text-transparent">
              所有模型
            </span>
          </h1>
          <p className="text-lg text-gray-400 max-w-2xl mx-auto mb-10 leading-relaxed">
            OpenAI 兼容接口，接入 GPT-4o、Claude、Qwen、DeepSeek、MiniMax、fal.ai 等 100+ 模型。
            国内直连零延迟，海外自动中转。按量计费，透明定价。
          </p>
          <div className="flex items-center justify-center gap-4">
            <Link
              to="/register"
              className="inline-flex items-center gap-2 bg-amber-500 hover:bg-amber-400 text-gray-900 font-semibold px-6 py-3 rounded-xl text-sm transition-colors"
            >
              免费注册 <ArrowRight className="w-4 h-4" />
            </Link>
            <a
              href="#models"
              className="inline-flex items-center gap-2 bg-gray-800 hover:bg-gray-700 text-gray-200 font-medium px-6 py-3 rounded-xl text-sm transition-colors border border-gray-700"
            >
              浏览模型
            </a>
          </div>

          {/* Code snippet */}
          <div className="mt-16 max-w-2xl mx-auto">
            <div className="bg-gray-900 border border-gray-800 rounded-xl overflow-hidden text-left">
              <div className="flex items-center gap-2 px-4 py-3 border-b border-gray-800 bg-gray-900/50">
                <div className="w-3 h-3 rounded-full bg-red-500/60" />
                <div className="w-3 h-3 rounded-full bg-yellow-500/60" />
                <div className="w-3 h-3 rounded-full bg-green-500/60" />
                <span className="text-xs text-gray-500 ml-2">curl</span>
              </div>
              <div className="p-4 font-mono text-sm leading-relaxed overflow-x-auto">
                <span className="text-gray-500">$ </span>
                <span className="text-green-400">curl</span>
                <span className="text-gray-300"> https://api.star-ai.net/v1/chat/completions \</span>
                {'\n  '}
                <span className="text-purple-400">-H</span>
                <span className="text-amber-300"> "Authorization: Bearer sk-star-xxx"</span>
                <span className="text-gray-300"> \</span>
                {'\n  '}
                <span className="text-purple-400">-d</span>
                <span className="text-amber-300"> '{`{"model":"qwen/qwen-turbo","messages":[...]}`}'</span>
              </div>
            </div>
          </div>
        </div>
      </section>

      {/* Models */}
      <section id="models" className="max-w-6xl mx-auto px-6 py-20">
        <div className="text-center mb-12">
          <h2 className="text-3xl font-bold mb-3">支持 100+ 模型</h2>
          <p className="text-gray-400">所有模型通过统一接口访问，无需切换 SDK</p>
        </div>
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-3">
          {models.map(m => (
            <div key={m.name} className="bg-gray-900 border border-gray-800 rounded-xl p-4 hover:border-gray-700 transition-colors group">
              <div className="flex items-start justify-between mb-2">
                <div>
                  <div className="text-white font-medium text-sm">{m.name}</div>
                  <div className="text-gray-500 text-xs">{m.provider}</div>
                </div>
                <span className={`text-[10px] font-medium px-1.5 py-0.5 rounded ${
                  m.tag === '旗舰' ? 'bg-purple-500/10 text-purple-400' :
                  m.tag === '国产' ? 'bg-red-500/10 text-red-400' :
                  m.tag === '性价比' ? 'bg-green-500/10 text-green-400' :
                  m.tag === '极速' ? 'bg-blue-500/10 text-blue-400' :
                  m.tag === '多模态' ? 'bg-cyan-500/10 text-cyan-400' :
                  'bg-amber-500/10 text-amber-400'
                }`}>
                  {m.tag}
                </span>
              </div>
              <div className="flex items-center gap-3 mt-3 text-xs text-gray-400">
                <span>输入 <span className="text-gray-300">{m.input}</span></span>
                <span>输出 <span className="text-gray-300">{m.output}</span></span>
              </div>
            </div>
          ))}
        </div>
        <div className="text-center mt-8">
          <Link to="/register" className="text-amber-400 hover:text-amber-300 text-sm inline-flex items-center gap-1">
            查看全部 100+ 模型 <ArrowRight className="w-3 h-3" />
          </Link>
        </div>
      </section>

      {/* Features */}
      <section id="features" className="border-t border-gray-800/50">
        <div className="max-w-6xl mx-auto px-6 py-20">
          <div className="text-center mb-12">
            <h2 className="text-3xl font-bold mb-3">为什么选择 Star<span className="bg-gradient-to-r from-amber-400 to-orange-400 bg-clip-text text-transparent">AI</span></h2>
            <p className="text-gray-400">专为中国开发者设计的 AI 算力平台</p>
          </div>
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-6">
            {features.map(f => (
              <div key={f.title} className="bg-gray-900/50 border border-gray-800 rounded-xl p-6 hover:border-gray-700 transition-colors">
                <div className="w-10 h-10 bg-amber-500/10 rounded-lg flex items-center justify-center text-amber-400 mb-4">
                  {f.icon}
                </div>
                <h3 className="text-white font-semibold mb-2">{f.title}</h3>
                <p className="text-gray-400 text-sm leading-relaxed">{f.desc}</p>
              </div>
            ))}
          </div>
        </div>
      </section>

      {/* Pricing */}
      <section id="pricing" className="border-t border-gray-800/50">
        <div className="max-w-4xl mx-auto px-6 py-20">
          <div className="text-center mb-12">
            <h2 className="text-3xl font-bold mb-3">简单透明的定价</h2>
            <p className="text-gray-400">按量付费，用多少算多少。充值越多赠送越多。</p>
          </div>
          <div className="grid grid-cols-2 sm:grid-cols-5 gap-3">
            {[
              { amount: '¥10', bonus: '', total: '¥10' },
              { amount: '¥50', bonus: '+10%', total: '¥55' },
              { amount: '¥100', bonus: '+20%', total: '¥120' },
              { amount: '¥500', bonus: '+30%', total: '¥650' },
              { amount: '¥1000', bonus: '+40%', total: '¥1400' },
            ].map(p => (
              <div key={p.amount} className={`bg-gray-900 border rounded-xl p-4 text-center ${
                p.bonus === '+30%' ? 'border-amber-500/40 ring-1 ring-amber-500/20' : 'border-gray-800'
              }`}>
                <div className="text-xl font-bold text-white">{p.amount}</div>
                {p.bonus && <div className="text-green-400 text-xs font-medium mt-1">{p.bonus}</div>}
                <div className="text-gray-400 text-xs mt-2">到账 {p.total}</div>
              </div>
            ))}
          </div>
          <div className="text-center mt-6 text-gray-500 text-xs">
            新用户注册赠送 100 万 Token 免费额度
          </div>
        </div>
      </section>

      {/* Provider Partnership */}
      <section id="partner" className="border-t border-gray-800/50">
        <div className="max-w-5xl mx-auto px-6 py-20">
          <div className="grid lg:grid-cols-2 gap-12 items-center">
            <div>
              <div className="inline-flex items-center gap-2 bg-purple-500/10 border border-purple-500/20 text-purple-400 text-xs font-medium px-3 py-1.5 rounded-full mb-4">
                <Handshake className="w-3 h-3" />
                算力供应商合作
              </div>
              <h2 className="text-3xl font-bold mb-4">成为 Star<span className="bg-gradient-to-r from-amber-400 to-orange-400 bg-clip-text text-transparent">AI</span> 算力供应商</h2>
              <p className="text-gray-400 leading-relaxed mb-6">
                将你的 AI 模型或 GPU 算力接入 Star-AI 平台，获得海量开发者流量。
                无需自建用户系统和计费平台，专注做好算力，其余交给我们。
              </p>
              <a
                href="mailto:partner@star-ai.net"
                className="inline-flex items-center gap-2 bg-purple-500 hover:bg-purple-400 text-white font-medium px-5 py-2.5 rounded-xl text-sm transition-colors"
              >
                申请入驻 <ArrowRight className="w-4 h-4" />
              </a>
            </div>
            <div className="grid grid-cols-2 gap-4">
              <div className="bg-gray-900 border border-gray-800 rounded-xl p-5">
                <Server className="w-6 h-6 text-purple-400 mb-3" />
                <h3 className="text-white font-medium text-sm mb-1">零门槛接入</h3>
                <p className="text-gray-500 text-xs leading-relaxed">提供 OpenAI 兼容 API 即可上架，无需额外开发</p>
              </div>
              <div className="bg-gray-900 border border-gray-800 rounded-xl p-5">
                <TrendingUp className="w-6 h-6 text-green-400 mb-3" />
                <h3 className="text-white font-medium text-sm mb-1">按量分成</h3>
                <p className="text-gray-500 text-xs leading-relaxed">按实际调用量结算，T+7 自动打款，透明可查</p>
              </div>
              <div className="bg-gray-900 border border-gray-800 rounded-xl p-5">
                <Globe className="w-6 h-6 text-blue-400 mb-3" />
                <h3 className="text-white font-medium text-sm mb-1">流量分发</h3>
                <p className="text-gray-500 text-xs leading-relaxed">智能路由自动分配流量，故障转移保障 SLA</p>
              </div>
              <div className="bg-gray-900 border border-gray-800 rounded-xl p-5">
                <Settings className="w-6 h-6 text-amber-400 mb-3" />
                <h3 className="text-white font-medium text-sm mb-1">自定义定价</h3>
                <p className="text-gray-500 text-xs leading-relaxed">YAML 配置模型定价，实时生效，灵活调整</p>
              </div>
            </div>
          </div>

          {/* Partnership process */}
          <div className="mt-16 grid grid-cols-1 sm:grid-cols-4 gap-4">
            {[
              { step: '01', title: '提交申请', desc: '发送 API 文档和模型信息' },
              { step: '02', title: '技术对接', desc: '配置 Provider YAML，联调测试' },
              { step: '03', title: '审核上架', desc: '性能评测通过后上线模型列表' },
              { step: '04', title: '运营分成', desc: '用户调用产生收入，按月结算' },
            ].map(s => (
              <div key={s.step} className="relative">
                <div className="text-3xl font-black text-gray-800 mb-2">{s.step}</div>
                <h3 className="text-white font-medium text-sm mb-1">{s.title}</h3>
                <p className="text-gray-500 text-xs">{s.desc}</p>
              </div>
            ))}
          </div>
        </div>
      </section>

      {/* CTA */}
      <section className="border-t border-gray-800/50">
        <div className="max-w-4xl mx-auto px-6 py-20 text-center">
          <h2 className="text-3xl font-bold mb-4">准备好了吗？</h2>
          <p className="text-gray-400 mb-8">30 秒注册，立即获取 API Key，开始调用 100+ AI 模型</p>
          <Link
            to="/register"
            className="inline-flex items-center gap-2 bg-amber-500 hover:bg-amber-400 text-gray-900 font-semibold px-8 py-3.5 rounded-xl text-sm transition-colors"
          >
            免费开始 <ArrowRight className="w-4 h-4" />
          </Link>
        </div>
      </section>

      {/* Footer */}
      <footer className="border-t border-gray-800/50">
        <div className="max-w-6xl mx-auto px-6 py-10">
          {/* Top row */}
          <div className="flex flex-col sm:flex-row items-center justify-between gap-6 mb-8">
            <div className="flex items-center gap-2.5">
              <div className="w-7 h-7 bg-gradient-to-br from-amber-400 to-orange-500 rounded-lg flex items-center justify-center">
                <Zap className="w-3.5 h-3.5 text-white" />
              </div>
              <span className="font-bold tracking-tight">Star<span className="text-amber-400">AI</span></span>
              <span className="text-gray-600 text-xs ml-2">AI 算力平台</span>
            </div>
            <div className="flex items-center gap-6 text-sm text-gray-400">
              <a href="#features" className="hover:text-gray-200 transition-colors">关于我们</a>
              <span className="text-gray-700">|</span>
              <Link to="/terms" className="hover:text-gray-200 transition-colors">服务条款</Link>
              <span className="text-gray-700">|</span>
              <Link to="/privacy" className="hover:text-gray-200 transition-colors">隐私政策</Link>
              <span className="text-gray-700">|</span>
              <a href="mailto:service@star-ai.net" className="hover:text-gray-200 transition-colors">联系我们</a>
            </div>
          </div>
          {/* Divider */}
          <div className="border-t border-gray-800/50 pt-6 flex flex-col sm:flex-row items-center justify-between gap-4 text-xs text-gray-600">
            <div>
              © 2026 [ STARAI ] - 浙江银河天启科技有限公司 版权所有
            </div>
            <div className="flex items-center gap-4">
              <a href="https://beian.miit.gov.cn/" target="_blank" rel="noopener" className="hover:text-gray-400 transition-colors">
                浙ICP备2020032632号-5
              </a>
              <a href="https://github.com/yinhe/starclaw" target="_blank" className="hover:text-gray-400 transition-colors">GitHub</a>
              <a href="https://starclaw.me" target="_blank" className="hover:text-gray-400 transition-colors">StarClaw</a>
            </div>
          </div>
        </div>
      </footer>
    </div>
  );
}
