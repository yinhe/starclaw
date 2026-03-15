import { useState, useEffect, useCallback } from 'react';
import { Link, useParams } from 'react-router-dom';
import { LogoMark } from '../components/Logo';
import { Search, Download, X, CheckCircle2, AlertCircle } from 'lucide-react';
import { marketplaceAPI, type MarketplaceItem } from '../lib/api';

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

const TABS: { key: MarketType; label: string }[] = [
  { key: 'agents', label: 'Agent 市场' },
  { key: 'skills', label: '技能市场' },
  { key: 'mcp', label: 'MCP 工具' },
  { key: 'workflows', label: '工作流' },
];

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
          name: item.name,
          description: item.description,
          system_prompt: cfg.system_prompt || '',
          tools: cfg.tools || '[]',
          config: cfg.config || '{}',
          icon: item.icon,
          source: 'starclaw.net',
          source_id: item.id,
        }),
      });
      if (!res.ok) {
        const data = await res.json().catch(() => ({}));
        throw new Error(data.error || `HTTP ${res.status}`);
      }
      setStatus('success');
    } catch (e: any) {
      setErrorMsg(e.message || '安装失败');
      setStatus('error');
    }
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
            <div className="flex items-center gap-3 p-3 bg-gray-50 rounded-xl mb-4">
              <div className="w-10 h-10 rounded-xl bg-indigo-50 flex items-center justify-center text-lg">{item.icon || '🤖'}</div>
              <div className="flex-1 min-w-0">
                <p className="font-medium text-sm truncate">{item.name}</p>
                <p className="text-xs text-gray-400 truncate">{item.description}</p>
              </div>
            </div>

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
                <p className="text-[11px] text-gray-400 mt-1">Claw → 设置 → 个人信息 → 复制 Token</p>
              </div>
            </div>

            {status === 'error' && (
              <div className="mt-3 flex items-center gap-2 p-2.5 bg-red-50 text-red-700 rounded-lg text-xs">
                <AlertCircle className="w-4 h-4 shrink-0" />
                <span>{errorMsg}</span>
              </div>
            )}

            <div className="flex gap-2 mt-4">
              <button onClick={onClose} className="flex-1 px-4 py-2.5 border rounded-lg text-sm text-gray-600 hover:bg-gray-50">取消</button>
              <button onClick={handleInstall} disabled={!clawUrl || !clawToken || status === 'installing'}
                className="flex-1 px-4 py-2.5 bg-indigo-600 text-white rounded-lg text-sm font-medium hover:bg-indigo-700 disabled:opacity-50 flex items-center justify-center gap-2">
                <Download className="w-4 h-4" />
                {status === 'installing' ? '安装中...' : '安装'}
              </button>
            </div>
          </>
        )}
      </div>
    </div>
  );
}

