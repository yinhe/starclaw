import { useEffect, useState } from 'react';
import { Plus, Trash2, Copy, Check, Key } from 'lucide-react';
import { dash } from '../lib/api';

interface APIKey {
  id: string;
  name: string;
  key_prefix: string;
  is_enabled: boolean;
  last_used: string | null;
  created_at: string;
}

export default function KeysPage() {
  const [keys, setKeys] = useState<APIKey[]>([]);
  const [newKeyName, setNewKeyName] = useState('');
  const [newKey, setNewKey] = useState('');
  const [copied, setCopied] = useState('');
  const [showCreate, setShowCreate] = useState(false);
  const [loading, setLoading] = useState(false);

  const load = () => dash.keys().then(r => setKeys(r.keys || [])).catch(console.error);

  useEffect(() => { load(); }, []);

  const createKey = async () => {
    if (!newKeyName.trim()) return;
    setLoading(true);
    try {
      const res = await dash.createKey(newKeyName.trim());
      setNewKey(res.key);
      setNewKeyName('');
      load();
    } catch (err) {
      console.error(err);
    } finally {
      setLoading(false);
    }
  };

  const deleteKey = async (id: string) => {
    if (!confirm('确定删除此 Key？删除后无法恢复。')) return;
    try {
      await dash.deleteKey(id);
      load();
    } catch (err) {
      console.error(err);
    }
  };

  const copyText = (text: string, id: string) => {
    navigator.clipboard.writeText(text);
    setCopied(id);
    setTimeout(() => setCopied(''), 2000);
  };

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-white">API Keys</h1>
          <p className="text-gray-400 text-sm mt-1">管理你的 API 密钥</p>
        </div>
        <button
          onClick={() => { setShowCreate(true); setNewKey(''); }}
          className="flex items-center gap-2 bg-amber-500 hover:bg-amber-400 text-gray-900 font-medium px-4 py-2 rounded-lg text-sm transition-colors cursor-pointer"
        >
          <Plus className="w-4 h-4" /> 创建 Key
        </button>
      </div>

      {/* New key created */}
      {newKey && (
        <div className="bg-green-500/10 border border-green-500/20 rounded-xl p-4 space-y-2">
          <p className="text-green-400 text-sm font-medium">新 Key 已创建，请立即复制：</p>
          <div className="flex items-center gap-2 bg-gray-800 rounded-lg p-3">
            <code className="text-amber-400 text-xs break-all flex-1">{newKey}</code>
            <button onClick={() => copyText(newKey, 'new')} className="p-1.5 rounded hover:bg-gray-700 cursor-pointer">
              {copied === 'new' ? <Check className="w-4 h-4 text-green-400" /> : <Copy className="w-4 h-4 text-gray-400" />}
            </button>
          </div>
          <p className="text-amber-400 text-xs">此 Key 仅显示一次。</p>
        </div>
      )}

      {/* Create dialog */}
      {showCreate && !newKey && (
        <div className="bg-gray-900 border border-gray-800 rounded-xl p-4 flex items-end gap-3">
          <div className="flex-1">
            <label className="block text-sm text-gray-400 mb-1">Key 名称</label>
            <input
              value={newKeyName}
              onChange={e => setNewKeyName(e.target.value)}
              placeholder="例如 Production Key"
              className="w-full bg-gray-800 border border-gray-700 rounded-lg px-3 py-2 text-white text-sm focus:outline-none focus:border-amber-500"
              onKeyDown={e => e.key === 'Enter' && createKey()}
            />
          </div>
          <button
            onClick={createKey}
            disabled={loading || !newKeyName.trim()}
            className="bg-amber-500 hover:bg-amber-400 disabled:opacity-50 text-gray-900 font-medium px-4 py-2 rounded-lg text-sm transition-colors cursor-pointer"
          >
            {loading ? '创建中...' : '创建'}
          </button>
          <button
            onClick={() => setShowCreate(false)}
            className="text-gray-400 hover:text-gray-200 px-3 py-2 text-sm cursor-pointer"
          >
            取消
          </button>
        </div>
      )}

      {/* Keys table */}
      <div className="bg-gray-900 border border-gray-800 rounded-xl overflow-hidden">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-gray-800 text-gray-400 text-left">
              <th className="px-5 py-3 font-medium">名称</th>
              <th className="px-5 py-3 font-medium">Key</th>
              <th className="px-5 py-3 font-medium">创建时间</th>
              <th className="px-5 py-3 font-medium">最后使用</th>
              <th className="px-5 py-3 font-medium w-12"></th>
            </tr>
          </thead>
          <tbody>
            {keys.length === 0 ? (
              <tr>
                <td colSpan={5} className="px-5 py-12 text-center text-gray-500">
                  <Key className="w-8 h-8 mx-auto mb-2 opacity-30" />
                  还没有 API Key
                </td>
              </tr>
            ) : (
              keys.map(k => (
                <tr key={k.id} className="border-b border-gray-800/50 hover:bg-gray-800/30">
                  <td className="px-5 py-3 text-white">{k.name || '未命名'}</td>
                  <td className="px-5 py-3">
                    <code className="text-gray-400 text-xs">{k.key_prefix}...</code>
                    <button onClick={() => copyText(k.key_prefix, k.id)} className="ml-2 p-1 rounded hover:bg-gray-700 cursor-pointer inline-flex align-middle">
                      {copied === k.id ? <Check className="w-3.5 h-3.5 text-green-400" /> : <Copy className="w-3.5 h-3.5 text-gray-500" />}
                    </button>
                  </td>
                  <td className="px-5 py-3 text-gray-400">{new Date(k.created_at).toLocaleDateString()}</td>
                  <td className="px-5 py-3 text-gray-400">{k.last_used ? new Date(k.last_used).toLocaleDateString() : '从未'}</td>
                  <td className="px-5 py-3">
                    <button onClick={() => deleteKey(k.id)} className="p-1.5 rounded hover:bg-red-500/10 text-gray-500 hover:text-red-400 cursor-pointer">
                      <Trash2 className="w-4 h-4" />
                    </button>
                  </td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>
    </div>
  );
}
