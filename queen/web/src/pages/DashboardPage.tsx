import { useState, useEffect } from 'react';
import { Link, useNavigate, useLocation } from 'react-router-dom';
import { LogoMark } from '../components/Logo';
import { isLoggedIn, getUser, clearAuth, getUserInitial, getUserDisplayName } from '../lib/auth';
import { userAPI, nodeAPI, type NodeBinding } from '../lib/api';
import { LayoutGrid, User, Users, Blocks, GitBranch, Server, Key, Settings, LogOut, Plus, Trash2, Wifi, WifiOff, Clock } from 'lucide-react';

type Panel = 'overview' | 'profile' | 'my-agents' | 'my-skills' | 'my-workflows' | 'my-claws' | 'api-keys' | 'settings';

const SIDEBAR: { panel: Panel; label: string; icon: typeof LayoutGrid }[] = [
  { panel: 'overview', label: '概览', icon: LayoutGrid },
  { panel: 'profile', label: '个人资料', icon: User },
  { panel: 'my-agents', label: '我的 Agent', icon: Users },
  { panel: 'my-skills', label: '我的技能', icon: Blocks },
  { panel: 'my-workflows', label: '我的工作流', icon: GitBranch },
  { panel: 'my-claws', label: '我的节点', icon: Server },
  { panel: 'api-keys', label: 'API 密钥', icon: Key },
  { panel: 'settings', label: '账号设置', icon: Settings },
];

