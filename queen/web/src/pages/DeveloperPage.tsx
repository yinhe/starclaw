import { useState, useEffect } from 'react';
import { Link } from 'react-router-dom';
import { LogoMark } from '../components/Logo';
import { marketplaceAPI, type MarketplaceItem } from '../lib/api';
import { isLoggedIn } from '../lib/auth';
import { Plus, Package, Clock, CheckCircle, Send, Pencil, Trash2, ChevronDown } from 'lucide-react';

const statusBadge: Record<string, { label: string; cls: string }> = {
  draft: { label: '草稿', cls: 'bg-gray-100 text-gray-500' },
  pending_review: { label: '审核中', cls: 'bg-amber-50 text-amber-600' },
  approved: { label: '已通过', cls: 'bg-emerald-50 text-emerald-600' },
  rejected: { label: '已拒绝', cls: 'bg-red-50 text-red-600' },
  published: { label: '已发布', cls: 'bg-blue-50 text-blue-600' },
  removed: { label: '已下架', cls: 'bg-gray-100 text-gray-400' },
};

const typeOptions = [
  { value: 'agent', label: 'Agent' },
  { value: 'skill', label: '技能' },
  { value: 'workflow', label: '工作流' },
  { value: 'mcp', label: 'MCP 工具' },
];

const emptyForm = { type: 'agent', name: '', description: '', icon: '', tags: '', config: '' };

