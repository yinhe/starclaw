import { useState } from 'react';
import { Link } from 'react-router-dom';
import { LogoMark } from '../components/Logo';

type Section = 'quickstart' | 'features' | 'deploy' | 'api' | 'architecture' | 'faq';

const SECTIONS: { key: Section; label: string }[] = [
  { key: 'quickstart', label: '快速开始' },
  { key: 'features', label: '核心功能' },
  { key: 'deploy', label: '部署指南' },
  { key: 'api', label: 'API 参考' },
  { key: 'architecture', label: '架构设计' },
  { key: 'faq', label: '常见问题' },
];

export function DocsPage() {
  const [section, setSection] = useState<Section>('quickstart');

  return (
    <div className="bg-gray-50 text-gray-900 antialiased min-h-screen">
      {/* Top Nav */}
      <header className="fixed top-0 inset-x-0 z-50 bg-white/80 backdrop-blur-lg border-b border-gray-200">
        <div className="max-w-screen-2xl mx-auto px-6 h-14 flex items-center justify-between">
          <div className="flex items-center gap-3">
            <Link to="/" className="flex items-center gap-2">
              <LogoMark className="w-7 h-7" />
              <span className="font-bold text-gray-900">StarClaw</span>
            </Link>
            <span className="text-gray-300">|</span>
            <span className="text-sm font-medium text-gray-500">文档</span>
          </div>
          <div className="flex items-center gap-4">
            <a href="https://github.com/yinhe/starclaw" target="_blank" rel="noreferrer" className="text-sm text-gray-500 hover:text-gray-900 transition">GitHub</a>
            <Link to="/" className="text-sm text-gray-500 hover:text-gray-900 transition">官网</Link>
            <a href="https://app.starclaw.me" className="px-3 py-1.5 text-xs font-medium bg-indigo-600 text-white rounded-lg hover:bg-indigo-700 transition">在线体验</a>
          </div>
        </div>
      </header>

      <div className="flex pt-14">
        {/* Sidebar */}
        <aside className="fixed top-14 left-0 bottom-0 w-56 bg-white border-r border-gray-200 p-4 overflow-y-auto">
          <nav className="space-y-1">
            {SECTIONS.map(s => (
              <button key={s.key} onClick={() => setSection(s.key)}
                className={`sidebar-link w-full text-left px-3 py-2 rounded-lg text-sm transition ${section === s.key ? 'active' : ''}`}>
                {s.label}
              </button>
            ))}
          </nav>
        </aside>

        {/* Content */}
        <main className="ml-56 flex-1 min-h-screen p-8 max-w-4xl">
          {section === 'quickstart' && (
            <article className="prose prose-sm max-w-none">
              <h1 className="text-3xl font-bold mb-6">快速开始</h1>
              <h2 className="text-xl font-bold mt-8 mb-4">环境要求</h2>
              <ul className="list-disc pl-6 space-y-1 text-gray-600">
                <li>Docker &ge; 20.10 + Docker Compose &ge; 2.0</li>
                <li>2 核 CPU / 4GB 内存 / 20GB 磁盘（最低）</li>
                <li>推荐 4 核 / 8GB 以获得更好体验</li>
              </ul>
              <h2 className="text-xl font-bold mt-8 mb-4">一键安装</h2>
              <pre className="bg-gray-900 text-green-400 rounded-xl p-4 text-sm overflow-x-auto">
{`curl -fsSL https://raw.githubusercontent.com/yinhe/starclaw/main/scripts/install.sh | bash`}
              </pre>
              <p className="text-gray-500 mt-2">或使用 Docker Compose 手动部署：</p>
              <pre className="bg-gray-900 text-green-400 rounded-xl p-4 text-sm overflow-x-auto">
{`git clone https://github.com/yinhe/starclaw.git
cd starclaw
cp .env.example .env
# 编辑 .env 配置数据库密码、API Key 等
docker compose up -d`}
              </pre>
              <h2 className="text-xl font-bold mt-8 mb-4">访问</h2>
              <ul className="list-disc pl-6 space-y-1 text-gray-600">
                <li><strong>前端</strong>：<code className="bg-gray-100 px-1.5 py-0.5 rounded text-sm">http://localhost</code>（或你的服务器 IP）</li>
                <li><strong>API</strong>：<code className="bg-gray-100 px-1.5 py-0.5 rounded text-sm">http://localhost/v1</code>（通过 Nginx 代理）</li>
                <li>首个注册用户自动成为管理员</li>
              </ul>
              <h2 className="text-xl font-bold mt-8 mb-4">配置 API Key</h2>
              <p className="text-gray-600">登录后进入「设置 → 模型管理」，添加你的 API Key：</p>
              <ul className="list-disc pl-6 space-y-1 text-gray-600">
                <li><strong>通义千问</strong>：在 DashScope 控制台获取</li>
                <li><strong>OpenAI</strong>：在 platform.openai.com 获取</li>
                <li><strong>DeepSeek</strong>：在 platform.deepseek.com 获取</li>
                <li><strong>fal.ai</strong>：在 fal.ai/dashboard 获取（用于图片/视频/音乐）</li>
              </ul>
            </article>
          )}

          {section === 'features' && (
            <article className="prose prose-sm max-w-none">
              <h1 className="text-3xl font-bold mb-6">核心功能</h1>
              <h2 className="text-xl font-bold mt-8 mb-4">多模型统一接入</h2>
              <p className="text-gray-600">支持 300+ 模型，包括 OpenAI GPT-4o、通义千问 Qwen3.5、DeepSeek R1、Google Gemini、Ollama 本地模型等。统一的 Provider 接口，切换模型只需修改配置。</p>
              <h2 className="text-xl font-bold mt-8 mb-4">可视化工作流</h2>
              <p className="text-gray-600">基于 React Flow 的拖拽式画布编辑器。5 种节点类型（Start / LLM / Tool / Condition / End），支持手动、Webhook、Cron 三种触发方式。</p>
              <h2 className="text-xl font-bold mt-8 mb-4">Multi-Agent 协作</h2>
              <p className="text-gray-600">全能助手（SuperAgent）自动识别意图，通过 delegate_to_agent 将任务分配给专业 Agent。支持 @mention 精确路由。</p>
              <h2 className="text-xl font-bold mt-8 mb-4">RAG 知识库</h2>
              <p className="text-gray-600">上传文档自动分块、向量化（Milvus）。支持 PDF、Word、Markdown、代码等格式。Python gRPC 微服务处理文档解析。</p>
              <h2 className="text-xl font-bold mt-8 mb-4">AI 内容创作</h2>
              <p className="text-gray-600">视频生成（wan2.6）、图片生成（flux）、音乐生成（ace-step）、MV 合成（视频+音乐+歌词字幕）。</p>
              <h2 className="text-xl font-bold mt-8 mb-4">自主编程</h2>
              <p className="text-gray-600">安全沙箱环境，支持 Python、JavaScript、TypeScript、Go、Rust、C/C++、Java、Ruby、PHP、Perl、Lua 等 13+ 语言。</p>
              <h2 className="text-xl font-bold mt-8 mb-4">MCP 兼容</h2>
              <p className="text-gray-600">完整实现 Model Context Protocol，可接入 GitHub、Notion、Slack、数据库等 MCP 服务器。</p>
            </article>
          )}

          {section === 'deploy' && (
            <article className="prose prose-sm max-w-none">
              <h1 className="text-3xl font-bold mb-6">部署指南</h1>
              <h2 className="text-xl font-bold mt-8 mb-4">Docker Compose 部署（推荐）</h2>
              <pre className="bg-gray-900 text-green-400 rounded-xl p-4 text-sm overflow-x-auto">
{`# 克隆仓库
git clone https://github.com/yinhe/starclaw.git
cd starclaw

# 配置环境变量
cp .env.example .env
nano .env

# 启动所有服务
docker compose up -d

# 查看日志
docker compose logs -f`}
              </pre>
              <h2 className="text-xl font-bold mt-8 mb-4">环境变量说明</h2>
              <div className="overflow-x-auto">
                <table className="w-full text-sm border-collapse">
                  <thead><tr className="bg-gray-50"><th className="border px-3 py-2 text-left">变量</th><th className="border px-3 py-2 text-left">说明</th><th className="border px-3 py-2 text-left">默认值</th></tr></thead>
                  <tbody className="text-gray-600">
                    <tr><td className="border px-3 py-2 font-mono text-xs">MYSQL_ROOT_PASSWORD</td><td className="border px-3 py-2">数据库密码</td><td className="border px-3 py-2">starclaw</td></tr>
                    <tr><td className="border px-3 py-2 font-mono text-xs">JWT_SECRET</td><td className="border px-3 py-2">JWT 签名密钥</td><td className="border px-3 py-2">-</td></tr>
                    <tr><td className="border px-3 py-2 font-mono text-xs">STARCLAW_SERVER_DEPLOY_MODE</td><td className="border px-3 py-2">opensource / hosted</td><td className="border px-3 py-2">opensource</td></tr>
                  </tbody>
                </table>
              </div>
              <h2 className="text-xl font-bold mt-8 mb-4">HTTPS 配置</h2>
              <p className="text-gray-600">推荐使用 Nginx 反向代理 + Let's Encrypt 证书。参考 <code>deploy/nginx.conf</code> 模板。</p>
            </article>
          )}

          {section === 'api' && (
            <article className="prose prose-sm max-w-none">
              <h1 className="text-3xl font-bold mb-6">API 参考</h1>
              <p className="text-gray-600 mb-6">Base URL: <code>http://localhost/v1</code>（通过 Nginx 代理）或直连 <code>http://localhost:8080/v1</code></p>
              <h2 className="text-xl font-bold mt-8 mb-4">认证</h2>
              <p className="text-gray-600">所有需要认证的接口使用 Bearer Token：</p>
              <pre className="bg-gray-900 text-green-400 rounded-xl p-4 text-sm">Authorization: Bearer {'<token>'}</pre>

              <h2 className="text-xl font-bold mt-8 mb-4">对话 & Agent</h2>
              <div className="overflow-x-auto">
                <table className="w-full text-sm border-collapse">
                  <thead><tr className="bg-gray-50"><th className="border px-3 py-2">方法</th><th className="border px-3 py-2">路径</th><th className="border px-3 py-2">说明</th></tr></thead>
                  <tbody className="text-gray-600">
                    {[
                      ['POST', '/chat/completions', '发送对话（SSE 流式）'],
                      ['GET', '/conversations', '会话列表'],
                      ['GET', '/conversations/:id/messages', '获取会话消息'],
                      ['DELETE', '/conversations/:id', '删除会话'],
                      ['GET', '/agents', 'Agent 列表'],
                      ['POST', '/agents', '创建 Agent'],
                      ['PUT', '/agents/:id', '更新 Agent'],
                      ['DELETE', '/agents/:id', '删除 Agent'],
                    ].map(([m, p, d]) => (
                      <tr key={p+m}><td className="border px-3 py-2 font-mono text-xs"><span className={m === 'GET' ? 'text-green-600' : m === 'POST' ? 'text-blue-600' : m === 'DELETE' ? 'text-red-500' : 'text-amber-600'}>{m}</span></td><td className="border px-3 py-2 font-mono text-xs">{p}</td><td className="border px-3 py-2">{d}</td></tr>
                    ))}
                  </tbody>
                </table>
              </div>

              <h2 className="text-xl font-bold mt-8 mb-4">工作流</h2>
              <div className="overflow-x-auto">
                <table className="w-full text-sm border-collapse">
                  <thead><tr className="bg-gray-50"><th className="border px-3 py-2">方法</th><th className="border px-3 py-2">路径</th><th className="border px-3 py-2">说明</th></tr></thead>
                  <tbody className="text-gray-600">
                    {[
                      ['GET', '/workflows', '工作流列表'],
                      ['POST', '/workflows', '创建工作流'],
                      ['PUT', '/workflows/:id', '更新工作流'],
                      ['POST', '/workflows/:id/run', '执行工作流'],
                      ['GET', '/workflows/:id/history', '执行历史'],
                    ].map(([m, p, d]) => (
                      <tr key={p+m}><td className="border px-3 py-2 font-mono text-xs"><span className={m === 'GET' ? 'text-green-600' : m === 'POST' ? 'text-blue-600' : 'text-amber-600'}>{m}</span></td><td className="border px-3 py-2 font-mono text-xs">{p}</td><td className="border px-3 py-2">{d}</td></tr>
                    ))}
                  </tbody>
                </table>
              </div>

              <h2 className="text-xl font-bold mt-8 mb-4">知识库 & RAG</h2>
              <div className="overflow-x-auto">
                <table className="w-full text-sm border-collapse">
                  <thead><tr className="bg-gray-50"><th className="border px-3 py-2">方法</th><th className="border px-3 py-2">路径</th><th className="border px-3 py-2">说明</th></tr></thead>
                  <tbody className="text-gray-600">
                    {[
                      ['GET', '/knowledge-bases', '知识库列表'],
                      ['POST', '/knowledge-bases', '创建知识库'],
                      ['POST', '/knowledge-bases/:id/documents', '上传文档'],
                      ['POST', '/knowledge-bases/:id/search', '语义搜索'],
                    ].map(([m, p, d]) => (
                      <tr key={p+m}><td className="border px-3 py-2 font-mono text-xs"><span className={m === 'GET' ? 'text-green-600' : 'text-blue-600'}>{m}</span></td><td className="border px-3 py-2 font-mono text-xs">{p}</td><td className="border px-3 py-2">{d}</td></tr>
                    ))}
                  </tbody>
                </table>
              </div>

              <h2 className="text-xl font-bold mt-8 mb-4">多媒体创作</h2>
              <div className="overflow-x-auto">
                <table className="w-full text-sm border-collapse">
                  <thead><tr className="bg-gray-50"><th className="border px-3 py-2">方法</th><th className="border px-3 py-2">路径</th><th className="border px-3 py-2">说明</th></tr></thead>
                  <tbody className="text-gray-600">
                    {[
                      ['POST', '/multimodal/upload-image', '上传图片'],
                      ['POST', '/multimodal/stt', '语音转文字（Whisper）'],
                      ['POST', '/multimodal/tts', '文字转语音'],
                    ].map(([m, p, d]) => (
                      <tr key={p}><td className="border px-3 py-2 font-mono text-xs"><span className="text-blue-600">{m}</span></td><td className="border px-3 py-2 font-mono text-xs">{p}</td><td className="border px-3 py-2">{d}</td></tr>
                    ))}
                  </tbody>
                </table>
              </div>

              <h2 className="text-xl font-bold mt-8 mb-4">系统 & 设置</h2>
              <div className="overflow-x-auto">
                <table className="w-full text-sm border-collapse">
                  <thead><tr className="bg-gray-50"><th className="border px-3 py-2">方法</th><th className="border px-3 py-2">路径</th><th className="border px-3 py-2">说明</th></tr></thead>
                  <tbody className="text-gray-600">
                    {[
                      ['POST', '/auth/register', '注册'],
                      ['POST', '/auth/login', '登录'],
                      ['GET', '/auth/oauth/providers', 'OAuth 提供商列表'],
                      ['GET', '/user/profile', '获取个人资料'],
                      ['PUT', '/user/profile', '更新个人资料'],
                      ['GET', '/model-configs', '模型配置列表'],
                      ['POST', '/model-configs', '添加模型配置'],
                      ['GET', '/version', '版本信息'],
                    ].map(([m, p, d]) => (
                      <tr key={p+m}><td className="border px-3 py-2 font-mono text-xs"><span className={m === 'GET' ? 'text-green-600' : m === 'POST' ? 'text-blue-600' : 'text-amber-600'}>{m}</span></td><td className="border px-3 py-2 font-mono text-xs">{p}</td><td className="border px-3 py-2">{d}</td></tr>
                    ))}
                  </tbody>
                </table>
              </div>

              <p className="text-gray-400 text-xs mt-6">完整 API 文档见 <a href="https://github.com/yinhe/starclaw/blob/main/docs/API_EN.md" target="_blank" rel="noreferrer" className="text-indigo-500 hover:underline">GitHub docs/API_EN.md</a></p>
            </article>
          )}

          {section === 'architecture' && (
            <article className="prose prose-sm max-w-none">
              <h1 className="text-3xl font-bold mb-6">架构设计</h1>
              <h2 className="text-xl font-bold mt-8 mb-4">三层架构</h2>
              <div className="bg-white rounded-xl border p-6 space-y-4 text-sm text-gray-600">
                <div className="flex items-start gap-3">
                  <span className="text-2xl">👑</span>
                  <div><strong className="text-gray-900">Queen（虫后）</strong> — 中央控制，全局唯一。提供虫群注册、赏金网络、社区论坛、市场等公共服务。</div>
                </div>
                <div className="flex items-start gap-3">
                  <span className="text-2xl">👁️</span>
                  <div><strong className="text-gray-900">Overlord（领主）</strong> — 企业管理节点。资源配额、监控告警、任务调度、负载均衡。</div>
                </div>
                <div className="flex items-start gap-3">
                  <span className="text-2xl">🦞</span>
                  <div><strong className="text-gray-900">Claw（小龙虾）</strong> — 最小执行单元，开源免费。独立运行 AI Agent、工作流、编程沙箱。</div>
                </div>
              </div>
              <h2 className="text-xl font-bold mt-8 mb-4">技术栈</h2>
              <ul className="list-disc pl-6 space-y-1 text-gray-600">
                <li><strong>前端</strong>：React 19 + Vite + TypeScript + TailwindCSS</li>
                <li><strong>后端</strong>：Go 1.24 + Gin + GORM</li>
                <li><strong>数据库</strong>：MySQL 8.0 + Redis</li>
                <li><strong>向量库</strong>：Milvus</li>
                <li><strong>部署</strong>：Docker Compose</li>
              </ul>
              <h2 className="text-xl font-bold mt-8 mb-4">虫族生存机制</h2>
              <ul className="list-disc pl-6 space-y-1 text-gray-600">
                <li><strong>Creep（菌毯）</strong> — 共享智能网络：知识传播、缓存加速</li>
                <li><strong>Feral（失控）</strong> — 断线生存模式：Queen 下线后各节点独立运行</li>
                <li><strong>Nydus（坑道虫）</strong> — P2P 加密隧道：同 Brood 内 Claw 直连通信</li>
                <li><strong>Evolution Chamber（进化腔）</strong> — 插件市场：独立安装/卸载能力</li>
              </ul>
            </article>
          )}

          {section === 'faq' && (
            <article className="prose prose-sm max-w-none">
              <h1 className="text-3xl font-bold mb-6">常见问题</h1>
              {[
                ['StarClaw 和 Dify 有什么区别？', 'StarClaw 除了 Dify 的核心功能外，还原生支持多媒体创作（视频/音乐/MV）、自主编程（13+ 语言沙箱）、分布式虫群架构。且完全 MIT 开源。'],
                ['需要自己准备 API Key 吗？', '两种模式：BYOK（自带 Key，完全免费）和平台托管（按量付费）。自部署时默认使用 BYOK 模式。'],
                ['支持哪些大模型？', '支持 300+ 模型：OpenAI（GPT-4o）、通义千问（Qwen3.5）、DeepSeek（R1）、Google Gemini、Ollama 本地模型等。'],
                ['如何接入 MCP 工具？', '在 Claw 控制台「设置 → MCP」中添加 MCP 服务器地址即可，支持 stdio 和 HTTP 两种传输方式。'],
                ['数据安全吗？', '自部署版本数据完全存储在你自己的服务器上。BYOK 模式下平台不接触你的 API Key 和对话内容。'],
                ['如何升级？', 'Claw 内置版本检查，有新版本时会提示。也可手动 git pull && docker compose up -d --build。'],
              ].map(([q, a]) => (
                <div key={q} className="mb-6">
                  <h3 className="text-base font-bold text-gray-900 mb-2">{q}</h3>
                  <p className="text-gray-600">{a}</p>
                </div>
              ))}
            </article>
          )}
        </main>
      </div>
    </div>
  );
}