export function MarketplacePage() {
  const { type: paramType } = useParams<{ type: string }>();
  const currentType = (paramType || 'agents') as MarketType;
  const meta = TYPE_META[currentType] || TYPE_META.agents;
  const [search, setSearch] = useState('');
  const [category, setCategory] = useState('全部');
  const [items, setItems] = useState<MarketplaceItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [installItem, setInstallItem] = useState<MarketplaceItem | null>(null);

  const loadItems = useCallback(async (q?: string) => {
    setLoading(true);
    try {
      const res = await marketplaceAPI.list({ type: currentType === 'mcp' ? 'mcp' : currentType === 'workflows' ? 'workflow' : currentType === 'skills' ? 'skill' : 'agent', q: q || undefined });
      setItems(res.items || []);
    } catch {
      setItems([]);
    }
    setLoading(false);
  }, [currentType]);

  useEffect(() => { loadItems(); }, [loadItems]);

  const handleSearch = (val: string) => {
    setSearch(val);
    loadItems(val);
  };

  const parseTags = (tags: string): string[] => {
    if (!tags) return [];
    try { return JSON.parse(tags); } catch { return tags.split(',').map(t => t.trim()).filter(Boolean); }
  };

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
              <input type="text" value={search} onChange={e => handleSearch(e.target.value)} placeholder={`搜索${meta.title}...`}
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
            {loading ? (
              <div className="grid md:grid-cols-2 lg:grid-cols-3 gap-5">
                {[1,2,3,4,5,6].map(i => (
                  <div key={i} className="bg-white rounded-2xl p-6 border border-gray-100 animate-pulse">
                    <div className="flex items-center gap-3 mb-3">
                      <div className="w-10 h-10 rounded-xl bg-gray-200" />
                      <div className="flex-1"><div className="h-4 bg-gray-200 rounded w-2/3 mb-1" /><div className="h-3 bg-gray-200 rounded w-1/3" /></div>
                    </div>
                    <div className="h-3 bg-gray-200 rounded w-full mb-2" />
                    <div className="h-3 bg-gray-200 rounded w-4/5" />
                  </div>
                ))}
              </div>
            ) : items.length === 0 ? (
              <div className="text-center py-20 text-gray-400">
                <p className="text-lg">暂无已发布的{meta.title.replace('市场', '').replace('模板', '')}模板</p>
                <p className="text-sm mt-2">在 <Link to="/developer" className="text-indigo-600 hover:underline">开发者中心</Link> 发布你的作品</p>
              </div>
            ) : (
              <div className="grid md:grid-cols-2 lg:grid-cols-3 gap-5">
                {items.map((item) => (
                  <div key={item.id} className="group bg-white rounded-2xl p-6 border border-gray-100 hover:shadow-lg hover:border-indigo-100 transition-all">
                    <div className="flex items-center gap-3 mb-3">
                      <div className="w-10 h-10 rounded-xl bg-gray-50 flex items-center justify-center text-lg">{item.icon || '🤖'}</div>
                      <div className="flex-1 min-w-0">
                        <h3 className="font-bold text-sm truncate">{item.name}</h3>
                        <p className="text-[11px] text-gray-400">by {item.author?.nickname || '匿名'}</p>
                      </div>
                      <button onClick={() => setInstallItem(item)}
                        className="opacity-0 group-hover:opacity-100 flex items-center gap-1 px-3 py-1.5 text-xs bg-indigo-600 text-white rounded-lg hover:bg-indigo-700 transition-all whitespace-nowrap">
                        <Download className="w-3.5 h-3.5" />
                        安装
                      </button>
                    </div>
                    <p className="text-sm text-gray-500 leading-relaxed line-clamp-2">{item.description}</p>
                    {parseTags(item.tags).length > 0 && (
                      <div className="flex gap-1.5 mt-3 flex-wrap">
                        {parseTags(item.tags).slice(0, 4).map(tag => (
                          <span key={tag} className="px-2 py-0.5 bg-gray-50 text-gray-500 text-[11px] rounded-full">{tag}</span>
                        ))}
                      </div>
                    )}
                    <div className="flex items-center gap-3 mt-3 text-xs text-gray-400">
                      <span>⬇ {item.downloads}</span>
                      {item.rating > 0 && <span>⚡ {item.rating.toFixed(1)}</span>}
                    </div>
                  </div>
                ))}
              </div>
            )}
          </div>
        </section>

        {/* CTA */}
        <section className="py-16 text-center">
          <p className="text-gray-400 text-sm">更多内容持续上架中...</p>
          <Link to="/developer" className="inline-block mt-4 px-6 py-2.5 bg-indigo-600 text-white rounded-lg text-sm font-medium hover:bg-indigo-700 transition">发布你的作品</Link>
        </section>
      </main>

      <footer className="border-t py-8 text-center text-sm text-gray-400">
        <p>&copy; 2026 StarClaw. MIT License.</p>
      </footer>

      {installItem && <InstallModal item={installItem} onClose={() => setInstallItem(null)} />}
    </div>
  );
}
