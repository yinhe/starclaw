import { useEffect, useState } from 'react';
import { admin, type UsageLog } from '../lib/api';

export default function LogsPage() {
  const [logs, setLogs] = useState<UsageLog[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [pages, setPages] = useState(1);
  const [model, setModel] = useState('');
  const [provider, setProvider] = useState('');

  useEffect(() => {
    admin.logs({ page, page_size: 30, model: model || undefined, provider: provider || undefined }).then((res) => {
      setLogs(res.logs || []);
      setTotal(res.total);
      setPages(res.pages);
    });
  }, [page, model, provider]);

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-2xl font-bold">请求日志 <span className="text-sm font-normal text-gray-500">({total})</span></h1>
        <div className="flex gap-2">
          <input value={model} onChange={(e) => { setModel(e.target.value); setPage(1); }} placeholder="模型" className="bg-gray-900 border border-gray-800 rounded-lg px-3 py-2 text-sm text-white placeholder-gray-600 w-40 focus:outline-none focus:border-rose-500/50" />
          <input value={provider} onChange={(e) => { setProvider(e.target.value); setPage(1); }} placeholder="Provider" className="bg-gray-900 border border-gray-800 rounded-lg px-3 py-2 text-sm text-white placeholder-gray-600 w-32 focus:outline-none focus:border-rose-500/50" />
        </div>
      </div>

      <div className="bg-gray-900 border border-gray-800 rounded-xl overflow-x-auto">
        <table className="w-full text-sm whitespace-nowrap">
          <thead>
            <tr className="border-b border-gray-800 text-gray-400">
              <th className="text-left px-4 py-3 font-medium">时间</th>
              <th className="text-left px-4 py-3 font-medium">用户ID</th>
              <th className="text-left px-4 py-3 font-medium">模型</th>
              <th className="text-left px-4 py-3 font-medium">Provider</th>
              <th className="text-right px-4 py-3 font-medium">Tokens</th>
              <th className="text-right px-4 py-3 font-medium">耗时</th>
              <th className="text-right px-4 py-3 font-medium">费用(分)</th>
              <th className="text-center px-4 py-3 font-medium">Via</th>
              <th className="text-center px-4 py-3 font-medium">状态</th>
            </tr>
          </thead>
          <tbody>
            {logs.map((l) => (
              <tr key={l.id} className="border-b border-gray-800/50 hover:bg-gray-800/30">
                <td className="px-4 py-2.5 text-gray-400">{new Date(l.created_at).toLocaleString()}</td>
                <td className="px-4 py-2.5 text-gray-500 font-mono text-xs">{l.user_id.slice(0, 8)}...</td>
                <td className="px-4 py-2.5 text-white">{l.model}</td>
                <td className="px-4 py-2.5 text-gray-300">{l.provider}</td>
                <td className="px-4 py-2.5 text-right text-gray-300">{l.total_tokens}</td>
                <td className="px-4 py-2.5 text-right text-gray-400">{l.duration}ms</td>
                <td className="px-4 py-2.5 text-right text-gray-300">{l.cost_cents}</td>
                <td className="px-4 py-2.5 text-center">
                  <span className={`text-xs px-1.5 py-0.5 rounded ${l.via === 'proxy' ? 'bg-blue-500/10 text-blue-400' : 'bg-gray-700 text-gray-400'}`}>{l.via}</span>
                </td>
                <td className="px-4 py-2.5 text-center">
                  <span className={`text-xs px-1.5 py-0.5 rounded ${l.status === 'ok' ? 'bg-emerald-500/10 text-emerald-400' : 'bg-red-500/10 text-red-400'}`}>{l.status}</span>
                </td>
              </tr>
            ))}
            {logs.length === 0 && (
              <tr><td colSpan={9} className="px-4 py-8 text-center text-gray-500">暂无数据</td></tr>
            )}
          </tbody>
        </table>
      </div>

      {pages > 1 && (
        <div className="flex items-center justify-center gap-2 mt-4">
          <button onClick={() => setPage(Math.max(1, page - 1))} disabled={page <= 1} className="px-3 py-1.5 rounded bg-gray-800 text-gray-300 text-sm disabled:opacity-40 cursor-pointer">上一页</button>
          <span className="text-sm text-gray-500">{page} / {pages}</span>
          <button onClick={() => setPage(Math.min(pages, page + 1))} disabled={page >= pages} className="px-3 py-1.5 rounded bg-gray-800 text-gray-300 text-sm disabled:opacity-40 cursor-pointer">下一页</button>
        </div>
      )}
    </div>
  );
}
