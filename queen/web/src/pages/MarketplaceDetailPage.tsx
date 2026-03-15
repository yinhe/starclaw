import { useState, useEffect } from 'react';
import { Link, useParams } from 'react-router-dom';
import { LogoMark } from '../components/Logo';
import {
  ArrowLeft, Download, X, CheckCircle2, AlertCircle, Bot,
  Music, Video, Code2, BookOpen, Briefcase, Clapperboard,
  BarChart3, PenTool, Server, Palette, Search,
  Wrench, Zap, GitBranch, type LucideIcon,
} from 'lucide-react';
import { marketplaceAPI, type MarketplaceItem } from '../lib/api';

const ICON_MAP: Record<string, LucideIcon> = {
  Music, Video, Code2, Search, BookOpen, Briefcase, Clapperboard,
  BarChart3, PenTool, Server, Palette, Bot,
};

function AgentIcon({ name, className }: { name?: string; className?: string }) {
  const Icon = name ? ICON_MAP[name] : null;
  if (Icon) return <Icon className={className} />;
  return <Bot className={className} />;
}

const TOOL_LABELS: Record<string, { label: string; desc: string; color: string }> = {
  video_generation: { label: '视频生成', desc: '生成AI视频片段，支持多种模型', color: 'bg-blue-50 text-blue-700 border-blue-200' },
  dubbing: { label: '配音字幕', desc: '为视频添加AI配音和字幕', color: 'bg-purple-50 text-purple-700 border-purple-200' },
  mv_production: { label: 'MV合成', desc: '将视频、音乐、字幕合成MV', color: 'bg-fuchsia-50 text-fuchsia-700 border-fuchsia-200' },
  comic_production: { label: '漫剧制作', desc: '生成漫画风格图片并组装成视频', color: 'bg-orange-50 text-orange-700 border-orange-200' },
  music_generation: { label: '音乐生成', desc: '作曲、生成歌曲或纯音乐', color: 'bg-pink-50 text-pink-700 border-pink-200' },
  image_generation: { label: '图片生成', desc: '生成AI图片和插画', color: 'bg-cyan-50 text-cyan-700 border-cyan-200' },
  code: { label: '代码执行', desc: '在沙盒中执行代码', color: 'bg-emerald-50 text-emerald-700 border-emerald-200' },
  code_sandbox: { label: '代码沙盒', desc: '安全的代码执行环境', color: 'bg-emerald-50 text-emerald-700 border-emerald-200' },
  web_search: { label: '网页搜索', desc: '搜索互联网获取实时信息', color: 'bg-amber-50 text-amber-700 border-amber-200' },
  browser: { label: '浏览器', desc: '浏览网页和抓取内容', color: 'bg-indigo-50 text-indigo-700 border-indigo-200' },
  http_request: { label: 'HTTP请求', desc: '调用外部API接口', color: 'bg-teal-50 text-teal-700 border-teal-200' },
  system: { label: '系统管理', desc: '执行系统级操作', color: 'bg-rose-50 text-rose-700 border-rose-200' },
};

