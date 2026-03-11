import { useState } from 'react';
import { Link, useParams } from 'react-router-dom';
import { LogoMark } from '../components/Logo';
import { Search } from 'lucide-react';

type MarketType = 'agents' | 'skills' | 'mcp' | 'workflows';

const TYPE_META: Record<MarketType, { title: string; desc: string; categories: string[] }> = {
  agents: {
    title: 'Agent 市场',
    desc: '发现、安装、发布 AI Agent 模板 — 让你的 Claw 即刻拥有专业能力',
    categories: ['全部', '推荐', '客服', '写作', '编程', '数据分析', '自动化', '多媒体'],
  },
  skills: {
    title: '技能市场',
    desc: '浏览和安装社区开发的技能插件 — JSON 声明即可上架',
    categories: ['全部', '推荐', '数据', '通信', '文件', '搜索', '工具', 'API'],
  },
  mcp: {
    title: 'MCP 工具市场',
    desc: '发现并接入 MCP 协议工具服务器 — 填入地址即可让 Agent 调用',
    categories: ['全部', '推荐', '开发', '效率', '数据库', '通信', '云服务'],
  },
  workflows: {
    title: '工作流模板',
    desc: '一键克隆高质量工作流模板 — 拖拽修改即可投入使用',
    categories: ['全部', '推荐', '内容生产', '数据处理', '自动化', '分析', '客服'],
  },
};

