import { useEffect, useState } from 'react';
import { FileText, ChevronLeft, ChevronRight, Search, AlertCircle, CheckCircle } from 'lucide-react';
import { dash } from '../lib/api';

interface LogEntry {
  id: string;
  model: string;
  endpoint: string;
  provider: string;
  prompt_tokens: number;
  completion_tokens: number;
  total_tokens: number;
  cost_cents: number;
  duration: number;
  status: string;
  error_msg: string;
  via: string;
  created_at: string;
}

export default function LogsPage() {
  const [logs, setLogs] = useState<LogEntry[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [pages, setPages] = useState(1);
  const [modelFilter, setModelFilter] = useState('');
  const [statusFilter, setStatusFilter] = useState('');
  const [loading, setLoading] = useState(true);

  const load = (p: number) => {
    setLoading(true);
    dash.logs({
      page: p,
      page_size: 30,
      model: modelFilter || undefined,
      status: statusFilter || undefined,
    }).then(r => {
      setLogs(r.logs || []);
      setTotal(r.total);
      setPage(r.page);
      setPages(r.pages);
    }).catch(console.error).finally(() => setLoading(false));
  };

  useEffect(() => { load(1); }, [modelFilter, statusFilter]);

  const fmt = (cents: number) => `¥${(cents / 100).toFixed(4)}`;

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-white">请求日志</h1>
          <p className="text-gray-400 text-sm mt-1">每个 API 调用的详细记录（共 {total} 条）</p>
        </div>
      </div>

      {/* Filters */}
      <div className="flex items-center gap-3">
        <div className="relative flex-1 max-w-xs">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-gray-500" />
          <input
            value={modelFilter}
            onChange={e => setModelFilter(e.target.value)}
            placeholder="搜索模型名称..."
            className="w-full bg-gray-900 border border-gray-800 rounded-lg pl-9 pr-3 py-2 text-white text-sm focus:outline-none focus:border-amber-500"
          />
        </div>
        <select
          value={statusFilter}
          onChange={e => setStatusFilter(e.target.value)}
          className="bg-gray-900 border border-gray-800 rounded-lg px-3 py-2 text-white text-sm focus:outline-none focus:border-amber-500"
        >
          <option value="">全部状态</option>
          <option value="ok">成功</option>
          <option value="error">失败</option>
        </select>
      </div>

      {/* Logs table */}
      <div className="bg-gray-900 border border-gray-800 rounded-xl overflow-hidden">
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-gray-800 text-gray-400 text-left text-xs">
                <th className="px-4 py-3 font-medium">时间</th>
                <th className="px-4 py-3 font-medium">状态</th>
                <th className="px-4 py-3 font-medium">模型</th>
                <th className="px-4 py-3 font-medium">端点</th>
                <th className="px-4 py-3 font-medium text-right">输入</th>
                <th className="px-4 py-3 font-medium text-right">输出</th>
                <th className="px-4 py-3 font-medium text-right">耗时</th>
                <th className="px-4 py-3 font-medium text-right">费用</th>
                <th className="px-4 py-3 font-medium">线路</th>
              </tr>
            </thead>
            <tbody>
              {loading ? (
                <tr>
                  <td colSpan={9} className="px-4 py-12 text-center text-gray-500">加载中...</td>
                </tr>
              ) : logs.length === 0 ? (
                <tr>
                  <td colSpan={9} className="px-4 py-12 text-center text-gray-500">
                    <FileText className="w-8 h-8 mx-auto mb-2 opacity-30" />
                    暂无日志记录
                  </td>
                </tr>
              ) : (
                logs.map(l => (
                  <tr key={l.id} className="border-b border-gray-800/50 hover:bg-gray-800/30">
                    <td className="px-4 py-2.5 text-gray-400 text-xs whitespace-nowrap">{new Date(l.created_at).toLocaleString()}</td>
                    <td className="px-4 py-2.5">
                      {l.status === 'ok' ? (
                        <CheckCircle className="w-4 h-4 text-green-400" />
                      ) : (
                        <span title={l.error_msg}>
                          <AlertCircle className="w-4 h-4 text-red-400" />
                        </span>
                      )}
                    </td>
                    <td className="px-4 py-2.5">
                      <code className="text-amber-400 text-xs bg-amber-500/10 px-1.5 py-0.5 rounded">{l.model}</code>
                    </td>
                    <td className="px-4 py-2.5 text-gray-500 text-xs">{l.endpoint}</td>
                    <td className="px-4 py-2.5 text-gray-400 text-xs text-right">{l.prompt_tokens}</td>
                    <td className="px-4 py-2.5 text-gray-400 text-xs text-right">{l.completion_tokens}</td>
                    <td className="px-4 py-2.5 text-gray-400 text-xs text-right">{l.duration}ms</td>
                    <td className="px-4 py-2.5 text-white text-xs text-right">{fmt(l.cost_cents)}</td>
                    <td className="px-4 py-2.5">
                      <span className={`text-xs px-1.5 py-0.5 rounded ${
                        l.via === 'proxy' ? 'bg-blue-500/10 text-blue-400' : 'bg-green-500/10 text-green-400'
                      }`}>
                        {l.via === 'proxy' ? '中转' : '直连'}
                      </span>
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>

        {/* Pagination */}
        {pages > 1 && (
          <div className="flex items-center justify-between px-4 py-3 border-t border-gray-800">
            <div className="text-xs text-gray-500">
              第 {page} / {pages} 页，共 {total} 条
            </div>
            <div className="flex items-center gap-2">
              <button
                onClick={() => load(page - 1)}
                disabled={page <= 1}
                className="p-1.5 rounded hover:bg-gray-800 disabled:opacity-30 text-gray-400 cursor-pointer disabled:cursor-not-allowed"
              >
                <ChevronLeft className="w-4 h-4" />
              </button>
              <button
                onClick={() => load(page + 1)}
                disabled={page >= pages}
                className="p-1.5 rounded hover:bg-gray-800 disabled:opacity-30 text-gray-400 cursor-pointer disabled:cursor-not-allowed"
              >
                <ChevronRight className="w-4 h-4" />
              </button>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