function InstallModal({ item, onClose }: { item: MarketplaceItem; onClose: () => void }) {
  const [clawUrl, setClawUrl] = useState(() => localStorage.getItem('sc_claw_url') || '');
  const [clawToken, setClawToken] = useState(() => localStorage.getItem('sc_claw_token') || '');
  const [status, setStatus] = useState<'idle' | 'installing' | 'success' | 'error'>('idle');
  const [errorMsg, setErrorMsg] = useState('');

  const handleInstall = async () => {
    const url = clawUrl.replace(/\/+$/, '');
    if (!url || !clawToken) return;
    setStatus('installing');
    localStorage.setItem('sc_claw_url', url);
    localStorage.setItem('sc_claw_token', clawToken);
    try {
      const cfg = item.config ? JSON.parse(item.config) : {};
      const res = await fetch(`${url}/v1/templates/install-remote`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${clawToken}` },
        body: JSON.stringify({
          name: item.name, description: item.description,
          system_prompt: cfg.system_prompt || '', tools: cfg.tools || '[]',
          config: cfg.config || '{}', icon: item.icon,
          source: 'starclaw.net', source_id: item.id,
        }),
      });
      if (!res.ok) { const data = await res.json().catch(() => ({})); throw new Error(data.error || `HTTP ${res.status}`); }
      setStatus('success');
    } catch (e: any) { setErrorMsg(e.message || '安装失败'); setStatus('error'); }
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 backdrop-blur-sm" onClick={onClose}>
      <div className="bg-white rounded-2xl shadow-2xl w-full max-w-md mx-4 p-6" onClick={e => e.stopPropagation()}>
        <div className="flex items-center justify-between mb-4">
          <h3 className="font-bold text-lg">安装到我的 Claw</h3>
          <button onClick={onClose} className="p-1 rounded-lg hover:bg-gray-100"><X className="w-4 h-4" /></button>
        </div>
        {status === 'success' ? (
          <div className="text-center py-6">
            <CheckCircle2 className="w-12 h-12 text-green-500 mx-auto mb-3" />
            <p className="font-medium text-green-700">安装成功</p>
            <p className="text-sm text-gray-500 mt-1">「{item.name}」已添加到你的 Claw Agent 列表</p>
            <button onClick={onClose} className="mt-4 px-6 py-2 bg-indigo-600 text-white rounded-lg text-sm hover:bg-indigo-700">完成</button>
          </div>
        ) : (
          <>
            <div className="space-y-3">
              <div>
                <label className="block text-xs font-medium text-gray-600 mb-1">Claw 地址</label>
                <input value={clawUrl} onChange={e => setClawUrl(e.target.value)} placeholder="https://your-claw.example.com"
                  className="w-full px-3 py-2.5 border rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-indigo-500" />
              </div>
              <div>
                <label className="block text-xs font-medium text-gray-600 mb-1">访问令牌 (Token)</label>
                <input value={clawToken} onChange={e => setClawToken(e.target.value)} placeholder="在 Claw 设置页获取" type="password"
                  className="w-full px-3 py-2.5 border rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-indigo-500" />
              </div>
            </div>
            {status === 'error' && (
              <div className="mt-3 flex items-center gap-2 p-2.5 bg-red-50 text-red-700 rounded-lg text-xs">
                <AlertCircle className="w-4 h-4 shrink-0" /><span>{errorMsg}</span>
              </div>
            )}
            <div className="flex gap-2 mt-4">
              <button onClick={onClose} className="flex-1 px-4 py-2.5 border rounded-lg text-sm text-gray-600 hover:bg-gray-50">取消</button>
              <button onClick={handleInstall} disabled={!clawUrl || !clawToken || status === 'installing'}
                className="flex-1 px-4 py-2.5 bg-indigo-600 text-white rounded-lg text-sm font-medium hover:bg-indigo-700 disabled:opacity-50 flex items-center justify-center gap-2">
                <Download className="w-4 h-4" />{status === 'installing' ? '安装中...' : '安装'}
              </button>
            </div>
          </>
        )}
      </div>
    </div>
  );
}

export function MarketplaceDetailPage() {
  const { id } = useParams<{ id: string }>();
  const [item, setItem] = useState<MarketplaceItem | null>(null);
  const [loading, setLoading] = useState(true);
  const [showInstall, setShowInstall] = useState(false);

  useEffect(() => {
    if (!id) return;
    setLoading(true);
    marketplaceAPI.get(id).then(res => setItem(res.item)).catch(() => setItem(null)).finally(() => setLoading(false));
  }, [id]);

  const parseTags = (tags: string): string[] => {
    if (!tags) return [];
    try { return JSON.parse(tags); } catch { return tags.split(',').map(t => t.trim()).filter(Boolean); }
  };

  const parseConfig = (config: string) => {
    try { return JSON.parse(config); } catch { return {}; }
  };

  const parseTools = (toolsStr: string): string[] => {
    try { return JSON.parse(toolsStr); } catch { return []; }
  };

  if (loading) {
    return (
      <div className="bg-gray-50 min-h-screen">
        <header className="fixed top-0 inset-x-0 z-50 bg-white/80 backdrop-blur-lg border-b border-gray-200">
          <div className="max-w-7xl mx-auto px-6 h-14 flex items-center">
            <Link to="/" className="flex items-center gap-2"><LogoMark className="w-7 h-7" /><span className="font-bold text-gray-900">StarClaw</span></Link>
          </div>
        </header>
        <div className="pt-14 flex items-center justify-center min-h-[60vh]">
          <div className="animate-pulse text-gray-400">加载中...</div>
        </div>
      </div>
    );
  }

  if (!item) {
    return (
      <div className="bg-gray-50 min-h-screen">
        <header className="fixed top-0 inset-x-0 z-50 bg-white/80 backdrop-blur-lg border-b border-gray-200">
          <div className="max-w-7xl mx-auto px-6 h-14 flex items-center">
            <Link to="/" className="flex items-center gap-2"><LogoMark className="w-7 h-7" /><span className="font-bold text-gray-900">StarClaw</span></Link>
          </div>
        </header>
        <div className="pt-14 flex flex-col items-center justify-center min-h-[60vh] text-gray-400">
          <p className="text-lg mb-4">未找到该智能体</p>
          <Link to="/marketplace/agents" className="text-indigo-600 hover:underline text-sm">← 返回市场</Link>
        </div>
      </div>
    );
  }

  const cfg = parseConfig(item.config);
  const tools = parseTools(cfg.tools || '[]');
  const tags = parseTags(item.tags);

  return (
    <div className="bg-gray-50 text-gray-900 antialiased min-h-screen">
      {/* Nav */}
      <header className="fixed top-0 inset-x-0 z-50 bg-white/80 backdrop-blur-lg border-b border-gray-200">
        <div className="max-w-7xl mx-auto px-6 h-14 flex items-center justify-between">
          <div className="flex items-center gap-3">
            <Link to="/" className="flex items-center gap-2"><LogoMark className="w-7 h-7" /><span className="font-bold text-gray-900">StarClaw</span></Link>
            <span className="text-gray-300">/</span>
            <Link to="/marketplace/agents" className="text-sm text-gray-500 hover:text-gray-900 transition">Agent 市场</Link>
            <span className="text-gray-300">/</span>
            <span className="text-sm font-medium text-indigo-600 truncate max-w-[200px]">{item.name}</span>
          </div>
          <a href="https://app.starclaw.me" className="px-3 py-1.5 text-xs font-medium bg-indigo-600 text-white rounded-lg hover:bg-indigo-700 transition">在线体验</a>
        </div>
      </header>

      <main className="pt-14">
        <div className="max-w-4xl mx-auto px-6 py-10">
          {/* Back */}
          <Link to="/marketplace/agents" className="inline-flex items-center gap-1.5 text-sm text-gray-500 hover:text-indigo-600 mb-8 transition">
            <ArrowLeft className="w-4 h-4" />返回市场
          </Link>

          {/* Hero section */}
          <div className="bg-white rounded-2xl border border-gray-100 p-8">
            <div className="flex items-start gap-5">
              <div className="w-16 h-16 rounded-2xl bg-gradient-to-br from-indigo-100 to-purple-100 flex items-center justify-center shrink-0">
                <AgentIcon name={item.icon} className="w-8 h-8 text-indigo-600" />
              </div>
              <div className="flex-1 min-w-0">
                <h1 className="text-2xl font-bold mb-1">{item.name}</h1>
                <p className="text-gray-500 text-sm mb-3">by {item.author?.nickname || 'StarClaw 官方'} · v{item.version || '1.0.0'}</p>
                <p className="text-gray-600 leading-relaxed">{item.description}</p>
                {tags.length > 0 && (
                  <div className="flex gap-1.5 mt-4 flex-wrap">
                    {tags.map(tag => (
                      <span key={tag} className="px-2.5 py-1 bg-gray-50 text-gray-500 text-xs rounded-full border border-gray-100">{tag}</span>
                    ))}
                  </div>
                )}
              </div>
              <button onClick={() => setShowInstall(true)}
                className="shrink-0 flex items-center gap-2 px-5 py-2.5 bg-indigo-600 text-white rounded-xl text-sm font-medium hover:bg-indigo-700 transition">
                <Download className="w-4 h-4" />安装到 Claw
              </button>
            </div>

            {/* Stats */}
            <div className="flex gap-6 mt-6 pt-6 border-t border-gray-100">
              <div className="text-center">
                <p className="text-lg font-bold text-gray-900">{item.downloads}</p>
                <p className="text-xs text-gray-400">安装次数</p>
              </div>
              <div className="text-center">
                <p className="text-lg font-bold text-gray-900">{item.rating > 0 ? item.rating.toFixed(1) : '-'}</p>
                <p className="text-xs text-gray-400">评分</p>
              </div>
              <div className="text-center">
                <p className="text-lg font-bold text-gray-900">{item.review_status === 'approved' ? '✓' : '—'}</p>
                <p className="text-xs text-gray-400">官方认证</p>
              </div>
            </div>
          </div>

          {/* Capabilities grid */}
          <div className="grid md:grid-cols-2 gap-6 mt-8">
            {/* Skills (tools) — 被动能力 */}
            <div className="bg-white rounded-2xl border border-gray-100 p-6">
              <div className="flex items-center gap-2 mb-4">
                <Wrench className="w-4 h-4 text-indigo-600" />
                <h2 className="font-bold text-sm">技能（被动）</h2>
                <span className="text-xs text-gray-400">Agent 可调用的工具</span>
              </div>
              {tools.length === 0 ? (
                <p className="text-sm text-gray-400 py-4 text-center">暂无内置技能</p>
              ) : (
                <div className="space-y-2">
                  {tools.map(tool => {
                    const info = TOOL_LABELS[tool] || { label: tool, desc: '', color: 'bg-gray-50 text-gray-600 border-gray-200' };
                    return (
                      <div key={tool} className={`flex items-center gap-3 px-3 py-2.5 rounded-xl border ${info.color}`}>
                        <Wrench className="w-3.5 h-3.5 shrink-0" />
                        <div className="min-w-0">
                          <p className="text-xs font-medium">{info.label}</p>
                          {info.desc && <p className="text-[10px] opacity-70">{info.desc}</p>}
                        </div>
                      </div>
                    );
                  })}
                </div>
              )}
            </div>

            {/* Workflow & Instinct — placeholder */}
            <div className="space-y-6">
              <div className="bg-white rounded-2xl border border-gray-100 p-6">
                <div className="flex items-center gap-2 mb-4">
                  <GitBranch className="w-4 h-4 text-green-600" />
                  <h2 className="font-bold text-sm">工作流</h2>
                  <span className="text-xs text-gray-400">预设执行流程</span>
                </div>
                <p className="text-sm text-gray-400 py-4 text-center">暂无预设工作流</p>
              </div>

              <div className="bg-white rounded-2xl border border-gray-100 p-6">
                <div className="flex items-center gap-2 mb-4">
                  <Zap className="w-4 h-4 text-amber-500" />
                  <h2 className="font-bold text-sm">本能（主动）</h2>
                  <span className="text-xs text-gray-400">Agent 自发行为</span>
                </div>
                <p className="text-sm text-gray-400 py-4 text-center">暂无主动行为配置</p>
              </div>
            </div>
          </div>

          {/* System Prompt preview */}
          {cfg.system_prompt && (
            <div className="bg-white rounded-2xl border border-gray-100 p-6 mt-8">
              <h2 className="font-bold text-sm mb-3">系统提示词</h2>
              <div className="bg-gray-50 rounded-xl p-4 text-sm text-gray-600 whitespace-pre-wrap leading-relaxed max-h-[400px] overflow-y-auto font-mono text-xs">
                {cfg.system_prompt}
              </div>
            </div>
          )}
        </div>
      </main>

      <footer className="border-t py-8 text-center text-sm text-gray-400">
        <p>&copy; 2026 StarClaw. MIT License.</p>
      </footer>

      {showInstall && <InstallModal item={item} onClose={() => setShowInstall(false)} />}
    </div>
  );
}