const SAMPLE_ITEMS: Record<MarketType, { emoji: string; name: string; author: string; desc: string; badge: string; badgeColor: string; downloads: string; rating: string; tools: string }[]> = {
  agents: [
    { emoji: '🤖', name: '全能助手', author: 'StarClaw 官方', desc: '路由中枢 — 自动识别用户意图，将任务分发给最适合的专业 Agent。支持 @mention 精确路由。', badge: '内置', badgeColor: 'bg-green-50 text-green-600', downloads: '预装', rating: '4.9', tools: '全部工具' },
    { emoji: '🎬', name: '多媒体创作者', author: 'StarClaw 官方', desc: '视频、图片、音乐、MV 一站式创作。支持多场景视频合并、TTS 配音、歌词字幕。', badge: '内置', badgeColor: 'bg-green-50 text-green-600', downloads: '预装', rating: '4.8', tools: 'fal.ai' },
    { emoji: '💻', name: '代码执行器', author: 'StarClaw 官方', desc: '安全沙箱代码执行：Python、JavaScript、Go、Bash 等 13+ 语言。自主编程、自我进化。', badge: '内置', badgeColor: 'bg-green-50 text-green-600', downloads: '预装', rating: '4.9', tools: 'sandbox' },
    { emoji: '🌐', name: '浏览器控制器', author: 'StarClaw 官方', desc: '无头浏览器操控：网页导航、截图、表单填写、数据抓取。支持复杂的多步骤网页交互。', badge: '内置', badgeColor: 'bg-green-50 text-green-600', downloads: '预装', rating: '4.7', tools: 'Chromium' },
    { emoji: '📊', name: '数据分析师', author: '社区', desc: '上传 CSV/Excel，自动生成统计分析、可视化图表。支持 pandas、matplotlib 数据处理管道。', badge: '社区', badgeColor: 'bg-blue-50 text-blue-600', downloads: '1.2k', rating: '4.6', tools: 'code + RAG' },
    { emoji: '📝', name: '技术文档写手', author: '社区', desc: '根据代码仓库自动生成 API 文档、README、CHANGELOG。支持多语言翻译。', badge: '社区', badgeColor: 'bg-blue-50 text-blue-600', downloads: '890', rating: '4.5', tools: 'code + web' },
    { emoji: '🛠️', name: 'DevOps 助手', author: '社区', desc: '服务器运维自动化：日志分析、故障排查、Docker 容器管理、CI/CD 流水线编排。', badge: '社区', badgeColor: 'bg-blue-50 text-blue-600', downloads: '650', rating: '4.4', tools: 'shell + MCP' },
    { emoji: '🎨', name: 'UI 设计师', author: '社区', desc: '根据需求描述生成 UI 设计稿、前端代码。支持 Tailwind、React 组件输出。', badge: '社区', badgeColor: 'bg-blue-50 text-blue-600', downloads: '520', rating: '4.3', tools: 'code + image' },
    { emoji: '💬', name: '智能客服', author: '社区', desc: '绑定知识库后秒变专业客服。自动回答常见问题，复杂问题转人工。支持多轮对话。', badge: '社区', badgeColor: 'bg-blue-50 text-blue-600', downloads: '2.1k', rating: '4.7', tools: 'RAG' },
  ],
  skills: [
    { emoji: '🌤️', name: '天气查询', author: 'StarClaw 官方', desc: '查询全球城市实时天气、未来 7 天预报。支持中文城市名。', badge: '内置', badgeColor: 'bg-green-50 text-green-600', downloads: '预装', rating: '4.8', tools: 'HTTP API' },
    { emoji: '📧', name: '邮件发送', author: '社区', desc: 'SMTP 邮件发送插件，支持 HTML 模板、附件。', badge: '社区', badgeColor: 'bg-blue-50 text-blue-600', downloads: '1.5k', rating: '4.6', tools: 'SMTP' },
    { emoji: '📈', name: '股票行情', author: '社区', desc: '实时查询 A 股/港股/美股行情、K 线数据。', badge: '社区', badgeColor: 'bg-blue-50 text-blue-600', downloads: '980', rating: '4.5', tools: 'HTTP API' },
    { emoji: '🔗', name: '网页抓取', author: '社区', desc: 'URL 内容提取，支持 Markdown 输出。', badge: '社区', badgeColor: 'bg-blue-50 text-blue-600', downloads: '2.3k', rating: '4.7', tools: 'HTTP' },
    { emoji: '🗄️', name: 'SQL 执行', author: '社区', desc: '连接 MySQL/PostgreSQL 执行 SQL 查询。', badge: '社区', badgeColor: 'bg-blue-50 text-blue-600', downloads: '760', rating: '4.4', tools: 'Database' },
    { emoji: '📊', name: '图表生成', author: '社区', desc: '根据数据自动生成 ECharts 图表。', badge: '社区', badgeColor: 'bg-blue-50 text-blue-600', downloads: '1.1k', rating: '4.5', tools: 'JavaScript' },
  ],
  mcp: [
    { emoji: '🐙', name: 'GitHub MCP', author: '官方', desc: '仓库管理、Issue、PR、代码搜索。', badge: 'MCP', badgeColor: 'bg-purple-50 text-purple-600', downloads: '3.2k', rating: '4.9', tools: 'stdio' },
    { emoji: '📝', name: 'Notion MCP', author: '社区', desc: '页面 CRUD、数据库查询、搜索。', badge: 'MCP', badgeColor: 'bg-purple-50 text-purple-600', downloads: '1.8k', rating: '4.7', tools: 'stdio' },
    { emoji: '💬', name: 'Slack MCP', author: '社区', desc: '发送消息、管理频道、搜索历史。', badge: 'MCP', badgeColor: 'bg-purple-50 text-purple-600', downloads: '950', rating: '4.5', tools: 'stdio' },
    { emoji: '🐘', name: 'PostgreSQL MCP', author: '社区', desc: '数据库查询、表结构浏览。', badge: 'MCP', badgeColor: 'bg-purple-50 text-purple-600', downloads: '1.4k', rating: '4.6', tools: 'stdio' },
    { emoji: '📂', name: 'Filesystem MCP', author: '官方', desc: '安全的文件系统读写操作。', badge: 'MCP', badgeColor: 'bg-purple-50 text-purple-600', downloads: '2.1k', rating: '4.8', tools: 'stdio' },
    { emoji: '🔍', name: 'Brave Search MCP', author: '社区', desc: 'Brave 搜索引擎接入。', badge: 'MCP', badgeColor: 'bg-purple-50 text-purple-600', downloads: '1.6k', rating: '4.6', tools: 'stdio' },
  ],
  workflows: [
    { emoji: '📰', name: '每日新闻摘要', author: '社区', desc: '自动采集新闻 → LLM 摘要 → 邮件推送。', badge: '模板', badgeColor: 'bg-cyan-50 text-cyan-600', downloads: '1.8k', rating: '4.7', tools: '4 节点' },
    { emoji: '🎥', name: '批量视频生成', author: '官方', desc: '输入主题 → 生成脚本 → 批量生成视频 → 合并。', badge: '模板', badgeColor: 'bg-cyan-50 text-cyan-600', downloads: '2.5k', rating: '4.8', tools: '6 节点' },
    { emoji: '📊', name: '数据报告流水线', author: '社区', desc: '数据采集 → 清洗 → 分析 → 生成 PDF 报告。', badge: '模板', badgeColor: 'bg-cyan-50 text-cyan-600', downloads: '1.2k', rating: '4.6', tools: '5 节点' },
    { emoji: '🤖', name: '客服自动化', author: '社区', desc: '接收消息 → 意图识别 → RAG 回答 → 转人工。', badge: '模板', badgeColor: 'bg-cyan-50 text-cyan-600', downloads: '980', rating: '4.5', tools: '4 节点' },
    { emoji: '📝', name: '博客自动发布', author: '社区', desc: '选题 → 写作 → 配图 → 发布到 WordPress。', badge: '模板', badgeColor: 'bg-cyan-50 text-cyan-600', downloads: '760', rating: '4.4', tools: '5 节点' },
    { emoji: '🔔', name: '监控告警', author: '社区', desc: 'Webhook 触发 → 检查阈值 → 发送告警。', badge: '模板', badgeColor: 'bg-cyan-50 text-cyan-600', downloads: '650', rating: '4.3', tools: '3 节点' },
  ],
};