export function DeveloperPage() {
  const [items, setItems] = useState<MarketplaceItem[]>([]);
  const [showForm, setShowForm] = useState(false);
  const [form, setForm] = useState({ ...emptyForm });
  const [editingId, setEditingId] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [expanded, setExpanded] = useState<string | null>(null);

  const load = async () => {
    try {
      const data = await marketplaceAPI.my();
      setItems(data.items || []);
    } catch { /* ignore */ }
  };

  useEffect(() => { load(); }, []);

  const handleSubmitForm = async () => {
    if (!form.name.trim()) { setError('请输入名称'); return; }
    setLoading(true);
    setError('');
    try {
      if (editingId) {
        await marketplaceAPI.update(editingId, form);
      } else {
        await marketplaceAPI.create(form);
      }
      setShowForm(false);
      setEditingId(null);
      setForm({ ...emptyForm });
      load();
    } catch (e: any) {
      setError(e.message || '操作失败');
    } finally {
      setLoading(false);
    }
  };

  const handleEdit = (item: MarketplaceItem) => {
    setForm({
      type: item.type,
      name: item.name,
      description: item.description,
      icon: item.icon,
      tags: item.tags,
      config: item.config,
    });
    setEditingId(item.id);
    setShowForm(true);
    setError('');
  };

  const handleDelete = async (id: string) => {
    if (!confirm('确定删除该提交？')) return;
    try {
      await marketplaceAPI.delete(id);
      load();
    } catch { /* ignore */ }
  };

  const handleSubmitForReview = async (id: string) => {
    try {
      await marketplaceAPI.submit(id);
      load();
    } catch (e: any) {
      alert(e.message || '提交失败');
    }
  };

  if (!isLoggedIn()) {
    return (
      <div className="bg-gray-50 min-h-screen flex items-center justify-center">
        <div className="text-center">
          <h2 className="text-xl font-bold text-gray-900 mb-2">请先登录</h2>
          <p className="text-gray-500 mb-4">登录后即可提交作品到市场</p>
          <Link to="/auth" className="px-4 py-2 bg-indigo-600 text-white rounded-lg text-sm hover:bg-indigo-700 transition">
            前往登录
          </Link>
        </div>
      </div>
    );
  }

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
            <span className="text-sm font-medium text-indigo-600">开发者中心</span>
          </div>
          <div className="flex items-center gap-4">
            <Link to="/marketplace" className="text-sm text-gray-500 hover:text-gray-900 transition">市场</Link>
            <Link to="/dashboard" className="text-sm text-gray-500 hover:text-gray-900 transition">控制台</Link>
          </div>
        </div>
      </header>

      <main className="pt-14">
        {/* Hero */}
        <section className="py-12 bg-gradient-to-b from-indigo-50/50 to-transparent">
          <div className="max-w-4xl mx-auto px-6">
            <h1 className="text-3xl font-extrabold">开发者中心</h1>
            <p className="mt-2 text-gray-500">提交你的 Agent、技能、工作流或 MCP 工具到 StarClaw 市场</p>
            <button onClick={() => { setShowForm(true); setEditingId(null); setForm({ ...emptyForm }); setError(''); }}
              className="mt-4 inline-flex items-center gap-2 px-4 py-2.5 bg-indigo-600 text-white rounded-xl text-sm font-medium hover:bg-indigo-700 transition">
              <Plus size={16} /> 提交新作品
            </button>
          </div>
        </section>

        {/* Submit / Edit form */}
        {showForm && (
          <section className="py-6">
            <div className="max-w-4xl mx-auto px-6">
              <div className="bg-white rounded-2xl border border-gray-200 p-6">
                <h3 className="font-bold mb-4">{editingId ? '编辑作品' : '提交新作品'}</h3>

                <div className="grid grid-cols-2 gap-4 mb-4">
                  <div>
                    <label className="text-xs text-gray-500 mb-1 block">类型</label>
                    <select value={form.type} onChange={e => setForm({ ...form, type: e.target.value })}
                      className="w-full px-3 py-2 rounded-lg border border-gray-200 text-sm focus:outline-none focus:ring-2 focus:ring-indigo-500">
                      {typeOptions.map(t => <option key={t.value} value={t.value}>{t.label}</option>)}
                    </select>
                  </div>
                  <div>
                    <label className="text-xs text-gray-500 mb-1 block">名称 *</label>
                    <input value={form.name} onChange={e => setForm({ ...form, name: e.target.value })}
                      placeholder="如：智能客服 Agent"
                      className="w-full px-3 py-2 rounded-lg border border-gray-200 text-sm focus:outline-none focus:ring-2 focus:ring-indigo-500" />
                  </div>
                </div>

                <div className="mb-4">
                  <label className="text-xs text-gray-500 mb-1 block">描述</label>
                  <textarea value={form.description} onChange={e => setForm({ ...form, description: e.target.value })}
                    rows={3} placeholder="详细描述你的作品功能、使用场景..."
                    className="w-full px-3 py-2 rounded-lg border border-gray-200 text-sm focus:outline-none focus:ring-2 focus:ring-indigo-500 resize-none" />
                </div>

                <div className="grid grid-cols-2 gap-4 mb-4">
                  <div>
                    <label className="text-xs text-gray-500 mb-1 block">图标 URL</label>
                    <input value={form.icon} onChange={e => setForm({ ...form, icon: e.target.value })}
                      placeholder="https://..."
                      className="w-full px-3 py-2 rounded-lg border border-gray-200 text-sm focus:outline-none focus:ring-2 focus:ring-indigo-500" />
                  </div>
                  <div>
                    <label className="text-xs text-gray-500 mb-1 block">标签（逗号分隔）</label>
                    <input value={form.tags} onChange={e => setForm({ ...form, tags: e.target.value })}
                      placeholder="客服,自动化,RAG"
                      className="w-full px-3 py-2 rounded-lg border border-gray-200 text-sm focus:outline-none focus:ring-2 focus:ring-indigo-500" />
                  </div>
                </div>

                <div className="mb-4">
                  <label className="text-xs text-gray-500 mb-1 block">配置 (JSON)</label>
                  <textarea value={form.config} onChange={e => setForm({ ...form, config: e.target.value })}
                    rows={4} placeholder='{"system_prompt": "你是...", "tools": ["web_search"]}'
                    className="w-full px-3 py-2 rounded-lg border border-gray-200 text-sm font-mono focus:outline-none focus:ring-2 focus:ring-indigo-500 resize-none" />
                </div>

                {error && <p className="text-sm text-red-500 mb-3">{error}</p>}

                <div className="flex gap-3">
                  <button onClick={handleSubmitForm} disabled={loading}
                    className="px-4 py-2 bg-indigo-600 text-white rounded-lg text-sm font-medium hover:bg-indigo-700 transition disabled:opacity-50">
                    {loading ? '提交中...' : editingId ? '保存修改' : '创建并提交审核'}
                  </button>
                  <button onClick={() => { setShowForm(false); setEditingId(null); }}
                    className="px-4 py-2 bg-gray-100 text-gray-600 rounded-lg text-sm hover:bg-gray-200 transition">
                    取消
                  </button>
                </div>
              </div>
            </div>
          </section>
        )}

        {/* My submissions */}
        <section className="py-6">
          <div className="max-w-4xl mx-auto px-6">
            <h3 className="font-bold text-lg mb-4">我的提交</h3>

            {items.length === 0 ? (
              <div className="bg-white rounded-2xl border border-gray-100 p-12 text-center">
                <Package size={40} className="mx-auto text-gray-300 mb-3" />
                <p className="text-gray-400">还没有提交作品</p>
                <p className="text-gray-400 text-sm mt-1">点击上方「提交新作品」开始</p>
              </div>
            ) : (
              <div className="space-y-3">
                {items.map(item => {
                  const sb = statusBadge[item.status] || statusBadge.draft;
                  const isExpanded = expanded === item.id;
                  const canEdit = item.status === 'draft' || item.status === 'rejected';
                  const canSubmit = item.status === 'draft' || item.status === 'rejected';

                  return (
                    <div key={item.id} className="bg-white rounded-2xl border border-gray-100 overflow-hidden">
                      <div className="flex items-center gap-4 px-5 py-4 cursor-pointer hover:bg-gray-50 transition"
                        onClick={() => setExpanded(isExpanded ? null : item.id)}>
                        <div className="w-10 h-10 rounded-xl bg-indigo-50 flex items-center justify-center">
                          <Package size={18} className="text-indigo-500" />
                        </div>
                        <div className="flex-1 min-w-0">
                          <div className="flex items-center gap-2">
                            <span className="font-medium text-sm">{item.name}</span>
                            <span className="text-[10px] px-1.5 py-0.5 rounded bg-gray-100 text-gray-500">
                              {typeOptions.find(t => t.value === item.type)?.label || item.type}
                            </span>
                            <span className={`text-[10px] px-1.5 py-0.5 rounded font-medium ${sb.cls}`}>{sb.label}</span>
                          </div>
                          <p className="text-xs text-gray-400 mt-0.5 truncate">{item.description || '无描述'}</p>
                        </div>
                        <ChevronDown size={16} className={`text-gray-400 transition ${isExpanded ? 'rotate-180' : ''}`} />
                      </div>

                      {isExpanded && (
                        <div className="px-5 pb-4 border-t border-gray-100 pt-3">
                          <div className="grid grid-cols-3 gap-3 text-xs text-gray-400 mb-3">
                            <div>版本: v{item.version}</div>
                            <div>下载: {item.downloads}</div>
                            <div>创建: {new Date(item.created_at).toLocaleString('zh-CN')}</div>
                          </div>

                          {item.tags && (
                            <div className="flex flex-wrap gap-1.5 mb-3">
                              {item.tags.split(',').map(t => (
                                <span key={t} className="text-[10px] px-2 py-0.5 rounded-full bg-gray-100 text-gray-500">
                                  {t.trim()}
                                </span>
                              ))}
                            </div>
                          )}

                          {/* Review feedback */}
                          {item.review_note && (
                            <div className={`text-xs p-3 rounded-lg mb-3 ${
                              item.status === 'rejected' ? 'bg-red-50 text-red-600' : 'bg-gray-50 text-gray-500'
                            }`}>
                              <span className="font-medium">审核反馈：</span> {item.review_note}
                              {item.reviewed_at && (
                                <span className="ml-2 text-gray-400">({new Date(item.reviewed_at).toLocaleString('zh-CN')})</span>
                              )}
                            </div>
                          )}

                          {/* Pending notice */}
                          {item.status === 'pending_review' && (
                            <div className="text-xs p-3 rounded-lg mb-3 bg-amber-50 text-amber-600 flex items-center gap-2">
                              <Clock size={12} /> 审核中，通常 1-3 个工作日内完成
                            </div>
                          )}

                          {/* Approved notice */}
                          {item.status === 'approved' && (
                            <div className="text-xs p-3 rounded-lg mb-3 bg-emerald-50 text-emerald-600 flex items-center gap-2">
                              <CheckCircle size={12} /> 审核通过，已在市场上架
                            </div>
                          )}

                          {/* Actions */}
                          <div className="flex gap-2">
                            {canSubmit && (
                              <button onClick={() => handleSubmitForReview(item.id)}
                                className="flex items-center gap-1.5 px-3 py-1.5 bg-indigo-50 text-indigo-600 rounded-lg text-xs font-medium hover:bg-indigo-100 transition">
                                <Send size={12} /> {item.status === 'rejected' ? '重新提交审核' : '提交审核'}
                              </button>
                            )}
                            {canEdit && (
                              <button onClick={() => handleEdit(item)}
                                className="flex items-center gap-1.5 px-3 py-1.5 bg-gray-100 text-gray-600 rounded-lg text-xs hover:bg-gray-200 transition">
                                <Pencil size={12} /> 编辑
                              </button>
                            )}
                            <button onClick={() => handleDelete(item.id)}
                              className="flex items-center gap-1.5 px-3 py-1.5 bg-red-50 text-red-500 rounded-lg text-xs hover:bg-red-100 transition ml-auto">
                              <Trash2 size={12} /> 删除
                            </button>
                          </div>
                        </div>
                      )}
                    </div>
                  );
                })}
              </div>
            )}
          </div>
        </section>

        {/* Guidelines */}
        <section className="py-8">
          <div className="max-w-4xl mx-auto px-6">
            <div className="bg-white rounded-2xl border border-gray-100 p-6">
              <h3 className="font-bold mb-4">提交指南</h3>
              <div className="grid md:grid-cols-2 gap-6 text-sm text-gray-500">
                <div>
                  <h4 className="font-medium text-gray-700 mb-2">审核标准</h4>
                  <ul className="space-y-1.5">
                    <li>• 名称清晰、描述准确</li>
                    <li>• 配置完整、可正常运行</li>
                    <li>• 不含违规或恶意内容</li>
                    <li>• 与现有作品不重复</li>
                  </ul>
                </div>
                <div>
                  <h4 className="font-medium text-gray-700 mb-2">审核流程</h4>
                  <ul className="space-y-1.5">
                    <li>• 提交后进入审核队列（1-3 工作日）</li>
                    <li>• 通过后自动上架到市场</li>
                    <li>• 拒绝后可修改并重新提交</li>
                    <li>• 上架后可随时更新版本</li>
                  </ul>
                </div>
              </div>
            </div>
          </div>
        </section>
      </main>

      <footer className="border-t py-8 text-center text-sm text-gray-400">
        <p>&copy; 2026 StarClaw. MIT License.</p>
      </footer>
    </div>
  );
}
