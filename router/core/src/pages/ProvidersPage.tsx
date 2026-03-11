import { useEffect, useState } from 'react';
import { admin, type ProviderInfo, type ModelInfo } from '../lib/api';

export default function ProvidersPage() {
  const [providers, setProviders] = useState<ProviderInfo[]>([]);
  const [models, setModels] = useState<ModelInfo[]>([]);
  const [filter, setFilter] = useState('');

  useEffect(() => {
    admin.providers().then((res) => {
      setProviders(res.providers || []);
      setModels(res.models || []);
    });
  }, []);

  const filtered = filter ? models.filter((m) => m.provider === filter) : models;

  return (
    <div>
      <h1 className="text-2xl font-bold mb-6">模型 / Provider</h1>

      <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-4 gap-3 mb-8">
        <button
          onClick={() => setFilter('')}
          className={`px-4 py-3 rounded-xl text-sm border transition-colors cursor-pointer ${
            !filter ? 'bg-rose-500/10 border-rose-500/30 text-rose-400' : 'bg-gray-900 border-gray-800 text-gray-400 hover:border-gray-700'
          }`}
        >
          全部 <span className="text-gray-500">({models.length})</span>
        </button>
        {providers.map((p) => (
          <button
            key={p.slug}
            onClick={() => setFilter(p.slug)}
            className={`px-4 py-3 rounded-xl text-sm border transition-colors cursor-pointer ${
              filter === p.slug ? 'bg-rose-500/10 border-rose-500/30 text-rose-400' : 'bg-gray-900 border-gray-800 text-gray-400 hover:border-gray-700'
            }`}
          >
            {p.slug} <span className="text-gray-500">({p.model_count})</span>
          </button>
        ))}
      </div>

      <div className="bg-gray-900 border border-gray-800 rounded-xl overflow-x-auto">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-gray-800 text-gray-400">
              <th className="text-left px-4 py-3 font-medium">模型名称</th>
              <th className="text-left px-4 py-3 font-medium">Provider</th>
              <th className="text-center px-4 py-3 font-medium">类型</th>
              <th className="text-right px-4 py-3 font-medium">上下文长度</th>
              <th className="text-right px-4 py-3 font-medium">输入价格</th>
              <th className="text-right px-4 py-3 font-medium">输出价格</th>
            </tr>
          </thead>
          <tbody>
            {filtered.map((m) => (
              <tr key={`${m.provider}/${m.name}`} className="border-b border-gray-800/50 hover:bg-gray-800/30">
                <td className="px-4 py-2.5 text-white font-mono text-xs">{m.name}</td>
                <td className="px-4 py-2.5 text-gray-300">{m.provider}</td>
                <td className="px-4 py-2.5 text-center">
                  <span className="text-xs px-1.5 py-0.5 rounded bg-gray-700 text-gray-300">{m.type || 'chat'}</span>
                </td>
                <td className="px-4 py-2.5 text-right text-gray-400">{m.context_length ? m.context_length.toLocaleString() : '-'}</td>
                <td className="px-4 py-2.5 text-right text-gray-400">{m.input_price ? `¥${m.input_price}` : '-'}</td>
                <td className="px-4 py-2.5 text-right text-gray-400">{m.output_price ? `¥${m.output_price}` : '-'}</td>
              </tr>
            ))}
            {filtered.length === 0 && (
              <tr><td colSpan={6} className="px-4 py-8 text-center text-gray-500">暂无模型</td></tr>
            )}
          </tbody>
        </table>
      </div>
    </div>
  );
}