export function DashboardPage() {
  const navigate = useNavigate();
  const location = useLocation();
  const hash = location.hash.slice(1) as Panel;
  const [panel, setPanel] = useState<Panel>(hash || 'overview');

  // Auth guard
  useEffect(() => {
    if (!isLoggedIn()) navigate('/auth?redirect=/dashboard');
  }, [navigate]);

  const user = getUser();
  const initial = getUserInitial();
  const displayName = getUserDisplayName();

  // Profile form
  const [nickname, setNickname] = useState(user?.nickname || '');
  const [email, setEmail] = useState(user?.email || '');
  const [phone, setPhone] = useState(user?.phone || '');
  const [bio, setBio] = useState(user?.bio || '');
  const [profileMsg, setProfileMsg] = useState('');

  function switchPanel(p: Panel) {
    setPanel(p);
    window.history.replaceState(null, '', `#${p}`);
  }

  async function saveProfile() {
    try {
      await userAPI.updateProfile({ nickname, bio });
      // Update local
      const u = getUser();
      if (u) {
        u.nickname = nickname;
        u.bio = bio;
        localStorage.setItem('sc_user', JSON.stringify(u));
      }
      setProfileMsg('资料已保存');
      setTimeout(() => setProfileMsg(''), 2000);
    } catch (e: any) {
      setProfileMsg(e.message);
    }
  }

  function logout() {
    clearAuth();
    navigate('/auth');
  }

  const INPUT = 'w-full px-4 py-3 rounded-xl border border-gray-200 text-sm focus:outline-none focus:ring-2 focus:ring-indigo-500 transition';

  return (
    <div className="bg-gray-50 text-gray-900 antialiased min-h-screen">
      {/* Top Bar */}
      <header className="fixed top-0 inset-x-0 z-50 bg-white border-b border-gray-200 h-14">
        <div className="h-full px-6 flex items-center justify-between">
          <div className="flex items-center gap-3">
            <Link to="/" className="flex items-center gap-2">
              <LogoMark className="w-7 h-7" />
              <span className="font-bold text-gray-900">StarClaw</span>
            </Link>
            <span className="text-gray-300">|</span>
            <span className="text-sm font-medium text-gray-500">开发者后台</span>
          </div>
          <div className="flex items-center gap-4">
            <a href="https://app.starclaw.me" className="text-sm text-gray-500 hover:text-gray-900 transition">Claw 控制台</a>
            <Link to="/marketplace" className="text-sm text-gray-500 hover:text-gray-900 transition">市场</Link>
            <Link to="/docs" className="text-sm text-gray-500 hover:text-gray-900 transition">文档</Link>
            <div className="w-8 h-8 rounded-full bg-indigo-100 text-indigo-600 flex items-center justify-center text-sm font-bold">{initial}</div>
          </div>
        </div>
      </header>

      <div className="flex pt-14">
        {/* Sidebar */}
        <aside className="fixed top-14 left-0 bottom-0 w-56 bg-white border-r border-gray-200 p-4">
          <nav className="space-y-1">
            {SIDEBAR.map((s) => (
              <button key={s.panel} onClick={() => switchPanel(s.panel)}
                className={`sidebar-link w-full flex items-center gap-2.5 px-3 py-2.5 rounded-lg text-sm transition text-left ${panel === s.panel ? 'active' : ''}`}>
                <s.icon className="w-4 h-4" />
                {s.label}
              </button>
            ))}
          </nav>
          <div className="absolute bottom-4 left-4 right-4">
            <button onClick={logout} className="w-full px-3 py-2 rounded-lg text-sm text-gray-400 hover:text-red-500 hover:bg-red-50 transition text-left flex items-center gap-2">
              <LogOut className="w-4 h-4" />
              退出登录
            </button>
          </div>
        </aside>

        {/* Main */}
        <main className="ml-56 flex-1 min-h-screen p-8">

          {/* Overview */}
          {panel === 'overview' && (
            <div>
              <h1 className="text-2xl font-bold mb-6">概览</h1>
              <div className="grid grid-cols-2 lg:grid-cols-4 gap-4 mb-8">
                {[
                  { n: 0, label: '已发布 Agent', color: 'text-indigo-600' },
                  { n: 0, label: '已发布技能', color: 'text-amber-600' },
                  { n: 0, label: '已发布工作流', color: 'text-cyan-600' },
                  { n: 0, label: '在线节点', color: 'text-green-600' },
                ].map(s => (
                  <div key={s.label} className="bg-white rounded-xl border p-5">
                    <p className={`text-2xl font-bold ${s.color}`}>{s.n}</p>
                    <p className="text-sm text-gray-500 mt-1">{s.label}</p>
                  </div>
                ))}
              </div>
              <div className="grid lg:grid-cols-2 gap-6">
                <div className="bg-white rounded-xl border p-6">
                  <h3 className="font-bold text-sm mb-4">快速操作</h3>
                  <div className="space-y-2">
                    {[
                      { href: 'https://app.starclaw.me', emoji: '🦞', title: '打开 Claw 控制台', sub: '管理 Agent、对话、工作流', bg: 'bg-indigo-50' },
                      { href: '/marketplace', emoji: '🏪', title: '浏览市场', sub: '发现 Agent、技能、MCP 工具', bg: 'bg-blue-50', internal: true },
                      { href: '/docs', emoji: '📖', title: '阅读文档', sub: '快速开始、部署指南、API 参考', bg: 'bg-emerald-50', internal: true },
                    ].map(q => {
                      const cls = "flex items-center gap-3 px-4 py-3 rounded-lg border hover:border-indigo-300 hover:bg-indigo-50/50 transition text-sm";
                      const inner = (
                        <>
                          <span className={`w-8 h-8 rounded-lg ${q.bg} flex items-center justify-center text-base`}>{q.emoji}</span>
                          <div><p className="font-medium">{q.title}</p><p className="text-xs text-gray-400">{q.sub}</p></div>
                        </>
                      );
                      return q.internal
                        ? <Link key={q.title} to={q.href} className={cls}>{inner}</Link>
                        : <a key={q.title} href={q.href} className={cls}>{inner}</a>;
                    })}
                  </div>
                </div>
                <div className="bg-white rounded-xl border p-6">
                  <h3 className="font-bold text-sm mb-4">最近动态</h3>
                  <div className="text-center py-8 text-gray-400 text-sm">暂无动态</div>
                </div>
              </div>
            </div>
          )}

          {/* Profile */}
          {panel === 'profile' && (
            <div>
              <h1 className="text-2xl font-bold mb-6">个人资料</h1>
              <div className="bg-white rounded-xl border p-8 max-w-2xl">
                <div className="flex items-center gap-6 mb-8">
                  <div className="w-20 h-20 rounded-full bg-indigo-100 text-indigo-600 flex items-center justify-center text-2xl font-bold">{initial}</div>
                  <div>
                    <h2 className="text-lg font-bold">{displayName}</h2>
                    <p className="text-sm text-gray-400">{user?.email || user?.phone || ''}</p>
                  </div>
                </div>
                <div className="space-y-5">
                  <div>
                    <label className="block text-sm font-medium text-gray-700 mb-1.5">昵称</label>
                    <input type="text" value={nickname} onChange={e => setNickname(e.target.value)} className={INPUT} />
                  </div>
                  <div>
                    <label className="block text-sm font-medium text-gray-700 mb-1.5">邮箱</label>
                    <input type="email" value={email} onChange={e => setEmail(e.target.value)} className={INPUT} />
                  </div>
                  <div>
                    <label className="block text-sm font-medium text-gray-700 mb-1.5">手机号</label>
                    <input type="tel" value={phone} onChange={e => setPhone(e.target.value)} className={INPUT} />
                  </div>
                  <div>
                    <label className="block text-sm font-medium text-gray-700 mb-1.5">个人简介</label>
                    <textarea rows={3} value={bio} onChange={e => setBio(e.target.value)} className={`${INPUT} resize-none`} placeholder="介绍一下你自己..." />
                  </div>
                  {profileMsg && <p className="text-sm text-green-600">{profileMsg}</p>}
                  <button onClick={saveProfile} className="px-6 py-2.5 rounded-xl bg-indigo-600 text-white text-sm font-medium hover:bg-indigo-700 transition">保存修改</button>
                </div>
              </div>
            </div>
          )}

          {/* My Agents / Skills / Workflows */}
          {(['my-agents', 'my-skills', 'my-workflows'] as Panel[]).includes(panel) && (
            <div>
              <div className="flex items-center justify-between mb-6">
                <h1 className="text-2xl font-bold">{SIDEBAR.find(s => s.panel === panel)?.label}</h1>
                <button className="px-4 py-2 rounded-xl bg-indigo-600 text-white text-sm font-medium hover:bg-indigo-700 transition">+ 发布新项目</button>
              </div>
              <div className="bg-white rounded-xl border p-8 text-center">
                <p className="text-gray-400 text-sm mb-4">暂无已发布的项目</p>
                <p className="text-xs text-gray-300">在 Claw 控制台创建后可一键发布到市场</p>
              </div>
            </div>
          )}

          {/* My Claws */}
          {panel === 'my-claws' && <MyClawsPanel />}

          {/* API Keys */}
          {panel === 'api-keys' && (
            <div>
              <div className="flex items-center justify-between mb-6">
                <h1 className="text-2xl font-bold">API 密钥</h1>
                <button className="px-4 py-2 rounded-xl bg-indigo-600 text-white text-sm font-medium hover:bg-indigo-700 transition">+ 创建密钥</button>
              </div>
              <div className="bg-white rounded-xl border overflow-hidden">
                <table className="w-full text-sm">
                  <thead className="bg-gray-50">
                    <tr>
                      <th className="text-left px-6 py-3 font-medium text-gray-500">名称</th>
                      <th className="text-left px-6 py-3 font-medium text-gray-500">密钥</th>
                      <th className="text-left px-6 py-3 font-medium text-gray-500">创建时间</th>
                      <th className="text-left px-6 py-3 font-medium text-gray-500">操作</th>
                    </tr>
                  </thead>
                  <tbody>
                    <tr><td colSpan={4} className="px-6 py-8 text-center text-gray-400">暂无 API 密钥</td></tr>
                  </tbody>
                </table>
              </div>
              <p className="text-xs text-gray-400 mt-4">API 密钥用于通过 StarClaw Open API 管理你的资源。<Link to="/docs" className="text-indigo-500 hover:underline">查看文档</Link></p>
            </div>
          )}

          {/* Settings */}
          {panel === 'settings' && (
            <div>
              <h1 className="text-2xl font-bold mb-6">账号设置</h1>
              <div className="space-y-6 max-w-2xl">
                <div className="bg-white rounded-xl border p-6">
                  <h3 className="font-bold text-sm mb-4">修改密码</h3>
                  <div className="space-y-4">
                    <input type="password" placeholder="当前密码" className={INPUT} />
                    <input type="password" placeholder="新密码（至少 6 位）" className={INPUT} />
                    <button className="px-6 py-2.5 rounded-xl bg-indigo-600 text-white text-sm font-medium hover:bg-indigo-700 transition">更新密码</button>
                  </div>
                </div>
                <div className="bg-white rounded-xl border p-6">
                  <h3 className="font-bold text-sm mb-2">绑定手机号</h3>
                  <p className="text-xs text-gray-400 mb-4">绑定手机号后可使用短信验证码登录</p>
                  <div className="flex gap-2">
                    <select className="w-20 px-2 py-3 rounded-xl border border-gray-200 text-sm bg-white"><option>+86</option><option>+1</option><option>+852</option></select>
                    <input type="tel" placeholder="手机号" className={`flex-1 ${INPUT}`} />
                    <button className="px-4 py-3 rounded-xl border border-indigo-200 text-indigo-600 text-xs font-medium hover:bg-indigo-50 transition">发送验证码</button>
                  </div>
                </div>
                <div className="bg-white rounded-xl border border-red-100 p-6">
                  <h3 className="font-bold text-sm text-red-600 mb-2">危险操作</h3>
                  <p className="text-xs text-gray-400 mb-4">删除账号后所有数据将被清除，此操作不可恢复</p>
                  <button className="px-4 py-2 rounded-xl border border-red-200 text-red-500 text-sm hover:bg-red-50 transition">删除账号</button>
                </div>
              </div>
            </div>
          )}
        </main>
      </div>
    </div>
  );
}