const TABS: { key: MarketType; label: string }[] = [
  { key: 'agents', label: 'Agent 市场' },
  { key: 'skills', label: '技能市场' },
  { key: 'mcp', label: 'MCP 工具' },
  { key: 'workflows', label: '工作流' },
];

export function MarketplacePage() {
  const { type: paramType } = useParams<{ type: string }>();
  const currentType = (paramType || 'agents') as MarketType;
  const meta = TYPE_META[currentType] || TYPE_META.agents;
  const items = SAMPLE_ITEMS[currentType] || SAMPLE_ITEMS.agents;
  const [search, setSearch] = useState('');
  const [category, setCategory] = useState('全部');

  const filtered = items.filter(i => {
    if (search && !i.name.includes(search) && !i.desc.includes(search)) return false;
    if (category === '推荐') return true;
    return true;
  });

  return (
    <div className="bg-gray-50 text-gray-900 antialiased min-h-screen">
      {/* Nav */}
      <header className="fixed top-0 inset-x-0 z-50 bg-white/80 backdrop-blur-lg border-b border-gray-200">
        <div className="max-w-7xl mx-auto px-6 h-14 flex items-center justify-between">
          <div className="flex items-center gap-3">
            <Link to="/" className="flex items-center gap-2">
              <LogoMark className="w-7 h-7" />
              <span className="font-bold text-gray-900">StarClaw</span>
            </Link>
            <span className="text-gray-300">/</span>
            <span className="text-sm font-medium text-indigo-600">{meta.title}</span>
          </div>
          <div className="flex items-center gap-4">
            {TABS.filter(t => t.key !== currentType).map(t => (
              <Link key={t.key} to={`/marketplace/${t.key}`} className="text-sm text-gray-500 hover:text-gray-900 transition">{t.label}</Link>
            ))}
            <a href="https://app.starclaw.me" className="px-3 py-1.5 text-xs font-medium bg-indigo-600 text-white rounded-lg hover:bg-indigo-700 transition">在线体验</a>
          </div>
        </div>
      </header>

      <main className="pt-14">
        {/* Hero */}
        <section className="py-16 bg-gradient-to-b from-indigo-50/50 to-transparent">
          <div className="max-w-5xl mx-auto px-6 text-center">
            <h1 className="text-4xl font-extrabold">{meta.title}</h1>
            <p className="mt-4 text-gray-500 text-lg">{meta.desc}</p>
            <div className="mt-6 max-w-md mx-auto relative">
              <input type="text" value={search} onChange={e => setSearch(e.target.value)} placeholder={`搜索${meta.title}...`}
                className="w-full px-4 py-3 pl-10 rounded-xl border border-gray-200 text-sm focus:outline-none focus:ring-2 focus:ring-indigo-500 bg-white" />
              <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-gray-400" />
            </div>
            <div className="flex flex-wrap justify-center gap-2 mt-4">
              {meta.categories.map(c => (
                <button key={c} onClick={() => setCategory(c)}
                  className={`px-3 py-1 rounded-full text-xs transition ${c === category ? 'bg-indigo-600 text-white' : 'bg-white border text-gray-500 cursor-pointer hover:border-indigo-300 hover:text-indigo-600'}`}>
                  {c}
                </button>
              ))}
            </div>
          </div>
        </section>

        {/* Grid */}
        <section className="py-8">
          <div className="max-w-6xl mx-auto px-6">
            <div className="grid md:grid-cols-2 lg:grid-cols-3 gap-5">
              {filtered.map((item) => (
                <div key={item.name} className="card-hover bg-white rounded-2xl p-6 border border-gray-100">
                  <div className="flex items-center gap-3 mb-3">
                    <div className="w-10 h-10 rounded-xl bg-gray-50 flex items-center justify-center text-lg">{item.emoji}</div>
                    <div>
                      <h3 className="font-bold text-sm">{item.name}</h3>
                      <p className="text-[11px] text-gray-400">by {item.author}</p>
                    </div>
                    <span className={`ml-auto text-[10px] px-2 py-0.5 rounded-full font-medium ${item.badgeColor}`}>{item.badge}</span>
                  </div>
                  <p className="text-sm text-gray-500 leading-relaxed">{item.desc}</p>
                  <div className="flex items-center gap-3 mt-4 text-xs text-gray-400">
                    <span>⬇ {item.downloads}</span>
                    <span>⭐ {item.rating}</span>
                    <span>🔧 {item.tools}</span>
                  </div>
                </div>
              ))}
            </div>
          </div>
        </section>

        {/* CTA */}
        <section className="py-16 text-center">
          <p className="text-gray-400 text-sm">更多内容持续上架中...</p>
          <a href="https://app.starclaw.me" className="inline-block mt-4 px-6 py-2.5 bg-indigo-600 text-white rounded-lg text-sm font-medium hover:bg-indigo-700 transition">发布你的作品</a>
        </section>
      </main>

      <footer className="border-t py-8 text-center text-sm text-gray-400">
        <p>&copy; 2026 StarClaw. MIT License.</p>
      </footer>
    </div>
  );
}