// ─── My Claws (Node Binding) Panel ───

function MyClawsPanel() {
  const [nodes, setNodes] = useState<NodeBinding[]>([]);
  const [loading, setLoading] = useState(true);
  const [showBind, setShowBind] = useState(false);
  const [bindNodeId, setBindNodeId] = useState('');
  const [bindLocalUserId, setBindLocalUserId] = useState('');
  const [bindNodeName, setBindNodeName] = useState('');
  const [bindNodeAddr, setBindNodeAddr] = useState('');
  const [bindMsg, setBindMsg] = useState('');
  const [binding, setBinding] = useState(false);

  const INPUT = 'w-full px-4 py-3 rounded-xl border border-gray-200 text-sm focus:outline-none focus:ring-2 focus:ring-indigo-500 transition';

  async function loadNodes() {
    try {
      const r = await nodeAPI.list();
      setNodes(r.data?.nodes || []);
    } catch { /* ignore */ }
    setLoading(false);
  }

  useEffect(() => { loadNodes(); }, []);

  async function handleBind() {
    if (!bindNodeId || !bindLocalUserId) return;
    setBinding(true);
    setBindMsg('');
    try {
      await nodeAPI.bind({
        node_id: bindNodeId,
        local_user_id: bindLocalUserId,
        node_name: bindNodeName || undefined,
        node_addr: bindNodeAddr || undefined,
      });
      setShowBind(false);
      setBindNodeId('');
      setBindLocalUserId('');
      setBindNodeName('');
      setBindNodeAddr('');
      loadNodes();
    } catch (e: any) {
      setBindMsg(e.message || '绑定失败');
    }
    setBinding(false);
  }

  async function handleUnbind(nodeId: string) {
    if (!confirm('确认解绑此节点？')) return;
    try {
      await nodeAPI.unbind(nodeId);
      loadNodes();
    } catch { /* ignore */ }
  }

  function timeAgo(dateStr: string) {
    if (!dateStr) return '-';
    const d = new Date(dateStr);
    const diff = Math.floor((Date.now() - d.getTime()) / 1000);
    if (diff < 60) return '刚刚';
    if (diff < 3600) return `${Math.floor(diff / 60)} 分钟前`;
    if (diff < 86400) return `${Math.floor(diff / 3600)} 小时前`;
    return `${Math.floor(diff / 86400)} 天前`;
  }

  function isOnline(lastSeen: string) {
    if (!lastSeen) return false;
    return Date.now() - new Date(lastSeen).getTime() < 5 * 60 * 1000;
  }

  if (loading) return <div className="text-center py-12 text-gray-400">加载中...</div>;

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-2xl font-bold">我的节点</h1>
        <button onClick={() => setShowBind(true)}
          className="px-4 py-2 rounded-xl bg-indigo-600 text-white text-sm font-medium hover:bg-indigo-700 transition flex items-center gap-1.5">
          <Plus className="w-4 h-4" />绑定节点
        </button>
      </div>

      {/* Bind Dialog */}
      {showBind && (
        <div className="bg-white rounded-xl border p-6 mb-6 max-w-lg">
          <h3 className="font-bold text-sm mb-4">绑定 Claw 节点</h3>
          <div className="space-y-3">
            <div>
              <label className="block text-xs text-gray-500 mb-1">节点 ID (claw:xxxx) *</label>
              <input value={bindNodeId} onChange={e => setBindNodeId(e.target.value)} placeholder="claw:b49edd9c..." className={INPUT} />
            </div>
            <div>
              <label className="block text-xs text-gray-500 mb-1">本地用户 ID *</label>
              <input value={bindLocalUserId} onChange={e => setBindLocalUserId(e.target.value)} placeholder="Claw 上的用户 ID" className={INPUT} />
            </div>
            <div>
              <label className="block text-xs text-gray-500 mb-1">节点名称</label>
              <input value={bindNodeName} onChange={e => setBindNodeName(e.target.value)} placeholder="给这只小龙虾起个名字" className={INPUT} />
            </div>
            <div>
              <label className="block text-xs text-gray-500 mb-1">节点地址</label>
              <input value={bindNodeAddr} onChange={e => setBindNodeAddr(e.target.value)} placeholder="http://192.168.1.100:8080" className={INPUT} />
            </div>
            {bindMsg && <p className="text-sm text-red-500">{bindMsg}</p>}
            <div className="flex gap-2 pt-1">
              <button onClick={handleBind} disabled={binding || !bindNodeId || !bindLocalUserId}
                className="px-5 py-2.5 rounded-xl bg-indigo-600 text-white text-sm font-medium hover:bg-indigo-700 disabled:opacity-50 transition">
                {binding ? '绑定中...' : '确认绑定'}
              </button>
              <button onClick={() => setShowBind(false)} className="px-5 py-2.5 rounded-xl border text-sm text-gray-500 hover:bg-gray-50 transition">取消</button>
            </div>
          </div>
        </div>
      )}

      {/* Node List */}
      {nodes.length === 0 ? (
        <div className="bg-white rounded-xl border p-8 text-center">
          <Server className="w-12 h-12 text-gray-200 mx-auto mb-3" />
          <p className="text-gray-400 text-sm mb-2">暂无已绑定的 Claw 节点</p>
          <p className="text-xs text-gray-300">部署 Claw 后，点击上方「绑定节点」将其关联到你的账号</p>
        </div>
      ) : (
        <div className="grid gap-4">
          {nodes.map(node => (
            <div key={node.id} className="bg-white rounded-xl border p-5 flex items-center justify-between">
              <div className="flex items-center gap-4">
                <div className={`w-10 h-10 rounded-lg flex items-center justify-center ${isOnline(node.last_seen) ? 'bg-green-50' : 'bg-gray-100'}`}>
                  {isOnline(node.last_seen) ? <Wifi className="w-5 h-5 text-green-500" /> : <WifiOff className="w-5 h-5 text-gray-400" />}
                </div>
                <div>
                  <div className="flex items-center gap-2">
                    <span className="font-medium text-sm">{node.node_name || '未命名节点'}</span>
                    <span className={`text-[10px] px-1.5 py-0.5 rounded-full font-medium ${isOnline(node.last_seen) ? 'bg-green-100 text-green-700' : 'bg-gray-100 text-gray-500'}`}>
                      {isOnline(node.last_seen) ? '在线' : '离线'}
                    </span>
                    {node.node_version && <span className="text-[10px] px-1.5 py-0.5 rounded-full bg-indigo-50 text-indigo-600">{node.node_version}</span>}
                  </div>
                  <p className="text-xs text-gray-400 mt-0.5 font-mono">{node.node_id}</p>
                  <div className="flex items-center gap-3 mt-1 text-xs text-gray-400">
                    {node.node_addr && <span>{node.node_addr}</span>}
                    {node.node_region && <span>{node.node_region}</span>}
                    <span className="flex items-center gap-0.5"><Clock className="w-3 h-3" />{timeAgo(node.last_seen)}</span>
                  </div>
                </div>
              </div>
              <button onClick={() => handleUnbind(node.node_id)} className="p-2 rounded-lg text-gray-300 hover:text-red-500 hover:bg-red-50 transition" title="解绑节点">
                <Trash2 className="w-4 h-4" />
              </button>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
